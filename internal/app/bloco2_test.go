package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"radlog/internal/gmc"
	"radlog/internal/store"
)

// D1: toda gravação do app passa pelo writeFileAtomic (atomic.go).
func TestNoBareWriteFile(t *testing.T) {
	bare := regexp.MustCompile(`\bos\.(WriteFile|Rename)\(`)
	allowed := map[string]bool{ // trocas de nome que não são gravação
		"applog.go":   true, // log-anterior.txt
		"migrate.go":  true, // mudança de pasta
		"rcconfig.go": true, // desfazer
		"update.go":   true, // troca do .exe
		"atomic.go":   true, // o próprio writeFileAtomic e a cópia .ilegivel
	}
	files, _ := filepath.Glob("*.go")
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, _ := os.ReadFile(f)
		for i, line := range strings.Split(string(b), "\n") {
			if bare.MatchString(line) && !allowed[f] {
				t.Errorf("%s:%d: %s fora do writeFileAtomic", f, i+1, strings.TrimSpace(line))
			}
		}
	}
}

// B7: alertas não somem quando os eventos comuns passam do limite.
func TestAlertsKeptPastLimit(t *testing.T) {
	dir := t.TempDir()
	var b strings.Builder
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)
	b.WriteString(t0.Format(time.RFC3339) + ",Alarme: 120 CPM (limite 100),alerta,alarme\n")
	for i := 1; i <= maxEvents+10; i++ {
		fmt.Fprintf(&b, "%s,Evento %d,evento,\n", t0.Add(time.Duration(i)*time.Minute).Format(time.RFC3339), i)
	}
	if err := os.WriteFile(filepath.Join(dir, "eventos.csv"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	st, _ := store.Open(dir)
	a := NewApp()
	a.dataDir, a.store, a.cal = dir, st, gmc.DefaultCalibration()
	a.settings = defaultSettings()
	a.events = a.loadEvents()
	if al := a.GetEvents(kindAlert); len(al) != 1 {
		t.Fatalf("o alerta mais antigo sumiu: %d", len(al))
	}
	if ev := a.GetEvents(kindEvent); len(ev) != maxEvents {
		t.Fatalf("eventos comuns: %d, queria %d", len(ev), maxEvents)
	}
	a.saveEvents()
	again := a.loadEvents()
	n := 0
	for _, e := range again {
		if e.Kind == kindAlert {
			n++
		}
	}
	if n != 1 || len(again) != maxEvents+1 {
		t.Fatalf("depois de regravar: %d alerta(s), %d no total", n, len(again))
	}
	al, _ := os.ReadFile(filepath.Join(dir, "alertas.csv"))
	if !strings.Contains(string(al), "Alarme: 120 CPM") {
		t.Fatalf("alertas.csv: %s", al)
	}
	// alerta nos dois arquivos (app fechou no meio): não repete
	ev, _ := os.ReadFile(filepath.Join(dir, "eventos.csv"))
	_ = os.WriteFile(filepath.Join(dir, "eventos.csv"), append(ev, al...), 0o644)
	if got := a.loadEvents(); len(got) != maxEvents+1 {
		t.Fatalf("alerta repetido: %d", len(got))
	}
}

// B11: linha ilegível vai para .ilegivel ao regravar; linha enorme não
// esconde o resto do mês.
func TestRcBadLinesKept(t *testing.T) {
	dir := t.TempDir()
	a := NewApp()
	a.dataDir = dir
	_ = os.MkdirAll(a.rcDir(), 0o755)
	m := time.Date(2026, 3, 10, 12, 0, 0, 0, time.Local)
	name := m.Format("2006-01") + ".csv"
	path := filepath.Join(a.rcDir(), name)
	huge := strings.Repeat("x", 100*1024)
	body := rcHeader + "\n" + huge + "\n" +
		m.Format(time.RFC3339) + ",10.0,0.0800,60\n" +
		"lixo,1,2\n" +
		m.Add(time.Minute).Format(time.RFC3339) + ",11.0,0.0900,60\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if pts := a.rcRange(m.Add(-time.Hour), m.Add(time.Hour)); len(pts) != 2 {
		t.Fatalf("leitura depois da linha enorme: %d minuto(s)", len(pts))
	}
	// regrava pelo backup (junta pela hora)
	in := rcHeader + "\n" + m.Add(2*time.Minute).Format(time.RFC3339) + ",12.0,0.1000,60\n"
	a.rcMergeFile(rcDirName+"/"+name, []byte(in))
	b, _ := os.ReadFile(path)
	if strings.Count(string(b), "\n") != 4 {
		t.Fatalf("arquivo regravado:\n%s", b)
	}
	bad, err := os.ReadFile(path + ".ilegivel")
	if err != nil || !strings.Contains(string(bad), "lixo,1,2") || !strings.Contains(string(bad), huge) {
		t.Fatalf("ilegível: %v %d bytes", err, len(bad))
	}
}

// D3: sem pasta nenhuma o app segue, com aviso, e os minutos esperam.
func TestUnopenedStore(t *testing.T) {
	f := filepath.Join(t.TempDir(), "arquivo")
	_ = os.WriteFile(f, []byte("x"), 0o644)
	st := store.Unopened(f) // um arquivo no lugar da pasta: nada abre
	now := time.Now().Truncate(time.Minute)
	err := st.Add(store.Point{T: now, CPM: 20, Src: store.SrcLive})
	if !errors.Is(err, store.ErrNotSaved) {
		t.Fatalf("Add: %v", err)
	}
	if n, _ := st.Pending(); n != 1 {
		t.Fatalf("pendentes: %d", n)
	}
	if pts, _ := st.Range(now.Add(-time.Hour), now.Add(time.Hour)); len(pts) != 1 {
		t.Fatalf("o minuto esperando não aparece: %d", len(pts))
	}
}
