//go:build linux

package app

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

// No Linux os segredos da nuvem (acesso do Google e código de recuperação)
// são cifrados com uma chave guardada no cofre do sistema (Secret Service:
// GNOME Keyring, KWallet), como o DPAPI no Windows (auditoria 2.79.0, N10).
// Sem cofre (sessão sem D-Bus, servidor), o arquivo fica como antes: em
// claro, só com a permissão 0600.

const (
	ssName       = "org.freedesktop.secrets"
	ssPath       = dbus.ObjectPath("/org/freedesktop/secrets")
	ssService    = "org.freedesktop.Secret.Service"
	ssCollection = "org.freedesktop.Secret.Collection"
	ssItem       = "org.freedesktop.Secret.Item"
	ssPrompt     = "org.freedesktop.Secret.Prompt"
	// arquivo cifrado com a chave do cofre: "RLK1" + nonce + AES-GCM
	keyMagic = "RLK1"
)

// errNoKeyring: o cofre não existe ou não tem a chave desta pasta.
var errNoKeyring = errors.New("cofre do sistema indisponível")

var (
	// keyringBus abre a conexão com a sessão (trocada nos testes)
	keyringBus = func() (*dbus.Conn, error) {
		// sem sessão de D-Bus não tenta abrir uma (dbus-launch)
		if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
			p := filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "bus")
			if os.Getenv("XDG_RUNTIME_DIR") == "" {
				return nil, errNoKeyring
			}
			if _, err := os.Stat(p); err != nil {
				return nil, errNoKeyring
			}
			return dbus.Connect("unix:path=" + p)
		}
		return dbus.ConnectSessionBus()
	}
	// keyringTimeout limita cada conversa com o cofre (o desbloqueio pode
	// pedir a senha da sessão)
	keyringTimeout = 2 * time.Minute
	keyringCache   sync.Map // pasta → chave
)

// ssSecret é o Secret da especificação: (oayays).
type ssSecret struct {
	Session     dbus.ObjectPath
	Parameters  []byte
	Value       []byte
	ContentType string
}

func keyringAttrs(dir string) map[string]string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	h := sha256.Sum256([]byte(abs))
	return map[string]string{"application": "radlog", "radlog-pasta": hex.EncodeToString(h[:8])}
}

// keyringKey devolve a chave desta pasta no cofre; create cria uma se não
// houver.
func keyringKey(dir string, create bool) ([]byte, error) {
	if k, ok := keyringCache.Load(dir); ok {
		return k.([]byte), nil
	}
	conn, err := keyringBus()
	if err != nil {
		return nil, errNoKeyring
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), keyringTimeout)
	defer cancel()
	svc := conn.Object(ssName, ssPath)
	var out dbus.Variant
	var session dbus.ObjectPath
	if err := svc.CallWithContext(ctx, ssService+".OpenSession", 0, "plain", dbus.MakeVariant("")).Store(&out, &session); err != nil {
		return nil, errNoKeyring
	}
	defer conn.Object(ssName, session).CallWithContext(ctx, "org.freedesktop.Secret.Session.Close", 0)
	attrs := keyringAttrs(dir)
	var unlocked, locked []dbus.ObjectPath
	if err := svc.CallWithContext(ctx, ssService+".SearchItems", 0, attrs).Store(&unlocked, &locked); err != nil {
		return nil, fmt.Errorf("cofre do sistema: %w", err)
	}
	if len(unlocked) == 0 && len(locked) > 0 {
		if unlocked, err = ssUnlock(ctx, conn, locked); err != nil {
			return nil, err
		}
	}
	if len(unlocked) > 0 {
		var sec ssSecret
		if err := conn.Object(ssName, unlocked[0]).CallWithContext(ctx, ssItem+".GetSecret", 0, session).Store(&sec); err != nil {
			return nil, fmt.Errorf("cofre do sistema: %w", err)
		}
		if len(sec.Value) != 32 {
			return nil, errors.New("cofre do sistema: chave com tamanho errado")
		}
		keyringCache.Store(dir, sec.Value)
		return sec.Value, nil
	}
	if !create {
		return nil, errNoKeyring
	}
	var coll dbus.ObjectPath
	if err := svc.CallWithContext(ctx, ssService+".ReadAlias", 0, "default").Store(&coll); err != nil || coll == "/" || coll == "" {
		return nil, errNoKeyring
	}
	if _, err := ssUnlock(ctx, conn, []dbus.ObjectPath{coll}); err != nil {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	props := map[string]dbus.Variant{
		ssItem + ".Label":      dbus.MakeVariant("RadLog: chave da nuvem"),
		ssItem + ".Attributes": dbus.MakeVariant(attrs),
	}
	sec := ssSecret{Session: session, Value: key, ContentType: "application/octet-stream"}
	var item, prompt dbus.ObjectPath
	if err := conn.Object(ssName, coll).CallWithContext(ctx, ssCollection+".CreateItem", 0, props, sec, true).Store(&item, &prompt); err != nil {
		return nil, fmt.Errorf("cofre do sistema: %w", err)
	}
	if prompt != "/" && prompt != "" {
		if _, err := ssPromptWait(ctx, conn, prompt); err != nil {
			return nil, err
		}
	}
	keyringCache.Store(dir, key)
	return key, nil
}

