"use strict";

// ---------- confirmação cruzada (2.59) ----------
//
// Quando um aparelho apita (atenção ou alarme), o outro confere: a linha na
// faixa (GetCrossCheck, crosscheck.go) se atualiza a cada 5 s; em Alertas,
// selo ao lado do pico de cada alerta e, no alerta aberto, o gráfico dos dois
// aparelhos em % sobre o fundo de cada um. No perigo a faixa não ganha a
// linha (só o alerta guarda). Se o outro também passou do limite, fica a
// linha "Também" de sempre. Nunca mexe no som. Spec: ux/cruzada.html.
//
// Usa os globais do app.js (api, state, $, fmt, hasGo...) e do alertas.js.
// Carregado antes do app.js: só define funções.

const CX_IC = {
  ok: '<svg viewBox="0 0 16 16" aria-hidden="true"><circle cx="8" cy="8" r="7" fill="none" stroke="currentColor" stroke-width="1.5"/><path d="M4.6 8.2 7 10.5 11.4 5.8" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/></svg>',
  no: '<svg viewBox="0 0 16 16" aria-hidden="true"><circle cx="8" cy="8" r="7" fill="none" stroke="currentColor" stroke-width="1.5"/><path d="M5 8h6" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"/></svg>',
  wait: '<svg viewBox="0 0 16 16" aria-hidden="true"><circle cx="8" cy="8" r="7" fill="none" stroke="currentColor" stroke-width="1.5" stroke-dasharray="3 2.4"/><path d="M8 4.6V8l2.2 1.6" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>',
  na: '<svg viewBox="0 0 16 16" aria-hidden="true"><circle cx="8" cy="8" r="6.5" fill="none" stroke="currentColor" stroke-width="1.4" stroke-dasharray="2.4 2.2"/></svg>',
};
const CX = { busy: false, seq: 0, pend: false };
const cxName = (dev) => (dev === "radiacode" ? "RC102" : "GMC-800");
const cxOther = (dev) => (dev === "radiacode" ? "gmc" : "radiacode");

// ---------- faixa ----------

// qual faixa está à vista e de qual aparelho: [prefixo, dev] ou null
function cxBand() {
  const ok = (k) => k === "real" || k === "rc" || (!hasGo && k === "demo");
  const al = $("alarmBar"), at = $("attnBar");
  if (al && !al.hidden && alarmCtl.sev !== "perigo" && ok(alarmCtl.kind)) return ["alarm", alarmCtl.kind === "rc" ? "radiacode" : "gmc"];
  if (at && !at.hidden && ok(attnCtl.kind)) return ["attn", attnCtl.kind === "rc" ? "radiacode" : "gmc"];
  return null;
}

function cxLine(pfx, c) {
  const el = $(pfx + "Cross");
  if (!el) return;
  // o outro também passou do limite: a linha "Também" já responde
  const other = !$(pfx + "Also").hidden || !$(pfx + "Both").hidden;
  const show = !!c && !other && !!c.text && c.why !== "sem";
  el.hidden = !show;
  const wb = $("btnWhyAttn");
  if (pfx === "attn" && wb) wb.hidden = !(show && c.state === "no");
  if (!show) return;
  el.className = "cx " + c.state;
  el.innerHTML = CX_IC[c.state] || CX_IC.na;
  const sp = document.createElement("span");
  sp.textContent = c.text;
  el.appendChild(sp);
  if (c.state === "wait" && c.frac > 0) {
    const b = document.createElement("span");
    b.className = "cx-bar";
    b.setAttribute("aria-hidden", "true");
    b.innerHTML = `<i style="width:${Math.round(Math.min(1, c.frac) * 100)}%"></i>`;
    el.appendChild(b);
  }
}

