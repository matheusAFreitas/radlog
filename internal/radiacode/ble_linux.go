package radiacode

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

// Bluetooth no Linux pelo BlueZ (D-Bus), sem nada a instalar além do
// bluetoothd que já vem no sistema.

const bluez = "org.bluez"

func managedObjects(conn *dbus.Conn) (map[dbus.ObjectPath]map[string]map[string]dbus.Variant, error) {
	var objs map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	err := conn.Object(bluez, "/").Call("org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&objs)
	return objs, err
}

func adapterPath(objs map[dbus.ObjectPath]map[string]map[string]dbus.Variant) dbus.ObjectPath {
	for p, ifs := range objs {
		if _, ok := ifs["org.bluez.Adapter1"]; ok {
			return p
		}
	}
	return ""
}

func str(v dbus.Variant) string { s, _ := v.Value().(string); return s }

// ScanBLE procura RadiaCodes por alguns segundos.
func ScanBLE(d time.Duration) ([]BLEDevice, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, fmt.Errorf("Bluetooth: sem D-Bus (%w)", err)
	}
	objs, err := managedObjects(conn)
	if err != nil {
		return nil, fmt.Errorf("Bluetooth: BlueZ não respondeu (%w)", err)
	}
	ad := adapterPath(objs)
	if ad == "" {
		return nil, fmt.Errorf("nenhum adaptador Bluetooth")
	}
	a := conn.Object(bluez, ad)
	if v, err := a.GetProperty("org.bluez.Adapter1.Powered"); err == nil {
		if on, _ := v.Value().(bool); !on {
			return nil, fmt.Errorf("o Bluetooth do PC está desligado")
		}
	}
	a.Call("org.bluez.Adapter1.SetDiscoveryFilter", 0, map[string]interface{}{"Transport": "le"})
	if call := a.Call("org.bluez.Adapter1.StartDiscovery", 0); call.Err == nil {
		time.Sleep(d)
		a.Call("org.bluez.Adapter1.StopDiscovery", 0)
	}
	objs, err = managedObjects(conn)
	if err != nil {
		return nil, err
	}
	var out []BLEDevice
	for _, ifs := range objs {
		dev, ok := ifs["org.bluez.Device1"]
		if !ok {
			continue
		}
		name := str(dev["Name"])
		if name == "" {
			name = str(dev["Alias"])
		}
		if !isRadiacodeName(name) {
			continue
		}
		b := BLEDevice{Address: str(dev["Address"]), Name: name}
		if v, ok := dev["RSSI"]; ok {
			if r, ok := v.Value().(int16); ok {
				b.RSSI = int(r)
			}
		}
		if v, ok := dev["Paired"]; ok {
			b.Paired, _ = v.Value().(bool)
		}
		out = append(out, b)
	}
	return out, nil
}

type bleLinux struct {
	conn   *dbus.Conn
	dev    dbus.BusObject
	write  dbus.BusObject
	notify dbus.ObjectPath
	sig    chan *dbus.Signal
	mu     sync.Mutex
	resp   chan []byte
	as     *assembler
	done   chan struct{}
	lost   bool
}

