package app

import (
	"strings"
	"testing"
	"time"

	"radlog/internal/radiacode"
)

// O erro que o aparelho repete a cada ~100 s entra uma vez por hora, com a
// contagem; reler as mesmas mensagens (reconexão) não repete nada.
func TestRcTextsRepeat(t *testing.T) {
	var tx rcTexts
	now := time.Date(2026, 11, 8, 23, 0, 0, 0, time.Local)
	var lines []radiacode.TextLine
	for ago := 3 * 3600; ago >= 0; ago -= 100 { // 3 h de erro a cada 100 s
		lines = append(lines, radiacode.TextLine{AgoSec: ago, Text: "Error: S1 Data frame corrupted!"})
	}
	lines = append(lines, radiacode.TextLine{AgoSec: 50, Text: "BLE: client connected"})
	out := tx.take(lines, now)
	if len(out) != 5 { // 1 por hora (20, 21, 22 e 23 h) + o BLE
		t.Fatalf("esperava 5 linhas, vieram %d:\n%s", len(out), strings.Join(out, "\n"))
	}
	if !strings.Contains(out[1], "repetiu mais 35 vezes desde 20:00") {
		t.Fatalf("contagem: %q", out[1])
	}
	// reconexão 7 s depois: o aparelho devolve tudo de novo, com "segundos
	// atrás" deslocados
	for i := range lines {
		lines[i].AgoSec += 7
	}
	if again := tx.take(lines, now.Add(7*time.Second)); len(again) != 0 {
		t.Fatalf("repetiu na reconexão: %v", again)
	}
}
