package radiacode

import (
	"encoding/hex"
	"testing"
	"time"
)

// Fuzz dos decodificadores: os bytes chegam do USB e do Bluetooth (que perde
// e corta pacotes), então nenhum formato estranho pode dar panic. Rodar mais
// tempo com, por exemplo:
//
//	go test -run '^$' -fuzz FuzzDecodeDataBuf -fuzztime 1m ./internal/radiacode/
//
// Entradas que já derrubaram o decodificador ficam em testdata/fuzz/ e rodam
// sempre no go test normal.

// dataBuf415 é a resposta real de um RC-102 com firmware 4.15 (proto_test.go).
const dataBuf415 = "8f0000e31d00009d284841d50278372900960040000000900101434000000200320000000000c0405f294b3600002041a69bc436910003524000005d0c0000931a5a373f128a174010"

func dataBufSeeds(f *testing.F) {
	b, _ := hex.DecodeString(dataBuf415)
	f.Add(b)
	f.Add([]byte{})
	f.Add(b[:len(b)/2])
	f.Add(le(uint8(7), uint8(0), uint8(7), int32(0), uint8(1), uint8(0), uint16(0)))
	f.Add(le(uint8(2), uint8(1), uint8(2), int32(160), uint16(0xffff), uint32(500)))
	f.Add(le(uint8(0), uint8(0), uint8(0), int32(-1)))
}

func FuzzDecodeDataBuf(f *testing.F) {
	dataBufSeeds(f)
	base := time.Unix(1_700_000_000, 0)
	f.Fuzz(func(t *testing.T, b []byte) {
		_ = decodeDataBuf(b, base)
	})
}

func FuzzSplitDataBuf(f *testing.F) {
	dataBufSeeds(f)
	base := time.Unix(1_700_000_000, 0)
	f.Fuzz(func(t *testing.T, b []byte) {
		for _, r := range SplitDataBuf(b, base) {
			_ = r.Name
		}
	})
}

func FuzzDecodeSpectrum(f *testing.F) {
	f.Add(le(uint32(60), float32(-5), float32(2.4), float32(0.0004), uint32(1), uint32(2), uint32(3)), 0)
	f.Add(le(uint32(60), float32(0), float32(1), float32(0), uint8(0x11), uint8(3)), 1)
	f.Add([]byte{}, 1)
	f.Add(le(uint32(1), float32(0), float32(0), float32(0), uint8(0xff), uint8(0xff), uint8(0xff)), 1)
	f.Fuzz(func(t *testing.T, b []byte, format int) {
		_, _ = decodeSpectrum(b, format&3)
	})
}

// FuzzAssembler manda os bytes em pedaços de tamanho variado, como chegam
// pelo Bluetooth (notificações de ~20 bytes) e pelo USB (blocos de 256).
func FuzzAssembler(f *testing.F) {
	f.Add(append(le(uint32(4)), 1, 2, 3, 4), uint8(2))
	f.Add(le(uint32(1<<21)), uint8(20))
	f.Add([]byte{1, 2}, uint8(1))
	f.Fuzz(func(t *testing.T, b []byte, chunk uint8) {
		n := int(chunk%64) + 1
		a := newAssembler()
		for len(b) > 0 {
			k := min(n, len(b))
			out, done, err := a.add(b[:k])
			b = b[k:]
			if err != nil {
				a = newAssembler()
				continue
			}
			if done && out == nil && a.want != -1 {
				t.Fatal("resposta completa sem voltar a esperar cabeçalho")
			}
		}
	})
}

// FuzzDevice abre um aparelho falso e lê DATA_BUF e espectro com respostas
// estranhas, passando pelo caminho todo (execute, readString, decodificação).
func FuzzDevice(f *testing.F) {
	b, _ := hex.DecodeString(dataBuf415)
	f.Add(b, le(uint32(60), float32(0), float32(1), float32(0), uint32(5)))
	f.Add([]byte{0xff}, []byte{})
	f.Fuzz(func(t *testing.T, db, spec []byte) {
		d, err := Open(&fakeRC{dataBuf: db, spec: spec})
		if err != nil {
			return
		}
		_, _ = d.DataBuf()
		_, _, _ = d.DataBufRaw()
		_, _ = d.Spectrum()
		_, _ = d.SpectrumAccum()
	})
}
