package app

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	rs "radlog/internal/sync"
)

// testes do passo 2 da auditoria 2.79.0 (formato 2: A2, N4, N5, N6, N7, N11)

func cloudPuts(calls []string) int {
	n := 0
	for _, c := range calls {
		if strings.HasPrefix(c, "PUT") {
			n++
		}
	}
	return n
}

// A2: arquivo de uma versão mais nova fica para depois, sem erro.
func TestCloudNewerFileWaits(t *testing.T) {
	fakeCloud(t)
	a, _ := testApp(t)
	cloudPair(t, a, nil)
	k := a.cloud.keys
	name := k.Name("futuro", "dia|2026-09-01")
	env := rs.Wrap(rs.KindDay, []byte("conteúdo do formato 3"))
	env[2] = rs.FormatVersion + 1
	sealed, _ := k.Seal(name, env)
	d := a.cloudDriveWith(a.cloudToken)
	if _, err := d.Put(t.Context(), a.cloud.f.Folder, "", name, sealed); err != nil {
		t.Fatal(err)
	}
	a.cloudSync()
	st := a.GetCloud()
	if st.State != "ok" {
		t.Fatalf("estado: %+v", st)
	}
	if v := a.cloud.f.Seen[name]; !strings.HasPrefix(v, "v2:") {
		t.Fatalf("visto: %q", v)
	}
}

// N5: desconectar e entrar de novo no mesmo PC continua o mesmo aparelho, sem
// arquivos repetidos.
func TestCloudSameDeviceAfterReconnect(t *testing.T) {
	srv := fakeCloud(t)
	a, _ := testApp(t)
	if _, err := a.store.Import(series(time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC), 20, 21)); err != nil {
		t.Fatal(err)
	}
	code := cloudPair(t, a, nil)
	dev, files := a.cloud.f.DevID, srv.Count()
	a.CloudDisconnect()
	if done := cloudLogin(t, a); done.Next != "existing" {
		t.Fatalf("login: %+v", done)
	}
	if _, err := a.CloudOpen(code); err != nil {
		t.Fatal(err)
	}
	if a.cloud.f.DevID != dev {
		t.Fatal("virou outro aparelho")
	}
	a.cloudSync()
	if st := a.GetCloud(); st.State != "ok" || len(st.Devices) != 1 {
		t.Fatalf("estado: %+v", st)
	}
	if srv.Count() != files {
		t.Fatalf("arquivos: %d, antes %d", srv.Count(), files)
	}
}

// N6: a pasta copiada para outro PC pergunta; "aparelho novo" ganha ID novo.
func TestCloudCopiedPC(t *testing.T) {
	fakeCloud(t)
	a, dirA := testApp(t)
	cloudPair(t, a, nil)
	a.cloudSync()
	c, dirC := testApp(t)
	for _, n := range []string{"nuvem.json", "nuvem-chave.bin", "nuvem-aparelho.json"} {
		b, err := os.ReadFile(filepath.Join(dirA, n))
		if err != nil {
			t.Fatal(err)
		}
		_ = os.WriteFile(filepath.Join(dirC, n), b, 0o600)
	}
	c.GetCloud()
	c.cloud.sec.Writer = "outro-pc" // como se o nome do PC fosse outro
	// a continua subindo
	if _, err := a.store.Import(series(time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC), 20)); err != nil {
		t.Fatal(err)
	}
	a.cloudSync()
	c.cloudSync()
	st := c.GetCloud()
	if st.State != "dup" || st.DupOf == "" {
		t.Fatalf("cópia: %+v", st)
	}
	if _, err := c.CloudDupAnswer(true); err != nil {
		t.Fatal(err)
	}
	c.cloudSync()
	if st := c.GetCloud(); st.State != "ok" || c.cloud.f.DevID == a.cloud.f.DevID || len(st.Devices) != 2 {
		t.Fatalf("aparelho novo: %+v", st)
	}
	a.cloudSync()
	if st := a.GetCloud(); st.State != "ok" || len(st.Devices) != 2 {
		t.Fatalf("a vê os dois: %+v", st)
	}
}

