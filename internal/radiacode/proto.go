// Package radiacode conversa com os RadiaCode 10x (101/102/103) por USB ou
// Bluetooth.
//
// O protocolo é o mesmo nos dois meios: cada pedido é
//
//	u32 tamanho | u16 comando | u8 0 | u8 0x80+seq | argumentos
//
// e a resposta começa com u32 tamanho e repete os 4 bytes do cabeçalho do
// pedido. Os comandos e o formato dos dados seguem a biblioteca aberta
// cdump/radiacode (MIT), que fez a engenharia reversa.
package radiacode

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Transport leva um pedido inteiro (já com o tamanho na frente) e devolve a
// resposta sem o tamanho.
type Transport interface {
	Exchange(req []byte) ([]byte, error)
	Close() error
	Kind() string // "usb" ou "bluetooth"
}

// ErrNotFound: nenhum aparelho nesse meio.
var ErrNotFound = errors.New("RadiaCode não encontrado")

// comandos
const (
	cmdGetStatus    = 0x0005
	cmdSetExchange  = 0x0007
	cmdGetVersion   = 0x000A
	cmdGetSerial    = 0x000B
	cmdRdVirtSFR    = 0x0824
	cmdWrVirtSFR    = 0x0825
	cmdRdVirtString = 0x0826
	cmdWrVirtString = 0x0827
	cmdRdVirtSFRBat = 0x082A
	cmdWrVirtSFRBat = 0x082B
	cmdSetTime      = 0x0A04
)

// strings virtuais
const (
	vsConfiguration = 2
	vsSerialNumber  = 8
	vsDataBuf       = 0x100
	vsSpectrum      = 0x200
	vsEnergyCalib   = 0x202
	vsSpecAccum     = 0x205
)

// registradores virtuais
const (
	sfrDeviceTime = 0x0504
	sfrDRLev1     = 0x8000 // µR/h
	sfrDRLev2     = 0x8001
	sfrDSUnits    = 0x8004
	sfrCRLev1     = 0x8008 // contagens / 10 s
	sfrCRLev2     = 0x8009
	sfrCRUnits    = 0x8013
	sfrDSLev1     = 0x8014 // µR
	sfrDSLev2     = 0x8015
)

// BaseLead é quanto o instante zero das leituras ao vivo fica à frente do
// momento em que o relógio do aparelho foi zerado (como na biblioteca aberta).
// Os registros que vêm da memória contam do momento zerado mesmo.
const BaseLead = 128 * time.Second

// DoseScale converte o valor do protocolo (dose em R e taxa em R/h, com
// 1 Sv = 100 R como o aparelho usa) para µSv e µSv/h.
const DoseScale = 1e4

// Device é um RadiaCode conectado.
type Device struct {
	tr       Transport
	mu       sync.Mutex
	seq      byte
	base     time.Time // instante zero dos registros do DATA_BUF
	specFmt  int
	Serial   string
	Firmware string
	Model    string
}

// Open inicializa o aparelho do jeito que o app oficial faz: troca de
// parâmetros, acerta o relógio e zera o contador de tempo dos registros.
func Open(tr Transport) (*Device, error) {
	d := &Device{tr: tr}
	if _, err := d.execute(cmdSetExchange, []byte{0x01, 0xff, 0x12, 0xff}); err != nil {
		tr.Close()
		return nil, fmt.Errorf("início: %w", err)
	}
	if err := d.setLocalTime(time.Now()); err != nil {
		tr.Close()
		return nil, err
	}
	if err := d.writeSFR(sfrDeviceTime, u32(0)); err != nil {
		tr.Close()
		return nil, err
	}
	d.base = time.Now().Add(BaseLead)
	if maj, min, date, err := d.fwVersion(); err == nil {
		d.Firmware = fmt.Sprintf("%d.%d (%s)", maj, min, date)
		if maj < 4 || (maj == 4 && min < 8) {
			tr.Close()
			return nil, fmt.Errorf("firmware %d.%d é antigo demais (precisa de 4.8 ou mais novo)", maj, min)
		}
	} else {
		tr.Close()
		return nil, err
	}
	if cfg, err := d.Configuration(); err == nil {
		for _, ln := range strings.Split(cfg, "\n") {
			k, v, ok := strings.Cut(strings.TrimSpace(ln), "=")
			if !ok {
				continue
			}
			switch k {
			case "SpecFormatVersion":
				d.specFmt, _ = strconv.Atoi(strings.TrimSpace(v))
			case "DeviceName", "Model":
				if d.Model == "" {
					d.Model = strings.TrimSpace(v)
				}
			}
		}
	}
	if s, err := d.readString(vsSerialNumber); err == nil {
		d.Serial = strings.TrimRight(string(s), "\x00")
	}
	// a configuração do firmware 4.15 não traz o nome: sai do número de série
	// (RC-102-015102 → RadiaCode-102)
	if d.Model == "" {
		if p := strings.Split(d.Serial, "-"); len(p) >= 2 && p[0] == "RC" {
			d.Model = "RadiaCode-" + p[1]
		}
	}
	return d, nil
}

