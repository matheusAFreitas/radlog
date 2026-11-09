package app

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"radlog/internal/gmc"
	"radlog/internal/store"
)

func TestSetWhere(t *testing.T) {
	a := newTestApp(t)
	t0 := time.Date(2026, 11, 9, 10, 0, 0, 0, time.Local)
	w := func(at time.Time) Whereabouts { a.mu.Lock(); defer a.mu.Unlock(); return a.whereaboutsLocked(at) }

	if n := w(t0); !n.Normal || n.GMC != WhereHome || n.RC != WhereMe || n.Personal != devRC || n.Home != "gmc" {
		t.Fatalf("normal: %+v", n)
	}
	if _, err := a.setWhere(t0, "rua", WhereMe, ""); err == nil {
		t.Fatal("aceitou local inválido")
	}
	// leva os dois: ninguém em casa
	if _, err := a.setWhere(t0, WhereMe, WhereMe, ""); err != nil {
		t.Fatal(err)
	}
	if n := w(t0.Add(time.Hour)); n.Normal || n.Personal != devRC || n.Home != "" || n.Open == nil || n.Open.Name != "Levei os dois" || n.Since != t0.UnixMilli() {
		t.Fatalf("os dois com você: %+v", n)
	}
	// deixa o RadiaCode em casa: trocados
	a.setWhere(t0.Add(time.Hour), WhereMe, WhereHome, " Feira ")
	if n := w(t0.Add(2 * time.Hour)); n.Personal != "gmc" || n.Home != devRC || n.Open.Name != "Feira" {
		t.Fatalf("trocados: %+v", n)
	}
	// GMC volta: os dois em casa
	a.setWhere(t0.Add(2*time.Hour), WhereHome, WhereHome, "")
	if n := w(t0.Add(3 * time.Hour)); n.Personal != "" || n.Home != "gmc" {
		t.Fatalf("os dois em casa: %+v", n)
	}
	// o mesmo de novo não abre trecho; mudar e voltar no mesmo minuto não deixa trecho
	a.setWhere(t0.Add(3*time.Hour), WhereHome, WhereHome, "")
	a.setWhere(t0.Add(3*time.Hour), WhereMe, WhereHome, "")
	a.setWhere(t0.Add(3*time.Hour+30*time.Second), WhereHome, WhereHome, "")
	// volta ao normal: fecha o aberto
	a.setWhere(t0.Add(4*time.Hour), WhereHome, WhereMe, "")
	ps := a.GetPeriods()
	if len(ps) != 4 || ps[0].To != t0.Add(time.Hour).UnixMilli() || ps[2].From != t0.Add(2*time.Hour).UnixMilli() || ps[2].To != t0.Add(3*time.Hour).UnixMilli() {
		t.Fatalf("trechos: %+v", ps)
	}
	if ps[3].From != t0.Add(3*time.Hour).UnixMilli() || ps[3].To != t0.Add(4*time.Hour).UnixMilli() || ps[3].GMC != WhereHome {
		t.Fatalf("último trecho: %+v", ps[3])
	}
	if n := w(t0.Add(5 * time.Hour)); !n.Normal || n.Open != nil {
		t.Fatalf("de volta: %+v", n)
	}

	// trecho passado: não pode ser o normal nem cruzar com outro
	if _, err := a.SavePeriod(Period{From: t0.Add(-5 * time.Hour).UnixMilli(), To: t0.Add(-4 * time.Hour).UnixMilli(), GMC: WhereHome, RC: WhereMe}); err == nil {
		t.Fatal("aceitou o normal")
	}
	if _, err := a.SavePeriod(Period{From: t0.Add(30 * time.Minute).UnixMilli(), To: t0.Add(90 * time.Minute).UnixMilli()}); err == nil {
		t.Fatal("aceitou trecho cruzado")
	}
	if _, err := a.SavePeriod(Period{From: t0.Add(-5 * time.Hour).UnixMilli()}); err == nil {
		t.Fatal("aceitou trecho sem fim")
	}

	// mescla (backup / outro PC): o aberto aqui fecha, o repetido não entra
	c := newTestApp(t)
	c.setWhere(t0, WhereMe, WhereMe, "")
	if n := c.mergePeriods(a.GetPeriods()); n != 4 {
		t.Fatalf("mescla: %d", n)
	}
	if n := c.mergePeriods(a.GetPeriods()); n != 0 {
		t.Fatalf("mescla repetida: %d", n)
	}
	if ps := c.GetPeriods(); len(ps) != 4 || ps[0].To == 0 {
		t.Fatalf("depois da mescla: %+v", ps)
	}
}

