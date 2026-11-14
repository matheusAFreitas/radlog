package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
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
// pessoa, cifrados com a chave do código de recuperação (internal/sync), no
// formato 2 (docs/nuvem-formato.md; envio e recebimento em cloudsync.go): um
// arquivo do aparelho, um "comum" com os pequenos, um por dia de minutos, um
// por mês de espectros e um por anexo. Os pacotes são os mesmos da
// sincronização pela rede (buildDelta) e quem baixa junta com a mesma mescla
// do "Importar backup".
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
	ID     string `json:"id"`
	Name   string `json:"name"`
	LastUp int64  `json:"lastUp"` // ms; 0 = ainda não subiu nada
	Me     bool   `json:"me"`
}

// CloudInfo é o estado do cartão "Nuvem (Google Drive)".
type CloudInfo struct {
	// off, wait, ok, send, offline, login, clock, err, dup, old
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
	// removido por outro aparelho (ms); recomeço em andamento e quando
	// terminou (ms; a pasta antiga sai da lixeira 30 dias depois)
	Removed   int64 `json:"removed,omitempty"`
	Restart   bool  `json:"restart,omitempty"`
	Restarted int64 `json:"restarted,omitempty"`
}

// CloudLoginDone é o fim do login: next diz qual diálogo abre.
type CloudLoginDone struct {
	Info   CloudInfo `json:"info"`
	Next   string    `json:"next"`   // "new", "existing", "old" (formato antigo) ou "" (só entrou de novo)
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
	Devs      map[string]cloudDev `json:"devs,omitempty"` // ID de outro aparelho → nome e último envio
	Rev       int                 `json:"rev,omitempty"`  // cloudRev desta cópia do app
	// o arquivo deste aparelho como foi escrito por último daqui (N6) e
	// quantas vezes foi escrito
	DevMD5 string `json:"devMd5,omitempty"`
	Seq    int64  `json:"seq,omitempty"`
	// outro PC escreveu no arquivo deste aparelho (pasta copiada): pausada até
	// a pessoa escolher (CloudDupAnswer)
	Dup   bool   `json:"dup,omitempty"`
	DupOf string `json:"dupOf,omitempty"`
	// a nuvem é do formato antigo (2.79.x): pausada até começar uma nova
	Old bool `json:"old,omitempty"`
	// versão da meta conferida por último (rs.MetaVersion)
	MetaV int `json:"metaV,omitempty"`
	// a pasta já tem a marca do app (N17)
	Tagged bool `json:"tagged,omitempty"`
	// outro aparelho tirou este da nuvem (ms): pausada até entrar de novo
	Removed int64 `json:"removed,omitempty"`
	// nuvem recomeçada (CloudRestart, ms) e ainda subindo tudo de novo
	Restart   int64 `json:"restart,omitempty"`
	Restarted int64 `json:"restarted,omitempty"`
}

// cloudRev sobe quando a mescla passa a aproveitar algo que antes ficava de
// fora: a nuvem baixa e junta de novo o que já tinha visto.
//
//	1: 2.79.1, anexos das medições (N1)
const cloudRev = 1

// cloudSecret é o nuvem-chave.bin. Writer muda a cada instalação: ligar a
// nuvem de novo, chave ilegível (pasta copiada para outro PC) ou outro nome
// de PC (Host).
type cloudSecret struct {
	Code    string `json:"code"`
	Refresh string `json:"refresh"`
	Writer  string `json:"writer,omitempty"`
	Host    string `json:"host,omitempty"`
}

// cloudDevFile é o nuvem-aparelho.json: o ID deste aparelho na nuvem, que
// fica mesmo depois de desconectar (o mesmo PC volta como o mesmo aparelho,
// N5).
type cloudDevFile struct {
	ID string `json:"id"`
}

