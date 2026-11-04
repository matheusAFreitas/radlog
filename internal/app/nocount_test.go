package app

import (
	"testing"
	"time"
)

func TestNoCount(t *testing.T) {
	var n noCountState
	t0 := time.Now()
	at := func(s int) time.Time { return t0.Add(time.Duration(s) * time.Second) }
	for s := 0; s < 119; s++ {
		if st, _ := n.step(at(s), 0, false); st {
			t.Fatalf("avisou cedo, em %d s", s)
		}
	}
	if st, _ := n.step(at(120), 0, false); !st || !n.on {
		t.Fatal("não avisou com 2 min em 0")
	}
	if st, _ := n.step(at(121), 0, false); st {
		t.Fatal("avisou de novo")
	}
	if _, end := n.step(at(150), 3, false); !end || n.on {
		t.Fatal("não encerrou quando voltou a contar")
	}
	// um 0 solto no meio não conta
	n.step(at(200), 0, false)
	n.step(at(201), 25, false)
	if !n.since.IsZero() {
		t.Fatal("não zerou a sequência")
	}
	// saturado: quem avisa é a saturação
	for s := 300; s < 500; s++ {
		if st, _ := n.step(at(s), 0, true); st {
			t.Fatal("avisou com o Geiger saturado")
		}
	}
	if got := noCountText(at(0), at(185)); got != "Contador voltou a contar depois de 3 min em 0 CPM" {
		t.Fatal(got)
	}
}
