package app

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"time"

	"radlog/internal/gmc"
)

// ---------- diagnóstico: o GMC-800 informa a dose que soma? (2.2) ----------
//
// O protocolo da GQ não tem comando documentado para a dose acumulada que o
// contador mostra na tela. Este teste só lê: pega a configuração duas vezes
// com ~90 s de intervalo (se a dose estiver lá, algum número anda) e manda
// alguns comandos <GET…>> candidatos. Nada é gravado no aparelho. O
// resultado vai para um arquivo de texto.

var gmcDiagWait = 90 * time.Second

// comandos conhecidos (controle: têm de responder) e candidatos
var gmcDiagKnown = []string{"GETVER", "GETSERIAL", "GETDATETIME", "GETTEMP", "GETVOLT", "GETGYRO", "GETMAXCPS", "GETCPMH", "GETCPML"}
var gmcDiagTry = []string{"GETDOSE", "GETUSV", "GETUSVH", "GETDOSERATE", "GETTOTALDOSE", "GETACCDOSE", "GETACCUMDOSE",
	"GETTOTAL", "GETTOTALCOUNT", "GETCOUNT", "GETCOUNTS", "GETINFO", "GETSTATUS", "GETSTAT", "GETDOSEALL"}

type gmcDiagRound struct {
	at  time.Time
	cfg []byte
	ans map[string][]byte
	err map[string]string
}

// GMCDiag é o resumo do diagnóstico.
type GMCDiag struct {
	Path       string   `json:"path"`
	CfgChanged int      `json:"cfgChanged"` // bytes da configuração que mudaram
	Unknown    []string `json:"unknown"`    // candidatos que responderam
	Changed    []string `json:"changed"`    // comandos cuja resposta mudou
	Verdict    string   `json:"verdict"`
}

func (a *App) gmcDiagRound(d *gmc.Device) gmcDiagRound {
	r := gmcDiagRound{at: time.Now(), ans: map[string][]byte{}, err: map[string]string{}}
	if cfg, err := d.Config(); err == nil {
		r.cfg = cfg
	} else {
		r.err["GETCFG"] = err.Error()
	}
	for _, c := range append(append([]string{}, gmcDiagKnown...), gmcDiagTry...) {
		b, err := d.Probe("<"+c+">>", 400*time.Millisecond)
		if err != nil && !errors.Is(err, gmc.ErrTimeout) {
			r.err[c] = err.Error()
		}
		if len(b) > 0 {
			r.ans[c] = b
		}
	}
	return r
}

// DiagnoseGMCDose roda o teste (leva ~2 min; a leitura ao vivo pausa uns
// segundos em cada rodada) e grava o relatório na pasta de dados.
func (a *App) DiagnoseGMCDose() (GMCDiag, error) {
	var r1, r2 gmcDiagRound
	a.emit("gmcdiag", "1ª leitura")
	if err := a.exclusive(func(d *gmc.Device) error { r1 = a.gmcDiagRound(d); return nil }); err != nil {
		return GMCDiag{}, err
	}
	a.emit("gmcdiag", "esperando")
	time.Sleep(gmcDiagWait)
	a.emit("gmcdiag", "2ª leitura")
	if err := a.exclusive(func(d *gmc.Device) error { r2 = a.gmcDiagRound(d); return nil }); err != nil {
		return GMCDiag{}, err
	}
	out, txt := gmcDiagReport(r1, r2)
	a.mu.Lock()
	info := a.info
	a.mu.Unlock()
	head := fmt.Sprintf("RadLog %s — diagnóstico da dose do GMC\nContador: %s\nLeituras: %s e %s\n\n",
		AppVersion, info.Version, r1.at.Format("02/01/2006 15:04:05"), r2.at.Format("15:04:05"))
	out.Path = filepath.Join(a.dataDir, "diagnostico-dose-gmc-"+time.Now().Format("2006-01-02_150405")+".txt")
	if err := writeFileAtomic(out.Path, []byte(head+txt), 0o644); err != nil {
		return out, err
	}
	a.logEvent("Diagnóstico da dose do GMC: " + out.Verdict)
	a.emit("gmcdiag", "")
	return out, nil
}

