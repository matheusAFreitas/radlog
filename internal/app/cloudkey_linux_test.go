//go:build linux

package app

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

// cofre de mentira (Secret Service) num D-Bus só do teste
type fakeVault struct {
	mu    sync.Mutex
	conn  *dbus.Conn
	n     int
	items map[dbus.ObjectPath]*fakeItem
	lock  bool // itens novos nascem trancados
}

type fakeItem struct {
	v      *fakeVault
	attrs  map[string]string
	value  []byte
	locked bool
}

type fakeSvc struct{ v *fakeVault }
type fakeColl struct{ v *fakeVault }
type fakeSession struct{}

func (fakeSession) Close() *dbus.Error { return nil }

func (s fakeSvc) OpenSession(alg string, _ dbus.Variant) (dbus.Variant, dbus.ObjectPath, *dbus.Error) {
	if alg != "plain" {
		return dbus.MakeVariant(""), "/", dbus.MakeFailedError(fmt.Errorf("alg"))
	}
	return dbus.MakeVariant(""), "/org/freedesktop/secrets/session/s1", nil
}

func (s fakeSvc) SearchItems(attrs map[string]string) ([]dbus.ObjectPath, []dbus.ObjectPath, *dbus.Error) {
	s.v.mu.Lock()
	defer s.v.mu.Unlock()
	unl, lck := []dbus.ObjectPath{}, []dbus.ObjectPath{}
	for p, it := range s.v.items {
		ok := true
		for k, x := range attrs {
			if it.attrs[k] != x {
				ok = false
			}
		}
		if !ok {
			continue
		}
		if it.locked {
			lck = append(lck, p)
		} else {
			unl = append(unl, p)
		}
	}
	return unl, lck, nil
}

func (s fakeSvc) Unlock(objs []dbus.ObjectPath) ([]dbus.ObjectPath, dbus.ObjectPath, *dbus.Error) {
	s.v.mu.Lock()
	defer s.v.mu.Unlock()
	for _, p := range objs {
		if it := s.v.items[p]; it != nil {
			it.locked = false
		}
	}
	return objs, "/", nil
}

func (s fakeSvc) ReadAlias(name string) (dbus.ObjectPath, *dbus.Error) {
	return "/org/freedesktop/secrets/collection/login", nil
}

func (c fakeColl) CreateItem(props map[string]dbus.Variant, sec ssSecret, replace bool) (dbus.ObjectPath, dbus.ObjectPath, *dbus.Error) {
	c.v.mu.Lock()
	defer c.v.mu.Unlock()
	attrs, _ := props[ssItem+".Attributes"].Value().(map[string]string)
	c.v.n++
	p := dbus.ObjectPath(fmt.Sprintf("/org/freedesktop/secrets/collection/login/%d", c.v.n))
	it := &fakeItem{v: c.v, attrs: attrs, value: sec.Value, locked: c.v.lock}
	c.v.items[p] = it
	_ = c.v.conn.Export(it, p, ssItem)
	return p, "/", nil
}

func (it *fakeItem) GetSecret(session dbus.ObjectPath) (ssSecret, *dbus.Error) {
	it.v.mu.Lock()
	defer it.v.mu.Unlock()
	if it.locked {
		return ssSecret{}, dbus.NewError("org.freedesktop.Secret.Error.IsLocked", nil)
	}
	return ssSecret{Session: session, Value: it.value, ContentType: "application/octet-stream"}, nil
}

func (it *fakeItem) Delete() (dbus.ObjectPath, *dbus.Error) {
	it.v.mu.Lock()
	defer it.v.mu.Unlock()
	for p, x := range it.v.items {
		if x == it {
			delete(it.v.items, p)
		}
	}
	return "/", nil
}

