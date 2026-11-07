package app

import (
	"math"
	"sort"
)

// ---------- atividade (Bq) com o RadiaCode: física e ajuste dos picos ----------
//
// Atividade de uma linha gama: A = N / (ε · y · t), com N as contagens do
// pico (amostra − fundo no mesmo tempo), ε a eficiência do aparelho naquela
// energia e naquela posição (o que a calibração mede), y a fração de
// decaimentos que solta aquele gama e t o tempo.
//
// No cristal do RadiaCode os picos são largos (~10% em 662 keV) e linhas
// vizinhas se misturam (583 do tório e 609 do rádio, 338 e 352…). Por isso
// cada região é ajustada com gaussianas de posição e largura conhecidas (uma
// por linha da região) mais uma reta: com posição e largura fixas o ajuste é
// linear (mínimos quadrados com o peso de Poisson) e a área de cada pico sai
// com a incerteza.

// actLine é uma linha gama usada na conta.
type actLine struct {
	KeV   float64
	Yield float64 // gamas por decaimento do nuclídeo "pai" (com a ramificação)
	Who   string  // quem emite
}

// ActNuclide é um radionuclídeo (ou série em equilíbrio) que dá para medir.
type ActNuclide struct {
	ID    string    `json:"id"`
	Name  string    `json:"name"`
	Note  string    `json:"note"`
	KeVs  []float64 `json:"kevs"`
	lines []actLine // as que entram na conta
}

func init() {
	for i := range actNuclides {
		for _, l := range actNuclides[i].lines {
			actNuclides[i].KeVs = append(actNuclides[i].KeVs, l.KeV)
		}
	}
}

// Rendimentos: ENSDF/NNDC (arredondados).
var actNuclides = []ActNuclide{
	{ID: "K-40", Name: "Potássio-40", lines: []actLine{{1460.8, 0.1066, "K-40"}}},
	{ID: "Cs-137", Name: "Césio-137", lines: []actLine{{661.7, 0.851, "Ba-137m"}}},
	{ID: "Ra-226", Name: "Rádio-226 (série do urânio)", Note: "pelos filhos Pb-214 e Bi-214: a amostra precisa ficar fechada ~3 semanas antes, para o radônio não escapar",
		lines: []actLine{{351.9, 0.356, "Pb-214"}, {609.3, 0.455, "Bi-214"}, {1764.5, 0.153, "Bi-214"}}},
	{ID: "Ra-228", Name: "Rádio-228 (início da série do tório)", Note: "pelo Ac-228, que acompanha o Ra-228 em poucos dias",
		lines: []actLine{{911.2, 0.258, "Ac-228"}, {969.0, 0.158, "Ac-228"}}},
	{ID: "Th-232", Name: "Tório-232 (série, em equilíbrio)", Note: "supõe a série inteira em equilíbrio (areia, rocha); em coisa nova ou purificada as linhas não batem",
		lines: []actLine{{238.6, 0.436, "Pb-212"}, {583.2, 0.304, "Tl-208"}, {911.2, 0.258, "Ac-228"}, {2614.5, 0.356, "Tl-208"}}},
	{ID: "U-238", Name: "Urânio-238 (urânio natural)", Note: "pelo Pa-234m em 1.001 keV: linha fraca, precisa de amostra forte (vidro ou esmalte de urânio)",
		lines: []actLine{{1001.0, 0.00842, "Pa-234m"}}},
}

// actOther são linhas que não entram na conta, mas aparecem nos ajustes para
// não contaminar as vizinhas.
var actOther = []actLine{
	{185.7, 0.57, "U-235"}, {295.2, 0.192, "Pb-214"}, {300.1, 0.033, "Pb-212"}, {338.3, 0.113, "Ac-228"},
	{510.8, 0.081, "Tl-208"}, {727.3, 0.066, "Bi-212"}, {794.9, 0.043, "Ac-228"}, {860.6, 0.045, "Tl-208"},
	{1120.3, 0.149, "Bi-214"}, {1173.2, 1, "Co-60"}, {1238.1, 0.058, "Bi-214"}, {1332.5, 1, "Co-60"},
	{1377.7, 0.040, "Bi-214"}, {1588.2, 0.032, "Ac-228"}, {1729.6, 0.029, "Bi-214"}, {2204.2, 0.049, "Bi-214"},
}

