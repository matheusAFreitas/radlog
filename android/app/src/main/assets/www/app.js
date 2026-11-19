// RadLog no Android: a tela (Agora, Alertas, Ajustes e a primeira vez).
// O motor é o mesmo do desktop (Go): RadLogNative.call chama os métodos do
// App e as respostas voltam por __rl.done; os eventos (leitura nova, nuvem)
// chegam por __rl.event. Fora do Android (navegador, para conferir o
// visual) a página usa dados de exemplo.
"use strict";

const $ = (id) => document.getElementById(id);
const esc = (s) => String(s == null ? "" : s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));

// ---------------- ponte com o Android ----------------
const native = window.RadLogNative || null;
const waits = new Map();
let seq = 0;
const evHandlers = {};
window.__rl = {
  done(id, res) {
    const w = waits.get(id);
    if (!w) return;
    waits.delete(id);
    if (res && res.e) w.reject(new Error(res.e)); else w.resolve(res ? res.r : null);
  },
  event(name, data) { (evHandlers[name] || []).forEach((f) => { try { f(data); } catch (e) { console.error(e); } }); },
  back() { if (!$("sheet").hidden) closeSheet(); else if (state.rcView === "espectro") spClose(); else if (state.rcView === "medir") mdClose(); else if (state.rcView) { state.rcView = ""; render(); } else if (state.tab !== "agora") go("agora"); else nat("exit"); },
};
const on = (name, f) => (evHandlers[name] = evHandlers[name] || []).push(f);
function call(method, ...args) {
  if (!native) return mock.call(method, args);
  return new Promise((resolve, reject) => {
    const id = "c" + ++seq;
    waits.set(id, { resolve, reject });
    native.call(id, method, JSON.stringify(args));
  });
}
function nat(name, arg) {
  if (!native) return mock.native(name, arg);
  return new Promise((resolve, reject) => {
    const id = "n" + ++seq;
    waits.set(id, { resolve, reject });
    native.native(id, name, arg == null ? "" : String(arg));
  });
}

// ---------------- números e textos ----------------
let lang = "pt";
const nfs = {};
const nf = (d) => (nfs[lang + d] = nfs[lang + d] || new Intl.NumberFormat(lang === "en" ? "en-US" : "pt-BR", { minimumFractionDigits: d, maximumFractionDigits: d }));
const fmt = (v, d) => (v == null || !isFinite(v) ? "—" : nf(d).format(v));
const usvDigits = (v) => (v < 1 ? 3 : v < 10 ? 2 : 1); // o mesmo do desktop
const usv = (v) => fmt(v, usvDigits(v));
const pad = (n) => String(n).padStart(2, "0");
const hhmm = (ms) => { const t = new Date(ms); return pad(t.getHours()) + ":" + pad(t.getMinutes()); };
const ddmm = (ms) => { const t = new Date(ms); return pad(t.getDate()) + "/" + pad(t.getMonth() + 1); };
const sameDay = (a, b) => new Date(a).toDateString() === new Date(b).toDateString();
const when = (ms) => (sameDay(ms, Date.now()) ? hhmm(ms) : ddmm(ms) + " " + hhmm(ms));
function T(s, ...a) {
  let out = lang === "en" && window.I18N_EN && I18N_EN[s] ? I18N_EN[s] : s;
  a.forEach((x, i) => { out = out.split("{" + i + "}").join(x); });
  return out;
}

// ---------------- desenho ----------------
const ARCS = "M34.07 40.11A19 19 0 0 1 13.93 40.11M5.01 24.66A19 19 0 0 1 15.08 7.22M32.92 7.22A19 19 0 0 1 42.99 24.66";
const SH = {
  seguro: '<circle cx="8" cy="8" r="6.5"/>',
  atencao: '<path d="M8 1.2 15.2 14.4H.8Z"/>',
  alarme: '<path d="M8 .6 15.4 8 8 15.4.6 8Z"/>',
  perigo: '<path d="M5 .8h6L15.2 5v6L11 15.2H5L.8 11V5Z"/>',
};
const COL = { seguro: "#6EE7A0", atencao: "#F5B84A", alarme: "#F28C38", perigo: "#F0656A", off: "#5D666E" };
const LBL = () => ({ seguro: T("Seguro"), atencao: T("Atenção"), alarme: T("Alarme"), perigo: T("Perigo") });
const AWAY = '<svg viewBox="0 0 16 16" fill="none" stroke="#B79CFF" stroke-width="1.5"><path d="M2.5 8.2 8 3.2l5.5 5"/><path d="M4.5 7v6h7V7"/><path d="M11 1.8l3.2 3.2"/></svg>';
const RCICO = '<svg viewBox="0 0 28 28" fill="none" stroke="#F472B6" stroke-width="1.6"><rect x="7" y="3" width="14" height="22" rx="3"/><circle cx="14" cy="10" r="3"/><path d="M10 18h8M10 21h5"/></svg>';

// ---------------- estado ----------------
const state = {
  tab: "agora", paired: false, rc: null, chart: null, span: "6h", events: [], cloud: null, settings: null,
  where: null, alarm: { sev: null, quiet: false }, perms: null, scan: null, scanning: false, scanErr: "", pick: "",
  info: null, err: "",
};

function lvOf(rc) {
  if (!rc || rc.state !== "conectado") return rc && (rc.off || rc.state === "off") ? "off" : "off";
  return rc.alertLevel || "seguro";
}
const isOff = (rc) => !!rc && (rc.off || rc.state === "off");
const todayDose = (rc) => (rc ? (rc.todayDevOK ? rc.todayDev : rc.today) : 0);

function toast(msg) {
  const t = $("toast");
  t.textContent = msg;
  t.hidden = false;
  clearTimeout(toast.t);
  toast.t = setTimeout(() => (t.hidden = true), 3500);
}
const errText = (e) => (e && e.message) || String(e);

// ---------------- cabeçalho, faixa e navegação ----------------
function renderConn() {
  const rc = state.rc, el = $("conn");
  let cls = "", txt = "";
  if (!state.paired) { cls = "off"; txt = T("Radiacode 102"); }
  else if (isOff(rc)) { cls = "off"; txt = T("RC102 com o PC"); }
  else if (rc && rc.state === "conectado") txt = T("RC102 · Bluetooth");
  else if (rc && rc.state === "erro") { cls = "err"; txt = T("RC102 · erro"); }
  else { cls = "wait"; txt = rc && rc.state === "conectando" ? T("Conectando…") : T("Procurando…"); }
  el.className = "conn " + cls;
  el.lastElementChild.textContent = txt;
}

function renderBand() {
  const b = $("band"), a = state.alarm, rc = state.rc;
  if (!a.sev || !rc) { b.hidden = true; return; }
  const perigo = a.sev === "perigo";
  const lim = (rc.limits || {})[perigo ? "danger" : "alarm"];
  b.className = "band " + a.sev;
  b.innerHTML = `<svg viewBox="0 0 16 16" fill="currentColor">${SH[a.sev]}</svg>`
    + `<b>${esc(perigo ? T("Perigo no RC102") : T("Alarme no RC102"))} <span class="bv">${usv(rc.usvh)} µSv/h</span></b>`
    + `<span class="bm">${esc(lim ? T("Acima do limite ({0}). Afaste-se e procure a fonte.", usv(lim)) : T("Afaste-se e procure a fonte."))}</span>`
    + `<div class="ba"><button id="bWhat">${esc(T("O que fazer"))}</button><button class="s" id="bSil">${esc(a.quiet ? (perigo ? T("Silenciado") : T("Tocar de novo")) : T("Silenciar"))}</button></div>`;
  b.hidden = false;
  $("bSil").onclick = () => nat("silence");
  $("bWhat").onclick = () => sheet(`<h3>${esc(T("O que fazer"))}</h3><p>${esc(T("Afaste-se do lugar onde a leitura subiu. A dose cai com a distância e com o tempo. Se a leitura continuar alta longe dali, confira se o RC102 está limpo e se não há algo radioativo com você."))}</p><button class="btn pri" onclick="closeSheet()">${esc(T("Entendi"))}</button>`);
}

function go(tab) {
  if (state.rcView && tab !== "rc102") { clearInterval(sp.timer); sp.timer = 0; state.rcView = ""; }
  state.tab = tab;
  if (tab === "rc102") state.rcUnitAsk = false; // relê a unidade do aparelho (pode ter mudado nele)
  document.querySelectorAll(".nav button").forEach((b) => b.classList.toggle("on", b.dataset.tab === tab));
  render();
  $("main").scrollTop = 0;
  if (tab === "alertas") loadEvents();
  if (tab === "ajustes") loadSettings();
}
document.querySelectorAll(".nav button").forEach((b) => (b.onclick = () => go(b.dataset.tab)));

function sheet(html) {
  const s = $("sheet");
  s.innerHTML = `<div class="scrim" onclick="closeSheet()"></div><div class="sheet"><div class="grab"></div>${html}</div>`;
  s.hidden = false;
}
function closeSheet() { $("sheet").hidden = true; $("sheet").innerHTML = ""; }
window.closeSheet = closeSheet;

function render() {
  renderConn();
  renderBand();
  $("nav").hidden = !state.paired;
  const m = $("main");
  if (!state.paired) m.innerHTML = firstRun();
  else if (state.tab === "alertas") m.innerHTML = alertas();
  else if (state.tab === "ajustes") m.innerHTML = ajustes();
  else if (state.tab === "rc102") m.innerHTML = state.rcView === "espectro" ? espectro() : state.rcView === "medir" ? medir() : state.rcView === "sgram" ? sgram() : rc102();
  else m.innerHTML = agora();
  bind();
}

// ---------------- Agora ----------------
// o número cabe dentro do círculo (~104 de 200 úteis; a fonte mono tem 0,6 em
// por letra); o tamanho sai em fração do medidor, que encolhe na tela pequena
const dialSize = (v) => Math.max(18, Math.min(34, Math.floor(104 / (String(v).length * 0.6))));

function dial(v, lv, sub) {
  const c = COL[lv] || COL.off;
  const lvl = lv === "off"
    ? `<span class="lvl" style="color:#8C969E">${esc(state.rc && state.rc.lastT ? T("última leitura às {0}", when(state.rc.lastT)) : T("sem leitura ainda"))}</span>`
    : `<span class="lvl" style="color:${c}"><svg viewBox="0 0 16 16" fill="${c}">${SH[lv]}</svg>${esc(LBL()[lv])}</span>`;
  return `<div class="hero"><div class="dial"><svg viewBox="0 0 48 48" aria-hidden="true"><path d="${ARCS}" fill="none" stroke="${c}" stroke-width="2.6" stroke-linecap="round"/><circle cx="24" cy="24" r="15.2" fill="none" stroke="#232A31" stroke-width=".8"/></svg>`
    + `<div class="in"><span class="v" style="color:${lv === "off" ? "#8C969E" : "#E7ECEF"};font-size:calc(var(--d) * ${(dialSize(v) / 200).toFixed(3)})">${v}</span><span class="u">µSv/h</span></div></div>${lvl}<span class="vs">${sub}</span></div>`;
}

// série do RC102 no gráfico (µSv/h), com o máximo e quando foi
function rcSeries() {
  const c = state.chart;
  if (!c || !c.rc) return { pts: [], max: null, maxAt: 0 };
  const pts = [];
  let max = null, maxAt = 0;
  c.rc.forEach((v, i) => {
    if (v == null) { pts.push(null); return; }
    pts.push([c.times[i], v]);
    if (max == null || v > max) { max = v; maxAt = c.times[i]; }
  });
  return { pts, max, maxAt };
}

function chartSvg() {
  const c = state.chart;
  const W = 300, H = 96;
  if (!c || !c.times || !c.times.length) return `<div class="empty">${esc(T("Sem minutos gravados ainda."))}</div>`;
  const { pts } = rcSeries();
  const vals = pts.filter(Boolean).map((p) => p[1]);
  if (!vals.length) return `<div class="empty">${esc(T("Sem minutos do RC102 neste período."))}</div>`;
  let lo = Math.min(...vals), hi = Math.max(...vals);
  // faixa mínima de ±25% em volta do meio (a mesma regra do desktop)
  const mid = (lo + hi) / 2 || 0.1;
  lo = Math.min(lo, mid * 0.75); hi = Math.max(hi, mid * 1.25);
  const pad = (hi - lo) * 0.08; lo = Math.max(0, lo - pad); hi += pad;
  const x0 = c.from, x1 = c.to;
  const x = (t) => ((t - x0) / (x1 - x0)) * W, y = (v) => H - 4 - ((v - lo) / (hi - lo)) * (H - 10);
  let g = "";
  // fora de casa: faixa lilás
  for (const p of state.periods || []) {
    const a = Math.max(p.from, x0), b = Math.min(p.to || Date.now(), x1);
    if (b > a) g += `<rect x="${x(a).toFixed(1)}" y="0" width="${(x(b) - x(a)).toFixed(1)}" height="${H}" fill="#B79CFF" fill-opacity=".08"/>`;
  }
  const bg = state.rc && state.rc.stats && state.rc.stats.bgUSv;
  if (bg && bg > lo && bg < hi) g += `<line x1="0" x2="${W}" y1="${y(bg).toFixed(1)}" y2="${y(bg).toFixed(1)}" stroke="#34404A" stroke-dasharray="3 3"/>`;
  let d = "", pen = false;
  for (const p of pts) {
    if (!p) { pen = false; continue; }
    d += (pen ? "L" : "M") + x(p[0]).toFixed(1) + " " + y(p[1]).toFixed(1);
    pen = true;
  }
  g += `<path d="${d}" fill="none" stroke="#F472B6" stroke-width="1.8" stroke-linejoin="round"/>`;
  g += `<g id="chCur" hidden><line id="chLine" y1="0" y2="${H}" stroke="#A3ADB4" stroke-width=".8"/><circle id="chDot" r="3" fill="#F472B6"/></g>`;
  return `<svg id="chSvg" viewBox="0 0 ${W} ${H}" preserveAspectRatio="none" aria-label="${esc(T("Leitura das últimas horas"))}">${g}</svg>`
    + `<div class="ax"><span>${hhmm(x0)}</span><span>${hhmm((x0 + x1) / 2)}</span><span>${esc(T("agora"))}</span></div>`;
}

