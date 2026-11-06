//go:build !linux && !windows

package radiacode

// FindUSB: USB só no Linux e no Windows por enquanto.
func FindUSB() (string, error) { return "", ErrNotFound }

// OpenUSB: USB só no Linux e no Windows por enquanto.
func OpenUSB() (Transport, error) { return nil, ErrNotFound }
