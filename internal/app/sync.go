package app

import (
	"archive/zip"
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"radlog/internal/store"
)

// ---------- sincronização entre PCs ----------
//
// Dois apps (ex.: o PC com o contador e o notebook) trocam o que falta um no
// outro: leituras por minuto, alertas, períodos fora de casa, medições,
// anotações e os segundos dos alertas. Nada é apagado nem substituído: é a
// mesma mescla do "Importar backup", só que pela rede.
//
//   - Um dos dois "aceita conexões" (escuta na porta 47862). No Windows isso
//     pede liberação no firewall; no Linux, não. O outro só conecta.
//   - Pareamento: quem aceita conexões mostra um código de 6 dígitos; o outro
//     digita. O código vira uma chave de 32 bytes pelo CPace (cpace.go) e não
//     vale mais depois.
//   - A cada 10 min (ou em "Sincronizar agora"), quem conecta pede o
//     inventário do outro (minutos de cada dia, resumidos), puxa os dias que
//     faltam aqui e manda os que faltam lá. Tudo cifrado com a chave do par.
//   - Endereços: o app guarda os IPs que o outro informou (rede de casa e
//     Tailscale, 100.x.y.z) e tenta um por um. Fora de casa, com o Tailscale
//     ligado nos dois, funciona sem mudar nada.

const (
	syncPort     = 47862
	syncEvery    = 10 * time.Minute
	pairCodeTTL  = 5 * time.Minute
	pairMaxTries = 5
	syncBase     = "/radlog/v1/"
	syncSkew     = 5 * time.Minute
	syncQuery    = "RADLOG?v1"
	// tamanho máximo de uma mensagem e prazo para o servidor ler o pedido
	// inteiro (B4 da auditoria; eram 512 MB e sem prazo). Um ano de minutos
	// dos dois aparelhos fica bem abaixo disto.
	syncMaxMsg      = 128 << 20
	syncReadTimeout = 5 * time.Minute
)

type syncPeer struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Key         []byte   `json:"key"`
	Addrs       []string `json:"addrs"`       // informados pelo outro (host:porta)
	ManualAddrs []string `json:"manualAddrs"` // digitados aqui (ex.: Tailscale)
	Paired      int64    `json:"paired"`
	LastSync    int64    `json:"lastSync"`
	LastTry     int64    `json:"lastTry"`
	LastAddr    string   `json:"lastAddr"`
	LastError   string   `json:"lastError"`
	LastResult  string   `json:"lastResult"`
}

type syncFile struct {
	ID    string     `json:"id"`
	Peers []syncPeer `json:"peers"`
}

type pairPending struct {
	kc       []byte
	id, name string
	addrs    []string
	until    time.Time
}

type invMonth struct {
	mod  time.Time
	size int64
	days map[string]string
}

type syncState struct {
	mu        sync.Mutex
	loaded    bool
	file      syncFile
	srv       *http.Server
	udp       *net.UDPConn
	ln        net.Listener
	port      int
	listenErr string
	code      string
	codeUntil time.Time
	codeTries int
	pending   map[string]*pairPending
	nonces    map[string]time.Time
	running   bool
	inv       map[string]invMonth
	runMu     sync.Mutex // uma sincronização por vez
}

// ---------- o que a tela vê ----------

// SyncPeerInfo é um PC pareado, para a tela.
type SyncPeerInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Addrs       []string `json:"addrs"`
	ManualAddrs []string `json:"manualAddrs"`
	Paired      int64    `json:"paired"`
	LastSync    int64    `json:"lastSync"`
	LastTry     int64    `json:"lastTry"`
	LastAddr    string   `json:"lastAddr"`
	LastError   string   `json:"lastError"`
	LastResult  string   `json:"lastResult"`
}

// SyncInfo é o estado da sincronização.
type SyncInfo struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Listen    bool           `json:"listen"`
	Listening bool           `json:"listening"`
	ListenErr string         `json:"listenErr"`
	Addrs     []string       `json:"addrs"`
	Code      string         `json:"code"`
	CodeUntil int64          `json:"codeUntil"`
	Auto      bool           `json:"auto"`
	Running   bool           `json:"running"`
	Peers     []SyncPeerInfo `json:"peers"`
}

// SyncFound é um PC achado na rede.
type SyncFound struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Addr   string `json:"addr"`
	Paired bool   `json:"paired"`
}

func (a *App) syncPath() string { return filepath.Join(a.dataDir, "sincronizacao.json") }

// syncLoad lê o arquivo (uma vez) e cria a identidade deste PC.
func (a *App) syncLoad() {
	s := &a.sync
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loaded {
		return
	}
	s.loaded = true
	s.pending = map[string]*pairPending{}
	s.nonces = map[string]time.Time{}
	s.inv = map[string]invMonth{}
	if b, err := os.ReadFile(a.syncPath()); err == nil {
		_ = json.Unmarshal(b, &s.file)
	}
	if s.file.ID == "" {
		s.file.ID = randHex(8)
		_ = a.syncSaveLocked()
	}
}

