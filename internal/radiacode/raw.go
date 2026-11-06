package radiacode

import (
	"encoding/binary"
	"fmt"
	"math"
	"time"
)

// ---------- dados brutos: tudo o que o aparelho manda, para análise ----------

// RawRec é um registro do DATA_BUF como veio, com o que se sabe dele.
type RawRec struct {
	T      time.Time
	Seq    int
	EID    int
	GID    int
	Name   string
	Fields map[string]float64
	Data   []byte // o registro inteiro (cabeçalho + conteúdo)
}

type recFmt struct {
	name   string
	fields []string // nome:tipo (f=f32, I=u32, H=u16, B=u8, b=i8)
}

// formatos conhecidos (cdump/radiacode decoders/databuf.py)
var recFormats = map[[2]int]recFmt{
	{0, 0}: {"RealTimeData", []string{"count_rate:f", "dose_rate:f", "count_rate_err:H", "dose_rate_err:H", "flags:H", "rt_flags:B"}},
	{0, 1}: {"RawData", []string{"count_rate:f", "dose_rate:f"}},
	{0, 2}: {"DoseRateDB", []string{"count:I", "count_rate:f", "dose_rate:f", "dose_rate_err:H", "flags:H"}},
	{0, 3}: {"RareData", []string{"duration:I", "dose:f", "temperature:H", "charge_level:H", "flags:H"}},
	{0, 4}: {"UserData", []string{"count:I", "count_rate:f", "dose_rate:f", "dose_rate_err:H", "flags:H"}},
	{0, 5}: {"SheduleData", []string{"count:I", "count_rate:f", "dose_rate:f", "dose_rate_err:H", "flags:H"}},
	{0, 6}: {"AccelData", []string{"acc_x:H", "acc_y:H", "acc_z:H"}},
	{0, 7}: {"Event", []string{"event:B", "event_param1:B", "flags:H"}},
	{0, 8}: {"RawCountRate", []string{"count_rate:f", "flags:H"}},
	{0, 9}: {"RawDoseRate", []string{"dose_rate:f", "flags:H"}},
}

// amostras em bloco (eid 1): tamanho de cada amostra
var blockSizes = map[int]int{1: 8, 2: 16, 3: 14}

func fieldSize(t byte) int {
	switch t {
	case 'f', 'I':
		return 4
	case 'H':
		return 2
	default:
		return 1
	}
}

// SplitDataBuf separa a resposta do DATA_BUF em registros, sem perder nada:
// os de tipo desconhecido vão com o resto da resposta (não dá para saber o
// tamanho deles).
func SplitDataBuf(b []byte, base time.Time) []RawRec {
	var out []RawRec
	p := 0
	for len(b)-p >= 7 {
		start := p
		seq, eid, gid := int(b[p]), int(b[p+1]), int(b[p+2])
		off := int32(binary.LittleEndian.Uint32(b[p+3:]))
		p += 7
		r := RawRec{T: base.Add(time.Duration(off) * 10 * time.Millisecond), Seq: seq, EID: eid, GID: gid, Fields: map[string]float64{}}
		if f, ok := recFormats[[2]int{eid, gid}]; ok && eid == 0 {
			r.Name = f.name
			size := 0
			for _, fd := range f.fields {
				size += fieldSize(fd[len(fd)-1])
			}
			if len(b)-p < size {
				r.Name += " (cortado)"
				r.Data = b[start:]
				return append(out, r)
			}
			q := p
			for _, fd := range f.fields {
				name, t := fd[:len(fd)-2], fd[len(fd)-1]
				var v float64
				switch t {
				case 'f':
					v = float64(math.Float32frombits(binary.LittleEndian.Uint32(b[q:])))
				case 'I':
					v = float64(binary.LittleEndian.Uint32(b[q:]))
				case 'H':
					v = float64(binary.LittleEndian.Uint16(b[q:]))
				default:
					v = float64(b[q])
				}
				r.Fields[name] = v
				q += fieldSize(t)
			}
			p += size
		} else if sz, ok := blockSizes[gid]; ok && eid == 1 {
			r.Name = fmt.Sprintf("Bloco%d", gid)
			if len(b)-p < 6 {
				r.Data = b[start:]
				return append(out, r)
			}
			n := int(binary.LittleEndian.Uint16(b[p:]))
			r.Fields["samples"] = float64(n)
			r.Fields["sample_ms"] = float64(binary.LittleEndian.Uint32(b[p+2:]))
			p += 6 + n*sz
			if p > len(b) {
				p = len(b)
			}
		} else {
			r.Name = "Desconhecido"
			r.Data = b[start:]
			return append(out, r)
		}
		r.Data = b[start:p]
		out = append(out, r)
	}
	return out
}

// DataBufRaw é o DataBuf com a resposta crua junto.
func (d *Device) DataBufRaw() ([]byte, []Record, error) {
	b, err := d.readString(vsDataBuf)
	if err != nil {
		return nil, nil, err
	}
	return b, decodeDataBuf(b, d.base), nil
}

// Base é o instante zero dos registros do DATA_BUF.
func (d *Device) Base() time.Time { return d.base }

// ReadVS lê uma "string virtual" qualquer, crua.
func (d *Device) ReadVS(id uint32) ([]byte, error) { return d.readString(id) }