// ssUnlock desbloqueia os objetos (o sistema pode pedir a senha da sessão).
func ssUnlock(ctx context.Context, conn *dbus.Conn, objs []dbus.ObjectPath) ([]dbus.ObjectPath, error) {
	var done []dbus.ObjectPath
	var prompt dbus.ObjectPath
	if err := conn.Object(ssName, ssPath).CallWithContext(ctx, ssService+".Unlock", 0, objs).Store(&done, &prompt); err != nil {
		return nil, fmt.Errorf("cofre do sistema: %w", err)
	}
	if prompt == "/" || prompt == "" {
		return done, nil
	}
	v, err := ssPromptWait(ctx, conn, prompt)
	if err != nil {
		return nil, err
	}
	if got, ok := v.Value().([]dbus.ObjectPath); ok {
		return got, nil
	}
	return objs, nil
}

// ssPromptWait mostra o pedido do cofre e espera a resposta.
func ssPromptWait(ctx context.Context, conn *dbus.Conn, p dbus.ObjectPath) (dbus.Variant, error) {
	opts := []dbus.MatchOption{dbus.WithMatchObjectPath(p), dbus.WithMatchInterface(ssPrompt), dbus.WithMatchMember("Completed")}
	if err := conn.AddMatchSignalContext(ctx, opts...); err != nil {
		return dbus.Variant{}, fmt.Errorf("cofre do sistema: %w", err)
	}
	defer conn.RemoveMatchSignalContext(context.Background(), opts...)
	ch := make(chan *dbus.Signal, 4)
	conn.Signal(ch)
	defer conn.RemoveSignal(ch)
	if err := conn.Object(ssName, p).CallWithContext(ctx, ssPrompt+".Prompt", 0, "").Err; err != nil {
		return dbus.Variant{}, fmt.Errorf("cofre do sistema: %w", err)
	}
	for {
		select {
		case <-ctx.Done():
			return dbus.Variant{}, errors.New("cofre do sistema: sem resposta")
		case s := <-ch:
			if s == nil || s.Path != p || s.Name != ssPrompt+".Completed" || len(s.Body) < 2 {
				continue
			}
			if dismissed, _ := s.Body[0].(bool); dismissed {
				return dbus.Variant{}, errors.New("cofre do sistema: pedido recusado")
			}
			v, _ := s.Body[1].(dbus.Variant)
			return v, nil
		}
	}
}

// protect cifra com a chave do cofre. Sem cofre, devolve b como veio (o
// arquivo é 0600, como antes da 2.80.7).
func protect(dir string, b []byte) ([]byte, error) {
	key, err := keyringKey(dir, true)
	if err != nil {
		return b, nil
	}
	blk, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	g, err := cipher.NewGCM(blk)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := append([]byte(keyMagic), nonce...)
	return g.Seal(out, nonce, b, []byte(keyMagic)), nil
}

// unprotect desfaz o protect. Arquivo em claro (de antes, ou sem cofre) vem
// como está; cifrado sem a chave no cofre dá erro (a nuvem pede o código).
func unprotect(dir string, b []byte) ([]byte, error) {
	if !bytes.HasPrefix(b, []byte(keyMagic)) {
		return b, nil
	}
	key, err := keyringKey(dir, false)
	if err != nil {
		return nil, err
	}
	blk, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	g, err := cipher.NewGCM(blk)
	if err != nil {
		return nil, err
	}
	b = b[len(keyMagic):]
	if len(b) < g.NonceSize() {
		return nil, errors.New("chave da nuvem estragada")
	}
	return g.Open(nil, b[:g.NonceSize()], b[g.NonceSize():], []byte(keyMagic))
}

// forgetProtect tira a chave desta pasta do cofre (ao desconectar).
func forgetProtect(dir string) {
	keyringCache.Delete(dir)
	conn, err := keyringBus()
	if err != nil {
		return
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var unlocked, locked []dbus.ObjectPath
	if conn.Object(ssName, ssPath).CallWithContext(ctx, ssService+".SearchItems", 0, keyringAttrs(dir)).Store(&unlocked, &locked) != nil {
		return
	}
	for _, it := range append(unlocked, locked...) {
		var prompt dbus.ObjectPath
		_ = conn.Object(ssName, it).CallWithContext(ctx, ssItem+".Delete", 0).Store(&prompt)
	}
}
