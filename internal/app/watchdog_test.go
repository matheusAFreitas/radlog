package app

import (
	"errors"
	"testing"
	"time"
)

func TestCallTimeout(t *testing.T) {
	v, err := callTimeout(func() (uint32, error) { return 42, nil }, time.Second)
	if v != 42 || err != nil {
		t.Fatal(v, err)
	}
	block := make(chan struct{})
	defer close(block)
	start := time.Now()
	_, err = callTimeout(func() (uint32, error) { <-block; return 0, nil }, 50*time.Millisecond)
	if !errors.Is(err, errStuck) || time.Since(start) > time.Second {
		t.Fatal("não detectou a leitura travada", err)
	}
}
