package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"
)

// ---------- atividade (Bq): as amostras da pessoa ----------
//
// "Suas amostras" são cartões guardados pela pessoa (o que é, o que procurar,
// o tempo, se é pedra solta). Ficam em radiacode/atividade/amostras.json.
// Sem o arquivo, a lista começa com três minerais de sugestão; apagar ou
// guardar grava o arquivo, e as sugestões não voltam.

const actSamplesFile = "amostras.json"

// ActSample é uma amostra guardada.
type ActSample struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Sub      string   `json:"sub"` // linha curta embaixo do nome no cartão
	Nuclides []string `json:"nuclides"`
	SmpSec   int      `json:"smpSec"`
	MassG    float64  `json:"massG"` // 0 = não pesada (pedra solta)
	Loose    bool     `json:"loose"` // pedra solta: resultado em Bq da peça
	Prep     string   `json:"prep"`
	Note     string   `json:"note"` // "para o RC102" (o que esperar)
	Seed     bool     `json:"seed,omitempty"`
	T        int64    `json:"t"`
}

// ActSampleView é a amostra com a referência tirada das medições da pessoa.
type ActSampleView struct {
	ActSample
	// última "Medir objeto" com o mesmo nome: leitura com o objeto (µSv/h)
	RefUSv  float64 `json:"refUSv,omitempty"`
	RefDev  string  `json:"refDev,omitempty"` // radiacode | gmc
	RefT    int64   `json:"refT,omitempty"`
	RefDist string  `json:"refDist,omitempty"`
}

var actSeedSamples = []ActSample{
	{ID: "uraninita", Name: "Uraninita", Sub: "urânio", Nuclides: []string{"U-238", "Ra-226", "Th-232"}, SmpSec: 1800, Loose: true, Seed: true,
		Prep: "Pedra solta: o resultado sai em Bq da peça inteira, não precisa pesar. A forma é diferente do pote da calibração, então é aproximado.",
		Note: "forte: sai rápido"},
	{ID: "uranocircita", Name: "Uranocircita", Sub: "urânio", Nuclides: []string{"U-238", "Ra-226", "Th-232"}, SmpSec: 1800, Loose: true, Seed: true,
		Prep: "Pedra solta: o resultado sai em Bq da peça inteira. Tem muito beta, que o RC102 quase não conta para a atividade.",
		Note: "forte: sai rápido"},
	{ID: "euxenita", Name: "Euxenita", Sub: "urânio e tório?", Nuclides: []string{"U-238", "Ra-226", "Th-232"}, SmpSec: 3600, Loose: true, Seed: true,
		Prep: "Euxenita pode ter urânio e tório. O app só conta tório com as linhas de 911 ou 2.614 keV.",
		Note: "tório só se aparecer 911 ou 2.614 keV"},
}

func (a *App) actSamples() []ActSample {
	b, err := os.ReadFile(a.actPath(actSamplesFile))
	if err != nil {
		return append([]ActSample(nil), actSeedSamples...)
	}
	var out []ActSample
	if json.Unmarshal(b, &out) != nil {
		return append([]ActSample(nil), actSeedSamples...)
	}
	return out
}

// foldName deixa o nome comparável: minúsculo e sem acento.
func foldName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.NewReplacer("á", "a", "à", "a", "â", "a", "ã", "a", "é", "e", "ê", "e", "í", "i",
		"ó", "o", "ô", "o", "õ", "o", "ú", "u", "ü", "u", "ç", "c").Replace(s)
}

// actSampleViews junta a referência: a medição de objeto mais nova cujo nome
// contém o da amostra (de preferência a do RC102).
func (a *App) actSampleViews() []ActSampleView {
	list := a.actSamples()
	a.mu.Lock()
	ms := append([]SavedMeasure(nil), a.measurements...)
	a.mu.Unlock()
	out := make([]ActSampleView, 0, len(list))
	for _, s := range list {
		v := ActSampleView{ActSample: s}
		key := foldName(s.Name)
		var best *SavedMeasure
		bestRC := false
		for i := range ms {
			m := &ms[i]
			if key == "" || m.Test != "" || !strings.Contains(foldName(m.Name), key) {
				continue
			}
			rc := m.Device == devRC && m.Obj.USv > 0
			if !rc && m.Obj.USv <= 0 {
				continue
			}
			if best == nil || (rc && !bestRC) || (rc == bestRC && m.Time > best.Time) {
				best, bestRC = m, rc
			}
		}
		if best != nil {
			v.RefUSv, v.RefT, v.RefDist = best.Obj.USv, best.Time, best.Distance
			v.RefDev = "gmc"
			if bestRC {
				v.RefDev = devRC
			}
		}
		out = append(out, v)
	}
	return out
}

// SaveActSample guarda (ou troca) uma amostra em "Suas amostras" e devolve o id.
func (a *App) SaveActSample(s ActSample) (string, error) {
	s.Name = strings.TrimSpace(s.Name)
	if s.Name == "" {
		return "", errors.New("dê um nome para a amostra")
	}
	if len([]rune(s.Name)) > 60 {
		s.Name = string([]rune(s.Name)[:60])
	}
	var ok []string
	for _, id := range s.Nuclides {
		if actNuclide(id) != nil {
			ok = append(ok, id)
		}
	}
	if len(ok) == 0 {
		return "", errors.New("escolha pelo menos um radionuclídeo")
	}
	s.Nuclides = ok
	if s.MassG < 0 || s.SmpSec < 0 {
		return "", errors.New("massa ou tempo inválido")
	}
	s.Seed, s.T = false, time.Now().UnixMilli()
	list := a.actSamples()
	found := false
	for i := range list {
		if s.ID != "" && list[i].ID == s.ID {
			list[i], found = s, true
			break
		}
	}
	if !found {
		if s.ID == "" {
			b := make([]byte, 6)
			rand.Read(b)
			s.ID = hex.EncodeToString(b)
		}
		list = append([]ActSample{s}, list...)
	}
	a.actWriteJSON(actSamplesFile, list)
	return s.ID, nil
}

// DeleteActSample tira uma amostra de "Suas amostras".
func (a *App) DeleteActSample(id string) {
	list := a.actSamples()
	out := make([]ActSample, 0, len(list))
	for _, s := range list {
		if s.ID != id {
			out = append(out, s)
		}
	}
	a.actWriteJSON(actSamplesFile, out)
}
