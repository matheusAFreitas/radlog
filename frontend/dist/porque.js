"use strict";

// ---------- "Por que subiu?" (ideia 8, 2.55) ----------
//
// Painel lateral que confere as causas comuns de uma subida com os dados do
// app (whyrise.go). Abre pelo botão da faixa de alarme, de cada alerta da
// lista e da frase do Agora quando o fundo está acima do normal. Calcula ao
// abrir; fecha com Esc. Spec: ux/porque.html.
//
// Usa os globais do app.js (api, $, fmt, hhmm, showView, setSub, showRange,
// escHtml). Carregado antes do app.js: só define funções.

const WHY = { dlg: null, d: null, seq: 0 };
const WHY_ICON = {
  yes: '<svg class="why-ic" viewBox="0 0 18 18" aria-hidden="true"><circle cx="9" cy="9" r="8" fill="#1F4030"/><path d="M5 9.3 7.8 12 13 6.4" fill="none" stroke="#6EE7A0" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>',
  no: '<svg class="why-ic" viewBox="0 0 18 18" aria-hidden="true"><path d="M5 9h8" stroke="#8C969E" stroke-width="2" stroke-linecap="round"/></svg>',
  na: '<svg class="why-ic" viewBox="0 0 18 18" aria-hidden="true"><circle cx="9" cy="9" r="6.5" fill="none" stroke="#5D666E" stroke-width="1.6" stroke-dasharray="2.4 2"/></svg>',
};
const WHY_TAG = { yes: "apoia", no: "não apoia", na: "sem dado" };

function whyBuild() {
  if (WHY.dlg) return WHY.dlg;
  const d = document.createElement("dialog");
  d.id = "whyDlg";
  d.className = "why-dlg";
  d.setAttribute("aria-labelledby", "whyTitle");
  d.innerHTML =
    '<div class="why-in">' +
      '<div class="dlg-head"><div><h2 id="whyTitle">Por que subiu?</h2><span class="small muted" id="whyWhen"></span></div>' +
        '<button type="button" class="btn icon" id="whyClose" aria-label="Fechar">' +
          '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><path d="M6 6l12 12M18 6L6 18"/></svg>' +
        "</button></div>" +
      '<div class="why-body" id="whyBody"><p class="muted small">Conferindo…</p></div>' +
    "</div>";
  document.body.appendChild(d);
  d.querySelector("#whyClose").addEventListener("click", () => d.close());
  d.addEventListener("click", (e) => {
    if (e.target === d) { d.close(); return; }
    const b = e.target.closest("[data-why-go]");
    if (b) { d.close(); whyGo(b.dataset.whyGo); }
  });
  WHY.dlg = d;
  return d;
}

// dev: "gmc" ou "radiacode"; at: ms (0 = agora); title: o que abriu
async function openWhy(dev, at, title) {
  if (!api.GetWhyRise) return;
  const d = whyBuild();
  $("whyWhen").textContent = title || (dev === "radiacode" ? "Agora, no RC102" : "Agora, no GMC-800");
  $("whyBody").innerHTML = '<p class="muted small">Conferindo…</p>';
  if (!d.open) d.showModal();
  const my = ++WHY.seq;
  let w;
  try { w = await api.GetWhyRise(dev, at || 0); } catch (e) { w = null; }
  if (my !== WHY.seq) return;
  if (!w) { $("whyBody").innerHTML = '<p class="muted small">Não deu para conferir agora.</p>'; return; }
  WHY.d = w;
  whyRender(w);
}

function whyGo(link) {
  const d = WHY.d || {};
  if (link === "padroes") { setSub("graficos", "padroes"); showView("graficos", true); }
  else if (link === "graficos") showRange(d.at - 12 * 3600e3, d.at);
  else showView(link, true);
}

