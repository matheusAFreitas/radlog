package app

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// rcScanner lê linhas sem o limite de 64 KB do bufio.Scanner: uma linha
// grande demais (arquivo estragado) não esconde mais o resto do mês
// (auditoria 2.76.0, B11).
func rcScanner(r io.Reader) *bufio.Scanner {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 16<<20)
	return sc
}

// rcBad junta as linhas que um arquivo do Radiacode tinha e que não vão
// voltar quando ele for regravado. Elas vão para <arquivo>.ilegivel, como
// já acontece com os meses do GMC-800, em vez de sumirem sem cópia.
type rcBad []string

// add guarda l se ela não for vazia nem um cabeçalho ("hora,..." em
// todos os arquivos do Radiacode).
func (b *rcBad) add(l string) {
	l = strings.TrimRight(l, "\r")
	if l == "" || strings.HasPrefix(l, rcDoseHeader[:5]) {
		return
	}
	*b = append(*b, l)
}

// save acrescenta as linhas em path.ilegivel (só depois de o arquivo novo
// ter sido gravado).
func (b rcBad) save(a *App, path string) {
	if len(b) == 0 {
		return
	}
	bad := path + ".ilegivel"
	f, err := os.OpenFile(bad, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	_, err = f.WriteString(strings.Join(b, "\n") + "\n")
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		a.logf("%s: %d linha(s) ilegível(is) guardada(s) em %s", filepath.Base(path), len(b), filepath.Base(bad))
	}
}
