package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// O nome do arquivo (assinado no SHA256SUMS.txt) tem de trazer a versão da
// etiqueta: um .exe antigo sob uma etiqueta nova não é instalado (2.70.1).
func TestAssetIsVersion(t *testing.T) {
	for _, c := range []struct {
		name, ver string
		want      bool
	}{
		{"RadLog-v2.70.1.exe", "2.70.1", true},
		{"RadLog-v2.70.1-x86_64.AppImage", "2.70.1", true},
		{"GMC-Monitor-v1.46.1.exe", "1.46.1", true},
		{"RadLog-v2.16.0.exe", "9.9.9", false},
		{"RadLog-v2.70.1.exe.part", "2.70.1", false},
		{"RadLog.exe", "2.70.1", false},
		{"qualquer.exe", "2.70.1", false},
		{"RadLog-v2.70.1.exe", "", false},
	} {
		if got := assetIsVersion(c.name, c.ver); got != c.want {
			t.Errorf("assetIsVersion(%q, %q) = %v, quero %v", c.name, c.ver, got, c.want)
		}
	}
}

func TestCheckUpdateRefusesOldFileUnderNewTag(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Setenv("APPIMAGE", filepath.Join(t.TempDir(), "RadLog-v1.0.0-x86_64.AppImage"))
	}
	name := "RadLog-v2.16.0.exe"
	if runtime.GOOS == "linux" {
		name = "RadLog-v2.16.0-x86_64.AppImage"
	}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(ghRelease{Tag: "v99.0.0", HTMLURL: srv.URL + "/page",
			Assets: []ghAsset{{Name: name, URL: srv.URL + "/bin", Size: 1}, {Name: updateSums, URL: srv.URL + "/sums"}, {Name: updateSig, URL: srv.URL + "/sig"}}})
	}))
	defer srv.Close()
	old := updateAPI
	defer func() { updateAPI = old }()
	updateAPI = srv.URL + "/latest"
	a, _ := testApp(t)
	info, err := a.CheckUpdate()
	if err != nil {
		t.Fatal(err)
	}
	if !info.Available || info.CanAuto || !strings.Contains(info.Manual, "não é o da versão 99.0.0") {
		t.Fatalf("devia recusar a troca automática: %+v", info)
	}
	if err := a.StartUpdate(); err == nil {
		t.Fatal("StartUpdate devia recusar")
	}
}

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.json")
	if err := writeFileAtomic(p, []byte("um"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeFileAtomic(p, []byte("dois"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if string(b) != "dois" {
		t.Fatalf("conteúdo: %q", b)
	}
	ents, _ := os.ReadDir(dir)
	if len(ents) != 1 {
		t.Fatalf("sobrou temporário: %v", ents)
	}
}

// config.json cortado: volta ao padrão inteiro, guarda o arquivo ruim e
// marca para avisar, em vez de zerar em silêncio (2.70.1).
func TestLoadSettingsBrokenFile(t *testing.T) {
	a := newTestApp(t)
	a.settings.AlarmUSv = 3.21
	a.mu.Lock()
	a.saveSettingsLocked()
	a.mu.Unlock()
	if got := a.loadSettings(); got.AlarmUSv != 3.21 || a.settingsBroken {
		t.Fatalf("leitura normal: %v %v", got.AlarmUSv, a.settingsBroken)
	}
	good, _ := os.ReadFile(a.settingsPath())
	cut := good[:len(good)/2]
	if err := os.WriteFile(a.settingsPath(), cut, 0o644); err != nil {
		t.Fatal(err)
	}
	got := a.loadSettings()
	def := defaultSettings()
	if !a.settingsBroken || got.AlarmUSv != def.AlarmUSv || got.AutoConnect != def.AutoConnect {
		t.Fatalf("arquivo cortado: broken=%v alarm=%v", a.settingsBroken, got.AlarmUSv)
	}
	b, err := os.ReadFile(a.settingsPath() + ".ilegivel")
	if err != nil || string(b) != string(cut) {
		t.Fatalf("cópia do arquivo ruim: %v", err)
	}
}
