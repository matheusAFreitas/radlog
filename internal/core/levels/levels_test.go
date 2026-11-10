package levels

import (
	"math"
	"testing"
)

func TestLevel(t *testing.T) {
	lv := Default()
	cases := map[float64]string{20: "seguro", 50: "atencao", 99: "atencao", 100: "alarme", 999: "alarme", 1000: "perigo"}
	for cpm, want := range cases {
		if got := Level(cpm, lv); got != want {
			t.Errorf("Level(%v) = %q, quero %q", cpm, got, want)
		}
	}
}

func TestSteps(t *testing.T) {
	lv := Default()
	on := false
	if s, _ := DangerStep(&on, 1000, lv); !s || !on {
		t.Fatal("perigo não começou em 1000")
	}
	if _, e := DangerStep(&on, 901, lv); e || !on {
		t.Fatal("perigo não pode terminar acima de 90%")
	}
	if _, e := DangerStep(&on, 899, lv); !e || on {
		t.Fatal("perigo não terminou abaixo de 90%")
	}
	on = false
	if !AttentionStep(&on, true, 50, lv) || !on {
		t.Fatal("atenção não começou em 50")
	}
	if AttentionStep(&on, true, 46, lv) || !on {
		t.Fatal("atenção não pode sair acima de 45")
	}
	if AttentionStep(&on, true, 44, lv); on {
		t.Fatal("atenção não saiu abaixo de 45")
	}
	on = true
	if AttentionStep(&on, false, 200, lv) || on {
		t.Fatal("desligada tem que apagar o estado")
	}
}

func TestNormAndDeadTime(t *testing.T) {
	var zero Levels
	if zero.Norm() != Default() {
		t.Fatal("zeros têm que virar os padrões")
	}
	lv := Default()
	if lv.DtCPM(30000) != 30000 {
		t.Fatal("tempo morto desligado não pode mudar nada")
	}
	lv.DeadTimeOn = true
	if g := lv.DtCPS(500); math.Abs(g-555.56) > 0.01 {
		t.Fatalf("500 cps → %.2f", g)
	}
	if g := lv.DtCPS(10000); math.Abs(g-100000) > 0.5 {
		t.Fatalf("teto de 10×: %.1f", g)
	}
}
