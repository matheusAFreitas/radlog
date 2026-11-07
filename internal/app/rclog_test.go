package app

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"radlog/internal/radiacode"
)

func TestRcLog(t *testing.T) {
	a := newTestApp(t)
	a.rcLogRecords([]radiacode.Record{
		radiacode.DoseRateDB{Count: 1202, CPS: 12.52, DoseRate: 0.154, DoseErr: 20.7, Flags: 0x3000},
		radiacode.Rare{Duration: 14086, Dose: 0.57, TempC: 27.5, Battery: 100, Flags: 0x1040},
		radiacode.Event{ID: 7},
		radiacode.Event{ID: 17}, // texto: fora
		radiacode.Event{ID: 3},
		radiacode.Event{ID: 3}, // mesmo minuto: fora
		radiacode.RealTime{CPS: 12},
	})
	p := a.GetRcLog(0, 0, []string{"taxa", "raro", "evento"}, 10, 0)
	if p.Total != 4 || p.Shown != 4 || len(p.Items) != 4 || p.Kinds["evento"] != 2 || p.First == 0 {
		t.Fatalf("log: %+v", p)
	}
	var tx, rr RcLogItem
	for _, it := range p.Items {
		switch it.Kind {
		case "taxa":
			tx = it
		case "raro":
			rr = it
		}
	}
	if math.Abs(tx.USv-0.154) > 1e-9 || math.Abs(tx.CPSErr-5.8) > 0.01 || tx.Src != "pc" {
		t.Fatalf("taxa: %+v", tx)
	}
	if !rr.HasTemp || rr.Temp != 27.5 || rr.Power != "" || rr.Dose != 0.57 || rr.DoseSec != 14086 {
		t.Fatalf("raro: %+v", rr)
	}
	p = a.GetRcLog(0, 0, []string{"evento"}, 1, 0)
	if p.Shown != 2 || p.Total != 4 || len(p.Items) != 1 || !p.More {
		t.Fatalf("filtro/página: %+v", p)
	}

	// log exportado pelo app do celular (Latin-1, como ele grava)
	mob := "Id,Timestamp,LastModified,DecimationMask,LinkType,Link,Real time,Channels,Event Id,Event,Event type,Event param. 1,Pulses,Count rate,Duration of d.a.,Duration of meas.,CR error,Dose rate,DR Error,Dose,Hardness,Dose Counter,Dose rate alarm,Dose alarm,Count rate alarm,Temp alarm,Temp.,Charge,Flags,Comment,Device S/N,Firmware,Latitude,Longitude,SystemComment\n" +
		`10,01DD6E700426B8CA,0000000000000000,0,0,"",2026.11.07  01.25.27,481,3,"Current device parameters",0,0,,,7:11:46,,,,,1.06 µSv,,,None,None,None,None,28.1°C,100%,0,"",RC-102-015102,262145,0,0,""` + "\n" +
		`11,0,0,0,0,"",2026.11.07  00.46.51,,,,,,1270,13.2 cps,,96,5.5%,15.3 µR/h,19.7%,,,,,,,,,,0,"",RC-102-015102,262145,0,0,""` + "\n" +
		`12,0,0,0,0,"",2026.11.06  18.28.31,,7,"Battery charge begin",0,0,,,,,,,,,,,,,,,,,0,"",RC-102-015102,262145,0,0,""` + "\n" +
		`13,0,0,0,0,"",lixo,,,,,,,,,,,,,,,,,,,,,,0,"",RC-102-015102,262145,0,0,""` + "\n"
	lat := make([]byte, 0, len(mob))
	for _, r := range mob {
		lat = append(lat, byte(r)) // Latin-1
	}
	res, err := a.importRcLog(lat)
	if err != nil || res.Read != 3 || res.Added != 3 || res.Skipped != 1 {
		t.Fatalf("importar: %+v %v", res, err)
	}
	day := time.Date(2026, 11, 7, 0, 0, 0, 0, time.Local)
	p = a.GetRcLog(0, 0, []string{"taxa", "raro", "evento"}, 100, 0)
	var mobile []RcLogItem
	for _, it := range p.Items {
		if it.Src == "celular" {
			mobile = append(mobile, it)
		}
	}
	if len(mobile) != 3 {
		t.Fatalf("importado: %+v", p)
	}
	for _, it := range mobile {
		switch it.Kind {
		case "taxa":
			if math.Abs(it.USv-0.153) > 1e-9 || it.CPS != 13.2 || it.CPSErr != 5.5 || it.USvErr != 19.7 || it.Count != 1270 {
				t.Fatalf("taxa do celular: %+v", it)
			}
		case "raro":
			if it.Temp != 28.1 || it.Battery != 100 || it.Dose != 1.06 || it.DoseSec != 7*3600+11*60+46 {
				t.Fatalf("raro do celular: %+v", it)
			}
		case "evento":
			if it.Code != 7 || it.Event != "começou a carregar" {
				t.Fatalf("evento do celular: %+v", it)
			}
		}
	}
	// importar de novo não repete
	if res, _ := a.importRcLog(lat); res.Added != 0 {
		t.Fatalf("repetiu: %+v", res)
	}
	if _, err := a.importRcLog([]byte("a,b\n1,2\n")); err == nil {
		t.Fatalf("arquivo errado deveria dar erro")
	}

	// backup leva o log, e outro PC junta sem repetir
	var buf bytes.Buffer
	if err := a.writeBackup(&buf); err != nil {
		t.Fatal(err)
	}
	b := newTestApp(t)
	b.rcLogRecords([]radiacode.Record{radiacode.Event{ID: 1}})
	zp := filepath.Join(t.TempDir(), "b.zip")
	os.WriteFile(zp, buf.Bytes(), 0o644)
	if _, err := b.mergeZip(zp, true); err != nil {
		t.Fatal(err)
	}
	if _, err := b.mergeZip(zp, true); err != nil {
		t.Fatal(err)
	}
	if p := b.GetRcLog(0, 0, []string{"taxa", "raro", "evento"}, 100, 0); p.Total != 8 {
		t.Fatalf("no outro PC: %d", p.Total)
	}
	// os minutos do RadiaCode não confundem o log com minutos
	if len(b.rcRange(day.Add(-48*time.Hour), time.Now())) != 0 {
		t.Fatalf("log virou minuto")
	}
}

