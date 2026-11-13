//go:build !windows

package app

// Fora do Windows os segredos da nuvem ficam num arquivo só do usuário
// (0600), sem cifra a mais.
func protect(b []byte) ([]byte, error) { return b, nil }

func unprotect(b []byte) ([]byte, error) { return b, nil }
