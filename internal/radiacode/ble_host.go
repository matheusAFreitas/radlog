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

// BLELogf, se não for nil, recebe o diagnóstico de cada resposta que não
// veio (o app liga no modo celular): quantos pedaços chegaram depois do
// pedido e quanto da resposta montou. Assim uma queda no Bluetooth mostra no
// registro se o RC102 calou ou se a resposta veio pela metade.
var BLELogf func(format string, args ...any)

type bleAndroid struct {
	h    BLEHost
	mu   sync.Mutex
	as   *assembler
	resp chan []byte
	lost bool
	done bool
	// desde o último pedido: pedaços que chegaram e erros da montagem
	got  int
	bad  string
	sent time.Time
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
	b.got++
	out, done, err := b.as.add(append([]byte(nil), p...))
	if err != nil {
		b.bad = err.Error()
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
	b.got, b.bad, b.sent = 0, "", time.Now()
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
	// sem nenhum pedaço em bleQuiet a conexão travou (no celular o GATT
	// inteiro fica preso: nada mais é aceito); com pedaços chegando, uma
	// resposta grande tem até bleSlow
	quiet := time.NewTimer(bleQuiet)
	defer quiet.Stop()
	slow := time.NewTimer(bleSlow)
	defer slow.Stop()
	for {
		select {
		case out := <-b.resp:
			if out == nil {
				return nil, errors.New("Bluetooth: conexão perdida")
			}
			return out, nil
		case <-quiet.C:
			b.mu.Lock()
			got := b.got
			b.mu.Unlock()
			if got > 0 {
				continue // está chegando: espera o bleSlow
			}
			b.logTimeout(req, bleQuiet)
			// travou: derruba já para o laço reconectar sem esperar mais
			// pedidos falharem (ACE, 17/11: 30 s parado a cada queda)
			b.mu.Lock()
			b.lost = true
			b.mu.Unlock()
			b.h.Close()
			return nil, errors.New("Bluetooth: o RC102 não respondeu")
		case <-slow.C:
			b.logTimeout(req, bleSlow)
			return nil, errors.New("Bluetooth: o RC102 não respondeu")
		}
	}
}

// bleQuiet: sem nenhum pedaço de resposta nesse tempo, a conexão travou (uma
// resposta normal chega em menos de 0,5 s). bleSlow: o máximo para uma
// resposta que está chegando.
var (
	bleQuiet = 4 * time.Second
	bleSlow  = 10 * time.Second
)

func (b *bleAndroid) logTimeout(req []byte, after time.Duration) {
	f := BLELogf
	if f == nil {
		return
	}
	b.mu.Lock()
	got, bad, part, want := b.got, b.bad, len(b.as.data), b.as.want
	b.mu.Unlock()
	cmd := ""
	if len(req) >= 6 {
		cmd = fmt.Sprintf("%02x%02x", req[5], req[4])
	}
	// qual string virtual (a leitura de cada segundo ou o espectro)
	if len(req) >= 12 && uint16(req[4])|uint16(req[5])<<8 == cmdRdVirtString {
		id := uint32(req[8]) | uint32(req[9])<<8 | uint32(req[10])<<16 | uint32(req[11])<<24
		name := fmt.Sprintf("%#x", id)
		if id == vsDataBuf {
			name = "DATA_BUF"
		}
		for _, k := range KnownVS {
			if k.ID == id {
				name = k.Name
			}
		}
		cmd += " (" + name + ")"
	}
	what := "nada chegou"
	if got > 0 {
		what = fmt.Sprintf("chegaram %d pedaço(s)", got)
		if want >= 0 {
			what += fmt.Sprintf(" · resposta pela metade (%d de %d bytes)", part, want)
		}
	}
	if bad != "" {
		what += " · montagem: " + bad
	}
	// o sinal do RC102 no celular (o último lido): separa rádio fraco de travamento
	if r, ok := b.h.(interface{ RSSI() int }); ok {
		if v := r.RSSI(); v != 0 {
			what += fmt.Sprintf(" · sinal %d dBm", v)
		}
	}
	// 19/11: queda com sinal forte (-43 dBm lado a lado): o rádio ainda
	// responde na hora do travamento? Separa "o RC102 parou de mandar" de
	// "a ligação caiu"
	if p, ok := b.h.(interface{ Probe() int }); ok {
		switch v := p.Probe(); {
		case v < 0:
			what += fmt.Sprintf(" · o rádio ainda responde (%d dBm)", v)
		case v == 0:
			what += " · o rádio não respondeu"
		default:
			what += " · não deu para perguntar ao rádio"
		}
	}
	f("RC102 (Bluetooth): sem resposta em %s ao pedido %s: %s", after, cmd, what)
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
