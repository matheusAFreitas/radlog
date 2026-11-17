package app

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"radlog/internal/i18n"
	"radlog/internal/pdf"
)

// 2.101.1: todo texto escrito num PDF com o tradutor ligado tem de sair em
// inglês. Os testes que geram PDF em inglês passam por aqui; no fim, o que
// ainda parece português (e não tem tradução) derruba os testes.
var (
	ptLeftMu sync.Mutex
	ptLeft   = map[string]bool{}
	ptLooks  = regexp.MustCompile(`(?i)[ãçõâêôáéíóúà]|\b(não|nenhum|nenhuma|aqui|ainda|com|para|sem|uma|pelo|pela|desde|agora|hoje|ontem|aparelho|contagens|fundo|objeto|medição|limite|tempo|vezes|pico|picos|espectro|casa|dias|horas|minutos|segundos|da|das|dos|na|nos|nas|em|de|que|ou|mais|menos|muito|pouco|chuva|fora|medida|série|filho|chumbo|tório|urânio|até|sobre|entre|cada|quando|onde|como)\b`)
)

// nome que a pessoa deu à medição nos testes: fica como foi escrito
const ptUserName = "Eletrodo de tungstênio toriado"

func TestMain(m *testing.M) {
	pdf.SetTextHook(func(s string) {
		if ptLooks.MatchString(s) && i18n.T("en", s) == s && !strings.Contains(s, ptUserName) {
			ptLeftMu.Lock()
			ptLeft[s] = true
			ptLeftMu.Unlock()
		}
	})
	code := m.Run()
	if len(ptLeft) > 0 && os.Getenv("RADLOG_PT_OK") == "" {
		var l []string
		for s := range ptLeft {
			l = append(l, s)
		}
		sort.Strings(l)
		fmt.Fprintf(os.Stderr, "PDF em inglês com %d texto(s) em português:\n  %s\n", len(l), strings.Join(l, "\n  "))
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

// 2.101.1: bandeja e título da janela em inglês, em todos os estados.
func TestTrayEnglish(t *testing.T) {
	var ds []trayDev
	for _, n := range []string{"GMC-800", "RC102"} {
		ds = append(ds, trayDev{Name: n, State: "off"}, trayDev{Name: n, State: "wait"},
			trayDev{Name: n, State: "ok", Sat: true, SatAbove: 1000}, trayDev{Name: n, State: "ok", Sat: true},
			trayDev{Name: n, State: "ok", NoCount: true}, trayDev{Name: n, State: "ok", USv: 0.12},
			trayDev{Name: n, State: "ok", USv: 0.12, Away: true})
		for _, lv := range []string{"atencao", "alarme", "perigo"} {
			ds = append(ds, trayDev{Name: n, State: "ok", USv: 1.5, Level: lv})
		}
	}
	var txt []string
	for _, d := range ds {
		txt = append(txt, trayLine(d, "."))
	}
	txt = append(txt, "ALARME", "PERIGO", "ALARME (demo)", "PERIGO (demo)", "Abrir o RadLog", "Mostra a janela", "Sair", "Fecha o app (para de monitorar)")
	for _, s := range txt {
		if en := i18n.T("en", s); ptLooks.MatchString(en) {
			t.Errorf("%q → %q", s, en)
		}
	}
}