function whyRender(w) {
  const b = $("whyBody");
  let h = "";
  h += `<div class="why-head"><span></span>${w.noData ? "" : ` <span class="small mono muted">· ${fmt(Math.max(0, w.sigma), w.sigma < 10 ? 1 : 0)} σ</span>`}</div>`;
  h += `<div class="why-verdict"><span class="why-lab">${w.acaso ? "O mais provável" : w.noData ? "Sem dado" : "O mais provável"}</span><p></p></div>`;
  h += `<div class="why-charts" id="whyCharts">` +
    `<div class="why-row"><div class="why-nm"><span>GMC-800</span><b class="mono" id="whyGv"></b></div><svg id="whyG" viewBox="0 0 360 58" preserveAspectRatio="none"></svg></div>` +
    `<div class="why-row"><div class="why-nm"><span>RC102</span><b class="mono" id="whyRv"></b></div><svg id="whyR" viewBox="0 0 360 58" preserveAspectRatio="none"></svg></div>` +
    `<div class="why-ax mono"><span>−12 h</span><span>−6 h</span><span>${w.now ? "agora" : hhmm(w.at)}</span></div>` +
    `<div class="why-leg small muted" id="whyLeg"></div><div class="tip mono why-tip" id="whyTip" hidden></div></div>`;
  const yes = w.causes.filter((c) => c.state === "yes"), rest = w.causes.filter((c) => c.state !== "yes");
  const li = (c, open) => `<li class="why-c ${c.state}"><details${open ? " open" : ""}><summary>${WHY_ICON[c.state]}<span class="why-tt">${escHtml(c.title)}</span><span class="why-tag">${WHY_TAG[c.state]}</span><span class="why-ev">${escHtml(c.ev)}</span></summary>` +
    (c.more || c.link ? `<div class="why-more">${c.more ? `<p>${escHtml(c.more)}</p>` : ""}${c.link ? `<button type="button" class="link small-link" data-why-go="${c.link}">${escHtml(c.linkText)}</button>` : ""}</div>` : "") + "</details></li>";
  if (!w.noData) {
    if (yes.length) h += `<div class="why-grp">O que os dados apoiam</div><ul class="why-list">${yes.map((c) => li(c, !w.acaso || c.key === "acaso")).join("")}</ul>`;
    if (rest.length) h += `<div class="why-grp">O que foi conferido e não apoia</div><ul class="why-list">${rest.map((c) => li(c, false)).join("")}</ul>`;
  }
  h += `<p class="small muted why-disc">Não é diagnóstico: o painel confere as causas comuns com os seus dados.</p>`;
  b.innerHTML = h;
  b.querySelector(".why-head span").textContent = w.head;
  b.querySelector(".why-verdict p").textContent = w.verdict;
  whyCharts(w);
}

