package app

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"radlog/internal/store"
)

func TestYearCal(t *testing.T) {
	a, _ := testApp(t)
	now := time.Now()
	y, m, d := now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, time.Local)
	d0, d1 := today.AddDate(0, 0, -3), today.AddDate(0, 0, -2)
	if d0.Year() != d1.Year() {
		d0, d1 = d0.AddDate(0, 0, -2), d1.AddDate(0, 0, -2)
	}
	var pts []store.Point
	for i := 0; i < 1440; i++ {
		pts = append(pts, store.Point{T: d0.Add(time.Duration(i) * time.Minute), CPM: 30, Src: store.SrcDevice})
	}
	for i := 0; i < 600; i++ {
		pts = append(pts, store.Point{T: d1.Add(time.Duration(i) * time.Minute), CPM: 40, Src: store.SrcDevice})
	}
	if _, err := a.store.Import(pts); err != nil {
		t.Fatal(err)
	}
	// RC102: o dia d0 inteiro a 0,1 µSv/h → 2,4 µSv
	_ = os.MkdirAll(a.rcDir(), 0o755)
	f, err := os.OpenFile(filepath.Join(a.rcDir(), d0.Format("2006-01")+".csv"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1440; i++ {
		fmt.Fprintf(f, "%s,%.3f,%.5f,%d\n", d0.Add(time.Duration(i)*time.Minute).Format(time.RFC3339), 10.0, 0.1, 60)
	}
	f.Close()
	a.rainDays = &rainDaysStore{V: rainDaysVersion, Days: map[string]RainDay{d0.Format("2006-01-02"): {MM: 4.2, Hours: 24}, d1.Format("2006-01-02"): {MM: 0.2, Hours: 24}}}
	pk := d1.Add(5 * time.Hour).UnixMilli()
	a.events = append(a.events,
		Event{ID: 1, Kind: kindAlert, Level: "atencao", Time: pk - 1000, Peak: 90, PeakAt: pk - 1000},
		Event{ID: 2, Kind: kindAlert, Level: "alarme", Time: pk, Peak: 200, PeakAt: pk})

	c, err := a.GetYearCal(d0.Year())
	if err != nil {
		t.Fatal(err)
	}
	if c.Year != d0.Year() || c.First != d0.Year() {
		t.Fatalf("ano %d primeiro %d", c.Year, c.First)
	}
	day := func(c YearCal, x time.Time) YearDay {
		for _, v := range c.Days {
			if v.Day == x.UnixMilli() {
				return v
			}
		}
		t.Fatalf("sem o dia %s", x)
		return YearDay{}
	}
	a0, a1 := day(c, d0), day(c, d1)
	if a0.N != 1440 || a0.Bg <= 0 || a1.N != 600 || a1.Bg <= a0.Bg {
		t.Fatalf("fundo: %+v %+v", a0, a1)
	}
	if math.Abs(a0.Dose-2.4) > 0.01 || a0.DoseN != 1440 || a0.Away {
		t.Fatalf("dose: %+v", a0)
	}
	if !a0.Rain || a0.MM != 4.2 || a1.Rain {
		t.Fatalf("chuva: %+v %+v", a0, a1)
	}
	if a1.Level != "alarme" || a1.Peak <= 0 || a1.PeakAt != pk || a0.Level != "" {
		t.Fatalf("alarme: %+v", a1)
	}
	if last := c.Days[len(c.Days)-1]; last.Day != today.UnixMilli() {
		t.Fatalf("último dia %v", time.UnixMilli(last.Day))
	}
	// trecho fora de casa com os dois aparelhos: refaz o mês
	a.periods = append(a.periods, Period{ID: 1, From: d1.UnixMilli(), To: d1.Add(2 * time.Hour).UnixMilli(), GMC: WhereMe, RC: WhereMe})
	c, _ = a.GetYearCal(d0.Year())
	if b := day(c, d1); b.N != 480 || !b.Away || day(c, d0).Away {
		t.Fatalf("fora de casa: %+v", b)
	}
	// ano fora do intervalo cai no ano de hoje
	if c, _ = a.GetYearCal(1990); c.Year != now.Year() {
		t.Fatalf("ano %d", c.Year)
	}
}
