package gmc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// Layout da configuração de 512 bytes do GMC-800. A GQ não publicou o mapa;
// estas posições vêm do PyGMC, conferidas com a configuração de um aparelho
// real (firmware 1.08). Outros firmwares podem mudar o layout, por isso cada
// campo tem uma faixa de valores esperados: se o valor lido estiver fora dela,
// o campo fica travado e o app não grava nele.

type fieldKind int

const (
	kindByte fieldKind = iota
	kindBool
	kindU32
)

type field struct {
	Key    string
	Name   string
	Off    int
	Kind   fieldKind
	Min    int
	Max    int
	OneOf  []int
	Invert bool // bool gravado ao contrário (0 = ligado)
}

var fields800 = []field{
	{Key: "saveMode", Name: "gravação na memória", Off: 32, Kind: kindByte, Min: 0, Max: 5},
	{Key: "powerSaving", Name: "economia de energia", Off: 44, Kind: kindByte, Min: 0, Max: 2},
	{Key: "backlight", Name: "brilho da tela", Off: 53, Kind: kindByte, Min: 0, Max: 30},
	{Key: "led", Name: "LED", Off: 60, Kind: kindBool},
	{Key: "fastEstimate", Name: "estimativa rápida", Off: 69, Kind: kindByte, OneOf: []int{3, 5, 10, 15, 20, 30, 60}},
	{Key: "alarmVolume", Name: "volume do alarme", Off: 71, Kind: kindByte, Min: 0, Max: 15},
	{Key: "clickSound", Name: "som de clique", Off: 121, Kind: kindBool},
	{Key: "voiceVolume", Name: "volume da voz", Off: 122, Kind: kindByte, Min: 0, Max: 15},
	{Key: "vibration", Name: "vibração", Off: 123, Kind: kindBool},
	{Key: "alarmCPM", Name: "limite do alarme", Off: 125, Kind: kindU32, Min: 1, Max: 999999},
	{Key: "darkMode", Name: "modo escuro", Off: 129, Kind: kindBool, Invert: true},
}

const (
	cfgTubeVoltage = 72 // só leitura
	calPoints      = 6
	// Faixa plausível de sensibilidade (CPM por µSv/h) para tubos de Geiger
	// de contadores de mão. Fora disso, provavelmente é erro de digitação.
	minSensitivity = 20
	maxSensitivity = 1000
)

func (f field) size() int {
	if f.Kind == kindU32 {
		return 4
	}
	return 1
}

func (f field) get(cfg []byte) int {
	switch f.Kind {
	case kindU32:
		return int(binary.BigEndian.Uint32(cfg[f.Off:]))
	case kindBool:
		v := int(cfg[f.Off])
		if f.Invert && v <= 1 {
			v = 1 - v
		}
		return v
	}
	return int(cfg[f.Off])
}

func (f field) raw(cfg []byte) int {
	if f.Kind == kindU32 {
		return int(binary.BigEndian.Uint32(cfg[f.Off:]))
	}
	return int(cfg[f.Off])
}

func (f field) valid(v int) bool {
	switch {
	case f.Kind == kindBool:
		return v == 0 || v == 1
	case len(f.OneOf) > 0:
		for _, o := range f.OneOf {
			if v == o {
				return true
			}
		}
		return false
	}
	return v >= f.Min && v <= f.Max
}

func (f field) put(cfg []byte, v int) {
	switch f.Kind {
	case kindU32:
		binary.BigEndian.PutUint32(cfg[f.Off:], uint32(v))
	case kindBool:
		if f.Invert {
			v = 1 - v
		}
		cfg[f.Off] = byte(v)
	default:
		cfg[f.Off] = byte(v)
	}
}

func (f field) describe(v int) string {
	switch {
	case f.Key == "saveMode":
		return saveModeNames[v]
	case f.Key == "powerSaving":
		return powerSavingNames[v]
	case f.Key == "fastEstimate" && v == 3:
		return "dinâmica"
	case f.Key == "fastEstimate":
		return fmt.Sprintf("%d s", v)
	case f.Kind == kindBool && v == 1:
		return "ligado"
	case f.Kind == kindBool:
		return "desligado"
	case f.Key == "alarmCPM":
		return fmt.Sprintf("%d CPM", v)
	}
	return fmt.Sprint(v)
}

var saveModeNames = map[int]string{
	0: "desligada", 1: "a cada segundo", 2: "a cada minuto", 3: "a cada hora",
	4: "por limiar (segundo)", 5: "por limiar (minuto)",
}

var powerSavingNames = map[int]string{0: "escurecer a tela", 1: "apagar a tela", 2: "desligada"}