function agora() {
  const rc = state.rc || {};
  const lv = lvOf(rc);
  const off = isOff(rc);
  const val = rc.state === "conectado" ? rc.usvh : rc.lastUSv;
  const bg = rc.stats && rc.stats.bgUSv;
  const sub = off ? esc(T("O RC102 está com o PC"))
    : rc.state !== "conectado" ? esc(rc.state === "erro" ? rc.err || T("erro") : rc.state === "conectando" ? T("Conectando ao RC102…") : T("Procurando o Radiacode 102…"))
    : bg > 0 ? esc(T("{0}× casa (casa: {1})", fmt(val / bg, 1), usv(bg))) : esc(T("ainda sem fundo de casa"));
  let h = dial(val > 0 || rc.state === "conectado" ? usv(val || 0) : "—", lv, sub);
  const w = state.where;
  if (w && w.open && w.radiacode === "comigo") {
    h += `<div class="away">${AWAY}<button class="since" id="bSince">${esc(T("Fora de casa desde {0}", when(w.since || w.open.from)))}</button><button id="bBack">${esc(T("Voltei"))}</button></div>`;
  }
  if (off) {
    h += `<div class="card" style="gap:8px"><b style="font-size:14px">${esc(T("O RC102 está com o PC"))}</b><span class="small">${esc(T("O celular não tenta conectar até você pegar de volta. Nada se perde: o RC102 guarda o histórico e o PC lê quando conectar."))}</span><button class="btn pri" id="bTake">${esc(T("Pegar o RC102"))}</button></div>`;
  }
  const { max, maxAt } = rcSeries();
  const today = todayDose(rc);
  h += `<div class="two"><div class="card"><span class="k">${esc(T("Dose hoje"))}</span><span class="v">${fmt(today, today < 10 ? 2 : 1)} µSv</span><span class="d">${esc(rc.todayDevOK ? T("somada pelo RC102") : T("pelo que o app gravou"))}</span></div>`
    + `<div class="card"><span class="k">${esc(T("Pico ({0})", state.span))}</span><span class="v">${max != null ? usv(max) : "—"}</span><span class="d">${max != null ? esc(T("µSv/h às {0}", hhmm(maxAt))) : "&nbsp;"}</span></div></div>`;
  h += `<div class="card ch"><div class="chhead"><span class="k">${esc(T("Últimas {0}", state.span === "6h" ? T("6 h") : T("24 h")))}</span><span class="seg"><button data-span="6h" class="${state.span === "6h" ? "on" : ""}">6 h</button><button data-span="24h" class="${state.span === "24h" ? "on" : ""}">24 h</button></span></div>${chartSvg()}<div class="small" id="chTip">&nbsp;</div></div>`;
  if (!off) {
    h += `<div class="two"><button class="btn" id="bAway">${esc(w && w.open ? T("Voltei") : T("Saí de casa"))}</button><button class="btn ghost" id="bRelease">${esc(T("Soltar o RC102"))}</button></div>`;
  }
  if (rc.state === "conectado" && rc.battery > 0) {
    h += `<div class="small" style="text-align:center">${esc(T("Bateria do RC102: {0}%", fmt(rc.battery, 0)))}</div>`;
  }
  return h;
}

// ---------------- RC102 ----------------
// o aparelho em detalhe: leitura com a margem, contagem, bateria, temperatura,
// a dose de hoje e o acumulado do aparelho (só leitura: o app não zera nem muda),
// e a configuração dele
async function loadRcUnit() {
  try {
    const s = await call("GetRcSettings");
    const cpm = !!(s && s.unitCPM);
    if (cpm !== !!state.rcCPM) { state.rcCPM = cpm; if (state.tab === "rc102" && !state.rcView) render(); }
  } catch (e) { state.rcUnitAsk = false; }
}

function rc102() {
  const rc = state.rc || {};
  const live = rc.state === "conectado";
  const age = live && rc.time ? Math.max(0, Math.round((Date.now() - rc.time) / 1000)) : null;
  const st = isOff(rc) ? T("O RC102 está com o PC") : live ? (age != null ? T("há {0} s", age) : "") : rc.state === "erro" ? rc.err || T("erro") : rc.state === "conectando" ? T("Conectando ao RC102…") : T("Procurando o Radiacode 102…");
  const v = live ? rc.usvh : null;
  const cell = (k, x) => `<div class="cell"><span class="k">${esc(k)}</span><span class="v">${x}</span></div>`;
  // 19/11 (ACE): a contagem segue a unidade que o RC102 usa na tela dele
  if (live && !state.rcUnitAsk) { state.rcUnitAsk = true; loadRcUnit(); }
  const cnt = state.rcCPM ? `${fmt(rc.cps * 60, 0)} cpm` : `${fmt(rc.cps, rc.cps < 100 ? 1 : 0)} cps`;
  let h = `<div class="card rcnow"><div class="row"><span class="k">${esc(T("Agora no RC102"))}</span><span class="k">${esc(st)}</span></div>`
    + `<div class="row big"><span class="n">${v != null ? usv(v) : "—"}</span><span class="u">µSv/h</span>${v != null ? `<span class="x">${fmt(v / 10, v < 1 ? 4 : 3)} mrem/h</span>` : ""}</div>`
    + (live && rc.usvhErr > 0 ? `<span class="small">${esc(T("margem de ± {0}%", fmt(rc.usvhErr, 0)))}</span>` : "")
    + `<div class="cells">${cell(T("Contagem"), live ? cnt : "—")}${cell(T("Bateria"), live && rc.battery > 0 ? `${fmt(rc.battery, 0)}%` : "—")}${cell(T("Temperatura"), live && rc.tempC ? `${fmt(rc.tempC, 0)} °C` : "—")}</div></div>`;
  const today = todayDose(rc);
  const acc = rc.doseLive || rc.dose;
  h += `<div class="card"><span class="k">${esc(T("Dose"))}</span>`
    + `<div class="row"><span>${esc(rc.todayDevOK ? T("Hoje (somada pelo RC102)") : T("Hoje (pelo que o app gravou)"))}</span><span class="m">${fmt(today, today < 10 ? 2 : 1)} µSv</span></div>`
    + `<div class="row"><span>${esc(T("Acumulado do aparelho"))}</span><span class="m">${acc > 0 ? (acc >= 1000 ? `${fmt(acc / 1000, 3)} mSv` : `${fmt(acc, acc < 10 ? 2 : 1)} µSv`) : "—"}</span></div>`
    + `<span class="small">${esc(T("O acumulado é o total que o RC102 soma sozinho. O app só mostra, não zera nem muda."))}</span></div>`;
  h += `<div class="list">${li(T("Espectro"), T("Picos e isótopos"), "›", "rSpec")}${li(T("Espectrograma"), T("Espectro ao longo do tempo"), "›", "rSgram")}${li(T("Medir objeto"), T("Fundo, objeto, resultado"), "›", "rMeas")}${li(T("Configurar o RC102"), T("Alarmes, som, vibração e tela"), "›", "rCfg")}</div>`;
  h += `<div class="card"><div class="row"><span>${esc([rc.model || "Radiacode 102", rc.firmware ? T("firmware {0}", rc.firmware) : ""].filter(Boolean).join(" · "))}</span>${isOff(rc) ? "" : `<button class="btn" id="rRecon" style="width:auto;padding:8px 14px">${esc(T("Reconectar"))}</button>`}</div></div>`;
  return h;
}

// ---------------- Espectro ----------------
// O RC102 junta dois espectros: o da tela dele (recomeça quando zera) e o
// acumulado do aparelho (horas em casa, não dá para zerar). A tela mostra um
// dos dois, com os picos achados no Go (o mesmo do PC): na tela do RC102, o
// que subiu acima do acumulado (o fundo da casa); no acumulado, o que sobe
// acima do contínuo. Pelo Bluetooth cada leitura é pesada: relê a cada 10 s,
// e o acumulado (fundo) a cada 10 min, só com a tela aberta.
const sp = { mode: "tela", log: true, cur: null, acc: null, accAt: 0, peaks: [], timer: 0, busy: false, err: "", tip: "" };
const SP_W = 344, SP_H = 230, SP_L = 30, SP_B = 20, SP_EMAX = 3000, SP_MINW = 40;
// faixa de energia à vista (zoom com dois dedos, arrastar com um quando ampliado)
sp.v0 = 0; sp.v1 = SP_EMAX;

function spOpen() {
  state.rcView = "espectro";
  sp.cur = null; sp.peaks = []; sp.err = ""; sp.tip = "";
  render(); $("main").scrollTop = 0;
  spTick(true);
  clearInterval(sp.timer);
  sp.timer = setInterval(() => spTick(false), 10000);
}
function spClose() {
  clearInterval(sp.timer); sp.timer = 0;
  state.rcView = "";
  render();
}

async function spTick(force) {
  if (state.rcView !== "espectro" || sp.busy) return;
  if (!state.rc || state.rc.state !== "conectado") { sp.err = T("Conecte o RC102 para ver o espectro."); spRedraw(); return; }
  sp.busy = true;
  try {
    if (!sp.acc || force || Date.now() - sp.accAt > (sp.mode === "acc" ? 60e3 : 10 * 60e3)) {
      const a = await call("GetRcSpectrum", true);
      if (a && a.counts && a.counts.length) { sp.acc = a; sp.accAt = Date.now(); }
    }
    if (sp.mode === "tela") {
      const s = await call("GetRcSpectrum", false);
      if (s && s.counts && s.counts.length) sp.cur = s;
    }
    const shown = sp.mode === "acc" ? sp.acc : sp.cur;
    sp.peaks = [];
    if (shown && shown.seconds > 60) {
      const zero = new Array(shown.counts.length).fill(0);
      const bg = sp.mode === "tela" && sp.acc ? sp.acc : null;
      sp.peaks = (await call("AnalyzeSpectrum", { a: shown.a, obj: shown.counts, objSec: shown.seconds, bg: bg ? bg.counts : zero, bgSec: bg ? bg.seconds : 1 })) || [];
    }
    sp.err = "";
  } catch (e) { sp.err = errText(e); }
  sp.busy = false;
  spRedraw();
}

// Gradua os picos que o Go achou. O σ sozinho não serve: com 1 h de espectro
// qualquer diferença pequena entre a tela e o acumulado (radônio que muda de
// lugar para lugar, deriva do ganho com a temperatura) passa de 5σ. Então
// pesa também quanto o pico sobe acima do que tem embaixo (net ÷ cont) e se
// outra linha da mesma série apareceu junto.
// - 186 keV: o Ra-226 também solta 186,2 keV e está em qualquer coisa com
//   urânio; U-235 só se desse para ver o 143,8 keV (que não é procurado),
//   então vira Ra-226 (ou U-235).
// - Linha sozinha na tela que sobe pouco (< 10% do que tem embaixo) ou não
//   passa de 5σ é ruído ou deriva: não aparece (abaixo de 100 keV pede mais).
function spGrade(peaks, tela) {
  const ps = (peaks || []).map((p) => (Math.abs(p.kev - 185.7) < 1
    ? { ...p, nuclide: "Ra-226 (ou U-235)", group: "Rádio-226 (série do urânio)" } : { ...p }));
  const n = {};
  for (const p of ps) n[p.group] = (n[p.group] || 0) + 1;
  const out = [];
  for (const p of ps) {
    const r = p.cont > 0 ? p.net / p.cont : 1, conf = n[p.group] > 1;
    // sozinha: tem que subir 10% e passar de 5σ; embaixo de 100 keV (raios X
    // e retroespalhamento, onde o Am-241 de 60 keV cai) pede 10σ e 30%
    if (tela && !conf && (r < 0.1 || p.sigma < 5 || (p.kev < 100 && (p.sigma < 10 || r < 0.3)))) continue;
    p.lv = p.sigma >= 10 && r >= 0.3 && (conf || r >= 0.6) ? "forte" : p.sigma >= 5 && (r >= 0.1 || conf) ? "provável" : "fraco";
    out.push(p);
  }
  const rank = { forte: 0, "provável": 1, fraco: 2 };
  return out.sort((a, b) => rank[a.lv] - rank[b.lv] || b.sigma - a.sigma);
}

