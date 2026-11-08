package dose

import (
	"math"
	"testing"
	"time"
)

func TestCalendar(t *testing.T) {
	loc := time.FixedZone("BRT", -3*3600)
	// quarta-feira, 11/11/2026 15:30
	now := time.Date(2026, 11, 11, 15, 30, 0, 0, loc)
	ps := Calendar(now)
	want := map[string]time.Time{
		"dia":    time.Date(2026, 11, 11, 0, 0, 0, 0, loc),
		"semana": time.Date(2026, 11, 9, 0, 0, 0, 0, loc), // segunda
		"mes":    time.Date(2026, 11, 1, 0, 0, 0, 0, loc),
		"ano":    time.Date(2026, 1, 1, 0, 0, 0, 0, loc),
	}
	if len(ps) != 4 {
		t.Fatalf("%d períodos", len(ps))
	}
	for _, p := range ps {
		if !p.From.Equal(want[p.Key]) || !p.To.Equal(now) {
			t.Errorf("%s: %s → %s", p.Key, p.From, p.To)
		}
	}
	// domingo: a semana começou na segunda anterior
	sun := time.Date(2026, 11, 15, 8, 0, 0, 0, loc)
	if w := Calendar(sun)[1].From; !w.Equal(time.Date(2026, 11, 9, 0, 0, 0, 0, loc)) {
		t.Errorf("semana no domingo: %s", w)
	}
	// segunda: começa hoje
	mon := time.Date(2026, 11, 9, 0, 5, 0, 0, loc)
	if w := Calendar(mon)[1].From; !w.Equal(time.Date(2026, 11, 9, 0, 0, 0, 0, loc)) {
		t.Errorf("semana na segunda: %s", w)
	}
}

func TestYearProjection(t *testing.T) {
	// 0,12 µSv/h durante um dia medido inteiro → ~1,05 mSv/ano
	got := YearProjection(0.12*24, 1440)
	if math.Abs(got-0.12*24*365.25) > 1e-9 {
		t.Fatalf("projeção %v", got)
	}
	if YearProjection(5, 0) != 0 {
		t.Fatal("sem minutos deveria ser 0")
	}
}

func TestAbove(t *testing.T) {
	if v := Above(1.03, 1); math.Abs(v-0.03) > 1e-12 {
		t.Fatalf("acima %v", v)
	}
	if Above(1, 0) != 0 {
		t.Fatal("sem fundo deveria ser 0")
	}
}
