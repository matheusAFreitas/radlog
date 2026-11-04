package gmc

import (
	"testing"
	"time"
)

// porta cuja leitura trava de vez (como o driver no Windows com o USB
// suspenso) até alguém fechar.
type stuckPort struct{ closed chan struct{} }

func (p *stuckPort) Read(b []byte) (int, error)  { <-p.closed; return 0, ErrTimeout }
func (p *stuckPort) Write(b []byte) (int, error) { return len(b), nil }
func (p *stuckPort) ResetInputBuffer() error     { return nil }
func (p *stuckPort) SetReadTimeout(time.Duration) error {
	return nil
}
func (p *stuckPort) Close() error {
	select {
	case <-p.closed:
	default:
		close(p.closed)
	}
	return nil
}

func TestCloseWhileReadIsStuck(t *testing.T) {
	p := &stuckPort{closed: make(chan struct{})}
	d := &Device{port: p}
	go func() { _, _ = d.CPM() }() // fica presa segurando o mutex
	time.Sleep(50 * time.Millisecond)
	done := make(chan struct{})
	go func() { _ = d.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Close travou esperando a leitura presa")
	}
}