func (a *App) syncSaveLocked() error {
	if a.dataDir == "" {
		return nil
	}
	b, _ := json.MarshalIndent(a.sync.file, "", "  ")
	tmp := a.syncPath() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, a.syncPath())
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// GetSync devolve o estado para a tela.
func (a *App) GetSync() SyncInfo {
	a.syncLoad()
	a.mu.Lock()
	listen, auto := a.settings.SyncListen, !a.settings.NoSyncAuto
	a.mu.Unlock()
	s := &a.sync
	s.mu.Lock()
	defer s.mu.Unlock()
	info := SyncInfo{ID: s.file.ID, Name: hostName(), Listen: listen, Listening: s.ln != nil, ListenErr: s.listenErr,
		Auto: auto, Running: s.running, Peers: []SyncPeerInfo{}}
	if s.ln != nil {
		info.Addrs = localAddrs(s.port)
	}
	if s.code != "" && time.Now().Before(s.codeUntil) {
		info.Code, info.CodeUntil = s.code, s.codeUntil.UnixMilli()
	}
	for _, p := range s.file.Peers {
		info.Peers = append(info.Peers, SyncPeerInfo{ID: p.ID, Name: p.Name, Addrs: p.Addrs, ManualAddrs: p.ManualAddrs, Paired: p.Paired,
			LastSync: p.LastSync, LastTry: p.LastTry, LastAddr: p.LastAddr, LastError: p.LastError, LastResult: p.LastResult})
	}
	return info
}

func (a *App) emitSync() { a.emit("sync", a.GetSync()) }

// localAddrs são os IPv4 deste PC (rede de casa, Tailscale…) com a porta.
func localAddrs(port int) []string {
	var out []string
	ifs, _ := net.Interfaces()
	for _, ifc := range ifs {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, ad := range addrs {
			ipn, ok := ad.(*net.IPNet)
			if !ok || ipn.IP.To4() == nil || ipn.IP.IsLinkLocalUnicast() {
				continue
			}
			out = append(out, net.JoinHostPort(ipn.IP.String(), strconv.Itoa(port)))
		}
	}
	sort.Strings(out)
	return out
}

// ---------- servidor (quem aceita conexões) ----------

// SetSyncListen liga ou desliga "aceitar conexões de outros PCs".
func (a *App) SetSyncListen(on bool) SyncInfo {
	a.mu.Lock()
	a.settings.SyncListen = on
	a.saveSettingsLocked()
	a.mu.Unlock()
	if on {
		a.syncStartServer(fmt.Sprintf(":%d", syncPort))
	} else {
		a.syncStopServer()
	}
	a.emitSync()
	return a.GetSync()
}

// SetSyncAuto liga ou desliga a sincronização automática.
func (a *App) SetSyncAuto(on bool) SyncInfo {
	a.mu.Lock()
	a.settings.NoSyncAuto = !on
	a.saveSettingsLocked()
	a.mu.Unlock()
	a.emitSync()
	return a.GetSync()
}

func (a *App) syncHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(syncBase+"pair/start", a.handlePairStart)
	mux.HandleFunc(syncBase+"pair/confirm", a.handlePairConfirm)
	mux.HandleFunc(syncBase+"inventory", a.secure(a.handleInventory))
	mux.HandleFunc(syncBase+"pull", a.secure(a.handlePull))
	mux.HandleFunc(syncBase+"push", a.secure(a.handlePush))
	return mux
}

// syncStartServer escuta em addr (":47862"; os testes usam "127.0.0.1:0").
func (a *App) syncStartServer(addr string) {
	a.syncLoad()
	s := &a.sync
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ln != nil {
		return
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		s.listenErr = err.Error()
		a.logf("sincronização: não consegui escutar em %s: %v", addr, err)
		return
	}
	s.ln, s.listenErr = ln, ""
	s.port = ln.Addr().(*net.TCPAddr).Port
	s.srv = &http.Server{Handler: a.syncHandler(), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: syncReadTimeout, MaxHeaderBytes: 16 << 10}
	srv := s.srv
	a.goSafe("sync serve", func() { _ = srv.Serve(ln) })
	// descoberta: responde quem pergunta "RADLOG?v1" na mesma porta (UDP)
	if host, _, _ := net.SplitHostPort(addr); host == "" || host == "0.0.0.0" {
		if uc, err := net.ListenUDP("udp4", &net.UDPAddr{Port: s.port}); err == nil {
			s.udp = uc
			id, port := s.file.ID, s.port
			a.goSafe("syncAnswerDiscovery", func() { a.syncAnswerDiscovery(uc, id, port) })
		}
	}
	a.logf("sincronização: aceitando conexões na porta %d (%s)", s.port, strings.Join(localAddrs(s.port), ", "))
}

func (a *App) syncStopServer() {
	s := &a.sync
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.srv != nil {
		_ = s.srv.Close()
	}
	if s.udp != nil {
		_ = s.udp.Close()
	}
	if s.ln != nil {
		a.logf("sincronização: parou de aceitar conexões")
	}
	s.srv, s.udp, s.ln, s.code = nil, nil, nil, ""
}

