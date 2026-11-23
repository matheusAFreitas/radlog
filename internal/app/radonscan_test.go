package app

import (
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"radlog/internal/gmc"
	"radlog/internal/radonscan"
	"radlog/internal/store"
)

// 2.105.0: o RadonScan tem o mesmo chip USB do GMC-800.
func TestRadonCandidatesSkipGmc(t *testing.T) {
	id := "1A86:7523"
	ports := []gmc.PortInfo{
		{Name: "COM13", IsUSB: true, USBID: id},
		{Name: "COM17", IsUSB: true, USBID: id},
		{Name: "COM3", IsUSB: true, USBID: "2341:0043"}, // outro chip
		{Name: "COM1"}, // porta de verdade, sem USB
	}
	names := func(ps []gmc.PortInfo) string {
		var s []string
		for _, p := range ps {
			s = append(s, p.Name)
		}
		return strings.Join(s, " ")
	}
	if got := names(radonCandidates(ports, "", "COM17")); got != "COM13" {
		t.Fatalf("sem a porta do GMC: %q", got)
	}
	if got := names(radonCandidates(ports, "COM17", "COM13")); got != "COM17" {
		t.Fatalf("a do RadonScan primeiro, sem a do GMC: %q", got)
	}
	// trocaram os cabos: a última do RadonScan é a salva do GMC (desconectado)
	if got := names(radonCandidates(ports, "COM17", "COM17")); got != "COM17 COM13" {
		t.Fatalf("porta conhecida dos dois: %q", got)
	}
}

// No Linux os dois aparelhos ficam com o mesmo USBID: a reconexão do GMC-800
// não pode pular para a porta do RadonScan.
func TestResolvePortTwinChips(t *testing.T) {
	id := "1A86:7523"
	two := []gmc.PortInfo{{Name: "/dev/ttyUSB0", USBID: id}, {Name: "/dev/ttyUSB1", USBID: id}}
	if p, ok := resolvePort(two, "/dev/ttyUSB1", id); p != "/dev/ttyUSB1" || !ok {
		t.Fatalf("na porta salva: %q %v", p, ok)
	}
	if p, ok := resolvePort(two, "/dev/ttyUSB5", id); p != "/dev/ttyUSB5" || !ok {
		t.Fatalf("contador fora, só o RadonScan e outro: devia ficar na salva, deu %q %v", p, ok)
	}
	// o RadonScan ocupando a porta tirada da lista: volta a seguir o contador
	one := withoutPort(two, "/dev/ttyUSB0")
	if p, _ := resolvePort(one, "/dev/ttyUSB5", id); p != "/dev/ttyUSB1" {
		t.Fatalf("sem a do RadonScan: %q", p)
	}
}

func TestRadonRowRoundTrip(t *testing.T) {
	r := RadonRow{T: time.Date(2026, 11, 23, 11, 0, 0, 0, time.Local).UnixMilli(), Place: "sala", Hour: 0.33, Day: 0.41, Week: 0.5, Long: 1.2}
	back, ok := parseRadonRow(formatRadonRow(r))
	if !ok || back != r {
		t.Fatalf("%+v %v", back, ok)
	}
	for _, bad := range []string{radonHeader, "", "x,sala,1,2,3,4", "2026-11-23T11:00:00-03:00,sala,1,2,3"} {
		if _, ok := parseRadonRow(bad); ok {
			t.Errorf("%q devia ser ignorada", bad)
		}
	}
}

func TestRadonReadingSavesOnChange(t *testing.T) {
	a := newTestApp(t)
	t0 := time.Date(2026, 11, 23, 10, 0, 0, 0, time.Local)
	avg := radonscan.Averages{Hour: 0.33, Day: 0.33, Week: 0.33, Long: 0.33}
	a.radonReading(t0, avg)
	a.radonReading(t0.Add(time.Minute), avg) // igual: não grava
	if n := len(a.radonRows()); n != 1 {
		t.Fatalf("gravou %d linhas", n)
	}
	avg.Hour = 0.4
	a.radonReading(t0.Add(2*time.Minute), avg)  // mudou: grava
	a.radonReading(t0.Add(63*time.Minute), avg) // passou 1 h: grava
	if _, err := a.SetRadonPlace("quarto"); err != nil {
		t.Fatal(err)
	}
	a.radonReading(t0.Add(64*time.Minute), avg) // mudou o cômodo: grava
	rows := a.radonRows()
	if len(rows) != 4 || rows[3].Place != "quarto" || rows[1].Hour != 0.4 {
		t.Fatalf("%+v", rows)
	}
	b, _ := os.ReadFile(a.radonPath())
	if !strings.HasPrefix(string(b), radonHeader+"\n") {
		t.Fatal("sem cabeçalho")
	}
	if _, err := a.SetRadonPlace("cozinha"); err == nil {
		t.Fatal("cômodo fora da lista devia dar erro")
	}
	st := a.GetRadonScan()
	if !st.Seen || st.Last == nil || st.Last.Place != "quarto" || st.Place != "quarto" {
		t.Fatalf("%+v", st)
	}
}

func TestRadonPlaceStats(t *testing.T) {
	a := newTestApp(t)
	t0 := time.Now().Add(-48 * time.Hour).Truncate(time.Hour)
	var rows []RadonRow
	for h := 0; h < 24; h++ { // 24 h na sala a 0,5 pCi/L
		rows = append(rows, RadonRow{T: t0.Add(time.Duration(h) * time.Hour).UnixMilli(), Place: "sala", Hour: 0.5})
	}
	for h := 24; h < 36; h++ { // 12 h no quarto a 0,2
		rows = append(rows, RadonRow{T: t0.Add(time.Duration(h) * time.Hour).UnixMilli(), Place: "quarto", Hour: 0.2})
	}
	// GMC-800: 900 CPM no tempo da sala, 772 no do quarto
	var pts []store.Point
	for m := 0; m < 36*60; m++ {
		tm := t0.Add(time.Duration(m) * time.Minute)
		cpm := 900.0
		if m >= 24*60 {
			cpm = 772
		}
		pts = append(pts, store.Point{T: tm, CPM: cpm, Src: store.SrcLive})
	}
	if _, err := a.store.Import(pts); err != nil {
		t.Fatal(err)
	}
	// o último ponto vale só até radonGapMax depois dele
	out := radonPlaceStats(rows, t0.Add(100*time.Hour), a.radonGmcMean())
	if len(out) != 2 {
		t.Fatalf("%+v", out)
	}
	s, q := out[0], out[1]
	if s.Place != "sala" || s.Hours != 24 || math.Abs(s.PCiL-0.5) > 1e-9 || math.Abs(s.GmcCPM-900) > 1e-9 {
		t.Fatalf("sala %+v", s)
	}
	if q.Place != "quarto" || q.Hours != 13 || math.Abs(q.PCiL-0.2) > 1e-9 {
		t.Fatalf("quarto %+v", q)
	}
	// o GMC-800 só tem minutos até 36 h: a última hora do quarto fica sem
	if q.GmcMin != 12*60 || math.Abs(q.GmcCPM-772) > 1e-9 {
		t.Fatalf("quarto GMC %+v", q)
	}
	// linhas sem cômodo não entram
	if len(radonPlaceStats([]RadonRow{{T: t0.UnixMilli(), Hour: 1}}, t0.Add(time.Hour), nil)) != 0 {
		t.Fatal("sem cômodo não entra")
	}
}
