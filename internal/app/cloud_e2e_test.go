package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	rs "radlog/internal/sync"
)

// Teste de ponta a ponta da tela da nuvem (auditoria 2.79.0, T2): a página
// de verdade (frontend/dist) num Chromium, falando com este App de verdade
// por uma ponte HTTP no lugar da Wails, e o App falando com o Drive de
// mentira. O roteiro do navegador está em scripts/nuvem_e2e.js.
//
// Só roda com RADLOG_E2E=1 (precisa do node e do playwright, como o
// scripts/telas_test.js):
//
//	RADLOG_E2E=1 go test -tags webkit2_41 -run TestCloudScreenE2E ./internal/app/
func TestCloudScreenE2E(t *testing.T) {
	if os.Getenv("RADLOG_E2E") == "" {
		t.Skip("RADLOG_E2E=1 para rodar")
	}
	srv := fakeCloud(t)
	a, _ := testApp(t)
	if _, err := a.store.Import(series(time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC), 20, 21)); err != nil {
		t.Fatal(err)
	}
	// o laço de 15 min não roda no teste: "Sincronizar agora" sincroniza
	a.cloudLoadLocked()
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			case <-a.cloud.kick:
				a.cloudSync()
			}
		}
	}()
	t.Cleanup(func() { close(stop); wg.Wait() })

	dist, _ := filepath.Abs("../../frontend/dist")
	mux := http.NewServeMux()
	var mu sync.Mutex
	mux.HandleFunc("/__go/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/__go/")
		var args []json.RawMessage
		_ = json.NewDecoder(r.Body).Decode(&args)
		mu.Lock()
		res, err := e2eCall(a, name, args)
		mu.Unlock()
		out := map[string]any{"r": res}
		if err != nil {
			out["e"] = err.Error()
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			b, err := os.ReadFile(filepath.Join(dist, "index.html"))
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			b = bytes.Replace(b, []byte(`<script src="i18n-en.js"></script>`), []byte(`<script src="__ponte.js"></script><script src="i18n-en.js"></script>`), 1)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(b)
			return
		}
		if r.URL.Path == "/__ponte.js" { // a CSP da página não deixa script embutido
			w.Header().Set("Content-Type", "text/javascript")
			_, _ = w.Write([]byte(e2eShim))
			return
		}
		http.ServeFile(w, r, filepath.Join(dist, filepath.FromSlash(r.URL.Path)))
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	hs := &http.Server{Handler: mux}
	go func() { _ = hs.Serve(ln) }()
	t.Cleanup(func() { _ = hs.Close() })

	cmd := exec.Command("node", "../../scripts/nuvem_e2e.js", "http://"+ln.Addr().String()+"/")
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	t.Logf("%s", out)
	if err != nil {
		t.Fatalf("roteiro do navegador: %v", err)
	}
	// o que a tela fez chegou mesmo no Drive e no estado do app
	if n := len(srv.Named(rs.MetaFile)); n != 1 {
		t.Fatalf("metas no Drive: %d", n)
	}
	if st := a.GetCloud(); st.State != "off" {
		t.Fatalf("depois de desconectar: %+v", st)
	}
	if _, err := os.Stat(a.cloudKeyPath()); err == nil {
		t.Fatal("a chave ficou no disco")
	}
}

// ponte no lugar da Wails: window.go.app.App.X(...) vira POST /__go/X
const e2eShim = `window.go = { app: { App: new Proxy({}, { get: (_, name) => typeof name !== "string" || name === "then" ? undefined : (...args) =>
  fetch("/__go/" + name, { method: "POST", body: JSON.stringify(args) }).then((r) => r.json()).then((x) => { if (x.e) throw new Error(x.e); return x.r; }) }) } };
window.runtime = new Proxy({}, { get: () => () => {} });
`

// e2eCall chama o método exportado do App com os argumentos em JSON.
func e2eCall(a *App, name string, args []json.RawMessage) (any, error) {
	m := reflect.ValueOf(a).MethodByName(name)
	if !m.IsValid() {
		return nil, fmt.Errorf("sem o método %s", name)
	}
	mt := m.Type()
	in := make([]reflect.Value, mt.NumIn())
	for i := range in {
		p := reflect.New(mt.In(i))
		if i < len(args) {
			if err := json.Unmarshal(args[i], p.Interface()); err != nil {
				return nil, err
			}
		}
		in[i] = p.Elem()
	}
	var res any
	var err error
	for _, v := range m.Call(in) {
		if e, ok := v.Interface().(error); ok || (v.Kind() == reflect.Interface && v.Type().Name() == "error") {
			if ok {
				err = e
			}
			continue
		}
		res = v.Interface()
	}
	return res, err
}