// startVault sobe um dbus-daemon e o cofre de mentira nele.
func startVault(t *testing.T) *fakeVault {
	t.Helper()
	bin, err := exec.LookPath("dbus-daemon")
	if err != nil {
		t.Skip("sem dbus-daemon")
	}
	dir := t.TempDir()
	conf := filepath.Join(dir, "bus.conf")
	_ = os.WriteFile(conf, []byte(`<!DOCTYPE busconfig PUBLIC "-//freedesktop//DTD D-Bus Bus Configuration 1.0//EN" "http://www.freedesktop.org/standards/dbus/1.0/busconfig.dtd">
<busconfig><type>session</type><listen>unix:path=`+filepath.Join(dir, "bus")+`</listen>
<policy context="default"><allow send_destination="*" eavesdrop="true"/><allow eavesdrop="true"/><allow own="*"/></policy></busconfig>`), 0o644)
	cmd := exec.Command(bin, "--config-file="+conf, "--nofork", "--print-address=1")
	out, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Skip("dbus-daemon não subiu: ", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	addrCh := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(out)
		if sc.Scan() {
			addrCh <- strings.TrimSpace(sc.Text())
		}
	}()
	var addr string
	select {
	case addr = <-addrCh:
	case <-time.After(5 * time.Second):
		t.Skip("dbus-daemon sem endereço")
	}
	conn, err := dbus.Connect(addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	v := &fakeVault{conn: conn, items: map[dbus.ObjectPath]*fakeItem{}}
	_ = conn.Export(fakeSvc{v}, ssPath, ssService)
	_ = conn.Export(fakeColl{v}, "/org/freedesktop/secrets/collection/login", ssCollection)
	_ = conn.Export(fakeSession{}, "/org/freedesktop/secrets/session/s1", "org.freedesktop.Secret.Session")
	if r, err := conn.RequestName(ssName, dbus.NameFlagDoNotQueue); err != nil || r != dbus.RequestNameReplyPrimaryOwner {
		t.Fatal("nome do cofre", r, err)
	}
	old := keyringBus
	keyringBus = func() (*dbus.Conn, error) { return dbus.Connect(addr) }
	t.Cleanup(func() { keyringBus = old })
	return v
}

func clearKeyCache() {
	keyringCache.Range(func(k, _ any) bool { keyringCache.Delete(k); return true })
}

// N10: no Linux com cofre, o arquivo da chave da nuvem sai cifrado; sem a
// chave no cofre (outro PC, cofre apagado) não abre; arquivo em claro de
// antes continua abrindo.
func TestKeyringProtect(t *testing.T) {
	v := startVault(t)
	clearKeyCache()
	t.Cleanup(clearKeyCache)
	dir := t.TempDir()
	plain := []byte(`{"code":"K7QF3XWM9ABCDEFGHJKMNPQR","refresh":"rt-1"}`)
	enc, err := protect(dir, plain)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(enc, []byte(keyMagic)) || bytes.Contains(enc, []byte("K7QF")) {
		t.Fatalf("não cifrou: %q", enc)
	}
	clearKeyCache()
	got, err := unprotect(dir, enc)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("abrir: %q %v", got, err)
	}
	// a chave reaproveita o item do cofre (não cria outro)
	clearKeyCache()
	if _, err := protect(dir, plain); err != nil {
		t.Fatal(err)
	}
	v.mu.Lock()
	n := len(v.items)
	v.mu.Unlock()
	if n != 1 {
		t.Fatalf("itens no cofre: %d", n)
	}
	// item trancado: desbloqueia e abre
	v.mu.Lock()
	for _, it := range v.items {
		it.locked = true
	}
	v.mu.Unlock()
	clearKeyCache()
	if got, err := unprotect(dir, enc); err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("trancado: %v", err)
	}
	// antes da 2.80.7: em claro
	if got, err := unprotect(dir, plain); err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("em claro: %v", err)
	}
	// outra pasta não usa a mesma chave
	clearKeyCache()
	if _, err := unprotect(t.TempDir(), enc); err == nil {
		t.Fatal("abriu com a chave de outra pasta")
	}
	// desconectar tira a chave do cofre
	forgetProtect(dir)
	clearKeyCache()
	if _, err := unprotect(dir, enc); err == nil {
		t.Fatal("abriu depois de esquecer")
	}
	v.mu.Lock()
	n = len(v.items)
	v.mu.Unlock()
	if n != 0 {
		t.Fatalf("ficou no cofre: %d", n)
	}
}

// N10: sem cofre, o arquivo fica como antes (em claro, 0600).
func TestKeyringMissing(t *testing.T) {
	old := keyringBus
	keyringBus = func() (*dbus.Conn, error) { return nil, errNoKeyring }
	t.Cleanup(func() { keyringBus = old })
	clearKeyCache()
	b := []byte("x")
	got, err := protect(t.TempDir(), b)
	if err != nil || !bytes.Equal(got, b) {
		t.Fatalf("%q %v", got, err)
	}
}
