package app

// QA, conferência do PR #198 (2.101.13), item 62 (teste do QA, virou regressão): programada vencida aceita
// "corrigir aparelhos" (mesmo horário) e continua recusando horário novo no passado.

import (
	"testing"
	"time"
)

func TestQA21013PlannedFixDevices(t *testing.T) {
	a := newTestApp(t)
	past := time.Now().Add(-3 * time.Hour).Truncate(time.Minute).UnixMilli()
	a.mu.Lock()
	a.planned = []PlannedTrip{
		{ID: 9, Name: "Mercado", From: past, RC: true},
		{ID: 10, Name: "Academia", From: past + 60_000, RC: true, Left: past + 120_000, LeftDev: devRC},
	}
	a.mu.Unlock()

	// 1. vencida, mesmo horário, aparelhos corrigidos: aceita
	p, err := a.SavePlannedTrip(PlannedTrip{ID: 9, Name: "Mercado", From: past, RC: true, GMC: true})
	if err != nil || !p.GMC || p.From != past {
		t.Fatalf("corrigir aparelhos da vencida: %+v %v", p, err)
	}
	// 2. mesma programada, horário novo no passado: recusa
	if _, err := a.SavePlannedTrip(PlannedTrip{ID: 9, Name: "Mercado", From: past - 3600_000, RC: true}); err == nil {
		t.Fatal("aceitou mudar o horário para o passado")
	}
	// 3. nova (ID 0) no passado: recusa
	if _, err := a.SavePlannedTrip(PlannedTrip{Name: "x", From: past, RC: true}); err == nil {
		t.Fatal("aceitou programada nova no passado")
	}
	// 4. ID que não existe, no passado: recusa
	if _, err := a.SavePlannedTrip(PlannedTrip{ID: 77, Name: "x", From: past, RC: true}); err == nil {
		t.Fatal("aceitou ID que não existe no passado")
	}
	// 5. a que já começou (saiu): mesmo horário e mesmos aparelhos guarda o "saiu"
	p, err = a.SavePlannedTrip(PlannedTrip{ID: 10, Name: "Academia", From: past + 60_000, RC: true})
	if err != nil || p.Left != past+120_000 {
		t.Fatalf("começou, sem mudar nada: %+v %v", p, err)
	}
	// e com o GMC-800 indo também, o Go aceita mas esquece o "saiu" (o RC102 continua indo)
	p, err = a.SavePlannedTrip(PlannedTrip{ID: 10, Name: "Academia", From: past + 60_000, RC: true, GMC: true})
	t.Logf("começou, GMC-800 passa a ir: err=%v saiu=%d saiuAparelho=%q", err, p.Left, p.LeftDev)
	l := a.GetPlannedTrips()
	if len(l) != 2 || l[0].ID != 9 || !l[0].GMC || l[0].From != past {
		t.Fatalf("lista: %+v", l)
	}
}

// 2.101.13 (item 62, conferência do QA): na que já começou, corrigir os
// aparelhos guarda o "saiu" enquanto o aparelho que saiu continua indo.
func TestPlannedStartedFixDevices(t *testing.T) {
	a := newTestApp(t)
	past := time.Now().Add(-3 * time.Hour).Truncate(time.Minute).UnixMilli()
	left := past + 120_000
	a.mu.Lock()
	a.planned = []PlannedTrip{{ID: 10, Name: "Academia", From: past, RC: true, Left: left, LeftDev: devRC}}
	a.mu.Unlock()
	p, err := a.SavePlannedTrip(PlannedTrip{ID: 10, Name: "Academia", From: past, RC: true, GMC: true})
	if err != nil || p.Left != left || p.LeftDev != devRC {
		t.Fatalf("o RC102 continua indo e o saiu sumiu: %+v %v", p, err)
	}
	p, err = a.SavePlannedTrip(PlannedTrip{ID: 10, Name: "Academia", From: past, GMC: true})
	if err != nil || p.Left != 0 {
		t.Fatalf("o RC102 saiu da lista e o saiu ficou: %+v %v", p, err)
	}
}

// 2.101.13 (item 66, conferência do QA): o σ do cartão do Agora é o dos
// últimos 10 min; com menos de 5 min de leitura não há σ.
func TestSigmaOverItem66(t *testing.T) {
	mk := func(n int, cpm float64) []sample {
		ss := make([]sample, n)
		for i := range ss {
			ss[i] = sample{cpm: cpm}
		}
		return ss
	}
	if sigmaOver(mk(299, 40), 31) != nil {
		t.Fatal("σ com menos de 5 min")
	}
	z := sigmaOver(mk(600, 37.2), 31) // +20% por 10 min
	if z == nil || *z < 3 || *z > 4 {
		t.Fatalf("subida de 20%% em 10 min deu σ %v; esperado ~3,5", z)
	}
	if z := sigmaOver(mk(600, 31), 31); z == nil || *z != 0 {
		t.Fatalf("fundo normal deu σ %v", z)
	}
}
