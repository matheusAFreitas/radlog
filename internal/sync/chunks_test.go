package sync_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	rs "radlog/internal/sync"
	"radlog/internal/sync/synctest"
)

func testDrive(t *testing.T) (*synctest.Server, *rs.Drive) {
	t.Helper()
	srv := synctest.New()
	t.Cleanup(srv.Close)
	d := rs.NewDrive(func(context.Context) (string, error) { return "tok", nil })
	d.Base, d.Upload = srv.Base(), srv.Upload()
	return srv, d
}

// N14: arquivo grande sobe em pedaços e continua de onde parou quando a
// conexão cai no meio.
func TestDrivePutChunksResume(t *testing.T) {
	old := rs.ChunkSize
	rs.ChunkSize = 256 << 10
	t.Cleanup(func() { rs.ChunkSize = old })
	srv, d := testDrive(t)
	ctx := context.Background()
	folder, err := d.CreateFolder(ctx, "RadLog")
	if err != nil {
		t.Fatal(err)
	}
	data := bytes.Repeat([]byte("radlog-0123456789"), 70000) // ~1,2 MB, 5 pedaços
	srv.Lock()
	srv.DropChunks = 2
	srv.Unlock()
	f, err := d.Put(ctx, folder, "", "grande", data)
	if err != nil {
		t.Fatal(err)
	}
	got, err := d.Get(ctx, f.ID)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("conteúdo: %d bytes, %v", len(got), err)
	}
	srv.Lock()
	n := srv.Chunks
	srv.Unlock()
	// 2 que caíram pela metade + os 4 do resto: continuou do meio, sem
	// recomeçar do zero (recomeçar daria 2 + 5)
	if n != 6 {
		t.Fatalf("pedaços: %d", n)
	}
	// trocar o conteúdo também vai em pedaços
	if _, err := d.Put(ctx, folder, f.ID, "", data[:600000]); err != nil {
		t.Fatal(err)
	}
	if got, _ := d.Get(ctx, f.ID); len(got) != 600000 {
		t.Fatalf("troca: %d", len(got))
	}
}

// N14: o download para no limite em vez de ler tudo na memória.
func TestDriveGetLimit(t *testing.T) {
	old := rs.MaxDownload
	rs.MaxDownload = 1000
	t.Cleanup(func() { rs.MaxDownload = old })
	_, d := testDrive(t)
	ctx := context.Background()
	folder, _ := d.CreateFolder(ctx, "RadLog")
	f, err := d.Put(ctx, folder, "", "x", make([]byte, 1001))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Get(ctx, f.ID); !errors.Is(err, rs.ErrTooBig) {
		t.Fatalf("passou do limite: %v", err)
	}
	g, _ := d.Put(ctx, folder, "", "y", make([]byte, 1000))
	if b, err := d.Get(ctx, g.ID); err != nil || len(b) != 1000 {
		t.Fatalf("no limite: %d %v", len(b), err)
	}
}

// QA da 2.80.5: 429 (limite de pedidos) espera e tenta de novo, em vez de
// desistir como os outros 4xx.
func TestDrivePutChunksThrottle(t *testing.T) {
	old := rs.ChunkSize
	rs.ChunkSize = 256 << 10
	t.Cleanup(func() { rs.ChunkSize = old })
	srv, d := testDrive(t)
	ctx := context.Background()
	folder, _ := d.CreateFolder(ctx, "RadLog")
	data := bytes.Repeat([]byte("x"), 600000)
	srv.Lock()
	srv.Throttle = 1
	srv.Unlock()
	f, err := d.Put(ctx, folder, "", "grande", data)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := d.Get(ctx, f.ID); !bytes.Equal(got, data) {
		t.Fatal("conteúdo")
	}
}
