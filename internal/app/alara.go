package app

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// ---------- ALARA (2.43) ----------
//
// "Tão baixo quanto razoavelmente possível": a tela ALARA mostra de onde vem
// a sua dose e deixa testar com os aparelhos os três jeitos de baixar:
// tempo (conta na tela), distância (o inverso do quadrado em 4 pontos) e
// blindagem (antes × depois). Os testes usam o motor do Medir objeto com o
// fundo da casa: cada distância (ou cada fase da blindagem) é uma fase do
// objeto, e as anteriores ficam em Prev (MeasureAgainAt).

// relErrMinSec é o mínimo de segundos antes de parar pela incerteza.
const relErrMinSec = 20

func alaraTest(t string) string {
	switch t {
	case "distancia", "blindagem":
		return t
	}
	return ""
}

// DistPoint é uma distância do teste.
type DistPoint struct {
	CM      float64 `json:"cm"`
	NetUSv  float64 `json:"netUSv"`
	ErrUSv  float64 `json:"errUSv"`
	Predict float64 `json:"predict"` // k/r²
	Seconds int     `json:"seconds"`
	Lost    bool    `json:"lost,omitempty"` // sumiu no fundo (< 2σ)
}

// DistFit é o ajuste k/r² (peso 1/σ²) dos pontos do teste de distância.
type DistFit struct {
	Points  []DistPoint `json:"points"`
	K       float64     `json:"k"`       // µSv/h·cm²
	Chi2    float64     `json:"chi2"`    // χ²/gl
	Verdict string      `json:"verdict"` // seguiu, devagar, torto
	Title   string      `json:"title"`
	Text    string      `json:"text"`
}

func distFit(in deriveIn, k float64) *DistFit {
	var pts []DistPoint
	for _, p := range in.prev {
		if p.DistCM > 0 && p.ErrUSv > 0 {
			pts = append(pts, DistPoint{CM: p.DistCM, NetUSv: p.NetUSv, ErrUSv: p.ErrUSv, Seconds: p.Seconds})
		}
	}
	if e := in.r.NetErr * k; in.distCM > 0 && e > 0 {
		pts = append(pts, DistPoint{CM: in.distCM, NetUSv: in.r.NetUSv, ErrUSv: e, Seconds: in.obj.Seconds})
	}
	f := &DistFit{Points: pts}
	if len(pts) < 2 {
		return f
	}
	sort.SliceStable(f.Points, func(i, j int) bool { return f.Points[i].CM < f.Points[j].CM })
	var sxy, sxx float64
	for _, p := range f.Points {
		x, w := 1/(p.CM*p.CM), 1/(p.ErrUSv*p.ErrUSv)
		sxy += w * x * p.NetUSv
		sxx += w * x * x
	}
	f.K = sxy / sxx
	chi := 0.0
	for i := range f.Points {
		p := &f.Points[i]
		p.Predict = f.K / (p.CM * p.CM)
		p.Lost = p.NetUSv < 2*p.ErrUSv
		chi += math.Pow((p.NetUSv-p.Predict)/p.ErrUSv, 2)
	}
	f.Chi2 = chi / float64(len(f.Points)-1)
	slow := len(f.Points) >= 3
	for _, p := range f.Points[min(2, len(f.Points)):] {
		if p.NetUSv <= p.Predict+p.ErrUSv {
			slow = false
		}
	}
	switch {
	case f.K <= 0:
		f.Verdict, f.Title = "torto", "Não deu para ver a fonte."
		f.Text = "Nas distâncias medidas a taxa não ficou acima do fundo. Aproxime a fonte ou meça mais tempo."
	case f.Chi2 <= 2:
		f.Verdict, f.Title = "seguiu", "Seguiu o inverso do quadrado."
		f.Text = "Cada vez que a distância dobrou, a taxa acima do fundo caiu para cerca de ¼. A fonte se comporta como pequena: afastar um palmo já corta muito."
	case slow:
		f.Verdict, f.Title = "devagar", "Caiu mais devagar que o previsto."
		f.Text = "Longe, a taxa ficou acima da curva. A fonte deve ser grande ou espalhada (piso, bancada, várias peças). Perto de uma fonte assim, afastar ajuda menos. Bem longe dela, a regra do quadrado volta a valer."
	default:
		f.Verdict, f.Title = "torto", "Não bateu bem com a curva."
		f.Text = "Pode ter sido a posição entre uma medida e outra. Vale repetir com o aparelho sempre na mesma linha, medindo do centro do cristal."
	}
	var lost []string
	for _, p := range f.Points {
		if p.Lost {
			lost = append(lost, fmtBR(p.CM, 0)+" cm")
		}
	}
	if len(lost) > 0 {
		f.Text += fmt.Sprintf(" A %s a fonte quase sumiu no fundo: esse ponto pesa pouco na conta.", strings.Join(lost, " e "))
	}
	return f
}

