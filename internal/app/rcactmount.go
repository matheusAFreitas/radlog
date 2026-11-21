package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"
)

// ---------- atividade (Bq): calibração por montagem ----------
//
// 2.104.0 (avaliação do Bq, 21/11): a eficiência só vale para a montagem em
// que foi medida (mesmo pote, mesmo material, mesma altura, RC102 na mesma
// posição). Cada montagem guarda as rodadas de calibração e a eficiência é a
// soma delas (contagens com contagens), nunca "a melhor": escolher a que saiu
// melhor guardava a que saiu alta por sorte.

const actMountsFile = "montagens.json"

// ActRound é uma rodada de calibração com o KCl numa montagem.
type ActRound struct {
	T       int64   `json:"t"`
	Sec     float64 `json:"sec"`   // tempo com o KCl
	BgSec   float64 `json:"bgSec"` // tempo do fundo
	Area    float64 `json:"area"`  // contagens líquidas no pico do K-40
	AreaErr float64 `json:"areaErr"`
	ExpK    float64 `json:"expK"` // Bq de K-40 × rendimento × tempo (contagens se a eficiência fosse 1)
	PeakKeV float64 `json:"peakKeV"`
	Gain    float64 `json:"gain"`
	Found   bool    `json:"found"`
}

// ActMount é uma montagem calibrada.
type ActMount struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Setup    string     `json:"setup"`    // pote1l | becker | outra
	Material string     `json:"material"` // adubo | puro | light
	Pct      float64    `json:"pct"`      // % de KCl
	MassG    float64    `json:"massG"`
	BgMode   string     `json:"bgMode"` // branco (pote igual com sal comum) | vazio (sem nada, sal a 3 m)
	Rounds   []ActRound `json:"rounds"`
	Upd      int64      `json:"upd"` // última mudança do nome ou apagada (para juntar com a nuvem)
	Deleted  bool       `json:"deleted,omitempty"`
}

// ActMountView é a montagem com a conta feita, para a tela.
type ActMountView struct {
	ActMount
	Eff      float64 `json:"eff"`
	CountErr float64 `json:"countErr"` // só a contagem (relativa)
	FixErr   float64 `json:"fixErr"`   // rótulo do material, sombra do fundo
	TotErr   float64 `json:"totErr"`
	Sec      float64 `json:"sec"` // somado
	BgSec    float64 `json:"bgSec"`
	Usable   bool    `json:"usable"`
	PeakKeV  float64 `json:"peakKeV"` // da última rodada
	Gain     float64 `json:"gain"`
	Found    bool    `json:"found"`
	LastT    int64   `json:"lastT"`
}

const actMountMaxErr = 0.20 // acima disso (total), a montagem ainda não serve para medir

// actFixErr: o que não cai com o tempo. Rótulo do KCl de adubo ~±5%, KCl
// puro ~±3%, sal light ~±15% (o rótulo dá o potássio por porção); sem o
// branco de sal comum, o sal blinda o fundo e a eficiência sai 2–5% baixa.
func actFixErr(material, bgMode string) float64 {
	f := 0.06
	switch material {
	case "puro":
		f = 0.04
	case "light":
		f = 0.15
	}
	if bgMode == "vazio" {
		f = math.Hypot(f, 0.035)
	}
	return f
}

func (m ActMount) view() ActMountView {
	v := ActMountView{ActMount: m, FixErr: actFixErr(m.Material, m.BgMode)}
	var area, v2, exp float64
	for _, r := range m.Rounds {
		area += r.Area
		v2 += r.AreaErr * r.AreaErr
		exp += r.ExpK
		v.Sec += r.Sec
		v.BgSec += r.BgSec
		if r.T >= v.LastT {
			v.LastT, v.PeakKeV, v.Gain, v.Found = r.T, r.PeakKeV, r.Gain, r.Found
		}
	}
	if area > 0 && exp > 0 {
		v.Eff = area / exp
		v.CountErr = math.Sqrt(v2) / area
		v.TotErr = math.Hypot(v.CountErr, v.FixErr)
		v.Usable = v.TotErr <= actMountMaxErr
	}
	return v
}

// cal é a calibração no formato da conta das amostras (a forma da curva
// fica a padrão: a do eletrodo de tório está escondida até corrigir o
// tungstênio).
func (v ActMountView) cal() ActCal {
	if v.Eff <= 0 {
		return ActCal{}
	}
	return ActCal{Geometry: v.Name, KCl: &ActCalKCl{T: v.LastT, Eff: v.Eff, EffErr: v.TotErr, MassG: v.MassG, Pct: v.Pct, Sec: v.Sec, PeakKeV: v.PeakKeV, Gain: v.Gain}}
}

