package app

import (
	"sync"
	"time"
)

// ---------- dose do Radiacode pelo próprio aparelho (2.2) ----------
//
// O app soma os minutos que gravou; o aparelho soma o tempo todo, conectado
// ou não. Quando o registro dos totais do aparelho cobre o começo do período,
// a dose "do aparelho" é a diferença entre os totais — inclui o que ele
// contou longe do PC. Senão fica só a conta do app, marcada como tal.

// rcDeviceDose é a dose que o aparelho somou em [from, to). upTo é até onde
// o registro vai (o último total recebido, ou agora se conectado).
func (a *App) rcDeviceDose(from, to time.Time) (dose float64, upTo time.Time, ok bool) {
	raw := a.rcDoseLog(from.AddDate(0, 0, -7), to)
	a.rc.mu.Lock()
	if d := a.rc.dose; d.has && a.rc.st.State == "conectado" {
		now := time.Now()
		if now.Before(to) && (len(raw) == 0 || now.After(raw[len(raw)-1].T)) {
			raw = append(raw, rcDosePt{now, d.base + d.acc, d.baseSec + now.Sub(d.baseAt).Seconds()})
		}
	}
	a.rc.mu.Unlock()
	// precisa de um total de antes (ou logo depois) do começo do período
	if len(raw) < 2 || raw[0].T.After(from.Add(10*time.Minute)) || !raw[len(raw)-1].T.After(from) {
		return 0, time.Time{}, false
	}
	curve := rcDoseCurve(raw)
	upTo = raw[len(raw)-1].T
	if upTo.After(to) {
		upTo = to
	}
	dose = curveAt(curve, upTo) - curveAt(curve, from)
	if dose < 0 {
		dose = 0
	}
	return dose, upTo, true
}

// rcDevToday guarda a dose de hoje do aparelho, para não reler o registro a
// cada segundo: entre uma leitura e outra soma o que o total ao vivo andou.
type rcDevToday struct {
	mu   sync.Mutex
	day  time.Time
	at   time.Time
	dose float64
	ok   bool
	live float64 // total ao vivo na hora da leitura (0 = desconectado)
}

func (a *App) rcLiveTotal() float64 {
	a.rc.mu.Lock()
	defer a.rc.mu.Unlock()
	if d := a.rc.dose; d.has && a.rc.st.State == "conectado" {
		return d.base + d.acc
	}
	return 0
}

// rcTodayDevice devolve a dose de hoje pelo aparelho. Não pode ser chamada
// com rc.mu travado.
func (a *App) rcTodayDevice(now time.Time) (float64, bool) {
	c := &a.rcDevT
	c.mu.Lock()
	defer c.mu.Unlock()
	y, m, d := now.Date()
	day := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	live := a.rcLiveTotal()
	if c.day.Equal(day) && now.Sub(c.at) < time.Minute && (live == 0 || live >= c.live) {
		if live > 0 && c.live > 0 {
			return c.dose + live - c.live, c.ok
		}
		return c.dose, c.ok
	}
	dose, _, ok := a.rcDeviceDose(day, now.Add(time.Second))
	c.day, c.at, c.dose, c.ok, c.live = day, now, dose, ok, live
	return dose, ok
}
