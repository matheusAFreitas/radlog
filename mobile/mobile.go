// Package mobile é a ponte entre o app de Android (Kotlin) e o motor do
// RadLog em Go. Ele é compilado com o gomobile (gomobile bind) numa
// biblioteca .aar.
//
// A tela do celular é HTML e JavaScript num WebView, como no desktop. Ela
// chama os mesmos métodos do App que o desktop expõe pelo Wails, pelo Call
// (nome do método e argumentos em JSON). Os eventos do App (leitura nova,
// alertas, nuvem) chegam ao Kotlin por Host.Event.
//
// Só tipos que o gomobile entende passam pela fronteira: string, []byte,
// int, bool, error e a interface Host.
package mobile

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sync"
	"time"
	_ "time/tzdata" // reserva, se o sistema não tiver o fuso

	"radlog/internal/app"
	"radlog/internal/radiacode"
)

// Host é o lado do Android, implementado em Kotlin.
type Host interface {
	// Event recebe um evento do motor (nome e dado em JSON).
	Event(name, data string)
	// OpenURL abre um endereço no navegador do sistema (login do Google).
	OpenURL(url string)
	// BLEScan procura aparelhos Bluetooth por ms milissegundos e devolve
	// JSON: [{"address":…,"name":…,"rssi":…}].
	BLEScan(ms int) (string, error)
	// BLEConnect conecta no endereço, acha o serviço do RC102 e liga as
	// notificações. Só volta quando está pronto (ou com o erro).
	BLEConnect(addr string) error
	// BLEWrite escreve um pedaço na característica de escrita (sem
	// resposta) e só volta quando o Android aceitou.
	BLEWrite(chunk []byte) error
	// BLEClose desconecta.
	BLEClose()
	// BLERssi é o último sinal lido do RC102 em dBm (0 = não sabe).
	BLERssi() int
	// BLEProbe pergunta o sinal ao rádio agora: o valor em dBm, 0 se o
	// rádio não respondeu em 1,5 s, 1 se não deu para perguntar.
	BLEProbe() int
}

var (
	mu  sync.Mutex
	eng *app.App
)

// Start cria e liga o motor. dir é a pasta privada do app; device é o nome
// do aparelho (o modelo do celular). Chamar de novo não faz nada.
func Start(dir, device string, host Host) error {
	mu.Lock()
	defer mu.Unlock()
	if eng != nil {
		return nil
	}
	if host == nil {
		return errors.New("sem host")
	}
	radiacode.SetBLEHost(bleHost{host})
	app.SetMobileOpenURL(host.OpenURL)
	if device != "" {
		app.SetMobileHost(device)
	}
	eng = app.NewMobile(dir, func(name string, data any) {
		b, err := json.Marshal(data)
		if err != nil {
			return
		}
		host.Event(name, string(b))
	})
	eng.StartMobile()
	return nil
}

// SetTimeZone acerta o fuso do Go (ex.: "America/Sao_Paulo"). No Android o
// Go não lê o fuso do sistema e fica em UTC: o "hoje" da dose virava às 21h
// e as horas gravadas saíam erradas. O Kotlin chama antes do Start e quando
// o fuso muda.
func SetTimeZone(id string) error {
	if id == "" {
		return errors.New("fuso vazio")
	}
	loc, err := time.LoadLocation(id)
	if err != nil {
		return err
	}
	time.Local = loc
	return nil
}

// Running diz se o motor já foi ligado.
func Running() bool {
	mu.Lock()
	defer mu.Unlock()
	return eng != nil
}

// Version é a versão do motor (a mesma do desktop desta compilação).
func Version() string { return app.AppVersion }

// Flush grava o que está pendente (o Android pode fechar o app a qualquer
// hora em segundo plano).
func Flush() {
	if a := cur(); a != nil {
		a.MobileFlush()
	}
}

// SetNetwork avisa a rede de agora: metered = dados móveis (pagos).
func SetNetwork(metered bool) {
	if a := cur(); a != nil {
		a.NetChanged(metered)
	}
}

// Stop grava e solta o aparelho. O processo vai fechar.
func Stop() {
	if a := cur(); a != nil {
		a.StopMobile()
	}
}

