"use strict";

// Modo demonstração: a api falsa que a tela usa quando abre no navegador,
// sem o Go (prints do README, teste de tela em scripts/telas_test.js).
// Saiu de dentro do app.js na auditoria de 12/11. Carrega antes do app.js,
// que chama makeDemoApi() quando não acha o window.go.
// ---------- modo demonstração (só no navegador, sem o Go) ----------

var demoBus;
// ---------- espectro no modo demonstração (2.69) ----------
// cópia em JS do contínuo do SNIP (snip.go) e dos números que o Go devolve
// por pico (centro do que sobra, contínuo embaixo, meia janela)
const demoEn = (a, i) => a[0] + a[1] * (i + 0.5) + a[2] * (i + 0.5) ** 2;
const demoFW = (e) => 65 * Math.sqrt(Math.max(e, 1) / 661.7);
function demoSnip(sp) {
  const y = sp.obj, n = y.length, a = sp.a;
  const wd = (i) => { const e = demoEn(a, i), de = demoEn(a, i + 1) - e; return de <= 0 ? 1 : Math.max(1, Math.round(demoFW(e) / de)); };
  const sm = y.map((_, i) => { const k = wd(i); let s = 0, c = 0; for (let j = i - k; j <= i + k; j++) if (j >= 0 && j < n) { s += y[j]; c++; } return s / c; });
  let v = sm.map((x) => Math.log(Math.max(x, 0) + 1)), maxW = 0;
  for (let i = 0; i < n; i++) maxW = Math.max(maxW, wd(i));
  for (let p = maxW; p >= 1; p--) {
    const t = v.slice();
    for (let i = p; i < n - p; i++) { if (p > wd(i)) continue; const m = (v[i - p] + v[i + p]) / 2; if (m < t[i]) t[i] = m; }
    v = t;
  }
  return v.map((x) => Math.max(0, Math.exp(x) - 1));
}
function demoPeakDetail(sp, peaks) {
  const acc = sp.bgSec === 1, base = acc ? demoSnip(sp) : null, k = acc ? 0 : sp.objSec / sp.bgSec;
  return peaks.map((p) => {
    const h = 0.6 * demoFW(p.kev);
    let ce = 0, cw = 0, b = 0;
    for (let i = 0; i < sp.obj.length; i++) {
      const e = demoEn(sp.a, i); if (e < p.kev - h || e >= p.kev + h) continue;
      const bi = acc ? base[i] : sp.bg[i] * k, v = sp.obj[i] - bi;
      b += bi; if (v > 0) { ce += v * e; cw += v; }
    }
    return { ...p, center: cw > 0 ? ce / cw : 0, cont: Math.max(b, 1), half: h, side: 0 };
  });
}
function makeDemoApi() {
  let demoSync = { id: "a", name: "pc-sala", listen: false, listening: false, auto: true, peers: [
    { id: "b", name: "notebook", addrs: ["192.168.0.20:47862", "100.88.1.7:47862"], manualAddrs: [], lastSync: Date.now() - 4 * 6e4, lastTry: Date.now() - 4 * 6e4, lastResult: "trouxe: 12 minutos · levou: 1.440 minutos, 2 alertas" }] };
  // versões que o modo demonstração mostra em "Atualizações e sobre"; ao refazer
  // os prints do README, acerte para a versão atual e a seguinte (U4)
  let demoUpd = { current: "2.73.0", latest: "2.74.0", available: true, canAuto: true, url: "https://github.com/matheusAFreitas/radlog/releases",
    notes: "- **Atualização pelo app**: avisa quando sai versão nova.\n- Outra coisa melhor.", checkedAt: Date.now(), state: "" };
  demoBus = {};
  const emit = (n, d) => (demoBus[n] || []).forEach((f) => f(d));
  const now = Date.now();
  const pts = [];
  let seed = 7;
  const rnd = () => { seed = (seed * 9301 + 49297) % 233280; return seed / 233280; };
  for (let i = 7 * 24 * 60; i > 0; i--) {
    if (i > 3 * 24 * 60 && i < 4 * 24 * 60) continue; // um buraco de um dia
    pts.push({ t: Math.floor(now / 60000) * 60000 - i * 60000, cpm: Math.round(20 + (rnd() - 0.5) * 14) });
  }
  const dev = { port: "COM5", version: "GMC-800Re1.23", serial: "5A 10 33 38 31 30 03 00 48 36 37 31", clockDrift: 4, usvPerCPM: 0.0101, calibration: { source: "contador" } };
  let status = { state: "conectado", message: "", device: dev, downloading: false, dataDir: "C:\\Users\\voce\\AppData\\Roaming\\RadLog" };
  let demoUsageOn = true;
  let demoUsageReset = 0; // "Zerar contagem" no demo: some o exemplo
  let settings = { usageOn: true, levels: {}, port: "COM5", autoConnect: true, alarmEnabled: true, alarmCPM: 100, alarmSound: "sirene", alarmRaise: true };
  const demoCfg = {
    values: { saveMode: 2, powerSaving: 1, backlight: 10, led: 1, fastEstimate: 3, alarmVolume: 15, clickSound: 0, voiceVolume: 0, vibration: 1, alarmCPM: 100, theme: 4 },
    calibration: { source: "contador", points: [[986, 10], [8546, 100], [16408, 200], [37970, 500], [76010, 1000], [139671, 2000]].map(([cpm, usv]) => ({ cpm, usv })) },
    tubeVoltage: 44, locked: null, calibrationEditable: true, editable: true, problems: null };
  const events = [
    { time: now - 3600e3, text: "Contador conectado (GMC-800Re1.23 em COM5)", kind: "evento" },
    { time: now - 1800e3, text: "Histórico baixado: 1440 registros, 32 minutos novos", kind: "evento" },
    { time: now - 900e3, text: "Atenção: 54 CPM (acima de 50)", kind: "alerta", level: "atencao", cpm: 54, peak: 61, peakAt: now - 780e3, end: now - 540e3 },
    { time: now - 2 * 864e5 - 5 * 36e5, text: "Alarme (histórico do contador): pico de 134 CPM às 03:12 (limite 100)", kind: "alerta", level: "alarme", cpm: 58, peak: 134, peakAt: now - 2 * 864e5 - 4.6 * 36e5, end: now - 2 * 864e5 - 4.2 * 36e5, retro: true },
    { time: now - 3 * 864e5 - 9 * 36e5, text: "Atenção (histórico do contador): pico de 63 CPM às 10:40 (acima de 50)", kind: "alerta", level: "atencao", cpm: 52, peak: 63, peakAt: now - 3 * 864e5 - 8.8 * 36e5, end: now - 3 * 864e5 - 8.5 * 36e5, retro: true },
    { time: now - 6 * 864e5, text: "Alarme: 112 CPM (limite 100)", kind: "alerta", level: "alarme", cpm: 112, peak: 118, peakAt: now - 6 * 864e5 + 120e3, end: now - 6 * 864e5 + 600e3 },
    { time: now - 5 * 36e5, text: "Atenção no RC102: 0,412 µSv/h (alarme 1 do aparelho: 0,400)", kind: "alerta", level: "atencao", cpm: 0.412, peak: 1.62, peakAt: now - 5 * 36e5 + 200e3, end: now - 5 * 36e5 + 420e3, device: "radiacode" },
    { time: now - 5 * 36e5 + 150e3, text: "Alarme no RC102: 1,24 µSv/h (alarme 2 do aparelho: 1,20)", kind: "alerta", level: "alarme", cpm: 1.24, peak: 1.62, peakAt: now - 5 * 36e5 + 200e3, end: now - 5 * 36e5 + 260e3, device: "radiacode" },
  ].sort((x, y) => x.time - y.time).map((e, i) => ({ id: i + 1, ...e }));
  let demoAlerts = [], demoAlertSeq = 1000;
  const mt = now - 40 * 60e3;
  let measurements = [{ id: 1, name: "Banana com casca", note: "", time: mt,
    bg: { counts: 155, seconds: 300, cpm: 31, errCPM: 2.5, usv: 0.2, start: mt, end: mt + 300e3 },
    obj: { counts: 161, seconds: 300, cpm: 32.2, errCPM: 2.5, usv: 0.21, start: mt + 360e3, end: mt + 660e3 },
    result: { netCPM: 1.2, netErr: 3.6, netUSv: 0.012, ratio: 1.04, sigma: 0.33, verdict: "nenhuma diferença que dê para separar do fundo", level: "nada" },
    samples: (() => { const out = []; let cpm = 30; for (let i = 0; i < 600; i++) { const ph = i < 300 ? "fundo" : "objeto"; const t = (ph === "fundo" ? i : i + 60) * 1000; const cps = Math.random() < 0.52 ? 1 : 0; cpm = Math.max(10, Math.round(cpm + (Math.random() - 0.5) * 3)); out.push({ t, phase: ph, cpm, cps }); } return out; })() },
    { id: 2, name: "Medição antiga", note: "", time: mt - 864e5, bg: { counts: 26, seconds: 60, cpm: 26, errCPM: 5.1, usv: 0.17, start: mt - 864e5, end: mt - 864e5 + 60e3 },
      obj: { counts: 26, seconds: 60, cpm: 26, errCPM: 5.1, usv: 0.17, start: mt - 864e5 + 90e3, end: mt - 864e5 + 150e3 },
      result: { netCPM: 0, netErr: 7.2, netUSv: 0, ratio: 1, sigma: 0, verdict: "nenhuma diferença que dê para separar do fundo", level: "nada" } }].sort((a, b) => a.time - b.time);
  const mockWhere = () => {
    const now = Date.now();
    const o = periods.find((q) => now >= q.from && now < (q.to || Infinity));
    const g = o ? o.gmc || "comigo" : "casa", r = o ? o.radiacode || "comigo" : "comigo";
    const nh = !!(o && o.notHome && g === "casa" && r !== "fora");
    const pers = r === "comigo" ? "radiacode" : g === "comigo" ? "gmc" : g === "casa" && r === "casa" && !nh ? "gmc" : "", home = g === "casa" ? "gmc" : r === "casa" ? "radiacode" : "";
    return { gmc: g, radiacode: r, normal: g === "casa" && r === "comigo" && !nh, personal: pers, home, notHome: nh, since: o ? o.from : 0, open: o && !o.to ? o : null, live: mockLive() };
  };
  // estado ao vivo de mentira: ?whlive=gmc:zero,radiacode:off (zero, sat, off) para ver as falhas
  const mockLive = () => {
    const now = Date.now(), want = {};
    for (const kv of (new URLSearchParams(location.search).get("whlive") || "").split(",")) { const [d, v] = kv.split(":"); if (d) want[d] = v; }
    const one = (dev) => {
      const v = want[dev];
      if (v === "zero") return { connected: true, state: "sem contagens", since: now - 14 * 6e4, last: now, lastGood: now - 15 * 6e4 };
      if (v === "sat") return { connected: true, state: "saturado", since: now - 3 * 6e4, last: now, lastGood: now - 4 * 6e4 };
      if (v === "off") return { connected: false, state: "sem dado", since: 0, last: now - 42 * 6e4, lastGood: now - 42 * 6e4 };
      const on = dev === "gmc" || demoRcFake();
      return on ? { connected: true, state: "lendo", since: 0, last: now, lastGood: now } : { connected: false, state: "sem dado", since: 0, last: 0, lastGood: 0 };
    };
    return { gmc: one("gmc"), radiacode: one("radiacode") };
  };
  const d1 = new Date(now - 864e5).setHours(0, 0, 0, 0);
  let periods = [{ id: 1, name: "Passeio no parque", from: Math.floor((now - 2 * 864e5) / 36e5) * 36e5 + 2 * 36e5, to: Math.floor((now - 2 * 864e5) / 36e5) * 36e5 + 5.5 * 36e5 },
    { id: 2, name: "Centro e metrô", from: d1 + 13 * 36e5 + 10 * 6e4, to: d1 + 17 * 36e5 + 40 * 6e4, gmc: "casa", radiacode: "comigo", notHome: true }];
  const rcY = Math.floor((now - 864e5) / 6e4) * 6e4 - 3 * 36e5;
  const rcAsksAll = [{ id: rcY, from: rcY, to: rcY + 130 * 6e4, memMinutes: 128, avgUSv: 0.142, maxUSv: 0.231, maxAt: rcY + 57 * 6e4, doseUSv: 0.30, gmcUSv: 0.098, ratio: 41, normalRatio: 28, hint: "fora" }];
  // demonstração da pergunta "voltou para casa?" (2.66.0): localStorage rcBack=1
  if (localGet("rcBack", "") === "1") rcAsksAll.splice(0, 1, { id: rcY, from: rcY, to: rcY + 130 * 6e4, kind: "volta", period: 99, since: rcY + 4 * 6e4, name: "Saí de casa", memMinutes: 128, avgUSv: 0.142, maxUSv: 0.231, maxAt: rcY + 57 * 6e4, doseUSv: 0.30, gmcUSv: 0.098, ratio: 41, normalRatio: 28, hint: "fora" });
  // saída programada (2.75): localStorage gmc.rcPlan=1 mostra a pergunta, rcPlan=2 a "não aconteceu?"
  const plY = new Date(now - 3 * 864e5).setHours(5, 30, 0, 0);
  // o nome é o que a pessoa escreveu (não passa pela tradução): no demo em inglês já vem em inglês
  const plDemoName = LANG === "en" ? "Office" : "Escritório";
  const plMode = localGet("rcPlan", "");
  if (plMode) rcAsksAll.splice(0, 1, { id: plY, from: plMode === "2" ? plY : plY + 11 * 6e4, to: plY + 13 * 36e5 + 2 * 6e4, kind: "programada", planned: plY, name: plDemoName, rcGoes: true, gmcGoes: false, planAt: plY, source: plMode === "2" ? "plano" : "razao", estimated: plMode === "2", missed: plMode === "2", backHint: plMode === "2" ? 0 : plY + 11 * 36e5 + 53 * 6e4, dev: "radiacode", memMinutes: 780, avgUSv: 0.13, maxUSv: 0.2, maxAt: plY + 3 * 36e5, doseUSv: 1.7, gmcUSv: 0.098, ratio: 39, normalRatio: 28, hint: "fora" });
  let rcAsks = rcAsksAll.slice();
  const plTomorrow = new Date(now + 864e5).setHours(5, 30, 0, 0);
  let planned = [{ id: plTomorrow, nome: plDemoName, de: plTomorrow, rc: true, gmc: false }];
  let rainBusy = false, rainFetched = 0;
  const rainStatus = () => ({ enabled: !!settings.rainEnabled, place: settings.rainPlace || "", busy: rainBusy, fetched: rainFetched, next: rainFetched ? rainFetched + 18e5 : 0, error: "", radarFetched: rainFetched, radarCovered: rainFetched ? true : null, radarNow: rainFetched ? { time: Math.floor(Date.now() / 6e5) * 6e5, mmh: 0, dbz: 0 } : null });
  let notes = [{ id: 1, time: Math.floor(now / 6e4) * 6e4 - 35 * 6e4, text: "abri a janela", created: now }], noteSeq = 1;
  let demoSt = { active: false, cpm: 20, realistic: false };
  const withPeaks = (c) => {
    c.peaks = events.concat(demoAlerts).filter((e) => e.kind === "alerta" && e.peak && (e.peakAt || e.time) >= c.from && (e.peakAt || e.time) < c.to)
      .map((e) => ({ t: e.peakAt || e.time, cpm: e.peak, level: e.level, retro: !!e.retro }));
    for (const p of c.peaks) if (c.max != null && p.cpm > c.max) c.max = p.cpm;
    return c;
  };
  const chartRangePreview = (from, to) => api._chartRange(from, to);
  setInterval(() => {
    if (demoSt.active) {
      const c = demoSt.cpm;
      const dl = (settings.levels || {}).dangerCPM || 1000, al = (settings.levels || {}).attentionCPM || 50;
      const want = c >= settings.alarmCPM ? ["alarme"] : c >= al ? ["atencao"] : [];
      if (c >= settings.alarmCPM && c >= al) want.push("atencao");
      if (c >= dl) want.unshift("perigo");
      let changed = false;
      for (const e of demoAlerts) if (e.active && !want.includes(e.level)) { e.active = false; e.end = Date.now(); changed = true; }
      for (const lv of want) if (!demoAlerts.some((e) => e.active && e.level === lv)) {
        demoAlerts.push({ id: ++demoAlertSeq, time: Date.now(), kind: "alerta", level: lv, cpm: c, peak: c, peakAt: Date.now(), active: true, demo: true,
          text: lv === "perigo" ? `[demo] Perigo: ${c} CPM (a partir de ${dl})` : lv === "alarme" ? `[demo] Alarme: ${c} CPM (limite ${settings.alarmCPM})` : `[demo] Atenção: ${c} CPM (a partir de ${al})` });
        changed = true;
      }
      if (changed) { emit("event", { kind: "alerta" }); }
      emit("reading", { time: Date.now(), cpm: c, cps: pulsePoisson(c / 60), usv: c * 0.0101, mrh: c * 0.00101, level: c >= dl ? "perigo" : c >= 100 ? "alarme" : c >= 50 ? "atencao" : "seguro", alarm: settings.alarmEnabled && c >= settings.alarmCPM, danger: c >= dl, saturated: c * 0.0101 >= 2000, satReason: "limite", rise: c >= dl ? { fromUSv: 0.13, seconds: 3 } : null, demo: true, attention: c >= al, bgCPM: 20, sigma: (c - 20) / Math.sqrt(20) });
      feed(Math.max(0, Math.round(c / 60 + (Math.random() - 0.5) * 2)));
      emit("realReading", { time: Date.now(), cpm: 21, cps: 0, usv: 0.137, mrh: 0.0137, level: "seguro", alarm: false });
      return;
    }
    if (status.state !== "conectado") return;
    const cpm = Math.round(20 + (Math.random() - 0.5) * 12);
    emit("reading", { time: Date.now(), cpm, cps: pulsePoisson(cpm / 60), usv: cpm * 0.0101, mrh: cpm * 0.00101, level: cpm >= 100 ? "alarme" : cpm >= 50 ? "atencao" : "seguro", alarm: false, attention: cpm >= 50, bgCPM: 20, sigma: (cpm - 20) / Math.sqrt(20) });
    // os dois perto: o GMC vê mais o objeto (beta)
    feed(meas.other && meas.other.role === "objeto" && meas.phase === "objeto" ? (Math.random() < 0.9 ? 1 : 0) + (Math.random() < 0.3 ? 1 : 0) : Math.random() < 0.33 ? 1 : 0);
  }, 1000);
  const range = (from, to) => pts.filter((p) => p.t >= from && p.t < to);
  let meas = { phase: "", seconds: 0, elapsed: 0, counts: 0, name: "", demo: false, bg: null, obj: null, result: null };
  const phaseOf = (counts, secs) => { const m = secs / 60, cpm = counts / m; return { counts, seconds: secs, cpm, errCPM: Math.sqrt(Math.max(counts, 1)) / m, usv: cpm * (meas.device === "radiacode" ? 0.092 / 718 : 0.0101) }; };
  // mesmas contas do measure.go / measureauto.go
  const compare = (b, ph) => {
    const net = ph.cpm - b.cpm, err = Math.hypot(b.errCPM, ph.errCPM), z = net / err;
    const level = z >= 3 ? "sim" : z >= 2 ? "talvez" : "nada";
    return { netCPM: net, netErr: err, netUSv: ph.usv - b.usv, ratio: ph.cpm / b.cpm, sigma: z, level,
      verdict: { sim: "radioativo: a diferença para o fundo é clara", talvez: "provavelmente um pouco radioativo; meça por mais tempo para confirmar", nada: "nenhuma diferença que dê para separar do fundo" }[level] };
  };
  const thrOf = (b, pct) => Math.max(1, b.cpm * pct / 100);
  const etaOf = (b, ph, el, max, pct) => {
    if (el < 5) return -1;
    const net = ph.cpm - b.cpm, rate = Math.max(ph.cpm, b.cpm);
    let need = Infinity;
    const q1 = (net / 4) ** 2 - b.errCPM ** 2;
    if (net > 0 && q1 > 0) need = Math.min(need, rate / q1 * 60);
    const d = thrOf(b, pct) - Math.max(net, 0), q2 = (d / 2) ** 2 - b.errCPM ** 2;
    if (d > 0 && q2 > 0) need = Math.min(need, rate / q2 * 60);
    need = Math.max(need, 20);
    return !isFinite(need) || need > max ? -1 : Math.ceil(need) - el;
  };
  function feedSide(cps, usv) {
    const o = meas.other;
    o.nowUSv = usv;
    if (o.role === "voce" && meas.phase !== "objeto") return;
    if (o.role === "objeto" && meas.phase === "fundo" && o.bg) return;
    o.counts += cps; o.elapsed++; o.usvSum = (o.usvSum || 0) + usv; o.nowUSv = usv;
    const m = o.elapsed / 60, cpm = o.counts / m;
    const ph = { counts: o.counts, seconds: o.elapsed, cpm, errCPM: Math.sqrt(Math.max(o.counts, 1)) / m, usv: o.usvSum / o.elapsed };
    if (meas.phase === "objeto") {
      o.maxUSv = Math.max(o.maxUSv || 0, usv);
      if (o.role === "objeto" && o.bg) o.live = compare(o.bg, ph);
      if (o.role === "voce") { o.avgUSv = ph.usv; o.dose = ph.usv * o.elapsed / 3600; o.excess = o.bgUSv ? (ph.usv - o.bgUSv) * o.elapsed / 3600 : 0; }
    }
    o.cur = ph;
  }
  function endSide(phase) {
    const o = meas.other;
    if (!o || !o.cur) return;
    if (phase === "fundo" && o.role === "objeto") o.bg = o.cur;
    if (phase === "objeto") {
      o.obj = o.cur;
      if (o.role === "objeto" && o.bg) {
        o.result = compare(o.bg, o.cur); o.live = null;
        const g = o.device === "gmc" ? o.result : meas.result, r = o.device === "gmc" ? meas.result : o.result;
        o.hint = g.sigma >= 3 && r.sigma < 2 ? "O GMC viu o objeto e o RC102 quase não: ele deve emitir principalmente beta (o cristal do RC102 quase não vê beta)."
          : g.sigma >= 3 && r.sigma < 3 ? "O GMC viu claramente e o RC102 só um pouco: o objeto deve emitir bastante beta."
          : g.sigma >= 3 && r.sigma >= 3 ? (g.ratio - 1 > 2 * (r.ratio - 1) ? `Os dois viram, mas o GMC subiu bem mais (${fmt(g.ratio, 1)}× o fundo contra ${fmt(r.ratio, 1)}× no RC102): além de gama, o objeto deve emitir beta.` : "Os dois viram, numa proporção parecida: o objeto deve emitir principalmente gama.")
          : r.sigma >= 3 ? "Só o RC102 viu: ele é bem mais sensível a gama que o tubo do GMC, então o objeto deve emitir gama fraco." : "Nenhum dos dois viu uma diferença clara.";
      }
    }
    o.counts = 0; o.elapsed = 0; o.usvSum = 0; o.cur = null;
  }
  function feed(cps, dev) {
    if (meas.phase !== "fundo" && meas.phase !== "objeto") return;
    if (meas.other && (dev || "gmc") === meas.other.device) { feedSide(cps, dev === "radiacode" ? cps / 120 : cps * 60 * 0.0065); return; }
    if ((meas.device || "gmc") !== (dev || "gmc")) return;
    meas.counts += cps; meas.elapsed++;
    let stop = "";
    if (meas.phase === "objeto") {
      const part = phaseOf(meas.counts, meas.elapsed);
      meas.live = compare(meas.bg, part);
      if (meas.stopReq && meas.elapsed >= 5) { stop = "manual"; meas.stopReq = false; }
      else if (meas.relErr > 0 && meas.elapsed >= 20 && meas.live.netCPM > 0 && meas.live.netErr <= meas.relErr * meas.live.netCPM) stop = "precisao";
      else if (meas.auto) {
        if (meas.elapsed >= 20 && meas.live.sigma >= 4) stop = "achou";
        else if (meas.elapsed >= 20 && Math.max(meas.live.netCPM, 0) + 2 * meas.live.netErr < thrOf(meas.bg, meas.pct)) stop = "nada";
        else if (meas.elapsed >= meas.seconds) stop = "limite";
        meas.eta = etaOf(meas.bg, part, meas.elapsed, meas.maxSec, meas.pct);
      }
    }
    if (meas.elapsed >= meas.seconds || stop) {
      const ph = phaseOf(meas.counts, meas.elapsed);
      const endedPh = meas.phase;
      if (meas.phase === "fundo") { meas.bg = ph; meas.phase = "fundo-pronto"; endSide("fundo"); }
      else {
        meas.obj = ph; meas.phase = "pronto";
        const r = compare(meas.bg, ph);
        r.stop = stop;
        if (r.level !== "sim") r.limit = Math.max(r.netCPM, 0) + 2 * r.netErr;
        if (stop === "nada") { r.level = "nada"; r.verdict = `igual ao fundo: se o objeto emite algo, é menos que +${usvTxt(r.limit * (ph.cpm > 0 ? ph.usv / ph.cpm : 0.0101))}`; }
        if (stop === "limite" && r.level !== "sim") r.verdict += " (chegou ao tempo máximo sem decidir)";
        if (meas.auto) { r.pct = meas.pct; r.threshold = thrOf(meas.bg, meas.pct); }
        meas.result = r; meas.live = null; meas.eta = -1;
        if (meas.device === "radiacode" || (meas.other && meas.other.device === "radiacode")) {
          meas.spectrum = demoMeasSpec(meas.elapsed, meas.bg.source === "casa");
          if (meas.device !== "radiacode") meas.spectrum.role = "voce";
        }
        if (endedPh === "objeto") endSide("objeto");
        meas.derived = demoDerive(meas);
      }
    }
    emit("measure", { ...meas });
  }
  // Radiacode medindo no modo navegador: fundo ~12 cps, objeto ~19 cps
  setInterval(() => {
    const rcIn = meas.device === "radiacode" || (meas.other && meas.other.device === "radiacode");
    if (!rcIn || !demoRcFake() || (meas.phase !== "fundo" && meas.phase !== "objeto")) return;
    let lam = meas.other && meas.other.device === "radiacode" && meas.other.role === "voce" ? 14 : meas.phase === "objeto" ? 19 : 12;
    // testes do ALARA: a fonte cai com a distância (quase k/r²) e o material corta 30%
    if (meas.phase === "objeto" && meas.test === "distancia" && meas.distCm > 0) lam = 12 + 40 * (25 + 4) / (meas.distCm ** 2 + 4);
    if (meas.phase === "objeto" && meas.test === "blindagem") lam = 12 + 20 * ((meas.prev || []).length ? 0.7 : 1);
    let k = 0, p = 1; const L = Math.exp(-lam); for (;;) { p *= Math.random(); if (p <= L) break; k++; }
    emit("radiacode", { ...demoRc(), cps: lam + (Math.random() - 0.5) * 2 });
    feed(k, "radiacode");
  }, 1000);
  // testes do ALARA (alara.go), para o modo navegador
  const demoDist = (m, k) => {
    const pts = (m.prev || []).filter((p) => p.distCm > 0 && p.errUSv > 0).map((p) => ({ cm: p.distCm, netUSv: p.netUSv, errUSv: p.errUSv, seconds: p.seconds }));
    if (m.distCm > 0) pts.push({ cm: m.distCm, netUSv: m.result.netUSv, errUSv: m.result.netErr * k, seconds: m.obj.seconds });
    pts.sort((a, b) => a.cm - b.cm);
    const f = { points: pts, k: 0, chi2: 0 };
    if (pts.length < 2) return f;
    let sxy = 0, sxx = 0;
    for (const p of pts) { const x = 1 / p.cm ** 2, w = 1 / p.errUSv ** 2; sxy += w * x * p.netUSv; sxx += w * x * x; }
    f.k = sxy / sxx;
    let chi = 0;
    for (const p of pts) { p.predict = f.k / p.cm ** 2; p.lost = p.netUSv < 2 * p.errUSv; chi += ((p.netUSv - p.predict) / p.errUSv) ** 2; }
    f.chi2 = chi / (pts.length - 1);
    const slow = pts.length >= 3 && pts.slice(2).every((p) => p.netUSv > p.predict + p.errUSv);
    if (f.k <= 0) Object.assign(f, { verdict: "torto", title: "Não deu para ver a fonte.", text: "Nas distâncias medidas a taxa não ficou acima do fundo. Aproxime a fonte ou meça mais tempo." });
    else if (f.chi2 <= 2) Object.assign(f, { verdict: "seguiu", title: "Seguiu o inverso do quadrado.", text: "Cada vez que a distância dobrou, a taxa acima do fundo caiu para cerca de ¼. A fonte se comporta como pequena: afastar um palmo já corta muito." });
    else if (slow) Object.assign(f, { verdict: "devagar", title: "Caiu mais devagar que o previsto.", text: "Longe, a taxa ficou acima da curva. A fonte deve ser grande ou espalhada (piso, bancada, várias peças). Perto de uma fonte assim, afastar ajuda menos. Bem longe dela, a regra do quadrado volta a valer." });
    else Object.assign(f, { verdict: "torto", title: "Não bateu bem com a curva.", text: "Pode ter sido a posição entre uma medida e outra. Vale repetir com o aparelho sempre na mesma linha, medindo do centro do cristal." });
    const lost = pts.filter((p) => p.lost).map((p) => fmt(p.cm, 0) + " cm");
    if (lost.length) f.text += ` A ${lost.join(" e ")} a fonte quase sumiu no fundo: esse ponto pesa pouco na conta.`;
    return f;
  };
  const demoShield = (m, k) => {
    const a = (m.prev || [])[m.prev.length - 1];
    if (!a) return null;
    const s = { material: m.material || "", beforeUSv: a.netUSv, beforeErr: a.errUSv, afterUSv: m.result.netUSv, afterErr: m.result.netErr * k, beforeSecs: a.seconds, afterSecs: m.obj.seconds };
    if (s.beforeUSv <= 2 * s.beforeErr) return Object.assign(s, { title: "Sem nada no meio, a fonte não ficou acima do fundo.", text: "Assim não dá para medir o corte. Aproxime a fonte do aparelho ou meça mais tempo em cada fase." });
    const q = s.afterUSv / s.beforeUSv, rel = s.beforeErr / s.beforeUSv;
    s.cut = 1 - q;
    s.cutErr = s.afterUSv > 0 ? Math.abs(q) * Math.hypot(s.afterErr / s.afterUSv, rel) : Math.hypot(s.afterErr / s.beforeUSv, rel);
    s.seen = Math.abs(s.cut) >= 2 * s.cutErr;
    const pc = (v) => fmt(Math.round(v * 100), 0) + "%";
    if (!s.seen) Object.assign(s, { title: "Não deu para ver diferença.", text: `A diferença (${pc(s.cut)} ± ${pc(s.cutErr)}) ficou menor que duas vezes a incerteza. Meça mais tempo em cada fase, ou o material corta pouco essa radiação.` });
    else if (s.cut > 0) Object.assign(s, { title: `Cortou ${pc(s.cut)} ± ${pc(s.cutErr)}.`, text: `Com o material no meio, a taxa acima do fundo caiu de ${fmt(s.beforeUSv, usvDigits(s.beforeUSv))} para ${fmt(Math.max(s.afterUSv, 0), usvDigits(Math.max(s.afterUSv, 0)))} µSv/h, na mesma posição.` });
    else Object.assign(s, { title: "Subiu com o material no meio.", text: "Confira se o aparelho ficou no mesmo lugar nas duas fases: mudar a distância muda mais que o material." });
    return s;
  };
  // contas do resultado (measderive.go), para o modo navegador
  const demoDerive = (m) => {
    const r = m.result, k = m.obj.cpm > 0 ? m.obj.usv / m.obj.cpm : 0, real = r.level === "sim" && r.netUSv > 0, d = m.distCm || 0;
    const out = { netUSv: r.netUSv, errUSv: r.netErr * k, bgUSv: m.bg.usv, safe: { ok: false } };
    let inv = null;
    const pv = (m.prev || []).filter((p) => p.distCm > 0 && p.level === "sim" && Math.abs(p.distCm - d) >= 0.5).pop();
    if (pv && real && d > 0) {
      const ratio = pv.netUSv / r.netUSv, expect = (d / pv.distCm) ** 2, pow = Math.log(ratio) / Math.log(d / pv.distCm), okp = Math.abs(pow - 2) <= 0.35;
      const a = `${fmt(Math.max(ratio, 1 / ratio), 1)}×`, b = `${fmt(Math.max(expect, 1 / expect), 1)}×`, dd = `De ${fmt(pv.distCm, 0)} cm para ${fmt(d, 0)} cm a radiação caiu`;
      inv = { ok: okp, power: pow, text: okp ? `${dd} ${a} (o inverso do quadrado dá ${b}): o objeto se comporta como uma fonte pequena, e a distância segura e o Bq ficam mais confiáveis.`
        : pow < 2 ? `${dd} menos que o inverso do quadrado (${a} contra ${b}): o objeto deve ser espalhado, então a distância segura é só aproximada.`
        : `${dd} mais que o inverso do quadrado (${a} contra ${b}): confira as distâncias; o detector fica alguns milímetros para dentro da caixa.` };
      out.invSq = inv;
    }
    if (m.test === "distancia") out.dist = demoDist(m, k);
    if (m.test === "blindagem") out.shield = demoShield(m, k);
    if (!real) out.safe.why = "o objeto não ficou claramente acima do fundo";
    else if (!(d > 0)) out.safe.why = "informe a distância (encostado não dá para usar na conta)";
    else { const x = d * Math.sqrt(r.netUSv / (0.1 * m.bg.usv)); out.safe = { ok: true, cm: Math.max(x, d), below: x < d, approx: m.size !== "pequeno" || !inv || !inv.ok }; }
    if (m.device !== "radiacode") return out;
    out.isotopes = m.spectrum && m.spectrum.role !== "voce" ? [{ group: "Tório-232 (série)", nuclides: ["Pb-212", "Tl-208"], kev: [238.6, 583.2, 2614.5], lines: 3, of: 4, conf: 0.84 }] : [];
    const top = out.isotopes[0], g = top ? { "Tório-232 (série)": 0.2 }[top.group] || 0 : 0;
    const checks = [
      { text: "medido com o RC102 no objeto (tem espectro)", ok: true }, { text: "objeto claramente acima do fundo", ok: real },
      { text: "isótopo identificado com confiança", ok: !!top && top.conf >= 0.6 && g > 0 }, { text: "distância informada", ok: d > 0 },
      { text: "objeto pequeno, como uma fonte pontual", ok: m.size === "pequeno" && !(inv && !inv.ok && inv.power < 2) }];
    const bq = { ok: checks.every((c) => c.ok), checks };
    if (bq.ok) { bq.bq = (r.netUSv * (d / 100) ** 2 / g) * 1e6; bq.lo = bq.bq / 2; bq.hi = bq.bq * 2; bq.group = top.group; bq.gamma = g; }
    out.bq = bq;
    return out;
  };
  // procurando o ponto mais forte no modo navegador: o RC102 fingido sobe e
  // desce como se estivesse chegando perto do objeto e se afastando
  setInterval(() => {
    if (!demoRcFake() || typeof state === "undefined" || !state.mBusca) return;
    const near = 0.5 + 0.5 * Math.sin(Date.now() / 4000), usvh = 0.092 + 0.42 * near * near + (Math.random() - 0.5) * 0.01;
    emit("radiacode", { ...demoRc(), usvh, cps: usvh / 0.092 * 12 });
  }, 1000);
  // espectro de mentira: contínuo + linhas do tório (camisa de lampião)
  const demoSpecArr = (sec, cont, peaks) => {
    const out = [];
    for (let i = 0; i < 1024; i++) {
      const e = (i + 0.5) * 3;
      let r = cont * Math.exp(-e / 350) / 3;
      for (const [pe, pr] of peaks) { const sg = 65 * Math.sqrt(pe / 661.7) / 2.355; r += pr * 3 / (sg * 2.5066) * Math.exp(-(((e - pe) / sg) ** 2) / 2); }
      const l = r * sec;
      out.push(Math.max(0, Math.round(l + Math.sqrt(l) * (Math.random() + Math.random() + Math.random() - 1.5) * 2)));
    }
    return out;
  };
  const demoMeasSpec = (objSec, home) => {
    const bgPk = [[1460.8, 0.08], [609.3, 0.05], [351.9, 0.05]];
    return { a: [0, 3, 0], bgSrc: home ? "acumulado" : "", bgSec: home ? 50 * 3600 : objSec, objSec,
      bg: demoSpecArr(home ? 50 * 3600 : objSec, 0.3, bgPk), obj: demoSpecArr(objSec, 0.8, [[238.6, 3], [583.2, 1], [911.2, 0.5], [2614.5, 0.4], ...bgPk]),
      peaks: [{ group: "Tório-232 (série)", nuclide: "Pb-212", kev: 238.6, net: 260, sigma: 11 }, { group: "Tório-232 (série)", nuclide: "Tl-208", kev: 583.2, net: 80, sigma: 5.2 }, { group: "Tório-232 (série)", nuclide: "Tl-208", kev: 2614.5, net: 30, sigma: 4.1 }] };
  };
  {
    // uma medição salva do Radiacode, com espectro
    const t0 = Date.now() - 3 * 3600e3, sec = 95, samples = [];
    let cpm = 2300;
    for (let i = 0; i < sec; i++) { cpm = Math.max(1500, cpm + (Math.random() - 0.5) * 60); samples.push({ t: (i + 1) * 1000, phase: "objeto", cpm: Math.round(cpm), cps: Math.round(38 + (Math.random() - 0.5) * 12) }); }
    measurements.push({ id: 3, name: "Camisa de lampião antiga", note: "", time: t0, device: "radiacode", distance: "encostado",
      bg: { counts: 40926, seconds: 3420, cpm: 718, errCPM: 3.5, usv: 0.092, start: t0 - 3600e3, end: t0 - 60e3, source: "casa" },
      obj: { counts: 3790, seconds: sec, cpm: 2393.7, errCPM: 38.9, usv: 0.31, start: t0, end: t0 + sec * 1000 },
      result: { netCPM: 1675.7, netErr: 39.1, netUSv: 0.218, ratio: 3.33, sigma: 42.9, level: "sim", verdict: "radioativo: a diferença para o fundo é clara", stop: "achou", pct: 20, threshold: 143.6 },
      samples, spectrum: demoMeasSpec(sec, true) });
  }
  const demoSpecLive = { counts: null, seconds: 0, at: 0 };
  const demoRcCfg = { off: false, ble: "" };
  // localStorage rcFake=1: finge um Radiacode conectado (para ver a tela no demo)
  const usvEv = (e) => (e.kind !== "alerta" || e.device === "radiacode" ? e
    : { ...e, usvPerCPM: 0.0101, usv: e.cpm ? e.cpm * 0.0101 : 0, peakUSv: e.peak ? e.peak * 0.0101 : 0 });
  const demoRcFake = () => { try { return localStorage.getItem("rcFake") === "1"; } catch { return false; } };
  // medição de atividade de mentira (para ver a tela sem o aparelho)
  const demoAct = {
    run: null,
    bg: null,
    cal: { geometry: "pote de 500 mL encostado", kcl: { t: Date.now() - 3 * 86400e3, eff: 0.00182, effErr: 0.031, massG: 500, pct: 100, sec: 3600 }, th: null },
    hist: [],
    nucs: [
      { id: "K-40", name: "Potássio-40", kevs: [1460.8] }, { id: "Cs-137", name: "Césio-137", kevs: [661.7] },
      { id: "Ra-226", name: "Rádio-226 (série do urânio)", note: "pelos filhos Pb-214 e Bi-214: a amostra precisa ficar fechada ~3 semanas antes, para o radônio não escapar", kevs: [351.9, 609.3, 1764.5] },
      { id: "Ra-228", name: "Rádio-228 (início da série do tório)", note: "pelo Ac-228, que acompanha o Ra-228 em poucos dias", kevs: [911.2, 969] },
      { id: "Th-232", name: "Tório-232 (série, em equilíbrio)", kevs: [238.6, 583.2, 911.2, 2614.5] }, { id: "U-238", name: "Urânio-238 (urânio natural)", kevs: [1001] },
    ],
    sec() { return this.run ? Math.min(this.run.target || 1e9, ((Date.now() - this.run.at) / 1000) * 120) : 0; },
    start(o) {
      if (!demoRcFake()) throw new Error("RC102 não conectado");
      if (!(o.massG > 0)) throw new Error("informe a massa em gramas");
      o.name = o.name || (o.kind === "kcl" ? "KCl (" + o.pct + "%)" : o.kind === "torio" ? "eletrodo de tório (" + o.pct + "% ThO₂)" : "amostra");
      this.run = { o, phase: o.newBg ? "fundo" : "amostra", at: Date.now(), target: o.newBg ? o.bgSec : o.smpSec, bgSec: o.newBg ? 0 : 7200 };
    },
    next() {
      const r = this.run; if (!r) return;
      if (r.phase === "fundo") { r.bgSec = this.sec(); r.phase = "trocar"; this.bg = { t: Date.now(), sec: r.bgSec }; }
      else if (r.phase === "trocar") { r.phase = "amostra"; r.at = Date.now(); r.target = r.o.smpSec; }
      else if (r.phase === "amostra") {
        r.sec = this.sec(); r.phase = "pronto"; r.result = this.result(r.sec); this.hist.unshift(r.result);
        if (r.o.kind === "kcl") this.cal.kcl = { ...r.result.kcl };
        if (r.o.kind === "torio") this.cal.th = { ...r.result.th };
      }
    },
    nucRes(sec, massG, ids) {
      const f = Math.sqrt(3600 / Math.max(sec, 60));
      const base = { "K-40": [180, 0.08], "Ra-226": [58, 0.35], "Ra-228": [40, 0.6], "Th-232": [25, 0.9], "Cs-137": [0, 0], "U-238": [0, 0] };
      return ids.map((id) => {
        const nu = this.nucs.find((n) => n.id === id) || { id, name: id }, [bq] = base[id] || [0], rel = (base[id] || [0, 0])[1], kg = massG / 1000;
        const stat = bq ? bq * rel * f : 6 * f, det = bq > 1.645 * stat, err = Math.hypot(stat, bq * 0.05);
        return { id, name: nu.name, note: nu.note || "", bq, err, statErr: stat, detected: det, mda: 4.65 * stat, bqKg: bq / kg, errKg: err / kg, mdaKg: (4.65 * stat) / kg,
          needSec: bq ? sec * (stat / bq / 0.1) ** 2 : 0, lines: (nu.kevs || []).map((e) => ({ kev: e, who: id, bq: bq * (0.9 + Math.random() * 0.2), err: stat * 1.4 })) };
      });
    },
    result(sec) {
      const o = this.run.o, r = { t: Date.now(), kind: o.kind, name: o.name || (o.kind === "kcl" ? "KCl (" + o.pct + "%)" : "eletrodo de tório"), massG: o.massG, geometry: o.geometry, sec, bgSec: this.run.bgSec, calSrc: "kcl", nuclides: [] };
      if (o.kind === "kcl") { r.expected = (o.massG * o.pct) / 100 * 0.52445 * 31.7; r.kcl = { t: r.t, eff: 0.00179 + Math.random() * 0.00008, effErr: 0.03 * Math.sqrt(3600 / sec), massG: o.massG, pct: o.pct, sec }; r.prev = this.cal.kcl && this.cal.kcl.eff; }
      else if (o.kind === "torio") { r.th = { t: r.t, b: -1.24, bErr: 0.07, pivot: 583.2 }; r.thLines = [{ kev: 238.6, area: 41230, areaErr: 410 }, { kev: 583.2, area: 9120, areaErr: 260 }, { kev: 2614.5, area: 2210, areaErr: 120 }]; r.nuclides = this.nucRes(sec, o.massG, ["Th-232"]); }
      else r.nuclides = this.nucRes(sec, o.massG, o.nuclides);
      return r;
    },
    view() {
      const cal = this.cal, effAt = (e) => (cal.kcl ? cal.kcl.eff : 0.001) * Math.pow(e / 1460.8, cal.th ? cal.th.b : -1.3);
      const E = [250, 500, 1000, 1500, 2000, 2500];
      const v = { phase: "", cal, calSrc: cal.kcl ? "kcl" : "chute", calEff: E.map(effAt), calErr: E.map((e) => Math.hypot(cal.kcl ? cal.kcl.effErr : 1, (cal.th ? 0.07 : 0.4) * Math.log(e / 1460.8))),
        savedBg: this.bg || { t: Date.now() - 86400e3, sec: 7200 }, history: this.hist, nuclides: this.nucs, connected: demoRcFake() };
      if (this.run && (this.run.phase === "fundo" || this.run.phase === "amostra") && this.run.target > 0 && this.sec() >= this.run.target) this.next();
      const r = this.run; if (!r) return v;
      v.opts = r.o; v.phase = r.phase; v.phaseAt = r.at; v.bgSec = r.bgSec;
      if (r.phase === "fundo" || r.phase === "amostra") { v.phaseSec = this.sec(); v.target = r.target; }
      if (r.phase === "amostra" && v.phaseSec > 10) {
        if (r.o.kind === "kcl") { v.liveKcl = { eff: 0.0018, effErr: 0.03 * Math.sqrt(3600 / v.phaseSec) }; v.expected = 8300; }
        else if (r.o.kind === "torio") v.liveTh = { b: -1.24, bErr: 0.07 * Math.sqrt(3600 / v.phaseSec) };
        else v.live = this.nucRes(v.phaseSec, r.o.massG, r.o.nuclides);
      }
      if (r.phase === "pronto") v.result = r.result;
      const pk = (e, c, a) => a * Math.exp(-0.5 * ((e - c) / (27.6 * Math.sqrt(c / 661.7))) ** 2);
      const bgf = (e) => 0.02 * Math.exp(-e / 300) + 0.0004 + pk(e, 1460.8, 0.0012) + pk(e, 609.3, 0.0015) + pk(e, 351.9, 0.0018);
      const smpf = (e) => bgf(e) * 1.25 + pk(e, 1460.8, 0.002) + pk(e, 609.3, 0.0012) + pk(e, 351.9, 0.0013);
      const mk = (fn) => Array.from({ length: 300 }, (_, i) => fn((i + 0.5) * 10) * (0.92 + Math.random() * 0.16));
      if (r.phase !== "trocar") v.chart = { eMax: 3000, bg: mk(bgf), smp: r.phase === "fundo" ? null : mk(smpf) };
      return v;
    },
  };
  const demoFirst = () => Date.now() - 12 * 864e5;
  const rcHeat = (h) => { const k = 0.0061, sc = (rows) => rows.map((r) => r.map((v) => (v == null ? null : v * k))); return Object.assign({}, h, { device: "radiacode", values: sc(h.values), usv: sc(h.values), awayValues: sc(h.awayValues || []), hourAvg: h.hourAvg.map((v) => (v == null ? null : v * k)), min: h.min == null ? null : h.min * k, max: h.max == null ? null : h.max * k, alerts: {} }); };
  const demoCal = { retro: 0, vers: [{ id: 1, desde: 0, fator: 0, origem: "início", pontos: [] }, { id: 2, desde: Date.now() - 2 * 86400e3, fator: 0, origem: "contador", pontos: [], nota: "a curva do contador mudou" }] };
const demoRc = () => {
    const st = demoRcReal();
    if (!(demoSt && demoSt.active && demoSt.rc)) return st;
    const u = demoSt.rcUSv || 0, l = st.limits || { attn: 0.4, alarm: 1.2, danger: 10.1 };
    return Object.assign({ limits: l, stats: { bgUSv: 0.097 } }, st, { state: "conectado", via: st.state === "conectado" ? st.via : "demo", demo: true, time: Date.now(), usvh: u * (0.97 + Math.random() * 0.06), cps: u * 85, usvhErr: 8,
      alertLevel: u >= l.danger ? "perigo" : u >= l.alarm ? "alarme" : u >= l.attn ? "atencao" : "", limits: l });
  };
  const demoRcReal = () => demoRcFake() && !demoRcCfg.off
    ? { alertLevel: localStorage.getItem("rcAlert") || "", limits: { attn: 0.4, alarm: 1.2, danger: 10.1, device: true }, today: 0.84, lastT: Date.now() - 60e3, lastUSv: 0.098, lastCPS: 3.2, attnUSv: 0.505, alarmUSv: 1.01, dangerUSv: 10.1,
        alarms: { CountRate1: 20, CountRate2: 60, DoseRate1: 0.4, DoseRate2: 1.2, Dose1: 99999.99, Dose2: 99999.99, ShowSv: true, ShowCPM: true },
        stats: { bgUSv: 0.097, bgLo: 0.085, bgHi: 0.11, bgMinutes: 4000, avg1h: 0.099, n1h: 60, min24h: 0.071, max24h: 0.142, max24hAt: Date.now() - 5 * 3600e3, n24h: 1300, todayMinutes: 862 }, state: localStorage.getItem("rcFake2") ? "procurando" : demoRcCfg.st || "conectado", via: demoRcCfg.via || "usb", model: "RadiaCode-102", serial: "RC-102-001234", firmware: "4.14 (Sep 10 2026)", time: Date.now(), cps: 3.1 + Math.random() * 0.3, cpsErr: 9, usvh: ({ alarme: 1.38, perigo: 12.4, atencao: 0.47 }[localStorage.getItem("rcAlert")] || 0.095) + Math.random() * 0.01, usvhErr: 11, dose: 2.41, doseSec: 93600, doseLive: 2.41 + (Date.now() % 600000) / 3.6e7, doseLiveSec: 93600, doseLiveAt: Date.now(), doseLiveOn: !localStorage.getItem("rcFake2"), battery: 87, tempC: 24.5, hasRare: true, ble: demoRcCfg.ble, bleOK: true, off: false }
    : { state: demoRcCfg.off ? "off" : "procurando", ble: demoRcCfg.ble, bleOK: true, off: demoRcCfg.off };
  const demoHomeBg = () => ({ ok: true, cpm: 20.4, errCPM: 0.6, minutes: 58, counts: 1183, from: Date.now() - 3600e3, to: Date.now() - 60e3, demo: true });
  const chartPreview = (period) => api._unused(period);
  let api;
  api = {
    ...demoIntercal(),
    ...demoCounterCal(),
    ...demoRcAjustes(),
    GetSettings: async () => settings,
    GetAutostartStatus: async () => ({ available: true, on: !!settings.startWithWindows, path: "C:\\Users\\voce\\Downloads\\RadLog-v2.0.0.exe", exists: false, current: false, disabled: false, started: false }),
    FixAutostart: async () => ({}),
    SaveSettings: async (s) => { settings = s; setTimeout(() => emit("rain-status", rainStatus()), 0); return s; },
    GetStatus: async () => status,
    GetLogs: async (q) => {
      const now = Date.now(), from = q.from || now - 3 * 86400e3, to = q.to || now;
      const msgs = [["INFO", "conexão", "conexão: ok, GMC-800Re1.23 em COM5, série 5A 10 33 38 31 30 03 00 48 36 37 31"], ["INFO", "leitura ao vivo", "leitura ao vivo: começou"],
        ["INFO", "histórico", "histórico: terminou"], ["WARN", "leitura ao vivo", "leitura ao vivo: erro 1/3 em GETCPM: Access is denied."],
        ["DEBUG", "reconexão", "reconexão: COM5 não encontrada"], ["DEBUG", "conexão", "conexão: abrindo COM5"],
        ["ERROR", "leitura ao vivo", "leitura ao vivo: GETCPM sem resposta em 6 s (porta travada)"], ["INFO", "evento", "evento: Histórico baixado: 1440 registros, 12 minutos novos"]];
      let all = [];
      for (let t = from - (from % 600e3); t < to; t += 600e3) {
        const k = Math.floor(t / 600e3);
        const m = msgs[(k * 7) % msgs.length];
        if (m[0] === "ERROR" && k % 9) continue;
        all.push({ t: t + (k % 60) * 1000, level: m[0], comp: m[1], msg: m[2] });
      }
      const txt = (q.text || "").toLowerCase();
      all = all.filter((e) => !txt || e.msg.toLowerCase().includes(txt));
      const counts = {}, comps = {};
      for (const e of all) comps[e.comp] = (comps[e.comp] || 0) + 1;
      all = all.filter((e) => !q.comp || e.comp === q.comp);
      for (const e of all) counts[e.level] = (counts[e.level] || 0) + 1;
      const hits = all.filter((e) => !q.levels.length || q.levels.includes(e.level));
      const steps = [60e3, 300e3, 900e3, 3600e3, 3 * 3600e3, 6 * 3600e3, 86400e3];
      const step = steps.find((s) => (to - from) / s <= 72) || 7 * 86400e3;
      const start = from - (from % step), n = Math.floor((to - start) / step) + 1;
      const buckets = Array.from({ length: n }, (_, i) => ({ t: start + i * step, counts: {} }));
      for (const e of hits) { const i = Math.floor((e.t - start) / step); if (buckets[i]) buckets[i].counts[e.level] = (buckets[i].counts[e.level] || 0) + 1; }
      return { entries: hits.slice().reverse().slice(0, q.limit || 1000), total: hits.length, counts, comps: Object.entries(comps).map(([name, count]) => ({ name, count })).sort((a, b) => b.count - a.count), buckets, bucketMs: step, from, to };
    },
    ExportReport: async () => { throw new Error("no modo navegador não dá para salvar arquivo; abra pelo app"); },
    ExportReportWith: async () => { throw new Error("no modo navegador não dá para salvar arquivo; abra pelo app"); },
    GetReportDir: async () => ({ dir: settings.reportDir || "C:\\Users\\voce\\Documents\\RadLog\\Relatórios", default: !settings.reportDir }),
    ChooseReportDir: async () => { settings.reportDir = "D:\\Radiação\\Relatórios"; return api.GetReportDir(); },
    ResetReportDir: async () => { settings.reportDir = ""; return api.GetReportDir(); },
    OpenReportDir: async () => {},
    // como o servidor (2.29): alertas do GMC levam o fator da hora e o pico em µSv/h
    GetEvents: async (kind) => events.concat(demoAlerts).filter((e) => !kind || e.kind === kind).map(usvEv).sort((x, y) => y.time - x.time),
    GetAlertDetail: async (id) => {
      const e = events.concat(demoAlerts).find((x) => x.id === id);
      if (!e) throw new Error("alerta não encontrado");
      if (e.device === "radiacode") {
        const end = e.end || Date.now(), from = e.time - 120e3, to = Math.min(Date.now(), end + 120e3);
        const lim = e.level === "alarme" ? 1.2 : 0.4;
        const f = (t) => (t >= e.time && t <= end ? lim + (e.peak - lim) * Math.exp(-Math.abs(t - e.peakAt) / 90e3) : 0.15) * (1 + (Math.random() - 0.5) * 0.12);
        const seconds = [], minutes = [];
        for (let t = from; t < to; t += 750) seconds.push({ t, cpm: f(t) });
        for (let t = Math.floor(from / 6e4) * 6e4; t < to; t += 6e4) minutes.push({ t, cpm: f(t + 3e4) * 0.9 });
        return { event: e, from, to, end, seconds, minutes, limit: lim, attention: 0.4, alarm: 1.2, danger: 10.1, detailed: true, device: "radiacode" };
      }
      const end = e.active || !e.end ? Date.now() : e.end;
      const lim = +((e.text.match(/(?:a partir de|limite|acima de) (\d+)/) || [])[1] || 50);
      const live = !e.retro && Date.now() - e.time < 7 * 864e5;
      const pad = live ? Math.min(Math.max((end - e.time) / 4, 120e3), 18e5) : Math.min(Math.max(end - e.time, 6e5), 72e5);
      const from = e.time - pad, to = Math.min(Date.now(), end + pad);
      const bump = (t) => (t >= e.time && t <= end ? lim + (e.peak - lim) * Math.exp(-Math.abs(t - (e.peakAt || e.time)) / Math.max(60e3, (end - e.time) / 3)) : 22 + 4 * Math.sin(t / 7e4));
      const seconds = [], minutes = [];
      if (live) for (let t = Math.ceil(from / 1e3) * 1e3; t < to; t += 2e3) seconds.push({ t, cpm: Math.max(5, Math.round(bump(t) + (Math.random() - 0.5) * 6)) });
      for (let t = Math.floor(from / 6e4) * 6e4; t < to; t += 6e4) minutes.push({ t, cpm: Math.round(bump(t + 3e4)) });
      const K = 0.0101, cv = (l) => l.map((q) => ({ ...q, cpm: q.cpm * K }));
      return { event: usvEv({ ...e, active: !!e.active }), from, to, end, seconds: cv(seconds), minutes: cv(minutes), limit: lim * K, limitCPM: lim, attention: 50 * K, alarm: (settings.alarmCPM || 100) * K, danger: 1000 * K, detailed: live, unit: "usv" };
    },
    GetActiveAlerts: async () => {
      const o = { perigo: 0, alarme: 1, atencao: 2 };
      const lvl = demoRcFake() && localStorage.getItem("rcAlert");
      const rc = lvl ? [{ id: 990, time: Date.now() - 95e3, kind: "alerta", level: lvl, device: "radiacode", active: true, cpm: 1.25, peak: 1.41, peakAt: Date.now() - 40e3, text: "Alarme no RC102: 1,25 µSv/h (alarme 2 do aparelho: 1,20)" }] : [];
      return demoAlerts.filter((e) => e.active).map(usvEv).concat(rc).sort((x, y) => o[x.level] - o[y.level]);
    },
    ListPorts: async () => [{ name: "COM5", description: "USB-SERIAL CH340", isUSB: true }, { name: "COM1", description: "", isUSB: false }],
    DetectPort: async () => "COM5",
    Connect: async (p) => { status = { ...status, state: "conectado", device: { ...dev, port: p } }; emit("status", status); return status.device; },
    Disconnect: async () => { status = { ...status, state: "desconectado" }; emit("status", status); },
    SyncClock: async () => { dev.clockDrift = 0; emit("status", status); return 0; },
    DownloadHistory: async () => {
      const d = new Date(); d.setHours(0, 0, 0, 0);
      return { records: 900, added: 720, newRanges: [
        { from: d.getTime() + 1 * 36e5, to: d.getTime() + 7.5 * 36e5, minutes: 390, avgCPM: 19.6, rcSuggest: "casa" },
        { from: d.getTime() - 864e5 + 13 * 36e5, to: d.getTime() - 864e5 + 17.25 * 36e5, minutes: 255, avgCPM: 31.4, rcSuggest: "comigo" },
      ] };
    },
    CancelDownload: async () => {},
    ExportCSV: async () => "",
    OpenDataFolder: async () => {},
    OpenConfigBackups: async () => {},
    RestoreDeviceConfig: async () => "",
    SnapshotDeviceConfig: async () => {},
    GetDemo: async () => demoSt,
    StartDemo: async (cpm, realistic) => (demoSt = Object.assign({}, demoSt, { active: true, cpm, realistic })),
    SetDemoRc: async (on, usv) => { if (!demoSt.active) return demoSt; demoSt = Object.assign({}, demoSt, { rc: on, rcUSv: usv }); emit("radiacode", demoRc()); return demoSt; },
    StopDemo: async () => { demoAlerts = []; demoSt = { active: false, cpm: 20, realistic: false }; emit("event", { kind: "alerta" }); emit("demo", demoSt); return demoSt; },
    GetVersion: async () => "navegador",
    ExportMeasureReport: async (id) => { await new Promise((r) => setTimeout(r, 300)); return `C:\\Users\\voce\\Documents\\RadLog\\Relatórios\\gmc-medicao-${id}.pdf`; },
    OpenReportFile: async () => {},
    GetVersionsDir: async () => "C:\\Users\\voce\\Documents\\RadLog\\Versões",
    OpenVersionsDir: async () => {},
    GetSync: async () => demoSync,
    GetRadiacode: async () => demoRc(),
    SetRadiacode: async (on, ble) => {
      const nb = (ble || "").toUpperCase(), changed = nb && nb !== demoRcCfg.ble;
      demoRcCfg.off = !on; demoRcCfg.ble = nb;
      if (changed && demoRcFake()) {
        // finge a conexão pelo Bluetooth: conectando… e, 3 s depois, conectado
        demoRcCfg.via = "bluetooth"; demoRcCfg.st = "conectando";
        setTimeout(() => emit("radiacode", demoRc()), 50);
        setTimeout(() => { demoRcCfg.st = ""; emit("radiacode", demoRc()); }, 3000);
      }
      return demoRc();
    },
    ReconnectRadiacode: async () => demoRc(),
    ScanRadiacodeBLE: async () => { await new Promise((r) => setTimeout(r, 600)); return [{ address: "52:43:06:50:01:D7", name: "RadiaCode-102#RC-102-001234", rssi: -62 }]; },
    GetRcCompare: async (min) => {
      if (!demoRcFake()) return { minutes: 0, rcOnly: 0 };
      const n = Math.min(min, 600), t = [], gmc = [], rc = [];
      for (let i = 0; i < n; i++) { const g = 0.1 + 0.02 * Math.sin(i / 60) + (Math.random() - 0.5) * 0.04; t.push(Date.now() - (n - i) * 60e3); gmc.push(g); rc.push(g * 0.92 + (Math.random() - 0.5) * 0.02); }
      return { t, gmc, rc, gmcCPM: gmc.map((v) => v * 150), rcCPS: rc.map((v) => v * 30), ratio: 0.92, ratioErr: 0.01, corr: 0.81, minutes: n, rcOnly: 0, suggestK: 0.0055, curK: 0.01014 };
    },
    ResetRcSpectrum: async () => { if (!demoRcFake()) throw new Error("RC102 não conectado"); },
    GetRcSpectrum: async (accum) => {
      if (!demoRcFake()) throw new Error("RC102 não conectado");
      // espectro de mentira: fundo (acumulado, 7 h) e o atual, que vai
      // juntando com um eletrodo de tório encostado
      const A = [5.43, 2.351, 0.000381], En = (i) => A[0] + A[1] * (i + 0.5) + A[2] * (i + 0.5) ** 2;
      const rate = (i, th) => {
        const e = En(i); if (e < 20) return 0;
        const pk = (pe, r) => { const sg = 65 * Math.sqrt(pe / 661.7) / 2.355; return r * 2.35 / (sg * 2.5066) * Math.exp(-(((e - pe) / sg) ** 2) / 2); };
        let v = 0.035 * Math.exp(-e / 350) + pk(1460.8, 0.08) + pk(609.3, 0.06) + pk(351.9, 0.06) + pk(295.2, 0.03) + pk(238.6, 0.04) + pk(2614.5, 0.01) + pk(1764.5, 0.01);
        if (th) v += 0.09 * Math.exp(-e / 300) + pk(238.6, 3) + pk(583.2, 1) + pk(911.2, 0.5) + pk(2614.5, 0.4) + pk(75, 1.5);
        return v;
      };
      const pois = (l) => { if (l > 30) return Math.max(0, Math.round(l + Math.sqrt(l) * (Math.random() + Math.random() + Math.random() - 1.5) * 2)); let k = 0, p = 1; const L = Math.exp(-l); do { k++; p *= Math.random(); } while (p > L); return k - 1; };
      if (accum) {
        const sec = 26000, counts = []; for (let i = 0; i < 1024; i++) counts.push(pois(rate(i, false) * sec));
        return { seconds: sec, a: A, counts, accum: true };
      }
      const now = Date.now();
      if (!demoSpecLive.counts) { demoSpecLive.counts = new Array(1024).fill(0); demoSpecLive.seconds = 600; demoSpecLive.at = now; }
      const dt = Math.min(10, (now - demoSpecLive.at) / 1000) * 10; // 10× mais rápido
      demoSpecLive.at = now; demoSpecLive.seconds += dt;
      for (let i = 0; i < 1024; i++) demoSpecLive.counts[i] += pois(rate(i, true) * dt);
      return { seconds: Math.round(demoSpecLive.seconds), a: A, counts: demoSpecLive.counts.slice(), accum: false };
    },
    GetRcSpectrogram: async (from, to, rowSec, eMax) => {
      // espectrograma de mentira: fundo, uma chuva há ~5 h e um eletrodo de
      // tório perto há ~40 min; o app "fechado" de 20 a 14 h atrás
      const bins = 256, n = Math.ceil((to - from) / (rowSec * 1000)), now = Date.now();
      const first = demoRcFake() ? now - 30 * 3600e3 : 0;
      const out = { from, to, rowSec, eMax, bins, rows: [], secs: [], cps: [], gap: [], first };
      const pk = (e, pe, r) => { const sg = 65 * Math.sqrt(pe / 661.7) / 2.355; return r * Math.exp(-(((e - pe) / sg) ** 2) / 2); };
      const rnd = (k) => { const x = Math.sin(k * 12.9898) * 43758.5453; return x - Math.floor(x); };
      for (let k = 0; k < n; k++) {
        const t0 = from + k * rowSec * 1000, t1 = t0 + rowSec * 1000, ago = (now - t0) / 3600e3;
        if (!first || t1 <= first || t0 > now) { out.rows.push(null); out.secs.push(0); out.cps.push(0); out.gap.push(false); continue; }
        const rain = ago < 6 && ago > 2 ? Math.exp(-Math.abs(ago - 4.6) * 1.2) * 1.5 : 0, th = ago < 0.7 && ago > 0.4 ? 1 : 0, gap = ago > 14 && ago < 20;
        const row = [], f = rowSec / 60;
        let tot = 0;
        for (let b = 0; b < bins; b++) {
          const e = (b + 0.5) * eMax / bins; if (e < 25) { row.push(0); continue; }
          const lam = 6 * Math.exp(-e / 260) + 0.04 + pk(e, 1460.8, 0.25) + rain * (pk(e, 351.9, 1.6) + pk(e, 295.2, 0.9) + pk(e, 609.3, 1.4) + 2 * Math.exp(-e / 300))
            + th * (pk(e, 238.6, 9) + pk(e, 583.2, 3) + pk(e, 911.2, 1.2) + pk(e, 2614.5, 0.9) + 4 * Math.exp(-e / 300));
          const c = Math.max(0, lam * f + Math.sqrt(lam * f) * (rnd(k * 997 + b + t0 / 6e4) + rnd(k * 31 + b * 7) - 1) * 2.4);
          row.push(Math.round((c / f) * 100) / 100); tot += c / f;
        }
        out.rows.push(row); out.secs.push(Math.min(rowSec, (Math.min(t1, now) - t0) / 1000)); out.cps.push(Math.round(tot / 60 * 1.6 * 100) / 100); out.gap.push(gap);
      }
      return out;
    },
    GetRcHardness: async (from, to, rowSec) => {
      // mesma história do espectrograma de mentira: chuva há ~5 h (dureza
      // cai), eletrodo de tório há ~40 min (sobe)
      const n = Math.ceil((to - from) / (rowSec * 1000)), now = Date.now(), first = demoRcFake() ? now - 30 * 3600e3 : 0;
      const win = Math.max(1200, rowSec), half = Math.floor(win / 2 / rowSec);
      const out = { from, to, rowSec, winSec: (2 * half + 1) * rowSec, cps: [], hard: [], hardErr: [], gap: [], rain: [], wet: [], ref: 0.0125, refCps: 4.6, first };
      const rnd = (k) => { const x = Math.sin(k * 12.9898) * 43758.5453; return x - Math.floor(x); };
      for (let k = 0; k < n; k++) {
        const t0 = from + k * rowSec * 1000, ago = (now - t0) / 3600e3;
        if (!first || t0 + rowSec * 1000 <= first || t0 > now) { out.cps.push(0); out.hard.push(0); out.hardErr.push(0); out.gap.push(false); out.rain.push(0); out.wet.push(false); continue; }
        const rain = ago < 6 && ago > 2 ? Math.exp(-Math.abs(ago - 4.6) * 1.2) : 0, th = ago < 0.7 && ago > 0.4 ? 1 : 0, wet = ago > 4.2 && ago < 5.4;
        const noise = (rnd(t0 / 6e4) - 0.5) * Math.sqrt(60 / rowSec);
        out.cps.push(Math.round((4.6 * (1 + 0.55 * rain) + 9 * th + noise * 0.6) * 100) / 100);
        const hd = 1 - 0.28 * rain + 0.4 * th + (rnd(t0 / 6e4 + 7) - 0.5) * 0.08;
        out.hard.push(Math.round(hd * 1000) / 1000); out.hardErr.push(0.05 + 0.03 * Math.sqrt(60 / out.winSec * 20));
        out.gap.push(ago > 14 && ago < 20); out.rain.push(wet ? 2.4 : ago > 3.8 && ago < 4.2 ? 0.3 : 0); out.wet.push(wet);
      }
      return out;
    },
    GetRcWindowPeaks: async (from, to) => {
      const ago = (Date.now() - (from + to) / 2) / 3600e3;
      if (ago < 0.75 && ago > 0.35) return [{ group: "Tório-232 (série)", nuclide: "Pb-212", kev: 238.6, net: 400, sigma: 14 }, { group: "Tório-232 (série)", nuclide: "Tl-208", kev: 583.2, net: 120, sigma: 6 }];
      if (ago < 6 && ago > 3) return [{ group: "Rádio-226 (série do urânio)", nuclide: "Pb-214", kev: 351.9, net: 200, sigma: 7 }, { group: "Rádio-226 (série do urânio)", nuclide: "Bi-214", kev: 609.3, net: 150, sigma: 5 }];
      return [];
    },
    GetRcLog: async (from, to, kinds, limit, before) => {
      if (!demoRcFake()) return { items: [], shown: 0, total: 0, kinds: {}, more: false, first: 0 };
      // log de mentira, parecido com o do celular: taxa a cada 96 s, raros a
      // cada 5 min, alguns eventos; uma parte veio do celular
      const now = Date.now(), all = [], rnd = (k) => { const x = Math.sin(k * 12.9898) * 43758.5453; return x - Math.floor(x); };
      const t0 = now - 2 * 86400e3;
      for (let t = now - ((now / 1000) % 96) * 1000, k = 0; t > t0; t -= 96e3, k++) {
        const cps = 12.3 + (rnd(k) - 0.5) * 1.6, n = Math.round(cps * 96);
        all.push({ t, kind: "taxa", usv: Math.round((0.148 + (rnd(k + 9) - 0.5) * 0.04) * 1e4) / 1e4, usvErr: 19 + rnd(k + 3) * 3, cps, cpsErr: 200 / Math.sqrt(n), count: n, src: (now - t) > 20 * 3600e3 && (now - t) < 26 * 3600e3 ? "celular" : "pc" });
        if (k % 3 === 1) all.push({ t: t - 31e3, kind: "raro", temp: 27 + rnd(k + 5) * 1.5, hasTemp: true, battery: 100, hasBat: true, dose: 1.2 - k * 0.0006, hasDose: true, doseSec: 26000 - k * 96, src: "pc" });
      }
      all.push({ t: now - 7.5 * 3600e3, kind: "evento", event: "começou a carregar", code: 7, src: "celular" });
      all.push({ t: now - 7.5 * 3600e3 - 2e3, kind: "evento", event: "ligou", code: 1, src: "celular" });
      all.push({ t: now - 3.2 * 3600e3, kind: "evento", event: "configuração alterada", code: 3, src: "pc" });
      all.push({ t: now - 7.5 * 3600e3 + 1e3, kind: "raro", hasBat: true, battery: 62, power: "carregando", hasTemp: true, temp: 26.7, hasDose: true, dose: 0.03, doseSec: 120, src: "celular" });
      const inR = all.filter((it) => (!from || it.t >= from) && (!to || it.t <= to)).sort((a, b) => b.t - a.t);
      const k = {}; inR.forEach((it) => { k[it.kind] = (k[it.kind] || 0) + 1; });
      const sel = inR.filter((it) => kinds.includes(it.kind)), page = sel.filter((it) => !before || it.t < before);
      return { items: page.slice(0, limit), shown: sel.length, total: inR.length, kinds: k, more: page.length > limit, first: t0 };
    },
    ImportRcLog: async () => ({ file: "RadiaCode-log.csv", read: 57, added: 41, skipped: 1, from: Date.now() - 26 * 3600e3, to: Date.now() - 20 * 3600e3 }),
    GetRcSearch: async (since) => {
      // busca de mentira: fundo de 12 cps e, a cada 40 s, o aparelho passa
      // perto de uma amostra (sobe até ~90 cps e volta)
      const live = demoRcFake(), now = Date.now(), out = { live, now, pts: [], raw: true, cps: 12.3, usv: 0.15, refCps: 12.1, refUsv: 0.148, ble: false };
      if (!live) return out;
      const p0 = Math.max(since || now - 60e3, now - 60e3);
      const rate = (t) => { const ph = (t / 1000) % 40; return 12 + (ph > 22 && ph < 30 ? 80 * Math.exp(-(((ph - 26) / 1.6) ** 2)) : 0); };
      for (let t = Math.floor(p0 / 500) * 500 + 500; t <= now; t += 500) {
        const lam = rate(t) * 0.5; let n = 0, L = Math.exp(-lam), pp = 1; do { n++; pp *= Math.random(); } while (pp > L); n--;
        out.pts.push({ t, sec: 0.5, cps: n / 0.5, usv: (n / 0.5) * 0.0122 });
      }
      return out;
    },
    // atividade de mentira: o tempo passa 120× mais rápido
    GetConfigHistory: async () => [
      { t: Date.now() - 3600e3, campo: "Alarme", de: "a partir de 100 CPM", para: "a partir de 120 CPM", origem: "você" },
      { t: Date.now() - 86400e3, campo: "Calibração do GMC", de: "", para: "0,01014 µSv/h por CPM no fundo (contador; 986 CPM = 10,00 µSv/h, …)", origem: "início do registro" },
      { t: Date.now() - 86400e3, campo: "Tempo morto", de: "", para: "desligado", origem: "início do registro" },
    ],
    GetCounterSync: async () => ({ known: true, at: Date.now() - 2 * 864e5, complete: true, records: 4100, added: 32, memFirst: Date.now() - 30 * 864e5, memLast: Date.now() - 2 * 864e5, saveMode: "a cada minuto", saveOn: true,
      usedBytes: 1720000, totalBytes: 2097152, usedPct: 82, estPct: 84.3, fullIn: 19 * 86400, memLevel: "atencao", state: "pendente", pendingMinutes: 412, pendingFrom: Date.now() - 864e5,
      text: "Último download em 05/11 20:10. Depois dele, o app ficou 6 h 52 min sem leitura (desde 06/11 21:00); a memória do contador pode ter esses minutos: baixe o histórico." }),
    GetMeasurementContext: async (id) => {
      const all = (await api.GetMeasurements()) || [], m = all.find((x) => x.id === id) || {};
      const key = (n) => (n || "").toLowerCase().trim();
      return { cfgKnown: true, changed: ["Calibração em uso"], cfg: [{ field: "Calibração em uso", then: "0,01014 µSv/h por CPM no fundo (curva do contador)", now: "0,00550 µSv/h por CPM no fundo (informado à mão)" }, { field: "Tempo morto", then: "desligado", now: "desligado" }],
        related: all.filter((x) => x.id !== id && key(x.name) === key(m.name)).map((x) => ({ id: x.id, time: x.time, device: x.device || "", netUSv: x.result.netUSv, netErrUSv: 0.01, distance: x.distance || "", notComparable: !!x.notComparable, ncReason: x.ncReason || "", diff: x.notComparable ? ["marcada como não comparável"] : (x.distance || "") !== (m.distance || "") ? ["outra distância"] : [] })) };
    },
    SetMeasurementComparable: async (id, comparable, reason) => { const m = measurements.find((x) => x.id === id); if (m) { m.notComparable = !comparable; m.ncReason = comparable ? "" : reason; } setTimeout(() => emit("measurements", measurements.slice()), 0); },
    AddMeasurementAttachment: async (id) => { const m = measurements.find((x) => x.id === id); if (m) { m.attachments = (m.attachments || []).concat([{ file: "foto.jpg", name: "foto.jpg", size: 182000, type: "image/jpeg", added: Date.now() }]); } setTimeout(() => emit("measurements", measurements.slice()), 0); return m ? m.attachments : []; },
    RemoveMeasurementAttachment: async (id, file) => { const m = measurements.find((x) => x.id === id); if (m) m.attachments = (m.attachments || []).filter((a) => a.file !== file); setTimeout(() => emit("measurements", measurements.slice()), 0); },
    GetMeasurementAttachment: async () => "data:image/svg+xml;base64," + btoa('<svg xmlns="http://www.w3.org/2000/svg" width="120" height="90"><rect width="120" height="90" fill="#2a3136"/><circle cx="60" cy="45" r="22" fill="' + COR.accent + '"/></svg>'),
    OpenMeasurementAttachment: async () => {},
    GetPatterns: async (days, dev) => {
      const now = Date.now(), from = now - days * 864e5, k = dev === "radiacode" ? 0.0061 : 1;
      const pts = range(from, now).map((q) => ({ t: q.t, v: q.cpm * k }));
      const st = (vs) => { if (!vs.length) return { n: 0, days: 0, median: 0, p25: 0, p75: 0, mean: 0 }; const x = vs.map((q) => q.v).sort((a, b) => a - b), q = (f) => x[Math.min(x.length - 1, Math.floor(f * x.length))];
        return { n: x.length, days: new Set(vs.map((v) => new Date(v.t).toDateString())).size, median: q(0.5), p25: q(0.25), p75: q(0.75), mean: x.reduce((a, b) => a + b, 0) / x.length }; };
      const hours = Array.from({ length: 24 }, (_, h) => st(pts.filter((q) => new Date(q.t).getHours() === h)));
      const weekdays = Array.from({ length: 7 }, (_, d) => st(pts.filter((q) => new Date(q.t).getDay() === d)));
      const isDay = (t) => { const h = new Date(t).getHours(); return h >= 6 && h < 18; };
      const mStart = (t) => { const d = new Date(t); return new Date(d.getFullYear(), d.getMonth(), 1).getTime(); };
      const months = [...new Set(pts.map((q) => mStart(q.t)))].sort().map((m) => Object.assign({ month: m }, st(pts.filter((q) => mStart(q.t) === m))));
      let pk = -1, lw = -1;
      hours.forEach((h, i) => { if (h.n < 30) return; if (pk < 0 || h.median > hours[pk].median) pk = i; if (lw < 0 || h.median < hours[lw].median) lw = i; });
      return { device: dev, unit: dev === "radiacode" ? "µSv/h" : "CPM", days, from, to: now, minutes: pts.length, first: demoFirst(), hours, weekdays, months,
        day: st(pts.filter((q) => isDay(q.t))), night: st(pts.filter((q) => !isDay(q.t))), dnDays: 6, dnDiff: 1.2 * k, dnErr: 0.3 * k, dnPct: 6,
        dnText: "A noite (18h–6h) fica 6% acima do dia (+1,2 ± 0,3 CPM, 6 dias com as duas metades). É o padrão comum do radônio, que se acumula com a casa fechada e o ar parado à noite.", peakHour: pk, lowHour: lw };
    },
    GetHeatmapDev: async (days, dev) => { const h = await api.GetHeatmap(days); return dev === "radiacode" ? rcHeat(h) : h; },
    GetHeatmapMonthDev: async (y, m, dev) => { const h = await api.GetHeatmapMonth(y, m); return dev === "radiacode" ? rcHeat(h) : h; },
    GetYearCal: async (year) => {
      const now = new Date(), today = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
      const first = now.getFullYear() - 1, y = year >= first && year <= now.getFullYear() ? year : now.getFullYear();
      let seed = y * 31 + 7;
      const rnd = () => { seed = (seed * 16807) % 2147483647; return (seed - 1) / 2147483646; };
      const gauss = () => Math.sqrt(-2 * Math.log(rnd() + 1e-9)) * Math.cos(2 * Math.PI * rnd());
      const start = new Date(first, 1, 16).getTime(), days = [];
      for (let t = new Date(y, 0, 1); t.getFullYear() === y && t.getTime() <= today; t.setDate(t.getDate() + 1)) {
        const k = t.getTime(), m = t.getMonth(), doy = Math.round((k - new Date(y, 0, 1)) / 864e5);
        const d = { day: k, n: 0 };
        if (k >= start && !(m === 3 && t.getDate() >= 8 && t.getDate() < 12)) {
          const season = Math.cos(2 * Math.PI * (doy - 196) / 365), rain = rnd() < (m >= 10 || m <= 2 ? .42 : .18);
          d.n = k === today ? Math.floor((now - k) / 6e4) : rnd() < .06 ? 200 + Math.floor(rnd() * 400) : 1380 + Math.floor(rnd() * 60);
          d.bg = 0.105 * (1 + 0.035 * season + 0.012 * gauss() + (rain ? 0.04 : 0));
          if (rain) { d.rain = true; d.mm = Math.round((2 + rnd() * 25) * 10) / 10; } else if (rnd() < .1) d.mm = 0.2;
          if (rnd() < .025) { d.lvl = rnd() < .7 ? "atencao" : "alarme"; d.peak = d.lvl === "alarme" ? .8 + rnd() * 1.2 : .32 + rnd() * .3; d.peakAt = k + Math.floor(rnd() * 864e5); }
          d.away = rnd() < .22; d.doseN = d.n; d.dose = Math.max(.6, (2.5 + .25 * season + .25 * gauss()) * (d.away ? .92 : 1)) * d.n / 1440;
          if (m === 6 && t.getDate() === 22) { d.away = true; d.dose = 7.9; }
        }
        days.push(d);
      }
      return { year: y, first, today, days };
    },
    GetCrossCheck: async (dev, from, to, series) => cxDemo(dev, from, to, series),
    GetCrossChecks: async (reqs) => reqs.map((r) => cxDemo(r.dev, r.from, r.to, false)),
    GetBgSlow: async () => bsDemo(localGet("fundoDemo", "amb"), +localGet("fundoMoved", 0)),
    MarkBgMoved: async (d) => { const t = new Date(d || Date.now()); t.setHours(0, 0, 0, 0); localSet("fundoMoved", t.getTime()); return bsDemo(localGet("fundoDemo", "amb"), t.getTime()); },
    ClearBgMoved: async () => { localSet("fundoMoved", 0); return bsDemo(localGet("fundoDemo", "amb"), 0); },
    GetTubeHealth: async () => {
      // cenário pelo localStorage "tuboDemo": ok (padrão), ruido, sens, falha, semrc
      const sc = localGet("tuboDemo", "ok"), now = Date.now(), day0 = new Date(); day0.setHours(0, 0, 0, 0);
      const D = sc === "ruido" ? 1.65 : 1.04, lam = 19.6, pois = (l, k) => { let p = Math.exp(-l); for (let i = 1; i <= k; i++) p *= l / i; return p; };
      const hist = [];
      for (let k = 4; k <= 40; k++) {
        let p = pois(lam, k);
        if (D > 1.05) { p = 0; const sd = Math.sqrt((D - 1) * lam); for (let j = -3; j <= 3; j += 0.5) p += Math.exp(-j * j / 2) * pois(Math.max(1, lam + j * sd), k); p /= Math.sqrt(2 * Math.PI) * 2; }
        hist.push({ k, n: Math.round(p * 43200), ref: pois(lam, k) * 43200 });
      }
      const drift = sc === "sens" ? 0.18 : sc === "ruido" ? -0.1 : 0;
      const ratio = [];
      for (let i = 0; i < 88; i++) ratio.push({ day: day0.getTime() - (87 - i) * 864e5, r: 28.05 * (1 + drift * i / 87) + 0.12 * Math.sin(i * 0.9) * Math.cos(i * 0.37) });
      const r0 = 28.05, r1 = ratio[87].r, chg = r1 / r0 - 1;
      const at = new Date(); at.setHours(6, 0, 0, 0);
      return {
        at: at.getTime(), first: now - 120 * 864e5,
        gaps: sc === "falha" ? [{ at: now - 3 * 864e5, min: 14 }, { at: now - 1.5 * 864e5, min: 125 }] : [], gapsState: sc === "falha" ? "warn" : "ok", gapDays: 30,
        disp: D, dispDays: 30, dispHigh: sc === "ruido" ? 6 : 0, dispLast: 7, hist, dispState: sc === "ruido" ? "warn" : "ok",
        bursts: sc === "ruido" ? Array.from({ length: 31 }, (_, i) => now - (30 - i) * 864e5) : [now - 9 * 864e5], burstHour: sc === "ruido" ? 19 : -1, burstState: sc === "ruido" ? "warn" : "ok",
        ratio: sc === "semrc" ? [] : ratio, r0: sc === "semrc" ? 0 : r0, r1: sc === "semrc" ? 0 : r1, ratioChg: sc === "semrc" ? 0 : chg, ratioWhy: sc === "semrc" ? "semrc" : "",
        ratioState: sc === "semrc" ? "na" : Math.abs(chg) >= 0.05 ? "warn" : "ok",
      };
    },
    GetWhyRise: async (dev, at) => {
      const t = Math.floor((at || Date.now()) / 6e4) * 6e4 + 6e4, from = t - 12 * 3600e3, bins = [];
      let seed = 11;
      const nz = () => { seed = (seed * 16807) % 2147483647; return (seed / 2147483647 - 0.5) * 2; };
      const bump = (i) => { const d = i - 40; return d < 0 ? 0 : 0.11 * Math.exp(-d / 10) * (1 - Math.exp(-d / 1.5)); };
      for (let i = 0; i < 72; i++) bins.push({ t: from + i * 6e5, g: 0.110 * (1 + (i > 24 ? 0.07 : 0) + bump(i) + 0.025 * nz()), r: 0.098 * (1 + (i > 24 ? 0.07 : 0) + bump(i) * 0.9 + 0.025 * nz()) });
      const hh = (ms) => { const d = new Date(ms); return pad(d.getHours()) + ":" + pad(d.getMinutes()); };
      return { device: dev, at: t, now: !at, from: from + 40 * 6e5, pct: 18, sigma: 14.2, acaso: false, head: `O fundo subiu 18% desde ${hh(from + 40 * 6e5)}`,
        verdict: `Choveu 12,4 mm nas 4 horas antes, a partir de ${hh(from + 38 * 6e5)}. Esta é a hora em que o radônio costuma subir na sua casa. Os dois aparelhos subiram juntos, então é radiação de verdade.`,
        gNorm: 0.110, rNorm: 0.098, bins, rain: [[from + 38 * 6e5, from + 47 * 6e5]], marks: [],
        causes: [
          { key: "chuva", state: "yes", title: "Chuva", ev: `Choveu 12,4 mm nas 4 horas antes, a partir de ${hh(from + 38 * 6e5)}. Na sua casa, a chuva costuma subir o fundo uns 9%.`, more: "A chuva traz para o chão os produtos do radônio que estavam no ar. Eles têm meia-vida curta (minutos), então a subida dura poucas horas e some sozinha.", link: "chuva", linkText: "Abrir a tela Clima" },
          { key: "radonio", state: "yes", title: "Radônio da noite", ev: "Às 3h o seu fundo costuma ficar 7,5% acima da média do dia.", more: "Com a casa fechada e o ar parado, o radônio que sai do chão e das paredes se acumula. De dia, com janela aberta e vento, ele se espalha.", link: "padroes", linkText: "Abrir Padrões do fundo" },
          { key: "ambos", state: "yes", title: "Os dois aparelhos viram", ev: "O RC102 também subiu, 16% no mesmo trecho.", more: "Se fosse o ambiente, os dois subiriam juntos. É o que separa radiação de verdade de coisa de um aparelho só." },
          { key: "acaso", state: "no", title: "Acaso", ev: "A subida é de 14 σ. O acaso não chega nisso.", more: "O decaimento é aleatório, então a contagem varia sozinha. Até uns 2 σ isso é normal e acontece toda hora." },
          { key: "pressao", state: "no", title: "Pressão caindo", ev: "A pressão mudou 1,2 hPa nas 12 horas antes.", more: "Quando a pressão do ar cai, sai mais radônio do solo e das paredes, e o fundo em casa sobe algumas horas depois.", link: "chuva", linkText: "Abrir a tela Clima" },
          { key: "objeto", state: "no", title: "Objeto ou fonte perto", ev: `Nenhuma medição de objeto nem anotação desde ${hh(from + 34 * 6e5)}.`, more: "Um objeto perto do aparelho sobe muito a leitura dele, e quase nada a do outro aparelho. Para medir direito, use o Medir objeto." },
          { key: "aparelho", state: "no", title: "Coisa do aparelho", ev: "Os dois aparelhos subiram juntos.", more: "Confira se o GMC-800 foi mexido ou ficou perto de alguma coisa nova, e o cabo. Ruído no tubo também faz a contagem pular.", link: "gmc", linkText: "Abrir a tela do GMC-800" },
        ] };
    },
    GetBgTrend: async (days) => {
      const now = Date.now(), to = Math.floor(now / 6e4) * 6e4 + 6e4, from = to - days * 864e5;
      const step = days <= 7 ? 36e5 : days <= 30 ? 108e5 : days <= 90 ? 432e5 : 864e5;
      const all = range(from - 864e5, to), pts = [];
      for (let t = from + step; t <= to; t += step) {
        const w = all.filter((p) => p.t >= t - 864e5 && p.t < t).map((p) => p.cpm).sort((a, b) => a - b);
        const pt = { t, n: w.length, mean: null, median: null };
        if (w.length >= 360) { pt.mean = w.reduce((a, b) => a + b, 0) / w.length; pt.median = w[Math.floor(w.length / 2)]; }
        pts.push(pt);
      }
      const byDay = new Map();
      for (const p of all) { if (p.t < from) continue; const d = new Date(p.t); d.setHours(0, 0, 0, 0); const k = d.getTime(); (byDay.get(k) || byDay.set(k, []).get(k)).push(p.cpm); }
      const daily = [...byDay.entries()].sort((a, b) => a[0] - b[0]).map(([day, v]) => {
        v.sort((a, b) => a - b); const n = v.length, mean = v.reduce((a, b) => a + b, 0) / n, sd = Math.sqrt(v.reduce((a, b) => a + (b - mean) ** 2, 0) / Math.max(1, n - 1));
        return { day, n, mean, trimmed: mean, median: v[Math.floor(n / 2)], sd, poissonSD: Math.sqrt(mean), dispersion: sd / Math.sqrt(mean), full: n >= 720 };
      });
      const full = daily.filter((d) => d.full).length;
      const fit = full < 7 ? { state: "insuficiente", daysUsed: full, text: `Poucos dias completos para falar de tendência: ${full} de 7 (um dia conta com pelo menos 12 h medidas em casa).` }
        : { state: "estavel", daysUsed: full, text: `Sem tendência: a média dos dias variou dentro do esperado (+0,03 ± 0,05 CPM por dia, +1,2% no período, ${full} dias completos).` };
      return { days, from, to, stepMs: step, minutes: daily.reduce((a, d) => a + d.n, 0), first: demoFirst(), pts, daily, fit };
    },
    GetMaturity: async () => {
      const now = Date.now(), f = demoFirst();
      const win = (d) => { const from = now - d * 864e5, hf = Math.max(from, f), hist = Math.round((now - hf) / 6e4), n = range(from, now).length;
        return { days: d, state: f > from ? "formacao" : "disponivel", histDays: hist / 1440, minutes: n, histMinutes: hist, spanMinutes: d * 1440, coverage: hist ? Math.min(1, n / hist) : 0, coverageAll: n / (d * 1440) }; };
      const g = [1, 7, 30, 90, 365].map(win);
      return { gmcFirst: f, rcFirst: demoRcFake() ? f + 5 * 864e5 : 0, gmc: g, rc: demoRcFake() ? g.map((w) => ({ ...w, state: w.days > 7 ? "formacao" : w.state, histDays: Math.min(w.histDays, 7) })) : [] };
    },
    GetCalHistory: async () => {
      const d = demoCal, now = Date.now();
      const vs = d.vers.map((v, i) => ({ ...v, ate: d.vers[i + 1] ? d.vers[i + 1].desde : 0, k: v.fator || 0.01014, devK: 0.01014, atual: i === d.vers.length - 1, minutos: i === d.vers.length - 1 ? Math.round((now - v.desde) / 6e4) : 40320 }));
      return { versions: vs.reverse(), retro: d.retro, devK: 0.01014 };
    },
    SetGMCFactor: async (k, note, src) => { demoCal.vers.push({ id: demoCal.vers.length + 1, desde: Date.now(), fator: k, origem: src || "você", nota: note || "", pontos: [] }); },
    SetCalRetro: async (id) => { demoCal.retro = id; },
    CompareCal: async (id) => { const v = demoCal.vers.find((x) => x.id === id) || {}; const f = (v.fator || 0.01014) / 0.01014; return { id, from: Date.now() - 28 * 86400e3, to: v.desde, minutes: 40320, doseOrig: 81.2, doseNew: 81.2 * f, avgOrig: 0.121, avgNew: 0.121 * f, maxOrig: 0.42, maxNew: 0.42 * f, changePct: (f - 1) * 100 }; },
    DiagnoseGMCDose: async () => ({ path: "C:\\Users\\voce\\AppData\\Roaming\\RadLog\\diagnostico-dose-gmc-2026-11-07_101500.txt", verdict: "nenhum comando de leitura informa a dose do aparelho; o app continua calculando a dose", unknown: [], changed: [], cfgChanged: 0 }),
    GetActivity: async () => demoAct.view(),
    StartActivity: async (o) => demoAct.start(o),
    ActivityNext: async () => demoAct.next(),
    CancelActivity: async () => { demoAct.run = null; },
    CloseActivity: async () => { demoAct.run = null; },
    ForgetActivityCal: async (w) => { if (w === "kcl") demoAct.cal.kcl = null; if (w === "torio") demoAct.cal.th = null; },
    // o mesmo SNIP do Go (snip.go), para o detalhe do isótopo no modo demonstração
    SpectrumContinuum: async (sp) => demoSnip(sp),
    // acumulado do aparelho (sem fundo): as linhas do fundo natural
    AnalyzeSpectrum: async (sp) => demoPeakDetail(sp, sp.bgSec === 1 ? [
      { group: "Tório-232 (série)", nuclide: "Pb-212", kev: 238.6, net: 5200, sigma: 22 },
      { group: "Rádio-226 (série do urânio)", nuclide: "Pb-214", kev: 351.9, net: 44000, sigma: 125 }, { group: "Rádio-226 (série do urânio)", nuclide: "Pb-214", kev: 295.2, net: 21000, sigma: 60 },
      { group: "Rádio-226 (série do urânio)", nuclide: "Bi-214", kev: 609.3, net: 30000, sigma: 80 }, { group: "Rádio-226 (série do urânio)", nuclide: "Bi-214", kev: 1764.5, net: 2000, sigma: 9 },
      { group: "Potássio-40", nuclide: "K-40", kev: 1460.8, net: 9000, sigma: 40 }, { group: "Tório-232 (série)", nuclide: "Tl-208", kev: 2614.5, net: 900, sigma: 7 },
      { group: "Tório-232 (série)", nuclide: "Ac-228", kev: 911.2, net: 700, sigma: 6 }] : sp.objSec > 20 ? [
      { group: "Tório-232 (série)", nuclide: "Pb-212", kev: 238.6, net: 300, sigma: 12 }, { group: "Tório-232 (série)", nuclide: "Tl-208", kev: 583.2, net: 90, sigma: 6 },
      { group: "Tório-232 (série)", nuclide: "Tl-208", kev: 2614.5, net: 30, sigma: 4 }] : []),
    SetSyncListen: async (on) => (demoSync = { ...demoSync, listen: on, listening: on, addrs: on ? ["192.168.0.10:47862", "100.101.102.103:47862"] : [] }),
    SetSyncAuto: async (on) => (demoSync = { ...demoSync, auto: on }),
    // uso do app (uso.js): números de exemplo, como na maquete da spec
    GetUsage: async (days) => {
      if (demoUsageReset) return { on: demoUsageOn, since: demoUsageReset, days: 0, totals: {} };
      const f = days === 7 ? 0.25 : days === 30 ? 1 : 1.6;
      // chaves sem aspas: o i18n_extract não confunde "chuva" ou "hora" com texto da tela
      const pg = { agora: [420, 41 * 3600], radiacode: [260, 9 * 3600], graficos: [180, 6 * 3600], resumo: [60, 2 * 3600], alertas: [40, 3600], gmc: [30, 3300], config: [25, 2400], fora: [12, 1080], alara: [5, 360], chuva: [2, 60] };
      const tabs = { radiacode: { agora: [212, 5 * 3600], espectro: [88, 3 * 3600], atividade: [9, 720], busca: [4, 480] },
        graficos: { tempo: [150, 5 * 3600], hora: [20, 2400], padroes: [8, 600], tendencia: [2, 120] }, gmc: { agora: [28, 3000], tubo: [2, 300] } };
      const base = { "s:radiacode/rcSpecView=tempo": [61, 0], "s:radiacode/rcSpecMode=acc": [34, 0], "c:busca/ctrlk": [6, 0] };
      for (const [k, v] of Object.entries(pg)) base["p:" + k] = v;
      for (const [k, m] of Object.entries(tabs)) for (const [t, v] of Object.entries(m)) base["a:" + k + "/" + t] = v;
      const totals = {};
      for (const [k, [n, sec]] of Object.entries(base)) totals[k] = { n: Math.max(1, Math.round(n * f)), s: Math.round(sec * f) };
      return { on: demoUsageOn, since: Date.now() - 34 * 864e5, days: days || 34, totals };
    },
    SetUsageOn: async (on) => { demoUsageOn = on; settings.usageOn = on; return { on, since: 0, days: 0, totals: {} }; },
    ResetUsage: async () => { demoUsageReset = Date.now(); return { on: demoUsageOn, since: demoUsageReset, days: 0, totals: {} }; },
    RecordUsage: async () => {},
    ShowPairCode: async () => (demoSync = { ...demoSync, code: "482913", codeUntil: Date.now() + 300e3 }),
    CancelPairCode: async () => (demoSync = { ...demoSync, code: "" }),
    DiscoverPeers: async () => { await new Promise((r) => setTimeout(r, 800)); return [{ id: "b", name: "notebook", addr: "192.168.0.20:47862", paired: false }]; },
    PairWith: async () => { throw new Error("no modo navegador não dá para parear; abra pelo app"); },
    SyncNow: async () => ({}),
    SetPeerAddrs: async () => demoSync,
    ForgetPeer: async () => demoSync,
    GetUpdateInfo: async () => demoUpd,
    CheckUpdate: async () => { demoUpd = { ...demoUpd, available: true, checkedAt: Date.now() }; emit("update", demoUpd); return demoUpd; },
    SkipUpdate: async (v) => { demoUpd = { ...demoUpd, skipped: !!v }; emit("update", demoUpd); },
    CancelUpdate: async () => { demoUpd = { ...demoUpd, state: "cancelado" }; emit("update", demoUpd); },
    StartUpdate: async () => {
      let p = 0;
      const t = setInterval(() => {
        p += 0.2;
        demoUpd = { ...demoUpd, state: p < 1 ? "baixando" : "esperando", progress: Math.min(1, p), wait: "atenção ativa" };
        emit("update", demoUpd);
        if (p >= 1) clearInterval(t);
      }, 400);
    },
    GetMeasure: async () => ({ ...meas }),
    StopMeasure: async () => { if (meas.phase === "objeto") meas.stopReq = true; return { ...meas }; },
    MeasureAgainAt: async (distance, cm) => {
      if (meas.phase !== "pronto" || !meas.result) throw new Error("termine a medição antes de medir em outra distância");
      const k = meas.obj.cpm > 0 ? meas.obj.usv / meas.obj.cpm : 0;
      meas.prev = [...(meas.prev || []), { distance: meas.distance, distCm: meas.distCm, netUSv: meas.result.netUSv, errUSv: meas.result.netErr * k, sigma: meas.result.sigma, level: meas.result.level, seconds: meas.obj.seconds }];
      meas.distance = distance; meas.distCm = cm; meas.derived = null; meas.spectrum = null;
      if (meas.other) Object.assign(meas.other, { counts: 0, elapsed: 0, usvSum: 0, obj: null, result: null, live: null, hint: "", dose: 0, maxUSv: 0, excess: 0 });
      return api.StartMeasurePhase("objeto", 0, "");
    },
    GetMeasureLive: async () => {
      const rcIn = meas.device === "radiacode" || (meas.other && meas.other.device === "radiacode");
      if (!rcIn || (meas.phase !== "fundo" && meas.phase !== "objeto") || !meas.elapsed) return { phase: meas.phase };
      const sp = demoMeasSpec(meas.elapsed, !!(meas.bg && meas.bg.source === "casa"));
      if (meas.device !== "radiacode") sp.role = "voce";
      if (meas.phase === "fundo") { sp.obj = sp.bg; sp.objSec = meas.elapsed; delete sp.bg; sp.peaks = []; }
      else if (meas.elapsed < 20) sp.peaks = sp.peaks.slice(0, 1);
      return { phase: meas.phase, spectrum: sp };
    },
    StartMeasurePhase: async (phase, secs, name) => {
      if (phase === "fundo") meas = { phase, seconds: secs || 60, elapsed: 0, counts: 0, name, demo: demoSt.active, bg: null, obj: null, result: null, eta: -1 };
      else { if (!meas.bg) throw new Error("meça o fundo primeiro"); Object.assign(meas, { phase, elapsed: 0, counts: 0, obj: null, result: null, live: null, eta: -1, phaseStart: Date.now(), derived: null, spectrum: null, stopReq: false, name: name || meas.name }); if (meas.auto) meas.seconds = meas.maxSec; }
      emit("measure", { ...meas }); return { ...meas };
    },
    GetHomeBackground: async () => demoHomeBg(),
    GetHomeBackgroundDev: async (dev) => {
      if (dev !== "radiacode") return { ...demoHomeBg(), usv: 20.4 * 0.0101, device: "gmc" };
      if (!demoRcFake()) return { ok: false, minutes: 0, why: "só 0 min do RC102 em casa na última hora (precisa de 30)", device: "radiacode" };
      return { ok: true, cpm: 718, errCPM: 3.5, minutes: 57, counts: 40926, from: Date.now() - 3600e3, to: Date.now() - 60e3, usv: 0.092, device: "radiacode" };
    },
    StartMeasure: async (o) => {
      const maxSec = o.maxSec || 1800, pct = o.pct || 20;
      const mode = o.mode === "ambos" || o.mode === "voce" ? o.mode : "";
      const device = mode === "ambos" ? "radiacode" : o.device === "radiacode" ? "radiacode" : "gmc";
      if ((device === "radiacode" || mode) && (!demoRcFake() || localStorage.getItem("rcFake2"))) throw new Error("conecte o RC102 para medir");
      meas = { phase: "fundo", seconds: o.auto ? 300 : o.seconds || 60, elapsed: 0, counts: 0, name: (o.name || "").trim(), distance: (o.distance || "").trim(), demo: demoSt.active && device === "gmc",
        distCm: o.distCm > 0 ? o.distCm : 0, size: o.size || "", prev: [], derived: null, spectrum: null,
        test: o.test === "distancia" || o.test === "blindagem" ? o.test : "", material: (o.material || "").trim(), relErr: Math.min(Math.max(o.relErr || 0, 0), 0.5),
        auto: !!o.auto, maxSec, pct, bg: null, obj: null, result: null, live: null, eta: -1, phaseStart: Date.now(), device };
      if (mode) {
        const od = device === "radiacode" ? "gmc" : "radiacode", oh = await api.GetHomeBackgroundDev(od);
        meas.mode = mode;
        meas.other = { device: od, role: mode === "ambos" ? "objeto" : "voce", counts: 0, elapsed: 0, bgUSv: oh.usv || 0, dose: 0, maxUSv: 0 };
        if (mode === "ambos" && o.homeBg) meas.other.bg = { counts: oh.counts, seconds: oh.minutes * 60, cpm: oh.cpm, errCPM: oh.errCPM, usv: oh.usv, source: "casa" };
      }
      if (o.homeBg) {
        const h = await api.GetHomeBackgroundDev(device);
        if (!h.ok) throw new Error("fundo da casa indisponível: " + h.why);
        meas.bg = { counts: h.counts, seconds: h.minutes * 60, cpm: h.cpm, errCPM: h.errCPM, usv: h.usv, start: h.from, end: h.to, source: "casa" };
        meas.phase = "objeto";
        meas.seconds = o.auto ? maxSec : o.seconds || 60;
      }
      emit("measure", { ...meas }); return { ...meas };
    },
    CancelMeasure: async () => { meas = { phase: "", seconds: 0, elapsed: 0, counts: 0, name: "", demo: false, bg: null, obj: null, result: null }; emit("measure", { ...meas }); return { ...meas }; },
    GetRainStatus: async () => rainStatus(),
    OpenLogFile: async () => {},
    GetNotes: async (from, to) => notes.filter((n) => (!from && !to) || (Math.max(n.end || 0, n.time) >= from && n.time < to)),
    SaveNote: async (n) => {
      if (!String(n.text || "").trim()) throw new Error("escreva o texto da anotação");
      if (n.id) notes = notes.map((q) => (q.id === n.id ? { ...q, ...n, text: n.text.trim() } : q));
      else notes.push({ ...n, id: ++noteSeq, text: n.text.trim(), created: Date.now() });
      notes.sort((a, b) => a.time - b.time);
      setTimeout(() => emit("notes", notes.slice()), 0);
      return n;
    },
    DeleteNote: async (id) => { notes = notes.filter((n) => n.id !== id); setTimeout(() => emit("notes", notes.slice()), 0); },
    ExportBackup: async () => "C:\\Users\\voce\\Documents\\gmc-backup-PC-2026-10-29.zip",
    ImportBackup: async () => ({ from: "LINUX-PC", created: "", minutes: 340, events: 3, periods: 1, measurements: 0, alertFiles: 1, settings: 3 }),
    ApplyBackupSettings: async () => { settings = Object.assign({}, settings, { alarmSound: "bipes" }); return settings; },
    GetSummary: async (from, to) => {
      to = Math.min(to, Date.now() + 6e4);
      const span = to - from, step = span <= 26 * 36e5 ? 9e5 : span <= 8 * 864e5 ? 36e5 : 216e5;
      const p = range(from, to);
      const r = { from, to, minutes: p.length, coverage: Math.min(1, p.length / (span / 6e4)), awayMinutes: 0, stepMs: step, curve: [],
        bgCPM: 20, bgOK: true, attention: 0, alarm: 0, danger: 0, peakAlert: 0, rainMM: 0, rainHours: 0, rainKnown: !!settings.rainEnabled, measurements: 0, dose: 0 };
      if (p.length) {
        let sum = 0, mx = p[0], mn = p[0];
        for (const q of p) { sum += q.cpm; if (q.cpm > mx.cpm) mx = q; if (q.cpm < mn.cpm) mn = q; r.dose += q.cpm * 0.0101 / 60; }
        r.avg = sum / p.length; r.max = mx.cpm; r.maxAt = mx.t; r.min = mn.cpm; r.minAt = mn.t; r.avgUSv = r.avg * 0.0101; r.maxUSv = r.max * 0.0101; r.minUSv = r.min * 0.0101; r.bgUSv = 20 * 0.0101; r.vsBgPct = (r.avg - 20) / 20 * 100;
      }
      for (let t = from; t < to; t += step) { const q = range(t, t + step); r.curve.push({ t, cpm: q.length ? q.reduce((a, b) => a + b.cpm, 0) / q.length : null }); }
      for (const e of events.concat(demoAlerts)) if (e.kind === "alerta" && e.time >= from && e.time < to) { r[e.level === "perigo" ? "danger" : e.level === "alarme" ? "alarm" : "attention"]++; r.peakAlert = Math.max(r.peakAlert, e.peak || 0); r.peakAlertUSv = r.peakAlert * 0.0101; }
      if (settings.rainEnabled && span > 20 * 36e5) { r.rainMM = 3.4; r.rainHours = 2; }
      const effTo = Math.min(to, Date.now());
      r.spanMinutes = Math.round((effTo - from) / 6e4);
      const hf = Math.max(from, demoFirst());
      r.histFrom = hf; r.histMinutes = Math.max(0, Math.round((effTo - hf) / 6e4)); r.preMinutes = r.spanMinutes - r.histMinutes; r.ongoing = to > Date.now();
      r.coverage = r.histMinutes ? Math.min(1, p.length / r.histMinutes) : 0; r.coverageAll = r.spanMinutes ? Math.min(1, p.length / r.spanMinutes) : 0;
      r.daysData = new Set(p.map((q) => new Date(q.t).toDateString())).size;
      r.gapMinutes = Math.max(0, r.histMinutes - p.length);
      r.gaps = []; r.gapCount = 0;
      for (let i = 1; i < p.length; i++) if (p[i].t - p[i - 1].t >= 10 * 6e4) { r.gapCount++; r.gaps.unshift({ from: p[i - 1].t + 6e4, to: p[i].t, minutes: Math.round((p[i].t - p[i - 1].t - 6e4) / 6e4), cause: "contador desconectado", open: false }); }
      if (span >= 6 * 36e5) { r.gapCount++; r.gaps.unshift({ from: from + 3 * 36e5, to: from + 3 * 36e5 + 42 * 6e4, minutes: 42, cause: "app fechado e sem histórico do contador", open: false }); }
      if (p.length > 30) { const m = r.avg; r.sd = Math.sqrt(m) * 1.04; r.dispersion = 1.04; }
      if (demoRcFake() && p.length) { // RC102 fingido: segue o GMC um pouco abaixo
        const k = 0.0101 * 0.6, rc = { minutes: p.length, coverage: r.coverage, coverageAll: r.coverageAll, histMinutes: r.histMinutes, spanMinutes: r.spanMinutes, histFrom: r.histFrom, dose: r.dose * 0.6, avg: r.avg * k, bgUSv: 20 * k, curve: [] };
        rc.vsBgPct = (rc.avg - rc.bgUSv) / rc.bgUSv * 100;
        rc.max = r.max * k; rc.maxAt = r.maxAt; rc.min = r.min * k; rc.minAt = r.minAt;
        rc.awayMinutes = Math.round(p.length * 0.2); rc.awayDose = rc.dose * 0.25; rc.awayAvg = rc.avg * 1.25; rc.awayMax = rc.max * 1.1; rc.awayMaxAt = r.maxAt;
        rc.homeMinutes = p.length - rc.awayMinutes; rc.homeDose = rc.dose - rc.awayDose;
        rc.curve = r.curve.map((q) => ({ t: q.t, usv: q.cpm == null ? null : q.cpm * k }));
        r.rc = rc;
      }
      return r;
    },
    // pressão × radônio (2.44): 7 dias de exemplo, a pressão cai na 5ª e o
    // fundo sobe ~3 h depois, além da subida de toda madrugada
    GetPressureRadon: async (dev) => {
      const devN = dev === "radiacode" ? "radiacode" : "gmc";
      if (!settings.rainEnabled) return { enabled: false, device: devN, hours: [] };
      const N = 169, t0 = Math.floor(Date.now() / 36e5) * 36e5 - (N - 1) * 36e5, P = [], F = [];
      let seed = 11;
      const rnd = () => { seed = (seed * 16807) % 2147483647; return seed / 2147483647; };
      const g = () => (rnd() + rnd() + rnd() - 1.5) * 1.4;
      for (let i = 0; i < N; i++) { const d = i / 24; P.push(1016 + 3 * Math.sin((d * 2 * Math.PI) / 3.5) - 11 * Math.exp(-Math.pow((d - 3.4) / 0.55, 2)) + 1.2 * Math.sin((i / 12) * Math.PI) + g() * 0.3); }
      const base = devN === "radiacode" ? 0.1 : 0.148;
      for (let i = 0; i < N; i++) {
        const hr = new Date(t0 + i * 36e5).getHours(), night = Math.exp(-Math.pow((((hr + 18) % 24) - 12) / 3.5, 2)) * 0.022;
        const p3 = P[Math.max(0, i - 3)], drop = Math.max(0, P[Math.max(0, i - 4)] - P[i]) * 0.0035;
        F.push(base + night + Math.max(0, 1016 - p3) * 0.0016 + drop + g() * 0.004);
      }
      const corr = (lag) => { const xs = [], ys = []; for (let i = lag; i < N; i++) { xs.push(P[i - lag]); ys.push(F[i]); }
        const mx = xs.reduce((a, v) => a + v, 0) / xs.length, my = ys.reduce((a, v) => a + v, 0) / ys.length; let sxy = 0, sxx = 0, syy = 0;
        for (let i = 0; i < xs.length; i++) { sxy += (xs[i] - mx) * (ys[i] - my); sxx += (xs[i] - mx) ** 2; syy += (ys[i] - my) ** 2; } return sxy / Math.sqrt(sxx * syy); };
      let lag = 0, r = corr(0);
      for (let l = 1; l <= 12; l++) { const c = corr(l); if (Math.abs(c) > Math.abs(r)) { r = c; lag = l; } }
      const rs = (r < 0 ? "−" : "") + Math.abs(r).toFixed(2).replace(".", ",");
      return { enabled: true, device: devN, lag, r, days: 7, pairs: N - lag, status: "pista",
        hours: P.map((p, i) => ({ t: t0 + i * 36e5, hPa: p, usv: F[i] })),
        title: lag ? `Pista: quando a pressão cai, o fundo sobe cerca de ${lag} h depois.` : "Pista: quando a pressão cai, o fundo sobe quase ao mesmo tempo.",
        text: `r = ${rs} em 7 dias. O sinal negativo quer dizer que um sobe quando o outro desce. Com menos de 28 dias isso ainda é uma pista.` };
    },
    GetRainImpactDev: async (dev) => {
      const r = await api.GetRainImpact();
      if (dev !== "radiacode" || !r.enabled) return { ...r, device: "gmc" };
      if (!demoRcFake()) return { enabled: true, episodes: [], curve: [], n: 0, skippedNoData: 0, device: "radiacode" };
      // Radiacode: ~720 CPM, a mesma subida em %
      const f = 720 / 26.4, sc = (x) => (x == null ? x : x * f);
      return { ...r, device: "radiacode", usvPerCPM: 0.1 / 720, avgBase: r.avgBase * f, avgRise: r.avgRise * f, dryRise: r.dryRise * f, effect: r.effect * f,
        curve: r.curve.map((p) => ({ min: p.min, rain: sc(p.rain), dry: sc(p.dry) })),
        now: r.now && { ...r.now, baseline: r.now.baseline * f, nowCPM: r.now.nowCPM * f },
        episodes: r.episodes.map((e) => ({ ...e, baseline: e.baseline * f, peak: e.peak * f, rise: e.rise * f })) };
    },
    GetRainImpact: async () => {
      if (!settings.rainEnabled) return { enabled: false, episodes: [], curve: [], n: 0 };
      const curve = [];
      for (let i = 0; i < 24; i++) {
        const m = i * 15 - 120 + 7;
        const bump = m > 0 ? 5.5 * Math.exp(-Math.pow((m - 50) / 55, 2)) : 0;
        curve.push({ min: m, rain: bump + Math.sin(i) * 0.3, dry: 0.4 + Math.cos(i * 1.3) * 0.35 });
      }
      const h = 36e5, base = Math.floor(now / h) * h;
      const rnow = { start: Date.now() - 50 * 60e3, end: Date.now() - 3 * 60e3, raining: true, mm: 0.7, maxMMh: 1.2, source: "radar", status: "medindo",
        resultAt: Date.now() + 70 * 60e3, reason: "", prevEnd: 0, baseline: 26.4, hasBase: true, nowCPM: 29.8, hasNow: true };
      return { now: rnow, enabled: true, n: 4, avgRise: 6.1, avgBase: 26.4, dryN: 112, dryRise: 1.9, effect: 4.2, peakMin: 45, backMin: 150, backN: 3, curve, skippedNoData: 1,
        episodes: [
          { start: base - 3 * h, end: base - 2 * h, mm: 1.4, maxMMh: 3, source: "radar", baseline: 26.0, peak: 30.2, peakAt: base - 3 * h + 35 * 60e3, rise: 4.2, backAt: 0, partial: true },
          { start: base - 20 * h, end: base - 19 * h, mm: 4.2, maxMMh: 14, source: "radar", baseline: 26.1, peak: 33.4, peakAt: base - 20 * h + 40 * 60e3, rise: 7.3, backAt: base - 20 * h + 160 * 60e3 },
          { start: base - 70 * h, end: base - 68 * h, mm: 9.5, maxMMh: 22, source: "radar+modelo", baseline: 27.0, peak: 34.2, peakAt: base - 70 * h + 55 * 60e3, rise: 7.2, backAt: base - 70 * h + 190 * 60e3 },
          { start: base - 120 * h, end: base - 119 * h, mm: 1.1, maxMMh: 2, source: "modelo", baseline: 25.8, peak: 29.1, peakAt: base - 120 * h + 30 * 60e3, rise: 3.3, backAt: 0 },
          { start: base - 160 * h, end: base - 159 * h, mm: 2.3, maxMMh: 6, source: "modelo", baseline: 26.7, peak: 33.3, peakAt: base - 160 * h + 50 * 60e3, rise: 6.6, backAt: base - 160 * h + 140 * 60e3 }] };
    },
    GetRainRadar: async (from, to) => {
      const out = [];
      if (!settings.rainEnabled) return out;
      const step = 6e5, last = Math.floor(now / step) * step;
      for (let t = Math.max(Math.ceil(from / step) * step, last - 3 * 36e5); t <= Math.min(to, last); t += step) {
        const m = (last - t) / step; // chuva forte há ~40 min, passando
        out.push({ time: t, dbz: 0, mmh: m >= 3 && m <= 7 ? [1.2, 6, 14, 5, 0.8][m - 3] : 0 });
      }
      return out;
    },
    RefreshRain: async () => { rainFetched = Date.now(); return rainStatus(); },
    SearchPlace: async (q) => [{ name: q || "Rio de Janeiro", detail: "Rio de Janeiro, Brasil", latitude: -22.9, longitude: -43.2 }, { name: q || "Rio de Janeiro", detail: "Minas Gerais, Brasil", latitude: -19.9, longitude: -43.9 }],
    GetRain: async (from, to) => {
      const out = [];
      if (!settings.rainEnabled) return out;
      if (!rainFetched) { rainBusy = true; emit("rain-status", rainStatus()); await new Promise((r) => setTimeout(r, 900)); rainBusy = false; rainFetched = Date.now(); emit("rain-status", rainStatus()); }
      for (let t = Math.floor(from / 36e5) * 36e5; t < to; t += 36e5) {
        const h = Math.floor((now - t) / 36e5);
        if (h >= 5 && h <= 8) out.push({ time: t, mm: [0.4, 2.5, 4, 1.2][h - 5] });
      }
      return out;
    },
    GetPeriods: async () => periods.slice(),
    // "o Radiacode estava fora de casa?": uma pergunta de exemplo (de ontem à tarde)
    GetRcAwayAsks: async () => rcAsks.map((q) => ({ ...q })),
    AnswerRcAway: async (id, ans, from, to, name) => {
      const q = rcAsks.find((x) => x.id === id);
      if (!q) throw new Error("essa pergunta já foi respondida");
      let p = { id: 0 };
      if (ans === "voltei") p = { id: q.period, from: q.since, to: to || q.to, name: q.name };
      if (ans === "fora") p = await api.SavePeriod({ id: 0, from: from || q.from, to: to || q.to, name, gmc: "casa", radiacode: "comigo", notHome: true });
      rcAsks = rcAsks.filter((x) => x.id !== id);
      setTimeout(() => emit("rcaway", rcAsks.slice()), 0);
      return p;
    },
    GetPlannedTrips: async () => planned.map((p) => ({ ...p })),
    SavePlannedTrip: async (p) => {
      if (!p.gmc && !p.rc) throw new Error("escolha pelo menos um aparelho que vai com você");
      const q = { ...p, nome: (p.nome || "").trim() || "Saída programada", id: p.id || Date.now() };
      planned = planned.filter((x) => x.id !== q.id).concat(q).sort((a, b) => a.de - b.de);
      setTimeout(() => emit("planned", planned.slice()), 0);
      return q;
    },
    DeletePlannedTrip: async (id) => { planned = planned.filter((x) => x.id !== id); setTimeout(() => emit("planned", planned.slice()), 0); },
    NotLeaving: async (id) => { planned = planned.map((x) => (x.id === id ? { ...x, saiu: 0 } : x)); },
    AnswerPlanned: async (id, ans, from, to, name, gmc, rc) => {
      const q = rcAsks.find((x) => x.id === id);
      if (!q) throw new Error("essa pergunta já foi respondida");
      let p = { id: 0 };
      if (ans === "sai") p = await api.SavePeriod({ id: 0, from: from || q.from, to: to || q.to, name: name || q.name, gmc: gmc ? "comigo" : "casa", radiacode: rc ? "comigo" : "casa", notHome: true, planned: q.planned, estimated: q.estimated && from === q.from });
      rcAsks = rcAsks.filter((x) => x.id !== id);
      setTimeout(() => emit("rcaway", rcAsks.slice()), 0);
      return p;
    },
    UndoRcAway: async (ask, pid) => {
      if (pid && ask.kind !== "volta") await api.DeletePeriod(pid);
      const q = rcAsksAll.find((x) => x.id === ask.id);
      if (q && !rcAsks.some((x) => x.id === q.id)) rcAsks.push(q);
      setTimeout(() => emit("rcaway", rcAsks.slice()), 0);
    },
    GetMeasurements: async () => measurements.slice(),
    UpdateMeasurement: async (id, name, note) => { measurements = measurements.map((m) => (m.id === id ? { ...m, name: name.trim() || m.name, note: note.trim() } : m)); setTimeout(() => emit("measurements", measurements.slice()), 0); },
    DeleteMeasurement: async (id) => { measurements = measurements.filter((m) => m.id !== id); setTimeout(() => emit("measurements", measurements.slice()), 0); },
    SavePeriod: async (p) => {
      const g = p.gmc || "comigo", r = p.radiacode || "comigo";
      const out = !!p.notHome && g === "casa" && r === "comigo"; // saí com o RC102 (2.38.0)
      if (g === "casa" && r === "comigo" && !out) throw new Error("GMC-800 em casa e RC102 com você é o normal: não precisa marcar");
      const open = !p.to && periods.some((q) => q.id === p.id && !q.to);
      if (!open && !(p.to - p.from >= 6e4)) throw new Error("o período precisa ter pelo menos 1 minuto");
      if (periods.some((q) => q.id !== p.id && p.from < (q.to || Infinity) && q.from < (p.to || Infinity))) throw new Error("esse trecho cruza com outro já marcado");
      p = { ...p, gmc: g, radiacode: r, notHome: !!p.notHome && g === "casa" && (r === "casa" || r === "comigo"), name: (p.name || "").trim() || whKind({ gmc: g, radiacode: r, notHome: out }) };
      if (p.id) periods = periods.map((q) => (q.id === p.id ? p : q));
      else { p.id = Math.max(0, ...periods.map((q) => q.id)) + 1; periods.push(p); }
      periods.sort((a, b) => a.from - b.from);
      setTimeout(() => { emit("periods", periods.slice()); emit("whereabouts", mockWhere()); }, 0);
      return p;
    },
    DeletePeriod: async (id) => { periods = periods.filter((q) => q.id !== id); setTimeout(() => { emit("periods", periods.slice()); emit("whereabouts", mockWhere()); }, 0); },
    GetWhereabouts: async () => mockWhere(),
    // resumo da saída (2.67): números do resumo fingido + espectro de exemplo com mais K-40
    GetTripSummary: async (id) => {
      const p = periods.find((q) => q.id === id);
      if (!p) throw new Error("período não encontrado");
      const to = p.to || Date.now();
      const sum = await api.GetSummary(p.from, to);
      const rc = sum.rc ? { ...sum.rc, curve: [] } : null;
      const step = 6e4;
      if (rc) {
        let seed = 7;
        const rnd = () => { seed = (seed * 16807) % 2147483647; return seed / 2147483647; };
        for (let t = p.from; t < to; t += step) {
          const bump = Math.exp(-Math.pow((t - p.from - (to - p.from) * 0.42) / 9e5, 2)) * 0.09;
          rc.curve.push({ t, usv: 0.158 + bump + (rnd() - 0.5) * 0.025 });
        }
        rc.max = Math.max(...rc.curve.map((q) => q.usv)); rc.maxAt = rc.curve.find((q) => q.usv === rc.max).t;
        rc.avg = rc.curve.reduce((a, q) => a + q.usv, 0) / rc.curve.length; rc.dose = (rc.avg * (to - p.from)) / 36e5; rc.devOK = false; rc.coverageAll = 1;
      }
      const n = 1024, a = [0, 2.93, 0.0001], obj = [], bg = [];
      const en = (i) => a[0] + a[1] * (i + 0.5) + a[2] * (i + 0.5) ** 2;
      const pk = (e, c, w) => Math.exp(-Math.pow((e - c) / w, 2));
      for (let i = 0; i < n; i++) {
        const e = en(i), cont = 9000 / (1 + e / 60) + 30;
        const lines = (k) => 600 * pk(e, 609, 28) + k * 900 * pk(e, 1461, 40) + 260 * pk(e, 2614, 55) + 300 * pk(e, 352, 22);
        obj.push(Math.round((cont + lines(1.6)) * 16200 / 3600));
        bg.push(Math.round((cont + lines(1)) * 86400 / 3600));
      }
      const peaks = [{ group: "Rádio-226 (série do urânio)", nuclide: "Pb-214", kev: 351.9 }, { group: "Rádio-226 (série do urânio)", nuclide: "Bi-214", kev: 609.3 }, { group: "Potássio-40", nuclide: "K-40", kev: 1460.8 }, { group: "Tório-232 (série)", nuclide: "Tl-208", kev: 2614.5 }];
      return {
        period: p, from: p.from, to, open: !p.to, defaultName: p.name === whKind(whOf(p)), rc, stepMs: step, homeUSv: 0.157, homeSrc: "gmc",
        spec: { a, obj, objSec: (to - p.from) / 1000, bg, bgSec: 86400, peaks }, specRows: 1,
        lines: [{ nuclide: "K-40", group: "Potássio-40", kev: 1460.8, pct: 60, more: true, hasHome: true, out: 4.8, home: 3.0 }, { nuclide: "Pb-214", group: "Rádio-226 (série do urânio)", kev: 351.9, pct: -22, less: true, hasHome: true, out: 6.2, home: 7.95 },
          { nuclide: "Bi-214", group: "Rádio-226 (série do urânio)", kev: 609.3, pct: 2, more: false, hasHome: true, out: 9.1, home: 8.9 }, { nuclide: "Tl-208", group: "Tório-232 (série)", kev: 2614.5, pct: -3, more: false, hasHome: true, out: 1.2, home: 1.24 }],
        hasBat: true, bat0: 92, bat1: 71, hasTemp: true, tempMin: 24, tempMax: 31, tempMaxAt: p.from + (to - p.from) * 0.47, events: [],
      };
    },
    SetWhereAt: async (g, r, name, from) => {
      const w = await api.SetWhere(g, r, name);
      const o = periods.find((q) => q.to === 0);
      if (o && from < o.from) { o.from = Math.floor(from / 6e4) * 6e4; setTimeout(() => emit("periods", periods.slice()), 0); }
      return mockWhere();
    },
    GetDeviceMarks: async (from, to) => {
      const h = Math.floor((Date.now() - 864e5) / 36e5) * 36e5;
      return [{ device: "radiacode", kind: "sem dado", from: h - 5 * 36e5, to: h - 2 * 36e5 }, { device: "gmc", kind: "saturando", from: h + 36e5, to: h + 36e5 + 9e5 },
        { device: "gmc", kind: "sem contagens", from: h - 3 * 864e5, to: h - 3 * 864e5 + 2 * 36e5 }].filter((m) => m.to > from && m.from < to);
    },
    LeaveHome: async (gGoes, rGoes) => api.SetWhere(gGoes ? "comigo" : "casa", rGoes ? "comigo" : "casa", "", !gGoes),
    SetWhere: async (g, r, name, notHome) => {
      const now = Math.floor(Date.now() / 6e4) * 6e4;
      const cur = mockWhere(), nh = !!notHome && g === "casa" && r !== "fora";
      if (cur.gmc === g && cur.radiacode === r && cur.notHome === nh) return cur;
      periods = periods.filter((q) => !(q.to === 0 && now - q.from < 6e4)).map((q) => (q.to === 0 ? { ...q, to: now } : q));
      if (!(g === "casa" && r === "comigo" && !nh)) periods.push({ id: Math.max(0, ...periods.map((q) => q.id)) + 1, from: now, to: 0, name: (name || "").trim() || (nh && r === "comigo" ? "Saí com o RC102" : whKind({ gmc: g, radiacode: r })), gmc: g, radiacode: r, notHome: nh });
      setTimeout(() => { emit("periods", periods.slice()); emit("whereabouts", mockWhere()); }, 0);
      return mockWhere();
    },
    GetDataStart: async () => { const d = new Date(); return new Date(d.getFullYear(), d.getMonth() - 1, 1).getTime(); },
    GetHeatmapMonth: async (y, m) => {
      const first = new Date(y, m - 1, 1), t = new Date(); t.setHours(0, 0, 0, 0); t.setDate(t.getDate() + 1);
      const end = Math.min(new Date(y, m, 1).getTime(), t.getTime());
      return api.GetHeatmap(Math.max(0, Math.round((end - first.getTime()) / 864e5)), first.getTime());
    },
    GetHeatmap: async (days, from) => {
      const d0 = new Date(); d0.setHours(0, 0, 0, 0); d0.setDate(d0.getDate() - (days - 1));
      if (from) d0.setTime(from);
      const first = d0.getTime(), dayList = [], values = [], usv = [], counts = [], hs = new Array(24).fill(0), hc = new Array(24).fill(0);
      let mn = null, mx = null;
      for (let i = 0; i < days; i++) {
        const ds = new Date(first); ds.setDate(ds.getDate() + i); dayList.push(ds.getTime());
        const row = [], urow = [], crow = [];
        for (let hr = 0; hr < 24; hr++) {
          const a = ds.getTime() + hr * 36e5, p = range(a, a + 36e5);
          crow.push(p.length);
          if (!p.length) { row.push(null); urow.push(null); continue; }
          const v = p.reduce((x, q) => x + q.cpm, 0) / p.length + 2 * Math.sin((hr - 4) / 24 * 2 * Math.PI) * -1;
          row.push(v); urow.push(v * 0.0101); hs[hr] += v; hc[hr]++;
          mn = mn == null || v < mn ? v : mn; mx = mx == null || v > mx ? v : mx;
        }
        values.push(row); usv.push(urow); counts.push(crow);
      }
      const hourAvg = hs.map((v, i) => (hc[i] ? v / hc[i] : null));
      let peak = -1, low = -1;
      hourAvg.forEach((v, i) => { if (v == null) return; if (peak < 0 || v > hourAvg[peak]) peak = i; if (low < 0 || v < hourAvg[low]) low = i; });
      const alerts = {};
      for (const e of events.concat(demoAlerts)) {
        if (e.kind !== "alerta") continue;
        const stop = e.active ? Date.now() : e.end || e.time + 6e4;
        for (let t = Math.floor(e.time / 36e5) * 36e5; t < stop; t += 36e5) {
          const dd = new Date(t), d0 = new Date(dd); d0.setHours(0, 0, 0, 0);
          const i = Math.round((d0.getTime() - first) / 864e5);
          if (i < 0 || i >= days) continue;
          (alerts[`${i}-${dd.getHours()}`] = alerts[`${i}-${dd.getHours()}`] || []).push({ level: e.level || "atencao", time: e.time, peak: e.peak || e.cpm || 0, retro: !!e.retro });
        }
      }
      return { days: dayList, values, usv, counts, alerts, hourAvg, min: mn, max: mx, peakHour: peak, lowHour: low, minutes: 0 };
    },
    GetMemoryInfo: async () => ({ known: true, usedBytes: 38912, totalBytes: 2097152, at: Date.now() - 3600e3, serial: "" }),
    DiffDeviceConfig: async () => [{ index: 124, before: 1, after: 0, field: "" }],
    GetDeviceConfig: async () => { await new Promise((r) => setTimeout(r, 600)); return JSON.parse(JSON.stringify(demoCfg)); },
    SetDeviceConfig: async (c) => { await new Promise((r) => setTimeout(r, 1500)); Object.assign(demoCfg, JSON.parse(JSON.stringify(c))); return JSON.parse(JSON.stringify(demoCfg)); },
    GetStats: async () => {
      const t = Date.now();
      const h = range(t - 3600e3, t + 1), d = range(t - 864e5, t + 1);
      const today = new Date(); today.setHours(0, 0, 0, 0);
      const td = range(today.getTime(), t + 1);
      const mx = d.reduce((a, p) => (p.cpm > a.cpm ? p : a), d[0] || { cpm: null, t: 0 });
      return {
        avg1h: h.length ? h.reduce((a, p) => a + p.cpm, 0) / h.length : null,
        min24h: d.length ? Math.min(...d.map((p) => p.cpm)) : null,
        max24h: mx.cpm, max24hAt: mx.t,
        doseToday: td.reduce((a, p) => a + (p.cpm * 0.0101) / 60, 0), minutesToday: td.length,
      };
    },
    GetDaily: async () => {
      const out = [];
      for (let k = 0; k < 7; k++) {
        const d0 = new Date(); d0.setHours(0, 0, 0, 0); d0.setDate(d0.getDate() - k);
        const p = range(d0.getTime(), d0.getTime() + 864e5);
        if (p.length) { const c = p.reduce((a, x) => a + x.cpm, 0) / p.length; out.push({ day: d0.getTime(), cpm: c, usv: c * 0.0101, minutes: p.length }); }
      }
      return out;
    },
    GetDoseRange: async (from, to, limit) => {
      const span = Math.max(3e5, to - from), steps = [6e4, 12e4, 3e5, 6e5, 9e5, 18e5, 36e5, 72e5, 216e5, 432e5, 864e5];
      const step = steps.find((x) => span / x <= 720) || 864e5;
      const f = Math.floor(Math.min(from, to) / step) * step;
      return api._dose(f, f + Math.ceil(span / step) * step, step, limit);
    },
    GetDose: async (period, limit) => {
      const [span, step] = { "15m": [9e5, 6e4], "30m": [18e5, 6e4], "1h": [36e5, 6e4], "3h": [108e5, 6e4], "6h": [216e5, 6e4], "12h": [432e5, 12e4], "24h": [864e5, 3e5], "7d": [6048e5, 36e5], "30d": [2592e6, 216e5], "1y": [31536e6, 864e5] }[period] || [864e5, 3e5];
      const to = Math.floor(Date.now() / 6e4) * 6e4 + 6e4;
      return api._dose(to - span, to, step, limit);
    },
    // Radiacode fingido: dose um pouco abaixo da do GMC
    _doseSrc: (c, src) => {
      if (src !== "radiacode" || !demoRcFake()) return Object.assign(c, { device: "gmc", other: "radiacode", otherTotal: demoRcFake() ? c.total * 0.6 : 0, otherMinutes: demoRcFake() ? c.measuredMinutes : 0 });
      const k = 0.6, sc = (a) => a.map((v) => v * k);
      return Object.assign({}, c, { device: "radiacode", other: "gmc", otherTotal: c.total, otherMinutes: c.measuredMinutes, doseSource: "aparelho", appMinutes: Math.round(c.measuredMinutes * 0.9), appCoverage: c.coverage * 0.9,
        dose: sc(c.dose), background: sc(c.background), ceiling: c.ceiling.map((v, i) => v - c.background[i] + c.background[i] * k),
        total: c.total * k, appTotal: c.total * k * 0.9, backgroundTotal: c.backgroundTotal * k, ceilingTotal: c.ceilingTotal - c.backgroundTotal * (1 - k), excess: c.excess * k, backgroundRate: c.backgroundRate * k, backgroundSource: "mediana em casa dos últimos 30 dias" });
    },
    GetDoseSrc: async (period, limit, src) => api._doseSrc(await api.GetDose(period, limit), src),
    GetDoseRangeSrc: async (from, to, limit, src) => api._doseSrc(await api.GetDoseRange(from, to, limit), src),
    // Minha dose (2.17): os mesmos cálculos da dose nos períodos do calendário
    GetMyDose: async (limit) => {
      const now = Math.floor(Date.now() / 6e4) * 6e4 + 6e4, d = new Date(now);
      const day = new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
      const wk = day - ((d.getDay() + 6) % 7) * 864e5;
      const ps = [["dia", day], ["semana", wk], ["mes", new Date(d.getFullYear(), d.getMonth(), 1).getTime()], ["ano", new Date(d.getFullYear(), 0, 1).getTime()]];
      const lim = limit === "trabalhador" ? 20 : 1;
      const one = async (src) => {
        const out = { device: src === "radiacode" ? "radiacode" : "gmc", hasData: false, periods: [], backgroundRate: 0 };
        for (const [key, from] of ps) {
          const c = await api.GetDoseRangeSrc(from, now, limit, src);
          const above = c.backgroundTotal > 0 ? c.total / c.backgroundTotal - 1 : 0;
          out.periods.push({ key, from, to: now, total: c.total, background: c.backgroundTotal, allowance: c.allowance, excess: c.excess, above, measuredMinutes: c.measuredMinutes, coverage: c.coverage, doseSource: c.doseSource });
          if (c.measuredMinutes > 0) out.hasData = true;
          out.backgroundRate = c.backgroundRate;
          if (key === "ano") {
            out.yearProjection = c.measuredMinutes ? (c.total / c.measuredMinutes) * 525960 : 0;
            out.yearBackground = c.backgroundRate * 24 * 365.25;
            out.yearCeiling = out.yearBackground + lim * 1000;
          }
        }
        return out;
      };
      const personal = demoRcFake() ? await one("radiacode") : { device: "radiacode", hasData: false, periods: [], backgroundRate: 0 };
      const home = await one("gmc");
      // trechos fora do normal: só marca quem mediu (o modo navegador não refaz a conta)
      const w = mockWhere();
      for (const [d, role] of [[personal, "personal"], [home, "home"]]) {
        d.device = w[role];
        for (const p of d.periods) {
          let other = 0, gap = 0;
          for (const q of periods) {
            const f = Math.max(q.from, p.from), t = Math.min(q.to || now, now);
            if (t <= f) continue;
            const g = q.gmc || "comigo", r = q.radiacode || "comigo";
            const has = role === "personal" ? (r === "comigo" || g === "comigo" || (g === "casa" && r === "casa" && !q.notHome)) : (g === "casa" || r === "casa");
            const swapped = role === "personal" ? r !== "comigo" && g !== "fora" : g !== "casa" && r === "casa";
            if (!has) gap += (t - f) / 6e4; else if (swapped) other += (t - f) / 6e4;
          }
          p.gapMinutes = Math.round(gap); p.swapMinutes = Math.round(Math.min(other, p.measuredMinutes));
          p.device = p.swapMinutes ? "misto" : role === "personal" ? "radiacode" : "gmc";
          p.mixed = !!p.swapMinutes;
        }
      }
      return { now, limitName: "", limitMSvYear: lim, personal, home, normal: w.normal, default: personal.hasData ? "pessoal" : "casa" };
    },
    _dose: (from, to, step, limit) => {
      const span = to - from;
      const p = range(from, to);
      const all = range(Date.now() - 2592e6, to).map((q) => q.cpm).sort((a, b) => a - b);
      const rate = (all[Math.floor(all.length / 2)] || 0) * 0.0101;
      const limY = limit === "trabalhador" ? 20 : 1, limMin = limY * 1000 / 525960;
      const times = [], dose = [], bg = [], ceil = [];
      let d = 0, b = 0, al = 0, k = 0;
      for (let e = from + step; e <= to; e += step) {
        while (k < p.length && p[k].t < e) { d += p[k].cpm * 0.0101 / 60; b += rate / 60; al += limMin; k++; }
        times.push(e); dose.push(d); bg.push(b); ceil.push(b + al);
      }
      return { from, to, stepMs: step, times, dose, background: bg, ceiling: ceil, total: d, doseSource: "app", calNote: "mudou no período · até 05/11/2026 16:27: 0,01014 µSv/h por CPM no fundo (versão 1, curva do contador) · desde 05/11/2026 16:27: 0,00550 µSv/h por CPM no fundo (versão 2, informado à mão)", backgroundTotal: b, ceilingTotal: b + al,
        allowance: al, excess: d - b, backgroundRate: rate, backgroundSource: "mediana dos últimos 30 dias", limitMSvYear: limY, limitName: limY === 20 ? "Trabalhador exposto" : "Público",
        measuredMinutes: p.length, coverage: Math.min(1, p.length / (span / 6e4)) };
    },
    // de onde vem a dose (2.43): pela fita, com o fundo fingido do RC102 a 0,092
    GetDoseOrigin: async (days) => {
      const to = Date.now(), from = to - (days || 7) * 864e5, s = await api.GetDayStrip(from, to);
      const o = { from, to, device: demoRcFake() ? "radiacode" : "gmc", total: 0, homeBg: 0, awayBg: 0, above: 0, minutes: 0, items: [], homeBgRate: demoRcFake() ? 0.092 : 0.206 };
      for (const x of s.slots) {
        if (!x.pers && !x.home) continue;
        const u = x.persUSv || x.homeUSv, d = u * 10 / 60, bg = o.homeBgRate;
        o.total += d; o.minutes += 10;
        const base = u > bg * 1.2 ? bg * 10 / 60 : d;
        o.above += d - base;
        if (x.away) o.awayBg += base; else o.homeBg += base;
      }
      o.items = [{ from: to - 40 * 6e4, to: to - 30 * 6e4, dose: 0.021, label: "Medir objeto · Banana com casca", kind: "medicao" },
        { from: to - 5 * 36e5, to: to - 5 * 36e5 + 6e5, dose: 0.012, label: "Alarme no RC102", kind: "alerta" }];
      o.above += 0.033; o.total += 0.033;
      return o;
    },
    // fita do dia (2.42): pedaços de 10 min, nível pela média; RC102 fingido a 0,6× o GMC
    GetDayStrip: async (from, to) => {
      const step = 6e5, f = Math.floor(from / step) * step, n = Math.ceil((Math.min(to, f + 8 * 864e5) - f) / step);
      const lv = (u, b) => (u >= b[2] ? "perigo" : u >= b[1] ? "alarme" : u >= b[0] ? "atencao" : "seguro");
      const gb = [50 * 0.0101, (settings.alarmCPM || 100) * 0.0101, 10.1], rb = [0.4, 1.2, 12];
      const slots = [];
      for (let i = 0; i < n; i++) {
        const t = f + i * step, p = range(t, t + step), s = { t };
        const o = periods.find((q) => q.from < t + step && (q.to || Infinity) > t);
        if (o) s.away = true;
        const g = o ? o.gmc || "comigo" : "casa";
        if (p.length) {
          const u = (p.reduce((x, q) => x + q.cpm, 0) / p.length) * 0.0101;
          if (g === "casa") Object.assign(s, { home: lv(u, gb), homeUSv: u, homeDev: "gmc" });
          else Object.assign(s, { pers: lv(u, gb), persUSv: u, persDev: "gmc" });
          if (demoRcFake() && g === "casa") { const r = u * 0.6 + Math.sin(i / 3) * 0.004; Object.assign(s, { pers: lv(r, rb), persUSv: r, persDev: "radiacode" }); }
        }
        if (i % 144 >= 93 && i % 144 < 104 && t < Date.now() - 864e5) s.rain = 1.4;
        slots.push(s);
      }
      const marks = events.concat(demoAlerts).filter((e) => e.kind === "alerta" && e.time >= f && e.time < to)
        .map((e) => ({ t: e.time, kind: "alerta", level: e.level, device: /RC102/.test(e.text) ? "radiacode" : "", text: e.text }))
        .concat(notes.filter((x) => x.time >= f && x.time < to).map((x) => ({ t: x.time, kind: "nota", text: x.text })))
        .sort((x, y) => x.t - y.t);
      return { from: f, to, stepMs: step, slots, marks };
    },
    GetChartRange: async (from, to) => {
      const c = withPeaks(await chartRangePreview(from, to));
      if (demoRcFake()) {
        c.rc = (c.usv || []).map((v, i) => (v == null ? null : v * 0.6 + Math.sin(i / 3) * 0.004));
        c.rcCPM = c.rc.map((v) => (v == null ? null : v * 4950));
      }
      return c;
    },
    _chartRange: async (from, to) => {
      const span = Math.max(12e4, to - from), step = span <= 36e5 ? 6e4 : span <= 864e5 ? 3e5 : 18e5;
      const f = Math.floor(Math.min(from, to) / step) * step, n = Math.ceil(span / step);
      const vals = [], times = [];
      for (let i = 0; i < n; i++) {
        const a = f + i * step, p = range(a, a + step);
        times.push(a);
        vals.push(p.length ? p.reduce((x, q) => x + q.cpm, 0) / p.length : null);
      }
      const ok = vals.filter((v) => v != null);
      const avg = ok.length ? ok.reduce((x, v) => x + v, 0) / ok.length : null;
      return { from: f, to: f + n * step, stepMs: step, times, values: vals, usv: vals.map((v) => (v == null ? null : v * 0.0101)),
        avg, min: ok.length ? Math.min(...ok) : null, max: ok.length ? Math.max(...ok) : null,
        avgUSv: avg == null ? null : avg * 0.0101, minUSv: ok.length ? Math.min(...ok) * 0.0101 : null, maxUSv: ok.length ? Math.max(...ok) * 0.0101 : null,
        attentionCPM: 50, attentionUSv: 50 * 0.0101, alarmCPM: settings.alarmCPM || 100, alarmUSv: (settings.alarmCPM || 100) * 0.0101, dangerCPM: 1000, dangerUSv: 10.1 };
    },
    GetChart: async (period) => {
      const c = await chartPreview(period);
      const u = (v) => (v == null ? null : v * 0.0101);
      withPeaks(c);
      if (demoRcFake()) { // RC102 fingido: segue o GMC um pouco abaixo
        c.rc = (c.usv || []).map((v, i) => (v == null || i < c.usv.length * 0.3 ? null : v * 0.9 + (Math.sin(i / 3) * 0.004)));
        const ok = c.rc.filter((v) => v != null);
        c.rcAvg = ok.length ? ok.reduce((a, b) => a + b, 0) / ok.length : null;
        // CPM do aparelho: umas 28× as contagens do GMC (≈ 4950 CPM por µSv/h)
        c.rcCPM = c.rc.map((v) => (v == null ? null : v * 4950));
        c.rcAvgCPM = c.rcAvg == null ? null : c.rcAvg * 4950;
      }
      return Object.assign(c, { avgUSv: u(c.avg), minUSv: u(c.min), maxUSv: u(c.max) });
    },
    _unused: async (period) => {
      const [span, step] = { "15m": [9e5, 5e3], "30m": [18e5, 1e4], "3h": [108e5, 6e4], "6h": [216e5, 6e4], "12h": [432e5, 12e4], "24h": [864e5, 3e5], "7d": [6048e5, 18e5], "30d": [2592e6, 72e5], "1y": [31536e6, 864e5] }[period] || [3600e3, 6e4];
      const to = Math.floor(Date.now() / step) * step + step, from = to - span;
      const n = span / step, sums = new Array(n).fill(0), cnt = new Array(n).fill(0);
      const all = range(from, to);
      all.forEach((p) => {
        // curto: o valor do minuto preenche os intervalos daquele minuto
        for (let t = p.t; t < p.t + Math.max(step, 6e4); t += step) {
          const i = Math.floor((t - from) / step);
          if (i >= 0 && i < n) { sums[i] += p.cpm; cnt[i]++; }
        }
      });
      return {
        from, to, stepMs: step,
        times: sums.map((_, i) => from + i * step),
        values: sums.map((s, i) => (cnt[i] ? s / cnt[i] : null)),
        usv: sums.map((s, i) => (cnt[i] ? (s / cnt[i]) * 0.0101 : null)),
        attentionUSv: 50 * 0.0101,
        alarmCPM: settings.alarmCPM || 100, alarmUSv: (settings.alarmCPM || 100) * 0.0101,
        dangerCPM: (settings.levels || {}).dangerCPM || 1000, dangerUSv: ((settings.levels || {}).dangerCPM || 1000) * 0.0101,
        attentionCPM: (settings.levels || {}).attentionCPM || 50,
        avg: all.length ? all.reduce((a, p) => a + p.cpm, 0) / all.length : null,
        min: all.length ? Math.min(...all.map((p) => p.cpm)) : null,
        max: all.length ? Math.max(...all.map((p) => p.cpm)) : null,
      };
    },
  };
  return api;
}
