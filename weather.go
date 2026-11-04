package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ---------- chuva (Open-Meteo, grátis e sem chave) ----------
//
// A chuva "lava" do ar os produtos do radônio e os deposita no chão, o que dá
// picos de alguns CPM que somem em poucas horas. Marcar a chuva no gráfico
// ajuda a não confundir isso com outra coisa.

var (
	geocodeURL  = "https://geocoding-api.open-meteo.com/v1/search"
	forecastURL = "https://api.open-meteo.com/v1/forecast"
	httpClient  = &http.Client{Timeout: 15 * time.Second}
)

const rainRefresh = 30 * time.Minute

// rainState é o que a tela mostra sobre a busca de chuva.
type rainState struct {
	busy  bool
	err   string
	errAt time.Time
}

// RainStatus diz se a busca de chuva está ligada, buscando agora, quando
// atualizou e se deu erro.
type RainStatus struct {
	Enabled bool   `json:"enabled"`
	Place   string `json:"place"`
	Busy    bool   `json:"busy"`
	Fetched int64  `json:"fetched"` // última busca que deu certo (ms), 0 = nunca
	Next    int64  `json:"next"`    // quando busca de novo (ms)
	Error   string `json:"error"`   // erro da última tentativa

	// radar (RainViewer)
	RadarFetched int64       `json:"radarFetched"` // última leitura do radar (ms)
	RadarCovered *bool       `json:"radarCovered"` // o local tem cobertura? (nil = não sabe)
	RadarError   string      `json:"radarError"`
	RadarNow     *RadarPoint `json:"radarNow"` // leitura mais recente
}

// GetRainStatus devolve o estado da busca de chuva.
func (a *App) GetRainStatus() RainStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.rainStatusLocked()
}

func (a *App) rainStatusLocked() RainStatus {
	set := a.settings
	st := RainStatus{
		Enabled: set.RainEnabled && !(set.RainLat == 0 && set.RainLon == 0),
		Place:   set.RainPlace,
		Busy:    a.rainSt.busy,
		Error:   a.rainSt.err,
	}
	if a.rain != nil && a.rain.Lat == set.RainLat && a.rain.Lon == set.RainLon && !a.rain.Fetched.IsZero() {
		st.Fetched = a.rain.Fetched.UnixMilli()
		st.Next = a.rain.Fetched.Add(rainRefresh).UnixMilli()
	}
	if st.Error != "" {
		st.Next = a.rainSt.errAt.Add(rainRetry).UnixMilli()
	}
	if r := a.radar; r != nil && r.Lat == set.RainLat && r.Lon == set.RainLon {
		if !r.Fetched.IsZero() {
			st.RadarFetched = r.Fetched.UnixMilli()
		}
		st.RadarCovered, st.RadarError = r.Covered, r.Err
		var last int64
		for k, p := range r.Points {
			if k > last {
				last = k
				q := p
				st.RadarNow = &q
			}
		}
	}
	return st
}

func (a *App) emitRain() {
	a.emit("rain-status", a.GetRainStatus())
}

// depois de um erro, espera um pouco antes de tentar de novo
const rainRetry = 10 * time.Minute

