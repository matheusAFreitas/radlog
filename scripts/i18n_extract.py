#!/usr/bin/env python3
"""Lista os textos em português da interface para traduzir.

Saída (stdout), um por linha, sem repetir:
  E<TAB>texto            frase exata (HTML, strings fixas do JS e do Go)
  P<TAB>molde            frase com partes variáveis: {0}, {1}… no lugar delas
Uso: python3 scripts/i18n_extract.py > /tmp/catalogo.tsv
"""
import glob, os, re, sys
from html.parser import HTMLParser

ROOT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..")
PT = re.compile(r"[ãçõéêáíóúâôà]|\b(de|do|da|não|com|para|sem|um|uma|em|no|na|os|as|ao|à|que|por|mais|menos|hoje|ontem|dias?|minutos?|horas?|fundo|contador|medição|alerta|alarme|atenção|perigo|leitura|salvar|abrir|fechar|voltar|baixar|gráfico|configura\w*|porta|conectad\w*|desconectad\w*)\b", re.I)
WORD = re.compile(r"[A-Za-zÀ-ú]{2}")
exact, pats = set(), set()

def norm(s):
    return " ".join(s.split())

def add_exact(s):
    s = norm(s)
    if s and WORD.search(s) and PT.search(s) and "<" not in s and not s.startswith(("http", "#", ".", "--")):
        exact.add(s)

# ---- HTML
class P(HTMLParser):
    def __init__(self):
        super().__init__(); self.skip = 0
    def handle_starttag(self, t, at):
        if t in ("script", "style"): self.skip += 1
        for k, v in at:
            if k in ("title", "placeholder", "aria-label") and v: add_exact(v)
    def handle_endtag(self, t):
        if t in ("script", "style"): self.skip -= 1
    def handle_data(self, d):
        if not self.skip: add_exact(d)
P().feed(open(os.path.join(ROOT, "frontend/dist/index.html"), encoding="utf-8").read())

# ---- JS: strings fixas e moldes `...${x}...`
def js_unescape(s):
    return s.replace('\\"', '"').replace("\\'", "'").replace("\\n", "\n").replace("\\\\", "\\")
for f in glob.glob(os.path.join(ROOT, "frontend/dist/*.js")):
    if f.endswith("i18n-en.js"): continue
    src = open(f, encoding="utf-8").read()
    src = re.sub(r"^\s*//.*$", "", src, flags=re.M)
    for q in ('"', "'"):
        for m in re.finditer(q + r"((?:[^" + q + r"\\\n]|\\.)*)" + q, src):
            s = js_unescape(m.group(1))
            if "<" in s:  # pedaço de HTML: pega o texto entre as tags
                for part in re.split(r"<[^>]*>", s): add_exact(part)
            else:
                add_exact(s)
    for m in re.finditer(r"`((?:[^`\\]|\\.)*)`", src, flags=re.S):
        body = m.group(1)
        for chunk in re.split(r"<[^>]*>", body):
            if "${" not in chunk:
                add_exact(chunk); continue
            n = [0]
            def rep(_):
                n[0] += 1; return "{%d}" % (n[0] - 1)
            # tira ${...} (com chaves aninhadas simples)
            out, i = "", 0
            while i < len(chunk):
                if chunk.startswith("${", i):
                    depth, j = 1, i + 2
                    while j < len(chunk) and depth:
                        depth += {"{": 1, "}": -1}.get(chunk[j], 0); j += 1
                    out += "{%d}" % n[0]; n[0] += 1; i = j
                else:
                    out += chunk[i]; i += 1
            out = norm(out)
            lit = re.sub(r"\{\d+\}", "", out)
            if WORD.search(lit) and PT.search(lit):
                pats.add(out)

# ---- Go: strings que podem chegar na tela (eventos, mensagens, conclusões)
for f in glob.glob(os.path.join(ROOT, "internal/*/*.go")):
    if f.endswith("_test.go"): continue
    src = open(f, encoding="utf-8").read()
    src = re.sub(r"^\s*//.*$", "", src, flags=re.M)
    for m in re.finditer(r'"((?:[^"\\\n]|\\.)*)"', src):
        s = m.group(1).replace('\\"', '"').replace("\\n", " ")
        if re.search(r"%[-+ #0-9.]*[vdsfqxg]", s):
            n = [0]
            def rep(_):
                n[0] += 1; return "{%d}" % (n[0] - 1)
            out = norm(re.sub(r"%[-+ #0-9.]*[vdsfqxg]", rep, s.replace("%%", "%")))
            lit = re.sub(r"\{\d+\}", "", out)
            if WORD.search(lit) and PT.search(lit): pats.add(out)
        else:
            add_exact(s)

for s in sorted(exact): print("E\t" + s)
for s in sorted(pats): print("P\t" + s)
