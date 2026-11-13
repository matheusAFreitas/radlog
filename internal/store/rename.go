package store

import (
	"errors"
	"io/fs"
	"os"
	"time"
)

// RenameRetry troca tmp por path. No Windows a troca falha com "acesso
// negado" enquanto outra parte do app (ou o antivírus) está com o arquivo
// aberto para leitura; isso passa em milissegundos. Então tenta de novo por
// até 1 s antes de desistir (auditoria 2.76.0, B3).
func RenameRetry(tmp, path string) error {
	deadline := time.Now().Add(time.Second)
	for {
		err := os.Rename(tmp, path)
		if err == nil || errors.Is(err, fs.ErrNotExist) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
}
