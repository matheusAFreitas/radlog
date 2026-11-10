package radiacode

import (
	"fmt"
	"math"
	"strings"
)

// ---------- ajustes gravados no aparelho ----------
//
// Os registradores e a escala de cada um seguem a biblioteca aberta
// (cdump/radiacode) e o SFR_FILE que o próprio aparelho manda. Na leitura o
// valor vem no byte baixo do u32 (conferido com um RadiaCode-102, firmware
// 4.15); a biblioteca lê alguns no byte alto, então os dois são aceitos.

const (
	sfrDispBrt     = 0x0511
	sfrDispOffTime = 0x0513
	sfrDispDir     = 0x0515
	sfrSoundCtrl   = 0x0520
	sfrSoundOn     = 0x0522
	sfrVibroCtrl   = 0x0530
	sfrVibroOn     = 0x0531
	sfrDSLev1x100  = 0x8002 // 100 µR (cópia do DS_LEV1 em outra escala)
	sfrDSLev2x100  = 0x8003
	sfrDoseReset   = 0x8007
)

// Bits de SOUND_CTRL e VIBRO_CTRL: o que faz o aparelho apitar ou vibrar.
// Os bits acima de 7 não têm nome conhecido e são mantidos como estão.
const (
	CtrlButtons     = 1 << 0
	CtrlClicks      = 1 << 1 // só som: um clique por partícula
	CtrlRateAlarm1  = 1 << 2
	CtrlRateAlarm2  = 1 << 3
	CtrlRateOver    = 1 << 4
	CtrlDoseAlarm1  = 1 << 5
	CtrlDoseAlarm2  = 1 << 6
	CtrlDoseOver    = 1 << 7
	ctrlKnown       = 0xff
	ctrlVibroKnown  = ctrlKnown &^ CtrlClicks
	doseOffRegister = 999999999 // µR: o "desligado" de fábrica (~10 Sv)
)

// Limites aceitos (do DeviceParams do aparelho: rem/h e rem → µSv/h e µSv).
const (
	RateMinUSv  = 0.01
	RateMaxUSv  = 1000
	DoseMinUSv  = 0.01
	DoseMaxUSv  = 1e7 // 1.000 rem = 10 Sv
	CountMinCPS = 0.1
	CountMaxCPS = 50000
	// DoseOffUSv: acima disso o alarme de dose conta como desligado
	DoseOffUSv = 9e6
)

// DefaultOffTimes são os tempos de apagar a tela quando a configuração do
// aparelho não lista os dela (a biblioteca conhece só os quatro primeiros).
var DefaultOffTimes = []string{"5 s", "10 s", "15 s", "30 s"}

// Settings são os ajustes do aparelho que o app sabe ler e gravar.
type Settings struct {
	// alarmes: taxa de dose (µSv/h), dose acumulada (µSv), contagem (cps)
	DoseRate1  float64 `json:"doseRate1"`
	DoseRate2  float64 `json:"doseRate2"`
	Dose1      float64 `json:"dose1"`
	Dose2      float64 `json:"dose2"`
	CountRate1 float64 `json:"countRate1"`
	CountRate2 float64 `json:"countRate2"`
	// avisos
	SoundOn   bool   `json:"soundOn"`
	SoundCtrl uint32 `json:"soundCtrl"`
	VibroOn   bool   `json:"vibroOn"`
	VibroCtrl uint32 `json:"vibroCtrl"`
	// tela
	Brightness int      `json:"brightness"` // 0–9
	OffTime    int      `json:"offTime"`    // índice em OffTimes
	OffTimes   []string `json:"offTimes"`
	Direction  int      `json:"direction"` // 0 automática, 1 direita, 2 esquerda
	// unidades da tela do aparelho
	UnitSv  bool `json:"unitSv"`  // Sv (senão R)
	UnitCPM bool `json:"unitCPM"` // cpm (senão cps)
	// só leitura
	EnergyA0 float64 `json:"energyA0"`
	EnergyA1 float64 `json:"energyA1"`
	EnergyA2 float64 `json:"energyA2"`
}

// byteVal lê um registro de 1 byte (no byte baixo ou no alto).
func byteVal(v uint32) uint32 {
	if v&0xff != 0 || v>>8 == 0 {
		return v & 0xff
	}
	return v >> 24
}

// half lê um registro de 2 bytes (na metade baixa ou na alta).
func half(v uint32) uint32 {
	if v&0xffff != 0 || v>>16 == 0 {
		return v & 0xffff
	}
	return v >> 16
}