// dois gráficos empilhados, cada um com a sua escala (nunca menor que ±25%
// do normal), chuva em azul claro, fora de casa em lilás, nota e objeto em
// turquesa, cursor comum
function whyCharts(w) {
  const W = 360, H = 58, n = w.bins.length, t0 = w.at - 12 * 3600e3, span = 12 * 3600e3;
  const xt = (t) => Math.max(0, Math.min(W, (t - t0) / span * W));
  const draw = (svg, key, norm, col) => {
    const vals = w.bins.map((b) => b[key]).filter((v) => v != null);
    if (!vals.length) { svg.innerHTML = `<text x="${W / 2}" y="${H / 2 + 4}" text-anchor="middle" class="why-none">sem dado</text>`; return; }
    let lo = Math.min(...vals), hi = Math.max(...vals);
    const ref = norm > 0 ? norm : vals.slice().sort((a, b) => a - b)[vals.length >> 1];
    lo = Math.min(lo, ref * 0.75); hi = Math.max(hi, ref * 1.25);
    const pad = (hi - lo) * 0.08; lo -= pad; hi += pad;
    const y = (v) => H - 3 - (v - lo) / (hi - lo) * (H - 8);
    let s = "";
    for (const r of w.rain) s += `<rect x="${xt(r[0]).toFixed(1)}" y="0" width="${Math.max(1, xt(r[1]) - xt(r[0])).toFixed(1)}" height="${H}" class="why-rain"/>`;
    for (const m of w.marks) {
      if (m.kind === "fora") { s += `<rect x="${xt(m.t).toFixed(1)}" y="${H - 4}" width="${Math.max(2, xt(m.end || m.t) - xt(m.t)).toFixed(1)}" height="4" class="why-away"/>`; continue; }
      const x = xt(m.t).toFixed(1);
      s += `<line x1="${x}" x2="${x}" y1="0" y2="${H}" class="why-mark"/>`;
    }
    if (norm > 0) s += `<line x1="0" x2="${W}" y1="${y(norm).toFixed(1)}" y2="${y(norm).toFixed(1)}" class="why-norm"/>`;
    let path = "", pen = false;
    w.bins.forEach((b, i) => {
      const v = b[key];
      if (v == null) { pen = false; return; }
      const x = (i + 0.5) / n * W;
      path += `${pen ? "L" : "M"}${x.toFixed(1)} ${y(v).toFixed(1)}`;
      pen = true;
    });
    s += `<path d="${path}" fill="none" stroke="${col}" stroke-width="2" stroke-linejoin="round" stroke-linecap="round" vector-effect="non-scaling-stroke"/>`;
    s += `<line class="why-cur" x1="0" x2="0" y1="0" y2="${H}" visibility="hidden"/>`;
    svg.innerHTML = s;
  };
  draw($("whyG"), "g", w.gNorm, "var(--accent)");
  draw($("whyR"), "r", w.rNorm, "var(--rc)");
  const last = (k) => { for (let i = n - 1; i >= 0; i--) if (w.bins[i][k] != null) return w.bins[i][k]; return null; };
  $("whyGv").textContent = last("g") == null ? "—" : fmt(last("g"), 3);
  $("whyRv").textContent = last("r") == null ? "—" : fmt(last("r"), 3);
  const leg = [];
  if (w.rain.length) leg.push('<span><i class="why-k rain"></i>chuva</span>');
  if (w.marks.some((m) => m.kind === "fora")) leg.push('<span><i class="why-k away"></i>fora de casa</span>');
  if (w.marks.some((m) => m.kind !== "fora")) leg.push('<span><i class="why-k mark"></i>nota ou medição de objeto</span>');
  leg.push('<span><i class="why-k norm"></i>normal</span>');
  leg.push("<span>µSv/h · cada gráfico com a sua escala, nunca menor que ±25% do normal, para o ruído não parecer subida</span>");
  $("whyLeg").innerHTML = leg.join("");
  const box = $("whyCharts"), tip = $("whyTip");
  box.addEventListener("mousemove", (e) => {
    const rb = $("whyG").getBoundingClientRect(), f = (e.clientX - rb.left) / rb.width;
    if (f < 0 || f > 1) return;
    const i = Math.min(n - 1, Math.floor(f * n)), b = w.bins[i];
    box.querySelectorAll(".why-cur").forEach((l) => { const x = ((i + 0.5) / n * W).toFixed(1); l.setAttribute("x1", x); l.setAttribute("x2", x); l.setAttribute("visibility", "visible"); });
    tip.hidden = false;
    tip.textContent = `${hhmm(b.t)} · GMC ${b.g == null ? "—" : fmt(b.g, 3)} · RC ${b.r == null ? "—" : fmt(b.r, 3)}`;
    const cr = box.getBoundingClientRect();
    tip.style.left = Math.min(cr.width - tip.offsetWidth - 4, Math.max(0, e.clientX - cr.left + 10)) + "px";
    tip.style.top = "-26px";
  });
  box.addEventListener("mouseleave", () => { tip.hidden = true; box.querySelectorAll(".why-cur").forEach((l) => l.setAttribute("visibility", "hidden")); });
}

// botão "Por que subiu?" na faixa de alarme: o alerta mais grave ativo
function whyFromBand() {
  const list = state.activeAlerts || [];
  const rank = { atencao: 1, alarme: 2, perigo: 3 };
  const top = list.slice().sort((a, b) => (rank[lvlCls(b)] || 1) - (rank[lvlCls(a)] || 1))[0];
  const dev = top && isRcEv(top) ? "radiacode" : "gmc";
  openWhy(dev, 0, top ? `${lvlName(top)} no ${dev === "radiacode" ? "RC102" : "GMC-800"} · agora` : "");
}

// de um alerta da lista: olha a hora do pico daquele alerta
function whyFromAlert(e) {
  const dev = isRcEv(e) ? "radiacode" : "gmc";
  const at = e.active ? 0 : e.peakAt || e.time;
  openWhy(dev, at, `${lvlName(e)} no ${dev === "radiacode" ? "RC102" : "GMC-800"} · ${whenLabel(at || Date.now())}`);
}

function whyInit() {
  const bs = $("btnSilence");
  if (bs && !$("btnWhyBand")) {
    const b = document.createElement("button");
    b.type = "button"; b.className = "band-btn"; b.id = "btnWhyBand"; b.textContent = "Por que subiu?";
    b.addEventListener("click", whyFromBand);
    bs.before(b);
  }
  const ag = $("agWhy");
  if (ag) ag.addEventListener("click", () => openWhy("gmc", 0, "Agora, no GMC-800"));
}
document.addEventListener("DOMContentLoaded", whyInit);
