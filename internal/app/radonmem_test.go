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
	fill, info := radonFill(mem, rows, nil, now, radonPlaceAt(rows, nil, ""))
	t.Logf("%+v", info)
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
	if g := radonGaps(rows, nil, now); len(g) != 1 {
		t.Fatalf("buracos: %+v", g)
	}
	fill, _ := radonFill(mem, rows, nil, now, radonPlaceAt(rows, nil, ""))
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

// radonSim imita o aparelho: o relógio dele só anda enquanto tem energia,
// a tabela anda uma entrada por hora fechada (a mais nova em HourNewest) e
// o app só lê nos trechos "ao vivo".
type radonSimSeg struct {
	on, off time.Time
	level   int // contagens por hora, mais ou menos
	live    bool
	place   string
}

type radonSimHour struct {
	end    time.Time // fim da hora no relógio de parede
	counts int
}

func radonSim(segs []radonSimSeg) (radonscan.Memory, []RadonRow, []radonSimHour) {
	seed := uint32(7)
	rnd := func(n int) int {
		seed = seed*1103515245 + 12345
		return int(seed>>16) % n
	}
	var hours []radonSimHour
	var evOp []int // minuto ligado de cada contagem
	op := 0
	cur, curC := 0, -1
	var rows []RadonRow
	r2 := func(v float64) float64 { return math.Round(v*100) / 100 }
	for _, sg := range segs {
		for t := sg.on; t.Before(sg.off); t = t.Add(time.Minute) {
			h, mm := op/60, op%60
			if h != curC {
				curC = h
				cur = sg.level/2 + rnd(sg.level+1)
			}
			if (mm*cur)/60 != ((mm+1)*cur)/60 {
				evOp = append(evOp, op)
			}
			op++
			if op%60 == 0 {
				n := 0
				for _, e := range evOp {
					if e >= op-60 && e < op {
						n++
					}
				}
				hours = append(hours, radonSimHour{end: t.Add(time.Minute), counts: n})
			}
			if sg.live {
				n := 0
				for _, e := range evOp {
					if e >= op-60 && e < op {
						n++
					}
				}
				v := r2(float64(n) / 24)
				if k := len(rows); k == 0 || rows[k-1].Hour != v || t.Sub(time.UnixMilli(rows[k-1].T)) >= time.Hour {
					rows = append(rows, RadonRow{T: t.UnixMilli(), Place: sg.place, Hour: v})
				}
			}
		}
	}
	mem := radonscan.Memory{Hours: make([]int, radonscan.HourSlots), Uptime: len(hours) * 3600}
	for i, h := range hours {
		mem.Hours[radonscan.HourNewest-(len(hours)-1-i)] = h.counts
	}
	return mem, rows, hours
}

