package radiacode

import (
	"encoding/binary"
	"strings"
	"testing"
)

// fakeRegs é um aparelho com registradores de verdade (valores de um
// RadiaCode-102 com firmware 4.15, lidos em 10/11/2026).
type fakeRegs struct {
	regs   map[uint32]uint32
	writes []uint32
	resets int
	accSec uint32
	refuse map[uint32]bool
}

func newFakeRegs() *fakeRegs {
	return &fakeRegs{regs: map[uint32]uint32{
		sfrDRLev1: 40, sfrDRLev2: 120, sfrDSLev1: 999999999, sfrDSLev2: 999999999,
		sfrDSLev1x100: 9999999, sfrDSLev2x100: 9999999,
		sfrCRLev1: 200, sfrCRLev2: 600, sfrSoundOn: 1, sfrSoundCtrl: 0x1ffc,
		sfrVibroOn: 1, sfrVibroCtrl: 0x1cfe, sfrDispBrt: 1, sfrDispOffTime: 1, sfrDispDir: 0,
		sfrDSUnits: 1, sfrCRUnits: 1, 0x8010: 0x40adea14, 0x8011: 0x401677bc, 0x8012: 0x39c7e938,
	}, refuse: map[uint32]bool{}}
}

func (f *fakeRegs) Exchange(req []byte) ([]byte, error) {
	hdr := req[4:8]
	cmd := binary.LittleEndian.Uint16(hdr)
	args := req[8:]
	var body []byte
	switch cmd {
	case cmdRdVirtSFRBat:
		n := binary.LittleEndian.Uint32(args)
		var ok uint32
		var vals []byte
		for i := uint32(0); i < n; i++ {
			id := binary.LittleEndian.Uint32(args[4+4*i:])
			v, has := f.regs[id]
			if has {
				ok |= 1 << i
			}
			vals = append(vals, le(v)...)
		}
		body = append(le(ok), vals...)
	case cmdWrVirtSFR:
		id := binary.LittleEndian.Uint32(args)
		if f.refuse[id] {
			body = le(uint32(0))
			break
		}
		if id == sfrDoseReset {
			f.resets++
		} else {
			f.regs[id] = binary.LittleEndian.Uint32(args[4:])
			f.writes = append(f.writes, id)
		}
		body = le(uint32(1))
	case cmdWrVirtString:
		if f.refuse[binary.LittleEndian.Uint32(args)] {
			body = le(uint32(0))
			break
		}
		body = le(uint32(1))
	case cmdRdVirtString:
		if binary.LittleEndian.Uint32(args) == vsSpecAccum {
			sp := le(f.accSec, float32(0), float32(3), float32(0), uint32(1), uint32(2), uint32(3))
			body = append(le(uint32(1), uint32(len(sp))), sp...)
			break
		}
		cfg := []byte("[DeviceParams]\nVSFR_DISP_OFF_TIME=\"5 s;10 s;15 s;30 s;2 min;5 min\"\nDispBacklight=0-9\n")
		body = append(le(uint32(1), uint32(len(cfg))), cfg...)
	}
	return append(append([]byte{}, hdr...), body...), nil
}
func (f *fakeRegs) Close() error { return nil }
func (f *fakeRegs) Kind() string { return "usb" }

func TestReadSettings(t *testing.T) {
	d := &Device{tr: newFakeRegs()}
	s, err := d.ReadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.DoseRate1 != 0.4 || s.DoseRate2 != 1.2 || s.CountRate1 != 20 || s.CountRate2 != 60 {
		t.Fatalf("alarmes: %+v", s)
	}
	if s.Dose1 < DoseOffUSv || !s.UnitSv || !s.UnitCPM || !s.SoundOn || !s.VibroOn {
		t.Fatalf("dose/unidades/avisos: %+v", s)
	}
	if s.SoundCtrl&CtrlClicks != 0 || s.SoundCtrl&CtrlButtons != 0 || s.SoundCtrl&CtrlRateAlarm1 == 0 {
		t.Fatalf("som: %#x", s.SoundCtrl)
	}
	if s.Brightness != 1 || s.OffTime != 1 || len(s.OffTimes) != 6 || s.OffTimes[5] != "5 min" {
		t.Fatalf("tela: %+v", s)
	}
	if s.EnergyA1 < 2.35 || s.EnergyA1 > 2.36 {
		t.Fatalf("calibração de energia: %v", s.EnergyA1)
	}
	if p := s.Check(); len(p) > 0 {
		t.Fatalf("o que veio do aparelho não passa no Check: %v", p)
	}
	// o booleano das unidades no byte baixo também vale para os alarmes
	al, err := d.Alarms()
	if err != nil || !al.ShowSv || !al.ShowCPM {
		t.Fatalf("Alarms: %+v %v", al, err)
	}
}

