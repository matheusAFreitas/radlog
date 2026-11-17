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
  // nome escrito pela pessoa (asIs em util.js): fica como está
  const named = (c) => c.charCodeAt(0) === 0x2068 && c.charCodeAt(c.length - 1) === 0x2069 && c.indexOf("\u2069") === c.length - 1;
  const part = (s) => {
    const core = s && s.trim();
    if (!core || named(core)) return s;
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
  const done = new Set(); // traduções já entregues
  return function tr(s) {
    if (s == null || typeof s !== "string" || !hasWord.test(s)) return s;
    const c = cache.get(s);
    if (c !== undefined) return c;
    const core = s.trim().replace(/\s+/g, " ");
    // 2.101.0: texto que já é uma tradução não passa de novo (o molde
    // genérico "{0} no {1}" virava "no limits" em "on the limits")
    if (done.has(core)) return s;
    if (named(core)) return s;
    let out = one(core, false, false);
    if (out == null) out = wide(core);
    if (out == null) out = sentences(core);
    if (out == null && core.includes(" · ")) {
      // pedaços juntados com " · ": traduz cada um (com e sem o "· ")
      const segs = core.split(/ (?=· )/);
      const ts = segs.map((g) => {
        const dot = g.startsWith("· ");
        const r = dot ? g.slice(2) : g;
        // primeiro os moldes que começam com texto fixo; os genéricos por último
        const t = (dot && one(g, false, false)) || one(r, false, false) || wide(r) || sentences(r) || one(r, false, true);
        if (t == null) return g;
        return dot && !t.startsWith("· ") ? "· " + t : t;
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
    if (cache.size > 5000) { cache.clear(); done.clear(); }
    cache.set(s, res);
    if (out != null) done.add(out);
    return res;
  };
})();

// traduz o que já está na tela e o que mudar depois
// 2.84.0: o mesmo observador troca µSv por mrem nos textos com número
// quando a taxa da tela é mrem/h (unidades.js); [data-unit-keep] fica como está
(() => {
  const translate = LANG === "en" && typeof I18N_EN !== "undefined";
  const rem = typeof RATE_MREM !== "undefined" && RATE_MREM;
  const words = typeof UNIT_WORDS !== "undefined" && !!UNIT_WORDS;
  const iso = typeof DATE_ISO !== "undefined" && DATE_ISO;
  if (!translate && !rem && !words && !iso) return;
  const ATTRS = ["title", "placeholder", "aria-label"];
  const skip = (n) => { const p = n.parentNode; return p && (p.nodeName === "SCRIPT" || p.nodeName === "STYLE" || (p.closest && p.closest("[data-no-tr]"))); };
  const keep = (el) => !rem || (el && el.closest && el.closest("[data-unit-keep]"));
  const conv = (s, el) => { const t = dateText(translate ? tr(s) : s); return keep(el) ? unitWord(t, el) : unitWord(remText(t), el); };
  const doText = (n) => {
    if (skip(n)) return;
    const t = conv(n.data, n.parentNode);
    if (t !== n.data) n.data = t;
  };
  const doAttrs = (el) => {
    for (const a of ATTRS) {
      const v = el.getAttribute && el.getAttribute(a);
      if (v) { const t = conv(v, el); if (t !== v) el.setAttribute(a, t); }
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
