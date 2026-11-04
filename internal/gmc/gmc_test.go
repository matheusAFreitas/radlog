package gmc

import (
	"errors"
	"math"
	"testing"
	"time"
)

func TestBasicCommands(t *testing.T) {
	s := newSim()
	s.cpm = 2495
	s.cps = 39
	d := New(s)

	v, err := d.Version()
	if err != nil || v != "GMC-800Re1.08" {
		t.Fatalf("Version = %q, %v", v, err)
	}
	ser, err := d.Serial()
	if err != nil || ser != "A1 B2 C3 D4 E5 F6 01 02 03 04 05 06" {
		t.Fatalf("Serial = %q, %v", ser, err)
	}
	cpm, err := d.CPM()
	if err != nil || cpm != 2495 {
		t.Fatalf("CPM = %d, %v", cpm, err)
	}
	cps, err := d.CPS()
	if err != nil || cps != 39 {
		t.Fatalf("CPS = %d, %v", cps, err)
	}
	dt, err := d.DateTime()
	if err != nil || !dt.Equal(s.clock) {
		t.Fatalf("DateTime = %v, %v", dt, err)
	}
	want := time.Date(2026, 3, 9, 15, 33, 58, 0, time.Local)
	if err := d.SetDateTime(want); err != nil {
		t.Fatal(err)
	}
	if !s.clock.Equal(want) {
		t.Fatalf("relógio do simulador = %v", s.clock)
	}
}

func TestCalibrationFromRealConfig(t *testing.T) {
	d := New(newSim())
	cfg, err := d.Config()
	if err != nil {
		t.Fatal(err)
	}
	cal, err := ParseCalibration800(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(cal.Points) != 6 || cal.Points[0].CPM != 1538 || cal.Points[0].USv != 10 || cal.Points[5].CPM != 307600 {
		t.Fatalf("pontos = %+v", cal.Points)
	}
	// Mesmo valor que o PyGMC calcula para 2495 CPM nesse aparelho.
	if got := cal.USv(2495); math.Abs(got-16.222366710013002) > 1e-9 {
		t.Fatalf("USv(2495) = %v", got)
	}
	if got := cal.USv(22); math.Abs(got-0.143042) > 1e-5 {
		t.Fatalf("USv(22) = %v", got)
	}
	// Acima do último ponto continua a última reta.
	if got := cal.USv(615200); math.Abs(got-4000) > 1e-6 {
		t.Fatalf("USv(615200) = %v", got)
	}
}

// A de fábrica é a que o GMC-800 Re1.20 volta a ter quando é formatado.
// (O contador gravado nos testes, de firmware 1.08, tem outra: 1538 CPM = 10
// µSv/h. Por isso o app sempre usa a lida do aparelho quando consegue.)
func TestDefaultCalibration(t *testing.T) {
	def := DefaultCalibration()
	if len(def.Points) != 6 || def.Points[0] != (CalPoint{986, 10}) || def.Points[5] != (CalPoint{139671, 2000}) {
		t.Fatalf("pontos: %+v", def.Points)
	}
	if got := def.USv(28); math.Abs(got-28*10.0/986) > 1e-12 {
		t.Fatalf("USv(28) = %v", got)
	}
	if !def.CompensatesDeadTime() {
		t.Fatal("a curva de fábrica compensa as perdas em taxa alta")
	}
	lin := def.Linear()
	if got := lin.USv(100000); math.Abs(got-100000*10.0/986) > 1e-9 {
		t.Fatalf("linear USv(100000) = %v", got)
	}
	dev, _ := ParseCalibration800(cfg800())
	if dev.CompensatesDeadTime() {
		t.Fatal("a curva do contador gravado é uma reta só")
	}
}

func TestTimeoutWhenSilent(t *testing.T) {
	s := newSim()
	d := New(s)
	s.silent = true
	start := time.Now()
	_, err := d.CPM()
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("esperava timeout, veio %v", err)
	}
	if time.Since(start) > 4*time.Second {
		t.Fatal("demorou demais para desistir")
	}
	// Volta a responder e o próximo comando funciona.
	s.silent = false
	if _, err := d.CPM(); err != nil {
		t.Fatal(err)
	}
}