// Detalhe do isótopo, com o mesmo texto do desktop (iso.js, gerado do
// radiacode.js): natural ou não, o que solta, de onde vem, meia-vida, onde
// aparece e as linhas dele neste espectro.
function isoSheet(label) {
  const I = window.ISO;
  if (!I) return;
  const keys = label.startsWith("Ra-226") ? ["Ra-226", "U-235"] : [label];
  const peaks = spGrade(sp.peaks, sp.mode === "tela");
  let h = "";
  keys.forEach((k, ki) => {
    const o = I.origin[k];
    if (!o) return;
    if (ki) h += `<p class="hint">${esc(T("Na mesma energia (186 keV):"))}</p>`;
    h += `<h3>${esc(k)} <span class="small">${esc(T(o[1]))}</span></h3>`;
    h += `<div class="small">${esc(o[0] ? T("Natural") : T("Feito pelo homem"))} · ${esc(T("meia-vida {0}", T(o[3])))}</div>`;
    h += `<p>${esc(T(o[2]))}</p>`;
    h += `<p><b>${esc(T("Onde aparece"))}:</b> ${esc(T(o[4]))}</p>`;
    const em = [...(I.emit[k] || "")];
    if (em.length) h += `<div class="isoem">${em.map((x) => `<div><b>${x === "x" ? "rX" : x}</b><span>${esc(T(I.tip[x]))}</span></div>`).join("")}</div>`;
  });
  const mine = peaks.filter((p) => p.nuclide === label);
  const fam = I.fam[keys[0]];
  const kin = peaks.filter((p) => p.nuclide !== label && ((fam && I.fam[p.nuclide.split(" ")[0]] === fam) || p.group === (mine[0] && mine[0].group)));
  if (mine.length) {
    h += `<p><b>${esc(T("Neste espectro"))}:</b> ${mine.map((p) => esc(T("{0} keV ({1}, {2} σ)", fmt(p.kev, 0), T(p.lv), fmt(p.sigma, 0)))).join(" · ")}</p>`;
    h += `<p class="hint">${esc(kin.length ? T("Outras linhas da mesma série apareceram junto ({0}): isso confirma.", kin.map((p) => p.nuclide.split(" ")[0] + " " + fmt(p.kev, 0)).join(", ")) : T("Só esta linha apareceu. Pode ser outra coisa na mesma energia; com mais tempo fica mais claro."))}</p>`;
  }
  sheet(h + `<button class="btn ghost" onclick="closeSheet()">${esc(T("Fechar"))}</button>`);
}

function spRedraw() { if (state.tab === "rc102" && state.rcView === "espectro" && $("sheet").hidden) { const y = $("main").scrollTop; render(); $("main").scrollTop = y; } }

const spEn = (a, i) => a[0] + a[1] * (i + 0.5) + a[2] * (i + 0.5) * (i + 0.5);
const spX = (e) => SP_L + ((Math.min(Math.max(e, sp.v0), sp.v1) - sp.v0) / (sp.v1 - sp.v0)) * (SP_W - SP_L - 4);
const spE = (x) => sp.v0 + ((x - SP_L) / (SP_W - SP_L - 4)) * (sp.v1 - sp.v0);
const spZoomed = () => sp.v0 > 0 || sp.v1 < SP_EMAX;
function spView(a, b) {
  let w = Math.min(SP_EMAX, Math.max(SP_MINW, b - a));
  a = Math.max(0, Math.min(a, SP_EMAX - w));
  sp.v0 = a; sp.v1 = a + w;
}
// marcas do eixo: passo redondo para a faixa à vista
function spTicks() {
  const w = sp.v1 - sp.v0, st = [10, 20, 50, 100, 200, 250, 500, 1000].find((x) => w / x <= 4) || 1000;
  const out = [];
  for (let e = Math.ceil(sp.v0 / st) * st; e <= sp.v1 + 1e-6; e += st) out.push(e);
  return out;
}

// média de contagens por canal em cada coluna da tela
function spCols(s) {
  const n = SP_W - SP_L - 4, sum = new Float64Array(n), cnt = new Uint16Array(n);
  // o último canal junta tudo que passou do fim da escala (estouro): não é
  // espectro e faria um "pico" falso em ~3.000 keV
  for (let i = 0; i < s.counts.length - 1; i++) {
    const e = spEn(s.a, i);
    if (e < sp.v0 || e > sp.v1) continue;
    const k = Math.min(n - 1, Math.floor(spX(e) - SP_L));
    sum[k] += s.counts[i]; cnt[k]++;
  }
  return Array.from(sum, (v, k) => (cnt[k] ? v / cnt[k] : null));
}

function spDur(sec) {
  const m = Math.floor(sec / 60), h = Math.floor(m / 60);
  return h ? T("{0} h {1} min", h, m % 60) : m ? T("{0} min", m) : T("{0} s", Math.round(sec));
}

function spSvg(s) {
  const cols = spCols(s);
  const vals = cols.filter((v) => v != null && v > 0);
  const hi = vals.length ? Math.max(...vals) : 1;
  const y = (v) => {
    const f = sp.log ? Math.log10(1 + v) / Math.log10(1 + hi) : v / hi;
    return SP_H - SP_B - f * (SP_H - SP_B - 8);
  };
  let d = "";
  cols.forEach((v, k) => { if (v != null) d += `${d ? "L" : "M"}${(SP_L + k + 0.5).toFixed(1)} ${y(v).toFixed(1)}`; });
  const grid = [0.25, 0.5, 0.75, 1].map((f) => `M${SP_L} ${(SP_H - SP_B - f * (SP_H - SP_B - 8)).toFixed(0)}H${SP_W - 4}`).join("");
  const ticks = spTicks().map((e) => `<text x="${spX(e)}" y="${SP_H - 5}" text-anchor="${spX(e) < SP_L + 12 ? "start" : spX(e) > SP_W - 16 ? "end" : "middle"}">${e}</text>`).join("");
  const marks = spGrade(sp.peaks, sp.mode === "tela").slice(0, 6).filter((p) => p.kev >= sp.v0 && p.kev <= sp.v1).map((p) => `<path d="M${spX(p.kev).toFixed(1)} ${SP_H - SP_B}V8" class="pk"/>`).join("");
  return `<svg id="spSvg" viewBox="0 0 ${SP_W} ${SP_H}" role="img" aria-label="${esc(T("Espectro"))}"><path d="${grid}" class="gr"/>${marks}<path d="${d}" class="ln"/><g class="ax">${ticks}<text x="${SP_W - 4}" y="12" text-anchor="end">${esc(sp.log ? T("contagens (log)") : T("contagens"))}</text></g><line id="spCur" y1="8" y2="${SP_H - SP_B}" class="cur" visibility="hidden"/></svg>`;
}

function espectro() {
  const s = sp.mode === "acc" ? sp.acc : sp.cur;
  const seg = (k, t) => `<button data-spm="${k}" class="${sp.mode === k ? "on" : ""}">${esc(t)}</button>`;
  let h = `<div class="subhead"><button id="spBack" aria-label="${esc(T("Voltar"))}">‹</button><b>${esc(T("Espectro"))}</b><span class="small">${s ? esc(T("juntando há {0}", spDur(s.seconds))) : ""}</span></div>`;
  h += `<div class="chips"><span class="seg">${seg("tela", T("Tela do RC102"))}${seg("acc", T("Acumulado"))}</span><button id="spLog" class="${sp.log ? "on" : ""}">${esc(T("Escala log"))}</button></div>`;
  if (sp.err) h += `<div class="card"><span class="err">${esc(sp.err)}</span></div>`;
  h += `<div class="card sp">${spZoomed() ? `<div class="spz"><span class="small">${esc(T("{0} a {1} keV", fmt(sp.v0, 0), fmt(sp.v1, 0)))}</span><button id="spAll">${esc(T("Ver tudo"))}</button></div>` : ""}<div id="spBox">${s ? spSvg(s) : `<span class="small">${esc(T("Lendo o espectro do RC102…"))}</span>`}</div><div class="small" id="spTip">${sp.tip || esc(T("Toque no gráfico para ver a energia e a contagem. Dois dedos ampliam."))}</div></div>`;
  const pk = spGrade(sp.peaks, sp.mode === "tela").slice(0, 8);
  h += `<div class="card"><span class="k">${esc(sp.mode === "tela" ? T("Picos acima do fundo de casa") : T("Picos achados"))}</span>`
    + (pk.length ? pk.map((p) => `<div class="pkrow" data-pk="${esc(p.nuclide)}"><span class="m">${fmt(p.kev, 0)} keV</span><span>${esc(T(p.nuclide))}${p.group && p.group !== p.nuclide ? `<i>${esc(T(p.group))}</i>` : ""}</span><span class="small lv-${p.lv}">${esc(T(p.lv))}</span></div>`).join("")
      : `<span class="small">${esc(s && s.seconds <= 60 ? T("Ainda pouco tempo: os picos aparecem depois de 1 min.") : T("Nenhum pico conhecido acima do esperado."))}</span>`)
    + `</div>`;
  h += `<span class="small">${esc(sp.mode === "tela" ? T("A tela do RC102 é o espectro que aparece nele e recomeça quando zera. O acumulado junta tudo desde que o aparelho foi zerado e serve de fundo de casa.") : T("O acumulado é o espectro que o RC102 junta sozinho. O app só mostra, não zera."))}</span>`;
  if (sp.mode === "tela") h += `<button class="btn danger" id="spReset">${esc(T("Zerar o espectro da tela…"))}</button>`;
  return h;
}

function bindSpectrum() {
  const q = (id, f) => { const el = $(id); if (el) el.onclick = f; };
  q("spBack", spClose);
  q("spLog", () => { sp.log = !sp.log; spRedraw(); });
  document.querySelectorAll("[data-pk]").forEach((b) => (b.onclick = () => isoSheet(b.dataset.pk)));
  document.querySelectorAll("[data-spm]").forEach((b) => (b.onclick = () => { if (sp.mode === b.dataset.spm) return; sp.mode = b.dataset.spm; sp.peaks = []; sp.tip = ""; spRedraw(); spTick(false); }));
  q("spReset", () => {
    sheet(`<h3>${esc(T("Zerar o espectro da tela do RC102?"))}</h3><p>${esc(T("O aparelho recomeça o espectro que aparece na tela dele. As contagens não se perdem: continuam no acumulado."))}</p><span class="err" id="zErr"></span><button class="btn danger" id="zOk">${esc(T("Zerar"))}</button><button class="btn ghost" onclick="closeSheet()">${esc(T("Cancelar"))}</button>`);
    $("zOk").onclick = async () => {
      $("zOk").disabled = true;
      try { await call("ResetRcSpectrum"); closeSheet(); toast(T("Espectro da tela zerado.")); sp.cur = null; sp.peaks = []; sp.tip = ""; spRedraw(); spTick(false); }
      catch (e) { $("zErr").textContent = errText(e); $("zOk").disabled = false; }
    };
  });
  q("spAll", () => { spView(0, SP_EMAX); spRedraw(); });
  const box = $("spBox");
  const s = sp.mode === "acc" ? sp.acc : sp.cur;
  if (!box || !s || !$("spSvg")) return;
  let cols = spCols(s);
  const svgX = (cx) => { const r = $("spSvg").getBoundingClientRect(); return ((cx - r.left) / r.width) * SP_W; };
  const draw = () => {
    box.innerHTML = spSvg(s); cols = spCols(s);
    const z = document.querySelector(".spz .small");
    if (z) z.textContent = T("{0} a {1} keV", fmt(sp.v0, 0), fmt(sp.v1, 0));
  };
  const cursor = (cx) => {
    const x = svgX(cx), k = Math.round(x - SP_L - 0.5);
    if (k < 0 || k >= cols.length) return;
    const c = $("spCur");
    c.setAttribute("x1", x); c.setAttribute("x2", x); c.setAttribute("visibility", "visible");
    sp.tip = esc(T("{0} keV · {1} contagens por canal", fmt(spE(x), 0), fmt(cols[k] || 0, cols[k] < 10 ? 1 : 0)));
    $("spTip").innerHTML = sp.tip;
  };
  // dedos na tela: um mostra a energia (ou arrasta, quando ampliado); dois
  // ampliam em volta do meio; toque duplo volta a ver tudo
  const pts = new Map();
  let g = null, lastTap = 0;
  const start = () => {
    const p = [...pts.values()];
    if (p.length >= 2) {
      const a = svgX(p[0].x), b = svgX(p[1].x);
      g = { pinch: true, d: Math.max(8, Math.abs(a - b)), e: spE((a + b) / 2), v0: sp.v0, v1: sp.v1 };
    } else if (p.length === 1) g = { x: p[0].x, v0: sp.v0, v1: sp.v1, pan: false };
  };
  box.addEventListener("pointerdown", (e) => {
    try { box.setPointerCapture(e.pointerId); } catch (_) { /* */ }
    pts.set(e.pointerId, { x: e.clientX });
    start();
    if (pts.size === 1) {
      if (Date.now() - lastTap < 300 && spZoomed()) { spView(0, SP_EMAX); spRedraw(); return; }
      lastTap = Date.now();
      cursor(e.clientX);
    }
  });
  box.addEventListener("pointermove", (e) => {
    if (!pts.has(e.pointerId)) { if (e.pointerType === "mouse" && e.buttons) cursor(e.clientX); return; }
    pts.get(e.pointerId).x = e.clientX;
    if (!g) return;
    const p = [...pts.values()];
    if (g.pinch && p.length >= 2) {
      const a = svgX(p[0].x), b = svgX(p[1].x), d = Math.max(8, Math.abs(a - b));
      const w = ((g.v1 - g.v0) * g.d) / d, f = ((a + b) / 2 - SP_L) / (SP_W - SP_L - 4);
      spView(g.e - f * w, g.e - f * w + w);
      draw();
    } else if (!g.pinch) {
      const dx = svgX(e.clientX) - svgX(g.x);
      if (spZoomed() && (g.pan || Math.abs(dx) > 10)) {
        g.pan = true;
        const de = (dx / (SP_W - SP_L - 4)) * (g.v1 - g.v0);
        spView(g.v0 - de, g.v1 - de);
        draw();
      } else cursor(e.clientX);
    }
  });
  const end = (e) => {
    pts.delete(e.pointerId);
    if (g && g.pinch && pts.size < 2) { g = null; spRedraw(); return; } // marcas e faixa no lugar
    if (g && g.pan && !pts.size) { g = null; spRedraw(); return; }
    if (!pts.size) g = null;
  };
  box.addEventListener("pointerup", end);
  box.addEventListener("pointercancel", end);
}