func (a *App) syncAnswerDiscovery(uc *net.UDPConn, id string, port int) {
	buf := make([]byte, 64)
	for {
		n, from, err := uc.ReadFromUDP(buf)
		if err != nil {
			return
		}
		if string(buf[:n]) != syncQuery {
			continue
		}
		// só responde enquanto o código de pareamento está na tela: fora disso,
		// numa rede pública, ninguém fica sabendo o nome deste PC (S6)
		if !a.syncCodeShown() {
			continue
		}
		b, _ := json.Marshal(map[string]any{"app": "radlog", "id": id, "name": hostName(), "port": port})
		_, _ = uc.WriteToUDP(b, from)
	}
}

// syncCodeShown diz se há um código de pareamento valendo agora.
func (a *App) syncCodeShown() bool {
	s := &a.sync
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.code != "" && time.Now().Before(s.codeUntil)
}

// ShowPairCode cria um código de 6 dígitos (vale 5 min, um pareamento).
func (a *App) ShowPairCode() (SyncInfo, error) {
	a.syncLoad()
	s := &a.sync
	s.mu.Lock()
	if s.ln == nil {
		s.mu.Unlock()
		return a.GetSync(), errors.New("ligue \"Aceitar conexões de outros PCs\" primeiro")
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	s.code = fmt.Sprintf("%06d", n.Int64())
	s.codeUntil = time.Now().Add(pairCodeTTL)
	s.codeTries = 0
	s.mu.Unlock()
	a.emitSync()
	return a.GetSync(), nil
}

// CancelPairCode apaga o código antes da hora.
func (a *App) CancelPairCode() SyncInfo {
	s := &a.sync
	s.mu.Lock()
	s.code = ""
	s.mu.Unlock()
	a.emitSync()
	return a.GetSync()
}

type pairStartReq struct {
	SID   string   `json:"sid"`
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Y     []byte   `json:"y"`
	Addrs []string `json:"addrs"`
}

type pairStartResp struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Y     []byte   `json:"y"`
	MAC   []byte   `json:"mac"`
	Addrs []string `json:"addrs"`
}

func pairTranscript(sid, cid string, cy []byte, sidSrv string, sy []byte) []byte {
	var b bytes.Buffer
	for _, x := range [][]byte{[]byte(sid), []byte(cid), cy, []byte(sidSrv), sy} {
		b.WriteString(strconv.Itoa(len(x)))
		b.WriteByte(':')
		b.Write(x)
	}
	return b.Bytes()
}

func readJSON(r *http.Request, v any) error {
	return json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(v)
}