func actNuclide(id string) *ActNuclide {
	for i := range actNuclides {
		if actNuclides[i].ID == id {
			return &actNuclides[i]
		}
	}
	return nil
}

// constantes das referências
const (
	actKPerKCl  = 39.0983 / 74.551 // fração de potássio no KCl
	actBqPerGK  = 31.7             // Bq de K-40 por g de potássio natural (0,0117% de K-40, T½ 1,248×10⁹ anos)
	actThPerTO2 = 232.038 / 264.036
	actBqPerGTh = 4071 // Bq por g de Th-232 (T½ 1,40×10¹⁰ anos)
)

// actThFactors é quanto da atividade do Th-232 o Ra-228 (e o Ac-228) e o
// Th-228 (e Pb-212, Tl-208) têm, anos depois de o tório ser purificado (o
// eletrodo é feito com tório purificado: o rádio sai e volta a crescer).
// idade ≤ 0: não sabe; usa o meio da faixa de 2 a 15 anos, com a dúvida.
func actThFactors(ageY float64) (ra, th, rel float64) {
	if ageY <= 0 {
		return 0.6, 0.55, 0.25
	}
	l1, l2 := math.Ln2/5.75, math.Ln2/1.912
	e1, e2 := math.Exp(-l1*ageY), math.Exp(-l2*ageY)
	ra = 1 - e1
	th = e2 + (1 - e2) - l2/(l2-l1)*(e1-e2)
	return ra, th, 0.10
}

// ---------- calibração (eficiência) ----------

// ActCal é a eficiência do aparelho numa posição: ln ε(E) = LnEps + B·ln(E/Pivot).
type ActCal struct {
	Geometry string     `json:"geometry"`
	KCl      *ActCalKCl `json:"kcl,omitempty"`
	Th       *ActCalTh  `json:"th,omitempty"`
}

// ActCalKCl é a calibração pelo K-40 do KCl (absoluta, em 1.460,8 keV).
type ActCalKCl struct {
	T      int64   `json:"t"`
	Eff    float64 `json:"eff"`
	EffErr float64 `json:"effErr"` // relativa
	MassG  float64 `json:"massG"`
	Pct    float64 `json:"pct"`
	Sec    float64 `json:"sec"`
}

// ActCalTh é a forma da curva (e um valor absoluto aproximado) pelo tório.
type ActCalTh struct {
	T        int64   `json:"t"`
	B        float64 `json:"b"`
	BErr     float64 `json:"bErr"`
	Pivot    float64 `json:"pivot"`
	LnEps    float64 `json:"lnEps"`
	LnEpsErr float64 `json:"lnEpsErr"`
	MassG    float64 `json:"massG"`
	Pct      float64 `json:"pct"`
	AgeY     float64 `json:"ageY"`
	Sec      float64 `json:"sec"`
}

const (
	actDefB    = -1.3 // inclinação típica de um cristal pequeno encostado (sem o tório)
	actDefBErr = 0.4
)

// actEff devolve ε(E) e a incerteza relativa; src diz de onde veio.
func (c ActCal) eff(e float64) (eps, rel float64, src string) {
	b, bErr := actDefB, actDefBErr
	if c.Th != nil {
		b, bErr = c.Th.B, c.Th.BErr
	}
	switch {
	case c.KCl != nil:
		x := math.Log(e / 1460.8)
		ln := math.Log(c.KCl.Eff) + b*x
		return math.Exp(ln), math.Hypot(c.KCl.EffErr, bErr*x), "kcl"
	case c.Th != nil:
		x := math.Log(e / c.Th.Pivot)
		return math.Exp(c.Th.LnEps + c.Th.B*x), math.Hypot(c.Th.LnEpsErr, c.Th.BErr*x), "torio"
	}
	// sem calibração: chute de 0,1% em 1.460 keV, com 100% de dúvida
	x := math.Log(e / 1460.8)
	return math.Exp(math.Log(0.001) + b*x), math.Hypot(1, bErr*x), "chute"
}

// ---------- ajuste ----------

type actPeak struct {
	Area, Err float64
	ok        bool
}

// actSpec é um espectro somado (contagens por canal) com o tempo.
type actSpec struct {
	C   []float64
	Sec float64
	A   [3]float64
}

func (s actSpec) energy(c float64) float64 { return s.A[0] + s.A[1]*c + s.A[2]*c*c }

