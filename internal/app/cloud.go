package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	rs "radlog/internal/sync"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ---------- nuvem no Google Drive (fase 2; spec ux/nuvem.html) ----------
//
// Cada aparelho sobe os próprios dados para a pasta "RadLog" do Drive da
// pessoa, cifrados com a chave do código de recuperação (internal/sync):
//   - um pacote por mês: minutos do GMC-800 e do RC102, segundos de alerta e
//     anexos das medições daquele mês;
//   - um pacote por arquivo de espectro (fechado, sobe uma vez);
//   - um pacote "comum" com os pequenos (alertas encerrados, saídas,
//     medições, anotações, atividade, históricos);
//   - um arquivo do aparelho com o nome que ele dá a si mesmo.
// Os pacotes são os mesmos da sincronização pela rede (buildDelta) e quem
// baixa junta com a mesma mescla do "Importar backup". Um resumo por pacote
// diz o que mudou desde o último envio.
//
// O e-mail da conta fica só na tela e no nuvem.json deste PC: nunca vai para
// o registro, o backup, os PDFs nem a nuvem. O código e o acesso do Google
// ficam em nuvem-chave.bin (DPAPI no Windows). Os dois arquivos ficam fora do
// backup e da sincronização (as listas de lá são fechadas).

// googleClientID e googleClientSecret entram no build pelos segredos do
// GitHub (-ldflags -X); sem eles a nuvem fica indisponível nesta cópia.
var (
	googleClientID     string
	googleClientSecret string
)

// trocados nos testes
var (
	cloudOAuth   = func() *rs.OAuth { return rs.Google(googleClientID, googleClientSecret) }
	cloudDriveFn = func(tok func(context.Context) (string, error)) *rs.Drive { return rs.NewDrive(tok) }
	cloudOpenURL = func(a *App, u string) {
		if a.ctx != nil {
			wruntime.BrowserOpenURL(a.ctx, u)
		}
	}
)

const (
	cloudFolderName = "RadLog"
	cloudEvery      = 15 * time.Minute
	cloudSkewMax    = 5 * 60 // s
	cloudNameMax    = 40
	cloudLoginMax   = 10 * time.Minute
)

// errCloudGone: o arquivo de metadados sumiu ou não abre mais com o código.
var errCloudGone = errors.New("a nuvem foi apagada ou recomeçada em outro aparelho: desconecte e entre de novo com o código novo")

// CloudDevice é um aparelho da nuvem (o nome vai cifrado junto dos dados).
type CloudDevice struct {
	Name   string `json:"name"`
	LastUp int64  `json:"lastUp"` // ms; 0 = ainda não subiu nada
	Me     bool   `json:"me"`
}

// CloudInfo é o estado do cartão "Nuvem (Google Drive)".
type CloudInfo struct {
	// off, wait, ok, send, offline, login, clock
	State    string        `json:"state"`
	Ready    bool          `json:"ready"` // esta cópia tem a chave do Google
	Email    string        `json:"email,omitempty"`
	LastSync int64         `json:"lastSync,omitempty"`
	Queue    []string      `json:"queue,omitempty"` // meses na fila ("2026-11")
	Used     int64         `json:"used,omitempty"`  // bytes no Drive
	Devices  []CloudDevice `json:"devices,omitempty"`
	Auto     bool          `json:"auto"`
	SkewSec  int           `json:"skewSec,omitempty"` // relógio daqui − Google
	DupOf    string        `json:"dupOf,omitempty"`
	SendNow  string        `json:"sendNow,omitempty"` // mês sendo enviado
	SendI    int           `json:"sendI,omitempty"`
	SendN    int           `json:"sendN,omitempty"`
	Err      string        `json:"err,omitempty"` // último erro do Google
}

// CloudLoginDone é o fim do login: next diz qual diálogo abre.
type CloudLoginDone struct {
	Info   CloudInfo `json:"info"`
	Next   string    `json:"next"`   // "new", "existing" ou "" (só entrou de novo)
	LastUp int64     `json:"lastUp"` // ms do último envio na nuvem existente
}

type cloudDev struct {
	Name   string `json:"name"`
	LastUp int64  `json:"lastUp"`
}

// cloudFile é o nuvem.json.
type cloudFile struct {
	On        bool                `json:"on"`
	DevID     string              `json:"devId,omitempty"`
	Name      string              `json:"name,omitempty"`
	NoAuto    bool                `json:"noAuto,omitempty"`
	LastSync  int64               `json:"lastSync,omitempty"`
	LastUp    int64               `json:"lastUp,omitempty"`
	Folder    string              `json:"folder,omitempty"`
	Salt      []byte              `json:"salt,omitempty"`
	Email     string              `json:"email,omitempty"`
	NeedLogin bool                `json:"needLogin,omitempty"`
	Used      int64               `json:"used,omitempty"`
	Queue     []string            `json:"queue,omitempty"`
	Up        map[string]string   `json:"up,omitempty"`   // pacote → resumo do que subiu
	Seen      map[string]string   `json:"seen,omitempty"` // arquivo de outro aparelho → md5 já juntado
	Devs      map[string]cloudDev `json:"devs,omitempty"` // arquivo de aparelho → nome e último envio
	Rev       int                 `json:"rev,omitempty"`  // cloudRev desta cópia do app
}

