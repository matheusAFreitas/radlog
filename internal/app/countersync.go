package app

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"radlog/internal/gmc"
)

// ---------- sincronização contador × app e memória do GMC (2.7) ----------
//
// O app guarda quando baixou a memória do contador pela última vez e até
// onde ela ia. Daí sai um estado claro: em dia, pendente (há trechos sem
// leitura no app depois do último download que a memória do contador pode
// cobrir), sem gravação (o contador não está gravando) ou nunca baixado.
// O uso da memória é estimado a partir do último download e do modo de
// gravação, com aviso quando passa de 80% e de 95%.

const counterSyncFile = "contador-sincronizacao.json"

type counterSyncData struct {
	Serial   string `json:"serial"`
	At       int64  `json:"baixadoEm"`
	Complete bool   `json:"completo"` // leu a memória até o fim
	Records  int    `json:"registros"`
	Added    int    `json:"minutosNovos"`
	Alerts   int    `json:"alertas"`
	MemFirst int64  `json:"memoriaDesde"` // primeiro e último registro da memória (ms)
	MemLast  int64  `json:"memoriaAte"`
	SaveMode string `json:"modoGravacao"` // último modo conhecido
	Warned   string `json:"avisado"`      // nível de memória já avisado desde o último download
}

// CounterSync é o estado para a tela.
type CounterSync struct {
	Known    bool   `json:"known"`
	At       int64  `json:"at"`
	Complete bool   `json:"complete"`
	Records  int    `json:"records"`
	Added    int    `json:"added"`
	MemFirst int64  `json:"memFirst"`
	MemLast  int64  `json:"memLast"`
	SaveMode string `json:"saveMode"`
	SaveOn   bool   `json:"saveOn"`

	// memória (estimada desde o último download)
	UsedBytes  int     `json:"usedBytes"`
	TotalBytes int     `json:"totalBytes"`
	UsedPct    float64 `json:"usedPct"` // no último download
	EstPct     float64 `json:"estPct"`  // estimado agora
	FullIn     int64   `json:"fullIn"`  // segundos até encher no modo atual (0 = sem estimativa)
	MemLevel   string  `json:"memLevel"`

	// "nunca", "em-dia", "pendente", "sem-gravacao"
	State          string `json:"state"`
	PendingMinutes int    `json:"pendingMinutes"`
	PendingFrom    int64  `json:"pendingFrom"`
	Text           string `json:"text"`
}

func (a *App) counterSyncPath() string { return filepath.Join(a.dataDir, counterSyncFile) }

func (a *App) readCounterSync() (counterSyncData, bool) {
	var d counterSyncData
	b, err := os.ReadFile(a.counterSyncPath())
	if err != nil || json.Unmarshal(b, &d) != nil {
		return d, false
	}
	return d, true
}

func (a *App) writeCounterSync(d counterSyncData) {
	if a.dataDir == "" {
		return
	}
	b, _ := json.MarshalIndent(d, "", "  ")
	_ = writeFileAtomic(a.counterSyncPath(), b, 0o644)
}

// recordCounterSync guarda o resultado de um download da memória.
func (a *App) recordCounterSync(h gmc.History, res ImportResult, complete bool) {
	a.mu.Lock()
	d := counterSyncData{Serial: a.info.Serial, At: time.Now().UnixMilli(), Complete: complete,
		Records: res.Records, Added: res.Added, Alerts: res.Alerts, SaveMode: a.devSaveMode}
	a.mu.Unlock()
	for _, r := range h.Records {
		t := r.Time.UnixMilli()
		if d.MemFirst == 0 || t < d.MemFirst {
			d.MemFirst = t
		}
		if t > d.MemLast {
			d.MemLast = t
		}
	}
	if old, ok := a.readCounterSync(); ok && d.SaveMode == "" {
		d.SaveMode = old.SaveMode
	}
	a.writeCounterSync(d)
}

// saveModeSecPerByte: segundos por byte gravado em cada modo (≈ 1 byte por
// registro; nos modos por limiar não dá para saber).
func saveModeSecPerByte(mode string) float64 {
	switch mode {
	case "a cada segundo":
		return 1
	case "a cada minuto":
		return 60
	case "a cada hora":
		return 3600
	}
	return 0
}

func memLevelOf(pct float64) string {
	switch {
	case pct >= 95:
		return "cheia"
	case pct >= 80:
		return "atencao"
	}
	return ""
}

// GetCounterSync devolve o estado da sincronização e da memória do GMC.
func (a *App) GetCounterSync() CounterSync {
	return a.counterSyncAt(time.Now())
}

