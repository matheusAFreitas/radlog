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
	// os dois em casa: você também estava, a dose pessoal usa a leitura de casa
	if n := w(t0.Add(3 * time.Hour)); n.Personal != "gmc" || n.Home != "gmc" || n.NotHome {
		t.Fatalf("os dois em casa: %+v", n)
	}
	// "eu não estava em casa": sem dosímetro
	op := *w(t0.Add(3 * time.Hour)).Open
	op.NotHome = true
	if _, err := a.SavePeriod(op); err != nil {
		t.Fatal(err)
	}
	if n := w(t0.Add(3 * time.Hour)); n.Personal != "" || n.Home != "gmc" || !n.NotHome {
		t.Fatalf("os dois em casa, eu fora: %+v", n)
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
	var ids []int64
	for _, p := range []Period{
		{From: h(1), To: h(2)},
		{From: h(2), To: h(3), GMC: WhereMe, RC: WhereHome},
		{From: h(3), To: h(4), GMC: WhereHome, RC: WhereHome},
	} {
		q, err := a.SavePeriod(p)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, q.ID)
	}
	dropRc(t, a, from.Add(60*time.Minute), from.Add(90*time.Minute))
	cases := []struct {
		role            string
		mins, swap, gap int
		total           float64
		notHome         bool
	}{
		{"pessoal", 240, 150, 0, (60*r + 30*g + 30*r + 60*g + 60*g) / 60, false}, // RC; RC (GMC nas falhas); GMC; leitura de casa
		{"casa", 180, 60, 60, (60*g + 60*r + 60*g) / 60, false},                  // GMC; ninguém; RC; GMC
		{"pessoal", 180, 90, 60, (60*r + 30*g + 30*r + 60*g) / 60, true},         // ...; eu não estava em casa: ninguém
	}
	for _, c := range cases {
		if c.notHome {
			if _, err := a.SavePeriod(Period{ID: ids[2], From: h(3), To: h(4), GMC: WhereHome, RC: WhereHome, NotHome: true}); err != nil {
				t.Fatal(err)
			}
		}
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

func TestWhereOff(t *testing.T) {
	a := newTestApp(t)
	from := time.Now().Add(-48 * time.Hour).Truncate(time.Hour)
	to := from.Add(4 * time.Hour)
	h := func(n int) int64 { return from.Add(time.Duration(n) * time.Hour).UnixMilli() }
	var pts []store.Point
	for i := 0; i < 240; i++ {
		pts = append(pts, store.Point{T: from.Add(time.Duration(i) * time.Minute), CPM: 20, Src: store.SrcDevice})
	}
	if _, err := a.store.Import(pts); err != nil {
		t.Fatal(err)
	}
	writeRc(t, a, from, 240, 0.10)
	// hora 0: GMC fora do cálculo; hora 1: RadiaCode fora do cálculo;
	// hora 2: normal, mas o RadiaCode não mandou nada; hora 3: normal
	for _, p := range []Period{
		{From: h(0), To: h(1), GMC: WhereOff, RC: WhereMe},
		{From: h(1), To: h(2), GMC: WhereHome, RC: WhereOff},
	} {
		got, err := a.SavePeriod(p)
		if err != nil {
			t.Fatal(err)
		}
		if p.GMC == WhereOff && got.Name != "GMC-800 fora do cálculo" {
			t.Fatalf("nome: %q", got.Name)
		}
	}
	dropRc(t, a, from.Add(2*time.Hour), from.Add(3*time.Hour))

	gmcAway, rcAway := a.awayChecker(), a.rcAwayChecker()
	at := func(n int) time.Time { return from.Add(time.Duration(n)*time.Hour + 30*time.Minute) }
	if !gmcAway(at(0)) || rcAway(at(0)) || gmcAway(at(1)) || !rcAway(at(1)) {
		t.Fatalf("fundo: GMC %v/%v RadiaCode %v/%v", gmcAway(at(0)), gmcAway(at(1)), rcAway(at(0)), rcAway(at(1)))
	}
	g, r := gmc.DefaultCalibration().USv(20), 0.10
	for _, c := range []struct {
		role              string
		mins, gap, noData int
		total             float64
	}{
		{"pessoal", 120, 60, 60, (60*r + 60*r) / 60}, // RC; ninguém; sem dado; RC
		{"casa", 180, 60, 0, (60*g + 60*g + 60*g) / 60},
	} {
		d, err := a.roleDose(from, to, time.Hour, "publico", c.role)
		if err != nil {
			t.Fatal(err)
		}
		if d.MeasuredMinutes != c.mins || d.GapMinutes != c.gap || d.NoDataMinutes != c.noData || math.Abs(d.Total-c.total) > 1e-9 {
			t.Errorf("%s: %d min, %d sem ninguém, %d sem dado, total %v (quero %d, %d, %d, %v)", c.role, d.MeasuredMinutes, d.GapMinutes, d.NoDataMinutes, d.Total, c.mins, c.gap, c.noData, c.total)
		}
	}
	if _, err := a.SavePeriod(Period{From: h(5), To: h(6), GMC: "quebrado"}); err == nil {
		t.Fatal("aceitou local inválido")
	}
}

func TestDevLive(t *testing.T) {
	a := newTestApp(t)
	now := time.Now().Truncate(time.Minute)
	if _, err := a.store.Import([]store.Point{{T: now.Add(-time.Minute), CPM: 20, Src: store.SrcLive}}); err != nil {
		t.Fatal(err)
	}
	writeRc(t, a, now.Add(-2*time.Hour), 10, 0.1)
	w := a.GetWhereabouts()
	if g := w.Live["gmc"]; g.State != "lendo" || g.Connected {
		t.Errorf("GMC: %+v", g)
	}
	if r := w.Live[devRC]; r.State != "sem dado" || r.Last == 0 || !r.CountsOffline {
		t.Errorf("RadiaCode: %+v", r)
	}
}

func TestSetWhereAt(t *testing.T) {
	a := newTestApp(t)
	now := time.Now().Truncate(time.Minute)
	// os dois com você desde 1 h atrás; o GMC quebrou há 2 h: o novo trecho
	// começa logo depois do aberto, que fecha
	if _, err := a.setWhere(now.Add(-time.Hour), WhereMe, WhereMe, ""); err != nil {
		t.Fatal(err)
	}
	w, err := a.SetWhereAt(WhereOff, WhereMe, "quebrado", now.Add(-2*time.Hour).UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	ps := a.GetPeriods()
	if len(ps) != 2 || ps[0].To != now.Add(-59*time.Minute).UnixMilli() || ps[1].From != ps[0].To || ps[1].GMC != WhereOff || ps[1].Name != "quebrado" || w.GMC != WhereOff {
		t.Fatalf("trechos: %+v / %+v", ps, w)
	}
	// não volta para dentro de um trecho fechado
	if _, err := a.SetWhereAt(WhereHome, WhereMe, "", now.Add(-90*time.Minute).UnixMilli()); err == nil {
		t.Fatal("aceitou começo dentro de outro trecho")
	}
}

func TestDeviceMarks(t *testing.T) {
	a := newTestApp(t)
	from := time.Now().Add(-48 * time.Hour).Truncate(time.Hour)
	var pts []store.Point
	for i := 0; i < 60; i++ {
		if i >= 20 && i < 30 { // 10 min sem dado
			continue
		}
		p := store.Point{T: from.Add(time.Duration(i) * time.Minute), CPM: 20, Src: store.SrcDevice}
		switch {
		case i >= 40 && i < 43:
			p.CPM, p.Has, p.Obs = 9000, true, store.ObsSaturation
		case i >= 50 && i < 53:
			p.CPM = 0
		case i == 45:
			p.CPM = 0 // um minuto em 0 sozinho não conta
		}
		pts = append(pts, p)
	}
	if _, err := a.store.Import(pts); err != nil {
		t.Fatal(err)
	}
	writeRc(t, a, from, 60, 0.1)
	ms, err := a.GetDeviceMarks(from.UnixMilli(), from.Add(time.Hour).UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	want := []DevMark{
		{"gmc", "sem dado", from.Add(20 * time.Minute).UnixMilli(), from.Add(30 * time.Minute).UnixMilli()},
		{"gmc", "saturando", from.Add(40 * time.Minute).UnixMilli(), from.Add(43 * time.Minute).UnixMilli()},
		{"gmc", "sem contagens", from.Add(50 * time.Minute).UnixMilli(), from.Add(53 * time.Minute).UnixMilli()},
	}
	if fmt.Sprint(ms) != fmt.Sprint(want) {
		t.Fatalf("marcas:\n%v\nquero\n%v", ms, want)
	}
	// a dose da casa diz que 3 min foram saturados (≥)
	if d, _ := a.roleDose(from, from.Add(time.Hour), time.Hour, "publico", "casa"); d.SatMinutes != 3 {
		t.Fatalf("saturados: %d", d.SatMinutes)
	}
}