// cloudRev sobe quando a mescla passa a aproveitar algo que antes ficava de
// fora: a nuvem baixa e junta de novo o que já tinha visto.
//
//	1: 2.79.1, anexos das medições (N1)
const cloudRev = 1

// cloudSecret é o nuvem-chave.bin.
type cloudSecret struct {
	Code    string `json:"code"`
	Refresh string `json:"refresh"`
}

type cloudPending struct {
	tok    rs.Token
	folder string
	meta   *rs.Meta
}

type cloudState struct {
	mu        sync.Mutex
	loaded    bool
	f         cloudFile
	sec       cloudSecret
	keys      *rs.Keys
	access    string
	expiry    time.Time
	offline   bool
	skew      int
	err       string
	login     *rs.Login
	loginCtx  context.Context
	loginStop context.CancelFunc
	pend      *cloudPending
	run       context.CancelFunc
	sendNow   string
	sendI     int
	sendN     int
	kick      chan struct{}
	// gen muda ao ligar e ao desconectar: uma sincronização que termina
	// depois disso não grava mais nada (auditoria 2.79.0, N8)
	gen int
}

func (a *App) cloudPath() string    { return filepath.Join(a.dataDir, "nuvem.json") }
func (a *App) cloudKeyPath() string { return filepath.Join(a.dataDir, "nuvem-chave.bin") }

// cloudLoadLocked lê os dois arquivos na primeira vez.
func (a *App) cloudLoadLocked() {
	c := &a.cloud
	if c.kick == nil {
		c.kick = make(chan struct{}, 1)
	}
	if c.loaded || a.dataDir == "" {
		return
	}
	c.loaded = true
	a.readStateJSON(a.cloudPath(), "nuvem", &c.f)
	if b, err := os.ReadFile(a.cloudKeyPath()); err == nil {
		if p, err := unprotect(b); err == nil {
			_ = json.Unmarshal(p, &c.sec)
		} else {
			a.logf("nuvem: não consegui abrir a chave guardada (%v)", err)
		}
	}
	if c.f.On {
		if k, err := rs.DeriveKeys(c.sec.Code, c.f.Salt); err == nil && c.sec.Code != "" {
			c.keys = k
		} else {
			// chave ilegível (pasta copiada de outro PC, Windows reinstalado):
			// entrar de novo e digitar o código (N3)
			c.f.NeedLogin = true
		}
		if c.f.Rev < cloudRev {
			seen := map[string]string{}
			if m, ok := c.f.Seen[rs.MetaFile]; ok {
				seen[rs.MetaFile] = m
			}
			c.f.Seen, c.f.Rev = seen, cloudRev
			a.cloudSaveLocked()
		}
	}
}

func (a *App) cloudSaveLocked() {
	c := &a.cloud
	if a.dataDir == "" {
		return
	}
	b, _ := json.MarshalIndent(c.f, "", "  ")
	if err := writeFileAtomic(a.cloudPath(), b, 0o600); err != nil {
		a.logf("nuvem: não consegui gravar o estado (%v)", err)
	}
}

func (a *App) cloudSaveSecretLocked() {
	c := &a.cloud
	b, _ := json.Marshal(c.sec)
	p, err := protect(b)
	if err == nil {
		err = writeFileAtomic(a.cloudKeyPath(), p, 0o600)
	}
	if err != nil {
		a.logf("nuvem: não consegui gravar a chave (%v)", err)
	}
}

func (a *App) cloudInfoLocked() CloudInfo {
	c := &a.cloud
	f := &c.f
	info := CloudInfo{Ready: googleClientID != "", Auto: !f.NoAuto, LastSync: f.LastSync, Queue: f.Queue, Used: f.Used, Err: c.err}
	switch {
	case c.login != nil:
		info.State = "wait"
	case !f.On:
		info = CloudInfo{State: "off", Ready: info.Ready, Auto: true}
		return info
	case f.NeedLogin:
		info.State = "login"
	case c.run != nil && c.sendN > 0:
		info.State = "send"
		info.SendNow, info.SendI, info.SendN = c.sendNow, c.sendI, c.sendN
	case c.err != "":
		info.State = "err" // amarelo na hora (N9)
	case c.offline:
		info.State = "offline"
	case c.skew > cloudSkewMax || c.skew < -cloudSkewMax:
		info.State = "clock"
	default:
		info.State = "ok"
	}
	info.SkewSec = c.skew
	info.Email = f.Email
	if f.On {
		info.Devices = append(info.Devices, CloudDevice{Name: f.Name, LastUp: f.LastUp, Me: true})
		var others []CloudDevice
		for _, d := range f.Devs {
			others = append(others, CloudDevice{Name: d.Name, LastUp: d.LastUp})
		}
		sort.Slice(others, func(i, j int) bool { return others[i].Name < others[j].Name })
		info.Devices = append(info.Devices, others...)
	}
	return info
}

