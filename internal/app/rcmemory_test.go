package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"radlog/internal/radiacode"
)

// Os pontos de 96 s que vieram da memória preenchem só os minutos vazios,
// com a hora do aparelho, e vão para o log e a dose.
func TestRcRecoverOld(t *testing.T) {
	a := newTestApp(t)
	now := time.Date(2026, 11, 10, 9, 0, 0, 0, time.Local)
	a.rc.st.Since = now.UnixMilli()
	// o app gravou ao vivo das 7:00 às 7:04
	writeRcMinutes(t, a, now.Add(-2*time.Hour), 5, 0.1)

	lead := radiacode.BaseLead
	var old []radiacode.Record
	start := now.Add(-2*time.Hour - 10*time.Minute) // 6:50
	for i := 0; i < 20; i++ {                       // 20 × 96 s = 32 min
		old = append(old, radiacode.DoseRateDB{T: start.Add(time.Duration(i)*96*time.Second + lead), Count: 1200,
			CPS: 12.5, DoseRate: 0.14, DoseErr: 20, Flags: 0xB000})
	}
	old = append(old, radiacode.Rare{T: start.Add(lead), Dose: 70, Duration: 1000, TempC: 25, Battery: 99, Flags: 0x9040})
	old = append(old, radiacode.Event{T: start.Add(time.Minute + lead), ID: 7, Flags: 0x9000})
	// hora impossível: no futuro
	old = append(old, radiacode.DoseRateDB{T: now.Add(time.Hour), CPS: 1, DoseRate: 1, Flags: 0x8000})

	a.rcRecoverOld(old, now)

	pts := a.rcRange(now.Add(-3*time.Hour), now)
	var live, mem int
	for i, p := range pts {
		if i > 0 && !p.T.After(pts[i-1].T) {
			t.Fatalf("fora de ordem: %v depois de %v", p.T, pts[i-1].T)
		}
		if p.USvh == 0.1 {
			live++
		} else if p.USvh > 0.139 && p.USvh < 0.141 {
			mem++
		}
	}
	if live != 5 {
		t.Errorf("minutos ao vivo: %d (quero 5 intactos)", live)
	}
	// 6:48:24 → 7:20:24 cobre 33 minutos; 5 já tinham leitura
	if mem != 28 {
		t.Errorf("minutos da memória: %d (quero 28)", mem)
	}
	b, _ := os.ReadFile(filepath.Join(a.rcDir(), now.Format("2006-01")+".csv"))
	if !strings.Contains(string(b), ",memoria\n") {
		t.Errorf("sem a marca de origem:\n%s", b)
	}
	items := a.GetRcLog(now.Add(-3*time.Hour).UnixMilli(), now.UnixMilli(), []string{"taxa", "raro", "evento"}, 2000, 0).Items
	nm := 0
	for _, it := range items {
		if it.Src == rcMemSrc {
			nm++
		}
	}
	if nm != 22 {
		t.Errorf("log da memória: %d linhas (quero 22)", nm)
	}
	if d := a.rcDoseLog(now.Add(-3*time.Hour), now); len(d) != 1 || d[0].Dose != 70 {
		t.Errorf("dose: %+v", d)
	}
	if a.rc.mem.recs != 22 || a.rc.mem.mins != 28 {
		t.Errorf("contagem: %+v", a.rc.mem)
	}
	// o mesmo atraso de novo (outra leitura): nada repete
	a.rcRecoverOld(old, now)
	if n := len(a.rcRange(now.Add(-3*time.Hour), now)); n != len(pts) {
		t.Errorf("repetiu minutos: %d → %d", len(pts), n)
	}
	a.rcMemFlush()
	if a.rc.mem.recs != 0 {
		t.Errorf("não zerou: %+v", a.rc.mem)
	}
}

func TestRcMinutesFromDB(t *testing.T) {
	t0 := time.Date(2026, 11, 10, 3, 1, 36, 0, time.UTC)
	m := rcMinutesFromDB([]radiacode.DoseRateDB{{T: t0, CPS: 10, DoseRate: 0.1}})
	// 3:00:00 → 3:01:36: 60 s no minuto 3:00 e 36 s no 3:01
	if len(m) != 2 || m[0].sec != 60 || m[1].sec != 36 || m[0].cps/m[0].sec != 10 {
		t.Fatalf("%+v", m)
	}
}

// O buffer recente chega sem a marca de antigo: vale a hora de antes da conexão.
func TestSplitOldBeforeConnection(t *testing.T) {
	since := time.Date(2026, 11, 10, 13, 48, 43, 0, time.Local)
	lead := radiacode.BaseLead
	recs := []radiacode.Record{
		radiacode.DoseRateDB{T: since.Add(-30*time.Minute + lead), Flags: 0x1000}, // buffer recente
		radiacode.DoseRateDB{T: since.Add(-60*time.Minute + lead), Flags: 0x9000}, // memória
		radiacode.DoseRateDB{T: since.Add(2*time.Minute + lead), Flags: 0x1000},   // ao vivo
		radiacode.RealTime{T: since.Add(-time.Minute + lead), CPS: 1},             // tempo real fica
		radiacode.Rare{Dose: 1}, // sem hora: ao vivo
	}
	live, old := splitOldRecords(recs, since)
	if len(old) != 2 || len(live) != 3 {
		t.Fatalf("ao vivo %d, antigos %d", len(live), len(old))
	}
	if _, old := splitOldRecords(recs, time.Time{}); len(old) != 1 {
		t.Fatalf("sem conexão: %d antigos (quero só o marcado)", len(old))
	}
}