func showBytes(b []byte) string {
	if len(b) == 0 {
		return "(sem resposta)"
	}
	s := hex.EncodeToString(b)
	if len(s) > 96 {
		s = s[:96] + "…"
	}
	asc := []byte{}
	for _, c := range b {
		if c >= 32 && c < 127 {
			asc = append(asc, c)
		} else {
			asc = append(asc, '.')
		}
	}
	if len(asc) > 48 {
		asc = append(asc[:48], "…"...)
	}
	return fmt.Sprintf("%s  |%s|  (%d bytes)", s, asc, len(b))
}

// gmcDiagReport compara as duas rodadas.
func gmcDiagReport(r1, r2 gmcDiagRound) (GMCDiag, string) {
	var out GMCDiag
	var sb strings.Builder
	sb.WriteString("Nada foi gravado no contador: só comandos de leitura.\n\n")

	sb.WriteString("1. Configuração (GETCFG) lida duas vezes\n")
	switch {
	case r1.cfg == nil || r2.cfg == nil:
		sb.WriteString("   não consegui ler a configuração\n")
	default:
		var offs []int
		for i := 0; i < len(r1.cfg) && i < len(r2.cfg); i++ {
			if r1.cfg[i] != r2.cfg[i] {
				offs = append(offs, i)
			}
		}
		out.CfgChanged = len(offs)
		if len(offs) == 0 {
			sb.WriteString("   nenhum byte mudou: a dose acumulada não fica na configuração\n")
		}
		seen := map[int]bool{}
		for _, o := range offs {
			fmt.Fprintf(&sb, "   byte %d (0x%03x): %02x → %02x\n", o, o, r1.cfg[o], r2.cfg[o])
			al := o - o%4
			if seen[al] || al+4 > len(r1.cfg) {
				continue
			}
			seen[al] = true
			a, b := r1.cfg[al:al+4], r2.cfg[al:al+4]
			fmt.Fprintf(&sb, "      bytes %d–%d: uint32 BE %d → %d · LE %d → %d · float BE %g → %g · LE %g → %g\n", al, al+3,
				binary.BigEndian.Uint32(a), binary.BigEndian.Uint32(b), binary.LittleEndian.Uint32(a), binary.LittleEndian.Uint32(b),
				f32(binary.BigEndian.Uint32(a)), f32(binary.BigEndian.Uint32(b)), f32(binary.LittleEndian.Uint32(a)), f32(binary.LittleEndian.Uint32(b)))
		}
	}

	sb.WriteString("\n2. Comandos conhecidos (controle)\n")
	for _, c := range gmcDiagKnown {
		fmt.Fprintf(&sb, "   %-14s %s\n", c, showBytes(r1.ans[c]))
		if !bytesEq(r1.ans[c], r2.ans[c]) {
			fmt.Fprintf(&sb, "   %-14s %s  (2ª leitura)\n", "", showBytes(r2.ans[c]))
			out.Changed = append(out.Changed, c)
		}
	}
	sb.WriteString("\n3. Comandos candidatos (não documentados)\n")
	for _, c := range gmcDiagTry {
		fmt.Fprintf(&sb, "   %-14s %s\n", c, showBytes(r1.ans[c]))
		if len(r1.ans[c]) > 0 || len(r2.ans[c]) > 0 {
			out.Unknown = append(out.Unknown, c)
		}
		if !bytesEq(r1.ans[c], r2.ans[c]) {
			fmt.Fprintf(&sb, "   %-14s %s  (2ª leitura)\n", "", showBytes(r2.ans[c]))
			out.Changed = append(out.Changed, c)
		}
	}
	for c, e := range r1.err {
		fmt.Fprintf(&sb, "   erro em %s: %s\n", c, e)
	}

	switch {
	case len(out.Unknown) > 0:
		out.Verdict = "candidatos responderam (" + strings.Join(out.Unknown, ", ") + "); mande o arquivo para análise"
	case out.CfgChanged > 0:
		out.Verdict = fmt.Sprintf("%d bytes da configuração mudaram; mande o arquivo para análise", out.CfgChanged)
	default:
		out.Verdict = "nenhum comando de leitura informa a dose do aparelho; o app continua calculando a dose"
	}
	sb.WriteString("\nConclusão: " + out.Verdict + "\n")
	return out, sb.String()
}

func f32(u uint32) float64 {
	v := float64(math.Float32frombits(u))
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

func bytesEq(a, b []byte) bool { return string(a) == string(b) }
