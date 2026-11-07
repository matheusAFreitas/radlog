// Package gmc fala com contadores Geiger GQ GMC pela porta serial (USB).
//
// O foco é o GMC-800. A GQ diz que ele segue o protocolo GQ-RFC1201, mas na
// prática algumas respostas são diferentes (CPM em 4 bytes, configuração de
// 512 bytes com layout próprio). Os detalhes vêm do manual do GMC-800 e de
// respostas gravadas de um aparelho real no projeto PyGMC.
package gmc

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// Port é o mínimo que o Device precisa de uma porta serial.
// Read deve voltar (0, nil) quando o tempo de leitura acabar.
type Port interface {
	io.ReadWriter
	ResetInputBuffer() error
	SetReadTimeout(time.Duration) error
	Close() error
}

// ErrTimeout indica que o contador não respondeu a tempo.
var ErrTimeout = errors.New("o contador não respondeu a tempo")

const (
	// FlashSize é o tamanho da memória de histórico do GMC-800 (2 MiB).
	FlashSize = 2 << 20
	// PageSize é quanto lemos da flash por comando SPIR.
	PageSize = 2048
	// ConfigSize é o tamanho da resposta de <GETCFG>> no GMC-800.
	ConfigSize = 512

	defaultTimeout = 2 * time.Second
	pollStep       = 50 * time.Millisecond
)

// Device representa um contador conectado. É seguro para uso concorrente:
// cada comando trava a porta até receber a resposta inteira.
type Device struct {
	mu        sync.Mutex
	port      Port
	closeOnce sync.Once
}

// New embrulha uma porta já aberta.
func New(p Port) *Device {
	d := &Device{port: p}
	// Se alguém deixou o "heartbeat" ligado, o contador fica mandando dados
	// sozinho e bagunça as respostas. Desliga e limpa o buffer.
	_ = d.send("<HEARTBEAT0>>")
	time.Sleep(100 * time.Millisecond)
	_ = p.ResetInputBuffer()
	return d
}

// Close fecha a porta. Não espera o comando em andamento: se a leitura ou a
// escrita travou no driver (USB suspenso, cabo mexido), esperar por ela
// travaria quem fecha (e o app não fecharia). Fechar a porta destrava a
// leitura pendente. Se nem o fechamento voltar em 2 s, desiste de esperar.
func (d *Device) Close() error {
	var err error
	d.closeOnce.Do(func() {
		done := make(chan error, 1)
		go func() { done <- d.port.Close() }()
		select {
		case err = <-done:
		case <-time.After(2 * time.Second):
			err = ErrTimeout
		}
	})
	return err
}

func (d *Device) send(cmd string) error {
	_, err := d.port.Write([]byte(cmd))
	return err
}

// readExact lê exatamente n bytes ou devolve ErrTimeout.
func (d *Device) readExact(n int, timeout time.Duration) ([]byte, error) {
	if err := d.port.SetReadTimeout(pollStep); err != nil {
		return nil, err
	}
	buf := make([]byte, n)
	got := 0
	deadline := time.Now().Add(timeout)
	for got < n {
		k, err := d.port.Read(buf[got:])
		if err != nil {
			return buf[:got], err
		}
		got += k
		if k == 0 && time.Now().After(deadline) {
			return buf[:got], fmt.Errorf("%w (recebi %d de %d bytes)", ErrTimeout, got, n)
		}
	}
	return buf, nil
}

// readIdle lê até a linha ficar em silêncio por idle (para respostas sem
// tamanho fixo, como <GETVER>>).
func (d *Device) readIdle(idle, max time.Duration) ([]byte, error) {
	if err := d.port.SetReadTimeout(pollStep); err != nil {
		return nil, err
	}
	var out []byte
	tmp := make([]byte, 64)
	start := time.Now()
	last := time.Now()
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

// query manda um comando e lê n bytes de resposta.
func (d *Device) query(cmd string, n int, timeout time.Duration) ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.port.ResetInputBuffer(); err != nil {
		return nil, err
	}
	if err := d.send(cmd); err != nil {
		return nil, err
	}
	b, err := d.readExact(n, timeout)
	if err != nil {
		// Sobrou lixo pela metade: limpa para não contaminar o próximo comando.
		_ = d.port.ResetInputBuffer()
		return nil, fmt.Errorf("%s: %w", strings.Trim(cmd, "<>"), err)
	}
	return b, nil
}

// Version devolve algo como "GMC-800Re1.08".
func (d *Device) Version() (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.port.ResetInputBuffer(); err != nil {
		return "", err
	}
	if err := d.send("<GETVER>>"); err != nil {
		return "", err
	}
	b, err := d.readIdle(150*time.Millisecond, defaultTimeout)
	if err != nil {
		return "", fmt.Errorf("GETVER: %w", err)
	}
	return strings.TrimSpace(strings.Trim(string(b), "\x00")), nil
}

// Serial devolve o número de série em hexadecimal, em bytes separados como
// na tela "Sobre" do contador ("A1 B2 C3 D4 E5 F6 01 02 03 04 05 06").
// O tamanho muda com o modelo (7 bytes nos antigos, 12 no GMC-800), então
// lê tudo o que vier até a porta ficar quieta.
func (d *Device) Serial() (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.port.ResetInputBuffer(); err != nil {
		return "", err
	}
	if err := d.send("<GETSERIAL>>"); err != nil {
		return "", err
	}
	b, err := d.readIdle(150*time.Millisecond, defaultTimeout)
	if err != nil {
		return "", fmt.Errorf("GETSERIAL: %w", err)
	}
	return FormatSerial(b), nil
}

