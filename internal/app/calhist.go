package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"radlog/internal/gmc"
	"radlog/internal/store"
)

// ---------- versões da calibração do GMC (2.2) ----------
//
// Cada mudança de calibração vira uma versão com a data em que passou a
// valer. O µSv/h e a dose de cada minuto saem da versão que valia naquele
// minuto: trocar a calibração hoje não muda o que já foi medido.
//
// O recálculo retroativo é opcional e manual: escolhe uma versão para valer
// também antes da data dela. Os minutos gravados (CPM) nunca mudam e as
// versões continuam lá; desligar o recálculo volta ao histórico original.

const calHistFile = "calibracao-versoes.json"

// CalVersion é uma versão da calibração.
type CalVersion struct {
	ID     int64          `json:"id"`
	Since  int64          `json:"desde"` // ms; 0 = desde o começo
	Points []gmc.CalPoint `json:"pontos"`
	// Factor é o µSv/h por CPM no fundo informado à mão (ex.: comparando
	// com o RadiaCode); a curva do contador é escalada para ele. 0 = a
	// curva do contador como ela é.
	Factor float64 `json:"fator,omitempty"`
	Source string  `json:"origem"` // "contador", "você", "início"
	Note   string  `json:"nota,omitempty"`
}

type calHistData struct {
	Versions []CalVersion `json:"versoes"`
	Retro    int64        `json:"retroativa,omitempty"` // versão aplicada também antes da data dela
}

// Cal é a calibração efetiva da versão.
func (v CalVersion) Cal() gmc.Calibration {
	src := v.Source
	if v.Factor <= 0 {
		return gmc.Calibration{Points: v.Points, Source: src}
	}
	if len(v.Points) == 0 || v.Points[0].CPM <= 0 || v.Points[0].USv <= 0 {
		return gmc.Calibration{Points: []gmc.CalPoint{{CPM: 1000, USv: 1000 * v.Factor}}, Source: src}
	}
	k := v.Factor / (v.Points[0].USv / v.Points[0].CPM)
	pts := make([]gmc.CalPoint, len(v.Points))
	for i, p := range v.Points {
		pts[i] = gmc.CalPoint{CPM: p.CPM, USv: p.USv * k}
	}
	return gmc.Calibration{Points: pts, Source: src}
}

// K é o µSv/h por CPM no fundo.
func (v CalVersion) K() float64 {
	c := v.Cal()
	if len(c.Points) == 0 {
		return 0
	}
	return c.Points[0].USv / c.Points[0].CPM
}

func samePoints(a, b []gmc.CalPoint) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if math.Abs(a[i].CPM-b[i].CPM) > 0.5 || math.Abs(a[i].USv-b[i].USv) > 1e-6*math.Max(1, a[i].USv) {
			return false
		}
	}
	return true
}

// calTL converte cada minuto pela versão que valia nele.
type calTL struct {
	vers  []CalVersion // por Since (e ID)
	cals  []gmc.Calibration
	retro int // índice da versão retroativa, -1 = nenhuma
	dtNow bool
}

func newCalTL(h calHistData, dtNow bool) calTL {
	vs := append([]CalVersion(nil), h.Versions...)
	sort.SliceStable(vs, func(i, j int) bool {
		if vs[i].Since != vs[j].Since {
			return vs[i].Since < vs[j].Since
		}
		return vs[i].ID < vs[j].ID
	})
	tl := calTL{vers: vs, retro: -1, dtNow: dtNow}
	for i, v := range vs {
		tl.cals = append(tl.cals, v.Cal())
		if h.Retro != 0 && v.ID == h.Retro {
			tl.retro = i
		}
	}
	return tl
}

// idx é a versão que vale em t (ms).
func (tl calTL) idx(t int64) int {
	if len(tl.vers) == 0 {
		return -1
	}
	if tl.retro >= 0 && t < tl.vers[tl.retro].Since {
		return tl.retro
	}
	i := sort.Search(len(tl.vers), func(i int) bool { return tl.vers[i].Since > t }) - 1
	if i < 0 {
		i = 0
	}
	return i
}

