package app

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// ---------- medir objeto: o que sai das contas (2.40) ----------
//
// Depois que a medição termina, o app junta o que dá para tirar dela:
//   - isótopos prováveis: as linhas do espectro líquido agrupadas pela fonte,
//     com uma confiança pelo número de linhas que bateram e pela força delas;
//   - distância segura: onde o extra do objeto cai para 10% do fundo, pelo
//     inverso do quadrado (x = d·√(líquido ÷ 0,1·fundo));
//   - atividade (Bq): só como ordem de grandeza e só quando dá (RC102 no
//     objeto, isótopo identificado com constante conhecida, distância
//     informada, objeto pequeno). A ≈ Ḣ·d² ÷ Γ;
//   - com duas distâncias: se a radiação caiu mesmo pelo inverso do quadrado.

// MeasureAt é uma medição anterior do mesmo objeto, em outra distância.
type MeasureAt struct {
	Distance string  `json:"distance"`
	DistCM   float64 `json:"distCm"`
	NetUSv   float64 `json:"netUSv"`
	ErrUSv   float64 `json:"errUSv"`
	Sigma    float64 `json:"sigma"`
	Level    string  `json:"level"`
	Seconds  int     `json:"seconds"`
}

// IsoGuess é uma fonte provável, com as linhas que bateram.
type IsoGuess struct {
	Group    string    `json:"group"`
	Nuclides []string  `json:"nuclides"`
	KeV      []float64 `json:"kev"`
	Lines    int       `json:"lines"` // linhas que bateram
	Of       int       `json:"of"`    // linhas procuradas dentro da faixa do aparelho
	Conf     float64   `json:"conf"`  // 0 a 1
}

// BqCheck é uma condição para estimar o Bq.
type BqCheck struct {
	Text string `json:"text"`
	OK   bool   `json:"ok"`
}

// BqEstimate é a atividade estimada (ordem de grandeza).
type BqEstimate struct {
	OK      bool      `json:"ok"`
	Bq      float64   `json:"bq"` // valor central
	Lo      float64   `json:"lo"` // ÷2
	Hi      float64   `json:"hi"` // ×2
	Group   string    `json:"group,omitempty"`
	Gamma   float64   `json:"gamma,omitempty"` // µSv·m²/h por MBq
	Checks  []BqCheck `json:"checks"`
	Missing string    `json:"missing,omitempty"` // a primeira condição que faltou
}

// SafeDist é onde o extra do objeto cai para 10% do fundo.
type SafeDist struct {
	OK     bool    `json:"ok"`
	CM     float64 `json:"cm"`
	Below  bool    `json:"below,omitempty"`  // já na distância medida fica abaixo
	Approx bool    `json:"approx,omitempty"` // objeto espalhado ou só uma distância
	Why    string  `json:"why,omitempty"`    // por que não deu (OK = false)
}

// InvSq compara duas distâncias com o inverso do quadrado.
type InvSq struct {
	D1     float64 `json:"d1"` // cm (a anterior)
	D2     float64 `json:"d2"` // cm (a atual)
	N1     float64 `json:"n1"` // µSv/h líquido
	N2     float64 `json:"n2"`
	Ratio  float64 `json:"ratio"`  // n1 ÷ n2 medido
	Expect float64 `json:"expect"` // (d2 ÷ d1)²
	Power  float64 `json:"power"`  // expoente medido (2 = inverso do quadrado)
	ErrPow float64 `json:"errPow"`
	OK     bool    `json:"ok"`
	Text   string  `json:"text"`
}

// MeasureDerived junta as contas da medição para a tela e o relatório.
type MeasureDerived struct {
	NetUSv   float64     `json:"netUSv"`
	ErrUSv   float64     `json:"errUSv"`
	BgUSv    float64     `json:"bgUSv"`
	Isotopes []IsoGuess  `json:"isotopes,omitempty"`
	Safe     SafeDist    `json:"safe"`
	Bq       *BqEstimate `json:"bq,omitempty"` // só com o RC102 no objeto
	InvSq    *InvSq      `json:"invSq,omitempty"`
	// 2.43: testes do ALARA (alara.go)
	Dist   *DistFit   `json:"dist,omitempty"`
	Shield *ShieldCut `json:"shield,omitempty"`
}

// isoGamma são as constantes de taxa de dose (equivalente de dose ambiente,
// µSv/h a 1 m por MBq) das fontes que o app reconhece. Para as séries, vale
// a série em equilíbrio. Urânio natural e U-235 ficam de fora: quase toda a
// dose deles é beta ou vem dos filhos, e a conta pela gama não fecha.
var isoGamma = map[string]float64{
	"Césio-137":                   0.089,
	"Cobalto-60":                  0.35,
	"Amerício-241":                0.003,
	"Potássio-40":                 0.021,
	"Rádio-226 (série do urânio)": 0.23,
	"Tório-232 (série)":           0.20,
}