async function cxTick() {
  const bd = cxBand();
  for (const p of ["alarm", "attn"]) if (!bd || bd[0] !== p) cxLine(p, null);
  if (!bd || !api.GetCrossCheck || CX.busy) return;
  CX.busy = true;
  let c = null;
  try { c = await api.GetCrossCheck(bd[1], 0, 0, false); } catch (e) { c = null; }
  CX.busy = false;
  const now = cxBand();
  if (now && now[0] === bd[0]) cxLine(bd[0], c);
}

// ---------- Alertas: selo na lista ----------

function cxTagEl(c) {
  const t = document.createElement("span");
  t.className = "cx-tag " + c.state;
  t.innerHTML = CX_IC[c.state] || CX_IC.na;
  t.append(c.tag);
  if (c.text) t.title = c.text;
  return t;
}

// alertLi chama: deixa o lugar do selo e pede depois, em lote
function cxSlot(e) {
  if (e.demo && hasGo) return null;
  if (e.retro) return null;
  const s = document.createElement("span");
  s.className = "cx-slot";
  s.dataset.dev = isRcEv(e) ? "radiacode" : "gmc";
  s.dataset.from = e.time;
  s.dataset.to = e.active ? 0 : e.end || 0;
  if (!CX.pend) { CX.pend = true; setTimeout(cxFill, 30); }
  return s;
}

async function cxFill() {
  CX.pend = false;
  if (!api.GetCrossChecks) return;
  const slots = [...document.querySelectorAll(".cx-slot:not([data-done])")];
  if (!slots.length) return;
  slots.forEach((s) => { s.dataset.done = "1"; });
  const reqs = slots.map((s) => ({ dev: s.dataset.dev, from: +s.dataset.from, to: +s.dataset.to }));
  let res = [];
  try { res = await api.GetCrossChecks(reqs); } catch (e) { return; }
  res.forEach((c, i) => { if (c && c.tag && c.why !== "sem") { slots[i].textContent = ""; slots[i].appendChild(cxTagEl(c)); } });
}

// ---------- Alertas: o alerta aberto ----------

const CX_EXPL = {
  ok: () => "Os dois subiram nos mesmos minutos: é radiação de verdade, no lugar onde os dois estão.",
  no: (d) => (d === "radiacode"
    ? "Só o RC102 subiu. O GMC-800, que conta bem menos, ficou dentro do vai e vem normal dele. Se havia algo perto só do RC102, afaste e veja se volta."
    : "Só o GMC-800 subiu. O RC102, que conta umas 30 vezes mais, ficou dentro do vai e vem normal dele. Se havia algo perto só do GMC-800, afaste e veja se volta. Se não havia nada, veja a Saúde do tubo."),
  wait: (d) => `A resposta sai quando o ${cxName(cxOther(d))} juntar contagens suficientes. Até lá, fica em "conferindo".`,
  na: () => "Para conferir, os dois precisam estar no mesmo lugar, conectados e sem medir objeto.",
};

async function cxDetail(d) {
  const box = $("adCross");
  if (!box) return;
  const my = ++CX.seq;
  if (!d || !api.GetCrossCheck || d.event.retro || (d.event.demo && hasGo)) { box.hidden = true; return; }
  const e = d.event, dev = d.device === "radiacode" ? "radiacode" : "gmc";
  const ep = d.episode;
  const from = ep ? ep.from : e.time, to = e.active ? 0 : ep ? ep.to : d.end;
  let c;
  try { c = await api.GetCrossCheck(dev, from, to, true); } catch (err) { c = null; }
  if (my !== CX.seq) return;
  if (!c || c.why === "sem") { box.hidden = true; return; }
  box.hidden = false;
  box.textContent = "";
  const h = document.createElement("div");
  h.className = "cx-head";
  const b = document.createElement("b");
  b.textContent = "Conferido com o outro aparelho";
  h.append(b, cxTagEl(c));
  box.appendChild(h);
  const p = document.createElement("p");
  p.className = "small cx-text";
  p.textContent = c.text;
  box.appendChild(p);
  if (c.series && c.series.me.some((v) => v != null)) box.insertAdjacentHTML("beforeend", cxChart(c, dev, Math.max(320, Math.round(box.clientWidth || 600))));
  const x = document.createElement("p");
  x.className = "small muted";
  x.textContent = CX_EXPL[c.state] ? CX_EXPL[c.state](dev) : "";
  box.appendChild(x);
}

