package app

import (
	"math"
	"time"
)

// gmcMinute junta as leituras de 1 s do GMC-800 num valor por minuto.
//
// O CPM que o GMC-800 manda (GETCPM) não é a soma dos últimos 60 s: é uma
// estimativa rápida que sobe e desce em poucos segundos. Gravar só o último
// valor do minuto perdia os picos curtos (dados reais de 14/11: às 14h11 o
// minuto ficou com 101 CPM, e a média segundo a segundo foi 793). O valor do
// minuto passa a ser a média dos CPS × 60, as contagens de verdade de cada
// segundo; com poucas leituras no minuto, fica o último CPM, como antes.
type gmcMinute struct {
	minute      time.Time // minuto das amostras
	lastCPM     uint32
	lastRaw     uint32
	lastLv      Levels
	lastAt      time.Time
	cps, rawCPS float64 // somas do minuto
	n           int
}

// gmcMinuteMinN é o mínimo de leituras de 1 s para usar a média dos CPS.
const gmcMinuteMinN = 10

// gmcMinutePoint é o minuto pronto para gravar.
type gmcMinutePoint struct {
	T        time.Time
	CPM, Raw float64
	Lv       Levels
}

// add entra com uma leitura; quando o minuto vira, devolve o anterior.
func (g *gmcMinute) add(now time.Time, cpm, raw, cps, rawCPS uint32, lv Levels) (gmcMinutePoint, bool) {
	var out gmcMinutePoint
	done := false
	m := now.Truncate(time.Minute)
	if !g.minute.IsZero() && m.After(g.minute) {
		// grava só se leu até perto do fim do minuto
		if g.lastAt.Sub(g.minute) >= 50*time.Second {
			out = gmcMinutePoint{T: g.minute, CPM: float64(g.lastCPM), Raw: float64(g.lastRaw), Lv: g.lastLv}
			if g.n >= gmcMinuteMinN {
				out.CPM = math.Round(g.cps * 60 / float64(g.n))
				out.Raw = math.Round(g.rawCPS * 60 / float64(g.n))
			}
			done = true
		}
	}
	if !m.Equal(g.minute) {
		g.minute, g.cps, g.rawCPS, g.n = m, 0, 0, 0
	}
	g.lastCPM, g.lastRaw, g.lastAt, g.lastLv = cpm, raw, now, lv
	g.cps += float64(cps)
	g.rawCPS += float64(rawCPS)
	g.n++
	return out, done
}
