package app

import "testing"

// S5: o radar só busca imagens no RainViewer, por https.
func TestRadarHost(t *testing.T) {
	for host, ok := range map[string]bool{
		"https://tilecache.rainviewer.com":      true,
		"https://rainviewer.com/":               true,
		"http://tilecache.rainviewer.com":       false,
		"https://rainviewer.com.mal.example":    false,
		"https://mal.example/rainviewer.com":    false,
		"https://x@tilecache.rainviewer.com":    false,
		"https://tilecache.rainviewer.com:8443": false,
		"":                                      false,
	} {
		if radarHostOK(host) != ok {
			t.Errorf("%q: queria %v", host, ok)
		}
	}
	for p, ok := range map[string]bool{
		"/v2/radar/1731500000": true,
		"//mal.example/x":      false,
		"/v2/radar?x=1":        false,
		"v2/radar":             false,
		"/@mal.example":        false,
	} {
		if radarPathOK(p) != ok {
			t.Errorf("caminho %q: queria %v", p, ok)
		}
	}
}
