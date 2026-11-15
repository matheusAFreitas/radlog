//go:build android

package radiacode

import (
	"time"
)

// No Android o Bluetooth é do sistema: quem fala com o rádio é o app em
// Kotlin, que se registra aqui (SetBLEHost). O Go manda os pedidos em pedaços
// de 18 bytes pelo host e recebe as notificações por BLENotify, juntando a
// resposta com o mesmo assembler do Linux e do Windows.

// BLESupported: o Android tem Bluetooth (pelo host do app).
const BLESupported = true

// ScanBLE procura Radiacodes pelo host.
func ScanBLE(d time.Duration) ([]BLEDevice, error) { return hostScan(d) }

// OpenBLE conecta pelo host.
func OpenBLE(addr string) (Transport, error) { return hostOpen(addr) }