// ---------------- Medir objeto ----------------
// O mesmo cálculo do Medir objeto do PC (no Go), com o RC102: primeiro o
// fundo sem o objeto, depois o objeto encostado, cada fase com o mesmo tempo.
// O resultado fica guardado no celular. O motor manda o estado a cada segundo
// (evento "measure").
const md = { secs: 120, name: "" };

async function mdOpen() {
  state.rcView = "medir";
  try { state.meas = await call("GetMeasure"); } catch (e) { state.meas = null; }
  render(); $("main").scrollTop = 0;
}
function mdClose() { state.rcView = ""; render(); }

const mdK = (m) => (m && m.bg && m.bg.cpm > 0 ? m.bg.usv / m.bg.cpm : 0); // µSv/h por CPM do fundo
const mdClock = (s) => `${Math.floor(s / 60)}:${pad(s % 60)}`;
const mdUsv = (v) => (v == null || !isFinite(v) ? "—" : usv(Math.abs(v)));

function mdStep(n, title, sub, val, on, prog) {
  return `<div class="card mstep${on ? " on" : ""}"><div class="row"><span class="num">${n}</span><div class="grow"><div>${esc(title)}</div><div class="small">${esc(sub)}</div></div>${val ? `<span class="m">${val}</span>` : ""}</div>${prog != null ? `<div class="bar"><i style="width:${Math.round(prog * 100)}%"></i></div>` : ""}</div>`;
}

function medir() {
  const m = state.meas || {};
  const ph = m.phase || "";
  let h = `<div class="subhead"><button id="mdBack" aria-label="${esc(T("Voltar"))}">‹</button><b>${esc(T("Medir objeto"))}</b></div>`;
  if (!ph) {
    const opt = (s) => `<option value="${s}"${s === md.secs ? " selected" : ""}>${esc(T("{0} min cada fase", s / 60))}</option>`;
    h += `<div class="card"><span class="small">${esc(T("Primeiro o RC102 mede o fundo, com o objeto longe. Depois você encosta o objeto nele e mede pelo mesmo tempo. O app compara os dois e diz se o objeto é radioativo."))}</span></div>`;
    h += `<div class="list"><label class="li sel"><span class="t">${esc(T("Nome da amostra"))}</span><input id="mdName" class="txt" value="${esc(md.name)}" placeholder="${esc(T("Amostra"))}"></label><label class="li sel"><span class="t">${esc(T("Tempo"))}</span><select id="mdSecs">${[60, 120, 300, 600].map(opt).join("")}</select></label></div>`;
    h += `<span class="err" id="mdErr"></span><button class="btn pri" id="mdGo">${esc(T("Medir o fundo"))}</button>`;
    return h;
  }
  const bgDone = m.bg && ph !== "fundo";
  const prog = (el, tot) => Math.min(1, el / Math.max(tot, 1));
  h += mdStep(1, T("Fundo, sem o objeto"),
    ph === "fundo" ? T("medindo · {0} de {1}", mdClock(m.elapsed || 0), mdClock(m.seconds || 0)) : T("{0} · pronto", mdClock((m.bg && m.bg.seconds) || 0)),
    bgDone ? `${mdUsv(m.bg.usv)} µSv/h` : "", ph === "fundo", ph === "fundo" ? prog(m.elapsed || 0, m.seconds) : null);
  const objSub = ph === "objeto" ? T("medindo · {0} de {1}", mdClock(m.elapsed || 0), mdClock(m.seconds || 0)) : ph === "pronto" ? T("{0} · pronto", mdClock((m.obj && m.obj.seconds) || 0)) : T("encoste o objeto no RC102 e toque em Medir o objeto");
  const objVal = ph === "objeto" && m.elapsed > 0 && m.live ? `${mdUsv(m.bg.usv + m.live.netUSv)} µSv/h` : ph === "pronto" && m.obj ? `${mdUsv(m.obj.usv)} µSv/h` : "";
  h += mdStep(2, T("Objeto encostado no RC102"), objSub, objVal, ph === "objeto" || ph === "fundo-pronto", ph === "objeto" ? prog(m.elapsed || 0, m.seconds) : null);
  const r = ph === "pronto" ? m.result : ph === "objeto" ? m.live : null;
  if (r) {
    const sign = r.netUSv < 0 ? "−" : "+";
    const lv = ph === "pronto" ? r.level : "";
    const verdict = { sim: T("Radioativo: a diferença para o fundo é clara."), talvez: T("Provavelmente um pouco radioativo. Meça por mais tempo para confirmar."), nada: T("Nenhuma diferença que dê para separar do fundo.") }[lv] || "";
    h += `<div class="card mres ${lv}"><span class="k">${esc(ph === "pronto" ? T("Resultado") : T("Resultado (parcial)"))}</span>`
      + `<div class="row big"><span class="n">${sign}${mdUsv(r.netUSv)}</span><span class="small">${esc(T("µSv/h acima do fundo"))}</span></div>`
      + `<div class="row"><span>${esc(T("Vezes o fundo"))}</span><span class="m">${fmt(r.ratio, 1)}×</span></div>`
      + `<div class="row"><span>${esc(T("Margem de erro"))}</span><span class="m">± ${mdUsv(r.netErrUSv > 0 ? r.netErrUSv : r.netErr * mdK(m))}</span></div>`
      + (verdict ? `<b class="vd">${esc(verdict)}</b>` : "")
      + `<span class="small">${esc(state.cloud && state.cloud.email ? T("Mesmo cálculo do Medir objeto do PC. O resultado fica guardado no celular e vai para o PC pela nuvem.") : T("Mesmo cálculo do Medir objeto do PC, com o resultado guardado no celular."))}</span></div>`;
  }
  if (ph === "fundo") h += `<div class="two"><button class="btn ghost" id="mdCancel">${esc(T("Cancelar"))}</button></div>`;
  if (ph === "fundo-pronto") h += `<span class="err" id="mdErr"></span><button class="btn pri" id="mdObj">${esc(T("Medir o objeto"))}</button><button class="btn ghost" id="mdCancel">${esc(T("Cancelar"))}</button>`;
  if (ph === "objeto") h += `<div class="two"><button class="btn ghost" id="mdCancel">${esc(T("Cancelar"))}</button><button class="btn pri" id="mdStop"${(m.elapsed || 0) < 5 ? " disabled" : ""}>${esc(T("Terminar agora"))}</button></div>`;
  if (ph === "pronto") h += `<button class="btn pri" id="mdNew">${esc(T("Medir outro objeto"))}</button>`;
  return h;
}

function bindMedir() {
  const q = (id, f) => { const el = $(id); if (el) el.onclick = f; };
  const act = async (f) => { try { state.meas = await f(); } catch (e) { if ($("mdErr")) $("mdErr").textContent = errText(e); else toast(errText(e)); return; } render(); };
  q("mdBack", mdClose);
  if ($("mdName")) $("mdName").oninput = () => { md.name = $("mdName").value; };
  if ($("mdSecs")) $("mdSecs").onchange = () => { md.secs = +$("mdSecs").value; };
  q("mdGo", () => act(() => call("StartMeasure", { name: md.name.trim(), device: "radiacode", seconds: md.secs, auto: false, homeBg: false })));
  q("mdObj", () => act(() => call("StartMeasurePhase", "objeto", md.secs, md.name.trim())));
  q("mdStop", () => act(() => call("StopMeasure")));
  q("mdCancel", () => act(() => call("CancelMeasure")));
  q("mdNew", () => act(() => call("CancelMeasure")));
}

// ---------------- Espectrograma ----------------
// O espectro ao longo do tempo, montado no Go com o que o app grava a cada
// minuto (o mesmo do PC): cada linha é um intervalo, de cima (mais antigo)
// para baixo (agora); a cor é a contagem por minuto em cada faixa de energia,
// com as cores originais do PC. Intervalo sem leitura sai mais escuro (média).
const SG_SPANS = { "6h": [6, 120], "24h": [24, 300], "7d": [168, 1800] };
const SG_CMAP = [[0, [6, 6, 26]], [0.15, [26, 26, 168]], [0.35, [31, 111, 224]], [0.5, [25, 195, 214]], [0.65, [78, 227, 106]], [0.78, [242, 227, 58]], [0.9, [242, 139, 43]], [1, [232, 49, 122]]];
const sg = { span: "24h", data: null, max: 1, busy: false, err: "", tip: "" };

function sgCmap(v) {
  v = Math.max(0, Math.min(1, v));
  for (let k = 1; k < SG_CMAP.length; k++) if (v <= SG_CMAP[k][0]) {
    const [a, ca] = SG_CMAP[k - 1], [b, cb] = SG_CMAP[k], u = (v - a) / (b - a);
    return ca.map((c, j) => Math.round(c + (cb[j] - c) * u));
  }
  return SG_CMAP[SG_CMAP.length - 1][1];
}

function sgOpen() { state.rcView = "sgram"; sg.tip = ""; render(); $("main").scrollTop = 0; sgLoad(); }

async function sgLoad() {
  if (sg.busy) return;
  sg.busy = true; sg.err = "";
  const [h, row] = SG_SPANS[sg.span], to = Date.now();
  try {
    const d = await call("GetRcSpectrogram", to - h * 3600e3, to, row, 1500);
    sg.data = d && d.rows ? d : null;
    // a cor vai até o 99,5% (um pico raro não apaga o resto)
    const v = [];
    if (sg.data) for (const r of sg.data.rows) if (r) for (const x of r) if (x > 0) v.push(x);
    v.sort((a, b) => a - b);
    sg.max = v.length ? v[Math.min(v.length - 1, Math.floor(v.length * 0.995))] : 1;
  } catch (e) { sg.err = errText(e); sg.data = null; }
  sg.busy = false;
  if (state.tab === "rc102" && state.rcView === "sgram") render();
}

function sgram() {
  const seg = (k, t) => `<button data-sgs="${k}" class="${sg.span === k ? "on" : ""}">${esc(t)}</button>`;
  const d = sg.data;
  const any = d && d.rows.some(Boolean);
  let h = `<div class="subhead"><button id="sgBack" aria-label="${esc(T("Voltar"))}">‹</button><b>${esc(T("Espectrograma"))}</b></div>`;
  h += `<div class="chips"><span class="seg">${seg("6h", T("6 h"))}${seg("24h", T("24 h"))}${seg("7d", T("7 dias"))}</span></div>`;
  if (sg.err) h += `<div class="card"><span class="err">${esc(sg.err)}</span></div>`;
  h += `<div class="card sgc">`;
  if (!d) h += `<span class="small">${esc(sg.busy ? T("Montando o espectrograma…") : T("Sem dados."))}</span>`;
  else {
    const t0 = d.from, t1 = d.to, long = t1 - t0 > 26 * 3600e3;
    const lbl = (t) => (long ? ddmm(t) + " " : "") + hhmm(t);
    h += `<div class="sgwrap"><div class="sgy"><span>${lbl(t0)}</span><span>${lbl((t0 + t1) / 2)}</span><span>${esc(T("agora"))}</span></div><canvas id="sgCv"></canvas></div>`
      + `<div class="sgx"><span>0</span><span>500</span><span>1000</span><span>1500 keV</span></div>`;
    if (!any) h += `<span class="small">${esc(T("Nada gravado nesse período. O app grava o espectro a cada minuto com o RC102 conectado."))}</span>`;
  }
  h += `<div class="small" id="sgTip">${sg.tip || esc(T("Toque para ver a hora, a energia e a contagem."))}</div></div>`;
  h += `<span class="small">${esc(T("De cima (mais antigo) para baixo (agora). A cor é a contagem por minuto em cada energia: um isótopo aparece como uma faixa em pé, e a chuva acende tudo de uma vez."))}</span>`;
  return h;
}

