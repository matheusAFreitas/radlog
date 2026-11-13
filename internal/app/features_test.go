package app

import (
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"radlog/internal/gmc"
	"radlog/internal/store"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := NewApp()
	a.dataDir, a.store, a.cal = dir, st, gmc.DefaultCalibration()
	a.settings = defaultSettings()
	// arquivos .gz abertos do RC102: no Windows o TempDir não sai com eles
	t.Cleanup(a.rcSpecClose)
	t.Cleanup(a.flushWrites) // config.json e uso.json vão para o disco depois (P2)
	return a
}

func TestSigmaFromBackground(t *testing.T) {
	a := newTestApp(t)
	r := Reading{CPM: 44}
	a.fillSigma(&r)
	if r.Sigma != nil {
		t.Fatal("sem dados não deveria ter sigma")
	}
	end := time.Now().Truncate(time.Minute)
	var pts []store.Point
	for i := 1; i <= 120; i++ {
		cpm := 25.0
		if i%2 == 0 {
			cpm = 29
		}
		pts = append(pts, store.Point{T: end.Add(-time.Duration(i) * time.Minute), CPM: cpm, Src: store.SrcLive})
	}
	if _, err := a.store.Import(pts); err != nil {
		t.Fatal(err)
	}
	a.bgAt = time.Time{} // força recalcular
	a.fillSigma(&r)
	if r.BgCPM != 27 || r.Sigma == nil || math.Abs(*r.Sigma-(44-27)/math.Sqrt(27)) > 1e-9 {
		t.Fatalf("bg %v sigma %v", r.BgCPM, r.Sigma)
	}
}

func TestHeatmap(t *testing.T) {
	a := newTestApp(t)
	now := time.Now()
	y, m, d := now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, time.Local)
	var pts []store.Point
	for day := 1; day <= 2; day++ { // ontem e anteontem, 24 h cada
		base := today.AddDate(0, 0, -day)
		for min := 0; min < 24*60; min++ {
			t0 := base.Add(time.Duration(min) * time.Minute)
			cpm := 20.0
			if t0.Hour() == 6 {
				cpm = 40 // radônio de madrugada
			}
			if t0.Hour() == 15 {
				cpm = 10
			}
			pts = append(pts, store.Point{T: t0, CPM: cpm, Src: store.SrcDevice})
		}
	}
	if _, err := a.store.Import(pts); err != nil {
		t.Fatal(err)
	}
	h, err := a.GetHeatmap(7)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Days) != 7 || len(h.Values) != 7 || h.PeakHour != 6 || h.LowHour != 15 || h.Minutes != 2*1440 {
		t.Fatalf("dias %d, pico %d, vale %d, min %d", len(h.Days), h.PeakHour, h.LowHour, h.Minutes)
	}
	if h.Counts[5][6] != 60 || h.Counts[6][0] != 0 || h.USv[5][6] == nil || math.Abs(*h.USv[5][6]-a.cal.USv(40)) > 1e-9 {
		t.Fatalf("contagens/µSv: %v %v", h.Counts[5], h.USv[5][6])
	}
	yest := h.Values[5]
	if yest[6] == nil || *yest[6] != 40 || *yest[0] != 20 || h.Values[6][0] != nil || h.Values[0][0] != nil {
		t.Fatal("células erradas")
	}
	if *h.Min != 10 || *h.Max != 40 {
		t.Fatalf("min %v max %v", *h.Min, *h.Max)
	}
}

