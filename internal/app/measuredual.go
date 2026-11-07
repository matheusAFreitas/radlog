package app

import (
	"fmt"
	"math"
	"time"
)

// ---------- medir um objeto com os dois aparelhos ----------
//
// Dois jeitos, além de um aparelho só:
//   - "ambos": o RadiaCode e o GMC perto do objeto, medindo ao mesmo tempo,
//     cada um contra o seu fundo. O GMC vê beta e gama; o cristal do RadiaCode
//     quase só gama. Comparando os dois dá para ter uma ideia do que o objeto
//     emite.
//   - "voce": um aparelho no objeto e o outro com a pessoa, para ver quanto
//     ela recebeu enquanto mexia no objeto.
//
// O aparelho principal (a.measure.Device) comanda as fases e o tempo; o
// segundo (a.measure.Other) só acumula nas mesmas fases.

// MeasureSide é o segundo aparelho da medição.
type MeasureSide struct {
	Device  string         `json:"device"`
	Role    string         `json:"role"` // "objeto" (os dois perto) ou "voce"
	Counts  int            `json:"counts"`
	Elapsed int            `json:"elapsed"`
	Bg      *MeasurePhase  `json:"bg"`
	Obj     *MeasurePhase  `json:"obj"`
	Result  *MeasureResult `json:"result"`
	Live    *MeasureResult `json:"live"`
	// com você: o que a pessoa recebeu na fase do objeto
	Dose   float64 `json:"dose"`   // µSv
	AvgUSv float64 `json:"avgUSv"` // µSv/h médio
	MaxUSv float64 `json:"maxUSv"`
	BgUSv  float64 `json:"bgUSv"`  // fundo desse aparelho em casa (0 = não sabe)
	Excess float64 `json:"excess"` // µSv acima do fundo
	NowUSv float64 `json:"nowUSv"` // última leitura (ao vivo)
	// os dois perto: o que a comparação sugere (beta × gama)
	Hint string `json:"hint,omitempty"`
	// um registro por segundo (só na medição salva, para o gráfico)
	Samples []MeasureSample `json:"samples,omitempty"`

	usvSum float64
	usvN   int
}

// clone copia o lado (para mandar à tela sem dividir o ponteiro).
func (s *MeasureSide) clone() *MeasureSide {
	if s == nil {
		return nil
	}
	c := *s
	return &c
}

// snapLocked é a medição para a tela (cópia; precisa de a.mu).
func (a *App) measureSnapLocked() MeasureState {
	st := a.measure
	st.Other = st.Other.clone()
	return st
}

// otherDev é o outro aparelho.
func otherDev(d string) string {
	if devName(d) == devRC {
		return "gmc"
	}
	return devRC
}

// sidePhaseLocked é a fase atual do segundo aparelho até agora.
func (a *App) sidePhaseLocked(s *MeasureSide) MeasurePhase {
	ph := phaseResult(s.Counts, max(s.Elapsed, 1), a.convCalLocked().USv)
	if s.Device == devRC && s.usvN > 0 {
		ph.USv = s.usvSum / float64(s.usvN)
	}
	return ph
}

// feedSideLocked recebe um segundo do segundo aparelho (precisa de a.mu).
// Devolve false se a leitura não é dele.
func (a *App) feedSideLocked(m *MeasureState, f measFeed) bool {
	s := m.Other
	if s == nil || f.dev != s.Device || f.demo {
		return false
	}
	if m.Phase != "fundo" && m.Phase != "objeto" {
		return true
	}
	// leitura de agora, para a tela e o gráfico (em qualquer fase)
	if s.Device == devRC {
		s.NowUSv = f.usv
	} else {
		s.NowUSv = a.convCalLocked().USv(f.cpm)
	}
	a.measSamples2 = append(a.measSamples2, MeasureSample{T: time.Now().UnixMilli() - measT0(m), Phase: m.Phase, CPM: f.cpm, CPS: uint32(max(f.counts, 0)), USv: s.NowUSv})
	if s.Role == "voce" && m.Phase != "objeto" {
		return true // com você só conta enquanto o objeto é medido
	}
	if s.Role == "objeto" && m.Phase == "fundo" && s.Bg != nil && s.Bg.Source == "casa" {
		return true
	}
	s.Counts += f.counts
	s.Elapsed++
	usv := f.usv
	if s.Device != devRC {
		usv = a.convCalLocked().USv(f.cpm)
	}
	s.usvSum += usv
	s.usvN++
	if m.Phase == "objeto" {
		s.MaxUSv = math.Max(s.MaxUSv, usv)
		if s.Role == "objeto" && s.Bg != nil {
			part := a.sidePhaseLocked(s)
			live := compareMeasure(*s.Bg, part, a.convCalLocked().USv)
			s.Live = &live
		}
		if s.Role == "voce" {
			s.fillDose(a.sidePhaseLocked(s))
		}
	}
	return true
}

// fillDose calcula o que a pessoa recebeu (fase do objeto até agora).
func (s *MeasureSide) fillDose(ph MeasurePhase) {
	s.AvgUSv = ph.USv
	s.Dose = ph.USv * float64(ph.Seconds) / 3600
	if s.BgUSv > 0 {
		s.Excess = (ph.USv - s.BgUSv) * float64(ph.Seconds) / 3600
	}
}

