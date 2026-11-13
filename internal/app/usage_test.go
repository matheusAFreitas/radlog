package app

import (
	"archive/zip"
	"bytes"
	"os"
	"testing"
	"time"
)

func TestUsage(t *testing.T) {
	a := newTestApp(t)
	now := time.Date(2026, 11, 12, 15, 0, 0, 0, time.Local)
	// desligado: não conta nada
	a.recordUsageAt(now, map[string]UsageCount{"p:agora": {N: 1, S: 30}})
	if u := a.usageAt(now, 0); u.On || len(u.Totals) != 0 {
		t.Fatalf("desligado contou: %+v", u)
	}
	a.SetUsageOn(true)
	a.recordUsageAt(now.AddDate(0, 0, -10), map[string]UsageCount{"p:agora": {N: 2, S: 60}})
	a.recordUsageAt(now, map[string]UsageCount{"p:agora": {N: 1, S: 9999}, "a:radiacode/espectro": {N: 3}, "x:ruim": {N: 1}, "p:\nx": {N: 1}})
	u := a.usageAt(now, 7)
	if u.Totals["p:agora"] != (UsageCount{N: 1, S: 600}) || u.Totals["a:radiacode/espectro"].N != 3 || len(u.Totals) != 2 || u.Days != 1 {
		t.Fatalf("7 dias: %+v", u)
	}
	if all := a.usageAt(now, 0); all.Totals["p:agora"] != (UsageCount{N: 3, S: 660}) || all.Days != 2 {
		t.Fatalf("tudo: %+v", all)
	}
	// abre de novo: continua; a opção fica mesmo com SaveSettings sem ela
	a.flushWrites()
	b := newTestApp(t)
	b.dataDir = a.dataDir
	b.settings.UsageOn = true
	if all := b.usageAt(now, 0); all.Totals["p:agora"].N != 3 {
		t.Fatalf("recarregado: %+v", all)
	}
	s := a.settings
	s.UsageOn = false
	if a.SaveSettings(s); !a.settings.UsageOn {
		t.Fatal("SaveSettings desligou a contagem")
	}
	// zerar e desligar
	if r := a.ResetUsage(); len(r.Totals) != 0 || r.Since == 0 {
		t.Fatalf("zerar: %+v", r)
	}
	if r := a.SetUsageOn(false); r.On {
		t.Fatal("não desligou")
	}
	// fica fora do backup
	a.flushWrites()
	if _, err := os.Stat(a.usagePath()); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := a.writeBackup(&buf); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zr.File {
		if f.Name == usageFile {
			t.Fatal("uso.json no backup")
		}
	}
}
