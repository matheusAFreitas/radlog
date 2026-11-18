package app

import (
	"testing"
	"time"
)

// 2.101.0: "Tudo" vai do primeiro dado gravado até agora.
func TestDoseAll(t *testing.T) {
	a := newTestApp(t)
	fakeGMC(t, a, 25) // 60 min gravados
	c, err := a.GetDoseSrc("all", "publico", "gmc")
	if err != nil {
		t.Fatal(err)
	}
	first := time.Now().Truncate(time.Minute).Add(-60 * time.Minute).UnixMilli()
	if c.From > first || first-c.From > 5*60e3 || c.MeasuredMinutes < 55 {
		t.Fatalf("de %v (primeiro dado %v), %d min", c.From, first, c.MeasuredMinutes)
	}
}
