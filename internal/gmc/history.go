package gmc

import (
	"time"
)

// Record é uma contagem gravada na memória do contador.
type Record struct {
	Time     time.Time
	Count    uint32
	Unit     string        // "CPS" ou "CPM"
	Interval time.Duration // 1 s, 1 min ou 1 h
}

// Note é uma anotação gravada no histórico (local, observação).
type Note struct {
	Time time.Time
	Text string
}

// History é o resultado da leitura da flash.
type History struct {
	Records []Record
	Notes   []Note
}

// ParseHistory interpreta o conteúdo bruto da memória de histórico.
//
// Formato (GQ-RFC1201, igual ao que o PyGMC lê do GMC-800):
//   - 55 AA 00 YY MM DD HH MI SS 55 AA modo  → marca de tempo e modo de gravação
//     (modo: 0 desligado, 1 CPS/s, 2 CPM/min, 3 CPM/h, 4 CPS limiar, 5 CPM limiar)
//   - 55 AA 01 hi lo                → contagem de 2 bytes
//   - 55 AA 03 b2 b1 b0             → contagem de 3 bytes
//   - 55 AA 04 b3 b2 b1 b0          → contagem de 4 bytes
//   - 55 AA 02 len texto...         → anotação
//   - 55 AA 05 tubo                 → seleção de tubo (ignorada)
//   - qualquer outro byte           → contagem de 1 byte
//
// Cada contagem fica um intervalo depois da anterior (a primeira fica um
// intervalo depois da marca de tempo). A memória vazia é 0xFF.
func ParseHistory(data []byte) History {
	var h History
	var cur time.Time
	var unit string
	var step time.Duration
	haveCtx := false

	add := func(v uint32) {
		if !haveCtx || step == 0 {
			return
		}
		cur = cur.Add(step)
		h.Records = append(h.Records, Record{Time: cur, Count: v, Unit: unit, Interval: step})
	}

	i := 0
	n := len(data)
	for i < n {
		b := data[i]
		if b == 0xFF && restIsEmpty(data[i:]) {
			break
		}
		if b == 0x55 && i+2 < n && data[i+1] == 0xAA {
			switch data[i+2] {
			case 0x00:
				if i+12 > n {
					return h
				}
				p := data[i+3 : i+12]
				t, ok := makeTime(p[0], p[1], p[2], p[3], p[4], p[5])
				if ok {
					cur = t
					unit, step = modeInfo(p[8])
					haveCtx = step != 0
				} else {
					haveCtx = false
				}
				i += 12
				continue
			case 0x01:
				if i+5 > n {
					return h
				}
				add(uint32(data[i+3])<<8 | uint32(data[i+4]))
				i += 5
				continue
			case 0x03:
				if i+6 > n {
					return h
				}
				add(uint32(data[i+3])<<16 | uint32(data[i+4])<<8 | uint32(data[i+5]))
				i += 6
				continue
			case 0x04:
				if i+7 > n {
					return h
				}
				add(uint32(data[i+3])<<24 | uint32(data[i+4])<<16 | uint32(data[i+5])<<8 | uint32(data[i+6]))
				i += 7
				continue
			case 0x02:
				if i+4 > n {
					return h
				}
				l := int(data[i+3])
				end := i + 4 + l
				if end > n {
					return h
				}
				if haveCtx {
					h.Notes = append(h.Notes, Note{Time: cur, Text: string(data[i+4 : end])})
				}
				i = end
				continue
			case 0x05:
				i += 4
				continue
			}
		}
		add(uint32(b))
		i++
	}
	return h
}

// restIsEmpty diz se daqui em diante só tem 0xFF por pelo menos 64 bytes
// (ou até o fim). Uma contagem real de 255 seguida de mais dados não conta.
func restIsEmpty(b []byte) bool {
	limit := 64
	if len(b) < limit {
		limit = len(b)
	}
	for _, c := range b[:limit] {
		if c != 0xFF {
			return false
		}
	}
	return true
}

func modeInfo(m byte) (string, time.Duration) {
	switch m {
	case 1, 4:
		return "CPS", time.Second
	case 2, 5:
		return "CPM", time.Minute
	case 3:
		return "CPM", time.Hour
	}
	return "", 0
}
