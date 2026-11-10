package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
)

// ---------- o lugar fora do backup (2.44, refeito na 2.44.1) ----------
//
// chuva.json e chuva-radar.json guardam a latitude e a longitude da busca.
// Na 2.44.0 elas viravam uma marca (hash das coordenadas) no backup. Só que
// os lugares possíveis são poucos (as cidades do serviço de busca), então
// dava para testar todos e achar a cidade. Desde a 2.44.1 o backup não leva
// nada do lugar: nem coordenada, nem marca, nem a mensagem de erro (que
// trazia o endereço pedido). Para saber, ao importar, se a chuva é do mesmo
// lugar, o app compara a pressão das horas que os dois já têm: a pressão na
// superfície depende da altitude e do tempo de cada lugar, então só bate
// quando é a mesma cidade.

const placeHashRounds = 100_000

// placeKey é a marca de lugar dos backups da 2.44.0 (só para ler).
func placeKey(lat, lon float64) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("radlog-lugar:%.4f,%.4f", lat, lon)))
	for i := 1; i < placeHashRounds; i++ {
		h = sha256.Sum256(h[:])
	}
	return hex.EncodeToString(h[:12])
}

// hidePlace tira do arquivo tudo que aponta o lugar.
func hidePlace(b []byte) []byte {
	var m map[string]json.RawMessage
	if json.Unmarshal(b, &m) != nil {
		return b
	}
	for _, k := range []string{"lat", "lon", "place", "err"} {
		delete(m, k)
	}
	out, err := json.Marshal(m)
	if err != nil {
		return b
	}
	return out
}

// samePlace diz se a chuva de um backup antigo é do lugar daqui: pela marca
// (2.44.0) ou pelas coordenadas (antes da 2.44). ok=false: o backup não diz.
func samePlace(lat, lon, bLat, bLon float64, bPlace string) (same, ok bool) {
	if bPlace != "" {
		return placeKey(lat, lon) == bPlace, true
	}
	if bLat != 0 || bLon != 0 {
		return lat == bLat && lon == bLon, true
	}
	return false, false
}

const (
	pressMatchHours = 12  // horas em comum, no mínimo
	pressMatchHPa   = 0.5 // diferença média máxima (hPa)
)

// samePressure diz se duas séries de pressão são do mesmo lugar: pelo menos
// pressMatchHours horas em comum e diferença média de até pressMatchHPa.
func samePressure(here, there map[int64]float64) bool {
	n, sum := 0, 0.0
	for t, v := range there {
		if w, ok := here[t]; ok {
			n++
			sum += math.Abs(v - w)
		}
	}
	return n >= pressMatchHours && sum/float64(n) <= pressMatchHPa
}

// noURL tira o endereço de um erro de rede (ele traz a coordenada ou o
// pedaço do mapa pedido).
func noURL(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) && ue.Err != nil {
		return ue.Err
	}
	return err
}
