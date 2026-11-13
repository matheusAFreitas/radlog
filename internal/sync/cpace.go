package sync

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"math/big"
)

// ---------- pareamento com código de 6 dígitos (CPace em X25519) ----------
//
// Um código de 6 dígitos tem só 1 milhão de valores: se ele virasse a chave
// direto, quem escutasse a rede testaria todos em segundos. Por isso o
// pareamento usa CPace, uma troca de chaves autenticada por senha:
//
//   - o ponto base G sai do código (hash → curva, Elligator 2);
//   - cada lado sorteia um número secreto y e manda Y = y·G;
//   - os dois chegam a K = y_a·y_b·G, e conferem com HMACs.
//
// Quem só escuta não aprende nada (é um Diffie-Hellman). Quem se mete no
// meio só consegue testar UM código por tentativa, e o app aceita poucas
// tentativas por código. Depois do pareamento, o código não vale mais nada:
// os dois lados guardam uma chave de 32 bytes derivada de K.
//
// O X25519 da biblioteca do Go faz a multiplicação y·P para qualquer ponto P
// (é o ECDH), então só o mapeamento hash → curva é feito aqui, com math/big.

var (
	p25519 = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 255), big.NewInt(19))
	a25519 = big.NewInt(486662)
)

// elligator2 leva 64 bytes de hash a um ponto da Curve25519 (coordenada u,
// 32 bytes little-endian), pelo mapa Elligator 2 com Z = 2.
func elligator2(h []byte) []byte {
	p := p25519
	r := new(big.Int).SetBytes(h)
	r.Mod(r, p)
	// w = -A / (1 + 2r²)
	den := new(big.Int).Mul(r, r)
	den.Lsh(den, 1).Add(den, big.NewInt(1)).Mod(den, p)
	w := new(big.Int).Neg(a25519)
	if den.Sign() != 0 {
		inv := new(big.Int).ModInverse(den, p)
		w.Mul(w, inv)
	}
	w.Mod(w, p)
	// e = legendre(w³ + A·w² + w): não quadrado → u = -w - A
	f := new(big.Int).Mul(w, w)
	f.Mul(f, w)
	aw2 := new(big.Int).Mul(w, w)
	aw2.Mul(aw2, a25519)
	f.Add(f, aw2).Add(f, w).Mod(f, p)
	exp := new(big.Int).Rsh(new(big.Int).Sub(p, big.NewInt(1)), 1)
	leg := new(big.Int).Exp(f, exp, p)
	u := w
	if leg.Cmp(big.NewInt(1)) != 0 && f.Sign() != 0 {
		u = new(big.Int).Neg(w)
		u.Sub(u, a25519).Mod(u, p)
	}
	be := u.FillBytes(make([]byte, 32))
	le := make([]byte, 32)
	for i := range be {
		le[i] = be[31-i]
	}
	return le
}

// pairGenerator é o ponto base G para um código.
func pairGenerator(code string) (*ecdh.PublicKey, error) {
	h := sha512.Sum512([]byte("radlog-pair-v1|" + code))
	return ecdh.X25519().NewPublicKey(elligator2(h[:]))
}

// CPace é um lado do pareamento: o segredo y e o Y = y·G mandado.
type CPace struct {
	priv *ecdh.PrivateKey
	Y    []byte
}

// NewCPace começa um lado do pareamento com o código de 6 dígitos.
func NewCPace(code string) (*CPace, error) {
	g, err := pairGenerator(code)
	if err != nil {
		return nil, err
	}
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	y, err := priv.ECDH(g)
	if err != nil {
		return nil, err
	}
	return &CPace{priv: priv, Y: y}, nil
}

// Finish calcula a chave de confirmação a partir do Y do outro lado.
// transcript: tudo o que foi trocado (sid, ids, Ys), nos dois lados igual.
func (c *CPace) Finish(otherY, transcript []byte) ([]byte, error) {
	pub, err := ecdh.X25519().NewPublicKey(otherY)
	if err != nil {
		return nil, err
	}
	k, err := c.priv.ECDH(pub)
	if err != nil {
		return nil, errors.New("ponto inválido no pareamento")
	}
	return hkdf.Key(sha256.New, k, transcript, "radlog-pair-confirm", 32)
}

// PairMAC confirma que o outro lado chegou à mesma chave (who: "client"/"server").
func PairMAC(kc []byte, who string) []byte {
	m := hmac.New(sha256.New, kc)
	m.Write([]byte("radlog-pair-mac|" + who))
	return m.Sum(nil)
}

// PairKey é a chave que fica guardada depois do pareamento.
func PairKey(kc []byte) ([]byte, error) {
	return hkdf.Key(sha256.New, kc, nil, "radlog-sync-key-v1", 32)
}

// ---------- envelope das mensagens de sincronização ----------
//
// Depois de pareados, cada pedido e cada resposta vai cifrado e autenticado
// com AES-256-GCM na chave do par. O "dados extra" (AAD) amarra a mensagem ao
// caminho, a quem mandou e à hora, então não dá para reaproveitar uma resposta
// em outro pedido.

// Seal cifra e autentica plain com AES-256-GCM; aad amarra a mensagem.
func Seal(key, plain, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return g.Seal(nonce, nonce, plain, aad), nil
}

// Unseal abre o que Seal fechou com a mesma chave e o mesmo aad.
func Unseal(key, box, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(box) < g.NonceSize() {
		return nil, errors.New("mensagem curta demais")
	}
	return g.Open(nil, box[:g.NonceSize()], box[g.NonceSize():], aad)
}
