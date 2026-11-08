// Command sign assina o SHA256SUMS.txt das releases com Ed25519.
//
//	go run ./scripts/sign gen              cria um par de chaves novo
//	go run ./scripts/sign sign ARQUIVO     escreve ARQUIVO.sig (chave em RADLOG_SIGN_KEY)
//	go run ./scripts/sign verify ARQUIVO   confere ARQUIVO.sig com a chave pública do app
//
// A chave privada fica só no segredo RADLOG_SIGN_KEY do GitHub (e onde quem
// gerou guardar). A pública vai em internal/updsig/updsig.go.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"radlog/internal/updsig"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "gen":
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		check(err)
		fmt.Println("Chave PRIVADA (vai no segredo RADLOG_SIGN_KEY do GitHub; não compartilhe):")
		fmt.Println(base64.StdEncoding.EncodeToString(priv.Seed()))
		fmt.Println()
		fmt.Println("Chave pública (vai no app, internal/updsig/updsig.go):")
		fmt.Println(base64.StdEncoding.EncodeToString(pub))
	case "sign":
		if len(os.Args) != 3 {
			usage()
		}
		seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(os.Getenv("RADLOG_SIGN_KEY")))
		if err != nil || len(seed) != ed25519.SeedSize {
			fail("RADLOG_SIGN_KEY ausente ou inválida (esperado: 32 bytes em base64)")
		}
		priv := ed25519.NewKeyFromSeed(seed)
		data, err := os.ReadFile(os.Args[2])
		check(err)
		sig := ed25519.Sign(priv, data)
		if !updsig.Verify(data, []byte(base64.StdEncoding.EncodeToString(sig))) {
			fail("a chave em RADLOG_SIGN_KEY não é a que o app conhece (chave pública em internal/updsig/updsig.go)")
		}
		check(os.WriteFile(os.Args[2]+".sig", []byte(base64.StdEncoding.EncodeToString(sig)+"\n"), 0o644))
		fmt.Println("assinado:", os.Args[2]+".sig")
	case "verify":
		if len(os.Args) != 3 {
			usage()
		}
		data, err := os.ReadFile(os.Args[2])
		check(err)
		sig, err := os.ReadFile(os.Args[2] + ".sig")
		check(err)
		if !updsig.Verify(data, sig) {
			fail("assinatura NÃO confere")
		}
		fmt.Println("assinatura confere")
	default:
		usage()
	}
}

func usage() { fail("uso: sign gen | sign ARQUIVO | verify ARQUIVO") }

func check(err error) {
	if err != nil {
		fail(err.Error())
	}
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
