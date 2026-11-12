package radiacode

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// USB no Linux direto pelo usbfs (/dev/bus/usb), sem libusb. Precisa de
// permissão no arquivo do aparelho: a regra do udev
//   SUBSYSTEMS=="usb", ATTRS{idVendor}=="0483", ATTRS{idProduct}=="f123", TAG+="uaccess", MODE="0660"
// em /etc/udev/rules.d/99-radiacode.rules.

const (
	usbdevfsBulk             = 0xC0185502 // _IOWR('U', 2, struct usbdevfs_bulktransfer) em 64 bits
	usbdevfsClaimInterface   = 0x8004550F
	usbdevfsReleaseInterface = 0x80045510
)

type bulkTransfer struct {
	ep, length, timeout uint32
	_                   uint32
	data                uintptr
}

type usbLinux struct {
	f  *os.File
	as *assembler
}

// FindUSB acha o primeiro Radiacode ligado no USB e devolve o caminho dele.
func FindUSB() (string, error) {
	devs, _ := filepath.Glob("/sys/bus/usb/devices/*")
	for _, d := range devs {
		if rd(d, "idVendor") != "0483" || rd(d, "idProduct") != "f123" {
			continue
		}
		bus, _ := strconv.Atoi(rd(d, "busnum"))
		dev, _ := strconv.Atoi(rd(d, "devnum"))
		return fmt.Sprintf("/dev/bus/usb/%03d/%03d", bus, dev), nil
	}
	return "", ErrNotFound
}

func rd(dir, name string) string {
	b, _ := os.ReadFile(filepath.Join(dir, name))
	return strings.TrimSpace(string(b))
}

// OpenUSB conecta pelo USB.
func OpenUSB() (Transport, error) {
	path, err := FindUSB()
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		if os.IsPermission(err) {
			return nil, fmt.Errorf("sem permissão em %s: instale a regra do udev do RC102", path)
		}
		return nil, err
	}
	iface := uint32(0)
	if err := ioctl(f, usbdevfsClaimInterface, uintptr(unsafe.Pointer(&iface))); err != nil {
		f.Close()
		return nil, fmt.Errorf("não consegui pegar o USB do RC102 (outro programa usando?): %w", err)
	}
	u := &usbLinux{f: f, as: newAssembler()}
	// descarta o que tiver sobrado de uma conversa anterior
	tmp := make([]byte, 256)
	for i := 0; i < 16; i++ {
		if n, err := u.bulk(0x81, tmp, 100); err != nil || n == 0 {
			break
		}
	}
	return u, nil
}

func ioctl(f *os.File, req, arg uintptr) error {
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), req, arg)
	if e != 0 {
		return e
	}
	return nil
}

func (u *usbLinux) bulk(ep uint32, p []byte, timeoutMs uint32) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	bt := bulkTransfer{ep: ep, length: uint32(len(p)), timeout: timeoutMs, data: uintptr(unsafe.Pointer(&p[0]))}
	n, _, e := syscall.Syscall(syscall.SYS_IOCTL, u.f.Fd(), usbdevfsBulk, uintptr(unsafe.Pointer(&bt)))
	if e != 0 {
		if e == syscall.ETIMEDOUT {
			return 0, nil
		}
		return 0, e
	}
	return int(n), nil
}

func (u *usbLinux) Exchange(req []byte) ([]byte, error) {
	if _, err := u.bulk(0x01, req, 3000); err != nil {
		return nil, fmt.Errorf("USB (envio): %w", err)
	}
	u.as = newAssembler()
	buf := make([]byte, 4096)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		n, err := u.bulk(0x81, buf, 3000)
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

func (u *usbLinux) Close() error {
	iface := uint32(0)
	ioctl(u.f, usbdevfsReleaseInterface, uintptr(unsafe.Pointer(&iface)))
	return u.f.Close()
}

func (u *usbLinux) Kind() string { return "usb" }
