"use strict";

// ---------- saúde do tubo do GMC-800 (2.57) ----------
//
// Cartão na tela do GQ GMC-800 com as quatro conferências (GetTubeHealth,
// tubehealth.go), aviso informativo no topo de Alertas enquanto alguma está
// em "vale olhar" (some sozinho; não vai para o histórico) e uma linha em
// Configurações › GQ GMC-800. Os estados não usam os níveis de radiação nem
// as formas deles. Spec: ux/tubo.html.
//
// Usa os globais do app.js (api, state, $, fmt, call). Carregado antes do
// app.js: só define funções.

const TB = { h: null, busy: false };
const TB_TAG = { ok: "tudo certo", warn: "vale olhar", na: "sem dado" };
const TB_ICO = {
  ok: '<svg class="tb-ico ok" viewBox="0 0 20 20" aria-hidden="true"><circle cx="10" cy="10" r="9"/><path d="M5.6 10.3 8.6 13.2 14.4 7"/></svg>',
  warn: '<svg class="tb-ico warn" viewBox="0 0 20 20" aria-hidden="true"><circle cx="10" cy="10" r="9"/><path d="M10 5.2v6"/><circle class="dot" cx="10" cy="14.6" r="1.3"/></svg>',
  na: '<svg class="tb-ico na" viewBox="0 0 20 20" aria-hidden="true"><circle cx="10" cy="10" r="7.5"/></svg>',
};

const tbEsc = (s) => String(s).replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c]);
const tbN = (x, d) => fmt(x, d);
// lista "a, b e c"; em inglês já sai traduzida ("a, b and c"), porque o
// tradutor da tela não conhece cada combinação
function tbList(a) {
  const en = typeof LANG !== "undefined" && LANG === "en" && typeof tr === "function";
  const b = en ? a.map((x) => tr(x)) : a;
  return b.length < 2 ? b.join("") : b.slice(0, -1).join(", ") + (en ? " and " : " e ") + b[b.length - 1];
}
function tbWhen(ms) {
  const d = new Date(ms), p = (n) => String(n).padStart(2, "0");
  return `${p(d.getDate())}/${p(d.getMonth() + 1)} ${p(d.getHours())}:${p(d.getMinutes())}`;
}
function tbDur(min) {
  return min < 60 ? `${min} min` : `${Math.floor(min / 60)} h ${String(min % 60).padStart(2, "0")}`;
}
function tbChecked(ms) {
  const d = new Date(ms), now = new Date(), p = (n) => String(n).padStart(2, "0");
  const hm = `${p(d.getHours())}:${p(d.getMinutes())}`;
  if (d.toDateString() === now.toDateString()) return `hoje às ${hm}`;
  const y = new Date(now); y.setDate(y.getDate() - 1);
  if (d.toDateString() === y.toDateString()) return `ontem às ${hm}`;
  return `${tbWhen(ms)}`;
}

