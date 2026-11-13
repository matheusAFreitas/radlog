package radiacode

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
	"time"
)

// S4: espectro malformado não vira milhões de canais.
func TestSpectrumChannelLimit(t *testing.T) {
	var b bytes.Buffer
	_ = binary.Write(&b, binary.LittleEndian, uint32(60))
	_ = binary.Write(&b, binary.LittleEndian, []float32{0, 3, 0})
	for i := 0; i < 20000; i++ { // cabeçalhos de 4.095 canais "zero" (2 bytes cada)
		_ = binary.Write(&b, binary.LittleEndian, uint16(0xfff0))
	}
	s, err := decodeSpectrum(b.Bytes(), 1)
	if err == nil || len(s.Counts) > MaxChannels {
		t.Fatalf("canais: %d, erro %v", len(s.Counts), err)
	}
	var ok bytes.Buffer
	_ = binary.Write(&ok, binary.LittleEndian, uint32(60))
	_ = binary.Write(&ok, binary.LittleEndian, []float32{0, 3, 0})
	for i := 0; i < 1024; i++ {
		_ = binary.Write(&ok, binary.LittleEndian, uint32(i))
	}
	if s, err := decodeSpectrum(ok.Bytes(), 0); err != nil || len(s.Counts) != 1024 {
		t.Fatalf("espectro normal: %d %v", len(s.Counts), err)
	}
}

// slowTr segura o Exchange até liberar e marca se o Close veio no meio.
type slowTr struct {
	inside  chan struct{}
	release chan struct{}
	busy    bool
	overlap bool
}

func (s *slowTr) Exchange(req []byte) ([]byte, error) {
	s.busy = true
	close(s.inside)
	<-s.release
	s.busy = false
	return nil, errors.New("fim")
}
func (s *slowTr) Close() error { s.overlap = s.busy; return nil }
func (s *slowTr) Kind() string { return "usb" }

// B9: o Close espera a leitura em andamento voltar.
func TestCloseWaitsForExchange(t *testing.T) {
	tr := &slowTr{inside: make(chan struct{}), release: make(chan struct{})}
	d := &Device{tr: tr}
	go func() { _, _ = d.execute(1, nil) }()
	<-tr.inside
	done := make(chan struct{})
	go func() { d.Close(); close(done) }()
	time.Sleep(50 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("fechou no meio da leitura")
	default:
	}
	close(tr.release)
	<-done
	if tr.overlap {
		t.Fatal("Close durante o Exchange")
	}
}