// Kind diz por onde está conectado.
func (d *Device) Kind() string { return d.tr.Kind() }

// Close solta o aparelho.
func (d *Device) Close() error { return d.tr.Close() }

func u32(v uint32) []byte { b := make([]byte, 4); binary.LittleEndian.PutUint32(b, v); return b }

func (d *Device) execute(cmd uint16, args []byte) (*buf, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	hdr := []byte{byte(cmd), byte(cmd >> 8), 0, 0x80 + d.seq}
	d.seq = (d.seq + 1) % 32
	req := make([]byte, 4, 8+len(args))
	binary.LittleEndian.PutUint32(req, uint32(4+len(args)))
	req = append(req, hdr...)
	req = append(req, args...)
	resp, err := d.tr.Exchange(req)
	if err != nil {
		return nil, err
	}
	if len(resp) < 4 || string(resp[:4]) != string(hdr) {
		return nil, fmt.Errorf("resposta fora de ordem (pedido %x, veio %x)", hdr, resp[:min(4, len(resp))])
	}
	return &buf{b: resp[4:]}, nil
}

func (d *Device) readString(id uint32) ([]byte, error) {
	r, err := d.execute(cmdRdVirtString, u32(id))
	if err != nil {
		return nil, err
	}
	ret, err := r.u32()
	if err != nil {
		return nil, err
	}
	n, err := r.u32()
	if err != nil {
		return nil, err
	}
	if ret != 1 {
		return nil, fmt.Errorf("leitura %#x recusada (código %d)", id, ret)
	}
	data := r.rest()
	// firmware novo às vezes manda um 0 a mais no fim
	if len(data) == int(n)+1 && data[len(data)-1] == 0 {
		data = data[:n]
	}
	if len(data) != int(n) {
		return nil, fmt.Errorf("leitura %#x: veio %d bytes, esperava %d", id, len(data), n)
	}
	return data, nil
}

func (d *Device) writeSFR(id uint32, data []byte) error {
	r, err := d.execute(cmdWrVirtSFR, append(u32(id), data...))
	if err != nil {
		return err
	}
	if ret, err := r.u32(); err != nil || ret != 1 {
		return fmt.Errorf("escrita %#x recusada", id)
	}
	return nil
}

func (d *Device) setLocalTime(t time.Time) error {
	_, err := d.execute(cmdSetTime, []byte{byte(t.Day()), byte(t.Month()), byte(t.Year() - 2000), 0,
		byte(t.Second()), byte(t.Minute()), byte(t.Hour()), 0})
	return err
}

func (d *Device) fwVersion() (maj, min int, date string, err error) {
	r, err := d.execute(cmdGetVersion, nil)
	if err != nil {
		return
	}
	// boot: minor, major, data; depois o firmware (target)
	if _, err = r.u16(); err != nil {
		return
	}
	if _, err = r.u16(); err != nil {
		return
	}
	if _, err = r.str(); err != nil {
		return
	}
	mi, err := r.u16()
	if err != nil {
		return
	}
	ma, err := r.u16()
	if err != nil {
		return
	}
	date, err = r.str()
	return int(ma), int(mi), strings.TrimRight(date, "\x00"), err
}

// Configuration é o arquivo de configuração do aparelho (texto CP1251).
func (d *Device) Configuration() (string, error) {
	b, err := d.readString(vsConfiguration)
	if err != nil {
		return "", err
	}
	return cp1251(b), nil
}