// settingsRegs é a ordem de leitura (todos de uma vez).
var settingsRegs = []uint32{
	sfrDRLev1, sfrDRLev2, sfrDSLev1, sfrDSLev2, sfrCRLev1, sfrCRLev2,
	sfrSoundOn, sfrSoundCtrl, sfrVibroOn, sfrVibroCtrl,
	sfrDispBrt, sfrDispOffTime, sfrDispDir, sfrDSUnits, sfrCRUnits,
	0x8010, 0x8011, 0x8012,
}

func (d *Device) readSFRs(ids []uint32) ([]uint32, error) {
	args := u32(uint32(len(ids)))
	for _, id := range ids {
		args = append(args, u32(id)...)
	}
	r, err := d.execute(cmdRdVirtSFRBat, args)
	if err != nil {
		return nil, err
	}
	ok, err := r.u32()
	if err != nil {
		return nil, err
	}
	if ok != 1<<len(ids)-1 {
		var bad []string
		for i, id := range ids {
			if ok&(1<<i) == 0 {
				bad = append(bad, fmt.Sprintf("%#x", id))
			}
		}
		return nil, fmt.Errorf("o aparelho recusou os registros %s", strings.Join(bad, ", "))
	}
	v := make([]uint32, len(ids))
	for i := range v {
		if v[i], err = r.u32(); err != nil {
			return nil, err
		}
	}
	return v, nil
}

// ReadSettings lê os ajustes do aparelho.
func (d *Device) ReadSettings() (Settings, error) {
	v, err := d.readSFRs(settingsRegs)
	if err != nil {
		return Settings{}, err
	}
	s := Settings{
		DoseRate1: float64(v[0]) / 100, DoseRate2: float64(v[1]) / 100,
		Dose1: float64(v[2]) / 100, Dose2: float64(v[3]) / 100,
		CountRate1: float64(v[4]) / 10, CountRate2: float64(v[5]) / 10,
		SoundOn: byteVal(v[6]) != 0, SoundCtrl: half(v[7]),
		VibroOn:    byteVal(v[8]) != 0,
		Brightness: int(byteVal(v[10])), OffTime: int(v[11]), Direction: int(byteVal(v[12])),
		UnitSv: byteVal(v[13]) != 0, UnitCPM: byteVal(v[14]) != 0,
		EnergyA0: float64(math.Float32frombits(v[15])),
		EnergyA1: float64(math.Float32frombits(v[16])),
		EnergyA2: float64(math.Float32frombits(v[17])),
	}
	// VIBRO_CTRL tem 1 byte no SFR_FILE, mas o aparelho devolve bits acima
	// (0x1cfe): guarda o u32 inteiro para regravar igual
	s.VibroCtrl = v[9]
	if v[9]&0xff == 0 && v[9]>>24 != 0 {
		s.VibroCtrl = v[9] >> 24
	}
	s.OffTimes = DefaultOffTimes
	if cfg, err := d.Configuration(); err == nil {
		if o := OffTimesFromConfig(cfg); len(o) > 0 {
			s.OffTimes = o
		}
	}
	return s, nil
}

// OffTimesFromConfig tira da configuração do aparelho a lista de tempos de
// apagar a tela (VSFR_DISP_OFF_TIME="5 s;10 s;…").
func OffTimesFromConfig(cfg string) []string {
	for _, ln := range strings.Split(cfg, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(ln), "=")
		if !ok || strings.TrimSpace(k) != "VSFR_DISP_OFF_TIME" {
			continue
		}
		var out []string
		for _, p := range strings.Split(strings.Trim(strings.TrimSpace(v), `"`), ";") {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		return out
	}
	return nil
}

// Check confere se os ajustes cabem no que o aparelho aceita. Devolve a
// lista de problemas em português (vazia = pode gravar).
func (s Settings) Check() []string {
	var p []string
	bad := func(v float64) bool { return math.IsNaN(v) || math.IsInf(v, 0) }
	if bad(s.DoseRate1) || bad(s.DoseRate2) || s.DoseRate1 < RateMinUSv || s.DoseRate2 > RateMaxUSv {
		p = append(p, "os alarmes de taxa de dose precisam ficar entre 0,01 e 1.000 µSv/h")
	} else if s.DoseRate2 <= s.DoseRate1 {
		p = append(p, "o alarme 2 de taxa de dose precisa ser maior que o alarme 1")
	}
	if bad(s.Dose1) || bad(s.Dose2) || s.Dose1 < DoseMinUSv || s.Dose2 > DoseMaxUSv {
		p = append(p, "os alarmes de dose precisam ficar entre 0,01 µSv e 10 Sv")
	} else if s.Dose2 < s.Dose1 {
		p = append(p, "o alarme 2 de dose não pode ser menor que o alarme 1")
	}
	if bad(s.CountRate1) || bad(s.CountRate2) || s.CountRate1 < CountMinCPS || s.CountRate2 > CountMaxCPS {
		p = append(p, "os alarmes de contagem precisam ficar entre 0,1 e 50.000 cps")
	} else if s.CountRate2 <= s.CountRate1 {
		p = append(p, "o alarme 2 de contagem precisa ser maior que o alarme 1")
	}
	if s.Brightness < 0 || s.Brightness > 9 {
		p = append(p, "o brilho vai de 0 a 9")
	}
	n := len(s.OffTimes)
	if n == 0 {
		n = len(DefaultOffTimes)
	}
	if s.OffTime < 0 || s.OffTime >= n {
		p = append(p, "tempo para apagar a tela desconhecido")
	}
	if s.Direction < 0 || s.Direction > 2 {
		p = append(p, "orientação da tela desconhecida")
	}
	return p
}