func (tl calTL) calAt(t int64, dtOn bool) gmc.Calibration {
	i := tl.idx(t)
	if i < 0 {
		return gmc.DefaultCalibration()
	}
	c := tl.cals[i]
	if dtOn && c.CompensatesDeadTime() {
		return c.Linear()
	}
	return c
}

// USv converte o minuto gravado. Os minutos de antes da 2.1 não têm a marca
// da correção de tempo morto; ela nunca foi ligada antes disso, então são
// brutos e usam a curva como ela é.
func (tl calTL) USv(p store.Point) float64 {
	return tl.calAt(p.T.UnixMilli(), p.Has && p.DTOn).USv(p.CPM)
}

// Tag identifica a calibração usada no minuto (exportação):
// "v2:0.00550", com ":manual" se o fator foi informado à mão e
// ":recalculado" se é a versão retroativa valendo antes da data dela.
func (tl calTL) Tag(p store.Point) string {
	t := p.T.UnixMilli()
	i := tl.idx(t)
	if i < 0 {
		return ""
	}
	v := tl.vers[i]
	s := fmt.Sprintf("v%d:%.5f", v.ID, v.K())
	if v.Factor > 0 {
		s += ":manual"
	}
	if i == tl.retro && t < v.Since {
		s += ":recalculado"
	}
	return s
}

// USvAt converte um CPM sem minuto gravado (pico por segundo, média) pela
// versão de t e o estado atual da correção.
func (tl calTL) USvAt(t time.Time, cpm float64) float64 {
	return tl.calAt(t.UnixMilli(), tl.dtNow).USv(cpm)
}

// withRetro devolve a linha do tempo com a versão id aplicada também antes
// da data dela (id 0 = sem recálculo).
func (tl calTL) withRetro(id int64) calTL {
	out := tl
	out.retro = -1
	for i, v := range tl.vers {
		if id != 0 && v.ID == id {
			out.retro = i
		}
	}
	return out
}

func (a *App) calHistPath() string { return filepath.Join(a.dataDir, calHistFile) }

// calTLLocked é a linha do tempo da calibração. (a.mu travado)
func (a *App) calTLLocked() calTL {
	h := a.calHist
	if len(h.Versions) == 0 {
		h.Versions = []CalVersion{{ID: 1, Points: a.cal.Points, Source: a.cal.Source}}
	}
	return newCalTL(h, a.settings.Levels.DeadTimeOn)
}

func (a *App) saveCalHistLocked() {
	if a.dataDir == "" {
		return
	}
	b, err := json.MarshalIndent(a.calHist, "", "  ")
	if err != nil {
		return
	}
	tmp := a.calHistPath() + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		_ = os.Rename(tmp, a.calHistPath())
	}
}

