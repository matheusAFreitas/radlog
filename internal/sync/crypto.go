package sync

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

// ---------- chaves da nuvem ----------
//
// O código de recuperação (24 símbolos, 120 bits) e um sal sorteado na criação
// da nuvem dão duas chaves (HKDF-SHA256): uma cifra o conteúdo (AES-256-GCM) e
// a outra dá os nomes dos arquivos (HMAC), para o Drive não ver de que mês ou
// aparelho é cada arquivo. O sal e um "teste" cifrado ficam no arquivo de
// metadados da pasta, que não é segredo: sem o código, nada abre.

// Keys são as chaves derivadas do código.
type Keys struct {
	enc  [32]byte
	name [32]byte
}

// ErrOpen é o erro de um arquivo que não abre com estas chaves (código errado
// ou arquivo mexido).
var ErrOpen = errors.New("não abre com este código")

// ErrOldCloud: a nuvem foi criada no formato antigo (2.79.x) e precisa ser
// começada de novo (sem conversão; ACE, 14/11).
var ErrOldCloud = errors.New("nuvem do formato antigo")

// metaCheck é o texto que o teste do arquivo de metadados guarda.
const metaCheck = "radlog-nuvem-v1"

// DeriveKeys tira as chaves do código (sem traços) e do sal.
func DeriveKeys(code string, salt []byte) (*Keys, error) {
	c, err := ParseRecoveryCode(code)
	if err != nil {
		return nil, err
	}
	if len(salt) < 16 {
		return nil, errors.New("sal curto")
	}
	out, err := hkdf.Key(sha256.New, []byte(c), salt, "radlog nuvem v1", 64)
	if err != nil {
		return nil, err
	}
	k := &Keys{}
	copy(k.enc[:], out[:32])
	copy(k.name[:], out[32:])
	return k, nil
}

// NewSalt sorteia o sal de uma nuvem nova.
func NewSalt() ([]byte, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	return b, err
}

// FileName é o nome do arquivo no Drive para uma chave interna (por exemplo
// "p|aparelho|2026-11"): 32 letras hexadecimais que só quem tem o código
// sabe calcular.
func (k *Keys) FileName(id string) string {
	m := hmac.New(sha256.New, k.name[:])
	m.Write([]byte(id))
	return hex.EncodeToString(m.Sum(nil))[:32]
}

// Seal cifra plain; o nome do arquivo entra como dado associado, então um
// arquivo trocado de nome no Drive não abre.
func (k *Keys) Seal(name string, plain []byte) ([]byte, error) {
	g, err := k.gcm()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return g.Seal(nonce, nonce, plain, []byte(name)), nil
}

// Open decifra o que Seal fez com o mesmo nome.
func (k *Keys) Open(name string, sealed []byte) ([]byte, error) {
	g, err := k.gcm()
	if err != nil {
		return nil, err
	}
	if len(sealed) < g.NonceSize()+g.Overhead() {
		return nil, ErrOpen
	}
	n := g.NonceSize()
	plain, err := g.Open(nil, sealed[:n], sealed[n:], []byte(name))
	if err != nil {
		return nil, ErrOpen
	}
	return plain, nil
}

func (k *Keys) gcm() (cipher.AEAD, error) {
	b, err := aes.NewCipher(k.enc[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(b)
}

// Meta é o arquivo de metadados da pasta (radlog-nuvem.json), em claro.
type Meta struct {
	V     int    `json:"v"`
	Salt  []byte `json:"salt"`
	Check []byte `json:"check"`
}

// MetaFile é o nome do arquivo de metadados na pasta.
const MetaFile = "radlog-nuvem.json"

// NewMeta cria os metadados de uma nuvem nova para o código e devolve as
// chaves.
func NewMeta(code string) (Meta, *Keys, error) {
	salt, err := NewSalt()
	if err != nil {
		return Meta{}, nil, err
	}
	k, err := DeriveKeys(code, salt)
	if err != nil {
		return Meta{}, nil, err
	}
	chk, err := k.Seal(MetaFile, []byte(metaCheck))
	if err != nil {
		return Meta{}, nil, err
	}
	return Meta{V: MetaVersion, Salt: salt, Check: chk}, k, nil
}

// Unlock confere o código com os metadados e devolve as chaves, ou ErrOpen.
func (m Meta) Unlock(code string) (*Keys, error) {
	switch {
	case m.V < MetaVersion:
		return nil, ErrOldCloud
	case m.V > MetaVersion:
		return nil, errors.New("nuvem de uma versão mais nova do RadLog")
	}
	k, err := DeriveKeys(code, m.Salt)
	if err != nil {
		return nil, err
	}
	b, err := k.Open(MetaFile, m.Check)
	if err != nil || string(b) != metaCheck {
		return nil, ErrOpen
	}
	return k, nil
}
