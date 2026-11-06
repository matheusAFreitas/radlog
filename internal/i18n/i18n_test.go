package i18n

import "testing"

func TestT(t *testing.T) {
	cases := map[string]string{
		"Alarme: 112 CPM (limite 100)":                    "Alarm: 112 CPM (limit 100)",
		"Atenção: 52,3 CPM (a partir de 50)":              "Attention: 52.3 CPM (from 50)",
		"Configurações":                                   "Settings",
		"ESTATÍSTICA DAS FASES":                           "PHASE STATISTICS",
		"Nenhuma diferença que dê para separar do fundo.": "No difference that can be told apart from background.",
		"Atenção: 54 CPM (a partir de 50) · possível interferência (40 contagens em 1 s às 23:41:07)": "Attention: 54 CPM (from 50) · possible interference (40 counts in 1 s at 23:41:07)",
		"23,5":                "23.5",
		"Abrir o GMC Monitor": "Open GMC Monitor",
	}
	for in, want := range cases {
		if got := T("en", in); got != want {
			t.Errorf("%q → %q, queria %q", in, got, want)
		}
		if T("pt", in) != in {
			t.Errorf("pt mudou %q", in)
		}
	}
	// traduzir de novo não muda (o PDF mede o texto e depois escreve)
	for in := range cases {
		once := T("en", in)
		if twice := T("en", once); twice != once {
			t.Errorf("não é estável: %q → %q", once, twice)
		}
	}
}

func TestLang(t *testing.T) {
	if Lang("auto", "pt-BR") != "pt" || Lang("", "en-US") != "en" || Lang("en", "pt-BR") != "en" || Lang("pt", "en-US") != "pt" {
		t.Fatal("Lang")
	}
}