// OpenBLE conecta pelo Bluetooth no endereço dado (AA:BB:CC:DD:EE:FF).
func OpenBLE(addr string) (Transport, error) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, fmt.Errorf("Bluetooth: sem D-Bus (%w)", err)
	}
	fail := func(err error) (Transport, error) { conn.Close(); return nil, err }
	objs, err := managedObjects(conn)
	if err != nil {
		return fail(fmt.Errorf("Bluetooth: BlueZ não respondeu (%w)", err))
	}
	ad := adapterPath(objs)
	if ad == "" {
		return fail(fmt.Errorf("nenhum adaptador Bluetooth"))
	}
	addr = strings.ToUpper(strings.TrimSpace(addr))
	devPath := dbus.ObjectPath(string(ad) + "/dev_" + strings.ReplaceAll(addr, ":", "_"))
	if _, ok := objs[devPath]; !ok {
		// ainda não visto: procura um pouco para o BlueZ conhecer o aparelho
		a := conn.Object(bluez, ad)
		a.Call("org.bluez.Adapter1.SetDiscoveryFilter", 0, map[string]interface{}{"Transport": "le"})
		a.Call("org.bluez.Adapter1.StartDiscovery", 0)
		for i := 0; i < 20; i++ {
			time.Sleep(500 * time.Millisecond)
			if objs, err = managedObjects(conn); err == nil {
				if _, ok := objs[devPath]; ok {
					break
				}
			}
		}
		a.Call("org.bluez.Adapter1.StopDiscovery", 0)
		if _, ok := objs[devPath]; !ok {
			return fail(fmt.Errorf("RadiaCode %s não apareceu no Bluetooth (ligado e perto? o celular não está conectado nele?)", addr))
		}
	}
	dev := conn.Object(bluez, devPath)
	if call := dev.Call("org.bluez.Device1.Connect", 0); call.Err != nil {
		return fail(fmt.Errorf("Bluetooth: não conectou em %s (%v)", addr, call.Err))
	}
	// espera os serviços GATT aparecerem
	var wr, nt dbus.ObjectPath
	for i := 0; i < 40 && (wr == "" || nt == ""); i++ {
		time.Sleep(250 * time.Millisecond)
		objs, err = managedObjects(conn)
		if err != nil {
			continue
		}
		for p, ifs := range objs {
			ch, ok := ifs["org.bluez.GattCharacteristic1"]
			if !ok || !strings.HasPrefix(string(p), string(devPath)+"/") {
				continue
			}
			switch strings.ToLower(str(ch["UUID"])) {
			case bleWriteUUID:
				wr = p
			case bleNotifyUUID:
				nt = p
			}
		}
	}
	if wr == "" || nt == "" {
		dev.Call("org.bluez.Device1.Disconnect", 0)
		return fail(fmt.Errorf("Bluetooth: o aparelho não mostrou o serviço do RadiaCode"))
	}
	b := &bleLinux{conn: conn, dev: dev, write: conn.Object(bluez, wr), notify: nt,
		sig: make(chan *dbus.Signal, 64), resp: make(chan []byte, 1), as: newAssembler(), done: make(chan struct{})}
	if err := conn.AddMatchSignal(dbus.WithMatchObjectPath(nt), dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchMember("PropertiesChanged")); err != nil {
		b.Close()
		return nil, err
	}
	conn.AddMatchSignal(dbus.WithMatchObjectPath(devPath), dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchMember("PropertiesChanged"))
	conn.Signal(b.sig)
	go b.listen(devPath)
	if call := conn.Object(bluez, nt).Call("org.bluez.GattCharacteristic1.StartNotify", 0); call.Err != nil {
		b.Close()
		return nil, fmt.Errorf("Bluetooth: StartNotify: %v", call.Err)
	}
	return b, nil
}

func (b *bleLinux) listen(devPath dbus.ObjectPath) {
	for {
		select {
		case <-b.done:
			return
		case s, ok := <-b.sig:
			if !ok {
				return
			}
			if len(s.Body) < 2 {
				continue
			}
			changed, _ := s.Body[1].(map[string]dbus.Variant)
			if s.Path == devPath {
				if v, ok := changed["Connected"]; ok {
					if on, _ := v.Value().(bool); !on {
						b.mu.Lock()
						b.lost = true
						b.mu.Unlock()
						select {
						case b.resp <- nil:
						default:
						}
					}
				}
				continue
			}
			if s.Path != b.notify {
				continue
			}
			v, ok := changed["Value"]
			if !ok {
				continue
			}
			data, _ := v.Value().([]byte)
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
		}
	}
}

func (b *bleLinux) Exchange(req []byte) ([]byte, error) {
	b.mu.Lock()
	if b.lost {
		b.mu.Unlock()
		return nil, fmt.Errorf("Bluetooth: conexão perdida")
	}
	b.as = newAssembler()
	b.mu.Unlock()
	select { // sobra de uma resposta antiga
	case <-b.resp:
	default:
	}
	for p := 0; p < len(req); p += 18 {
		chunk := req[p:min(p+18, len(req))]
		if call := b.write.Call("org.bluez.GattCharacteristic1.WriteValue", 0, chunk, map[string]interface{}{"type": "command"}); call.Err != nil {
			return nil, fmt.Errorf("Bluetooth (envio): %v", call.Err)
		}
	}
	select {
	case out := <-b.resp:
		if out == nil {
			return nil, fmt.Errorf("Bluetooth: conexão perdida")
		}
		return out, nil
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("Bluetooth: o RadiaCode não respondeu")
	}
}

func (b *bleLinux) Close() error {
	select {
	case <-b.done:
		return nil
	default:
	}
	close(b.done)
	b.conn.Object(bluez, b.notify).Call("org.bluez.GattCharacteristic1.StopNotify", 0)
	b.dev.Call("org.bluez.Device1.Disconnect", 0)
	b.conn.RemoveSignal(b.sig)
	return b.conn.Close()
}

func (b *bleLinux) Kind() string { return "bluetooth" }

// BLESupported diz se este sistema conecta pelo Bluetooth.
const BLESupported = true