type cloudPending struct {
	tok    rs.Token
	folder string
	meta   *rs.Meta
	// trash é a nuvem antiga que vai para a lixeira só depois de a nova
	// estar criada (auditoria 2.79.0, N11)
	trash string
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
func (a *App) cloudDevPath() string { return filepath.Join(a.dataDir, "nuvem-aparelho.json") }

// cloudDevID devolve o ID guardado deste aparelho ou sorteia (e guarda) um.
func (a *App) cloudDevID(fresh bool) string {
	var f cloudDevFile
	if !fresh {
		a.readStateJSON(a.cloudDevPath(), "aparelho da nuvem", &f)
	}
	if len(f.ID) < 8 || fresh {
		f.ID = randHex(8)
		if a.dataDir != "" {
			b, _ := json.Marshal(f)
			if err := writeFileAtomic(a.cloudDevPath(), b, 0o600); err != nil {
				a.logf("nuvem: não consegui gravar o aparelho (%v)", err)
			}
		}
	}
	return f.ID
}

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
			if h := hostName(); c.sec.Writer == "" || c.sec.Host != h {
				c.sec.Writer, c.sec.Host = randHex(8), h // outro PC com a mesma chave
				a.cloudSaveSecretLocked()
			}
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
	case f.Old:
		info.State = "old"
	case f.Removed != 0:
		info.State = "removed"
		info.Removed = f.Removed
	case f.Dup:
		info.State = "dup"
		info.DupOf = f.DupOf
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
	info.Restart, info.Restarted = f.Restart != 0, f.Restarted
	info.Email = f.Email
	if f.On {
		info.Devices = append(info.Devices, CloudDevice{ID: f.DevID, Name: f.Name, LastUp: f.LastUp, Me: true})
		var others []CloudDevice
		for id, d := range f.Devs {
			if id != f.DevID {
				others = append(others, CloudDevice{ID: id, Name: d.Name, LastUp: d.LastUp})
			}
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
		// N12: outra conta Google não enxerga a pasta desta nuvem
		if c.f.Email != "" && tok.Email != "" && !strings.EqualFold(c.f.Email, tok.Email) {
			c.mu.Unlock()
			return nil, errCloudOtherAccount
		}
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
	folders, err := d.FindFolders(ctx, cloudFolderName)
	if err != nil {
		return nil, err
	}
	// mais de uma pasta (N17): vale a mais antiga que tem nuvem
	for _, fd := range folders {
		files, err := d.List(ctx, fd.ID)
		if err != nil {
			return nil, err
		}
		if p.folder == "" {
			p.folder = fd.ID
		}
		has := false
		for _, f := range files {
			if f.Name == rs.MetaFile {
				has = true
			}
		}
		if !has {
			continue
		}
		p.folder = fd.ID
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
					if m.V < rs.MetaVersion {
						done.Next = "old" // só dá para começar uma nova (CloudReset)
					}
				}
			}
			if ms := f.Modified.UnixMilli(); ms > done.LastUp {
				done.LastUp = ms
			}
		}
		break
	}
	c.mu.Lock()
	c.pend = p
	c.mu.Unlock()
	done.Info = a.GetCloud()
	return done, nil
}

// errCloudOtherAccount: no "entrar de novo", a conta Google é outra (N12).
// O e-mail não entra na mensagem (ela pode ir para o registro).
var errCloudOtherAccount = errors.New("essa é outra conta Google. Entre com a mesma conta que aparece no cartão da nuvem, ou desconecte para usar outra")

// errCloudRace: dois aparelhos criaram a nuvem ao mesmo tempo (N17).
var errCloudRace = errors.New("outro aparelho acabou de criar a nuvem nesta conta. Entre de novo e use o código de recuperação dele")

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
	if err == nil {
		b, _ := json.Marshal(meta)
		_, err = d.Put(ctx, folder, "", rs.MetaFile, b)
	}
	if err == nil && folder != p.folder {
		// dois aparelhos criando ao mesmo tempo (N17): vale a mais antiga
		err = a.cloudRaceCheck(ctx, d, folder, p.trash)
	}
	if err == nil && p.trash != "" {
		if err = d.Trash(ctx, p.trash); err == nil {
			a.logf("nuvem: a nuvem antiga foi para a lixeira do Drive")
		}
	}
	if err != nil {
		if folder != p.folder {
			_ = d.Delete(ctx, folder) // desfaz: a pasta nova está vazia
		}
		return a.GetCloud(), err
	}
	a.cloudTurnOn(p, folder, code, meta.Salt, keys)
	return a.GetCloud(), nil
}

