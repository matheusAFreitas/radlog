package radiacode

import (
	"encoding/binary"
	"errors"
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
