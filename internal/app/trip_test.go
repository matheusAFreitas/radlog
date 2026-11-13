package app

import (
	"math"
	"testing"
	"time"
)

func TestTripSummary(t *testing.T) {
	a := newTestApp(t)
	t0 := time.Now().Add(-10 * time.Hour).Truncate(time.Minute)
	// contínuo que cai com a energia + pico do K-40 (1461 keV = canal 487)
	cont := func(i int) float64 { return 40 / (1 + float64(i)/40) }
	acc := make([]float64, 1024)
	c := make([]uint32, 1024)
	sec := 0.0
	add := func(min float64, k40 float64) {
		for i := range acc {
			acc[i] += cont(i) * min
		}
		for d := -8; d <= 8; d++ {
			acc[487+d] += k40 * min * math.Exp(-float64(d*d)/18)
		}
		for i := range c {
			c[i] = uint32(acc[i])
		}
		sec += min * 60
	}
	a.rcSpecAdd("RC-1", t0, fakeSpec(sec, c))
	// 3 h em casa, de minuto em minuto
	for i := 1; i <= 180; i++ {
		add(1, 3)
		a.rcSpecAdd("RC-1", t0.Add(time.Duration(i)*time.Minute), fakeSpec(sec, c))
	}
	// saiu às 3 h; o RC102 longe do PC por 4 h, com o dobro de K-40
	leave := t0.Add(180 * time.Minute)
	back := leave.Add(4 * time.Hour)
	add(240, 6)
	a.rcSpecAdd("RC-1", back.Add(5*time.Minute), fakeSpec(sec, c))

	p, err := a.SavePeriod(Period{From: leave.UnixMilli(), To: back.UnixMilli(), GMC: WhereHome, RC: WhereMe, NotHome: true})
	if err != nil {
		t.Fatal(err)
	}
	a.rcLogAppend([]RcLogItem{
		{T: leave.Add(time.Hour).UnixMilli(), Kind: "raro", Battery: 92, HasBat: true, Temp: 24, HasTemp: true, Src: "pc"},
		{T: leave.Add(2 * time.Hour).UnixMilli(), Kind: "raro", Battery: 80, HasBat: true, Temp: 31, HasTemp: true, Src: "pc"},
		{T: leave.Add(3 * time.Hour).UnixMilli(), Kind: "raro", Battery: 71, HasBat: true, Temp: 27, HasTemp: true, Src: "pc"},
		{T: leave.Add(90 * time.Minute).UnixMilli(), Kind: "evento", Event: "ligou", Code: 1, Src: "pc"},
		{T: leave.Add(100 * time.Minute).UnixMilli(), Kind: "evento", Event: "alarme 1 de taxa de dose", Code: 9, Src: "pc"},
	})

	s, err := a.GetTripSummary(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !s.DefaultName || s.Open {
		t.Fatalf("nome padrão %v aberto %v (%q)", s.DefaultName, s.Open, p.Name)
	}
	if s.Spec == nil || s.SpecRows != 1 || math.Abs(s.Spec.ObjSec-240*60) > 1 {
		t.Fatalf("espectro: %+v linhas %d", s.Spec != nil, s.SpecRows)
	}
	if s.Spec.BgSec < 170*60 || s.Spec.BgSec > 181*60 {
		t.Fatalf("casa: %v s", s.Spec.BgSec)
	}
	var k40 *TripLine
	for i := range s.Lines {
		if s.Lines[i].Nuclide == "K-40" {
			k40 = &s.Lines[i]
		}
	}
	if k40 == nil || !k40.More || !k40.HasHome || k40.Pct < 10 {
		t.Fatalf("K-40: %+v (linhas %+v)", k40, s.Lines)
	}
	// "mais que em casa" nunca com o pico menor que o de casa (2.69.1)
	if k40.Out <= k40.Home {
		t.Fatalf("K-40 subiu mas fora %v ≤ casa %v", k40.Out, k40.Home)
	}
	for _, l := range s.Lines {
		if (l.More && l.Pct < 0) || (l.Less && l.Pct > 0) || (l.More && l.Less) {
			t.Fatalf("linha contraditória: %+v", l)
		}
	}
	if !s.HasBat || s.Bat0 != 92 || s.Bat1 != 71 || !s.HasTemp || s.TempMin != 24 || s.TempMax != 31 {
		t.Fatalf("aparelho: %+v", s)
	}
	if len(s.Events) != 1 || s.Events[0].Event != "alarme 1 de taxa de dose" {
		t.Fatalf("avisos: %+v", s.Events)
	}

	// com nome dado e sem nenhuma linha do espectro na saída
	q, err := a.SavePeriod(Period{From: t0.Add(-5 * time.Hour).UnixMilli(), To: t0.Add(-4 * time.Hour).UnixMilli(), Name: "Centro", GMC: WhereHome, RC: WhereMe, NotHome: true})
	if err != nil {
		t.Fatal(err)
	}
	s, _ = a.GetTripSummary(q.ID)
	if s.DefaultName || s.Spec != nil {
		t.Fatalf("sem espectro: %+v", s)
	}
	if _, err := a.GetTripSummary(999); err == nil {
		t.Fatal("período que não existe")
	}
}

// a linha da volta começou horas antes da saída (o RC102 parou de mandar
// espectro antes de sair): o pedaço de casa sai pelo espectro de casa, e o
// tempo do espectro é o da saída (relatório do ACE, 13/11: 14 h numa saída de 10 h)
func TestTripSpectrumLongReturnRow(t *testing.T) {
	s := longReturnTrip(t, 6)
	if s.Spec == nil || math.Abs(s.Spec.ObjSec-240*60) > 60 {
		t.Fatalf("tempo do espectro: %+v", s.Spec)
	}
	for _, l := range s.Lines {
		if l.Nuclide == "K-40" {
			if l.Pct < 70 || l.Pct > 130 {
				t.Fatalf("K-40 fora deveria ser ~2× casa: %+v", l)
			}
			return
		}
	}
	t.Fatalf("sem K-40: %+v", s.Lines)
}

// fora igual a casa, com volta longa: nenhuma linha "mais" nem "menos" que
// casa (o erro da subtração entra na conta; QA da 2.78.2)
func TestTripSpectrumLongReturnSame(t *testing.T) {
	s := longReturnTrip(t, 3)
	for _, l := range s.Lines {
		if l.More || l.Less {
			t.Fatalf("linha diferente de casa com fora = casa: %+v", l)
		}
	}
}

func longReturnTrip(t *testing.T, awayK40 float64) TripSummary {
	t.Helper()
	a := newTestApp(t)
	t0 := time.Now().Add(-12 * time.Hour).Truncate(time.Minute)
	cont := func(i int) float64 { return 40 / (1 + float64(i)/40) }
	acc := make([]float64, 1024)
	c := make([]uint32, 1024)
	sec := 0.0
	add := func(min float64, k40 float64) {
		for i := range acc {
			acc[i] += cont(i) * min
		}
		for d := -8; d <= 8; d++ {
			acc[487+d] += k40 * min * math.Exp(-float64(d*d)/18)
		}
		for i := range c {
			c[i] = uint32(acc[i])
		}
		sec += min * 60
	}
	a.rcSpecAdd("RC-1", t0, fakeSpec(sec, c))
	for i := 1; i <= 180; i++ {
		add(1, 3)
		a.rcSpecAdd("RC-1", t0.Add(time.Duration(i)*time.Minute), fakeSpec(sec, c))
	}
	// mais 2 h em casa sem linha nova, depois 4 h fora com o dobro de K-40
	add(120, 3)
	leave := t0.Add(300 * time.Minute)
	back := leave.Add(4 * time.Hour)
	add(240, awayK40)
	a.rcSpecAdd("RC-1", back, fakeSpec(sec, c))
	p, err := a.SavePeriod(Period{From: leave.UnixMilli(), To: back.UnixMilli(), GMC: WhereHome, RC: WhereMe, NotHome: true})
	if err != nil {
		t.Fatal(err)
	}
	s, err := a.GetTripSummary(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestDoseVsHomeTxt(t *testing.T) {
	for _, c := range []struct {
		v    reportDoseVerdict
		want string
	}{
		{reportDoseVerdict{Dose: 1.01, Expected: 1.49, Level: "normal"}, "32% abaixo"},
		{reportDoseVerdict{Dose: 1.45, Expected: 1.49, Level: "normal"}, "perto"},
		{reportDoseVerdict{Dose: 1.2, Expected: 1.0, Level: "normal"}, "perto"},
		{reportDoseVerdict{Dose: 1.6, Expected: 1.0, Level: "acima"}, "acima"},
	} {
		if got := doseVsHomeTxt(c.v); got != c.want {
			t.Fatalf("%+v: %q, quero %q", c.v, got, c.want)
		}
	}
}
