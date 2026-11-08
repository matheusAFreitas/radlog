// Package updsig confere a assinatura Ed25519 do SHA256SUMS.txt das releases.
//
// O SHA256SUMS.txt só prova que o arquivo chegou inteiro, não quem publicou:
// quem conseguisse publicar uma release (token vazado, conta invadida)
// publicaria o .exe e a soma juntos. Por isso o job de release assina o
// SHA256SUMS.txt (scripts/sign, chave privada no segredo RADLOG_SIGN_KEY) e
// publica SHA256SUMS.txt.sig. O app só instala uma versão nova se a
// assinatura conferir com a chave pública abaixo.
package updsig

import (
	"crypto/ed25519"
	"encoding/base64"
	"strings"
)

// PubKey é a chave pública Ed25519 (base64) das releases.
const PubKey = "47Gd5qTSvqncJzSIKx3nXlBMOhHwdJkp/Wes9hC7EQk="

// Verify confere a assinatura (base64, como no .sig) de data com PubKey.
func Verify(data, sigB64 []byte) bool {
	pub, err := base64.StdEncoding.DecodeString(PubKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return false
	}
	return VerifyWith(ed25519.PublicKey(pub), data, sigB64)
}

// VerifyWith é o Verify com outra chave (testes e scripts/sign).
func VerifyWith(pub ed25519.PublicKey, data, sigB64 []byte) bool {
	if len(pub) != ed25519.PublicKeySize {
		return false
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sigB64)))
	if err != nil || len(sig) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(pub, data, sig)
}
