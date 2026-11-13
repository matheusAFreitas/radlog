package sync

import (
	"strings"
	"testing"
)

func TestRecoveryCode(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		c, err := NewRecoveryCode()
		if err != nil || len(c) != RecoveryLen {
			t.Fatalf("código %q, %v", c, err)
		}
		if strings.ContainsAny(c, "OILU") {
			t.Fatalf("letra confusa em %q", c)
		}
		if seen[c] {
			t.Fatal("código repetido")
		}
		seen[c] = true
		f := FormatRecoveryCode(c)
		if len(f) != 29 || strings.Count(f, "-") != 5 {
			t.Fatalf("formato %q", f)
		}
		for _, in := range []string{f, c, strings.ToLower(f), " " + strings.ReplaceAll(f, "-", " ") + "\n"} {
			if got, err := ParseRecoveryCode(in); err != nil || got != c {
				t.Fatalf("%q → %q, %v", in, got, err)
			}
		}
	}
	// O, I e L viram 0 e 1
	if got, _ := ParseRecoveryCode("O1IL-0000-0000-0000-0000-0000"); got != "011100000000000000000000" {
		t.Fatalf("troca de letras: %q", got)
	}
	for _, bad := range []string{"", "K7QF-3XWM", "K7QF-3XWM-9HTR-P2DN-V8CZ-6JYBU", "K7QF-3XWM-9HTR-P2DN-V8CZ-6JY!"} {
		if _, err := ParseRecoveryCode(bad); err == nil {
			t.Fatalf("%q deveria falhar", bad)
		}
	}
}
