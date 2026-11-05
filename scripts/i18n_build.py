#!/usr/bin/env python3
"""Gera frontend/dist/i18n-en.js a partir de i18n/en.tsv.

i18n/en.tsv: uma linha por frase, "português<TAB>inglês". Frases com {0},
{1}… são moldes: as partes variáveis são capturadas e reaparecem no inglês
na posição do mesmo número.
"""
import json, os, re

ROOT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..")
exact, pats = {}, []
for ln in open(os.path.join(ROOT, "i18n/en.tsv"), encoding="utf-8"):
    ln = ln.rstrip("\n")
    if not ln or ln.startswith("#") or "\t" not in ln:
        continue
    pt, en = ln.split("\t", 1)
    if re.search(r"\{\d+\}", pt):
        parts = re.split(r"(\{\d+\})", pt)
        rx, order = "", []
        for p in parts:
            m = re.fullmatch(r"\{(\d+)\}", p)
            if m:
                # a parte variável não atravessa " · " (o app junta pedaços assim)
                rx += "((?:(?! · ).)+?)"; order.append(int(m.group(1)))
            else:
                rx += re.escape(p).replace("\\ ", " ")
        # o inglês usa {n} do português; vira $k na ordem das capturas
        out = re.sub(r"\{(\d+)\}", lambda m: "${%d}" % (order.index(int(m.group(1))) + 1), en)
        lit = re.match(r"[^{]*", pt).group(0)
        # pedaços fixos: a tela só roda a expressão se todos aparecerem no texto
        pieces = [x for x in re.split(r"\{\d+\}", pt) if len(x.strip()) >= 2]
        pats.append([lit, "^" + rx + "$", out, pieces])
    else:
        exact[pt] = en
# moldes com mais texto fixo primeiro (os mais específicos ganham)
pats.sort(key=lambda p: -len(re.sub(r"\(\(\?:\(\?! · \)\.\)\+\?\)", "", p[1])))
js = "// gerado por scripts/i18n_build.py a partir de i18n/en.tsv: não edite à mão\n"
js += "const I18N_EN = " + json.dumps({"exact": exact, "patterns": pats}, ensure_ascii=False, indent=0) + ";\n"
open(os.path.join(ROOT, "frontend/dist/i18n-en.js"), "w", encoding="utf-8").write(js)
print(f"{len(exact)} frases, {len(pats)} moldes")
