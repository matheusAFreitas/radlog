package app

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// JPEG mínimo com um APP1 Exif que carrega um marcador de "GPS".
func qaJPEGWithGPS() []byte {
	var b bytes.Buffer
	b.Write([]byte{0xFF, 0xD8})
	pay := []byte("Exif\x00\x00GPSLAT=-23.550520;GPSLON=-46.633308")
	b.Write([]byte{0xFF, 0xE1})
	binary.Write(&b, binary.BigEndian, uint16(len(pay)+2))
	b.Write(pay)
	// DQT mínimo (tabela) para ter um segmento "normal"
	dqt := make([]byte, 65)
	b.Write([]byte{0xFF, 0xDB})
	binary.Write(&b, binary.BigEndian, uint16(len(dqt)+2))
	b.Write(dqt)
	// SOS + dados + EOI
	sos := []byte{0x01, 0x01, 0x00, 0x00, 0x3F, 0x00}
	b.Write([]byte{0xFF, 0xDA})
	binary.Write(&b, binary.BigEndian, uint16(len(sos)+2))
	b.Write(sos)
	b.Write([]byte{0x12, 0x34, 0x56})
	b.Write([]byte{0xFF, 0xD9})
	return b.Bytes()
}

func TestQA_AttachJFIFKeepsGPSInBackup(t *testing.T) {
	a, dir := testApp(t)
	id, err := a.addMeasurement(SavedMeasure{Name: "pedra", Time: time.Now().UnixMilli()})
	if err != nil {
		t.Fatal(err)
	}
	src := t.TempDir()
	for _, n := range []string{"foto.jpg", "foto.jfif", "foto.jpe"} {
		if err := os.WriteFile(filepath.Join(src, n), qaJPEGWithGPS(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.attachFiles(id, []string{filepath.Join(src, "foto.jpg"), filepath.Join(src, "foto.jfif"), filepath.Join(src, "foto.jpe")}); err != nil {
		t.Fatalf("attach: %v", err)
	}
	// na pasta do app
	for _, n := range []string{"foto.jpg", "foto.jfif", "foto.jpe"} {
		b, _ := os.ReadFile(filepath.Join(a.attachDir(id), n))
		t.Logf("pasta do app %-10s GPS dentro? %v", n, bytes.Contains(b, []byte("GPSLAT")))
	}
	var zb bytes.Buffer
	if err := a.writeBackup(&zb); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(zb.Bytes()), int64(zb.Len()))
	if err != nil {
		t.Fatal(err)
	}
	leak := 0
	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, attachDirName+"/") {
			continue
		}
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		has := bytes.Contains(b, []byte("GPSLAT"))
		t.Logf("backup %-35s GPS dentro? %v", f.Name, has)
		if has {
			leak++
		}
	}
	_ = dir
	if leak > 0 {
		t.Errorf("%d anexo(s) JPEG foram para o backup com a localização (EXIF GPS) dentro", leak)
	}
}

// Dois aparelhos: um alerta do GMC terminando (saveEvents regrava o
// alertas.csv inteiro) ao mesmo tempo que um alerta do RC102 começa
// (addEvent anexa no fim do alertas.csv).
func TestQA_EventsLostOnConcurrentSaveAndAppend(t *testing.T) {
	a, dir := testApp(t)
	// cria o alertas.csv (para o addEvent usar o caminho do append)
	a.addEvent(Event{Time: time.Now().UnixMilli(), Text: "Alarme inicial", Kind: kindAlert, Level: "alarme"}, true)
	a.saveEvents()
	const rounds = 300
	for i := 0; i < rounds; i++ {
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); a.saveEvents() }()
		go func(i int) {
			defer wg.Done()
			// o alerta do RC102 chega uns µs depois (tempo independente)
			time.Sleep(time.Duration(i%20) * 25 * time.Microsecond)
			a.addEvent(Event{Time: time.Now().UnixMilli(), Text: "Alarme RC102 " + itoaQA(i), Kind: kindAlert, Level: "alarme", Device: devRC}, true)
		}(i)
		wg.Wait()
	}
	a.mu.Lock()
	mem := 0
	for _, e := range a.events {
		if e.Kind == kindAlert {
			mem++
		}
	}
	a.mu.Unlock()
	disk := len(readEventsFile(filepath.Join(dir, "alertas.csv")))
	t.Logf("alertas na memória: %d, no alertas.csv: %d", mem, disk)
	if disk < mem {
		t.Errorf("%d alerta(s) sumiram do alertas.csv (estão só na memória; somem ao fechar o app)", mem-disk)
	}
}

func itoaQA(i int) string {
	return strings.TrimSpace(strings.Repeat(" ", 0) + func() string {
		b := []byte{}
		if i == 0 {
			return "0"
		}
		for i > 0 {
			b = append([]byte{byte('0' + i%10)}, b...)
			i /= 10
		}
		return string(b)
	}())
}