// GetCloud devolve o estado da nuvem.
func (a *App) GetCloud() CloudInfo {
	a.cloud.mu.Lock()
	defer a.cloud.mu.Unlock()
	a.cloudLoadLocked()
	return a.cloudInfoLocked()
}

func (a *App) emitCloud() { a.emit("cloud", a.GetCloud()) }

func (a *App) cloudKick() {
	a.cloud.mu.Lock()
	a.cloudLoadLocked()
	k := a.cloud.kick
	a.cloud.mu.Unlock()
	select {
	case k <- struct{}{}:
	default:
	}
}

// ---------- login ----------

// CloudLogin abre o navegador no login do Google (ou de novo, se já abriu).
func (a *App) CloudLogin() (CloudInfo, error) {
	c := &a.cloud
	c.mu.Lock()
	a.cloudLoadLocked()
	if c.login != nil {
		u := c.login.URL
		c.mu.Unlock()
		cloudOpenURL(a, u)
		return a.GetCloud(), nil
	}
	l, err := cloudOAuth().Start()
	if err != nil {
		c.mu.Unlock()
		return a.GetCloud(), err
	}
	c.login = l
	c.loginCtx, c.loginStop = context.WithTimeout(context.Background(), cloudLoginMax)
	c.mu.Unlock()
	cloudOpenURL(a, l.URL)
	a.emitCloud()
	return a.GetCloud(), nil
}

// CloudLoginWait espera o navegador voltar. Com a nuvem já ligada (o Google
// pediu para entrar de novo) só troca o acesso; senão olha se a conta já tem
// uma nuvem do RadLog.
func (a *App) CloudLoginWait() (*CloudLoginDone, error) {
	c := &a.cloud
	c.mu.Lock()
	l, ctx := c.login, c.loginCtx
	c.mu.Unlock()
	if l == nil {
		return nil, nil
	}
	code, err := l.Wait(ctx)
	var tok rs.Token
	if err == nil {
		tok, err = cloudOAuth().Exchange(ctx, l, code)
	}
	c.mu.Lock()
	if c.login == l {
		c.login = nil
		c.loginStop()
	}
	on := c.f.On && c.keys != nil
	c.mu.Unlock()
	defer a.emitCloud()
	if errors.Is(err, rs.ErrCanceled) {
		return &CloudLoginDone{Info: a.GetCloud()}, nil
	}
	if err != nil {
		a.logf("nuvem: login com o Google não deu certo (%v)", err)
		return nil, err
	}
	if on {
		c.mu.Lock()
		c.sec.Refresh = tok.Refresh
		c.access, c.expiry = tok.Access, tok.Expiry
		c.f.NeedLogin, c.f.Email, c.err = false, tok.Email, ""
		a.cloudSaveLocked()
		a.cloudSaveSecretLocked()
		c.mu.Unlock()
		a.logf("nuvem: entrou de novo no Google")
		a.cloudKick()
		return &CloudLoginDone{Info: a.GetCloud()}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	d := a.cloudDriveWith(func(context.Context) (string, error) { return tok.Access, nil })
	p := &cloudPending{tok: tok}
	done := &CloudLoginDone{Next: "new"}
	if p.folder, err = d.FindFolder(ctx, cloudFolderName); err != nil {
		return nil, err
	}
	if p.folder != "" {
		files, err := d.List(ctx, p.folder)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			if f.Name == rs.MetaFile {
				b, err := d.Get(ctx, f.ID)
				if err != nil {
					return nil, err
				}
				var m rs.Meta
				if json.Unmarshal(b, &m) == nil {
					p.meta = &m
					done.Next = "existing"
				}
			}
			if ms := f.Modified.UnixMilli(); ms > done.LastUp {
				done.LastUp = ms
			}
		}
	}
	c.mu.Lock()
	c.pend = p
	c.mu.Unlock()
	done.Info = a.GetCloud()
	return done, nil
}

// CloudCancel desiste do login ou da configuração.
func (a *App) CloudCancel() CloudInfo {
	c := &a.cloud
	c.mu.Lock()
	if c.login != nil {
		c.loginStop()
		c.login.Close()
		c.login = nil
	}
	c.pend = nil
	c.mu.Unlock()
	a.emitCloud()
	return a.GetCloud()
}

func (a *App) cloudPending() (*cloudPending, error) {
	a.cloud.mu.Lock()
	defer a.cloud.mu.Unlock()
	if a.cloud.pend == nil {
		return nil, errors.New("entre com o Google de novo")
	}
	return a.cloud.pend, nil
}

