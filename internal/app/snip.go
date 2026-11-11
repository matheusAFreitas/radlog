package app

import "math"

// ---------- fundo contínuo pelo SNIP ----------
//
// No acumulado do aparelho não há fundo para subtrair, e o contínuo é enorme
// (e liso, quando vem de beta). O SNIP (Statistics-sensitive Non-linear
// Iterative Peak-clipping, Morháč et al.) é o jeito padrão de achar esse
// contínuo: canal a canal, troca o valor pela média dos vizinhos a p canais
// de distância quando ela é menor, com p descendo da largura do pico até 1.
// O que sobra acima é pico. Trabalha no log das contagens: o contínuo é soma
// de exponenciais, que no log é reta ou curva para cima, e aí o corte não
// come o contínuo (no log(log(√y)) usual, a exponencial vira curva para baixo
// e o SNIP inventa picos).

// snipBase devolve o contínuo embaixo de y. width(i) é a largura do pico (em
// canais) no canal i: o corte nunca usa vizinhos mais longe que isso.
func snipBase(y []float64, width func(i int) int) []float64 {
	n := len(y)
	v := make([]float64, n)
	for i, x := range y {
		v[i] = math.Log(math.Max(x, 0) + 1)
	}
	maxW := 0
	for i := 0; i < n; i++ {
		if w := width(i); w > maxW {
			maxW = w
		}
	}
	tmp := make([]float64, n)
	for p := maxW; p >= 1; p-- {
		copy(tmp, v)
		for i := p; i < n-p; i++ {
			if p > width(i) {
				continue
			}
			if m := (v[i-p] + v[i+p]) / 2; m < tmp[i] {
				tmp[i] = m
			}
		}
		v, tmp = tmp, v
	}
	out := make([]float64, n)
	for i, x := range v {
		out[i] = math.Exp(x) - 1
		if out[i] < 0 {
			out[i] = 0
		}
	}
	return out
}

// findPeaksSnip procura as linhas conhecidas num espectro sem fundo para
// subtrair (o acumulado do aparelho), medindo cada pico acima do contínuo do
// SNIP. Com dezenas de milhões de contagens qualquer ondinha passa de 3σ,
// então pede 5σ e que o pico tenha pelo menos 1,5% do contínuo embaixo dele.
func (s *MeasureSpectrum) findPeaksSnip() []SpecPeak {
	n := len(s.Obj)
	y := make([]float64, n)
	for i, c := range s.Obj {
		y[i] = float64(c)
	}
	width := func(i int) int {
		e := s.energy(i)
		de := s.energy(i+1) - e
		if de <= 0 {
			return 1
		}
		w := int(math.Round(specFWHM(e) / de))
		if w < 1 {
			w = 1
		}
		return w
	}
	// o SNIP corta para baixo nas flutuações (pega o mínimo); alisando antes,
	// numa janela de meio pico, o contínuo não fica embaixo do ruído
	sm := make([]float64, n)
	for i := range y {
		k := width(i)
		var a float64
		var c int
		for j := i - k; j <= i+k; j++ {
			if j >= 0 && j < n {
				a += y[j]
				c++
			}
		}
		sm[i] = a / float64(c)
	}
	base := snipBase(sm, width)
	emax := s.energy(n - 1)
	var out []SpecPeak
	for _, l := range specLines {
		// abaixo de 100 keV o acumulado é um morro com dentes (raios X,
		// retroespalhamento), sem contínuo liso para o SNIP: não procura
		if l.kev < 100 {
			continue
		}
		h := 0.6 * specFWHM(l.kev)
		// o SNIP não precisa das janelas dos lados: basta o pico caber
		if l.kev+h > emax || l.kev-h < s.energy(0) {
			continue
		}
		var peak, b, vr, ce float64
		for i := range y {
			if e := s.energy(i); e >= l.kev-h && e < l.kev+h {
				peak += y[i] - base[i]
				b += base[i]
				vr += y[i]
				ce += (y[i] - base[i]) * e
			}
		}
		if vr <= 0 || b <= 0 || peak <= 0 {
			continue
		}
		// o centro do que sobra tem que cair na linha: a cauda de um pico
		// vizinho (1120 do Bi-214 embaixo de 1173) puxa o centro para o lado
		if math.Abs(ce/peak-l.kev) > 0.2*specFWHM(l.kev) {
			continue
		}
		if peak < 0.015*b {
			continue
		}
		// e tem que ter forma de pico: subir acima das duas janelas dos lados
		// (como no modo com fundo). Embaixo de uma borda Compton o contínuo no
		// log fica côncavo e o SNIP passa por baixo do platô; um platô ou uma
		// borda não sobem acima dos lados, um pico sobe. No fim da escala
		// (Tl-208 em 2614), sem janela à direita, fica só o SNIP.
		if l.kev+3*h <= emax && l.kev-3*h >= s.energy(0) {
			var c, lf, rt, cn, ln, rn float64
			for i := range y {
				switch e := s.energy(i); {
				case e >= l.kev-h && e < l.kev+h:
					c += y[i]
					cn++
				case e >= l.kev-3*h && e < l.kev-h:
					lf += y[i]
					ln++
				case e >= l.kev+h && e < l.kev+3*h:
					rt += y[i]
					rn++
				}
			}
			if ln == 0 || rn == 0 || lf <= 0 || rt <= 0 {
				continue
			}
			side := cn * math.Sqrt(lf/ln*rt/rn)
			f := cn / (ln + rn)
			ex := c - side
			if ex < 0.015*side || ex/math.Sqrt(c+f*f*(lf+rt)) < 5 {
				continue
			}
		}
		if sg := peak / math.Sqrt(vr); sg >= 5 {
			out = append(out, SpecPeak{Group: l.group, Nuclide: l.nuclide, KeV: l.kev, Net: peak, Sigma: sg})
		}
	}
	return dropBackscatter(out)
}

// dropBackscatter tira os picos que caem no retroespalhamento de um pico bem
// mais forte. O gama que bate na parede atrás do cristal e volta chega com
// E/(1+2E/511) (184 keV no Cs-137, 218 no K-40, 233 no Tl-208), num morro
// umas 2,5× mais largo que um pico, com 10 a 25% das contagens dele. Esse
// morro passa por pico no SNIP e nas janelas dos lados e vira U-235 em 186 ou
// Pb-212 em 239. Uma linha a menos de 1 FWHM desse ponto só fica se tiver
// pelo menos 30% das contagens do pico que a explicaria: o Pb-212 do tório é
// muito mais forte que o Tl-208, o Pb-214 de 295 passa da metade do Bi-214
// de 609.
func dropBackscatter(ps []SpecPeak) []SpecPeak {
	var out []SpecPeak
	for _, q := range ps {
		keep := true
		for _, p := range ps {
			if p.KeV < 300 {
				continue
			}
			eb := p.KeV / (1 + 2*p.KeV/511)
			if math.Abs(q.KeV-eb) < specFWHM(eb) && q.Net < 0.3*p.Net {
				keep = false
				break
			}
		}
		if keep {
			out = append(out, q)
		}
	}
	return out
}
