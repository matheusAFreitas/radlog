package gmc

import (
	"bytes"
	"strings"
	"testing"
)

func TestDecodeRealConfig(t *testing.T) {
	c, err := DecodeConfig800(cfg800())
	if err != nil {
		t.Fatal(err)
	}
	// Mesmos valores que o PyGMC leu desse aparelho.
	want := map[string]int{
		"saveMode": 1, "powerSaving": 1, "backlight": 10, "led": 1, "fastEstimate": 3,
		"alarmVolume": 15, "clickSound": 0, "voiceVolume": 0, "vibration": 1, "alarmCPM": 100,
		"theme": 0, // byte 129 (no 1.08, 0 = modo escuro)
	}
	for k, v := range want {
		if c.Values[k] != v {
			t.Errorf("%s = %d, esperava %d", k, c.Values[k], v)
		}
	}
	if c.TubeVoltage != 44 || !c.Editable || !c.CalibrationEditable || len(c.Locked) != 0 {
		t.Fatalf("%+v", c)
	}
}

func TestDecodeLocksStrangeField(t *testing.T) {
	cfg := cfg800()
	cfg[71] = 200 // volume do alarme absurdo
	c, _ := DecodeConfig800(cfg)
	if !c.Editable || len(c.Locked) != 1 || c.Locked[0] != "alarmVolume" {
		t.Fatalf("%+v", c)
	}
	// mudar outro campo funciona e não mexe no travado
	c.Values["clickSound"] = 1
	out, err := EncodeConfig800(cfg, c)
	if err != nil || out[71] != 200 || out[121] != 1 {
		t.Fatalf("out[71]=%d out[121]=%d err=%v", out[71], out[121], err)
	}
	// mexer no travado não
	c.Values["alarmVolume"] = 5
	if _, err := EncodeConfig800(cfg, c); err == nil {
		t.Fatal("deveria recusar campo travado")
	}
}

func TestDecodeBlocksWrongLayout(t *testing.T) {
	cfg := cfg800()
	copy(cfg[73:], bytes.Repeat([]byte{0x12}, 48)) // calibração vira lixo
	c, _ := DecodeConfig800(cfg)
	if c.Editable {
		t.Fatalf("deveria bloquear: %+v", c)
	}
	if _, err := EncodeConfig800(cfg, c); err == nil {
		t.Fatal("deveria recusar")
	}
}

func TestEncodeTouchesOnlyChangedBytes(t *testing.T) {
	base := cfg800()
	c, _ := DecodeConfig800(base)
	c.Values["saveMode"] = 2
	c.Values["alarmCPM"] = 0x3E3E // bytes iguais a '>' de propósito
	c.Values["theme"] = 4
	c.Values["fastEstimate"] = 60
	out, err := EncodeConfig800(base, c)
	if err != nil {
		t.Fatal(err)
	}
	changed := map[int]bool{32: true, 127: true, 128: true, 129: true, 69: true}
	for i := range base {
		if base[i] != out[i] && !changed[i] {
			t.Fatalf("byte %d mudou sem querer", i)
		}
	}
	if out[129] != 4 {
		t.Fatalf("tema Dark deveria gravar 4, gravou %d", out[129])
	}
	back, _ := DecodeConfig800(out)
	if back.Values["saveMode"] != 2 || back.Values["alarmCPM"] != 0x3E3E || back.Values["theme"] != 4 || back.Values["fastEstimate"] != 60 {
		t.Fatalf("%+v", back.Values)
	}
	if d := DescribeChanges(c0(t), back); !strings.Contains(d, "gravação na memória a cada minuto") || !strings.Contains(d, "tema Dark") || !strings.Contains(d, "estimativa rápida 60 s") {
		t.Fatalf("resumo: %s", d)
	}
	for k, v := range map[string]int{"saveMode": 6, "alarmCPM": 0, "alarmVolume": 16, "backlight": 31, "fastEstimate": 7, "led": 2} {
		bad, _ := DecodeConfig800(base)
		bad.Values[k] = v
		if _, err := EncodeConfig800(base, bad); err == nil {
			t.Fatalf("deveria recusar %s=%d", k, v)
		}
	}
}