func TestWhereAway(t *testing.T) {
	a := newTestApp(t)
	t0 := time.Date(2026, 11, 9, 10, 0, 0, 0, time.Local)
	at := func(m int) time.Time { return t0.Add(time.Duration(m) * time.Minute) }
	for _, p := range []Period{
		{From: at(0).UnixMilli(), To: at(60).UnixMilli()},                                   // antigo: os dois com você
		{From: at(60).UnixMilli(), To: at(120).UnixMilli(), GMC: WhereMe, RC: WhereHome},    // trocados
		{From: at(120).UnixMilli(), To: at(180).UnixMilli(), GMC: WhereHome, RC: WhereHome}, // os dois em casa
	} {
		if _, err := a.SavePeriod(p); err != nil {
			t.Fatal(err)
		}
	}
	gmcAway, rcAway := a.awayChecker(), a.rcAwayChecker()
	for _, c := range []struct {
		m       int
		gmc, rc bool
	}{
		{-10, false, false}, // normal
		{10, true, true},    // os dois com você, como os períodos de sempre
		{70, true, false},   // trocados
		{130, false, false}, // os dois em casa
		{190, false, false}, // normal
	} {
		if g, r := gmcAway(at(c.m)), rcAway(at(c.m)); g != c.gmc || r != c.rc {
			t.Errorf("%d min: GMC fora %v (quero %v), RadiaCode fora %v (quero %v)", c.m, g, c.gmc, r, c.rc)
		}
	}
}

// writeRc grava minutos do RadiaCode como o app grava.
func writeRc(t *testing.T, a *App, from time.Time, n int, usv float64) {
	t.Helper()
	files := map[string]*strings.Builder{}
	for i := 0; i < n; i++ {
		m := from.Add(time.Duration(i) * time.Minute)
		k := m.Format("2006-01")
		if files[k] == nil {
			files[k] = &strings.Builder{}
			files[k].WriteString(rcHeader + "\n")
		}
		fmt.Fprintf(files[k], "%s,%g,%g,60\n", m.Format(time.RFC3339), usv*120, usv)
	}
	_ = os.MkdirAll(a.rcDir(), 0o755)
	for k, b := range files {
		if err := os.WriteFile(filepath.Join(a.rcDir(), k+".csv"), []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRoleDose(t *testing.T) {
	a := newTestApp(t)
	from := time.Now().Add(-48 * time.Hour).Truncate(time.Hour)
	to := from.Add(4 * time.Hour)
	h := func(n int) int64 { return from.Add(time.Duration(n) * time.Hour).UnixMilli() }
	// 4 h dos dois aparelhos: GMC a 20 CPM, RadiaCode a 0,10 µSv/h
	var pts []store.Point
	for i := 0; i < 240; i++ {
		pts = append(pts, store.Point{T: from.Add(time.Duration(i) * time.Minute), CPM: 20, Src: store.SrcDevice})
	}
	if _, err := a.store.Import(pts); err != nil {
		t.Fatal(err)
	}
	writeRc(t, a, from, 240, 0.10)
	g, r := gmc.DefaultCalibration().USv(20), 0.10

	// nada marcado: igual à dose do próprio aparelho
	p, err := a.roleDose(from, to, time.Hour, "publico", "pessoal")
	if err != nil || p.Mixed || p.Device != devRC || p.MeasuredMinutes != 240 {
		t.Fatalf("normal: %+v %v", p, err)
	}

	// hora 1: os dois com você (o RadiaCode falha 30 min); hora 2: trocados; hora 3: os dois em casa
	for _, p := range []Period{
		{From: h(1), To: h(2)},
		{From: h(2), To: h(3), GMC: WhereMe, RC: WhereHome},
		{From: h(3), To: h(4), GMC: WhereHome, RC: WhereHome},
	} {
		if _, err := a.SavePeriod(p); err != nil {
			t.Fatal(err)
		}
	}
	dropRc(t, a, from.Add(60*time.Minute), from.Add(90*time.Minute))
	for _, c := range []struct {
		role            string
		mins, swap, gap int
		total           float64
	}{
		{"pessoal", 180, 90, 60, (60*r + 30*g + 30*r + 60*g) / 60}, // RC; RC (GMC nas falhas); GMC; ninguém
		{"casa", 180, 60, 60, (60*g + 60*r + 60*g) / 60},           // GMC; ninguém; RC; GMC
	} {
		d, err := a.roleDose(from, to, time.Hour, "publico", c.role)
		if err != nil {
			t.Fatal(err)
		}
		if !d.Mixed || d.Device != "misto" || d.MeasuredMinutes != c.mins || d.SwapMinutes != c.swap || d.GapMinutes != c.gap || math.Abs(d.Total-c.total) > 1e-9 {
			t.Errorf("%s: misto %v, %d min (%d do outro, %d sem ninguém), total %v (quero %v)", c.role, d.Mixed, d.MeasuredMinutes, d.SwapMinutes, d.GapMinutes, d.Total, c.total)
		}
		// cada minuto contra o fundo do aparelho que mediu: nada acima
		if math.Abs(d.Excess) > 1e-9 || len(d.Dose) != 4 || math.Abs(d.Dose[3]-d.Total) > 1e-9 {
			t.Errorf("%s: excesso %v, curva %v", c.role, d.Excess, d.Dose)
		}
	}
}

// dropRc apaga os minutos do RadiaCode em [from, to) (o aparelho falhou).
func dropRc(t *testing.T, a *App, from, to time.Time) {
	t.Helper()
	path := filepath.Join(a.rcDir(), from.Format("2006-01")+".csv")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var keep []string
	for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		ts, _, _ := strings.Cut(l, ",")
		if m, err := time.Parse(time.RFC3339, ts); err == nil && !m.Before(from) && m.Before(to) {
			continue
		}
		keep = append(keep, l)
	}
	if err := os.WriteFile(path, []byte(strings.Join(keep, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
