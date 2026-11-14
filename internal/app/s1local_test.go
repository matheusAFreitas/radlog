package app

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// auditoria 2.79.0, S1: anexo de um tipo que pode levar a localização e que
// o app não sabe limpar (vídeo, TIFF, RAW) é guardado, mas fica só no PC:
// não vai para o backup, a nuvem nem a sincronização pela rede.
func TestAttachRiskyStaysLocal(t *testing.T) {
	a := newTestApp(t)
	addTestMeasure(a, 1, "Pedra", "", time.Now().Add(-time.Hour))
	a.mu.Lock()
	a.measurements[0].UID = "u1"
	a.mu.Unlock()
	dir := t.TempDir()
	mov := filepath.Join(dir, "video.mov")
	txt := filepath.Join(dir, "nota.txt")
	_ = os.WriteFile(mov, []byte("moov...gps"), 0o644)
	_ = os.WriteFile(txt, []byte("texto"), 0o644)
	list, err := a.attachFiles(1, []string{mov, txt})
	if !errors.Is(err, errAttachLocal) && (err == nil || !strings.Contains(err.Error(), errAttachLocal.Error())) {
		t.Fatalf("sem aviso: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("anexos: %d", len(list))
	}
	// backup
	zp := filepath.Join(t.TempDir(), "b.zip")
	if err := a.writeBackupFile(zp); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(zp)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	hasTxt := false
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, ".mov") {
			t.Fatal("o vídeo foi para o backup")
		}
		if strings.HasSuffix(f.Name, "nota.txt") {
			hasTxt = true
		}
	}
	if !hasTxt {
		t.Fatal("o texto não foi para o backup")
	}
	// nuvem
	packs, err := a.cloudPacks()
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, p := range packs {
		if p.att != nil {
			n++
			if strings.HasSuffix(p.att.file, ".mov") {
				t.Fatal("o vídeo vai para a nuvem")
			}
		}
	}
	if n != 1 {
		t.Fatalf("anexos na nuvem: %d", n)
	}
	// rede
	for _, k := range a.attachInventory() {
		if strings.HasSuffix(k, ".mov") {
			t.Fatal("o vídeo é oferecido pela rede")
		}
	}
}
