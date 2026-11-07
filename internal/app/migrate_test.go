package app

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateDataDir(t *testing.T) {
	base := t.TempDir()
	oldDir, newDir := filepath.Join(base, "GMC Monitor"), filepath.Join(base, "RadLog")
	write := func(dir, rel, txt string) {
		p := filepath.Join(dir, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(txt), 0o644)
	}
	write(oldDir, "leituras/2026-10.csv", "data_hora,cpm,origem\n2026-10-27T20:18:00-03:00,25,ao vivo\n")
	write(oldDir, "eventos.csv", "x\n")
	write(oldDir, "radiacode/espectros/2026-11-07-120000.jsonl.gz", "gz")
	write(oldDir, "webview/EBWebView/Default/Local Storage/leveldb/000003.log", "lang=pt")
	write(oldDir, "webview/EBWebView/Default/Cache/data_0", "cache que se refaz")
	write(newDir, "config.json", `{"nova":true}`) // a pasta nova já tinha isto: não mexe
	write(oldDir, "config.json", `{"velha":true}`)

	// outra cópia aberta: não mexe e usa a pasta antiga
	if r := migrateDataDir(oldDir, newDir, func() bool { return true }); r.dir != oldDir || r.skip == "" {
		t.Fatalf("com outra cópia aberta: %+v", r)
	}
	if _, err := os.Stat(filepath.Join(newDir, "eventos.csv")); err == nil {
		t.Fatalf("não deveria ter copiado")
	}

	r := migrateDataDir(oldDir, newDir, func() bool { return false })
	if r.err != nil || r.dir != newDir || r.info == nil {
		t.Fatalf("migração: %+v", r)
	}
	i := r.info
	if i.Files != 4 || i.Kept != 1 || !i.Removed || i.Backup == "" {
		t.Fatalf("info: %+v", i)
	}
	if b, _ := os.ReadFile(filepath.Join(newDir, "leituras/2026-10.csv")); len(b) == 0 {
		t.Fatalf("leituras não vieram")
	}
	if b, _ := os.ReadFile(filepath.Join(newDir, "config.json")); string(b) != `{"nova":true}` {
		t.Fatalf("sobrescreveu a pasta nova: %s", b)
	}
	if _, err := os.Stat(filepath.Join(newDir, "webview/EBWebView/Default/Local Storage/leveldb/000003.log")); err != nil {
		t.Fatalf("o localStorage da tela tem que vir junto")
	}
	if _, err := os.Stat(filepath.Join(newDir, "webview/EBWebView/Default/Cache/data_0")); err == nil {
		t.Fatalf("cache não precisa vir")
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Fatalf("a pasta antiga deveria ter saído")
	}
	// o backup tem tudo da pasta antiga (menos o cache), inclusive o config
	zr, err := zip.OpenReader(i.Backup)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}
	zr.Close()
	if len(names) != 5 || !names["config.json"] || !names["leituras/2026-10.csv"] {
		t.Fatalf("backup: %v", names)
	}
	var m MigrationInfo
	b, _ := os.ReadFile(filepath.Join(newDir, migMarkName))
	if json.Unmarshal(b, &m) != nil || !m.Removed || m.Version != AppVersion {
		t.Fatalf("marca: %s", b)
	}

	// abriu de novo: nada a fazer
	if r := migrateDataDir(oldDir, newDir, func() bool { return false }); r.dir != newDir || r.info != nil || r.err != nil {
		t.Fatalf("segunda vez: %+v", r)
	}
	// a pasta antiga reapareceu (a 1.x foi aberta de novo): com a marca, só tenta apagar
	write(oldDir, "eventos.csv", "y\n")
	if r := migrateDataDir(oldDir, newDir, func() bool { return false }); r.dir != newDir {
		t.Fatalf("depois da marca: %+v", r)
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Fatalf("pasta antiga deveria sair")
	}

	// instalação nova: não existe a pasta antiga
	if r := migrateDataDir(filepath.Join(base, "nada"), filepath.Join(base, "RadLog2"), func() bool { return false }); r.dir != filepath.Join(base, "RadLog2") || r.info != nil {
		t.Fatalf("nova: %+v", r)
	}
}

