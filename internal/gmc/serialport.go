package gmc

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

// Baud é a velocidade padrão do GMC-800.
const Baud = 115200

// PortInfo descreve uma porta serial encontrada no computador.
type PortInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsUSB       bool   `json:"isUSB"`
	// USBID identifica o aparelho atrás da porta: "VID:PID" e, se o chip
	// tiver, ":número de série". O número da porta COM pode mudar; isto não.
	USBID string `json:"usbId,omitempty"`
}

// gmcVIDs são os fabricantes de chip USB-serial usados pelos contadores GQ
// (WCH: CH340, CH341, CH9102).
var gmcVIDs = map[string]bool{"1A86": true}

// LikelyGMC diz se a porta tem um chip USB-serial dos usados pelo GMC.
func (p PortInfo) LikelyGMC() bool {
	vid, _, _ := strings.Cut(p.USBID, ":")
	return gmcVIDs[vid]
}

// ListPorts lista as portas seriais disponíveis.
func ListPorts() ([]PortInfo, error) {
	var out []PortInfo
	details, err := enumerator.GetDetailedPortsList()
	if err == nil && len(details) > 0 {
		for _, p := range details {
			desc := p.Product
			if p.IsUSB && desc == "" {
				desc = fmt.Sprintf("USB %s:%s", p.VID, p.PID)
			}
			pi := PortInfo{Name: p.Name, Description: strings.TrimSpace(desc), IsUSB: p.IsUSB}
			if p.IsUSB {
				pi.USBID = strings.ToUpper(p.VID + ":" + p.PID)
				if sn := strings.TrimSpace(p.SerialNumber); sn != "" {
					pi.USBID += ":" + sn
				}
			}
			out = append(out, pi)
		}
	} else {
		names, err2 := serial.GetPortsList()
		if err2 != nil {
			return nil, err2
		}
		for _, n := range names {
			out = append(out, PortInfo{Name: n})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsUSB != out[j].IsUSB {
			return out[i].IsUSB
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// Open abre a porta e devolve o contador pronto para uso.
func Open(name string) (*Device, error) {
	p, err := serial.Open(name, &serial.Mode{
		BaudRate: Baud,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	})
	if err != nil {
		return nil, openError(name, err)
	}
	return New(p), nil
}

// ErrNotFound quer dizer que a porta não existe (contador desligado ou cabo
// fora).
var ErrNotFound = errors.New("porta não encontrada")

type notFound struct{ port string }

func (e notFound) Error() string        { return e.port + " não encontrada" }
func (e notFound) Is(target error) bool { return target == ErrNotFound }

// openError troca as mensagens em inglês da biblioteca serial por frases
// curtas em português.
func openError(name string, err error) error {
	var pe *serial.PortError
	if errors.As(err, &pe) {
		switch pe.Code() {
		case serial.PortNotFound:
			return notFound{name}
		case serial.PortBusy:
			return fmt.Errorf("%s em uso por outro programa (feche o Data Viewer da GQ)", name)
		case serial.InvalidSerialPort:
			// a porta existe mas o driver não responde: o USB do contador caiu
			return fmt.Errorf("%s não responde (o USB do contador travou; tire e recoloque o cabo)", name)
		case serial.PermissionDenied:
			return fmt.Errorf("sem permissão para abrir %s (talvez outro programa esteja usando)", name)
		}
	}
	return fmt.Errorf("não consegui abrir %s: %w", name, err)
}