function bindSgram() {
  const q = (id, f) => { const el = $(id); if (el) el.onclick = f; };
  q("sgBack", () => { state.rcView = ""; render(); });
  document.querySelectorAll("[data-sgs]").forEach((b) => (b.onclick = () => { if (sg.span === b.dataset.sgs) return; sg.span = b.dataset.sgs; sg.data = null; sg.tip = ""; render(); sgLoad(); }));
  const cv = $("sgCv"), d = sg.data;
  if (!cv || !d) return;
  const W = d.bins, H = d.rows.length;
  cv.width = W; cv.height = H;
  const g = cv.getContext("2d"), img = g.createImageData(W, H), lm = Math.log1p(sg.max);
  for (let y = 0; y < H; y++) {
    const row = d.rows[y], dim = d.gap[y] ? 0.5 : 1;
    for (let x = 0; x < W; x++) {
      const o = (y * W + x) * 4;
      if (!row) { img.data[o] = 13; img.data[o + 1] = 16; img.data[o + 2] = 18; img.data[o + 3] = 255; continue; }
      const [r, gg, b] = sgCmap(Math.log1p(row[x]) / lm);
      img.data[o] = r * dim; img.data[o + 1] = gg * dim; img.data[o + 2] = b * dim; img.data[o + 3] = 255;
    }
  }
  g.putImageData(img, 0, 0);
  const move = (ev) => {
    const r = cv.getBoundingClientRect();
    const k = Math.floor(((ev.clientY - r.top) / r.height) * H), e = Math.max(0, ((ev.clientX - r.left) / r.width) * d.eMax);
    if (k < 0 || k >= H) return;
    const a = d.from + k * d.rowSec * 1000, b = a + d.rowSec * 1000;
    const when = (d.to - d.from > 26 * 3600e3 ? ddmm(a) + " " : "") + hhmm(a) + "–" + hhmm(b);
    const row = d.rows[k];
    const bin = Math.min(d.bins - 1, Math.floor((e / d.eMax) * d.bins));
    sg.tip = esc(row ? T("{0} · {1} keV · {2} por minuto · {3} cps no total", when, fmt(e, 0), fmt(row[bin], row[bin] < 10 ? 2 : 1), fmt(d.cps[k], 1)) : T("{0} · sem dados", when));
    $("sgTip").innerHTML = sg.tip;
  };
  cv.addEventListener("pointerdown", (e) => { cv.setPointerCapture(e.pointerId); move(e); });
  cv.addEventListener("pointermove", (e) => { if (e.buttons || e.pointerType === "touch") move(e); });
}

async function reconnectRc() {
  try { state.rc = await call("ReconnectRadiacode"); toast(T("Reconectando ao RC102…")); } catch (e) { toast(errText(e)); }
  render();
}

function releaseSheet() {
  sheet(`<h3>${esc(T("Soltar o RC102?"))}</h3><p>${esc(T("O celular desconecta o Bluetooth para o PC poder usar o RC102. Ele não tenta conectar de novo até você tocar em \"Pegar o RC102\"."))}</p>`
    + `<div class="tl"><i style="background:var(--accent)"></i><span>${esc(T("Nada se perde: o RC102 guarda o histórico e o PC lê quando conectar."))}</span><i style="background:var(--info)"></i><span>${esc(T("O que o celular já gravou vai para o PC pela nuvem."))}</span></div>`
    + `<button class="btn pri" id="bRelOk">${esc(T("Soltar o RC102"))}</button><button class="btn ghost" onclick="closeSheet()">${esc(T("Cancelar"))}</button>`);
  $("bRelOk").onclick = async () => { closeSheet(); await nat("owner", "pc"); setTimeout(refreshRc, 600); };
}

// 19/11 (ACE): saiu às 5:30 e só tocou às 7:02: ajusta a hora da saída
function sinceSheet() {
  const w = state.where;
  if (!w || !w.open) return;
  const cur = new Date(w.open.from);
  const v = String(cur.getHours()).padStart(2, "0") + ":" + String(cur.getMinutes()).padStart(2, "0");
  sheet(`<h3>${esc(T("Que horas você saiu de casa?"))}</h3><p>${esc(T("As leituras desde essa hora contam como fora de casa e saem do fundo da casa."))}</p>`
    + `<input class="txt" id="sinceT" type="time" value="${v}">`
    + `<button class="btn pri" id="bSinceOk">${esc(T("Salvar"))}</button><button class="btn ghost" onclick="closeSheet()">${esc(T("Cancelar"))}</button>`);
  $("bSinceOk").onclick = async () => {
    const m = /^(\d{1,2}):(\d{2})/.exec($("sinceT").value || "");
    if (!m) return;
    const d = new Date();
    d.setHours(+m[1], +m[2], 0, 0);
    if (d.getTime() > Date.now()) d.setDate(d.getDate() - 1); // depois de agora: foi ontem
    try {
      await call("SavePeriod", Object.assign({}, w.open, { from: d.getTime() }));
      state.where = await call("GetWhereabouts");
      loadPeriods();
      closeSheet();
      toast(T("Saída ajustada para {0}", when(d.getTime())));
    } catch (e) { toast(errText(e)); }
    render();
  };
}

async function toggleAway() {
  try {
    state.where = state.where && state.where.open ? await call("SetWhere", "casa", "comigo", "") : await call("LeaveHome", false, true);
    loadPeriods();
  } catch (e) { toast(errText(e)); }
  render();
}

function bindChart() {
  const svg = $("chSvg");
  if (!svg) return;
  const c = state.chart;
  const pts = rcSeries().pts.filter(Boolean);
  const move = (ev) => {
    const r = svg.getBoundingClientRect();
    const t = c.from + ((ev.clientX - r.left) / r.width) * (c.to - c.from);
    let best = null;
    for (const p of pts) if (!best || Math.abs(p[0] - t) < Math.abs(best[0] - t)) best = p;
    if (!best) return;
    const W = 300, H = 96;
    const vals = pts.map((p) => p[1]);
    let lo = Math.min(...vals), hi = Math.max(...vals);
    const mid = (lo + hi) / 2 || 0.1; lo = Math.min(lo, mid * 0.75); hi = Math.max(hi, mid * 1.25);
    const pd = (hi - lo) * 0.08; lo = Math.max(0, lo - pd); hi += pd;
    const x = ((best[0] - c.from) / (c.to - c.from)) * W, y = H - 4 - ((best[1] - lo) / (hi - lo)) * (H - 10);
    $("chCur").hidden = false;
    $("chLine").setAttribute("x1", x); $("chLine").setAttribute("x2", x);
    $("chDot").setAttribute("cx", x); $("chDot").setAttribute("cy", y);
    $("chTip").textContent = `${hhmm(best[0])} · ${usv(best[1])} µSv/h`;
  };
  svg.addEventListener("pointerdown", (e) => { svg.setPointerCapture(e.pointerId); move(e); });
  svg.addEventListener("pointermove", (e) => { if (e.buttons || e.pointerType === "touch") move(e); });
}

// ---------------- Alertas ----------------
// junta os alertas do mesmo episódio numa linha, como o PC (frontend/dist/alertas.js,
// groupEpisodes): mesmo aparelho, um começando até 1 min depois do outro acabar
// (15 min se caiu a conexão); vale o nível mais alto e o maior pico
const LVL_RANK = { atencao: 1, alarme: 2, perigo: 3 };
const lvlOf = (e) => (e.level === "alarme" || e.level === "perigo" ? e.level : "atencao");
function episodes(list) {
  const asc = list.slice().sort((x, y) => x.time - y.time);
  const now = Date.now();
  const eps = [];
  let cur = null;
  for (const e of asc) {
    const end = e.active ? now : (e.end || e.time);
    const gap = cur && cur.endReason && cur.endReason !== "app fechado" ? 15 * 60e3 : 60e3;
    if (cur && !!e.demo === cur.demo && !!e.retro === cur.retro && (e.device || "") === cur.device && e.time <= cur.until + gap) {
      cur.ms.push(e);
      if (end >= cur.until) cur.endReason = e.endReason || "";
      cur.until = Math.max(cur.until, end);
      continue;
    }
    cur = { ms: [e], until: end, demo: !!e.demo, retro: !!e.retro, device: e.device || "", endReason: e.endReason || "" };
    eps.push(cur);
  }
  return eps.map((g) => {
    const ms = g.ms;
    if (ms.length === 1) return ms[0];
    const top = ms.reduce((b, x) => (LVL_RANK[lvlOf(x)] > LVL_RANK[lvlOf(b)] ? x : b), ms[0]);
    const pk = ms.reduce((b, x) => ((x.peak || 0) > (b.peak || 0) ? x : b), ms[0]);
    const active = ms.some((x) => x.active);
    const ended = ms.every((x) => x.end || x.active);
    return Object.assign({}, top, {
      time: ms[0].time, end: active || !ended ? 0 : Math.max(...ms.map((x) => x.end)), active,
      peak: pk.peak, peakUSv: pk.peakUSv, endReason: g.endReason,
    });
  }).sort((x, y) => y.time - x.time);
}

function alertas() {
  const evs = episodes(state.events.filter((e) => e.kind === "alerta"));
  if (!evs.length) return `<h2>${esc(T("Alertas"))}</h2><div class="empty">${esc(T("Nenhum alerta ainda."))}</div>`;
  let h = `<h2>${esc(T("Alertas"))}</h2><div class="list">`;
  for (const e of evs.slice(0, 200)) {
    const lv = lvlOf(e);
    const dev = e.device === "radiacode" ? "RC102" : "GMC-800";
    const peak = e.device === "radiacode" ? e.peak : e.peakUSv;
    const dur = e.end ? Math.max(1, Math.round((e.end - e.time) / 60000)) : 0;
    h += `<div class="ev"><svg viewBox="0 0 16 16" fill="${COL[lv]}">${SH[lv]}</svg><span class="t">${esc(LBL()[lv])} · ${dev}${e.active ? ` <b style="color:${COL[lv]}">${esc(T("agora"))}</b>` : ""}</span><span class="r">${when(e.time)}</span>`
      + `<span class="s">${peak ? esc(T("pico {0} µSv/h", usv(peak))) : ""}${dur ? " · " + esc(T("{0} min", dur)) : ""}${e.endReason ? " · " + esc(T(e.endReason)) : ""}</span></div>`;
  }
  return h + "</div>";
}

// ---------------- Ajustes ----------------
function li(t, s, r, id) {
  const right = r === "on" || r === "off" ? `<span class="sw ${r === "on" ? "on" : ""}"></span>` : r ? `<span class="r">${r}</span>` : "";
  return `<button class="li"${id ? ` id="${id}"` : ""}><span class="t">${esc(t)}</span>${s ? `<span class="s">${esc(s)}</span>` : ""}${right}</button>`;
}

const SOUNDS = () => ({ sirene: T("Sirene"), bipes: T("Bipes"), mudo: T("Mudo") });

function cloudLine() {
  const c = state.cloud;
  if (!c) return [T("Nuvem (Google Drive)"), T("carregando…")];
  const st = {
    off: T("desligada"), wait: T("esperando"), ok: T("em dia"), send: T("enviando…"), offline: T("sem internet"),
    login: T("entre com o Google de novo"), clock: T("relógio do celular errado"), err: T("erro"), dup: T("pausada: aparelho copiado?"),
    old: T("nuvem do formato antigo"), vault: T("esperando a chave"),
  }[c.state] || c.state;
  const last = c.lastSync ? " · " + T("último envio {0}", when(c.lastSync)) : "";
  return [T("Nuvem (Google Drive)"), st + last + (c.err ? " · " + c.err : "")];
}

function ajustes() {
  const rc = state.rc || {}, s = state.settings || {};
  let h = `<div class="gh">${esc(T("Aparelho"))}</div><div class="list">`;
  h += li("Radiacode 102", (rc.serial ? rc.serial + " · " : "") + (rc.ble || ""), "", "");
  h += li(isOff(rc) ? T("Pegar o RC102") : T("Soltar o RC102"), isOff(rc) ? T("O PC é o dono agora") : T("Para o PC usar o RC102"), "›", "sOwner");
  h += li(T("Esquecer o aparelho"), T("Volta para a primeira vez"), "›", "sForget");
  h += `</div><div class="gh">${esc(T("Alarmes"))}</div><div class="list">`;
  h += li(T("Som do alarme"), T("O perigo tem som próprio, como no PC"), SOUNDS()[s.alarmSound || "sirene"] || "", "sSound");
  h += li(T("Configurar o RC102"), T("Alarmes, som, vibração e tela (atenção {0}, alarme {1} µSv/h)", usv((rc.limits || {}).attn || 0), usv((rc.limits || {}).alarm || 0)), "›", "sRcAlarms");
  h += li(T("Testar o alarme"), T("Toca 5 s no volume de alarme"), "›", "sTest");
  const [ct, cs] = cloudLine();
  h += `</div><div class="gh">${esc(T("Nuvem"))}</div><div class="list">`;
  h += li(ct, cs, state.cloud && state.cloud.state !== "off" ? "on" : "off", "sCloud");
  if (state.cloud && state.cloud.state !== "off") {
    h += li(T("Sincronizar agora"), "", "›", "sSync");
    h += li(T("Na rede móvel"), CELL()[(state.settings || {}).cloudCell || ""], "›", "sCell");
  }
  h += `</div><div class="gh">${esc(T("Android"))}</div><div class="list">`;
  const p = state.perms || {};
  h += li(T("Sem restrição de bateria"), T("Sem isso, alguns celulares fecham o app de madrugada"), p.battery ? "on" : "off", "sBattery");
  h += li(T("Alarme por cima da tela bloqueada"), "", p.fullScreen !== false ? "on" : "off", "sFull");
  h += `</div><div class="gh">${esc(T("Idioma"))}</div><div class="list">`;
  h += li(T("Idioma"), "", lang === "en" ? "English" : "Português", "sLang");
  h += `</div><div class="gh">${esc(T("Sobre"))}</div><div class="list">`;
  const inf = state.info || {};
  h += li("RadLog " + (inf.app || ""), (inf.model || "") + (inf.android ? " · Android " + inf.android : ""), "", "");
  h += li(T("Copiar o registro"), T("As últimas 300 linhas, para mandar quando algo der errado"), "›", "sLog");
  h += "</div>";
  return h;
}

