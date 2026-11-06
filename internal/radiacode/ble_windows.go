package radiacode

import (
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/saltosystems/winrt-go"
	"github.com/saltosystems/winrt-go/windows/devices/bluetooth"
	"github.com/saltosystems/winrt-go/windows/devices/bluetooth/advertisement"
	"github.com/saltosystems/winrt-go/windows/devices/bluetooth/genericattributeprofile"
	"github.com/saltosystems/winrt-go/windows/foundation"
	"github.com/saltosystems/winrt-go/windows/storage/streams"
)

// Bluetooth no Windows pelas APIs WinRT (Windows 10 ou mais novo), sem cgo.
// O jeito de usar segue o tinygo-org/bluetooth (BSD), que usa as mesmas
// interfaces geradas pelo saltosystems/winrt-go.

// BLESupported diz se este sistema conecta pelo Bluetooth.
const BLESupported = true

var (
	winrtOnce sync.Once
	winrtErr  error
)

// winrtInit liga o WinRT em modo multithread numa thread que fica viva para
// sempre (com ela, as outras threads do processo entram no mesmo modo).
func winrtInit() error {
	winrtOnce.Do(func() {
		done := make(chan error)
		go func() {
			runtime.LockOSThread()
			err := ole.RoInitialize(1)
			if err != nil {
				var oe *ole.OleError
				if errors.As(err, &oe) && oe.Code() == 1 { // S_FALSE: já estava
					err = nil
				}
			}
			done <- err
			select {} // segura a thread
		}()
		winrtErr = <-done
	})
	return winrtErr
}

func await(op *foundation.IAsyncOperation, sig string) error {
	var status foundation.AsyncStatus
	iid := winrt.ParameterizedInstanceGUID(foundation.GUIDAsyncOperationCompletedHandler, sig)
	ch := make(chan struct{})
	h := foundation.NewAsyncOperationCompletedHandler(ole.NewGUID(iid), func(_ *foundation.AsyncOperationCompletedHandler, _ *foundation.IAsyncOperation, st foundation.AsyncStatus) {
		status = st
		close(ch)
	})
	if err := op.SetCompleted(h); err != nil {
		h.Release()
		return err
	}
	select {
	case <-ch:
		h.Release()
	case <-time.After(20 * time.Second):
		// não solta o handler: o Windows ainda pode chamá-lo depois
		return errors.New("o Windows não respondeu a tempo")
	}
	if status != foundation.AsyncStatusCompleted {
		return fmt.Errorf("operação do Bluetooth falhou (status %d)", status)
	}
	return nil
}

func parseMAC(addr string) (uint64, error) {
	s := strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(addr), ":", ""), "-", "")
	if len(s) != 12 {
		return 0, fmt.Errorf("endereço Bluetooth inválido: %q", addr)
	}
	return strconv.ParseUint(s, 16, 64)
}

func fmtMAC(v uint64) string {
	var p []string
	for i := 5; i >= 0; i-- {
		p = append(p, fmt.Sprintf("%02X", byte(v>>(8*i))))
	}
	return strings.Join(p, ":")
}