// CloudFinishSetup cria a nuvem com o código conferido (primeiro aparelho).
func (a *App) CloudFinishSetup(code string) (CloudInfo, error) {
	p, err := a.cloudPending()
	if err != nil {
		return a.GetCloud(), err
	}
	if p.meta != nil {
		return a.GetCloud(), errors.New("essa conta já tem uma nuvem do RadLog")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	d := a.cloudDriveWith(func(context.Context) (string, error) { return p.tok.Access, nil })
	folder := p.folder
	if folder == "" {
		if folder, err = d.CreateFolder(ctx, cloudFolderName); err != nil {
			return a.GetCloud(), err
		}
	}
	meta, keys, err := rs.NewMeta(code)
	if err != nil {
		return a.GetCloud(), err
	}
	b, _ := json.Marshal(meta)
	if _, err := d.Put(ctx, folder, "", rs.MetaFile, b); err != nil {
		return a.GetCloud(), err
	}
	a.cloudTurnOn(p, folder, code, meta.Salt, keys)
	return a.GetCloud(), nil
}

// CloudOpen abre uma nuvem que já existe com o código (outro aparelho).
func (a *App) CloudOpen(code string) (CloudInfo, error) {
	p, err := a.cloudPending()
	if err != nil {
		return a.GetCloud(), err
	}
	if p.meta == nil {
		return a.GetCloud(), errors.New("essa conta ainda não tem uma nuvem do RadLog")
	}
	keys, err := p.meta.Unlock(code)
	if err != nil {
		return a.GetCloud(), err
	}
	a.cloudTurnOn(p, p.folder, code, p.meta.Salt, keys)
	return a.GetCloud(), nil
}

func (a *App) cloudTurnOn(p *cloudPending, folder, code string, salt []byte, keys *rs.Keys) {
	code, _ = rs.ParseRecoveryCode(code)
	name := hostName()
	if utf8.RuneCountInString(name) > cloudNameMax {
		name = string([]rune(name)[:cloudNameMax])
	}
	c := &a.cloud
	c.mu.Lock()
	old := c.f
	c.f = cloudFile{On: true, DevID: randHex(8), Name: name, Folder: folder, Salt: salt, Email: p.tok.Email,
		Up: map[string]string{}, Seen: map[string]string{}, Devs: map[string]cloudDev{}, Rev: cloudRev}
	if old.On && old.Folder == folder && old.DevID != "" && bytes.Equal(old.Salt, salt) {
		// a mesma nuvem (a chave daqui ficou ilegível): continua o mesmo aparelho
		c.f.DevID, c.f.Name, c.f.NoAuto, c.f.LastUp, c.f.LastSync = old.DevID, old.Name, old.NoAuto, old.LastUp, old.LastSync
		if old.Up != nil {
			c.f.Up = old.Up
		}
		if old.Devs != nil {
			c.f.Devs = old.Devs
		}
	}
	c.gen++
	c.sec = cloudSecret{Code: code, Refresh: p.tok.Refresh}
	c.keys, c.access, c.expiry = keys, p.tok.Access, p.tok.Expiry
	c.pend, c.err, c.offline, c.skew = nil, "", false, 0
	a.cloudSaveLocked()
	a.cloudSaveSecretLocked()
	c.mu.Unlock()
	a.logf("nuvem ligada")
	a.cloudKick()
	a.emitCloud()
}

// CloudReset apaga a nuvem que está no Drive (o código foi perdido) para
// começar uma nova; os dados deste PC não mudam.
func (a *App) CloudReset() (CloudInfo, error) {
	p, err := a.cloudPending()
	if err != nil {
		return a.GetCloud(), err
	}
	if p.folder != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		d := a.cloudDriveWith(func(context.Context) (string, error) { return p.tok.Access, nil })
		if err := d.Delete(ctx, p.folder); err != nil {
			return a.GetCloud(), err
		}
		a.logf("nuvem: a nuvem antiga foi apagada do Drive para começar uma nova")
	}
	a.cloud.mu.Lock()
	p.folder, p.meta = "", nil
	a.cloud.mu.Unlock()
	return a.GetCloud(), nil
}

// ---------- código e ajustes ----------

// NewCloudCode sorteia um código de recuperação (com os traços).
func (a *App) NewCloudCode() (string, error) {
	c, err := rs.NewRecoveryCode()
	if err != nil {
		return "", err
	}
	return rs.FormatRecoveryCode(c), nil
}

// CheckCloudCode confere a forma de um código digitado ou colado e devolve
// ele com os traços.
func (a *App) CheckCloudCode(s string) (string, error) {
	c, err := rs.ParseRecoveryCode(s)
	if err != nil {
		return "", err
	}
	return rs.FormatRecoveryCode(c), nil
}

// CloudCode devolve o código guardado neste PC ("Ver código de recuperação").
func (a *App) CloudCode() (string, error) {
	a.cloud.mu.Lock()
	defer a.cloud.mu.Unlock()
	a.cloudLoadLocked()
	if !a.cloud.f.On || a.cloud.sec.Code == "" {
		return "", errors.New("a nuvem não está ligada neste PC")
	}
	return rs.FormatRecoveryCode(a.cloud.sec.Code), nil
}

// recoveryText é o .txt do "Salvar em arquivo": o código e o que ele é, sem
// e-mail nem nome do PC.
func recoveryText(code string) string {
	return strings.Join([]string{
		"RadLog: código de recuperação da nuvem",
		"",
		code,
		"",
		"Com este código você liga outro aparelho na nuvem do RadLog ou recupera",
		"a cópia da nuvem se perder os aparelhos. Guarde onde só você acessa.",
		"",
		"RadLog: cloud recovery code. With it you add another device to the",
		"RadLog cloud or recover the cloud copy if you lose your devices.",
		"Keep it where only you can reach it.",
		"",
	}, "\r\n")
}

