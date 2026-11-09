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

func TestSwapRoles(t *testing.T) {
	a := newTestApp(t)
	t0 := time.Date(2026, 11, 9, 10, 0, 0, 0, time.Local)

	if r := a.GetRoles(); r.Swapped || r.Personal != devRC || r.Home != "gmc" {
		t.Fatalf("normal: %+v", r)
	}
	r, err := a.swapRoles(t0, true, "  ")
	if err != nil || len(r.Swaps) != 1 || r.Swaps[0].To != 0 || r.Swaps[0].Name != "Papéis trocados" {
		t.Fatalf("trocar: %+v %v", r, err)
	}
	if r := a.rolesLocked(t0.Add(time.Hour)); !r.Swapped || r.Personal != "gmc" || r.Home != devRC {
		t.Fatalf("trocado: %+v", r)
	}
	// trocar de novo não abre outro trecho
	if r, _ := a.swapRoles(t0.Add(time.Minute), true, "x"); len(r.Swaps) != 1 {
		t.Fatalf("troca dupla: %+v", r.Swaps)
	}
	r, _ = a.swapRoles(t0.Add(90*time.Minute+30*time.Second), false, "")
	if len(r.Swaps) != 1 || r.Swaps[0].To != t0.Add(90*time.Minute).UnixMilli() {
		t.Fatalf("desfazer: %+v", r.Swaps)
	}
	// trocar e desfazer no mesmo minuto não deixa trecho
	a.swapRoles(t0.Add(3*time.Hour), true, "")
	if r, _ := a.swapRoles(t0.Add(3*time.Hour+20*time.Second), false, ""); len(r.Swaps) != 1 {
		t.Fatalf("mesmo minuto: %+v", r.Swaps)
	}

	// trecho passado: não pode cruzar com outro
	if _, err := a.SaveRoleSwap(RoleSwap{From: t0.Add(time.Hour).UnixMilli(), To: t0.Add(2 * time.Hour).UnixMilli()}); err == nil {
		t.Fatal("aceitou trecho cruzado")
	}
	r, err = a.SaveRoleSwap(RoleSwap{From: t0.Add(-5 * time.Hour).UnixMilli(), To: t0.Add(-4 * time.Hour).UnixMilli(), Name: "Feira"})
	if err != nil || len(r.Swaps) != 2 || r.Swaps[0].Name != "Feira" {
		t.Fatalf("salvar: %+v %v", r.Swaps, err)
	}

	// volta do disco
	b := newTestApp(t)
	b.dataDir = a.dataDir
	b.loadRoles()
	if len(b.swaps) != 2 || b.swaps[1].To != t0.Add(90*time.Minute).UnixMilli() {
		t.Fatalf("disco: %+v", b.swaps)
	}

	// mescla (backup / outro PC): o aberto lá fecha aqui, o repetido não entra
	c := newTestApp(t)
	c.swapRoles(t0, true, "")
	if n := c.mergeRoleSwaps(a.GetRoles().Swaps); n != 2 {
		t.Fatalf("mescla: %d", n)
	}
	if n := c.mergeRoleSwaps(a.GetRoles().Swaps); n != 0 {
		t.Fatalf("mescla repetida: %d", n)
	}
	if r := c.GetRoles(); len(r.Swaps) != 2 || r.Swaps[1].To == 0 || r.Swaps[0].ID == r.Swaps[1].ID {
		t.Fatalf("depois da mescla: %+v", r.Swaps)
	}

	if r, _ := a.DeleteRoleSwap(r.Swaps[0].ID); len(r.Swaps) != 1 {
		t.Fatalf("apagar: %+v", r.Swaps)
	}
}

func TestRolesAway(t *testing.T) {
	a := newTestApp(t)
	t0 := time.Date(2026, 11, 9, 10, 0, 0, 0, time.Local)
	at := func(m int) time.Time { return t0.Add(time.Duration(m) * time.Minute) }
	// fora de casa 10:00–11:00; papéis trocados 10:30–12:00
	if _, err := a.SavePeriod(Period{From: at(0).UnixMilli(), To: at(60).UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SaveRoleSwap(RoleSwap{From: at(30).UnixMilli(), To: at(120).UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	gmcAway, rcAway := a.awayChecker(), a.rcAwayChecker()
	for _, c := range []struct {
		m       int
		gmc, rc bool
	}{
		{-10, false, false}, // em casa, normal
		{10, true, true},    // fora, normal: os dois saíram com você
		{40, true, false},   // fora, trocado: o GMC está com você, o RadiaCode em casa
		{90, true, false},   // trocado: o GMC está fora
		{130, false, false}, // normal de novo
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
	// 4 h dos dois aparelhos: GMC a 20 CPM, RadiaCode a 0,10 µSv/h
	var pts []store.Point
	for i := 0; i < 240; i++ {
		pts = append(pts, store.Point{T: from.Add(time.Duration(i) * time.Minute), CPM: 20, Src: store.SrcDevice})
	}
	if _, err := a.store.Import(pts); err != nil {
		t.Fatal(err)
	}
	writeRc(t, a, from, 240, 0.10)
	usvG, usvR := gmc.DefaultCalibration().USv(20), 0.10

	// sem troca: igual à dose do próprio aparelho
	p, err := a.roleDose(from, to, time.Hour, "publico", "pessoal")
	if err != nil || p.Mixed || p.Device != devRC || p.MeasuredMinutes != 240 {
		t.Fatalf("sem troca: %+v %v", p, err)
	}

	// a última hora com os papéis trocados
	if _, err := a.SaveRoleSwap(RoleSwap{From: from.Add(3 * time.Hour).UnixMilli(), To: to.UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		role         string
		normal, swap float64
	}{
		{"pessoal", usvR, usvG},
		{"casa", usvG, usvR},
	} {
		d, err := a.roleDose(from, to, time.Hour, "publico", c.role)
		if err != nil {
			t.Fatal(err)
		}
		want := 180*c.normal/60 + 60*c.swap/60
		if !d.Mixed || d.Device != "misto" || d.MeasuredMinutes != 240 || d.SwapMinutes != 60 || math.Abs(d.Total-want) > 1e-9 {
			t.Errorf("%s: misto %v, %d min (%d trocados), total %v (quero %v)", c.role, d.Mixed, d.MeasuredMinutes, d.SwapMinutes, d.Total, want)
		}
		// cada minuto contra o fundo do aparelho que mediu: nada acima
		if math.Abs(d.Excess) > 1e-9 || len(d.Dose) != 4 || math.Abs(d.Dose[3]-d.Total) > 1e-9 {
			t.Errorf("%s: excesso %v, curva %v", c.role, d.Excess, d.Dose)
		}
	}
	// o GMC trocado sai do fundo da casa; o RadiaCode trocado entra
	if bg := a.homeOnly(pts); len(bg) != 180 {
		t.Errorf("fundo do GMC com %d min", len(bg))
	}
}