func TestMeasureObject(t *testing.T) {
	a := newTestApp(t)
	a.demo.Active = true // permite medir sem contador; alimenta como demo
	if _, err := a.StartMeasurePhase("objeto", 60, ""); err == nil {
		t.Fatal("objeto antes do fundo deveria falhar")
	}
	if _, err := a.StartMeasurePhase("fundo", 60, "vidro verde"); err != nil {
		t.Fatal(err)
	}
	// fundo: 27 contagens em 60 s; contagens reais (não demo) são ignoradas
	for i := 0; i < 60; i++ {
		c := uint32(0)
		if i < 27 {
			c = 1
		}
		a.feedMeasure(0, c, true)
		a.feedMeasure(0, 5, false)
	}
	if st := a.GetMeasure(); st.Phase != "fundo-pronto" || st.Bg == nil || st.Bg.Counts != 27 || st.Bg.CPM != 27 {
		t.Fatalf("fundo: %+v", st)
	}
	if _, err := a.StartMeasurePhase("objeto", 0, ""); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 60; i++ {
		c := uint32(1)
		if i < 30 {
			c = 2 // 90 contagens no minuto
		}
		a.feedMeasure(0, c, true)
	}
	st := a.GetMeasure()
	r := st.Result
	if st.Phase != "pronto" || r == nil || r.NetCPM != 63 || r.Level != "sim" {
		t.Fatalf("resultado: %+v %+v", st, r)
	}
	if want := 63 / math.Sqrt(27+90); math.Abs(r.Sigma-want) > 1e-9 {
		t.Fatalf("sigma %v, esperava %v", r.Sigma, want)
	}
	found := false
	for _, e := range a.GetEvents(kindEvent) {
		if strings.Contains(e.Text, "[demo] Medição de vidro verde: +0,639 ± 0,110 µSv/h") {
			found = true
		}
	}
	if !found {
		t.Fatalf("evento da medição não registrado: %+v", a.GetEvents(""))
	}

	// diferença pequena: veredito "nada"
	r2 := compareMeasure(phaseResult(27, 60, a.cal.USv), phaseResult(30, 60, a.cal.USv), a.cal.USv)
	if r2.Level != "nada" {
		t.Fatalf("27 vs 30 deveria ser ruído: %+v", r2)
	}
}

func TestRain(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Hour)
	var times, vals []string
	for i := -5; i <= 2; i++ {
		times = append(times, fmt.Sprintf("%q", now.Add(time.Duration(i)*time.Hour).Format("2006-01-02T15:04")))
		v := "0"
		if i == -2 {
			v = "3.5"
		}
		if i == -1 {
			v = "null"
		}
		vals = append(vals, v)
	}
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if strings.Contains(r.URL.Path, "fora") {
			http.Error(w, "fora do ar", http.StatusServiceUnavailable)
			return
		}
		if strings.Contains(r.URL.Path, "search") {
			fmt.Fprint(w, `{"results":[{"name":"Brasília","admin1":"Distrito Federal","country":"Brasil","latitude":-15.79,"longitude":-47.88}]}`)
			return
		}
		if r.URL.Query().Get("hourly") != "precipitation,surface_pressure" || r.URL.Query().Get("timezone") != "GMT" {
			http.Error(w, "parâmetros", 400)
			return
		}
		fmt.Fprintf(w, `{"hourly":{"time":[%s],"precipitation":[%s]}}`, strings.Join(times, ","), strings.Join(vals, ","))
	}))
	defer srv.Close()
	geocodeURL, forecastURL = srv.URL+"/v1/search", srv.URL+"/v1/forecast"

	a := newTestApp(t)
	places, err := a.SearchPlace("brasília")
	if err != nil || len(places) != 1 || places[0].Detail != "Distrito Federal, Brasil" {
		t.Fatalf("busca: %+v %v", places, err)
	}
	if pts, _ := a.GetRain(0, math.MaxInt64); len(pts) != 0 {
		t.Fatal("desligado deveria vir vazio")
	}
	a.settings.RainEnabled, a.settings.RainLat, a.settings.RainLon = true, -15.79, -47.88
	from := now.Add(-3 * time.Hour).UnixMilli()
	pts, err := a.GetRain(from, now.UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	// o valor das -2h é a chuva da hora anterior: aparece na hora que começa
	// em -3h; o das -1h vem nulo (ignorado); o das 0h vira a hora de -1h
	if len(pts) != 2 || pts[0].MM != 3.5 || pts[0].Time != now.Add(-3*time.Hour).UnixMilli() || pts[1].Time != now.Add(-time.Hour).UnixMilli() {
		t.Fatalf("chuva: %+v", pts)
	}
	before := calls
	_, _ = a.GetRain(from, now.UnixMilli())
	if calls != before {
		t.Fatal("deveria usar o cache")
	}
	st := a.GetRainStatus()
	if !st.Enabled || st.Busy || st.Error != "" || st.Fetched == 0 || st.Next-st.Fetched != rainRefresh.Milliseconds() {
		t.Fatalf("status: %+v", st)
	}

	// serviço fora do ar: mostra o erro e não fica tentando a cada minuto
	a.settings.RainLat = -22.9
	forecastURL = srv.URL + "/fora"
	geoBefore := calls
	if _, err := a.GetRain(from, now.UnixMilli()); err == nil {
		t.Fatal("deveria dar erro")
	}
	st = a.GetRainStatus()
	if st.Error == "" || st.Busy || st.Fetched != 0 {
		t.Fatalf("status com erro: %+v", st)
	}
	_, _ = a.GetRain(from, now.UnixMilli())
	if calls != geoBefore+1 {
		t.Fatalf("insistiu depois do erro: %d chamadas", calls-geoBefore)
	}
	forecastURL = srv.URL + "/v1/forecast"

	// outro app lendo o arquivo salvo
	b := NewApp()
	b.dataDir = a.dataDir
	b.loadRain()
	if b.rain == nil || len(b.rain.Hours) != 7 {
		t.Fatalf("cache em disco: %+v", b.rain)
	}
}

