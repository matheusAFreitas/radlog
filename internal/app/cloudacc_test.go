package app

import (
	"context"
	"errors"
	"testing"
	"time"

	rs "radlog/internal/sync"
)

// N12: no "entrar de novo", outra conta Google é recusada e o acesso de
// antes fica.
func TestCloudReloginOtherAccount(t *testing.T) {
	srv := fakeCloud(t)
	a, _ := testApp(t)
	cloudPair(t, a, nil)
	before := a.cloud.sec.Refresh
	srv.Email = "outra@example.com"
	if _, err := a.CloudLogin(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CloudLoginWait(); !errors.Is(err, errCloudOtherAccount) {
		t.Fatalf("aceitou outra conta: %v", err)
	}
	if a.cloud.sec.Refresh != before || a.cloud.f.Email != "voce@example.com" {
		t.Fatal("trocou o acesso pelo da outra conta")
	}
	srv.Email = "VOCE@example.com" // a mesma conta, com outra caixa
	if _, err := a.CloudLogin(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CloudLoginWait(); err != nil {
		t.Fatalf("recusou a mesma conta: %v", err)
	}
}

// N13: desconectar o único aparelho tira o RadLog da conta Google; com outro
// aparelho na nuvem, não (revogar derrubaria o outro).
func TestCloudDisconnectRevokes(t *testing.T) {
	srv := fakeCloud(t)
	a, _ := testApp(t)
	b, _ := testApp(t)
	cloudPair(t, a, b)
	b.cloudSync()
	a.cloudSync()
	if len(a.cloud.f.Devs) == 0 {
		t.Fatal("a não viu b")
	}
	a.CloudDisconnect()
	b.CloudDisconnect() // b também vê a
	time.Sleep(100 * time.Millisecond)
	srv.Lock()
	n := len(srv.Revokes)
	srv.Unlock()
	if n != 0 {
		t.Fatalf("revogou com outro aparelho na nuvem: %d", n)
	}
}

func TestCloudDisconnectRevokesAlone(t *testing.T) {
	srv := fakeCloud(t)
	n := 0
	c, _ := testApp(t)
	cloudPair(t, c, nil)
	c.CloudDisconnect()
	deadline := time.Now().Add(2 * time.Second)
	for {
		srv.Lock()
		n = len(srv.Revokes)
		srv.Unlock()
		if n == 1 || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if n != 1 {
		t.Fatalf("não revogou: %d", n)
	}
}

// N17: a pasta renomeada no Drive continua sendo achada (pela marca), e a
// pasta de antes da marca ganha a marca na sincronização.
func TestCloudFolderRenamed(t *testing.T) {
	srv := fakeCloud(t)
	a, _ := testApp(t)
	cloudPair(t, a, nil)
	// a pasta como a 2.80 criava: sem marca
	srv.Lock()
	for _, f := range srv.Files {
		if f.Mime != "" {
			f.Props = nil
		}
	}
	srv.Unlock()
	a.cloud.f.Tagged = false
	a.cloudSync()
	if !a.cloud.f.Tagged {
		t.Fatal("não marcou a pasta")
	}
	srv.Lock()
	for _, f := range srv.Files {
		if f.Mime != "" {
			f.Name = "RadLog antigo"
		}
	}
	srv.Unlock()
	b, _ := testApp(t)
	if done := cloudLogin(t, b); done.Next != "existing" {
		t.Fatalf("não achou a pasta renomeada: %+v", done)
	}
}

// N17: dois aparelhos criando a nuvem ao mesmo tempo: o segundo desiste e
// apaga a pasta dele.
func TestCloudCreateRace(t *testing.T) {
	srv := fakeCloud(t)
	a, _ := testApp(t)
	b, _ := testApp(t)
	cloudLogin(t, a)
	cloudLogin(t, b) // os dois viram "conta sem nuvem"
	ca, _ := a.NewCloudCode()
	if _, err := a.CloudFinishSetup(ca); err != nil {
		t.Fatal(err)
	}
	cb, _ := b.NewCloudCode()
	if _, err := b.CloudFinishSetup(cb); !errors.Is(err, errCloudRace) {
		t.Fatalf("b criou outra nuvem: %v", err)
	}
	d := cloudDriveFn(func(context.Context) (string, error) { return "x", nil })
	fs, err := d.FindFolders(context.Background(), cloudFolderName)
	if err != nil || len(fs) != 1 {
		t.Fatalf("pastas: %d %v", len(fs), err)
	}
	if len(srv.Named(rs.MetaFile)) != 1 {
		t.Fatal("ficaram duas nuvens")
	}
}

// N14: um arquivo grande demais na nuvem é pulado sem travar o resto.
func TestCloudTooBigSkipped(t *testing.T) {
	fakeCloud(t)
	a, _ := testApp(t)
	b, _ := testApp(t)
	if _, err := a.store.Import(series(time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC), 20)); err != nil {
		t.Fatal(err)
	}
	cloudPair(t, a, b)
	old := rs.MaxDownload
	rs.MaxDownload = 400 // passa a meta e o aparelho, barra os pacotes
	t.Cleanup(func() { rs.MaxDownload = old })
	b.cloudSync()
	if st := b.GetCloud(); st.State != "ok" || st.Err != "" {
		t.Fatalf("travou: %+v", st)
	}
	rs.MaxDownload = old
	b.cloudSync() // marcado como visto: não baixa de novo
	if pts, _ := b.store.Range(time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC), time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)); len(pts) != 0 {
		t.Fatalf("baixou o que devia pular: %d", len(pts))
	}
}