func (a *App) handlePairStart(w http.ResponseWriter, r *http.Request) {
	var req pairStartReq
	if r.Method != http.MethodPost || readJSON(r, &req) != nil || req.SID == "" || req.ID == "" || len(req.Y) != 32 {
		http.Error(w, "pedido inválido", http.StatusBadRequest)
		return
	}
	s := &a.sync
	s.mu.Lock()
	code, ok := s.code, s.code != "" && time.Now().Before(s.codeUntil)
	myID, port := s.file.ID, s.port
	s.mu.Unlock()
	if !ok {
		http.Error(w, "nenhum código ativo neste PC", http.StatusForbidden)
		return
	}
	side, err := newCPace(code)
	if err != nil {
		http.Error(w, "erro", http.StatusInternalServerError)
		return
	}
	kc, err := side.finish(req.Y, pairTranscript(req.SID, req.ID, req.Y, myID, side.Y))
	if err != nil {
		http.Error(w, "ponto inválido", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	for k, p := range s.pending {
		if time.Now().After(p.until) {
			delete(s.pending, k)
		}
	}
	s.pending[req.SID] = &pairPending{kc: kc, id: req.ID, name: req.Name, addrs: req.Addrs, until: time.Now().Add(time.Minute)}
	s.mu.Unlock()
	_ = json.NewEncoder(w).Encode(pairStartResp{ID: myID, Name: hostName(), Y: side.Y, MAC: pairMAC(kc, "server"), Addrs: localAddrs(port)})
}

func (a *App) handlePairConfirm(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SID string `json:"sid"`
		MAC []byte `json:"mac"`
	}
	if r.Method != http.MethodPost || readJSON(r, &req) != nil {
		http.Error(w, "pedido inválido", http.StatusBadRequest)
		return
	}
	s := &a.sync
	s.mu.Lock()
	p := s.pending[req.SID]
	delete(s.pending, req.SID)
	if p == nil || s.code == "" {
		s.mu.Unlock()
		http.Error(w, "pareamento expirou", http.StatusForbidden)
		return
	}
	if !hmac.Equal(req.MAC, pairMAC(p.kc, "client")) {
		s.codeTries++
		if s.codeTries >= pairMaxTries {
			s.code = "" // tentativas demais: o código para de valer
		}
		s.mu.Unlock()
		a.logf("sincronização: código errado no pareamento (%s)", r.RemoteAddr)
		a.emitSync()
		http.Error(w, "código errado", http.StatusForbidden)
		return
	}
	key, _ := pairKey(p.kc)
	s.code = ""
	s.addPeerLocked(syncPeer{ID: p.id, Name: p.name, Key: key, Addrs: p.addrs, Paired: time.Now().UnixMilli()})
	_ = a.syncSaveLocked()
	s.mu.Unlock()
	a.logEvent(fmt.Sprintf("Pareado com %s", p.name))
	a.emitSync()
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (s *syncState) addPeerLocked(p syncPeer) {
	for i := range s.file.Peers {
		if s.file.Peers[i].ID == p.ID {
			p.ManualAddrs = s.file.Peers[i].ManualAddrs
			s.file.Peers[i] = p
			return
		}
	}
	s.file.Peers = append(s.file.Peers, p)
}

func (s *syncState) peerLocked(id string) *syncPeer {
	for i := range s.file.Peers {
		if s.file.Peers[i].ID == id {
			return &s.file.Peers[i]
		}
	}
	return nil
}

// secure abre o envelope do pedido, chama fn e fecha o da resposta.
func (a *App) secure(fn func(peer string, body []byte) ([]byte, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "use POST", http.StatusMethodNotAllowed)
			return
		}
		id, ts := r.Header.Get("X-Radlog-Peer"), r.Header.Get("X-Radlog-TS")
		sec, _ := strconv.ParseInt(ts, 10, 64)
		if d := time.Since(time.Unix(sec, 0)); d > syncSkew || d < -syncSkew {
			http.Error(w, "relógio muito diferente entre os PCs", http.StatusForbidden)
			return
		}
		s := &a.sync
		s.mu.Lock()
		p := s.peerLocked(id)
		var key []byte
		if p != nil {
			key = p.Key
		}
		s.mu.Unlock()
		if key == nil {
			http.Error(w, "PC não pareado", http.StatusForbidden)
			return
		}
		box, err := io.ReadAll(io.LimitReader(r.Body, syncMaxMsg))
		if err != nil {
			return
		}
		plain, err := unseal(key, box, []byte("req|"+r.URL.Path+"|"+id+"|"+ts))
		if err != nil {
			http.Error(w, "mensagem inválida", http.StatusForbidden)
			return
		}
		// a mesma mensagem não vale duas vezes
		nonce := hex.EncodeToString(box[:12])
		s.mu.Lock()
		for k, t := range s.nonces {
			if time.Since(t) > 2*syncSkew {
				delete(s.nonces, k)
			}
		}
		seen := !s.nonces[nonce].IsZero()
		s.nonces[nonce] = time.Now()
		s.mu.Unlock()
		if seen {
			http.Error(w, "mensagem repetida", http.StatusForbidden)
			return
		}
		out, err := fn(id, plain)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		resp, err := seal(key, out, []byte("resp|"+r.URL.Path+"|"+id+"|"+ts))
		if err != nil {
			http.Error(w, "erro", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(resp)
	}
}

type invReq struct {
	Name  string   `json:"name"`
	Addrs []string `json:"addrs"`
}

type invResp struct {
	ID     string            `json:"id"`
	Name   string            `json:"name"`
	Addrs  []string          `json:"addrs"`
	Days   map[string]string `json:"days"`
	Alerts []string          `json:"alerts"`
	// 2.7.1: Radiacode e anexos (nil = PC com versão antiga)
	Rc     *rcInv   `json:"rc,omitempty"`
	Anexos []string `json:"anexos,omitempty"`
}

func (a *App) handleInventory(peer string, body []byte) ([]byte, error) {
	var req invReq
	_ = json.Unmarshal(body, &req)
	s := &a.sync
	s.mu.Lock()
	if p := s.peerLocked(peer); p != nil {
		if req.Name != "" {
			p.Name = req.Name
		}
		if req.Addrs != nil {
			p.Addrs = req.Addrs
		}
		_ = a.syncSaveLocked()
	}
	id, port := s.file.ID, s.port
	s.mu.Unlock()
	days, err := a.dayDigests()
	if err != nil {
		return nil, err
	}
	rc := a.rcInventory()
	return json.Marshal(invResp{ID: id, Name: hostName(), Addrs: localAddrs(port), Days: days, Alerts: a.alertFiles(), Rc: &rc, Anexos: a.attachInventory()})
}

type pullReq struct {
	Days    []string `json:"days"`
	Alerts  []string `json:"alerts"`
	RcDays  []string `json:"rcDays,omitempty"`
	RcFiles []string `json:"rcFiles,omitempty"`
	Anexos  []string `json:"anexos,omitempty"`
	Rc      bool     `json:"rc,omitempty"` // quem pede entende o pacote novo
}

func (a *App) handlePull(peer string, body []byte) ([]byte, error) {
	var req pullReq
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}
	return a.buildDelta(req)
}

func (a *App) handlePush(peer string, body []byte) ([]byte, error) {
	res, err := a.mergeDelta(body, a.syncPeerName(peer))
	if err != nil {
		return nil, err
	}
	if res.total() > 0 {
		s := &a.sync
		s.mu.Lock()
		if p := s.peerLocked(peer); p != nil {
			p.LastSync, p.LastResult, p.LastError = time.Now().UnixMilli(), "recebido: "+mergeText(res), ""
			_ = a.syncSaveLocked()
		}
		s.mu.Unlock()
		a.emitSync()
	}
	return json.Marshal(res)
}

func (a *App) syncPeerName(id string) string {
	s := &a.sync
	s.mu.Lock()
	defer s.mu.Unlock()
	if p := s.peerLocked(id); p != nil {
		return p.Name
	}
	return id
}

// ---------- inventário e pacote do que falta ----------

// dayDigests resume os minutos gravados de cada dia (UTC): "n:hash". Dois
// PCs com o mesmo resumo num dia têm os mesmos minutos.
func (a *App) dayDigests() (map[string]string, error) {
	if a.store == nil {
		return map[string]string{}, nil
	}
	files, err := a.store.MonthFiles()
	if err != nil {
		return nil, err
	}
	s := &a.sync
	out := map[string]string{}
	for _, fi := range files {
		s.mu.Lock()
		c, ok := s.inv[fi.Name()]
		s.mu.Unlock()
		if !ok || !c.mod.Equal(fi.ModTime()) || c.size != fi.Size() {
			pts, err := a.store.ReadMonthFile(fi.Name())
			if err != nil {
				return nil, err
			}
			c = invMonth{mod: fi.ModTime(), size: fi.Size(), days: digestDays(pts)}
			s.mu.Lock()
			s.inv[fi.Name()] = c
			s.mu.Unlock()
		}
		for d, h := range c.days {
			out[d] = h
		}
	}
	return out, nil
}

func digestDays(pts []store.Point) map[string]string {
	mins := map[string][]int64{}
	for _, p := range pts {
		d := p.T.UTC().Format("2006-01-02")
		mins[d] = append(mins[d], p.T.Unix()/60)
	}
	out := map[string]string{}
	for d, m := range mins {
		sort.Slice(m, func(i, j int) bool { return m[i] < m[j] })
		h := fnv.New64a()
		n := 0
		for i, x := range m {
			if i > 0 && x == m[i-1] {
				continue
			}
			n++
			h.Write([]byte(strconv.FormatInt(x, 36) + ","))
		}
		out[d] = fmt.Sprintf("%d:%x", n, h.Sum64())
	}
	return out
}

// alertFiles lista os segundos de alerta guardados.
func (a *App) alertFiles() []string {
	files, _ := filepath.Glob(filepath.Join(a.alertDir(), "*.json"))
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, filepath.Base(f))
	}
	return out
}