// ShieldCut é o resultado da blindagem: quanto o material cortou.
type ShieldCut struct {
	Material   string  `json:"material"`
	BeforeUSv  float64 `json:"beforeUSv"`
	BeforeErr  float64 `json:"beforeErr"`
	AfterUSv   float64 `json:"afterUSv"`
	AfterErr   float64 `json:"afterErr"`
	Cut        float64 `json:"cut"`    // 1 − depois/antes
	CutErr     float64 `json:"cutErr"` // σ (Poisson propagado)
	Seen       bool    `json:"seen"`   // corte ≥ 2σ
	Title      string  `json:"title"`
	Text       string  `json:"text"`
	BeforeSecs int     `json:"beforeSecs"`
	AfterSecs  int     `json:"afterSecs"`
}

func shieldCut(in deriveIn, k float64) *ShieldCut {
	if len(in.prev) == 0 {
		return nil
	}
	a := in.prev[len(in.prev)-1]
	s := &ShieldCut{BeforeUSv: a.NetUSv, BeforeErr: a.ErrUSv, AfterUSv: in.r.NetUSv, AfterErr: in.r.NetErr * k,
		BeforeSecs: a.Seconds, AfterSecs: in.obj.Seconds}
	if s.BeforeUSv <= 2*s.BeforeErr || s.BeforeUSv <= 0 {
		s.Title = "Sem nada no meio, a fonte não ficou acima do fundo."
		s.Text = "Assim não dá para medir o corte. Aproxime a fonte do aparelho ou meça mais tempo em cada fase."
		return s
	}
	q := s.AfterUSv / s.BeforeUSv
	s.Cut = 1 - q
	rel := s.BeforeErr / s.BeforeUSv
	if s.AfterUSv > 0 {
		s.CutErr = math.Abs(q) * math.Hypot(s.AfterErr/s.AfterUSv, rel)
	} else {
		s.CutErr = math.Hypot(s.AfterErr/s.BeforeUSv, rel)
	}
	s.Seen = math.Abs(s.Cut) >= 2*s.CutErr
	pct := func(v float64) string { return fmtBR(math.Round(v*100), 0) + "%" }
	switch {
	case !s.Seen:
		s.Title = "Não deu para ver diferença."
		s.Text = fmt.Sprintf("A diferença (%s ± %s) ficou menor que duas vezes a incerteza. Meça mais tempo em cada fase, ou o material corta pouco essa radiação.", pct(s.Cut), pct(s.CutErr))
	case s.Cut > 0:
		s.Title = fmt.Sprintf("Cortou %s ± %s.", pct(s.Cut), pct(s.CutErr))
		s.Text = fmt.Sprintf("Com o material no meio, a taxa acima do fundo caiu de %s para %s µSv/h, na mesma posição.", usvTxt(s.BeforeUSv), usvTxt(math.Max(s.AfterUSv, 0)))
	default:
		s.Title = "Subiu com o material no meio."
		s.Text = "Confira se o aparelho ficou no mesmo lugar nas duas fases: mudar a distância muda mais que o material."
	}
	return s
}

// ---------- de onde vem a sua dose ----------

// DoseOriginItem é um trecho em que a dose ficou acima do fundo.
type DoseOriginItem struct {
	From  int64   `json:"from"`
	To    int64   `json:"to"`
	Dose  float64 `json:"dose"` // µSv acima do fundo
	Label string  `json:"label"`
	Kind  string  `json:"kind"` // medicao, alerta, fora, nota, ""
}

// DoseOrigin é a dose pessoal de um período separada por origem.
type DoseOrigin struct {
	From       int64            `json:"from"`
	To         int64            `json:"to"`
	Device     string           `json:"device"` // quem mais mediu (radiacode, gmc, misto)
	Total      float64          `json:"total"`  // µSv
	HomeBg     float64          `json:"homeBg"` // fundo em casa
	AwayBg     float64          `json:"awayBg"` // fundo fora de casa
	Above      float64          `json:"above"`  // acima do fundo
	Minutes    int              `json:"minutes"`
	Items      []DoseOriginItem `json:"items"`
	HomeBgRate float64          `json:"homeBgRate"` // µSv/h do fundo do aparelho principal
}