const (
	safeFrac     = 0.10 // distância segura: extra = 10% do fundo
	isoConfOK    = 0.6  // "identificado com confiança"
	invSqTol     = 0.35 // folga mínima no expoente
	bqRangeTimes = 2.0
)

// isoGuesses agrupa os picos pela fonte e dá a confiança de cada uma.
func isoGuesses(sp *MeasureSpectrum) []IsoGuess {
	if sp == nil || len(sp.Peaks) == 0 || len(sp.Obj) == 0 {
		return nil
	}
	lo, hi := sp.energy(0), sp.energy(len(sp.Obj)-1)
	of := map[string]int{}
	for _, l := range specLines {
		h := 0.6 * specFWHM(l.kev)
		if l.kev-3*h >= lo && l.kev+3*h <= hi {
			of[l.group]++
		}
	}
	by := map[string]*IsoGuess{}
	maxSig := map[string]float64{}
	var order []string
	for _, p := range sp.Peaks {
		g := by[p.Group]
		if g == nil {
			g = &IsoGuess{Group: p.Group, Of: max(of[p.Group], 1)}
			by[p.Group] = g
			order = append(order, p.Group)
		}
		g.Lines++
		g.KeV = append(g.KeV, p.KeV)
		if !containsStr(g.Nuclides, p.Nuclide) {
			g.Nuclides = append(g.Nuclides, p.Nuclide)
		}
		maxSig[p.Group] = math.Max(maxSig[p.Group], p.Sigma)
	}
	out := make([]IsoGuess, 0, len(order))
	for _, k := range order {
		g := *by[k]
		sort.Float64s(g.KeV)
		frac := math.Min(1, float64(g.Lines)/float64(g.Of))
		// uma linha só, por mais forte, ainda pode ser outra fonte na mesma
		// energia; com várias linhas da mesma série, a chance cai muito
		strength := math.Min(1, (maxSig[k]-specPeakSigma)/(10-specPeakSigma))
		conf := 0.25 + 0.45*frac + 0.3*strength
		if g.Of > 1 && g.Lines == 1 {
			conf = math.Min(conf, 0.45)
		}
		g.Conf = math.Round(math.Min(conf, 0.95)*100) / 100
		out = append(out, g)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Conf > out[j].Conf })
	return out
}

// deriveIn é o que a conta precisa da medição.
type deriveIn struct {
	device string // quem ficou no objeto: gmc ou radiacode
	mode   string // "", ambos, voce
	bg     MeasurePhase
	obj    MeasurePhase
	r      MeasureResult
	spec   *MeasureSpectrum // do aparelho no objeto (nil = sem espectro)
	distCM float64          // 0 = encostado ou não informada
	size   string           // pequeno ou espalhado ("" = não disse)
	prev   []MeasureAt
	test   string // 2.43: distancia, blindagem ("" = medição comum)
}

// deriveMeasure faz as contas do resultado.
func deriveMeasure(in deriveIn) *MeasureDerived {
	k := 0.0
	if in.obj.CPM > 0 {
		k = in.obj.USv / in.obj.CPM
	} else if in.bg.CPM > 0 {
		k = in.bg.USv / in.bg.CPM
	}
	d := &MeasureDerived{NetUSv: in.r.NetUSv, ErrUSv: in.r.NetErr * k, BgUSv: in.bg.USv}
	rcObj := devName(in.device) == devRC
	if rcObj && in.spec != nil && in.spec.Role != "voce" {
		d.Isotopes = isoGuesses(in.spec)
	}
	real := in.r.Level == "sim" && in.r.NetUSv > 0
	d.InvSq = invSquare(in)
	switch in.test {
	case "distancia":
		d.Dist = distFit(in, k)
	case "blindagem":
		d.Shield = shieldCut(in, k)
	}
	// distância segura
	switch {
	case !real:
		d.Safe.Why = "o objeto não ficou claramente acima do fundo"
	case in.distCM <= 0:
		d.Safe.Why = "informe a distância (encostado não dá para usar na conta)"
	case in.bg.USv <= 0:
		d.Safe.Why = "sem o fundo em µSv/h"
	default:
		x := in.distCM * math.Sqrt(in.r.NetUSv/(safeFrac*in.bg.USv))
		d.Safe = SafeDist{OK: true, CM: x}
		if x < in.distCM {
			d.Safe.CM, d.Safe.Below = in.distCM, true
		}
		d.Safe.Approx = in.size != "pequeno" || d.InvSq == nil || !d.InvSq.OK
	}
	if !rcObj {
		return d
	}
	// atividade: só quando todas as condições batem
	bq := &BqEstimate{}
	var top *IsoGuess
	if len(d.Isotopes) > 0 {
		top = &d.Isotopes[0]
	}
	gamma := 0.0
	if top != nil {
		gamma = isoGamma[top.Group]
	}
	// com duas distâncias, se caiu bem menos que o inverso do quadrado, não
	// é uma fonte pontual, mesmo que tenha sido marcado como pequeno
	spread := d.InvSq != nil && !d.InvSq.OK && d.InvSq.Power < 2
	isoTxt := "isótopo identificado com confiança"
	if top != nil && top.Conf >= isoConfOK && gamma == 0 {
		isoTxt = "isótopo com constante de dose conhecida (" + top.Group + " não tem)"
	}
	bq.Checks = []BqCheck{
		{"medido com o RC102 no objeto (tem espectro)", in.spec != nil && len(in.spec.Obj) > 0},
		{"objeto claramente acima do fundo", real},
		{isoTxt, top != nil && top.Conf >= isoConfOK && gamma > 0},
		{"distância informada", in.distCM > 0},
		{"objeto pequeno, como uma fonte pontual", in.size == "pequeno" && !spread},
	}
	bq.OK = true
	for _, c := range bq.Checks {
		if !c.OK {
			bq.OK = false
			if bq.Missing == "" {
				bq.Missing = c.Text
			}
		}
	}
	if bq.OK {
		dm := in.distCM / 100
		mbq := in.r.NetUSv * dm * dm / gamma
		bq.Bq = mbq * 1e6
		bq.Lo, bq.Hi = bq.Bq/bqRangeTimes, bq.Bq*bqRangeTimes
		bq.Group, bq.Gamma = top.Group, gamma
	}
	d.Bq = bq
	return d
}