// SaveCloudCode pergunta onde salvar e grava o código num .txt. Devolve o
// caminho (vazio se cancelou).
func (a *App) SaveCloudCode(code string) (string, error) {
	c, err := rs.ParseRecoveryCode(code)
	if err != nil {
		return "", err
	}
	if a.ctx == nil {
		return "", errors.New("sem janela")
	}
	dst, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{
		Title: "Salvar código de recuperação", DefaultFilename: "radlog-codigo-de-recuperacao.txt",
		Filters: []wruntime.FileFilter{{DisplayName: "Texto (*.txt)", Pattern: "*.txt"}},
	})
	if err != nil || dst == "" {
		return "", err
	}
	if !strings.HasSuffix(strings.ToLower(dst), ".txt") {
		dst += ".txt"
	}
	if err := writeFileAtomic(dst, []byte(recoveryText(rs.FormatRecoveryCode(c))), 0o600); err != nil {
		return "", err
	}
	return dst, nil
}

// CloudSyncNow sincroniza já.
func (a *App) CloudSyncNow() (CloudInfo, error) {
	a.cloud.mu.Lock()
	a.cloudLoadLocked()
	on := a.cloud.f.On
	a.cloud.err = ""
	a.cloud.mu.Unlock()
	if !on {
		return a.GetCloud(), errors.New("a nuvem não está ligada neste PC")
	}
	a.cloudKick()
	return a.GetCloud(), nil
}

// CloudPause para o envio em andamento (continua na próxima vez).
func (a *App) CloudPause() CloudInfo {
	a.cloud.mu.Lock()
	if a.cloud.run != nil {
		a.cloud.run()
	}
	a.cloud.mu.Unlock()
	return a.GetCloud()
}

// SetCloudAuto liga ou desliga a sincronização a cada 15 min.
func (a *App) SetCloudAuto(on bool) CloudInfo {
	a.cloud.mu.Lock()
	a.cloudLoadLocked()
	a.cloud.f.NoAuto = !on
	a.cloudSaveLocked()
	a.cloud.mu.Unlock()
	if on {
		a.cloudKick()
	}
	return a.GetCloud()
}

// SetCloudName muda o nome deste aparelho na nuvem.
func (a *App) SetCloudName(name string) (CloudInfo, error) {
	name = strings.Join(strings.Fields(name), " ")
	if name == "" {
		return a.GetCloud(), errors.New("digite um nome")
	}
	if utf8.RuneCountInString(name) > cloudNameMax {
		name = string([]rune(name)[:cloudNameMax])
	}
	a.cloud.mu.Lock()
	a.cloudLoadLocked()
	on := a.cloud.f.On
	if on {
		a.cloud.f.Name = name
		a.cloudSaveLocked()
	}
	a.cloud.mu.Unlock()
	if on {
		a.cloudKick()
	}
	return a.GetCloud(), nil
}

// CloudDisconnect desliga a nuvem neste PC: apaga o acesso e o código daqui.
// Os dados daqui e a cópia no Drive não mudam.
func (a *App) CloudDisconnect() CloudInfo {
	c := &a.cloud
	c.mu.Lock()
	if c.run != nil {
		c.run()
	}
	_ = os.Remove(a.cloudKeyPath())
	_ = os.Remove(a.cloudPath())
	c.f, c.sec, c.keys, c.access, c.pend, c.err, c.offline, c.skew = cloudFile{}, cloudSecret{}, nil, "", nil, "", false, 0
	c.gen++
	c.mu.Unlock()
	a.logf("nuvem desligada neste PC")
	a.emitCloud()
	return a.GetCloud()
}

// ---------- envio e recebimento ----------

func (a *App) cloudDriveWith(tok func(context.Context) (string, error)) *rs.Drive {
	d := cloudDriveFn(tok)
	d.OnDate = func(t time.Time) {
		a.cloud.mu.Lock()
		a.cloud.skew = int(time.Since(t).Seconds())
		a.cloud.mu.Unlock()
	}
	return d
}

// cloudToken devolve um acesso válido, pedindo outro ao Google se venceu.
func (a *App) cloudToken(ctx context.Context) (string, error) {
	c := &a.cloud
	c.mu.Lock()
	if c.access != "" && time.Now().Before(c.expiry) {
		defer c.mu.Unlock()
		return c.access, nil
	}
	refresh := c.sec.Refresh
	c.mu.Unlock()
	if refresh == "" {
		return "", rs.ErrRevoked
	}
	t, err := cloudOAuth().Refresh(ctx, refresh)
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	c.access, c.expiry = t.Access, t.Expiry
	if t.Refresh != refresh {
		c.sec.Refresh = t.Refresh
		a.cloudSaveSecretLocked()
	}
	c.mu.Unlock()
	return t.Access, nil
}

// cloudLoop sincroniza a cada 15 min (se ligado) e quando pedem.
func (a *App) cloudLoop() {
	a.cloud.mu.Lock()
	a.cloudLoadLocked()
	kick := a.cloud.kick
	a.cloud.mu.Unlock()
	t := time.NewTimer(time.Minute)
	for {
		byTimer := false
		select {
		case <-kick:
		case <-t.C:
			byTimer = true
		}
		t.Reset(cloudEvery)
		if a.closing.Load() {
			return
		}
		a.cloud.mu.Lock()
		auto := !a.cloud.f.NoAuto
		a.cloud.mu.Unlock()
		if byTimer && !auto {
			continue
		}
		a.cloudSync()
	}
}