// DataBuf traz os registros novos desde a última leitura.
func (d *Device) DataBuf() ([]Record, error) {
	b, err := d.readString(vsDataBuf)
	if err != nil {
		return nil, err
	}
	return decodeDataBuf(b, d.base), nil
}

// Spectrum é o espectro atual (desde o último zerar).
func (d *Device) Spectrum() (Spectrum, error) {
	b, err := d.readString(vsSpectrum)
	if err != nil {
		return Spectrum{}, err
	}
	return decodeSpectrum(b, d.specFmt)
}

// SpectrumAccum é o espectro acumulado.
func (d *Device) SpectrumAccum() (Spectrum, error) {
	b, err := d.readString(vsSpecAccum)
	if err != nil {
		return Spectrum{}, err
	}
	return decodeSpectrum(b, d.specFmt)
}

// AlarmLimits são os alarmes do próprio aparelho.
type AlarmLimits struct {
	CountRate1, CountRate2 float64 // cps
	DoseRate1, DoseRate2   float64 // µSv/h
	Dose1, Dose2           float64 // µSv
	ShowSv                 bool    // o aparelho mostra em Sv (senão R)
	ShowCPM                bool
}

// Alarms lê os limites de alarme do aparelho.
func (d *Device) Alarms() (AlarmLimits, error) {
	ids := []uint32{sfrCRLev1, sfrCRLev2, sfrDRLev1, sfrDRLev2, sfrDSLev1, sfrDSLev2, sfrDSUnits, sfrCRUnits}
	args := u32(uint32(len(ids)))
	for _, id := range ids {
		args = append(args, u32(id)...)
	}
	r, err := d.execute(cmdRdVirtSFRBat, args)
	if err != nil {
		return AlarmLimits{}, err
	}
	ok, err := r.u32()
	if err != nil {
		return AlarmLimits{}, err
	}
	if ok != 1<<len(ids)-1 {
		return AlarmLimits{}, fmt.Errorf("alarmes: registros recusados (%b)", ok)
	}
	v := make([]uint32, len(ids))
	for i := range v {
		if v[i], err = r.u32(); err != nil {
			return AlarmLimits{}, err
		}
	}
	return AlarmLimits{
		CountRate1: float64(v[0]) / 10, CountRate2: float64(v[1]) / 10,
		DoseRate1: float64(v[2]) / 100, DoseRate2: float64(v[3]) / 100, // µR/h → µSv/h
		Dose1: float64(v[4]) / 100, Dose2: float64(v[5]) / 100,
		ShowSv: v[6]>>24 != 0, ShowCPM: v[7]>>24 != 0, // booleano no 4º byte
	}, nil
}

// ---------- registros do DATA_BUF ----------

// Record é um registro do aparelho (um dos tipos abaixo).
type Record interface{ When() time.Time }

// RealTime: taxa atual (a que o aparelho mostra na tela).
type RealTime struct {
	T                 time.Time
	CPS, CPSErr       float64 // CPSErr em %
	DoseRate, DoseErr float64 // µSv/h e %
	Flags, RTFlags    uint16
}

// FlagOld é o bit dos registros que o aparelho guardou na memória enquanto
// ninguém estava conectado e só agora está mandando ("dado antigo").
const FlagOld = 0x8000

// DoseRateDB: ponto do histórico do aparelho (o que ele guarda na memória).
type DoseRateDB struct {
	T        time.Time
	Count    uint32
	CPS      float64
	DoseRate float64 // µSv/h
	DoseErr  float64 // %
	Flags    uint16
}

// Old diz se o ponto veio da memória do aparelho (gravado sem o app).
func (r DoseRateDB) Old() bool { return r.Flags&FlagOld != 0 }

// Rare: dose acumulada, temperatura e bateria.
type Rare struct {
	T        time.Time
	Duration uint32  // s de acumulação da dose
	Dose     float64 // µSv
	TempC    float64
	Battery  float64 // %
	Flags    uint16
}

// Old diz se o registro veio da memória do aparelho (gravado sem o app).
func (r Rare) Old() bool { return r.Flags&FlagOld != 0 }

