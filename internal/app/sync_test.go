package app

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"radlog/internal/store"
)

func syncPair(t *testing.T) (srv, cli *App, addr string) {
	t.Helper()
	srv, _ = testApp(t)
	cli, _ = testApp(t)
	srv.syncStartServer("127.0.0.1:0")
	t.Cleanup(srv.syncStopServer)
	addr = fmt.Sprintf("127.0.0.1:%d", srv.sync.port)
	return
}

func TestSyncPairAndExchange(t *testing.T) {
	srv, cli, addr := syncPair(t)
	// sem código ativo: recusa
	if _, err := cli.PairWith(addr, "123456"); err == nil {
		t.Fatal("pareou sem código")
	}
	info, err := srv.ShowPairCode()
	if err != nil || len(info.Code) != 6 {
		t.Fatalf("código: %+v %v", info, err)
	}
	// código errado: recusa e conta a tentativa
	wrong := "000000"
	if info.Code == wrong {
		wrong = "111111"
	}
	if _, err := cli.PairWith(addr, wrong); err == nil || !strings.Contains(err.Error(), "código errado") {
		t.Fatalf("código errado: %v", err)
	}
	if srv.sync.codeTries != 1 {
		t.Fatalf("tentativas: %d", srv.sync.codeTries)
	}
	ci, err := cli.PairWith(addr, info.Code)
	if err != nil {
		t.Fatal(err)
	}
	if len(ci.Peers) != 1 || len(srv.GetSync().Peers) != 1 || srv.GetSync().Code != "" {
		t.Fatalf("pareamento: cli %+v srv %+v", ci.Peers, srv.GetSync())
	}
	// o código não vale de novo
	if _, err := cli.PairWith(addr, info.Code); err == nil {
		t.Fatal("código reaproveitado")
	}

	// dados diferentes nos dois
	base := time.Now().Add(-48 * time.Hour).Truncate(time.Minute)
	var sp, cp []store.Point
	for m := 0; m < 120; m++ {
		p := store.Point{T: base.Add(time.Duration(m) * time.Minute), CPM: 20, Src: store.SrcLive}
		if m < 80 {
			sp = append(sp, p) // o PC tem os minutos 0–79
		}
		if m >= 60 {
			cp = append(cp, p) // o notebook tem 60–119
		}
	}
	_, _ = srv.store.ImportNew(sp)
	_, _ = cli.store.ImportNew(cp)
	srv.mu.Lock()
	srv.events = append(srv.events, Event{ID: 1, Time: base.UnixMilli(), End: base.Add(time.Minute).UnixMilli(), Kind: kindAlert, Level: "atencao", Text: "Atenção: 55 CPM (a partir de 50)", Peak: 55})
	srv.mu.Unlock()
	srv.saveEvents()

	id := cli.GetSync().Peers[0].ID
	res, err := cli.SyncNow(id)
	if err != nil {
		t.Fatal(err)
	}
	if res.Got.Minutes != 60 || res.Sent.Minutes != 40 || res.Got.Events != 1 {
		t.Fatalf("resultado: %+v", res)
	}
	for _, x := range []*App{srv, cli} {
		pts, _ := x.store.Range(base, base.Add(3*time.Hour))
		if len(pts) != 120 {
			t.Fatalf("depois de sincronizar: %d minutos", len(pts))
		}
	}
	// de novo: nada a trocar
	res, err = cli.SyncNow(id)
	if err != nil || res.Got.total()+res.Sent.total() != 0 {
		t.Fatalf("segunda vez: %+v %v", res, err)
	}
}

func TestSyncRejectsStrangers(t *testing.T) {
	srv, cli, addr := syncPair(t)
	// um "par" que o servidor não conhece
	fake := syncPeer{ID: "x", Name: "x", Key: make([]byte, 32)}
	if _, err := cli.syncCall(addr, fake, "inventory", []byte("{}"), 2*time.Second); err == nil {
		t.Fatal("aceitou quem não é pareado")
	}
	_ = srv
}

func TestDigestDays(t *testing.T) {
	t0 := time.Date(2026, 11, 5, 23, 58, 0, 0, time.UTC)
	pts := []store.Point{{T: t0}, {T: t0.Add(time.Minute)}, {T: t0.Add(2 * time.Minute)}, {T: t0.Add(2 * time.Minute)}}
	d := digestDays(pts)
	if len(d) != 2 || !strings.HasPrefix(d["2026-11-05"], "2:") || !strings.HasPrefix(d["2026-11-06"], "1:") {
		t.Fatalf("%v", d)
	}
}

// S6: a descoberta só responde com o código de pareamento na tela
func TestSyncDiscoveryOnlyWithCode(t *testing.T) {
	a := newTestApp(t)
	srv, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Skip(err)
	}
	defer srv.Close()
	go a.syncAnswerDiscovery(srv, "id-teste", 47862)
	ask := func() bool {
		c, err := net.DialUDP("udp4", nil, srv.LocalAddr().(*net.UDPAddr))
		if err != nil {
			t.Fatal(err)
		}
		defer c.Close()
		_, _ = c.Write([]byte(syncQuery))
		_ = c.SetReadDeadline(time.Now().Add(400 * time.Millisecond))
		buf := make([]byte, 512)
		n, err := c.Read(buf)
		return err == nil && strings.Contains(string(buf[:n]), "id-teste")
	}
	if ask() {
		t.Fatal("respondeu sem código na tela")
	}
	a.sync.mu.Lock()
	a.sync.code, a.sync.codeUntil = "123456", time.Now().Add(time.Minute)
	a.sync.mu.Unlock()
	if !ask() {
		t.Fatal("não respondeu com o código na tela")
	}
}
