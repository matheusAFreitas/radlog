package app

import (
	"strings"
	"testing"
)

// 2.101.14 (item 17): no RC102 o automático não conclui "igual ao fundo" só
// pela contagem; com a dose subindo ele para como "dose" e o veredito salvo
// não é trocado.
func TestAutoRCDoseItem17(t *testing.T) {
	bg := MeasurePhase{Counts: 600, Seconds: 60, CPM: 600, ErrCPM: 24.5, USv: 0.169, ErrUSv: 0.0015}
	obj := func(sec int, usv, err float64) MeasurePhase {
		return MeasurePhase{Counts: 10 * sec, Seconds: sec, CPM: 600, ErrCPM: 600 / (float64(sec) / 60) / 600 * 24.5, USv: usv, ErrUSv: err}
	}
	usv := func(c float64) float64 { return c * 0.169 / 600 }
	// 33 s: contagem igual e dose subindo; antes de 10 blocos, nada decide
	r := compareMeasure(bg, obj(33, 0.188, 0.004), usv)
	if got := autoDecide(r, bg, 33, 10); got != "" {
		t.Fatalf("33 s no RC102 decidiu %q; esperado continuar", got)
	}
	// 330 s (10 blocos): dose 0,019 com ± 0,0015: para como "dose"
	r = compareMeasure(bg, obj(330, 0.188, 0.0015), usv)
	if !r.DoseUp {
		t.Fatal("dose subiu e DoseUp ficou false")
	}
	if got := autoDecide(r, bg, 330, 20); got != "dose" {
		t.Fatalf("autoDecide = %q; esperado dose", got)
	}
	finishResult(&r, "dose", bg, 20, true)
	if strings.Contains(r.Verdict, "igual ao fundo") || r.Limit != 0 || r.LimitUSv != 0 {
		t.Fatalf("dose subiu e saiu %q (limite %v / %v)", r.Verdict, r.Limit, r.LimitUSv)
	}
	// mesmo que algo chame "nada" com a dose subindo, o veredito fica
	r2 := compareMeasure(bg, obj(330, 0.188, 0.0015), usv)
	finishResult(&r2, "nada", bg, 20, true)
	if !strings.Contains(r2.Verdict, "a dose subiu") {
		t.Fatalf("nada com DoseUp trocou o veredito: %q", r2.Verdict)
	}
	// dose parada: "nada" só depois dos blocos e com um teto só, pela dose
	r3 := compareMeasure(bg, obj(330, 0.169, 0.0015), usv)
	if got := autoDecide(r3, bg, 330, 20); got != "nada" {
		t.Fatalf("dose parada: autoDecide = %q; esperado nada", got)
	}
	finishResult(&r3, "nada", bg, 20, true)
	want := "+" + usvTxt(r3.LimitUSv) + " µSv/h"
	if !strings.Contains(r3.Verdict, want) || r3.LimitUSv < r3.NetUSv {
		t.Fatalf("veredito %q sem o teto %s (líquido %v)", r3.Verdict, want, r3.NetUSv)
	}
	if lim := measLimitUSv(r3, 1); lim != r3.LimitUSv {
		t.Fatalf("PDF usaria outro teto: %v ≠ %v", lim, r3.LimitUSv)
	}
}

// GMC-800 (sem ± da dose) continua decidindo só pela contagem.
func TestAutoGMCItem17(t *testing.T) {
	bg := MeasurePhase{Counts: 30, Seconds: 60, CPM: 30, ErrCPM: 5.5, USv: 0.169}
	ob := MeasurePhase{Counts: 3000, Seconds: 6000, CPM: 30, ErrCPM: 0.55, USv: 0.169}
	r := compareMeasure(bg, ob, func(c float64) float64 { return c * 0.00564 })
	if r.NetErrUSv != 0 {
		t.Fatal("GMC-800 não tem ± da dose")
	}
	if got := autoDecide(r, bg, 6000, 50); got != "nada" {
		t.Fatalf("GMC-800: autoDecide = %q; esperado nada", got)
	}
}

