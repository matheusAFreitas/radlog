package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// ---------- chuva pelo radar (RainViewer) ----------
//
// O Open-Meteo é modelo do tempo e erra chuva localizada. O radar mede a
// chuva de verdade: o RainViewer junta os radares (no estado de SP, São Roque
// e Sumaré, entre outros) e publica imagens a cada 10 minutos das últimas 2
// horas. O app baixa o pedaço da imagem que cobre a sua cidade, lê a cor no
// ponto e converte para dBZ e mm/h. Enquanto o app está aberto (e a chuva
// ligada), guarda uma leitura a cada 10 minutos em chuva-radar.json.

var (
	radarIndexURL = "https://api.rainviewer.com/public/weather-maps.json"
	radarClient   = &http.Client{Timeout: 20 * time.Second}
)

const (
	radarZoom    = 7 // máximo do RainViewer
	radarEvery   = 10 * time.Minute
	radarKeep    = 60 * 24 * time.Hour
	radarMinDBZ  = 15 // abaixo disso é garoa fraca ou ruído (cinza no esquema)
	radarRadius  = 1  // pixels em volta do ponto (≈ 1 km cada no zoom 7)
	radarVersion = 1
)

// RadarPoint é uma leitura do radar no local escolhido.
type RadarPoint struct {
	Time int64   `json:"time"` // ms
	DBZ  float64 `json:"dbz"`  // 0 = sem chuva
	MMh  float64 `json:"mmh"`  // intensidade estimada (Marshall-Palmer)
}

type radarStore struct {
	V       int                  `json:"v"`
	Lat     float64              `json:"lat"`
	Lon     float64              `json:"lon"`
	Points  map[int64]RadarPoint `json:"points"`  // unix (s) -> leitura
	Covered *bool                `json:"covered"` // o local tem cobertura de radar?
	Fetched time.Time            `json:"fetched"`
	Err     string               `json:"err"`
}

func (a *App) radarPath() string { return filepath.Join(a.dataDir, "chuva-radar.json") }

func (a *App) loadRadar() {
	b, err := os.ReadFile(a.radarPath())
	if err != nil {
		return
	}
	var r radarStore
	if json.Unmarshal(b, &r) == nil && r.V == radarVersion && r.Points != nil {
		a.radar = &r
	}
}

func (a *App) saveRadarLocked() {
	if a.dataDir == "" || a.radar == nil {
		return
	}
	if b, err := json.Marshal(a.radar); err == nil {
		_ = os.WriteFile(a.radarPath(), b, 0o644)
	}
}

// Universal Blue (esquema 2 do RainViewer): cor -> dBZ, de 15 a 64.
var radarPalette = func() [][4]int {
	hex := []string{
		"88ddee", "6cd1eb", "51c5e8", "36bae5", "1baee2", "00a3e0", "009ad5", "0091ca", "0088bf", "007fb4",
		"0077aa", "0070a3", "00699c", "006295", "005b8e", "005588", "005180", "004e78", "004a70", "004768",
		"ffee00", "ffe000", "ffd200", "ffc500", "ffb700", "ffaa00", "ff9f00", "ff9500", "ff8b00", "ff8100",
		"ff4400", "f23600", "e62800", "d91b00", "cd0d00", "c10000", "a80000", "8f0000", "760000", "5d0000",
		"ffaaff", "ff9fff", "ff95ff", "ff8bff", "ff81ff", "ff77ff", "ff6cff", "ff62ff", "ff58ff", "ff4eff",
	}
	out := make([][4]int, len(hex))
	for i, h := range hex {
		var r, g, b int
		fmt.Sscanf(h, "%02x%02x%02x", &r, &g, &b)
		out[i] = [4]int{15 + i, r, g, b}
	}
	return out
}()

// colorDBZ acha o dBZ mais próximo da cor (0 se transparente ou só garoa).
func colorDBZ(r, g, b, al uint32) float64 {
	if al < 0x8000 { // as cores abaixo de 15 dBZ são cinzas semitransparentes
		return 0
	}
	r8, g8, b8 := int(r>>8), int(g>>8), int(b>>8)
	if r8 > 245 && g8 > 245 && b8 > 245 {
		return 65 // branco: 65 dBZ ou mais
	}
	best, bd := 0, math.MaxInt
	for _, p := range radarPalette {
		d := (p[1]-r8)*(p[1]-r8) + (p[2]-g8)*(p[2]-g8) + (p[3]-b8)*(p[3]-b8)
		if d < bd {
			best, bd = p[0], d
		}
	}
	if bd > 40*40*3 {
		return 0 // cor que não é da escala (borda, texto...)
	}
	return float64(best)
}

// dbzToMMh: Marshall-Palmer, Z = 200·R^1,6.
func dbzToMMh(dbz float64) float64 {
	if dbz < radarMinDBZ {
		return 0
	}
	return math.Pow(math.Pow(10, dbz/10)/200, 1/1.6)
}

// tileOf diz qual tile (zoom z) e qual pixel dele cobrem lat/lon (Web Mercator).
func tileOf(lat, lon float64, z, size int) (x, y, px, py int) {
	n := math.Exp2(float64(z))
	fx := (lon + 180) / 360 * n
	latR := lat * math.Pi / 180
	fy := (1 - math.Log(math.Tan(latR)+1/math.Cos(latR))/math.Pi) / 2 * n
	x, y = int(fx), int(fy)
	px = int((fx - float64(x)) * float64(size))
	py = int((fy - float64(y)) * float64(size))
	return
}

func radarGet(url string) (*http.Response, error) {
	resp, err := radarClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("sem acesso ao radar: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("radar respondeu %d", resp.StatusCode)
	}
	return resp, nil
}

