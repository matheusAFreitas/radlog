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
	for _, k := range Keys() {
		b, err := base64.StdEncoding.DecodeString(k)
		if err != nil || len(b) != ed25519.PublicKeySize {
			t.Fatalf("%q não é uma chave Ed25519 em base64 (%d bytes, %v)", k, len(b), err)
		}
	}
	if ReserveKey == PubKey {
		t.Fatal("a reserva não pode ser a mesma chave")
	}
}

// Verify só aceita as chaves do app: uma assinatura boa de outra chave, ou
// lixo, é recusada
func TestVerifyOnlyAppKeys(t *testing.T) {
	priv := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	data := []byte("abc  RadLog-v2.74.0.exe\n")
	sig := []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(priv, data)))
	if Verify(data, sig) {
		t.Fatal("aceitou assinatura de uma chave que não é do app")
	}
	for _, bad := range []string{"", "\n", "não é base64", base64.StdEncoding.EncodeToString(make([]byte, ed25519.SignatureSize))} {
		if Verify(data, []byte(bad)) {
			t.Fatalf("aceitou %q", bad)
		}
	}
	if len(Keys()) != 2 || Keys()[0] != PubKey || Keys()[1] != ReserveKey {
		t.Fatalf("chaves: %v", Keys())
	}
}

// chave pública com tamanho errado nunca confere
func TestVerifyWithBadKey(t *testing.T) {
	priv := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	data := []byte("x")
	sig := []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(priv, data)))
	pub := priv.Public().(ed25519.PublicKey)
	for _, k := range []ed25519.PublicKey{nil, pub[:16], append(append(ed25519.PublicKey{}, pub...), 0)} {
		if VerifyWith(k, data, sig) {
			t.Fatalf("aceitou chave de %d bytes", len(k))
		}
	}
}

// S7: ensaio da troca de chave do SECURITY.md com chaves de teste.
func TestKeyRotation(t *testing.T) {
	gen := func() (ed25519.PrivateKey, string) {
		pub, priv, _ := ed25519.GenerateKey(nil)
		return priv, base64.StdEncoding.EncodeToString(pub)
	}
	sign := func(priv ed25519.PrivateKey, data []byte) []byte {
		return []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(priv, data)))
	}
	data := []byte("SHA256SUMS.txt")
	_, mainPub := gen() // a principal se perdeu
	resPriv, resPub := gen()
	installed := []string{mainPub, resPub} // o app que já está nos PCs

	// passos 1 e 2: a reserva vira PubKey, entra uma reserva nova
	newResPriv, newResPub := gen()
	next := []string{resPub, newResPub}

	// passo 3: a release assinada com a reserva instala nos PCs
	sig := sign(resPriv, data)
	if !VerifyKeys(installed, data, sig) || !VerifyKeys(next, data, sig) {
		t.Fatal("a release assinada com a reserva não instala")
	}
	// e a versão nova aceita a reserva nova, não a principal perdida
	if !VerifyKeys(next, data, sign(newResPriv, data)) {
		t.Fatal("a reserva nova não vale")
	}
	if VerifyKeys(next, data, sign(newResPriv, []byte("outro"))) {
		t.Fatal("assinatura de outro arquivo valeu")
	}
	for _, k := range next {
		if k == mainPub {
			t.Fatal("a chave perdida continua aceita")
		}
	}
}