// cloudSync roda uma sincronização (uma por vez) e guarda o resultado.
func (a *App) cloudSync() {
	c := &a.cloud
	c.mu.Lock()
	a.cloudLoadLocked()
	if !c.f.On || c.f.NeedLogin || c.run != nil || c.keys == nil {
		c.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.run = cancel
	gen := c.gen
	c.mu.Unlock()
	err := a.cloudSyncRun(ctx, gen)
	cancel()
	c.mu.Lock()
	defer a.emitCloud()
	defer c.mu.Unlock()
	c.run, c.sendN, c.sendI, c.sendNow = nil, 0, 0, ""
	if c.gen != gen || !c.f.On {
		return // desconectou ou ligou outra nuvem no meio
	}
	var nerr net.Error
	var herr *rs.HTTPError
	switch {
	case err == nil:
		c.offline, c.err = false, ""
	case errors.Is(err, context.Canceled):
	case errors.Is(err, rs.ErrRevoked):
		c.f.NeedLogin, c.access = true, ""
		a.cloudSaveLocked()
		a.logf("nuvem: o Google pediu para entrar de novo")
	case errors.As(err, &nerr), errors.As(err, &herr) && herr.Code >= 500:
		c.offline = true // sem internet ou o Google fora do ar: tenta na próxima
	default:
		c.offline, c.err = false, cloudErrText(err)
		a.logf("nuvem: %v", err)
	}
}

// cloudErrText é o erro em palavras para a tela (o Google responde em inglês).
func cloudErrText(err error) string {
	var herr *rs.HTTPError
	if errors.As(err, &herr) {
		msg := strings.ToLower(herr.Msg)
		switch {
		case strings.Contains(msg, "quota") || strings.Contains(msg, "storage"):
			return "o Google Drive está cheio: libere espaço na conta Google para a nuvem voltar a subir"
		case herr.Code == 401 || herr.Code == 403:
			return fmt.Sprintf("o Google recusou o acesso ao Drive (%d); se continuar, desconecte e entre de novo", herr.Code)
		case herr.Code == 429:
			return "o Google pediu para esperar um pouco; a nuvem tenta de novo sozinha"
		default:
			return fmt.Sprintf("o Google respondeu com erro %d; a nuvem tenta de novo sozinha", herr.Code)
		}
	}
	return err.Error()
}

type cloudPack struct {
	key    string // "c", "m|2026-11", "s|espectros/…"
	month  string // para a fila na tela
	digest string
	req    pullReq
}

var reMonth = regexp.MustCompile(`\d{4}-\d{2}`)

// cloudPacks monta a lista de pacotes deste PC e o resumo de cada um.
func (a *App) cloudPacks() ([]cloudPack, error) {
	days, err := a.dayDigests()
	if err != nil {
		return nil, err
	}
	rc := a.rcInventory()
	months := map[string]*cloudPack{}
	parts := map[string][]string{}
	month := func(m string) *cloudPack {
		p := months[m]
		if p == nil {
			p = &cloudPack{key: "m|" + m, month: m, req: pullReq{Rc: true, cloud: true, noSmall: true}}
			months[m] = p
		}
		return p
	}
	for d, h := range days {
		if len(d) < 7 {
			continue
		}
		p := month(d[:7])
		p.req.Days = append(p.req.Days, d)
		parts[p.key] = append(parts[p.key], "g"+d+h)
	}
	for k, h := range rc.Days {
		_, d, ok := strings.Cut(k, "|")
		if !ok || len(d) < 7 {
			continue
		}
		p := month(d[:7])
		p.req.RcDays = append(p.req.RcDays, k)
		parts[p.key] = append(parts[p.key], "r"+k+h)
	}
	secMonth := func(s string) string {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return ""
		}
		return time.Unix(n, 0).UTC().Format("2006-01")
	}
	for _, n := range a.alertFiles() {
		s, _, _ := strings.Cut(n, "-")
		if m := secMonth(strings.TrimSuffix(s, ".json")); m != "" {
			p := month(m)
			p.req.Alerts = append(p.req.Alerts, n)
			parts[p.key] = append(parts[p.key], "a"+n)
		}
	}
	for _, k := range a.attachInventory() {
		s, _, _ := strings.Cut(k, "/")
		if m := secMonth(s); m != "" {
			p := month(m)
			p.req.Anexos = append(p.req.Anexos, k)
			parts[p.key] = append(parts[p.key], "x"+k)
		}
	}
	// o comum: o resumo é o próprio pacote (pequeno e igual se nada mudou)
	common := cloudPack{key: "c", month: time.Now().UTC().Format("2006-01"), req: pullReq{Rc: true, cloud: true}}
	z, err := a.buildDelta(common.req)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(z)
	common.digest = hex.EncodeToString(sum[:16])
	out := []cloudPack{common}
	var keys []string
	for m := range months {
		keys = append(keys, m)
	}
	sort.Strings(keys)
	for _, m := range keys {
		p := months[m]
		ps := parts[p.key]
		sort.Strings(ps)
		h := sha256.Sum256([]byte("v2\n" + strings.Join(ps, "\n"))) // v2: início das medições dos anexos (N1)
		p.digest = hex.EncodeToString(h[:16])
		out = append(out, *p)
	}
	for _, f := range rc.Files {
		out = append(out, cloudPack{key: "s|" + f, month: reMonth.FindString(f), digest: "1", req: pullReq{Rc: true, cloud: true, noSmall: true, RcFiles: []string{f}}})
	}
	return out, nil
}

