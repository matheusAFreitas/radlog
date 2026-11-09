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
	if _, err := a.setWhere(t0, "gmc", "rua"); err == nil {
		t.Fatal("aceitou local inválido")
	}
	// leva o GMC também: os dois com você, ninguém em casa
	if _, err := a.setWhere(t0, "gmc", WhereMe); err != nil {
		t.Fatal(err)
	}
	if n := w(t0.Add(time.Hour)); n.Normal || n.GMC != WhereMe || n.Personal != devRC || n.Home != "" {
		t.Fatalf("os dois com você: %+v", n)
	}
	// deixa o RadiaCode em casa: papéis trocados
	a.setWhere(t0.Add(time.Hour), devRC, WhereHome)
	if n := w(t0.Add(2 * time.Hour)); n.Personal != "gmc" || n.Home != devRC {
		t.Fatalf("trocados: %+v", n)
	}
	// GMC volta pra casa: os dois em casa (intercalibração)
	a.setWhere(t0.Add(2*time.Hour), "gmc", WhereHome)
	if n := w(t0.Add(3 * time.Hour)); n.Personal != "" || n.Home != "gmc" || len(n.Stays) != 2 {
		t.Fatalf("os dois em casa: %+v", n)
	}
	// mesmo local de novo não abre trecho; mudar e voltar no mesmo minuto não deixa trecho
	a.setWhere(t0.Add(3*time.Hour), "gmc", WhereHome)
	a.setWhere(t0.Add(3*time.Hour), "gmc", WhereMe)
	if r, _ := a.setWhere(t0.Add(3*time.Hour+30*time.Second), "gmc", WhereHome); len(r.Stays) != 2 {
		t.Fatalf("mesmo minuto: %+v", r.Stays)
	}
	if s := a.GetWhereabouts().Stays; s[0].Device != "gmc" || s[0].To != t0.Add(2*time.Hour).UnixMilli() || s[1].Device != devRC || s[1].To != 0 {
		t.Fatalf("trechos: %+v", s)
	}

	// trecho passado: não pode cruzar com outro do mesmo aparelho
	if _, err := a.SaveStay(Stay{Device: "gmc", Where: WhereMe, From: t0.Add(time.Hour).UnixMilli(), To: t0.Add(5 * time.Hour).UnixMilli()}); err == nil {
		t.Fatal("aceitou trecho cruzado")
	}
	r, err := a.SaveStay(Stay{Device: "gmc", Where: WhereMe, From: t0.Add(-5 * time.Hour).UnixMilli(), To: t0.Add(-4 * time.Hour).UnixMilli(), Name: " Feira "})
	if err != nil || len(r.Stays) != 3 || r.Stays[0].Name != "Feira" {
		t.Fatalf("salvar: %+v %v", r.Stays, err)
	}

	// volta do disco
	b := newTestApp(t)
	b.dataDir = a.dataDir
	b.loadStays()
	if len(b.stays) != 3 {
		t.Fatalf("disco: %+v", b.stays)
	}

	// mescla (backup / outro PC): o aberto aqui fecha, o repetido não entra
	c := newTestApp(t)
	c.setWhere(t0, "gmc", WhereMe)
	if n := c.mergeStays(a.GetWhereabouts().Stays); n != 3 {
		t.Fatalf("mescla: %d", n)
	}
	if n := c.mergeStays(a.GetWhereabouts().Stays); n != 0 {
		t.Fatalf("mescla repetida: %d", n)
	}
	if s := c.GetWhereabouts().Stays; len(s) != 3 || s[1].To == 0 {
		t.Fatalf("depois da mescla: %+v", s)
	}

	if r, _ := a.DeleteStay(r.Stays[0].ID); len(r.Stays) != 2 {
		t.Fatalf("apagar: %+v", r.Stays)
	}
}

func TestWhereAway(t *testing.T) {
	a := newTestApp(t)
	t0 := time.Date(2026, 11, 9, 10, 0, 0, 0, time.Local)
	at := func(m int) time.Time { return t0.Add(time.Duration(m) * time.Minute) }
	// você fora de casa 10:00–11:00 e 12:00–13:00; RadiaCode marcado em casa 10:30–12:30
	for _, p := range []Period{{From: at(0).UnixMilli(), To: at(60).UnixMilli()}, {From: at(120).UnixMilli(), To: at(180).UnixMilli()}} {
		if _, err := a.SavePeriod(p); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.SaveStay(Stay{Device: devRC, Where: WhereHome, From: at(30).UnixMilli(), To: at(150).UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	// GMC marcado com você 11:00–12:00 (você em casa: ainda é a casa)
	if _, err := a.SaveStay(Stay{Device: "gmc", Where: WhereMe, From: at(60).UnixMilli(), To: at(120).UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	gmcAway, rcAway := a.awayChecker(), a.rcAwayChecker()
	for _, c := range []struct {
		m       int
		gmc, rc bool
	}{
		{-10, false, false}, // você em casa
		{10, true, true},    // você fora, nada marcado: os dois fora, como sempre foi
		{40, true, false},   // você fora, RadiaCode marcado em casa
		{90, false, false},  // você em casa com o GMC
		{130, true, false},  // você fora, RadiaCode ainda em casa
		{170, true, true},   // você fora, nada marcado
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

	// hora 1: os dois com você; hora 2: trocados; hora 3: os dois em casa
	for _, s := range []Stay{
		{Device: "gmc", Where: WhereMe, From: h(1), To: h(3)},
		{Device: devRC, Where: WhereHome, From: h(2), To: h(4)},
	} {
		if _, err := a.SaveStay(s); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct {
		role       string
		mins, swap int
		total      float64
	}{
		{"pessoal", 180, 60, (120*r + 60*g) / 60}, // RC, RC, GMC, ninguém
		{"casa", 180, 60, (60*g + 60*r + 60*g) / 60},
	} {
		d, err := a.roleDose(from, to, time.Hour, "publico", c.role)
		if err != nil {
			t.Fatal(err)
		}
		if !d.Mixed || d.Device != "misto" || d.MeasuredMinutes != c.mins || d.SwapMinutes != c.swap || math.Abs(d.Total-c.total) > 1e-9 {
			t.Errorf("%s: misto %v, %d min (%d do outro), total %v (quero %v)", c.role, d.Mixed, d.MeasuredMinutes, d.SwapMinutes, d.Total, c.total)
		}
		// cada minuto contra o fundo do aparelho que mediu: nada acima
		if math.Abs(d.Excess) > 1e-9 || len(d.Dose) != 4 || math.Abs(d.Dose[3]-d.Total) > 1e-9 {
			t.Errorf("%s: excesso %v, curva %v", c.role, d.Excess, d.Dose)
		}
	}
}
