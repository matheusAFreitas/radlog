// Package intercal faz a intercalibração do GMC com o RadiaCode (ideia 6):
// com os dois aparelhos encostados, o fator do GMC (µSv/h por CPM) é a soma
// do µSv/h do RadiaCode dividida pela soma do CPM do GMC nos minutos em que
// os dois mediram. É a mesma conta do intercal.py da análise de 09/11.
//
// Tem também a vigia semanal: depois de calibrado, compara a razão de
// CONTAGENS dos dois (cps do RadiaCode ÷ CPM do GMC) nos minutos em que os
// dois estavam em casa com a razão normal das primeiras semanas. A razão de
// µSv/h não serve para a vigia porque o µSv/h por contagem do RadiaCode muda
// com a mistura de energias do fundo (radônio).
//
// Não depende da interface nem do Wails: o app Android vai usar o mesmo
// pacote.
package intercal

import (
	"errors"
	"math"
	"sort"
	"time"
)

// Pair é um minuto com os dois aparelhos.
type Pair struct {
	T      time.Time
	Counts float64 // contagens do GMC no minuto (CPM bruto)
	USvh   float64 // µSv/h do RadiaCode no minuto
	CPS    float64 // cps do RadiaCode no minuto
}

// Hour é o fator de uma hora da coleta.
type Hour struct {
	T   time.Time `json:"t"`
	Min int       `json:"min"`
	K   float64   `json:"k"`
	Err float64   `json:"err"` // 1σ pela contagem
}

// Result é o resultado da intercalibração.
type Result struct {
	K        float64 `json:"k"`     // fator novo, µSv/h por CPM
	Rel      float64 `json:"rel"`   // incerteza relativa (1σ)
	Rel95    float64 `json:"rel95"` // margem de 95%
	Lo       float64 `json:"lo"`
	Hi       float64 `json:"hi"`
	Minutes  int     `json:"minutes"`  // minutos que entraram
	Outliers int     `json:"outliers"` // minutos fora da curva que ficaram de fora
	GMCCPM   float64 `json:"gmcCpm"`   // CPM médio do GMC
	RCUSvh   float64 `json:"rcUsvh"`   // µSv/h médio do RadiaCode
	// Dispersion é variância/média do CPM minuto a minuto: perto de 1 num
	// tubo saudável (contagem de Poisson).
	Dispersion    float64 `json:"dispersion"`
	DispersionErr float64 `json:"dispersionErr"`
	CountRatio    float64 `json:"countRatio"` // contagens do RadiaCode por contagem do GMC
	Hours         []Hour  `json:"hours"`
}

// Opções da conta (as mesmas do intercal.py).
const (
	BlockMin = 60  // bloco do jackknife
	OutlierZ = 5.0 // ponto fora da curva
	MinPairs = 60  // menos que isso não dá conta
)

// ErrFewPairs: poucos minutos com os dois.
var ErrFewPairs = errors.New("poucos minutos com os dois aparelhos")

var t975tab = []float64{12.706, 4.303, 3.182, 2.776, 2.571, 2.447, 2.365, 2.306, 2.262, 2.228, 2.201, 2.179, 2.160, 2.145,
	2.131, 2.120, 2.110, 2.101, 2.093, 2.086, 2.080, 2.074, 2.069, 2.064, 2.060, 2.056, 2.052, 2.048, 2.045, 2.042}

// t975 é o quantil 0,975 da t de Student.
func t975(dof int) float64 {
	if dof <= 0 {
		return math.NaN()
	}
	if dof <= 30 {
		return t975tab[dof-1]
	}
	return 1.96 + 2.4/float64(dof)
}

