package i18n

import "testing"

// 19/11: registro do celular do ACE com português sobrando (o motivo do
// Kotlin dentro do molde, "nada chegou; sinal…", o "(repetiu mais…)" depois
// da mensagem em inglês do RC102)
func TestLogBLEInglesCompleto(t *testing.T) {
	for in, want := range map[string]string{
		"RC102: não conectou (Bluetooth: o RC102 não respondeu (longe, desligado ou com outro aparelho)); próxima tentativa pelo Bluetooth em 1m0s": "RC102: did not connect (Bluetooth: the RC102 didn't respond (too far, off, or with another device)); next Bluetooth try in 1m0s",
		"RC102: não conectou (Bluetooth: o aparelho não aceitou a conexão (status 133)); próxima tentativa pelo Bluetooth em 2m0s":                  "RC102: did not connect (Bluetooth: the device refused the connection (status 133)); next Bluetooth try in 2m0s",
		"RC102 (Bluetooth): sem resposta em 4s ao pedido 0826 (DATA_BUF): nada chegou · sinal -63 dBm · o rádio ainda responde (-62 dBm)":           "RC102 (Bluetooth): no answer in 4s to request 0826 (DATA_BUF): nothing arrived · signal -63 dBm · the radio still answers (-62 dBm)",
		"RC102 (mensagem do aparelho, 03:21:12): BLE: client connected (repetiu mais 1 vezes desde 01:24)":                                          "RC102 (device message, 03:21:12): BLE: client connected (repeated 1 more times since 01:24)",
		"==== RadLog 3.0.0 iniciado (android/arm64, go1.27.4, PC localhost)":                                                                        "==== RadLog 3.0.0 started (android/arm64, go1.27.4, PC localhost)",
		"modo celular (Android)": "phone mode (Android)",
	} {
		if got := T("en", in); got != want {
			t.Errorf("\n in:  %s\n got: %s\nwant: %s", in, got, want)
		}
	}
}
