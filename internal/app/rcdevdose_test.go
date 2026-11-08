package app

import (
	"math"
	"testing"
	"time"
)

func TestRcDoseCurveSmallDropIsNotReset(t *testing.T) {
	t0 := time.Date(2026, 11, 7, 12, 0, 0, 0, time.Local)
	at := func(m int) time.Time { return t0.Add(time.Duration(m) * time.Minute) }
	// um PC anotou 2,30 (estimado) e o outro, logo depois, 2,28 (lido);
	// o aparelho segue subindo devagar
	pts := []rcDosePt{
		{at(0), 2.00, 60000}, {at(30), 2.30, 61800}, {at(31), 2.28, 61790},
		{at(60), 2.40, 63600}, {at(90), 2.50, 65400},
	}
	c := rcDoseCurve(pts)
	if got := c[len(c)-1].Dose; math.Abs(got-0.50) > 1e-9 {
		t.Fatalf("dose: %.3f (quero 0,50, sem o degrau)", got)
	}
	s := rcSecsCurve(pts)
	if got := s[len(s)-1].Dose; math.Abs(got-5400) > 1e-9 {
		t.Fatalf("tempo: %.0f s (quero 5400)", got)
	}
	// zerado de verdade: dose e tempo voltam perto de zero
	pts = append(pts, rcDosePt{at(91), 0.001, 30}, rcDosePt{at(120), 0.10, 1800})
	c = rcDoseCurve(pts)
	if got := c[len(c)-1].Dose; math.Abs(got-0.60) > 1e-9 {
		t.Fatalf("depois de zerar: %.3f (quero 0,60)", got)
	}
	s = rcSecsCurve(pts)
	if got := s[len(s)-1].Dose; math.Abs(got-(5400+1800)) > 1e-9 {
		t.Fatalf("tempo depois de zerar: %.0f", got)
	}
	// registros sem o tempo: só a dose decide
	old := []rcDosePt{{at(0), 1.0, 0}, {at(10), 1.2, 0}, {at(11), 1.19, 0}, {at(20), 0.05, 0}, {at(30), 0.1, 0}}
	if got := rcDoseCurve(old)[4].Dose; math.Abs(got-0.3) > 1e-9 {
		t.Fatalf("sem tempo: %.3f (quero 0,30)", got)
	}
}

// 2.12.2: depois do último total do aparelho, a curva segue com os minutos
// do app (antes ficava parada, e o teto também)
func TestRcDoseFromDeviceTail(t *testing.T) {
	a := newTestApp(t)
	to := time.Now().Truncate(time.Minute)
	from := to.Add(-60 * time.Minute)
	// totais do aparelho só até 30 min atrás
	a.rcDoseLogAppend(from.Add(-time.Minute), 5.0, 1000)
	a.rcDoseLogAppend(from.Add(30*time.Minute), 5.06, 1000+31*60)
	var mins []doseMin
	for k := 0; k < 60; k++ {
		mins = append(mins, doseMin{T: from.Add(time.Duration(k) * time.Minute), USv: 0.12})
	}
	var c DoseChart
	if !a.rcDoseFromDevice(&c, from, to, time.Minute, mins, 0.1/60, 0.11/60) {
		t.Fatal("não usou o aparelho")
	}
	n := len(c.Dose)
	if c.Dose[n-1] <= c.Dose[n/2]+0.04 {
		t.Fatalf("a dose parou depois do último total: meio %v, fim %v", c.Dose[n/2], c.Dose[n-1])
	}
	if c.Ceiling[n-1] <= c.Ceiling[n/2]*1.5 {
		t.Fatalf("o teto parou: meio %v, fim %v", c.Ceiling[n/2], c.Ceiling[n-1])
	}
	// os ~29 min depois do último total: 0,12 µSv/h → 0,058 µSv
	if want := 0.06 + 29*0.12/60; math.Abs(c.Total-want) > 0.01 {
		t.Fatalf("total %v, quero ~%v", c.Total, want)
	}
}
