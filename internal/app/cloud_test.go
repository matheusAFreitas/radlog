package app

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"radlog/internal/store"
	rs "radlog/internal/sync"
	"radlog/internal/sync/synctest"
)

// fakeCloud aponta o login e o Drive para o servidor de mentira.
func fakeCloud(t *testing.T) *synctest.Server {
	t.Helper()
	srv := synctest.New()
	oldO, oldD, oldU, oldID := cloudOAuth, cloudDriveFn, cloudOpenURL, googleClientID
	googleClientID = "cid"
	cloudOAuth = func() *rs.OAuth {
		o := rs.Google("cid", "sec")
		o.TokenURL = srv.TokenURL()
		return o
	}
	cloudDriveFn = func(tok func(context.Context) (string, error)) *rs.Drive {
		d := rs.NewDrive(tok)
		d.Base, d.Upload = srv.Base(), srv.Upload()
		return d
	}
	// o "navegador": volta direto com o código
	cloudOpenURL = func(a *App, u string) {
		go func() {
			pu, _ := url.Parse(u)
			q := pu.Query()
			resp, err := http.Get(q.Get("redirect_uri") + "?code=abc&state=" + url.QueryEscape(q.Get("state")))
			if err == nil {
				resp.Body.Close()
			}
		}()
	}
	t.Cleanup(func() {
		cloudOAuth, cloudDriveFn, cloudOpenURL, googleClientID = oldO, oldD, oldU, oldID
		srv.Close()
	})
	return srv
}

func cloudLogin(t *testing.T, a *App) *CloudLoginDone {
	t.Helper()
	if _, err := a.CloudLogin(); err != nil {
		t.Fatal(err)
	}
	if st := a.GetCloud().State; st != "wait" {
		t.Fatalf("estado no login: %s", st)
	}
	done, err := a.CloudLoginWait()
	if err != nil || done == nil {
		t.Fatalf("login: %v", err)
	}
	return done
}