// regWrite é uma gravação de um registrador.
type regWrite struct {
	id  uint32
	val uint32
}

func b2u(b bool) uint32 {
	if b {
		return 1
	}
	return 0
}

func doseReg(usv float64) uint32 {
	if usv >= DoseOffUSv {
		return doseOffRegister
	}
	return uint32(math.Round(usv * 100))
}

// writes lista o que muda de cur para next (só os campos que o app grava).
func writes(cur, next Settings) []regWrite {
	var w []regWrite
	add := func(changed bool, id, val uint32) {
		if changed {
			w = append(w, regWrite{id, val})
		}
	}
	rate := func(v float64) uint32 { return uint32(math.Round(v * 100)) }
	cnt := func(v float64) uint32 { return uint32(math.Round(v * 10)) }
	add(rate(cur.DoseRate1) != rate(next.DoseRate1), sfrDRLev1, rate(next.DoseRate1))
	add(rate(cur.DoseRate2) != rate(next.DoseRate2), sfrDRLev2, rate(next.DoseRate2))
	if d1 := doseReg(next.Dose1); doseReg(cur.Dose1) != d1 {
		w = append(w, regWrite{sfrDSLev1, d1}, regWrite{sfrDSLev1x100, d1 / 100})
	}
	if d2 := doseReg(next.Dose2); doseReg(cur.Dose2) != d2 {
		w = append(w, regWrite{sfrDSLev2, d2}, regWrite{sfrDSLev2x100, d2 / 100})
	}
	add(cnt(cur.CountRate1) != cnt(next.CountRate1), sfrCRLev1, cnt(next.CountRate1))
	add(cnt(cur.CountRate2) != cnt(next.CountRate2), sfrCRLev2, cnt(next.CountRate2))
	add(cur.SoundOn != next.SoundOn, sfrSoundOn, b2u(next.SoundOn))
	// só os bits conhecidos vêm da tela; os outros ficam como o aparelho tem
	snd := cur.SoundCtrl&^ctrlKnown | next.SoundCtrl&ctrlKnown
	add(snd != cur.SoundCtrl, sfrSoundCtrl, snd)
	add(cur.VibroOn != next.VibroOn, sfrVibroOn, b2u(next.VibroOn))
	vib := cur.VibroCtrl&^ctrlVibroKnown | next.VibroCtrl&ctrlVibroKnown
	add(vib != cur.VibroCtrl, sfrVibroCtrl, vib)
	add(cur.Brightness != next.Brightness, sfrDispBrt, uint32(next.Brightness))
	add(cur.OffTime != next.OffTime, sfrDispOffTime, uint32(next.OffTime))
	add(cur.Direction != next.Direction, sfrDispDir, uint32(next.Direction))
	add(cur.UnitSv != next.UnitSv, sfrDSUnits, b2u(next.UnitSv))
	add(cur.UnitCPM != next.UnitCPM, sfrCRUnits, b2u(next.UnitCPM))
	return w
}

// WriteSettings grava no aparelho só o que mudou de cur para next e devolve
// o que foi lido de volta. cur deve ser a leitura de agora.
func (d *Device) WriteSettings(cur, next Settings) (Settings, int, error) {
	if p := next.Check(); len(p) > 0 {
		return cur, 0, fmt.Errorf("não gravei: %s", strings.Join(p, "; "))
	}
	ws := writes(cur, next)
	for i, w := range ws {
		if err := d.writeSFR(w.id, u32(w.val)); err != nil {
			// a cópia em 100 µR é só espelho: se o firmware não aceitar, segue
			if w.id == sfrDSLev1x100 || w.id == sfrDSLev2x100 {
				continue
			}
			got, _ := d.ReadSettings()
			return got, i, err
		}
	}
	got, err := d.ReadSettings()
	return got, len(ws), err
}

// ResetDose zera a dose acumulada do aparelho (não tem volta).
func (d *Device) ResetDose() error {
	r, err := d.execute(cmdWrVirtSFR, u32(sfrDoseReset))
	if err != nil {
		return err
	}
	if ret, err := r.u32(); err != nil || ret != 1 {
		return fmt.Errorf("o aparelho recusou zerar a dose")
	}
	return nil
}

