package radiacode

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
	"time"
)

// fakeRC responde como um RadiaCode com firmware 4.12.
type fakeRC struct {
	dataBuf []byte
	spec    []byte
	cmds    []uint16
}

func le(vs ...interface{}) []byte {
	var b bytes.Buffer
	for _, v := range vs {
		binary.Write(&b, binary.LittleEndian, v)
	}
	return b.Bytes()
}

func (f *fakeRC) Exchange(req []byte) ([]byte, error) {
	n := binary.LittleEndian.Uint32(req)
	if int(n) != len(req)-4 {
		panic("tamanho errado")
	}
	hdr := req[4:8]
	cmd := binary.LittleEndian.Uint16(hdr)
	f.cmds = append(f.cmds, cmd)
	args := req[8:]
	var body []byte
	str := func(data []byte) []byte { return append(le(uint32(1), uint32(len(data))), data...) }
	switch cmd {
	case cmdGetVersion:
		body = append(le(uint16(1), uint16(4)), 4)
		body = append(body, "boot"...)
		body = append(body, le(uint16(12), uint16(4))...)
		body = append(body, 11)
		body = append(body, "Sep 10 2026\x00"[:11]...)
	case cmdRdVirtString:
		switch binary.LittleEndian.Uint32(args) {
		case vsConfiguration:
			body = str([]byte("DeviceName=RadiaCode-102\nSpecFormatVersion=1\n"))
		case vsSerialNumber:
			body = append(str([]byte("RC-102-001234")), 0) // o 0 a mais do firmware novo
		case vsDataBuf:
			body = str(f.dataBuf)
		case vsSpectrum:
			body = str(f.spec)
		}
	case cmdWrVirtSFR:
		body = le(uint32(1))
	case cmdRdVirtSFRBat:
		body = le(uint32(0xff), uint32(100), uint32(500), uint32(30), uint32(100), uint32(100000), uint32(200000), uint32(1<<24), uint32(0))
	}
	out := append([]byte{}, hdr...)
	return append(out, body...), nil
}
func (f *fakeRC) Close() error { return nil }
func (f *fakeRC) Kind() string { return "usb" }

func TestOpenAndRead(t *testing.T) {
	// DATA_BUF: tempo real, histórico, raro, evento, e um tipo desconhecido no fim
	var db []byte
	db = append(db, le(uint8(0), uint8(0), uint8(0), int32(100))...)
	db = append(db, le(float32(4.5), float32(1.2e-5), uint16(150), uint16(200), uint16(0), uint8(0))...)
	db = append(db, le(uint8(1), uint8(0), uint8(2), int32(200))...)
	db = append(db, le(uint32(270), float32(4.5), float32(1.1e-5), uint16(120), uint16(0))...)
	db = append(db, le(uint8(2), uint8(0), uint8(3), int32(300))...)
	db = append(db, le(uint32(3600), float32(2.5e-4), uint16(2000+2450), uint16(8700), uint16(0))...)
	db = append(db, le(uint8(3), uint8(0), uint8(7), int32(400))...)
	db = append(db, le(uint8(9), uint8(1), uint16(0))...)
	db = append(db, le(uint8(4), uint8(0), uint8(1), int32(500))...)
	db = append(db, le(float32(1), float32(1))...) // dados crus: pulados
	db = append(db, le(uint8(5), uint8(9), uint8(9), int32(600), uint32(0))...)
	// espectro v1: 3 canais: 10 (vlen 1), 12 (vlen 2: +2), 0 (vlen 0)… em grupos
	var sp []byte
	sp = append(sp, le(uint32(60), float32(-6), float32(2.45), float32(0.0004))...)
	sp = append(sp, le(uint16(1<<4|1), uint8(10))...)
	sp = append(sp, le(uint16(1<<4|2), int8(2))...)
	sp = append(sp, le(uint16(2<<4|0))...)
	sp = append(sp, le(uint16(1<<4|3), int16(-5))...)
	f := &fakeRC{dataBuf: db, spec: sp}
	d, err := Open(f)
	if err != nil {
		t.Fatal(err)
	}
	if d.Serial != "RC-102-001234" || d.Model != "RadiaCode-102" || d.specFmt != 1 || d.Firmware != "4.12 (Sep 10 2026)" {
		t.Fatalf("aparelho: %q %q %d %q", d.Serial, d.Model, d.specFmt, d.Firmware)
	}
	if f.cmds[0] != cmdSetExchange || f.cmds[1] != cmdSetTime {
		t.Fatalf("ordem do início: %x", f.cmds)
	}
	recs, err := d.DataBuf()
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 4 {
		t.Fatalf("registros: %d %+v", len(recs), recs)
	}
	rt := recs[0].(RealTime)
	if rt.CPS != 4.5 || math.Abs(rt.DoseRate-0.12) > 1e-6 || rt.CPSErr != 15 || rt.DoseErr != 20 {
		t.Fatalf("tempo real: %+v", rt)
	}
	if dt := rt.T.Sub(d.base); dt != time.Second {
		t.Fatalf("tempo: %v", dt)
	}
	if h := recs[1].(DoseRateDB); h.Count != 270 || math.Abs(h.DoseRate-0.11) > 1e-6 {
		t.Fatalf("histórico: %+v", h)
	}
	if r := recs[2].(Rare); math.Abs(r.Dose-2.5) > 1e-6 || r.TempC != 24.5 || r.Battery != 87 {
		t.Fatalf("raro: %+v", r)
	}
	if e := recs[3].(Event); e.ID != 9 || EventName(e.ID) != "alarme 1 de taxa de dose" {
		t.Fatalf("evento: %+v", e)
	}
	s, err := d.Spectrum()
	if err != nil {
		t.Fatal(err)
	}
	if want := []uint32{10, 12, 0, 0, 0}; len(s.Counts) != 5 || s.Counts[0] != 10 || s.Counts[1] != 12 || s.Counts[2] != 0 || s.Counts[4] != 0 {
		t.Fatalf("espectro: %v (quero %v)", s.Counts, want)
	}
	if s.Duration != time.Minute || math.Abs(s.Energy(100)-(-6+245+4)) > 1e-3 {
		t.Fatalf("calibração: %+v", s)
	}
	al, err := d.Alarms()
	if err != nil {
		t.Fatal(err)
	}
	if al.CountRate1 != 10 || al.DoseRate1 != 0.3 || al.Dose2 != 2000 || !al.ShowSv || al.ShowCPM {
		t.Fatalf("alarmes: %+v", al)
	}
}

