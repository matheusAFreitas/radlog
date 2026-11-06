package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestClassifyLog(t *testing.T) {
	cases := map[string]string{
		"conexão: abrindo COM13":                                                                    lvDebug,
		"reconexão: COM13 não encontrada":                                                           lvDebug,
		"conexão: não abriu COM13: COM13 não encontrada":                                            lvDebug,
		"conexão: ok, GMC-800Re1.20 em COM13, série 1":                                              lvInfo,
		"leitura ao vivo: erro 1/3 em GETCPM: Access is denied.":                                    lvWarn,
		"leitura ao vivo: erro 3/3 em GETCPM: Access is denied.":                                    lvWarn,
		"leitura ao vivo: erro 3/3 em GETCPM: A device attached to the system is not functioning.":  lvError,
		"leitura ao vivo: COM13 sumiu da lista de portas do Windows: cabo desconectado ou USB caiu": lvWarn,
		"leitura ao vivo: GETCPM sem resposta em 6 s (porta travada)":                               lvError,
		"reconexão: falhou: não consegui abrir COM13: Invalid serial port":                          lvWarn,
		"evento: Conexão com o contador perdida":                                                    lvWarn,
		"==== GMC Monitor 1.10.1 iniciado":                                                          lvInfo,
	}
	for msg, want := range cases {
		if got := classifyLog(msg); got != want {
			t.Errorf("%q: %s, esperava %s", msg, got, want)
		}
	}
	if c := logComponent("portas (depois de 3 erros): COM3"); c != "portas" {
		t.Errorf("componente: %q", c)
	}
}

func TestGetLogs(t *testing.T) {
	a, dir := testApp(t)
	a.dataDir = dir
	now := time.Now()
	old := now.Add(-2*time.Hour).Format("2006-01-02 15:04:05.000") + "  leitura ao vivo: GETCPM sem resposta em 6 s (porta travada)\n"
	_ = os.WriteFile(filepath.Join(dir, "log.txt"), []byte(old), 0o644)
	a.logf("conexão: abrindo COM13")
	a.logf("conexão: ok, GMC em COM13")
	r, err := a.GetLogs(LogQuery{Levels: []string{"INFO", "WARN", "ERROR"}})
	if err != nil {
		t.Fatal(err)
	}
	if r.Total != 2 || r.Counts[lvDebug] != 1 || r.Counts[lvError] != 1 || r.Entries[0].Level != lvInfo {
		t.Fatalf("resultado: total %d counts %v primeiro %+v", r.Total, r.Counts, r.Entries[0])
	}
	r, _ = a.GetLogs(LogQuery{Text: "travada"})
	if r.Total != 1 || r.Entries[0].Comp != "leitura ao vivo" {
		t.Fatalf("busca: %+v", r)
	}
	// em inglês a mensagem sai traduzida; o componente fica (é o filtro)
	a.SetUILang("en")
	r, _ = a.GetLogs(LogQuery{Text: "travada"})
	if r.Total != 1 || r.Entries[0].Comp != "leitura ao vivo" || r.Entries[0].Msg != "leitura ao vivo: GETCPM with no answer in 6 s (port locked)" {
		t.Fatalf("inglês: %+v", r.Entries)
	}
}