// as quatro conferências: [estado, título, evidência (HTML), gráfico, como funciona]
function tbChecks(h) {
  const c = [];
  // 1 falhas
  if (h.gapsState === "na") c.push(["na", "Conta sem falhar", "Sem dado: nenhuma leitura do GMC-800 nos últimos 30 dias.", "", TB_HOW.gaps]);
  else if (!h.gaps.length) c.push(["ok", "Conta sem falhar", `Nenhum período sem contagens nos últimos <b>${h.gapDays < 30 ? h.gapDays + " dias com leitura" : "30 dias"}</b>.`, "", TB_HOW.gaps]);
  else c.push(["warn", "Conta sem falhar", `Parou de contar <b>${h.gaps.length === 1 ? "1 vez" : h.gaps.length + " vezes"}</b>: ${h.gaps.slice(-4).map((g) => `${tbWhen(g.at)} (${tbDur(g.min)})`).join(", ")}${h.gaps.length > 4 ? "…" : ""}. Confira o cabo USB e se o GMC-800 desligou sozinho (bateria).`, "", TB_HOW.gapsWarn]);
  // 2 dispersão
  if (h.dispState === "na") c.push(["na", "Varia só pelo acaso", "Sem dado: ainda não há um dia inteiro com o GMC-800 em casa, sem chuva e sem medição.", "", TB_HOW.disp]);
  else {
    const ok = h.dispState === "ok";
    c.push([h.dispState, "Varia só pelo acaso", ok ? `Dispersão <b>${tbN(h.disp, 2)}</b>: os seus minutos seguem a curva do acaso.`
      : `Dispersão <b>${tbN(h.disp, 2)}</b>, acima de 1,2 em ${h.dispHigh} dos últimos ${h.dispLast} dias: a contagem varia mais do que o acaso explica, e não foi chuva nem objeto. Pode ser ruído no tubo ou interferência elétrica.`, tbHist(h), TB_HOW.disp]);
  }
  // 3 rajadas
  const nb = h.bursts.length;
  if (h.burstState === "na") c.push(["na", "Sem rajadas estranhas", "Sem dado: nenhuma leitura do GMC-800 nos últimos 30 dias.", "", TB_HOW.burst]);
  else if (h.burstState === "ok") c.push(["ok", "Sem rajadas estranhas", nb === 0 ? "Nenhuma rajada suspeita em <b>30 dias</b>." : `<b>${nb}</b> ${nb === 1 ? "rajada suspeita" : "rajadas suspeitas"} em 30 dias (${h.bursts.map(tbWhen).join(", ")}). Normal.`, "", TB_HOW.burst]);
  else c.push(["warn", "Sem rajadas estranhas", `<b>${nb}</b> rajadas suspeitas em 30 dias${h.burstHour >= 0 ? `, a maioria perto das ${h.burstHour}h. Horário que se repete tem cara de interferência elétrica: veja o que liga nessa hora perto do GMC-800.` : `. A última foi ${tbWhen(h.bursts[nb - 1])}. Veja se algum aparelho elétrico ficou perto do GMC-800.`}`, "", TB_HOW.burst]);
  // 4 razão
  if (h.ratioState === "na") {
    c.push(["na", "Sensibilidade estável", h.ratioWhy === "poucos" ? "Sem dado ainda: os dois aparelhos precisam ficar juntos em casa por pelo menos duas semanas (uma no começo e outra no fim dos 90 dias)."
      : "Sem dado: o RC102 não ficou em casa junto com o GMC-800 nos últimos 30 dias.", "", TB_HOW.ratio]);
  } else {
    const ch = h.ratioChg * 100, sg = ch >= 0 ? "+" : "−", base = `Razão RC102 ÷ GMC-800: <b>${tbN(h.r0, 1)}</b> → <b>${tbN(h.r1, 1)}</b> em 90 dias (${sg}${tbN(Math.abs(ch), 1)}%).`;
    c.push([h.ratioState, "Sensibilidade estável", h.ratioState === "ok" ? `${base} O tubo conta igual.`
      : `${base} ${ch > 0 ? "O GMC-800 está contando menos que antes." : "O GMC-800 está contando mais que antes."} Vale refazer a intercalibração.`, tbRatio(h), TB_HOW.ratio]);
  }
  return [c[1], c[3], c[0], c[2]]; // os dois com gráfico em cima
}

const TB_HOW = {
  gaps: "O aviso \"sem contagens\" dispara quando o tubo passa 2 minutos sem nenhum pulso. Com o fundo de casa ele dá umas 20 contagens por minuto, então 2 minutos zerados quase nunca acontecem por acaso.",
  gapsWarn: "Um tubo saudável nunca fica minutos sem pulso com o fundo de casa. Se parou e voltou sozinho, costuma ser cabo, bateria ou o aparelho travado, mais que o tubo.",
  disp: "Se o tubo só conta radiação, o número de pulsos em cada minuto segue uma curva conhecida, a de Poisson: a largura dela é a raiz da média (√20 ≈ 4,5). A dispersão é a largura real dividida por essa. Perto de 1, é só o acaso. Bem acima de 1, tem alguma coisa a mais mexendo na contagem.",
  burst: "Rajada é um segundo com muito mais pulso do que a média permitiria, seguido de volta ao normal. Radiação de verdade não faz isso. Motor, fonte de computador e carregador perto do tubo fazem.",
  ratio: "Em casa, os dois aparelhos veem a mesma radiação. Se o RC102 conta umas 28 vezes mais que o GMC-800 hoje, deveria continuar assim. Se a razão muda e o RC102 está firme, foi o GMC-800 que mudou.",
};

