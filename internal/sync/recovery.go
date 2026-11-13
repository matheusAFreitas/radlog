// Package sync é a parte da sincronização entre aparelhos que não depende
// do app do PC (auditoria A1). Por enquanto, o código de recuperação da
// nuvem.
package sync

import (
	"crypto/rand"
	"errors"
	"strings"
)

// ---------- código de recuperação da nuvem ----------
//
// A chave da nuvem sai de um código de 24 caracteres em Crockford base32
// (sem 0/O/1/I/L/U para não confundir): 120 bits, mostrado em 6 grupos de 4
// ("K7QF-3XWM-…"). Quem digita pode colar com ou sem traços, em minúsculas,
// e trocar O por 0 ou I/L por 1: o código volta igual.

// RecoveryAlphabet são os 32 símbolos do Crockford base32.
const RecoveryAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// RecoveryLen é o tamanho do código, sem os traços.
const RecoveryLen = 24

// ErrRecoveryCode é o erro de um código que não tem a forma certa.
var ErrRecoveryCode = errors.New("o código tem 24 letras e números (sem O, I nem L)")

// NewRecoveryCode sorteia um código novo (sem traços).
func NewRecoveryCode() (string, error) {
	b := make([]byte, RecoveryLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	out := make([]byte, RecoveryLen)
	for i, x := range b {
		out[i] = RecoveryAlphabet[x&31] // 256 é múltiplo de 32: sem viés
	}
	return string(out), nil
}

// FormatRecoveryCode põe os traços: 6 grupos de 4.
func FormatRecoveryCode(code string) string {
	var b strings.Builder
	for i, c := range code {
		if i > 0 && i%4 == 0 {
			b.WriteByte('-')
		}
		b.WriteRune(c)
	}
	return b.String()
}

// ParseRecoveryCode aceita o que a pessoa colou ou digitou e devolve o
// código sem traços, ou ErrRecoveryCode.
func ParseRecoveryCode(s string) (string, error) {
	var b strings.Builder
	for _, c := range strings.ToUpper(s) {
		switch {
		case c == '-' || c == ' ' || c == '\t' || c == '\n' || c == '\r':
			continue
		case c == 'O':
			c = '0'
		case c == 'I' || c == 'L':
			c = '1'
		}
		if !strings.ContainsRune(RecoveryAlphabet, c) {
			return "", ErrRecoveryCode
		}
		b.WriteRune(c)
	}
	if b.Len() != RecoveryLen {
		return "", ErrRecoveryCode
	}
	return b.String(), nil
}
