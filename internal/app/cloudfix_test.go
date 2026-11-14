package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	rs "radlog/internal/sync"
)

// testes dos achados da auditoria 2.79.0 (passo 1: N1, N2, N3, N8, N9)

// cloudPair liga a e b na mesma nuvem e devolve o código.
func cloudPair(t *testing.T, a, b *App) string {
	t.Helper()
	cloudLogin(t, a)
	code, _ := a.NewCloudCode()
	if _, err := a.CloudFinishSetup(code); err != nil {
		t.Fatal(err)
	}
	a.cloudSync()
	if b != nil {
		cloudLogin(t, b)
		if _, err := b.CloudOpen(code); err != nil {
			t.Fatal(err)
		}
	}
	return code
}

// N1: o anexo de uma medição chega no outro aparelho.
func TestCloudAttachmentArrives(t *testing.T) {
	fakeCloud(t)
	a, b := newTestApp(t), newTestApp(t)
	t0 := time.Now().Add(-time.Hour)
	addTestMeasure(a, 1, "Eletrodo", "encostado", t0)
	addTestMeasure(b, 7, "Outra", "", t0.Add(-24*time.Hour)) // números diferentes nos dois
	src := filepath.Join(t.TempDir(), "foto.jpg")
	_ = os.WriteFile(src, testPhoto("jpg"), 0o644)
	if _, err := a.attachFiles(1, []string{src}); err != nil {
		t.Fatal(err)
	}
	cloudPair(t, a, b)
	b.cloudSync()
	var got *SavedMeasure
	for i := range b.measurements {
		if b.measurements[i].Name == "Eletrodo" {
			got = &b.measurements[i]
		}
	}
	if got == nil || len(got.Attachments) != 1 {
		t.Fatalf("medição: %+v", got)
	}
	if got.ID == 1 {
		t.Fatal("o teste precisa de números diferentes nos dois aparelhos")
	}
	if _, err := os.Stat(filepath.Join(b.attachDir(got.ID), "foto.jpg")); err != nil {
		t.Fatalf("o anexo não chegou: %v", err)
	}
	if st := b.GetCloud(); st.State != "ok" {
		t.Fatalf("estado: %+v", st)
	}
}

// N1: quem já tinha visto os pacotes antes da correção baixa de novo.
func TestCloudRevResetsSeen(t *testing.T) {
	a, dir := testApp(t)
	f := cloudFile{On: true, Seen: map[string]string{rs.MetaFile: "m", "x": "y"}}
	b, _ := json.Marshal(f)
	_ = os.WriteFile(filepath.Join(dir, "nuvem.json"), b, 0o600)
	a.GetCloud()
	if len(a.cloud.f.Seen) != 1 || a.cloud.f.Seen[rs.MetaFile] != "m" || a.cloud.f.Rev != cloudRev {
		t.Fatalf("vistos: %+v rev %d", a.cloud.f.Seen, a.cloud.f.Rev)
	}
}

// N2: um pacote que não junta não trava o resto.
func TestCloudBadPackDoesNotBlock(t *testing.T) {
	srv := fakeCloud(t)
	a, _ := testApp(t)
	b, _ := testApp(t)
	day := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	if _, err := b.store.Import(series(day, 30, 31)); err != nil {
		t.Fatal(err)
	}
	cloudPair(t, a, b)
	b.cloudSync()
	// um pacote estragado de "outro aparelho" (abre, mas não é zip)
	name := b.cloud.keys.Name("estragado", "dia|2026-09-01")
	sealed, _ := b.cloud.keys.Seal(name, rs.Wrap(rs.KindDay, []byte("isto não é zip")))
	d := b.cloudDriveWith(b.cloudToken)
	if _, err := d.Put(t.Context(), b.cloud.f.Folder, "", name, sealed); err != nil {
		t.Fatal(err)
	}
	if _, err := a.store.Import(series(day.AddDate(0, 0, 1), 20)); err != nil {
		t.Fatal(err)
	}
	calls := len(srv.Calls)
	a.cloudSync()
	st := a.GetCloud()
	if st.State != "err" || !strings.Contains(st.Err, "não entraram") || st.LastSync == 0 {
		t.Fatalf("estado com pacote ruim: %+v", st)
	}
	pts, _ := a.store.Range(day, day.Add(time.Hour))
	if len(pts) != 2 {
		t.Fatalf("o pacote bom de B não entrou: %d", len(pts))
	}
	put := false
	for _, c := range srv.Calls[calls:] {
		put = put || strings.HasPrefix(c, "PUT")
	}
	if !put {
		t.Fatal("com o pacote ruim, A não subiu os dados dele")
	}
}

