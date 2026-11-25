package app

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"radlog/internal/radonscan"
)

// sessão parecida com a do aparelho de verdade (24/11/2026): ligado às
// 11:04, tabela a partir da entrada 3865, 9 horas com o PC desligado
func radonMemFixture() (radonscan.Memory, []RadonRow, time.Time, time.Time) {
	counts := []int{10, 5, 15, 1, 3, 4, 3, 2, 9, 6, 4, 5, 1, 2, 3, 2, 4, 5, 7, 2, 2, 3, 6}
	loc := time.FixedZone("BRT", -3*3600)
	on := time.Date(2026, 11, 23, 11, 4, 0, 0, loc)
	mem := radonscan.Memory{Hours: make([]int, radonscan.HourSlots), Uptime: 23 * 3600}
	mem.Hours[976], mem.Hours[1200] = 1, 1 // restos antigos
	copy(mem.Hours[3865:], counts)
	r2 := func(v float64) float64 { return math.Round(v*100) / 100 }
	// contagens espalhadas por igual dentro de cada hora do aparelho
	var events []time.Time
	for h, c := range counts {
		for j := 0; j < c; j++ {
			events = append(events, on.Add(time.Duration(h)*time.Hour+time.Duration((j*60+7)/c)*time.Minute+30*time.Second))
		}
	}
	slide := func(t time.Time) int {
		n := 0
		for _, e := range events {
			if e.After(t.Add(-time.Hour)) && !e.After(t) {
				n++
			}
		}
		return n
	}
	var rows []RadonRow
	off0 := on.Add(14*time.Hour + 6*time.Minute)  // 01:10
	off1 := on.Add(23*time.Hour + 15*time.Minute) // 10:19
	for t := on.Add(5 * time.Minute); t.Before(on.Add(23*time.Hour + 40*time.Minute)); t = t.Add(time.Minute) {
		if !t.Before(off0) && t.Before(off1) {
			continue
		}
		// média de 1 hora deslizante: contagens nos últimos 60 min
		v := r2(float64(slide(t)) / 24)
		// grava quando muda ou a cada hora, como o app
		if n := len(rows); n == 0 || rows[n-1].Hour != v || t.Sub(time.UnixMilli(rows[n-1].T)) >= time.Hour {
			rows = append(rows, RadonRow{T: t.UnixMilli(), Place: "quarto", Hour: v})
		}
	}
	now := on.Add(23*time.Hour + 55*time.Minute) // 10:59 do dia seguinte
	return mem, rows, now, on
}

func TestRadonAlignAndFill(t *testing.T) {
	mem, rows, now, on := radonMemFixture()
	since := now.Add(-time.Duration(mem.Uptime+3600) * time.Second)
	al, err := radonAlign(mem.Hours, rows, now, since)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("virada aos %d s, deslocamento %d", al.phase, al.shift)
	if got := al.index(on.Add(90 * time.Minute)); got != 3865 {
		t.Fatalf("hora 11:04–12:04 na entrada %d, queria 3865", got)
	}
	if d := al.end(3865).Sub(on.Add(time.Hour)); d < -5*time.Minute || d > 5*time.Minute {
		t.Fatalf("virada da hora errada: %v", d)
	}
	fill := radonFill(mem, al, rows, now, since, radonPlaceAt(rows, ""))
	if len(fill) != 9 {
		for _, r := range fill {
			t.Log(time.UnixMilli(r.Start), r.Counts)
		}
		t.Fatalf("%d horas preenchidas, queria 9", len(fill))
	}
	want := []int{3, 2, 4, 5, 7, 2, 2, 3, 6}
	for i, r := range fill {
		if r.Counts != want[i] || r.Place != "quarto" {
			t.Fatalf("hora %d: %+v", i, r)
		}
	}
	if h := time.UnixMilli(fill[0].Start).Sub(on); h < 14*time.Hour-5*time.Minute || h > 14*time.Hour+5*time.Minute {
		t.Fatalf("primeira hora começa %v depois de ligar", h)
	}
}

func TestRadonAlignNeedsData(t *testing.T) {
	mem, rows, now, _ := radonMemFixture()
	if _, err := radonAlign(mem.Hours, rows[:2], now, time.Time{}); err == nil {
		t.Fatal("encaixou com 2 leituras")
	}
	// tabela toda zerada: nenhum encaixe é melhor que outro
	zero := make([]int, radonscan.HourSlots)
	if _, err := radonAlign(zero, rows, now, time.Time{}); err == nil {
		t.Fatal("encaixou numa tabela zerada")
	}
}

func TestRadonMemSaveAndGaps(t *testing.T) {
	mem, rows, now, _ := radonMemFixture()
	a := &App{dataDir: t.TempDir()}
	since := now.Add(-time.Duration(mem.Uptime+3600) * time.Second)
	if g := radonGaps(rows, nil, now); len(g) != 1 {
		t.Fatalf("buracos: %+v", g)
	}
	al, _ := radonAlign(mem.Hours, rows, now, since)
	fill := radonFill(mem, al, rows, now, since, radonPlaceAt(rows, ""))
	add, err := a.radonMemSave(fill)
	if err != nil || len(add) != 9 {
		t.Fatal(len(add), err)
	}
	again, _ := a.radonMemSave(fill)
	if len(again) != 0 {
		t.Fatalf("gravou de novo: %d", len(again))
	}
	back := a.radonMemRows()
	if len(back) != 9 || back[4].Counts != 7 {
		t.Fatalf("lido de volta: %+v", back)
	}
	if g := radonGaps(rows, back, now); len(g) != 0 {
		t.Fatalf("buraco continua: %+v", g)
	}
	all := radonMergeMem(rows, back)
	if len(all) != len(rows)+9 || !all[len(all)-1].Mem && all[len(all)-1].T < back[8].End {
		t.Fatal("junção errada")
	}
	b, _ := os.ReadFile(filepath.Join(a.dataDir, "radonscan-memoria.csv"))
	if len(b) == 0 || string(b[:len(radonMemHeader)]) != radonMemHeader {
		t.Fatalf("arquivo: %q", b)
	}
}