// actFit ajusta a região em volta de kev e devolve a área do pico de kev.
// Todas as linhas conhecidas com centro dentro da região entram no ajuste;
// linhas a menos de ¼ de largura uma da outra viram uma só.
func actFit(smp, bg actSpec, kev float64) actPeak {
	n := len(smp.C)
	if n == 0 || smp.Sec <= 0 {
		return actPeak{}
	}
	F := specFWHM(kev)
	lo, hi := kev-1.6*F, kev+1.6*F
	// linhas na região
	var cen []float64
	add := func(e float64) {
		if e < lo || e > hi {
			return
		}
		for _, c := range cen {
			if math.Abs(c-e) < 0.25*specFWHM(e) {
				return
			}
		}
		cen = append(cen, e)
	}
	add(kev)
	for _, nu := range actNuclides {
		for _, l := range nu.lines {
			add(l.KeV)
		}
	}
	for _, l := range actOther {
		add(l.KeV)
	}
	sort.Float64s(cen)
	target := -1
	for i, c := range cen {
		if c == kev {
			target = i
		}
	}
	if target < 0 { // fundiu com uma vizinha: a mais perto representa
		best := math.Inf(1)
		for i, c := range cen {
			if d := math.Abs(c - kev); d < best {
				best, target = d, i
			}
		}
	}
	// ajusta; as linhas vizinhas que não aparecem (área abaixo de 2σ) saem
	// uma a uma, senão cada uma "rouba" incerteza do pico que interessa
	for {
		area, err, ok := actFitSet(smp, bg, kev, F, lo, hi, cen)
		if !ok {
			return actPeak{}
		}
		worst, worstZ := -1, 2.0
		for i := range cen {
			if i == target {
				continue
			}
			z := area[i] / math.Max(err[i], 1e-9)
			if z < worstZ {
				worst, worstZ = i, z
			}
		}
		if worst < 0 {
			return actPeak{Area: area[target], Err: err[target], ok: true}
		}
		cen = append(cen[:worst:worst], cen[worst+1:]...)
		if worst < target {
			target--
		}
	}
}

// actFitSet ajusta as gaussianas cen (mais uma reta) na região [lo, hi].
func actFitSet(smp, bg actSpec, kev, F, lo, hi float64, cen []float64) (area, aerr []float64, ok bool) {
	n := len(smp.C)
	r := 0.0
	if bg.Sec > 0 && len(bg.C) == n {
		r = smp.Sec / bg.Sec
	}
	np := len(cen) + 2
	N := make([][]float64, np)
	for i := range N {
		N[i] = make([]float64, np)
	}
	rhs := make([]float64, np)
	row := make([]float64, np)
	type pt struct{ y, w float64 }
	var pts []pt
	var rows [][]float64
	for i := 0; i < n; i++ {
		e0, e1 := smp.energy(float64(i)), smp.energy(float64(i+1))
		ec := (e0 + e1) / 2
		if ec < lo || ec > hi || e1 <= e0 {
			continue
		}
		y := smp.C[i]
		v := smp.C[i]
		if r > 0 {
			y -= r * bg.C[i]
			v += r * r * bg.C[i]
		}
		w := 1 / math.Max(v, 1)
		for k, c := range cen {
			s := specFWHM(c) / 2.3548
			// área do canal sob a gaussiana de área 1
			row[k] = 0.5 * (math.Erf((e1-c)/(s*math.Sqrt2)) - math.Erf((e0-c)/(s*math.Sqrt2)))
		}
		row[len(cen)] = 1
		row[len(cen)+1] = (ec - kev) / F
		for a := 0; a < np; a++ {
			rhs[a] += row[a] * y * w
			for b := 0; b < np; b++ {
				N[a][b] += row[a] * row[b] * w
			}
		}
		pts = append(pts, pt{y, w})
		rows = append(rows, append([]float64(nil), row...))
	}
	if len(pts) <= np {
		return nil, nil, false
	}
	inv, ok := invert(N)
	if !ok {
		return nil, nil, false
	}
	p := make([]float64, np)
	for a := 0; a < np; a++ {
		for b := 0; b < np; b++ {
			p[a] += inv[a][b] * rhs[b]
		}
	}
	// χ²: se o ajuste não cabe (calibração de energia um pouco fora, linha
	// que não está na lista), a incerteza cresce junto
	chi := 0.0
	for j, q := range pts {
		m := 0.0
		for a := 0; a < np; a++ {
			m += rows[j][a] * p[a]
		}
		chi += (q.y - m) * (q.y - m) * q.w
	}
	scale := math.Max(1, math.Sqrt(chi/float64(len(pts)-np)))
	area, aerr = make([]float64, len(cen)), make([]float64, len(cen))
	for k := range cen {
		area[k] = p[k]
		aerr[k] = math.Sqrt(math.Max(inv[k][k], 0)) * scale
	}
	return area, aerr, true
}

