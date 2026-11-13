package app

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"radlog/internal/store"
)

// ---------- migração GMC Monitor → RadLog (2.0) ----------
//
// Até a 1.x a pasta de dados era "GMC Monitor" (%AppData% no Windows,
// ~/.config no Linux). Na primeira abertura da 2.0, sem outra cópia do app
// aberta, o app:
//
//  1. copia tudo para a pasta "RadLog" (o que já existir lá não é tocado;
//     caches do WebView, que se refazem sozinhos, ficam de fora);
//  2. confere arquivo por arquivo (existe e tem o mesmo tamanho);
//  3. grava um backup completo da pasta antiga em RadLog/migracao/;
//  4. só então apaga a pasta antiga e grava migrado-de-gmc-monitor.json.
//
// Se algo falhar antes do passo 4, nada é apagado e o app continua usando a
// pasta antiga; tenta de novo na próxima abertura. Os relatórios em
// Documentos/GMC Monitor/Relatórios vão para Documentos/RadLog/Relatórios.

const (
	appDirName  = "RadLog"
	oldDirName  = "GMC Monitor"
	migMarkName = "migrado-de-gmc-monitor.json"
	migBackDir  = "migracao"
)

// caches do WebView2 (se refazem sozinhos): não vale copiar
var migSkipDirs = map[string]bool{
	"cache": true, "code cache": true, "gpucache": true, "dawncache": true, "dawngraphitecache": true,
	"dawnwebgpucache": true, "grshadercache": true, "shadercache": true, "crashpad": true,
}

// MigrationInfo é o que aconteceu na migração (fica no migrado-de-gmc-monitor.json).
type MigrationInfo struct {
	From    string   `json:"from"`
	To      string   `json:"to"`
	At      string   `json:"at"`
	Version string   `json:"version"`
	Files   int      `json:"files"`   // copiados
	Kept    int      `json:"kept"`    // já existiam na pasta nova (não mexidos)
	Bytes   int64    `json:"bytes"`   // copiados
	Backup  string   `json:"backup"`  // zip da pasta antiga
	Removed bool     `json:"removed"` // pasta antiga apagada
	Reports string   `json:"reports,omitempty"`
	Notes   []string `json:"notes,omitempty"`
}

// migResult é o resultado desta abertura (para o log, depois que ele existe).
type migResult struct {
	dir  string // pasta de dados a usar
	info *MigrationInfo
	err  error
	skip string // por que não migrou agora
}

var (
	migNow       migResult
	migDocsMoved int
	migDocsErr   error
)

// prepareDataDir resolve a pasta de dados (migrando se for o caso). Roda
// antes da janela abrir, porque o WebView2 guarda o dele dentro dela.
func prepareDataDir() migResult {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base, _ = os.Getwd()
	}
	return migrateDataDir(filepath.Join(base, oldDirName), filepath.Join(base, appDirName), otherInstanceRunning)
}

func migrateDataDir(oldDir, newDir string, running func() bool) migResult {
	res := migResult{dir: newDir}
	if st, err := os.Stat(oldDir); err != nil || !st.IsDir() {
		return res // nada a migrar (instalação nova ou já migrado)
	}
	if b, err := os.ReadFile(filepath.Join(newDir, migMarkName)); err == nil {
		// já migrou; a pasta antiga sobrou (não deu para apagar): tenta de novo
		var m MigrationInfo
		if json.Unmarshal(b, &m) == nil && !running() {
			if os.RemoveAll(oldDir) == nil {
				m.Removed = true
				m.Notes = append(m.Notes, "pasta antiga apagada em "+time.Now().Format(time.RFC3339))
				writeMigInfo(newDir, &m)
			}
		}
		return res
	}
	if running() {
		// outra cópia (a versão antiga) ainda está aberta e escrevendo: usa a
		// pasta antiga desta vez e migra na próxima
		return migResult{dir: oldDir, skip: "outra cópia do app ainda aberta"}
	}
	info, err := copyAndVerify(oldDir, newDir)
	if err != nil {
		return migResult{dir: oldDir, err: err}
	}
	// backup completo da pasta antiga antes de apagar
	zipPath := filepath.Join(newDir, migBackDir, "gmc-monitor-"+time.Now().Format("2006-01-02-150405")+".zip")
	if err := zipFolder(oldDir, zipPath); err != nil {
		return migResult{dir: newDir, info: info, err: fmt.Errorf("cópia feita, mas o backup da pasta antiga falhou (%v); ela não foi apagada", err)}
	}
	info.Backup = zipPath
	writeMigInfo(newDir, info) // a marca vem antes de apagar: a cópia já está conferida
	if err := os.RemoveAll(oldDir); err != nil {
		info.Notes = append(info.Notes, "não consegui apagar a pasta antiga: "+err.Error())
	} else {
		info.Removed = true
	}
	writeMigInfo(newDir, info)
	res.info = info
	return res
}

func writeMigInfo(dir string, m *MigrationInfo) {
	b, _ := json.MarshalIndent(m, "", "  ")
	_ = writeFileAtomic(filepath.Join(dir, migMarkName), b, 0o644)
}