// Config800 são os ajustes do GMC-800 que o app sabe ler e mudar.
type Config800 struct {
	// Values usa as chaves de fields800; booleanos são 0 ou 1.
	Values      map[string]int `json:"values"`
	Calibration Calibration    `json:"calibration"`
	TubeVoltage int            `json:"tubeVoltage"`

	// Campos com valor fora do esperado: aparecem, mas não são gravados.
	Locked []string `json:"locked"`
	// Calibração com 6 pontos legíveis e plausíveis.
	CalibrationEditable bool     `json:"calibrationEditable"`
	Editable            bool     `json:"editable"`
	Problems            []string `json:"problems"`
}

// DecodeConfig800 interpreta a configuração.
//
// A calibração serve de "assinatura" do layout: se os 6 pontos não forem
// legíveis e plausíveis, nada é editável. Se mais de dois campos vierem com
// valores estranhos, também não (o layout provavelmente é outro).
func DecodeConfig800(cfg []byte) (Config800, error) {
	if len(cfg) != ConfigSize {
		return Config800{}, fmt.Errorf("configuração com %d bytes, esperava %d", len(cfg), ConfigSize)
	}
	c := Config800{Values: map[string]int{}, TubeVoltage: int(cfg[cfgTubeVoltage])}
	for _, f := range fields800 {
		v := f.get(cfg)
		c.Values[f.Key] = v
		if !f.valid(f.raw(cfg)) {
			c.Locked = append(c.Locked, f.Key)
			c.Problems = append(c.Problems, fmt.Sprintf("%s com valor %d", f.Name, f.raw(cfg)))
		}
	}
	if cal, err := ParseCalibration800(cfg); err == nil {
		c.Calibration = cal
		if len(cal.Points) == calPoints && checkCalibration(cal.Points) == nil {
			c.CalibrationEditable = true
		} else {
			c.Problems = append(c.Problems, "calibração com pontos estranhos")
		}
	} else {
		c.Calibration = DefaultCalibration()
		c.Problems = append(c.Problems, "calibração ilegível")
	}
	c.Editable = c.CalibrationEditable && len(c.Locked) <= 2
	return c, nil
}

// checkCalibration confere se os pontos fazem sentido.
func checkCalibration(pts []CalPoint) error {
	if len(pts) != calPoints {
		return fmt.Errorf("a calibração precisa de %d pontos", calPoints)
	}
	for i, p := range pts {
		if math.IsNaN(p.USv) || math.IsInf(p.USv, 0) || p.USv <= 0 || p.CPM < 1 || p.CPM > math.MaxUint32 || p.CPM != math.Trunc(p.CPM) {
			return fmt.Errorf("ponto %d inválido", i+1)
		}
		if i > 0 && (p.CPM <= pts[i-1].CPM || p.USv <= pts[i-1].USv) {
			return fmt.Errorf("os pontos precisam crescer em CPM e em µSv/h (ponto %d)", i+1)
		}
		s := p.CPM / p.USv
		if s < minSensitivity || s > maxSensitivity {
			return fmt.Errorf("ponto %d dá %.0f CPM por µSv/h, fora da faixa plausível (%d a %d)", i+1, s, minSensitivity, maxSensitivity)
		}
	}
	return nil
}

// EncodeConfig800 aplica os valores de c sobre uma cópia de base. Só os
// campos conhecidos e não travados são tocados; o resto fica igual.
func EncodeConfig800(base []byte, c Config800) ([]byte, error) {
	cur, err := DecodeConfig800(base)
	if err != nil {
		return nil, err
	}
	if !cur.Editable {
		return nil, fmt.Errorf("a configuração do contador tem valores que o app não reconhece (%s)", strings.Join(cur.Problems, "; "))
	}
	locked := map[string]bool{}
	for _, k := range cur.Locked {
		locked[k] = true
	}
	out := append([]byte(nil), base...)
	for _, f := range fields800 {
		v, ok := c.Values[f.Key]
		if !ok || v == cur.Values[f.Key] {
			continue
		}
		if locked[f.Key] {
			return nil, fmt.Errorf("%s está travado (valor não reconhecido no contador)", f.Name)
		}
		if !f.valid(v) {
			return nil, fmt.Errorf("%s: valor %d fora da faixa aceita", f.Name, v)
		}
		f.put(out, v)
	}
	if pts := c.Calibration.Points; len(pts) > 0 && !samePoints(pts, cur.Calibration.Points) {
		pts = append([]CalPoint(nil), pts...)
		sort.Slice(pts, func(i, j int) bool { return pts[i].CPM < pts[j].CPM })
		if err := checkCalibration(pts); err != nil {
			return nil, fmt.Errorf("calibração: %w", err)
		}
		for i, p := range pts {
			binary.BigEndian.PutUint32(out[cfgCalCPM+4*i:], uint32(p.CPM))
			binary.BigEndian.PutUint32(out[cfgCalUSv+4*i:], math.Float32bits(float32(p.USv)))
		}
	}
	return out, nil
}

func samePoints(a, b []CalPoint) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].CPM != b[i].CPM || float32(a[i].USv) != float32(b[i].USv) {
			return false
		}
	}
	return true
}

