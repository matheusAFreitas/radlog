package sync_test

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	rs "radlog/internal/sync"
	"radlog/internal/sync/synctest"
)

func TestKeysSealOpen(t *testing.T) {
	code, _ := rs.NewRecoveryCode()
	m, k, err := rs.NewMeta(code)
	if err != nil {
		t.Fatal(err)
	}
	k2, err := m.Unlock(rs.FormatRecoveryCode(code))
	if err != nil {
		t.Fatalf("código certo não abriu: %v", err)
	}
	if k.FileName("a") != k2.FileName("a") || len(k.FileName("a")) != 32 || k.FileName("a") == k.FileName("b") {
		t.Fatal("nomes")
	}
	box, _ := k.Seal("n1", []byte("oi"))
	if b, err := k2.Open("n1", box); err != nil || string(b) != "oi" {
		t.Fatalf("abrir: %q %v", b, err)
	}
	if _, err := k2.Open("n2", box); err != rs.ErrOpen {
		t.Fatal("nome trocado abriu")
	}
	other, _ := rs.NewRecoveryCode()
	if _, err := m.Unlock(other); err != rs.ErrOpen {
		t.Fatalf("código errado: %v", err)
	}
	// outro sal, mesmo código: outras chaves
	m2, _, _ := rs.NewMeta(code)
	k3, _ := m2.Unlock(code)
	if k3.FileName("a") == k.FileName("a") {
		t.Fatal("sal não mudou os nomes")
	}
}

func TestOAuthLoopback(t *testing.T) {
	srv := synctest.New()
	defer srv.Close()
	o := rs.Google("cid", "sec")
	o.TokenURL = srv.TokenURL()
	o.AuthURL = "https://example.invalid/auth"
	l, err := o.Start()
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(l.URL)
	q := u.Query()
	if q.Get("code_challenge_method") != "S256" || q.Get("scope") != rs.Scopes || q.Get("access_type") != "offline" {
		t.Fatalf("url: %s", l.URL)
	}
	// o navegador volta com o state errado: ignorado
	resp, err := http.Get(q.Get("redirect_uri") + "?code=x&state=errado")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("state errado: %d", resp.StatusCode)
	}
	resp, err = http.Get(q.Get("redirect_uri") + "?code=abc&state=" + url.QueryEscape(q.Get("state")))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	code, err := l.Wait(ctx)
	if err != nil || code != "abc" {
		t.Fatalf("wait: %q %v", code, err)
	}
	tok, err := o.Exchange(ctx, l, code)
	if err != nil || tok.Refresh != "rt-1" || tok.Email != "voce@example.com" || tok.Access == "" {
		t.Fatalf("exchange: %+v %v", tok, err)
	}
	srv.Revoked = true
	if _, err := o.Refresh(ctx, "rt-1"); err != rs.ErrRevoked {
		t.Fatalf("revogado: %v", err)
	}
}

func TestOAuthCancel(t *testing.T) {
	o := rs.Google("cid", "")
	l, err := o.Start()
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(l.URL)
	q := u.Query()
	resp, err := http.Get(q.Get("redirect_uri") + "?error=access_denied&state=" + url.QueryEscape(q.Get("state")))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if _, err := l.Wait(context.Background()); err != rs.ErrCanceled {
		t.Fatalf("cancelado: %v", err)
	}
	if _, err := rs.Google("", "").Start(); err == nil {
		t.Fatal("sem client id começou")
	}
}

func TestDrive(t *testing.T) {
	srv := synctest.New()
	defer srv.Close()
	srv.Skew = 10 * time.Minute
	d := rs.NewDrive(func(context.Context) (string, error) { return "t", nil })
	d.Base, d.Upload = srv.Base(), srv.Upload()
	var seen time.Time
	d.OnDate = func(t time.Time) { seen = t }
	ctx := context.Background()
	id, err := d.FindFolder(ctx, "RadLog")
	if err != nil || id != "" {
		t.Fatalf("pasta vazia: %q %v", id, err)
	}
	if id, err = d.CreateFolder(ctx, "RadLog"); err != nil {
		t.Fatal(err)
	}
	if got, _ := d.FindFolder(ctx, "RadLog"); got != id {
		t.Fatalf("achou %q, criou %q", got, id)
	}
	f, err := d.Put(ctx, id, "", "a.bin", []byte("um"))
	if err != nil || f.ID == "" || f.Size != 2 || f.MD5 == "" {
		t.Fatalf("put: %+v %v", f, err)
	}
	f2, err := d.Put(ctx, id, f.ID, "", []byte("dois"))
	if err != nil || f2.ID != f.ID || f2.MD5 == f.MD5 {
		t.Fatalf("troca: %+v %v", f2, err)
	}
	ls, err := d.List(ctx, id)
	if err != nil || len(ls) != 1 || ls[0].Name != "a.bin" {
		t.Fatalf("list: %+v %v", ls, err)
	}
	if b, err := d.Get(ctx, f.ID); err != nil || !bytes.Equal(b, []byte("dois")) {
		t.Fatalf("get: %q %v", b, err)
	}
	if err := d.Delete(ctx, f.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Get(ctx, f.ID); err != rs.ErrNotFound {
		t.Fatalf("apagado: %v", err)
	}
	if time.Until(seen) < 9*time.Minute {
		t.Fatalf("hora do servidor: %v", seen)
	}
	srv.Offline = true
	if _, err := d.List(ctx, id); err == nil {
		t.Fatal("fora do ar sem erro")
	}
}