func guidString(g syscall.GUID) string {
	return strings.ToLower(fmt.Sprintf("%08x-%04x-%04x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		g.Data1, g.Data2, g.Data3, g.Data4[0], g.Data4[1], g.Data4[2], g.Data4[3], g.Data4[4], g.Data4[5], g.Data4[6], g.Data4[7]))
}

// ScanBLE procura RadiaCodes pelos anúncios Bluetooth.
func ScanBLE(d time.Duration) ([]BLEDevice, error) {
	if err := winrtInit(); err != nil {
		return nil, fmt.Errorf("Bluetooth: WinRT não iniciou (%w)", err)
	}
	w, err := advertisement.NewBluetoothLEAdvertisementWatcher()
	if err != nil {
		return nil, fmt.Errorf("Bluetooth: %w", err)
	}
	defer w.Release()
	w.SetScanningMode(advertisement.BluetoothLEScanningModeActive)
	var mu sync.Mutex
	found := map[uint64]*BLEDevice{}
	iid := winrt.ParameterizedInstanceGUID(foundation.GUIDTypedEventHandler,
		advertisement.SignatureBluetoothLEAdvertisementWatcher, advertisement.SignatureBluetoothLEAdvertisementReceivedEventArgs)
	h := foundation.NewTypedEventHandler(ole.NewGUID(iid), func(_ *foundation.TypedEventHandler, _, arg unsafe.Pointer) {
		args := (*advertisement.BluetoothLEAdvertisementReceivedEventArgs)(arg)
		addr, err := args.GetBluetoothAddress()
		if err != nil {
			return
		}
		rssi, _ := args.GetRawSignalStrengthInDBm()
		name := ""
		if adv, err := args.GetAdvertisement(); err == nil && adv != nil {
			name, _ = adv.GetLocalName()
			adv.Release()
		}
		mu.Lock()
		defer mu.Unlock()
		b := found[addr]
		if b == nil {
			b = &BLEDevice{Address: fmtMAC(addr)}
			found[addr] = b
		}
		if name != "" {
			b.Name = name
		}
		b.RSSI = int(rssi)
	})
	defer h.Release()
	tok, err := w.AddReceived(h)
	if err != nil {
		return nil, err
	}
	defer w.RemoveReceived(tok)
	if err := w.Start(); err != nil {
		return nil, fmt.Errorf("Bluetooth: não deu para procurar (o Bluetooth do PC está ligado?) %w", err)
	}
	time.Sleep(d)
	w.Stop()
	mu.Lock()
	defer mu.Unlock()
	var out []BLEDevice
	for _, b := range found {
		if isRadiacodeName(b.Name) {
			out = append(out, *b)
		}
	}
	return out, nil
}

type bleWin struct {
	dev     *bluetooth.BluetoothLEDevice
	session *genericattributeprofile.GattSession
	write   *genericattributeprofile.GattCharacteristic
	notify  *genericattributeprofile.GattCharacteristic
	svcs    []*genericattributeprofile.GattDeviceService
	vh      *foundation.TypedEventHandler
	vtok    foundation.EventRegistrationToken
	sh      *foundation.TypedEventHandler
	stok    foundation.EventRegistrationToken

	mu     sync.Mutex
	as     *assembler
	resp   chan []byte
	lost   bool
	closed bool
}

// OpenBLE conecta pelo Bluetooth no endereço dado (AA:BB:CC:DD:EE:FF).
func OpenBLE(addr string) (Transport, error) {
	if err := winrtInit(); err != nil {
		return nil, fmt.Errorf("Bluetooth: WinRT não iniciou (%w)", err)
	}
	mac, err := parseMAC(addr)
	if err != nil {
		return nil, err
	}
	op, err := bluetooth.BluetoothLEDeviceFromBluetoothAddressAsync(mac)
	if err != nil {
		return nil, fmt.Errorf("Bluetooth: %w", err)
	}
	err = await(op, bluetooth.SignatureBluetoothLEDevice)
	var res unsafe.Pointer
	if err == nil {
		res, err = op.GetResults()
	}
	op.Release()
	if err != nil {
		return nil, fmt.Errorf("Bluetooth: %w", err)
	}
	if res == nil {
		return nil, fmt.Errorf("RadiaCode %s não apareceu no Bluetooth (ligado e perto? o celular não está conectado nele?)", fmtMAC(mac))
	}
	b := &bleWin{dev: (*bluetooth.BluetoothLEDevice)(res), as: newAssembler(), resp: make(chan []byte, 1)}
	fail := func(err error) (Transport, error) { b.Close(); return nil, err }

	// sessão GATT com "manter conectado": é o que faz o Windows conectar
	id, err := b.dev.GetBluetoothDeviceId()
	if err != nil {
		return fail(err)
	}
	sop, err := genericattributeprofile.GattSessionFromDeviceIdAsync(id)
	id.Release()
	if err != nil {
		return fail(err)
	}
	err = await(sop, genericattributeprofile.SignatureGattSession)
	if err == nil {
		res, err = sop.GetResults()
	}
	sop.Release()
	if err != nil {
		return fail(fmt.Errorf("Bluetooth: sessão: %w", err))
	}
	b.session = (*genericattributeprofile.GattSession)(res)
	if err := b.session.SetMaintainConnection(true); err != nil {
		return fail(err)
	}

	// queda da conexão
	siid := winrt.ParameterizedInstanceGUID(foundation.GUIDTypedEventHandler, bluetooth.SignatureBluetoothLEDevice, "cinterface(IInspectable)")
	b.sh = foundation.NewTypedEventHandler(ole.NewGUID(siid), func(_ *foundation.TypedEventHandler, _, _ unsafe.Pointer) {
		st, err := b.dev.GetConnectionStatus()
		if err == nil && st == bluetooth.BluetoothConnectionStatusDisconnected {
			b.mu.Lock()
			b.lost = true
			b.mu.Unlock()
			select {
			case b.resp <- nil:
			default:
			}
		}
	})
	if b.stok, err = b.dev.AddConnectionStatusChanged(b.sh); err != nil {
		return fail(err)
	}

	// serviços e características (sem cache: o aparelho pode ter mudado)
	var lastErr error
	for try := 0; try < 3 && (b.write == nil || b.notify == nil); try++ {
		if try > 0 {
			time.Sleep(time.Second)
		}
		lastErr = b.discover()
	}
	if b.write == nil || b.notify == nil {
		if lastErr == nil {
			lastErr = errors.New("o aparelho não mostrou o serviço do RadiaCode")
		}
		return fail(fmt.Errorf("Bluetooth: %w (o celular está conectado nele?)", lastErr))
	}

	// notificações: as respostas chegam aqui
	viid := winrt.ParameterizedInstanceGUID(foundation.GUIDTypedEventHandler, genericattributeprofile.SignatureGattCharacteristic, genericattributeprofile.SignatureGattValueChangedEventArgs)
	b.vh = foundation.NewTypedEventHandler(ole.NewGUID(viid), func(_ *foundation.TypedEventHandler, _, arg unsafe.Pointer) {
		ev := (*genericattributeprofile.GattValueChangedEventArgs)(arg)
		buf, err := ev.GetCharacteristicValue()
		if err != nil {
			return
		}
		data := bufBytes(buf)
		buf.Release()
		b.mu.Lock()
		out, done, err := b.as.add(data)
		if err != nil {
			b.as = newAssembler()
		}
		b.mu.Unlock()
		if done {
			select {
			case b.resp <- out:
			default:
			}
		}
	})
	if b.vtok, err = b.notify.AddValueChanged(b.vh); err != nil {
		return fail(err)
	}
	wop, err := b.notify.WriteClientCharacteristicConfigurationDescriptorAsync(genericattributeprofile.GattClientCharacteristicConfigurationDescriptorValueNotify)
	if err != nil {
		return fail(err)
	}
	err = await(wop, genericattributeprofile.SignatureGattCommunicationStatus)
	if err == nil {
		res, err = wop.GetResults()
	}
	wop.Release()
	if err != nil {
		return fail(fmt.Errorf("Bluetooth: ligar notificações: %w", err))
	}
	if st := genericattributeprofile.GattCommunicationStatus(uintptr(res)); st != genericattributeprofile.GattCommunicationStatusSuccess {
		return fail(fmt.Errorf("Bluetooth: o aparelho recusou as notificações (status %d)", st))
	}
	return b, nil
}

func bufBytes(buf *streams.IBuffer) []byte {
	n, err := buf.GetLength()
	if err != nil || n == 0 {
		return nil
	}
	r, err := streams.DataReaderFromBuffer(buf)
	if err != nil {
		return nil
	}
	defer r.Release()
	data, _ := r.ReadBytes(n)
	return data
}

func (b *bleWin) discover() error {
	op, err := b.dev.GetGattServicesWithCacheModeAsync(bluetooth.BluetoothCacheModeUncached)
	if err != nil {
		return err
	}
	defer op.Release()
	if err := await(op, genericattributeprofile.SignatureGattDeviceServicesResult); err != nil {
		return err
	}
	res, err := op.GetResults()
	if err != nil {
		return err
	}
	sr := (*genericattributeprofile.GattDeviceServicesResult)(res)
	defer sr.Release()
	if st, err := sr.GetStatus(); err != nil || st != genericattributeprofile.GattCommunicationStatusSuccess {
		return fmt.Errorf("não consegui ler os serviços (status %d)", st)
	}
	vec, err := sr.GetServices()
	if err != nil {
		return err
	}
	defer vec.Release()
	n, _ := vec.GetSize()
	for i := uint32(0); i < n; i++ {
		p, err := vec.GetAt(i)
		if err != nil {
			continue
		}
		svc := (*genericattributeprofile.GattDeviceService)(p)
		b.svcs = append(b.svcs, svc) // fecha no Close (senão o Windows segura o aparelho)
		cop, err := svc.GetCharacteristicsWithCacheModeAsync(bluetooth.BluetoothCacheModeUncached)
		if err != nil {
			continue
		}
		if await(cop, genericattributeprofile.SignatureGattCharacteristicsResult) != nil {
			cop.Release()
			continue
		}
		cres, err := cop.GetResults()
		cop.Release()
		if err != nil {
			continue
		}
		cr := (*genericattributeprofile.GattCharacteristicsResult)(cres)
		cvec, err := cr.GetCharacteristics()
		if err != nil {
			cr.Release()
			continue
		}
		cn, _ := cvec.GetSize()
		for j := uint32(0); j < cn; j++ {
			cp, err := cvec.GetAt(j)
			if err != nil {
				continue
			}
			ch := (*genericattributeprofile.GattCharacteristic)(cp)
			g, err := ch.GetUuid()
			if err != nil {
				continue
			}
			switch guidString(g) {
			case bleWriteUUID:
				b.write = ch
			case bleNotifyUUID:
				b.notify = ch
			}
		}
		cvec.Release()
		cr.Release()
	}
	return nil
}

func (b *bleWin) writeChunk(p []byte) error {
	w, err := streams.NewDataWriter()
	if err != nil {
		return err
	}
	defer w.Release()
	if err := w.WriteBytes(uint32(len(p)), p); err != nil {
		return err
	}
	buf, err := w.DetachBuffer()
	if err != nil {
		return err
	}
	defer buf.Release()
	op, err := b.write.WriteValueWithOptionAsync(buf, genericattributeprofile.GattWriteOptionWriteWithoutResponse)
	if err != nil {
		return err
	}
	defer op.Release()
	if err := await(op, genericattributeprofile.SignatureGattCommunicationStatus); err != nil {
		return err
	}
	res, err := op.GetResults()
	if err != nil {
		return err
	}
	if st := genericattributeprofile.GattCommunicationStatus(uintptr(res)); st != genericattributeprofile.GattCommunicationStatusSuccess {
		return fmt.Errorf("escrita recusada (status %d)", st)
	}
	return nil
}

func (b *bleWin) Exchange(req []byte) ([]byte, error) {
	b.mu.Lock()
	if b.lost || b.closed {
		b.mu.Unlock()
		return nil, errors.New("Bluetooth: conexão perdida")
	}
	b.as = newAssembler()
	b.mu.Unlock()
	select {
	case <-b.resp:
	default:
	}
	for p := 0; p < len(req); p += bleChunk {
		if err := b.writeChunk(req[p:min(p+bleChunk, len(req))]); err != nil {
			return nil, fmt.Errorf("Bluetooth (envio): %w", err)
		}
	}
	select {
	case out := <-b.resp:
		if out == nil {
			return nil, errors.New("Bluetooth: conexão perdida")
		}
		return out, nil
	case <-time.After(10 * time.Second):
		return nil, errors.New("Bluetooth: o RadiaCode não respondeu")
	}
}

func (b *bleWin) Close() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	b.mu.Unlock()
	if b.notify != nil && b.vh != nil {
		if op, err := b.notify.WriteClientCharacteristicConfigurationDescriptorAsync(genericattributeprofile.GattClientCharacteristicConfigurationDescriptorValueNone); err == nil {
			await(op, genericattributeprofile.SignatureGattCommunicationStatus)
			op.Release()
		}
		b.notify.RemoveValueChanged(b.vtok)
		b.vh.Release()
	}
	for _, s := range b.svcs {
		s.Close()
		s.Release()
	}
	if b.session != nil {
		b.session.SetMaintainConnection(false)
		b.session.Close()
		b.session.Release()
	}
	if b.dev != nil {
		if b.sh != nil {
			b.dev.RemoveConnectionStatusChanged(b.stok)
			b.sh.Release()
		}
		b.dev.Close()
		b.dev.Release()
	}
	return nil
}

func (b *bleWin) Kind() string { return "bluetooth" }
