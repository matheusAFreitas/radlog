package app

import (
	"testing"
	"time"
)

// Testes da varredura do QA da 2.101.2 (registros), agora de regressão (2.101.5).

// "Saí de casa" com o GMC-800 fora do cálculo (tubo com defeito): o GMC volta
// para o cálculo sem ninguém pedir.
func TestQALeaveHomeTiraForaDoCalculo(t *testing.T) {
	a, _ := testApp(t)
	// tirou o GMC-800 do cálculo há 2 h (sem contagens)
	if _, err := a.SetWhereAt(WhereOff, WhereMe, "quebrado", time.Now().Add(-2*time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	w0 := a.GetWhereabouts()
	t.Logf("antes: GMC=%s RC=%s aberto=%v", w0.GMC, w0.RC, w0.Open != nil && w0.Open.Name == "quebrado")
	// Ações rápidas > Saí de casa, padrão (só o RC102 vai)
	w, err := a.LeaveHome(false, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("depois do Saí de casa: GMC=%s RC=%s notHome=%v aberto=%q", w.GMC, w.RC, w.NotHome, w.Open.Name)
	w2, _ := a.SetWhere(WhereHome, WhereMe, "") // Voltei
	t.Logf("depois do Voltei: GMC=%s RC=%s", w2.GMC, w2.RC)
	if w.GMC != WhereOff {
		t.Errorf("o GMC-800 estava fora do cálculo e voltou para %q só com o Saí de casa", w.GMC)
	}
}

// "Mudei o aparelho de lugar" e depois "Desfazer a marca": a anotação fica.
func TestQAMovedUndoDeixaNota(t *testing.T) {
	a, _ := testApp(t)
	if _, err := a.MarkBgMoved(0); err != nil {
		t.Fatal(err)
	}
	if _, err := a.MarkBgMoved(0); err != nil { // clicou de novo
		t.Fatal(err)
	}
	if _, err := a.ClearBgMoved(); err != nil {
		t.Fatal(err)
	}
	ns := a.GetNotes(0, 0)
	for _, n := range ns {
		t.Logf("nota: %q", n.Text)
	}
	if a.settings.BgMoved != 0 {
		t.Fatal("marca não saiu")
	}
	if len(ns) > 0 {
		t.Errorf("desfez a marca, mas ficaram %d anotações dizendo que mudou de lugar", len(ns))
	}
}

// Editar um trecho com o nome automático e trocar os aparelhos: o nome velho fica.
func TestQAEditarTrechoNomeVelho(t *testing.T) {
	a, _ := testApp(t)
	t0 := time.Now().Add(-5 * time.Hour).Truncate(time.Minute)
	p, err := a.SavePeriod(Period{From: t0.UnixMilli(), To: t0.Add(90 * time.Minute).UnixMilli(), GMC: WhereMe, RC: WhereMe})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("criado: %q", p.Name)
	// a tela manda o nome que estava no campo (o automático) com os aparelhos novos
	q, err := a.SavePeriod(Period{ID: p.ID, From: p.From, To: p.To, Name: p.Name, GMC: WhereHome, RC: WhereMe, NotHome: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("editado: nome=%q GMC=%s RC=%s notHome=%v (o nome certo seria %q)", q.Name, q.GMC, q.RC, q.NotHome, periodNameOut)
	if q.Name != periodNameOut {
		t.Errorf("nome ficou %q", q.Name)
	}
}

// "Estou testando um objeto" no RC102: o minuto não conta como "no objeto"
// para a dose (o Medir objeto conta).
func TestQATesteObjetoNaoSaiDaDose(t *testing.T) {
	a, _ := testApp(t)
	if _, err := a.TestObject("radiacode"); err != nil {
		t.Fatal(err)
	}
	on := a.onObjectChecker(devRC)(time.Now())
	meas := a.measuringChecker()(time.Now())
	t.Logf("testando objeto no RC102: onObject=%v measuring=%v", on, meas)
	if !on {
		t.Errorf("os minutos com o RC102 encostado no objeto entram na dose e no fundo")
	}
}
