package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// ---------- dose acumulada do próprio Radiacode ----------
//
// O aparelho soma a dose o tempo todo (desde que foi zerado), conectado ou
// não, e manda esse total a cada ~4 min (RareData). Entre um envio e outro o
// app soma o µSv/h ao vivo por cima do último total, para o número andar a
// cada segundo; no envio seguinte volta a valer o do aparelho. O último total
// fica gravado para aparecer mesmo com o aparelho desconectado.

const rcDoseFile = "dose.json"

// rcDoseLive é o total do aparelho e o que veio depois (protegido por rc.mu).
type rcDoseLive struct {
	has     bool
	base    float64   // µSv do último RareData
	baseSec float64   // s de contagem nesse momento
	baseAt  time.Time // quando chegou
	acc     float64   // µSv somados ao vivo depois dele
	lastRT  time.Time
	loaded  bool // já leu o dose.json
	saved   rcDoseSaved
}

type rcDoseSaved struct {
	Dose float64 `json:"dose"` // µSv
	Sec  float64 `json:"sec"`  // s de contagem
	At   int64   `json:"at"`   // ms
	// estimado pelo app ao fechar/cair a conexão (não é um total do aparelho)
	Est bool `json:"est,omitempty"`
}

// rcDoseRareLocked recebe o total do aparelho (precisa de rc.mu).
func (a *App) rcDoseRareLocked(dose, sec float64, now time.Time) {
	d := &a.rc.dose
	d.has, d.base, d.baseSec, d.baseAt, d.acc = true, dose, sec, now, 0
	d.saved = rcDoseSaved{Dose: dose, Sec: sec, At: now.UnixMilli()}
	d.loaded = true
	a.rcDoseWriteLocked()
}

// rcDoseKeepGap é até quanto tempo depois de a conexão cair (ou o app fechar
// para atualizar) o total estimado ainda vale ao reconectar: o aparelho só
// manda o total a cada vários minutos, e sem isso a dose do aparelho ficava
// "desconectada" até o próximo envio, mesmo conectado.
const rcDoseKeepGap = 3 * time.Minute

// rcDoseSaveLiveLocked grava o total estimado de agora (o último do aparelho
// mais o que foi somado ao vivo) quando a conexão termina (precisa de rc.mu).
func (a *App) rcDoseSaveLiveLocked(now time.Time) {
	d := &a.rc.dose
	if !d.has {
		return
	}
	d.saved = rcDoseSaved{Dose: d.base + d.acc, Sec: d.baseSec + now.Sub(d.baseAt).Seconds(), At: now.UnixMilli(), Est: true}
	a.rcDoseWriteLocked()
}

// rcDoseResumeLocked, ao conectar: se a conexão anterior terminou há pouco,
// continua do total estimado dela; senão espera o próximo total do aparelho
// (precisa de rc.mu).
func (a *App) rcDoseResumeLocked(now time.Time) {
	d := &a.rc.dose
	d.has, d.lastRT = false, time.Time{}
	if !d.loaded && a.dataDir != "" {
		d.loaded = true
		a.readStateJSON(filepath.Join(a.rcDir(), rcDoseFile), "dose do RC102", &d.saved)
	}
	at := time.UnixMilli(d.saved.At)
	if d.saved.At > 0 && !at.After(now) && now.Sub(at) <= rcDoseKeepGap {
		gap := now.Sub(at).Seconds()
		d.has, d.base, d.baseSec, d.baseAt, d.acc = true, d.saved.Dose, d.saved.Sec+gap, now, 0
	}
}

func (a *App) rcDoseWriteLocked() {
	d := &a.rc.dose
	if a.dataDir == "" {
		return
	}
	if b, err := json.Marshal(d.saved); err == nil {
		dir := a.rcDir()
		_ = os.MkdirAll(dir, 0o755)
		_ = writeFileAtomic(filepath.Join(dir, rcDoseFile), b, 0o644)
	}
}

// rcDoseRTLocked soma a leitura ao vivo (precisa de rc.mu).
func (a *App) rcDoseRTLocked(usvh float64, now time.Time) {
	d := &a.rc.dose
	if d.has && !d.lastRT.IsZero() {
		dt := now.Sub(d.lastRT)
		if dt > 5*time.Second { // buraco: não inventa dose (o próximo total corrige)
			dt = 0
		}
		if dt > 0 {
			d.acc += usvh * dt.Hours()
		}
	}
	d.lastRT = now
}

// rcDoseFillLocked põe no estado a dose acumulada: ao vivo se conectado,
// senão a última gravada (precisa de rc.mu).
func (a *App) rcDoseFillLocked(st *RcState, connected bool) {
	d := &a.rc.dose
	if !d.loaded && a.dataDir != "" {
		d.loaded = true
		a.readStateJSON(filepath.Join(a.rcDir(), rcDoseFile), "dose do RC102", &d.saved)
	}
	switch {
	case connected && d.has:
		st.DoseLive = d.base + d.acc
		st.DoseLiveSec = d.baseSec + time.Since(d.baseAt).Seconds()
		st.DoseLiveAt, st.DoseLiveOn = time.Now().UnixMilli(), true
	case d.saved.At > 0:
		st.DoseLive, st.DoseLiveSec, st.DoseLiveAt = d.saved.Dose, d.saved.Sec, d.saved.At
	}
}