func TestRcLogFromRaw(t *testing.T) {
	lines := [][]byte{
		[]byte(`{"pc":1794013499591,"hex":"94000255e4ffffb204000055554841772d8137cf000030","tipo":"DoseRateDB"}`),
		[]byte(`{"pc":1794013709591,"hex":"3b0003d05500000637000049136f388a1210274030","tipo":"RareData"}`),
		[]byte(`{"pc":1794013990590,"hex":"6f0007a8c3000003004020","tipo":"Event"}`),
		[]byte(`{"pc":1794013996591,"hex":"7c000707c6000003004020","tipo":"Event"}`),
	}
	items := rcLogFromRaw(lines)
	if len(items) != 3 {
		t.Fatalf("itens: %+v", items)
	}
	if items[0].Kind != "taxa" || items[0].Count != 1202 || math.Abs(items[0].CPS-12.52) > 0.01 || items[0].USv <= 0.1 || items[0].USv > 0.2 {
		t.Fatalf("taxa: %+v", items[0])
	}
	if items[1].Kind != "raro" || items[1].Battery != 100 || items[1].Power != "" || items[1].DoseSec != 14086 || math.Abs(items[1].Temp-27.46) > 0.01 {
		t.Fatalf("raro: %+v", items[1])
	}
	if items[2].Kind != "evento" || !strings.Contains(items[2].Event, "configura") || items[2].T != 1794013990590 {
		t.Fatalf("evento: %+v", items[2])
	}
}