func TestAssembler(t *testing.T) {
	a := newAssembler()
	msg := append(le(uint32(6)), 1, 2, 3, 4, 5, 6)
	if _, done, _ := a.add(msg[:5]); done {
		t.Fatal("cedo demais")
	}
	out, done, err := a.add(msg[5:])
	if err != nil || !done || !bytes.Equal(out, []byte{1, 2, 3, 4, 5, 6}) {
		t.Fatalf("%v %v %v", out, done, err)
	}
	if _, _, err := a.add(append(le(uint32(1)), 1, 2)); err == nil {
		t.Fatal("deveria recusar resposta maior")
	}
}

func TestDataBufSeqJump(t *testing.T) {
	var db []byte
	db = append(db, le(uint8(7), uint8(0), uint8(7), int32(0), uint8(1), uint8(0), uint16(0))...)
	db = append(db, le(uint8(9), uint8(0), uint8(7), int32(0), uint8(1), uint8(0), uint16(0))...)
	if n := len(decodeDataBuf(db, time.Now())); n != 1 {
		t.Fatalf("pulo de sequência: %d registros", n)
	}
}

func TestSplitDataBuf(t *testing.T) {
	var db []byte
	db = append(db, le(uint8(0), uint8(0), uint8(0), int32(100))...)
	db = append(db, le(float32(4.5), float32(1.2e-5), uint16(150), uint16(200), uint16(0), uint8(3))...)
	db = append(db, le(uint8(1), uint8(0), uint8(8), int32(150), float32(7), uint16(1))...)
	db = append(db, le(uint8(2), uint8(1), uint8(2), int32(160), uint16(2), uint32(500))...)
	db = append(db, bytes.Repeat([]byte{0xAB}, 32)...)
	db = append(db, le(uint8(3), uint8(0), uint8(42), int32(170), uint32(0xdeadbeef))...)
	base := time.Unix(1000, 0)
	rs := SplitDataBuf(db, base)
	if len(rs) != 4 {
		t.Fatalf("%d registros: %+v", len(rs), rs)
	}
	if rs[0].Name != "RealTimeData" || rs[0].Fields["count_rate"] != 4.5 || rs[0].Fields["rt_flags"] != 3 || len(rs[0].Data) != 7+15 {
		t.Fatalf("tempo real: %+v", rs[0])
	}
	if rs[1].Name != "RawCountRate" || rs[1].Fields["count_rate"] != 7 || !rs[1].T.Equal(base.Add(1500*time.Millisecond)) {
		t.Fatalf("contagem crua: %+v", rs[1])
	}
	if rs[2].Name != "Bloco2" || rs[2].Fields["samples"] != 2 || len(rs[2].Data) != 7+6+32 {
		t.Fatalf("bloco: %+v", rs[2])
	}
	if rs[3].Name != "Desconhecido" || len(rs[3].Data) != 11 {
		t.Fatalf("desconhecido: %+v", rs[3])
	}
}
