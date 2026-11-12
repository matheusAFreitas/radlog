package app

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"radlog/internal/updsig"
)

func TestVersionCompare(t *testing.T) {
	cases := []struct {
		l, c string
		want bool
	}{
		{"1.17.2", "1.17.1", true}, {"v1.18.0", "1.17.9", true}, {"2.0.0", "1.99.99", true},
		{"1.17.1", "1.17.1", false}, {"1.9.0", "1.10.0", false}, {"lixo", "1.0.0", false}, {"1.2", "1.0.0", false},
	}
	for _, c := range cases {
		if got := newerVersion(c.l, c.c); got != c.want {
			t.Fatalf("%s > %s: %v", c.l, c.c, got)
		}
	}
}

func TestParseSumsAndNotes(t *testing.T) {
	sum := strings.Repeat("ab", 32)
	text := "0000  outro.exe\n" + sum + "  GMC-Monitor-v1.18.0.exe\n"
	if got, err := parseSums(text, "GMC-Monitor-v1.18.0.exe"); err != nil || got != sum {
		t.Fatalf("%q %v", got, err)
	}
	if _, err := parseSums(text, "nao-tem.exe"); err == nil {
		t.Fatal("achou o que não tem")
	}
	if n := releaseNotes("- coisa nova\n\n---\n**Downloads:** x"); n != "- coisa nova" {
		t.Fatalf("notas: %q", n)
	}
}

// release falsa num servidor local: procura, baixa, confere e "abre".
func TestUpdateFlow(t *testing.T) {
	home := t.TempDir() // a pasta das versões fica em Documentos: não suja o HOME de verdade
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if runtime.GOOS == "linux" {
		t.Setenv("APPIMAGE", filepath.Join(t.TempDir(), "GMC-Monitor-v1.0.0-x86_64.AppImage"))
	}
	name := "GMC-Monitor-v99.0.0.exe"
	if runtime.GOOS == "linux" {
		name = "GMC-Monitor-v99.0.0-x86_64.AppImage"
	}
	payload := []byte(strings.Repeat("binário novo ", 5000))
	h := sha256.Sum256(payload)
	good := hex.EncodeToString(h[:])
	sums := good
	// chave de teste no lugar da das releases
	_, priv, _ := ed25519.GenerateKey(nil)
	pub := priv.Public().(ed25519.PublicKey)
	oldVerify := verifySums
	defer func() { verifySums = oldVerify }()
	verifySums = func(data, sig []byte) bool { return updsig.VerifyWith(pub, data, sig) }
	badSig := false
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest":
			_ = json.NewEncoder(w).Encode(ghRelease{Tag: "v99.0.0", Body: "- novidade\n\n---\n**Downloads:** x", HTMLURL: srv.URL + "/page",
				Assets: []ghAsset{{Name: name, URL: srv.URL + "/bin", Size: int64(len(payload))}, {Name: updateSums, URL: srv.URL + "/sums"}, {Name: updateSig, URL: srv.URL + "/sig"}}})
		case "/bin":
			_, _ = w.Write(payload)
		case "/sums":
			_, _ = w.Write([]byte(sums + "  " + name + "\n"))
		case "/sig":
			data := []byte(sums + "  " + name + "\n")
			if badSig {
				data = append(data, 'x')
			}
			_, _ = w.Write([]byte(base64.StdEncoding.EncodeToString(ed25519.Sign(priv, data)) + "\n"))
		}
	}))
	defer srv.Close()
	oldAPI, oldLaunch := updateAPI, launchUpdated
	defer func() { updateAPI, launchUpdated = oldAPI, oldLaunch }()
	updateAPI = srv.URL + "/latest"
	launched := make(chan string, 1)
	launchUpdated = func(p string) error { launched <- p; return nil }

	a, _ := testApp(t)
	info, err := a.CheckUpdate()
	if err != nil || !info.Available || info.Latest != "99.0.0" || !info.CanAuto || info.Notes != "- novidade" {
		t.Fatalf("procurar: %+v %v", info, err)
	}
	// alerta ativo: baixa e espera
	a.mu.Lock()
	a.attnOn = true
	a.mu.Unlock()
	if err := a.StartUpdate(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "esperando", func() bool { return a.GetUpdateInfo().State == "esperando" })
	if w := a.GetUpdateInfo().Wait; w != "atenção ativa" {
		t.Fatalf("motivo: %q", w)
	}
	a.mu.Lock()
	a.attnOn = false
	a.mu.Unlock()
	var path string
	select {
	case path = <-launched:
	case <-time.After(10 * time.Second):
		t.Fatal("não abriu a versão nova")
	}
	want := name
	if runtime.GOOS == "windows" {
		want = fixedExe // no Windows a versão nova vira sempre RadLog.exe
	}
	b, err := os.ReadFile(path)
	if err != nil || string(b) != string(payload) || filepath.Base(path) != want {
		t.Fatalf("arquivo: %s %v", path, err)
	}
	if _, err := os.Stat(path + ".part"); err == nil {
		t.Fatal("ficou o .part")
	}

	// SHA errado: recusa e não deixa arquivo
	os.Remove(path)
	sums = strings.Repeat("0", 64)
	a2, _ := testApp(t)
	if _, err := a2.CheckUpdate(); err != nil {
		t.Fatal(err)
	}
	_ = a2.StartUpdate()
	waitFor(t, "erro", func() bool { return a2.GetUpdateInfo().State == "erro" })
	if e := a2.GetUpdateInfo().Error; !strings.Contains(e, "não confere") {
		t.Fatalf("erro: %q", e)
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("deixou o arquivo que não confere")
	}

	// assinatura que não confere: recusa antes de baixar
	sums, badSig = good, true
	a3, _ := testApp(t)
	if _, err := a3.CheckUpdate(); err != nil {
		t.Fatal(err)
	}
	_ = a3.StartUpdate()
	waitFor(t, "erro", func() bool { return a3.GetUpdateInfo().State == "erro" })
	if e := a3.GetUpdateInfo().Error; !strings.Contains(e, "assinatura") {
		t.Fatalf("erro: %q", e)
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("baixou com assinatura que não confere")
	}
	badSig = false

	// pular esta versão
	a2.SkipUpdate("99.0.0")
	if !a2.GetUpdateInfo().Skipped {
		t.Fatal("não marcou como pulada")
	}
}