function tbHist(h) {
  const b = h.hist || [];
  if (b.length < 3) return "";
  const W = 320, H = 96, mx = Math.max(...b.map((x) => Math.max(x.n, x.ref)), 1), bw = W / b.length;
  let s = "";
  b.forEach((x, i) => {
    const hh = x.n / mx * (H - 14);
    s += `<rect data-k="${x.k}" data-v="${x.n}" data-r="${Math.round(x.ref)}" x="${(i * bw + 1).toFixed(1)}" y="${(H - 2 - hh).toFixed(1)}" width="${Math.max(0.5, bw - 2).toFixed(1)}" height="${hh.toFixed(1)}" rx="2"/>`;
  });
  const pts = b.map((x, i) => `${((i + 0.5) * bw).toFixed(1)},${(H - 2 - x.ref / mx * (H - 14)).toFixed(1)}`).join(" ");
  s += `<polyline points="${pts}"/>`;
  return `<svg class="tb-viz tb-hist" viewBox="0 0 ${W} ${H}" preserveAspectRatio="none" role="img" aria-label="Histograma das contagens por minuto contra a curva do acaso">${s}</svg>` +
    `<div class="tb-ax mono"><span>${b[0].k}</span><span>contagens em 1 minuto</span><span>${b[b.length - 1].k}</span></div>` +
    `<div class="tb-leg small"><span><i class="sw sw-bar"></i>seus minutos (30 dias)</span><span><i class="sw sw-dash"></i>se fosse só o acaso (Poisson)</span></div>`;
}

function tbRatio(h) {
  const a = (h.ratio || []).map((p) => p.r);
  if (a.length < 2) return "";
  const W = 320, H = 80, base = h.r0;
  const lo = Math.min(base * 0.93, ...a), hi = Math.max(base * 1.07, ...a);
  const t0 = h.ratio[0].day, t1 = h.ratio[a.length - 1].day;
  const x = (i) => (h.ratio[i].day - t0) / Math.max(1, t1 - t0) * W, y = (v) => H - 4 - (v - lo) / (hi - lo) * (H - 10);
  let s = `<rect class="tb-band" x="0" y="${y(base * 1.05).toFixed(1)}" width="${W}" height="${(y(base * 0.95) - y(base * 1.05)).toFixed(1)}"/>`;
  s += `<line class="tb-base" x1="0" x2="${W}" y1="${y(base).toFixed(1)}" y2="${y(base).toFixed(1)}"/>`;
  s += `<polyline points="${a.map((v, i) => `${x(i).toFixed(1)},${y(v).toFixed(1)}`).join(" ")}"/>`;
  s += `<circle cx="${x(a.length - 1).toFixed(1)}" cy="${y(a[a.length - 1]).toFixed(1)}" r="3.5"/>`;
  const days = Math.round((Date.now() - t0) / 864e5);
  return `<svg class="tb-viz tb-ratio" viewBox="0 0 ${W} ${H}" preserveAspectRatio="none" role="img" aria-label="Razão de contagens RC102 por GMC-800 em 90 dias">${s}</svg>` +
    `<div class="tb-ax mono"><span>${days} dias atrás</span><span>hoje</span></div>` +
    `<div class="tb-leg small"><span><i class="sw sw-band"></i>faixa de ±5%</span></div>`;
}

