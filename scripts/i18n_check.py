#!/usr/bin/env python3
"""Falha se apareceu texto da interface sem tradução no en.tsv.

Roda o i18n_extract.py e compara com o en.tsv. O que já faltava quando a
conferência entrou fica em scripts/i18n_pendentes.txt (pedaços de frase,
textos do modo demonstração, etc.) e não reprova; um texto novo sem
tradução reprova. Ao traduzir um pendente, tire a linha dele do arquivo.

Uso: python3 scripts/i18n_check.py            (confere)
     python3 scripts/i18n_check.py --atualizar (regrava os pendentes)
"""
import os, subprocess, sys

ROOT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..")
TSV = os.path.join(ROOT, "internal/i18n/en.tsv")
PEND = os.path.join(ROOT, "scripts/i18n_pendentes.txt")

keys = set()
for l in open(TSV, encoding="utf-8"):
    if "\t" in l and not l.startswith("#"):
        keys.add(l.split("\t")[0])

out = subprocess.run([sys.executable, os.path.join(ROOT, "scripts/i18n_extract.py")],
                     capture_output=True, text=True, encoding="utf-8", check=True).stdout
# "#…" é comentário de arquivo gravado (ex.: o .ilegiveis do store), não tela
miss = sorted({l.split("\t", 1)[1] for l in out.splitlines() if "\t" in l and not l.split("\t", 1)[1].startswith("#")} - keys)

if "--atualizar" in sys.argv:
    with open(PEND, "w", encoding="utf-8") as f:
        f.write("# Textos sem tradução que já existiam (ver scripts/i18n_check.py).\n")
        f.writelines(m + "\n" for m in miss)
    print(f"{len(miss)} pendentes gravados")
    sys.exit(0)

pend = {l.rstrip("\n") for l in open(PEND, encoding="utf-8") if not l.startswith("#")}
new = [m for m in miss if m not in pend]
done = sorted(pend - set(miss))
if done:
    print(f"{len(done)} pendente(s) já traduzido(s) ou fora do app; pode tirar de {os.path.relpath(PEND, ROOT)}:")
    for m in done[:20]:
        print("  " + m)
if new:
    print(f"{len(new)} texto(s) novo(s) sem tradução em internal/i18n/en.tsv:")
    for m in new:
        print("  " + m)
    sys.exit(1)
print(f"tradução ok ({len(pend)} pendentes antigos)")
