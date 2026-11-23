// Package radonscan lê o detector de radônio GQ RadonScan pela porta serial
// (USB).
//
// A GQ cita um protocolo próprio (GQ-RFC2401) que nunca foi publicado. O que
// está aqui veio de uma sondagem só de leitura num RadonScan de verdade
// (firmware RadonScanRe2.02, 23/11/2026): mesmo chip USB do GMC-800 (CH340,
// 1A86:7523), 115200 baud, comandos no formato <COMANDO>>. As médias de
// radônio voltam em texto, em pCi/L, terminadas em CR LF ("0.33\r\n").
//
// Este pacote só manda comandos de LEITURA. Nada de SET, REBOOT, reset de
// fábrica ou Wi-Fi: o app nunca muda o aparelho.
package radonscan

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"radlog/internal/gmc"
)

// Baud é a velocidade do RadonScan (a 57600 ele não responde).
const Baud = 115200

// BqPerPCiL converte pCi/L em Bq/m³ (1 pCi/L = 37 Bq/m³).
const BqPerPCiL = 37.0

// ErrTimeout: o aparelho não respondeu a tempo.
var ErrTimeout = errors.New("o RadonScan não respondeu a tempo")

const pollStep = 50 * time.Millisecond

// Device é um RadonScan conectado. Seguro para uso concorrente.
type Device struct {
	mu   sync.Mutex
	port gmc.Port
}

// New embrulha uma porta já aberta.
func New(p gmc.Port) *Device { return &Device{port: p} }

// Close fecha a porta.
func (d *Device) Close() error { return d.port.Close() }

// IsRadonScan diz se a resposta do <GETVER>> é de um detector de radônio da
// GQ (RadonScan, RadonScan-Plus ou RadonPro).
func IsRadonScan(ver string) bool {
	return strings.HasPrefix(ver, "RadonScan") || strings.HasPrefix(ver, "RadonPro")
}

// ask manda o comando e lê até a linha ficar em silêncio. Logo depois de
// abrir a porta, a primeira resposta às vezes não vem (visto na sondagem):
// tenta mais uma vez antes de desistir.
func (d *Device) ask(cmd string) ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	var err error
	for try := 0; try < 2; try++ {
		_ = d.port.ResetInputBuffer()
		if _, err = d.port.Write([]byte(cmd)); err != nil {
			return nil, err
		}
		var b []byte
		b, err = d.readIdle(150*time.Millisecond, 1500*time.Millisecond)
		if err == nil {
			return b, nil
		}
	}
	return nil, fmt.Errorf("%s: %w", strings.Trim(cmd, "<>"), err)
}

func (d *Device) readIdle(idle, max time.Duration) ([]byte, error) {
	if err := d.port.SetReadTimeout(pollStep); err != nil {
		return nil, err
	}
	var out []byte
	tmp := make([]byte, 64)
	start, last := time.Now(), time.Now()
	for time.Since(start) < max {
		k, err := d.port.Read(tmp)
		if err != nil {
			return out, err
		}
		if k > 0 {
			out = append(out, tmp[:k]...)
			last = time.Now()
			continue
		}
		if len(out) > 0 && time.Since(last) >= idle {
			break
		}
	}
	if len(out) == 0 {
		return nil, ErrTimeout
	}
	return out, nil
}

// Version devolve algo como "RadonScanRe2.02".
func (d *Device) Version() (string, error) {
	b, err := d.ask("<GETVER>>")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// Averages são as quatro médias que o aparelho guarda, em pCi/L.
type Averages struct {
	Hour, Day, Week, Long float64
}

// number lê uma resposta em texto ("0.33\r\n").
func (d *Device) number(cmd string) (float64, error) {
	b, err := d.ask(cmd)
	if err != nil {
		return 0, err
	}
	return ParseNumber(b)
}

// ParseNumber lê o número em texto que o RadonScan manda ("0.33\r\n").
func ParseNumber(b []byte) (float64, error) {
	s := strings.TrimSpace(string(b))
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 || v > 1e5 {
		return 0, fmt.Errorf("resposta inesperada do RadonScan: %q", s)
	}
	return v, nil
}

// ReadAverages lê as quatro médias (1 hora, 1 dia, 1 semana, longo prazo).
func (d *Device) ReadAverages() (Averages, error) {
	var a Averages
	var err error
	if a.Hour, err = d.number("<GETPCI1HOUR>>"); err != nil {
		return a, err
	}
	if a.Day, err = d.number("<GETPCI1DAY>>"); err != nil {
		return a, err
	}
	if a.Week, err = d.number("<GETPCI1WEEK>>"); err != nil {
		return a, err
	}
	if a.Long, err = d.number("<GETPCILONGTERM>>"); err != nil {
		return a, err
	}
	return a, nil
}

// Open abre o RadonScan na porta indicada.
func Open(name string) (*Device, error) {
	p, err := gmc.OpenPort(name)
	if err != nil {
		return nil, err
	}
	return New(p), nil
}