func TestWriteSettingsOnlyChanged(t *testing.T) {
	f := newFakeRegs()
	d := &Device{tr: f}
	cur, _ := d.ReadSettings()
	next := cur
	next.DoseRate1 = 0.3
	next.Dose1 = 1000 // 1 mSv
	next.SoundCtrl |= CtrlClicks
	next.VibroCtrl |= CtrlClicks // não existe na vibração: ignorado
	next.OffTime = 4
	got, n, err := d.WriteSettings(cur, next)
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 { // taxa 1, dose 1 (+cópia em 100 µR), som, tela
		t.Fatalf("gravou %d registros: %#x", n, f.writes)
	}
	if f.regs[sfrDRLev1] != 30 || f.regs[sfrDSLev1] != 100000 || f.regs[sfrDSLev1x100] != 1000 {
		t.Fatalf("registros: taxa %d dose %d/%d", f.regs[sfrDRLev1], f.regs[sfrDSLev1], f.regs[sfrDSLev1x100])
	}
	if f.regs[sfrSoundCtrl] != 0x1ffe || f.regs[sfrVibroCtrl] != 0x1cfe {
		t.Fatalf("som %#x vibração %#x (bits altos precisam ficar)", f.regs[sfrSoundCtrl], f.regs[sfrVibroCtrl])
	}
	if got.DoseRate1 != 0.3 || got.Dose1 != 1000 || got.OffTime != 4 {
		t.Fatalf("lido de volta: %+v", got)
	}
	desc := DescribeChanges(cur, got)
	for _, w := range []string{"alarme 1 de taxa: 0,4 µSv/h → 0,3 µSv/h", "alarme 1 de dose: desligado → 1 mSv", "som em: +cliques", "apagar a tela: 10 s → 2 min"} {
		if !strings.Contains(desc, w) {
			t.Errorf("faltou %q em %q", w, desc)
		}
	}
	if strings.Contains(desc, "vibração") {
		t.Errorf("vibração não mudou: %q", desc)
	}
	// sem mudança: não grava nada
	f.writes = nil
	if _, n, err := d.WriteSettings(got, got); err != nil || n != 0 || len(f.writes) != 0 {
		t.Fatalf("sem mudança gravou %d (%v)", n, err)
	}
}

func TestWriteSettingsChecks(t *testing.T) {
	f := newFakeRegs()
	d := &Device{tr: f}
	cur, _ := d.ReadSettings()
	for name, mod := range map[string]func(*Settings){
		"taxa invertida":   func(s *Settings) { s.DoseRate2 = 0.1 },
		"taxa zero":        func(s *Settings) { s.DoseRate1 = 0 },
		"dose invertida":   func(s *Settings) { s.Dose1, s.Dose2 = 50, 10 },
		"contagem demais":  func(s *Settings) { s.CountRate2 = 60000 },
		"brilho 10":        func(s *Settings) { s.Brightness = 10 },
		"tempo inexistent": func(s *Settings) { s.OffTime = 6 },
		"orientação 3":     func(s *Settings) { s.Direction = 3 },
	} {
		next := cur
		mod(&next)
		f.writes = nil
		if _, _, err := d.WriteSettings(cur, next); err == nil || len(f.writes) != 0 {
			t.Errorf("%s: deveria recusar sem gravar (err %v, gravou %#x)", name, err, f.writes)
		}
	}
	// recusa do aparelho no meio: para e devolve o que ficou
	next := cur
	next.DoseRate1, next.Brightness = 0.3, 5
	f.refuse[sfrDispBrt] = true
	got, n, err := d.WriteSettings(cur, next)
	if err == nil || n != 1 || got.DoseRate1 != 0.3 || got.Brightness != 1 {
		t.Fatalf("recusa: n %d err %v %+v", n, err, got)
	}
}

func TestResetDose(t *testing.T) {
	f := newFakeRegs()
	d := &Device{tr: f}
	if err := d.ResetDose(); err != nil || f.resets != 1 {
		t.Fatalf("zerar: %v (%d)", err, f.resets)
	}
}

func TestResetSpectrum(t *testing.T) {
	f := newFakeRegs()
	d := &Device{tr: f}
	if err := d.ResetSpectrum(); err != nil {
		t.Fatalf("recomeçar: %v", err)
	}
	f.refuse[vsSpectrum] = true
	if err := d.ResetSpectrum(); err == nil {
		t.Fatal("recusa passou como ok")
	}
}

func TestOffTimesFromConfig(t *testing.T) {
	if o := OffTimesFromConfig("x=1\nVSFR_DISP_OFF_TIME=\"5 s;10 s\"\n"); len(o) != 2 || o[1] != "10 s" {
		t.Fatal(o)
	}
	if OffTimesFromConfig("nada") != nil {
		t.Fatal("sem a chave")
	}
}