// ---------------- primeira vez ----------------
function firstRun() {
  const p = state.perms || {};
  const steps = [
    ["bluetooth", T("Dispositivos por perto"), T("Para achar e conectar no RC102 pelo Bluetooth."), "askBluetooth"],
    ["notifications", T("Notificações"), T("A leitura fica numa notificação fixa, e o alarme toca e aparece por cima de tudo."), "askNotifications"],
    ["battery", T("Sem restrição de bateria"), T("Sem isso, alguns celulares fecham o app de madrugada."), "askBattery"],
  ];
  let h = `<h2>${esc(T("Parear o Radiacode 102"))}</h2><p class="lead">${esc(T("Três permissões, uma de cada vez, e depois a busca. O RC102 precisa estar ligado e perto."))}</p><div class="card" style="gap:10px">`;
  let now = true;
  for (const [k, t, d, ask] of steps) {
    const ok = !!p[k];
    const cls = ok ? "ok" : now ? "now" : "";
    h += `<div class="step ${cls}"><span class="n">${ok ? "✓" : steps.findIndex((x) => x[0] === k) + 1}</span><div><b>${esc(t)}</b><p>${esc(d)}</p>${!ok && now ? `<button class="btn pri" data-ask="${ask}">${esc(T("Permitir"))}</button>` : ""}</div></div>`;
    if (!ok) now = false;
  }
  h += "</div>";
  if (p.bluetooth) {
    if (p.btOn === false) {
      h += `<div class="card"><b>${esc(T("O Bluetooth do celular está desligado"))}</b><button class="btn" id="bBtOn">${esc(T("Abrir o Bluetooth"))}</button></div>`;
    } else {
      h += `<div class="gh">${esc(T("Achar o RC102"))}</div>`;
      if (state.scanning) h += `<div class="card"><span><span class="spin"></span>${esc(T("Procurando o Radiacode 102…"))}</span></div>`;
      else if (state.scan) {
        if (!state.scan.length) h += `<div class="empty">${esc(T("Nenhum Radiacode por perto. Confira se ele está ligado e se o app do PC não está conectado nele pelo Bluetooth."))}</div>`;
        for (const d of state.scan) {
          const serial = (d.name.split("#")[1] || "").trim();
          h += `<button class="dev ${state.pick === d.address ? "on" : ""}" data-dev="${esc(d.address)}">${RCICO}<span class="t">Radiacode 102</span><span class="s">${esc(serial || d.name)} · ${esc(d.address)}</span><span class="sig">${d.rssi} dBm</span></button>`;
        }
      }
      if (state.scanErr) h += `<div class="err">${esc(state.scanErr)}</div>`;
      h += `<button class="btn ${state.scan && state.scan.length ? "" : "pri"}" id="bScan" ${state.scanning ? "disabled" : ""}>${esc(state.scan ? T("Procurar de novo") : T("Procurar"))}</button>`;
      if (state.pick) h += `<button class="btn pri" id="bPair">${esc(T("Usar este aparelho"))}</button>`;
    }
  }
  return h;
}

async function doScan() {
  state.scanning = true; state.scanErr = ""; render();
  try { state.scan = (await call("ScanRadiacodeBLE")) || []; if (state.scan.length === 1) state.pick = state.scan[0].address; }
  catch (e) { state.scanErr = errText(e); state.scan = state.scan || []; }
  state.scanning = false; render();
}

async function doPair() {
  try {
    state.rc = await call("SetRadiacode", true, state.pick);
    await nat("paired", "true");
    state.paired = true;
    go("agora");
    refreshAll();
  } catch (e) { toast(errText(e)); }
}

// ---------------- nuvem ----------------
async function cloudToggle() {
  const c = state.cloud || {};
  if (c.state !== "off") {
    sheet(`<h3>${esc(T("Nuvem (Google Drive)"))}</h3><p>${esc(T("Conta: {0}", c.email || "—"))}</p>${(c.devices || []).length ? `<div class="list">${c.devices.map((d) => li(d.name, d.lastUp ? T("último envio {0}", when(d.lastUp)) : "", d.me ? T("este") : "")).join("")}</div>` : ""}`
      + `<button class="btn" id="cSync">${esc(T("Sincronizar agora"))}</button><button class="btn danger" id="cOff">${esc(T("Desconectar do Google"))}</button><button class="btn ghost" onclick="closeSheet()">${esc(T("Fechar"))}</button>`);
    $("cSync").onclick = () => { closeSheet(); syncNow(); };
    $("cOff").onclick = async () => { closeSheet(); try { state.cloud = await call("CloudDisconnect"); } catch (e) { toast(errText(e)); } render(); };
    return;
  }
  sheet(`<h3>${esc(T("Entrar com o Google"))}</h3><p>${esc(T("O celular usa a mesma nuvem do PC: uma pasta do RadLog no seu Google Drive, cifrada. Depois de entrar, digite o código de recuperação que o PC mostrou."))}</p><div id="cMsg" class="small"></div><button class="btn pri" id="cGo">${esc(T("Entrar com o Google"))}</button><button class="btn ghost" onclick="closeSheet()">${esc(T("Cancelar"))}</button>`);
  $("cGo").onclick = async () => {
    $("cGo").disabled = true;
    $("cMsg").innerHTML = `<span class="spin"></span>${esc(T("Esperando o Google no navegador…"))}`;
    try {
      await call("CloudLogin");
      const done = await call("CloudLoginWait");
      if (!done) { closeSheet(); return; }
      state.cloud = done.info;
      if (done.next === "existing") codeSheet("open");
      else if (done.next === "new") newCloudSheet();
      else if (done.next === "old") { closeSheet(); toast(T("A nuvem desta conta é do formato antigo: recomece pelo PC.")); }
      else closeSheet();
    } catch (e) { $("cMsg").innerHTML = `<span class="err">${esc(errText(e))}</span>`; $("cGo").disabled = false; }
    render();
  };
}

function codeSheet() {
  sheet(`<h3>${esc(T("Código de recuperação"))}</h3><p>${esc(T("Digite o código de 24 letras e números que o PC mostrou quando a nuvem foi criada (Configurações › Dados e backup › Nuvem)."))}</p><input class="code" id="cCode" autocomplete="off" autocapitalize="characters" spellcheck="false" placeholder="XXXX-XXXX-XXXX-XXXX-XXXX-XXXX"><div id="cErr" class="err"></div><button class="btn pri" id="cOk">${esc(T("Abrir a nuvem"))}</button><button class="btn ghost" id="cCancel">${esc(T("Cancelar"))}</button>`);
  $("cCode").focus();
  $("cCancel").onclick = async () => { closeSheet(); state.cloud = await call("CloudCancel"); render(); };
  $("cOk").onclick = async () => {
    try {
      const code = await call("CheckCloudCode", $("cCode").value);
      state.cloud = await call("CloudOpen", code);
      closeSheet();
      toast(T("Nuvem ligada. A primeira sincronização começa agora."));
      render();
    } catch (e) { $("cErr").textContent = errText(e); }
  };
}

function newCloudSheet() {
  sheet(`<h3>${esc(T("Esta conta ainda não tem nuvem"))}</h3><p>${esc(T("Crie a nuvem pelo PC (Configurações › Dados e backup › Nuvem) e depois entre aqui com a mesma conta. Assim o código de recuperação fica guardado no PC."))}</p><button class="btn pri" id="cNo">${esc(T("Entendi"))}</button>`);
  $("cNo").onclick = async () => { closeSheet(); state.cloud = await call("CloudCancel"); render(); };
}

async function syncNow() {
  try { state.cloud = await call("CloudSyncNow"); toast(T("Sincronizando…")); } catch (e) { toast(errText(e)); }
  render();
}

// ---------------- ajustes: ações ----------------
function soundSheet() {
  const s = state.settings || {};
  const cur = s.alarmSound || "sirene";
  sheet(`<h3>${esc(T("Som do alarme"))}</h3><div class="list">${Object.entries(SOUNDS()).map(([k, v]) => `<button class="li" data-sound="${k}"><span class="t">${esc(v)}</span><span class="r">${k === cur ? "✓" : ""}</span></button>`).join("")}</div><button class="btn ghost" onclick="closeSheet()">${esc(T("Fechar"))}</button>`);
  document.querySelectorAll("[data-sound]").forEach((b) => (b.onclick = async () => {
    try { state.settings = await call("SaveSettings", Object.assign({}, state.settings, { alarmSound: b.dataset.sound })); } catch (e) { toast(errText(e)); }
    closeSheet(); render();
  }));
}

// Configurar o RC102: os mesmos ajustes do PC (Configurações › Radiacode),
// lidos do aparelho agora. Alarmes de taxa, dose e contagem, som e vibração
// (e o que faz cada um disparar), tela e unidades. Nada vai para o aparelho
// sem "Gravar no RC102", e antes a tela mostra o que muda. O Go guarda a
// cópia de antes, grava só o que mudou e lê de volta; "Desfazer a última"
// grava a cópia de volta. O app usa a taxa 1 e 2 como atenção e alarme.
const RC_CTRL = () => [
  { bit: 1 << 2, name: T("Alarme 1 de taxa") },
  { bit: 1 << 3, name: T("Alarme 2 de taxa") },
  { bit: 1 << 4, name: T("Taxa fora da escala") },
  { bit: 1 << 5, name: T("Alarme 1 de dose") },
  { bit: 1 << 6, name: T("Alarme 2 de dose") },
  { bit: 1 << 7, name: T("Dose fora da escala") },
  { bit: 1 << 0, name: T("Botões") },
];
const RC_CLICK = 1 << 1; // som a cada contagem (só som)
const RC_DOSE_OFF = 9e6; // µSv: acima disso o alarme de dose está desligado
const RC_DIRS = () => [T("Automática"), T("Virada para a direita"), T("Virada para a esquerda")];

const rcDoseTxt = (v) => (v >= RC_DOSE_OFF ? T("desligado") : v >= 1000 ? `${fmt(v / 1000, v % 1000 ? 3 : 0)} mSv` : `${fmt(v, v < 10 ? 2 : 0)} µSv`);
const rcNum = (v, d) => (v > 0 && v < RC_DOSE_OFF ? fmt(v, d) : "");
// número digitado como "0,50", "1500", "1.500" ou "0.5"; vazio = 0
const rcParse = (id) => {
  let t = $(id).value.trim().replace(/\s/g, "");
  if (!t) return 0;
  if (t.includes(",")) t = t.replace(/\./g, "").replace(",", ".");
  else if (/^\d{1,3}(\.\d{3})+$/.test(t)) t = t.replace(/\./g, "");
  const v = Number(t);
  return isFinite(v) ? v : NaN;
};

function rcCfgForm(c) {
  const inp = (id, v, label) => `<input id="${id}" inputmode="decimal" value="${esc(v)}" aria-label="${esc(label)}">`;
  const row = (label, a, b, unit) => `<span class="t">${esc(label)}</span>${a}${b}<i>${esc(unit)}</i>`;
  const sw = (id, on, label, sub) => `<button class="li" id="${id}" data-on="${on ? 1 : 0}" aria-pressed="${!!on}"><span class="t">${esc(label)}</span>${sub ? `<span class="s">${esc(sub)}</span>` : ""}<span class="sw ${on ? "on" : ""}"></span></button>`;
  const sel = (id, opts, v, label) => `<label class="li sel"><span class="t">${esc(label)}</span><select id="${id}">${opts.map((o, i) => `<option value="${i}"${i === v ? " selected" : ""}>${esc(o)}</option>`).join("")}</select></label>`;
  const offs = c.offTimes && c.offTimes.length ? c.offTimes : ["5 s", "10 s", "15 s", "30 s"];
  const ctrl = RC_CTRL().map((k) => `<span>${esc(k.name)}</span><input type="checkbox" data-snd="${k.bit}"${c.soundCtrl & k.bit ? " checked" : ""} aria-label="${esc(T("Som") + ": " + k.name)}"><input type="checkbox" data-vib="${k.bit}"${c.vibroCtrl & k.bit ? " checked" : ""} aria-label="${esc(T("Vibração") + ": " + k.name)}">`).join("");
  const bg = ((state.rc && state.rc.st && state.rc.st.stats) || {}).bgUSv;
  return `<h3>${esc(T("Configurar o RC102"))}</h3>
<p>${esc(T("Lido do aparelho agora. Fica gravado nele: vale também longe do celular e do PC."))}</p>
<div class="rcg"><span class="h">${esc(T("Alarmes"))}</span><span class="h">${esc(T("nível 1"))}</span><span class="h">${esc(T("nível 2"))}</span><span></span>
${row(T("Taxa de dose"), inp("cR1", rcNum(c.doseRate1, 2), T("Taxa de dose, nível 1")), inp("cR2", rcNum(c.doseRate2, 2), T("Taxa de dose, nível 2")), "µSv/h")}
${row(T("Dose acumulada"), inp("cD1", rcNum(c.dose1, c.dose1 < 10 ? 2 : 0), T("Dose acumulada, nível 1")), inp("cD2", rcNum(c.dose2, c.dose2 < 10 ? 2 : 0), T("Dose acumulada, nível 2")), "µSv")}
${row(T("Contagem"), inp("cC1", rcNum(c.countRate1 * 60, 0), T("Contagem, nível 1")), inp("cC2", rcNum(c.countRate2 * 60, 0), T("Contagem, nível 2")), "cpm")}
</div>
<p class="hint">${esc(T("O app usa a taxa 1 como atenção e a taxa 2 como alarme. Dose vazia = desligada."))}${bg ? " " + esc(T("Fundo em casa pelo RC102: {0} µSv/h.", usv(bg))) : ""}</p>
<div class="list">${sw("cSnd", c.soundOn, T("Som no aparelho"))}${sw("cVib", c.vibroOn, T("Vibração"))}${sw("cClk", c.soundCtrl & RC_CLICK, T("Clique a cada contagem"))}</div>
<details class="rcd"><summary>${esc(T("O que faz apitar e vibrar"))}</summary><div class="rcc"><span></span><span class="h">${esc(T("Som"))}</span><span class="h">${esc(T("Vibra"))}</span>${ctrl}</div></details>
<div class="list"><label class="li sel"><span class="t">${esc(T("Brilho da tela"))}</span><input id="cBrt" type="range" min="0" max="9" value="${c.brightness}"><span class="r" id="cBrtOut">${c.brightness}</span></label>
${sel("cOff", offs, c.offTime, T("Tela apaga em"))}${sel("cDir", RC_DIRS(), c.direction, T("Orientação"))}
${sel("cUD", ["R (rem)", "Sv"], c.unitSv ? 1 : 0, T("Unidade de dose na tela dele"))}${sel("cUC", ["cps", "cpm"], c.unitCPM ? 1 : 0, T("Unidade de contagem na tela dele"))}</div>
<span class="err" id="cErr"></span>
<button class="btn pri" id="cOk">${esc(T("Gravar no RC102"))}</button>
${c.canUndo ? `<button class="btn ghost" id="cUndo">${esc(T("Desfazer a última gravação"))}</button>` : ""}
<button class="btn ghost" onclick="closeSheet()">${esc(T("Cancelar"))}</button>`;
}

