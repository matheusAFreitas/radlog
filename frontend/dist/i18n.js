"use strict";

// ---------- idioma da interface ----------
//
// O app é escrito em português. Em inglês, esta camada troca os textos na
// tela: os fixos do HTML, os que o app.js escreve e os que vêm do Go
// (eventos, alertas, conclusões). Frases inteiras saem de I18N_EN.exact;
// frases com partes variáveis ("Alarme: 112 CPM (limite 100)") saem dos
// moldes de I18N_EN.patterns, que capturam as partes e as recolocam no
// inglês (números com vírgula viram ponto). Um MutationObserver traduz
// tudo o que muda na tela depois.
//
// Para editar a tradução: internal/i18n/en.tsv e depois scripts/i18n_build.py.

const LANG_PREF = (() => { try { return localStorage.getItem("lang") || "auto"; } catch (e) { return "auto"; } })();
const LANG = LANG_PREF === "auto" ? ((navigator.language || "pt").toLowerCase().startsWith("pt") ? "pt" : "en") : LANG_PREF;
const LOCALE = LANG === "en" ? "en-US" : "pt-BR";
document.documentElement.lang = LANG === "en" ? "en" : "pt-BR";

const tr = (() => {
  const dict = LANG === "en" && typeof I18N_EN !== "undefined" ? I18N_EN : null;
  if (!dict) return (s) => s;
  const exact = new Map(Object.entries(dict.exact));
  // moldes agrupados pelo texto fixo do começo: só testa os que podem bater
  const byLead = new Map();
  const anyLead = [];
  for (const [lead, src, out, pieces] of dict.patterns) {
    const p = { re: new RegExp(src), out, pieces: pieces || [], fixed: (pieces || []).join("").length };
    const k = lead.trim().slice(0, 3).toLowerCase();
    if (k.length < 3) anyLead.push(p);
    else (byLead.get(k) || byLead.set(k, []).get(k)).push(p);
  }
  const cache = new Map();
  const hasWord = /[A-Za-zÀ-ú]{2}/;
  // números do Go vêm com vírgula decimal ("52,3"); os do app já saem no
  // formato do idioma ("1,239" = mil duzentos…). Só troca a vírgula quando é
  // decimal com certeza: 1–2 ou 4+ casas, ou "0,xxx".
  const fixNums = (s) => s.replace(/(?<![\d.,])(\d+),(\d+)(?![\d,])/g, (x, i, d) =>
    d.length === 3 && i !== "0" && i.length <= 3 ? x : i + "." + d);
  const part = (s) => {
    const core = s && s.trim();
    if (!core) return s;
    const a = s.slice(0, s.indexOf(core)), z = s.slice(s.indexOf(core) + core.length);
    const t = one(core, true);
    return a + (t !== core ? t : fixNums(core)) + z;
  };
  // moldes que começam variáveis mas cobrem boa parte do texto (40%+ fixo):
  // valem antes de quebrar em frases ou pedaços
  function wide(s) {
    if (s.length > 300) return null;
    for (const p of anyLead) {
      if (p.fixed * 10 < s.length * 4) continue;
      if (!p.pieces.every((x) => s.includes(x))) continue;
      const m = p.re.exec(s);
      if (m) return p.out.replace(/\$\{(\d+)\}/g, (_, i) => part(m[+i] || ""));
    }
    return null;
  }

  // várias frases no mesmo texto: junta as que formam uma frase conhecida
  // (a mais longa primeiro) e traduz o resto uma a uma
  function sentences(core) {
    const bits = core.split(/(?<=[.!?;])\s+/);
    if (bits.length < 2) return null;
    const out = [];
    let changed = false;
    for (let i = 0; i < bits.length; ) {
      let done = false;
      for (let j = bits.length; j > i + 1; j--) {
        const cand = bits.slice(i, j).join(" ");
        const t = one(cand, false, false);
        if (t != null) { out.push(t); i = j; done = changed = true; break; }
      }
      if (done) continue;
      const t = one(bits[i], true);
      if (t !== bits[i]) changed = true;
      out.push(t);
      i++;
    }
    return changed ? out.join(" ") : null;
  }

  // generic = moldes que começam com uma parte variável ("{0} às {1}"): só
  // depois de tentar frase por frase, porque eles batem com quase tudo
  function one(s, inner, generic = true) {
    const e = exact.get(s);
    if (e != null) return e;
    const k = s.slice(0, 3).toLowerCase();
    const lists = generic ? [byLead.get(k) || [], anyLead] : [byLead.get(k) || []];
    for (const list of lists) {
      for (const p of list) {
        // os pedaços fixos precisam estar no texto (barato) antes da expressão
        if (!p.pieces.every((x) => s.includes(x))) continue;
        if (list === anyLead && s.length > 300) continue;
        const m = p.re.exec(s);
        if (m) return p.out.replace(/\$\{(\d+)\}/g, (_, i) => part(m[+i] || ""));
      }
    }
    // "Frase com maiúscula." ↔ "frase com maiúscula" do dicionário
    const low = s.charAt(0).toLowerCase() + s.slice(1);
    const bare = low.replace(/\.$/, "");
    if (bare !== s) {
      const t = exact.get(bare);
      if (t != null) return t.charAt(0).toUpperCase() + t.slice(1) + (s.endsWith(".") && !t.endsWith(".") ? "." : "");
    }
    return inner ? s : null;
  }
  return function tr(s) {
    if (s == null || typeof s !== "string" || !hasWord.test(s)) return s;
    const c = cache.get(s);
    if (c !== undefined) return c;
    const core = s.trim().replace(/\s+/g, " ");
    let out = one(core, false, false);
    if (out == null) out = wide(core);
    if (out == null) out = sentences(core);
    if (out == null && core.includes(" · ")) {
      // pedaços juntados com " · ": traduz cada um (com e sem o "· ")
      const segs = core.split(/ (?=· )/);
      const ts = segs.map((g) => {
        const t = one(g, true);
        if (t !== g || !g.startsWith("· ")) return t === g ? (sentences(g) || g) : t;
        const r = g.slice(2);
        return "· " + (one(r, false) || sentences(r) || r);
      });
      if (ts.some((g, i) => g !== segs[i])) out = ts.join(" ");
    }
    if (out == null) out = one(core, false, true);
    if (out == null) {
      // "nome (12)": contador no fim
      const m = /^(.+) \((\d+)\)$/.exec(core);
      if (m) { const t = one(m[1], false); if (t != null) out = t + " (" + m[2] + ")"; }
    }
    // sem tradução: deixa como está (espaços e quebras de linha inclusive)
    const res = out == null ? s : s.match(/^\s*/)[0] + out + s.match(/\s*$/)[0];
    if (cache.size > 5000) cache.clear();
    cache.set(s, res);
    return res;
  };
})();

