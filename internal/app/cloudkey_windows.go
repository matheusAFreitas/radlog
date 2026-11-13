//go:build windows

package app

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// protect cifra os segredos da nuvem com o DPAPI do Windows (só este usuário
// neste PC abre).
func protect(b []byte) ([]byte, error) { return dpapi(b, true) }

// unprotect desfaz o protect.
func unprotect(b []byte) ([]byte, error) { return dpapi(b, false) }

func dpapi(b []byte, enc bool) ([]byte, error) {
	if len(b) == 0 {
		return nil, nil
	}
	in := windows.DataBlob{Size: uint32(len(b)), Data: &b[0]}
	var out windows.DataBlob
	var err error
	if enc {
		err = windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	} else {
		err = windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	}
	if err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, out.Size)...), nil
}