// buildDelta monta um "backup" só com os dias e segundos de alerta pedidos,
// mais os alertas encerrados, períodos, medições e anotações (pequenos).
func (a *App) buildDelta(req pullReq) ([]byte, error) {
	days, alerts := req.Days, req.Alerts
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	man, _ := json.Marshal(backupManifest{App: backupAppID, Version: AppVersion, Host: hostName(), Created: time.Now().Format(time.RFC3339)})
	if f, err := zw.Create("backup.json"); err == nil {
		_, _ = f.Write(man)
	}
	if len(days) > 0 && a.store != nil {
		var pts []store.Point
		for _, d := range days {
			t, err := time.Parse("2006-01-02", d)
			if err != nil {
				continue
			}
			p, err := a.store.Range(t, t.Add(24*time.Hour))
			if err != nil {
				return nil, err
			}
			pts = append(pts, p...)
		}
		if f, err := zw.Create("leituras/sincronizacao.csv"); err == nil {
			if err := store.WriteCSV(f, pts); err != nil {
				return nil, err
			}
		}
	}
	// só alertas encerrados (os de agora ainda mudam)
	a.mu.Lock()
	var rows [][]string
	for _, e := range a.events {
		if e.Kind == kindAlert && !e.Demo && (e.End != 0 || e.Retro) {
			rows = append(rows, eventRow(e))
		}
	}
	a.mu.Unlock()
	if f, err := zw.Create("eventos.csv"); err == nil {
		_ = csv.NewWriter(f).WriteAll(rows)
	}
	for _, name := range []string{"fora-de-casa.json", "medicoes.json", "anotacoes.json"} {
		if b, err := os.ReadFile(filepath.Join(a.dataDir, name)); err == nil {
			if f, err := zw.Create(name); err == nil {
				_, _ = f.Write(b)
			}
		}
	}
	for _, n := range alerts {
		n = filepath.Base(n)
		if !strings.HasSuffix(n, ".json") {
			continue
		}
		if b, err := os.ReadFile(filepath.Join(a.alertDir(), n)); err == nil {
			if f, err := zw.Create("alertas-detalhe/" + n); err == nil {
				_, _ = f.Write(b)
			}
		}
	}
	if req.Rc {
		if err := a.rcAddToDelta(zw, req.RcDays, req.RcFiles); err != nil {
			return nil, err
		}
		if err := a.attachAddToDelta(zw, req.Anexos); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// mergeDelta junta um pacote recebido (mesma mescla do Importar backup).
func (a *App) mergeDelta(zipBytes []byte, from string) (MergeResult, error) {
	dir := filepath.Join(a.dataDir, "tmp")
	_ = os.MkdirAll(dir, 0o755)
	f, err := os.CreateTemp(dir, "sincronizacao-*.zip")
	if err != nil {
		return MergeResult{}, err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err := f.Write(zipBytes); err != nil {
		f.Close()
		return MergeResult{}, err
	}
	f.Close()
	res, err := a.mergeZip(name, false)
	res.From = from
	return res, err
}

// ---------- cliente (quem conecta) ----------

var syncClient = &http.Client{Timeout: 3 * time.Minute}

// call manda um pedido cifrado para o par em addr.
func (a *App) syncCall(addr string, p syncPeer, op string, body []byte, timeout time.Duration) ([]byte, error) {
	s := &a.sync
	s.mu.Lock()
	myID := s.file.ID
	s.mu.Unlock()
	path := syncBase + op
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	box, err := seal(p.Key, body, []byte("req|"+path+"|"+myID+"|"+ts))
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequest(http.MethodPost, "http://"+addr+path, bytes.NewReader(box))
	req.Header.Set("X-Radlog-Peer", myID)
	req.Header.Set("X-Radlog-TS", ts)
	cl := *syncClient
	cl.Timeout = timeout
	resp, err := cl.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, syncMaxMsg))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s respondeu: %s", p.Name, strings.TrimSpace(string(data)))
	}
	return unseal(p.Key, data, []byte("resp|"+path+"|"+myID+"|"+ts))
}

func normAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, strconv.Itoa(syncPort))
	}
	return addr
}

// PairWith pareia com o PC em addr usando o código que ele mostra.
func (a *App) PairWith(addr, code string) (SyncInfo, error) {
	a.syncLoad()
	addr = normAddr(addr)
	code = strings.TrimSpace(strings.ReplaceAll(code, " ", ""))
	if addr == "" || len(code) != 6 {
		return a.GetSync(), errors.New("informe o endereço e o código de 6 dígitos")
	}
	s := &a.sync
	s.mu.Lock()
	myID, port, listening := s.file.ID, s.port, s.ln != nil
	s.mu.Unlock()
	var myAddrs []string
	if listening {
		myAddrs = localAddrs(port)
	}
	side, err := newCPace(code)
	if err != nil {
		return a.GetSync(), err
	}
	sid := randHex(16)
	b, _ := json.Marshal(pairStartReq{SID: sid, ID: myID, Name: hostName(), Y: side.Y, Addrs: myAddrs})
	cl := &http.Client{Timeout: 10 * time.Second}
	resp, err := cl.Post("http://"+addr+syncBase+"pair/start", "application/json", bytes.NewReader(b))
	if err != nil {
		return a.GetSync(), fmt.Errorf("não consegui falar com %s: %w", addr, err)
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return a.GetSync(), fmt.Errorf("o outro PC recusou: %s", strings.TrimSpace(string(data)))
	}
	var sr pairStartResp
	if err := json.Unmarshal(data, &sr); err != nil || len(sr.Y) != 32 || sr.ID == "" {
		return a.GetSync(), errors.New("resposta estranha do outro PC")
	}
	kc, err := side.finish(sr.Y, pairTranscript(sid, myID, side.Y, sr.ID, sr.Y))
	if err != nil {
		return a.GetSync(), err
	}
	if !hmac.Equal(sr.MAC, pairMAC(kc, "server")) {
		// manda a confirmação errada mesmo assim, para o outro contar a tentativa
		b, _ = json.Marshal(map[string]any{"sid": sid, "mac": make([]byte, 32)})
		if r2, err := cl.Post("http://"+addr+syncBase+"pair/confirm", "application/json", bytes.NewReader(b)); err == nil {
			r2.Body.Close()
		}
		return a.GetSync(), errors.New("código errado")
	}
	b, _ = json.Marshal(map[string]any{"sid": sid, "mac": pairMAC(kc, "client")})
	r2, err := cl.Post("http://"+addr+syncBase+"pair/confirm", "application/json", bytes.NewReader(b))
	if err != nil {
		return a.GetSync(), err
	}
	data, _ = io.ReadAll(io.LimitReader(r2.Body, 4<<10))
	r2.Body.Close()
	if r2.StatusCode != http.StatusOK {
		return a.GetSync(), fmt.Errorf("o outro PC recusou: %s", strings.TrimSpace(string(data)))
	}
	key, _ := pairKey(kc)
	addrs := sr.Addrs
	if !containsStr(addrs, addr) {
		addrs = append([]string{addr}, addrs...)
	}
	s.mu.Lock()
	s.addPeerLocked(syncPeer{ID: sr.ID, Name: sr.Name, Key: key, Addrs: addrs, Paired: time.Now().UnixMilli(), LastAddr: addr})
	_ = a.syncSaveLocked()
	s.mu.Unlock()
	a.logEvent(fmt.Sprintf("Pareado com %s", sr.Name))
	a.emitSync()
	if a.ctx != nil { // já sincroniza (nos testes não há tela)
		a.goSafe("SyncNow", func() { _, _ = a.SyncNow(sr.ID) })
	}
	return a.GetSync(), nil
}