func TestMigrateDataDirCopyFails(t *testing.T) {
	base := t.TempDir()
	oldDir, newDir := filepath.Join(base, "GMC Monitor"), filepath.Join(base, "RadLog")
	os.MkdirAll(filepath.Join(oldDir, "leituras"), 0o755)
	os.WriteFile(filepath.Join(oldDir, "leituras", "2026-10.csv"), []byte("x"), 0o644)
	// a pasta nova é um arquivo: não dá para copiar → continua na antiga, nada apagado
	os.WriteFile(newDir, []byte("x"), 0o644)
	r := migrateDataDir(oldDir, newDir, func() bool { return false })
	if r.err == nil || r.dir != oldDir {
		t.Fatalf("deveria falhar e usar a antiga: %+v", r)
	}
	if _, err := os.Stat(filepath.Join(oldDir, "leituras", "2026-10.csv")); err != nil {
		t.Fatalf("a pasta antiga não pode sumir")
	}
}

func TestMigrateDocs(t *testing.T) {
	docs := t.TempDir()
	old := filepath.Join(docs, "GMC Monitor")
	os.MkdirAll(filepath.Join(old, "Relatórios"), 0o755)
	os.MkdirAll(filepath.Join(old, "Versões"), 0o755)
	os.WriteFile(filepath.Join(old, "Relatórios", "gmc-relatorio.pdf"), []byte("%PDF"), 0o644)
	os.WriteFile(filepath.Join(old, "Versões", "GMC-Monitor-v1.46.1.exe"), []byte("exe"), 0o644)
	if _, err := migrateDocs(docs); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(docs, "RadLog", "Relatórios", "gmc-relatorio.pdf")); err != nil {
		t.Fatalf("o relatório tinha que ir para RadLog")
	}
	// a pasta de versões fica (pode ter a versão aberta)
	if _, err := os.Stat(filepath.Join(old, "Versões", "GMC-Monitor-v1.46.1.exe")); err != nil {
		t.Fatalf("Versões não pode sumir com arquivo dentro")
	}
	// já vazia: na próxima, as pastas antigas saem
	os.Remove(filepath.Join(old, "Versões", "GMC-Monitor-v1.46.1.exe"))
	os.MkdirAll(filepath.Join(old, "Relatórios"), 0o755)
	os.WriteFile(filepath.Join(old, "Relatórios", "outro.pdf"), []byte("%PDF"), 0o644)
	if n, err := migrateDocs(docs); err != nil || n != 1 {
		t.Fatalf("segunda: %d %v", n, err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("Documentos/GMC Monitor vazia deveria sair")
	}
}

func TestOldBackupStillImports(t *testing.T) {
	a := newTestApp(t)
	zp := filepath.Join(t.TempDir(), "gmc-backup.zip")
	f, _ := os.Create(zp)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("backup.json")
	w.Write([]byte(`{"app":"gmc-monitor","version":"1.46.1","host":"ACENULL","created":"2026-11-07T10:00:00-03:00"}`))
	w, _ = zw.Create("leituras/2026-10.csv")
	w.Write([]byte("data_hora,cpm,origem\n2026-10-27T20:18:00-03:00,25,ao vivo\n"))
	zw.Close()
	f.Close()
	res, err := a.mergeZip(zp, true)
	if err != nil || res.Minutes != 1 {
		t.Fatalf("backup da 1.x: %+v %v", res, err)
	}
	// e um zip qualquer continua recusado
	zp2 := filepath.Join(t.TempDir(), "outro.zip")
	f, _ = os.Create(zp2)
	zw = zip.NewWriter(f)
	w, _ = zw.Create("backup.json")
	w.Write([]byte(`{"app":"outro"}`))
	zw.Close()
	f.Close()
	if _, err := a.mergeZip(zp2, true); err == nil {
		t.Fatalf("zip de outro app deveria ser recusado")
	}
}
