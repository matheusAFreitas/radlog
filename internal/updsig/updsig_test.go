package updsig

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"
)

func TestVerifyWith(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	pub := priv.Public().(ed25519.PublicKey)
	data := []byte("abc  RadLog-v2.16.0.exe\n")
	sig := []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(priv, data)) + "\n")
	if !VerifyWith(pub, data, sig) {
		t.Fatal("assinatura boa recusada")
	}
	if VerifyWith(pub, []byte("abd  RadLog-v2.16.0.exe\n"), sig) {
		t.Fatal("aceitou o SHA256SUMS.txt alterado")
	}
	other := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize)).Public().(ed25519.PublicKey)
	if VerifyWith(other, data, sig) {
		t.Fatal("aceitou assinatura de outra chave")
	}
	for _, bad := range []string{"", "xx", base64.StdEncoding.EncodeToString(make([]byte, 10))} {
		if VerifyWith(pub, data, []byte(bad)) {
			t.Fatalf("aceitou assinatura inválida %q", bad)
		}
	}
}

func TestPubKeyValid(t *testing.T) {
	b, err := base64.StdEncoding.DecodeString(PubKey)
	if err != nil || len(b) != ed25519.PublicKeySize {
		t.Fatalf("PubKey não é uma chave Ed25519 em base64 (%d bytes, %v)", len(b), err)
	}
}