// Place é um resultado da busca de cidade.
type Place struct {
	Name      string  `json:"name"`
	Detail    string  `json:"detail"` // estado, país
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// RainPoint é a chuva de uma hora.
type RainPoint struct {
	Time int64   `json:"time"` // início da hora (ms)
	MM   float64 `json:"mm"`
}

// SearchPlace procura uma cidade pelo nome.
func (a *App) SearchPlace(name string) ([]Place, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return []Place{}, nil
	}
	q := url.Values{}
	q.Set("name", name)
	q.Set("count", "6")
	q.Set("language", "pt")
	q.Set("format", "json")
	var body struct {
		Results []struct {
			Name      string  `json:"name"`
			Admin1    string  `json:"admin1"`
			Country   string  `json:"country"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"results"`
	}
	if err := getJSON(geocodeURL+"?"+q.Encode(), &body); err != nil {
		return nil, err
	}
	out := []Place{}
	for _, r := range body.Results {
		var parts []string
		for _, p := range []string{r.Admin1, r.Country} {
			if p != "" {
				parts = append(parts, p)
			}
		}
		out = append(out, Place{Name: r.Name, Detail: strings.Join(parts, ", "), Latitude: r.Latitude, Longitude: r.Longitude})
	}
	return out, nil
}

func getJSON(u string, v any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sem acesso ao serviço de clima: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("serviço de clima respondeu %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func (a *App) rainPath() string { return filepath.Join(a.dataDir, "chuva.json") }

// rainCacheVersion 2: a chave é o INÍCIO da hora em que a chuva caiu (o
// Open-Meteo dá a soma da hora ANTERIOR ao horário informado; a versão 1
// guardava o horário informado e desenhava a chuva uma hora atrasada).
const rainCacheVersion = 2

type rainCache struct {
	V       int               `json:"v"`
	Lat     float64           `json:"lat"`
	Lon     float64           `json:"lon"`
	Fetched time.Time         `json:"fetched"`
	Hours   map[int64]float64 `json:"hours"` // unix (s) do início da hora -> mm
}

// fetchRain busca os últimos 7 dias + hoje e junta ao que já tinha.
func (a *App) fetchRain(lat, lon float64) error {
	a.mu.Lock()
	if a.rainSt.busy {
		a.mu.Unlock()
		return nil
	}
	a.rainSt.busy = true
	a.mu.Unlock()
	a.emitRain()
	err := a.fetchRainNow(lat, lon)
	a.mu.Lock()
	a.rainSt.busy = false
	if err != nil {
		a.rainSt.err, a.rainSt.errAt = err.Error(), time.Now()
	} else {
		a.rainSt.err = ""
	}
	a.mu.Unlock()
	a.emitRain()
	return err
}

func (a *App) fetchRainNow(lat, lon float64) error {
	q := url.Values{}
	q.Set("latitude", fmt.Sprintf("%.4f", lat))
	q.Set("longitude", fmt.Sprintf("%.4f", lon))
	q.Set("hourly", "precipitation")
	// primeira busca (ou cache curto): pega 31 dias para trás, para a tela
	// "Chuva e radiação" ter com o que comparar; depois, só 7
	past := "31"
	a.mu.Lock()
	if c := a.rain; c != nil && c.Lat == lat && c.Lon == lon && c.V == rainCacheVersion {
		cut := time.Now().Add(-25 * 24 * time.Hour).Unix()
		for k := range c.Hours {
			if k < cut {
				past = "7"
				break
			}
		}
	}
	a.mu.Unlock()
	q.Set("past_days", past)
	q.Set("forecast_days", "1")
	q.Set("timezone", "GMT")
	var body struct {
		Hourly struct {
			Time          []string   `json:"time"`
			Precipitation []*float64 `json:"precipitation"`
		} `json:"hourly"`
	}
	if err := getJSON(forecastURL+"?"+q.Encode(), &body); err != nil {
		return err
	}
	if len(body.Hourly.Time) == 0 {
		return errors.New("serviço de clima não devolveu dados")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.rain == nil || a.rain.Lat != lat || a.rain.Lon != lon || a.rain.V != rainCacheVersion {
		a.rain = &rainCache{V: rainCacheVersion, Lat: lat, Lon: lon, Hours: map[int64]float64{}}
	}
	for i, ts := range body.Hourly.Time {
		if i >= len(body.Hourly.Precipitation) || body.Hourly.Precipitation[i] == nil {
			continue
		}
		t, err := time.Parse("2006-01-02T15:04", ts)
		if err != nil {
			continue
		}
		// o valor das 16:00 é a chuva das 15:00 às 16:00
		a.rain.Hours[t.Add(-time.Hour).Unix()] = *body.Hourly.Precipitation[i]
	}
	// guarda só os últimos 60 dias
	cut := time.Now().Add(-60 * 24 * time.Hour).Unix()
	for k := range a.rain.Hours {
		if k < cut {
			delete(a.rain.Hours, k)
		}
	}
	a.rain.Fetched = time.Now()
	if a.dataDir != "" {
		if b, err := json.Marshal(a.rain); err == nil {
			_ = os.WriteFile(a.rainPath(), b, 0o644)
		}
	}
	return nil
}

func (a *App) loadRain() {
	b, err := os.ReadFile(a.rainPath())
	if err != nil {
		return
	}
	var c rainCache
	if json.Unmarshal(b, &c) == nil && c.Hours != nil && c.V == rainCacheVersion {
		a.rain = &c
	}
}

// RefreshRain busca a chuva agora, sem esperar os 30 min (clique no aviso de
// chuva do gráfico).
func (a *App) RefreshRain() RainStatus {
	a.mu.Lock()
	set := a.settings
	a.rainSt.err = ""
	a.mu.Unlock()
	if set.RainEnabled && !(set.RainLat == 0 && set.RainLon == 0) {
		_ = a.fetchRain(set.RainLat, set.RainLon)
		a.radarOnce(set)
	}
	return a.GetRainStatus()
}

// GetRain devolve a chuva por hora entre from e to (ms). Busca de novo no
// serviço se os dados tiverem mais de 30 minutos. Sem local configurado, vem
// vazio.
func (a *App) GetRain(fromMs, toMs int64) ([]RainPoint, error) {
	a.mu.Lock()
	set := a.settings
	stale := a.rain == nil || a.rain.Lat != set.RainLat || a.rain.Lon != set.RainLon || time.Since(a.rain.Fetched) > rainRefresh
	if a.rainSt.err != "" && time.Since(a.rainSt.errAt) < rainRetry {
		stale = false // deu erro há pouco: não fica insistindo a cada minuto
	}
	a.mu.Unlock()
	if !set.RainEnabled || (set.RainLat == 0 && set.RainLon == 0) {
		return []RainPoint{}, nil
	}
	var fetchErr error
	if stale {
		fetchErr = a.fetchRain(set.RainLat, set.RainLon)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	out := []RainPoint{}
	if a.rain != nil && a.rain.Lat == set.RainLat && a.rain.Lon == set.RainLon {
		for k, mm := range a.rain.Hours {
			ms := k * 1000
			if ms+3600_000 > fromMs && ms < toMs {
				out = append(out, RainPoint{Time: ms, MM: mm})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time < out[j].Time })
	if len(out) == 0 && fetchErr != nil {
		return out, fetchErr
	}
	return out, nil
}