function rcCfgRead(c) {
  const s = Object.assign({}, c);
  // campo que ficou como veio mantém o valor exato do aparelho (o da tela é arredondado)
  const keep = (id, k, f) => { if ($(id).value !== $(id).defaultValue) s[k] = f(rcParse(id)); };
  keep("cR1", "doseRate1", (v) => v); keep("cR2", "doseRate2", (v) => v);
  keep("cD1", "dose1", (v) => v || 1e7); keep("cD2", "dose2", (v) => v || 1e7);
  keep("cC1", "countRate1", (v) => v / 60); keep("cC2", "countRate2", (v) => v / 60);
  const on = (id) => $(id).dataset.on === "1";
  s.soundOn = on("cSnd"); s.vibroOn = on("cVib");
  let snd = c.soundCtrl & ~0xff, vib = c.vibroCtrl & ~0xfd;
  if (on("cClk")) snd |= RC_CLICK;
  document.querySelectorAll("[data-snd]").forEach((el) => { if (el.checked) snd |= +el.dataset.snd; });
  document.querySelectorAll("[data-vib]").forEach((el) => { if (el.checked) vib |= +el.dataset.vib; });
  s.soundCtrl = snd >>> 0; s.vibroCtrl = vib >>> 0;
  s.brightness = parseInt($("cBrt").value, 10);
  s.offTime = parseInt($("cOff").value, 10);
  s.direction = parseInt($("cDir").value, 10);
  s.unitSv = $("cUD").value === "1";
  s.unitCPM = $("cUC").value === "1";
  return s;
}

function rcCfgCheck(s) {
  const bad = (v) => !(v > 0) || !isFinite(v);
  if (bad(s.doseRate1) || bad(s.doseRate2) || s.doseRate1 < 0.01 || s.doseRate2 > 1000) return T("Os alarmes de taxa de dose precisam ficar entre 0,01 e 1.000 µSv/h.");
  if (s.doseRate2 <= s.doseRate1) return T("O alarme 2 de taxa precisa ser maior que o alarme 1.");
  if (bad(s.dose1) || bad(s.dose2) || s.dose1 < 0.01) return T("Os alarmes de dose começam em 0,01 µSv (ou ficam vazios para desligar).");
  if (s.dose2 < s.dose1) return T("O alarme 2 de dose não pode ser menor que o alarme 1.");
  if (bad(s.countRate1) || bad(s.countRate2) || s.countRate1 < 0.1 || s.countRate2 > 50000) return T("Os alarmes de contagem precisam ficar entre 6 e 3.000.000 cpm.");
  if (s.countRate2 <= s.countRate1) return T("O alarme 2 de contagem precisa ser maior que o alarme 1.");
  return "";
}

// o que muda de a para b: [ajuste, agora, depois]
function rcCfgDiff(a, b) {
  const out = [];
  const f = (name, x, y) => { if (x !== y) out.push([name, x, y]); };
  const u = (v) => `${fmt(v, v < 10 ? 2 : 1)} µSv/h`;
  const cpm = (v) => `${fmt(Math.round(v * 60), 0)} cpm`;
  const lig = (v) => (v ? T("ligado") : T("desligado"));
  f(T("Alarme 1 de taxa"), u(a.doseRate1), u(b.doseRate1));
  f(T("Alarme 2 de taxa"), u(a.doseRate2), u(b.doseRate2));
  f(T("Alarme 1 de dose"), rcDoseTxt(a.dose1), rcDoseTxt(b.dose1));
  f(T("Alarme 2 de dose"), rcDoseTxt(a.dose2), rcDoseTxt(b.dose2));
  f(T("Alarme 1 de contagem"), cpm(a.countRate1), cpm(b.countRate1));
  f(T("Alarme 2 de contagem"), cpm(a.countRate2), cpm(b.countRate2));
  f(T("Som no aparelho"), lig(a.soundOn), lig(b.soundOn));
  f(T("Vibração"), lig(a.vibroOn), lig(b.vibroOn));
  f(T("Clique a cada contagem"), lig(a.soundCtrl & RC_CLICK), lig(b.soundCtrl & RC_CLICK));
  for (const c of RC_CTRL()) {
    f(T("Som em \"{0}\"", c.name), lig(a.soundCtrl & c.bit), lig(b.soundCtrl & c.bit));
    f(T("Vibração em \"{0}\"", c.name), lig(a.vibroCtrl & c.bit), lig(b.vibroCtrl & c.bit));
  }
  f(T("Brilho da tela"), String(a.brightness), String(b.brightness));
  const offs = a.offTimes || [];
  f(T("Tela apaga em"), offs[a.offTime] || String(a.offTime), offs[b.offTime] || String(b.offTime));
  f(T("Orientação"), RC_DIRS()[a.direction] || String(a.direction), RC_DIRS()[b.direction] || String(b.direction));
  f(T("Unidade de dose na tela dele"), a.unitSv ? "Sv" : "R", b.unitSv ? "Sv" : "R");
  f(T("Unidade de contagem na tela dele"), a.unitCPM ? "cpm" : "cps", b.unitCPM ? "cpm" : "cps");
  return out;
}

async function rcConfigSheet() {
  let cur;
  try { cur = await call("GetRcSettings"); } catch (e) { toast(errText(e)); return; }
  sheet(`<div id="cForm">${rcCfgForm(cur)}</div><div id="cAsk" hidden></div>`);
  document.querySelectorAll("#cForm button.li[data-on]").forEach((b) => (b.onclick = () => {
    const on = b.dataset.on !== "1";
    b.dataset.on = on ? "1" : "0"; b.setAttribute("aria-pressed", String(on));
    b.querySelector(".sw").classList.toggle("on", on);
  }));
  $("cBrt").oninput = () => { $("cBrtOut").textContent = $("cBrt").value; };
  const ask = (html, ok) => {
    $("cForm").hidden = true; $("cAsk").hidden = false;
    $("cAsk").innerHTML = html + `<span class="err" id="kErr"></span><button class="btn pri" id="kOk">${esc(ok)}</button><button class="btn ghost" id="kBack">${esc(T("Voltar"))}</button>`;
    $("kBack").onclick = () => { $("cAsk").hidden = true; $("cForm").hidden = false; };
    return $("kOk");
  };
  const run = async (b, fn, done) => {
    b.disabled = true;
    try { await fn(); closeSheet(); toast(done); state.rcUnitAsk = false; refreshRc(); }
    catch (e) { $("kErr").textContent = errText(e); b.disabled = false; }
  };
  $("cOk").onclick = () => {
    const next = rcCfgRead(cur);
    const err = rcCfgCheck(next);
    $("cErr").textContent = err;
    if (err) return;
    const diff = rcCfgDiff(cur, next);
    if (!diff.length) { $("cErr").textContent = T("Nada mudou: o aparelho já está assim."); return; }
    const rows = diff.map(([k, a, b]) => `<div class="li"><span class="t">${esc(k)}</span><span class="s">${esc(a)} → ${esc(b)}</span></div>`).join("");
    const ok = ask(`<h3>${esc(T("Gravar no RC102?"))}</h3><div class="list">${rows}</div><p>${esc(T("Antes de gravar, o app guarda uma cópia do que está agora."))}</p>`, T("Gravar"));
    ok.onclick = () => run(ok, () => call("SetRcSettings", next), T("Gravado no RC102."));
  };
  if ($("cUndo")) $("cUndo").onclick = () => {
    const ok = ask(`<h3>${esc(T("Desfazer a última gravação?"))}</h3><p>${esc(T("O app grava de volta no RC102 os ajustes de antes da última gravação."))}</p>`, T("Desfazer"));
    ok.onclick = () => run(ok, () => call("UndoRcSettings"), T("Ajustes de antes gravados de volta."));
  };
}

// nuvem pela rede móvel (dados pagos): a cada 15 min, de hora em hora ou só no Wi-Fi
const CELL = () => ({ "": T("Enviar a cada 15 min, como no Wi-Fi"), hora: T("Enviar de hora em hora"), wifi: T("Enviar só no Wi-Fi") });
function cellSheet() {
  const cur = (state.settings || {}).cloudCell || "";
  sheet(`<h3>${esc(T("Nuvem na rede móvel"))}</h3><p>${esc(T("Quando o celular está nos dados móveis. No Wi-Fi envia sempre a cada 15 min, e \"Sincronizar agora\" envia na hora em qualquer rede."))}</p><div class="list">${Object.entries(CELL()).map(([k, v]) => `<button class="li" data-cell="${k}"><span class="t">${esc(v)}</span><span class="r">${k === cur ? "✓" : ""}</span></button>`).join("")}</div><button class="btn ghost" onclick="closeSheet()">${esc(T("Fechar"))}</button>`);
  document.querySelectorAll("[data-cell]").forEach((b) => (b.onclick = async () => {
    try { state.settings = await call("SaveSettings", Object.assign({}, state.settings, { cloudCell: b.dataset.cell })); } catch (e) { toast(errText(e)); }
    closeSheet(); render();
  }));
}

async function copyLog() {
  try {
    const r = await call("GetLogs", { from: 0, to: 0, levels: [], comp: "", text: "", limit: 300 });
    const inf = state.info || {};
    const head = `RadLog ${inf.app || ""} · ${inf.model || ""} · Android ${inf.android || ""}\n`;
    const txt = head + (r.entries || []).slice().reverse().map((e) => `${new Date(e.t).toLocaleString("pt-BR")}  ${e.level}  ${e.msg}`).join("\n");
    await navigator.clipboard.writeText(txt);
    toast(T("Registro copiado. Cole na conversa."));
  } catch (e) { toast(errText(e)); }
}

async function setLang(l) {
  try { state.settings = await call("SaveSettings", Object.assign({}, state.settings, { lang: l })); } catch (e) { toast(errText(e)); }
  lang = l; document.documentElement.lang = l === "en" ? "en" : "pt-BR";
  translateStatic(); render();
}

function forgetSheet() {
  sheet(`<h3>${esc(T("Esquecer o aparelho?"))}</h3><p>${esc(T("O celular para de procurar o RC102 e volta para a primeira vez. O que já foi gravado continua aqui."))}</p><button class="btn danger" id="fOk">${esc(T("Esquecer"))}</button><button class="btn ghost" onclick="closeSheet()">${esc(T("Cancelar"))}</button>`);
  $("fOk").onclick = async () => {
    closeSheet();
    try { state.rc = await call("SetRadiacode", false, ""); } catch (e) { toast(errText(e)); }
    await nat("paired", "false");
    state.paired = false; state.scan = null; state.pick = "";
    render();
  };
}

// ---------------- ligações ----------------
function bind() {
  const q = (id, f) => { const el = $(id); if (el) el.onclick = f; };
  document.querySelectorAll("[data-ask]").forEach((b) => (b.onclick = async () => { state.perms = await nat(b.dataset.ask); if (b.dataset.ask === "askBattery") setTimeout(loadPerms, 1500); render(); }));
  document.querySelectorAll("[data-dev]").forEach((b) => (b.onclick = () => { state.pick = b.dataset.dev; render(); }));
  document.querySelectorAll("[data-span]").forEach((b) => (b.onclick = () => { state.span = b.dataset.span; loadChart(); }));
  q("bScan", doScan);
  q("bPair", doPair);
  q("bBtOn", () => nat("openBluetooth"));
  q("bTake", async () => { await nat("owner", "phone"); setTimeout(refreshRc, 600); });
  q("bRelease", releaseSheet);
  q("bAway", toggleAway);
  q("bBack", toggleAway);
  q("bSince", sinceSheet);
  q("sOwner", () => (isOff(state.rc) ? nat("owner", "phone").then(() => setTimeout(refreshRc, 600)) : releaseSheet()));
  q("sForget", forgetSheet);
  q("sSound", soundSheet);
  q("sRcAlarms", rcConfigSheet);
  q("rCfg", rcConfigSheet);
  q("rSpec", spOpen);
  q("rMeas", mdOpen);
  if (state.rcView === "espectro") bindSpectrum();
  if (state.rcView === "medir") bindMedir();
  q("rSgram", sgOpen);
  if (state.rcView === "sgram") bindSgram();
  q("rRecon", reconnectRc);
  q("sLog", copyLog);
  q("sTest", () => { const m = (state.settings || {}).alarmSound || "sirene"; nat("testAlarm", m === "mudo" ? "sirene" : m).catch(() => {}); });
  q("sCloud", cloudToggle);
  q("sSync", syncNow);
  q("sCell", cellSheet);
  q("sBattery", () => nat("askBattery").then(() => setTimeout(loadPerms, 1500)));
  q("sFull", () => nat("askFullScreen"));
  q("sLang", () => setLang(lang === "en" ? "pt" : "en"));
  bindChart();
}