func (a *App) counterSyncAt(now time.Time) CounterSync {
	d, ok := a.readCounterSync()
	a.mu.Lock()
	mode := a.devSaveMode
	serial := a.info.Serial
	a.mu.Unlock()
	if mode == "" {
		mode = d.SaveMode
	}
	cs := CounterSync{SaveMode: mode, SaveOn: mode != "" && mode != "desligada", TotalBytes: gmc.FlashSize}
	if ok && serial != "" && d.Serial != "" && serialKey(d.Serial) != serialKey(serial) {
		ok = false // é de outro contador
	}
	mi := a.GetMemoryInfo()
	if mi.Known {
		cs.UsedBytes = mi.UsedBytes
		cs.UsedPct = float64(mi.UsedBytes) / float64(gmc.FlashSize) * 100
		est := float64(mi.UsedBytes)
		if spb := saveModeSecPerByte(mode); spb > 0 {
			est += now.Sub(time.UnixMilli(mi.At)).Seconds() / spb
			cs.FullIn = int64(math.Max(0, (float64(gmc.FlashSize)-est)*spb))
		}
		cs.EstPct = math.Min(100, est/float64(gmc.FlashSize)*100)
		cs.MemLevel = memLevelOf(cs.EstPct)
	}
	if !ok {
		cs.State = "nunca"
		cs.Text = "A memória do contador ainda não foi baixada por este app."
		if !cs.SaveOn && mode != "" {
			cs.Text += " O contador não está gravando na memória (gravação desligada)."
		}
		return cs
	}
	cs.Known, cs.At, cs.Complete, cs.Records, cs.Added = true, d.At, d.Complete, d.Records, d.Added
	cs.MemFirst, cs.MemLast = d.MemFirst, d.MemLast

	// trechos sem leitura no app (≥ 10 min) depois do fim da memória baixada
	from := time.UnixMilli(d.MemLast)
	if d.MemLast == 0 {
		from = time.UnixMilli(d.At)
	}
	if now.Sub(from) > 10*time.Minute && a.store != nil {
		pts, _ := a.store.Range(from, now)
		for _, g := range findGaps(pts, from.Add(time.Minute), now.Add(-2*time.Minute)) {
			if m := g.Minutes(); m >= 10 {
				if cs.PendingFrom == 0 {
					cs.PendingFrom = g.From.UnixMilli()
				}
				cs.PendingMinutes += m
			}
		}
	}
	when := time.UnixMilli(d.At).Format("02/01 15:04")
	switch {
	case !cs.SaveOn && mode != "":
		cs.State = "sem-gravacao"
		cs.Text = fmt.Sprintf("Último download em %s. O contador não está gravando na memória: minutos sem o app não podem ser recuperados.", when)
	case cs.PendingMinutes > 0:
		cs.State = "pendente"
		cs.Text = fmt.Sprintf("Último download em %s. Depois dele, o app ficou %s sem leitura (desde %s); a memória do contador pode ter esses minutos: baixe o histórico.",
			when, fmtMinutes(cs.PendingMinutes), time.UnixMilli(cs.PendingFrom).Format("02/01 15:04"))
	default:
		cs.State = "em-dia"
		cs.Text = fmt.Sprintf("Em dia: último download em %s e nenhum trecho sem leitura no app desde então.", when)
	}
	if !d.Complete {
		cs.Text += " O último download foi interrompido antes do fim da memória."
	}
	return cs
}

// checkMemoryWarning avisa uma vez por nível (80% e 95%) desde o último
// download. Chamado de tempos em tempos.
func (a *App) checkMemoryWarning() {
	d, ok := a.readCounterSync()
	if !ok {
		return
	}
	cs := a.GetCounterSync()
	if cs.MemLevel == "" || cs.MemLevel == d.Warned || (d.Warned == "cheia" && cs.MemLevel == "atencao") {
		return
	}
	d.Warned = cs.MemLevel
	a.writeCounterSync(d)
	msg := fmt.Sprintf("Memória do GMC perto de encher: cerca de %s%% ocupada (estimado desde o download de %s).",
		fmtBR(cs.EstPct, 0), time.UnixMilli(cs.At).Format("02/01"))
	if cs.MemLevel == "cheia" {
		msg = fmt.Sprintf("Memória do GMC praticamente cheia: cerca de %s%% ocupada (estimado). Dados novos podem não ser gravados ou podem sobrescrever os antigos.", fmtBR(cs.EstPct, 0))
	}
	a.logEvent(msg + " Baixe o histórico para guardar tudo no app.")
	a.emit("countersync", cs)
}

// memoryWatch confere a memória do GMC a cada 30 min.
func (a *App) memoryWatch() {
	time.Sleep(2 * time.Minute)
	t := time.NewTicker(30 * time.Minute)
	defer t.Stop()
	for {
		if a.closing.Load() {
			return
		}
		a.checkMemoryWarning()
		<-t.C
	}
}