function cxChart(c, dev, W) {
  const s = c.series, N = s.me.length, H = 170, L = 46, T = 10, B = 8, pw = W - L - 10, ph = H - T - B;
  const me = cxName(dev), ot = cxName(cxOther(dev));
  const cMe = dev === "radiacode" ? "var(--rc)" : "var(--accent)", cOt = dev === "radiacode" ? "var(--accent)" : "var(--rc)";
  const all = s.me.concat(s.ot).filter((v) => v != null);
  const lo = Math.min(-0.3, Math.min(...all) - 0.05), hi = Math.max(1, Math.max(...all) + 0.1);
  const x = (i) => L + (i + 0.5) / N * pw, y = (v) => T + (hi - v) / (hi - lo) * ph, f = (v) => v.toFixed(1);
  const step = hi - lo > 12 ? 5 : hi - lo > 6 ? 2 : hi - lo > 2.4 ? 1 : 0.5;
  let g = "";
  for (let t = Math.ceil(lo / step) * step; t <= hi; t += step) {
    g += `<line class="${Math.abs(t) < 1e-9 ? "cx-zero" : "cx-grid"}" x1="${L}" x2="${L + pw}" y1="${f(y(t))}" y2="${f(y(t))}"/><text class="cx-yl" x="${L - 6}" y="${f(y(t) + 3.5)}" text-anchor="end">${t > 0 ? "+" : t < 0 ? "−" : ""}${Math.round(Math.abs(t) * 100)}%</text>`;
  }
  const hasOt = s.ot.some((v) => v != null);
  if (hasOt && s.otSd > 0) {
    const b2 = 2 * s.otSd;
    g += `<rect x="${L}" y="${f(y(b2))}" width="${pw}" height="${f(y(-b2) - y(b2))}" fill="${cOt}" fill-opacity=".08"/>`;
  }
  const ia = Math.round((c.at - s.t0) / s.step);
  const xa = L + ia / N * pw, right = N - ia < 4;
  g += `<line class="cx-al" x1="${f(xa)}" x2="${f(xa)}" y1="${T}" y2="${T + ph}"/><text class="cx-all" x="${f(xa + (right ? -5 : 5))}" y="${T + 12}" text-anchor="${right ? "end" : "start"}">alerta ${hhmm(c.at)}</text>`;
  const pl = (arr, col, w) => {
    let d = "", pen = false;
    arr.forEach((v, i) => { if (v == null) { pen = false; return; } d += `${pen ? "L" : "M"}${f(x(i))} ${f(y(v))}`; pen = true; });
    return d ? `<path d="${d}" fill="none" stroke="${col}" stroke-width="${w}" stroke-linejoin="round"/>` : "";
  };
  if (hasOt) g += pl(s.ot, cOt, 1.8);
  g += pl(s.me, cMe, 2.4);
  const leg = `<span><i class="sw" style="border-top:2.4px solid ${cMe}"></i>${me} (apitou)</span>` +
    (hasOt ? `<span><i class="sw" style="border-top:2px solid ${cOt}"></i>${ot}</span><span><i class="sw sq" style="background:${cOt}"></i>vai e vem normal do ${ot} (±2σ)</span>` : `<span>${ot}: sem dado neste trecho</span>`);
  return `<svg class="cx-chart" viewBox="0 0 ${W} ${H}" width="${W}" height="${H}" role="img" aria-label="Subida sobre o fundo de cada aparelho">${g}</svg>` +
    `<div class="cx-ax mono"><span style="left:${(L / W * 100).toFixed(1)}%">10 min antes</span><span style="left:${(xa / W * 100).toFixed(1)}%;transform:translateX(-50%)">${hhmm(c.at)}</span>${N > ia + 2 ? '<span style="left:100%;transform:translateX(-100%)">+5 min</span>' : ""}</div>` +
    `<div class="cx-leg small">${leg}</div><p class="small muted cx-unit">subida sobre o fundo de cada um</p>`;
}

