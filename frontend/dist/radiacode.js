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
  const EN = Float64Array.from({ length: NCH }, (_, i) => chE(i));
  const DE = Float64Array.from({ length: NCH }, (_, i) => chE(i + 0.5) - chE(i - 0.5));
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
  function spDraw() {
    const cv = $("rcSpec"); if (!cv) return;
    const f = fit(cv, 260); if (!f) return;
    const { g, w, h } = f, L = 44, R = 10, T = 10, B = 24, pw = w - L - R, ph = h - T - B;
    g.clearRect(0, 0, w, h);
    const EMAX = 3000, xOf = (e) => L + (Math.max(0, e) / EMAX) * pw;
    let max = 1; for (let i = 0; i < NCH; i++) if (EN[i] > 25 && sp.counts[i] > max) max = sp.counts[i];
    const yOf = sp.log ? (v) => T + ph - (Math.log10(1 + v) / Math.log10(1 + max * 1.15)) * ph : (v) => T + ph - (v / (max * 1.1)) * ph;
    // grade
    g.strokeStyle = css("--border"); g.lineWidth = 1; g.fillStyle = css("--dim"); g.font = "11px " + css("--mono");
    g.textAlign = "center";
    for (let e = 0; e <= EMAX; e += 500) { const x = xOf(e); g.beginPath(); g.moveTo(x + 0.5, T); g.lineTo(x + 0.5, T + ph); g.stroke(); g.textAlign = e === EMAX ? "right" : "center"; g.fillText(e === 0 ? "0" : e + "", e === EMAX ? x + 6 : x, h - 7); }
    g.textAlign = "right";
    const ticks = sp.log ? [0, 1, 10, 100, 1000, 10000, 100000].filter((v) => v <= max * 1.15) : [0, max * 0.25, max * 0.5, max * 0.75, max].map(Math.round);
    for (const v of ticks) { const y = yOf(v); g.globalAlpha = 0.5; g.beginPath(); g.moveTo(L, y + 0.5); g.lineTo(L + pw, y + 0.5); g.stroke(); g.globalAlpha = 1; g.fillText(v >= 1000 ? nf(v / 1000, v >= 10000 ? 0 : 1) + "k" : String(v), L - 6, y + 4); }
    // fundo esperado no mesmo tempo
    if (sp.bg && sp.scen !== "fundo" && sp.t > 0) {
      g.beginPath();
      for (let i = 0; i < NCH; i++) { const x = xOf(EN[i]), y = yOf(SHAPE.fundo[i] * sp.t); i ? g.lineTo(x, y) : g.moveTo(x, y); }
      g.strokeStyle = css("--muted"); g.setLineDash([4, 3]); g.stroke(); g.setLineDash([]);
    }
    // espectro
    const acc = "#F2A33A";
    g.beginPath(); g.moveTo(xOf(EN[0]), T + ph);
    for (let i = 0; i < NCH; i++) { if (EN[i] > EMAX) break; g.lineTo(xOf(EN[i]), yOf(sp.counts[i])); }
    g.lineTo(xOf(Math.min(EMAX, EN[NCH - 1])), T + ph); g.closePath();
    g.fillStyle = "rgba(242,163,58,.55)"; g.fill(); g.strokeStyle = acc; g.lineWidth = 1; g.stroke();
    // cursor
    if (sp.hover != null) {
      const x = xOf(sp.hover); g.strokeStyle = "#B79CFF"; g.beginPath(); g.moveTo(x + 0.5, T); g.lineTo(x + 0.5, T + ph); g.stroke();
    }
    g.fillStyle = css("--dim"); g.textAlign = "right"; g.fillText(LANG === "en" ? "energy (keV)" : "energia (keV)", L + pw, T + 12);
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
    let ch = 0; for (let i = 0; i < NCH; i++) if (Math.abs(EN[i] - e) < Math.abs(EN[ch] - e)) ch = i;
    const near = libNear(e);
    const lib = near.length ? near.map(([le, n, d]) => `<b>${esc(n)}</b> ${nf(le, 1)} keV${d ? ` <span class="muted">(${esc(d)})</span>` : ""}`).join("<br>") : '<span class="muted">nenhum pico conhecido aqui</span>';
    tip($("rcSpecTip"), `<span class="mono">${nf(e, 0)} keV · canal ${ch} · ${Math.round(sp.counts[ch])} contagens</span><br>${lib}`, x, ev.clientY - r.top);
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
    const cv = $("rcSgram"), r = cv.getBoundingClientRect();
    const t = Math.floor(((ev.clientY - r.top) / r.height) * TL_N), e = ((ev.clientX - r.left) / r.width) * SG_EMAX;
    if (t < 0 || t >= tl.shown) { tl.hover = null; tip($("rcSgTip"), null); sgDraw(); return; }
    tl.hover = t;
    const ev2 = tlLabel(t), near = libNear(e);
    tip($("rcSgTip"), `<span class="mono">min ${t} · ${nf(tl.cps[t], 1)} cps · ${nf(e, 0)} keV</span>${ev2 ? `<br>${ev2}` : ""}${near.length ? `<br><b>${esc(near[0][1])}</b> ${nf(near[0][0], 1)} keV` : ""}`, ev.clientX - r.left, ev.clientY - r.top);
    sgDraw(); hdDraw();
  }

  function hdDraw() {
    const cv = $("rcHard"); if (!cv) return;
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
    const cv = $("rcHard"), r = cv.getBoundingClientRect(), L = 40, R = 44, pw = r.width - L - R;
    const t = Math.round(((ev.clientX - r.left - L) / pw) * (TL_N - 1));
    if (t < 0 || t >= tl.shown) { tl.hover = null; tip($("rcHardTip"), null); hdDraw(); sgDraw(); return; }
    tl.hover = t;
    const ev2 = tlLabel(t), lab = tl.label[t];
    tip($("rcHardTip"), `<span class="mono">min ${t} · ${nf(tl.cps[t], 1)} cps · dureza ${nf(tl.hard[t], 2)}</span>${ev2 ? `<br>${ev2}` : ""}${lab ? `<br>parece: <b>${esc(lab)}</b>` : ""}`, ev.clientX - r.left, ev.clientY - r.top);
    hdDraw(); sgDraw();
  }

  // =====================================================================
  // 4. log
  // =====================================================================
  function logBuild() {
    const tb = $("rcLogBody"); if (!tb) return;
    tb.textContent = "";
    let t = Date.now() - 30 * 60e3;
    const rows = [];
    for (let k = 0; k < 10; k++) {
      const dt = 15 + Math.round(Math.random() * 30);
      const n = poisson(8.4 * dt), cps = n / dt, e = 100 / Math.sqrt(Math.max(1, n));
      rows.push([t, cps, e]); t += dt * 1000 + Math.random() * 60e3;
    }
    for (const [ts, cps, e] of rows.reverse()) {
      const tr = document.createElement("tr"), d = new Date(ts);
      const cells = [d.toLocaleDateString(LOCALE, { day: "2-digit", month: "2-digit" }) + " " + d.toLocaleTimeString(LOCALE),
        `${nf(cps / CPS_PER_USV, 2)} µSv/h ±${nf(e, 1)}%`, `${nf(cps, 1)} cps ±${nf(e, 1)}%`];
      for (const c of cells) { const td = document.createElement("td"); td.textContent = c; tr.appendChild(td); }
      tb.appendChild(tr);
    }
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
    $("rcSpecPlay").addEventListener("click", () => { sp.run = !sp.run; spMark(); });
    $("rcSpecReset").addEventListener("click", spReset);
    $("rcSpec").addEventListener("mousemove", spHover);
    $("rcSpec").addEventListener("mouseleave", () => { sp.hover = null; tip($("rcSpecTip"), null); spDraw(); });
    spRate();
    // linha do tempo
    tlBuild();
    $("rcTlReplay").addEventListener("click", () => { tlBuild(); tl.run = true; sgDraw(); hdDraw(); });
    $("rcSgram").addEventListener("mousemove", sgHover);
    $("rcSgram").addEventListener("mouseleave", () => { tl.hover = null; tip($("rcSgTip"), null); sgDraw(); hdDraw(); });
    $("rcHard").addEventListener("mousemove", hdHover);
    $("rcHard").addEventListener("mouseleave", () => { tl.hover = null; tip($("rcHardTip"), null); hdDraw(); sgDraw(); });
    const ax = $("rcSgAxis");
    for (let e = 0; e <= SG_EMAX; e += 250) { const s = document.createElement("span"); s.textContent = e; s.style.left = (e / SG_EMAX) * 100 + "%"; ax.appendChild(s); }
    // log
    logBuild();
    $("rcLogNew").addEventListener("click", logBuild);
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
    $("rcSpecLog").textContent = sp.log ? "Escala: log" : "Escala: linear";
    $("rcSpecPlay").textContent = sp.run ? "Pausar" : "Continuar";
  }
  function drawAll() { spDraw(); sgDraw(); hdDraw(); srDraw(); kclCalc(); }
  function show() {
    build();
    visible = true;
    drawAll();
    timers.forEach(clearInterval);
    timers = [
      setInterval(() => { if (sp.run) { spStep(sp.speed * 0.25); spDraw(); } }, 250),
      setInterval(() => { if (tl.run && tl.shown < TL_N) { tl.shown++; sgDraw(); hdDraw(); if (tl.shown >= TL_N) tl.run = false; } }, 110),
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