// Linha cortada no fim do arquivo do RC102 (queda de energia no meio da
// gravação): a próxima linha gruda nela.
func TestQA_RcMinuteAfterCutLine(t *testing.T) {
	a, _ := testApp(t)
	base := time.Date(2026, 11, 17, 10, 0, 0, 0, time.Local)
	os.MkdirAll(a.rcDir(), 0o755)
	path := filepath.Join(a.rcDir(), base.Format("2006-01")+".csv")
	// cabeçalho + um minuto bom + um minuto cortado no meio (sem \n)
	body := rcHeader + "\n" +
		base.Format(time.RFC3339) + ",5.100,0.11000,120\n" +
		base.Add(time.Minute).Format(time.RFC3339) + ",5.2"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	// app volta e grava o minuto seguinte normalmente
	a.rcAppend(rcMinute{t: base.Add(2 * time.Minute), cps: 5.3 * 120, usv: 0.12 * 120, n: 120})
	raw, _ := os.ReadFile(path)
	t.Logf("arquivo:\n%s", raw)
	pts := a.rcRangeDisk(base, base.Add(time.Hour))
	for _, p := range pts {
		t.Logf("minuto %s cps=%.3f usvh=%.5f n=%d", p.T.Format("15:04"), p.CPS, p.USvh, p.N)
		if p.USvh > 1 {
			t.Errorf("minuto %s ficou com %.3f µSv/h (era o CPS da linha seguinte); valor falso no gráfico e na dose", p.T.Format("15:04"), p.USvh)
		}
	}
}

// Linha cortada no fim do dose-AAAA-MM.csv (queda de energia no meio da
// gravação de um total): o próximo total gruda nela.
func TestQA_RcDoseLogAfterCutLine(t *testing.T) {
	a, _ := testApp(t)
	base := time.Date(2026, 11, 17, 10, 0, 0, 0, time.Local)
	os.MkdirAll(a.rcDir(), 0o755)
	path := filepath.Join(a.rcDir(), rcDoseFileName(base))
	body := rcDoseHeader + "\n" +
		base.Format(time.RFC3339) + ",100.0000,360000\n" +
		base.Add(4*time.Minute).Format(time.RFC3339) + ",100.0100,360240\n" +
		base.Add(8*time.Minute).Format(time.RFC3339) + ",100.02" // cortada
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	// o app volta e grava os próximos totais normalmente
	a.rcDoseLogAppend(base.Add(12*time.Minute), 100.0300, 360720)
	a.rcDoseLogAppend(base.Add(16*time.Minute), 100.0400, 360960)
	raw, _ := os.ReadFile(path)
	t.Logf("arquivo:\n%s", raw)
	for _, p := range a.rcDoseLog(base.Add(-time.Hour), base.Add(time.Hour)) {
		t.Logf("total %s dose=%.4f seg=%.0f", p.T.Format("15:04"), p.Dose, p.Sec)
	}
	dose, upTo, ok := a.rcDeviceDose(base, base.Add(30*time.Minute))
	t.Logf("dose do aparelho 10:00–10:30 = %.4f µSv (até %s, ok=%v); o certo é 0,04 µSv", dose, upTo.Format("15:04"), ok)
	if dose > 1 {
		t.Errorf("dose do período virou %.2f µSv (devia ser ~0,04): a linha grudada conta como 'aparelho zerado' e soma o total inteiro de novo", dose)
	}
}

// Medindo um objeto com o GMC-800, o cabo solta no meio do fundo. A medição
// fica parada em "fundo" para sempre e cala a atenção e o alarme do RC102.
func TestQA_MeasureStuckAfterDisconnectSilencesRC(t *testing.T) {
	a := newTestApp(t)
	fakeGMC(t, a, 25)
	if _, err := a.StartMeasure(MeasureOpts{Name: "pedra", Device: "gmc", Seconds: 60}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		a.feedMeasure(25, 0)
	}
	// conexão perdida (o que o disconnect faz com a porta; d.Close de um
	// Device falso não dá para chamar)
	a.mu.Lock()
	a.dev, a.stopPoll = nil, nil
	a.mu.Unlock()
	a.disconnect("Conexão perdida: cabo")
	st := a.GetMeasure()
	t.Logf("depois de desconectar: fase=%q elapsed=%d/%d", st.Phase, st.Elapsed, st.Seconds)

	l := a.rcLimits()
	later := time.Now().Add(6 * time.Hour)
	a.mu.Lock()
	active := a.measureActiveLocked(later)
	a.mu.Unlock()
	t.Logf("6 h depois: medição ainda 'ativa'? %v (limites RC102: atenção %.3f, alarme %.3f µSv/h)", active, l.Attn, l.Alarm)
	// o RC102 passa do alarme 6 h depois
	for i := 0; i < 5; i++ {
		a.rcAlertStep(later.Add(time.Duration(i)*time.Second), l.Alarm*3)
	}
	a.mu.Lock()
	on := a.rcAlert.alarmOn
	a.mu.Unlock()
	t.Logf("RC102 em %.3f µSv/h (3× o alarme): alarme ligado? %v", l.Alarm*3, on)
	if !on {
		t.Errorf("6 h depois do cabo soltar, o RC102 a 3× o alarme não abriu alarme: a medição presa em %q ainda cala os alertas", st.Phase)
	}
}