// Event: algo que aconteceu no aparelho (alarme, carga, desligou…).
type Event struct {
	T     time.Time
	ID    int
	Param int
	Flags uint16
}

// Old diz se o evento veio da memória do aparelho (gravado sem o app).
func (r Event) Old() bool { return r.Flags&FlagOld != 0 }

// rateEvent: alarmes e "fora da escala" de taxa e de contagem trazem a taxa
// (f32) e o erro (u16) depois do evento.
func rateEvent(id int) bool { return (id >= 9 && id <= 14) || (id >= 20 && id <= 22) }

// RawSamples são as amostras cruas do aparelho (cps de cada janela curta,
// ~0,5 s), chegam uma vez por segundo. Contagens = CPS × Period.
type RawSamples struct {
	T        time.Time
	Period   time.Duration
	CPS      []float64
	DoseRate []float64 // µSv/h
}

// Counts são as contagens somadas das amostras.
func (r RawSamples) Counts() float64 {
	var c float64
	for _, v := range r.CPS {
		c += v * r.Period.Seconds()
	}
	return c
}

func (r RawSamples) When() time.Time { return r.T }
func (r RealTime) When() time.Time   { return r.T }
func (r DoseRateDB) When() time.Time { return r.T }
func (r Rare) When() time.Time       { return r.T }
func (r Event) When() time.Time      { return r.T }

// EventName é o nome do evento em português.
func EventName(id int) string {
	names := []string{"desligou", "ligou", "desligou por bateria fraca", "configuração alterada", "dose zerada",
		"evento do usuário", "alarme de bateria vazia", "começou a carregar", "parou de carregar",
		"alarme 1 de taxa de dose", "alarme 2 de taxa de dose", "taxa de dose fora da escala",
		"alarme 1 de dose", "alarme 2 de dose", "dose fora da escala", "temperatura baixa demais",
		"temperatura alta demais", "mensagem de texto", "cópia da memória", "espectro zerado",
		"alarme 1 de contagem", "alarme 2 de contagem", "contagem fora da escala"}
	if id >= 0 && id < len(names) {
		return names[id]
	}
	return fmt.Sprintf("evento %d", id)
}

// resync: o firmware 4.15 manda um byte a mais depois do RealTimeData (que a
// biblioteca aberta não conhece). Se o próximo registro não começa com a
// sequência esperada, mas começa 1–3 bytes adiante, pula esses bytes.
func resync(b []byte, p, next int) int {
	if next < 0 || p >= len(b) || int(b[p]) == next {
		return p
	}
	for k := 1; k <= 3 && p+k < len(b); k++ {
		if int(b[p+k]) == next && p+k+2 < len(b) && b[p+k+1] <= 1 {
			return p + k
		}
	}
	return p
}

