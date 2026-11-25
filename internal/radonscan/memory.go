package radonscan

// Memória do RadonScan (2.106.0). Achado numa cópia só de leitura da flash
// de 2 MB (<SPIR>>, o mesmo comando de leitura do GMC-800) feita em
// 24/11/2026 no aparelho do ACE (RadonScanRe2.02):
//
//   - 0x1F9000: tabela de HourSlots entradas de 4 bytes; o 1º byte é quantas
//     contagens o detector teve numa hora. 1 contagem por hora vale 1/24
//     pCi/L (é o degrau da média de 1 hora: 0,04; 0,08; 0,13...). Conferido
//     contra 11 horas que o RadLog leu ao vivo: bateu dentro do arredondamento.
//   - 0x1FE000: um registro de 14 bytes por hora: AA 55, segundos ligado (4
//     bytes, big-endian) e 8 bytes zerados.
//
// Uma segunda cópia (25/11/2026, depois de o aparelho ficar 15 h fora do PC
// ligado num carregador) mostrou como a tabela anda: a hora mais nova fica
// sempre na entrada HourNewest e as outras vão descendo uma entrada por
// hora. Os segundos ligado não zeram quando ele fica sem energia (ele não
// tem bateria): a conta para e continua de onde estava quando ele volta.
// A média de 1 dia que ele mostra bate com a soma das 24 entradas que
// terminam em HourNewest.
//
// O aparelho não diz qual entrada é a hora de agora nem tem relógio: o app
// descobre isso comparando a tabela com as médias que ele mesmo gravou
// (ver app/radonmem.go). Só leitura: nada aqui escreve no aparelho.

import (
	"encoding/binary"
	"fmt"
	"time"
)

const (
	HourTableAddr = 0x1F9000
	HourSlots     = 4253 // 17012 bytes
	// HourNewest é a entrada da hora fechada mais nova (3888 = 162 dias de
	// horas; as entradas depois dela não são horas).
	HourNewest    = 3887
	HourMarksAddr = 0x1FE000
	hourMarksLen  = 4096
	spirBlock     = 4096
	// PCiPerCount: uma contagem numa hora, em pCi/L.
	PCiPerCount = 1.0 / 24
)

// Memory é o que o app lê da memória do aparelho.
type Memory struct {
	Hours  []int // contagens por hora, HourSlots entradas
	Uptime int   // segundos ligado no total, no último registro de hora (-1 = sem registro)
}

// spir lê n bytes da flash a partir de addr.
func (d *Device) spir(addr, n int) ([]byte, error) {
	cmd := []byte("<SPIR")
	cmd = append(cmd, byte(addr>>16), byte(addr>>8), byte(addr), byte(n>>8), byte(n))
	cmd = append(cmd, '>', '>')
	d.mu.Lock()
	defer d.mu.Unlock()
	var last error
	for try := 0; try < 2; try++ {
		_ = d.port.ResetInputBuffer()
		if _, err := d.port.Write(cmd); err != nil {
			return nil, err
		}
		b, err := d.readN(n, 4*time.Second)
		if err == nil {
			return b, nil
		}
		last = err
	}
	return nil, fmt.Errorf("SPIR 0x%06X: %w", addr, last)
}

// readN lê exatamente n bytes (ou desiste depois de max).
func (d *Device) readN(n int, max time.Duration) ([]byte, error) {
	if err := d.port.SetReadTimeout(pollStep); err != nil {
		return nil, err
	}
	out := make([]byte, 0, n)
	tmp := make([]byte, 1024)
	start := time.Now()
	for len(out) < n && time.Since(start) < max {
		k, err := d.port.Read(tmp[:min(len(tmp), n-len(out))])
		if err != nil {
			return out, err
		}
		out = append(out, tmp[:k]...)
	}
	if len(out) < n {
		return out, ErrTimeout
	}
	return out, nil
}

// ReadMemory lê a tabela de contagens por hora e os registros de hora
// (uns 5 blocos de 4 KB, poucos segundos).
func (d *Device) ReadMemory() (Memory, error) {
	var raw []byte
	total := HourSlots * 4
	for off := 0; off < total; off += spirBlock {
		n := min(spirBlock, total-off)
		b, err := d.spir(HourTableAddr+off, n)
		if err != nil {
			return Memory{}, err
		}
		raw = append(raw, b...)
	}
	marks, err := d.spir(HourMarksAddr, hourMarksLen)
	if err != nil {
		return Memory{}, err
	}
	return ParseMemory(raw, marks)
}

// ParseMemory interpreta os bytes lidos (separado para os testes).
func ParseMemory(table, marks []byte) (Memory, error) {
	if len(table) < HourSlots*4 {
		return Memory{}, fmt.Errorf("tabela de horas curta: %d bytes", len(table))
	}
	m := Memory{Hours: make([]int, HourSlots), Uptime: -1}
	erased := 0
	for i := range m.Hours {
		e := table[i*4 : i*4+4]
		if e[0] == 0xFF && e[1] == 0xFF && e[2] == 0xFF && e[3] == 0xFF {
			erased++ // flash apagada: sem dado
			continue
		}
		m.Hours[i] = int(e[0])
	}
	if erased == HourSlots {
		return Memory{}, fmt.Errorf("a memória do RadonScan está vazia")
	}
	for i := 0; i+14 <= len(marks); i += 14 {
		r := marks[i : i+14]
		if r[0] != 0xAA || r[1] != 0x55 {
			break
		}
		if s := int(binary.BigEndian.Uint32(r[2:6])); s > m.Uptime {
			m.Uptime = s
		}
	}
	return m, nil
}
