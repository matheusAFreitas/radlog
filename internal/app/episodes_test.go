package app

import (
	"testing"
	"time"
)

func TestAlertEpisodes(t *testing.T) {
	T := time.Date(2026, 11, 1, 9, 0, 0, 0, time.Local).UnixMilli()
	m := int64(60000)
	evs := []Event{
		{ID: 1, Time: T, End: T + 10*m, Kind: kindAlert, Level: "atencao", Peak: 70},
		{ID: 2, Time: T + 6*m, End: T + 9*m, Kind: kindAlert, Level: "alarme", Peak: 134},
		{ID: 3, Time: T + 9*m, End: T + 15*m, Kind: kindAlert, Level: "atencao", Peak: 99},
		{ID: 4, Time: T + 120*m, End: T + 121*m, Kind: kindAlert, Level: "atencao", Peak: 53},
		{ID: 5, Time: T + 121*m, End: T + 122*m, Kind: kindAlert, Level: "atencao", Peak: 60, Retro: true},
	}
	eps := alertEpisodes(evs, time.Now())
	if len(eps) != 3 {
		t.Fatalf("esperava 3 episódios, veio %d: %+v", len(eps), eps)
	}
	if eps[0].Level != "alarme" || eps[0].Peak != 134 || len(eps[0].IDs) != 3 || eps[0].Until != T+15*m {
		t.Fatalf("primeiro episódio errado: %+v", eps[0])
	}
	if eps[1].Level != "atencao" || len(eps[1].IDs) != 1 {
		t.Fatalf("segundo episódio errado: %+v", eps[1])
	}
}
