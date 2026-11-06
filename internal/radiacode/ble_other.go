//go:build !linux && !windows

package radiacode

import (
	"errors"
	"time"
)

// ErrNoBLE: Bluetooth não feito neste sistema.
var ErrNoBLE = errors.New("Bluetooth com o RadiaCode só no Windows e no Linux")

// BLESupported diz se este sistema conecta pelo Bluetooth.
const BLESupported = false

// ScanBLE: não feito neste sistema.
func ScanBLE(time.Duration) ([]BLEDevice, error) { return nil, ErrNoBLE }

// OpenBLE: não feito neste sistema.
func OpenBLE(string) (Transport, error) { return nil, ErrNoBLE }