func cloudQueue(todo []cloudPack) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range todo {
		if p.month != "" && !seen[p.month] {
			seen[p.month] = true
			out = append(out, p.month)
		}
	}
	sort.Strings(out)
	return out
}

// tipos do que vai cifrado
const (
	cloudKindDev    = 'd'
	cloudKindCommon = 'c'
	cloudKindPack   = 'p'
)

// cloudHold trava e confere que a nuvem ainda é a da sincronização; false =
// destravada, parar (N8).
func (a *App) cloudHold(gen int) bool {
	a.cloud.mu.Lock()
	if a.cloud.gen != gen || !a.cloud.f.On {
		a.cloud.mu.Unlock()
		return false
	}
	return true
}

func (a *App) cloudSyncRun(ctx context.Context, gen int) error {
	c := &a.cloud
	if !a.cloudHold(gen) {
		return context.Canceled
	}
	folder, keys, dev, code := c.f.Folder, c.keys, c.f.DevID, c.sec.Code
	mine := map[string]bool{keys.FileName("d|" + dev): true, keys.FileName("c|" + dev): true}
	for k := range c.f.Up {
		mine[keys.FileName("p|"+dev+"|"+k)] = true
	}
	c.mu.Unlock()
	d := a.cloudDriveWith(a.cloudToken)
	files, err := d.List(ctx, folder)
	if err != nil {
		return err
	}
	byName := map[string]rs.File{}
	for _, f := range files {
		byName[f.Name] = f
	}
	// a nuvem ainda é a mesma (ninguém começou outra)
	mf, ok := byName[rs.MetaFile]
	if !ok {
		return errCloudGone
	}
	if !a.cloudHold(gen) {
		return context.Canceled
	}
	metaSeen := c.f.Seen[rs.MetaFile] == mf.MD5
	c.mu.Unlock()
	if !metaSeen {
		b, err := d.Get(ctx, mf.ID)
		if err != nil {
			return err
		}
		var m rs.Meta
		if json.Unmarshal(b, &m) != nil {
			return errCloudGone
		}
		if _, err := m.Unlock(code); err != nil {
			return errCloudGone
		}
		a.cloudSeen(gen, rs.MetaFile, mf.MD5)
	}

	// 1) baixa o que os outros aparelhos subiram de novo
	got, bad, err := a.cloudDownload(ctx, d, files, mine, gen)
	if err != nil {
		return err
	}

	// 2) sobe o que mudou aqui
	packs, err := a.cloudPacks()
	if err != nil {
		return err
	}
	if !a.cloudHold(gen) {
		return context.Canceled
	}
	var todo []cloudPack
	for _, p := range packs {
		if c.f.Up[p.key] != p.digest {
			todo = append(todo, p)
		}
	}
	c.f.Queue = cloudQueue(todo)
	c.sendN = len(todo)
	c.mu.Unlock()
	sent := 0
	for i, p := range todo {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !a.cloudHold(gen) {
			return context.Canceled
		}
		c.sendI, c.sendNow = i+1, p.month
		c.mu.Unlock()
		a.emitCloud()
		z, err := a.buildDelta(p.req)
		if err != nil {
			return err
		}
		kind, id := byte(cloudKindPack), "p|"+dev+"|"+p.key
		if p.key == "c" {
			kind, id = cloudKindCommon, "c|"+dev
		}
		f, err := a.cloudPut(ctx, d, folder, keys, byName, keys.FileName(id), kind, z)
		if err != nil {
			return err
		}
		byName[f.Name] = f
		sent++
		if !a.cloudHold(gen) {
			return context.Canceled
		}
		c.f.Up[p.key] = p.digest
		c.f.LastUp = time.Now().UnixMilli()
		c.f.Queue = cloudQueue(todo[i+1:])
		a.cloudSaveLocked()
		c.mu.Unlock()
	}
	// o arquivo deste aparelho: nome e último envio
	if !a.cloudHold(gen) {
		return context.Canceled
	}
	me := cloudDev{Name: c.f.Name, LastUp: c.f.LastUp}
	devDigest := me.Name + "|" + strconv.FormatInt(me.LastUp, 10)
	devUp := c.f.Up["d"] == devDigest
	c.mu.Unlock()
	if !devUp {
		b, _ := json.Marshal(me)
		f, err := a.cloudPut(ctx, d, folder, keys, byName, keys.FileName("d|"+dev), cloudKindDev, b)
		if err != nil {
			return err
		}
		byName[f.Name] = f
		if !a.cloudHold(gen) {
			return context.Canceled
		}
		c.f.Up["d"] = devDigest
		c.mu.Unlock()
	}
	var used int64
	for _, f := range byName {
		used += f.Size
	}
	if !a.cloudHold(gen) {
		return context.Canceled
	}
	c.f.LastSync, c.f.Used, c.f.Queue = time.Now().UnixMilli(), used, nil
	a.cloudSaveLocked()
	c.mu.Unlock()
	if sent > 0 || got > 0 {
		a.logf("nuvem: subiu %d pacote(s), juntou %d", sent, got)
	}
	if bad > 0 {
		// o resto sincronizou; os que não entraram são tentados de novo (N2)
		return fmt.Errorf("%d pacote(s) de outro aparelho não entraram neste PC; o resto sincronizou e eles são tentados de novo", bad)
	}
	return nil
}

