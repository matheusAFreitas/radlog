package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// rcWriteTest grava minutos do Radiacode (src "" = ao vivo, "memoria" = da memória).
func rcWriteTest(t *testing.T, a *App, from time.Time, mins int, usv float64, src string) {
	t.Helper()
	os.MkdirAll(a.rcDir(), 0o755)
	path := filepath.Join(a.rcDir(), from.Format("2006-01")+".csv")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if st, _ := f.Stat(); st.Size() == 0 {
		fmt.Fprintln(f, rcHeader)
	}
	for i := 0; i < mins; i++ {
		line := fmt.Sprintf("%s,%.3f,%.5f,%d", from.Add(time.Duration(i)*time.Minute).Format(time.RFC3339), usv*60, usv, 60)
		if src != "" {
			line += "," + src
		}
		fmt.Fprintln(f, line)
	}
}

func TestRcAwayAskFlow(t *testing.T) {
	a := newTestApp(t)
	t0 := time.Now().Add(-6 * time.Hour).Truncate(time.Minute)
	rcWriteTest(t, a, t0, 60, 0.10, "")
	gapFrom := t0.Add(time.Hour)
	back := gapFrom.Add(2 * time.Hour)
	rcWriteTest(t, a, gapFrom, 120, 0.14, rcMemSrc)

	if got := a.rcLastMinuteBefore(gapFrom); !got.Equal(gapFrom) {
		t.Fatalf("último minuto: %v, quer %v", got, gapFrom)
	}
	if n := a.rcLiveMinutes(t0, back); n != 60 {
		t.Fatalf("minutos ao vivo: %d", n)
	}

	// curto demais: não pergunta
	a.rcAwayDetect(gapFrom, gapFrom.Add(10*time.Minute))
	if l := a.GetRcAwayAsks(); len(l) != 0 {
		t.Fatalf("perguntou um sumiço de 10 min: %+v", l)
	}
	a.rcAwayDetect(gapFrom, back)
	a.rcAwayDetect(gapFrom, back) // a mesma de novo não duplica
	l := a.GetRcAwayAsks()
	if len(l) != 1 || l[0].MemMinutes != 120 || l[0].AvgUSv < 0.139 || l[0].AvgUSv > 0.141 || l[0].MaxUSv < 0.139 {
		t.Fatalf("pergunta: %+v", l)
	}
	if d := l[0].DoseUSv; d < 0.27 || d > 0.29 {
		t.Fatalf("dose do trecho: %v", d)
	}
	// a pergunta sobrevive a reabrir o app
	b := NewApp()
	b.dataDir, b.settings = a.dataDir, defaultSettings()
	if l2 := b.GetRcAwayAsks(); len(l2) != 1 || l2[0].ID != l[0].ID {
		t.Fatalf("depois de reabrir: %+v", l2)
	}

	// "comigo, fora de casa": trecho GMC em casa, Radiacode comigo, eu fora
	away := a.rcAwayChecker()
	if away(gapFrom.Add(time.Hour)) {
		t.Fatal("fora antes de responder")
	}
	p, err := a.AnswerRcAway(l[0].ID, "fora", 0, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if p.GMC != WhereHome || p.RC != WhereMe || !p.NotHome || p.Name != periodNameOut || p.From != gapFrom.UnixMilli() || p.To != back.UnixMilli() {
		t.Fatalf("trecho: %+v", p)
	}
	if !a.rcAwayChecker()(gapFrom.Add(time.Hour)) {
		t.Fatal("RC102 não saiu do fundo da casa")
	}
	if a.awayChecker()(gapFrom.Add(time.Hour)) {
		t.Fatal("o GMC ficou em casa")
	}
	pers, _, home, _ := a.rolesChecker()(gapFrom.Add(time.Hour))
	if pers != devRC || home != "gmc" {
		t.Fatalf("papéis: %s %s", pers, home)
	}
	if len(a.GetRcAwayAsks()) != 0 {
		t.Fatal("a pergunta continuou")
	}
	if _, err := a.AnswerRcAway(l[0].ID, "casa", 0, 0, ""); err == nil {
		t.Fatal("respondeu duas vezes")
	}

	// desfazer: o trecho some e a pergunta volta
	if err := a.UndoRcAway(l[0].RcAwayAsk, p.ID); err != nil {
		t.Fatal(err)
	}
	if len(a.GetPeriods()) != 0 || len(a.GetRcAwayAsks()) != 1 {
		t.Fatalf("desfazer: %+v %+v", a.GetPeriods(), a.GetRcAwayAsks())
	}
	// "em casa": só tira a pergunta
	if p, err := a.AnswerRcAway(l[0].ID, "casa", 0, 0, ""); err != nil || p.ID != 0 || len(a.GetPeriods()) != 0 || len(a.GetRcAwayAsks()) != 0 {
		t.Fatalf("em casa: %+v %v", p, err)
	}

	// trecho já marcado: não pergunta
	if _, err := a.SavePeriod(Period{From: gapFrom.UnixMilli(), To: back.UnixMilli(), GMC: WhereMe, RC: WhereMe}); err != nil {
		t.Fatal(err)
	}
	a.rcAwayDetect(gapFrom, back)
	if len(a.GetRcAwayAsks()) != 0 {
		t.Fatal("perguntou um trecho já marcado")
	}
}

func TestRcAwayPeriodRules(t *testing.T) {
	a := newTestApp(t)
	t0 := time.Date(2026, 11, 9, 10, 0, 0, 0, time.Local).UnixMilli()
	// sem "eu não estava em casa", GMC em casa e Radiacode comigo continua sendo o normal
	if _, err := a.SavePeriod(Period{From: t0, To: t0 + 3600_000, GMC: WhereHome, RC: WhereMe}); err == nil || !strings.Contains(err.Error(), "normal") {
		t.Fatalf("aceitou o normal: %v", err)
	}
	p, err := a.SavePeriod(Period{From: t0, To: t0 + 3600_000, GMC: WhereHome, RC: WhereMe, NotHome: true, Name: " Feira "})
	if err != nil || p.Name != "Feira" || !p.NotHome {
		t.Fatalf("saí com o RC102: %+v %v", p, err)
	}
	// "eu não estava em casa" não vale com o GMC com você
	q, err := a.SavePeriod(Period{From: t0 + 7200_000, To: t0 + 10800_000, GMC: WhereMe, RC: WhereMe, NotHome: true})
	if err != nil || q.NotHome {
		t.Fatalf("NotHome com os dois comigo: %+v %v", q, err)
	}
}

func TestRcAwayHint(t *testing.T) {
	for _, c := range []struct {
		r, n float64
		want string
	}{{28, 28, "casa"}, {29.5, 28, "casa"}, {41, 28, "fora"}, {20, 28, "fora"}, {31.5, 28, ""}, {0, 28, ""}, {28, 0, ""}} {
		if got := rcAwayHint(c.r, c.n); got != c.want {
			t.Errorf("rcAwayHint(%v, %v) = %q, quer %q", c.r, c.n, got, c.want)
		}
	}
	if rcAwayGap(time.Time{}, time.Now(), time.Minute) {
		t.Error("sem último minuto não pergunta")
	}
	now := time.Now()
	if !rcAwayGap(now.Add(-21*24*time.Hour), now, time.Minute) {
		t.Error("viagem de 3 semanas deveria perguntar")
	}
	if rcAwayGap(now.Add(-41*24*time.Hour), now, time.Minute) {
		t.Error("mais que a memória do aparelho (≈ 40 dias) não pergunta")
	}
	if !rcAwayGap(now.Add(-20*time.Minute), now, 15*time.Minute) {
		t.Error("20 min com mínimo de 15 deveria perguntar")
	}
}