func containsStr(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// ForgetPeer desfaz o pareamento (deste lado).
func (a *App) ForgetPeer(id string) SyncInfo {
	a.syncLoad()
	s := &a.sync
	s.mu.Lock()
	for i := range s.file.Peers {
		if s.file.Peers[i].ID == id {
			a.logf("sincronização: pareamento com %s desfeito", s.file.Peers[i].Name)
			s.file.Peers = append(s.file.Peers[:i], s.file.Peers[i+1:]...)
			break
		}
	}
	_ = a.syncSaveLocked()
	s.mu.Unlock()
	a.emitSync()
	return a.GetSync()
}

// SetPeerAddrs guarda endereços digitados (ex.: o IP do Tailscale).
func (a *App) SetPeerAddrs(id string, addrs []string) SyncInfo {
	a.syncLoad()
	s := &a.sync
	s.mu.Lock()
	if p := s.peerLocked(id); p != nil {
		p.ManualAddrs = nil
		for _, x := range addrs {
			if x = normAddr(x); x != "" && !containsStr(p.ManualAddrs, x) {
				p.ManualAddrs = append(p.ManualAddrs, x)
			}
		}
		_ = a.syncSaveLocked()
	}
	s.mu.Unlock()
	a.emitSync()
	return a.GetSync()
}

// SyncResult é o que uma sincronização trouxe e levou.
type SyncResult struct {
	Peer     string      `json:"peer"`
	Addr     string      `json:"addr"`
	Got      MergeResult `json:"got"`
	Sent     MergeResult `json:"sent"`
	Duration int64       `json:"duration"`
}

// SyncNow sincroniza com um PC pareado (os dois lados ficam com tudo).
func (a *App) SyncNow(id string) (SyncResult, error) {
	a.syncLoad()
	s := &a.sync
	s.runMu.Lock()
	defer s.runMu.Unlock()
	s.mu.Lock()
	p := s.peerLocked(id)
	if p == nil {
		s.mu.Unlock()
		return SyncResult{}, errors.New("PC não pareado")
	}
	peer := *p
	myPort, listening := s.port, s.ln != nil
	s.running = true
	s.mu.Unlock()
	a.emitSync()
	res, err := a.syncOnce(peer, myPort, listening)
	s.mu.Lock()
	s.running = false
	prevErr := ""
	if p := s.peerLocked(id); p != nil {
		prevErr = p.LastError
		p.LastTry = time.Now().UnixMilli()
		if err != nil {
			p.LastError = err.Error()
		} else {
			p.LastSync, p.LastAddr, p.LastError = p.LastTry, res.Addr, ""
			p.LastResult = "trouxe: " + mergeText(res.Got) + " · levou: " + mergeText(res.Sent)
		}
		_ = a.syncSaveLocked()
	}
	s.mu.Unlock()
	// o mesmo erro a cada 10 min (o outro PC desligado) entra no log uma vez
	// só, e de novo quando muda ou quando volta a responder
	if err != nil {
		if err.Error() != prevErr {
			a.logf("sincronização com %s: %v", peer.Name, err)
		}
	} else if prevErr != "" {
		a.logf("sincronização com %s: voltou a responder", peer.Name)
	}
	if err == nil && res.Got.total()+res.Sent.total() > 0 {
		a.logf("sincronização com %s (%s): trouxe %s; levou %s", peer.Name, res.Addr, mergeText(res.Got), mergeText(res.Sent))
	}
	a.emitSync()
	return res, err
}

func (a *App) syncOnce(peer syncPeer, myPort int, listening bool) (SyncResult, error) {
	t0 := time.Now()
	var myAddrs []string
	if listening {
		myAddrs = localAddrs(myPort)
	}
	reqB, _ := json.Marshal(invReq{Name: hostName(), Addrs: myAddrs})
	// tenta o último endereço que deu certo, depois os outros
	var cands []string
	for _, x := range append(append([]string{peer.LastAddr}, peer.ManualAddrs...), peer.Addrs...) {
		if x != "" && !containsStr(cands, x) {
			cands = append(cands, x)
		}
	}
	if len(cands) == 0 {
		return SyncResult{}, errors.New("nenhum endereço conhecido; informe o IP do outro PC")
	}
	var inv invResp
	var addr string
	var lastErr error
	for _, c := range cands {
		b, err := a.syncCall(c, peer, "inventory", reqB, 6*time.Second)
		if err != nil {
			lastErr = err
			continue
		}
		if err := json.Unmarshal(b, &inv); err != nil {
			lastErr = err
			continue
		}
		addr = c
		break
	}
	if addr == "" {
		return SyncResult{}, fmt.Errorf("%s não respondeu (%v)", peer.Name, shortNetErr(lastErr))
	}
	res := SyncResult{Peer: peer.Name, Addr: addr}
	// endereços novos do outro (ex.: ligou o Tailscale)
	a.sync.mu.Lock()
	if p := a.sync.peerLocked(peer.ID); p != nil && len(inv.Addrs) > 0 {
		p.Addrs = inv.Addrs
		if inv.Name != "" {
			p.Name = inv.Name
		}
	}
	a.sync.mu.Unlock()

	mine, err := a.dayDigests()
	if err != nil {
		return res, err
	}
	// 1) puxa o que falta aqui
	var want []string
	for d, h := range inv.Days {
		if mine[d] != h {
			want = append(want, d)
		}
	}
	myAlerts := map[string]bool{}
	for _, n := range a.alertFiles() {
		myAlerts[n] = true
	}
	var wantAlerts []string
	for _, n := range inv.Alerts {
		if !myAlerts[n] {
			wantAlerts = append(wantAlerts, n)
		}
	}
	sort.Strings(want)
	preq := pullReq{Days: want, Alerts: wantAlerts}
	newPeer := inv.Rc != nil
	if newPeer {
		preq.Rc = true
		preq.RcDays, preq.RcFiles = rcWant(*inv.Rc, a.rcInventory())
		preq.Anexos = attachWant(inv.Anexos, a.attachInventory())
	}
	pb, _ := json.Marshal(preq)
	z, err := a.syncCall(addr, peer, "pull", pb, 5*time.Minute)
	if err != nil {
		return res, err
	}
	if res.Got, err = a.mergeDelta(z, peer.Name); err != nil {
		return res, err
	}
	// 2) manda o que falta lá
	mine, err = a.dayDigests()
	if err != nil {
		return res, err
	}
	var give []string
	for d, h := range mine {
		if inv.Days[d] != h {
			give = append(give, d)
		}
	}
	theirAlerts := map[string]bool{}
	for _, n := range inv.Alerts {
		theirAlerts[n] = true
	}
	var giveAlerts []string
	for _, n := range a.alertFiles() {
		if !theirAlerts[n] {
			giveAlerts = append(giveAlerts, n)
		}
	}
	sort.Strings(give)
	greq := pullReq{Days: give, Alerts: giveAlerts}
	if newPeer {
		greq.Rc = true
		greq.RcDays, greq.RcFiles = rcWant(a.rcInventory(), *inv.Rc)
		greq.Anexos = attachWant(a.attachInventory(), inv.Anexos)
	}
	delta, err := a.buildDelta(greq)
	if err != nil {
		return res, err
	}
	out, err := a.syncCall(addr, peer, "push", delta, 5*time.Minute)
	if err != nil {
		return res, err
	}
	_ = json.Unmarshal(out, &res.Sent)
	res.Duration = time.Since(t0).Milliseconds()
	return res, nil
}

func shortNetErr(err error) string {
	if err == nil {
		return "sem resposta"
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return "tempo esgotado"
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "refused"):
		return "conexão recusada: o outro PC está com \"Aceitar conexões\" ligado?"
	case strings.Contains(msg, "no route") || strings.Contains(msg, "unreachable"):
		return "sem rota até ele"
	}
	return msg
}

