package app

import (
	"bytes"
	"crypto/sha512"
	"math/big"
	"testing"
)

// o ponto do Elligator 2 tem que estar na curva: v² = u³ + A·u² + u tem raiz
func TestElligatorOnCurve(t *testing.T) {
	for i := 0; i < 50; i++ {
		h := sha512.Sum512([]byte{byte(i), 7})
		le := elligator2(h[:])
		be := make([]byte, 32)
		for j := range le {
			be[j] = le[31-j]
		}
		u := new(big.Int).SetBytes(be)
		f := new(big.Int).Exp(u, big.NewInt(3), p25519)
		f.Add(f, new(big.Int).Mul(a25519, new(big.Int).Mul(u, u))).Add(f, u).Mod(f, p25519)
		exp := new(big.Int).Rsh(new(big.Int).Sub(p25519, big.NewInt(1)), 1)
		if l := new(big.Int).Exp(f, exp, p25519); f.Sign() != 0 && l.Cmp(big.NewInt(1)) != 0 {
			t.Fatalf("ponto %d fora da curva", i)
		}
	}
}

func TestCPace(t *testing.T) {
	tr := []byte("sid|a|b")
	a, _ := newCPace("123456")
	b, _ := newCPace("123456")
	ka, err1 := a.finish(b.Y, tr)
	kb, err2 := b.finish(a.Y, tr)
	if err1 != nil || err2 != nil || !bytes.Equal(ka, kb) {
		t.Fatal("mesmo código deveria dar a mesma chave")
	}
	c, _ := newCPace("654321")
	kc, _ := c.finish(a.Y, tr)
	ka2, _ := a.finish(c.Y, tr)
	if bytes.Equal(kc, ka2) {
		t.Fatal("código errado deu a mesma chave")
	}
	// envelope
	key, _ := pairKey(ka)
	box, _ := seal(key, []byte("oi"), []byte("aad"))
	if p, err := unseal(key, box, []byte("aad")); err != nil || string(p) != "oi" {
		t.Fatal("abrir")
	}
	if _, err := unseal(key, box, []byte("outro")); err == nil {
		t.Fatal("AAD diferente deveria falhar")
	}
}
