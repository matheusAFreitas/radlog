package app

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"radlog/internal/radiacode"
)

// A versão antiga pôs o buffer recente na hora da reconexão: a reposição
// relê os dados brutos, preenche os minutos e tira as linhas erradas do log.
func TestRcMemRawBackfill(t *testing.T) {
	a := newTestApp(t)
	start := time.Date(2026, 11, 10, 13, 48, 43, 0, time.Local)
	pc := start.Add(2 * time.Second)
	base := start.Add(radiacode.BaseLead)
	raw := filepath.Join(a.rcDir(), rcRawDir)
	os.MkdirAll(raw, 0o755)
	os.WriteFile(filepath.Join(raw, "conexao-"+start.Format("20060102-150405")+".json"), []byte("{}"), 0o644)

	// lote de 10 pontos (96 s), o último 3 min antes da conexão, sem a marca
	first := start.Add(-3*time.Minute - 9*96*time.Second)
	off := int32(first.Sub(start) / (10 * time.Millisecond)) // conta do zerar do relógio
	var b bytes.Buffer
	w := func(v ...any) {
		for _, x := range v {
			binary.Write(&b, binary.LittleEndian, x)
		}
	}
	w(uint8(0), uint8(1), uint8(2), off, uint16(10), uint32(9600))
	for i := 0; i < 10; i++ {
		w(uint32(1200), float32(12.5), float32(1.4e-5), uint16(200), uint16(0x1000))
	}
	line, _ := json.Marshal(map[string]any{"pc": pc.UnixMilli(), "t": base.Add(time.Duration(off) * 10 * time.Millisecond).UnixMilli(), "hex": hex.EncodeToString(b.Bytes())})
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	zw.Write(append(line, '\n'))
	zw.Close()
	os.WriteFile(filepath.Join(raw, start.Format("2006-01-02-150405")+".jsonl.gz"), gz.Bytes(), 0o644)

	// o que a versão antiga gravou: as 10 linhas na hora da reconexão
	var bad []RcLogItem
	for i := 0; i < 10; i++ {
		bad = append(bad, *rcLogFromRecord(radiacode.DoseRateDB{Count: 1200, CPS: 12.5, DoseRate: 0.14, DoseErr: 20, Flags: 0x1000}, pc))
	}
	a.rcLogAppend(bad[:1])

	a.rcMemRawBackfill()
	pts := a.rcRange(first.Add(-2*time.Minute), start)
	if len(pts) < 15 {
		t.Fatalf("minutos repostos: %d", len(pts))
	}
	if math.Abs(pts[1].USvh-0.14) > 1e-6 {
		t.Errorf("taxa: %+v", pts[1])
	}
	lg := a.GetRcLog(first.Add(-time.Hour).UnixMilli(), start.Add(time.Hour).UnixMilli(), []string{"taxa"}, 100, 0)
	for _, it := range lg.Items {
		if it.Src != rcMemSrc {
			t.Errorf("sobrou linha na hora errada: %+v", it)
		}
	}
	if lg.Total != 10 {
		t.Errorf("log: %d linhas (quero 10)", lg.Total)
	}
	// uma vez só
	if _, err := os.Stat(filepath.Join(a.rcDir(), rcMemRawMark)); err != nil {
		t.Error("sem a marca")
	}
}