// cloudRaceCheck: depois de criar a pasta, confere se outra pasta com nuvem
// nasceu antes dela (fora skip, a antiga que vai para a lixeira).
func (a *App) cloudRaceCheck(ctx context.Context, d *rs.Drive, mine, skip string) error {
	folders, err := d.FindFolders(ctx, cloudFolderName)
	if err != nil {
		return err
	}
	for _, fd := range folders {
		if fd.ID == mine {
			return nil
		}
		if fd.ID == skip {
			continue
		}
		files, err := d.List(ctx, fd.ID)
		if err != nil {
			return err
		}
		for _, f := range files {
			if f.Name == rs.MetaFile {
				return errCloudRace
			}
		}
	}
	return nil
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
	// o mesmo PC volta como o mesmo aparelho (N5)
	c.f = cloudFile{On: true, DevID: a.cloudDevID(false), Name: name, Folder: folder, Salt: salt, Email: p.tok.Email,
		Up: map[string]string{}, Seen: map[string]string{}, Devs: map[string]cloudDev{}, Rev: cloudRev}
	if old.On && old.Folder == folder && bytes.Equal(old.Salt, salt) {
		// a mesma nuvem (a chave daqui ficou ilegível): continua de onde estava
		c.f.Name, c.f.NoAuto, c.f.LastUp, c.f.LastSync, c.f.DevMD5, c.f.Seq = old.Name, old.NoAuto, old.LastUp, old.LastSync, old.DevMD5, old.Seq
		if old.DevID != "" {
			c.f.DevID = old.DevID
		}
		if old.Up != nil {
			c.f.Up = old.Up
		}
		if old.Devs != nil {
			c.f.Devs = old.Devs
		}
	}
	c.gen++
	c.sec = cloudSecret{Code: code, Refresh: p.tok.Refresh, Writer: randHex(8), Host: hostName()}
	c.keys, c.access, c.expiry = keys, p.tok.Access, p.tok.Expiry
	c.pend, c.err, c.offline, c.skew = nil, "", false, 0
	a.cloudSaveLocked()
	a.cloudSaveSecretLocked()
	c.mu.Unlock()
	a.logf("nuvem ligada")
	a.cloudKick()
	a.emitCloud()
}

// CloudReset prepara uma nuvem nova (o código foi perdido ou a do Drive é do
// formato antigo). A antiga só vai para a lixeira do Drive quando a nova
// estiver criada (CloudFinishSetup): quem desiste no meio continua com ela
// (auditoria 2.79.0, N11). Os dados deste PC não mudam.
func (a *App) CloudReset() (CloudInfo, error) {
	p, err := a.cloudPending()
	if err != nil {
		return a.GetCloud(), err
	}
	a.cloud.mu.Lock()
	if p.folder != "" {
		p.trash = p.folder
	}
	p.folder, p.meta = "", nil
	a.cloud.mu.Unlock()
	return a.GetCloud(), nil
}

// ---------- código e ajustes ----------

// área de transferência (trocadas nos testes)
var (
	cloudClipSet = func(a *App, s string) error {
		if a.ctx == nil {
			return errors.New("área de transferência indisponível")
		}
		// no Windows, fora do histórico (Win+V) e da nuvem da Microsoft
		if s != "" && clipPrivateSet(s) == nil {
			return nil
		}
		return wruntime.ClipboardSetText(a.ctx, s)
	}
	cloudClipGet = func(a *App) (string, error) {
		if a.ctx == nil {
			return "", errors.New("área de transferência indisponível")
		}
		return wruntime.ClipboardGetText(a.ctx)
	}
	cloudClipTTL = time.Minute
)

