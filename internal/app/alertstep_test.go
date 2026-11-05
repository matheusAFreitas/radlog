package app

import (
	"testing"
	"time"
)

// Pular de 20 para 1.000 CPM liga atenção, alarme e perigo no mesmo segundo:
// só o perigo é aberto. Ao descer, os níveis de baixo abrem um de cada vez.
func TestAlertJumpOpensOnlyHighest(t *testing.T) {
	a, _ := testApp(t)
	lv := a.settings.Levels.norm()
	now := time.Now()
	var ids alertIDs
	step := func(sec int, cpm float64, s alertStep) {
		s.Now, s.CPM, s.Lv, s.AlarmCPM = now.Add(time.Duration(sec)*time.Second), cpm, lv, 100
		a.stepAlerts(&ids, s)
	}
	open := func() (lv []string) {
		a.mu.Lock()
		defer a.mu.Unlock()
		for _, id := range []int64{ids.danger, ids.alarm, ids.attn} {
			if e := a.eventLocked(id); e != nil {
				lv = append(lv, e.Level)
			}
		}
		return
	}
	step(0, 1000, alertStep{AttnStarted: true, AttnOn: true, AlarmStarted: true, AlarmOn: true, DangerStarted: true})
	if got := open(); len(got) != 1 || got[0] != "perigo" {
		t.Fatalf("no pulo: %v", got)
	}
	step(30, 300, alertStep{AttnOn: true, AlarmOn: true, DangerEnded: true})
	if got := open(); len(got) != 1 || got[0] != "alarme" {
		t.Fatalf("saiu do perigo: %v", got)
	}
	step(60, 60, alertStep{AttnOn: true, AlarmEnded: true})
	if got := open(); len(got) != 1 || got[0] != "atencao" {
		t.Fatalf("saiu do alarme: %v", got)
	}
	step(90, 20, alertStep{AttnEnded: true})
	if got := open(); len(got) != 0 {
		t.Fatalf("voltou ao fundo: %v", got)
	}
	// alarme desligado: do perigo direto para a atenção
	step(120, 1000, alertStep{AttnStarted: true, AttnOn: true, DangerStarted: true})
	step(150, 60, alertStep{AttnOn: true, DangerEnded: true})
	if got := open(); len(got) != 1 || got[0] != "atencao" {
		t.Fatalf("perigo → atenção sem alarme: %v", got)
	}
	// subida aos poucos continua anotando cada nível
	step(180, 20, alertStep{AttnEnded: true})
	step(200, 60, alertStep{AttnStarted: true, AttnOn: true})
	step(230, 150, alertStep{AttnOn: true, AlarmStarted: true, AlarmOn: true})
	if got := open(); len(got) != 2 {
		t.Fatalf("subida aos poucos: %v", got)
	}
}