// invert inverte uma matriz pequena (Gauss-Jordan com pivô).
func invert(m [][]float64) ([][]float64, bool) {
	n := len(m)
	a := make([][]float64, n)
	for i := range m {
		a[i] = make([]float64, 2*n)
		copy(a[i], m[i])
		a[i][n+i] = 1
	}
	for c := 0; c < n; c++ {
		p := c
		for r := c + 1; r < n; r++ {
			if math.Abs(a[r][c]) > math.Abs(a[p][c]) {
				p = r
			}
		}
		if math.Abs(a[p][c]) < 1e-12 {
			return nil, false
		}
		a[c], a[p] = a[p], a[c]
		d := a[c][c]
		for k := range a[c] {
			a[c][k] /= d
		}
		for r := 0; r < n; r++ {
			if r == c || a[r][c] == 0 {
				continue
			}
			f := a[r][c]
			for k := range a[r] {
				a[r][k] -= f * a[c][k]
			}
		}
	}
	out := make([][]float64, n)
	for i := range a {
		out[i] = a[i][n:]
	}
	return out, true
}

// ---------- resultado ----------

// ActLineRes é uma linha medida.
type ActLineRes struct {
	KeV     float64 `json:"kev"`
	Who     string  `json:"who"`
	Yield   float64 `json:"yield"`
	Area    float64 `json:"area"` // contagens do pico (amostra − fundo)
	AreaErr float64 `json:"areaErr"`
	Eff     float64 `json:"eff"`
	Bq      float64 `json:"bq"`
	Err     float64 `json:"err"`     // só a contagem
	NeedSec float64 `json:"needSec"` // tempo de amostra para ±10% na contagem
}

// ActNucRes é um radionuclídeo medido.
type ActNucRes struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Note     string       `json:"note"`
	Bq       float64      `json:"bq"`
	Err      float64      `json:"err"` // total (contagem + calibração)
	StatErr  float64      `json:"statErr"`
	Detected bool         `json:"detected"`
	MDA      float64      `json:"mda"` // atividade mínima detectável (Bq)
	BqKg     float64      `json:"bqKg"`
	ErrKg    float64      `json:"errKg"`
	MDAKg    float64      `json:"mdaKg"`
	Lines    []ActLineRes `json:"lines"`
	NeedSec  float64      `json:"needSec"` // tempo para ±10% (contagem)
}

// actAnalyze mede os radionuclídeos pedidos.
func actAnalyze(smp, bg actSpec, ids []string, cal ActCal, massG float64) []ActNucRes {
	out := []ActNucRes{}
	for _, id := range ids {
		nu := actNuclide(id)
		if nu == nil {
			continue
		}
		res := ActNucRes{ID: nu.ID, Name: nu.Name, Note: nu.Note, Lines: []ActLineRes{}}
		var sw, swa float64
		var effRel, effW float64
		for _, l := range nu.lines {
			pk := actFit(smp, bg, l.KeV)
			if !pk.ok {
				continue
			}
			eps, rel, _ := cal.eff(l.KeV)
			k := 1 / (eps * l.Yield * smp.Sec)
			lr := ActLineRes{KeV: l.KeV, Who: l.Who, Yield: l.Yield, Area: pk.Area, AreaErr: pk.Err, Eff: eps,
				Bq: pk.Area * k, Err: pk.Err * k}
			if pk.Area > 0 && pk.Err > 0 {
				lr.NeedSec = smp.Sec * math.Pow(pk.Err/pk.Area/0.10, 2)
			}
			res.Lines = append(res.Lines, lr)
			if lr.Err > 0 {
				w := 1 / (lr.Err * lr.Err)
				sw += w
				swa += w * lr.Bq
				effRel += w * rel
				effW += w
			}
		}
		if sw > 0 {
			res.Bq = swa / sw
			res.StatErr = 1 / math.Sqrt(sw)
			rel := effRel / effW
			res.Err = math.Hypot(res.StatErr, res.Bq*rel)
			// detectado: acima de 1,645σ da contagem (5% de chance de ser só
			// flutuação); MDA de Currie ≈ 2,71/(εyt) + 4,65σ₀, com σ₀ ≈ σ
			res.Detected = res.Bq > 1.645*res.StatErr
			res.MDA = 4.65 * res.StatErr
			if res.Bq > 0 {
				res.NeedSec = smp.Sec * math.Pow(res.StatErr/res.Bq/0.10, 2)
			}
		}
		if massG > 0 {
			kg := massG / 1000
			res.BqKg, res.ErrKg, res.MDAKg = res.Bq/kg, res.Err/kg, res.MDA/kg
		}
		out = append(out, res)
	}
	return out
}

