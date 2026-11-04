package gmc

import "testing"

func TestFormatSerial(t *testing.T) {
	b := []byte{0xA1, 0xB2, 0xC3, 0xD4, 0xE5, 0xF6, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
	if s := FormatSerial(b); s != "A1 B2 C3 D4 E5 F6 01 02 03 04 05 06" {
		t.Fatal(s)
	}
}