// DiscoverPeers procura na rede de casa quem aceita conexões (1,5 s).
func (a *App) DiscoverPeers() ([]SyncFound, error) {
	a.syncLoad()
	uc, err := net.ListenUDP("udp4", &net.UDPAddr{})
	if err != nil {
		return nil, err
	}
	defer uc.Close()
	for _, b := range broadcastAddrs() {
		_, _ = uc.WriteToUDP([]byte(syncQuery), &net.UDPAddr{IP: b, Port: syncPort})
	}
	s := &a.sync
	s.mu.Lock()
	myID := s.file.ID
	paired := map[string]bool{}
	for _, p := range s.file.Peers {
		paired[p.ID] = true
	}
	s.mu.Unlock()
	found := map[string]SyncFound{}
	deadline := time.Now().Add(1500 * time.Millisecond)
	buf := make([]byte, 512)
	for time.Now().Before(deadline) {
		_ = uc.SetReadDeadline(deadline)
		n, from, err := uc.ReadFromUDP(buf)
		if err != nil {
			break
		}
		var m struct {
			App  string `json:"app"`
			ID   string `json:"id"`
			Name string `json:"name"`
			Port int    `json:"port"`
		}
		if json.Unmarshal(buf[:n], &m) != nil || m.App != "radlog" || m.ID == "" || m.ID == myID {
			continue
		}
		found[m.ID] = SyncFound{ID: m.ID, Name: m.Name, Addr: net.JoinHostPort(from.IP.String(), strconv.Itoa(m.Port)), Paired: paired[m.ID]}
	}
	out := make([]SyncFound, 0, len(found))
	for _, f := range found {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func broadcastAddrs() []net.IP {
	out := []net.IP{net.IPv4bcast}
	ifs, _ := net.Interfaces()
	for _, ifc := range ifs {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagBroadcast == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, ad := range addrs {
			ipn, ok := ad.(*net.IPNet)
			if !ok || ipn.IP.To4() == nil {
				continue
			}
			ip, mask := ipn.IP.To4(), ipn.Mask
			if len(mask) == 16 {
				mask = mask[12:]
			}
			b := make(net.IP, 4)
			for i := 0; i < 4; i++ {
				b[i] = ip[i] | ^mask[i]
			}
			out = append(out, b)
		}
	}
	return out
}

// syncLoop: liga o servidor se estiver configurado e sincroniza sozinho.
func (a *App) syncLoop() {
	a.syncLoad()
	a.mu.Lock()
	listen := a.settings.SyncListen
	a.mu.Unlock()
	if listen {
		a.syncStartServer(fmt.Sprintf(":%d", syncPort))
	}
	time.Sleep(time.Minute)
	for {
		a.mu.Lock()
		auto := !a.settings.NoSyncAuto
		a.mu.Unlock()
		if auto && !a.closing.Load() {
			a.sync.mu.Lock()
			var ids []string
			for _, p := range a.sync.file.Peers {
				ids = append(ids, p.ID)
			}
			a.sync.mu.Unlock()
			for _, id := range ids {
				_, _ = a.SyncNow(id)
			}
		}
		time.Sleep(syncEvery)
	}
}
