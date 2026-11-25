package app

import (
	"io"
	"os"
	"path/filepath"
)

// appDirName é o nome da pasta de dados (na pasta de configuração do
// usuário) e da pasta de relatórios (em Documentos). 3.0.0: a migração da
// pasta "GMC Monitor" da 1.x saiu; quem vem da 1.x passa antes por uma 2.x.
const appDirName = "RadLog"

// copyFile copia src para dst (por um temporário, mantendo a hora do
// arquivo).
func copyFile(src, dst string) (int64, error) {
	in, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer in.Close()
	st, _ := in.Stat()
	tmp := dst + ".copiando"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(out, in)
	if err2 := out.Sync(); err == nil {
		err = err2
	}
	if err2 := out.Close(); err == nil {
		err = err2
	}
	if err != nil {
		os.Remove(tmp)
		return 0, err
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return 0, err
	}
	if st != nil {
		os.Chtimes(dst, st.ModTime(), st.ModTime())
	}
	return n, nil
}

// docsBase é a pasta Documentos (ou Documents) do usuário; senão a pessoal.
func docsBase() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	for _, n := range []string{"Documents", "Documentos"} {
		if st, err := os.Stat(filepath.Join(home, n)); err == nil && st.IsDir() {
			return filepath.Join(home, n)
		}
	}
	return home
}