// ReadSFR lê um registrador virtual, cru (u32).
func (d *Device) ReadSFR(id uint32) (uint32, error) {
	r, err := d.execute(cmdRdVirtSFRBat, append(u32(1), u32(id)...))
	if err != nil {
		return 0, err
	}
	ok, err := r.u32()
	if err != nil {
		return 0, err
	}
	if ok&1 == 0 {
		return 0, fmt.Errorf("registrador %#x recusado", id)
	}
	return r.u32()
}

// Named é um id com o nome que a biblioteca aberta dá a ele.
type Named struct {
	ID   uint32
	Name string
}

// KnownVS são as strings virtuais conhecidas (algumas sem formato conhecido).
var KnownVS = []Named{
	{2, "CONFIGURATION"}, {3, "FW_DESCRIPTOR"}, {8, "SERIAL_NUMBER"}, {0xD, "UNKNOWN_13"},
	{0xF, "TEXT_MESSAGE"}, {0xE0, "MEM_SNAPSHOT"}, {0xF0, "UNKNOWN_240"}, {0x101, "SFR_FILE"},
	{0x200, "SPECTRUM"}, {0x202, "ENERGY_CALIB"}, {0x205, "SPEC_ACCUM"}, {0x206, "SPEC_DIFF"}, {0x207, "SPEC_RESET"},
}

// KnownSFR são os registradores virtuais conhecidos (só leitura aqui).
var KnownSFR = []Named{
	{0x0500, "DEVICE_CTRL"}, {0x0502, "DEVICE_LANG"}, {0x0503, "DEVICE_ON"}, {0x0504, "DEVICE_TIME"},
	{0x0510, "DISP_CTRL"}, {0x0511, "DISP_BRT"}, {0x0512, "DISP_CONTR"}, {0x0513, "DISP_OFF_TIME"},
	{0x0514, "DISP_ON"}, {0x0515, "DISP_DIR"}, {0x0516, "DISP_BACKLT_ON"}, {0x0520, "SOUND_CTRL"},
	{0x0521, "SOUND_VOL"}, {0x0522, "SOUND_ON"}, {0x0523, "SOUND_BUTTON"}, {0x0530, "VIBRO_CTRL"},
	{0x0531, "VIBRO_ON"}, {0x0540, "LEDS_CTRL"}, {0x0541, "LED0_BRT"}, {0x0542, "LED1_BRT"},
	{0x0543, "LED2_BRT"}, {0x0544, "LED3_BRT"}, {0x0545, "LEDS_ON"}, {0x05E0, "ALARM_MODE"},
	{0x0600, "MS_CTRL"}, {0x0601, "MS_MODE"}, {0x0602, "MS_SUB_MODE"}, {0x0603, "MS_RUN"},
	{0x0700, "BLE_TX_PWR"}, {0x8000, "DR_LEV1_uR_h"}, {0x8001, "DR_LEV2_uR_h"}, {0x8002, "DS_LEV1_100uR"},
	{0x8003, "DS_LEV2_100uR"}, {0x8004, "DS_UNITS"}, {0x8005, "CPS_FILTER"}, {0x8006, "RAW_FILTER"},
	{0x8008, "CR_LEV1_cp10s"}, {0x8009, "CR_LEV2_cp10s"}, {0x800C, "USE_nSv_h"},
	{0x8010, "CHN_TO_keV_A0"}, {0x8011, "CHN_TO_keV_A1"}, {0x8012, "CHN_TO_keV_A2"}, {0x8013, "CR_UNITS"},
	{0x8014, "DS_LEV1_uR"}, {0x8015, "DS_LEV2_uR"}, {0x8020, "CPS"}, {0x8021, "DR_uR_h"}, {0x8022, "DS_uR"},
	{0x8024, "TEMP_degC"}, {0x8025, "ACC_X"}, {0x8026, "ACC_Y"}, {0x8027, "ACC_Z"}, {0x8028, "OPT"},
	{0x8033, "RAW_TEMP_degC"}, {0x8034, "TEMP_UP_degC"}, {0x8035, "TEMP_DN_degC"}, {0xC000, "VBIAS_mV"},
	{0xC001, "COMP_LEV"}, {0xC002, "CALIB_MODE"}, {0xC004, "DPOT_RDAC"}, {0xC005, "DPOT_RDAC_EEPROM"},
	{0xC006, "DPOT_TOLER"}, {0xFFFF0000, "SYS_MCU_ID0"}, {0xFFFF0001, "SYS_MCU_ID1"}, {0xFFFF0002, "SYS_MCU_ID2"},
	{0xFFFF0005, "SYS_DEVICE_ID"}, {0xFFFF0006, "SYS_SIGNATURE"}, {0xFFFF0007, "SYS_RX_SIZE"},
	{0xFFFF0008, "SYS_TX_SIZE"}, {0xFFFF0009, "SYS_BOOT_VERSION"}, {0xFFFF000A, "SYS_TARGET_VERSION"},
	{0xFFFF000B, "SYS_STATUS"}, {0xFFFF000C, "SYS_MCU_VREF"}, {0xFFFF000D, "SYS_MCU_TEMP"}, {0xFFFF0010, "SYS_FW_VER_BT"},
}