// CopyCloudCode põe o código na área de transferência e limpa depois de
// 1 min, se ele ainda estiver lá (auditoria 2.79.0, U2).
func (a *App) CopyCloudCode(code string) error {
	if err := cloudClipSet(a, code); err != nil {
		return err
	}
	time.AfterFunc(cloudClipTTL, func() {
		if t, err := cloudClipGet(a); err == nil && strings.TrimSpace(t) == code {
			_ = cloudClipSet(a, "")
		}
	})
	return nil
}

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
	a.cloudLoadLocked()
	// N13: tira o RadLog da conta Google, mas só se não há outro aparelho na
	// nuvem (revogar vale para todos os PCs na mesma conta)
	revoke := ""
	if len(c.f.Devs) == 0 {
		revoke = c.sec.Refresh
	}
	_ = os.Remove(a.cloudKeyPath())
	_ = os.Remove(a.cloudPath())
	c.f, c.sec, c.keys, c.access, c.pend, c.err, c.offline, c.skew = cloudFile{}, cloudSecret{}, nil, "", nil, "", false, 0
	c.gen++
	c.mu.Unlock()
	if revoke != "" {
		// em segundo plano: sem internet, o acesso só deixa de ser usado
		o := cloudOAuth()
		a.goSafe("nuvem: revogar", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_ = o.Revoke(ctx, revoke)
		})
	}
	a.logf("nuvem desligada neste PC")
	a.emitCloud()
	return a.GetCloud()
}

// CloudDupAnswer responde a pergunta do PC copiado (N6): newDevice = este é
// outro aparelho (ganha ID novo e sobe tudo como aparelho novo); false = este
// é o mesmo aparelho e o outro foi desligado (assume o arquivo dele).
func (a *App) CloudDupAnswer(newDevice bool) (CloudInfo, error) {
	c := &a.cloud
	c.mu.Lock()
	a.cloudLoadLocked()
	if !c.f.On || !c.f.Dup {
		c.mu.Unlock()
		return a.GetCloud(), nil
	}
	if newDevice {
		c.f.DevID = a.cloudDevID(true)
		c.f.Up = map[string]string{}
		c.f.LastUp, c.f.Seq = 0, 0
		a.logf("nuvem: este PC passou a ser um aparelho novo na nuvem")
	} else {
		a.logf("nuvem: este PC assumiu o aparelho que estava copiado")
	}
	c.f.Dup, c.f.DupOf, c.f.DevMD5 = false, "", ""
	c.gen++
	a.cloudSaveLocked()
	c.mu.Unlock()
	a.cloudKick()
	a.emitCloud()
	return a.GetCloud(), nil
}

