// Package sync é a parte da sincronização entre aparelhos que não depende
// do app do PC (auditoria A1): o pareamento e o envelope cifrado (cpace.go),
// o inventário de cada lado, a conta do que falta de cada lado e a ordem da
// troca (inventário → puxa → mescla → manda). Quem leva os bytes de um lado
// ao outro é um Transport: hoje a rede local (HTTP no app do PC); depois a
// nuvem e o celular usam a mesma conta com outro transporte.
//
// O formato das mensagens é o mesmo da sincronização pela rede desde a
// 2.7.1: um PC com esta versão conversa com um de versão anterior.
package sync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"time"

	"radlog/internal/store"
)

// RcInv é o inventário do Radiacode de um aparelho.
type RcInv struct {
	Days  map[string]string `json:"days"`  // "AAAA-MM.csv|2026-11-07" → "n:hash"
	Files []string          `json:"files"` // "espectros/…", "bruto/…" (já fechados)
}

// Inventory é o que um lado tem: o resumo de cada dia de minutos do GMC-800,
// os alertas guardados e, a partir da 2.7.1, o Radiacode e os anexos (Rc nil
// = versão antiga, que não entende essa parte).
type Inventory struct {
	Days   map[string]string `json:"days"`
	Alerts []string          `json:"alerts"`
	Rc     *RcInv            `json:"rc,omitempty"`
	Anexos []string          `json:"anexos,omitempty"`
}

// Request é o pedido de um pacote: o que mandar de um lado para o outro.
type Request struct {
	Days    []string `json:"days"`
	Alerts  []string `json:"alerts"`
	RcDays  []string `json:"rcDays,omitempty"`
	RcFiles []string `json:"rcFiles,omitempty"`
	Anexos  []string `json:"anexos,omitempty"`
	Rc      bool     `json:"rc,omitempty"` // quem pede entende o pacote novo
}

// Want é o que from tem e to não tem (ou tem diferente): para puxar,
// Want(deles, meu); para mandar, Want(meu, deles). Radiacode e anexos só
// entram quando os dois lados entendem essa parte.
func Want(from, to Inventory) Request {
	var r Request
	for d, h := range from.Days {
		if to.Days[d] != h {
			r.Days = append(r.Days, d)
		}
	}
	sort.Strings(r.Days)
	r.Alerts = missing(from.Alerts, to.Alerts)
	if from.Rc != nil && to.Rc != nil {
		r.Rc = true
		r.RcDays, r.RcFiles = RcWant(*from.Rc, *to.Rc)
		r.Anexos = missing(from.Anexos, to.Anexos)
	}
	return r
}

// RcWant compara inventários do Radiacode: os dias com linhas diferentes e
// os arquivos que só from tem.
func RcWant(from, to RcInv) (days, files []string) {
	for k, h := range from.Days {
		if to.Days[k] != h {
			days = append(days, k)
		}
	}
	files = missing(from.Files, to.Files)
	sort.Strings(days)
	sort.Strings(files)
	return
}

// missing devolve o que está em from e não em to, na ordem de from.
func missing(from, to []string) []string {
	have := make(map[string]bool, len(to))
	for _, k := range to {
		have[k] = true
	}
	var out []string
	for _, k := range from {
		if !have[k] {
			out = append(out, k)
		}
	}
	return out
}

// DigestDays resume os minutos de cada dia (UTC) como "n:hash". Dois lados
// com o mesmo resumo num dia têm os mesmos minutos.
func DigestDays(pts []store.Point) map[string]string {
	mins := map[string][]int64{}
	for _, p := range pts {
		d := p.T.UTC().Format("2006-01-02")
		mins[d] = append(mins[d], p.T.Unix()/60)
	}
	out := map[string]string{}
	for d, m := range mins {
		sort.Slice(m, func(i, j int) bool { return m[i] < m[j] })
		h := fnv.New64a()
		n := 0
		for i, x := range m {
			if i > 0 && x == m[i-1] {
				continue
			}
			n++
			h.Write([]byte(strconv.FormatInt(x, 36) + ","))
		}
		out[d] = fmt.Sprintf("%d:%x", n, h.Sum64())
	}
	return out
}

// PairTranscript junta o que os dois lados trocaram no pareamento, com o
// tamanho de cada pedaço na frente (não dá para deslocar bytes de um campo
// para o outro).
func PairTranscript(sid, cid string, cy []byte, sidSrv string, sy []byte) []byte {
	var b bytes.Buffer
	for _, x := range [][]byte{[]byte(sid), []byte(cid), cy, []byte(sidSrv), sy} {
		b.WriteString(strconv.Itoa(len(x)))
		b.WriteByte(':')
		b.Write(x)
	}
	return b.Bytes()
}

// ---------- a troca ----------

// Transport leva um pedido ao outro lado e traz a resposta. op é
// "inventory", "pull" ou "push".
type Transport interface {
	Call(op string, body []byte, timeout time.Duration) ([]byte, error)
}

// Local é o que a troca precisa do aparelho daqui.
type Local interface {
	// Inventory resume o que este lado tem agora.
	Inventory() (Inventory, error)
	// Build monta o pacote (zip) com o que o pedido lista.
	Build(Request) ([]byte, error)
	// Merge junta um pacote recebido.
	Merge(pkg []byte) error
}

// Exchange faz uma troca completa depois do inventário do outro lado:
// puxa e mescla o que falta aqui, refaz o inventário e manda o que falta
// lá. Devolve a resposta do "push" (o resumo do que o outro lado juntou).
func Exchange(t Transport, l Local, theirs Inventory) ([]byte, error) {
	mine, err := l.Inventory()
	if err != nil {
		return nil, err
	}
	pb, _ := json.Marshal(Want(theirs, mine))
	pkg, err := t.Call("pull", pb, 5*time.Minute)
	if err != nil {
		return nil, err
	}
	if err := l.Merge(pkg); err != nil {
		return nil, err
	}
	if mine, err = l.Inventory(); err != nil {
		return nil, err
	}
	delta, err := l.Build(Want(mine, theirs))
	if err != nil {
		return nil, err
	}
	return t.Call("push", delta, 5*time.Minute)
}
