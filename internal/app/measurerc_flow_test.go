package app

import (
	"testing"

	"radlog/internal/radiacode"
)

// 2.100.2: ponta a ponta com o RC102 escolhido (tempo fixo, fundo medido na
// hora): fundo, objeto, resultado e a medição salva ficam no RC102, e o
// GMC-800 (ligado ao mesmo tempo) não entra na conta.
func TestMeasureRcFlow(t *testing.T) {
	a := newTestApp(t)
	fakeGMC(t, a, 25)
	a.rc.dev = radiacode.NewRaw(&rcRegs{regs: map[uint32]uint32{}})
	st, err := a.StartMeasure(MeasureOpts{Name: "sal", Device: "radiacode", Seconds: 20, Distance: "encostado", Size: "espalhado"})
	if err != nil {
		t.Fatal(err)
	}
	if st.Phase != "fundo" || devName(st.Device) != devRC {
		t.Fatalf("início: fase %q aparelho %q", st.Phase, st.Device)
	}
	feed := func(n int, rc int) {
		for i := 0; i < n; i++ {
			a.feedMeasure(0, 9) // o GMC-800 contando ao mesmo tempo: não entra
			a.feedMeasureDev(measFeed{dev: "radiacode", cpm: float64(rc * 60), counts: rc, usv: 0.1})
		}
	}
	feed(20, 10)
	if st = a.GetMeasure(); st.Phase != "objeto" && st.Phase != "fundo-pronto" {
		t.Fatalf("depois do fundo: %q", st.Phase)
	}
	if st.Phase == "fundo-pronto" {
		if st, err = a.StartMeasurePhase("objeto", 0, ""); err != nil {
			t.Fatal(err)
		}
	}
	feed(20, 12)
	st = a.GetMeasure()
	if st.Phase != "pronto" || devName(st.Device) != devRC {
		t.Fatalf("fim: fase %q aparelho %q", st.Phase, st.Device)
	}
	if st.Bg == nil || st.Bg.Counts != 200 || st.Obj == nil || st.Obj.Counts != 240 {
		t.Fatalf("contagens do RC102: fundo %+v objeto %+v", st.Bg, st.Obj)
	}
	ms := a.GetMeasurements()
	if len(ms) != 1 || devName(ms[0].Device) != devRC || ms[0].Obj.Counts != 240 {
		t.Fatalf("salva: %+v", ms)
	}
}
