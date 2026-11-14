//go:build linux

package app

import (
	"bufio"
	"bytes"
	"errors"
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

// Os testes nunca falam com o cofre de verdade da máquina (no CI há sessão
// de D-Bus): só o startVault liga o cofre, com o de mentira.
func init() { keyringPossible = func() bool { return false } }

// cofre de mentira (Secret Service) num D-Bus só do teste
type fakeVault struct {
	mu    sync.Mutex
	conn  *dbus.Conn
	n     int
	items map[dbus.ObjectPath]*fakeItem
	lock  bool // itens novos nascem trancados
	// Unlock espera hold (o pedido de senha aberto) e recusa com refuse
	hold   chan struct{}
	refuse bool
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
	hold := s.v.hold
	s.v.mu.Unlock()
	if hold != nil {
		<-hold
	}
	s.v.mu.Lock()
	defer s.v.mu.Unlock()
	if s.v.refuse {
		return nil, "/", dbus.MakeFailedError(fmt.Errorf("pedido recusado"))
	}
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
	old, oldP := keyringBus, keyringPossible
	keyringBus = func() (*dbus.Conn, error) { return dbus.Connect(addr) }
	keyringPossible = func() bool { return true }
	t.Cleanup(func() { keyringBus, keyringPossible = old, oldP })
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
	// sem a chave no cache: fica em claro e não abre cifrado (não fala com o
	// cofre aqui, que pode estar com a trava da nuvem presa)
	if got, _ := protect(dir, plain); !bytes.Equal(got, plain) {
		t.Fatal("cifrou sem a chave no cache")
	}
	if err := warmProtect(dir, true); err != nil {
		t.Fatal(err)
	}
	enc, err := protect(dir, plain)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(enc, []byte(keyMagic)) || bytes.Contains(enc, []byte("K7QF")) {
		t.Fatalf("não cifrou: %q", enc)
	}
	clearKeyCache()
	if _, err := unprotect(dir, enc); !errors.Is(err, errVaultWait) {
		t.Fatalf("sem cache: %v", err)
	}
	if err := warmProtect(dir, false); err != nil {
		t.Fatal(err)
	}
	got, err := unprotect(dir, enc)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("abrir: %q %v", got, err)
	}
	// a chave reaproveita o item do cofre (não cria outro)
	clearKeyCache()
	if err := warmProtect(dir, true); err != nil {
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
	if err := warmProtect(dir, false); err != nil {
		t.Fatal(err)
	}
	if got, err := unprotect(dir, enc); err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("trancado: %v", err)
	}
	// antes da 2.80.7: em claro
	if got, err := unprotect(dir, plain); err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("em claro: %v", err)
	}
	// outra pasta não usa a mesma chave
	clearKeyCache()
	if err := warmProtect(t.TempDir(), false); !errors.Is(err, errVaultMissing) {
		t.Fatalf("outra pasta: %v", err)
	}
	// desconectar tira a chave do cofre
	forgetProtect(dir)
	if err := warmProtect(dir, false); !errors.Is(err, errVaultMissing) {
		t.Fatalf("depois de esquecer: %v", err)
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
	old, oldP := keyringBus, keyringPossible
	keyringBus = func() (*dbus.Conn, error) { return nil, errNoKeyring }
	keyringPossible = func() bool { return true }
	t.Cleanup(func() { keyringBus, keyringPossible = old, oldP })
	clearKeyCache()
	b := []byte("x")
	dir := t.TempDir()
	if err := warmProtect(dir, true); !errors.Is(err, errVaultMissing) {
		t.Fatal(err)
	}
	got, err := protect(dir, b)
	if err != nil || !bytes.Equal(got, b) {
		t.Fatalf("%q %v", got, err)
	}
}

// reopenApp é o app aberto de novo na mesma pasta (a nuvem relê os arquivos).
func reopenApp(a *App) *App {
	b := NewApp()
	b.dataDir, b.store, b.cal, b.settings = a.dataDir, a.store, a.cal, a.settings
	return b
}

func waitCloud(t *testing.T, a *App, ok func(CloudInfo) bool) CloudInfo {
	t.Helper()
	var st CloudInfo
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
		if st = a.GetCloud(); ok(st) {
			return st
		}
	}
	t.Fatalf("estado da nuvem: %+v", st)
	return st
}

// QA da 2.80.7: com o cofre esperando a senha, a tela da nuvem não trava;
// recusar o pedido não pede o código de novo, e "Tentar de novo" abre.
func TestCloudVaultLockedNoBlock(t *testing.T) {
	v := startVault(t)
	clearKeyCache()
	t.Cleanup(clearKeyCache)
	fakeCloud(t)
	a, _ := testApp(t)
	t.Cleanup(a.cloud.vaultWG.Wait)
	cloudPair(t, a, nil)
	// a chave passa para o cofre em segundo plano
	for end := time.Now().Add(5 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		b, _ := os.ReadFile(a.cloudKeyPath())
		if bytes.HasPrefix(b, []byte(keyMagic)) {
			break
		}
		if time.Now().After(end) {
			t.Fatal("a chave não foi para o cofre")
		}
	}
	// abre o app de novo com o cofre trancado e o pedido de senha aberto
	v.mu.Lock()
	for _, it := range v.items {
		it.locked = true
	}
	v.hold, v.refuse = make(chan struct{}), true
	v.mu.Unlock()
	clearKeyCache()
	b := reopenApp(a)
	t.Cleanup(b.cloud.vaultWG.Wait)
	done := make(chan CloudInfo, 1)
	go func() { done <- b.GetCloud() }()
	var st CloudInfo
	select {
	case st = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("GetCloud travou esperando o cofre")
	}
	if st.State != "vault" || !st.VaultWait {
		t.Fatalf("esperando o cofre: %+v", st)
	}
	close(v.hold) // a pessoa fecha o pedido
	st = waitCloud(t, b, func(s CloudInfo) bool { return !s.VaultWait })
	if st.State != "vault" {
		t.Fatalf("recusado virou %q (não pode pedir o código)", st.State)
	}
	v.mu.Lock()
	v.hold, v.refuse = nil, false
	v.mu.Unlock()
	b.CloudVaultRetry()
	waitCloud(t, b, func(s CloudInfo) bool { return s.State == "ok" })
	b.cloud.mu.Lock()
	keys := b.cloud.keys != nil
	b.cloud.mu.Unlock()
	if !keys {
		t.Fatal("abriu sem as chaves")
	}
}

// Sem a chave no cofre (cofre apagado), aí sim pede para entrar de novo.
func TestCloudVaultMissingNeedsLogin(t *testing.T) {
	v := startVault(t)
	clearKeyCache()
	t.Cleanup(clearKeyCache)
	fakeCloud(t)
	a, _ := testApp(t)
	t.Cleanup(a.cloud.vaultWG.Wait)
	cloudPair(t, a, nil)
	for end := time.Now().Add(5 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		b, _ := os.ReadFile(a.cloudKeyPath())
		if bytes.HasPrefix(b, []byte(keyMagic)) {
			break
		}
		if time.Now().After(end) {
			t.Fatal("a chave não foi para o cofre")
		}
	}
	v.mu.Lock()
	v.items = map[dbus.ObjectPath]*fakeItem{}
	v.mu.Unlock()
	clearKeyCache()
	b := reopenApp(a)
	t.Cleanup(b.cloud.vaultWG.Wait)
	waitCloud(t, b, func(s CloudInfo) bool { return s.State == "login" })
}
