package gmc

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

// cfg800Prefix são os primeiros 132 bytes de um <GETCFG>> gravado de um
// GMC-800 real (PyGMC, tests/test_gmc800.py). O resto é 0xFF.
const cfg800Prefix = "0000ff00ffffffffffffffffffffffffffffffffffffffffffff00ffffffffff01ffffffffffffffffffffff01ffffffffffffff000affffffffff0001ffffffffffffffff03ff0f2c0000060200003c140000782800012c64000258c80004b1904120000042c800004348000043fa0000447a000044fa0000000001010000006400ff06"

func cfg800() []byte {
	p, _ := hex.DecodeString(cfg800Prefix)
	out := bytes.Repeat([]byte{0xFF}, ConfigSize)
	copy(out, p)
	return out
}

// simPort finge ser um GMC-800 do outro lado do cabo.
type simPort struct {
	mu      sync.Mutex
	in      []byte
	out     []byte
	timeout time.Duration

	cpm, cps  uint32
	clock     time.Time
	flash     []byte
	spirExtra bool // imita o firmware que manda 1 byte a mais no SPIR
	heartbeat bool
	closed    bool
	silent    bool // não responde nada
	cfg       []byte
	dropWCFG  int // quantos WCFG ignorar em silêncio (simula falha)
	cfgLoaded int // quantos CFGUPDATE recebeu
}

func newSim() *simPort {
	flash := bytes.Repeat([]byte{0xFF}, FlashSize)
	return &simPort{
		timeout: 10 * time.Millisecond,
		cpm:     22, cps: 1,
		clock:     time.Date(2026, 10, 27, 18, 30, 0, 0, time.Local),
		flash:     flash,
		spirExtra: true,
		cfg:       cfg800(),
	}
}

func (s *simPort) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.in = append(s.in, p...)
	s.process()
	return len(p), nil
}

func (s *simPort) process() {
	for len(s.in) > 0 {
		switch {
		case bytes.HasPrefix(s.in, []byte("<SPIR")):
			if len(s.in) < 12 {
				return
			}
			a := int(s.in[5])<<16 | int(s.in[6])<<8 | int(s.in[7])
			n := int(s.in[8])<<8 | int(s.in[9])
			s.in = s.in[12:]
			if s.silent {
				continue
			}
			end := a + n
			if end > len(s.flash) {
				end = len(s.flash)
			}
			s.out = append(s.out, s.flash[a:end]...)
			if s.spirExtra {
				s.out = append(s.out, 0x00)
			}
		case bytes.HasPrefix(s.in, []byte("<WCFG")):
			if len(s.in) < 10 {
				return
			}
			a := int(s.in[5])<<8 | int(s.in[6])
			v := s.in[7]
			s.in = s.in[10:]
			if s.dropWCFG > 0 {
				s.dropWCFG--
			} else if a < len(s.cfg) {
				s.cfg[a] &= v // memória flash: sem apagar, só zera bits
			}
			if !s.silent {
				s.out = append(s.out, 0xAA)
			}
		case bytes.HasPrefix(s.in, []byte("<SETDATETIME")):
			if len(s.in) < 20 {
				return
			}
			d := s.in[12:18]
			s.clock = time.Date(2000+int(d[0]), time.Month(d[1]), int(d[2]), int(d[3]), int(d[4]), int(d[5]), 0, time.Local)
			s.in = s.in[20:]
			if !s.silent {
				s.out = append(s.out, 0xAA)
			}
		default:
			end := bytes.Index(s.in, []byte(">>"))
			if end < 0 {
				return
			}
			cmd := string(s.in[:end+2])
			s.in = s.in[end+2:]
			if !s.silent {
				s.answer(cmd)
			}
		}
	}
}

func (s *simPort) answer(cmd string) {
	u32 := func(v uint32) []byte { b := make([]byte, 4); binary.BigEndian.PutUint32(b, v); return b }
	switch strings.Trim(cmd, "<>") {
	case "GETVER":
		s.out = append(s.out, "GMC-800Re1.08"...)
	case "GETSERIAL":
		s.out = append(s.out, 0xA1, 0xB2, 0xC3, 0xD4, 0xE5, 0xF6, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06) // GMC-800: 12 bytes
	case "GETCPM":
		s.out = append(s.out, u32(s.cpm)...)
	case "GETCPS":
		s.out = append(s.out, u32(s.cps)...)
	case "GETDATETIME":
		c := s.clock
		s.out = append(s.out, byte(c.Year()-2000), byte(c.Month()), byte(c.Day()), byte(c.Hour()), byte(c.Minute()), byte(c.Second()), 0xAA)
	case "GETCFG":
		s.out = append(s.out, s.cfg...)
	case "ECFG":
		for i := range s.cfg {
			s.cfg[i] = 0xFF
		}
		s.out = append(s.out, 0xAA)
	case "CFGUPDATE":
		s.cfgLoaded++
		s.out = append(s.out, "0071\r\n"...)
		s.out = append(s.out, 0xAA)
	case "HEARTBEAT0":
		s.heartbeat = false
	case "HEARTBEAT1":
		s.heartbeat = true
	}
}

func (s *simPort) Read(p []byte) (int, error) {
	s.mu.Lock()
	if len(s.out) > 0 {
		n := copy(p, s.out)
		s.out = s.out[n:]
		s.mu.Unlock()
		return n, nil
	}
	t := s.timeout
	s.mu.Unlock()
	time.Sleep(t)
	return 0, nil
}

func (s *simPort) ResetInputBuffer() error {
	s.mu.Lock()
	s.out = nil
	s.mu.Unlock()
	return nil
}

func (s *simPort) SetReadTimeout(d time.Duration) error {
	s.mu.Lock()
	s.timeout = d
	s.mu.Unlock()
	return nil
}

func (s *simPort) Close() error {
	s.closed = true
	return nil
}