// FormatSerial escreve os bytes como "A1 B2 C3 ...".
func FormatSerial(b []byte) string {
	parts := make([]string, len(b))
	for i, c := range b {
		parts[i] = strings.ToUpper(hex.EncodeToString([]byte{c}))
	}
	return strings.Join(parts, " ")
}

// CPM devolve a contagem do último minuto.
func (d *Device) CPM() (uint32, error) {
	b, err := d.query("<GETCPM>>", 4, defaultTimeout)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(b), nil
}

// CPS devolve a contagem do último segundo.
func (d *Device) CPS() (uint32, error) {
	b, err := d.query("<GETCPS>>", 4, defaultTimeout)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(b), nil
}

// DateTime lê o relógio do contador (no fuso local do computador).
func (d *Device) DateTime() (time.Time, error) {
	b, err := d.query("<GETDATETIME>>", 7, defaultTimeout)
	if err != nil {
		return time.Time{}, err
	}
	t, ok := makeTime(b[0], b[1], b[2], b[3], b[4], b[5])
	if !ok {
		return time.Time{}, fmt.Errorf("GETDATETIME: data inválida % x", b[:6])
	}
	return t, nil
}

// SetDateTime acerta o relógio do contador.
func (d *Device) SetDateTime(t time.Time) error {
	if t.Year() < 2000 || t.Year() > 2099 {
		return fmt.Errorf("ano %d fora do que o contador aceita", t.Year())
	}
	cmd := "<SETDATETIME" + string([]byte{
		byte(t.Year() - 2000), byte(t.Month()), byte(t.Day()),
		byte(t.Hour()), byte(t.Minute()), byte(t.Second()),
	}) + ">>"
	b, err := d.query(cmd, 1, defaultTimeout)
	if err != nil {
		return err
	}
	if b[0] != 0xAA {
		return fmt.Errorf("SETDATETIME: resposta inesperada 0x%02x", b[0])
	}
	return nil
}

// Config lê os 512 bytes de configuração.
func (d *Device) Config() ([]byte, error) {
	return d.query("<GETCFG>>", ConfigSize, 3*time.Second)
}

// ReadFlash lê n bytes da memória de histórico a partir de addr.
func (d *Device) ReadFlash(addr uint32, n uint16) ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.port.ResetInputBuffer(); err != nil {
		return nil, err
	}
	cmd := []byte("<SPIR")
	cmd = append(cmd, byte(addr>>16), byte(addr>>8), byte(addr))
	cmd = append(cmd, byte(n>>8), byte(n))
	cmd = append(cmd, '>', '>')
	if _, err := d.port.Write(cmd); err != nil {
		return nil, err
	}
	b, err := d.readExact(int(n), 5*time.Second)
	// Alguns firmwares mandam um byte a mais no fim do SPIR. Espera um
	// instante e descarta para não atrapalhar o próximo comando.
	time.Sleep(20 * time.Millisecond)
	_ = d.port.ResetInputBuffer()
	if err != nil {
		return nil, fmt.Errorf("SPIR 0x%06x: %w", addr, err)
	}
	return b, nil
}

// ReadHistory lê a memória de histórico página por página até achar uma
// página vazia (só 0xFF) ou o fim da flash. progress pode ser nil.
func (d *Device) ReadHistory(cancel <-chan struct{}, progress func(bytesRead int)) ([]byte, error) {
	var out []byte
	for addr := 0; addr < FlashSize; addr += PageSize {
		select {
		case <-cancel:
			return out, errors.New("leitura cancelada")
		default:
		}
		page, err := d.ReadFlash(uint32(addr), PageSize)
		if err != nil {
			return out, err
		}
		if allFF(page) {
			break
		}
		out = append(out, page...)
		if progress != nil {
			progress(len(out))
		}
	}
	return out, nil
}

func allFF(b []byte) bool {
	for _, c := range b {
		if c != 0xFF {
			return false
		}
	}
	return true
}

// makeTime monta uma data a partir dos bytes do contador (ano com 2 dígitos).
func makeTime(yy, mo, dd, hh, mi, ss byte) (time.Time, bool) {
	if mo < 1 || mo > 12 || dd < 1 || dd > 31 || hh > 23 || mi > 59 || ss > 59 || yy > 99 {
		return time.Time{}, false
	}
	t := time.Date(2000+int(yy), time.Month(mo), int(dd), int(hh), int(mi), int(ss), 0, time.Local)
	if t.Day() != int(dd) { // 31 de fevereiro etc.
		return time.Time{}, false
	}
	return t, true
}

// ErrNotReadOnly: Probe só manda comandos de leitura.
var ErrNotReadOnly = errors.New("só comandos de leitura (<GET…>>)")

// Probe manda um comando de leitura e devolve o que voltar, para
// diagnóstico (procurar comandos não documentados). Só aceita <GETxxx>>,
// com letras e números: nada que grave ou mude o aparelho. Sem resposta em
// max volta ErrTimeout.
func (d *Device) Probe(cmd string, max time.Duration) ([]byte, error) {
	if !strings.HasPrefix(cmd, "<GET") || !strings.HasSuffix(cmd, ">>") {
		return nil, ErrNotReadOnly
	}
	for _, c := range cmd[1 : len(cmd)-2] {
		if !(c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			return nil, ErrNotReadOnly
		}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.port.ResetInputBuffer(); err != nil {
		return nil, err
	}
	if err := d.send(cmd); err != nil {
		return nil, err
	}
	b, err := d.readIdle(150*time.Millisecond, max)
	_ = d.port.ResetInputBuffer()
	return b, err
}