// copyAndVerify copia oldDir para newDir (sem sobrescrever) e confere.
func copyAndVerify(oldDir, newDir string) (*MigrationInfo, error) {
	info := &MigrationInfo{From: oldDir, To: newDir, At: time.Now().Format(time.RFC3339), Version: AppVersion}
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		return nil, err
	}
	type cp struct {
		rel  string
		size int64
	}
	var copied []cp
	err := filepath.WalkDir(oldDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(oldDir, p)
		if d.IsDir() {
			if rel != "." && migSkipDirs[strings.ToLower(d.Name())] {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(newDir, rel), 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		dst := filepath.Join(newDir, rel)
		if _, err := os.Stat(dst); err == nil {
			info.Kept++ // a pasta nova já tem: não mexe
			return nil
		}
		n, err := copyFile(p, dst)
		if err != nil {
			return fmt.Errorf("copiando %s: %w", rel, err)
		}
		copied = append(copied, cp{rel, n})
		info.Files++
		info.Bytes += n
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, c := range copied {
		a, err1 := os.Stat(filepath.Join(oldDir, c.rel))
		b, err2 := os.Stat(filepath.Join(newDir, c.rel))
		if err1 != nil || err2 != nil || a.Size() != b.Size() || b.Size() != c.size {
			return nil, fmt.Errorf("conferência falhou em %s", c.rel)
		}
	}
	return info, nil
}

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

// zipFolder grava dir (sem os caches do WebView) num .zip e confere que
// abre com todos os arquivos.
func zipFolder(dir, zipPath string) error {
	if err := os.MkdirAll(filepath.Dir(zipPath), 0o755); err != nil {
		return err
	}
	tmp := zipPath + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)
	n := 0
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		if d.IsDir() {
			if rel != "." && migSkipDirs[strings.ToLower(d.Name())] {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		st, _ := in.Stat()
		h := &zip.FileHeader{Name: filepath.ToSlash(rel), Method: zip.Deflate}
		if st != nil {
			h.Modified = st.ModTime()
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		if _, err := io.Copy(w, in); err != nil {
			return err
		}
		n++
		return nil
	})
	if err2 := zw.Close(); err == nil {
		err = err2
	}
	if err2 := f.Close(); err == nil {
		err = err2
	}
	if err == nil {
		if r, e := zip.OpenReader(tmp); e != nil {
			err = e
		} else {
			if len(r.File) != n {
				err = fmt.Errorf("o zip ficou com %d de %d arquivos", len(r.File), n)
			}
			r.Close()
		}
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}
	if err := store.RenameRetry(tmp, zipPath); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// ---------- Documentos ----------

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

// migrateDocs leva Documentos/GMC Monitor/Relatórios para
// Documentos/RadLog/Relatórios (sem sobrescrever nada) e apaga as pastas
// antigas que ficarem vazias. A pasta Versões antiga fica: a versão aberta
// pode estar nela (o atualizador da 1.x baixa lá); as versões velhas saem
// sozinhas depois (cleanOldVersionsLater) e a pasta vazia sai na próxima.
func migrateDocs(docs string) (moved int, err error) {
	if docs == "" {
		return 0, nil
	}
	oldDocs, newDocs := filepath.Join(docs, oldDirName), filepath.Join(docs, appDirName)
	oldRep, newRep := filepath.Join(oldDocs, "Relatórios"), filepath.Join(newDocs, "Relatórios")
	if st, e := os.Stat(oldRep); e == nil && st.IsDir() {
		if _, e := os.Stat(newRep); os.IsNotExist(e) {
			if err = os.MkdirAll(newDocs, 0o755); err == nil {
				if err = os.Rename(oldRep, newRep); err == nil {
					moved = -1 // a pasta inteira
				}
			}
		} else {
			ents, _ := os.ReadDir(oldRep)
			for _, en := range ents {
				src, dst := filepath.Join(oldRep, en.Name()), filepath.Join(newRep, en.Name())
				if _, e := os.Stat(dst); e == nil {
					continue
				}
				if e := os.Rename(src, dst); e == nil {
					moved++
				} else {
					err = e
				}
			}
		}
	}
	os.Remove(oldRep)                            // só se ficou vazia
	os.Remove(filepath.Join(oldDocs, "Versões")) // idem
	os.Remove(oldDocs)                           // idem
	return moved, err
}

// oldVersionsDir é a pasta de versões da 1.x (Documentos/GMC Monitor/Versões).
func oldVersionsDir() string {
	if d := docsBase(); d != "" {
		return filepath.Join(d, oldDirName, "Versões")
	}
	return ""
}

// logMigration conta no log e nos eventos o que a migração fez.
func (a *App) logMigration(m migResult, docsMoved int, docsErr error) {
	switch {
	case m.skip != "":
		a.logf("migração GMC Monitor → RadLog adiada: %s (usando a pasta antiga %s)", m.skip, m.dir)
	case m.err != nil:
		a.logf("migração GMC Monitor → RadLog: %v (usando %s)", m.err, m.dir)
		a.logEvent("Não consegui terminar a mudança da pasta de dados para RadLog: " + m.err.Error() + ". Os dados continuam na pasta antiga; tento de novo ao abrir.")
	case m.info != nil:
		i := m.info
		a.logf("migração GMC Monitor → RadLog: %d arquivos copiados (%d KB), %d já existiam; backup %s; pasta antiga apagada: %v %s",
			i.Files, i.Bytes/1024, i.Kept, i.Backup, i.Removed, strings.Join(i.Notes, "; "))
		a.logEvent(fmt.Sprintf("Dados levados de \"GMC Monitor\" para \"RadLog\": %d arquivos, conferidos; backup da pasta antiga em migracao/", i.Files))
	}
	if docsMoved != 0 {
		a.logf("relatórios levados para Documentos/RadLog/Relatórios (%d)", docsMoved)
	}
	if docsErr != nil {
		a.logf("relatórios: não consegui levar tudo para Documentos/RadLog: %v", docsErr)
	}
}
