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

// DefaultCalibration é a calibração de fábrica do GMC-800
// (1538 CPM = 10 µSv/h, ou seja, 0,0065 µSv/h por CPM).
func DefaultCalibration() Calibration {
	return Calibration{
		Points: []CalPoint{
			{1538, 10}, {15380, 100}, {30760, 200},
			{76900, 500}, {153800, 1000}, {307600, 2000},
		},
		Source: "padrão",
	}
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
