package app

// 2.101.9 (conferência do QA, item 124): o nome que a pessoa deu a uma
// medição não se traduz no evento, e o resto do evento sai em inglês.

import (
	"strings"
	"testing"

	"radlog/internal/i18n"
)

func TestMeasNameTrKeepsOwnName(t *testing.T) {
	cases := []struct {
		m    SavedMeasure
		want string
	}{
		{SavedMeasure{Name: "Relógio"}, "Relógio"},
		{SavedMeasure{Name: "Areia monazítica"}, "Areia monazítica"},
	}
	for _, c := range cases {
		ev := "Medição de " + measNameTr(c.m) + " (RC102): +0,120 ± 0,010 µSv/h sobre o fundo (12,0σ) — provavelmente um pouco radioativo; meça por mais tempo para confirmar"
		got := i18n.T("en", ev)
		if !strings.Contains(got, c.want) || strings.Contains(got, "Clock") || strings.Contains(got, "provavelmente") || !strings.HasPrefix(got, "Measurement of ") {
			t.Errorf("%q → %q (quer %q)", c.m.Name, got, c.want)
		}
	}
}

// motivo pronto do chip ("quebrado") traduz; nome da pessoa não
func TestPeriodNameOwnReasons(t *testing.T) {
	for n, want := range map[string]bool{"quebrado": false, "em manutenção": false, "Saí com o RC102": false, "": false, "Escritório": true} {
		if got := periodNameOwn(n); got != want {
			t.Errorf("periodNameOwn(%q) = %v, quer %v", n, got, want)
		}
	}
}
