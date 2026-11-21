package app

import "math"

// ---------- escala de energia: onde o K-40 está de verdade ----------
//
// 2.103.0 (avaliação do Bq, 21/11): o RC102 do ACE mostra o pico do K-40 em
// ~1.415 keV, não em 1.460,8: a escala de energia que o aparelho manda está
// ~3% baixa. Como o ajuste usa gaussianas de posição fixa, o app pegava só
// 40–60% do pico e a calibração saía baixa. Agora cada medida acha o K-40,
// corrige a escala (um fator só, o ganho) das duas partes e põe o fundo na
// mesma grade de canais da amostra.

// ActGain diz onde o K-40 foi achado.
type ActGain struct {
	Found   bool    `json:"found"`
	PeakKeV float64 `json:"peakKeV"` // centro do K-40 na escala que o aparelho manda
	Gain    float64 `json:"gain"`    // 1.460,8 / PeakKeV (1 = sem correção)
}

const (
	actK40     = 1460.8
	actGainMax = 0.06 // procura o K-40 até 6% para cada lado
	actGainZ   = 4.0  // e só aceita um pico claro (área acima de 4σ)
	actGainOff = 15.0 // a tela avisa quando o pico está a mais de 15 keV do esperado
)

// actScaled multiplica a escala de energia por g.
func (s actSpec) scaled(g float64) actSpec {
	s.A = [3]float64{s.A[0] * g, s.A[1] * g, s.A[2] * g}
	return s
}

// actResample põe as contagens de src na grade de canais de dst (pela
// energia de cada canal), dividindo cada canal de src pela fração que cai em
// cada canal de dst.
func actResample(src, dst actSpec) actSpec {
	out := actSpec{C: make([]float64, len(dst.C)), Sec: src.Sec, A: dst.A}
	if src.Sec <= 0 || len(src.C) == 0 || len(dst.C) == 0 {
		return out
	}
	if src.A == dst.A && len(src.C) == len(dst.C) {
		copy(out.C, src.C)
		return out
	}
	j := 0
	for i := range dst.C {
		d0, d1 := dst.energy(float64(i)), dst.energy(float64(i+1))
		if d1 <= d0 {
			continue
		}
		for j > 0 && src.energy(float64(j)) > d0 {
			j--
		}
		for ; j < len(src.C); j++ {
			s0, s1 := src.energy(float64(j)), src.energy(float64(j+1))
			if s0 >= d1 {
				break
			}
			if s1 <= d0 || s1 <= s0 {
				continue
			}
			ov := math.Min(s1, d1) - math.Max(s0, d0)
			if ov > 0 {
				out.C[i] += src.C[j] * ov / (s1 - s0)
			}
		}
	}
	return out
}

// actFindK40 procura o pico do K-40 em s (até 6% fora de 1.460,8 keV): a
// gaussiana de largura conhecida mais uma reta em cada centro possível; fica
// o centro com a área mais clara, refinado pela parábola nos vizinhos.
func actFindK40(s actSpec) (float64, bool) {
	if s.Sec <= 0 || len(s.C) == 0 {
		return 0, false
	}
	const step = 1.0
	var cs, zs []float64
	for c := actK40 * (1 - actGainMax); c <= actK40*(1+actGainMax); c += step {
		F := specFWHM(c)
		area, err, ok := actFitSet(s, actSpec{}, c, F, c-1.6*F, c+1.6*F, []float64{c})
		z := math.Inf(-1)
		if ok && err[0] > 0 {
			z = area[0] / err[0]
		}
		cs, zs = append(cs, c), append(zs, z)
	}
	best := -1
	for i, z := range zs {
		if best < 0 || z > zs[best] {
			best = i
		}
	}
	if best < 0 || zs[best] < actGainZ || best == 0 || best == len(zs)-1 {
		return 0, false // sem pico claro, ou na beira da busca (não é o K-40)
	}
	c := cs[best]
	if l, r := zs[best-1], zs[best+1]; !math.IsInf(l, 0) && !math.IsInf(r, 0) {
		if d := l - 2*zs[best] + r; d < 0 {
			c += step * 0.5 * (l - r) / d
		}
	}
	return c, true
}

// actAlign acha o K-40, corrige a escala das duas partes pelo mesmo ganho e
// põe o fundo na grade da amostra. Na calibração com KCl o pico é procurado
// na soma (sal + fundo, o mais contado); nas outras medidas, no fundo (o
// K-40 das paredes), para minério nenhum confundir a busca. Fundo curto
// quase não mostra o K-40 das paredes (~0,03 cps): sem pico claro, vale o
// ganho achado na calibração (fb), que o aparelho mantém de um dia para o
// outro dentro de ~1%.
func actAlign(smp, bg actSpec, inSum bool, fb float64) (actSpec, actSpec, ActGain) {
	g := ActGain{Gain: 1}
	if len(smp.C) == 0 {
		return smp, bg, g
	}
	if bg.Sec > 0 {
		bg = actResample(bg, smp)
	}
	ref := bg
	if inSum && bg.Sec > 0 {
		ref = actSpec{C: make([]float64, len(smp.C)), Sec: smp.Sec + bg.Sec, A: smp.A}
		for i := range ref.C {
			ref.C[i] = smp.C[i] + bg.C[i]
		}
	} else if bg.Sec <= 0 {
		ref = smp
	}
	// QA 2.103.0: numa medida de amostra, o ganho da calibração (achado no sal,
	// bem contado) vale mais que o K-40 das paredes num fundo de poucas horas:
	// ali o pico achado sai até 1–3% fora, e 1% fora já tira 15–20% da área
	// em 352/609 keV e metade em 1.764 keV. O fundo só decide sem calibração.
	if !inSum && fb > 0 && math.Abs(fb-1) <= actGainMax {
		g = ActGain{Gain: fb, PeakKeV: actK40 / fb}
	} else if c, ok := actFindK40(ref); ok {
		g = ActGain{Found: true, PeakKeV: c, Gain: actK40 / c}
	} else if fb > 0 && math.Abs(fb-1) <= actGainMax {
		g = ActGain{Gain: fb, PeakKeV: actK40 / fb}
	}
	smp = smp.scaled(g.Gain)
	if bg.Sec > 0 {
		bg = bg.scaled(g.Gain)
	}
	return smp, bg, g
}

// gain é o ganho achado na calibração do KCl em uso (0 = não tem).
func (c ActCal) gain() float64 {
	if c.KCl != nil && c.KCl.PeakKeV > 0 && c.KCl.Gain > 0 {
		return c.KCl.Gain
	}
	return 0
}