// 2.101.14 (item 35): calibração pior que a atual não troca sozinha; dá para
// usar a nova mesmo assim e voltar para a de antes.
func TestActWorseCalItem35(t *testing.T) {
	a := newTestApp(t)
	old := &ActCalKCl{T: 1, Eff: 0.00187, EffErr: 0.04}
	a.actWriteJSON(actCalFile, ActCal{KCl: old, Geometry: "pote antigo"})
	nova := &ActCalKCl{T: 2, Eff: 0.00282, EffErr: 0.15}
	res := ActResult{T: 2, Kind: "kcl", KCl: nova, Worse: true, PrevErr: old.EffErr, Geometry: "becker"}
	a.actAppendResult(res)
	a.rc.act.run = &actRun{Phase: "pronto", Result: &res}
	if a.actCal().KCl.EffErr != 0.04 {
		t.Fatal("a pior trocou a atual")
	}
	if err := a.UseActivityCal(3); err == nil {
		t.Fatal("usou um resultado que não está na tela")
	}
	if err := a.UseActivityCal(2); err != nil {
		t.Fatal(err)
	}
	c := a.actCal()
	if c.KCl == nil || c.KCl.EffErr != 0.15 || c.PrevKCl == nil || c.PrevKCl.EffErr != 0.04 {
		t.Fatalf("depois de usar: %+v / %+v", c.KCl, c.PrevKCl)
	}
	if h := a.actHistory(); len(h) != 1 || !h[0].Used {
		t.Fatalf("histórico não marcou a escolha: %+v", h)
	}
	if err := a.UseActivityCal(2); err == nil {
		t.Fatal("usou duas vezes")
	}
	if err := a.RestoreActivityCal("kcl"); err != nil {
		t.Fatal(err)
	}
	if c := a.actCal(); c.KCl.EffErr != 0.04 || c.PrevKCl.EffErr != 0.15 || c.Geometry != "pote antigo" {
		t.Fatalf("voltar não trocou: %+v / %+v", c.KCl, c.PrevKCl)
	}
	a.ForgetActivityCal("kcl")
	if c := a.actCal(); c.KCl != nil || c.PrevKCl != nil {
		t.Fatal("esquecer deixou a anterior")
	}
	if err := a.RestoreActivityCal("kcl"); err == nil {
		t.Fatal("voltou sem anterior")
	}
}

// Conferência do QA (item 17): entre 3σ e 4σ não para em "nada", e com 4
// blocos a dose ainda não decide.
func TestAutoAmbiguousItem17(t *testing.T) {
	bg := MeasurePhase{Counts: 600, Seconds: 60, CPM: 600, ErrCPM: 24.5, USv: 0.169, ErrUSv: 0.0015}
	r := MeasureResult{NetCPM: 50, NetErr: 15, Sigma: 3.3, NetUSv: 0.02, NetErrUSv: 0.01, DoseSigma: 2}
	if got := autoDecide(r, bg, 400, 20); got != "" {
		t.Fatalf("contagem a 3,3σ decidiu %q", got)
	}
	r = MeasureResult{NetCPM: 0, NetErr: 15, NetUSv: 0.006, NetErrUSv: 0.0015, DoseSigma: 4.2}
	if got := autoDecide(r, bg, 150, 20); got != "" {
		t.Fatalf("dose com 5 blocos decidiu %q", got)
	}
	// "nada" sempre com teto, mesmo com Level "sim" vindo de fora
	r = MeasureResult{Level: "sim", NetCPM: 10, NetErr: 5, NetUSv: 0.01, NetErrUSv: 0.004}
	finishResult(&r, "nada", bg, 20, true)
	if r.LimitUSv <= r.NetUSv {
		t.Fatalf("nada sem teto: %v", r.LimitUSv)
	}
}