func TestReadHistoryWithExtraSPIRByte(t *testing.T) {
	s := newSim()
	img := buildHistoryImage()
	copy(s.flash, img)
	d := New(s)

	var progressed int
	raw, err := d.ReadHistory(nil, func(n int) { progressed = n })
	if err != nil {
		t.Fatal(err)
	}
	pages := (len(img) + PageSize - 1) / PageSize
	if len(raw) != pages*PageSize || progressed != len(raw) {
		t.Fatalf("li %d bytes (progresso %d), esperava %d", len(raw), progressed, pages*PageSize)
	}
	h := ParseHistory(raw)
	checkHistory(t, h)

	// Depois do histórico a porta continua sincronizada.
	if cpm, err := d.CPM(); err != nil || cpm != 22 {
		t.Fatalf("CPM depois do histórico = %d, %v", cpm, err)
	}
}

// buildHistoryImage monta uma memória com: 3 minutos em CPM/min, uma nota,
// um valor de 2 bytes, um valor 0x55 isolado, e depois 4 segundos em CPS.
// Ocupa mais de uma página para testar a leitura paginada.
func buildHistoryImage() []byte {
	var b []byte
	stamp := func(yy, mo, dd, hh, mi, ss, mode byte) {
		b = append(b, 0x55, 0xAA, 0x00, yy, mo, dd, hh, mi, ss, 0x55, 0xAA, mode)
	}
	stamp(26, 10, 27, 10, 0, 0, 2)
	b = append(b, 20, 21, 19)                              // 10:01, 10:02, 10:03
	b = append(b, 0x55, 0xAA, 0x02, 4, 'c', 'a', 's', 'a') // nota
	b = append(b, 0x55, 0xAA, 0x01, 0x01, 0x2C)            // 300 em 10:04
	b = append(b, 0x55, 0x10)                              // 85 e 16 (10:05, 10:06)
	for i := 0; i < 2100; i++ {                            // enche mais de uma página
		b = append(b, 20)
	}
	stamp(26, 10, 28, 9, 0, 0, 1)
	b = append(b, 1, 0, 255, 2) // um 255 no meio é contagem, não fim
	return b
}

func checkHistory(t *testing.T, h History) {
	t.Helper()
	recs := h.Records
	if len(recs) != 3+1+2+2100+4 {
		t.Fatalf("%d registros", len(recs))
	}
	at := func(hh, mi, ss int, d int) time.Time {
		return time.Date(2026, 10, d, hh, mi, ss, 0, time.Local)
	}
	if !recs[0].Time.Equal(at(10, 1, 0, 27)) || recs[0].Count != 20 || recs[0].Unit != "CPM" {
		t.Fatalf("primeiro = %+v", recs[0])
	}
	if !recs[3].Time.Equal(at(10, 4, 0, 27)) || recs[3].Count != 300 {
		t.Fatalf("2 bytes = %+v", recs[3])
	}
	if recs[4].Count != 85 || recs[5].Count != 16 {
		t.Fatalf("0x55 isolado = %+v %+v", recs[4], recs[5])
	}
	last := recs[len(recs)-1]
	if last.Unit != "CPS" || last.Count != 2 || !last.Time.Equal(at(9, 0, 4, 28)) {
		t.Fatalf("último = %+v", last)
	}
	if recs[len(recs)-2].Count != 255 {
		t.Fatalf("penúltimo = %+v", recs[len(recs)-2])
	}
	if len(h.Notes) != 1 || h.Notes[0].Text != "casa" {
		t.Fatalf("notas = %+v", h.Notes)
	}
}

func TestParseHistoryStopsAtEmpty(t *testing.T) {
	img := buildHistoryImage()
	for i := 0; i < 500; i++ {
		img = append(img, 0xFF)
	}
	checkHistory(t, ParseHistory(img))
}

func TestParseHistoryIgnoresDataBeforeStamp(t *testing.T) {
	h := ParseHistory([]byte{10, 11, 12, 0x55, 0xAA, 0x00, 26, 13, 1, 0, 0, 0, 0x55, 0xAA, 2, 9})
	if len(h.Records) != 0 {
		t.Fatalf("data inválida deveria ser ignorada: %+v", h.Records)
	}
}

func TestNotFoundMessage(t *testing.T) {
	err := error(notFound{"COM13"})
	if err.Error() != "COM13 não encontrada" || !errors.Is(err, ErrNotFound) {
		t.Fatalf("%v", err)
	}
}
