package app

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"radlog/internal/radiacode"
)

// rcRegs responde como um RadiaCode-102 (só registradores).
type rcRegs struct{ regs map[uint32]uint32 }

func le32(vs ...uint32) []byte {
	b := make([]byte, 4*len(vs))
	for i, v := range vs {
		binary.LittleEndian.PutUint32(b[4*i:], v)
	}
	return b
}

func (f *rcRegs) Exchange(req []byte) ([]byte, error) {
	hdr, args := req[4:8], req[8:]
	var body []byte
	switch binary.LittleEndian.Uint16(hdr) {
	case 0x082A: // leitura em lote
		n := binary.LittleEndian.Uint32(args)
		var ok uint32
		var vals []byte
		for i := uint32(0); i < n; i++ {
			v, has := f.regs[binary.LittleEndian.Uint32(args[4+4*i:])]
			if has {
				ok |= 1 << i
			}
			vals = append(vals, le32(v)...)
		}
		body = append(le32(ok), vals...)
	case 0x0825: // escrita
		id := binary.LittleEndian.Uint32(args)
		if id == 0x8007 {
			f.regs[0x8022] = 0 // dose zerada
		} else {
			f.regs[id] = binary.LittleEndian.Uint32(args[4:])
		}
		body = le32(1)
	case 0x0826: // configuração (sem a lista de tempos: usa a padrão)
		body = le32(1, 0)
	}
	return append(append([]byte{}, hdr...), body...), nil
}
func (f *rcRegs) Close() error { return nil }
func (f *rcRegs) Kind() string { return "usb" }

func TestRcSettingsSetUndo(t *testing.T) {
	a := newTestApp(t)
	if _, err := a.GetRcSettings(); err == nil {
		t.Fatal("sem aparelho deveria dar erro")
	}
	f := &rcRegs{regs: map[uint32]uint32{
		0x8000: 40, 0x8001: 120, 0x8014: 999999999, 0x8015: 999999999, 0x8002: 9999999, 0x8003: 9999999,
		0x8008: 200, 0x8009: 600, 0x0522: 1, 0x0520: 0x1ffc, 0x0531: 1, 0x0530: 0x1cfe,
		0x0511: 1, 0x0513: 1, 0x0515: 0, 0x8004: 1, 0x8013: 1, 0x8010: 0, 0x8011: 0, 0x8012: 0, 0x8022: 500,
	}}
	a.rc.dev = radiacode.NewRaw(f)
	a.rc.st.Since = 1

	s, err := a.GetRcSettings()
	if err != nil || s.DoseRate1 != 0.4 || s.CanUndo {
		t.Fatalf("leitura: %+v %v", s, err)
	}
	if ms, _ := filepath.Glob(a.rcCfgPath("*-lida.json")); len(ms) != 1 {
		t.Fatalf("cópia da primeira leitura: %v", ms)
	}
	a.GetRcSettings() // mesma conexão: não copia de novo
	if ms, _ := filepath.Glob(a.rcCfgPath("*-lida.json")); len(ms) != 1 {
		t.Fatalf("copiou de novo: %v", ms)
	}

	next := s.Settings
	next.DoseRate1, next.DoseRate2, next.Brightness = 0.3, 0.9, 5
	got, err := a.SetRcSettings(next)
	if err != nil || got.DoseRate1 != 0.3 || got.Brightness != 5 || !got.CanUndo {
		t.Fatalf("gravação: %+v %v", got, err)
	}
	// os alertas do RadiaCode passam a usar os alarmes novos
	if l := a.rcLimits(); !l.Device || l.Attn != 0.3 || l.Alarm != 0.9 {
		t.Fatalf("limites dos alertas: %+v", l)
	}
	h := a.GetConfigHistory()
	if len(h) == 0 || h[0].Field != "Ajustes do RadiaCode" || !strings.Contains(h[0].To, "brilho: 1 → 5") {
		t.Fatalf("histórico: %+v", h)
	}

	// valor fora do aceito: não grava nada
	bad := got.Settings
	bad.DoseRate2 = 0.1
	if _, err := a.SetRcSettings(bad); err == nil || f.regs[0x8001] != 90 {
		t.Fatalf("aceitou alarme 2 menor que o 1 (%v, reg %d)", err, f.regs[0x8001])
	}

	// desfazer volta ao que estava e não deixa outra gravação para desfazer
	und, err := a.UndoRcSettings()
	if err != nil || und.DoseRate1 != 0.4 || und.Brightness != 1 || und.CanUndo {
		t.Fatalf("desfazer: %+v %v", und, err)
	}
	if _, err := a.UndoRcSettings(); err == nil {
		t.Fatal("desfez duas vezes")
	}

	if err := a.ResetRcDose(); err != nil || f.regs[0x8022] != 0 {
		t.Fatalf("zerar dose: %v", err)
	}
	if _, err := os.Stat(a.rcCfgPath("")); err != nil {
		t.Fatal(err)
	}
}