// DescribeChanges resume em português o que mudou entre duas configurações.
func DescribeChanges(a, b Config800) string {
	var parts []string
	for _, f := range fields800 {
		if a.Values[f.Key] != b.Values[f.Key] {
			parts = append(parts, f.Name+" "+f.describe(b.Values[f.Key]))
		}
	}
	if !samePoints(a.Calibration.Points, b.Calibration.Points) {
		parts = append(parts, "calibração")
	}
	if len(parts) == 0 {
		return "nada mudou"
	}
	return strings.Join(parts, ", ")
}

// ByteDiff é um byte que mudou entre duas leituras da configuração.
type ByteDiff struct {
	Index  int    `json:"index"`
	Before int    `json:"before"`
	After  int    `json:"after"`
	Field  string `json:"field"` // nome do campo conhecido, ou ""
}

// DiffConfig compara duas configurações byte a byte.
func DiffConfig(a, b []byte) []ByteDiff {
	var out []ByteDiff
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			out = append(out, ByteDiff{Index: i, Before: int(a[i]), After: int(b[i]), Field: FieldAt(i)})
		}
	}
	return out
}

// FieldAt diz qual campo conhecido ocupa a posição i.
func FieldAt(i int) string {
	for _, f := range fields800 {
		if i >= f.Off && i < f.Off+f.size() {
			return f.Name
		}
	}
	switch {
	case i == cfgTubeVoltage:
		return "tensão do tubo"
	case i >= cfgCalCPM && i < cfgCalCPM+4*calPoints:
		return fmt.Sprintf("calibração, CPM do ponto %d", (i-cfgCalCPM)/4+1)
	case i >= cfgCalUSv && i < cfgCalUSv+4*calPoints:
		return fmt.Sprintf("calibração, µSv/h do ponto %d", (i-cfgCalUSv)/4+1)
	}
	return ""
}

// ack lê a resposta 0xAA de um comando de configuração. Alguns firmwares
// mandam texto antes do 0xAA (ex.: "0071\r\n\xaa" no CFGUPDATE).
func (d *Device) ack(cmd []byte, timeout time.Duration) error {
	if err := d.port.ResetInputBuffer(); err != nil {
		return err
	}
	if _, err := d.port.Write(cmd); err != nil {
		return err
	}
	if err := d.port.SetReadTimeout(pollStep); err != nil {
		return err
	}
	deadline := time.Now().Add(timeout)
	b := make([]byte, 32)
	for time.Now().Before(deadline) {
		n, err := d.port.Read(b)
		if err != nil {
			return err
		}
		if bytes.IndexByte(b[:n], 0xAA) >= 0 {
			return nil
		}
	}
	return ErrTimeout
}

// writeConfigOnce apaga a configuração e grava cfg byte a byte.
func (d *Device) writeConfigOnce(cfg []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// ECFG às vezes não responde de primeira.
	var err error
	for i := 0; i < 3; i++ {
		if err = d.ack([]byte("<ECFG>>"), 2*time.Second); err == nil {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("ECFG: %w", err)
	}
	// Depois do ECFG tudo vira 0xFF; só precisa gravar até o último byte útil.
	end := len(cfg)
	for end > 0 && cfg[end-1] == 0xFF {
		end--
	}
	for i := 0; i < end; i++ {
		if cfg[i] == 0xFF {
			continue
		}
		cmd := []byte{'<', 'W', 'C', 'F', 'G', byte(i >> 8), byte(i), cfg[i], '>', '>'}
		if err := d.ack(cmd, time.Second); err != nil {
			return fmt.Errorf("WCFG no byte %d: %w", i, err)
		}
	}
	if err := d.ack([]byte("<CFGUPDATE>>"), 2*time.Second); err != nil {
		return fmt.Errorf("CFGUPDATE: %w", err)
	}
	time.Sleep(100 * time.Millisecond)
	_ = d.port.ResetInputBuffer()
	return nil
}

// WriteConfig grava a configuração inteira e confere lendo de volta.
// Tenta até 3 vezes, porque a gravação do GMC às vezes falha em silêncio.
func (d *Device) WriteConfig(cfg []byte) error {
	if len(cfg) != ConfigSize {
		return fmt.Errorf("configuração com %d bytes, esperava %d", len(cfg), ConfigSize)
	}
	var last error
	for attempt := 1; attempt <= 3; attempt++ {
		if err := d.writeConfigOnce(cfg); err != nil {
			last = err
			continue
		}
		got, err := d.Config()
		if err != nil {
			last = err
			continue
		}
		if bytes.Equal(got, cfg) {
			return nil
		}
		last = fmt.Errorf("a configuração lida de volta não confere (byte %d)", firstDiff(got, cfg))
	}
	return last
}

func firstDiff(a, b []byte) int {
	for i := range a {
		if i >= len(b) || a[i] != b[i] {
			return i
		}
	}
	return len(a)
}
