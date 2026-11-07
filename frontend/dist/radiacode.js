// ---------- aba RadiaCode: exemplos simulados ----------
//
// Tudo aqui usa dados SIMULADOS, no mesmo formato que o RadiaCode 102 manda
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
  // ---------- espectro ao vivo (RadiaCode conectado) ----------
  // O aparelho junta o espectro dele o tempo todo; a tela mostra o que entrou
  // desde o "Recomeçar" (o atual menos a foto tirada naquela hora). O fundo
  // tracejado é o acumulado do aparelho (horas em casa), na escala do tempo.
  const rl = { on: false, mode: "agora", base: null, last: null, acc: null, accAt: 0, busy: false, peaks: [], peaksAt: 0, n: NCH };
  const rcLive = () => typeof state !== "undefined" && state.rc && state.rc.st && state.rc.st.state === "conectado" && typeof api !== "undefined" && api.GetRcSpectrum;
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
        if (!rl.base) rl.base = s;
        if (rl.n !== s.counts.length || !rl.cal || rl.cal.join() !== s.a.join()) { rl.n = s.counts.length; rl.cal = s.a; EN = calEN(s.a, rl.n); DE = Float64Array.from(EN, (e, i) => (i + 1 < rl.n ? EN[i + 1] - e : e - EN[i - 1])); }
      }
      if (!rl.acc || Date.now() - rl.accAt > 10 * 60e3) { const a = await api.GetRcSpectrum(true); if (a && a.counts && a.counts.length) { rl.acc = a; rl.accAt = Date.now(); } }
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
    // linhas que subiram acima do fundo (ao vivo)
    if (real && rl.mode === "agora") {
      g.font = "11px " + css("--mono"); g.textAlign = "left";
      let row = 0;
      for (const p of [...rl.peaks].sort((a, b) => a.kev - b.kev)) {
        const x = xOf(p.kev); g.strokeStyle = "#F0656A"; g.setLineDash([4, 3]); g.beginPath(); g.moveTo(x + 0.5, T); g.lineTo(x + 0.5, T + ph); g.stroke(); g.setLineDash([]);
        g.fillStyle = "#F0656A"; g.fillText(`${p.nuclide} ${nf(p.kev, 0)}`, Math.min(x + 3, L + pw - 70), T + 24 + (row++ % 3) * 12);
      }
    }
    // cursor
    if (sp.hover != null) {
      const x = xOf(sp.hover); g.strokeStyle = "#B79CFF"; g.beginPath(); g.moveTo(x + 0.5, T); g.lineTo(x + 0.5, T + ph); g.stroke();
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
      return;
    }
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

  // =====================================================================
  // 2 e 3. linha do tempo simulada: espectrograma e dureza
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
    tip($("rcSgTip"), `<span class="mono">min ${t} · ${nf(tl.cps[t], 1)} cps · ${nf(e, 0)} keV</span>${ev2 ? `<br>${ev2}` : ""}${near.length ? `<br><b>${esc(near[0][1])}</b> ${nf(near[0][0], 1)} keV` : ""}`, ev.clientX - r.left, ev.clientY - r.top);
    sgDraw(); hdDraw();
  }

  // ---------- espectrograma de verdade (o que o app gravou do aparelho) ----------
  // Com o RadiaCode conectado, o app guarda o espectro de cada minuto. Cada
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
      ? "Do seu RadiaCode: com ele conectado, o app guarda o espectro de cada minuto. Cada linha é um pedaço do tempo (o mais recente embaixo), a energia vai para a direita e a cor mostra quantos fótons chegaram naquela energia. Cinza escuro: sem dados. Linhas mais apagadas: o app estava fechado ou o aparelho desconectado, e a linha mostra a média do intervalo (o aparelho segue contando sozinho)."
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
      ? "Do seu RadiaCode, no mesmo período do espectrograma: contagem (verde, eixo da esquerda) e dureza (roxo, eixo da direita). Dureza é a proporção de fótons acima de 1.500 keV sobre os de 150 a 1.500 keV, relativa ao fundo da casa (a mediana das horas dos últimos 7 dias): 1,00 é igual ao fundo. Como chegam poucos fótons acima de 1.500 keV, cada ponto usa uns 20 minutos em volta; a faixa clara é a incerteza. Azul: chuva."
      : "Os mesmos 180 minutos: contagem (verde, eixo da esquerda) e dureza (roxo, eixo da direita). Dureza é a proporção de fótons acima de 1.500 keV sobre os de 150 a 1.500 keV, relativa ao fundo da casa: 1,00 é igual ao fundo. Como chegam poucos fótons acima de 1.500 keV, a dureza usa 20 minutos em volta de cada ponto.";
    $("rcHdUse").innerHTML = hdOn
      ? "<strong>Para você:</strong> se a contagem sobe e a dureza <b>cai</b>, costuma ser radônio (chuva, casa fechada: os filhos dele ficam quase todos abaixo de 1.500 keV). Se a dureza <b>sobe</b> junto, tem perto algo com linha alta, como o tório (Tl-208 em 2.614 keV). Passe o mouse: o app soma os espectros daquele trecho, compara com o fundo e diz com o que parece."
      : "<strong>Para você:</strong> as duas subidas aumentam a contagem, mas a chuva <b>abaixa</b> a dureza (os filhos do radônio ficam quase todos abaixo de 1.500 keV) e o tório <b>sobe</b> (o pico do Tl-208 em 2.614 keV). Passe o mouse: quando a subida é grande o bastante, o app diz com o que ela parece.";
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
      html += `<br><span class="mono">${nf(d.cps[k], 2)} cps · ${nf(e, 0)} keV · ${nf(row[b], 2)} por min</span>`;
      if (d.gap[k]) html += "<br>sem leitura nesse intervalo: média dele";
      else if (d.secs[k] < d.rowSec * 0.9) html += `<br>${nf((d.secs[k] / d.rowSec) * 100, 0)}% do tempo com dados`;
      if (near.length) html += `<br><b>${esc(near[0][1])}</b> ${nf(near[0][0], 1)} keV`;
    }
    tip($("rcSgTip"), html, ev.clientX - r.left, ev.clientY - r.top);
    sgDraw(); hdDraw();
  }

  function hdDraw() {
    const cv = $("rcHard"); if (!cv) return;
    if (sg.on && sg.hd) { hdDrawReal(cv); return; }
    const f = fit(cv, 200); if (!f) return;
    const { g, w, h } = f, L = 40, R = 44, T = 10, B = 22, pw = w - L - R, ph = h - T - B;
    g.clearRect(0, 0, w, h);
    const xOf = (t) => L + (t / (TL_N - 1)) * pw;
    // faixas dos eventos
    g.fillStyle = "rgba(96,165,250,.12)"; g.fillRect(xOf(RAIN[0]), T, xOf(RAIN[1]) - xOf(RAIN[0]), ph);
    g.fillStyle = "rgba(94,234,212,.16)"; g.fillRect(xOf(ELEC[0]), T, xOf(ELEC[1]) - xOf(ELEC[0]), ph);
    const cmax = Math.max(12, ...tl.cps.slice(0, Math.max(1, tl.shown))) * 1.1;
    const hs = tl.hard.slice(0, Math.max(1, tl.shown)), hmin = Math.min(0.6, ...hs) * 0.95, hmax = Math.max(1.4, ...hs) * 1.05;
    const yC = (v) => T + ph - (v / cmax) * ph, yH = (v) => T + ph - ((v - hmin) / (hmax - hmin)) * ph;
    g.strokeStyle = css("--border"); g.lineWidth = 1; g.font = "11px " + css("--mono");
    for (let k = 0; k <= 4; k++) {
      const y = T + (ph * k) / 4; g.beginPath(); g.moveTo(L, y + 0.5); g.lineTo(L + pw, y + 0.5); g.stroke();
      g.fillStyle = "#6EE7A0"; g.textAlign = "right"; g.fillText(nf(cmax * (1 - k / 4), 0), L - 6, y + 4);
      g.fillStyle = "#B79CFF"; g.textAlign = "left"; g.fillText(nf(hmax - ((hmax - hmin) * k) / 4, 2), L + pw + 6, y + 4);
    }
    g.fillStyle = css("--dim"); g.textAlign = "center";
    for (let t = 0; t <= TL_N; t += 30) g.fillText("min " + t, xOf(Math.min(t, TL_N - 1)), h - 6);
    // dureza 1,0 (fundo)
    g.setLineDash([3, 3]); g.strokeStyle = "rgba(183,156,255,.5)"; g.beginPath(); g.moveTo(L, yH(1)); g.lineTo(L + pw, yH(1)); g.stroke(); g.setLineDash([]);
    const line = (arr, yf, color) => {
      g.beginPath();
      for (let t = 0; t < tl.shown; t++) { const x = xOf(t), y = yf(arr[t]); t ? g.lineTo(x, y) : g.moveTo(x, y); }
      g.strokeStyle = color; g.lineWidth = 1.6; g.stroke(); g.lineWidth = 1;
    };
    line(tl.cps, yC, "#6EE7A0");
    line(tl.hard, yH, "#B79CFF");
    if (tl.hover != null && tl.hover < tl.shown) { const x = xOf(tl.hover); g.strokeStyle = "rgba(255,255,255,.5)"; g.beginPath(); g.moveTo(x + 0.5, T); g.lineTo(x + 0.5, T + ph); g.stroke(); }
  }
  function hdHover(ev) {
    if (sg.on && sg.hd) { hdHoverReal(ev); return; }
    const cv = $("rcHard"), r = cv.getBoundingClientRect(), L = 40, R = 44, pw = r.width - L - R;
    const t = Math.round(((ev.clientX - r.left - L) / pw) * (TL_N - 1));
    if (t < 0 || t >= tl.shown) { tl.hover = null; tip($("rcHardTip"), null); hdDraw(); sgDraw(); return; }
    tl.hover = t;
    const ev2 = tlLabel(t), lab = tl.label[t];
    tip($("rcHardTip"), `<span class="mono">min ${t} · ${nf(tl.cps[t], 1)} cps · dureza ${nf(tl.hard[t], 2)}</span>${ev2 ? `<br>${ev2}` : ""}${lab ? `<br>parece: <b>${esc(lab)}</b>` : ""}`, ev.clientX - r.left, ev.clientY - r.top);
    hdDraw(); sgDraw();
  }

  // ---------- dureza de verdade ----------
  const hdX = (w) => { const L = 40, R = 44; return { L, R, pw: w - L - R }; };
  function hdDrawReal(cv) {
    const d = sg.hd, f = fit(cv, 200); if (!f) return;
    const { g, w, h } = f, { L, pw } = hdX(w), T = 10, B = 22, ph = h - T - B, n = d.cps.length;
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
    const yC = (v) => T + ph - (v / cmax) * ph, yH = (v) => T + ph - ((v - hmin) / (hmax - hmin)) * ph;
    g.strokeStyle = css("--border"); g.lineWidth = 1; g.font = "11px " + css("--mono");
    for (let k = 0; k <= 4; k++) {
      const y = T + (ph * k) / 4; g.beginPath(); g.moveTo(L, y + 0.5); g.lineTo(L + pw, y + 0.5); g.stroke();
      g.fillStyle = "#6EE7A0"; g.textAlign = "right"; g.fillText(nf(cmax * (1 - k / 4), cmax < 20 ? 1 : 0), L - 6, y + 4);
      g.fillStyle = "#B79CFF"; g.textAlign = "left"; g.fillText(nf(hmax - ((hmax - hmin) * k) / 4, 2), L + pw + 6, y + 4);
    }
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
    line(d.cps, yC, "#6EE7A0");
    line(d.hard, yH, "#B79CFF");
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
    evento: '<svg viewBox="0 0 24 24" fill="#F5B84A" stroke="#F5B84A" stroke-width="1.6" stroke-linejoin="round"><path d="M6 21V4"/><path d="M6 4h11l-2.5 4L17 12H6z"/></svg>',
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
    const src = it.src === "celular" ? '<span class="src">celular</span>' : "";
    const when = `<td class="when mono"><b>${fmtDate(it.t)}</b><span>${fmtTime(it.t)}</span>${src}</td>`;
    let data = "";
    if (it.kind === "taxa") {
      data = `<td class="v mono" colspan="2"><span>${nf(it.usv, 3)} µSv/h ±${nf(it.usvErr || 0, 1)}%</span><span>${nf(it.cps, 1)} cps ±${nf(it.cpsErr || 0, 1)}%</span></td>`;
    } else if (it.kind === "raro") {
      const pw = it.power === "carregando" ? "carregando na fonte externa" : it.power === "externa" ? "na fonte externa" : "";
      const bat = it.hasBat ? `Bateria: ${nf(it.battery, 0)}%` : "";
      const l1 = it.hasTemp ? `<span>t = ${nf(it.temp, 1)} °C</span>` : "";
      const l2 = bat || pw ? `<span>${esc([bat, pw].filter(Boolean).join(" · "))}</span>` : "";
      const dose = it.hasDose ? `<span>Σ = ${nf(it.dose, 2)} µSv</span>${it.doseSec ? `<span class="mute">em ${fmtDur(it.doseSec)}</span>` : ""}` : "";
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
    const rate = srRate(), n = poisson(rate * 0.5), inst = n / 0.5;
    sr.val += 0.4 * (inst - sr.val);
    sr.trend.push(inst); if (sr.trend.length > 120) sr.trend.shift();
    if (sr.sound && sr.ctx) srClicks(Math.min(n, 60));
    srDraw();
  }
  function srClicks(n) {
    const ctx = sr.ctx, now = ctx.currentTime;
    if (!sr.buf) { const b = ctx.createBuffer(1, Math.round(ctx.sampleRate * 0.002), ctx.sampleRate); const d = b.getChannelData(0); for (let i = 0; i < d.length; i++) d[i] = (Math.random() * 2 - 1) * (1 - i / d.length); sr.buf = b; }
    for (let k = 0; k < n; k++) { const s = ctx.createBufferSource(); s.buffer = sr.buf; const gn = ctx.createGain(); gn.gain.value = 0.35; s.connect(gn).connect(ctx.destination); s.start(now + Math.random() * 0.5); }
  }
  function srDraw() {
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
    g.strokeStyle = "#F0656A"; g.lineWidth = 1.5; g.stroke(); g.lineWidth = 1;
    g.fillStyle = css("--dim"); g.font = "11px " + css("--mono"); g.textAlign = "right"; g.fillText(nf(m, 0) + " cps", w - 4, 12); g.fillText("60 s", w - 4, h - 6);
  }
  function gaugeSVG() {
    const cx = 120, cy = 120, r = 96, pt = (a, rr) => { const t = ((a - 90) * Math.PI) / 180; return [cx + rr * Math.cos(t), cy + rr * Math.sin(t)]; };
    let ticks = "", labels = "";
    for (let k = 0; k <= 50; k++) {
      const a = -120 + (k / 50) * 240, big = k % 10 === 0, [x1, y1] = pt(a, r), [x2, y2] = pt(a, r - (big ? 12 : 6));
      const col = k > 40 ? "#F0656A" : k > 30 ? "#F5B84A" : "#6EE7A0";
      ticks += `<line x1="${x1.toFixed(1)}" y1="${y1.toFixed(1)}" x2="${x2.toFixed(1)}" y2="${y2.toFixed(1)}" stroke="${col}" stroke-width="${big ? 2 : 1}" stroke-opacity="${big ? 1 : 0.7}"/>`;
      if (big) { const [lx, ly] = pt(a, r - 26); labels += `<text x="${lx.toFixed(1)}" y="${(ly + 4).toFixed(1)}">0</text>`; }
    }
    return `<svg viewBox="0 0 240 200" class="rc-gauge-svg" role="img" aria-label="Mostrador de contagem">
      ${ticks}<g id="rcGaugeTicks" class="rc-gauge-lbl">${labels}</g>
      <g id="rcNeedle"><line x1="120" y1="120" x2="120" y2="36" stroke="#F0656A" stroke-width="3" stroke-linecap="round"/></g>
      <circle cx="120" cy="120" r="7" fill="#F0656A"/>
      <text x="120" y="162" class="rc-gauge-num" id="rcGaugeVal">0</text>
      <text x="120" y="182" class="rc-gauge-unit">cps</text></svg>`;
  }

  // =====================================================================
  // 6. atividade com KCl
  // =====================================================================
  const BQ_PER_G_K = 31.2, Y1460 = 0.1066, KCL_K = 39.098 / 74.551;
  function kclCalc() {
    const typ = $("rcKType").value, m = Math.max(1, +$("rcKMass").value || 0), tmin = +$("rcKTime").value, eff = 0.001;
    $("rcKMgRow").hidden = typ !== "rotulo";
    const fracK = typ === "kcl" ? KCL_K : typ === "light" ? 0.5 * KCL_K : Math.max(0, +$("rcKMg").value || 0) / 1000;
    const A = m * fracK * BQ_PER_G_K, perKg = (A / m) * 1000;
    const gam = A * Y1460, S = gam * eff * 0.866; // dentro da janela de ±1,5σ
    const s = sig(1460.8), B = sumRange(SHAPE.fundo, 1460.8 - 1.5 * s, 1460.8 + 1.5 * s);
    const T = tmin * 60, rel = S > 0 ? Math.sqrt((S + 2 * B) / T) / S : Infinity;
    const tFor = (r) => (S > 0 ? (S + 2 * B) / (r * r * S * S) / 60 : Infinity);
    const tm = (x) => (!isFinite(x) ? "—" : x < 90 ? nf(x, 0) + " min" : nf(x / 60, 1) + " h");
    $("rcKTimeLbl").textContent = tm(tmin);
    $("rcKOut").innerHTML = `
      <div><span class="muted small">atividade esperada</span><b class="mono">${nf(A, 0)} Bq</b><span class="mono small">${nf(perKg / 1000, 1)} mil Bq/kg</span></div>
      <div><span class="muted small">gamas de 1.460 keV</span><b class="mono">${nf(gam, 0)} /s</b><span class="small muted">saindo do pacote</span></div>
      <div><span class="muted small">no pico (estimado)</span><b class="mono">${nf(S * 60, 1)} /min</b><span class="small muted">fundo na janela: ${nf(B * 60, 1)} /min</span></div>
      <div><span class="muted small">incerteza em ${tm(tmin)}</span><b class="mono">±${isFinite(rel) ? nf(rel * 100, 0) : "—"}%</b><span class="small muted">±10%: ${tm(tFor(0.1))} · ±5%: ${tm(tFor(0.05))}</span></div>`;
    kclDraw(S, B, T);
  }
  function kclDraw(S, B, T) {
    const cv = $("rcKSpec"); if (!cv) return;
    const f = fit(cv, 150); if (!f) return;
    const { g, w, h } = f, L = 36, R = 8, T0 = 8, Bm = 22, pw = w - L - R, ph = h - T0 - Bm, E1 = 1100, E2 = 1900;
    g.clearRect(0, 0, w, h);
    const s = sig(1460.8), bgc = [], sc = [], es = [];
    for (let i = 0; i < NCH; i++) if (EN[i] >= E1 && EN[i] <= E2) {
      const pk = S / 0.866 * Math.exp(-0.5 * ((EN[i] - 1460.8) / s) ** 2) / (s * 2.5066) * DE[i];
      es.push(EN[i]); bgc.push(poisson(SHAPE.fundo[i] * T)); sc.push(poisson((SHAPE.fundo[i] + pk) * T));
    }
    const max = Math.max(3, ...sc, ...bgc) * 1.15, xOf = (e) => L + ((e - E1) / (E2 - E1)) * pw, yOf = (v) => T0 + ph - (v / max) * ph;
    g.strokeStyle = css("--border"); g.fillStyle = css("--dim"); g.font = "11px " + css("--mono"); g.textAlign = "center";
    for (let e = 1200; e <= 1800; e += 200) { const x = xOf(e); g.beginPath(); g.moveTo(x + 0.5, T0); g.lineTo(x + 0.5, T0 + ph); g.stroke(); g.fillText(e + " keV", x, h - 6); }
    g.textAlign = "right"; g.fillText(Math.round(max / 1.15) + "", L - 4, T0 + 10); g.fillText("0", L - 4, T0 + ph);
    const path = (arr, col, fill) => { g.beginPath(); g.moveTo(xOf(es[0]), T0 + ph); arr.forEach((v, k) => g.lineTo(xOf(es[k]), yOf(v))); g.lineTo(xOf(es[es.length - 1]), T0 + ph); if (fill) { g.fillStyle = fill; g.fill(); } g.strokeStyle = col; g.stroke(); };
    path(sc, "#F2A33A", "rgba(242,163,58,.4)");
    path(bgc, "#A3ADB4", null);
    const x = xOf(1460.8); g.setLineDash([3, 3]); g.strokeStyle = "#B79CFF"; g.beginPath(); g.moveTo(x, T0); g.lineTo(x, T0 + ph); g.stroke(); g.setLineDash([]);
    g.fillStyle = "#B79CFF"; g.textAlign = "left"; g.fillText("K-40", x + 4, T0 + 12);
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
      if (rl.on) { rl.base = rl.last; rl.peaks = []; rl.peaksAt = Date.now(); spDraw(); realTick(true); return; }
      spReset();
    });
    $("rcSpecMode").addEventListener("click", (e) => {
      const b = e.target.closest("button[data-m]"); if (!b) return;
      rl.mode = b.dataset.m; spMark(); spDraw();
    });
    $("rcSpec").addEventListener("mousemove", spHover);
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
    // KCl
    for (const id of ["rcKType", "rcKMass", "rcKMg", "rcKTime"]) $(id).addEventListener("input", kclCalc);
    $("rcKType").addEventListener("change", kclCalc);
    new ResizeObserver(() => { if (visible) drawAll(); }).observe($("viewRadiacode"));
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
      ? "Ao vivo, do seu RadiaCode: cada fóton que chega é contado no canal da energia dele, e os picos de cada isótopo vão aparecendo. Conta desde o Recomeçar; para medir uma amostra, encoste-a e clique em Recomeçar. A linha tracejada é o fundo da casa (o acumulado do aparelho) no mesmo tempo; os riscos vermelhos são as linhas que subiram acima dele. Passe o mouse num pico para ver qual isótopo costuma aparecer ali."
      : "Cada fóton que chega é contado no canal da energia dele. Com o tempo, os picos de cada isótopo aparecem. Passe o mouse num pico para ver qual isótopo costuma aparecer ali. A linha tracejada é o fundo esperado no mesmo tempo.";
    $("rcSpecLog").textContent = sp.log ? "Escala: log" : "Escala: linear";
    $("rcSpecPlay").textContent = sp.run ? "Pausar" : "Continuar";
  }
  function drawAll() { spDraw(); sgDraw(); hdDraw(); srDraw(); kclCalc(); }
  function show() {
    build();
    visible = true;
    realTick(true);
    sgLoad();
    lgLoad();
    drawAll();
    timers.forEach(clearInterval);
    timers = [
      setInterval(() => { if (sp.run && !rl.on) { spStep(sp.speed * 0.25); spDraw(); } }, 250),
      setInterval(() => realTick(), 2000),
      setInterval(() => { if (tl.run && tl.shown < TL_N) { tl.shown++; if (!sg.on) sgDraw(); hdDraw(); if (tl.shown >= TL_N) tl.run = false; } }, 110),
      setInterval(sgLoad, 60e3),
      setInterval(() => lgLoad(), 30e3),
      setInterval(srTick, 500),
    ];
  }
  function hide() {
    visible = false;
    timers.forEach(clearInterval);
    timers = [];
    if (sr.ctx) sr.ctx.suspend();
  }
  window.rcDemo = { show, hide };
})();