// remover aparelho: os arquivos dele vão para a lixeira.
func TestCloudRemoveDevice(t *testing.T) {
	srv := fakeCloud(t)
	a, _ := testApp(t)
	b, _ := testApp(t)
	if _, err := b.store.Import(series(time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC), 30)); err != nil {
		t.Fatal(err)
	}
	cloudPair(t, a, b)
	b.cloudSync()
	a.cloudSync()
	st := a.GetCloud()
	var id string
	for _, d := range st.Devices {
		if !d.Me {
			id = d.ID
		}
	}
	if id == "" || id != b.cloud.f.DevID {
		t.Fatalf("aparelhos: %+v", st.Devices)
	}
	if _, err := a.CloudRemoveDevice(a.cloud.f.DevID); err == nil {
		t.Fatal("removeu a si mesmo")
	}
	if _, err := a.CloudRemoveDevice(id); err != nil {
		t.Fatal(err)
	}
	if len(srv.Trashed) == 0 {
		t.Fatal("nada na lixeira")
	}
	for _, f := range srv.Files {
		if strings.HasPrefix(f.Name, a.cloud.keys.Prefix(id)) {
			t.Fatalf("sobrou arquivo do removido: %s", f.Name)
		}
	}
	a.cloudSync()
	if st := a.GetCloud(); len(st.Devices) != 1 {
		t.Fatalf("depois de remover: %+v", st.Devices)
	}
	// b ainda ligado: fica "removido" até entrar de novo com o código
	b.cloudSync()
	if st := b.GetCloud(); st.State != "removed" || st.Removed == 0 {
		t.Fatalf("b removido: %+v", st)
	}
	if _, err := b.CloudRejoin("0000-0000-0000-0000-0000-0000"); err == nil {
		t.Fatal("entrou com código errado")
	}
	code := b.cloud.sec.Code
	if _, err := b.CloudRejoin(code); err != nil {
		t.Fatal(err)
	}
	b.cloudSync()
	if st := b.GetCloud(); st.State != "ok" {
		t.Fatalf("b de volta: %+v", st)
	}
	a.cloudSync()
	if st := a.GetCloud(); len(st.Devices) != 2 {
		t.Fatalf("b voltou na lista: %+v", st.Devices)
	}
	if _, err := a.CloudRemoveDevice(id); err != nil {
		t.Fatal(err)
	}
	// o que veio de b continua na nuvem pela faixa de a
	c, _ := testApp(t)
	cloudLogin(t, c)
	if _, err := c.CloudOpen(b.cloud.sec.Code); err != nil {
		t.Fatal(err)
	}
	c.cloudSync()
	if pts, _ := c.store.Range(time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC), time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)); len(pts) != 1 {
		t.Fatalf("dados do removido: %d", len(pts))
	}
}

// N11: nuvem do formato antigo pausa e recomeça com o mesmo código; a pasta
// antiga vai para a lixeira.
func TestCloudOldFormatRestart(t *testing.T) {
	srv := fakeCloud(t)
	a, _ := testApp(t)
	if _, err := a.store.Import(series(time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC), 20)); err != nil {
		t.Fatal(err)
	}
	code := cloudPair(t, a, nil)
	old := a.cloud.f.Folder
	// a meta volta para a versão 1 (como a nuvem da 2.79.x)
	for _, f := range srv.Named(rs.MetaFile) {
		var m rs.Meta
		_ = json.Unmarshal(f.Data, &m)
		m.V = 1
		f.Data, _ = json.Marshal(m)
	}
	// como o nuvem.json da 2.79.x: a meta antiga já vista, sem MetaV
	for _, f := range srv.Named(rs.MetaFile) {
		sum := md5.Sum(f.Data)
		a.cloud.f.Seen[rs.MetaFile] = hex.EncodeToString(sum[:])
	}
	a.cloud.f.MetaV = 0
	a.cloudSync()
	if st := a.GetCloud(); st.State != "old" {
		t.Fatalf("antiga: %+v", st)
	}
	if _, err := a.CloudRestart(); err != nil {
		t.Fatal(err)
	}
	if srv.Trashed[old] == nil || a.cloud.f.Folder == old {
		t.Fatal("a pasta antiga não foi para a lixeira")
	}
	if st := a.GetCloud(); !st.Restart {
		t.Fatalf("recomeço: %+v", st)
	}
	a.cloudSync()
	if st := a.GetCloud(); st.State != "ok" || st.Restart || st.Restarted == 0 {
		t.Fatalf("depois: %+v", st)
	}
	if got, _ := a.CloudCode(); got != code {
		t.Fatal("o código mudou")
	}
	// outro aparelho entra na nuvem nova com o mesmo código
	b, _ := testApp(t)
	cloudLogin(t, b)
	if _, err := b.CloudOpen(code); err != nil {
		t.Fatal(err)
	}
	b.cloudSync()
	if pts, _ := b.store.Range(time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC), time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)); len(pts) != 1 {
		t.Fatalf("dados na nuvem nova: %d", len(pts))
	}
}