// endSidePhaseLocked fecha a fase do segundo aparelho junto com a do
// principal (precisa de a.mu).
func (a *App) endSidePhaseLocked(m *MeasureState, phase string, start, end int64) {
	s := m.Other
	if s == nil {
		return
	}
	defer func() { s.Counts, s.Elapsed, s.usvSum, s.usvN = 0, 0, 0, 0 }()
	if s.Elapsed == 0 {
		return
	}
	ph := a.sidePhaseLocked(s)
	ph.Start, ph.End = start, end
	switch {
	case phase == "fundo" && s.Role == "objeto":
		s.Bg = &ph
	case phase == "objeto" && s.Role == "objeto" && s.Bg != nil:
		s.Obj = &ph
		r := compareMeasure(*s.Bg, ph, a.convCalLocked().USv)
		if r.Level != "sim" {
			r.Limit = math.Max(r.NetCPM, 0) + 2*r.NetErr
		}
		s.Result, s.Live = &r, nil
	case phase == "objeto" && s.Role == "voce":
		s.Obj = &ph
		s.fillDose(ph)
	}
}

// startSideObjLocked limpa o segundo aparelho para medir o objeto de novo.
func startSideObjLocked(m *MeasureState) {
	if s := m.Other; s != nil {
		s.Counts, s.Elapsed, s.usvSum, s.usvN = 0, 0, 0, 0
		s.Obj, s.Result, s.Live, s.Hint = nil, nil, nil, ""
		s.Dose, s.AvgUSv, s.MaxUSv, s.Excess = 0, 0, 0, 0
	}
}

// sideDecide é a decisão automática do segundo aparelho (os dois perto).
func sideDecide(m *MeasureState) string {
	s := m.Other
	if s == nil || s.Role != "objeto" || s.Live == nil || s.Bg == nil {
		return "-" // não tem o que esperar
	}
	return autoDecide(*s.Live, *s.Bg, s.Elapsed, m.Pct)
}

// dualHint compara o GMC (beta e gama) com o RadiaCode (quase só gama).
func dualHint(gmc, rc MeasureResult) string {
	gs, rs := gmc.Sigma >= 3, rc.Sigma >= 3
	switch {
	case gs && rc.Sigma < 2:
		return "O GMC viu o objeto e o RadiaCode quase não: ele deve emitir principalmente beta (o cristal do RadiaCode quase não vê beta)."
	case gs && !rs:
		return "O GMC viu claramente e o RadiaCode só um pouco: o objeto deve emitir bastante beta."
	case rs && !gs:
		return "Só o RadiaCode viu: ele é bem mais sensível a gama que o tubo do GMC, então o objeto deve emitir gama fraco."
	case gs && rs:
		relG, relR := gmc.Ratio-1, rc.Ratio-1
		if relR > 0 && relG > 2*relR {
			return fmt.Sprintf("Os dois viram, mas o GMC subiu bem mais (%s× o fundo contra %s× no RadiaCode): além de gama, o objeto deve emitir beta.",
				fmtBR(gmc.Ratio, 1), fmtBR(rc.Ratio, 1))
		}
		return "Os dois viram, numa proporção parecida: o objeto deve emitir principalmente gama."
	}
	return "Nenhum dos dois viu uma diferença clara."
}

// finishSideLocked fecha a medição do segundo aparelho no fim.
func (a *App) finishSideLocked(m *MeasureState) {
	s := m.Other
	if s == nil || s.Role != "objeto" || s.Result == nil || m.Result == nil {
		return
	}
	g, r := *s.Result, *m.Result
	if devName(m.Device) == "gmc" {
		g, r = r, g
	}
	s.Hint = dualHint(g, r)
}

// sideBgAt prepara o segundo aparelho no começo da medição (fora do a.mu).
func (a *App) newSide(primary, mode string, homeBg bool) (*MeasureSide, error) {
	s := &MeasureSide{Device: otherDev(primary), Role: "objeto"}
	if mode == "voce" {
		s.Role = "voce"
	}
	if s.Device == devRC && !a.rcConnected() {
		return nil, fmt.Errorf("conecte o RadiaCode para medir com os dois")
	}
	a.mu.Lock()
	gmcOK, demo := a.dev != nil, a.demo.Active
	a.mu.Unlock()
	if demo {
		return nil, fmt.Errorf("desligue o modo demo para medir com os dois aparelhos")
	}
	if s.Device == "gmc" && !gmcOK {
		return nil, fmt.Errorf("conecte o GMC-800 para medir com os dois")
	}
	hb := a.GetHomeBackgroundDev(s.Device)
	if s.Role == "voce" {
		if hb.OK {
			s.BgUSv = hb.USv
		} else if s.Device == devRC {
			if st := a.rcStats(); st != nil {
				s.BgUSv = st.BgUSv
			}
		}
		return s, nil
	}
	if homeBg {
		if !hb.OK {
			return nil, fmt.Errorf("fundo da casa do %s indisponível: %s", map[string]string{"gmc": "GMC-800", devRC: "RadiaCode"}[s.Device], hb.Why)
		}
		s.Bg = &MeasurePhase{Counts: hb.Counts, Seconds: hb.Minutes * 60, CPM: hb.CPM, ErrCPM: hb.ErrCPM,
			USv: hb.USv, Start: hb.From, End: hb.To, Source: "casa"}
	}
	return s, nil
}
