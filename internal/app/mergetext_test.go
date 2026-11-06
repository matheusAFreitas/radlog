package app

import "testing"

func TestMergeTextPlural(t *testing.T) {
	cases := map[string]MergeResult{
		"nada novo": {},
		"+1 minuto de leitura, 1 evento/alerta, 1 período fora de casa, 1 medição, 1 detalhe de alerta, 1 anotação": {Minutes: 1, Events: 1, Periods: 1, Measurements: 1, AlertFiles: 1, Notes: 1},
		"+6020 minutos de leitura, 4 eventos/alertas, 2 anotações":                                                  {Minutes: 6020, Events: 4, Notes: 2},
	}
	for want, r := range cases {
		if got := mergeText(r); got != want {
			t.Errorf("mergeText = %q, quero %q", got, want)
		}
	}
}
