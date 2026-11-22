package app

import (
	"math"
	"math/rand"
	"testing"
	"time"
)

// 2.104.0: a eficiência da montagem é a soma das rodadas (contagens com
// contagens), e o ± cai com a raiz do tempo somado
func TestActMountSum(t *testing.T) {
	m := ActMount{Material: "adubo", BgMode: "branco", Rounds: []ActRound{
		{T: 1, Sec: 3600, BgSec: 3600, Area: 100, AreaErr: 20, ExpK: 1e6},
		{T: 2, Sec: 3600, BgSec: 3600, Area: 120, AreaErr: 20, ExpK: 1e6},
	}}
	v := m.view()
	if math.Abs(v.Eff-1.1e-4) > 1e-12 {
		t.Fatalf("eficiência %v", v.Eff)
	}
	if want := math.Sqrt(800) / 220; math.Abs(v.CountErr-want) > 1e-12 {
		t.Fatalf("contagem ±%v, queria %v", v.CountErr, want)
	}
	if v.Sec != 7200 || v.LastT != 2 || !v.Usable {
		t.Fatalf("%+v", v)
	}
	// sal light: o rótulo (±15%) pesa; sem branco, mais 3,5%
	if f := actFixErr("light", "vazio"); math.Abs(f-math.Hypot(0.15, 0.035)) > 1e-12 {
		t.Fatalf("fixos %v", f)
	}
	one := ActMount{Material: "light", Rounds: []ActRound{{T: 1, Sec: 3600, Area: 30, AreaErr: 12, ExpK: 1e6}}}
	if one.view().Usable {
		t.Fatal("±40% de contagem não serve para medir")
	}
}

// a nuvem soma as rodadas sem repetir; nome e "apagada" ficam com a mais nova
func TestActMergeMounts(t *testing.T) {
	cur := []ActMount{{ID: "a", Name: "velho", Upd: 1, Rounds: []ActRound{{T: 10}, {T: 20}}}}
	in := []ActMount{{ID: "a", Name: "novo", Upd: 2, Rounds: []ActRound{{T: 20}, {T: 30}}}, {ID: "b", Name: "outra", Upd: 1}}
	out := actMergeMounts(cur, in)
	if len(out) != 2 || out[0].Name != "novo" || len(out[0].Rounds) != 3 || out[0].Rounds[2].T != 30 {
		t.Fatalf("%+v", out)
	}
	out = actMergeMounts(out, []ActMount{{ID: "a", Deleted: true, Upd: 3}})
	if !out[0].Deleted || out[0].Rounds != nil {
		t.Fatalf("apagada não pegou: %+v", out[0])
	}
	// uma cópia velha não ressuscita a apagada
	out = actMergeMounts(out, []ActMount{{ID: "a", Name: "novo", Upd: 2, Rounds: []ActRound{{T: 40}}}})
	if !out[0].Deleted || len(out[0].Rounds) != 0 {
		t.Fatalf("ressuscitou: %+v", out[0])
	}
}

