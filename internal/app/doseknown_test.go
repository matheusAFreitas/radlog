package app

import "testing"

func TestDoseKnown(t *testing.T) {
	const bg = 0.12 // µSv/h
	for _, c := range []struct {
		usv  float64
		want string
	}{
		{2.9, "≈ 1 dia do fundo da sua casa · ≈ meia hora de voo"},
		{0.72, "≈ 6 h do fundo da sua casa"},
		{12, "≈ 4 dias do fundo da sua casa · ≈ 3 horas de voo"},
		{60, "≈ 3 semanas do fundo da sua casa · ≈ 3 raios-X de tórax"},
		{0.05, "≈ 25 min do fundo da sua casa"},
		{0.001, "menos de 1 min do fundo da sua casa"},
		{1051, "≈ 1 ano do fundo da sua casa"},
		{20, "≈ 7 dias do fundo da sua casa · ≈ 1 raio-X de tórax"},
		{9000, "≈ 8,5 anos do fundo da sua casa · ≈ 1,5 tomografias de tórax"},
	} {
		if got := doseKnown(c.usv, bg); got != c.want {
			t.Errorf("%v µSv: %q (queria %q)", c.usv, got, c.want)
		}
	}
	if got := doseKnown(8, 0); got != "≈ 2 horas de voo" {
		t.Errorf("sem fundo: %q", got)
	}
	if doseKnown(0, bg) != "" {
		t.Error("dose zero deveria ficar vazia")
	}
	for in, want := range map[float64]float64{0.8: 1, 2.3: 2.5, 12.4: 12, 123: 120, 1460: 1500} {
		if got := knownNum(in); got != want {
			t.Errorf("knownNum(%v) = %v (queria %v)", in, got, want)
		}
	}
}
