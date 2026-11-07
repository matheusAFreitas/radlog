package app

import (
	"math"
	"sort"
	"time"

	"radlog/internal/radiacode"
)

// ---------- modo busca (RadiaCode) ----------
//
// O aparelho manda a contagem em amostras curtas (as "amostras cruas", de
// ~0,5 s), junto com o resto, a cada leitura (1 vez por segundo). O modo
// busca guarda os últimos minutos dessas amostras e a tela pergunta "o que
// chegou desde a última vez" várias vezes por segundo.

const (
	rcSearchKeep  = 3 * time.Minute
	rcSearchRefTT = 5 * time.Minute // refaz a referência no máximo a cada 5 min
)

type rcSearchPt struct {
	T   int64   `json:"t"` // fim da amostra (ms)
	Sec float64 `json:"sec"`
	CPS float64 `json:"cps"`
	USv float64 `json:"usv"` // µSv/h
}

// rcSearchAdd guarda as amostras que chegaram (a hora vem do PC: a última
// termina agora, as outras antes dela).
func (a *App) rcSearchAdd(v radiacode.RawSamples, now time.Time) {
	p := v.Period.Seconds()
	if p <= 0 || len(v.CPS) == 0 {
		return
	}
	a.rc.mu.Lock()
	defer a.rc.mu.Unlock()
	end := now.UnixMilli()
	if n := len(a.rc.search); n > 0 && a.rc.search[n-1].T > end-int64(float64(len(v.CPS))*p*1000) {
		// leituras muito juntas: encaixa logo depois da última
		end = a.rc.search[n-1].T + int64(float64(len(v.CPS))*p*1000)
	}
	for i, c := range v.CPS {
		t := end - int64(float64(len(v.CPS)-1-i)*p*1000)
		u := 0.0
		if i < len(v.DoseRate) {
			u = v.DoseRate[i]
		}
		a.rc.search = append(a.rc.search, rcSearchPt{T: t, Sec: p, CPS: c, USv: u})
	}
	cut := now.Add(-rcSearchKeep).UnixMilli()
	i := 0
	for i < len(a.rc.search) && a.rc.search[i].T < cut {
		i++
	}
	if i > 0 {
		a.rc.search = append([]rcSearchPt(nil), a.rc.search[i:]...)
	}
}

// RcSearch é o que a tela do modo busca precisa.
type RcSearch struct {
	Live   bool         `json:"live"`
	Now    int64        `json:"now"`
	Pts    []rcSearchPt `json:"pts"`    // amostras curtas depois de sinceMs
	Raw    bool         `json:"raw"`    // o aparelho está mandando amostras curtas
	CPS    float64      `json:"cps"`    // a taxa ao vivo do aparelho
	USv    float64      `json:"usv"`    // µSv/h ao vivo do aparelho
	RefCPS float64      `json:"refCps"` // fundo de referência (mediana das últimas 24 h)
	RefUSv float64      `json:"refUsv"`
	BLE    bool         `json:"ble"`
}

// GetRcSearch devolve as amostras curtas depois de sinceMs (0 = os últimos
// 60 s) e a taxa ao vivo.
func (a *App) GetRcSearch(sinceMs int64) RcSearch {
	now := time.Now()
	out := RcSearch{Now: now.UnixMilli(), Pts: []rcSearchPt{}}
	if sinceMs <= 0 {
		sinceMs = now.Add(-time.Minute).UnixMilli()
	}
	a.rc.mu.Lock()
	out.Live = a.rc.dev != nil
	out.BLE = a.rc.st.Via == "bluetooth"
	if out.Live {
		out.CPS, out.USv = a.rc.st.CPS, a.rc.st.USvh
	}
	for _, p := range a.rc.search {
		if p.T > sinceMs {
			out.Pts = append(out.Pts, p)
		}
	}
	out.Raw = len(a.rc.search) > 0 && a.rc.search[len(a.rc.search)-1].T > now.Add(-5*time.Second).UnixMilli()
	// sem amostras curtas (firmware antigo): usa a taxa ao vivo de cada segundo
	if !out.Raw {
		out.Pts = out.Pts[:0]
		for _, s := range a.rc.ring {
			if t := s.t.UnixMilli(); t > sinceMs {
				out.Pts = append(out.Pts, rcSearchPt{T: t, Sec: 1, CPS: s.cps, USv: s.usv})
			}
		}
	}
	ref, refAt := a.rc.searchRef, a.rc.searchRefAt
	a.rc.mu.Unlock()
	if now.Sub(refAt) > rcSearchRefTT {
		ref = a.rcSearchReference(now)
		a.rc.mu.Lock()
		a.rc.searchRef, a.rc.searchRefAt = ref, now
		a.rc.mu.Unlock()
	}
	out.RefCPS, out.RefUSv = ref[0], ref[1]
	return out
}

// rcSearchReference é a mediana dos minutos completos das últimas 24 h, fora
// dos alertas: o "normal" de onde o aparelho costuma ficar.
func (a *App) rcSearchReference(now time.Time) [2]float64 {
	pts := a.rcRange(now.Add(-24*time.Hour), now.Add(time.Minute))
	inAlert := a.alertChecker()
	var cps, usv []float64
	for _, p := range pts {
		if p.N < 20 || inAlert(p.T) || p.CPS <= 0 {
			continue
		}
		cps, usv = append(cps, p.CPS), append(usv, p.USvh)
	}
	if len(cps) < 10 {
		return [2]float64{}
	}
	sort.Float64s(cps)
	sort.Float64s(usv)
	r := func(v float64, d float64) float64 { return math.Round(v*d) / d }
	return [2]float64{r(cps[len(cps)/2], 1000), r(usv[len(usv)/2], 1e4)}
}