function tbRender() {
  const h = TB.h, card = $("tubeCard");
  if (!h) return;
  const c = tbChecks(h), warns = c.filter((x) => x[0] === "warn"), oks = c.filter((x) => x[0] === "ok").length, nas = c.filter((x) => x[0] === "na").length;
  const when = `conferido ${tbChecked(h.at)}`;
  if (card) {
    let s = `<div class="tb-head"><h2>Saúde do tubo</h2><span class="small muted">GQ GMC-800 · ${when}</span></div>`;
    if (warns.length) s += `<div class="tb-sum">${TB_ICO.warn}<div><b>Vale olhar: <span data-no-tr>${tbList(warns.map((w) => w[1].toLowerCase()))}</span></b><p class="small muted">O resto está normal. Abaixo tem o que foi visto e o que conferir.</p></div></div>`;
    else if (!oks) s += `<div class="tb-sum">${TB_ICO.na}<div><b>Ainda sem dado para conferir</b><p class="small muted">O cartão começa a conferir quando o GMC-800 tiver leituras gravadas.</p></div></div>`;
    else s += `<div class="tb-sum">${TB_ICO.ok}<div><b>O tubo está contando direito</b><p class="small muted">${oks === 1 ? "A conferência com dado passou." : `As ${oks} conferências com dado passaram.`}${nas ? (nas === 1 ? " Uma ficou sem dado." : ` ${nas} ficaram sem dado.`) : ""}</p></div></div>`;
    s += `<div class="tb-checks">${c.map((x) => `<div class="tb-ck ${x[0]}"><div class="tb-top">${TB_ICO[x[0]]}<span class="tb-tt">${x[1]}</span><span class="tb-st mono">${TB_TAG[x[0]]}</span></div><p class="tb-ev small">${x[2]}</p>${x[3] || ""}<details class="how"><summary>Como funciona</summary><p class="small muted">${tbEsc(x[4])}</p></details></div>`).join("")}</div>`;
    s += `<div class="tb-foot"><button type="button" class="btn small" id="btnTubeCheck"${TB.busy ? " disabled" : ""}>Conferir agora</button><span class="small muted">Usa só o que o app já gravou. Nada é escrito no aparelho.</span></div>`;
    card.innerHTML = s;
    card.hidden = false;
  }
  // aviso informativo em Alertas: só enquanto durar
  const n = $("tubeNotice");
  if (n) {
    n.hidden = !warns.length;
    if (warns.length) {
      n.innerHTML = `<strong>Saúde do tubo.</strong> Vale olhar: <span data-no-tr></span>. <span>Não é alarme: é sobre o aparelho, não sobre a radiação.</span> <button type="button" class="link small-link" data-tubeopen="1">Ver no GQ GMC-800</button>`;
      n.querySelector("[data-no-tr]").textContent = tbList(warns.map((w) => w[1].toLowerCase()));
    }
  }
  // linha em Configurações
  const l = $("tubeLine");
  if (l) l.textContent = `Saúde do tubo: ${warns.length ? "vale olhar" : oks ? "tudo certo" : "sem dado"} · ${when}`;
}

async function tbLoad(force) {
  if (!api.GetTubeHealth || TB.busy) return;
  TB.busy = !!force;
  if (force) tbRender();
  try { TB.h = await api.GetTubeHealth(!!force); } catch (e) { /* fica o anterior */ }
  TB.busy = false;
  tbRender();
}

function tubeInit() {
  if (!api.GetTubeHealth) return;
  document.addEventListener("click", (e) => {
    if (e.target.closest("#btnTubeCheck")) tbLoad(true);
    else if (e.target.closest("[data-tubeopen]")) { showView("gmc"); setTimeout(() => { const c = $("tubeCard"); if (c) c.scrollIntoView({ block: "start" }); }, 50); }
  });
  const hs = (e) => {
    const r = e.target.closest && e.target.closest(".tb-hist rect"), tip = $("tubeTip");
    if (!tip) return;
    if (!r) { tip.hidden = true; return; }
    tip.hidden = false;
    tip.textContent = `${r.dataset.k} contagens: ${r.dataset.v} minutos (acaso: ${r.dataset.r})`;
    tip.style.left = e.clientX + 12 + "px";
    tip.style.top = e.clientY + 12 + "px";
  };
  document.addEventListener("mousemove", hs);
  tbLoad(false);
  // o Go só recalcula depois das 06:00; aqui só pergunta de vez em quando
  setInterval(() => tbLoad(false), 10 * 60000);
}
document.addEventListener("DOMContentLoaded", tubeInit);
