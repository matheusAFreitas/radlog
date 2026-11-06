package radiacode

import (
	"encoding/binary"
	"fmt"
)

// assembler junta pedaços de resposta (USB lê em blocos de 256, Bluetooth em
// notificações de ~20 bytes) até completar o tamanho anunciado.
type assembler struct {
	want int // bytes que faltam (-1 = esperando o cabeçalho)
	data []byte
}

func newAssembler() *assembler { return &assembler{want: -1} }

// add devolve a resposta completa (sem o u32 de tamanho) quando terminar.
func (a *assembler) add(p []byte) ([]byte, bool, error) {
	if a.want < 0 {
		if len(p) < 4 {
			return nil, false, fmt.Errorf("pedaço sem cabeçalho (%d bytes)", len(p))
		}
		n := int(binary.LittleEndian.Uint32(p))
		if n < 0 || n > 1<<20 {
			return nil, false, fmt.Errorf("tamanho de resposta absurdo: %d", n)
		}
		a.want, a.data = n, make([]byte, 0, n)
		p = p[4:]
	}
	a.data = append(a.data, p...)
	if len(a.data) > a.want {
		return nil, false, fmt.Errorf("resposta maior que o anunciado (%d > %d)", len(a.data), a.want)
	}
	if len(a.data) == a.want {
		out := a.data
		a.want, a.data = -1, nil
		return out, true, nil
	}
	return nil, false, nil
}
