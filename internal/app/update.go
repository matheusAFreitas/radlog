package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"radlog/internal/updsig"
)

// ---------- atualização pelo GitHub (com clique) ----------
//
// O app olha a última release pública do repositório ao abrir e uma vez por
// dia. Se houver versão nova, a tela avisa; ao clicar em Atualizar, ele baixa
// o .exe (ou o AppImage) para a mesma pasta, confere o SHA-256 com o
// SHA256SUMS.txt da release e abre a versão nova, que assume no lugar desta
// (a mesma troca de quando você abre um .exe novo à mão, handoff.go).
// Se houver alerta ativo, medição de objeto ou histórico baixando, ele espera
// tudo voltar ao normal antes de trocar.

const (
	updateRepo     = "matheusAFreitas/radlog"
	updateSums     = "SHA256SUMS.txt"
	updateSig      = updateSums + ".sig" // assinatura Ed25519 (updsig, 2.16)
	updateEvery    = 24 * time.Hour
	updateFirstIn  = 30 * time.Second
	updateWaitTick = 5 * time.Second
)

// endereço da API (os testes trocam)
var updateAPI = "https://api.github.com/repos/" + updateRepo + "/releases/latest"

// UpdateInfo é o que a tela mostra sobre atualização.
type UpdateInfo struct {
	Current   string  `json:"current"`
	Latest    string  `json:"latest"`    // "" = ainda não olhou
	Available bool    `json:"available"` // há versão mais nova
	Skipped   bool    `json:"skipped"`   // você pediu para pular esta
	Notes     string  `json:"notes"`     // notas da versão (do CHANGELOG)
	URL       string  `json:"url"`       // página da release
	CanAuto   bool    `json:"canAuto"`   // dá para baixar e trocar sozinho
	Manual    string  `json:"manual"`    // por que não dá (baixe pela página)
	CheckedAt int64   `json:"checkedAt"`
	State     string  `json:"state"` // "", procurando, baixando, esperando, abrindo, erro
	Progress  float64 `json:"progress"`
	Wait      string  `json:"wait"` // o que está segurando a troca
	Error     string  `json:"error"`

	asset, sums, sig ghAsset
}

type ghAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

type ghRelease struct {
	Tag        string    `json:"tag_name"`
	Body       string    `json:"body"`
	HTMLURL    string    `json:"html_url"`
	Draft      bool      `json:"draft"`
	Prerelease bool      `json:"prerelease"`
	Assets     []ghAsset `json:"assets"`
}

type updater struct {
	mu   sync.Mutex
	info UpdateInfo
	busy bool
}

