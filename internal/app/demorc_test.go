package app

import (
	"math/rand"
	"testing"
	"time"
)

// 2.14: Radiacode simulado no demo: aparece na tela, abre alerta só na
// memória e some ao sair do demo
func TestDemoRc(t *testing.T) {
	a := newTestApp(t)
	if st := a.SetDemoRc(true, 1); st.Rc {
		t.Fatal("ligou sem o demo")
	}
	a.StartDemo(20, false)
	a.SetDemoRc(true, 50) // bem acima de qualquer limite
	rng := rand.New(rand.NewSource(1))
	now := time.Now()
	for i := 0; i < 3; i++ {
		a.demoRcTick(rng, now.Add(time.Duration(i)*time.Second))
	}
	st := a.GetRadiacode()
	if !st.Demo || st.State != "conectado" || st.USvh != 50 || st.AlertLevel == "" {
		t.Fatalf("estado simulado: %+v", st)
	}
	act := a.GetActiveAlerts()
	if len(act) == 0 || !act[0].Demo || act[0].Device != devRC {
		t.Fatalf("alerta simulado: %+v", act)
	}
	c, _ := a.GetChart("15m")
	got := false
	for _, v := range c.RC {
		if v != nil && *v == 50 {
			got = true
		}
	}
	if !got {
		t.Fatal("o gráfico não mostrou o simulado")
	}
	a.StopDemo()
	if st := a.GetRadiacode(); st.Demo || st.State == "conectado" {
		t.Fatalf("continuou simulado: %+v", st)
	}
	if len(a.GetActiveAlerts()) != 0 {
		t.Fatal("alerta simulado ficou aberto")
	}
}
