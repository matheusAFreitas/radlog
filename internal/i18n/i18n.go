// Package i18n traduz os textos do app (escritos em português) para outros
// idiomas na hora de mostrar: relatórios em PDF, menu da bandeja, título da
// janela e linhas do registro técnico. É o mesmo arquivo e o mesmo jeito da
// tela (frontend/dist/i18n.js): frases inteiras e moldes com partes
// variáveis ({0}, {1}…) que são capturadas e recolocadas no inglês.
package i18n

import (
	_ "embed"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

//go:embed en.tsv
var enTSV string

type pattern struct {
	re     *regexp.Regexp
	out    string // inglês com {n} = captura n (na ordem do português)
	order  []int  // número {n} de cada captura, na ordem do texto
	pieces []string
	lead   string // 3 primeiras letras do texto fixo do começo ("" = começa variável)
}

type dict struct {
	exact  map[string]string
	byLead map[string][]*pattern
	any    []*pattern
	mu     sync.Mutex
	cache  map[string]string
}

var (
	once sync.Once
	en   *dict
	ph   = regexp.MustCompile(`\{(\d+)\}`)
)

func load() {
	d := &dict{exact: map[string]string{}, byLead: map[string][]*pattern{}, cache: map[string]string{}}
	var pats []*pattern
	for _, ln := range strings.Split(enTSV, "\n") {
		ln = strings.TrimRight(ln, "\r")
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		pt, out, ok := strings.Cut(ln, "\t")
		if !ok || out == "" {
			continue
		}
		if !ph.MatchString(pt) {
			d.exact[pt] = out
			continue
		}
		p := &pattern{out: out}
		var rx strings.Builder
		rx.WriteString("^")
		last := 0
		for _, m := range ph.FindAllStringSubmatchIndex(pt, -1) {
			lit := pt[last:m[0]]
			rx.WriteString(regexp.QuoteMeta(lit))
			if len(strings.TrimSpace(lit)) >= 2 {
				p.pieces = append(p.pieces, lit)
			}
			n := 0
			for _, c := range pt[m[2]:m[3]] {
				n = n*10 + int(c-'0')
			}
			p.order = append(p.order, n)
			// a parte variável não atravessa " · " (o app junta pedaços assim)
			rx.WriteString(`([^·]+?)`)
			last = m[1]
		}
		tail := pt[last:]
		rx.WriteString(regexp.QuoteMeta(tail) + "$")
		if len(strings.TrimSpace(tail)) >= 2 {
			p.pieces = append(p.pieces, tail)
		}
		re, err := regexp.Compile(rx.String())
		if err != nil {
			continue
		}
		p.re = re
		lead := strings.TrimSpace(pt[:strings.Index(pt, "{")])
		if utf8.RuneCountInString(lead) >= 3 {
			p.lead = lower3(lead)
		}
		pats = append(pats, p)
	}
	// os mais específicos (mais texto fixo) primeiro
	sort.SliceStable(pats, func(i, j int) bool { return fixedLen(pats[i]) > fixedLen(pats[j]) })
	for _, p := range pats {
		if p.lead == "" {
			d.any = append(d.any, p)
		} else {
			d.byLead[p.lead] = append(d.byLead[p.lead], p)
		}
	}
	en = d
}

func fixedLen(p *pattern) int {
	n := 0
	for _, x := range p.pieces {
		n += len(x)
	}
	return n
}

func lower3(s string) string {
	r := []rune(strings.ToLower(s))
	if len(r) > 3 {
		r = r[:3]
	}
	return string(r)
}

// Lang normaliza a escolha das Configurações: "en" ou "pt" ("auto" e
// vazio usam o idioma do sistema, que o app passa em sys).
func Lang(choice, sys string) string {
	switch choice {
	case "en", "pt":
		return choice
	}
	if strings.HasPrefix(strings.ToLower(sys), "pt") || sys == "" {
		return "pt"
	}
	return "en"
}

// T traduz um texto pronto para o idioma lang ("pt" devolve igual).
func T(lang, s string) string {
	if lang != "en" {
		return s
	}
	if !hasWord(s) {
		return FixNums(s)
	}
	once.Do(load)
	return en.tr(s)
}

func hasWord(s string) bool {
	n := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			n++
			if n >= 2 {
				return true
			}
		} else {
			n = 0
		}
	}
	return false
}

func (d *dict) tr(s string) string {
	d.mu.Lock()
	if c, ok := d.cache[s]; ok {
		d.mu.Unlock()
		return c
	}
	d.mu.Unlock()
	core := strings.Join(strings.Fields(s), " ")
	out, ok := d.one(core, false)
	if !ok {
		// molde que começa variável, mas cobre boa parte do texto: vale antes
		// de quebrar em frases ou em pedaços
		out, ok = d.wide(core)
	}
	if !ok && strings.HasPrefix(core, "• ") {
		if t := d.tr(core[len("• "):]); t != core[len("• "):] {
			out, ok = "• "+t, true
		}
	}
	if !ok {
		out, ok = d.sentences(core)
	}
	if !ok && strings.Contains(core, " · ") {
		segs := strings.Split(core, " · ")
		changed := false
		for i, g := range segs {
			if i > 0 {
				// o dicionário guarda alguns pedaços com o "· " na frente
				if t, ok2 := d.one("· "+g, false); ok2 {
					segs[i], changed = strings.TrimPrefix(t, "· "), true
					continue
				}
			}
			// primeiro os moldes que começam com texto fixo; os genéricos por último
			t, ok2 := d.one(g, false)
			if !ok2 {
				t, ok2 = d.wide(g)
			}
			if !ok2 {
				t, ok2 = d.sentences(g)
			}
			if !ok2 {
				t, ok2 = d.one(g, true)
			}
			if ok2 {
				segs[i], changed = t, true
			} else {
				segs[i] = FixNums(g)
			}
		}
		if changed {
			out, ok = strings.Join(segs, " · "), true
		}
	}
	if !ok {
		out, ok = d.one(core, true)
	}
	res := s
	if ok {
		lead := s[:len(s)-len(strings.TrimLeft(s, " \t\n"))]
		tail := s[len(strings.TrimRight(s, " \t\n")):]
		res = lead + out + tail
	} else {
		res = FixNums(s)
	}
	d.mu.Lock()
	if len(d.cache) > 5000 {
		d.cache = map[string]string{}
	}
	d.cache[s] = res
	d.mu.Unlock()
	return res
}