// parseVersion lê "v1.17.1" ou "1.17.1"; ok=false se não for X.Y.Z.
func parseVersion(s string) (v [3]int, ok bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

func newerVersion(latest, current string) bool {
	l, ok1 := parseVersion(latest)
	c, ok2 := parseVersion(current)
	if !ok1 || !ok2 {
		return false
	}
	for i := 0; i < 3; i++ {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

// pickAsset escolhe o arquivo desta plataforma na release.
func pickAsset(assets []ghAsset) (ghAsset, string) {
	switch runtime.GOOS {
	case "windows":
		for _, a := range assets {
			if strings.HasSuffix(strings.ToLower(a.Name), ".exe") {
				return a, ""
			}
		}
		return ghAsset{}, "a release não tem o .exe"
	case "linux":
		if os.Getenv("APPIMAGE") == "" {
			return ghAsset{}, "instalado pelo .tar.gz: baixe a versão nova pela página e rode o install.sh"
		}
		for _, a := range assets {
			if strings.HasSuffix(a.Name, "-x86_64.AppImage") {
				return a, ""
			}
		}
		return ghAsset{}, "a release não tem o AppImage"
	}
	return ghAsset{}, "sem atualização automática neste sistema"
}

// assetIsVersion diz se name é o arquivo de release da versão ver
// (RadLog-vX.Y.Z.exe ou RadLog-vX.Y.Z-x86_64.AppImage).
func assetIsVersion(name, ver string) bool {
	m := releaseName.FindStringSubmatch(name)
	if m == nil || m[3] != "" {
		return false
	}
	a, ok1 := parseVersion(m[1])
	b, ok2 := parseVersion(ver)
	return ok1 && ok2 && a == b
}

// releaseNotes tira o rodapé de downloads das notas.
func releaseNotes(body string) string {
	if i := strings.Index(body, "\n---"); i >= 0 {
		body = body[:i]
	}
	return strings.TrimSpace(strings.ReplaceAll(body, "\r\n", "\n"))
}

var updateClient = &http.Client{Timeout: 20 * time.Second}

func (a *App) updateInfo() UpdateInfo {
	a.upd.mu.Lock()
	defer a.upd.mu.Unlock()
	i := a.upd.info
	i.Current = AppVersion
	a.mu.Lock()
	i.Skipped = i.Available && a.settings.SkipVersion != "" && a.settings.SkipVersion == i.Latest
	a.mu.Unlock()
	return i
}

func (a *App) setUpdate(f func(*UpdateInfo)) {
	a.upd.mu.Lock()
	f(&a.upd.info)
	a.upd.mu.Unlock()
	a.emit("update", a.updateInfo())
}

// GetUpdateInfo devolve o último estado (sem ir à internet).
func (a *App) GetUpdateInfo() UpdateInfo { return a.updateInfo() }

// CheckUpdate olha a última release no GitHub.
func (a *App) CheckUpdate() (UpdateInfo, error) {
	a.upd.mu.Lock()
	if a.upd.busy {
		a.upd.mu.Unlock()
		return a.updateInfo(), nil
	}
	a.upd.mu.Unlock()
	a.setUpdate(func(i *UpdateInfo) { i.State, i.Error = "procurando", "" })
	rel, err := fetchRelease()
	if err != nil {
		a.logf("atualização: não consegui olhar o GitHub: %v", err)
		a.setUpdate(func(i *UpdateInfo) { i.State, i.Error = "", "não consegui olhar o GitHub: "+err.Error() })
		return a.updateInfo(), err
	}
	asset, manual := pickAsset(rel.Assets)
	var sums, sig ghAsset
	for _, x := range rel.Assets {
		switch x.Name {
		case updateSums:
			sums = x
		case updateSig:
			sig = x
		}
	}
	if manual == "" && sums.URL == "" {
		manual = "a release não tem o " + updateSums + " para conferir o arquivo"
	}
	if manual == "" && sig.URL == "" {
		manual = "a release não tem a assinatura (" + updateSig + "); baixe pela página e confira"
	}
	latest := strings.TrimPrefix(rel.Tag, "v")
	// A assinatura cobre o SHA256SUMS.txt (nome e soma de cada arquivo), mas
	// não a etiqueta da release. Sem esta conferência, quem conseguisse
	// publicar uma release poderia pôr um .exe antigo, com a soma e a
	// assinatura antigas, sob uma etiqueta nova, e o app instalaria a versão
	// velha. O nome do arquivo, que está assinado, tem de trazer a versão
	// anunciada.
	if manual == "" && !assetIsVersion(asset.Name, latest) {
		manual = fmt.Sprintf("o arquivo da release (%s) não é o da versão %s; baixe pela página e confira", asset.Name, latest)
	}
	avail := newerVersion(latest, AppVersion)
	a.setUpdate(func(i *UpdateInfo) {
		i.Latest, i.Available, i.Notes, i.URL = latest, avail, releaseNotes(rel.Body), rel.HTMLURL
		i.CanAuto, i.Manual, i.asset, i.sums, i.sig = manual == "", manual, asset, sums, sig
		i.CheckedAt, i.State, i.Error = time.Now().UnixMilli(), "", ""
	})
	if avail {
		a.logf("atualização: versão %s disponível (esta é %s)", latest, AppVersion)
	}
	return a.updateInfo(), nil
}

func fetchRelease() (ghRelease, error) {
	req, _ := http.NewRequest("GET", updateAPI, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "RadLog-updater/"+AppVersion)
	resp, err := updateClient.Do(req)
	if err != nil {
		return ghRelease{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return ghRelease{}, errors.New("nenhuma release publicada")
	}
	if resp.StatusCode != http.StatusOK {
		return ghRelease{}, fmt.Errorf("GitHub respondeu %s", resp.Status)
	}
	var rel ghRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&rel); err != nil {
		return ghRelease{}, err
	}
	if rel.Draft || rel.Prerelease {
		return ghRelease{}, errors.New("a última release ainda não é definitiva")
	}
	return rel, nil
}

// SkipUpdate marca a versão para não avisar de novo ("" volta a avisar).
func (a *App) SkipUpdate(version string) {
	a.mu.Lock()
	a.settings.SkipVersion = version
	a.saveSettingsLocked()
	a.mu.Unlock()
	a.emit("update", a.updateInfo())
}

// updateLoop procura versão nova logo depois de abrir e uma vez por dia.
func (a *App) updateLoop() {
	t := time.NewTimer(updateFirstIn)
	for {
		<-t.C
		a.mu.Lock()
		off := a.settings.NoUpdateCheck
		a.mu.Unlock()
		if !off {
			_, _ = a.CheckUpdate()
		}
		t.Reset(updateEvery)
	}
}

// updateBlocker diz o que impede trocar de versão agora ("" = pode).
func (a *App) updateBlocker() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	switch {
	case a.alerts.danger != 0 || a.dangerOn:
		return "perigo ativo"
	case a.alerts.alarm != 0 || a.alarmOn && a.settings.AlarmEnabled:
		return "alarme ativo"
	case a.alerts.attn != 0 || a.attnOn:
		return "atenção ativa"
	case a.sat.reason != "":
		return "Geiger saturado"
	case a.measure.Phase == "fundo" || a.measure.Phase == "objeto":
		return "medição de objeto em andamento"
	case a.downloading.Load():
		return "baixando o histórico do contador"
	case a.cfgBusy.Load():
		return "gravando a configuração do contador"
	}
	return ""
}

// StartUpdate baixa, confere e troca de versão (esperando o CPM normal).
func (a *App) StartUpdate() error {
	info := a.updateInfo()
	if !info.Available {
		return errors.New("não há versão nova")
	}
	if !info.CanAuto {
		return errors.New(info.Manual)
	}
	a.upd.mu.Lock()
	if a.upd.busy {
		a.upd.mu.Unlock()
		return nil
	}
	a.upd.busy = true
	asset, sums, sig := a.upd.info.asset, a.upd.info.sums, a.upd.info.sig
	a.upd.mu.Unlock()
	a.goSafe("runUpdate", func() { a.runUpdate(info.Latest, asset, sums, sig) })
	return nil
}

// CancelUpdate desiste de esperar (o arquivo baixado fica na pasta).
func (a *App) CancelUpdate() {
	a.upd.mu.Lock()
	cancel := a.upd.info.State == "esperando"
	if cancel {
		a.upd.info.State = "cancelado"
	}
	a.upd.mu.Unlock()
	if cancel {
		a.emit("update", a.updateInfo())
	}
}

func (a *App) runUpdate(version string, asset, sums, sig ghAsset) {
	fail := func(err error) {
		a.logf("atualização para %s falhou: %v", version, err)
		a.setUpdate(func(i *UpdateInfo) { i.State, i.Error = "erro", err.Error() })
		a.upd.mu.Lock()
		a.upd.busy = false
		a.upd.mu.Unlock()
	}
	a.setUpdate(func(i *UpdateInfo) { i.State, i.Progress, i.Error, i.Wait = "baixando", 0, "", "" })
	if !assetIsVersion(asset.Name, version) {
		fail(fmt.Errorf("o arquivo da release (%s) não é o da versão %s; não vou instalar", asset.Name, version))
		return
	}
	want, err := expectedSum(sums.URL, sig.URL, asset.Name)
	if err != nil {
		fail(err)
		return
	}
	dir := updateDir()
	if dir == "" {
		dir = filepath.Join(a.dataDir, "versoes")
	}
	path, err := a.downloadVerified(asset, want, dir)
	if err != nil && dir != filepath.Join(a.dataDir, "versoes") {
		// pasta do .exe sem permissão de escrita: guarda na pasta de dados
		a.logf("atualização: não deu para salvar em %s (%v); usando a pasta de dados", dir, err)
		path, err = a.downloadVerified(asset, want, filepath.Join(a.dataDir, "versoes"))
	}
	if err != nil {
		fail(err)
		return
	}
	a.logf("atualização: %s baixado e conferido (SHA-256 %s…) em %s", asset.Name, want[:12], path)
	if runtime.GOOS == "windows" {
		if fixed, err := installFixed(path); err != nil {
			a.logf("atualização: não deu para pôr como %s (%v); abrindo %s", fixedExe, err, filepath.Base(path))
		} else {
			path = fixed
		}
	}

	// espera o CPM normal
	for {
		a.upd.mu.Lock()
		cancelled := a.upd.info.State == "cancelado"
		a.upd.mu.Unlock()
		if cancelled {
			a.logf("atualização: troca cancelada; %s ficou em %s", asset.Name, path)
			a.upd.mu.Lock()
			a.upd.busy = false
			a.upd.mu.Unlock()
			return
		}
		why := a.updateBlocker()
		if why == "" {
			break
		}
		a.setUpdate(func(i *UpdateInfo) { i.State, i.Wait = "esperando", why })
		time.Sleep(updateWaitTick)
	}
	a.setUpdate(func(i *UpdateInfo) { i.State, i.Wait = "abrindo", "" })
	a.logEvent("Atualizando para a versão " + version)
	if err := launchUpdated(path); err != nil {
		fail(fmt.Errorf("não consegui abrir a versão nova: %w", err))
		return
	}
	a.quitApp()
}

// ---------- nome fixo no Windows ----------
//
// O Windows guarda pelo caminho do .exe se o ícone fica fixo na barra ou
// escondido na setinha, e também a regra do firewall da sincronização. Com
// um nome por versão (RadLog-vX.Y.Z.exe), cada atualização parecia outro
// programa: o ícone voltava para a setinha e o firewall perguntava de novo.
// Agora a versão baixada e conferida vira sempre RadLog.exe na pasta das
// versões. Um .exe aberto não pode ser sobrescrito, mas pode ser renomeado:
// o atual vira RadLog.old.exe (apagado depois, com as versões antigas) e o
// novo assume o nome. Se algo falhar, abre o arquivo com o nome da versão,
// como antes.

const (
	fixedExe = "RadLog.exe"
	oldExe   = "RadLog.old.exe"
)

// installFixed renomeia o .exe baixado para RadLog.exe na mesma pasta e
// devolve o caminho novo.
func installFixed(path string) (string, error) {
	dir := filepath.Dir(path)
	fixed := filepath.Join(dir, fixedExe)
	if sameExe(path, fixed) {
		return path, nil
	}
	old := filepath.Join(dir, oldExe)
	_ = os.Remove(old) // sobra da troca anterior
	moved := false
	if _, err := os.Stat(fixed); err == nil {
		if err := os.Rename(fixed, old); err != nil {
			return path, err
		}
		moved = true
	}
	if err := os.Rename(path, fixed); err != nil {
		if moved {
			_ = os.Rename(old, fixed)
		}
		return path, err
	}
	return fixed, nil
}

// updateDir é a pasta das versões, só do app: Documentos\RadLog\Versões
// (ao lado de Relatórios), para não encher a pasta Downloads.
func updateDir() string {
	if d := defaultReportDir(); d != "" {
		return filepath.Join(filepath.Dir(d), "Versões")
	}
	return ""
}

// GetVersionsDir devolve a pasta das versões (para o botão "Abrir pasta").
func (a *App) GetVersionsDir() string {
	if d := updateDir(); d != "" {
		return d
	}
	return filepath.Join(a.dataDir, "versoes")
}

// OpenVersionsDir abre a pasta das versões no explorador.
func (a *App) OpenVersionsDir() error {
	d := a.GetVersionsDir()
	if err := os.MkdirAll(d, 0o755); err != nil {
		return err
	}
	return openFolder(d)
}

// ---------- apagar as versões antigas ----------
//
// Depois que a versão nova fica aberta e funcionando por cleanAfter, o app
// apaga os .exe (ou AppImages) de versões mais antigas que ela: os da pasta
// das versões e os da pasta de onde veio a versão anterior (normalmente
// Downloads). Só arquivos com o nome das releases (RadLog-vX.Y.Z.exe ou,
// da 1.x, GMC-Monitor-vX.Y.Z.exe) e o RadLog.old.exe da troca de nome fixo,
// nunca a versão que está aberta nem uma mais nova.

const (
	prevArg    = "--anterior=" // caminho do .exe que abriu esta versão pelo atualizador
	cleanAfter = 2 * time.Minute
)

var releaseName = regexp.MustCompile(`^(?i)(?:GMC-Monitor|RadLog)-v(\d+\.\d+\.\d+)(\.exe|-x86_64\.AppImage)(\.part)?$`)

// oldVersionsIn lista os arquivos de releases mais antigas que AppVersion.
func oldVersionsIn(dir, self string) []string {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range ents {
		m := releaseName.FindStringSubmatch(e.Name())
		if e.IsDir() || m == nil {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if sameExe(p, self) {
			continue
		}
		if m[3] == ".part" || newerVersion(AppVersion, m[1]) {
			out = append(out, p)
		}
	}
	if p := filepath.Join(dir, oldExe); !sameExe(p, self) {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			out = append(out, p) // a versão anterior, de antes da troca
		}
	}
	return out
}

func previousExe() string {
	for _, a := range os.Args[1:] {
		if strings.HasPrefix(a, prevArg) {
			return strings.TrimPrefix(a, prevArg)
		}
	}
	return ""
}

// cleanOldVersionsLater espera a versão nova firmar e apaga as antigas.
func (a *App) cleanOldVersionsLater() {
	time.Sleep(cleanAfter)
	a.mu.Lock()
	off := a.settings.KeepOldVersions
	a.mu.Unlock()
	if off || a.closing.Load() {
		return
	}
	self := selfExe()
	if p := os.Getenv("APPIMAGE"); p != "" {
		self = p
	}
	dirs := map[string]bool{}
	if d := updateDir(); d != "" {
		dirs[d] = true
	}
	dirs[filepath.Join(a.dataDir, "versoes")] = true
	if d := oldVersionsDir(); d != "" {
		dirs[d] = true // Documentos/GMC Monitor/Versões, da 1.x
	}
	if prev := previousExe(); prev != "" {
		dirs[filepath.Dir(prev)] = true
	}
	var gone []string
	for d := range dirs {
		for _, f := range oldVersionsIn(d, self) {
			if err := os.Remove(f); err == nil {
				gone = append(gone, filepath.Base(f))
			} else {
				a.logf("versões antigas: não consegui apagar %s: %v", f, err)
			}
		}
	}
	if len(gone) > 0 {
		sort.Strings(gone)
		a.logf("versões antigas apagadas: %s", strings.Join(gone, ", "))
	}
}

// expectedSum lê o SHA-256 do arquivo no SHA256SUMS.txt da release, depois
// de conferir a assinatura dele (SHA256SUMS.txt.sig, updsig).
func expectedSum(sumsURL, sigURL, name string) (string, error) {
	b, err := fetchSmall(sumsURL, updateSums)
	if err != nil {
		return "", err
	}
	sig, err := fetchSmall(sigURL, updateSig)
	if err != nil {
		return "", err
	}
	return checkSums(b, sig, name, verifySums)
}

// verifySums confere a assinatura do SHA256SUMS.txt (troca só nos testes).
var verifySums = updsig.Verify

// checkSums confere a assinatura do SHA256SUMS.txt e acha a soma de name.
func checkSums(sums, sig []byte, name string, verify func(data, sig []byte) bool) (string, error) {
	if !verify(sums, sig) {
		return "", errors.New("a assinatura do " + updateSums + " não confere; não vou instalar esta versão")
	}
	return parseSums(string(sums), name)
}

// fetchSmall baixa um arquivo pequeno da release (até 64 KB).
func fetchSmall(url, what string) ([]byte, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "RadLog-updater/"+AppVersion)
	resp, err := updateClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("não consegui baixar o %s: %w", what, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: GitHub respondeu %s", what, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<10))
}

// parseSums acha a linha "<sha256>  <arquivo>" (formato do sha256sum).
func parseSums(text, name string) (string, error) {
	for _, ln := range strings.Split(text, "\n") {
		f := strings.Fields(ln)
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == name && len(f[0]) == 64 {
			if _, err := hex.DecodeString(f[0]); err == nil {
				return strings.ToLower(f[0]), nil
			}
		}
	}
	return "", fmt.Errorf("%s não tem o %s", updateSums, name)
}

// downloadVerified baixa para dir/<nome>.part, confere o tamanho e o
// SHA-256 e só então renomeia. Se o arquivo já existe e confere, usa ele.
func (a *App) downloadVerified(asset ghAsset, want, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	final := filepath.Join(dir, filepath.Base(asset.Name))
	if got, err := fileSHA256(final); err == nil && got == want {
		a.setUpdate(func(i *UpdateInfo) { i.Progress = 1 })
		return final, nil
	}
	part := final + ".part"
	f, err := os.Create(part)
	if err != nil {
		return "", err
	}
	ok := false
	defer func() {
		f.Close()
		if !ok {
			os.Remove(part)
		}
	}()
	client := &http.Client{Timeout: 15 * time.Minute}
	req, _ := http.NewRequest("GET", asset.URL, nil)
	req.Header.Set("User-Agent", "RadLog-updater/"+AppVersion)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("não consegui baixar: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download: GitHub respondeu %s", resp.Status)
	}
	h := sha256.New()
	var n int64
	buf := make([]byte, 256<<10)
	lastEmit := time.Time{}
	for {
		k, rerr := resp.Body.Read(buf)
		if k > 0 {
			if _, werr := f.Write(buf[:k]); werr != nil {
				return "", werr
			}
			h.Write(buf[:k])
			n += int64(k)
			if asset.Size > 0 && time.Since(lastEmit) > 300*time.Millisecond {
				lastEmit = time.Now()
				p := float64(n) / float64(asset.Size)
				a.setUpdate(func(i *UpdateInfo) { i.Progress = p })
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return "", fmt.Errorf("download interrompido: %w", rerr)
		}
	}
	if asset.Size > 0 && n != asset.Size {
		return "", fmt.Errorf("download incompleto (%d de %d bytes)", n, asset.Size)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return "", errors.New("o arquivo baixado não confere com o SHA256SUMS.txt; não vou abrir")
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	mode := os.FileMode(0o644)
	if runtime.GOOS != "windows" {
		mode = 0o755
	}
	_ = os.Chmod(part, mode)
	if err := os.Rename(part, final); err != nil {
		return "", err
	}
	ok = true
	a.setUpdate(func(i *UpdateInfo) { i.Progress = 1 })
	return final, nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// launchUpdated abre a versão nova. No Windows ela espera esta soltar a
// trava (--atualizando); no Linux um shell espera 3 s, para esta já ter
// saído quando a nova pedir a trava de cópia única.
var launchUpdated = func(path string) error {
	prev := selfExe()
	if p := os.Getenv("APPIMAGE"); p != "" {
		prev = p
	}
	if runtime.GOOS == "windows" {
		return startDetached(path, []string{updatingArg, prevArg + prev})
	}
	return startDetached("/bin/sh", []string{"-c", `sleep 3; exec "$0" ` + updatingArg + ` "$1"`, path, prevArg + prev})
}