func decodeDataBuf(b []byte, base time.Time) []Record {
	r := &buf{b: b}
	var out []Record
	next := -1
	for r.left() >= 7 {
		r.p = resync(r.b, r.p, next)
		if r.left() < 7 {
			break
		}
		seq, _ := r.u8()
		eid, _ := r.u8()
		gid, _ := r.u8()
		off, _ := r.i32()
		t := base.Add(time.Duration(off) * 10 * time.Millisecond)
		if next >= 0 && int(seq) != next {
			break // pulo na sequência: o resto não é confiável
		}
		next = (int(seq) + 1) % 256
		var err error
		switch {
		case eid == 0 && gid == 0:
			var rt RealTime
			var cr, dr float32
			var ce, de uint16
			cr, err = r.f32()
			dr, _ = r.f32()
			ce, _ = r.u16()
			de, _ = r.u16()
			rt.Flags, _ = r.u16()
			var rf uint8
			rf, err = r.u8()
			if err == nil {
				rt.T, rt.CPS, rt.DoseRate = t, float64(cr), float64(dr)*DoseScale
				rt.CPSErr, rt.DoseErr, rt.RTFlags = float64(ce)/10, float64(de)/10, uint16(rf)
				out = append(out, rt)
			}
		case eid == 0 && gid == 1: // dados crus
			err = r.skip(8)
		case eid == 0 && gid == 2:
			var p DoseRateDB
			if p, err = readDoseRateDB(r, t); err == nil {
				out = append(out, p)
			}
		case eid == 0 && gid == 3:
			var p Rare
			if p, err = readRare(r, t); err == nil {
				out = append(out, p)
			}
		case eid == 0 && gid == 4: // contador de dose: u32 + flags
			err = r.skip(6)
		case eid == 0 && gid == 5:
			err = r.skip(16)
		case eid == 0 && gid == 6:
			err = r.skip(6)
		case eid == 0 && gid == 7:
			var e Event
			var id, p uint8
			id, _ = r.u8()
			p, _ = r.u8()
			e.Flags, err = r.u16()
			if err == nil && rateEvent(int(id)) {
				err = r.skip(6) // taxa (f32) e erro (u16) do alarme
			}
			if err == nil {
				e.T, e.ID, e.Param = t, int(id), int(p)
				out = append(out, e)
			}
		case eid == 0 && (gid == 8 || gid == 9):
			err = r.skip(6)
		case eid == 1 && gid == 1:
			// amostras cruas: n × (cps, dose) a cada "unidade" × 10 ms
			var n uint16
			var unit uint32
			n, _ = r.u16()
			unit, err = r.u32()
			rs := RawSamples{T: t, Period: time.Duration(unit) * 10 * time.Millisecond}
			for i := 0; i < int(n) && err == nil; i++ {
				var cr, dr float32
				cr, _ = r.f32()
				dr, err = r.f32()
				rs.CPS = append(rs.CPS, float64(cr))
				rs.DoseRate = append(rs.DoseRate, float64(dr)*DoseScale)
			}
			if err == nil && rs.Period > 0 && rs.Period < time.Minute {
				out = append(out, rs)
			}
		case eid == 1 && (gid == 2 || gid == 3):
			// lote: n registros, um a cada "unidade" × 10 ms, do mais antigo
			// para o mais novo. É assim que o aparelho manda o que guardou na
			// memória enquanto ninguém estava conectado.
			var n uint16
			var unit uint32
			n, _ = r.u16()
			unit, err = r.u32()
			size := map[uint8]int{2: 16, 3: 14}[gid]
			if err == nil && r.left() < int(n)*size {
				return out // lote cortado: não dá para confiar no resto
			}
			step := time.Duration(unit) * 10 * time.Millisecond
			for i := 0; i < int(n) && err == nil; i++ {
				ti := t.Add(time.Duration(i) * step)
				if gid == 2 {
					var p DoseRateDB
					if p, err = readDoseRateDB(r, ti); err == nil {
						out = append(out, p)
					}
				} else {
					var p Rare
					if p, err = readRare(r, ti); err == nil {
						out = append(out, p)
					}
				}
			}
		case eid == 1 && gid == 4:
			var n uint16
			n, _ = r.u16()
			_, err = r.u32()
			if err == nil {
				err = r.skip(int(n) * 6)
			}
		default:
			return out // tipo desconhecido: não dá para saber o tamanho
		}
		if err != nil {
			break
		}
	}
	return out
}

func readDoseRateDB(r *buf, t time.Time) (DoseRateDB, error) {
	var p DoseRateDB
	var cr, dr float32
	var de uint16
	p.Count, _ = r.u32()
	cr, _ = r.f32()
	dr, _ = r.f32()
	de, _ = r.u16()
	var err error
	p.Flags, err = r.u16()
	p.T, p.CPS, p.DoseRate, p.DoseErr = t, float64(cr), float64(dr)*DoseScale, float64(de)/10
	return p, err
}

func readRare(r *buf, t time.Time) (Rare, error) {
	var p Rare
	var dose float32
	var temp, chg uint16
	p.Duration, _ = r.u32()
	dose, _ = r.f32()
	temp, _ = r.u16()
	chg, _ = r.u16()
	var err error
	p.Flags, err = r.u16()
	p.T, p.Dose = t, float64(dose)*DoseScale
	p.TempC, p.Battery = (float64(temp)-2000)/100, float64(chg)/100
	return p, err
}

// ---------- espectro ----------

// Spectrum é o espectro do aparelho: contagens por canal e a calibração de
// energia E(keV) = A0 + A1·c + A2·c².
type Spectrum struct {
	Duration   time.Duration
	A0, A1, A2 float64
	Counts     []uint32
}