func TestOldVersionsIn(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"GMC-Monitor-v1.0.0.exe", "GMC-Monitor-v1.21.0.exe", "GMC-Monitor-v99.0.0.exe",
		"GMC-Monitor-v1.2.0-x86_64.AppImage", "GMC-Monitor-v1.5.0.exe.part", "outro.exe", "GMC-Monitor-v1.0.0-notas.txt",
		"RadLog-v1.9.0.exe", "RadLog-v99.0.0.exe", "RadLog.exe", "RadLog.old.exe"} {
		_ = os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644)
	}
	self := filepath.Join(dir, "GMC-Monitor-v1.21.0.exe")
	var got []string
	for _, p := range oldVersionsIn(dir, self) {
		got = append(got, filepath.Base(p))
	}
	want := "GMC-Monitor-v1.0.0.exe,GMC-Monitor-v1.2.0-x86_64.AppImage,GMC-Monitor-v1.5.0.exe.part,RadLog-v1.9.0.exe,RadLog.old.exe"
	if strings.Join(got, ",") != want {
		t.Fatalf("apagaria %v", got)
	}
	// aberta como RadLog.old.exe (a troca falhou ao abrir a nova): não se apaga
	for _, p := range oldVersionsIn(dir, filepath.Join(dir, "RadLog.old.exe")) {
		if filepath.Base(p) == "RadLog.old.exe" || filepath.Base(p) == "RadLog.exe" {
			t.Fatalf("apagaria %s", p)
		}
	}
}

func TestInstallFixed(t *testing.T) {
	dir := t.TempDir()
	read := func(n string) string {
		b, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			return "-"
		}
		return string(b)
	}
	put := func(n, v string) { _ = os.WriteFile(filepath.Join(dir, n), []byte(v), 0o644) }

	// primeira vez: não há RadLog.exe ainda
	put("RadLog-v2.40.1.exe", "v2.40.1")
	got, err := installFixed(filepath.Join(dir, "RadLog-v2.40.1.exe"))
	if err != nil || filepath.Base(got) != "RadLog.exe" || read("RadLog.exe") != "v2.40.1" || read("RadLog-v2.40.1.exe") != "-" {
		t.Fatalf("1ª troca: %s %v %q", got, err, read("RadLog.exe"))
	}
	// próxima: a atual vira .old, a sobra anterior some
	put("RadLog.old.exe", "sobra")
	put("RadLog-v2.41.0.exe", "v2.41.0")
	got, err = installFixed(filepath.Join(dir, "RadLog-v2.41.0.exe"))
	if err != nil || filepath.Base(got) != "RadLog.exe" || read("RadLog.exe") != "v2.41.0" || read("RadLog.old.exe") != "v2.40.1" {
		t.Fatalf("2ª troca: %s %v %q %q", got, err, read("RadLog.exe"), read("RadLog.old.exe"))
	}
	// já com o nome fixo: não mexe
	if got, err := installFixed(filepath.Join(dir, "RadLog.exe")); err != nil || filepath.Base(got) != "RadLog.exe" {
		t.Fatalf("mesmo nome: %s %v", got, err)
	}
	// arquivo sumiu: devolve o caminho original com erro e não perde o RadLog.exe
	if got, err := installFixed(filepath.Join(dir, "RadLog-v9.9.9.exe")); err == nil || filepath.Base(got) != "RadLog-v9.9.9.exe" || read("RadLog.exe") != "v2.41.0" {
		t.Fatalf("falha: %s %v %q", got, err, read("RadLog.exe"))
	}
}

func TestCheckSumsSignature(t *testing.T) {
	sum := strings.Repeat("ab", 32)
	text := []byte(sum + "  RadLog-v2.16.0.exe\n")
	ok := func(data, sig []byte) bool { return string(sig) == "boa" }
	if got, err := checkSums(text, []byte("boa"), "RadLog-v2.16.0.exe", ok); err != nil || got != sum {
		t.Fatalf("assinatura boa: %q %v", got, err)
	}
	if _, err := checkSums(text, []byte("ruim"), "RadLog-v2.16.0.exe", ok); err == nil || !strings.Contains(err.Error(), "assinatura") {
		t.Fatalf("assinatura ruim deveria recusar: %v", err)
	}
}
