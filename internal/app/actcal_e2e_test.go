package app

import (
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"radlog/internal/radiacode"
)

// TestActivityCalScreenE2E calibra pelo KCl pela tela, com o App de verdade
// por trás e um RC102 de mentira (2.101.3). Na 2.101.2 a massa digitada
// (250 g) voltava para 500 g quando se clicava em "Sal light" ou no cartão
// "1 · KCl" depois de digitar. Confere que % de KCl, massa e posição chegam
// do formulário até o resultado, a calibração e o que fica salvo no disco.
//
//	RADLOG_E2E=1 go test -tags webkit2_41 -run TestActivityCalScreenE2E ./internal/app/
func TestActivityCalScreenE2E(t *testing.T) {
	if os.Getenv("RADLOG_E2E") == "" {
		t.Skip("RADLOG_E2E=1 para rodar")
	}
	a, _ := testApp(t)
	rng := rand.New(rand.NewSource(7))
	dev := &actDev{}
	var mu sync.Mutex
	peaks := map[string]map[float64]float64{
		"bg":  {1460.8: 2, 609.3: 1.5},
		"kcl": {1460.8: 2 + 1.5, 609.3: 1.5},
	}
	feed := func(p string, sec float64) {
		pk := map[float64]float64{}
		for k, v := range peaks[p] {
			pk[k] = v * sec
		}
		mu.Lock()
		dev.add(actFake(rng, sec, pk, 0.5))
		mu.Unlock()
	}
	feed("bg", 1)
	a.rc.mu.Lock()
	a.rc.specTest = func(bool) *radiacode.Spectrum {
		mu.Lock()
		defer mu.Unlock()
		return &radiacode.Spectrum{Duration: time.Duration(dev.sec * float64(time.Second)), A0: 0, A1: 3, A2: 0, Counts: append([]uint32(nil), dev.c...)}
	}
	a.rc.mu.Unlock()
	url := e2eServe(t, a, func(w http.ResponseWriter, r *http.Request) bool {
		switch r.URL.Path {
		case "/__fundo":
			feed("bg", 7200)
		case "/__sal":
			feed("kcl", 7200)
		default:
			return false
		}
		_, _ = w.Write([]byte("ok"))
		return true
	})
	cmd := exec.Command("node", "../../scripts/calibracao_e2e.js", url)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	t.Logf("%s", out)
	if err != nil {
		t.Fatalf("roteiro do navegador: %v", err)
	}
	check := func(where string) {
		v := a.GetActivity()
		k := v.Cal.KCl
		if k == nil || k.MassG != 250 || k.Pct != 51 || v.Cal.Geometry != "Becker Marinelli" {
			t.Fatalf("%s: calibração %+v, posição %q", where, k, v.Cal.Geometry)
		}
		if len(v.History) != 1 || v.History[0].MassG != 250 || v.History[0].Pct != 51 || v.History[0].Geometry != "Becker Marinelli" || v.History[0].Name != "KCl (51%)" {
			t.Fatalf("%s: histórico %+v", where, v.History)
		}
		aK := 250 * 0.51 * actKPerKCl * actBqPerGK
		if math.Abs(v.History[0].Expected/aK-1) > 1e-9 {
			t.Fatalf("%s: Bq esperados %v, queria %v", where, v.History[0].Expected, aK)
		}
		// 1,5 contagem/s a mais em 1.460 keV com 250 g a 51%
		if want := 1.5 / (aK * 0.1066); math.Abs(k.Eff/want-1) > 3*k.EffErr+0.02 {
			t.Fatalf("%s: eficiência %v, queria %v", where, k.Eff, want)
		}
	}
	check("na memória")
	// o que ficou no disco: lê de novo
	a.rc.act.mu.Lock()
	a.rc.act.loaded, a.rc.act.run = false, nil
	a.rc.act.mu.Unlock()
	check("no disco")
}
