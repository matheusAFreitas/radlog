package radonscan

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// fakePort responde como o RadonScan da sondagem de 23/11/2026.
type fakePort struct {
	mu      sync.Mutex
	out     []byte
	answers map[string]string
	skip    int // quantas perguntas ignorar em silêncio (1º comando depois de abrir)
	sent    []string
}

func (f *fakePort) Write(b []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cmd := string(b)
	f.sent = append(f.sent, cmd)
	if f.skip > 0 {
		f.skip--
		return len(b), nil
	}
	f.out = append(f.out, f.answers[cmd]...)
	return len(b), nil
}

func (f *fakePort) Read(b []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.out) == 0 {
		f.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
		f.mu.Lock()
		return 0, nil
	}
	n := copy(b, f.out)
	f.out = f.out[n:]
	return n, nil
}

func (f *fakePort) ResetInputBuffer() error            { f.mu.Lock(); f.out = nil; f.mu.Unlock(); return nil }
func (f *fakePort) SetReadTimeout(time.Duration) error { return nil }
func (f *fakePort) Close() error                       { return nil }

func probed() *fakePort {
	return &fakePort{answers: map[string]string{
		"<GETVER>>":         "RadonScanRe2.02",
		"<GETPCI1HOUR>>":    "0.33\r\n",
		"<GETPCI1DAY>>":     "0.41\r\n",
		"<GETPCI1WEEK>>":    "0.52\r\n",
		"<GETPCILONGTERM>>": "1.20\r\n",
	}}
}

func TestVersionAndAverages(t *testing.T) {
	d := New(probed())
	v, err := d.Version()
	if err != nil || v != "RadonScanRe2.02" || !IsRadonScan(v) {
		t.Fatalf("versão %q, %v", v, err)
	}
	a, err := d.ReadAverages()
	if err != nil {
		t.Fatal(err)
	}
	if a != (Averages{0.33, 0.41, 0.52, 1.20}) {
		t.Fatalf("médias %+v", a)
	}
}

// Na sondagem, o GETVER logo depois de abrir a porta voltou vazio uma vez.
func TestRetriesFirstSilentAnswer(t *testing.T) {
	p := probed()
	p.skip = 1
	v, err := New(p).Version()
	if err != nil || v != "RadonScanRe2.02" {
		t.Fatalf("versão %q, %v", v, err)
	}
	if len(p.sent) != 2 {
		t.Fatalf("mandou %d vezes", len(p.sent))
	}
}

func TestOnlyReadCommands(t *testing.T) {
	p := probed()
	d := New(p)
	_, _ = d.Version()
	_, _ = d.ReadAverages()
	for _, c := range p.sent {
		if !strings.HasPrefix(c, "<GET") {
			t.Fatalf("mandou um comando que não é de leitura: %q", c)
		}
	}
}

func TestParseNumber(t *testing.T) {
	for in, want := range map[string]float64{"0.33\r\n": 0.33, "12\r\n": 12, " 4.0 \r\n0.1": 4} {
		if v, err := ParseNumber([]byte(in)); err != nil || v != want {
			t.Errorf("%q: %v %v", in, v, err)
		}
	}
	for _, bad := range []string{"", "abc\r\n", "-1\r\n", "\xb0\x09R273"} {
		if _, err := ParseNumber([]byte(bad)); err == nil {
			t.Errorf("%q devia falhar", bad)
		}
	}
	if IsRadonScan("GMC-800Re1.08") {
		t.Error("GMC não é RadonScan")
	}
	if !IsRadonScan("RadonProRe2.02") {
		t.Error("RadonPro também serve")
	}
}

func TestSilentDeviceTimesOut(t *testing.T) {
	p := &fakePort{answers: map[string]string{}}
	if _, err := New(p).Version(); err == nil {
		t.Fatal("devia dar erro")
	}
}