func (a *App) cloudSeen(gen int, name, md5 string) {
	if !a.cloudHold(gen) {
		return
	}
	if a.cloud.f.Seen == nil {
		a.cloud.f.Seen = map[string]string{}
	}
	a.cloud.f.Seen[name] = md5
	a.cloudSaveLocked()
	a.cloud.mu.Unlock()
}

// cloudPut cifra e sobe (troca o conteúdo se o arquivo já existe).
func (a *App) cloudPut(ctx context.Context, d *rs.Drive, folder string, keys *rs.Keys, byName map[string]rs.File, name string, kind byte, data []byte) (rs.File, error) {
	sealed, err := keys.Seal(name, append([]byte{kind}, data...))
	if err != nil {
		return rs.File{}, err
	}
	id := byName[name].ID
	f, err := d.Put(ctx, folder, id, name, sealed)
	if errors.Is(err, rs.ErrNotFound) && id != "" {
		f, err = d.Put(ctx, folder, "", name, sealed)
	}
	if err == nil && f.Name == "" {
		f.Name = name
	}
	return f, err
}

// cloudDownload baixa e junta os arquivos novos dos outros aparelhos: o
// comum primeiro (as medições, para os anexos acharem a delas), depois os
// meses e espectros. Um pacote que não junta fica de fora desta vez e não
// para o resto (N2). Devolve quantos pacotes juntou e quantos não entraram.
func (a *App) cloudDownload(ctx context.Context, d *rs.Drive, files []rs.File, mine map[string]bool, gen int) (got, bad int, err error) {
	c := &a.cloud
	if !a.cloudHold(gen) {
		return 0, 0, context.Canceled
	}
	keys := c.keys
	seen := map[string]string{}
	for k, v := range c.f.Seen {
		seen[k] = v
	}
	c.mu.Unlock()
	dir := filepath.Join(a.dataDir, "tmp")
	_ = os.MkdirAll(dir, 0o755)
	type later struct{ name, md5, path string }
	var packs []later
	defer func() {
		for _, p := range packs {
			_ = os.Remove(p.path)
		}
	}()
	for _, f := range files {
		if f.Name == rs.MetaFile || mine[f.Name] || (f.MD5 != "" && seen[f.Name] == f.MD5) {
			continue
		}
		if err := ctx.Err(); err != nil {
			return got, bad, err
		}
		b, err := d.Get(ctx, f.ID)
		if errors.Is(err, rs.ErrNotFound) {
			continue
		}
		if err != nil {
			return got, bad, err
		}
		plain, err := keys.Open(f.Name, b)
		if err != nil || len(plain) == 0 {
			a.cloudSeen(gen, f.Name, f.MD5) // não é da nuvem (ou estragou): não tenta de novo
			continue
		}
		switch plain[0] {
		case cloudKindDev:
			var dv cloudDev
			if json.Unmarshal(plain[1:], &dv) == nil && a.cloudHold(gen) {
				if c.f.Devs == nil {
					c.f.Devs = map[string]cloudDev{}
				}
				c.f.Devs[f.Name] = dv
				c.mu.Unlock()
			}
			a.cloudSeen(gen, f.Name, f.MD5)
		case cloudKindCommon:
			if _, err := a.mergeDelta(plain[1:], "nuvem"); err != nil {
				a.logf("nuvem: um pacote de outro aparelho não entrou (%v)", err)
				bad++
				continue
			}
			got++
			a.cloudSeen(gen, f.Name, f.MD5)
		case cloudKindPack:
			tf, err := os.CreateTemp(dir, "nuvem-*.zip")
			if err != nil {
				return got, bad, err
			}
			_, werr := tf.Write(plain[1:])
			tf.Close()
			packs = append(packs, later{f.Name, f.MD5, tf.Name()})
			if werr != nil {
				return got, bad, werr
			}
		default:
			a.cloudSeen(gen, f.Name, f.MD5)
		}
	}
	for _, p := range packs {
		if err := ctx.Err(); err != nil {
			return got, bad, err
		}
		if _, err := a.mergeZip(p.path, false); err != nil {
			a.logf("nuvem: um pacote de outro aparelho não entrou (%v)", err)
			bad++
			continue
		}
		got++
		a.cloudSeen(gen, p.name, p.md5)
	}
	return got, bad, nil
}