func c0(t *testing.T) Config800 {
	c, err := DecodeConfig800(cfg800())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestEncodeCalibration(t *testing.T) {
	base := cfg800()
	c, _ := DecodeConfig800(base)
	// sensibilidade única de 132 CPM por µSv/h (a do manual Re.1.04)
	for i := range c.Calibration.Points {
		c.Calibration.Points[i].CPM = 132 * c.Calibration.Points[i].USv
	}
	out, err := EncodeConfig800(base, c)
	if err != nil {
		t.Fatal(err)
	}
	for i := range base {
		if base[i] != out[i] && (i < 73 || i >= 97) {
			t.Fatalf("byte %d fora dos CPM da calibração mudou", i)
		}
	}
	back, _ := DecodeConfig800(out)
	if back.Calibration.Points[0].CPM != 1320 || back.Calibration.Points[5].CPM != 264000 {
		t.Fatalf("%+v", back.Calibration.Points)
	}
	if got := back.Calibration.USv(132); got < 0.9999 || got > 1.0001 {
		t.Fatalf("132 CPM = %v µSv/h", got)
	}
	if !strings.Contains(DescribeChanges(c0(t), back), "calibração") {
		t.Fatal("resumo sem calibração")
	}

	for name, mut := range map[string]func(p []CalPoint){
		"fora de ordem":   func(p []CalPoint) { p[2].USv = p[1].USv },
		"sensível demais": func(p []CalPoint) { p[0].CPM = 5000 * p[0].USv },
		"zero":            func(p []CalPoint) { p[0].USv = 0 },
		"cinco pontos":    nil,
	} {
		bad, _ := DecodeConfig800(base)
		if mut == nil {
			bad.Calibration.Points = bad.Calibration.Points[:5]
		} else {
			mut(bad.Calibration.Points)
		}
		if _, err := EncodeConfig800(base, bad); err == nil {
			t.Fatalf("deveria recusar calibração %s", name)
		}
	}
}

func TestDiffConfig(t *testing.T) {
	a := cfg800()
	b := append([]byte(nil), a...)
	b[124] = 0
	b[129] = 1
	b[80] = 9
	d := DiffConfig(a, b)
	if len(d) != 3 || d[0].Field != "calibração, CPM do ponto 2" || d[1].Field != "" || d[1].Index != 124 || d[2].Field != "tema" {
		t.Fatalf("%+v", d)
	}
}

func TestWriteConfigRoundTrip(t *testing.T) {
	s := newSim()
	d := New(s)
	base, _ := d.Config()
	c, _ := DecodeConfig800(base)
	c.Values["saveMode"] = 2
	c.Values["alarmCPM"] = 250
	c.Values["clickSound"] = 1
	out, _ := EncodeConfig800(base, c)

	if err := d.WriteConfig(out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(s.cfg, out) || s.cfgLoaded != 1 {
		t.Fatalf("contador ficou diferente (CFGUPDATE %d)", s.cfgLoaded)
	}
	if cpm, err := d.CPM(); err != nil || cpm != 22 {
		t.Fatalf("CPM depois = %d %v", cpm, err)
	}
}

func TestWriteConfigRetriesSilentFailure(t *testing.T) {
	s := newSim()
	d := New(s)
	base, _ := d.Config()
	c, _ := DecodeConfig800(base)
	c.Values["clickSound"] = 1
	out, _ := EncodeConfig800(base, c)
	s.dropWCFG = 5
	if err := d.WriteConfig(out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(s.cfg, out) || s.cfgLoaded != 2 {
		t.Fatalf("esperava acertar na 2ª tentativa (CFGUPDATE %d)", s.cfgLoaded)
	}
}

func TestWriteConfigGivesUp(t *testing.T) {
	s := newSim()
	d := New(s)
	base, _ := d.Config()
	s.dropWCFG = 1 << 20
	if err := d.WriteConfig(base); err == nil {
		t.Fatal("deveria falhar")
	}
}
