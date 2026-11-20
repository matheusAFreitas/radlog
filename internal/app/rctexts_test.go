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

// Histórico velho (de ontem) e o que já entrou não voltam ao log, mesmo com
// a hora escorregando mais de 5 s entre uma leitura e outra (19/11).
func TestRcTextsOldAndDrift(t *testing.T) {
	var tx rcTexts
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.Local)
	lines := []radiacode.TextLine{
		{AgoSec: 20 * 3600, Text: "Error: S1 Data frame corrupted!"}, // ontem
		{AgoSec: 600, Text: "BLE: client connected"},
	}
	if out := tx.take(lines, now); len(out) != 1 || !strings.Contains(out[0], "BLE") {
		t.Fatalf("primeira leitura: %v", out)
	}
	// 30 min depois, a mesma de 10 min atrás volta com 40 s de diferença,
	// e uma nova chega
	later := now.Add(30 * time.Minute)
	again := []radiacode.TextLine{{AgoSec: 1800 + 600 - 40, Text: "BLE: client connected"}, {AgoSec: 60, Text: "BLE: disconnected by remote user"}}
	if out := tx.take(again, later); len(out) != 1 || !strings.Contains(out[0], "disconnected") {
		t.Fatalf("segunda leitura: %v", out)
	}
}

// dev.619 (20/11): mensagens com hora no futuro não entram nem mexem na marca.
func TestRcTextsFuture(t *testing.T) {
	var tx rcTexts
	now := time.Date(2026, 11, 20, 3, 11, 0, 0, time.Local)
	lines := []radiacode.TextLine{
		{AgoSec: -9 * 3600, Text: "Error: S1 Data frame corrupted!"},
		{AgoSec: 30, Text: "BLE: client connected"},
	}
	if out := tx.take(lines, now); len(out) != 1 || !strings.Contains(out[0], "BLE") {
		t.Fatalf("futuro entrou: %v", out)
	}
	if out := tx.take([]radiacode.TextLine{{AgoSec: 10, Text: "BLE: disconnected by remote user"}}, now.Add(time.Minute)); len(out) != 1 {
		t.Fatalf("a marca andou com a do futuro: %v", out)
	}
}