// wide tenta os moldes que começam com parte variável, aceitando só os que
// têm texto fixo em pelo menos 40% da frase.
func (d *dict) wide(s string) (string, bool) {
	if len(s) > 300 {
		return "", false
	}
next:
	for _, p := range d.any {
		if fixedLen(p)*10 < len(s)*4 {
			continue
		}
		for _, x := range p.pieces {
			if !strings.Contains(s, x) {
				continue next
			}
		}
		if p.re.MatchString(s) {
			return d.apply(p, s), true
		}
	}
	return "", false
}

func (d *dict) apply(p *pattern, s string) string {
	m := p.re.FindStringSubmatch(s)
	return ph.ReplaceAllStringFunc(p.out, func(tok string) string {
		n := 0
		for _, c := range tok[1 : len(tok)-1] {
			n = n*10 + int(c-'0')
		}
		for i, o := range p.order {
			if o == n {
				return d.part(m[i+1])
			}
		}
		return tok
	})
}

// one traduz uma frase: exata, por molde ou com a primeira letra trocada.
func (d *dict) one(s string, generic bool) (string, bool) {
	if e, ok := d.exact[s]; ok {
		return e, true
	}
	lists := [][]*pattern{d.byLead[lower3(s)]}
	if generic && len(s) <= 300 {
		lists = append(lists, d.any)
	}
	for _, list := range lists {
	next:
		for _, p := range list {
			for _, x := range p.pieces {
				if !strings.Contains(s, x) {
					continue next
				}
			}
			m := p.re.FindStringSubmatch(s)
			if m == nil {
				continue
			}
			return ph.ReplaceAllStringFunc(p.out, func(tok string) string {
				n := 0
				for _, c := range tok[1 : len(tok)-1] {
					n = n*10 + int(c-'0')
				}
				for i, o := range p.order {
					if o == n {
						return d.part(m[i+1])
					}
				}
				return tok
			}), true
		}
	}
	// "Frase com maiúscula." ↔ "frase com maiúscula" do dicionário
	if r, size := utf8.DecodeRuneInString(s); size > 0 {
		bare := strings.TrimSuffix(string(unicode.ToLower(r))+s[size:], ".")
		if bare != s {
			if e, ok := d.exact[bare]; ok {
				er, es := utf8.DecodeRuneInString(e)
				e = string(unicode.ToUpper(er)) + e[es:]
				if strings.HasSuffix(s, ".") && !strings.HasSuffix(e, ".") {
					e += "."
				}
				return e, true
			}
		}
	}
	return s, false
}

func (d *dict) part(s string) string {
	core := strings.TrimSpace(s)
	if core == "" {
		return s
	}
	i := strings.Index(s, core)
	t, ok := d.one(core, true)
	if !ok {
		t = FixNums(core)
	}
	return s[:i] + t + s[i+len(core):]
}

// sentences: várias frases no mesmo texto; junta as que formam uma frase
// conhecida (a mais longa primeiro) e traduz o resto uma a uma.
func (d *dict) sentences(core string) (string, bool) {
	var bits []string
	start := 0
	for i := 0; i < len(core)-1; i++ {
		if strings.ContainsRune(".!?;", rune(core[i])) && core[i+1] == ' ' {
			bits = append(bits, core[start:i+1])
			start = i + 2
		}
	}
	bits = append(bits, core[start:])
	if len(bits) < 2 {
		return "", false
	}
	var out []string
	changed := false
	for i := 0; i < len(bits); {
		done := false
		for j := len(bits); j > i+1; j-- {
			if t, ok := d.one(strings.Join(bits[i:j], " "), false); ok {
				out, i, done, changed = append(out, t), j, true, true
				break
			}
		}
		if done {
			continue
		}
		t, ok := d.one(bits[i], true)
		if ok {
			changed = true
		} else {
			t = FixNums(bits[i])
		}
		out = append(out, t)
		i++
	}
	return strings.Join(out, " "), changed
}

// FixNums troca a vírgula decimal por ponto ("52,3" → "52.3"). Os números
// do Go não têm separador de milhar, então toda vírgula entre dígitos é
// decimal; as de "1,024" das traduções não passam por aqui.
func FixNums(s string) string {
	b := []byte(s)
	for i := 1; i+1 < len(b); i++ {
		if b[i] == ',' && isDigit(b[i-1]) && isDigit(b[i+1]) {
			b[i] = '.'
		}
	}
	return string(b)
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