// o caso de 25/11: aparelho tirado do PC, 15 h ligado num carregador na
// sala e de volta ao PC; o app baixa logo que ele reconecta
func TestRadonFillAfterPowerCycle(t *testing.T) {
	loc := time.FixedZone("BRT", -3*3600)
	d := func(day, h, m int) time.Time { return time.Date(2026, 11, day, h, m, 0, 0, loc) }
	segs := []radonSimSeg{
		{d(23, 11, 4), d(24, 10, 37), 5, true, "quarto"},
		{d(24, 11, 10), d(24, 11, 20), 5, false, ""}, // ligado só pra copiar a memória
		{d(24, 12, 40), d(24, 19, 58), 5, true, "quarto"},
		{d(24, 20, 0), d(25, 11, 22), 14, false, ""}, // na sala, sem o PC
		{d(25, 11, 25), d(25, 11, 28), 5, true, "sala"},
	}
	mem, rows, hours := radonSim(segs)
	now := d(25, 11, 28)
	// o cômodo foi trocado pra sala com o aparelho desconectado
	evs := []radonPlaceEvent{{d(24, 20, 0).UnixMilli(), "sala"}}
	fill, info := radonFill(mem, rows, nil, now, radonPlaceAt(rows, evs, "quarto"))
	t.Logf("%+v", info)
	var want []radonSimHour
	for _, h := range hours {
		if h.end.After(d(24, 20, 0)) {
			want = append(want, h)
		}
	}
	if len(fill) != len(want) || len(want) < 14 {
		for _, r := range fill {
			t.Log(time.UnixMilli(r.End).In(loc), r.Counts, r.Place)
		}
		t.Fatalf("%d horas, queria %d", len(fill), len(want))
	}
	for i, r := range fill {
		if r.Counts != want[i].counts || r.Place != "sala" {
			t.Fatalf("hora %d: %+v, queria %+v", i, r, want[i])
		}
		// a hora de parede só erra o tempo da mudança (2 min) e o minuto da virada
		if e := time.UnixMilli(r.End).Sub(want[i].end); e < -10*time.Minute || e > 10*time.Minute {
			t.Fatalf("hora %d termina %v fora", i, e)
		}
	}
	// sem a troca gravada (antes da 2.106.2), vale o cômodo de quando voltou
	fill2, _ := radonFill(mem, rows, nil, now, radonPlaceAt(rows, nil, "quarto"))
	for _, r := range fill2 {
		if time.UnixMilli(r.Start).After(d(24, 20, 0)) && r.Place != "sala" {
			t.Fatalf("sem troca gravada: %+v", r)
		}
	}
	// horas depois, sem o trecho de agora encaixar e sem estar ligado direto,
	// não chuta onde param as horas do buraco
	_, info3 := radonFill(mem, rows, nil, now.Add(3*time.Hour), radonPlaceAt(rows, evs, "quarto"))
	if !info3.pending {
		t.Fatalf("devia esperar: %+v", info3)
	}
}

// horas depois de reconectar, o tempo ligado gravado ao conectar diz
// quantas horas da memória são da conexão de agora
func TestRadonFillLaterWithUptime(t *testing.T) {
	loc := time.FixedZone("BRT", -3*3600)
	d := func(day, h, m int) time.Time { return time.Date(2026, 11, day, h, m, 0, 0, loc) }
	segs := []radonSimSeg{
		{d(23, 11, 4), d(24, 19, 58), 5, true, "quarto"},
		{d(24, 21, 30), d(25, 11, 22), 14, false, ""}, // 1h30 desligado antes de ligar na sala
		{d(25, 11, 25), d(25, 15, 10), 5, true, "sala"},
	}
	mem, rows, hours := radonSim(segs)
	now := d(25, 15, 10)
	conn := d(25, 11, 25)
	k := 0
	for _, h := range hours {
		if !h.end.After(conn) {
			k++
		}
	}
	ons := []radonOn{{conn.Add(5 * time.Second).UnixMilli(), k * 3600}}
	evs := []radonPlaceEvent{{d(24, 20, 0).UnixMilli(), "sala"}}
	fill, info := radonFill(mem, rows, ons, now, radonPlaceAt(rows, evs, "quarto"))
	t.Logf("%+v", info)
	var want []radonSimHour
	for _, h := range hours {
		// a hora que mistura o fim do quarto com o começo da sala fica de
		// fora: no relógio do app ela cai em cima das leituras ao vivo
		if h.end.After(d(24, 22, 30)) && !h.end.After(conn) {
			want = append(want, h)
		}
	}
	if info.pending || len(fill) != len(want) {
		t.Fatalf("%d horas, queria %d (%+v)", len(fill), len(want), info)
	}
	for i, r := range fill {
		if r.Counts != want[i].counts || r.Place != "sala" {
			t.Fatalf("hora %d: %+v, queria %+v", i, r, want[i])
		}
	}
	// sem o registro, o trecho de agora (3h45 ao vivo) se encaixa sozinho
	if fill2, _ := radonFill(mem, rows, nil, now, radonPlaceAt(rows, evs, "quarto")); len(fill2) != len(want) {
		t.Fatalf("sem registro: %d horas", len(fill2))
	}
}
