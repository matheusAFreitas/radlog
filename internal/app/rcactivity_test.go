package app

import (
	"bytes"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"radlog/internal/radiacode"
)

// aparelho de mentira: espectro acumulado que vai crescendo
type actDev struct {
	c   []uint32
	sec float64
}

func (d *actDev) add(s actSpec) radiacode.Spectrum {
	if d.c == nil {
		d.c = make([]uint32, len(s.C))
	}
	for i, v := range s.C {
		d.c[i] += uint32(v)
	}
	d.sec += s.Sec
	return radiacode.Spectrum{Duration: time.Duration(d.sec * float64(time.Second)), A0: s.A[0], A1: s.A[1], A2: s.A[2], Counts: append([]uint32(nil), d.c...)}
}

func TestActivityFlow(t *testing.T) {
	a := newTestApp(t)
	rng := rand.New(rand.NewSource(3))
	dev := &actDev{}
	bgPeaks := map[float64]float64{1460.8: 2, 609.3: 1.5}              // contagens por segundo nos picos do fundo
	kclPeaks := map[float64]float64{1460.8: 2 + 1.5, 609.3: 1.5}       // + 1,5/s do KCl
	nutPeaks := map[float64]float64{1460.8: 2 + 0.3, 609.3: 1.5 + 0.4} // castanha: K-40 e rádio
	scale := func(p map[float64]float64, sec float64) map[float64]float64 {
		out := map[float64]float64{}
		for k, v := range p {
			out[k] = v * sec
		}
		return out
	}
	now := time.Now()
	st := &a.rc.act
	start := func(o ActOpts, phase string) {
		if err := actValidate(&o); err != nil {
			t.Fatal(err)
		}
		r := &actRun{Opts: o, Phase: phase, PhaseAt: now.UnixMilli()}
		r.Cur.feed(dev.add(actSpec{C: make([]float64, 1024), Sec: 1, A: [3]float64{0, 3, 0}}))
		if phase == "amostra" {
			r.Bg = a.actSavedBg()
		}
		st.mu.Lock()
		st.loaded, st.run = true, r
		st.mu.Unlock()
	}
	step := func(peaks map[float64]float64, sec float64) radiacode.Spectrum {
		s := dev.add(actFake(rng, sec, scale(peaks, sec), 0.5))
		st.mu.Lock()
		st.run.Cur.feed(s)
		st.mu.Unlock()
		return s
	}
	next := func(s radiacode.Spectrum) {
		st.mu.Lock()
		a.actAdvanceLocked(&s, now)
		st.mu.Unlock()
	}

	// 1) calibração com 500 g de KCl puro: fundo de 2 h, KCl de 1 h
	start(ActOpts{Kind: "kcl", MassG: 500, Pct: 100, Geometry: "pote de 500 mL", NewBg: true}, "fundo")
	step(bgPeaks, 3600)
	// o aparelho zerou o espectro no meio do fundo
	dev.c, dev.sec = nil, 0
	next(step(bgPeaks, 3600))
	if v := a.GetActivity(); v.Phase != "trocar" || v.SavedBg == nil || math.Abs(v.SavedBg.Sec-7201) > 2 {
		t.Fatalf("depois do fundo: %+v", v.SavedBg)
	}
	next(step(nil, 1)) // começa a amostra
	step(kclPeaks, 1800)
	if v := a.GetActivity(); v.Phase != "amostra" || v.LiveKCl == nil || v.Expected <= 0 || v.Chart == nil {
		t.Fatalf("parcial: %+v", v)
	}
	next(step(kclPeaks, 1800))
	v := a.GetActivity()
	if v.Phase != "pronto" || v.Result == nil || v.Result.KCl == nil || v.Cal.KCl == nil || v.CalSrc != "kcl" || v.Cal.Geometry != "pote de 500 mL" {
		t.Fatalf("resultado KCl: %+v", v.Result)
	}
	aK := 500 * actKPerKCl * actBqPerGK
	wantEff := 1.5 / (aK * 0.1066)
	if math.Abs(v.Cal.KCl.Eff/wantEff-1) > 3*v.Cal.KCl.EffErr {
		t.Fatalf("eficiência %v, queria %v", v.Cal.KCl.Eff, wantEff)
	}
	if len(v.History) != 1 || v.History[0].Kind != "kcl" {
		t.Fatalf("histórico: %+v", v.History)
	}
	a.CloseActivity()

	// 2) castanha (300 g) com o fundo salvo: K-40 e Ra-226
	start(ActOpts{Kind: "amostra", Name: "castanha", MassG: 300, Nuclides: []string{"K-40", "Ra-226", "xx"}}, "amostra")
	next(step(nutPeaks, 6*3600))
	v = a.GetActivity()
	if v.Result == nil || len(v.Result.Nuclides) != 2 {
		t.Fatalf("castanha: %+v", v.Result)
	}
	k40 := v.Result.Nuclides[0]
	wantK := 0.3 / (wantEff * 0.1066)
	if !k40.Detected || math.Abs(k40.Bq-wantK) > 3*k40.Err || k40.BqKg <= 0 {
		t.Fatalf("K-40 da castanha: %.1f ± %.1f (queria %.1f)", k40.Bq, k40.Err, wantK)
	}
	if ra := v.Result.Nuclides[1]; !ra.Detected || len(ra.Lines) != 3 {
		t.Fatalf("rádio: %+v", ra)
	}

	// 3) o backup leva calibração, fundo e medidas; o outro PC fica igual
	var buf bytes.Buffer
	if err := a.writeBackup(&buf); err != nil {
		t.Fatal(err)
	}
	b := newTestApp(t)
	zp := filepath.Join(t.TempDir(), "b.zip")
	os.WriteFile(zp, buf.Bytes(), 0o644)
	for i := 0; i < 2; i++ {
		if _, err := b.mergeZip(zp, true); err != nil {
			t.Fatal(err)
		}
	}
	vb := b.GetActivity()
	if vb.Cal.KCl == nil || vb.SavedBg == nil || len(vb.History) != 2 || vb.Phase != "" {
		t.Fatalf("no outro PC: %+v %+v %d %q", vb.Cal.KCl, vb.SavedBg, len(vb.History), vb.Phase)
	}

	// 4) validação
	for _, o := range []ActOpts{{Kind: "kcl", MassG: 100}, {Kind: "amostra", MassG: 10}, {Kind: "torio", Pct: 2}, {Kind: "x", MassG: 1}} {
		if err := actValidate(&o); err == nil {
			t.Fatalf("deveria recusar %+v", o)
		}
	}
	if err := a.StartActivity(ActOpts{Kind: "kcl", MassG: 10, Pct: 100, NewBg: true}); err == nil {
		t.Fatalf("sem aparelho deveria dar erro")
	}
}