// Energy é a energia (keV) do canal c.
func (s Spectrum) Energy(c float64) float64 { return s.A0 + s.A1*c + s.A2*c*c }

func decodeSpectrum(b []byte, format int) (Spectrum, error) {
	r := &buf{b: b}
	ts, err := r.u32()
	if err != nil {
		return Spectrum{}, err
	}
	a0, _ := r.f32()
	a1, _ := r.f32()
	a2, err := r.f32()
	if err != nil {
		return Spectrum{}, err
	}
	s := Spectrum{Duration: time.Duration(ts) * time.Second, A0: float64(a0), A1: float64(a1), A2: float64(a2)}
	switch format {
	case 0:
		for r.left() >= 4 {
			v, _ := r.u32()
			s.Counts = append(s.Counts, v)
		}
	case 1:
		var last int64
		for r.left() >= 2 {
			h, _ := r.u16()
			cnt, vlen := int(h>>4)&0x0fff, h&0x0f
			for i := 0; i < cnt; i++ {
				var v int64
				switch vlen {
				case 0:
					v = 0
				case 1:
					x, e := r.u8()
					v, err = int64(x), e
				case 2:
					x, e := r.u8()
					v, err = last+int64(int8(x)), e
				case 3:
					x, e := r.u16()
					v, err = last+int64(int16(x)), e
				case 4:
					a, _ := r.u8()
					b2, _ := r.u8()
					c, e := r.u8()
					v, err = last+int64(int32(uint32(a)|uint32(b2)<<8|uint32(int8(c))<<16)), e
				case 5:
					x, e := r.i32()
					v, err = last+int64(x), e
				default:
					return s, fmt.Errorf("espectro: formato de valor %d desconhecido", vlen)
				}
				if err != nil {
					return s, err
				}
				last = v
				if v < 0 {
					v = 0
				}
				s.Counts = append(s.Counts, uint32(v))
			}
		}
	default:
		return s, fmt.Errorf("espectro: formato %d desconhecido", format)
	}
	return s, nil
}

// ---------- leitura de bytes ----------

type buf struct {
	b []byte
	p int
}

var errShort = errors.New("resposta curta")

func (r *buf) left() int    { return len(r.b) - r.p }
func (r *buf) rest() []byte { return r.b[r.p:] }
func (r *buf) need(n int) error {
	if r.left() < n {
		return errShort
	}
	return nil
}
func (r *buf) skip(n int) error {
	if err := r.need(n); err != nil {
		return err
	}
	r.p += n
	return nil
}
func (r *buf) u8() (uint8, error) {
	if err := r.need(1); err != nil {
		return 0, err
	}
	r.p++
	return r.b[r.p-1], nil
}
func (r *buf) u16() (uint16, error) {
	if err := r.need(2); err != nil {
		return 0, err
	}
	r.p += 2
	return binary.LittleEndian.Uint16(r.b[r.p-2:]), nil
}
func (r *buf) u32() (uint32, error) {
	if err := r.need(4); err != nil {
		return 0, err
	}
	r.p += 4
	return binary.LittleEndian.Uint32(r.b[r.p-4:]), nil
}
func (r *buf) i32() (int32, error) { v, err := r.u32(); return int32(v), err }
func (r *buf) f32() (float32, error) {
	v, err := r.u32()
	return math.Float32frombits(v), err
}
func (r *buf) str() (string, error) {
	n, err := r.u8()
	if err != nil {
		return "", err
	}
	if err := r.need(int(n)); err != nil {
		return "", err
	}
	r.p += int(n)
	return string(r.b[r.p-int(n) : r.p]), nil
}

// cp1251 converte o texto do aparelho (só o que importa: ASCII passa, o
// cirílico vira o caractere certo).
func cp1251(b []byte) string {
	var sb strings.Builder
	for _, c := range b {
		switch {
		case c < 0x80:
			sb.WriteByte(c)
		case c >= 0xC0:
			sb.WriteRune(rune(0x410 + int(c) - 0xC0))
		case c == 0xA8:
			sb.WriteRune('Ё')
		case c == 0xB8:
			sb.WriteRune('ё')
		default:
			sb.WriteRune('?')
		}
	}
	return sb.String()
}