// invSquare compara a medição atual com a última anterior em outra
// distância, as duas claramente acima do fundo.
func invSquare(in deriveIn) *InvSq {
	if in.distCM <= 0 || in.r.Level != "sim" || in.r.NetUSv <= 0 {
		return nil
	}
	k := 0.0
	if in.obj.CPM > 0 {
		k = in.obj.USv / in.obj.CPM
	}
	e2 := in.r.NetErr * k
	for i := len(in.prev) - 1; i >= 0; i-- {
		p := in.prev[i]
		if p.DistCM <= 0 || p.Level != "sim" || p.NetUSv <= 0 || math.Abs(p.DistCM-in.distCM) < 0.5 {
			continue
		}
		q := &InvSq{D1: p.DistCM, D2: in.distCM, N1: p.NetUSv, N2: in.r.NetUSv}
		q.Ratio = q.N1 / q.N2
		q.Expect = math.Pow(q.D2/q.D1, 2)
		lr := math.Log(q.D2 / q.D1)
		q.Power = math.Log(q.Ratio) / lr
		q.ErrPow = math.Hypot(p.ErrUSv/p.NetUSv, e2/q.N2) / math.Abs(lr)
		tol := math.Max(invSqTol, 2*q.ErrPow)
		q.OK = math.Abs(q.Power-2) <= tol
		dd := func(cm float64) string { return fmtBR(cm, 0) + " cm" }
		switch {
		case q.OK:
			q.Text = fmt.Sprintf("De %s para %s a radiação caiu %s× (o inverso do quadrado dá %s×): o objeto se comporta como uma fonte pequena, e a distância segura e o Bq ficam mais confiáveis.",
				dd(q.D1), dd(q.D2), fmtBR(math.Max(q.Ratio, 1/q.Ratio), 1), fmtBR(math.Max(q.Expect, 1/q.Expect), 1))
		case q.Power < 2:
			q.Text = fmt.Sprintf("De %s para %s a radiação caiu menos que o inverso do quadrado (%s× contra %s×): o objeto deve ser espalhado, então a distância segura é só aproximada.",
				dd(q.D1), dd(q.D2), fmtBR(math.Max(q.Ratio, 1/q.Ratio), 1), fmtBR(math.Max(q.Expect, 1/q.Expect), 1))
		default:
			q.Text = fmt.Sprintf("De %s para %s a radiação caiu mais que o inverso do quadrado (%s× contra %s×): confira as distâncias; o detector fica alguns milímetros para dentro da caixa.",
				dd(q.D1), dd(q.D2), fmtBR(math.Max(q.Ratio, 1/q.Ratio), 1), fmtBR(math.Max(q.Expect, 1/q.Expect), 1))
		}
		return q
	}
	return nil
}

// distCMFrom lê os centímetros de um texto como "5 cm", "1 m" ou
// "encostado" (0). Devolve -1 se não entendeu.
func distCMFrom(s string) float64 {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return -1
	}
	if strings.HasPrefix(s, "encost") {
		return 0
	}
	var v float64
	var unit string
	if n, _ := fmt.Sscanf(strings.Replace(s, ",", ".", 1), "%f%s", &v, &unit); n == 0 || v < 0 {
		return -1
	}
	switch strings.TrimSpace(unit) {
	case "", "cm":
		return v
	case "mm":
		return v / 10
	case "m":
		return v * 100
	}
	return -1
}
