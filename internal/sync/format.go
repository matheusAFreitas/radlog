package sync

import (
	"encoding/binary"
	"encoding/json"
	"errors"
)

// ---------- formato dos arquivos da nuvem (v2; docs/nuvem-formato.md) ----------
//
// Cada arquivo da pasta (menos o radlog-nuvem.json) é Seal(nome, envelope):
//
//	"RL" | versão (1 byte) | tipo (1 byte) | conteúdo
//
// A versão é a do formato do conteúdo (FormatVersion); quem lê um arquivo de
// versão maior não marca como visto para sempre: tenta de novo quando o app
// atualizar (auditoria 2.79.0, A2).
//
// O nome no Drive é Prefix(aparelho) + FileName(chave), 8 + 24 letras
// hexadecimais. O prefixo agrupa os arquivos de cada aparelho (para remover
// um aparelho sem precisar adivinhar os nomes) e não mostra data nem tipo.

// FormatVersion é a versão do formato que este app escreve e entende.
const FormatVersion = 2

// MetaVersion é a versão do radlog-nuvem.json que este app cria. Nuvens com
// a versão 1 (2.79.x) precisam ser começadas de novo.
const MetaVersion = 2

// Tipos de arquivo.
const (
	KindDevice     byte = 'D' // JSON Device
	KindCommon     byte = 'C' // zip com os pequenos (saídas, medições, anotações…)
	KindDay        byte = 'P' // zip com um dia (minutos, alertas)
	KindSpectra    byte = 'S' // zip com os espectros de um mês
	KindAttachment byte = 'X' // um anexo de medição (cabeçalho JSON + bytes)
)

// ErrNewer: arquivo de um formato mais novo que este app.
var ErrNewer = errors.New("arquivo de uma versão mais nova do RadLog")

// ErrFormat: o conteúdo não é um arquivo da nuvem do RadLog.
var ErrFormat = errors.New("arquivo da nuvem em formato desconhecido")

// Wrap monta o envelope.
func Wrap(kind byte, body []byte) []byte {
	out := make([]byte, 0, len(body)+4)
	out = append(out, 'R', 'L', FormatVersion, kind)
	return append(out, body...)
}

// Unwrap lê o envelope: tipo e conteúdo. ErrNewer para versão maior.
func Unwrap(b []byte) (kind byte, body []byte, err error) {
	if len(b) < 4 || b[0] != 'R' || b[1] != 'L' || b[2] == 0 {
		return 0, nil, ErrFormat
	}
	if b[2] > FormatVersion {
		return 0, nil, ErrNewer
	}
	return b[3], b[4:], nil
}

// Prefix é o começo do nome de todos os arquivos de um aparelho.
func (k *Keys) Prefix(dev string) string { return k.FileName("aparelho|" + dev)[:8] }

// Name é o nome no Drive do arquivo key de um aparelho.
func (k *Keys) Name(dev, key string) string {
	return k.Prefix(dev) + k.FileName(dev + "|" + key)[:24]
}

// Device é o arquivo de cada aparelho.
type Device struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	LastUp int64  `json:"lastUp"` // ms
	// Writer muda a cada instalação (nuvem ligada de novo, pasta copiada):
	// dois PCs com o mesmo ID e Writers diferentes são o caso "PC copiado"
	Writer string `json:"writer"`
	Seq    int64  `json:"seq"` // sobe a cada gravação do Writer
}

// AttachmentHeader identifica um anexo pela medição (início em s) e o nome.
type AttachmentHeader struct {
	Start int64  `json:"start"`
	File  string `json:"file"`
}

// WrapAttachment monta o conteúdo de um anexo: tamanho do cabeçalho (4 bytes),
// cabeçalho JSON e os bytes do arquivo.
func WrapAttachment(h AttachmentHeader, data []byte) []byte {
	hb, _ := json.Marshal(h)
	out := make([]byte, 4, 4+len(hb)+len(data))
	binary.BigEndian.PutUint32(out, uint32(len(hb)))
	out = append(out, hb...)
	return append(out, data...)
}

// UnwrapAttachment desfaz o WrapAttachment.
func UnwrapAttachment(b []byte) (AttachmentHeader, []byte, error) {
	var h AttachmentHeader
	if len(b) < 4 {
		return h, nil, ErrFormat
	}
	n := int(binary.BigEndian.Uint32(b))
	if n <= 0 || 4+n > len(b) {
		return h, nil, ErrFormat
	}
	if err := json.Unmarshal(b[4:4+n], &h); err != nil || h.File == "" {
		return h, nil, ErrFormat
	}
	return h, b[4+n:], nil
}