// 2.101.4: uma calibração em que o pico do K-40 quase não aparece (o sal longe
// do cristal) não é guardada: estragaria todos os Bq dali para frente.
func TestActivityCalWeakNotSaved(t *testing.T) {
	a := newTestApp(t)
	rng := rand.New(rand.NewSource(11))
	bg := actFake(rng, 3600, map[float64]float64{1460.8: 2 * 3600, 609.3: 1.5 * 3600}, 0.5)
	smp := actFake(rng, 2940, map[float64]float64{1460.8: (2 + 0.01) * 2940, 609.3: 1.5 * 2940}, 0.5)
	st := &a.rc.act
	st.mu.Lock()
	st.loaded = true
	r := &actRun{Opts: ActOpts{Kind: "kcl", Name: "KCl (51%)", MassG: 250, Pct: 51, Geometry: "Becker"}, Phase: "amostra"}
	r.Bg = &actSaved{Sec: bg.Sec, A: bg.A, C: bg.C}
	r.Smp = &actSaved{Sec: smp.Sec, A: smp.A, C: smp.C}
	res := a.actFinish(r, time.Now())
	st.mu.Unlock()
	if res.KCl == nil || !res.NotSaved || res.KCl.EffErr <= actCalMaxErr {
		t.Fatalf("pico fraco: %+v (KCl %+v)", res, res.KCl)
	}
	if v := a.GetActivity(); v.Cal.KCl != nil {
		t.Fatalf("guardou a calibração ruim: %+v", v.Cal.KCl)
	}
}

// 2.101.4: aos 5 min já dá para avisar que o RC102 não está vendo o sal, em
// vez de deixar rodar 2 h. O KCl sobe o pico de 1.460 keV e o contínuo
// (espalhamento), e a contagem total junto; sem o sal, nada sobe.
func TestActivityKCLWeakEarly(t *testing.T) {
	aK := 250 * 0.51 * actKPerKCl * actBqPerGK
	for _, c := range []struct {
		eff, rise float64 // eficiência no pico e quanto o contínuo sobe
		weak      bool
	}{{0, 0, true}, {0.00003, 0.01, true}, {0.0006, 0.15, false}, {0.002, 0.5, false}, {0.006, 1.5, false}} {
		for seed := int64(1); seed <= 20; seed++ {
			rng := rand.New(rand.NewSource(seed))
			net := c.eff * aK * 0.1066
			bg := actFake(rng, 3600, map[float64]float64{1460.8: 0.3 * 3600, 609.3: 0.2 * 3600}, 0.5)
			smp := actFake(rng, actWeakSec, map[float64]float64{1460.8: (0.3 + net) * actWeakSec, 609.3: 0.2 * actWeakSec}, 0.5*(1+c.rise))
			k, _, _ := actCalKCl(smp, bg, 250, 51)
			if got := actKCLWeak(smp.Sec, smp.cps(), bg.cps(), k); got != c.weak {
				t.Fatalf("eficiência %g%%, contínuo +%g (semente %d): aviso %v, queria %v (cps %v / %v, %+v)", c.eff*100, c.rise, seed, got, c.weak, smp.cps(), bg.cps(), k)
			}
			if actKCLWeak(actWeakSec-1, smp.cps(), bg.cps(), k) {
				t.Fatal("avisou antes dos 5 min")
			}
		}
	}
	// a medida da madrugada de 18/11: 0,003% ± 80% aos 49 min, contagem igual
	// ao fundo; avisa (e pelo pico sozinho também, mesmo se o total subisse)
	k := &ActCalKCl{Eff: 0.00003, EffErr: 0.796}
	if !actKCLWeak(2940, 10, 10, k) || !actKCLWeak(2940, 20, 10, k) {
		t.Fatal("não avisou a medida de 18/11")
	}
	for _, sec := range []float64{actWeakSec, actWeakPeakSec} {
		if actKCLWeak(sec, 20, 10, &ActCalKCl{Eff: 0.002, EffErr: 0.1}) {
			t.Fatal("avisou com o pico normal")
		}
	}
}
