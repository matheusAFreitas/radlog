//go:build android

package radiacode

// No Android o RC102 só entra pelo Bluetooth (o USB do celular fica para
// depois, se fizer falta).

// FindUSB: sem USB no Android.
func FindUSB() (string, error) { return "", ErrNotFound }

// OpenUSB: sem USB no Android.
func OpenUSB() (Transport, error) { return nil, ErrNotFound }
