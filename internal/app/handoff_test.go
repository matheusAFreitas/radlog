package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHandoff(t *testing.T) {
	// a mesma cópia aberta de novo: só mostra a janela
	if _, _, ok := shouldHandoff(tagArgs(nil)); ok {
		t.Fatal("a mesma cópia não pode se trocar")
	}
	// versão antiga, sem identificação: só mostra
	if _, _, ok := shouldHandoff([]string{"--bandeja"}); ok {
		t.Fatal("sem identificação não troca")
	}
	// outro .exe que existe: troca e repassa os argumentos (sem a identificação)
	other := filepath.Join(t.TempDir(), "GMC-Monitor-v9.9.9.exe")
	if err := os.WriteFile(other, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	exe, rest, ok := shouldHandoff([]string{"--bandeja", instArg + "9.9.9|" + other})
	if !ok || exe != other || len(rest) != 1 || rest[0] != "--bandeja" {
		t.Fatalf("troca: ok=%v exe=%q rest=%v", ok, exe, rest)
	}
	// versão mais antiga: não troca (B6), mesmo com o .exe existindo
	if _, _, ok := shouldHandoff([]string{instArg + "0.1.0|" + other}); ok {
		t.Fatal("trocou para uma versão mais antiga")
	}
	if v, older := olderInstance([]string{instArg + "0.1.0|" + other}); !older || v != "0.1.0" {
		t.Fatalf("mais antiga: %q %v", v, older)
	}
	if _, older := olderInstance([]string{instArg + AppVersion + "|" + other}); older {
		t.Fatal("a mesma versão não é mais antiga")
	}
	// .exe que não existe: não troca
	if _, _, ok := shouldHandoff([]string{instArg + "9.9.9|" + other + ".sumiu"}); ok {
		t.Fatal("trocou para um .exe que não existe")
	}
	// o --atualizando não é repassado
	_, _, rest = splitInstance([]string{updatingArg, "--bandeja"})
	if len(rest) != 1 || rest[0] != "--bandeja" {
		t.Fatalf("resto: %v", rest)
	}
	if runExe(`"C:\Program Files\GMC\GMC-Monitor-v1.6.1.exe" --bandeja`) != `C:\Program Files\GMC\GMC-Monitor-v1.6.1.exe` {
		t.Fatal("caminho do Run")
	}
}
