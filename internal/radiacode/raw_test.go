package radiacode

import (
	"encoding/binary"
	"testing"
	"time"
)

// rawRC é o fakeRC com as mensagens do aparelho e um registrador recusado.
type rawRC struct {
	fakeRC
	text   []byte
	refuse bool
}

func (f *rawRC) Exchange(req []byte) ([]byte, error) {
	hdr, cmd, args := req[4:8], binary.LittleEndian.Uint16(req[4:]), req[8:]
	switch {
	case cmd == cmdRdVirtString && binary.LittleEndian.Uint32(args) == 0xF:
		out := append([]byte{}, hdr...)
		out = append(out, le(uint32(1), uint32(len(f.text)))...)
		return append(out, f.text...), nil
	case cmd == cmdRdVirtSFRBat && f.refuse:
		return append(append([]byte{}, hdr...), le(uint32(0), uint32(0))...), nil
	}
	return f.fakeRC.Exchange(req)
}

func TestIsRadiacodeName(t *testing.T) {
	for name, want := range map[string]bool{
		"RadiaCode-102": true, "radiacode 103": true, "RC-102-001234": true, "rc-110": true,
		"": false, "GMC-800": false, "Fone do Zé": false, "RC-2": false,
	} {
		if got := isRadiacodeName(name); got != want {
			t.Errorf("isRadiacodeName(%q) = %v", name, got)
		}
	}
}

func TestCp1251(t *testing.T) {
	// "BLE: " + "Привет" em cp1251 + Ё ё + um byte sem tradução
	in := append([]byte("BLE: "), 0xCF, 0xF0, 0xE8, 0xE2, 0xE5, 0xF2, ' ', 0xA8, 0xB8, 0x81)
	if got, want := cp1251(in), "BLE: Привет Ёё?"; got != want {
		t.Fatalf("cp1251 = %q, quero %q", got, want)
	}
}

func TestTextMessages(t *testing.T) {
	f := &rawRC{text: []byte("-12, BLE: client connected\n\n-3,Low battery\r\nsem numero\x00\x00")}
	d, err := Open(f)
	if err != nil {
		t.Fatal(err)
	}
	got, err := d.TextMessages()
	if err != nil {
		t.Fatal(err)
	}
	want := []TextLine{{12, "BLE: client connected"}, {3, "Low battery"}, {0, "sem numero"}}
	if len(got) != len(want) {
		t.Fatalf("mensagens: %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("mensagem %d: %+v, quero %+v", i, got[i], want[i])
		}
	}
}

func TestReadSFRAndVS(t *testing.T) {
	f := &rawRC{}
	d, err := Open(f)
	if err != nil {
		t.Fatal(err)
	}
	if v, err := d.ReadSFR(0x8000); err != nil || v != 100 {
		t.Fatalf("ReadSFR = %d, %v", v, err)
	}
	f.refuse = true
	if _, err := d.ReadSFR(0x8000); err == nil {
		t.Fatal("registrador recusado passou")
	}
	b, err := d.ReadVS(vsSerialNumber)
	if err != nil || string(b) != "RC-102-001234" {
		t.Fatalf("ReadVS = %q, %v", b, err)
	}
	raw, err := d.ReadVSRaw(vsSerialNumber)
	if err != nil || len(raw) != 8+len("RC-102-001234")+1 {
		t.Fatalf("ReadVSRaw: %d bytes, %v", len(raw), err)
	}
}

func TestDataBufRawRoundTrip(t *testing.T) {
	var db []byte
	db = append(db, le(uint8(0), uint8(0), uint8(0), int32(100))...)
	db = append(db, le(float32(4.5), float32(1.2e-5), uint16(150), uint16(200), uint16(0), uint8(0))...)
	d, err := Open(&rawRC{fakeRC: fakeRC{dataBuf: db}})
	if err != nil {
		t.Fatal(err)
	}
	b, recs, err := d.DataBufRaw()
	if err != nil || len(recs) != 1 {
		t.Fatalf("DataBufRaw: %d registros, %v", len(recs), err)
	}
	// a resposta guardada, lida depois, dá os mesmos registros
	again := DecodeDataBuf(b, d.Base())
	if len(again) != 1 || again[0].When() != recs[0].When() || !recs[0].When().After(d.Base()) {
		t.Fatalf("DecodeDataBuf: %+v", again)
	}
	if again[0].When().Sub(d.Base()) != time.Second {
		t.Fatalf("tempo: %v", again[0].When().Sub(d.Base()))
	}
}
