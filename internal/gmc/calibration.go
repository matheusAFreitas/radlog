package gmc

import (
	"encoding/binary"
	"fmt"
	"math"
	"sort"
)

// CalPoint é um ponto de calibração: tantos CPM equivalem a tantos µSv/h.
type CalPoint struct {
	CPM float64 `json:"cpm"`
	USv float64 `json:"usv"`
}

// Calibration converte CPM em µSv/h interpolando entre os pontos,
// como o contador faz na tela.
type Calibration struct {
	Points []CalPoint `json:"points"`
	Source string     `json:"source"` // "contador" ou "padrão"
}

// DefaultCalibration é a calibração de fábrica do GMC-800, a que ele volta
// ao ser formatado (986 CPM = 10 µSv/h no 1º ponto, ou seja, ≈ 0,0101 µSv/h
// por CPM no fundo). Os pontos de cima têm menos CPM por µSv/h: compensam as
// contagens que o tubo perde em taxas altas (tempo morto).
func DefaultCalibration() Calibration {
	return Calibration{
		Points: []CalPoint{
			{986, 10}, {8546, 100}, {16408, 200},
			{37970, 500}, {76010, 1000}, {139671, 2000},
		},
		Source: "padrão",
	}
}

// Linear devolve a calibração só com a sensibilidade do 1º ponto (reta da
// origem até ele, continuada para cima). Serve para CPM já corrigido do
// tempo morto: a contagem verdadeira cresce em linha reta com a dose, então
// a curva do contador (que já compensa as perdas) contaria a correção duas
// vezes.
func (c Calibration) Linear() Calibration {
	if len(c.Points) == 0 {
		return c
	}
	return Calibration{Points: []CalPoint{c.Points[0]}, Source: c.Source}
}

// CompensatesDeadTime diz se a curva já compensa perdas em taxas altas
// (sensibilidade do último ponto mais de 5% abaixo da do primeiro).
func (c Calibration) CompensatesDeadTime() bool {
	if len(c.Points) < 2 {
		return false
	}
	first := c.Points[0].CPM / c.Points[0].USv
	last := c.Points[len(c.Points)-1].CPM / c.Points[len(c.Points)-1].USv
	return last < first*0.95
}

// Offsets dos 6 pontos de calibração na configuração do GMC-800.
const (
	cfgCalCPM = 73 // 6 x uint32 big-endian
	cfgCalUSv = 97 // 6 x float32 big-endian
)

// ParseCalibration800 extrai a calibração da resposta de <GETCFG>> do GMC-800.
func ParseCalibration800(cfg []byte) (Calibration, error) {
	if len(cfg) < cfgCalUSv+24 {
		return Calibration{}, fmt.Errorf("configuração curta demais (%d bytes)", len(cfg))
	}
	var pts []CalPoint
	for i := 0; i < 6; i++ {
		cpm := binary.BigEndian.Uint32(cfg[cfgCalCPM+4*i:])
		usv := math.Float32frombits(binary.BigEndian.Uint32(cfg[cfgCalUSv+4*i:]))
		if cpm == 0 || cpm == math.MaxUint32 {
			continue
		}
		u := float64(usv)
		if math.IsNaN(u) || math.IsInf(u, 0) || u <= 0 {
			continue
		}
		pts = append(pts, CalPoint{CPM: float64(cpm), USv: u})
	}
	if len(pts) == 0 {
		return Calibration{}, fmt.Errorf("nenhum ponto de calibração válido")
	}
	sort.Slice(pts, func(i, j int) bool { return pts[i].CPM < pts[j].CPM })
	return Calibration{Points: pts, Source: "contador"}, nil
}

// USv converte CPM em µSv/h. Entre pontos, interpola em linha reta; acima do
// último, continua a última reta.
func (c Calibration) USv(cpm float64) float64 {
	if cpm <= 0 || len(c.Points) == 0 {
		return 0
	}
	prev := CalPoint{0, 0}
	for _, p := range c.Points {
		if p.CPM <= prev.CPM {
			continue
		}
		if cpm <= p.CPM {
			return interp(prev, p, cpm)
		}
		prev = p
	}
	// Acima do último ponto: usa a reta do último trecho.
	n := len(c.Points)
	if n == 1 {
		return interp(CalPoint{0, 0}, c.Points[0], cpm)
	}
	return interp(c.Points[n-2], c.Points[n-1], cpm)
}

func interp(a, b CalPoint, x float64) float64 {
	if b.CPM == a.CPM {
		return b.USv
	}
	return a.USv + (x-a.CPM)*(b.USv-a.USv)/(b.CPM-a.CPM)
}
