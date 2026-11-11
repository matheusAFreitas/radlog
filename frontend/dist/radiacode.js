// ---------- aba Radiacode: exemplos simulados ----------
//
// Tudo aqui usa dados SIMULADOS, no mesmo formato que o Radiacode 102 manda
// (1024 canais com calibração de energia E = a0 + a1·c + a2·c²), para deixar
// as telas prontas antes do aparelho chegar. Quando ele chegar, é trocar a
// fonte dos dados (simulação → aparelho) e manter os desenhos.
//
// O modelo do detector é simples (pico gaussiano com a resolução do 102,
// Compton plano até a borda, retroespalhamento e um contínuo do ambiente) e
// serve para a tela parecer com a de verdade, não para análise.

(() => {
  "use strict";

  // ---------- detector: canais, energia, resolução ----------
  const NCH = 1024;
  const CAL = [-6, 2.45, 0.00042]; // keV = a0 + a1·c + a2·c² (típico do 102)
  const chE = (c) => CAL[0] + CAL[1] * c + CAL[2] * c * c;
  const SIM_EN = Float64Array.from({ length: NCH }, (_, i) => chE(i));
  const SIM_DE = Float64Array.from({ length: NCH }, (_, i) => chE(i + 0.5) - chE(i - 0.5));
  // EN/DE: energia de cada canal; no espectro ao vivo, a calibração do aparelho
  let EN = SIM_EN, DE = SIM_DE;
  const calEN = (a, n) => Float64Array.from({ length: n }, (_, i) => a[0] + a[1] * (i + 0.5) + a[2] * (i + 0.5) * (i + 0.5));
  const RES = 0.094; // FWHM no Cs-137 (102: 9,4%)
  const fwhm = (e) => RES * 662 * Math.sqrt(Math.max(e, 20) / 662);
  const sig = (e) => fwhm(e) / 2.355;
  const CPS_PER_USV = 30; // 102: 30 cps = 1 µSv/h (Cs-137)

  const effPeak = (e) => 1 / (1 + Math.pow(e / 180, 2.2));
  const comptonFactor = (e) => Math.min(25, 0.15 + Math.pow(e / 250, 1.3));
  const thresh = (e) => 1 / (1 + Math.exp(-(e - 22) / 3));
  function erfc(x) { // Abramowitz-Stegun 7.1.26
    const z = Math.abs(x), t = 1 / (1 + 0.3275911 * z);
    const y = t * (0.254829592 + t * (-0.284496736 + t * (1.421413741 + t * (-1.453152027 + t * 1.061405429)))) * Math.exp(-z * z);
    return x >= 0 ? y : 2 - y;
  }

  function addLine(arr, e0, w) {
    const s = sig(e0), pk = w * effPeak(e0), comp = pk * comptonFactor(e0);
    const k = (2 * e0) / 511, ec = (e0 * k) / (1 + k), bs = e0 / (1 + k);
    const sEc = sig(ec), sBs = sig(bs);
    for (let i = 0; i < NCH; i++) {
      const e = EN[i], d = DE[i];
      if (e < 15) continue;
      let v = 0;
      const zp = (e - e0) / s;
      if (zp > -6 && zp < 6) v += (pk * Math.exp(-0.5 * zp * zp)) / (s * 2.5066) * d;
      if (e < ec + 4 * sEc) v += (comp / ec) * (0.75 + 0.5 * Math.min(1, e / ec)) * 0.5 * erfc((e - ec) / (1.4142 * sEc)) * d;
      const zb = (e - bs) / sBs;
      if (zb > -5 && zb < 5) v += (comp * 0.06 * Math.exp(-0.5 * zb * zb)) / (sBs * 2.5066) * d;
      arr[i] += v * thresh(e);
    }
  }
  function envCont(arr, w) {
    for (let i = 0; i < NCH; i++) {
      const e = EN[i];
      arr[i] += w * ((e / 70) * Math.exp(-e / 75) + 0.12 * Math.exp(-e / 420)) * thresh(e) * DE[i];
    }
  }

  // linhas gama (keV, peso relativo) de cada "componente"
  const LINES = {
    fundo: [[1460.8, 1], [609.3, 0.6], [1120.3, 0.2], [1764.5, 0.2], [351.9, 0.45], [295.2, 0.24], [241.9, 0.1],
      [583.2, 0.25], [2614.5, 0.25], [911.2, 0.2], [969, 0.12], [338.3, 0.08], [238.6, 0.3], [186.2, 0.04], [77, 0.08]],
    torio: [[238.6, 0.43], [300.1, 0.033], [911.2, 0.26], [969, 0.16], [338.3, 0.11], [463, 0.044], [794.9, 0.043],
      [209.3, 0.039], [129.1, 0.024], [583.2, 0.304], [2614.5, 0.359], [510.7, 0.081], [860.6, 0.045], [277.4, 0.023],
      [727.3, 0.066], [1620.5, 0.015], [59.3, 0.15], [67.2, 0.04], [75, 0.12], [85, 0.08], [241, 0.04]],
    fumaca: [[59.5, 1], [26.3, 0.02]],
    kcl: [[1460.8, 1]],
    chuva: [[351.9, 0.36], [295.2, 0.19], [241.9, 0.075], [609.3, 0.46], [1120.3, 0.15], [1764.5, 0.15], [768.4, 0.05], [2204.1, 0.05]],
  };
  const COMP = {
    fundo: { cps: 8.4, env: 0.55 },
    torio: { cps: 38, env: 0.08 },
    fumaca: { cps: 14, env: 0.03 },
    kcl: { cps: 1.1, env: 0.05 },
    chuva: { cps: 1, env: 0.25 }, // forma por 1 cps (a amplitude vem do tempo)
  };
  const SHAPE = {};
  for (const [k, c] of Object.entries(COMP)) {
    const l = new Float64Array(NCH), en = new Float64Array(NCH);
    for (const [e, w] of LINES[k]) addLine(l, e, w);
    envCont(en, 1);
    const sl = l.reduce((a, b) => a + b, 0), se = en.reduce((a, b) => a + b, 0);
    SHAPE[k] = Float64Array.from(l, (v, i) => ((v / sl) * (1 - c.env) + (en[i] / se) * c.env) * c.cps);
  }
  const sumRange = (a, e1, e2) => { let s = 0; for (let i = 0; i < NCH; i++) if (EN[i] >= e1 && EN[i] < e2) s += a[i]; return s; };

  // ---------- biblioteca para o cursor ----------
  const LIB = [
    [46.5, "Pb-210", "série do urânio"], [59.5, "Am-241", "detector de fumaça"], [59.3, "raio X do tungstênio", "eletrodo"],
    [63.3, "Th-234", "série do urânio"], [75, "raio X do chumbo (Kα)", ""], [85, "raio X do chumbo (Kβ)", ""],
    [92.6, "Th-234", "série do urânio"], [129.1, "Ac-228", "série do tório"], [140.5, "Tc-99m", "medicina nuclear"],
    [186.2, "Ra-226 / U-235", "rádio, urânio"], [238.6, "Pb-212", "série do tório"], [241.9, "Pb-214", "filho do radônio"],
    [295.2, "Pb-214", "filho do radônio"], [338.3, "Ac-228", "série do tório"], [351.9, "Pb-214", "filho do radônio"],
    [364.5, "I-131", "medicina nuclear"], [511, "aniquilação", "pósitron"], [583.2, "Tl-208", "série do tório"],
    [609.3, "Bi-214", "filho do radônio"], [661.7, "Cs-137", "artificial"], [727.3, "Bi-212", "série do tório"],
    [768.4, "Bi-214", "filho do radônio"], [911.2, "Ac-228", "série do tório"], [969, "Ac-228", "série do tório"],
    [1001, "Pa-234m", "série do urânio"], [1120.3, "Bi-214", "filho do radônio"], [1173.2, "Co-60", "artificial"],
    [1332.5, "Co-60", "artificial"], [1460.8, "K-40", "potássio"], [1764.5, "Bi-214", "filho do radônio"],
    [2204.1, "Bi-214", "filho do radônio"], [2614.5, "Tl-208", "série do tório"],
  ];
  function libNear(e) {
    const tol = 1.2 * sig(e);
    return LIB.filter(([le]) => Math.abs(le - e) <= tol).sort((a, b) => Math.abs(a[0] - e) - Math.abs(b[0] - e)).slice(0, 2);
  }

  // ---------- sorteio de Poisson ----------
  function poisson(l) {
    if (l <= 0) return 0;
    if (l > 30) { // aproximação normal
      let u = 0, v = 0;
      while (u === 0) u = Math.random();
      while (v === 0) v = Math.random();
      return Math.max(0, Math.round(l + Math.sqrt(l) * Math.sqrt(-2 * Math.log(u)) * Math.cos(2 * Math.PI * v)));
    }
    const L = Math.exp(-l);
    let k = 0, p = 1;
    do { k++; p *= Math.random(); } while (p > L);
    return k - 1;
  }

  // ---------- dureza ----------
  // razão (fótons acima de 1.500 keV) / (150 a 1.500 keV), relativa à do
  // fundo: 1,00 = igual ao fundo da casa. O tório sobe (Tl-208 em 2.614 keV);
  // os filhos do radônio da chuva descem (quase tudo abaixo de 1.500 keV).
  const hardRaw = (a) => sumRange(a, 1500, 3100) / Math.max(1e-9, sumRange(a, 150, 1500));
  const H_BG = hardRaw(SHAPE.fundo);
  const hardRel = (a) => hardRaw(a) / H_BG;
  const GUESS = { torio: "~Th-232 (tório)", fumaca: "~Am-241", kcl: "~K-40 (potássio)", chuva: "~Rn-222 (filhos do radônio)" };
    // palpite: o formato do que passou do fundo comparado com cada fonte
  // conhecida (semelhança de cosseno, com o espectro em bins de ~30 keV)
  const BIN = 12;
  const binned = (a) => { const o = []; for (let i = 0; i < NCH; i += BIN) { let v = 0; for (let j = i; j < i + BIN && j < NCH; j++) if (EN[j] > 30) v += a[j]; o.push(v); } return o; };
  const REF_B = Object.fromEntries(Object.keys(GUESS).map((k) => [k, binned(SHAPE[k])]));
  function guessFrom(net) { // net = espectro menos o fundo esperado
    const n = binned(net).map((v) => Math.max(0, v));
    const nn = Math.hypot(...n);
    if (!(nn > 0)) return "";
    let best = "", bs = -1;
    for (const [k, r] of Object.entries(REF_B)) {
      let d = 0; for (let i = 0; i < n.length; i++) d += n[i] * r[i];
      const sc = d / (nn * Math.hypot(...r));
      if (sc > bs) { bs = sc; best = k; }
    }
    return GUESS[best];
  }

  // ---------- utilidades de tela ----------
  const $ = (id) => document.getElementById(id);
  const nf = (v, d = 1) => Number(v).toLocaleString(LOCALE, { minimumFractionDigits: d, maximumFractionDigits: d });
  const css = (n) => getComputedStyle(document.documentElement).getPropertyValue(n).trim() || "#888";
  function fit(cv, h) {
    const dpr = window.devicePixelRatio || 1, w = cv.clientWidth;
    if (!w) return null;
    if (cv.width !== Math.round(w * dpr) || cv.height !== Math.round(h * dpr)) { cv.width = Math.round(w * dpr); cv.height = Math.round(h * dpr); }
    cv.style.height = h + "px";
    const g = cv.getContext("2d");
    g.setTransform(dpr, 0, 0, dpr, 0, 0);
    return { g, w, h };
  }
  function tip(el, html, x, y) {
    if (html == null) { el.hidden = true; return; }
    el.innerHTML = html;
    el.hidden = false;
    const p = el.parentElement.getBoundingClientRect(), r = el.getBoundingClientRect();
    el.style.left = Math.min(Math.max(4, x + 12), p.width - r.width - 4) + "px";
    el.style.top = Math.max(4, y - r.height - 10) + "px";
  }
  const esc = (s) => String(s).replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c]);
  const fmtT = (s) => { s = Math.round(s); const h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60), ss = s % 60; return (h ? h + ":" + String(m).padStart(2, "0") : m) + ":" + String(ss).padStart(2, "0"); };

  // =====================================================================
  // 1. espectro
  // =====================================================================
  const SCEN = {
    fundo: { name: "Fundo da casa", parts: ["fundo"] },
    torio: { name: "Eletrodo de tório", parts: ["fundo", "torio"] },
    fumaca: { name: "Detector de fumaça", parts: ["fundo", "fumaca"] },
    kcl: { name: "Sal light (500 g)", parts: ["fundo", "kcl"] },
    chuva: { name: "Chuva", parts: ["fundo", "chuva"], amp: { chuva: 3 } },
  };
  const sp = { scen: "torio", counts: new Float64Array(NCH), t: 0, log: false, bg: true, speed: 10, run: true, rate: null, hover: null };
  function spRate() {
    const s = SCEN[sp.scen], r = new Float64Array(NCH);
    for (const p of s.parts) { const a = (s.amp && s.amp[p]) || 1; for (let i = 0; i < NCH; i++) r[i] += SHAPE[p][i] * a; }
    sp.rate = r;
  }
  function spReset() { sp.counts = new Float64Array(NCH); sp.t = 0; spRate(); spDraw(); }
  function spStep(dt) {
    for (let i = 0; i < NCH; i++) sp.counts[i] += poisson(sp.rate[i] * dt);
    sp.t += dt;
  }
  // ---------- espectro ao vivo (Radiacode conectado) ----------
  // O aparelho junta o espectro dele o tempo todo; a tela mostra o que entrou
  // desde o "Recomeçar" (o atual menos a foto tirada naquela hora). O fundo
  // tracejado é o acumulado do aparelho (horas em casa), na escala do tempo.
  const rl = { on: false, mode: "agora", base: null, last: null, acc: null, accAt: 0, busy: false, peaks: [], accPeaks: [], peaksAt: 0, n: NCH };
  const rcLive = () => typeof state !== "undefined" && state.rc && state.rc.st && state.rc.st.state === "conectado" && typeof api !== "undefined" && api.GetRcSpectrum;
  // zerado no RadLog (o RC102 não deixa apagar o acumulado dele): guarda a
  // foto daquela hora e mostra só o que veio depois; se o acumulado do
  // aparelho voltou a zero por outro caminho, a foto não vale mais
  function accBaseGet() { try { return JSON.parse(localGet("rcAccBase", "null")); } catch (e) { return null; } }
  function accMinusBase(a) {
    const b = accBaseGet();
    if (!b || !b.counts || b.counts.length !== a.counts.length || a.seconds < b.seconds) { if (b) localSet("rcAccBase", "null"); return a; }
    const c = new Array(a.counts.length);
    for (let i = 0; i < c.length; i++) { c[i] = a.counts[i] - b.counts[i]; if (c[i] < 0) { localSet("rcAccBase", "null"); return a; } }
    return Object.assign({}, a, { counts: c, seconds: a.seconds - b.seconds });
  }
  async function accZero() {
    const a = rl.acc;
    const ok = await askConfirm({ title: "Zerar o espectro acumulado do RC102?", rows: a ? [["Acumulado agora", fmtT(a.seconds) + " · " + Math.round(a.counts.reduce((x, y) => x + y, 0)).toLocaleString(LOCALE) + " contagens"]] : null, note: "O RC102 não deixa apagar o acumulado dele, então o RadLog zera do lado dele: daqui pra frente mostra só o que vier depois. Ele também é o fundo da casa (a linha tracejada), que volta a se formar nas próximas horas.", ok: "Zerar o acumulado", danger: true });
    if (!ok) return;
    try {
      const raw = await api.GetRcSpectrum(true);
      await api.ResetRcSpecAccum();
      localSet("rcAccBase", JSON.stringify({ counts: raw.counts, seconds: raw.seconds }));
      toast("Zerado no RadLog (o RC102 não deixa apagar o dele)");
    } catch (e) { toast("Não deu para zerar: " + ((e && e.message) || e), true); return; }
    rl.acc = null; rl.accAt = 0; rl.accPeaks = []; pk.sel = null; pk.range = null; pk.win = null;
    spDraw(); realTick(true);
  }
  // "Recomeçar" zera o espectro da tela do RC102 (as contagens vão para o
  // acumulado do aparelho); a tela do app passa a mostrar o mesmo que a dele
  async function screenReset() {
    const ok = await askConfirm({ title: "Recomeçar o espectro da tela do RC102?", note: "O aparelho zera o espectro que aparece na tela dele, igual a segurar ▼. As contagens não se perdem: vão para o Acumulado do aparelho.", ok: "Recomeçar" });
    if (!ok) return;
    try { await api.ResetRcSpectrum(); } catch (e) { toast("Não deu para recomeçar: " + ((e && e.message) || e), true); return; }
    rl.base = null; rl.last = null; rl.peaks = []; rl.peaksAt = Date.now(); pk.sel = null; pk.range = null; pk.win = null;
    spDraw(); realTick(true);
  }
  function realCounts() {
    if (rl.mode === "acc") return rl.acc ? { c: rl.acc.counts, t: rl.acc.seconds } : null;
    if (!rl.last) return null;
    const b = rl.base, L = rl.last;
    let reset = !b || L.seconds < b.seconds || L.counts.length !== b.counts.length;
    const c = new Float64Array(L.counts.length);
    if (!reset) for (let i = 0; i < c.length; i++) { const d = L.counts[i] - b.counts[i]; if (d < 0) { reset = true; break; } c[i] = d; }
    if (reset) { rl.base = { counts: new Array(L.counts.length).fill(0), seconds: 0 }; return { c: Float64Array.from(L.counts), t: L.seconds }; }
    return { c, t: Math.max(L.seconds - b.seconds, 0) };
  }
  async function realTick(force) {
    if (!rcLive()) { if (rl.on) { rl.on = false; EN = SIM_EN; DE = SIM_DE; spMark(); spDraw(); } return; }
    if (!rl.on) { rl.on = true; rl.base = null; spMark(); }
    if ((!sp.run && !force) || rl.busy) return;
    // pelo Bluetooth a leitura do espectro é mais lenta: lê a cada 6 s
    const every = state.rc.st.via === "bluetooth" ? 6000 : 2000;
    if (!force && rl.readAt && Date.now() - rl.readAt < every - 200) return;
    rl.readAt = Date.now();
    rl.busy = true;
    try {
      const s = await api.GetRcSpectrum(false);
      if (s && s.counts && s.counts.length) {
        rl.last = s;
        if (rl.n !== s.counts.length || !rl.cal || rl.cal.join() !== s.a.join()) { rl.n = s.counts.length; rl.cal = s.a; EN = calEN(s.a, rl.n); DE = Float64Array.from(EN, (e, i) => (i + 1 < rl.n ? EN[i + 1] - e : e - EN[i - 1])); }
      }
      // no modo "Acumulado do aparelho" relê a cada 30 s (o tempo anda e um reset
      // no aparelho aparece logo); no outro modo ele é só o fundo, a cada 10 min
      if (!rl.acc || Date.now() - rl.accAt > (rl.mode === "acc" ? 30e3 : 10 * 60e3)) {
        let a = await api.GetRcSpectrum(true);
        if (a && a.counts && a.counts.length) a = accMinusBase(a);
        if (a && a.counts && a.counts.length) {
          rl.acc = a; rl.accAt = Date.now();
          // isótopos do acumulado: picos acima do contínuo (sem fundo para tirar)
          rl.accPeaks = api.AnalyzeSpectrum && a.seconds > 60 ? (await api.AnalyzeSpectrum({ a: rl.cal || a.a, obj: Array.from(a.counts, Math.round), objSec: a.seconds, bg: new Array(a.counts.length).fill(0), bgSec: 1 })) || [] : [];
        }
      }
      // picos (no Go, igual à medição de objeto), a cada ~10 s
      const rc = realCounts();
      if (rc && rl.acc && rl.mode === "agora" && rc.t > 5 && Date.now() - rl.peaksAt > 10e3 && api.AnalyzeSpectrum) {
        rl.peaksAt = Date.now();
        rl.peaks = (await api.AnalyzeSpectrum({ a: rl.cal, obj: Array.from(rc.c, Math.round), objSec: rc.t, bg: rl.acc.counts, bgSec: rl.acc.seconds })) || [];
      }
    } catch (e) { /* aparelho ocupado: tenta no próximo */ }
    rl.busy = false;
    spDraw();
  }
  const GROUP_SHORT = { "Tório-232 (série)": "~Th-232 (tório)", "Rádio-226 (série do urânio)": "~Ra-226 (urânio/radônio)", "Urânio-235": "~U-235",
    "Amerício-241": "~Am-241", "Césio-137": "~Cs-137", "Cobalto-60": "~Co-60", "Potássio-40": "~K-40 (potássio)" };

  function spDraw() {
    const cv = $("rcSpec"); if (!cv) return;
    const real = rl.on ? realCounts() : null;
    const counts = real ? real.c : sp.counts, tNow = real ? real.t : sp.t;
    const bgRate = real && rl.acc && rl.acc.seconds > 0 && rl.mode === "agora" ? Float64Array.from(rl.acc.counts, (v) => v / rl.acc.seconds) : null;
    const nch = counts.length;
    const f = fit(cv, 260); if (!f) return;
    const { g, w, h } = f, L = 44, R = 10, T = 10, B = 24, pw = w - L - R, ph = h - T - B;
    g.clearRect(0, 0, w, h);
    const EMAX = 3000, xOf = (e) => L + (Math.max(0, e) / EMAX) * pw;
    let max = 1; for (let i = 0; i < nch; i++) if (EN[i] > 25 && counts[i] > max) max = counts[i];
    const yOf = sp.log ? (v) => T + ph - (Math.log10(1 + v) / Math.log10(1 + max * 1.15)) * ph : (v) => T + ph - (v / (max * 1.1)) * ph;
    // grade
    g.strokeStyle = css("--border"); g.lineWidth = 1; g.fillStyle = css("--dim"); g.font = "11px " + css("--mono");
    g.textAlign = "center";
    for (let e = 0; e <= EMAX; e += 500) { const x = xOf(e); g.beginPath(); g.moveTo(x + 0.5, T); g.lineTo(x + 0.5, T + ph); g.stroke(); g.textAlign = e === EMAX ? "right" : "center"; g.fillText(e === 0 ? "0" : e + "", e === EMAX ? x + 6 : x, h - 7); }
    g.textAlign = "right";
    const ticks = sp.log ? [0, 1, 10, 100, 1000, 10000, 100000].filter((v) => v <= max * 1.15) : [0, max * 0.25, max * 0.5, max * 0.75, max].map(Math.round);
    for (const v of ticks) { const y = yOf(v); g.globalAlpha = 0.5; g.beginPath(); g.moveTo(L, y + 0.5); g.lineTo(L + pw, y + 0.5); g.stroke(); g.globalAlpha = 1; g.fillText(v >= 1000 ? nf(v / 1000, v >= 10000 ? 0 : 1) + "k" : String(v), L - 6, y + 4); }
    // fundo esperado no mesmo tempo
    if (sp.bg && (real ? bgRate : sp.scen !== "fundo") && tNow > 0) {
      g.beginPath();
      for (let i = 0; i < nch; i++) { if (EN[i] > EMAX) break; const x = xOf(EN[i]), y = yOf((real ? bgRate[i] : SHAPE.fundo[i]) * tNow); i ? g.lineTo(x, y) : g.moveTo(x, y); }
      g.strokeStyle = css("--muted"); g.setLineDash([4, 3]); g.stroke(); g.setLineDash([]);
    }
    // espectro
    const acc = "#F2A33A";
    g.beginPath(); g.moveTo(xOf(EN[0]), T + ph);
    for (let i = 0; i < nch; i++) { if (EN[i] > EMAX) break; g.lineTo(xOf(EN[i]), yOf(counts[i])); }
    g.lineTo(xOf(Math.min(EMAX, EN[nch - 1])), T + ph); g.closePath();
    g.fillStyle = "rgba(242,163,58,.55)"; g.fill(); g.strokeStyle = acc; g.lineWidth = 1; g.stroke();
    // trecho do zoom (ou o que está sendo arrastado)
    const win = pk.drag ? [Math.min(pk.drag[0], pk.drag[1]), Math.max(pk.drag[0], pk.drag[1])] : pk.win;
    if (win) {
      const x1 = xOf(win[0]), x2 = xOf(win[1]);
      g.fillStyle = "rgba(255,255,255,.06)"; g.fillRect(x1, T, x2 - x1, ph);
      g.strokeStyle = "rgba(255,255,255,.35)"; g.setLineDash([3, 3]); g.strokeRect(x1 + 0.5, T + 0.5, x2 - x1, ph); g.setLineDash([]);
    }
    // isótopos achados: um risquinho em cada linha e o nome em cima da mais forte
    const peaks = real ? curPeaks() : [];
    if (peaks.length) {
      const chOf = (e) => { let c = 0; for (let i = 0; i < nch; i++) if (Math.abs(EN[i] - e) < Math.abs(EN[c] - e)) c = i; return c; };
      const top = (e) => { const c = chOf(e); let m = 0; for (let i = Math.max(0, c - 3); i <= Math.min(nch - 1, c + 3); i++) m = Math.max(m, counts[i]); return Math.max(T + 14, yOf(m) - 6); };
      g.font = "11px " + css("--mono"); g.textAlign = "center";
      const best = {};
      for (const p of peaks) if (!best[p.nuclide] || p.sigma > best[p.nuclide].sigma) best[p.nuclide] = p;
      for (const p of peaks) {
        if (p.kev > EMAX) continue;
        const on = pk.sel === p.nuclide, x = xOf(p.kev), y = top(p.kev);
        g.strokeStyle = on ? COR.text : COR.muted; g.lineWidth = on ? 2 : 1;
        g.beginPath(); g.moveTo(x + 0.5, y); g.lineTo(x + 0.5, y - 8); g.stroke(); g.lineWidth = 1;
        if (best[p.nuclide] === p) { g.fillStyle = on ? COR.text : COR.muted; g.fillText(p.nuclide, Math.min(Math.max(x, L + 24), L + pw - 24), y - 12); }
      }
    }
    // cursor
    if (sp.hover != null) {
      const x = xOf(sp.hover); g.strokeStyle = COR.away; g.beginPath(); g.moveTo(x + 0.5, T); g.lineTo(x + 0.5, T + ph); g.stroke();
    }
    g.fillStyle = css("--dim"); g.textAlign = "right"; g.fillText(LANG === "en" ? "energy (keV)" : "energia (keV)", L + pw, T + 12);
    // números
    if (real) {
      const tot = counts.reduce((a, b) => a + b, 0), cps = tNow ? tot / tNow : 0, err = tot ? 100 / Math.sqrt(tot) : 0;
      const st = state.rc.st || {};
      let txt = rl.mode === "acc"
        ? `acumulado do aparelho · ${fmtT(tNow)} · ${Math.round(tot).toLocaleString(LOCALE)} contagens · ${nf(cps, 2)} cps`
        : `${fmtT(tNow)} acumulados · ${Math.round(tot).toLocaleString(LOCALE)} contagens · ${nf(cps, 2)} cps ±${nf(err, 1)}% · ≈ ${nf(st.usvh || 0, 3)} µSv/h agora`;
      if (rl.mode === "agora" && bgRate && tNow > 0) {
        const bgT = Float64Array.from(bgRate, (v) => v * tNow);
        const hr = (a) => { let hi = 0, lo = 0; for (let i = 0; i < a.length; i++) { if (EN[i] >= 1500 && EN[i] < 3100) hi += a[i]; else if (EN[i] >= 150 && EN[i] < 1500) lo += a[i]; } return hi / Math.max(1e-9, lo); };
        const bgTot = bgT.reduce((a, b) => a + b, 0), net = tot - bgTot, nerr = Math.sqrt(tot + bgTot * tNow / Math.max(1, rl.acc.seconds));
        txt += ` · dureza ${nf(hr(counts) / Math.max(1e-9, hr(bgT)), 2)}`;
        const groups = [...new Set(rl.peaks.map((p) => p.group))];
        if (groups.length) txt += ` · parece: <b>${groups.map((g) => esc(GROUP_SHORT[g] || g)).join(", ")}</b>`;
        else if (net > 3 * nerr) txt += ` · acima do fundo (${nf(net / tNow, 2)} cps a mais), sem linha conhecida ainda`;
        else txt += tNow < 60 ? " · juntando contagens…" : " · igual ao fundo";
      }
      $("rcSpecInfo").innerHTML = txt;
      isoRender(peaks);
      zoomDraw(counts);
      return;
    }
    isoRender([]);
    zoomDraw(sp.counts);
    // números
    const tot = sp.counts.reduce((a, b) => a + b, 0), cps = sp.t ? tot / sp.t : 0, err = tot ? 100 / Math.sqrt(tot) : 0;
    let net = "";
    if (sp.scen !== "fundo" && sp.t > 0) {
      const n = Float64Array.from(sp.counts, (v, i) => v - SHAPE.fundo[i] * sp.t);
      const nt = n.reduce((a, b) => a + b, 0), nerr = Math.sqrt(tot + SHAPE.fundo.reduce((a, b) => a + b, 0) * sp.t);
      net = ` · dureza ${nf(hardRel(sp.counts), 2)}` + (nt > 3 * nerr ? ` · parece: <b>${guessFrom(n)}</b>` : " · ainda não dá para separar do fundo");
    } else if (sp.t > 0) net = ` · dureza ${nf(hardRel(sp.counts), 2)} (1,00 = fundo)`;
    $("rcSpecInfo").innerHTML = `${fmtT(sp.t)} acumulados · ${Math.round(tot).toLocaleString(LOCALE)} contagens · ${nf(cps, 2)} cps ±${nf(err, 1)}% · ≈ ${nf(cps / CPS_PER_USV, 3)} µSv/h${net}`;
  }
  function spHover(ev) {
    const cv = $("rcSpec"), r = cv.getBoundingClientRect(), L = 44, R = 10, pw = r.width - L - R;
    const x = ev.clientX - r.left;
    if (pk.drag) { pk.drag[1] = Math.min(3000, Math.max(0, ((x - L) / pw) * 3000)); }
    if (x < L || x > L + pw) { sp.hover = null; tip($("rcSpecTip"), null); spDraw(); return; }
    const e = ((x - L) / pw) * 3000;
    sp.hover = e;
    let ch = 0; for (let i = 0; i < EN.length; i++) if (Math.abs(EN[i] - e) < Math.abs(EN[ch] - e)) ch = i;
    const rc = rl.on ? realCounts() : null, cnt = rc ? rc.c : sp.counts;
    const near = libNear(e);
    const lib = near.length ? near.map(([le, n, d]) => `<b>${esc(n)}</b> ${nf(le, 1)} keV${d ? ` <span class="muted">(${esc(d)})</span>` : ""}`).join("<br>") : '<span class="muted">nenhum pico conhecido aqui</span>';
    tip($("rcSpecTip"), `<span class="mono">${nf(e, 0)} keV · canal ${ch} · ${Math.round(cnt[ch] || 0)} contagens</span><br>${lib}`, x, ev.clientY - r.top);
    spDraw();
  }

  // ---------- isótopos mais detectados e zoom na crista (2.52) ----------
  // Os picos vêm do Go (AnalyzeSpectrum): ao vivo, comparados com o fundo da
  // casa; no acumulado do aparelho, acima do contínuo. Cada isótopo vira uma
  // etiqueta com as linhas achadas e um selo de certeza; clicar (ou arrastar
  // no espectro) abre o zoom: os canais do trecho, a linha de base, a
  // gaussiana ajustada e a crista.
  const pk = { sel: null, win: null, range: null, drag: null };
  const curPeaks = () => (rl.on ? (rl.mode === "acc" ? rl.accPeaks : rl.peaks) || [] : []);
  const SURE = (sg) => (sg >= 10 ? ["certo", "sure"] : sg >= 5 ? ["provável", "likely"] : ["fraco", "weak"]);
  function isoList(peaks) {
    const by = {};
    for (const p of peaks) {
      const o = by[p.nuclide] || (by[p.nuclide] = { nuclide: p.nuclide, group: p.group, lines: [], sigma: 0 });
      o.lines.push(p);
      o.sigma = Math.max(o.sigma, p.sigma);
    }
    const out = Object.values(by).sort((a, b) => b.sigma - a.sigma);
    for (const o of out) o.lines.sort((a, b) => b.sigma - a.sigma);
    return out;
  }
  function isoRender(peaks) {
    const box = $("rcIsos"), list = isoList(peaks);
    if (pk.sel && !list.some((o) => o.nuclide === pk.sel)) { pk.sel = null; if (!pk.range) pk.win = null; }
    const key = list.map((o) => o.nuclide + ":" + SURE(o.sigma)[1] + ":" + o.lines.map((l) => l.kev).join(",")).join("|") + "#" + pk.sel;
    box.hidden = !list.length;
    // sem isótopo achado, a dica só fala do arrastar
    const hint = $("rcDragHint");
    if (hint) hint.textContent = list.length ? "Clique num isótopo ou arraste no espectro para ver um trecho de perto." : "Arraste no espectro para ver um trecho de perto.";
    if (box.dataset.k === key) return;
    box.dataset.k = key;
    box.textContent = "";
    for (const o of list) {
      const b = document.createElement("button");
      b.type = "button";
      b.className = "rc-iso" + (pk.sel === o.nuclide ? " on" : "");
      b.dataset.nuc = o.nuclide;
      b.setAttribute("aria-pressed", String(pk.sel === o.nuclide));
      const [pt, cls] = SURE(o.sigma);
      b.innerHTML = `<span class="rc-iso-n mono" data-no-tr>${esc(o.nuclide)}</span><span class="rc-iso-s ${cls}">${pt}</span>` +
        `<span class="rc-iso-e mono" data-no-tr>${o.lines.map((l) => nf(l.kev, 0)).join(" · ")} keV</span>`;
      b.title = `${o.group} · ${nf(o.sigma, 0)} σ`;
      box.appendChild(b);
    }
  }
  function isoPick(nuc) {
    if (pk.sel === nuc) { zoomClose(); return; }
    const o = isoList(curPeaks()).find((x) => x.nuclide === nuc);
    if (!o) return;
    const e = o.lines[0].kev, w = 2.5 * fwhm(e);
    pk.sel = nuc; pk.range = null; pk.win = [Math.max(15, e - w), e + w];
    spDraw();
  }
  function zoomClose() { pk.sel = null; pk.range = null; pk.win = null; $("rcZoom").hidden = true; spDraw(); }
  // canal fracionário → energia
  const enAt = (x) => { const i = Math.max(0, Math.min(EN.length - 2, Math.floor(x))), f = x - i; return EN[i] + (EN[i + 1] - EN[i]) * f; };
  // ajusta base linear + gaussiana nos canais do trecho
  function fitPeak(counts, e0, e1, hint) {
    const idx = [];
    for (let i = 0; i < counts.length; i++) if (EN[i] >= e0 && EN[i] <= e1) idx.push(i);
    if (idx.length < 8) return null;
    const c = idx.map((i) => counts[i]), n = c.length, k = Math.max(2, Math.min(4, Math.floor(n / 8)));
    const avg = (a) => a.reduce((x, y) => x + y, 0) / a.length;
    const bl = avg(c.slice(0, k)), br = avg(c.slice(n - k)), base = (j) => bl + ((br - bl) * (j - (k - 1) / 2)) / Math.max(1, n - 1 - (k - 1));
    const net = c.map((v, j) => v - base(j));
    // começo: crista no canal mais alto (ou perto da linha procurada)
    let m = 0;
    if (hint != null) { let d = 1e9; idx.forEach((i, j) => { if (Math.abs(EN[i] - hint) < d) { d = Math.abs(EN[i] - hint); m = j; } }); for (let j = Math.max(0, m - 3); j <= Math.min(n - 1, m + 3); j++) if (net[j] > net[m]) m = j; }
    else for (let j = 1; j < n; j++) if (net[j] > net[m]) m = j;
    const dE = (EN[idx[n - 1]] - EN[idx[0]]) / Math.max(1, n - 1);
    let A = Math.max(1, net[m]), mu = m, s = Math.max(0.8, sig(EN[idx[m]]) / dE);
    // Gauss-Newton em (A, µ, s), pesos 1/contagem
    for (let it = 0; it < 12; it++) {
      let JTJ = [[0, 0, 0], [0, 0, 0], [0, 0, 0]], JTr = [0, 0, 0];
      for (let j = 0; j < n; j++) {
        const z = (j - mu) / s, e = Math.exp(-0.5 * z * z), f = A * e, wgt = 1 / Math.max(1, c[j]);
        const J = [e, (f * z) / s, (f * z * z) / s], r = net[j] - f;
        for (let a = 0; a < 3; a++) { JTr[a] += wgt * J[a] * r; for (let b = 0; b < 3; b++) JTJ[a][b] += wgt * J[a] * J[b]; }
      }
      for (let a = 0; a < 3; a++) JTJ[a][a] *= 1.001;
      const d = solve3(JTJ, JTr);
      if (!d) break;
      A += d[0]; mu += Math.max(-2, Math.min(2, d[1])); s += Math.max(-s / 2, Math.min(s, d[2]));
      if (!(A > 0) || !(s > 0.3) || mu < 0 || mu > n - 1) return { idx, c, base, net, ok: false };
    }
    const muCh = idx[0] + mu, dEmu = enAt(muCh + 0.5) - enAt(muCh - 0.5), crest = enAt(muCh);
    const sKeV = s * dEmu, fw = 2.355 * sKeV, area = A * s * Math.sqrt(2 * Math.PI);
    let tot = 0, bsum = 0;
    for (let j = 0; j < n; j++) if (Math.abs(j - mu) <= 1.27 * s * 2.355 / 2 * 1.2) { tot += c[j]; bsum += base(j); }
    const unc = Math.sqrt(Math.max(1, tot + bsum * (2 / k)));
    return { idx, c, base, net, ok: true, A, mu, s, crest, fw, res: (fw / crest) * 100, area, unc, sig: area / unc };
  }
  function solve3(M, v) {
    const a = M.map((r, i) => [...r, v[i]]);
    for (let i = 0; i < 3; i++) {
      let p = i; for (let r = i + 1; r < 3; r++) if (Math.abs(a[r][i]) > Math.abs(a[p][i])) p = r;
      if (Math.abs(a[p][i]) < 1e-12) return null;
      [a[i], a[p]] = [a[p], a[i]];
      for (let r = 0; r < 3; r++) if (r !== i) { const f = a[r][i] / a[i][i]; for (let k = i; k < 4; k++) a[r][k] -= f * a[i][k]; }
    }
    return [a[0][3] / a[0][0], a[1][3] / a[1][1], a[2][3] / a[2][2]];
  }
  function zoomDraw(counts) {
    const box = $("rcZoom");
    if (!pk.win || !counts) { box.hidden = true; return; }
    const sel = pk.sel ? isoList(curPeaks()).find((o) => o.nuclide === pk.sel) : null;
    const hint = sel ? sel.lines[0].kev : null;
    const f = fitPeak(counts, pk.win[0], pk.win[1], hint);
    box.hidden = false;
    const lib = sel ? LIB.find((l) => Math.abs(l[0] - hint) < 3) : null;
    $("rcZoomTitle").textContent = sel ? `${tr("Zoom no pico")} · ${sel.nuclide}` : `${tr("Zoom no trecho")} · ${nf(pk.win[0], 0)}–${nf(pk.win[1], 0)} keV`;
    $("rcZoomSub").textContent = lib && lib[2] ? tr(lib[2]) : "";
    const cv = $("rcZoomCv"), F = fit(cv, 220);
    const st = $("rcZoomStats"), warn = $("rcZoomWarn");
    if (!F) return;
    const { g, w, h } = F, L = 10, R = 10, T = 26, B = 22, pw = w - L - R, ph = h - T - B;
    g.clearRect(0, 0, w, h);
    if (!f) { g.fillStyle = css("--dim"); g.font = "12px " + css("--sans"); g.textAlign = "center"; g.fillText(tr("Trecho estreito demais: arraste uma faixa maior."), w / 2, h / 2); st.innerHTML = ""; warn.hidden = true; return; }
    const n = f.c.length, bw = pw / n, xOf = (j) => L + (j + 0.5) * bw;
    // eixo começa um pouco abaixo do menor canal: o pico aparece, não o contínuo
    let max = 1, min = Infinity; for (const v of f.c) { max = Math.max(max, v); min = Math.min(min, v); }
    if (f.ok) max = Math.max(max, f.base(f.mu) + f.A);
    max *= 1.08;
    const lo = Math.max(0, min - (max - min) * 0.25);
    const yOf = (v) => T + ph - (Math.max(0, v - lo) / (max - lo)) * ph;
    // canais
    g.fillStyle = "rgba(242,163,58,.38)";
    for (let j = 0; j < n; j++) { const y = yOf(f.c[j]); g.fillRect(L + j * bw + 0.5, y, Math.max(1, bw - 1), T + ph - y); }
    // linha de base
    g.strokeStyle = css("--muted"); g.setLineDash([5, 4]); g.beginPath(); g.moveTo(xOf(0), yOf(f.base(0))); g.lineTo(xOf(n - 1), yOf(f.base(n - 1))); g.stroke(); g.setLineDash([]);
    // eixo de energia
    g.fillStyle = css("--dim"); g.font = "11px " + css("--mono");
    [[0, "left"], [Math.floor((n - 1) / 2), "center"], [n - 1, "right"]].forEach(([j, al]) => { g.textAlign = al; g.fillText(nf(EN[f.idx[j]], 0) + (j === n - 1 ? " keV" : ""), al === "left" ? L : al === "right" ? L + pw : xOf(j), h - 6); });
    // picos vizinhos dentro do trecho
    const near = curPeaks().filter((p) => p.kev > pk.win[0] && p.kev < pk.win[1] && (!f.ok || Math.abs(p.kev - f.crest) > 0.45 * f.fw));
    g.textAlign = "center"; g.fillStyle = css("--muted"); g.font = "11px " + css("--sans");
    for (const p of near) { let j = 0; f.idx.forEach((i, q) => { if (Math.abs(EN[i] - p.kev) < Math.abs(EN[f.idx[j]] - p.kev)) j = q; }); g.fillText(`${tr("outro pico")}: ${p.nuclide} ${nf(p.kev, 0)} keV`, Math.min(Math.max(xOf(j), L + 80), L + pw - 80), T + 4); }
    if (f.ok) {
      // curva ajustada
      g.strokeStyle = COR.text; g.lineWidth = 2; g.beginPath();
      for (let q = 0; q <= 200; q++) { const j = (q / 200) * (n - 1), z = (j - f.mu) / f.s, v = f.base(j) + f.A * Math.exp(-0.5 * z * z); q ? g.lineTo(xOf(j), yOf(v)) : g.moveTo(xOf(j), yOf(v)); }
      g.stroke(); g.lineWidth = 1;
      // crista e FWHM
      const xc = xOf(f.mu), yc = yOf(f.base(f.mu) + f.A), yh = yOf(f.base(f.mu) + f.A / 2), hw = 1.1774 * f.s * bw;
      g.setLineDash([2, 3]); g.strokeStyle = COR.text; g.beginPath(); g.moveTo(xc + 0.5, yc); g.lineTo(xc + 0.5, T + ph); g.stroke(); g.setLineDash([]);
      g.fillStyle = COR.text; g.beginPath(); g.arc(xc, yc, 3.5, 0, 7); g.fill();
      g.font = "12px " + css("--mono"); g.textAlign = xc > L + pw - 120 ? "right" : "left"; g.fillText(`${tr("crista")} ${nf(f.crest, 1)} keV`, xc + (g.textAlign === "right" ? -8 : 8), Math.max(T - 8, yc - 8));
      g.strokeStyle = COR.att; g.lineWidth = 2; g.beginPath(); g.moveTo(xc - hw, yh); g.lineTo(xc + hw, yh); g.stroke(); g.lineWidth = 1;
      g.fillStyle = COR.att; g.font = "11px " + css("--mono"); g.textAlign = "left"; g.fillText("FWHM", Math.min(xc + hw + 6, L + pw - 34), yh + 4);
      const row = (k, v) => `<dt>${tr(k)}</dt><dd class="mono" data-no-tr>${v}</dd>`;
      st.innerHTML = row("Crista", `${nf(f.crest, 1)} keV`) + row("Largura (FWHM)", `${nf(f.fw, 1)} keV · ${tr("resolução")} ${nf(f.res, 1)}%`) +
        row("Contagens do pico", `${Math.round(f.area).toLocaleString(LOCALE)} ± ${Math.round(f.unc).toLocaleString(LOCALE)}`) + row("Acima do fundo", `${nf(f.sig, f.sig < 10 ? 1 : 0)} σ`);
    } else st.innerHTML = `<dt>${tr("Sem pico claro neste trecho")}</dt>`;
    warn.hidden = !near.length || !f.ok;
    if (!warn.hidden) warn.textContent = tr("Tem outro pico perto: a largura e as contagens podem sair maiores que as de verdade.");
  }

  // ---------- lupa no espectrograma (2.52) ----------
  // um quadrado de N×N pixels em volta do cursor, ampliado, com as contagens
  // de cada pixel; embaixo, o tempo e a faixa de energia do pixel do meio
  const lp = { n: (() => { try { const v = localStorage.getItem("rcLoupe"); return v === "0" || v === "4" ? +v : 5; } catch (e) { return 5; } })() };
  function loupeHtml(cell, rows, cols, y, x, lm) {
    const N = lp.n, h0 = Math.floor((N - 1) / 2);
    let html = `<div class="rc-loupe" style="grid-template-columns:repeat(${N},1fr)">`;
    for (let dy = -h0; dy < N - h0; dy++) for (let dx = -h0; dx < N - h0; dx++) {
      const yy = y + dy, xx = x + dx, v = yy >= 0 && yy < rows && xx >= 0 && xx < cols ? cell(yy, xx) : null;
      const mid = dy === 0 && dx === 0;
      if (v == null) { html += `<i class="${mid ? "mid " : ""}none"></i>`; continue; }
      const [r, gg, b] = cmap(Math.log1p(v.rate) / lm), dark = r * 0.3 + gg * 0.59 + b * 0.11 > 140;
      html += `<i class="${mid ? "mid" : ""}" style="background:rgb(${r},${gg},${b});color:${dark ? "#0D1012" : "#E9EDEF"}">${Math.round(v.n)}</i>`;
    }
    return html + "</div>";
  }
  // =====================================================================
  const TL_N = 180, TL_DT = 60; // 180 linhas de 1 minuto
  const RAIN = [30, 80], ELEC = [118, 135];
  const tl = { rows: [], cps: [], hard: [], label: [], shown: 0, run: true, max: 1, hover: null };
  function tlRain(t) {
    if (t < RAIN[0]) return 0;
    const A = 7, up = (x) => A * (1 - Math.exp(-(x - RAIN[0]) / 25));
    if (t < RAIN[1]) return up(t);
    return up(RAIN[1]) * Math.exp(-((t - RAIN[1]) * Math.LN2) / 40);
  }
  function tlElec(t) {
    if (t < ELEC[0] || t > ELEC[1]) return 0;
    if (t < ELEC[0] + 2) return (t - ELEC[0]) / 2;
    if (t > ELEC[1] - 2) return (ELEC[1] - t) / 2;
    return 1;
  }
  function tlBuild() {
    tl.rows = []; tl.cps = []; tl.hard = []; tl.label = [];
    const all = [];
    for (let t = 0; t < TL_N; t++) {
      const r = tlRain(t), q = tlElec(t), row = new Uint16Array(NCH);
      let tot = 0;
      const net = new Float64Array(NCH);
      for (let i = 0; i < NCH; i++) {
        const lam = (SHAPE.fundo[i] + SHAPE.chuva[i] * r + SHAPE.torio[i] * q) * TL_DT;
        const c = poisson(lam); row[i] = c; tot += c; net[i] = c - SHAPE.fundo[i] * TL_DT;
      }
      tl.rows.push(row); tl.cps.push(tot / TL_DT);
      for (let i = 0; i < NCH; i++) if (EN[i] <= 1500) all.push(row[i]);
    }
    // dureza e palpite com a soma de 20 minutos em volta (1 minuto tem poucos
    // fótons acima de 1.500 keV: sozinho, o número pula demais)
    const bgTot = SHAPE.fundo.reduce((a, b) => a + b, 0);
    for (let t = 0; t < TL_N; t++) {
      const a = Math.max(0, t - 10), b = Math.min(TL_N - 1, t + 10), sum = new Float64Array(NCH);
      for (let k = a; k <= b; k++) for (let i = 0; i < NCH; i++) sum[i] += tl.rows[k][i];
      tl.hard.push(hardRel(sum));
      const n = b - a + 1, net = Float64Array.from(sum, (v, i) => v - SHAPE.fundo[i] * TL_DT * n);
      const tot = sum.reduce((x, y) => x + y, 0), netT = tot - bgTot * TL_DT * n;
      tl.label.push(netT > 5 * Math.sqrt(tot) ? guessFrom(net) : "");
    }
    all.sort((a, b) => a - b);
    tl.max = Math.max(3, all[Math.floor(all.length * 0.997)]);
    tl.shown = 0;
  }
  const CMAP = [[0, [6, 6, 26]], [0.15, [26, 26, 168]], [0.35, [31, 111, 224]], [0.5, [25, 195, 214]], [0.65, [78, 227, 106]], [0.78, [242, 227, 58]], [0.9, [242, 139, 43]], [1, [232, 49, 122]]];
  function cmap(v) {
    v = Math.max(0, Math.min(1, v));
    for (let k = 1; k < CMAP.length; k++) if (v <= CMAP[k][0]) {
      const [a, ca] = CMAP[k - 1], [b, cb] = CMAP[k], u = (v - a) / (b - a);
      return ca.map((c, j) => Math.round(c + (cb[j] - c) * u));
    }
    return CMAP[CMAP.length - 1][1];
  }
  const SG_EMAX = 1500;
  const SG_COLS = (() => { let n = 0; for (let i = 0; i < NCH; i++) if (EN[i] <= SG_EMAX) n = i + 1; return n; })();
  function sgDraw() {
    const cv = $("rcSgram"); if (!cv) return;
    if (sg.on) { sgDrawReal(cv); return; }
    const W = Math.ceil(SG_COLS / 2);
    if (cv.width !== W || cv.height !== TL_N) { cv.width = W; cv.height = TL_N; }
    const g = cv.getContext("2d"), img = g.createImageData(W, TL_N), lm = Math.log1p(tl.max);
    for (let y = 0; y < TL_N; y++) for (let x = 0; x < W; x++) {
      const o = (y * W + x) * 4;
      if (y >= tl.shown) { img.data[o] = 13; img.data[o + 1] = 16; img.data[o + 2] = 18; img.data[o + 3] = 255; continue; }
      const c = (tl.rows[y][2 * x] || 0) + (tl.rows[y][2 * x + 1] || 0);
      const [r, gg, b] = cmap(Math.log1p(c / 2) / lm);
      img.data[o] = r; img.data[o + 1] = gg; img.data[o + 2] = b; img.data[o + 3] = 255;
    }
    g.putImageData(img, 0, 0);
    if (tl.hover != null && tl.hover < tl.shown) { g.fillStyle = "rgba(255,255,255,.55)"; g.fillRect(0, tl.hover, W, 1); }
  }
  function tlLabel(t) {
    const ev = [];
    if (t >= RAIN[0] && t < RAIN[1]) ev.push("chovendo");
    else if (t >= RAIN[1] && tlRain(t) > 0.3) ev.push("depois da chuva");
    if (tlElec(t) > 0) ev.push("eletrodo perto");
    return ev.join(" · ");
  }
  function sgHover(ev) {
    if (sg.on) { sgHoverReal(ev); return; }
    const cv = $("rcSgram"), r = cv.getBoundingClientRect();
    const t = Math.floor(((ev.clientY - r.top) / r.height) * TL_N), e = ((ev.clientX - r.left) / r.width) * SG_EMAX;
    if (t < 0 || t >= tl.shown) { tl.hover = null; tip($("rcSgTip"), null); sgDraw(); return; }
    tl.hover = t;
    const ev2 = tlLabel(t), near = libNear(e);
    let lh = "";
    if (lp.n) {
      const W = Math.ceil(SG_COLS / 2), x = Math.min(W - 1, Math.floor(((ev.clientX - r.left) / r.width) * W));
      const cell = (yy, xx) => (yy < tl.shown ? { rate: ((tl.rows[yy][2 * xx] || 0) + (tl.rows[yy][2 * xx + 1] || 0)) / 2, n: (tl.rows[yy][2 * xx] || 0) + (tl.rows[yy][2 * xx + 1] || 0) } : null);
      lh = loupeHtml(cell, TL_N, W, t, x, Math.log1p(tl.max)) + `<span class="mono">${nf(EN[2 * x], 0)}–${nf(EN[Math.min(NCH - 1, 2 * x + 2)], 0)} keV · ${tr("pixel do meio")}</span><br>`;
    }
    tip($("rcSgTip"), `${lh}<span class="mono">min ${t} · ${nf(tl.cps[t], 1)} cps · ${nf(e, 0)} keV</span>${ev2 ? `<br>${ev2}` : ""}${near.length ? `<br><b>${esc(near[0][1])}</b> ${nf(near[0][0], 1)} keV` : ""}`, ev.clientX - r.left, ev.clientY - r.top);
    sgDraw(); hdDraw();
  }

  // ---------- espectrograma de verdade (o que o app gravou do aparelho) ----------
  // Com o Radiacode conectado, o app guarda o espectro de cada minuto. Cada
  // linha aqui junta um pedaço do período (1 min em 3 h, 30 min em 7 dias);
  // a mais recente fica embaixo.
  const SG_PER = { "3h": [3 * 3600, 60], "12h": [12 * 3600, 120], "24h": [24 * 3600, 300], "7d": [7 * 86400, 1800] };
  const sg = { on: false, data: null, hd: null, guess: {}, period: "3h", emax: 1500, busy: false, at: 0, hover: null, max: 1, want: 0 };
  const pad2 = (v) => String(v).padStart(2, "0");
  const hhmm = (ms) => { const d = new Date(ms); return pad2(d.getHours()) + ":" + pad2(d.getMinutes()); };
  const ddmm = (ms) => { const d = new Date(ms); return pad2(d.getDate()) + "/" + pad2(d.getMonth() + 1); };
  async function sgLoad() {
    if (typeof api === "undefined" || !api.GetRcSpectrogram || sg.busy) return;
    const [span, rowSec] = SG_PER[sg.period], rowMs = rowSec * 1000;
    const to = Math.ceil(Date.now() / rowMs) * rowMs, from = to - span * 1000;
    const want = ++sg.want;
    sg.busy = true;
    let d = null, hd = null;
    try {
      [d, hd] = await Promise.all([api.GetRcSpectrogram(from, to, rowSec, sg.emax), api.GetRcHardness ? api.GetRcHardness(from, to, rowSec).catch(() => null) : null]);
    } catch (e) { d = null; }
    sg.busy = false;
    if (want !== sg.want) return;
    if (hd && (!sg.hd || sg.hd.from !== hd.from || sg.hd.rowSec !== hd.rowSec)) sg.guess = {};
    sg.hd = hd && hd.cps ? hd : null;
    const on = !!(d && d.rows && (d.first > 0 || rcLive()));
    if (on) {
      sg.data = d;
      // escala de cor: quase o máximo do que aparece (sem os poucos pontos isolados)
      const all = [];
      for (const r of d.rows) if (r) for (const v of r) if (v > 0) all.push(v);
      all.sort((a, b) => a - b);
      sg.max = Math.max(1, all.length ? all[Math.floor(all.length * 0.997)] : 1);
    }
    if (on !== sg.on) { sg.on = on; sg.hover = null; tip($("rcSgTip"), null); }
    sgMark();
    sgDraw();
    hdDraw();
  }
  function sgMark() {
    $("rcSgSimTags").hidden = sg.on;
    $("rcSgLiveTag").hidden = !sg.on;
    $("rcSgCtrl").hidden = !sg.on;
    $("rcTlReplay").hidden = sg.on;
    document.querySelectorAll(".rc-per button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.p === sg.period)));
    document.querySelectorAll("#rcSgE button").forEach((b) => b.setAttribute("aria-pressed", String(+b.dataset.e === sg.emax)));
    // o aviso de "dados simulados" passa para depois deste cartão
    const note = $("rcSimNote"), before = sg.on ? $("rcLogCard").nextElementSibling : $("rcSgCard");
    if (note && before && note.nextElementSibling !== before) before.before(note);
    $("rcSgDesc").textContent = sg.on
      ? "Do seu RC102: com ele conectado, o app guarda o espectro de cada minuto. Cada linha é um pedaço do tempo (o mais recente embaixo), a energia vai para a direita e a cor mostra quantos fótons chegaram naquela energia. Cinza escuro: sem dados. Linhas mais apagadas: o app estava fechado ou o aparelho desconectado, e a linha mostra a média do intervalo (o aparelho segue contando sozinho)."
      : "Três horas comprimidas: cada linha é 1 minuto, o tempo desce, a energia vai para a direita e a cor mostra quanto chegou. Neste exemplo chove do minuto 30 ao 80, e o eletrodo fica perto do minuto 118 ao 135.";
    // eixos
    const emax = sg.on ? sg.emax : SG_EMAX, step = emax > 2000 ? 500 : 250, ax = $("rcSgAxis");
    if (ax.dataset.emax !== String(emax)) {
      ax.dataset.emax = String(emax); ax.innerHTML = "";
      for (let e = 0; e <= emax; e += step) { const el = document.createElement("span"); el.textContent = e; el.style.left = (e / emax) * 100 + "%"; ax.appendChild(el); }
    }
    const tm = $("rcSgTime"), d = sg.data;
    let labels = ["min 0", "min 60", "min 120", "min 180"];
    if (sg.on && d) {
      const long = d.to - d.from > 26 * 3600e3;
      labels = [0, 1, 2, 3, 4].map((k) => { const t = d.from + ((d.to - d.from) * k) / 4; return long ? ddmm(t) : hhmm(t); });
    }
    const key = labels.join("|");
    if (tm.dataset.k !== key) { tm.dataset.k = key; tm.innerHTML = labels.map((l) => `<span>${esc(l)}</span>`).join(""); }
    let info = "";
    if (sg.on && d) {
      const has = d.rows.some((r) => r);
      info = d.first > 0
        ? `um espectro por minuto · gravando desde ${ddmm(d.first)} ${hhmm(d.first)}` + (has ? "" : " · nada neste período")
        : "ainda sem dados: a primeira linha aparece em até 2 minutos";
    }
    $("rcSgInfo").textContent = info;
    // cartão 3 (dureza): mesmo período
    const hdOn = sg.on && !!sg.hd;
    $("rcHdSimTags").hidden = hdOn;
    $("rcHdLiveTag").hidden = !hdOn;
    $("rcHdCtrl").hidden = !hdOn;
    document.querySelector("#rcHardCard .lg-elec").hidden = hdOn;
    document.querySelector("#rcHardCard .lg-gap").hidden = !hdOn;
    $("rcHdDesc").textContent = hdOn
      ? "Do seu RC102, no mesmo período do espectrograma: contagem em cima e dureza embaixo. Dureza é a proporção de fótons acima de 1.500 keV sobre os de 150 a 1.500 keV, relativa ao fundo da casa (a mediana das horas dos últimos 7 dias): 1,00 é igual ao fundo. Como chegam poucos fótons acima de 1.500 keV, cada ponto usa uns 20 minutos em volta; a faixa clara é a incerteza. Azul: chuva."
      : "Os mesmos 180 minutos: contagem em cima e dureza embaixo. Dureza é a proporção de fótons acima de 1.500 keV sobre os de 150 a 1.500 keV, relativa ao fundo da casa: 1,00 é igual ao fundo. Como chegam poucos fótons acima de 1.500 keV, a dureza usa 20 minutos em volta de cada ponto.";
    const hi = $("rcHdInfo");
    hi.hidden = !hdOn;
    if (hdOn) hi.textContent = sg.hd.ref > 0 ? `fundo da casa: ${nf(sg.hd.refCps, 2)} cps · dureza 1,00 = ${nf(sg.hd.ref * 100, 2)}% acima de 1.500 keV` : "fundo da casa: ainda pouco dado (precisa de algumas horas gravadas)";
  }
  function sgDrawReal(cv) {
    const d = sg.data; if (!d) return;
    const W = d.bins, H = d.rows.length;
    if (cv.width !== W || cv.height !== H) { cv.width = W; cv.height = H; }
    const g = cv.getContext("2d"), img = g.createImageData(W, H), lm = Math.log1p(sg.max);
    for (let y = 0; y < H; y++) {
      const row = d.rows[y], dim = d.gap[y] ? 0.5 : 1;
      for (let x = 0; x < W; x++) {
        const o = (y * W + x) * 4;
        if (!row) { img.data[o] = 13; img.data[o + 1] = 16; img.data[o + 2] = 18; img.data[o + 3] = 255; continue; }
        const [r, gg, b] = cmap(Math.log1p(row[x]) / lm);
        img.data[o] = r * dim; img.data[o + 1] = gg * dim; img.data[o + 2] = b * dim; img.data[o + 3] = 255;
      }
    }
    g.putImageData(img, 0, 0);
    if (sg.hover != null) { g.fillStyle = "rgba(255,255,255,.55)"; g.fillRect(0, sg.hover, W, 1); }
  }
  function sgHoverReal(ev) {
    const d = sg.data; if (!d) return;
    const cv = $("rcSgram"), r = cv.getBoundingClientRect(), H = d.rows.length;
    const k = Math.floor(((ev.clientY - r.top) / r.height) * H), e = Math.max(0, ((ev.clientX - r.left) / r.width) * d.eMax);
    if (k < 0 || k >= H) { sg.hover = null; tip($("rcSgTip"), null); sgDraw(); return; }
    sg.hover = k;
    const t0 = d.from + k * d.rowSec * 1000, t1 = t0 + d.rowSec * 1000;
    const when = (d.to - d.from > 26 * 3600e3 ? ddmm(t0) + " " : "") + hhmm(t0) + "–" + hhmm(t1);
    const row = d.rows[k];
    let html = `<span class="mono">${when}</span>`;
    if (!row) html += "<br>sem dados";
    else {
      const b = Math.min(d.bins - 1, Math.floor((e / d.eMax) * d.bins)), near = libNear(e);
      if (lp.n) {
        const bwk = d.eMax / d.bins;
        const cell = (yy, xx) => (d.rows[yy] ? { rate: d.rows[yy][xx], n: (d.rows[yy][xx] * d.secs[yy]) / 60 } : null);
        html = loupeHtml(cell, H, d.bins, k, b, Math.log1p(sg.max)) + html + `<br><span class="mono">${nf(b * bwk, 0)}–${nf((b + 1) * bwk, 0)} keV · ${tr("pixel do meio")}</span>`;
      }
      html += `<br><span class="mono">${nf(d.cps[k], 2)} cps · ${nf(e, 0)} keV · ${nf(row[b], 2)} por min</span>`;
      if (d.gap[k]) html += "<br>sem leitura nesse intervalo: média dele";
      else if (d.secs[k] < d.rowSec * 0.9) html += `<br>${nf((d.secs[k] / d.rowSec) * 100, 0)}% do tempo com dados`;
      if (near.length) html += `<br><b>${esc(near[0][1])}</b> ${nf(near[0][0], 1)} keV`;
    }
    tip($("rcSgTip"), html, ev.clientX - r.left, ev.clientY - r.top);
    sgDraw(); hdDraw();
  }

  // contagem e dureza em dois gráficos empilhados, com o mesmo tempo (2.51):
  // nada de dois eixos no mesmo gráfico
  const HD_H = 280;
  const hdX = (w) => { const L = 44, R = 10; return { L, R, pw: w - L - R }; };
  function hdPanels(h) {
    const T = 22, B = 22, gap = 34, ph = (h - T - B - gap) / 2;
    return { T1: T, T2: T + ph + gap, ph, B };
  }
  function hdFrame(g, L, pw, P, c, hd) {
    g.strokeStyle = css("--border"); g.lineWidth = 1; g.font = "11px " + css("--mono");
    for (const [T0, lo, hi, dig, color] of [[P.T1, 0, c.max, c.dig, COR.accent], [P.T2, hd.min, hd.max, 2, COR.away]]) {
      for (let k = 0; k <= 2; k++) {
        const y = T0 + (P.ph * k) / 2; g.beginPath(); g.moveTo(L, y + 0.5); g.lineTo(L + pw, y + 0.5); g.stroke();
        g.fillStyle = css("--dim"); g.textAlign = "right"; g.fillText(nf(hi - ((hi - lo) * k) / 2, dig), L - 6, y + 4);
      }
      g.fillStyle = color; g.textAlign = "left"; g.font = "12px " + css("--sans");
      g.fillText(T0 === P.T1 ? tr("Contagem (cps)") : tr("Dureza (1,00 = igual ao fundo)"), L, T0 - 8);
      g.font = "11px " + css("--mono");
    }
  }
  function hdDraw() {
    const cv = $("rcHard"); if (!cv) return;
    if (sg.on && sg.hd) { hdDrawReal(cv); return; }
    const f = fit(cv, HD_H); if (!f) return;
    const { g, w, h } = f, { L, pw } = hdX(w), P = hdPanels(h), T = P.T1, ph = P.T2 + P.ph - P.T1;
    g.clearRect(0, 0, w, h);
    const xOf = (t) => L + (t / (TL_N - 1)) * pw;
    // faixas dos eventos
    g.fillStyle = "rgba(96,165,250,.12)"; g.fillRect(xOf(RAIN[0]), T, xOf(RAIN[1]) - xOf(RAIN[0]), ph);
    g.fillStyle = "rgba(94,234,212,.16)"; g.fillRect(xOf(ELEC[0]), T, xOf(ELEC[1]) - xOf(ELEC[0]), ph);
    const cmax = Math.max(12, ...tl.cps.slice(0, Math.max(1, tl.shown))) * 1.1;
    const hs = tl.hard.slice(0, Math.max(1, tl.shown)), hmin = Math.min(0.6, ...hs) * 0.95, hmax = Math.max(1.4, ...hs) * 1.05;
    const yC = (v) => P.T1 + P.ph - (v / cmax) * P.ph, yH = (v) => P.T2 + P.ph - ((v - hmin) / (hmax - hmin)) * P.ph;
    hdFrame(g, L, pw, P, { max: cmax, dig: 0 }, { min: hmin, max: hmax });
    g.fillStyle = css("--dim"); g.textAlign = "center";
    for (let t = 0; t <= TL_N; t += 30) g.fillText("min " + t, xOf(Math.min(t, TL_N - 1)), h - 6);
    // dureza 1,0 (fundo)
    g.setLineDash([3, 3]); g.strokeStyle = "rgba(183,156,255,.5)"; g.beginPath(); g.moveTo(L, yH(1)); g.lineTo(L + pw, yH(1)); g.stroke(); g.setLineDash([]);
    const line = (arr, yf, color) => {
      g.beginPath();
      for (let t = 0; t < tl.shown; t++) { const x = xOf(t), y = yf(arr[t]); t ? g.lineTo(x, y) : g.moveTo(x, y); }
      g.strokeStyle = color; g.lineWidth = 1.6; g.stroke(); g.lineWidth = 1;
    };
    line(tl.cps, yC, COR.accent);
    line(tl.hard, yH, COR.away);
    if (tl.hover != null && tl.hover < tl.shown) { const x = xOf(tl.hover); g.strokeStyle = "rgba(255,255,255,.5)"; g.beginPath(); g.moveTo(x + 0.5, T); g.lineTo(x + 0.5, T + ph); g.stroke(); }
  }
  function hdHover(ev) {
    if (sg.on && sg.hd) { hdHoverReal(ev); return; }
    const cv = $("rcHard"), r = cv.getBoundingClientRect(), { L, pw } = hdX(r.width);
    const t = Math.round(((ev.clientX - r.left - L) / pw) * (TL_N - 1));
    if (t < 0 || t >= tl.shown) { tl.hover = null; tip($("rcHardTip"), null); hdDraw(); sgDraw(); return; }
    tl.hover = t;
    const ev2 = tlLabel(t), lab = tl.label[t];
    tip($("rcHardTip"), `<span class="mono">min ${t} · ${nf(tl.cps[t], 1)} cps · dureza ${nf(tl.hard[t], 2)}</span>${ev2 ? `<br>${ev2}` : ""}${lab ? `<br>parece: <b>${esc(lab)}</b>` : ""}`, ev.clientX - r.left, ev.clientY - r.top);
    hdDraw(); sgDraw();
  }

  // ---------- dureza de verdade ----------
  function hdDrawReal(cv) {
    const d = sg.hd, f = fit(cv, HD_H); if (!f) return;
    const { g, w, h } = f, { L, pw } = hdX(w), P = hdPanels(h), T = P.T1, ph = P.T2 + P.ph - P.T1, n = d.cps.length;
    g.clearRect(0, 0, w, h);
    const xOf = (k) => L + ((k + 0.5) / n) * pw, bw = pw / n;
    // chuva e intervalos sem leitura
    for (let k = 0; k < n; k++) {
      if (d.wet[k]) { g.fillStyle = "rgba(96,165,250,.16)"; g.fillRect(L + k * bw, T, bw + 0.5, ph); }
      else if (d.rain[k] > 0) { g.fillStyle = "rgba(96,165,250,.07)"; g.fillRect(L + k * bw, T, bw + 0.5, ph); }
      if (d.gap[k]) { g.fillStyle = "rgba(255,255,255,.05)"; g.fillRect(L + k * bw, T, bw + 0.5, ph); }
    }
    const cs = d.cps.filter((v) => v > 0), hs = [];
    d.hard.forEach((v, k) => { if (v > 0) hs.push(v - d.hardErr[k], v + d.hardErr[k]); });
    const cmax = Math.max(d.refCps * 1.6, ...cs, 1) * 1.1;
    const hmin = Math.min(0.6, ...hs) * 0.95, hmax = Math.max(1.4, ...hs) * 1.05;
    const yC = (v) => P.T1 + P.ph - (v / cmax) * P.ph, yH = (v) => P.T2 + P.ph - ((v - hmin) / (hmax - hmin)) * P.ph;
    hdFrame(g, L, pw, P, { max: cmax, dig: cmax < 20 ? 1 : 0 }, { min: hmin, max: hmax });
    const long = d.to - d.from > 26 * 3600e3;
    g.fillStyle = css("--dim");
    for (let k = 0; k <= 4; k++) {
      const t = d.from + ((d.to - d.from) * k) / 4, x = L + (pw * k) / 4;
      g.textAlign = k === 0 ? "left" : k === 4 ? "right" : "center";
      g.fillText(long ? ddmm(t) : hhmm(t), x, h - 6);
    }
    // referências: dureza 1,00 e o fundo em cps
    g.setLineDash([3, 3]);
    g.strokeStyle = "rgba(183,156,255,.5)"; g.beginPath(); g.moveTo(L, yH(1)); g.lineTo(L + pw, yH(1)); g.stroke();
    if (d.refCps > 0) { g.strokeStyle = "rgba(110,231,160,.35)"; g.beginPath(); g.moveTo(L, yC(d.refCps)); g.lineTo(L + pw, yC(d.refCps)); g.stroke(); }
    g.setLineDash([]);
    // faixa de incerteza da dureza (em pedaços, onde há valor)
    g.fillStyle = "rgba(183,156,255,.16)";
    for (let k = 0; k < n;) {
      if (!(d.hard[k] > 0)) { k++; continue; }
      let e = k; while (e + 1 < n && d.hard[e + 1] > 0) e++;
      g.beginPath();
      for (let i = k; i <= e; i++) g.lineTo(xOf(i), yH(d.hard[i] + d.hardErr[i]));
      for (let i = e; i >= k; i--) g.lineTo(xOf(i), yH(d.hard[i] - d.hardErr[i]));
      g.closePath(); g.fill();
      k = e + 1;
    }
    const line = (arr, yf, color) => {
      g.beginPath();
      let pen = false;
      for (let k = 0; k < n; k++) {
        if (!(arr[k] > 0)) { pen = false; continue; }
        const x = xOf(k), y = yf(arr[k]);
        if (pen) g.lineTo(x, y); else { g.moveTo(x, y); if (k + 1 >= n || !(arr[k + 1] > 0)) g.lineTo(x + 0.5, y); }
        pen = true;
      }
      g.strokeStyle = color; g.lineWidth = 1.6; g.stroke(); g.lineWidth = 1;
    };
    line(d.cps, yC, COR.accent);
    line(d.hard, yH, COR.away);
    if (!cs.length) { g.fillStyle = css("--dim"); g.textAlign = "center"; g.fillText("sem dados neste período", L + pw / 2, T + ph / 2); }
    if (sg.hover != null && sg.hover < n) { const x = xOf(sg.hover); g.strokeStyle = "rgba(255,255,255,.5)"; g.beginPath(); g.moveTo(x + 0.5, T); g.lineTo(x + 0.5, T + ph); g.stroke(); }
  }
  // com o que parece o trecho (no Go, como na medição de objeto)
  function hdGuessText(peaks) {
    if (!peaks || !peaks.length) return "";
    const by = {};
    for (const p of peaks) by[p.group] = (by[p.group] || 0) + p.sigma;
    const best = Object.entries(by).sort((a, b) => b[1] - a[1])[0][0];
    return GROUP_SHORT[best] || best;
  }
  let hdGuessTimer = 0;
  function hdTipHtml(k) {
    const d = sg.hd, t0 = d.from + k * d.rowSec * 1000, t1 = t0 + d.rowSec * 1000;
    const when = (d.to - d.from > 26 * 3600e3 ? ddmm(t0) + " " : "") + hhmm(t0) + "–" + hhmm(t1);
    let html = `<span class="mono">${when}</span>`;
    if (!(d.cps[k] > 0)) return html + "<br>sem dados";
    html += `<br><span class="mono">${nf(d.cps[k], 2)} cps`;
    if (d.refCps > 0) html += ` (${nf(d.cps[k] / d.refCps, 2)}× o fundo)`;
    html += "</span>";
    if (d.hard[k] > 0) html += `<br><span class="mono">dureza ${nf(d.hard[k], 2)} ± ${nf(d.hardErr[k], 2)}</span>`;
    if (d.wet[k]) html += `<br>chovendo (${nf(d.rain[k], 1)} mm/h)`;
    if (d.gap[k]) html += "<br>sem leitura nesse intervalo: média dele";
    const gs = sg.guess[k];
    if (gs) html += `<br>parece: <b>${esc(gs)}</b>`;
    return html;
  }
  function hdHoverReal(ev) {
    const d = sg.hd, cv = $("rcHard"), r = cv.getBoundingClientRect(), { L, pw } = hdX(r.width), n = d.cps.length;
    const k = Math.floor(((ev.clientX - r.left - L) / pw) * n);
    if (k < 0 || k >= n) { sg.hover = null; tip($("rcHardTip"), null); hdDraw(); sgDraw(); return; }
    sg.hover = k;
    const pos = [ev.clientX - r.left, ev.clientY - r.top];
    tip($("rcHardTip"), hdTipHtml(k), ...pos);
    hdDraw(); sgDraw();
    // palpite: só quando a contagem subiu ou a dureza saiu do fundo
    const up = d.refCps > 0 && d.cps[k] > d.refCps * 1.1, off = d.hard[k] > 0 && Math.abs(d.hard[k] - 1) > 2 * d.hardErr[k];
    if (!(k in sg.guess) && (up || off) && api.GetRcWindowPeaks) {
      clearTimeout(hdGuessTimer);
      hdGuessTimer = setTimeout(async () => {
        const c = d.from + (k + 0.5) * d.rowSec * 1000, half = (d.winSec * 1000) / 2;
        let p = null;
        try { p = await api.GetRcWindowPeaks(Math.round(c - half), Math.round(c + half)); } catch (e) { p = null; }
        sg.guess[k] = hdGuessText(p);
        if (sg.hover === k && sg.hd === d) tip($("rcHardTip"), hdTipHtml(k), ...pos);
      }, 250);
    }
  }

  // =====================================================================
  // 4. log
  // =====================================================================
  // O log de verdade (como a aba Log do app do celular): o que o PC recebeu do
  // aparelho e o que veio do celular (importado).
  const LOG_IC = {
    taxa: '<svg viewBox="0 0 24 24" fill="none" stroke="#38BDF8" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="4" width="14" height="16" rx="2.5"/><path d="M7 9h6M7 13h3"/><circle cx="17" cy="16" r="4.2" fill="#0D1012"/><path d="M17 14v2l1.3 1"/></svg>',
    evento: '<svg viewBox="0 0 24 24" fill="' + COR.att + '" stroke="' + COR.att + '" stroke-width="1.6" stroke-linejoin="round"><path d="M6 21V4"/><path d="M6 4h11l-2.5 4L17 12H6z"/></svg>',
    raro: '<svg viewBox="0 0 24 24" fill="none" stroke="#A78BFA" stroke-width="1.8" stroke-linecap="round"><path d="M8 14.5V5a2 2 0 0 1 4 0v9.5a3.5 3.5 0 1 1-4 0z"/><path d="M10 9v7"/><rect x="15" y="7" width="6" height="12" rx="1.5"/><path d="M17 5h2M16.5 13h3M16.5 16h3"/></svg>',
  };
  const lg = { kinds: new Set(["taxa", "evento", "raro"]), period: 0, items: [], more: false, busy: false, shown: 0, total: 0, loaded: false };
  const lgFrom = () => {
    if (!lg.period) return 0;
    const d = new Date(); d.setHours(0, 0, 0, 0);
    return d.getTime() - (lg.period - 1) * 86400e3;
  };
  async function lgLoad(more) {
    if (typeof api === "undefined" || !api.GetRcLog || lg.busy) return;
    lg.busy = true;
    const before = more && lg.items.length ? lg.items[lg.items.length - 1].t : 0;
    const limit = more ? 100 : Math.max(100, Math.min(lg.items.length, 1000));
    let p = null;
    try { p = await api.GetRcLog(lgFrom(), 0, [...lg.kinds], limit, before); } catch (e) { p = null; }
    lg.busy = false;
    if (!p) return;
    // o "antes de" junta registros do mesmo segundo: tira os repetidos
    if (more) {
      const seen = new Set(lg.items.map((it) => JSON.stringify(it)));
      lg.items = lg.items.concat(p.items.filter((it) => !seen.has(JSON.stringify(it))));
    } else lg.items = p.items || [];
    lg.more = p.more; lg.shown = p.shown; lg.total = p.total; lg.loaded = true;
    lgRender();
  }
  const fmtDate = (t) => new Date(t).toLocaleDateString(LOCALE, { day: "2-digit", month: "2-digit", year: "numeric" });
  const fmtTime = (t) => new Date(t).toLocaleTimeString(LOCALE, { hour: "2-digit", minute: "2-digit", second: "2-digit" });
  const fmtDur = (s) => { s = Math.round(s); const d = Math.floor(s / 86400), h = Math.floor((s % 86400) / 3600), m = Math.floor((s % 3600) / 60), ss = s % 60; return (d ? d + "d " : "") + h + ":" + pad2(m) + ":" + pad2(ss); };
  function lgRow(it) {
    const src = it.src === "celular" ? '<span class="src">celular</span>' : it.src === "memoria" ? '<span class="src">memória do aparelho</span>' : "";
    const when = `<td class="when mono"><b>${fmtDate(it.t)}</b><span>${fmtTime(it.t)}</span>${src}</td>`;
    let data = "";
    if (it.kind === "taxa") {
      data = `<td class="v mono" colspan="2"><span>${nf(it.usv, 3)} µSv/h ±${nf(it.usvErr || 0, 1)}%</span><span>${nf(it.cps, 1)} cps ±${nf(it.cpsErr || 0, 1)}%</span></td>`;
    } else if (it.kind === "raro") {
      const pw = it.power === "carregando" ? "carregando na fonte externa" : it.power === "externa" ? "na fonte externa" : "";
      const bat = it.hasBat ? `Bateria: ${nf(it.battery, 0)}%` : "";
      const l1 = it.hasTemp ? `<span>t = ${nf(it.temp, 1)} °C</span>` : "";
      const l2 = bat || pw ? `<span>${esc([bat, pw].filter(Boolean).join(" · "))}</span>` : "";
      const dose = it.hasDose ? `<span>Σ = ${nf(it.dose, 2)} µSv</span>${it.doseSec ? `<span class="mute">durante ${fmtDur(it.doseSec)}</span>` : ""}` : "";
      data = `<td class="v mono">${l1}${l2}</td><td class="v mono">${dose}</td>`;
    } else {
      data = `<td class="ev" colspan="2">${esc(it.event || "")}</td>`;
    }
    return `<tr><td class="ic">${LOG_IC[it.kind] || ""}</td>${when}${data}</tr>`;
  }
  function lgRender() {
    $("rcLogBody").innerHTML = lg.items.map(lgRow).join("");
    $("rcLogCount").textContent = lg.loaded ? `${lg.shown}/${lg.total}` : "";
    $("rcLogMore").hidden = !lg.more;
    $("rcLogEmpty").hidden = !lg.loaded || lg.items.length > 0;
    document.querySelectorAll("#rcLogKinds button").forEach((b) => b.setAttribute("aria-pressed", String(lg.kinds.has(b.dataset.k))));
    document.querySelectorAll("#rcLogPeriod button").forEach((b) => b.setAttribute("aria-pressed", String(+b.dataset.p === lg.period)));
  }
  async function lgImport() {
    if (!api.ImportRcLog) return;
    const m = $("rcLogMsg");
    let r = null;
    try { r = await api.ImportRcLog(); } catch (e) {
      m.hidden = false; m.className = "small warn-text"; m.textContent = "Não consegui importar: " + (e && e.message ? e.message : e); return;
    }
    if (!r || !r.file) return; // cancelou
    m.hidden = false; m.className = "small muted";
    const span = r.from ? ` · de ${fmtDate(r.from)} ${fmtTime(r.from)} a ${fmtDate(r.to)} ${fmtTime(r.to)}` : "";
    m.textContent = `${r.file}: ${r.added} registros novos de ${r.read}` + (r.skipped ? ` · ${r.skipped} linhas ignoradas` : "") + span;
    lg.items = []; lgLoad();
  }

  // =====================================================================
  // 5. modo busca
  // =====================================================================
  const SRC = { torio: ["Eletrodo de tório", 38], fumaca: ["Detector de fumaça", 14], radio: ["Bússola com tinta de rádio", 2500] };
  const sr = { src: "torio", dist: 30, val: 8.4, trend: [], sound: false, ctx: null };
  function srRate() { const d = sr.dist, s = SRC[sr.src][1]; return 8.4 + s / (d * d); }
  function srTick() {
    if (rcLive() && api.GetRcSearch) { srPoll(); return; }
    if (sr.real) srMark(false);
    const rate = srRate(), n = poisson(rate * 0.5), inst = n / 0.5;
    sr.val += 0.4 * (inst - sr.val);
    sr.trend.push(inst); if (sr.trend.length > 120) sr.trend.shift();
    if (sr.sound && sr.ctx) srClicks(Math.min(n, 60));
    srDraw();
  }
  function srClicks(n, dur = 0.5) {
    const ctx = sr.ctx, now = ctx.currentTime;
    if (!sr.buf) { const b = ctx.createBuffer(1, Math.round(ctx.sampleRate * 0.002), ctx.sampleRate); const d = b.getChannelData(0); for (let i = 0; i < d.length; i++) d[i] = (Math.random() * 2 - 1) * (1 - i / d.length); sr.buf = b; }
    for (let k = 0; k < n; k++) { const s = ctx.createBufferSource(); s.buffer = sr.buf; const gn = ctx.createGain(); gn.gain.value = 0.35; s.connect(gn).connect(ctx.destination); s.start(now + Math.random() * dur); }
  }
  function srDraw() {
    if (sr.real) { srDrawReal(); return; }
    const v = sr.val, mult = v < 100 ? 1 : v < 1000 ? 10 : v < 10000 ? 100 : 1000, frac = Math.min(1, v / (100 * mult));
    const ang = -120 + frac * 240;
    const nd = $("rcNeedle"); if (nd) nd.setAttribute("transform", `rotate(${ang.toFixed(1)} 120 120)`);
    const val = $("rcGaugeVal"); if (val) val.textContent = nf(v, v < 100 ? 2 : 0);
    document.querySelectorAll("#rcMult span").forEach((s) => s.classList.toggle("on", +s.dataset.m === mult));
    const lbl = $("rcGaugeTicks"); if (lbl) lbl.querySelectorAll("text").forEach((t, k) => { t.textContent = String(k * 20 * mult); });
    $("rcSearchInfo").textContent = `${SRC[sr.src][0]} a ${nf(sr.dist, 0)} cm · esperado ${nf(srRate(), 1)} cps (fundo 8,4) · ≈ ${nf(v / CPS_PER_USV, 2)} µSv/h`;
    const cv = $("rcTrend"); if (!cv) return;
    const f = fit(cv, 90); if (!f) return;
    const { g, w, h } = f; g.clearRect(0, 0, w, h);
    const m = Math.max(15, ...sr.trend) * 1.1;
    g.strokeStyle = css("--border"); g.beginPath(); g.moveTo(0, h - 0.5); g.lineTo(w, h - 0.5); g.stroke();
    g.beginPath();
    sr.trend.forEach((y, i) => { const x = (i / 119) * w, yy = h - 4 - (y / m) * (h - 8); i ? g.lineTo(x, yy) : g.moveTo(x, yy); });
    g.strokeStyle = COR.warn; g.lineWidth = 1.5; g.stroke(); g.lineWidth = 1;
    g.fillStyle = css("--dim"); g.font = "11px " + css("--mono"); g.textAlign = "right"; g.fillText(nf(m, 0) + " cps", w - 4, 12); g.fillText("60 s", w - 4, h - 6);
  }

  // ---------- modo busca de verdade ----------
  // A cada 0,5 s pergunta ao app o que chegou do aparelho (amostras de ~0,5 s)
  // e mostra uma média que se adapta: longa (até 20 s) enquanto nada muda e
  // curta (~1 s) quando a contagem pula, como os medidores de busca.
  Object.assign(sr, { real: false, pts: [], hist: [], last: 0, busy: false, ref: 0, myRef: 0, refUsv: 0, k: 0, max: 0, maxAt: 0, sm: 0, raw: true, ble: false, mode: "", osc: null, oscGain: null });
  function srMark(on) {
    sr.real = on;
    $("rcSrSimTags").hidden = on;
    $("rcSrLiveTag").hidden = !on;
    $("rcSrSim").hidden = on;
    $("rcSrReal").hidden = !on;
    $("rcSoundBox").hidden = on;
    $("rcSrSoundBox").hidden = !on;
    $("rcSrDesc").textContent = on
      ? "Do seu RC102, ao vivo: atualiza 2 vezes por segundo, com escala automática (×1 a ×1000). Enquanto a contagem fica parada, o número é a média de até 20 s (mais estável); quando ela pula, ele passa a usar só o último segundo e responde na hora. A cor compara com o normal: a mediana das últimas 24 h, ou o lugar onde você estiver, se marcar."
      : "Atualiza 2 vezes por segundo, com escala automática (×1 a ×1000). Escolha uma fonte e mexa na distância, como se estivesse aproximando o RC102 dela.";
    if (!on) srSound("");
    else srSound($("rcSrSound").value);
  }
  async function srPoll() {
    if (sr.busy) return;
    sr.busy = true;
    let r = null;
    try { r = await api.GetRcSearch(sr.last); } catch (e) { r = null; }
    sr.busy = false;
    if (!r || !r.live) { if (sr.real) { srMark(false); srDraw(); } return; }
    if (!sr.real) { sr.pts = []; sr.hist = []; sr.last = 0; sr.max = 0; srMark(true); }
    sr.raw = r.raw; sr.ble = r.ble; sr.ref = r.refCps; sr.refUsv = r.refUsv;
    if (r.cps > 0 && r.usv > 0) sr.k = r.usv / r.cps;
    else if (!sr.k && r.refCps > 0) sr.k = r.refUsv / r.refCps;
    const fresh = r.pts || [];
    for (const p of fresh) {
      if (sr.pts.length && p.t - sr.pts[sr.pts.length - 1].t > 3000) sr.pts = []; // caiu: recomeça
      sr.pts.push(p);
    }
    if (fresh.length) sr.last = fresh[fresh.length - 1].t;
    const cut = r.now - 180e3;
    while (sr.pts.length && sr.pts[0].t < cut) sr.pts.shift();
    sr.sm = srSmooth();
    if (fresh.length) { sr.hist.push({ t: sr.last, v: sr.sm }); while (sr.hist.length && sr.hist[0].t < cut) sr.hist.shift(); }
    if (sr.sm > sr.max) { sr.max = sr.sm; sr.maxAt = sr.pts.length ? sr.pts[sr.pts.length - 1].t : r.now; }
    srPlay(fresh);
    srDraw();
  }
  function srSmooth() {
    const p = sr.pts; if (!p.length) return 0;
    let c1 = 0, s1 = 0;
    for (let i = p.length - 1; i >= 0 && s1 < 0.99; i--) { c1 += p[i].cps * p[i].sec; s1 += p[i].sec; }
    const fast = c1 / s1;
    let best = fast, c = 0, s = 0;
    for (let j = p.length - 1; j >= 0 && s < 20; j--) {
      c += p[j].cps * p[j].sec; s += p[j].sec;
      const m = c / s, sig = Math.sqrt(Math.max(m, 0.5) / s1);
      if (Math.abs(fast - m) > 3 * sig) break;
      best = m;
    }
    return best;
  }
  const srRefNow = () => sr.myRef || sr.ref;
  function srDrawReal() {
    const v = sr.sm, mult = v < 100 ? 1 : v < 1000 ? 10 : v < 10000 ? 100 : 1000, frac = Math.min(1, v / (100 * mult));
    const nd = $("rcNeedle"); if (nd) nd.setAttribute("transform", `rotate(${(-120 + frac * 240).toFixed(1)} 120 120)`);
    const val = $("rcGaugeVal"); if (val) val.textContent = nf(v, v < 100 ? 2 : 0);
    document.querySelectorAll("#rcMult span").forEach((el) => el.classList.toggle("on", +el.dataset.m === mult));
    const lbl = $("rcGaugeTicks"); if (lbl) lbl.querySelectorAll("text").forEach((t, k) => { t.textContent = String(k * 20 * mult); });
    const ref = srRefNow(), ratio = ref > 0 ? v / ref : 0, rt = $("rcSrRatio");
    rt.textContent = ref > 0 ? `${nf(ratio, ratio < 10 ? 2 : 0)}× o normal` : "sem referência";
    rt.classList.toggle("warn", ratio >= 1.5 && ratio < 3);
    rt.classList.toggle("hot", ratio >= 3);
    $("rcSrUsv").textContent = sr.k > 0 ? `≈ ${nf(v * sr.k, 3)} µSv/h` : "";
    $("rcSrRef").textContent = ref > 0
      ? (sr.myRef ? `normal: ${nf(ref, 2)} cps (marcado aqui)` : `normal: ${nf(ref, 2)} cps (mediana das últimas 24 h)`)
      : "normal: ainda sem 24 h de dados; use \"Usar o agora como normal\"";
    $("rcSrRefReset").hidden = !sr.myRef;
    $("rcSrMax").textContent = sr.max > 0 ? `máximo: ${nf(sr.max, 2)} cps às ${fmtTime(sr.maxAt)}` + (ref > 0 ? ` (${nf(sr.max / ref, 2)}× o normal)` : "") : "";
    $("rcSearchInfo").textContent = (sr.raw ? "amostras de 0,5 s do aparelho" : "sem amostras curtas: a taxa vem de 1 em 1 s") + (sr.ble ? " · Bluetooth: chega com até 1–2 s de atraso" : "");
    // tendência: últimos 60 s, com o normal tracejado
    const cv = $("rcTrend"); if (!cv) return;
    const f = fit(cv, 90); if (!f) return;
    const { g, w, h } = f; g.clearRect(0, 0, w, h);
    const now = sr.pts.length ? sr.pts[sr.pts.length - 1].t : Date.now(), win = sr.pts.filter((p) => p.t > now - 60e3);
    const m = Math.max(15, ref * 1.6, ...win.map((p) => p.cps)) * 1.1;
    const yOf = (y) => h - 4 - (y / m) * (h - 8), xOf = (t) => w - ((now - t) / 60e3) * w;
    g.strokeStyle = css("--border"); g.beginPath(); g.moveTo(0, h - 0.5); g.lineTo(w, h - 0.5); g.stroke();
    if (ref > 0) { g.setLineDash([3, 3]); g.strokeStyle = "rgba(110,231,160,.55)"; g.beginPath(); g.moveTo(0, yOf(ref)); g.lineTo(w, yOf(ref)); g.stroke(); g.setLineDash([]); }
    g.beginPath();
    win.forEach((p, i) => { const x = xOf(p.t), y = yOf(p.cps); i ? g.lineTo(x, y) : g.moveTo(x, y); });
    g.strokeStyle = "rgba(240,101,106,.55)"; g.lineWidth = 1; g.stroke();
    // a média que se adapta, por cima
    g.beginPath();
    sr.hist.filter((p) => p.t > now - 60e3).forEach((p, i) => { const x = xOf(p.t), y = yOf(p.v); i ? g.lineTo(x, y) : g.moveTo(x, y); });
    g.strokeStyle = COR.warn; g.lineWidth = 1.8; g.stroke(); g.lineWidth = 1;
    g.fillStyle = css("--dim"); g.font = "11px " + css("--mono"); g.textAlign = "right"; g.fillText(nf(m, 0) + " cps", w - 4, 12); g.fillText("60 s", w - 4, h - 6);
  }
  // som: cliques (um por contagem, espalhados no tempo da amostra) ou um tom
  // que fica mais agudo quanto mais acima do normal
  function srAudio() {
    if (!sr.ctx) { try { sr.ctx = new (window.AudioContext || window.webkitAudioContext)(); } catch (e) { return null; } }
    sr.ctx.resume();
    return sr.ctx;
  }
  function srSound(mode) {
    sr.mode = mode;
    if (sr.osc) { try { sr.osc.stop(); } catch (e) { /* já parou */ } sr.osc = null; }
    if (!mode) { if (sr.ctx && !sr.sound) sr.ctx.suspend(); return; }
    const ctx = srAudio(); if (!ctx) return;
    if (mode === "tom") {
      sr.osc = ctx.createOscillator(); sr.oscGain = ctx.createGain();
      sr.osc.type = "sine"; sr.oscGain.gain.value = 0.06;
      sr.osc.connect(sr.oscGain).connect(ctx.destination); sr.osc.start();
    }
  }
  function srPlay(fresh) {
    if (!sr.real || !sr.mode || !sr.ctx) return;
    if (sr.mode === "cliques") {
      // uma amostra por vez, atrasada de 1 s para dar tempo de espalhar
      for (const p of fresh) {
        const n = Math.min(60, Math.round(p.cps * p.sec));
        if (n > 0) srClicks(n, p.sec);
      }
    } else if (sr.osc) {
      const ref = srRefNow() || sr.sm || 1, r = Math.max(0.5, sr.sm / ref);
      const f = Math.min(1760, 220 * Math.pow(2, Math.log2(r) * 1.5 + 1));
      sr.osc.frequency.setTargetAtTime(f, sr.ctx.currentTime, 0.12);
    }
  }
  function gaugeSVG() {
    const cx = 120, cy = 120, r = 96, pt = (a, rr) => { const t = ((a - 90) * Math.PI) / 180; return [cx + rr * Math.cos(t), cy + rr * Math.sin(t)]; };
    let ticks = "", labels = "";
    for (let k = 0; k <= 50; k++) {
      const a = -120 + (k / 50) * 240, big = k % 10 === 0, [x1, y1] = pt(a, r), [x2, y2] = pt(a, r - (big ? 12 : 6));
      const col = k > 40 ? COR.warn : k > 30 ? COR.att : COR.accent;
      ticks += `<line x1="${x1.toFixed(1)}" y1="${y1.toFixed(1)}" x2="${x2.toFixed(1)}" y2="${y2.toFixed(1)}" stroke="${col}" stroke-width="${big ? 2 : 1}" stroke-opacity="${big ? 1 : 0.7}"/>`;
      if (big) { const [lx, ly] = pt(a, r - 26); labels += `<text x="${lx.toFixed(1)}" y="${(ly + 4).toFixed(1)}">0</text>`; }
    }
    return `<svg viewBox="0 0 240 200" class="rc-gauge-svg" role="img" aria-label="Mostrador de contagem">
      ${ticks}<g id="rcGaugeTicks" class="rc-gauge-lbl">${labels}</g>
      <g id="rcNeedle"><line x1="120" y1="120" x2="120" y2="36" stroke="${COR.warn}" stroke-width="3" stroke-linecap="round"/></g>
      <circle cx="120" cy="120" r="7" fill="${COR.warn}"/>
      <text x="120" y="162" class="rc-gauge-num" id="rcGaugeVal">0</text>
      <text x="120" y="182" class="rc-gauge-unit">cps</text></svg>`;
  }

  // =====================================================================
  // 6. atividade (Bq): calibrar com KCl e tório, medir amostras
  // =====================================================================
  // Valores típicos para comparar (fontes no comentário de cada um).
  const ACT_PRESETS = {
    castanha: { name: "castanha-do-pará", mass: 500, nucs: ["Ra-226", "Ra-228", "K-40"],
      hint: "Castanha moída ou picada, pote cheio e bem fechado ~3 semanas antes (para o Ra-226). Típico: Ra-226 ≈ 105 e Ra-228 ≈ 99 Bq/kg na média de 30 lotes (Acta Amazonica, 2012); outro estudo achou 24 e 26 Bq/kg. Para o RC102 é fraco: conte com 12 a 24 h e talvez só um limite (\"abaixo de\")." },
    areia: { name: "areia monazítica", mass: 500, nucs: ["Th-232", "Ra-226", "K-40"],
      hint: "Areia seca, pote cheio. Típico: Th-232 de ~1.000 a 2.300 Bq/kg em praias de Marataízes (ES); solo comum tem ~45 Bq/kg (UNSCEAR 2000). Sai em menos de 1 h." },
    rocha: { name: "granito / porcelanato", mass: 500, nucs: ["K-40", "Ra-226", "Th-232"],
      hint: "Pedaços que caibam no pote, ou a peça encostada do mesmo jeito. Para comparar, a média do solo no mundo: K-40 ≈ 420, Ra-226 ≈ 32 e Th-232 ≈ 45 Bq/kg (UNSCEAR 2000)." },
    urania: { name: "vidro ou esmalte de urânio", mass: 100, nucs: ["U-238", "Ra-226"],
      hint: "O urânio-238 sai pela linha de 1.001 keV (fraca): só funciona com peça forte. O rádio costuma ser baixo, porque o urânio foi purificado ao fazer o vidro." },
    comida: { name: "alimento", mass: 500, nucs: ["Cs-137", "K-40"],
      hint: "Na comida comum, o que aparece é o K-40 (o potássio). O Cs-137 de hoje em comida brasileira fica bem abaixo do que o RC102 enxerga: o resultado deve ser \"abaixo de\"." },
    adubo: { name: "adubo de potássio", mass: 500, nucs: ["K-40"],
      hint: "Pelo K-40 o app calcula quanto potássio tem (31,7 Bq por grama de potássio)." },
    outro: { name: "", mass: 100, nucs: ["K-40", "Ra-226", "Th-232"], hint: "" },
  };
  const act = { v: null, kind: "amostra", preset: "castanha", busy: false, at: 0, chartKey: "" };
  const fmtBq = (v) => (Math.abs(v) >= 1e4 ? nf(v / 1000, 1) + " k" : nf(v, Math.abs(v) >= 100 ? 0 : Math.abs(v) >= 10 ? 1 : 2) + " ");
  const fmtPct = (v) => nf(v * 100, v >= 0.01 ? 2 : 3) + "%";
  const fmtSec = (s) => { if (!isFinite(s) || s <= 0) return "—"; if (s < 90) return nf(s, 0) + " s"; if (s < 5400) return nf(s / 60, 0) + " min"; return nf(s / 3600, s < 36000 ? 1 : 0) + " h"; };
  const setHTML = (el, html) => { if (el._h !== html) { el._h = html; el.innerHTML = html; } };
  async function actLoad() {
    if (typeof api === "undefined" || !api.GetActivity || act.busy) return;
    act.busy = true;
    let v = null;
    try { v = await api.GetActivity(); } catch (e) { v = null; }
    act.busy = false;
    if (!v) return;
    act.v = v;
    actRender();
  }
  function actCalHtml(v) {
    const c = v.cal || {}, parts = [];
    parts.push(`<b>Calibração</b> · <span>posição: ${esc(c.geometry || "")}</span>`);
    if (c.kcl) parts.push(`KCl em ${fmtDate(c.kcl.t)}: eficiência de <span class="mono">${fmtPct(c.kcl.eff)}</span> em 1.460 keV (±${nf(c.kcl.effErr * 100, 1)}%)<button type="button" class="link" data-forget="kcl">esquecer</button>`);
    else parts.push(v.cal && v.cal.th ? "sem KCl: a escala vem do tório (aproximada)" : "<span class=\"warn-text\">sem calibração: os Bq são só um chute (±100%). Comece pelo KCl.</span>");
    if (c.th) parts.push(`forma da curva pelo tório em ${fmtDate(c.th.t)}: inclinação <span class="mono">${nf(c.th.b, 2)} ± ${nf(c.th.bErr, 2)}</span><button type="button" class="link" data-forget="torio">esquecer</button>`);
    else parts.push(`forma da curva: estimada (inclinação −1,3 ± 0,4); o tório melhora`);
    // eficiência em cada energia: uma tabela pequena em vez da linha técnica
    const es = [250, 500, 1000, 1500, 2000, 2500];
    const tab = `<table class="rc-act-eff"><thead><tr><th>${tr("energia (keV)")}</th>${es.map((e) => `<th class="mono">${nf(e, 0)}</th>`).join("")}</tr></thead><tbody>` +
      `<tr><th>${tr("eficiência")}</th>${es.map((_, i) => `<td class="mono">${fmtPct(v.calEff[i])}</td>`).join("")}</tr>` +
      `<tr><th>${tr("incerteza")}</th>${es.map((_, i) => `<td class="mono">±${nf(v.calErr[i] * 100, 0)}%</td>`).join("")}</tr></tbody></table>`;
    return parts.join("<br>") + `<div class="rc-act-eff-wrap" data-no-tr>${tab}</div>`;
  }
  function actNucRows(list, massG, live) {
    if (!list || !list.length) return "";
    let h = `<table class="rc-act-tab"><thead><tr><th>radionuclídeo</th><th>atividade</th><th>por kg</th>${live ? "<th>para ±10%</th>" : ""}</tr></thead><tbody>`;
    for (const n of list) {
      const lines = (n.lines || []).map((l) => `${nf(l.kev, 1)} keV (${esc(l.who)}): ${fmtBq(l.bq)}± ${fmtBq(l.err)}Bq`).join(" · ");
      let a, k;
      if (n.detected) {
        a = `${fmtBq(n.bq)}± ${fmtBq(n.err)}Bq`; k = massG > 0 ? `${fmtBq(n.bqKg)}± ${fmtBq(n.errKg)}Bq/kg` : "";
      } else {
        a = `abaixo de ${fmtBq(n.mda)}Bq`; k = massG > 0 ? `abaixo de ${fmtBq(n.mdaKg)}Bq/kg` : "";
      }
      let extra = "";
      if (n.id === "K-40" && n.detected && massG > 0) extra = `<span class="sub">≈ ${nf(n.bqKg / 31.7, 1)} g de potássio por kg</span>`;
      const need = n.detected ? (n.needSec > 0 ? (n.needSec <= ((act.v && act.v.phaseSec) || 0) ? "já chegou" : "~" + fmtSec(n.needSec) + " no total") : "—") : "ainda não aparece";
      h += `<tr><td>${esc(n.name)}${n.note ? `<span class="sub">${esc(n.note)}</span>` : ""}</td><td class="n">${a}</td><td class="n">${k}${extra}</td>${live ? `<td>${need}</td>` : ""}</tr>`;
      if (!live && lines) h += `<tr class="lines"><td colspan="3"><span class="sub mono">${lines}</span></td></tr>`;
    }
    return `<div class="rc-act-wrap">${h}</tbody></table></div>`;
  }
  function actResHtml(r) {
    let h = "";
    if (r.kind === "kcl") {
      h += r.kcl ? `<p>KCl: ${fmtBq(r.expected)}Bq de K-40 esperados · eficiência medida em 1.460 keV: <b class="mono">${fmtPct(r.kcl.eff)} ± ${nf(r.kcl.effErr * 100, 1)}%</b>${r.prev ? ` (antes: ${fmtPct(r.prev)})` : ""}</p><p>A calibração foi atualizada.</p>` : `<p class="warn-text">O pico do K-40 não apareceu: confira a massa, a posição e o tempo.</p>`;
    } else if (r.kind === "torio") {
      h += r.th ? `<p>Tório: inclinação da curva <b class="mono">${nf(r.th.b, 2)} ± ${nf(r.th.bErr, 2)}</b> (pelas linhas de 238,6, 583,2 e 2.614,5 keV)</p><p>${act.v && act.v.cal && act.v.cal.kcl ? "A calibração foi atualizada; a escala continua a do KCl." : "A calibração foi atualizada; sem KCl, a escala vem do tório (aproximada, depende da idade)."}</p>` : `<p class="warn-text">As linhas do tório não apareceram o bastante (precisa de pelo menos duas).</p>`;
      if (r.thLines && r.thLines.length) h += `<p class="small muted mono">${r.thLines.map((l) => `${nf(l.kev, 1)} keV: ${nf(l.area, 0)} ± ${nf(l.areaErr, 0)} contagens`).join(" · ")}</p>`;
    }
    if (r.kind === "amostra" || r.kind === "torio") h += actNucRows(r.nuclides, r.massG, false);
    const calTxt = { kcl: "calibração pelo KCl", torio: "calibração pelo tório (aproximada)", chute: "sem calibração: chute" }[r.calSrc] || "";
    h += `<p class="small muted">${esc(r.name)} · ${nf(r.massG, 1)} g · amostra ${fmtSec(r.sec)} · fundo ${fmtSec(r.bgSec)} · ${calTxt}${r.geometry ? " · " + esc(r.geometry) : ""}</p>`;
    return h;
  }
  function actSummary(r) {
    if (r.kind === "kcl") return r.kcl ? `eficiência ${fmtPct(r.kcl.eff)} em 1.460 keV` : "sem pico";
    if (r.kind === "torio") return r.th ? `inclinação ${nf(r.th.b, 2)}` : "sem linhas";
    return (r.nuclides || []).map((n) => `${n.id} ${n.detected ? fmtBq(n.bqKg) + "Bq/kg" : "< " + fmtBq(n.mdaKg) + "Bq/kg"}`).join(" · ");
  }
  function actRender() {
    const v = act.v; if (!v) return;
    setHTML($("rcActCal"), actCalHtml(v));
    const running = !!v.phase;
    $("rcActForm").hidden = running;
    $("rcActRun").hidden = !running;
    // formulário
    document.querySelectorAll("#rcActKind button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.k === act.kind)));
    document.querySelectorAll("#rcActForm [data-for]").forEach((el) => { el.hidden = !el.dataset.for.split(" ").includes(act.kind); });
    $("rcActPctBox").hidden = act.kind !== "kcl" || $("rcActKclType").value === "100";
    const bgSel = $("rcActBg"), saved = v.savedBg;
    bgSel.options[1].disabled = !saved;
    bgSel.options[1].textContent = saved ? `usar o fundo salvo (${fmtDate(saved.t)}, ${fmtSec(saved.sec)})` : "usar o fundo salvo (não tem)";
    if (!saved && bgSel.value === "salvo") bgSel.value = "novo";
    $("rcActBgTBox").hidden = bgSel.value !== "novo";
    if (!$("rcActGeom").value) $("rcActGeom").value = (v.cal && v.cal.geometry) || "";
    const pr = ACT_PRESETS[act.preset];
    $("rcActHint").textContent = act.kind === "amostra" ? pr.hint : act.kind === "kcl"
      ? "Pote cheio de sal, o RC102 encostado sempre do mesmo jeito (anote a posição). Com 500 g de KCl puro, 1 h já dá uma boa medida; com sal light, 2 h. O fundo é medido com o pote longe (uns 3 m) e o aparelho no mesmo lugar."
      : "O eletrodo encostado no RC102. Serve para a forma da curva (como a eficiência muda com a energia): as três linhas usadas vêm do mesmo ramo da série, então a idade não atrapalha a forma, só a escala.";
    $("rcActStart").disabled = !v.connected;
    $("rcActErr").textContent = v.connected ? $("rcActErr").dataset.msg || "" : "Conecte o RC102 para medir.";
    // em andamento
    if (running) actRenderRun(v);
    // gráfico
    const ch = v.chart;
    $("rcActPlot").hidden = !ch; $("rcActLegend").hidden = !ch;
    if (ch) actDraw(ch, v);
    // histórico
    const hist = v.history || [];
    setHTML($("rcActHist"), hist.length ? `<p class="small muted">Medidas anteriores</p>` + hist.map((r) => `<details><summary><span class="mono">${fmtDate(r.t)} ${fmtTime(r.t)}</span>${esc(r.name)} · ${esc(actSummary(r))}</summary><div class="rc-act-res">${actResHtml(r)}</div></details>`).join("") : "");
  }
  function actRenderRun(v) {
    const o = v.opts || {}, ph = v.phase, t = v.phaseSec || 0, tg = v.target || 0;
    const step = {
      fundo: `<b>1. Fundo:</b> tire a amostra de perto (uns 3 m) e deixe o RC102 no lugar de sempre.`,
      trocar: `<b>2. Amostra:</b> coloque ${esc(o.name || "a amostra")} na posição (${esc(o.geometry || "a mesma da calibração")}) e clique em Começar.`,
      amostra: `<b>${o.newBg ? "3" : "1"}. Medindo:</b> ${esc(o.name || "")} (${nf(o.massG, 1)} g)`,
      pronto: `<b>Pronto:</b> ${esc(o.name || "")}`,
    }[ph] || "";
    $("rcActStep").innerHTML = step;
    $("rcActBar").style.width = (tg > 0 && (ph === "fundo" || ph === "amostra") ? Math.min(100, (t / tg) * 100) : ph === "pronto" ? 100 : 0) + "%";
    $("rcActTime").textContent = ph === "fundo" || ph === "amostra" ? `${fmtDur(t)}${tg > 0 ? " / " + fmtDur(tg) : ""}${v.bgSec && ph === "amostra" ? " · fundo de " + fmtSec(v.bgSec) : ""}` : "";
    const nx = $("rcActNext");
    nx.hidden = false;
    nx.textContent = { fundo: "Encerrar o fundo agora", trocar: "Começar", amostra: "Encerrar agora", pronto: "Nova medida" }[ph] || "";
    $("rcActCancel").hidden = ph === "pronto";
    let live = "";
    if (ph === "amostra") {
      if (v.liveKcl) live += `<p class="mono small">eficiência até agora: ${fmtPct(v.liveKcl.eff)} ± ${nf(v.liveKcl.effErr * 100, 1)}%${v.liveKcl.effErr > 0 ? ` · para ±3%: ~${fmtSec(t * (v.liveKcl.effErr / 0.03) ** 2)} no total` : ""}</p>`;
      if (v.liveTh) live += `<p class="mono small">inclinação até agora: ${nf(v.liveTh.b, 2)} ± ${nf(v.liveTh.bErr, 2)}</p>`;
      if (o.kind !== "kcl") live += actNucRows(v.live, o.massG, true);
      if (!live && t < 10) live = `<p class="small muted">Os números aparecem depois de alguns segundos.</p>`;
    } else if (ph === "pronto" && v.result) live = `<div class="rc-act-res">${actResHtml(v.result)}</div>`;
    setHTML($("rcActLive"), live);
  }
  function actDraw(ch, v) {
    const cv = $("rcActSpec"), f = fit(cv, 170); if (!f) return;
    const { g, w, h } = f, L = 46, R = 8, T0 = 8, B = 22, pw = w - L - R, ph = h - T0 - B, n = Math.max((ch.smp || []).length, (ch.bg || []).length);
    g.clearRect(0, 0, w, h);
    if (!n) return;
    const all = [...(ch.smp || []), ...(ch.bg || [])].filter((x) => x > 0);
    const lmax = Math.log10(Math.max(...all, 1e-4) * 1.5), lmin = Math.log10(Math.max(Math.min(...all), 1e-5));
    const xOf = (e) => L + (e / ch.eMax) * pw, yOf = (val) => T0 + ph - ((Math.log10(Math.max(val, 10 ** lmin)) - lmin) / (lmax - lmin || 1)) * ph;
    g.strokeStyle = css("--border"); g.fillStyle = css("--dim"); g.font = "11px " + css("--mono"); g.textAlign = "center";
    for (let e = 500; e < ch.eMax; e += 500) { const x = xOf(e); g.beginPath(); g.moveTo(x + 0.5, T0); g.lineTo(x + 0.5, T0 + ph); g.stroke(); g.fillText(e + " keV", x, h - 6); }
    g.textAlign = "right"; g.fillText("cps/keV (log)", L + pw - 4, T0 + 10);
    const path = (arr, col, fill) => {
      if (!arr) return;
      const bw = ch.eMax / arr.length;
      g.beginPath(); g.moveTo(xOf(0), T0 + ph);
      arr.forEach((val, k) => { g.lineTo(xOf(k * bw), yOf(val)); g.lineTo(xOf((k + 1) * bw), yOf(val)); });
      g.lineTo(xOf(ch.eMax), T0 + ph);
      if (fill) { g.fillStyle = fill; g.fill(); }
      g.strokeStyle = col; g.stroke();
    };
    path(ch.smp, "#F2A33A", "rgba(242,163,58,.25)");
    path(ch.bg, COR.muted, null);
    // linhas dos radionuclídeos da medida
    const ids = (v.opts && v.opts.nuclides) || [], seen = new Set();
    g.setLineDash([3, 3]); g.strokeStyle = COR.away; g.fillStyle = COR.away; g.textAlign = "left";
    for (const nu of v.nuclides || []) if (ids.includes(nu.id)) for (const e of nu.kevs || []) {
      if (seen.has(e)) continue; seen.add(e);
      const x = xOf(e); g.beginPath(); g.moveTo(x, T0); g.lineTo(x, T0 + ph); g.stroke();
      g.fillText(nf(e, 0), x + 3, T0 + 10 + (seen.size % 2) * 12);
    }
    g.setLineDash([]);
  }
  function actPreset(k) {
    act.preset = k;
    const p = ACT_PRESETS[k];
    $("rcActName").value = p.name;
    $("rcActMass").value = p.mass;
    document.querySelectorAll("#rcActNucs input").forEach((c) => { c.checked = p.nucs.includes(c.value); });
  }
  async function actStart() {
    const kind = act.kind, num = (id) => parseFloat(String($(id).value).replace(",", ".")) || 0;
    const o = {
      kind, name: kind === "amostra" ? $("rcActName").value : "", massG: num("rcActMass"), geometry: $("rcActGeom").value,
      newBg: $("rcActBg").value === "novo", bgSec: +$("rcActBgT").value, smpSec: +$("rcActSmpT").value, nuclides: [], pct: 0, ageY: 0,
    };
    if (kind === "kcl") o.pct = $("rcActKclType").value === "100" ? 100 : num("rcActPct");
    if (kind === "torio") { o.pct = +$("rcActTh").value; o.ageY = num("rcActAge"); }
    if (kind === "amostra") o.nuclides = [...document.querySelectorAll("#rcActNucs input:checked")].map((c) => c.value);
    const err = $("rcActErr");
    try { await api.StartActivity(o); err.dataset.msg = ""; } catch (e) { err.dataset.msg = String(e && e.message ? e.message : e); }
    actLoad();
  }
  async function actNext() {
    if (act.v && act.v.phase === "pronto") { await api.CloseActivity(); }
    else { try { await api.ActivityNext(); } catch (e) { $("rcActErr").dataset.msg = String(e && e.message ? e.message : e); } }
    actLoad();
  }

  // =====================================================================
  // montagem e ciclo de vida
  // =====================================================================
  let built = false, visible = false, timers = [];
  function build() {
    if (built) return;
    built = true;
    // espectro
    const sc = $("rcSpecScen");
    for (const [k, s] of Object.entries(SCEN)) { const b = document.createElement("button"); b.type = "button"; b.dataset.s = k; b.textContent = s.name; sc.appendChild(b); }
    sc.addEventListener("click", (e) => { const b = e.target.closest("button[data-s]"); if (!b) return; sp.scen = b.dataset.s; spMark(); spReset(); });
    $("rcSpecLog").addEventListener("click", () => { sp.log = !sp.log; spMark(); spDraw(); });
    $("rcSpecBg").addEventListener("change", (e) => { sp.bg = e.target.checked; spDraw(); });
    $("rcSpecSpeed").addEventListener("change", (e) => { sp.speed = +e.target.value; });
    $("rcSpecPlay").addEventListener("click", () => { sp.run = !sp.run; spMark(); if (sp.run && rl.on) realTick(); });
    $("rcSpecReset").addEventListener("click", () => {
      if (rl.on && rl.mode === "acc") { accZero(); return; }
      if (rl.on) { screenReset(); return; }
      spReset();
    });
    $("rcSpecMode").addEventListener("click", (e) => {
      pk.sel = null; pk.range = null; pk.win = null;
      const b = e.target.closest("button[data-m]"); if (!b) return;
      rl.mode = b.dataset.m; spMark(); spDraw();
      if (rl.mode === "acc" && rl.on && Date.now() - rl.accAt > 30e3) { rl.accAt = 0; realTick(true); }
    });
    $("rcSpec").addEventListener("mousemove", spHover);
    $("rcSpec").addEventListener("mousedown", (ev) => {
      const r = $("rcSpec").getBoundingClientRect(), pw = r.width - 54, e = ((ev.clientX - r.left - 44) / pw) * 3000;
      if (e < 0 || e > 3000) return;
      pk.drag = [e, e];
      ev.preventDefault();
    });
    window.addEventListener("mouseup", () => {
      if (!pk.drag) return;
      const [a, b] = pk.drag, lo = Math.min(a, b), hi = Math.max(a, b);
      pk.drag = null;
      if (hi - lo >= 20) { pk.sel = null; pk.range = [lo, hi]; pk.win = [lo, hi]; spDraw(); return; }
      // clique perto de um pico achado: abre o isótopo dele
      const p = curPeaks().filter((q) => Math.abs(q.kev - a) < 0.6 * fwhm(q.kev)).sort((x, y) => Math.abs(x.kev - a) - Math.abs(y.kev - a))[0];
      if (p) isoPick(p.nuclide); else spDraw();
    });
    $("rcIsos").addEventListener("click", (e) => { const b = e.target.closest("button[data-nuc]"); if (b) isoPick(b.dataset.nuc); });
    $("rcZoomClose").addEventListener("click", zoomClose);
    const lpSeg = $("rcLoupe");
    const lpMark = () => lpSeg.querySelectorAll("button").forEach((b) => b.setAttribute("aria-pressed", String(+b.dataset.n === lp.n)));
    lpSeg.addEventListener("click", (e) => { const b = e.target.closest("button[data-n]"); if (!b) return; lp.n = +b.dataset.n; try { localStorage.setItem("rcLoupe", String(lp.n)); } catch (er) { /* sem armazenamento */ } lpMark(); });
    lpMark();
    $("rcSpec").addEventListener("mouseleave", () => { sp.hover = null; tip($("rcSpecTip"), null); spDraw(); });
    spRate();
    // linha do tempo
    tlBuild();
    $("rcTlReplay").addEventListener("click", () => { tlBuild(); tl.run = true; sgDraw(); hdDraw(); });
    $("rcSgram").addEventListener("mousemove", sgHover);
    $("rcSgram").addEventListener("mouseleave", () => { tl.hover = null; sg.hover = null; tip($("rcSgTip"), null); sgDraw(); hdDraw(); });
    for (const id of ["rcSgPeriod", "rcHdPeriod"]) $(id).addEventListener("click", (e) => { const b = e.target.closest("button[data-p]"); if (!b) return; sg.period = b.dataset.p; sg.hover = null; sgMark(); sgLoad(); });
    $("rcSgE").addEventListener("click", (e) => { const b = e.target.closest("button[data-e]"); if (!b) return; sg.emax = +b.dataset.e; sgMark(); sgLoad(); });
    $("rcHard").addEventListener("mousemove", hdHover);
    $("rcHard").addEventListener("mouseleave", () => { tl.hover = null; sg.hover = null; tip($("rcHardTip"), null); hdDraw(); sgDraw(); });
    sgMark();
    // log
    for (const b of document.querySelectorAll("#rcLogKinds button")) b.innerHTML = LOG_IC[b.dataset.k];
    $("rcLogKinds").addEventListener("click", (e) => {
      const b = e.target.closest("button[data-k]"); if (!b) return;
      if (lg.kinds.has(b.dataset.k)) lg.kinds.delete(b.dataset.k); else lg.kinds.add(b.dataset.k);
      lg.items = []; lgRender(); lgLoad();
    });
    $("rcLogPeriod").addEventListener("click", (e) => { const b = e.target.closest("button[data-p]"); if (!b) return; lg.period = +b.dataset.p; lg.items = []; lgRender(); lgLoad(); });
    $("rcLogMore").addEventListener("click", () => lgLoad(true));
    $("rcLogImport").addEventListener("click", lgImport);
    // busca
    $("rcGauge").innerHTML = gaugeSVG();
    const ss = $("rcSrc");
    for (const [k, [n]] of Object.entries(SRC)) { const o = document.createElement("option"); o.value = k; o.textContent = n; ss.appendChild(o); }
    ss.addEventListener("change", () => { sr.src = ss.value; });
    const dist = $("rcDist");
    const setDist = () => { sr.dist = Math.round(Math.pow(10, +dist.value)); $("rcDistLbl").textContent = sr.dist + " cm"; };
    dist.addEventListener("input", setDist); setDist();
    $("rcSound").addEventListener("change", (e) => {
      sr.sound = e.target.checked;
      if (sr.sound && !sr.ctx) { try { sr.ctx = new (window.AudioContext || window.webkitAudioContext)(); } catch (err) { sr.sound = false; e.target.checked = false; } }
      if (sr.ctx && sr.sound) sr.ctx.resume();
    });
    $("rcSrSound").addEventListener("change", (e) => srSound(e.target.value));
    $("rcSrHere").addEventListener("click", () => { if (sr.sm > 0) { sr.myRef = sr.sm; srDraw(); } });
    $("rcSrRefReset").addEventListener("click", () => { sr.myRef = 0; srDraw(); });
    $("rcSrMaxReset").addEventListener("click", () => { sr.max = 0; srDraw(); });
    // atividade
    const psel = $("rcActPreset");
    for (const [k, p] of Object.entries(ACT_PRESETS)) { const o = document.createElement("option"); o.value = k; o.textContent = p.name || "outra coisa"; psel.appendChild(o); }
    $("rcActNucs").innerHTML = ["K-40", "Cs-137", "Ra-226", "Ra-228", "Th-232", "U-238"].map((id) => `<label><input type="checkbox" value="${id}"> ${id}</label>`).join("");
    psel.addEventListener("change", () => { actPreset(psel.value); actRender(); });
    actPreset("castanha");
    $("rcActKind").addEventListener("click", (e) => {
      const b = e.target.closest("button[data-k]"); if (!b) return;
      act.kind = b.dataset.k;
      $("rcActMass").value = act.kind === "torio" ? 40 : act.kind === "kcl" ? 500 : ACT_PRESETS[act.preset].mass;
      actRender();
    });
    for (const id of ["rcActKclType", "rcActBg"]) $(id).addEventListener("change", actRender);
    $("rcActStart").addEventListener("click", actStart);
    $("rcActNext").addEventListener("click", actNext);
    $("rcActCancel").addEventListener("click", async () => { await api.CancelActivity(); actLoad(); });
    $("rcActCal").addEventListener("click", async (e) => { const b = e.target.closest("[data-forget]"); if (!b) return; await api.ForgetActivityCal(b.dataset.forget); actLoad(); });
    new ResizeObserver(() => { if (visible) drawAll(); }).observe($("viewRadiacode"));
    // Espectro: espectro (acumulado) ou espectrograma (com a dureza)
    const sv = $("rcSpecView");
    const setSv = (v) => {
      sv.querySelectorAll("button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.v === v)));
      document.querySelectorAll("[data-specview]").forEach((el) => { el.hidden = el.dataset.specview !== v; });
      try { localStorage.setItem("rcSpecView", v); } catch (e) { /* sem armazenamento */ }
      if (visible) requestAnimationFrame(drawAll);
    };
    sv.addEventListener("click", (e) => { const b = e.target.closest("button[data-v]"); if (b) setSv(b.dataset.v); });
    let sv0 = "acc";
    try { sv0 = localStorage.getItem("rcSpecView") === "tempo" ? "tempo" : "acc"; } catch (e) { /* sem armazenamento */ }
    setSv(sv0);
    spMark();
  }
  function spMark() {
    document.querySelectorAll("#rcSpecScen button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.s === sp.scen)));
    document.querySelectorAll("#rcSpecMode button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.m === rl.mode)));
    $("rcSpecScen").hidden = rl.on;
    $("rcSpecMode").hidden = !rl.on;
    $("rcSpecSpeedBox").hidden = rl.on;
    $("rcSpecSimTag").hidden = rl.on;
    $("rcSpecLiveTag").hidden = !rl.on;
    $("rcSpecDesc").textContent = rl.on
      ? "Ao vivo, do seu RC102: é o mesmo espectro da tela do aparelho. Cada fóton que chega é contado no canal da energia dele, e os picos de cada isótopo vão aparecendo. Para medir uma amostra, encoste-a e clique em Recomeçar (o aparelho zera junto). A linha tracejada é o fundo da casa (o acumulado do aparelho) no mesmo tempo; os isótopos achados ficam marcados em cima dos picos. Aqui só aparece o que está acima do fundo da casa; os isótopos do próprio fundo aparecem no “Acumulado do aparelho”, depois de algumas horas. Passe o mouse num pico para ver qual isótopo costuma aparecer ali. O morro grande à esquerda, abaixo de uns 200 keV, não é um isótopo: são fótons que perderam energia no caminho (espalhados), e aparece em todo espectro."
      : "Cada fóton que chega é contado no canal da energia dele. Com o tempo, os picos de cada isótopo aparecem. Passe o mouse num pico para ver qual isótopo costuma aparecer ali. A linha tracejada é o fundo esperado no mesmo tempo.";
    $("rcSpecLog").textContent = sp.log ? "Escala: log" : "Escala: linear";
    $("rcSpecPlay").textContent = sp.run ? "Pausar" : "Continuar";
    $("rcSpecReset").textContent = rl.on && rl.mode === "acc" ? "Zerar o acumulado…" : "Recomeçar";
  }
  function drawAll() { spDraw(); sgDraw(); hdDraw(); srDraw(); if (act.v) actRender(); }
  function show() {
    build();
    visible = true;
    realTick(true);
    sgLoad();
    lgLoad();
    actLoad();
    if (sr.real && sr.mode) srSound(sr.mode);
    drawAll();
    timers.forEach(clearInterval);
    timers = [
      setInterval(() => { if (sp.run && !rl.on) { spStep(sp.speed * 0.25); spDraw(); } }, 250),
      setInterval(() => realTick(), 2000),
      setInterval(() => { if (tl.run && tl.shown < TL_N) { tl.shown++; if (!sg.on) sgDraw(); hdDraw(); if (tl.shown >= TL_N) tl.run = false; } }, 110),
      setInterval(sgLoad, 60e3),
      setInterval(() => lgLoad(), 30e3),
      setInterval(() => { const p = act.v && act.v.phase; if (p === "fundo" || p === "amostra" || Date.now() - act.at > 10e3) { act.at = Date.now(); actLoad(); } }, 3000),
      setInterval(srTick, 500),
    ];
  }
  function hide() {
    visible = false;
    timers.forEach(clearInterval);
    timers = [];
    if (sr.osc) { try { sr.osc.stop(); } catch (e) { /* já parou */ } sr.osc = null; }
    if (sr.ctx) sr.ctx.suspend();
  }
  window.rcDemo = { show, hide, redraw: () => { if (visible) drawAll(); } };
})();
