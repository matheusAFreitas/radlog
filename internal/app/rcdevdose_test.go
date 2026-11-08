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