// a calibração por montagem termina sempre somando a rodada, sem mexer na
// calibração antiga, e a amostra medida com a montagem usa a eficiência dela
func TestActFinishMount(t *testing.T) {
	a := newTestApp(t)
	rng := rand.New(rand.NewSource(21))
	const g = 1 / 1.0324
	run := func(o ActOpts, salt float64) ActResult {
		bg := gainFake(rng, 3600, 0, g)
		smp := gainFake(rng, 3600, salt, g)
		r := &actRun{Opts: o, Phase: "amostra"}
		r.Bg = &actSaved{Sec: bg.Sec, A: bg.A, C: bg.C}
		r.Smp = &actSaved{Sec: smp.Sec, A: smp.A, C: smp.C}
		a.rc.act.mu.Lock()
		defer a.rc.act.mu.Unlock()
		res := a.actFinish(r, time.Now())
		return res
	}
	o := ActOpts{Kind: "kcl", MassG: 1000, Pct: 95, Setup: "pote1l", Material: "adubo", BgMode: "branco"}
	if err := actValidate(&o); err != nil {
		t.Fatal(err)
	}
	r1 := run(o, 0.07)
	if r1.Mount == nil || r1.MountID == "" || len(r1.Mount.Rounds) != 1 {
		t.Fatalf("primeira rodada não criou a montagem: %+v", r1.Mount)
	}
	if a.actCal().KCl != nil {
		t.Fatal("a calibração antiga não pode mudar")
	}
	o.MountID = r1.MountID
	time.Sleep(2 * time.Millisecond)
	r2 := run(o, 0.07)
	if r2.Mount == nil || len(r2.Mount.Rounds) != 2 || r2.Mount.CountErr >= r1.Mount.CountErr {
		t.Fatalf("segunda rodada não somou: %+v", r2.Mount)
	}
	if !r2.Mount.Found || math.Abs(r2.Mount.PeakKeV-actK40*g) > 8 {
		t.Fatalf("pico %v", r2.Mount.PeakKeV)
	}
	smp := ActOpts{Kind: "amostra", MassG: 1000, Nuclides: []string{"K-40"}, MountID: r1.MountID}
	if err := actValidate(&smp); err != nil {
		t.Fatal(err)
	}
	r3 := run(smp, 0.07)
	if r3.CalSrc != "kcl" || r3.MountID != r1.MountID || len(r3.Nuclides) != 1 {
		t.Fatalf("amostra sem a montagem: %+v", r3)
	}
	// mesma montagem, mesmo material: o K-40 medido bate com o do KCl (~±25%)
	want := 1000 * 0.95 * actKPerKCl * actBqPerGK
	if got := r3.Nuclides[0].Bq; math.Abs(got/want-1) > 0.35 {
		t.Fatalf("K-40 %v Bq, esperado ~%v", got, want)
	}
}

// a previsão pelo sinal real: com sinal forte cai, e o melhor possível com o
// fundo fica abaixo da previsão
func TestActCalCheck(t *testing.T) {
	if actCalCheck(300, 3600, 3600, 1.5, 1, nil, 0) != nil {
		t.Fatal("antes de 10 min não confere nada")
	}
	c := actCalCheck(900, 3600, 3600, 1.49, 1, nil, 0)
	if c == nil || c.Ready || math.Abs(c.BandRise-0.49) > 1e-9 {
		t.Fatalf("%+v", c)
	}
	k := &ActCalKCl{EffErr: 0.3}
	c = actCalCheck(1800, 3600, 3600, 1.49, 1, k, 60)
	if !c.Ready || c.SignalCpm != 2 || c.Forecast <= 0 || c.Forecast >= 0.3 || c.Best >= c.Forecast {
		t.Fatalf("%+v", c)
	}
	if math.Abs(c.ExpectLo-0.49*60/9*0.5) > 1e-9 {
		t.Fatalf("esperado %v", c.ExpectLo)
	}
}

// a calibração única feita na 2.103.0 (com ganho) vira montagem; a de antes
// da correção do pico, não
func TestActMigrateLegacy(t *testing.T) {
	a := newTestApp(t)
	a.actWriteJSON(actCalFile, ActCal{Geometry: "béquer", KCl: &ActCalKCl{T: 5, Eff: 6e-5, EffErr: 0.2, MassG: 250, Pct: 51, Sec: 21600}})
	a.actMigrateLegacy()
	if len(a.actMountsAll()) != 0 {
		t.Fatal("calibração de antes da correção virou montagem")
	}
	a.actWriteJSON(actCalFile, ActCal{Geometry: "béquer", KCl: &ActCalKCl{T: 7, Eff: 1e-4, EffErr: 0.15, MassG: 250, Pct: 51, Sec: 3600, PeakKeV: 1415, Gain: 1.0324}})
	a.actMigrateLegacy()
	ms := a.actMountViews()
	if len(ms) != 1 || ms[0].ID != "kcl-7" || ms[0].Name != "béquer" || ms[0].Material != "light" || math.Abs(ms[0].Eff-1e-4) > 1e-12 || math.Abs(ms[0].CountErr-0.15) > 1e-9 {
		t.Fatalf("%+v", ms)
	}
	a.DeleteActMount("kcl-7")
	a.actMigrateLegacy()
	if len(a.actMountViews()) != 0 {
		t.Fatal("apagada voltou")
	}
}
