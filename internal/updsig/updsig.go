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

// ReserveKey é a chave pública de reserva (base64), para o caso de a
// privada principal se perder: a privada dela fica guardada fora do GitHub
// e só passa a assinar se a principal sumir (SECURITY.md). Entrou na 2.72.0.
const ReserveKey = "WikNSgdGuqYSX8kLHJQ+lHqxeACcwDFeCDiQj1e/ujQ="

// Keys são as chaves públicas aceitas: a principal e a de reserva.
func Keys() []string {
	if ReserveKey == "" {
		return []string{PubKey}
	}
	return []string{PubKey, ReserveKey}
}

// Verify confere a assinatura (base64, como no .sig) de data com qualquer
// das chaves aceitas.
func Verify(data, sigB64 []byte) bool {
	return VerifyKeys(Keys(), data, sigB64)
}

// VerifyKeys é o Verify com outra lista de chaves (o ensaio da troca de
// chave em updsig_test.go).
func VerifyKeys(keys []string, data, sigB64 []byte) bool {
	for _, k := range keys {
		pub, err := base64.StdEncoding.DecodeString(k)
		if err == nil && len(pub) == ed25519.PublicKeySize && VerifyWith(ed25519.PublicKey(pub), data, sigB64) {
			return true
		}
	}
	return false
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
