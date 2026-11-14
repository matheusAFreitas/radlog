//go:build windows

package app

import (
	"errors"
	"runtime"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Área de transferência "privada" do Windows: o código de recuperação não
// entra no histórico (Win+V) nem sobe para a área de transferência na nuvem
// da Microsoft (QA da 2.80.3). O Wails não marca isso, então aqui o texto é
// gravado direto, junto com os formatos que o Windows respeita.

var (
	modUser32            = windows.NewLazySystemDLL("user32.dll")
	modKernel32          = windows.NewLazySystemDLL("kernel32.dll")
	procOpenClipboard    = modUser32.NewProc("OpenClipboard")
	procCloseClipboard   = modUser32.NewProc("CloseClipboard")
	procEmptyClipboard   = modUser32.NewProc("EmptyClipboard")
	procSetClipboardData = modUser32.NewProc("SetClipboardData")
	procRegisterFormat   = modUser32.NewProc("RegisterClipboardFormatW")
	procGlobalAlloc      = modKernel32.NewProc("GlobalAlloc")
	procGlobalLock       = modKernel32.NewProc("GlobalLock")
	procGlobalUnlock     = modKernel32.NewProc("GlobalUnlock")
	procGlobalFree       = modKernel32.NewProc("GlobalFree")
	procMoveMemory       = modKernel32.NewProc("RtlMoveMemory")
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002
)

// clipPrivateSet grava s como texto que não vai para o histórico nem para a
// nuvem da Microsoft.
func clipPrivateSet(s string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	opened := false
	for i := 0; i < 10; i++ {
		if r, _, _ := procOpenClipboard.Call(0); r != 0 {
			opened = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !opened {
		return errors.New("área de transferência ocupada")
	}
	defer procCloseClipboard.Call()
	if r, _, err := procEmptyClipboard.Call(); r == 0 {
		return err
	}
	u := utf16.Encode([]rune(s + "\x00"))
	text := unsafe.Slice((*byte)(unsafe.Pointer(&u[0])), len(u)*2)
	if err := clipPut(cfUnicodeText, text); err != nil {
		return err
	}
	zero := []byte{0, 0, 0, 0} // DWORD 0
	for _, name := range []string{"ExcludeClipboardContentFromMonitorProcessing", "CanIncludeInClipboardHistory", "CanUploadToCloudClipboard"} {
		p, err := windows.UTF16PtrFromString(name)
		if err != nil {
			return err
		}
		f, _, err := procRegisterFormat.Call(uintptr(unsafe.Pointer(p)))
		if f == 0 {
			return err
		}
		if err := clipPut(f, zero); err != nil {
			return err
		}
	}
	return nil
}

// clipPut copia data para um bloco global e entrega à área de transferência
// (que passa a ser dona do bloco).
func clipPut(format uintptr, data []byte) error {
	h, _, err := procGlobalAlloc.Call(gmemMoveable, uintptr(len(data)))
	if h == 0 {
		return err
	}
	p, _, err := procGlobalLock.Call(h)
	if p == 0 {
		procGlobalFree.Call(h)
		return err
	}
	procMoveMemory.Call(p, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)))
	procGlobalUnlock.Call(h)
	if r, _, err := procSetClipboardData.Call(format, h); r == 0 {
		procGlobalFree.Call(h)
		return err
	}
	return nil
}
