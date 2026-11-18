package app

// QA, conferência do 7fd684f (PR #198, item 66; teste do QA, virou regressão): o σ dos últimos 10 min do
// cartão do Agora conta os segundos em que o GMC-800 está num objeto (Medir
// objeto ou "Estou testando um objeto"), com os alertas calados.
// Regra do cartão (agora.js:78-79): aparece com σ10 ≥ 3 e só some com σ10 < 2.

import (
	"testing"
)

func qaSigma10Run(t *testing.T, name string, start func(a *App), stop func(a *App), objCPM float64) {
	a := newTestApp(t)
	fakeGMC(t, a, 31) // fundo da casa: 31 CPM na última hora
	var last Reading
	a.emitHook = func(n string, v any) {
		if r, ok := v.(Reading); ok && n == "reading" {
			last = r
		}
	}
	z := func() float64 {
		if last.Sigma10 == nil {
			return -99
		}
		return *last.Sigma10
	}
	s := newGMCSim(a)
	s.feed(31, 600) // 10 min de fundo
	t.Logf("%s: fundo, 10 min: σ10 %.1f, fundo %.0f CPM", name, z(), last.BgCPM)
	start(a)
	up, upAt, alertSeen, maxZ := false, -1, false, 0.0
	for i := 1; i <= 300; i++ { // 5 min com o objeto
		s.feed(objCPM, 1)
		if z() > maxZ {
			maxZ = z()
		}
		if last.Attention || last.Alarm || last.Danger {
			alertSeen = true
		}
		up = (up && z() >= 2) || (!up && z() >= 3)
		if up && upAt < 0 {
			upAt = i
		}
	}
	t.Logf("%s: objeto a %.0f CPM por 5 min: cartão \"Fundo acima do normal, sem alerta\" aparece aos %d s (σ10 até %.1f); algum alerta: %v", name, objCPM, upAt, maxZ, alertSeen)
	stop(a)
	off := -1
	for i := 1; i <= 900; i++ { // depois, só o fundo
		s.feed(31, 1)
		up = (up && z() >= 2) || (!up && z() >= 3)
		if !up && off < 0 {
			off = i
		}
	}
	t.Logf("%s: objeto tirado, fundo normal (31 CPM): o cartão ainda fica %d s (%.1f min)", name, off, float64(off)/60)
	if upAt > 0 && !alertSeen {
		t.Errorf("%s: o cartão diz \"Fundo acima do normal, sem alerta\" com o GMC-800 no objeto e fica %.1f min depois de tirar", name, float64(off)/60)
	}
}

func TestQA21013bSigma10Objeto(t *testing.T) {
	// Medir objeto com o GMC-800, fundo da casa, 5 min no objeto
	qaSigma10Run(t, "Medir objeto", func(a *App) {
		st, err := a.StartMeasure(MeasureOpts{Name: "pedra", HomeBg: true, Seconds: 300})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("Medir objeto: fase %q", st.Phase)
	}, func(a *App) {
		t.Logf("Medir objeto: fase no fim %q", a.GetMeasure().Phase)
	}, 62)
	// "Estou testando um objeto" no GMC-800
	qaSigma10Run(t, "Testando um objeto", func(a *App) {
		if _, err := a.TestObject("gmc"); err != nil {
			t.Fatal(err)
		}
	}, func(a *App) { a.StopTestObject("gmc") }, 62)
}
