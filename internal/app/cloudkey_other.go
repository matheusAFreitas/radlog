//go:build !windows && !linux

package app

// Fora do Windows e do Linux os segredos da nuvem ficam num arquivo só do
// usuário (0600), sem cifra a mais.
func protect(_ string, b []byte) ([]byte, error) { return b, nil }

func unprotect(_ string, b []byte) ([]byte, error) { return b, nil }

func forgetProtect(string) {}

// a chave não vem de um cofre que pede senha: nada a esperar
func vaultCached(string) bool { return true }

func warmProtect(string, bool) error { return nil }