function translateStatic() {
  document.querySelectorAll("[data-t]").forEach((el) => {
    el.dataset.pt = el.dataset.pt || el.textContent;
    el.textContent = T(el.dataset.pt);
  });
}

// ---------------- dados ----------------
async function refreshRc() { try { state.rc = await call("GetRadiacode"); } catch (e) { /* motor ainda subindo */ } render(); }
async function loadChart() {
  try { state.chart = await call("GetChart", state.span); } catch (e) { /* sem gráfico */ }
  if (state.tab === "agora" && state.paired) render();
}
async function loadPeriods() { try { state.periods = await call("GetPeriods"); } catch (e) { state.periods = []; } }
async function loadEvents() { try { state.events = (await call("GetEvents", "alerta")) || []; } catch (e) { state.events = []; } if (state.tab === "alertas") render(); }
async function loadPerms() { try { state.perms = await nat("perms"); } catch (e) { /* fora do Android */ } render(); }
async function loadSettings() {
  try { state.settings = await call("GetSettings"); } catch (e) { /* */ }
  try { state.cloud = await call("GetCloud"); } catch (e) { /* */ }
  await loadPerms();
}
async function refreshAll() {
  await refreshRc();
  try { state.where = await call("GetWhereabouts"); } catch (e) { /* */ }
  await loadPeriods();
  loadChart();
}

on("radiacode", (rc) => {
  const was = state.rc && state.rc.state;
  state.rc = rc;
  if (state.tab === "agora" && state.paired) {
    // não redesenha o gráfico a cada segundo enquanto a pessoa arrasta
    const tip = $("chTip");
    if (tip && $("chCur") && !$("chCur").hidden) { renderConn(); renderBand(); return; }
    render();
  } else if (state.tab === "rc102" && !state.rcView && $("sheet").hidden) render();
  else { renderConn(); renderBand(); }
  if (was !== rc.state && rc.state === "conectado") loadChart();
});
on("measure", (m) => {
  state.meas = m;
  // não redesenha enquanto a pessoa digita o nome
  if (state.tab === "rc102" && state.rcView === "medir" && $("sheet").hidden && document.activeElement && document.activeElement.id !== "mdName") render();
});
on("alarm", (a) => { state.alarm = a; renderBand(); });
on("cloud", (c) => { state.cloud = c; if (state.tab === "ajustes") render(); });
on("alerts", () => { if (state.tab === "alertas") loadEvents(); });
on("periods", () => loadPeriods());
on("whereabouts", (w) => { state.where = w; if (state.tab === "agora") render(); });
on("resume", () => { loadPerms(); refreshAll(); });

async function boot() {
  try { state.info = await nat("info"); } catch (e) { /* */ }
  try { state.settings = await call("GetSettings"); lang = state.settings.lang === "en" ? "en" : "pt"; } catch (e) { /* */ }
  if (lang === "en") document.documentElement.lang = "en";
  translateStatic();
  try { state.paired = !!(await nat("isPaired")); } catch (e) { /* */ }
  try { state.alarm = (await nat("alarm")) || state.alarm; } catch (e) { /* */ }
  await loadPerms();
  if (state.paired) { await refreshAll(); go("agora"); } else render();
  setInterval(loadChart, 60000);
}

// ---------------- dados de exemplo (fora do Android) ----------------
const mock = (() => {
  const qs = new URLSearchParams(location.search);
  const scene = qs.get("cena") || "agora";
  const now = Date.now();
  const lvl = scene === "alarme" ? "alarme" : scene === "atencao" ? "atencao" : "";
  const val = scene === "alarme" ? 0.742 : scene === "atencao" ? 0.284 : 0.112;
  const rc = {
    state: scene === "solto" ? "off" : "conectado", off: scene === "solto", via: "bluetooth", serial: "RC-102-000000", ble: "AA:BB:CC:DD:EE:FF",
    usvh: val, cps: 12.3, today: 1.86, todayDev: 1.92, todayDevOK: true, lastT: now - 60000, lastUSv: 0.111, battery: 87, time: now - 1000, tempC: 27, usvhErr: 12, model: "RadiaCode-102", firmware: "4.15", doseLive: 48.7,
    alertLevel: lvl, limits: { attn: 0.25, alarm: 0.56, danger: 10 }, stats: { bgUSv: 0.108 },
  };
  const N = 72, from = now - 6 * 3600e3, times = [], rcs = [];
  let s = 7;
  const r = () => { s = (s * 16807) % 2147483647; return s / 2147483647; };
  for (let i = 0; i < N; i++) {
    times.push(from + i * 300e3);
    let v = 0.11 + (r() - 0.5) * 0.02;
    if (i > 30 && i < 62) v += 0.012;
    if (scene === "alarme" && i >= 68) v = 0.74 + (r() - 0.5) * 0.04;
    rcs.push(v);
  }
  const chart = { from, to: now, times, rc: rcs };
  const events = [
    { kind: "alerta", level: "alarme", device: "radiacode", time: now - 3 * 60e3, peak: 0.78, active: scene === "alarme" },
    { kind: "alerta", level: "atencao", device: "radiacode", time: now - 26 * 3600e3, end: now - 26 * 3600e3 + 7 * 60e3, peak: 0.31 },
    { kind: "alerta", level: "atencao", device: "", time: now - 50 * 3600e3, end: now - 50 * 3600e3 + 3 * 60e3, peakUSv: 0.27 },
  ];
  const settings = { alarmSound: "sirene", lang: qs.get("lang") || "pt", levels: { snoozeMin: 5 } };
  const where = scene === "fora" ? { open: { from: now - 2.5 * 3600e3 }, since: now - 2.5 * 3600e3, radiacode: "comigo", gmc: "casa" } : { open: null };
  const cloud = { state: "ok", email: "pessoa@exemplo.com", lastSync: now - 9 * 60e3, devices: [{ name: "PC de casa", lastUp: now - 12 * 60e3 }, { name: "Galaxy Z Flip6", lastUp: now - 9 * 60e3, me: true }] };
  const scan = [{ address: "AA:BB:CC:DD:EE:FF", name: "RadiaCode-102#RC-102-000000", rssi: -58 }];
  // medição de mentira: ?cena=medir-fundo|medir-objeto|medir-pronto
  const bg = { counts: 1600, seconds: 120, cpm: 800, errCPM: 20, usv: 0.11 };
  let meas = scene === "medir-fundo" ? { phase: "fundo", seconds: 120, elapsed: 48, device: "radiacode" }
    : scene === "medir-objeto" ? { phase: "objeto", seconds: 120, elapsed: 84, device: "radiacode", bg, live: { netUSv: 0.35, netErr: 290, ratio: 4.2, netCPM: 2545 } }
    : scene === "medir-pronto" ? { phase: "pronto", seconds: 120, elapsed: 120, device: "radiacode", bg, obj: { seconds: 120, usv: 0.46, cpm: 3345 }, result: { netUSv: 0.35, netErr: 290, ratio: 4.2, netCPM: 2545, level: "sim" } }
    : { phase: "" };
  let rcs2 = {
    doseRate1: 0.5, doseRate2: 2, dose1: 500, dose2: 1000, countRate1: 25, countRate2: 100,
    soundOn: true, soundCtrl: 0x1ffc, vibroOn: true, vibroCtrl: 0x1cfe, brightness: 5, offTime: 3,
    offTimes: ["5 s", "10 s", "15 s", "30 s", "2 min", "5 min"], direction: 0, unitSv: true, unitCPM: true,
    energyA0: 5.43, energyA1: 2.351, energyA2: 0.000381, canUndo: true,
  };
  const ok = (v) => new Promise((res) => setTimeout(() => res(JSON.parse(JSON.stringify(v))), 30));
  return {
    call(m) {
      switch (m) {
        case "GetRadiacode": case "ReconnectRadiacode": return ok(rc);
        case "GetChart": return ok(chart);
        case "GetEvents": return ok(events);
        case "GetSettings": return ok(settings);
        case "GetCloud": return ok(cloud);
        case "GetWhereabouts": return ok(where);
        case "SavePeriod": { if (where.open) where.open.from = where.since = arguments[1][0].from; return ok(arguments[1][0]); }
        case "GetPeriods": return ok(scene === "fora" ? [{ from: now - 2.5 * 3600e3, to: 0 }] : []);
        case "ScanRadiacodeBLE": return new Promise((res) => setTimeout(() => res(scan), 800));
        case "GetRcSettings": case "UndoRcSettings": return ok(rcs2);
        case "GetRcSpectrum": {
          // fundo de casa de mentira: contínuo caindo + Pb-214, Bi-214 e K-40
          const acc = arguments[1][0], sec = acc ? 11520 : 600, a = [-6, 2.4, 0.0004];
          const counts = Array.from({ length: 1024 }, (_, i) => {
            const e = a[0] + a[1] * i + a[2] * i * i;
            const g = (c, w, h) => h * Math.exp(-((e - c) ** 2) / (2 * w * w));
            const v = (e < 30 ? e / 30 : 1) * 900 * Math.exp(-e / 260) + 6 + g(352, 14, 60) + g(609, 20, 45) + g(1461, 34, 14);
            return Math.max(0, Math.round((v * sec) / 11520 * (1 + (r() - 0.5) * 0.2)));
          });
          return ok({ seconds: sec, a, counts, accum: !!acc });
        }
        case "GetMeasure": return ok(meas);
        case "StartMeasure": meas = { phase: "fundo", seconds: arguments[1][0].seconds, elapsed: 0, name: arguments[1][0].name, device: "radiacode" }; return ok(meas);
        case "StartMeasurePhase": meas = Object.assign({}, meas, { phase: "objeto", elapsed: 0, live: null }); return ok(meas);
        case "StopMeasure": return ok(meas);
        case "CancelMeasure": meas = { phase: "" }; return ok(meas);
        case "GetRcSpectrogram": {
          const [f, t, rs] = arguments[1], n = Math.ceil((t - f) / (rs * 1000)), rows = [], gap = [], cps = [];
          for (let k = 0; k < n; k++) {
            const rain = k > n * 0.55 && k < n * 0.62 ? 1.8 : 1;
            rows.push(k < n * 0.08 ? null : Array.from({ length: 256 }, (_, b) => { const e = b * 1500 / 256; return rain * (40 * Math.exp(-e / 200) + 0.6 + 6 * Math.exp(-((e - 352) ** 2) / 300) + 5 * Math.exp(-((e - 609) ** 2) / 500)) * (0.8 + r() * 0.4); }));
            gap.push(k > n * 0.3 && k < n * 0.34); cps.push(rows[k] ? 5.8 * rain : 0);
          }
          return ok({ from: f, to: t, rowSec: rs, eMax: 1500, bins: 256, rows, gap, cps, secs: [] });
        }
        case "AnalyzeSpectrum": return ok([ // o caso do ACE em 19/11: escritório × fundo de casa
          { kev: 351.9, nuclide: "Pb-214", group: "Rádio-226 (série do urânio)", sigma: 31, net: 900, cont: 6000 },
          { kev: 295.2, nuclide: "Pb-214", group: "Rádio-226 (série do urânio)", sigma: 19, net: 520, cont: 4800 },
          { kev: 185.7, nuclide: "U-235", group: "Urânio-235", sigma: 8, net: 400, cont: 5200 },
          { kev: 661.7, nuclide: "Cs-137", group: "Césio-137", sigma: 6, net: 150, cont: 3600 },
          { kev: 911.2, nuclide: "Ac-228", group: "Tório-232 (série)", sigma: 5, net: 110, cont: 2300 },
          { kev: 1460.8, nuclide: "K-40", group: "Potássio-40", sigma: 12, net: 700, cont: 1000 },
          { kev: 59.5, nuclide: "Am-241", group: "Amerício-241", sigma: 6, net: 900, cont: 6000 }]);
        case "SetRcSettings": rcs2 = Object.assign({}, arguments[1][0], { canUndo: true }); return ok(rcs2);
        default: return ok(null);
      }
    },
    native(n) {
      switch (n) {
        case "perms": return ok(scene === "primeira" ? { bluetooth: true, btOn: true, notifications: false, battery: false } : { bluetooth: true, notifications: true, battery: true, btOn: true });
        case "isPaired": return ok(scene !== "primeira" && scene !== "busca");
        case "alarm": return ok({ sev: lvl === "alarme" ? "alarme" : null, quiet: false });
        case "info": return ok({ app: "3.0.0-alpha.1", model: "Galaxy Z Flip6", android: "17" });
        default: return ok(true);
      }
    },
  };
})();

boot();
if (!native && new URLSearchParams(location.search).get("cena") === "busca") setTimeout(() => { state.perms = { bluetooth: true, notifications: true, battery: true, btOn: true }; doScan(); }, 200);