func TestCloudTwoDevices(t *testing.T) {
	srv := fakeCloud(t)
	a, dirA := testApp(t)
	b, dirB := testApp(t)
	day := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	if _, err := a.store.Import(series(day, 20, 21, 22)); err != nil {
		t.Fatal(err)
	}
	if _, err := b.store.Import(series(day.AddDate(0, 1, 0), 30, 31)); err != nil {
		t.Fatal(err)
	}

	// primeiro aparelho
	done := cloudLogin(t, a)
	if done.Next != "new" {
		t.Fatalf("conta vazia: %q", done.Next)
	}
	code, _ := a.NewCloudCode()
	info, err := a.CloudFinishSetup(code)
	if err != nil || info.State != "ok" || info.Email != "voce@example.com" {
		t.Fatalf("ligar: %+v %v", info, err)
	}
	a.cloudSync()
	if st := a.GetCloud(); st.LastSync == 0 || st.Err != "" || st.Used == 0 {
		t.Fatalf("depois de subir: %+v", st)
	}
	n := srv.Count()
	if n < 3 { // metadados, comum, outubro, aparelho
		t.Fatalf("arquivos no Drive: %d", n)
	}
	// nada mudou: não sobe de novo
	calls := len(srv.Calls)
	a.cloudSync()
	for _, c := range srv.Calls[calls:] {
		if strings.HasPrefix(c, "PUT") {
			t.Fatalf("subiu de novo sem mudança: %v", srv.Calls[calls:])
		}
	}

	// segundo aparelho: código errado, depois o certo
	done = cloudLogin(t, b)
	if done.Next != "existing" || done.LastUp == 0 {
		t.Fatalf("conta com nuvem: %+v", done)
	}
	other, _ := rs.NewRecoveryCode()
	if _, err := b.CloudOpen(other); err != rs.ErrOpen {
		t.Fatalf("código errado: %v", err)
	}
	if _, err := b.CloudOpen(strings.ToLower(code)); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetCloudName("Notebook"); err != nil {
		t.Fatal(err)
	}
	b.cloudSync()
	pts, _ := b.store.Range(day, day.Add(time.Hour))
	if len(pts) != 3 {
		t.Fatalf("B recebeu %d minutos de A", len(pts))
	}
	a.cloudSync()
	pts, _ = a.store.Range(day.AddDate(0, 1, 0), day.AddDate(0, 1, 0).Add(time.Hour))
	if len(pts) != 2 {
		t.Fatalf("A recebeu %d minutos de B", len(pts))
	}
	devs := a.GetCloud().Devices
	if len(devs) != 2 || !devs[0].Me || devs[1].Name != "Notebook" || devs[1].LastUp == 0 {
		t.Fatalf("aparelhos: %+v", devs)
	}

	// o e-mail e o código não vão para o backup nem para o Drive em claro
	for _, f := range srv.Named(rs.MetaFile) {
		if strings.Contains(string(f.Data), "voce@") {
			t.Fatal("e-mail nos metadados")
		}
	}
	for _, f := range srv.Files {
		if strings.Contains(string(f.Data), "voce@") || strings.Contains(string(f.Data), code[:9]) {
			t.Fatalf("segredo em claro no Drive: %s", f.Name)
		}
	}
	key, _ := os.ReadFile(filepath.Join(dirA, "nuvem-chave.bin"))
	if len(key) == 0 {
		t.Fatal("sem nuvem-chave.bin")
	}
	if got, _ := a.CloudCode(); got != code {
		t.Fatalf("código guardado: %q", got)
	}
	_ = dirB

	// o Google revoga: pede para entrar de novo, e entrar volta ao normal
	srv.Revoked = true
	a.cloud.mu.Lock()
	a.cloud.access = ""
	a.cloud.mu.Unlock()
	a.cloudSync()
	if st := a.GetCloud().State; st != "login" {
		t.Fatalf("revogado: %s", st)
	}
	srv.Revoked = false
	if d := cloudLogin(t, a); d.Next != "" || d.Info.State != "ok" {
		t.Fatalf("entrar de novo: %+v", d)
	}

	// sem internet
	srv.Offline = true
	a.cloud.mu.Lock()
	a.cloud.access = ""
	a.cloud.mu.Unlock()
	a.cloudSync()
	if st := a.GetCloud().State; st == "ok" {
		t.Fatalf("fora do ar ficou ok")
	}
	srv.Offline = false

	// relógio errado
	srv.Skew = -10 * time.Minute
	a.cloudSync()
	if st := a.GetCloud(); st.State != "clock" || st.SkewSec < 500 {
		t.Fatalf("relógio: %+v", st)
	}
	srv.Skew = 0

	// desconectar apaga o código daqui e mantém o Drive
	before := srv.Count()
	if info := a.CloudDisconnect(); info.State != "off" {
		t.Fatalf("desconectar: %+v", info)
	}
	if _, err := os.Stat(filepath.Join(dirA, "nuvem-chave.bin")); err == nil {
		t.Fatal("chave ficou")
	}
	if srv.Count() != before {
		t.Fatal("desconectar mexeu no Drive")
	}
}

func TestCloudResetAndCancel(t *testing.T) {
	srv := fakeCloud(t)
	a, _ := testApp(t)
	cloudLogin(t, a)
	code, _ := a.NewCloudCode()
	if _, err := a.CloudFinishSetup(code); err != nil {
		t.Fatal(err)
	}
	b, _ := testApp(t)
	cloudLogin(t, b)
	before := srv.Count()
	if _, err := b.CloudReset(); err != nil {
		t.Fatal(err)
	}
	// N11: a antiga só vai para a lixeira quando a nova existir
	if srv.Count() != before || len(srv.Trashed) != 0 {
		t.Fatalf("a nuvem antiga saiu antes da nova: %d de %d, %d na lixeira", srv.Count(), before, len(srv.Trashed))
	}
	code2, _ := b.NewCloudCode()
	if _, err := b.CloudFinishSetup(code2); err != nil {
		t.Fatal(err)
	}
	if len(srv.Trashed) == 0 {
		t.Fatal("a antiga não foi para a lixeira")
	}
	// A percebe que a nuvem mudou
	a.cloudSync()
	if st := a.GetCloud(); st.Err == "" {
		t.Fatalf("A não percebeu a nuvem nova: %+v", st)
	}
	// cancelar no meio do login
	c, _ := testApp(t)
	cloudOpenURL = func(*App, string) {}
	if _, err := c.CloudLogin(); err != nil {
		t.Fatal(err)
	}
	canceled := make(chan struct{})
	go func() { time.Sleep(50 * time.Millisecond); c.CloudCancel(); close(canceled) }()
	d, err := c.CloudLoginWait()
	<-canceled // o CloudCancel termina antes do teste trocar as variáveis de volta
	if err != nil || d.Info.State != "off" {
		t.Fatalf("cancelar: %+v %v", d, err)
	}
}