function crossInit() {
  if (!api.GetCrossCheck) return;
  const ok = $("btnAttnOk");
  if (ok && !$("btnWhyAttn")) {
    const b = document.createElement("button");
    b.type = "button"; b.className = "band-btn"; b.id = "btnWhyAttn"; b.hidden = true; b.textContent = "Por que subiu?";
    b.addEventListener("click", () => { if (typeof whyFromBand === "function") whyFromBand(); });
    ok.before(b);
  }
  setInterval(cxTick, 5000);
  setTimeout(cxTick, 1500);
}
document.addEventListener("DOMContentLoaded", crossInit);

// ---------- demonstração (navegador sem o app) ----------
// cenário pelo localStorage "cruzDemo": conf (padrão), nao, espera, longe, desc
function cxDemo(dev, from, to, series) {
  const sc = localGet("cruzDemo", "conf");
  dev = dev === "radiacode" ? "radiacode" : "gmc";
  const now = Date.now(), at = from || now - 4 * 60e3;
  const me = cxName(dev), ot = cxName(cxOther(dev));
  const main = 1.65, oth = { conf: 1.55, nao: 0.02, espera: 0.3 }[sc];
  const c = { dev, at, to: to || now, state: "na", why: "", text: "", tag: "sem como conferir", rise: main, z: 20, oRise: oth || 0, oZ: 0, left: 0, frac: 0 };
  if (sc === "conf") Object.assign(c, { state: "ok", text: `Confirmado pelo ${ot}: ele também subiu +${Math.round(oth * 100)}% sobre o fundo dele.`, tag: `confirmado pelo ${ot}` });
  else if (sc === "nao") Object.assign(c, { state: "no", text: `O ${ot} não viu: +2% sobre o fundo dele. Pode ser algo perto só do ${me}, interferência ou o aparelho.`, tag: `só no ${me}` });
  else if (sc === "espera") Object.assign(c, { state: "wait", left: 40, frac: 0.4, tag: `conferindo com o ${ot}`, text: ot === "GMC-800" ? `Conferindo com o ${ot}: ele é menos sensível e precisa de mais uns 40 s.` : `Conferindo com o ${ot}: precisa de mais uns 40 s.` });
  else if (sc === "longe") Object.assign(c, { why: "longe", text: `Sem como conferir: o ${ot} está com você, fora de casa.` });
  else Object.assign(c, { why: "desc", text: `Sem como conferir: o ${ot} está desconectado.` });
  if (series) {
    let s = 11;
    const r = () => { s = (s * 16807) % 2147483647; return s / 2147483647; };
    const g = () => (r() + r() + r() + r() + r() + r() - 3) / Math.sqrt(0.5);
    const t0 = Math.floor(at / 60e3) * 60e3 - 10 * 60e3, n = Math.min(15, Math.floor((Math.min(now, t0 + 15 * 60e3) - t0) / 60e3));
    const sdMe = dev === "gmc" ? 0.22 : 0.04, sdOt = dev === "gmc" ? 0.04 : 0.22;
    const S = { t0, step: 60e3, me: [], ot: [], otSd: sc === "longe" || sc === "desc" ? 0 : sdOt };
    for (let i = 0; i < n; i++) {
      const on = i >= 10 ? Math.min(1, (i - 9) / 2) : 0;
      S.me.push(main * on + g() * sdMe);
      S.ot.push(sc === "longe" || sc === "desc" ? null : (oth || 0) * on + g() * sdOt);
    }
    c.series = S;
  }
  return c;
}
