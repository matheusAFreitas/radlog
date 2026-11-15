package radiacode

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// O host do Bluetooth do app de Android. Fica em todos os sistemas para o
// pacote radlog/mobile compilar e ser testado no PC; só o Android usa (o
// ScanBLE e o OpenBLE de lá estão em ble_android.go).

// BLEHost é o lado do sistema (Kotlin).
type BLEHost interface {
	// Scan procura Radiacodes por d e devolve os achados.
	Scan(d time.Duration) ([]BLEDevice, error)
	// Connect conecta, acha o serviço do RC102 e liga as notificações.
	Connect(addr string) error
	// Write manda um pedaço (sem resposta do rádio).
	Write(chunk []byte) error
	// Close desconecta.
	Close()
}

var (
	bleMu   sync.Mutex
	bleHost BLEHost
	bleCur  *bleAndroid
)

// ErrNoHost: o app de Android ainda não registrou o Bluetooth.
var ErrNoHost = errors.New("Bluetooth: o app ainda não ligou o rádio")

// SetBLEHost registra o Bluetooth do app de Android.
func SetBLEHost(h BLEHost) {
	bleMu.Lock()
	bleHost = h
	bleMu.Unlock()
}

func host() BLEHost {
	bleMu.Lock()
	defer bleMu.Unlock()
	return bleHost
}

type bleAndroid struct {
	h    BLEHost
	mu   sync.Mutex
	as   *assembler
	resp chan []byte
	lost bool
	done bool
}

// BLENotify entrega uma notificação que chegou no Android.
func BLENotify(p []byte) {
	bleMu.Lock()
	b := bleCur
	bleMu.Unlock()
	if b == nil {
		return
	}
	b.mu.Lock()
	out, done, err := b.as.add(append([]byte(nil), p...))
	if err != nil {
		b.as = newAssembler()
	}
	b.mu.Unlock()
	if done {
		select {
		case b.resp <- out:
		default:
		}
	}
}

// BLELost avisa que a conexão caiu.
func BLELost() {
	bleMu.Lock()
	b := bleCur
	bleMu.Unlock()
	if b == nil {
		return
	}
	b.mu.Lock()
	b.lost = true
	b.mu.Unlock()
	select {
	case b.resp <- nil:
	default:
	}
}

// hostScan procura Radiacodes pelo host.
func hostScan(d time.Duration) ([]BLEDevice, error) {
	h := host()
	if h == nil {
		return nil, ErrNoHost
	}
	list, err := h.Scan(d)
	if err != nil {
		return nil, err
	}
	out := list[:0]
	for _, x := range list {
		if isRadiacodeName(x.Name) {
			out = append(out, x)
		}
	}
	return out, nil
}

// hostOpen conecta pelo host.
func hostOpen(addr string) (Transport, error) {
	h := host()
	if h == nil {
		return nil, ErrNoHost
	}
	b := &bleAndroid{h: h, as: newAssembler(), resp: make(chan []byte, 1)}
	bleMu.Lock()
	old := bleCur
	bleCur = b
	bleMu.Unlock()
	if old != nil {
		old.Close()
	}
	if err := h.Connect(addr); err != nil {
		bleMu.Lock()
		if bleCur == b {
			bleCur = nil
		}
		bleMu.Unlock()
		return nil, fmt.Errorf("Bluetooth: %v", err)
	}
	return b, nil
}

func (b *bleAndroid) Exchange(req []byte) ([]byte, error) {
	b.mu.Lock()
	if b.lost || b.done {
		b.mu.Unlock()
		return nil, errors.New("Bluetooth: conexão perdida")
	}
	b.as = newAssembler()
	b.mu.Unlock()
	select { // sobra de uma resposta antiga
	case <-b.resp:
	default:
	}
	for p := 0; p < len(req); p += bleChunk {
		if err := b.h.Write(req[p:min(p+bleChunk, len(req))]); err != nil {
			return nil, fmt.Errorf("Bluetooth (envio): %v", err)
		}
	}
	select {
	case out := <-b.resp:
		if out == nil {
			return nil, errors.New("Bluetooth: conexão perdida")
		}
		return out, nil
	case <-time.After(10 * time.Second):
		return nil, errors.New("Bluetooth: o RC102 não respondeu")
	}
}

func (b *bleAndroid) Close() error {
	b.mu.Lock()
	if b.done {
		b.mu.Unlock()
		return nil
	}
	b.done = true
	b.mu.Unlock()
	bleMu.Lock()
	if bleCur == b {
		bleCur = nil
	}
	bleMu.Unlock()
	b.h.Close()
	return nil
}

func (b *bleAndroid) Kind() string { return "bluetooth" }
