package radiacode

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// USB no Windows pelo WinUSB (winusb.dll, sem instalar nada além do driver).
// O aparelho precisa estar com o driver WinUSB; se estiver com outro (ex.:
// libusb0 do app oficial), o erro diz qual é.

var (
	modWinUSB           = windows.NewLazySystemDLL("winusb.dll")
	procWinUsbInit      = modWinUSB.NewProc("WinUsb_Initialize")
	procWinUsbFree      = modWinUSB.NewProc("WinUsb_Free")
	procWinUsbWritePipe = modWinUSB.NewProc("WinUsb_WritePipe")
	procWinUsbReadPipe  = modWinUSB.NewProc("WinUsb_ReadPipe")
	procWinUsbSetPolicy = modWinUSB.NewProc("WinUsb_SetPipePolicy")
	procWinUsbFlush     = modWinUSB.NewProc("WinUsb_FlushPipe")
)

const pipeTransferTimeout = 0x03

type usbWin struct {
	file windows.Handle
	wu   uintptr
	as   *assembler
}

// usbFound descreve o aparelho achado no Windows.
type usbFound struct {
	instance string
	service  string // driver (WinUSB, libusb0, libusbK…)
	paths    []string
}

func findUSBWin() (*usbFound, error) {
	set, err := windows.SetupDiGetClassDevsEx(nil, "USB", 0, windows.DIGCF_PRESENT|windows.DIGCF_ALLCLASSES, 0, "")
	if err != nil {
		return nil, err
	}
	defer set.Close()
	for i := 0; ; i++ {
		info, err := set.EnumDeviceInfo(i)
		if err != nil {
			if errors.Is(err, windows.ERROR_NO_MORE_ITEMS) {
				break
			}
			continue
		}
		hw, _ := set.DeviceRegistryProperty(info, windows.SPDRP_HARDWAREID)
		ids, _ := hw.([]string)
		match := false
		for _, id := range ids {
			if strings.Contains(strings.ToUpper(id), "VID_0483&PID_F123") {
				match = true
			}
		}
		if !match {
			continue
		}
		f := &usbFound{}
		f.instance, _ = set.DeviceInstanceID(info)
		if s, err := set.DeviceRegistryProperty(info, windows.SPDRP_SERVICE); err == nil {
			f.service, _ = s.(string)
		}
		// GUID da interface que o driver registrou
		var guids []string
		if k, err := set.OpenDevRegKey(info, windows.DICS_FLAG_GLOBAL, 0, windows.DIREG_DEV, windows.KEY_READ); err == nil {
			rk := registry.Key(k)
			if v, _, err := rk.GetStringsValue("DeviceInterfaceGUIDs"); err == nil {
				guids = append(guids, v...)
			}
			if v, _, err := rk.GetStringValue("DeviceInterfaceGUID"); err == nil {
				guids = append(guids, v)
			}
			rk.Close()
		}
		// GUID genérico do WinUSB / dispositivos USB
		guids = append(guids, "{dee824ef-729b-4a0e-9c14-b7117d33a817}", "{A5DCBF10-6530-11D2-901F-00C04FB951ED}")
		for _, gs := range guids {
			g, err := windows.GUIDFromString(gs)
			if err != nil {
				continue
			}
			ps, err := windows.CM_Get_Device_Interface_List(f.instance, &g, windows.CM_GET_DEVICE_INTERFACE_LIST_PRESENT)
			if err == nil {
				f.paths = append(f.paths, ps...)
			}
		}
		return f, nil
	}
	return nil, ErrNotFound
}

// FindUSB diz se tem um Radiacode no USB (e qual driver ele está usando).
func FindUSB() (string, error) {
	f, err := findUSBWin()
	if err != nil {
		return "", err
	}
	return f.instance + " (driver " + f.service + ")", nil
}

// OpenUSB conecta pelo USB.
func OpenUSB() (Transport, error) {
	f, err := findUSBWin()
	if err != nil {
		return nil, err
	}
	var last error = fmt.Errorf("sem interface USB aberta")
	for _, p := range f.paths {
		u, err := openWinUSB(p)
		if err == nil {
			return u, nil
		}
		last = err
	}
	if !strings.EqualFold(f.service, "WinUSB") {
		return nil, fmt.Errorf("o RC102 está com o driver %q; o app precisa do WinUSB (%v)", f.service, last)
	}
	return nil, last
}

func openWinUSB(path string) (*usbWin, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OVERLAPPED, 0)
	if err != nil {
		return nil, fmt.Errorf("abrir %s: %w", path, err)
	}
	var wu uintptr
	if r, _, e := procWinUsbInit.Call(uintptr(h), uintptr(unsafe.Pointer(&wu))); r == 0 {
		windows.CloseHandle(h)
		return nil, fmt.Errorf("WinUsb_Initialize: %w", e)
	}
	u := &usbWin{file: h, wu: wu, as: newAssembler()}
	u.setTimeout(0x81, 100)
	tmp := make([]byte, 256)
	for i := 0; i < 16; i++ { // descarta sobras
		if n, err := u.read(tmp); err != nil || n == 0 {
			break
		}
	}
	u.setTimeout(0x81, 3000)
	u.setTimeout(0x01, 3000)
	return u, nil
}

func (u *usbWin) setTimeout(ep byte, ms uint32) {
	procWinUsbSetPolicy.Call(u.wu, uintptr(ep), pipeTransferTimeout, 4, uintptr(unsafe.Pointer(&ms)))
}

func (u *usbWin) read(p []byte) (int, error) {
	var n uint32
	r, _, e := procWinUsbReadPipe.Call(u.wu, 0x81, uintptr(unsafe.Pointer(&p[0])), uintptr(len(p)), uintptr(unsafe.Pointer(&n)), 0)
	if r == 0 {
		if errno, ok := e.(syscall.Errno); ok && errno == windows.ERROR_SEM_TIMEOUT {
			return 0, nil
		}
		return 0, e
	}
	return int(n), nil
}

func (u *usbWin) Exchange(req []byte) ([]byte, error) {
	var n uint32
	if r, _, e := procWinUsbWritePipe.Call(u.wu, 0x01, uintptr(unsafe.Pointer(&req[0])), uintptr(len(req)), uintptr(unsafe.Pointer(&n)), 0); r == 0 {
		return nil, fmt.Errorf("USB (envio): %w", e)
	}
	u.as = newAssembler()
	buf := make([]byte, 4096)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		n, err := u.read(buf)
		if err != nil {
			return nil, fmt.Errorf("USB (resposta): %w", err)
		}
		if n == 0 {
			continue
		}
		out, done, err := u.as.add(buf[:n])
		if err != nil {
			return nil, err
		}
		if done {
			return out, nil
		}
	}
	return nil, fmt.Errorf("USB: o RC102 não respondeu")
}

func (u *usbWin) Close() error {
	procWinUsbFree.Call(u.wu)
	return windows.CloseHandle(u.file)
}

func (u *usbWin) Kind() string { return "usb" }

var _ = procWinUsbFlush
