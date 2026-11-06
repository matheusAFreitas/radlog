//go:build !linux

package radiacode

import (
	"errors"
	"time"
)

// BLEDevice é um RadiaCode visto pelo Bluetooth.
type BLEDevice struct {
	Address string `json:"address"`
	Name    string `json:"name"`
	RSSI    int    `json:"rssi"`
	Paired  bool   `json:"paired"`
}

// ErrNoBLE: Bluetooth ainda não feito neste sistema.
var ErrNoBLE = errors.New("Bluetooth com o RadiaCode ainda só no Linux; no Windows, use o USB por enquanto")

// ScanBLE: ainda não feito fora do Linux.
func ScanBLE(time.Duration) ([]BLEDevice, error) { return nil, ErrNoBLE }

// OpenBLE: ainda não feito fora do Linux.
func OpenBLE(string) (Transport, error) { return nil, ErrNoBLE }

// BLESupported diz se este sistema conecta pelo Bluetooth.
const BLESupported = false