// login numa conta com nuvem antiga avisa (só dá para começar uma nova).
func TestCloudLoginOld(t *testing.T) {
	srv := fakeCloud(t)
	a, _ := testApp(t)
	cloudPair(t, a, nil)
	for _, f := range srv.Named(rs.MetaFile) {
		var m rs.Meta
		_ = json.Unmarshal(f.Data, &m)
		m.V = 1
		f.Data, _ = json.Marshal(m)
	}
	b, _ := testApp(t)
	if done := cloudLogin(t, b); done.Next != "old" {
		t.Fatalf("login: %+v", done)
	}
	if _, err := b.CloudReset(); err != nil {
		t.Fatal(err)
	}
	if len(srv.Trashed) == 0 {
		t.Fatal("a antiga não foi para a lixeira")
	}
}

// N7: só o dia que mudou sobe; o que veio de outro aparelho não volta (eco);
// anexo tirado da medição sai da nuvem.
func TestCloudDaysEchoAttach(t *testing.T) {
	srv := fakeCloud(t)
	a, _ := testApp(t)
	b, _ := testApp(t)
	d0 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	if _, err := a.store.Import(append(append(series(d0, 20), series(d0.AddDate(0, 0, 1), 21)...), series(d0.AddDate(0, 0, 2), 22)...)); err != nil {
		t.Fatal(err)
	}
	cloudPair(t, a, b)
	b.cloudSync()
	calls := len(srv.Calls)
	b.cloudSync()
	if n := cloudPuts(srv.Calls[calls:]); n != 0 {
		t.Fatalf("b subiu de novo o que veio de a: %d", n)
	}
	// a muda um dia: só ele sobe
	if _, err := a.store.Import(series(d0.AddDate(0, 0, 1).Add(time.Hour), 25)); err != nil {
		t.Fatal(err)
	}
	calls = len(srv.Calls)
	a.cloudSync()
	if n := cloudPuts(srv.Calls[calls:]); n != 2 { // o dia e o arquivo do aparelho
		t.Fatalf("envios: %d", n)
	}
	// anexo: sobe uma vez; tirado, sai da nuvem
	addTestMeasure(a, 1, "Eletrodo", "", d0)
	src := filepath.Join(t.TempDir(), "foto.jpg")
	_ = os.WriteFile(src, testPhoto("jpg"), 0o644)
	if _, err := a.attachFiles(1, []string{src}); err != nil {
		t.Fatal(err)
	}
	before := srv.Count()
	a.cloudSync()
	if srv.Count() != before+1 {
		t.Fatalf("anexo: %d → %d", before, srv.Count())
	}
	calls = len(srv.Calls)
	a.cloudSync()
	if n := cloudPuts(srv.Calls[calls:]); n != 0 {
		t.Fatalf("subiu de novo sem mudança: %d", n)
	}
	b.cloudSync()
	calls = len(srv.Calls)
	b.cloudSync()
	if n := cloudPuts(srv.Calls[calls:]); n != 0 {
		t.Fatalf("b subiu o anexo de a: %d", n)
	}
	if err := a.RemoveMeasurementAttachment(1, "foto.jpg"); err != nil {
		t.Fatal(err)
	}
	a.cloudSync()
	if srv.Count() != before {
		t.Fatalf("o anexo tirado ficou na nuvem: %d", srv.Count())
	}
}
