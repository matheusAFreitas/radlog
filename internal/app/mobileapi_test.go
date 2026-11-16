package app

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

var updateMobileAPI = flag.Bool("update", false, "regrava testdata/mobile-api.txt")

// TestMobileAPI trava o contrato com a tela do celular (mobileapi.go): o
// nome, os argumentos e o retorno de cada método, e os campos JSON de todo
// tipo que passa por eles. Se mudou de propósito, rode com -update e
// confira a tela do celular.
func TestMobileAPI(t *testing.T) {
	got := mobileAPIDesc(t)
	path := filepath.Join("testdata", "mobile-api.txt")
	if *updateMobileAPI {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// no Windows o git pode trazer o arquivo com \r\n
	wantS := strings.ReplaceAll(string(want), "\r\n", "\n")
	if got != wantS {
		gl, wl := strings.Split(got, "\n"), strings.Split(wantS, "\n")
		var diff []string
		in := func(l []string, s string) bool {
			for _, x := range l {
				if x == s {
					return true
				}
			}
			return false
		}
		for _, s := range wl {
			if !in(gl, s) {
				diff = append(diff, "- "+s)
			}
		}
		for _, s := range gl {
			if !in(wl, s) {
				diff = append(diff, "+ "+s)
			}
		}
		t.Fatalf("o contrato com o celular mudou (confira android/app/src/main/assets/www/app.js e rode com -update):\n%s", strings.Join(diff, "\n"))
	}
}

func mobileAPIDesc(t *testing.T) string {
	at := reflect.TypeOf(&App{})
	structs := map[string]string{}
	var name func(reflect.Type) string
	name = func(ty reflect.Type) string {
		switch ty.Kind() {
		case reflect.Pointer:
			return "*" + name(ty.Elem())
		case reflect.Slice:
			return "[]" + name(ty.Elem())
		case reflect.Array:
			return fmt.Sprintf("[%d]%s", ty.Len(), name(ty.Elem()))
		case reflect.Map:
			return "map[" + name(ty.Key()) + "]" + name(ty.Elem())
		case reflect.Struct:
			n := ty.String()
			if ty.PkgPath() == "time" || n == "" {
				return n
			}
			if _, ok := structs[n]; ok {
				return n
			}
			structs[n] = "" // marca antes, para tipos que se repetem
			var fs []string
			for i := 0; i < ty.NumField(); i++ {
				f := ty.Field(i)
				if !f.IsExported() {
					continue
				}
				tag := f.Tag.Get("json")
				if tag == "-" {
					continue
				}
				k := strings.Split(tag, ",")[0]
				if k == "" {
					k = f.Name
				}
				if f.Anonymous && tag == "" {
					k = "(embutido)"
				}
				fs = append(fs, k+" "+name(f.Type))
			}
			structs[n] = strings.Join(fs, "; ")
			return n
		}
		return ty.String()
	}
	var lines []string
	for _, m := range MobileMethods {
		mt, ok := at.MethodByName(m)
		if !ok {
			t.Errorf("o celular chama %s, que não existe mais no App", m)
			continue
		}
		var in, out []string
		for i := 1; i < mt.Type.NumIn(); i++ {
			in = append(in, name(mt.Type.In(i)))
		}
		for i := 0; i < mt.Type.NumOut(); i++ {
			out = append(out, name(mt.Type.Out(i)))
		}
		lines = append(lines, fmt.Sprintf("func %s(%s) (%s)", m, strings.Join(in, ", "), strings.Join(out, ", ")))
	}
	var ts []string
	for n, f := range structs {
		ts = append(ts, fmt.Sprintf("type %s {%s}", n, f))
	}
	sort.Strings(ts)
	return strings.Join(append(lines, ts...), "\n") + "\n"
}