// traduz o que já está na tela e o que mudar depois
(() => {
  if (LANG !== "en" || typeof I18N_EN === "undefined") return;
  const ATTRS = ["title", "placeholder", "aria-label"];
  const skip = (n) => { const p = n.parentNode; return p && (p.nodeName === "SCRIPT" || p.nodeName === "STYLE" || (p.closest && p.closest("[data-no-tr]"))); };
  const doText = (n) => {
    if (skip(n)) return;
    const t = tr(n.data);
    if (t !== n.data) n.data = t;
  };
  const doAttrs = (el) => {
    for (const a of ATTRS) {
      const v = el.getAttribute && el.getAttribute(a);
      if (v) { const t = tr(v); if (t !== v) el.setAttribute(a, t); }
    }
  };
  const walk = (root) => {
    if (root.nodeType === 3) { doText(root); return; }
    if (root.nodeType !== 1 || root.nodeName === "SCRIPT" || root.nodeName === "STYLE") return;
    doAttrs(root);
    const it = document.createTreeWalker(root, NodeFilter.SHOW_TEXT | NodeFilter.SHOW_ELEMENT);
    let n;
    while ((n = it.nextNode())) {
      if (n.nodeType === 3) doText(n);
      else doAttrs(n);
    }
  };
  document.title = tr(document.title);
  walk(document.body);
  new MutationObserver((muts) => {
    for (const m of muts) {
      if (m.type === "characterData") doText(m.target);
      else if (m.type === "attributes") doAttrs(m.target);
      else for (const n of m.addedNodes) walk(n);
    }
  }).observe(document.body, { subtree: true, childList: true, characterData: true, attributes: true, attributeFilter: ATTRS });
  // confirm() e o título da janela também
  const c = window.confirm.bind(window);
  window.confirm = (msg) => c(tr(String(msg)).split("\n").map(tr).join("\n"));
})();