func TestCloudPacksSkipSmallAndHost(t *testing.T) {
	a, _ := testApp(t)
	day := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	_, _ = a.store.Import(append(series(day, 20), store.Point{T: day.AddDate(0, 1, 0), CPM: 1, Src: store.SrcDevice}))
	ps, err := a.cloudPacks()
	if err != nil {
		t.Fatal(err)
	}
	keys := []string{}
	for _, p := range ps {
		keys = append(keys, p.key)
	}
	if strings.Join(keys, ",") != "comum,dia|2026-09-20,dia|2026-10-20" {
		t.Fatalf("pacotes: %v", keys)
	}
	z1, _ := a.buildDelta(ps[0].req)
	time.Sleep(1100 * time.Millisecond)
	z2, _ := a.buildDelta(ps[0].req)
	if string(z1) != string(z2) {
		t.Fatal("o comum muda sem os dados mudarem")
	}
	if strings.Contains(string(z1), hostName()) {
		t.Fatal("nome do PC no pacote da nuvem")
	}
	ps2, _ := a.cloudPacks()
	if ps2[1].digest != ps[1].digest {
		t.Fatal("resumo do mês mudou sem mudança")
	}
}

// N11: quem começa uma nuvem nova e desiste continua com a antiga.
func TestCloudResetThenCancelKeepsOld(t *testing.T) {
	srv := fakeCloud(t)
	a, _ := testApp(t)
	cloudPair(t, a, nil)
	b, _ := testApp(t)
	cloudLogin(t, b)
	if _, err := b.CloudReset(); err != nil {
		t.Fatal(err)
	}
	b.CloudCancel()
	if len(srv.Trashed) != 0 {
		t.Fatal("a nuvem antiga foi para a lixeira sem a nova existir")
	}
	c, _ := testApp(t)
	if done := cloudLogin(t, c); done.Next != "existing" {
		t.Fatalf("a nuvem antiga sumiu: %+v", done)
	}
}

// U2: o código sai da área de transferência depois do prazo, se ainda estiver
// lá; o que a pessoa copiou depois fica.
func TestCopyCloudCodeClears(t *testing.T) {
	var mu sync.Mutex
	clip := ""
	oldSet, oldGet, oldTTL := cloudClipSet, cloudClipGet, cloudClipTTL
	t.Cleanup(func() { cloudClipSet, cloudClipGet, cloudClipTTL = oldSet, oldGet, oldTTL })
	cloudClipSet = func(_ *App, s string) error { mu.Lock(); clip = s; mu.Unlock(); return nil }
	cloudClipGet = func(*App) (string, error) { mu.Lock(); defer mu.Unlock(); return clip, nil }
	cloudClipTTL = 20 * time.Millisecond
	read := func() string { mu.Lock(); defer mu.Unlock(); return clip }
	a, _ := testApp(t)
	if err := a.CopyCloudCode("K7QF-3XWM"); err != nil {
		t.Fatal(err)
	}
	if read() != "K7QF-3XWM" {
		t.Fatal("não copiou")
	}
	time.Sleep(80 * time.Millisecond)
	if read() != "" {
		t.Fatalf("o código ficou: %q", read())
	}
	_ = a.CopyCloudCode("K7QF-3XWM")
	_ = cloudClipSet(a, "outra coisa")
	time.Sleep(80 * time.Millisecond)
	if read() != "outra coisa" {
		t.Fatalf("apagou o que a pessoa copiou depois: %q", read())
	}
}
