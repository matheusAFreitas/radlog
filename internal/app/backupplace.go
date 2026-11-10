package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// ---------- o lugar fora do backup (2.44) ----------
//
// chuva.json e chuva-radar.json guardam a latitude e a longitude da busca.
// No backup elas viram uma marca (hash lento das coordenadas), que só serve
// para saber, ao importar, se a chuva é do mesmo lugar. A cidade e as
// coordenadas ficam só nos ajustes deste PC.

const placeHashRounds = 100_000

// placeKey é a marca de um lugar. Lenta de propósito, para não dar para
// testar coordenada por coordenada.
func placeKey(lat, lon float64) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("radlog-lugar:%.4f,%.4f", lat, lon)))
	for i := 1; i < placeHashRounds; i++ {
		h = sha256.Sum256(h[:])
	}
	return hex.EncodeToString(h[:12])
}

// hidePlace troca lat/lon do arquivo pela marca do lugar.
func hidePlace(b []byte) []byte {
	var m map[string]json.RawMessage
	if json.Unmarshal(b, &m) != nil {
		return b
	}
	var lat, lon float64
	_ = json.Unmarshal(m["lat"], &lat)
	_ = json.Unmarshal(m["lon"], &lon)
	delete(m, "lat")
	delete(m, "lon")
	if lat != 0 || lon != 0 {
		k, _ := json.Marshal(placeKey(lat, lon))
		m["place"] = k
	}
	out, err := json.Marshal(m)
	if err != nil {
		return b
	}
	return out
}

// samePlace diz se a chuva de um backup é do lugar daqui: pela marca, ou
// pelas coordenadas nos backups antigos (antes da 2.44).
func samePlace(lat, lon, bLat, bLon float64, bPlace string) bool {
	if bPlace != "" {
		return placeKey(lat, lon) == bPlace
	}
	return (bLat != 0 || bLon != 0) && lat == bLat && lon == bLon
}