func TestHeatmapMonth(t *testing.T) {
	a := newTestApp(t)
	if a.GetDataStart() != 0 {
		t.Fatal("sem dados deveria ser 0")
	}
	// três meses atrás, dia 10, 6h: 50 CPM por uma hora
	now := time.Now()
	old := time.Date(now.Year(), now.Month()-3, 10, 6, 0, 0, 0, now.Location())
	var pts []store.Point
	for m := 0; m < 60; m++ {
		pts = append(pts, store.Point{T: old.Add(time.Duration(m) * time.Minute), CPM: 50, Src: store.SrcDevice})
	}
	if _, err := a.store.Import(pts); err != nil {
		t.Fatal(err)
	}
	first := time.Date(old.Year(), old.Month(), 1, 0, 0, 0, 0, now.Location())
	if a.GetDataStart() != first.UnixMilli() {
		t.Fatalf("início %v", time.UnixMilli(a.GetDataStart()))
	}
	h, err := a.GetHeatmapMonth(old.Year(), int(old.Month()))
	if err != nil {
		t.Fatal(err)
	}
	want := first.AddDate(0, 1, 0).AddDate(0, 0, -1).Day()
	if len(h.Days) != want || h.Days[0] != first.UnixMilli() {
		t.Fatalf("dias %d (queria %d)", len(h.Days), want)
	}
	if v := h.Values[9][6]; v == nil || *v != 50 || h.Counts[9][6] != 60 || h.PeakHour != 6 {
		t.Fatalf("dia 10 6h: %v", h.Values[9][6])
	}
	// mês atual vai só até hoje
	cur, _ := a.GetHeatmapMonth(now.Year(), int(now.Month()))
	if len(cur.Days) != now.Day() {
		t.Fatalf("mês atual: %d dias", len(cur.Days))
	}
	// mês futuro: vazio, mas sem nil (a tela usa .length)
	fut, _ := a.GetHeatmapMonth(now.Year()+1, 1)
	if fut.Days == nil || len(fut.Days) != 0 || fut.Values == nil {
		t.Fatalf("futuro: %+v", fut)
	}
	if _, err := a.GetHeatmapMonth(2026, 13); err == nil {
		t.Fatal("mês 13 deveria dar erro")
	}
}
