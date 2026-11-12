package app

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// writeFileAtomic grava data em path sem nunca deixar o arquivo pela metade:
// escreve num temporário ao lado, força a gravação no disco e só então
// troca o nome. Se a energia cair no meio, fica o arquivo antigo inteiro.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp, perm)
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}

// readStateJSON lê um JSON de estado (saídas, anotações, medições) em v.
// Se o arquivo existe mas está estragado, guarda uma cópia em .ilegivel e
// avisa em Eventos, em vez de começar vazio em silêncio e sobrescrever o
// arquivo na próxima gravação. Devolve false se não leu.
func (a *App) readStateJSON(path, what string, v any) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	if jerr := json.Unmarshal(b, v); jerr != nil {
		bad := path + ".ilegivel"
		_ = os.WriteFile(bad, b, 0o644)
		a.logf("%s: não consegui ler o %s (%v); comecei vazio e guardei o arquivo em %s", what, filepath.Base(path), jerr, bad)
		return false
	}
	return true
}