// BLENotify entrega uma notificação do RC102.
func BLENotify(b []byte) { radiacode.BLENotify(b) }

// BLELost avisa que a conexão com o RC102 caiu.
func BLELost() { radiacode.BLELost() }

func cur() *app.App {
	mu.Lock()
	defer mu.Unlock()
	return eng
}

// reply é o que o Call devolve: {"r": resultado} ou {"e": "mensagem"}.
type reply struct {
	R any    `json:"r,omitempty"`
	E string `json:"e,omitempty"`
}

func answer(r any, err error) string {
	var out reply
	if err != nil {
		out.E = err.Error()
	} else {
		out.R = r
	}
	b, jerr := json.Marshal(out)
	if jerr != nil {
		b, _ = json.Marshal(reply{E: "resposta que não vira JSON: " + jerr.Error()})
	}
	return string(b)
}

var errType = reflect.TypeOf((*error)(nil)).Elem()

// Call chama um método exportado do App, como o Wails faz no desktop. args
// é um array JSON com os argumentos na ordem. Devolve {"r": …} ou {"e": …}.
func Call(method, args string) (out string) {
	a := cur()
	if a == nil {
		return answer(nil, errors.New("o motor ainda não ligou"))
	}
	defer func() {
		if p := recover(); p != nil {
			out = answer(nil, fmt.Errorf("erro interno em %s: %v", method, p))
		}
	}()
	if !allowed(method) {
		return answer(nil, fmt.Errorf("método desconhecido: %s", method))
	}
	return call(reflect.ValueOf(a), method, args)
}

func call(v reflect.Value, method, args string) string {
	m := v.MethodByName(method)
	if !m.IsValid() || method == "StartMobile" {
		return answer(nil, fmt.Errorf("método desconhecido: %s", method))
	}
	t := m.Type()
	var raw []json.RawMessage
	if args != "" && args != "null" {
		if err := json.Unmarshal([]byte(args), &raw); err != nil {
			return answer(nil, fmt.Errorf("argumentos de %s: %v", method, err))
		}
	}
	if len(raw) != t.NumIn() {
		return answer(nil, fmt.Errorf("%s pede %d argumento(s), vieram %d", method, t.NumIn(), len(raw)))
	}
	in := make([]reflect.Value, t.NumIn())
	for i := range in {
		p := reflect.New(t.In(i))
		if err := json.Unmarshal(raw[i], p.Interface()); err != nil {
			return answer(nil, fmt.Errorf("argumento %d de %s: %v", i+1, method, err))
		}
		in[i] = p.Elem()
	}
	res := m.Call(in)
	var r any
	var err error
	for i, x := range res {
		if t.Out(i).Implements(errType) && t.Out(i).Kind() == reflect.Interface {
			if !x.IsNil() {
				err = x.Interface().(error)
			}
			continue
		}
		r = x.Interface()
	}
	return answer(r, err)
}

// allowed: só o que está em app.MobileMethods (lista branca, A4 da
// auditoria 2.79). Método novo na tela do celular entra lá e no golden do
// TestMobileAPI.
func allowed(method string) bool {
	return slices.Contains(app.MobileMethods, method)
}

// bleHost adapta o Host ao Bluetooth do pacote radiacode.
type bleHost struct{ h Host }

func (b bleHost) Scan(d time.Duration) ([]radiacode.BLEDevice, error) {
	s, err := b.h.BLEScan(int(d / time.Millisecond))
	if err != nil {
		return nil, err
	}
	var list []radiacode.BLEDevice
	if err := json.Unmarshal([]byte(s), &list); err != nil {
		return nil, fmt.Errorf("lista do Bluetooth: %v", err)
	}
	return list, nil
}

func (b bleHost) Connect(addr string) error { return b.h.BLEConnect(addr) }
func (b bleHost) Write(chunk []byte) error  { return b.h.BLEWrite(chunk) }
func (b bleHost) Close()                    { b.h.BLEClose() }
func (b bleHost) RSSI() int                 { return b.h.BLERssi() }
func (b bleHost) Probe() int                { return b.h.BLEProbe() }
