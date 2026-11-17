package app

import "testing"

func TestIsoText(t *testing.T) {
	cases := map[string]string{
		"16/11 04:58":                   "11-16 04:58",
		"gerado em 16/11/2026 15:04":    "gerado em 2026-11-16 15:04",
		"13/11–14/11":                   "11-13–11-14",
		"de 13/11 a 14/11/2026":         "de 11-13 a 2026-11-14",
		"1/2 do período":                "1/2 do período",
		"32/13 não é data":              "32/13 não é data",
		"arquivo 2026-11-16 fica igual": "arquivo 2026-11-16 fica igual",
		"0,05/12 h":                     "0,05/12 h",
	}
	for in, want := range cases {
		if got := isoText(in); got != want {
			t.Errorf("isoText(%q) = %q, quer %q", in, got, want)
		}
	}
	if normDateFormat("iso") != "iso" || normDateFormat("mdy") != "" {
		t.Error("normDateFormat")
	}
}