// actCalKCl mede a eficiência em 1.460,8 keV com o KCl.
func actCalKCl(smp, bg actSpec, massG, pct float64) (*ActCalKCl, float64, float64) {
	aK := massG * pct / 100 * actKPerKCl * actBqPerGK
	pk := actFit(smp, bg, 1460.8)
	if !pk.ok || aK <= 0 || pk.Area <= 0 {
		return nil, aK, 0
	}
	eff := pk.Area / (aK * 0.1066 * smp.Sec)
	return &ActCalKCl{Eff: eff, EffErr: pk.Err / pk.Area, MassG: massG, Pct: pct, Sec: smp.Sec}, aK, pk.Area
}

// actCalTh mede a forma da curva (e um valor absoluto aproximado) com o
// eletrodo: as linhas 238,6 / 583,2 / 2.614,5 keV vêm todas do Th-228 (mesma
// atividade, qualquer que seja a idade), então a razão entre elas é só a
// eficiência.
func actCalTh(smp, bg actSpec, massG, pct, ageY float64) (*ActCalTh, []ActLineRes) {
	lines := []actLine{{238.6, 0.436, "Pb-212"}, {583.2, 0.304, "Tl-208"}, {2614.5, 0.356, "Tl-208"}}
	var xs, ys, ws []float64
	var res []ActLineRes
	const pivot = 583.2
	for _, l := range lines {
		pk := actFit(smp, bg, l.KeV)
		if !pk.ok {
			continue
		}
		res = append(res, ActLineRes{KeV: l.KeV, Who: l.Who, Yield: l.Yield, Area: pk.Area, AreaErr: pk.Err})
		if pk.Area <= 0 || pk.Err <= 0 || pk.Area < 3*pk.Err {
			continue
		}
		rate := pk.Area / (l.Yield * smp.Sec)
		xs = append(xs, math.Log(l.KeV/pivot))
		ys = append(ys, math.Log(rate))
		rel := pk.Err / pk.Area
		ws = append(ws, 1/(rel*rel))
	}
	if len(xs) < 2 {
		return nil, res
	}
	var S, Sx, Sy, Sxx, Sxy float64
	for i := range xs {
		S += ws[i]
		Sx += ws[i] * xs[i]
		Sy += ws[i] * ys[i]
		Sxx += ws[i] * xs[i] * xs[i]
		Sxy += ws[i] * xs[i] * ys[i]
	}
	d := S*Sxx - Sx*Sx
	if d <= 0 {
		return nil, res
	}
	b := (S*Sxy - Sx*Sy) / d
	c := (Sxx*Sy - Sx*Sxy) / d
	bErr, cErr := math.Sqrt(S/d), math.Sqrt(Sxx/d)
	if len(xs) == 2 { // duas linhas: sem como conferir; um pouco mais de dúvida
		bErr = math.Max(bErr, 0.1)
	}
	_, f228, fRel := actThFactors(ageY)
	aTh := massG * pct / 100 * actThPerTO2 * actBqPerGTh * f228
	cal := &ActCalTh{B: b, BErr: bErr, Pivot: pivot, MassG: massG, Pct: pct, AgeY: ageY, Sec: smp.Sec}
	if aTh > 0 {
		cal.LnEps = c - math.Log(aTh)
		cal.LnEpsErr = math.Hypot(cErr, fRel)
	}
	return cal, res
}
