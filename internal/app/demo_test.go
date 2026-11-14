package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"radlog/internal/gmc"
	"radlog/internal/store"
)

func TestRingWrapsAround(t *testing.T) {
	r := newRing()
	base := time.Now().Add(-2 * time.Hour)
	for i := 0; i < ringSize+100; i++ {
		r.add(base.Add(time.Duration(i)*time.Second), float64(i))
	}
	all := r.between(base, base.Add(3*time.Hour))
	if len(all) != ringSize || all[0].cpm != 100 || all[len(all)-1].cpm != ringSize+99 {
		t.Fatalf("%d amostras, primeira %v, última %v", len(all), all[0].cpm, all[len(all)-1].cpm)
	}
}

// 2.82: a demonstração só guarda o estado na memória, recusa o que tiraria
// dado do app e não mexe em nenhum arquivo; a leitura real continua.
func TestDemoDoesNotTouchFiles(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.Open(dir)
	a := NewApp()
	a.dataDir, a.store, a.cal = dir, st, gmc.DefaultCalibration()
	a.settings = defaultSettings()
	t.Cleanup(a.flushWrites)
	realT := time.Now().Truncate(time.Minute).Add(-10 * time.Minute)
	if err := st.Add(store.Point{T: realT, CPM: 20, Src: store.SrcLive}); err != nil {
		t.Fatal(err)
	}

	d := a.SetDemo(DemoState{Scenario: "nada disso", Speed: 3, USv: -2})
	if !d.Active || d.Scenario != "casa" || d.Speed != 1 || d.USv != 0 || d.Sound || d.Since == 0 {
		t.Fatalf("estado: %+v", d)
	}
	since := d.Since
	d = a.SetDemo(DemoState{Scenario: "fonte", Speed: 10, Sound: true})
	if d.Scenario != "fonte" || d.Speed != 10 || !d.Sound || d.Since != since {
		t.Fatalf("troca de cenário: %+v", d)
	}
	a.DemoTray(6.5, "perigo", 13, "perigo", "conectado")
	if r, show := a.trayRc(); !show || r.State != "ok" || r.USv != 13 || r.Level != "perigo" {
		t.Fatalf("bandeja do RC102: %+v", r)
	}

	for name, err := range map[string]error{
		"csv":    second(a.ExportCSV()),
		"pdf":    second(a.ExportReport(realT.UnixMilli(), time.Now().UnixMilli(), "")),
		"medida": second(a.ExportMeasureReport(1)),
		"backup": second(a.ExportBackup()),
		"nuvem":  second(a.CloudSyncNow()),
		"login":  second(a.CloudLogin()),
		"sync":   second(a.SyncNow("x")),
	} {
		if !errors.Is(err, errDemo) {
			t.Fatalf("%s na demonstração: %v", name, err)
		}
	}
	if _, err := a.ImportBackup(); !errors.Is(err, errDemo) {
		t.Fatalf("importar na demonstração: %v", err)
	}

	// a leitura real continua indo para o disco por baixo
	sim := newGMCSim(a)
	sim.feed(150, 1)
	if l := a.GetActiveAlerts(); len(l) == 0 {
		t.Fatal("o alerta real não abriu")
	}
	// e o alarme de verdade tirou da demonstração
	if a.GetDemo().Active {
		t.Fatal("alarme de verdade não tirou da demonstração")
	}
	if r, _ := a.trayRc(); r.USv == 13 {
		t.Fatal("a bandeja continuou com o RC102 da demonstração")
	}
	ev := a.GetEvents(kindEvent)
	if len(ev) == 0 || !strings.Contains(ev[0].Text, "Demonstração desligada") {
		t.Fatalf("eventos: %+v", ev)
	}

	// liga e desliga de novo sem leitura nenhuma: nada muda nos arquivos
	b := NewApp()
	dir2 := t.TempDir()
	st2, _ := store.Open(dir2)
	b.dataDir, b.store, b.cal = dir2, st2, gmc.DefaultCalibration()
	b.settings = defaultSettings()
	t.Cleanup(b.flushWrites)
	before2 := snapshotDir(t, dir2)
	b.SetDemo(DemoState{Scenario: "perigo"})
	b.DemoTray(6.5, "perigo", 0, "", "")
	b.StopDemo()
	b.flushWrites()
	if after := snapshotDir(t, dir2); after != before2 {
		t.Fatalf("a demonstração mexeu nos arquivos:\nantes:\n%s\ndepois:\n%s", before2, after)
	}
	if b.GetDemo().Active {
		t.Fatal("demonstração continuou ligada")
	}
}

func second[T any](_ T, err error) error { return err }

// snapshotDir lista nome, tamanho e conteúdo de todos os arquivos (menos o
// log.txt, o registro de diagnóstico, que anota quando a demonstração liga).
func snapshotDir(t *testing.T, dir string) string {
	var b strings.Builder
	filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && info.Name() != "log.txt" {
			data, _ := os.ReadFile(p)
			b.WriteString(p + "\n" + string(data) + "\n")
		}
		return nil
	})
	return b.String()
}
