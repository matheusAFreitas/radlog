//go:build !windows

package app

import "errors"

// clipPrivateSet só existe no Windows; nos outros sistemas vale o do Wails.
func clipPrivateSet(string) error { return errors.New("sem área de transferência privada") }
