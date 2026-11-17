package radiacode

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// fakeBLE faz o papel do Kotlin: junta os pedaços de 18 bytes do pedido,
// passa para o fakeRC e devolve a resposta em notificações de 20 bytes.
type fakeBLE struct {
	rc      *fakeRC
	req     []byte
	chunks  int
	closed  bool
	failCon bool
}

func (f *fakeBLE) Scan(time.Duration) ([]BLEDevice, error) {
	return []BLEDevice{{Address: "AA", Name: "RadiaCode-102#RC-102-001234"}, {Address: "BB", Name: "fone"}}, nil
}

func (f *fakeBLE) Connect(string) error {
	if f.failCon {
		return errors.New("sem resposta")
	}
	return nil
}

func (f *fakeBLE) Write(p []byte) error {
	if len(p) > bleChunk {
		return errors.New("pedaço grande demais")
	}
	f.chunks++
	f.req = append(f.req, p...)
	if len(f.req) < 4 || len(f.req)-4 < int(binary.LittleEndian.Uint32(f.req)) {
		return nil
	}
	resp, _ := f.rc.Exchange(f.req)
	f.req = nil
	out := append(le(uint32(len(resp))), resp...)
	go func() {
		for p := 0; p < len(out); p += 20 {
			BLENotify(out[p:min(p+20, len(out))])
		}
	}()
	return nil
}

func (f *fakeBLE) Close() { f.closed = true }

func TestHostBLE(t *testing.T) {
	defer SetBLEHost(nil)
	if _, err := hostOpen("AA"); !errors.Is(err, ErrNoHost) {
		t.Fatalf("sem host: %v", err)
	}
	f := &fakeBLE{rc: &fakeRC{}}
	SetBLEHost(f)
	list, err := hostScan(time.Second)
	if err != nil || len(list) != 1 || list[0].Address != "AA" {
		t.Fatalf("busca: %v %+v", err, list)
	}
	tr, err := hostOpen("AA")
	if err != nil {
		t.Fatal(err)
	}
	d, err := Open(tr)
	if err != nil {
		t.Fatal(err)
	}
	if d.Serial != "RC-102-001234" || tr.Kind() != "bluetooth" || f.chunks < 3 {
		t.Fatalf("aparelho: %q %s pedaços=%d", d.Serial, tr.Kind(), f.chunks)
	}
	BLELost()
	if _, err := tr.Exchange(le(uint32(4), uint32(0))); err == nil {
		t.Fatal("depois de cair a conexão o pedido passou")
	}
	tr.Close()
	if !f.closed {
		t.Fatal("não desconectou")
	}
	f.failCon = true
	if _, err := hostOpen("AA"); err == nil {
		t.Fatal("conexão que falha não deu erro")
	}
}

// resposta que não vem: o registro diz se nada chegou ou se veio pela metade.
func TestBLEHostTimeoutLog(t *testing.T) {
	defer func(q, s time.Duration) { bleQuiet, bleSlow = q, s }(bleQuiet, bleSlow)
	bleQuiet, bleSlow = 200*time.Millisecond, 600*time.Millisecond
	var lines []string
	BLELogf = func(f string, a ...any) { lines = append(lines, fmt.Sprintf(f, a...)) }
	defer func() { BLELogf = nil }()
	b := &bleAndroid{h: nopHost{}, as: newAssembler(), resp: make(chan []byte, 1)}
	bleMu.Lock()
	bleCur = b
	bleMu.Unlock()
	defer func() { bleMu.Lock(); bleCur = nil; bleMu.Unlock() }()
	go func() {
		time.Sleep(100 * time.Millisecond)
		BLENotify([]byte{20, 0, 0, 0, 1, 2, 3}) // anuncia 20 bytes, chegam 3
	}()
	if _, err := b.Exchange([]byte{4, 0, 0, 0, 0x05, 0x00, 0, 0x80}); err == nil {
		t.Fatal("devia estourar o tempo")
	}
	if len(lines) != 1 || !strings.Contains(lines[0], "pedido 0005") || !strings.Contains(lines[0], "3 de 20 bytes") {
		t.Fatalf("registro: %q", lines)
	}
}

type nopHost struct{ closed *bool }

func (nopHost) Scan(time.Duration) ([]BLEDevice, error) { return nil, nil }
func (nopHost) Connect(string) error                    { return nil }
func (nopHost) Write([]byte) error                      { return nil }
func (h nopHost) Close() {
	if h.closed != nil {
		*h.closed = true
	}
}

// nada chega: em bleQuiet a conexão cai na hora (o próximo pedido nem tenta)
func TestBLEHostQuietDrops(t *testing.T) {
	defer func(q, s time.Duration) { bleQuiet, bleSlow = q, s }(bleQuiet, bleSlow)
	bleQuiet, bleSlow = 100*time.Millisecond, time.Second
	closed := false
	b := &bleAndroid{h: nopHost{closed: &closed}, as: newAssembler(), resp: make(chan []byte, 1)}
	start := time.Now()
	if _, err := b.Exchange([]byte{4, 0, 0, 0, 0x26, 0x08, 0, 0x80}); err == nil {
		t.Fatal("devia falhar")
	}
	if time.Since(start) > 500*time.Millisecond || !closed {
		t.Fatalf("demorou %s ou não fechou (%v)", time.Since(start), closed)
	}
	if _, err := b.Exchange([]byte{4, 0, 0, 0, 0x26, 0x08, 0, 0x81}); err == nil || !strings.Contains(err.Error(), "perdida") {
		t.Fatalf("depois da queda: %v", err)
	}
}
