package app

import (
	"testing"
	"time"
)

// 2.101.8 (varredura do QA, item 65): ignorar o GMC-800 ou deixar os dois em
// casa não é sair de casa. Saída é GMC-800 junto ou "eu não estava em casa".
func TestYouOutRule(t *testing.T) {
	cases := []struct {
		p         Period
		out, trip bool
	}{
		{Period{GMC: WhereOff, RC: WhereMe}, false, false},
		{Period{GMC: WhereOff, RC: WhereMe, NotHome: true}, true, true},
		{Period{GMC: WhereHome, RC: WhereHome}, false, false},
		{Period{GMC: WhereHome, RC: WhereHome, NotHome: true}, true, false},
		{Period{GMC: WhereHome, RC: WhereMe, NotHome: true}, true, true},
		{Period{GMC: WhereMe, RC: WhereMe}, true, true},
		{Period{GMC: WhereMe, RC: WhereHome}, true, false},
		{Period{}, true, true}, // trechos antigos: os dois com você
	}
	for _, c := range cases {
		if youOut(c.p) != c.out || isTrip(c.p) != c.trip {
			t.Errorf("%+v: youOut=%v isTrip=%v, quer %v %v", c.p, youOut(c.p), isTrip(c.p), c.out, c.trip)
		}
	}
}

func TestYouOutDayStripAndNotHomeWithGmcOff(t *testing.T) {
	a := newTestApp(t)
	t0 := time.Now().Add(-6 * time.Hour).Truncate(time.Hour)
	// GMC-800 ignorado (tubo quebrado), você em casa
	if _, err := a.SavePeriod(Period{From: t0.UnixMilli(), To: t0.Add(time.Hour).UnixMilli(), GMC: WhereOff, RC: WhereMe}); err != nil {
		t.Fatal(err)
	}
	// GMC-800 ignorado e você saiu com o RC102: o "eu não estava em casa" fica
	p, err := a.SavePeriod(Period{From: t0.Add(2 * time.Hour).UnixMilli(), To: t0.Add(3 * time.Hour).UnixMilli(), GMC: WhereOff, RC: WhereMe, NotHome: true})
	if err != nil || !p.NotHome {
		t.Fatalf("NotHome com o GMC-800 ignorado sumiu: %+v %v", p, err)
	}
	s, err := a.GetDayStrip(t0.UnixMilli(), t0.Add(4*time.Hour).UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range s.Slots {
		in := func(h int) bool {
			return x.T >= t0.Add(time.Duration(h)*time.Hour).UnixMilli() && x.T+s.StepMs <= t0.Add(time.Duration(h+1)*time.Hour).UnixMilli()
		}
		if in(0) && x.Away {
			t.Fatalf("trecho ignorado em casa marcado fora de casa em %v", time.UnixMilli(x.T))
		}
		if in(2) && !x.Away {
			t.Fatalf("saída com o RC102 não marcada fora de casa em %v", time.UnixMilli(x.T))
		}
	}
}