// ---------- texto das mudanças ----------

var dirNames = []string{"automática", "virada para a direita", "virada para a esquerda"}

func onOff(b bool) string {
	if b {
		return "ligado"
	}
	return "desligado"
}

func usvStr(v float64) string { return strings.Replace(fmt.Sprintf("%g", v), ".", ",", 1) }

func doseStr(v float64) string {
	if v >= DoseOffUSv {
		return "desligado"
	}
	if v >= 1000 {
		return usvStr(v/1000) + " mSv"
	}
	return usvStr(v) + " µSv"
}

// CtrlNames dá nome aos bits conhecidos de som/vibração.
var CtrlNames = []struct {
	Bit  uint32
	Name string
}{
	{CtrlButtons, "botões"}, {CtrlClicks, "cliques"}, {CtrlRateAlarm1, "alarme 1 de taxa"},
	{CtrlRateAlarm2, "alarme 2 de taxa"}, {CtrlRateOver, "taxa fora da escala"},
	{CtrlDoseAlarm1, "alarme 1 de dose"}, {CtrlDoseAlarm2, "alarme 2 de dose"}, {CtrlDoseOver, "dose fora da escala"},
}

func ctrlDiff(a, b uint32) string {
	var on, off []string
	for _, c := range CtrlNames {
		if a&c.Bit != b&c.Bit {
			if b&c.Bit != 0 {
				on = append(on, c.Name)
			} else {
				off = append(off, c.Name)
			}
		}
	}
	var p []string
	if len(on) > 0 {
		p = append(p, "+"+strings.Join(on, ", +"))
	}
	if len(off) > 0 {
		p = append(p, "−"+strings.Join(off, ", −"))
	}
	return strings.Join(p, ", ")
}

func pick(list []string, i int) string {
	if i >= 0 && i < len(list) {
		return list[i]
	}
	return fmt.Sprint(i)
}

// DescribeChanges escreve, em português, o que mudou de a para b.
func DescribeChanges(a, b Settings) string {
	var p []string
	f := func(name, x, y string) {
		if x != y {
			p = append(p, name+": "+x+" → "+y)
		}
	}
	f("alarme 1 de taxa", usvStr(a.DoseRate1)+" µSv/h", usvStr(b.DoseRate1)+" µSv/h")
	f("alarme 2 de taxa", usvStr(a.DoseRate2)+" µSv/h", usvStr(b.DoseRate2)+" µSv/h")
	f("alarme 1 de dose", doseStr(a.Dose1), doseStr(b.Dose1))
	f("alarme 2 de dose", doseStr(a.Dose2), doseStr(b.Dose2))
	f("alarme 1 de contagem", usvStr(a.CountRate1)+" cps", usvStr(b.CountRate1)+" cps")
	f("alarme 2 de contagem", usvStr(a.CountRate2)+" cps", usvStr(b.CountRate2)+" cps")
	f("som", onOff(a.SoundOn), onOff(b.SoundOn))
	if d := ctrlDiff(a.SoundCtrl, b.SoundCtrl&ctrlKnown|a.SoundCtrl&^ctrlKnown); d != "" {
		p = append(p, "som em: "+d)
	}
	f("vibração", onOff(a.VibroOn), onOff(b.VibroOn))
	if d := ctrlDiff(a.VibroCtrl&ctrlVibroKnown, b.VibroCtrl&ctrlVibroKnown); d != "" {
		p = append(p, "vibração em: "+d)
	}
	f("brilho", fmt.Sprint(a.Brightness), fmt.Sprint(b.Brightness))
	offs := a.OffTimes
	if len(offs) == 0 {
		offs = DefaultOffTimes
	}
	f("apagar a tela", pick(offs, a.OffTime), pick(offs, b.OffTime))
	f("orientação", pick(dirNames, a.Direction), pick(dirNames, b.Direction))
	unit := func(sv bool) string {
		if sv {
			return "Sv"
		}
		return "R"
	}
	cunit := func(cpm bool) string {
		if cpm {
			return "cpm"
		}
		return "cps"
	}
	f("unidade de dose", unit(a.UnitSv), unit(b.UnitSv))
	f("unidade de contagem", cunit(a.UnitCPM), cunit(b.UnitCPM))
	if len(p) == 0 {
		return "nada mudou"
	}
	return strings.Join(p, "; ")
}

// NewRaw monta um Device sobre um Transport sem a inicialização do Open
// (para testes de quem usa o pacote).
func NewRaw(tr Transport) *Device { return &Device{tr: tr, Model: "RadiaCode-102", Firmware: "4.15"} }