type actMountsDoc struct {
	Mounts []ActMount `json:"mounts"`
}

func (a *App) actMountsAll() []ActMount {
	var d actMountsDoc
	if b, err := os.ReadFile(a.actPath(actMountsFile)); err == nil {
		json.Unmarshal(b, &d)
	}
	return d.Mounts
}

func (a *App) actSaveMounts(ms []ActMount) {
	sort.SliceStable(ms, func(i, j int) bool { return ms[i].ID < ms[j].ID })
	a.actWriteJSON(actMountsFile, actMountsDoc{Mounts: ms})
}

// actMigrateLegacy: a calibração única feita depois da correção do pico
// (2.103.0, tem ganho) vira a primeira montagem, para não ter de refazer.
// O ID sai do horário dela: dois PCs migram para a mesma montagem.
func (a *App) actMigrateLegacy() {
	if _, err := os.Stat(a.actPath(actMountsFile)); err == nil {
		return
	}
	c := a.actCal()
	k := c.KCl
	if k == nil || k.Gain <= 0 || k.Eff <= 0 || k.Sec <= 0 {
		return
	}
	aK := k.MassG * k.Pct / 100 * actKPerKCl * actBqPerGK
	exp := aK * 0.1066 * k.Sec
	mat := "light"
	switch {
	case k.Pct >= 99.5:
		mat = "puro"
	case k.Pct >= 90:
		mat = "adubo"
	}
	name := strings.TrimSpace(c.Geometry) // vazio: a tela mostra o nome do tipo, no idioma dela
	m := ActMount{ID: fmt.Sprintf("kcl-%d", k.T), Name: name, Setup: "outra", Material: mat, Pct: k.Pct, MassG: k.MassG, BgMode: "vazio", Upd: k.T,
		Rounds: []ActRound{{T: k.T, Sec: k.Sec, BgSec: k.Sec, Area: k.Eff * exp, AreaErr: k.EffErr * k.Eff * exp, ExpK: exp, PeakKeV: k.PeakKeV, Gain: k.Gain, Found: k.PeakKeV > 0}}}
	a.actSaveMounts([]ActMount{m})
}

// actMountViews são as montagens (sem as apagadas), a usada por último primeiro.
func (a *App) actMountViews() []ActMountView {
	out := []ActMountView{}
	for _, m := range a.actMountsAll() {
		if !m.Deleted {
			out = append(out, m.view())
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].LastT > out[j].LastT })
	return out
}

func (a *App) actMount(id string) (ActMountView, bool) {
	for _, m := range a.actMountsAll() {
		if m.ID == id && !m.Deleted {
			return m.view(), true
		}
	}
	return ActMountView{}, false
}

func actNewID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// actAddRound põe a rodada na montagem (cria a montagem na primeira).
func (a *App) actAddRound(o ActOpts, r ActRound) ActMountView {
	ms := a.actMountsAll()
	for i := range ms {
		if ms[i].ID == o.MountID && !ms[i].Deleted {
			ms[i].Rounds = append(ms[i].Rounds, r)
			a.actSaveMounts(ms)
			return ms[i].view()
		}
	}
	m := ActMount{ID: o.MountID, Name: strings.TrimSpace(o.MountName), Setup: o.Setup, Material: o.Material, Pct: o.Pct, MassG: o.MassG, BgMode: o.BgMode,
		Rounds: []ActRound{r}, Upd: r.T}
	if m.ID == "" {
		m.ID = actNewID()
	}
	ms = append(ms, m)
	a.actSaveMounts(ms)
	return m.view()
}

// RenameActMount muda o nome de uma montagem.
func (a *App) RenameActMount(id, name string) error {
	a.rc.act.mu.Lock()
	defer a.rc.act.mu.Unlock()
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("escreva um nome")
	}
	ms := a.actMountsAll()
	for i := range ms {
		if ms[i].ID == id && !ms[i].Deleted {
			ms[i].Name, ms[i].Upd = name, time.Now().UnixMilli()
			a.actSaveMounts(ms)
			return nil
		}
	}
	return errors.New("montagem não encontrada")
}

// DeleteActMount apaga uma montagem (fica marcada, para a nuvem não trazer de volta).
func (a *App) DeleteActMount(id string) error {
	a.rc.act.mu.Lock()
	defer a.rc.act.mu.Unlock()
	ms := a.actMountsAll()
	for i := range ms {
		if ms[i].ID == id && !ms[i].Deleted {
			ms[i].Deleted, ms[i].Upd, ms[i].Rounds = true, time.Now().UnixMilli(), nil
			a.actSaveMounts(ms)
			return nil
		}
	}
	return errors.New("montagem não encontrada")
}

