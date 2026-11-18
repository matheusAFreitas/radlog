package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"radlog/internal/store"
)

// 2.101.10 (QA, depois do merge da 2.101.9): textos montados no Go com o nome que a pessoa deu,
// que o demo não mostra (o demo devolve rótulos fixos).
// 1) "De onde vem a sua dose" (Resumo): o rótulo de cada trecho acima do fundo.
// 2) "Por que subiu?": a frase da anotação perto da subida.

func qa21010Data(t *testing.T, a *App, from time.Time, spikes [][2]int, n int) {
	t.Helper()
	var pts []store.Point
	for i := 0; i < n; i++ {
		cpm := 20.0
		for _, s := range spikes {
			if i >= s[0] && i < s[1] {
				cpm = 200
			}
		}
		pts = append(pts, store.Point{T: from.Add(time.Duration(i) * time.Minute), CPM: cpm, Src: store.SrcLive})
	}
	if _, err := a.store.Import(pts); err != nil {
		t.Fatal(err)
	}
}

// qa21010RC grava minutos do RC102 (com você): 0,10 µSv/h e 1,0 µSv/h nos picos
func qa21010RC(t *testing.T, a *App, from time.Time, spikes [][2]int, n int) {
	t.Helper()
	_ = os.MkdirAll(a.rcDir(), 0o755)
	files := map[string]*strings.Builder{}
	for i := 0; i < n; i++ {
		u := 0.10
		for _, s := range spikes {
			if i >= s[0] && i < s[1] {
				u = 1.0
			}
		}
		tm := from.Add(time.Duration(i) * time.Minute)
		f := tm.Format("2006-01") + ".csv"
		if files[f] == nil {
			files[f] = &strings.Builder{}
			files[f].WriteString(rcHeader + "\n")
		}
		fmt.Fprintf(files[f], "%s,%.1f,%.4f,60\n", tm.Format(time.RFC3339), u*60, u)
	}
	for f, b := range files {
		if err := os.WriteFile(filepath.Join(a.rcDir(), f), []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestQA21010OriginLabels(t *testing.T) {
	a, _ := testApp(t)
	end := time.Now().Truncate(stripStep)
	from := end.Add(-6 * time.Hour)
	at := func(m int) int64 { return from.Add(time.Duration(m) * time.Minute).UnixMilli() }
	spikes := [][2]int{{30, 50}, {90, 110}, {150, 170}, {210, 230}, {270, 290}}
	qa21010Data(t, a, from, spikes, 360)
	qa21010RC(t, a, from, spikes, 360)
	// 1º pico: medição "Relógio"
	m := SavedMeasure{ID: 1, Name: "Relógio", Time: at(30)}
	m.Obj.Start, m.Obj.End = at(31), at(45)
	a.measurements = append(a.measurements, m)
	// 2º pico: anotação "Areia monazítica"
	a.notes = append(a.notes, Note{ID: 1, Time: at(95), Text: "Areia monazítica", Created: at(95)})
	// 3º pico: saída "Escritório" (levei os dois)
	a.periods = append(a.periods, Period{ID: 1, From: at(145), To: at(175), Name: "Escritório", GMC: WhereMe, RC: WhereMe})
	// 4º pico: em casa com o GMC-800 quebrado (motivo pronto "quebrado")
	a.periods = append(a.periods, Period{ID: 2, From: at(205), To: at(235), Name: "quebrado", GMC: WhereOff, RC: WhereMe})
	// 5º pico: "Deixei os dois em casa" (você em casa)
	a.periods = append(a.periods, Period{ID: 3, From: at(265), To: at(295), Name: periodName(WhereHome, WhereHome), GMC: WhereHome, RC: WhereHome})
	o, err := a.GetDoseOriginRange(from.UnixMilli(), end.UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("trechos acima do fundo: %d (fundo em casa %.3f, fora de casa %.3f µSv)", len(o.Items), o.HomeBg, o.AwayBg)
	all := ""
	for _, it := range o.Items {
		t.Logf("%s–%s %-8s %q", time.UnixMilli(it.From).Format("15:04"), time.UnixMilli(it.To).Format("15:04"), it.Kind, it.Label)
		all += it.Label + "\n"
	}
	for _, want := range []string{"Medir objeto · " + asIs("Relógio"), "Anotação · " + asIs("Areia monazítica"), "Fora de casa · " + asIs("Escritório")} {
		if !strings.Contains(all, want) {
			t.Errorf("falta %q em:\n%s", want, all)
		}
	}
	// em casa (GMC-800 ignorado, ou os dois em casa) não é "Fora de casa"
	for _, bad := range []string{"Fora de casa · quebrado", "Deixei os dois em casa"} {
		if strings.Contains(all, bad) {
			t.Errorf("não devia ter %q:\n%s", bad, all)
		}
	}
}

func TestQA21010WhyNote(t *testing.T) {
	a, _ := testApp(t)
	end := time.Now().Truncate(time.Minute)
	from := end.Add(-14 * time.Hour)
	// fundo de 20 CPM e subida nos últimos 40 min
	qa21010Data(t, a, from, [][2]int{{14*60 - 40, 14 * 60}}, 14*60)
	a.notes = append(a.notes, Note{ID: 1, Time: end.Add(-30 * time.Minute).UnixMilli(), Text: "Areia monazítica", Created: end.UnixMilli()})
	w, err := a.GetWhyRise("gmc", end.Add(-time.Minute).UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	ok := false
	for _, c := range w.Causes {
		if c.Key == "objeto" {
			t.Logf("causa %q (%s): %q", c.Title, c.State, c.Ev)
			ok = ok || strings.Contains(c.Ev, `"`+asIs("Areia monazítica")+`"`)
		}
	}
	if !ok {
		t.Error("a anotação da subida devia levar o nome marcado (asIs)")
	}
}
