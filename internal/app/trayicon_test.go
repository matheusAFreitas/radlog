package app

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"radlog/internal/i18n"
)

func TestTrayNum(t *testing.T) {
	cases := map[float64]string{
		0.12: ",12", 0.05: ",05", 0.996: "1,0", 1.42: "1,4", 9.94: "9,9", 9.96: "10",
		48.2: "48", 999: "999", 999.6: "1k", 5000: "1k", 0: ",00",
	}
	for v, want := range cases {
		if got := trayNum(v, ","); got != want {
			t.Errorf("trayNum(%v) = %q, quero %q", v, got, want)
		}
	}
	if got := trayNum(0.12, "."); got != ".12" {
		t.Errorf("en: %q", got)
	}
	for v := 0.0; v < 2000; v += 0.37 {
		if n := len(trayNum(v, ",")); n > 3 {
			t.Fatalf("trayNum(%v) passou de 3 caracteres", v)
		}
	}
}

func TestTrayPick(t *testing.T) {
	g := trayDev{Key: "gmc", State: "ok"}
	r := trayDev{Key: "rc", State: "ok"}
	if trayPick("", g, r).Key != "gmc" {
		t.Error("empate fica no GMC-800")
	}
	r.Level = "alarme"
	if trayPick("", g, r).Key != "rc" {
		t.Error("RC102 mais grave vai para o ícone")
	}
	if trayPick("gmc", g, r).Key != "gmc" {
		t.Error("fixo no GMC-800")
	}
	g.State = "off"
	r.Level = ""
	if trayPick("", g, r).Key != "rc" {
		t.Error("só o RC102 conectado")
	}
	g.State, r.State = "ok", "off"
	if trayPick("radiacode", g, r).Key != "rc" {
		t.Error("fixo no RC102 mesmo desconectado")
	}
}

func TestTrayLines(t *testing.T) {
	en := func(s string) string { return i18n.T("en", s) }
	cases := []struct {
		d      trayDev
		pt, en string
	}{
		{trayDev{Name: "GMC-800", State: "ok", USv: 0.12}, "GMC-800 · 0.12 µSv/h · seguro", "GMC-800 · 0.12 µSv/h · safe"},
		{trayDev{Name: "RC102", State: "ok", USv: 0.1, Away: true}, "RC102 · 0.10 µSv/h · fora de casa", "RC102 · 0.10 µSv/h · away from home"},
		{trayDev{Name: "GMC-800", State: "ok", USv: 1.42, Level: "alarme"}, "ALARME no GMC-800 · 1.42 µSv/h", "ALARM on the GMC-800 · 1.42 µSv/h"},
		{trayDev{Name: "GMC-800", State: "ok", USv: 0.3, Level: "atencao"}, "ATENÇÃO no GMC-800 · 0.30 µSv/h", "ATTENTION on the GMC-800 · 0.30 µSv/h"},
		{trayDev{Name: "RC102", State: "ok", Sat: true, SatAbove: 1000}, "RC102 saturado · acima de 1000 µSv/h", "RC102 saturated · above 1000 µSv/h"},
		{trayDev{Name: "GMC-800", State: "ok", Sat: true}, "GMC-800 saturado · o valor real é maior", "GMC-800 saturated · the real value is higher"},
		{trayDev{Name: "GMC-800", State: "ok", NoCount: true}, "GMC-800 sem contagens · confira o aparelho", "GMC-800 no counts · check the device"},
		{trayDev{Name: "RC102", State: "wait"}, "RC102 conectando…", "RC102 connecting…"},
		{trayDev{Name: "RC102", State: "off"}, "RC102 desconectado", "RC102 disconnected"},
	}
	for _, c := range cases {
		got := trayLine(c.d, ".")
		if got != c.pt {
			t.Errorf("pt: %q, quero %q", got, c.pt)
		}
		if e := en(got); e != c.en {
			t.Errorf("en: %q, quero %q", e, c.en)
		}
	}
	if got := trayLine(trayDev{Name: "GMC-800", State: "ok", USv: 0.12}, ","); got != "GMC-800 · 0,12 µSv/h · seguro" {
		t.Errorf("pt com vírgula: %q", got)
	}
}

func TestTrayICO(t *testing.T) {
	d := trayDev{Name: "GMC-800", Key: "gmc", State: "ok", USv: 0.12}
	b := trayICO(d, ",")
	if binary.LittleEndian.Uint16(b[2:]) != 1 || binary.LittleEndian.Uint16(b[4:]) != 4 {
		t.Fatal("cabeçalho do .ico")
	}
	for i, want := range []int{16, 20, 24, 32} {
		e := b[6+16*i:]
		if int(e[0]) != want {
			t.Fatalf("entrada %d: %d px", i, e[0])
		}
		size, off := binary.LittleEndian.Uint32(e[8:]), binary.LittleEndian.Uint32(e[12:])
		img, err := png.Decode(bytes.NewReader(b[off : off+size]))
		if err != nil || img.Bounds().Dx() != want {
			t.Fatalf("PNG %d px: %v", want, err)
		}
	}
	// o número sai na cor do aparelho, sem cinza de antisserrilhado
	img := drawTrayIcon(16, d, ",")
	green := 0
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			if img.NRGBAAt(x, y) == trayAcc {
				green++
			}
		}
	}
	if green < 10 {
		t.Fatalf("poucos pixels do número: %d", green)
	}
	if img.NRGBAAt(0, 0).A == 255 {
		t.Fatal("canto deveria ser arredondado")
	}
	// RADLOG_TRAY_PNG=pasta grava uma folha com todos os estados (para olhar)
	if dir := os.Getenv("RADLOG_TRAY_PNG"); dir != "" {
		states := []trayDev{
			d,
			{Key: "rc", State: "ok", USv: 0.1},
			{Key: "gmc", State: "ok", USv: 0.31, Level: "atencao"},
			{Key: "gmc", State: "ok", USv: 1.42, Level: "alarme"},
			{Key: "gmc", State: "ok", USv: 48.2, Level: "perigo"},
			{Key: "gmc", State: "ok", Sat: true},
			{Key: "gmc", State: "ok", NoCount: true},
			{Key: "gmc", State: "wait"},
			{Key: "gmc", State: "off"},
			{Key: "gmc", State: "ok", USv: 999},
			{Key: "rc", State: "ok", USv: 2400, Level: "perigo"},
		}
		for _, n := range []int{16, 20, 24, 32} {
			for i, s := range states {
				f, _ := os.Create(filepath.Join(dir, "t"+string(rune('a'+i))+"-"+itoa(n)+".png"))
				_ = png.Encode(f, drawTrayIcon(n, s, ","))
				f.Close()
			}
		}
	}
}

func itoa(n int) string { return string([]byte{byte('0' + n/10), byte('0' + n%10)}) }
