package app

import "testing"

func TestRemText(t *testing.T) {
	for _, c := range []struct{ in, sep, want string }{
		{"Alarme: 0,52 µSv/h (limite 0,50)", ",", "Alarme: 0,052 mrem/h (limite 0,050 mrem/h)"},
		{"Alarme no RC102: 2,1 µSv/h (alarme 2 do aparelho: 2)", ",", "Alarme no RC102: 0,21 mrem/h (alarme 2 do aparelho: 0,2 mrem/h)"},
		{"GMC-800 · 0,26 µSv/h · seguro", ",", "GMC-800 · 0,026 mrem/h · seguro"},
		{"GMC-800 · 0.26 µSv/h · safe", ".", "GMC-800 · 0.026 mrem/h · safe"},
		{"entre 0,08 e 0,12 µSv/h", ",", "entre 0,008 e 0,012 mrem/h"},
		{"0,10–0,20 µSv/h", ",", "0,010–0,020 mrem/h"},
		{"hoje 2,4 µSv", ",", "hoje 0,24 mrem"},
		{"1.239,5 µSv", ",", "123,95 mrem"},
		{"1,5 mSv/ano", ",", "150 mrem/ano"},
		{"+0,05 µSv/h", ",", "+0,005 mrem/h"},
		{"sem número µSv/h", ",", "sem número µSv/h"},
		{"50 CPM", ",", "50 CPM"},
		{"0,31 ± 0,02 µSv/h", ",", "0,031 ± 0,002 mrem/h"},
		// 2.101.5 (varredura do QA): "para" liga, e o "-800" do nome não é número
		{"caiu de 0,151 para 0,108 µSv/h", ",", "caiu de 0,0151 para 0,0108 mrem/h"},
		{"GMC-800 e 0,151 µSv/h", ",", "GMC-800 e 0,0151 mrem/h"},
		{"Cs-137 a 0,5 µSv/h", ",", "Cs-137 a 0,05 mrem/h"},
	} {
		if got := remText(c.in, c.sep); got != c.want {
			t.Errorf("remText(%q) = %q, quero %q", c.in, got, c.want)
		}
	}
}