// aboveFrac: um pedaço de 10 min só conta como "acima do fundo" com a
// média 20% acima (o mesmo +20% do Medir objeto); abaixo disso é flutuação.
const aboveFrac = 0.2

// GetDoseOrigin separa a dose pessoal dos últimos dias em fundo em casa,
// fundo fora de casa e acima do fundo, com os trechos acima e o que havia
// neles (medição de objeto, alerta, fora de casa, anotação).
func (a *App) GetDoseOrigin(days int) (DoseOrigin, error) {
	if days <= 0 || days > 31 {
		days = 7
	}
	now := time.Now()
	from := now.Add(-time.Duration(days) * 24 * time.Hour)
	out := DoseOrigin{From: from.UnixMilli(), To: now.UnixMilli(), Items: []DoseOriginItem{}}
	ds, err := a.GetDayStrip(from.UnixMilli(), now.UnixMilli())
	if err != nil {
		return out, err
	}
	devMin := map[string]int{}
	type run struct {
		from, to int64
		dose     float64
	}
	var runs []run
	var cur *run
	for _, s := range ds.Slots {
		if s.persN == 0 {
			cur = nil
			continue
		}
		dose := s.PersUSv * float64(s.persN) / 60
		out.Total += dose
		out.Minutes += s.persN
		devMin[s.PersDev] += s.persN
		bg := s.persBg
		base := dose
		if bg > 0 && s.PersUSv > bg*(1+aboveFrac) {
			base = bg * float64(s.persN) / 60
			ex := dose - base
			out.Above += ex
			if cur != nil && cur.to == s.T {
				cur.to, cur.dose = s.T+ds.StepMs, cur.dose+ex
			} else {
				runs = append(runs, run{s.T, s.T + ds.StepMs, ex})
				cur = &runs[len(runs)-1]
			}
		} else {
			cur = nil
		}
		if s.Away {
			out.AwayBg += base
		} else {
			out.HomeBg += base
		}
	}
	switch {
	case devMin[devRC] > 0 && devMin["gmc"] > 0:
		out.Device = "misto"
	case devMin[devRC] > 0:
		out.Device = devRC
	case devMin["gmc"] > 0:
		out.Device = "gmc"
	}
	// o que havia em cada trecho
	a.mu.Lock()
	meas := append([]SavedMeasure(nil), a.measurements...)
	periods := append([]Period(nil), a.periods...)
	a.mu.Unlock()
	alerts := a.GetEvents(kindAlert)
	notes := a.GetNotes(out.From, out.To)
	for _, r := range runs {
		it := DoseOriginItem{From: r.from, To: r.to, Dose: r.dose}
		over := func(t0, t1 int64) bool { return t0 < r.to && t1 > r.from }
		for _, m := range meas {
			if over(m.Time, max(m.Obj.End, m.Time+int64(m.Obj.Seconds)*1000)) {
				it.Label, it.Kind = "Medir objeto · "+m.Name, "medicao"
				break
			}
		}
		if it.Kind == "" {
			for _, e := range alerts {
				end := e.End
				if end == 0 {
					end = e.Time + 60e3
				}
				if over(e.Time, end) {
					dev := "GMC-800"
					if e.Device == devRC {
						dev = "RC102"
					}
					it.Label, it.Kind = levelName(e.Level)+" no "+dev, "alerta"
					break
				}
			}
		}
		if it.Kind == "" {
			for _, p := range periods {
				if over(p.From, periodEnd(p)) {
					it.Label, it.Kind = "Fora de casa", "fora"
					if p.Name != "" {
						it.Label += " · " + p.Name
					}
					break
				}
			}
		}
		if it.Kind == "" {
			for _, n := range notes {
				if over(n.Time, max(n.End, n.Time+60e3)) {
					it.Label, it.Kind = "Anotação · "+n.Text, "nota"
					break
				}
			}
		}
		if it.Kind == "" {
			it.Label = "Acima do normal"
		}
		out.Items = append(out.Items, it)
	}
	sort.SliceStable(out.Items, func(i, j int) bool { return out.Items[i].Dose > out.Items[j].Dose })
	if len(out.Items) > 12 {
		out.Items = out.Items[:12]
	}
	out.HomeBgRate = a.homeBgRate(out.Device)
	return out, nil
}

// homeBgRate é o fundo em µSv/h do aparelho (o mesmo da dose).
func (a *App) homeBgRate(dev string) float64 {
	if dev != "gmc" {
		dev = devRC
	}
	now := time.Now()
	_, bg, _, err := a.doseSeries(dev, now.Add(-time.Hour), now)
	if err != nil {
		return 0
	}
	return bg
}
