#!/usr/bin/env python3
"""Gera android/app/src/main/assets/www/iso.js a partir do desktop.

Tira ORIGIN, EMIT, EMIT_TIP e FAM de frontend/dist/radiacode.js (o detalhe
do isótopo da 2.69) e as traduções do internal/i18n/en.tsv, para o celular
mostrar o mesmo texto. Rode de novo quando o desktop mudar.
"""
import json, os, re, sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
src = open(os.path.join(ROOT, "frontend/dist/radiacode.js"), encoding="utf-8").read()
en = {}
for ln in open(os.path.join(ROOT, "internal/i18n/en.tsv"), encoding="utf-8"):
    ln = ln.rstrip("\n")
    if "\t" in ln and not ln.startswith("#"):
        pt, tr = ln.split("\t", 1)
        en[pt] = tr


def block(name):
    m = re.search(r"const " + name + r" = (\{.*?\n?\s*\});", src, re.S)
    if not m:
        sys.exit("não achei " + name)
    return m.group(1)


def js_obj(txt):
    # objetos JS simples: chaves sem aspas viram com aspas, vírgula final sai
    t = re.sub(r"^\s*//.*$", "", txt, flags=re.M)
    t = re.sub(r'([{,]\s*)([A-Za-zαβγ][\w-]*)\s*:', r'\1"\2":', t)
    t = re.sub(r",\s*([}\]])", r"\1", t)
    return json.loads(t)


origin = js_obj(block("ORIGIN"))
emit = js_obj(block("EMIT"))
tip = js_obj(block("EMIT_TIP"))
fam = js_obj(block("FAM"))
need = set()
for v in origin.values():
    need.update(v[1:])
need.update(tip.values())
miss = [s for s in sorted(need) if s not in en]
if miss:
    sys.exit("sem tradução no en.tsv: " + "; ".join(miss))
out = "// gerado por scripts/celular_iso.py a partir do desktop (radiacode.js e en.tsv): não edite à mão\n"
out += "window.ISO = " + json.dumps({"origin": origin, "emit": emit, "tip": tip, "fam": fam}, ensure_ascii=False) + ";\n"
out += "Object.assign(window.I18N_EN = window.I18N_EN || {}, " + json.dumps({s: en[s] for s in sorted(need)}, ensure_ascii=False, indent=0) + ");\n"
open(os.path.join(ROOT, "android/app/src/main/assets/www/iso.js"), "w", encoding="utf-8").write(out)
print("ok:", len(origin), "isótopos,", len(need), "frases")
