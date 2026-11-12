package app

import (
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