// loadCalHist lê as versões; na primeira vez, a calibração atual (a última
// lida do contador) vale desde o começo — é a que o app já usava.
func (a *App) loadCalHist() {
	var h calHistData
	if b, err := os.ReadFile(a.calHistPath()); err == nil {
		_ = json.Unmarshal(b, &h)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(h.Versions) == 0 {
		h = calHistData{Versions: []CalVersion{{ID: 1, Points: a.cal.Points, Source: "início"}}}
		if a.cal.Source == "padrão" {
			h.Versions[0].Source = "padrão"
		}
		a.calHist = h
		a.saveCalHistLocked()
		return
	}
	a.calHist = h
}

func (a *App) calLastLocked() *CalVersion {
	vs := a.calHist.Versions
	if len(vs) == 0 {
		return nil
	}
	best := 0
	for i, v := range vs {
		if v.Since > vs[best].Since || (v.Since == vs[best].Since && v.ID > vs[best].ID) {
			best = i
		}
	}
	return &vs[best]
}

func (a *App) calNextIDLocked() int64 {
	var max int64
	for _, v := range a.calHist.Versions {
		if v.ID > max {
			max = v.ID
		}
	}
	return max + 1
}

// calHistDevice registra a calibração lida do contador: se a curva mudou,
// vira uma versão nova a partir de agora. O fator manual, se houver, é
// mantido (ele vem da comparação com outro aparelho, não da curva).
func (a *App) calHistDevice(c gmc.Calibration) {
	if a.dataDir == "" || len(c.Points) == 0 {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	last := a.calLastLocked()
	switch {
	case last == nil:
		a.calHist.Versions = []CalVersion{{ID: 1, Points: c.Points, Source: "contador"}}
	case samePoints(last.Points, c.Points):
		return
	case len(a.calHist.Versions) == 1 && last.Source == "padrão":
		// nunca tinha lido o contador: a de fábrica era só um palpite
		last.Points, last.Source = c.Points, "contador"
	default:
		v := CalVersion{ID: a.calNextIDLocked(), Since: time.Now().UnixMilli(), Points: c.Points,
			Factor: last.Factor, Source: "contador", Note: "a curva do contador mudou"}
		if v.Factor > 0 {
			v.Note += "; o fator manual foi mantido"
		}
		a.calHist.Versions = append(a.calHist.Versions, v)
	}
	a.saveCalHistLocked()
}

// ---------- o que a tela usa ----------

// CalVersionView é uma versão para a tela.
type CalVersionView struct {
	CalVersion
	Until   int64   `json:"ate"` // 0 = em uso
	K       float64 `json:"k"`   // µSv/h por CPM no fundo
	DevK    float64 `json:"devK"`
	Current bool    `json:"atual"`
	Minutes int     `json:"minutos"` // minutos gravados enquanto valia
}

// CalHistory é o que a tela de calibração mostra.
type CalHistory struct {
	Versions []CalVersionView `json:"versions"` // da mais nova para a mais velha
	Retro    int64            `json:"retro"`
	DevK     float64          `json:"devK"` // do contador, sem fator
}

// GetCalHistory devolve as versões da calibração do GMC.
func (a *App) GetCalHistory() CalHistory {
	a.mu.Lock()
	tl := a.calTLLocked()
	retro := a.calHist.Retro
	devK := 0.0
	if len(a.cal.Points) > 0 {
		devK = a.cal.Points[0].USv / a.cal.Points[0].CPM
	}
	a.mu.Unlock()
	out := CalHistory{Retro: retro, DevK: devK, Versions: []CalVersionView{}}
	pts, _ := a.store.Range(time.Now().AddDate(-5, 0, 0), time.Now().Add(time.Minute))
	for i, v := range tl.vers {
		vv := CalVersionView{CalVersion: v, K: v.K(), Current: i == len(tl.vers)-1}
		if len(v.Points) > 0 {
			vv.DevK = v.Points[0].USv / v.Points[0].CPM
		}
		if i+1 < len(tl.vers) {
			vv.Until = tl.vers[i+1].Since
		}
		for _, p := range pts {
			t := p.T.UnixMilli()
			if t >= v.Since && (vv.Until == 0 || t < vv.Until) {
				vv.Minutes++
			}
		}
		out.Versions = append(out.Versions, vv)
	}
	for i, j := 0, len(out.Versions)-1; i < j; i, j = i+1, j-1 {
		out.Versions[i], out.Versions[j] = out.Versions[j], out.Versions[i]
	}
	return out
}

// SetGMCFactor cria uma versão nova, valendo a partir de agora, com o µSv/h
// por CPM no fundo informado (0 = volta à curva do contador).
func (a *App) SetGMCFactor(k float64, note string) error {
	if k != 0 && (k < 0.0005 || k > 0.1) {
		return errors.New("fator fora da faixa (0,0005 a 0,1 µSv/h por CPM)")
	}
	a.mu.Lock()
	last := a.calLastLocked()
	pts := a.cal.Points
	if last != nil {
		pts = last.Points
		if math.Abs(last.Factor-k) < 1e-9 {
			a.mu.Unlock()
			return errors.New("esse já é o fator em uso")
		}
	}
	v := CalVersion{ID: a.calNextIDLocked(), Since: time.Now().UnixMilli(), Points: pts, Factor: k, Source: "você", Note: note}
	a.calHist.Versions = append(a.calHist.Versions, v)
	a.saveCalHistLocked()
	a.mu.Unlock()
	if k == 0 {
		a.logEvent("Calibração do GMC: voltou à curva do contador (vale a partir de agora)")
	} else {
		a.logEvent(fmt.Sprintf("Calibração do GMC: %s µSv/h por CPM no fundo, informado à mão (vale a partir de agora)", fmtBR(k, 5)))
	}
	a.cfgRecord("você")
	a.emitCalChanged()
	return nil
}

// SetCalRetro aplica a versão id também antes da data dela (0 = desliga e
// volta ao histórico original). Nada gravado muda.
func (a *App) SetCalRetro(id int64) error {
	a.mu.Lock()
	if id != 0 {
		ok := false
		for _, v := range a.calHist.Versions {
			ok = ok || v.ID == id
		}
		if !ok {
			a.mu.Unlock()
			return errors.New("versão não encontrada")
		}
	}
	a.calHist.Retro = id
	a.saveCalHistLocked()
	a.mu.Unlock()
	if id == 0 {
		a.logEvent("Calibração do GMC: recálculo retroativo desligado (histórico original)")
	} else {
		a.logEvent(fmt.Sprintf("Calibração do GMC: histórico recalculado com a versão %d (os minutos gravados não mudam)", id))
	}
	a.cfgRecord("você")
	a.emitCalChanged()
	return nil
}

func (a *App) emitCalChanged() {
	a.mu.Lock()
	st := a.statusLocked()
	a.mu.Unlock()
	a.emit("status", st)
	a.emit("calibration", nil)
}

// CalCompare compara o histórico original com o recalculado por uma versão.
type CalCompare struct {
	ID        int64   `json:"id"`
	From      int64   `json:"from"`
	To        int64   `json:"to"`
	Minutes   int     `json:"minutes"`
	DoseOrig  float64 `json:"doseOrig"` // µSv
	DoseNew   float64 `json:"doseNew"`
	AvgOrig   float64 `json:"avgOrig"` // µSv/h
	AvgNew    float64 `json:"avgNew"`
	MaxOrig   float64 `json:"maxOrig"`
	MaxNew    float64 `json:"maxNew"`
	ChangePct float64 `json:"changePct"`
}

// CompareCal mostra quanto muda o histórico de antes da versão id se ela
// for aplicada também a ele (até um ano para trás). Não muda nada.
func (a *App) CompareCal(id int64) (CalCompare, error) {
	a.mu.Lock()
	tl := a.calTLLocked().withRetro(0)
	a.mu.Unlock()
	var v *CalVersion
	for i := range tl.vers {
		if tl.vers[i].ID == id {
			v = &tl.vers[i]
		}
	}
	if v == nil {
		return CalCompare{}, errors.New("versão não encontrada")
	}
	to := time.UnixMilli(v.Since)
	if v.Since == 0 {
		return CalCompare{ID: id}, nil
	}
	from := to.AddDate(-1, 0, 0)
	pts, err := a.store.Range(from, to)
	if err != nil {
		return CalCompare{}, err
	}
	nt := tl.withRetro(id)
	c := CalCompare{ID: id, From: from.UnixMilli(), To: to.UnixMilli(), Minutes: len(pts)}
	for _, p := range pts {
		o, n := tl.USv(p), nt.USv(p)
		c.DoseOrig += o / 60
		c.DoseNew += n / 60
		c.MaxOrig = math.Max(c.MaxOrig, o)
		c.MaxNew = math.Max(c.MaxNew, n)
	}
	if len(pts) > 0 {
		c.From = pts[0].T.UnixMilli()
		c.AvgOrig = c.DoseOrig * 60 / float64(len(pts))
		c.AvgNew = c.DoseNew * 60 / float64(len(pts))
	}
	if c.DoseOrig > 0 {
		c.ChangePct = (c.DoseNew/c.DoseOrig - 1) * 100
	}
	return c, nil
}

// calInUseDesc descreve a calibração em uso, para o histórico de
// configurações. (a.mu travado)
func (a *App) calInUseDesc() string {
	last := a.calLastLocked()
	if last == nil {
		return calDesc(a.cal)
	}
	s := fmt.Sprintf("%s µSv/h por CPM no fundo", fmtBR(last.K(), 5))
	if last.Factor > 0 {
		s += " (informado à mão)"
	} else {
		s += " (curva do contador)"
	}
	if a.calHist.Retro != 0 {
		s += fmt.Sprintf(" · histórico recalculado com a versão %d", a.calHist.Retro)
	}
	return s
}

// calMerge junta as versões de um backup: entra o que não existe aqui (mesma
// data e mesma curva = mesma versão).
func (a *App) calMerge(b []byte) {
	var h calHistData
	if json.Unmarshal(b, &h) != nil || len(h.Versions) == 0 {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	changed := false
	for _, v := range h.Versions {
		dup := false
		for _, w := range a.calHist.Versions {
			if w.Since == v.Since && w.Factor == v.Factor && samePoints(w.Points, v.Points) {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		if v.Since == 0 {
			// o começo: se aqui é só o palpite de fábrica (este PC nunca leu o
			// contador) e lá veio a curva lida do contador, fica a de lá
			for i := range a.calHist.Versions {
				w := &a.calHist.Versions[i]
				if w.Since == 0 && w.Source == "padrão" && v.Source != "padrão" && len(v.Points) > 0 {
					w.Points, w.Source, w.Factor, w.Note = v.Points, v.Source, v.Factor, v.Note
					changed = true
				}
			}
			continue
		}
		v.ID = a.calNextIDLocked()
		a.calHist.Versions = append(a.calHist.Versions, v)
		changed = true
	}
	if changed {
		a.saveCalHistLocked()
	}
}

// calSourceLocked diz de onde vem a calibração em uso. (a.mu travado)
func (a *App) calSourceLocked() string {
	if last := a.calLastLocked(); last != nil && last.Factor > 0 {
		return "informado à mão"
	}
	return a.cal.Source
}

// periodDesc descreve as calibrações que valeram em [from, to) (relatório).
func (tl calTL) periodDesc(from, to int64) string {
	var parts []string
	seen := map[int]bool{}
	add := func(i int, since, until int64) {
		if i < 0 || seen[i] {
			return
		}
		seen[i] = true
		v := tl.vers[i]
		src := "curva do contador"
		if v.Factor > 0 {
			src = "informado à mão"
		}
		s := fmt.Sprintf("%s µSv/h por CPM no fundo (versão %d, %s", fmtBR(v.K(), 5), v.ID, src)
		if i == tl.retro && since < v.Since {
			s += ", recalculando o histórico de antes dela"
		}
		s += ")"
		if since > from {
			s = "desde " + time.UnixMilli(since).Format("02/01/2006 15:04") + ": " + s
		} else if until < to {
			s = "até " + time.UnixMilli(until).Format("02/01/2006 15:04") + ": " + s
		}
		parts = append(parts, s)
	}
	// pontos de troca dentro do período
	cuts := []int64{from}
	for _, v := range tl.vers {
		if v.Since > from && v.Since < to {
			cuts = append(cuts, v.Since)
		}
	}
	cuts = append(cuts, to)
	for k := 0; k+1 < len(cuts); k++ {
		add(tl.idx(cuts[k]), cuts[k], cuts[k+1])
	}
	if len(parts) == 0 {
		return "—"
	}
	if len(parts) > 1 {
		return "mudou no período · " + strings.Join(parts, " · ")
	}
	return parts[0]
}