// O arquivo do mês do RC102 não deixa gravar (aberto no Excel, antivírus,
// OneDrive). Simulado com uma pasta no lugar do arquivo (como root o chmod
// não impede). O minuto some sem aviso e não é gravado depois.
func TestQA_RcMinuteLostWhenFileLocked(t *testing.T) {
	a, _ := testApp(t)
	now := time.Now().Truncate(time.Minute)
	os.MkdirAll(a.rcDir(), 0o755)
	path := filepath.Join(a.rcDir(), now.Format("2006-01")+".csv")
	if err := os.Mkdir(path, 0o755); err != nil { // "trancado"
		t.Fatal(err)
	}
	evBefore := len(a.GetEvents(""))
	a.rcWriteMinute(rcMinute{t: now.Add(-2 * time.Minute), cps: 6 * 120, usv: 0.1 * 120, n: 120})
	a.rcWriteMinute(rcMinute{t: now.Add(-1 * time.Minute), cps: 6 * 120, usv: 0.1 * 120, n: 120})
	evAfter := a.GetEvents("")
	t.Logf("eventos novos depois da falha: %d", len(evAfter)-evBefore)
	for _, e := range evAfter[:len(evAfter)-evBefore] {
		t.Logf("  evento: %s", e.Text)
	}
	// "Excel fechou": o arquivo libera
	os.Remove(path)
	a.rcWriteMinute(rcMinute{t: now, cps: 6 * 120, usv: 0.1 * 120, n: 120})
	pts := a.rcRangeDisk(now.Add(-time.Hour), now.Add(time.Hour))
	t.Logf("minutos no disco depois de liberar: %d (esperado 3)", len(pts))
	st := a.GetStatus()
	t.Logf("status: savePending=%d saveFile=%q", st.SavePending, st.SaveFile)
	if len(pts) < 3 {
		t.Errorf("%d minuto(s) do RC102 sumiram sem aviso (o GMC-800 guarda na memória e avisa; o RC102 não)", 3-len(pts))
	}
}

// A tela guarda as configurações que leu ao abrir e manda tudo de volta ao
// salvar. O contador conectou depois (o connect grava Port e PortUSB). O
// SaveSettings preserva Port, mas não PortUSB.
func TestQA_SaveSettingsErasesPortUSB(t *testing.T) {
	a, dir := testApp(t)
	screen := a.GetSettings() // a tela abriu antes de conectar
	a.mu.Lock()
	a.settings.Port, a.settings.PortUSB = "COM3", "1A86:7523" // connect()
	a.mu.Unlock()
	screen.AlarmSound = "bipes" // a pessoa muda o som do alarme e salva
	a.SaveSettings(screen)
	a.flushWrites()
	b, _ := os.ReadFile(filepath.Join(dir, "config.json"))
	a.mu.Lock()
	port, usb := a.settings.Port, a.settings.PortUSB
	a.mu.Unlock()
	t.Logf("depois de salvar: port=%q portUsb=%q; config.json tem portUsb? %v", port, usb, bytes.Contains(b, []byte("portUsb")))
	if usb == "" {
		t.Errorf("PortUSB apagado ao salvar outra configuração; a reconexão volta a confiar só no número da porta (B5)")
	}
}

// medicoes.json estragado (cortado): o app começa vazio. O comentário do
// readStateJSON diz "avisa em Eventos", mas só vai para o log técnico.
func TestQA_CorruptMeasurementsNoEvent(t *testing.T) {
	a, dir := testApp(t)
	good := `[{"id":1,"name":"pedra","time":1763380800000},{"id":2,"name":"sal","time":1763384400000}]`
	os.WriteFile(filepath.Join(dir, "medicoes.json"), []byte(good[:50]), 0o644) // cortado
	a.loadMeasurements()
	t.Logf("medições carregadas: %d", len(a.GetMeasurements()))
	evs := a.GetEvents("")
	t.Logf("eventos: %d", len(evs))
	for _, e := range evs {
		t.Logf("  evento: %s", e.Text)
	}
	lg, _ := os.ReadFile(filepath.Join(dir, "log.txt"))
	t.Logf("log técnico: %s", strings.TrimSpace(string(lg)))
	if _, err := os.Stat(filepath.Join(dir, "medicoes.json.ilegivel")); err == nil {
		t.Logf("cópia em medicoes.json.ilegivel: sim")
	}
	if len(evs) == 0 {
		t.Errorf("medições sumiram da tela sem nenhum aviso em Eventos")
	}
}