// actMergeMounts junta as montagens de outro PC: as rodadas somam (sem
// repetir o horário); nome e "apagada" ficam com a mudança mais nova.
func actMergeMounts(cur, in []ActMount) []ActMount {
	idx := map[string]int{}
	out := append([]ActMount(nil), cur...)
	for i, m := range out {
		idx[m.ID] = i
	}
	for _, m := range in {
		i, ok := idx[m.ID]
		if !ok {
			idx[m.ID] = len(out)
			out = append(out, m)
			continue
		}
		c := &out[i]
		if m.Upd > c.Upd {
			c.Name, c.Deleted, c.Upd = m.Name, m.Deleted, m.Upd
		}
		if c.Deleted {
			c.Rounds = nil
			continue
		}
		have := map[int64]bool{}
		for _, r := range c.Rounds {
			have[r.T] = true
		}
		for _, r := range m.Rounds {
			if !have[r.T] {
				c.Rounds = append(c.Rounds, r)
				have[r.T] = true
			}
		}
		sort.Slice(c.Rounds, func(x, y int) bool { return c.Rounds[x].T < c.Rounds[y].T })
	}
	return out
}

// ---------- conferências e previsão da calibração rodando ----------

// ActCalCheck é o que a tela mostra com a calibração rodando.
type ActCalCheck struct {
	BandRise  float64 `json:"bandRise"`  // subida de 300 a 1.400 keV (0,49 = +49%), desde 10 min
	SignalCpm float64 `json:"signalCpm"` // contagens/min no pico do K-40, desde 30 min
	ExpectLo  float64 `json:"expectLo"`  // o que a subida da faixa diz que o pico deve dar
	ExpectHi  float64 `json:"expectHi"`
	Forecast  float64 `json:"forecast"`            // ± da contagem no fim do tempo marcado
	Best      float64 `json:"best"`                // o melhor possível com este fundo
	Ready     bool    `json:"ready"`               // já tem 30 min: mostra pico, sinal e previsão
	SignalErr float64 `json:"signalErr,omitempty"` // ± relativo do sinal (QA: entra antes de dizer "fora")
	// QA 2.104.0: a montagem no fim desta rodada (as rodadas que já tem mais
	// esta): o que decide se fica pronta (total ≤ 20%)
	SumCount float64 `json:"sumCount,omitempty"`
	SumTotal float64 `json:"sumTotal,omitempty"`
}

// sum soma a previsão desta rodada às rodadas que a montagem já tem.
func (c *ActCalCheck) sum(prev *ActMountView, S, tgt, fix float64) {
	if S <= 0 {
		return
	}
	if tgt <= 0 {
		tgt = 1
	}
	A := S * tgt
	V := math.Pow(c.Forecast*A, 2)
	if prev != nil {
		for _, r := range prev.Rounds {
			A += r.Area
			V += r.AreaErr * r.AreaErr
		}
	}
	if A <= 0 {
		return
	}
	c.SumCount = math.Sqrt(V) / A
	c.SumTotal = math.Hypot(c.SumCount, fix)
}

const (
	actCheckBandSec = 600  // a faixa já diz algo com 10 min
	actCheckPeakSec = 1800 // o pico, o sinal e a previsão, com 30 min
)

// actCalCheck faz as conferências: a faixa de 300 a 1.400 keV sobe com o
// KCl em volta do aparelho (o espalhamento do K-40), e o pico do K-40 deve
// dar ~1/9 dessa subida; a previsão usa o sinal real (não a massa):
// rel² = (S+B)/(S²·Ts) + B/(S²·Tb), com B tirado da incerteza de agora.
func actCalCheck(smpSec, tgt, bgSec, smpCps, bgCps float64, k *ActCalKCl, area float64) *ActCalCheck {
	if smpSec < actCheckBandSec || bgCps <= 0 {
		return nil
	}
	c := &ActCalCheck{BandRise: smpCps/bgCps - 1}
	if smpSec < actCheckPeakSec {
		return c
	}
	c.Ready = true
	rise := (smpCps - bgCps) * 60 / 9
	if rise > 0 {
		c.ExpectLo, c.ExpectHi = rise*0.5, rise*2
	}
	if k == nil || area <= 0 || k.EffErr <= 0 || bgSec <= 0 {
		return c
	}
	S := area / smpSec
	c.SignalCpm = S * 60
	c.SignalErr = k.EffErr
	rel2 := k.EffErr * k.EffErr
	B := (rel2*S*S - S/smpSec) / (1/smpSec + 1/bgSec)
	if B < 0 {
		B = 0
	}
	if tgt <= 0 {
		tgt = smpSec
	}
	c.Forecast = math.Sqrt((S+B)/(S*S*tgt) + B/(S*S*bgSec))
	c.Best = math.Sqrt(B / (S * S * bgSec))
	return c
}