// CloudRemoveDevice tira um aparelho da nuvem: os arquivos dele vão para a
// lixeira do Drive. Os dados que já chegaram aqui ficam, e este PC sobe de
// novo o que tinha recebido dele, para a cópia da nuvem continuar completa.
func (a *App) CloudRemoveDevice(id string) (CloudInfo, error) {
	c := &a.cloud
	c.mu.Lock()
	a.cloudLoadLocked()
	on, keys, folder, me := c.f.On && c.keys != nil, c.keys, c.f.Folder, c.f.DevID
	c.mu.Unlock()
	if !on {
		return a.GetCloud(), errors.New("a nuvem não está ligada neste PC")
	}
	if id == "" || id == me {
		return a.GetCloud(), errors.New("este aparelho não pode se remover; use Desconectar")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	d := a.cloudDriveWith(a.cloudToken)
	files, err := d.List(ctx, folder)
	if err != nil {
		return a.GetCloud(), err
	}
	prefix, n := keys.Prefix(id), 0
	for _, f := range files {
		if strings.HasPrefix(f.Name, prefix) {
			if err := d.Trash(ctx, f.ID); err != nil {
				return a.GetCloud(), err
			}
			n++
		}
	}
	c.mu.Lock()
	delete(c.f.Devs, id)
	for k, v := range c.f.Up {
		if strings.HasPrefix(v, cloudEcho) {
			delete(c.f.Up, k)
		}
	}
	a.cloudSaveLocked()
	c.mu.Unlock()
	a.logf("nuvem: um aparelho foi removido (%d arquivo(s) na lixeira do Drive)", n)
	a.cloudKick()
	a.emitCloud()
	return a.GetCloud(), nil
}

// CloudRejoin volta este PC para a nuvem depois de outro aparelho tirar ele
// (pede o código de novo, como a tela promete): continua o mesmo aparelho e
// sobe tudo outra vez.
func (a *App) CloudRejoin(code string) (CloudInfo, error) {
	c := &a.cloud
	c.mu.Lock()
	a.cloudLoadLocked()
	if !c.f.On || c.f.Removed == 0 {
		c.mu.Unlock()
		return a.GetCloud(), nil
	}
	got, err := rs.ParseRecoveryCode(code)
	if err != nil || got != c.sec.Code {
		c.mu.Unlock()
		return a.GetCloud(), errors.New("esse código não é o desta nuvem")
	}
	c.f.Removed, c.f.DevMD5, c.f.Seq = 0, "", 0
	c.f.Up = map[string]string{}
	c.gen++
	a.cloudSaveLocked()
	c.mu.Unlock()
	a.logf("nuvem: este PC entrou de novo na nuvem")
	a.cloudKick()
	a.emitCloud()
	return a.GetCloud(), nil
}

// CloudRestart começa uma nuvem nova no formato atual com o mesmo código
// (a do Drive é do formato antigo): cria a pasta nova e só depois põe a
// antiga na lixeira (N11). Os dados deste PC não mudam e sobem de novo.
func (a *App) CloudRestart() (CloudInfo, error) {
	c := &a.cloud
	c.mu.Lock()
	a.cloudLoadLocked()
	on, code, oldFolder := c.f.On && c.keys != nil, c.sec.Code, c.f.Folder
	c.mu.Unlock()
	if !on {
		return a.GetCloud(), errors.New("a nuvem não está ligada neste PC")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	d := a.cloudDriveWith(a.cloudToken)
	folder, err := d.CreateFolder(ctx, cloudFolderName)
	if err != nil {
		return a.GetCloud(), err
	}
	meta, keys, err := rs.NewMeta(code)
	if err == nil {
		b, _ := json.Marshal(meta)
		_, err = d.Put(ctx, folder, "", rs.MetaFile, b)
	}
	if err == nil && oldFolder != "" {
		err = d.Trash(ctx, oldFolder)
	}
	if err != nil {
		_ = d.Delete(ctx, folder) // desfaz: a pasta nova está vazia
		return a.GetCloud(), err
	}
	c.mu.Lock()
	c.f.Folder, c.f.Salt, c.f.Old, c.f.MetaV = folder, meta.Salt, false, rs.MetaVersion
	c.f.Up, c.f.Seen, c.f.Devs = map[string]string{}, map[string]string{}, map[string]cloudDev{}
	c.f.DevMD5, c.f.Seq, c.f.Queue, c.f.Used = "", 0, nil, 0
	c.f.Restart, c.f.Restarted = time.Now().UnixMilli(), 0
	c.keys, c.err = keys, ""
	c.gen++
	a.cloudSaveLocked()
	c.mu.Unlock()
	a.logf("nuvem: nuvem nova começada; a antiga foi para a lixeira do Drive")
	a.cloudKick()
	a.emitCloud()
	return a.GetCloud(), nil
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
	if !c.f.On || c.f.NeedLogin || c.f.Old || c.f.Dup || c.f.Removed != 0 || c.run != nil || c.keys == nil {
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
		if c.f.Restart != 0 {
			c.f.Restart, c.f.Restarted = 0, time.Now().UnixMilli()
			a.cloudSaveLocked()
		}
	case errors.Is(err, context.Canceled):
	case errors.Is(err, errCloudDup), errors.Is(err, errCloudRemoved):
		c.offline, c.err = false, ""
	case errors.Is(err, errCloudOld):
		c.f.Old, c.offline, c.err = true, false, ""
		a.cloudSaveLocked()
		a.logf("nuvem: a nuvem no Drive é do formato antigo e precisa ser começada de novo")
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