func radarImage(url string) (image.Image, error) {
	resp, err := radarGet(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return png.Decode(resp.Body)
}

// pixelMax lê o maior dBZ num quadradinho em volta do ponto.
func pixelMax(img image.Image, px, py, rad int) float64 {
	b := img.Bounds()
	best := 0.0
	for dy := -rad; dy <= rad; dy++ {
		for dx := -rad; dx <= rad; dx++ {
			x, y := b.Min.X+px+dx, b.Min.Y+py+dy
			if x < b.Min.X || y < b.Min.Y || x >= b.Max.X || y >= b.Max.Y {
				continue
			}
			if d := colorDBZ(img.At(x, y).RGBA()); d > best {
				best = d
			}
		}
	}
	return best
}

// fetchRadar baixa os quadros do radar que ainda não tem (até 2 h para trás).
func (a *App) fetchRadar(lat, lon float64) error {
	resp, err := radarGet(radarIndexURL)
	if err != nil {
		return err
	}
	var idx struct {
		Host  string `json:"host"`
		Radar struct {
			Past []struct {
				Time int64  `json:"time"`
				Path string `json:"path"`
			} `json:"past"`
		} `json:"radar"`
	}
	err = json.NewDecoder(resp.Body).Decode(&idx)
	resp.Body.Close()
	if err != nil {
		return err
	}
	if idx.Host == "" || len(idx.Radar.Past) == 0 {
		return errors.New("radar sem imagens agora")
	}

	a.mu.Lock()
	if a.radar == nil || a.radar.Lat != lat || a.radar.Lon != lon {
		a.radar = &radarStore{V: radarVersion, Lat: lat, Lon: lon, Points: map[int64]RadarPoint{}}
	}
	covered := a.radar.Covered
	have := map[int64]bool{}
	for k := range a.radar.Points {
		have[k] = true
	}
	a.mu.Unlock()

	const size = 256
	x, y, px, py := tileOf(lat, lon, radarZoom, size)

	// cobertura: transparente = tem radar, preto = não tem (só confere uma vez)
	if covered == nil {
		if img, err := radarImage(fmt.Sprintf("%s/v2/coverage/0/%d/%d/%d/%d/0/0_0.png", idx.Host, size, radarZoom, x, y)); err == nil {
			_, _, _, al := img.At(img.Bounds().Min.X+px, img.Bounds().Min.Y+py).RGBA()
			c := al < 0x8000
			covered = &c
		}
	}

	got := map[int64]RadarPoint{}
	var lastErr error
	for _, f := range idx.Radar.Past {
		if have[f.Time] {
			continue
		}
		img, err := radarImage(fmt.Sprintf("%s%s/%d/%d/%d/%d/2/0_0.png", idx.Host, f.Path, size, radarZoom, x, y))
		if err != nil {
			lastErr = err
			continue
		}
		d := pixelMax(img, px, py, radarRadius)
		got[f.Time] = RadarPoint{Time: f.Time * 1000, DBZ: d, MMh: dbzToMMh(d)}
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.radar.Lat != lat || a.radar.Lon != lon {
		return nil // trocaram o lugar no meio
	}
	for k, v := range got {
		a.radar.Points[k] = v
	}
	cut := time.Now().Add(-radarKeep).Unix()
	for k := range a.radar.Points {
		if k < cut {
			delete(a.radar.Points, k)
		}
	}
	a.radar.Covered = covered
	a.radar.Fetched = time.Now()
	a.radar.Err = ""
	if len(got) == 0 && lastErr != nil {
		a.radar.Err = lastErr.Error()
	}
	a.saveRadarLocked()
	return lastErr
}

// radarOnce busca o radar uma vez (se a chuva estiver ligada) e avisa a tela.
func (a *App) radarOnce(set Settings) {
	if !set.RainEnabled || (set.RainLat == 0 && set.RainLon == 0) {
		return
	}
	if err := a.fetchRadar(set.RainLat, set.RainLon); err != nil {
		a.mu.Lock()
		if a.radar != nil && a.radar.Lat == set.RainLat && a.radar.Lon == set.RainLon {
			a.radar.Err = err.Error()
		}
		a.mu.Unlock()
	}
	a.emitRain()
	a.emit("radar", time.Now().UnixMilli())
}

func (a *App) kickRadarLocked() {
	if a.radarKick == nil {
		return
	}
	select {
	case a.radarKick <- struct{}{}:
	default:
	}
}

// radarLoop busca o radar a cada 10 min (alinhado 1 min depois dos quadros
// do RainViewer) enquanto a chuva estiver ligada.
func (a *App) radarLoop(stop <-chan struct{}) {
	for {
		a.mu.Lock()
		set := a.settings
		a.mu.Unlock()
		a.radarOnce(set)

		now := time.Now()
		next := now.Truncate(radarEvery).Add(radarEvery + time.Minute)
		if next.Sub(now) > radarEvery {
			next = next.Add(-radarEvery)
		}
		t := time.NewTimer(next.Sub(now))
		select {
		case <-stop:
			t.Stop()
			return
		case <-a.radarKick:
			t.Stop()
		case <-t.C:
		}
	}
}

// GetRainRadar devolve as leituras do radar entre from e to (ms).
func (a *App) GetRainRadar(fromMs, toMs int64) []RadarPoint {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := []RadarPoint{}
	set := a.settings
	if a.radar == nil || a.radar.Lat != set.RainLat || a.radar.Lon != set.RainLon {
		return out
	}
	for _, p := range a.radar.Points {
		if p.Time+int64(radarEvery/time.Millisecond) > fromMs && p.Time < toMs {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time < out[j].Time })
	return out
}
