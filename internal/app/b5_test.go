package app

import (
	"testing"

	"radlog/internal/gmc"
)

// B5: a reconexão segue o contador pelo USB, não pelo número da porta.
func TestResolvePort(t *testing.T) {
	gmcID := "1A86:7523"
	cases := []struct {
		ports []gmc.PortInfo
		name  string
		usb   string
		want  string
		ok    bool
	}{
		{[]gmc.PortInfo{{Name: "COM5", USBID: gmcID}}, "COM5", gmcID, "COM5", true},
		{[]gmc.PortInfo{{Name: "COM7", USBID: gmcID}}, "COM5", gmcID, "COM7", true},                                     // mudou de número
		{[]gmc.PortInfo{{Name: "COM5", USBID: "2341:0043"}}, "COM5", gmcID, "", false},                                  // outro aparelho
		{[]gmc.PortInfo{{Name: "COM5", USBID: "2341:0043"}, {Name: "COM9", USBID: gmcID}}, "COM5", gmcID, "COM9", true}, // os dois
		{nil, "COM5", gmcID, "COM5", true},                                                                              // desconectado
		{[]gmc.PortInfo{{Name: "COM5", USBID: "2341:0043"}}, "COM5", "", "COM5", true},                                  // config antiga
	}
	for i, c := range cases {
		got, ok := resolvePort(c.ports, c.name, c.usb)
		if got != c.want || ok != c.ok {
			t.Errorf("caso %d: %q %v, queria %q %v", i, got, ok, c.want, c.ok)
		}
	}
}

func TestDetectOrder(t *testing.T) {
	ports := []gmc.PortInfo{
		{Name: "COM3", IsUSB: true, USBID: "2341:0043"},
		{Name: "COM4", IsUSB: true, USBID: "1A86:7523"},
		{Name: "COM1"},
		{Name: "COM8", IsUSB: true, USBID: "10C4:EA60:XYZ"},
	}
	got := detectOrder(ports, "10C4:EA60:XYZ")
	if len(got) != 2 || got[0].Name != "COM8" || got[1].Name != "COM4" {
		t.Fatalf("ordem: %+v", got)
	}
	// nenhum conhecido: testa as USB (botão apertado de propósito)
	if got := detectOrder(ports[:1], ""); len(got) != 1 || got[0].Name != "COM3" {
		t.Fatalf("sem conhecidos: %+v", got)
	}
}
