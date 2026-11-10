package radiacode

import (
	"testing"
	"time"
)

// O atraso da memória chega em lote (eid 1), com o bit "dado antigo".
func TestDataBufOldBatch(t *testing.T) {
	base := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	var b []byte
	// lote de 3 pontos da taxa, um a cada 96 s, começando 2 h antes
	b = append(b, le(uint8(0), uint8(1), uint8(2), int32(-720000), uint16(3), uint32(9600))...)
	for i := 0; i < 3; i++ {
		b = append(b, le(uint32(1200+i), float32(12.5), float32(1.4e-5), uint16(200), uint16(0xB000))...)
	}
	// lote de 2 raros
	b = append(b, le(uint8(1), uint8(1), uint8(3), int32(-700000), uint16(2), uint32(6000))...)
	for i := 0; i < 2; i++ {
		b = append(b, le(uint32(1000+60*i), float32(7e-6), uint16(4500), uint16(9900), uint16(0x9040))...)
	}
	// alarme de taxa (traz taxa e erro) e depois um evento normal
	b = append(b, le(uint8(2), uint8(0), uint8(7), int32(-500000), uint8(9), uint8(0), uint16(0x9001), float32(5e-5), uint16(100))...)
	b = append(b, le(uint8(3), uint8(0), uint8(7), int32(-400000), uint8(7), uint8(0), uint16(0x9000))...)
	// contador de dose (6 bytes) e um ponto ao vivo
	b = append(b, le(uint8(4), uint8(0), uint8(4), int32(-1), uint32(123), uint16(0))...)
	b = append(b, le(uint8(5), uint8(0), uint8(2), int32(100), uint32(1300), float32(13), float32(1.5e-5), uint16(200), uint16(0x1000))...)

	recs := decodeDataBuf(b, base)
	var db []DoseRateDB
	var rare []Rare
	var ev []Event
	for _, r := range recs {
		switch v := r.(type) {
		case DoseRateDB:
			db = append(db, v)
		case Rare:
			rare = append(rare, v)
		case Event:
			ev = append(ev, v)
		}
	}
	if len(db) != 4 || len(rare) != 2 || len(ev) != 2 {
		t.Fatalf("registros: %d taxa, %d raros, %d eventos", len(db), len(rare), len(ev))
	}
	for i := 0; i < 3; i++ {
		want := base.Add(-2*time.Hour + time.Duration(i)*96*time.Second)
		if !db[i].Old() || !db[i].T.Equal(want) || db[i].Count != uint32(1200+i) {
			t.Errorf("ponto %d: %+v (quero %v)", i, db[i], want)
		}
	}
	if db[3].Old() || db[3].Count != 1300 {
		t.Errorf("ponto ao vivo: %+v", db[3])
	}
	if !rare[1].Old() || rare[1].Duration != 1060 || !rare[1].T.Equal(base.Add(-7000*time.Second+60*time.Second)) {
		t.Errorf("raro: %+v", rare[1])
	}
	if ev[0].ID != 9 || ev[1].ID != 7 || !ev[1].Old() {
		t.Errorf("eventos: %+v", ev)
	}
}

// Lote que diz ter mais pontos do que cabe: não inventa nada.
func TestDataBufBatchCut(t *testing.T) {
	b := le(uint8(0), uint8(1), uint8(2), int32(0), uint16(50), uint32(9600), uint32(1))
	if recs := decodeDataBuf(b, time.Now()); len(recs) != 0 {
		t.Fatalf("lote cortado virou %d registros", len(recs))
	}
}