// N3: chave ilegível pede login e código, e continua o mesmo aparelho.
func TestCloudUnreadableKeyAsksCode(t *testing.T) {
	fakeCloud(t)
	a, dir := testApp(t)
	code := cloudPair(t, a, nil)
	dev := a.cloud.f.DevID
	_ = os.WriteFile(filepath.Join(dir, "nuvem-chave.bin"), []byte("ilegível"), 0o600)
	a.cloud = cloudState{} // como no próximo início do app
	if st := a.GetCloud(); st.State != "login" {
		t.Fatalf("chave ilegível: %s", st.State)
	}
	a.cloudSync() // não roda sem chave
	done := cloudLogin(t, a)
	if done.Next != "existing" {
		t.Fatalf("depois do login devia pedir o código: %+v", done)
	}
	if _, err := a.CloudOpen(code); err != nil {
		t.Fatal(err)
	}
	if a.cloud.f.DevID != dev {
		t.Fatal("virou outro aparelho")
	}
	a.cloudSync()
	if st := a.GetCloud(); st.State != "ok" || st.Err != "" {
		t.Fatalf("depois do código: %+v", st)
	}
	if got, err := a.CloudCode(); err != nil || got != code {
		t.Fatalf("ver código: %q %v", got, err)
	}
}

// N8: desconectar no fim de um envio não derruba nem trava a nuvem.
func TestCloudDisconnectDuringSend(t *testing.T) {
	srv := fakeCloud(t)
	a, dir := testApp(t)
	cloudPair(t, a, nil)
	if _, err := a.store.Import(series(time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC), 20)); err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	hooked := make(chan struct{})
	srv.OnPut = func() { once.Do(func() { a.CloudDisconnect(); close(hooked) }) }
	a.cloudSync()
	// o desconectar roda no servidor depois da resposta: espera ele terminar
	// antes do teste trocar as variáveis de volta
	select {
	case <-hooked:
	case <-time.After(2 * time.Second):
		t.Fatal("o envio não chegou ao servidor")
	}
	done := make(chan CloudInfo)
	go func() { done <- a.GetCloud() }()
	select {
	case st := <-done:
		if st.State != "off" {
			t.Fatalf("estado: %+v", st)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a nuvem travou")
	}
	if _, err := os.Stat(filepath.Join(dir, "nuvem.json")); err == nil {
		t.Fatal("a sincronização gravou o estado depois de desconectar")
	}
}

// N9: erro que se repete fica amarelo na hora, com texto próprio de cota.
func TestCloudQuotaError(t *testing.T) {
	srv := fakeCloud(t)
	a, _ := testApp(t)
	cloudPair(t, a, nil)
	if _, err := a.store.Import(series(time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC), 20)); err != nil {
		t.Fatal(err)
	}
	srv.Quota = true
	a.cloudSync()
	st := a.GetCloud()
	if st.State != "err" || !strings.Contains(st.Err, "cheio") {
		t.Fatalf("cota cheia: %+v", st)
	}
	srv.Quota = false
	a.cloudSync()
	if st := a.GetCloud(); st.State != "ok" || st.Err != "" {
		t.Fatalf("depois de liberar espaço: %+v", st)
	}
}
