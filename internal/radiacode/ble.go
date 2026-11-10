package radiacode

import "strings"

// Bluetooth: o Radiacode tem um serviço próprio com uma característica de
// escrita (pedidos em pedaços de 18 bytes, sem resposta) e uma de
// notificação (as respostas chegam em pedaços).
const (
	bleWriteUUID  = "e63215e6-7003-49d8-96b0-b024798fb901"
	bleNotifyUUID = "e63215e7-7003-49d8-96b0-b024798fb901"
	bleChunk      = 18
)

// BLEDevice é um Radiacode visto pelo Bluetooth.
type BLEDevice struct {
	Address string `json:"address"`
	Name    string `json:"name"`
	RSSI    int    `json:"rssi"`
	Paired  bool   `json:"paired"`
}

func isRadiacodeName(name string) bool {
	n := strings.ToLower(name)
	return strings.HasPrefix(n, "radiacode") || strings.HasPrefix(n, "rc-1")
}
