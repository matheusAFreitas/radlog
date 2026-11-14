package sync

import (
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

// vetores do docs/nuvem-formato.md: o Android (e qualquer outro leitor) tem
// que chegar nos mesmos valores
const (
	vecCode    = "K7QF-3XWM-9ABC-DEFG-HJKM-NPQR"
	vecDev     = "0011223344556677"
	vecEnc     = "e77684f9861da824d669fb7672691fc1e011378a7ffe9b031ebca178c5716184"
	vecName    = "5ac18db1b9a79dfaa2b9c757562d8b6e9e49b3f19a78c773649bd16f6d7c7eb0"
	vecPrefix  = "0ce3f80b"
	vecCommon  = "0ce3f80b47db492ec604b7cc7ede40b2"
	vecDevFile = "0ce3f80b16eaf95f0227232136a5fecf"
	vecSealed  = "000000000000000000000000cf794b1dac5031aa2d0cee0c826097dd0abf758a1286871ebd4cb84324a828d7c9f837fef8f1673848b5ecfe75b5a93eb34c246f0407a85a87ed6fb1c58ccd7143ce0738195b929d995d52f42289d6664404e087a3e66c9d92"
	vecPlain   = `{"id":"0011223344556677","name":"pc","lastUp":0,"writer":"w","seq":1}`
)

func vecSalt() []byte {
	s := make([]byte, 32)
	for i := range s {
		s[i] = byte(i)
	}
	return s
}

func TestFormatVectors(t *testing.T) {
	code, err := ParseRecoveryCode(vecCode)
	if err != nil {
		t.Fatal(err)
	}
	k, err := DeriveKeys(code, vecSalt())
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(k.enc[:]) != vecEnc || hex.EncodeToString(k.name[:]) != vecName {
		t.Fatal("chaves")
	}
	if k.Prefix(vecDev) != vecPrefix || k.Name(vecDev, "comum") != vecCommon || k.Name(vecDev, "aparelho") != vecDevFile {
		t.Fatalf("nomes: %s %s %s", k.Prefix(vecDev), k.Name(vecDev, "comum"), k.Name(vecDev, "aparelho"))
	}
	sealed, _ := hex.DecodeString(vecSealed)
	plain, err := k.Open(vecDevFile, sealed)
	if err != nil {
		t.Fatal(err)
	}
	kind, body, err := Unwrap(plain)
	if err != nil || kind != KindDevice || string(body) != vecPlain {
		t.Fatalf("envelope: %c %q %v", kind, body, err)
	}
	// com outro nome (arquivo trocado de lugar) não abre
	if _, err := k.Open(vecCommon, sealed); !errors.Is(err, ErrOpen) {
		t.Fatal("abriu com outro nome")
	}
}

func TestFormatEnvelope(t *testing.T) {
	b := Wrap(KindDay, []byte("zip"))
	if string(b[:2]) != "RL" || b[2] != FormatVersion || b[3] != KindDay {
		t.Fatalf("cabeçalho: %q", b[:4])
	}
	if k, body, err := Unwrap(b); err != nil || k != KindDay || string(body) != "zip" {
		t.Fatal("volta")
	}
	b[2] = FormatVersion + 1
	if _, _, err := Unwrap(b); !errors.Is(err, ErrNewer) {
		t.Fatalf("versão nova: %v", err)
	}
	for _, x := range [][]byte{nil, []byte("RL"), []byte("PK\x03\x04"), {'R', 'L', 0, 'D'}} {
		if _, _, err := Unwrap(x); !errors.Is(err, ErrFormat) {
			t.Fatalf("%q: %v", x, err)
		}
	}
}

func TestFormatNames(t *testing.T) {
	k, _ := DeriveKeys("K7QF3XWM9ABCDEFGHJKMNPQR", vecSalt())
	a, b := k.Name("aaa", "dia|2026-11-01"), k.Name("aaa", "dia|2026-11-02")
	if len(a) != 32 || a[:8] != b[:8] || a == b || !strings.HasPrefix(a, k.Prefix("aaa")) {
		t.Fatalf("mesmo aparelho: %s %s", a, b)
	}
	if k.Name("bbb", "dia|2026-11-01")[:8] == a[:8] {
		t.Fatal("prefixo igual para outro aparelho")
	}
	if strings.Contains(a, "2026") {
		t.Fatal("data no nome")
	}
}

func TestFormatAttachment(t *testing.T) {
	h := AttachmentHeader{UID: "m-1", Start: 1700000000, File: "foto.jpg"}
	b := WrapAttachment(h, []byte{1, 2, 3})
	got, data, err := UnwrapAttachment(b)
	if err != nil || got != h || string(data) != "\x01\x02\x03" {
		t.Fatalf("anexo: %+v %v %v", got, data, err)
	}
	for _, x := range [][]byte{{0, 0}, {0, 0, 0, 9, '{'}, {0, 0, 0, 2, '{', '}'}} {
		if _, _, err := UnwrapAttachment(x); !errors.Is(err, ErrFormat) {
			t.Fatalf("%v: %v", x, err)
		}
	}
}

func TestMetaVersion(t *testing.T) {
	m, _, err := NewMeta(vecCode)
	if err != nil || m.V != MetaVersion {
		t.Fatalf("meta: %+v %v", m, err)
	}
	if _, err := m.Unlock(vecCode); err != nil {
		t.Fatal(err)
	}
	m.V = 1
	if _, err := m.Unlock(vecCode); !errors.Is(err, ErrOldCloud) {
		t.Fatalf("antiga: %v", err)
	}
	m.V = MetaVersion + 1
	if _, err := m.Unlock(vecCode); err == nil {
		t.Fatal("abriu meta mais nova")
	}
}
