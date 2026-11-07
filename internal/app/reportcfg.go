package app

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"time"
)

// ---------- relatório: configurações que valeram no período (2.6) ----------

// cfgPeriodRow é um ajuste no relatório: o valor no início do período e o
// que mudou dentro dele.
type cfgPeriodRow struct {
	Field   string
	AtStart string
	Known   bool // havia registro antes do início do período
	Changes []ConfigChange
}

// ordem em que os ajustes aparecem (os outros vêm depois, em ordem alfabética)
var cfgFieldOrder = []string{"Aviso de atenção", "Alarme", "Alarme de perigo", "Tempo morto", "Calibração em uso",
	"Calibração do GMC", "Saturação do GMC", "Gravação na memória do GMC"}

// cfgInPeriod monta as linhas a partir do histórico de configurações.
// first é o primeiro registro do histórico (zero se não houver).
func cfgInPeriod(hist []ConfigChange, from, to time.Time) (rows []cfgPeriodRow, first time.Time) {
	f, t := from.UnixMilli(), to.UnixMilli()
	byField := map[string]*cfgPeriodRow{}
	var fields []string
	for _, c := range hist {
		if first.IsZero() || c.T < first.UnixMilli() {
			first = time.UnixMilli(c.T)
		}
		r := byField[c.Field]
		if r == nil {
			r = &cfgPeriodRow{Field: c.Field}
			byField[c.Field] = r
			fields = append(fields, c.Field)
		}
		switch {
		case c.T <= f:
			r.AtStart, r.Known = c.To, true
		case c.T < t:
			r.Changes = append(r.Changes, c)
		}
	}
	rank := map[string]int{}
	for i, k := range cfgFieldOrder {
		rank[k] = i + 1
	}
	sort.SliceStable(fields, func(i, j int) bool {
		ri, rj := rank[fields[i]], rank[fields[j]]
		if ri == 0 {
			ri = 1000
		}
		if rj == 0 {
			rj = 1000
		}
		if ri != rj {
			return ri < rj
		}
		return fields[i] < fields[j]
	})
	for _, k := range fields {
		r := byField[k]
		if !r.Known && len(r.Changes) == 0 {
			continue // só registrado depois do período
		}
		rows = append(rows, *r)
	}
	return rows, first
}

var cfgCPMRe = regexp.MustCompile(`a partir de (\d+) CPM`)

// cfgValueAt é o valor de um ajuste em t (ms), pelo histórico.
func cfgValueAt(hist []ConfigChange, field string, t int64) (string, bool) {
	v, ok := "", false
	for _, c := range hist {
		if c.Field == field && c.T <= t {
			v, ok = c.To, true
		}
	}
	return v, ok
}

// cfgLimitAt é o limite em CPM de um nível em t, pelo histórico (ok=false
// sem registro).
func cfgLimitAt(hist []ConfigChange, level string, t int64) (int, bool) {
	field := map[string]string{"perigo": "Alarme de perigo", "alarme": "Alarme"}[level]
	if field == "" {
		field = "Aviso de atenção"
	}
	v, ok := cfgValueAt(hist, field, t)
	if !ok {
		return 0, false
	}
	m := cfgCPMRe.FindStringSubmatch(v)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	return n, err == nil
}

// cfgChangeTxt descreve uma mudança para a tabela do relatório.
func cfgChangeTxt(c ConfigChange) string {
	when := time.UnixMilli(c.T).Format("02/01 15:04")
	src := ""
	if c.Source != "" {
		src = " (" + c.Source + ")"
	}
	if c.From == "" {
		return fmt.Sprintf("registrado em %s como %s%s", when, c.To, src)
	}
	return fmt.Sprintf("alterado em %s de %s para %s%s", when, c.From, c.To, src)
}

// maturityAt é a maturidade do histórico com as janelas terminando em end.
func (a *App) maturityAt(end time.Time) Maturity {
	var m Maturity
	gf := a.gmcFirst()
	var gt []time.Time
	if a.store != nil {
		pts, _ := a.store.Range(end.AddDate(0, 0, -365), end.Add(time.Minute))
		for _, p := range pts {
			if !p.T.After(end) {
				gt = append(gt, p.T)
			}
		}
	}
	if len(gt) > 0 && (gf.IsZero() || gt[0].Before(gf)) {
		gf = gt[0]
	}
	m.GMC = matWins(gt, gf, end)
	if !gf.IsZero() {
		m.GMCFirst = gf.UnixMilli()
	}
	if rf := a.rcFirst(); !rf.IsZero() && rf.Before(end) {
		m.RCFirst = rf.UnixMilli()
		var rt []time.Time
		for _, p := range a.rcRange(end.AddDate(0, 0, -365), end.Add(time.Minute)) {
			if !p.T.After(end) {
				rt = append(rt, p.T)
			}
		}
		m.RC = matWins(rt, rf, end)
	}
	return m
}
