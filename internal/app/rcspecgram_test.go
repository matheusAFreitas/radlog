package app

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"radlog/internal/radiacode"
)

// espectro de mentira: n contagens no canal ch (resto zero)
func fakeSpec(sec float64, base []uint32) radiacode.Spectrum {
	return radiacode.Spectrum{Duration: time.Duration(sec * float64(time.Second)), A0: 0, A1: 3, A2: 0, Counts: append([]uint32(nil), base...)}
}

func TestRcSpectrogram(t *testing.T) {
	a := newTestApp(t)
	t0 := time.Now().Add(-2 * time.Hour).Truncate(time.Minute)
	c := make([]uint32, 1024)
	sec := 1000.0
	// 1ª leitura: só referência
	a.rcSpecAdd("RC-1", t0, fakeSpec(sec, c))
	// 10 minutos, 60 contagens por minuto no canal 200 (600 keV) e 30 no 500
	// (1.500 keV, fora do espectrograma até 1.500? fica na última faixa)
	for i := 1; i <= 10; i++ {
		c[200] += 60
		c[100] += 30
		sec += 60
		a.rcSpecAdd("RC-1", t0.Add(time.Duration(i)*time.Minute), fakeSpec(sec, c))
	}
	// caiu por 30 min; o aparelho seguiu contando (30 × 60 contagens)
	c[200] += 1800
	sec += 1800
	a.rcSpecAdd("RC-1", t0.Add(40*time.Minute), fakeSpec(sec, c))
	// zerou o espectro no aparelho: vale o atual (2 min)
	c = make([]uint32, 1024)
	c[100] = 120
	a.rcSpecAdd("RC-1", t0.Add(42*time.Minute), fakeSpec(120, c))
	// a leitura anterior fica guardada: outro App (reabriu) continua
	a.rc.spec.mu.Lock()
	a.rc.spec.closeLocked()
	a.rc.spec.mu.Unlock()
	a.rc.spec = rcSpecLog{}
	c[100] += 60
	a.rcSpecAdd("RC-1", t0.Add(43*time.Minute), fakeSpec(180, c))

	sg, err := a.GetRcSpectrogram(t0.UnixMilli(), t0.Add(60*time.Minute).UnixMilli(), 60, 1500)
	if err != nil {
		t.Fatal(err)
	}
	if len(sg.Rows) != 60 || sg.Bins != rcSgBins {
		t.Fatalf("linhas %d bins %d", len(sg.Rows), sg.Bins)
	}
	if sg.First != t0.UnixMilli() {
		t.Fatalf("primeira %v", time.UnixMilli(sg.First))
	}
	if sg.Rows[0] == nil || sg.Secs[0] != 60 || math.Abs(sg.CPS[0]-1.5) > 1e-6 {
		t.Fatalf("linha 0: %v s %v cps", sg.Secs[0], sg.CPS[0])
	}
	sum := func(r []float64) (s float64) {
		for _, v := range r {
			s += v
		}
		return
	}
	if math.Abs(sum(sg.Rows[0])-90) > 0.5 {
		t.Fatalf("contagens por minuto: %v", sum(sg.Rows[0]))
	}
	// canal 200 = 600–603 keV: cai na faixa 102 (5,86 keV cada)
	if b := 102; sg.Rows[0][b]+sg.Rows[0][b+1] < 59 {
		t.Fatalf("faixa de 600 keV: %v", sg.Rows[0][b-1:b+2])
	}
	// buraco: média do intervalo, marcada
	if sg.Rows[20] == nil || !sg.Gap[20] || math.Abs(sum(sg.Rows[20])-60) > 0.5 || sg.Gap[5] {
		t.Fatalf("buraco: %v %v", sg.Gap[20], sum(sg.Rows[20]))
	}
	// zerou: os 2 minutos antes da leitura
	if sg.Rows[41] == nil || math.Abs(sum(sg.Rows[41])-60) > 0.5 || sg.Gap[41] {
		t.Fatalf("depois de zerar: %v", sg.Rows[41])
	}
	if sg.Rows[42] == nil || math.Abs(sum(sg.Rows[42])-60) > 0.5 {
		t.Fatalf("depois de reabrir: %v", sg.Rows[42])
	}
	if sg.Rows[50] != nil || sg.Secs[50] != 0 {
		t.Fatalf("sem dados deveria ser nulo")
	}
	// linhas de 10 min juntam
	sg2, _ := a.GetRcSpectrogram(t0.UnixMilli(), t0.Add(60*time.Minute).UnixMilli(), 600, 3000)
	if len(sg2.Rows) != 6 || sg2.Secs[0] != 600 || math.Abs(sum(sg2.Rows[0])-90) > 0.5 {
		t.Fatalf("10 min: %v %v", sg2.Secs, sum(sg2.Rows[0]))
	}

	// outro aparelho: a primeira leitura dele é só referência
	a.rcSpecAdd("RC-2", t0.Add(50*time.Minute), fakeSpec(5000, c))
	sg, _ = a.GetRcSpectrogram(t0.UnixMilli(), t0.Add(60*time.Minute).UnixMilli(), 60, 1500)
	if sg.Rows[45] != nil {
		t.Fatalf("aparelho trocado não pode virar linha")
	}

	// backup leva os espectros (sem o ultimo.json) e outro PC recebe
	var buf bytes.Buffer
	if err := a.writeBackup(&buf); err != nil {
		t.Fatal(err)
	}
	b := newTestApp(t)
	zp := filepath.Join(t.TempDir(), "b.zip")
	os.WriteFile(zp, buf.Bytes(), 0o644)
	if _, err := b.mergeZip(zp, true); err != nil {
		t.Fatal(err)
	}
	sgb, _ := b.GetRcSpectrogram(t0.UnixMilli(), t0.Add(60*time.Minute).UnixMilli(), 60, 1500)
	if sgb.Rows[0] == nil || math.Abs(sum(sgb.Rows[0])-90) > 0.5 {
		t.Fatalf("no outro PC: %v", sgb.Rows[0])
	}
	if _, err := os.Stat(b.rcSpecPath(rcSpecLast)); err == nil {
		t.Fatalf("ultimo.json não vai no backup")
	}
}