func median(v []float64) float64 {
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	n := len(s)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

func hypot0(a, b float64) float64 {
	x, y := 0.0, 0.0
	if a > 0 {
		x = 1 / math.Sqrt(a)
	}
	if b > 0 {
		y = 1 / math.Sqrt(b)
	}
	return math.Hypot(x, y)
}

// Compute faz a conta com os minutos pareados (em qualquer ordem).
func Compute(in []Pair) (Result, error) {
	ps := make([]Pair, 0, len(in))
	for _, p := range in {
		if p.Counts >= 0 && p.USvh >= 0 && p.CPS >= 0 && !math.IsNaN(p.USvh) {
			ps = append(ps, p)
		}
	}
	if len(ps) < MinPairs {
		return Result{Minutes: len(ps)}, ErrFewPairs
	}
	sort.Slice(ps, func(i, j int) bool { return ps[i].T.Before(ps[j].T) })

	// pontos fora da curva (fonte perto, pancada no cabo…)
	var mg float64
	rs := make([]float64, len(ps))
	for i, p := range ps {
		mg += p.Counts
		rs[i] = p.USvh
	}
	mg /= float64(len(ps))
	medR := median(rs)
	dev := make([]float64, len(rs))
	for i, r := range rs {
		dev[i] = math.Abs(r - medR)
	}
	madR := median(dev) * 1.4826
	if madR == 0 {
		madR = 1e-9
	}
	keep := ps[:0:0]
	out := 0
	for _, p := range ps {
		if math.Abs(p.Counts-mg)/math.Sqrt(math.Max(mg, 1)) > OutlierZ || math.Abs(p.USvh-medR)/madR > OutlierZ {
			out++
			continue
		}
		keep = append(keep, p)
	}
	if len(keep) < MinPairs {
		return Result{Minutes: len(keep), Outliers: out}, ErrFewPairs
	}

	var sG, sR, sC float64
	for _, p := range keep {
		sG += p.Counts
		sR += p.USvh
		sC += p.CPS * 60
	}
	if sG <= 0 {
		return Result{Minutes: len(keep), Outliers: out}, ErrFewPairs
	}
	k := sR / sG
	relPois := hypot0(sG, sC)

	// jackknife por blocos de 1 h (o µSv/h do RadiaCode é suavizado: minutos
	// vizinhos não são independentes)
	w0 := keep[0].T
	type acc struct {
		g, r, c float64
		n       int
	}
	blocks := map[int64]*acc{}
	var order []int64
	for _, p := range keep {
		b := int64(p.T.Sub(w0) / (BlockMin * time.Minute))
		a := blocks[b]
		if a == nil {
			a = &acc{}
			blocks[b] = a
			order = append(order, b)
		}
		a.g += p.Counts
		a.r += p.USvh
		a.c += p.CPS * 60
		a.n++
	}
	var bl []*acc
	for _, b := range order {
		if blocks[b].n >= BlockMin/2 {
			bl = append(bl, blocks[b])
		}
	}
	relJK := math.NaN()
	if B := len(bl); B >= 4 {
		var TG, TR float64
		for _, b := range bl {
			TG += b.g
			TR += b.r
		}
		th := make([]float64, B)
		var tm float64
		for j, b := range bl {
			th[j] = (TR - b.r) / (TG - b.g)
			tm += th[j]
		}
		tm /= float64(B)
		var v float64
		for _, x := range th {
			v += (x - tm) * (x - tm)
		}
		v *= float64(B-1) / float64(B)
		relJK = math.Sqrt(v) / k
	}
	rel, dof := relPois, 1000
	if !math.IsNaN(relJK) && relJK >= relPois {
		rel, dof = relJK, len(bl)-1
	}
	r := Result{
		K: k, Rel: rel, Rel95: rel * t975(dof), Minutes: len(keep), Outliers: out,
		GMCCPM: sG / float64(len(keep)), RCUSvh: sR / float64(len(keep)), CountRatio: sC / sG,
	}
	r.Lo, r.Hi = k*(1-r.Rel95), k*(1+r.Rel95)

	// tubo: variância/média do CPM minuto a minuto
	if n := len(keep); n > 1 {
		var v float64
		for _, p := range keep {
			v += (p.Counts - r.GMCCPM) * (p.Counts - r.GMCCPM)
		}
		v /= float64(n - 1)
		if r.GMCCPM > 0 {
			r.Dispersion = v / r.GMCCPM
		}
		r.DispersionErr = math.Sqrt(2 / float64(n-1))
	}

	// hora a hora
	hours := map[int64]*acc{}
	var hord []int64
	for _, p := range keep {
		h := int64(p.T.Sub(w0) / time.Hour)
		a := hours[h]
		if a == nil {
			a = &acc{}
			hours[h] = a
			hord = append(hord, h)
		}
		a.g += p.Counts
		a.r += p.USvh
		a.c += p.CPS * 60
		a.n++
	}
	for _, h := range hord {
		a := hours[h]
		if a.n < 30 || a.g <= 0 {
			continue
		}
		hk := a.r / a.g
		r.Hours = append(r.Hours, Hour{T: w0.Add(time.Duration(h) * time.Hour), Min: a.n, K: hk, Err: hk * hypot0(a.g, a.c)})
	}
	return r, nil
}

// Estados da vigia.
const (
	WatchNoData   = "sem-dados"  // ainda não há minutos suficientes com os dois em casa
	WatchLearning = "aprendendo" // juntando as 4 primeiras semanas
	WatchAgree    = "concordam"
	WatchDrift    = "afastando"  // mais de 5% do normal por duas semanas seguidas
	WatchOneWeek  = "uma-semana" // fora do normal só na última semana (ainda não avisa)
)

// Parâmetros da vigia (aprovados pelo ACE em 10/11).
const (
	LearnDays   = 28   // o normal sai das 4 primeiras semanas
	WeekMinutes = 720  // pelo menos 12 h juntos numa semana
	DriftPct    = 5.0  // limite do aviso
	learnMinMin = 2880 // pelo menos 48 h juntos no aprendizado
)

// Week é a razão de contagens de uma semana.
type Week struct {
	From    time.Time `json:"from"`
	Minutes int       `json:"minutes"`
	Ratio   float64   `json:"ratio"` // 0 = poucos minutos
	Pct     float64   `json:"pct"`   // diferença do normal, %
}

// Watch é o estado da vigia.
type Watch struct {
	State      string    `json:"state"`
	Since      time.Time `json:"since"`      // começo do aprendizado (a calibração em uso)
	LearnUntil time.Time `json:"learnUntil"` // fim do aprendizado
	Normal     float64   `json:"normal"`     // razão normal (0 enquanto aprende)
	NormalMin  int       `json:"normalMin"`  // minutos do aprendizado
	Last       Week      `json:"last"`       // últimos 7 dias
	Prev       Week      `json:"prev"`       // os 7 antes
	LearnedMin int       `json:"learnedMin"` // minutos juntos até agora no aprendizado
}

func ratioOf(ps []Pair, from, to time.Time) (float64, int) {
	var g, c float64
	n := 0
	for _, p := range ps {
		if p.T.Before(from) || !p.T.Before(to) {
			continue
		}
		g += p.Counts
		c += p.CPS * 60
		n++
	}
	if g <= 0 {
		return 0, n
	}
	return c / g, n
}

// WatchState calcula a vigia com os minutos em que os dois estavam em casa
// desde a calibração em uso (since).
func WatchState(ps []Pair, since, now time.Time) Watch {
	w := Watch{Since: since, LearnUntil: since.Add(LearnDays * 24 * time.Hour)}
	if now.Before(w.LearnUntil) {
		_, n := ratioOf(ps, since, now)
		w.LearnedMin = n
		w.State = WatchLearning
		if n == 0 {
			w.State = WatchNoData
		}
		return w
	}
	w.Normal, w.NormalMin = ratioOf(ps, since, w.LearnUntil)
	w.LearnedMin = w.NormalMin
	if w.NormalMin < learnMinMin || w.Normal <= 0 {
		w.State = WatchNoData
		return w
	}
	week := func(to time.Time) Week {
		from := to.Add(-7 * 24 * time.Hour)
		r, n := ratioOf(ps, from, to)
		wk := Week{From: from, Minutes: n}
		if n >= WeekMinutes && r > 0 {
			wk.Ratio = r
			wk.Pct = (r/w.Normal - 1) * 100
		}
		return wk
	}
	w.Last = week(now)
	w.Prev = week(now.Add(-7 * 24 * time.Hour))
	if w.Last.Ratio == 0 {
		w.State = WatchNoData
		return w
	}
	off := func(wk Week) bool { return wk.Ratio > 0 && math.Abs(wk.Pct) > DriftPct }
	switch {
	case off(w.Last) && off(w.Prev) && w.Prev.From.After(w.LearnUntil.Add(-time.Minute)):
		w.State = WatchDrift
	case off(w.Last):
		w.State = WatchOneWeek
	default:
		w.State = WatchAgree
	}
	return w
}
