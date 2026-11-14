package app

import (
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"
)

// TestDemoScreenE2E liga a demonstração pela tela com o App de verdade por
// trás, passa pelos cenários e pelas páginas, tenta exportar e sai, e confere
// que nenhum arquivo do app mudou enquanto ela estava ligada (2.82).
//
//	RADLOG_E2E=1 go test -tags webkit2_41 -run TestDemoScreenE2E ./internal/app/
func TestDemoScreenE2E(t *testing.T) {
	if os.Getenv("RADLOG_E2E") == "" {
		t.Skip("RADLOG_E2E=1 para rodar")
	}
	a, dir := testApp(t)
	if _, err := a.store.Import(series(time.Now().Add(-3*time.Hour).Truncate(time.Minute), 20, 21, 22)); err != nil {
		t.Fatal(err)
	}
	var before string
	url := e2eServe(t, a, func(w http.ResponseWriter, r *http.Request) bool {
		switch r.URL.Path {
		case "/__snap": // a demonstração acabou de ligar
			a.flushWrites()
			before = snapshotDir(t, dir)
		case "/__check": // antes de sair
			a.flushWrites()
			if after := snapshotDir(t, dir); after != before {
				_, _ = w.Write([]byte("mudou"))
				return true
			}
		default:
			return false
		}
		_, _ = w.Write([]byte("ok"))
		return true
	})
	cmd := exec.Command("node", "../../scripts/demo_e2e.js", url)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	t.Logf("%s", out)
	if err != nil {
		t.Fatalf("roteiro do navegador: %v", err)
	}
	if a.GetDemo().Active {
		t.Fatal("a demonstração ficou ligada")
	}
}
