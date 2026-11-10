// Medir um objeto: medição, segundo aparelho, espectro e medições salvas.
// Carregado antes do app.js; só define funções e constantes.

// ---------- medir um objeto ----------

// ---------- gráfico durante a medição ----------
// CPM ao vivo (linha fina) e a média acumulada de cada fase com a margem de
// erro (faixa), segundo a segundo.

const mlog = { start: 0, live: [], ph: [], demo: false, other: [] };

function measureLogStep(st) {
  const active = st.phase === "fundo" || st.phase === "objeto";
  if (!st.phase) { mlog.start = 0; mlog.live = []; mlog.ph = []; mlog.other = []; return; }
  const fresh = st.elapsed <= 1 && (st.phase === "fundo" || (st.phase === "objeto" && st.bg && st.bg.source === "casa"));
  if (fresh) { mlog.start = Date.now(); mlog.live = []; mlog.ph = []; mlog.other = []; mlog.demo = !!st.demo; state.mSel = null; if (state.mMode === "salvas") setMeasureMode("nova"); }
  if (!mlog.start) mlog.start = Date.now();
  if (active && st.elapsed > 0) {
    const m = st.elapsed / 60;
    // ao recomeçar a fase do objeto, tira o que tinha dela
    if (st.phase === "objeto" && st.elapsed <= 1) mlog.ph = mlog.ph.filter((p) => p.phase !== "objeto");
    mlog.ph.push({ t: Date.now(), phase: st.phase, avg: st.counts / m, err: Math.sqrt(Math.max(st.counts, 1)) / m });
  }
}

function measureLogReading(r) {
  const st = state.measureSt;
  if (!st || (st.phase !== "fundo" && st.phase !== "objeto")) return;
  // GMC como segundo aparelho: vai para a linha dele (µSv/h)
  if (st.other && st.other.device === "gmc" && !r.demo) { mlog.other.push({ t: Date.now(), usv: r.usv, phase: st.phase }); return; }
  if (!!r.demo !== !!st.demo || isRcMeasure(st)) return;
  mlog.live.push({ t: Date.now(), cpm: r.cpm });
}

// dose acumulada do próprio RadiaCode (desde que foi zerado), no cartão de
// dose: anda ao vivo com ele conectado
// dose do próprio RadiaCode (desde que foi zerado) e a conta do app, em duas
// linhas curtas dentro do quadro do RadiaCode
function renderDoseDevice() {
  const st = state.rc.st || {}, c = state.dose;
  $("doseDevice").hidden = true;
  const on = st.state === "conectado";
  const lines = [];
  if (c && c.device === "radiacode" && (st.doseLiveAt > 0 || on)) {
    if (st.doseLiveAt > 0) {
      const v = st.doseLive;
      const when = st.doseLiveOn ? "ao vivo" : on ? `total das ${hhmm(st.doseLiveAt)}` : `${whenLabel(st.doseLiveAt)}, desconectado`;
      lines.push(["Contador do aparelho:", `${fmt(v, v < 10 ? 3 : 2)} µSv desde que foi zerado` + (st.doseLiveSec ? ` (${rcDur(st.doseLiveSec)})` : "") + ` · ${when}`]);
    } else lines.push(["Contador do aparelho:", "esperando o primeiro total (vem a cada 3 a 4 min)"]);
    const dev = c.doseSource === "aparelho";
    const tot = dev ? c.appTotal : c.total, mins = dev ? c.appMinutes : c.measuredMinutes;
    const cov = (dev ? c.appCoverage : c.coverage) * 100;
    if (mins) lines.push(["Pelo app:", `${doseTxt(tot)} (${fmt(cov, cov < 10 ? 1 : 0)}% do período gravado)`]);
  }
  state.rcDevLines = lines;
  placeRcDevLines();
}
function placeRcDevLines() {
  const box = document.querySelector("#doseDevs .dd-box.rc");
  if (!box) return;
  let el = box.querySelector(".dd-dev");
  if (!el) { el = document.createElement("span"); el.className = "dd-dev small"; box.insertBefore(el, box.querySelector(".meter")); }
  el.textContent = "";
  for (const [k, v] of state.rcDevLines || []) {
    const row = document.createElement("span"), b = document.createElement("b");
    b.textContent = k;
    row.append(b, " " + v);
    el.appendChild(row);
  }
  el.hidden = !(state.rcDevLines || []).length;
}

// alertas do RadiaCode: a faixa de alarme/atenção e o som seguem o nível que
// o app abriu para ele
function rcAlarmState(st) {
  const lvl = st && st.state === "conectado" ? st.alertLevel || "" : "";
  const al = lvl === "alarme" || lvl === "perigo", dg = lvl === "perigo";
  const changed = al !== !!alarmCtl.rc || dg !== !!alarmCtl.rcDanger;
  alarmCtl.rc = al;
  alarmCtl.rcDanger = dg;
  if (changed || alarmCtl.kind === "rc") updateAlarm();
  setAttention("rc", !!lvl);
}

// RadiaCode medindo: a leitura ao vivo é o CPS do aparelho × 60
function measureLogRc(rs) {
  const st = state.measureSt;
  if (!st || (st.phase !== "fundo" && st.phase !== "objeto") || !rs || rs.state !== "conectado") return;
  if (st.other && st.other.device === "radiacode") {
    const lo = mlog.other[mlog.other.length - 1];
    if (!lo || Date.now() - lo.t >= 500) mlog.other.push({ t: Date.now(), usv: rs.usvh, phase: st.phase });
    return;
  }
  if (!isRcMeasure(st)) return;
  const last = mlog.live[mlog.live.length - 1];
  if (last && Date.now() - last.t < 500) return;
  mlog.live.push({ t: Date.now(), cpm: rs.cps * 60 });
}

// fonte do gráfico: a medição em andamento ou uma salva (clicada na lista)
function liveMeasureSrc() {
  const st = state.measureSt || {};
  const home = st.bg && st.bg.source === "casa";
  // eixo: até o fim previsto (no automático, o que falta pela estimativa)
  let expectSec = (st.seconds || 60) * (home ? 1 : 2) + 15;
  if (st.auto && st.phase === "objeto") {
    const before = mlog.start ? Math.max(0, (Date.now() - mlog.start) / 1000 - st.elapsed) : 0;
    expectSec = before + st.elapsed + (st.eta >= 0 ? st.eta : 60) + 5;
  } else if (st.auto && st.phase === "fundo") expectSec = st.seconds + 60;
  else if (st.phase !== "fundo" && st.phase !== "objeto") expectSec = 1; // terminou: o eixo vai até o último ponto
  return { start: mlog.start, live: mlog.live, ph: mlog.ph, seconds: st.seconds || 60, expectSec, bgFlat: home ? st.bg : null, k: mK(st),
    dev: isRcMeasure(st) ? "radiacode" : "gmc", other: st.other ? mlog.other : null, otherDev: st.other && st.other.device, otherRole: st.other && st.other.role };
}

function savedMeasureSrc(m) {
  const home = m.bg.source === "casa";
  const start = home ? m.obj.start : m.bg.start;
  const live = [], ph = [];
  const acc = {};
  for (const s of m.samples || []) {
    const t = start + s.t;
    if (s.cpm > 0) live.push({ t, cpm: s.cpm });
    const a = (acc[s.phase] = acc[s.phase] || { counts: 0, n: 0 });
    a.counts += s.cps;
    a.n++;
    const min = a.n / 60;
    ph.push({ t, phase: s.phase, avg: a.counts / min, err: Math.sqrt(Math.max(a.counts, 1)) / min });
  }
  const o = m.other;
  const other = o && o.samples ? o.samples.filter((s) => s.usv > 0).map((s) => ({ t: start + s.t, usv: s.usv, phase: s.phase })) : null;
  return { start, live, ph, seconds: m.bg.seconds || 60, done: true, bgFlat: home ? m.bg : null, k: mK(m),
    dev: isRcMeasure(m) ? "radiacode" : "gmc", other, otherDev: o && o.device, otherRole: o && o.role };
}

function drawMeasureChart() {
  const sel = state.mSel ? (state.measurements || []).find((m) => m.id === state.mSel) : null;
  if (state.mSel && !sel) state.mSel = null;
  $("btnMLiveBack").hidden = true;
  $("mLiveTitle").textContent = sel ? "Segundo a segundo" : "Durante a medição";
  if (sel && !(sel.samples && sel.samples.length)) {
    state.mSrc = null;
    $("mChart").innerHTML = "";
    $("mLiveSub").textContent = "— essa medição é de antes da 0.20.3 e não tem os dados por segundo";
    return;
  }
  drawMeasureSrc(sel ? savedMeasureSrc(sel) : liveMeasureSrc());
}

function drawMeasureSrc(src) {
  state.mSrc = src;
  const svg = $("mChart");
  if (!src.start || (!src.live.length && !src.ph.length)) {
    svg.innerHTML = "";
    $("mLiveSub").textContent = "— comece a medição";
    const ys = $("mY").children;
    for (let i = 0; i < 4; i++) ys[i].textContent = i === 3 ? "0" : "";
    return;
  }
  const t0 = src.start;
  const k = src.k || calK();
  const oth = (src.other || []).map((p) => ({ t: p.t, cpm: p.usv / k })); // na escala do principal
  const tAll = [...src.live.map((p) => p.t), ...src.ph.map((p) => p.t), ...oth.map((p) => p.t)];
  // o eixo vai até o fim previsto da medição (fundo + objeto), para ir enchendo
  const expected = src.done ? 0 : (src.expectSec || src.seconds * 2 + 15) * 1000;
  const t1 = Math.max(t0 + expected, t0 + 10e3, ...tAll);
  src.t1 = t1;
  let top = 0;
  for (const p of src.live) top = Math.max(top, p.cpm);
  for (const p of oth) top = Math.max(top, p.cpm);
  // a média dos primeiros segundos pula muito (1 contagem em 1 s = 60 CPM):
  // ela não entra na escala; a faixa de erro também pode passar do topo
  const first = {};
  for (const p of src.ph) { if (!(p.phase in first)) first[p.phase] = p.t; if (p.t - first[p.phase] >= 5000) top = Math.max(top, p.avg); }
  if (src.bgFlat) top = Math.max(top, src.bgFlat.cpm + src.bgFlat.errCPM);
  top = Math.max(10, Math.ceil((top * 1.15) / 10) * 10);
  const x = (t) => ((t - t0) / (t1 - t0)) * W;
  const y = (v) => H - (Math.min(top, Math.max(0, v)) / top) * H;
  // o gráfico fica em CPM por dentro; os números saem em µSv/h
  const ys = $("mY").children;
  for (let i = 0; i < 4; i++) { const v = (top - (top / 3) * i) * k; ys[i].textContent = i === 3 ? "0" : fmt(v, usvDigits(top * k)); }
  let out = [0, 1, 2, 3].map((i) => {
    const gy = i === 3 ? H - 0.5 : i === 0 ? 0.5 : (H / 3) * i;
    return `<line x1="0" y1="${gy}" x2="${W}" y2="${gy}" stroke="${COR.grid}" stroke-width="1" vector-effect="non-scaling-stroke"/>`;
  }).join("");
  if (src.bgFlat) {
    // fundo da casa: faixa reta (média ± σ) na medição toda
    const b = src.bgFlat, y1 = y(b.cpm + b.errCPM), y2 = y(b.cpm - b.errCPM);
    out += `<rect x="0" y="${y1.toFixed(1)}" width="${W}" height="${Math.max(1, y2 - y1).toFixed(1)}" fill="${COR.muted}" fill-opacity="0.18"/>`;
    out += `<line x1="0" y1="${y(b.cpm).toFixed(1)}" x2="${W}" y2="${y(b.cpm).toFixed(1)}" stroke="${COR.muted}" stroke-width="2" stroke-dasharray="6 4" vector-effect="non-scaling-stroke"/>`;
  }
  for (const [phase, color] of [["fundo", COR.muted], ["objeto", COR.obj]]) {
    const pts = src.ph.filter((p) => p.phase === phase);
    if (!pts.length) continue;
    const up = pts.map((p) => `${x(p.t).toFixed(1)} ${y(p.avg + p.err).toFixed(1)}`);
    const dn = pts.slice().reverse().map((p) => `${x(p.t).toFixed(1)} ${y(p.avg - p.err).toFixed(1)}`);
    out += `<path d="M${up.join("L")}L${dn.join("L")}Z" fill="${color}" fill-opacity="0.18"/>`;
    out += `<path d="M${pts.map((p) => `${x(p.t).toFixed(1)} ${y(p.avg).toFixed(1)}`).join("L")}" fill="none" stroke="${color}" stroke-width="2.5" vector-effect="non-scaling-stroke"/>`;
    if (phase === "objeto") {
      const xs = x(pts[0].t).toFixed(1);
      out += `<line x1="${xs}" y1="0" x2="${xs}" y2="${H}" stroke="${COR.obj}" stroke-width="1" stroke-dasharray="3 3" vector-effect="non-scaling-stroke"/>`;
    }
  }
  if (src.live.length > 1) {
    // quebra a linha nos intervalos (ex.: entre o fundo e o objeto)
    const d = src.live.map((p, i) => (i && p.t - src.live[i - 1].t <= 3000 ? "L" : "M") + `${x(p.t).toFixed(1)} ${y(p.cpm).toFixed(1)}`).join("");
    out += `<path d="${d}" fill="none" stroke="${DEV_COLOR[src.dev] || COR.accent}" stroke-opacity="0.6" stroke-width="1.2" vector-effect="non-scaling-stroke"/>`;
  }
  // segundo aparelho: a leitura dele em µSv/h, por cima
  if (oth.length > 1) {
    const d = oth.map((p, i) => (i && p.t - oth[i - 1].t <= 5000 ? "L" : "M") + `${x(p.t).toFixed(1)} ${y(p.cpm).toFixed(1)}`).join("");
    out += `<path d="${d}" fill="none" stroke="${DEV_COLOR[src.otherDev] || COR.rc}" stroke-width="1.6" stroke-dasharray="${src.otherRole === "voce" ? "5 3" : "0"}" vector-effect="non-scaling-stroke"/>`;
  }
  svg.innerHTML = out;
  // legenda: de que aparelho é cada linha
  $("mLiveKey").style.borderColor = DEV_COLOR[src.dev] || COR.accent;
  $("mLiveKeyTxt").textContent = src.otherDev ? `${devLabel(src.dev)} ao vivo (no objeto)` : "leitura ao vivo";
  $("mOtherKey").hidden = !src.otherDev;
  if (src.otherDev) {
    $("mOtherKeyI").style.borderColor = DEV_COLOR[src.otherDev];
    $("mOtherKeyI").style.borderTopStyle = src.otherRole === "voce" ? "dashed" : "solid";
    $("mOtherKeyTxt").textContent = `${devLabel(src.otherDev)} ao vivo (${src.otherRole === "voce" ? "com você" : "no objeto"})`;
  }
  const last = (ph) => { const l = src.ph.filter((p) => p.phase === ph); return l[l.length - 1]; };
  const b = last("fundo"), o = last("objeto");
  const parts = [];
  if (b) parts.push(`fundo ${numU(b.avg * k)} ± ${numU(b.err * k)}`);
  else if (src.bgFlat) parts.push(`fundo da casa ${numU(src.bgFlat.cpm * k)} ± ${numU(src.bgFlat.errCPM * k)}`);
  if (o) parts.push(`objeto ${numU(o.avg * k)} ± ${numU(o.err * k)}`);
  $("mLiveSub").textContent = parts.length ? "— " + parts.join(" · ") + " µSv/h" : "";
}

function setupMeasureHover() {
  const plot = $("mPlot");
  plot.addEventListener("mousemove", (ev) => {
    const src = state.mSrc;
    if (!src || !src.start || !(src.live.length || src.ph.length)) return;
    const r = plot.getBoundingClientRect();
    const t0 = src.start, t1 = src.t1;
    const t = t0 + ((ev.clientX - r.left) / r.width) * (t1 - t0);
    const near = (arr) => (arr.length ? arr.reduce((a, p) => (Math.abs(p.t - t) < Math.abs(a.t - t) ? p : a), arr[0]) : null);
    const lv = near(src.live), ph = near(src.ph);
    const ref = lv || ph;
    $("mCursor").hidden = false;
    $("mCursor").style.left = ((ref.t - t0) / (t1 - t0)) * 100 + "%";
    const tip = $("mTip");
    tip.hidden = false;
    const k = src.k || calK();
    const ov = src.other && src.other.length ? near(src.other) : null;
    tip.textContent = `${Math.round((ref.t - t0) / 1000)} s` + (lv ? ` · ${src.otherDev ? devLabel(src.dev) : "ao vivo"} ${uS(lv.cpm, k)}` : "") +
      (ov && Math.abs(ov.t - ref.t) < 5000 ? ` · ${devLabel(src.otherDev)} ${usvVal(ov.usv)}` : "") + (ph ? ` · média (${ph.phase}) ${numU(ph.avg * k)} ± ${numU(ph.err * k)} µSv/h` : "");
    tip.style.left = Math.min(ev.clientX - r.left + 12, r.width - tip.offsetWidth - 4) + "px";
  });
  plot.addEventListener("mouseleave", () => { $("mCursor").hidden = true; $("mTip").hidden = true; });
  $("btnMLiveBack").addEventListener("click", () => setMeasureMode("nova"));
}

// tabela com as contas da medição: contagens, tempo, taxa média e a
// incerteza de Poisson (1σ = raiz das contagens, levada para CPM)
const plusMinus = (v) => (v >= 0 ? "+" : "−") + fmt(Math.abs(v), 1);

// ---------- medição em µSv/h ----------
// As contas (incerteza, σ) são feitas nas contagens; a tela mostra tudo em
// µSv/h. k = µSv/h por CPM: o RadiaCode tem a dose dele (k da fase), o GMC
// usa a calibração.
const rcK = () => { const s = state.rc.st || {}; return s.cps > 0 && s.usvh > 0 ? s.usvh / (s.cps * 60) : 0.1 / 720; };
const devK = (dev) => (dev === "radiacode" ? rcK() : calK());
const phK = (ph, dev) => (ph && ph.cpm > 0 && ph.usv > 0 ? ph.usv / ph.cpm : devK(dev));
const mK = (m) => phK(m && (m.bg || m.obj), isRcMeasure(m) ? "radiacode" : "gmc");
const uS = (cpm, k) => usvVal(cpm * k); // CPM → "0,146 µSv/h"
const sgnUS = (v) => (v >= 0 ? "+" : "−") + usvVal(Math.abs(v));
const numU = (v) => fmt(v, v === 0 ? 0 : usvDigits(Math.abs(v)));
const cntTxt = (n) => `${(n || 0).toLocaleString(LOCALE)} ${n === 1 ? "contagem" : "contagens"}`;

function measureStats(m) {
  const tbl = document.createElement("table");
  tbl.className = "m-stats mono";
  const row = (cells, cls) => {
    const tr = document.createElement("tr");
    if (cls) tr.className = cls;
    cells.forEach((c, i) => { const td = document.createElement(i === 0 ? "th" : "td"); td.textContent = c; tr.appendChild(td); });
    return tr;
  };
  const thead = document.createElement("thead");
  thead.appendChild(row(["", "contagens", "tempo", "µSv/h", "± 1σ"]));
  const tb = document.createElement("tbody");
  const dev = isRcMeasure(m) ? "radiacode" : "gmc";
  for (const [nm, x] of [["Fundo", m.bg], ["Com o objeto", m.obj]]) {
    if (!x) continue;
    const k = phK(x, dev);
    tb.appendChild(row([nm === "Com o objeto" ? "Objeto" : x.source === "casa" ? "Fundo (casa)" : nm, x.counts.toLocaleString(LOCALE), mDurTxt(x.seconds), numU(x.cpm * k), `±${numU(x.errCPM * k)}`]));
  }
  const r = m.result;
  if (r) {
    const k = phK(m.obj, dev);
    tb.appendChild(row(["Diferença", "", "", (r.netUSv >= 0 ? "+" : "−") + numU(Math.abs(r.netUSv)), `±${numU(r.netErr * k)}`], "m-diff"));
  }
  tbl.append(thead, tb);
  const box = document.createElement("div");
  box.className = "m-stats-box";
  box.appendChild(tbl);
  if (r) {
    const p = document.createElement("p");
    p.className = "small muted";
    const sig = Math.abs(r.sigma);
    p.textContent = `A incerteza de cada linha é a de Poisson: raiz das contagens, convertida para µSv/h. A da diferença junta as duas (raiz da soma dos quadrados). ` +
      `A diferença vale ${fmt(sig, 1)} vezes a incerteza (${fmt(sig, 1)}σ)` +
      (r.sigma >= 3 ? ": é real, não variação da contagem." : r.sigma >= 2 ? ": provavelmente real, mas vale repetir." : r.sigma <= -3 ? ": o objeto deu abaixo do fundo, então o fundo provavelmente mudou (outro lugar, chuva)." : ": cabe na variação natural da contagem.");
    box.appendChild(p);
  }
  return box;
}

// opções da tela de medir (lembradas entre as vezes)
function measureOpts() {
  const t = $("mSecs").value, auto = t.startsWith("auto");
  return {
    name: $("mName").value, distance: $("mDist").value, auto, homeBg: $("mBgSrc").value === "casa", ...measureDevOpts(),
    seconds: auto ? 0 : parseInt(t, 10), maxSec: auto ? parseInt(t.slice(4), 10) : 0, pct: parseFloat($("mPct").value),
  };
}

// aparelho no objeto e modo (os dois perto, ou um com você)
function measureDevOpts() {
  const v = $("mDev").value;
  if (v === "ambos") return { device: "radiacode", mode: "ambos" };
  if (v === "voce") return { device: $("mObjDev").value, mode: "voce" };
  return { device: v, mode: "" };
}

async function refreshHomeBg(force) {
  const dev = measureDevOpts().device;
  if (!force && state.homeBgAt && state.homeBgDev === dev && Date.now() - state.homeBgAt < 30e3) return;
  state.homeBgAt = Date.now();
  state.homeBgDev = dev;
  if (state.homeBg && state.homeBg.device !== dev) state.homeBg = null;
  let hb;
  try { hb = await api.GetHomeBackgroundDev(dev); } catch (e) { hb = { ok: false, why: errMsg(e), device: dev }; }
  if (measureDevOpts().device !== dev) return; // trocou de aparelho no meio
  state.homeBg = { ...hb, device: dev };
  if (state.measureSt) renderMeasure(state.measureSt);
}

// aparelho da medição: o escolhido antes ou, da primeira vez, o RadiaCode
// se estiver conectado
const rcOnline = () => !!(state.rc.st && state.rc.st.state === "conectado");
function measureDevDefault() {
  if (localGet("m-mDev")) return;
  const want = rcOnline() ? "radiacode" : "gmc";
  if ($("mDev").value !== want) { $("mDev").value = want; state.homeBgAt = 0; }
}
const isRcMeasure = (x) => (x && x.device) === "radiacode";

const fmtMin = (sec) => (sec < 90 ? `${Math.max(1, Math.round(sec))} s` : `${Math.round(sec / 60)} min`);

function renderMeasure(st) {
  state.measureSt = st;
  measureLogStep(st);
  drawMeasureChart();
  const running = st.phase === "fundo" || st.phase === "objeto";
  const started = running || !!st.bg;
  $("measureTitle").textContent = "Medir um objeto" + (st.demo ? " (demo)" : "");
  for (const id of ["mSecs", "mBgSrc", "mPct", "mDist", "mDev", "mObjDev"]) $(id).disabled = started;
  if (!started) measureDevDefault();
  else if (st.mode) { $("mDev").value = st.mode; if (st.mode === "voce") $("mObjDev").value = st.device === "radiacode" ? "radiacode" : "gmc"; }
  else if (st.device) $("mDev").value = st.device === "radiacode" ? "radiacode" : "gmc";
  $("mObjDevBox").hidden = $("mDev").value !== "voce";
  const o = measureOpts();
  const rc = started ? isRcMeasure(st) : o.device === "radiacode";
  $("mDistLabel").textContent = o.mode === "ambos" ? "Distância até os aparelhos (opcional)" : rc ? "Distância até o RadiaCode (opcional)" : "Distância até o tubo (opcional)";
  const home = started ? !!(st.bg && st.bg.source === "casa") : o.homeBg;
  const auto = started ? !!st.auto : o.auto;
  $("mPctBox").style.visibility = o.auto ? "" : "hidden";
  if (!started) refreshHomeBg(false);
  const hb = state.homeBg;
  // dica embaixo das opções
  $("mOptHint").textContent = !started ? [
    (rc || o.mode) && !rcOnline() ? "O RadiaCode não está conectado." : "",
    o.mode && state.status.state !== "conectado" ? "O GMC-800 não está conectado." : "",
    o.mode === "ambos" ? "Os dois perto do objeto, ao mesmo tempo. O GMC vê beta e gama; o RadiaCode, quase só gama: comparando, dá uma pista do que o objeto emite." : "",
    o.mode === "voce" ? `Deixe o ${devLabel(o.device)} no objeto e o ${devLabel(o.device === "radiacode" ? "gmc" : "radiacode")} com você (no bolso ou na mão): no fim, o app mostra quanto você recebeu enquanto o objeto era medido.` : "",
    o.homeBg ? (rc ? "Fundo da casa: só vale se o RadiaCode passou a última hora parado no lugar onde vai medir o objeto (no bolso, o fundo é outro)." : "Fundo da casa: meça o objeto no lugar de sempre do contador (o fundo muda de um canto para outro).") : "O fundo é medido primeiro, com o objeto longe (2 m ou mais)" + (o.auto ? ", por 5 min." : ", pelo mesmo tempo do objeto."),
    o.auto ? `Para quando o objeto estiver claramente acima do fundo, ou quando der para garantir que ele sobe menos de ${o.pct}% do fundo.` : "",
  ].filter(Boolean).join(" ") : "";
  // passo 1: fundo
  $("mBgTitle").textContent = home ? "1. Fundo da casa" : "1. Fundo";
  $("btnMeasureBg").hidden = home;
  $("btnMeasureBg").disabled = running;
  $("btnMeasureBg").textContent = st.bg ? "Medir o fundo de novo" : "Medir o fundo";
  const objReady = !running && (st.bg ? true : home && hb && hb.ok);
  $("btnMeasureObj").disabled = !objReady;
  $("btnMeasureObj").textContent = st.obj ? "Medir de novo com o objeto" : "Medir com o objeto";
  const k = mK(st);
  const prog = (phase, fill, val, done) => {
    if (st.phase === phase) {
      const auto = phase === "objeto" && st.auto;
      const total = auto ? (st.eta >= 0 ? st.elapsed + st.eta : st.seconds) : st.seconds;
      $(fill).style.width = Math.min(100, (st.elapsed / Math.max(1, total)) * 100) + "%";
      // até agora: taxa e incerteza com as contagens já feitas
      const so = st.elapsed > 0 ? ` · ${numU(st.counts * 60 / st.elapsed * k)} ± ${uS(Math.sqrt(Math.max(st.counts, 1)) * 60 / st.elapsed, k)}` : "";
      $(val).textContent = (auto ? `${Math.floor(st.elapsed / 60)}:${pad(st.elapsed % 60)}` : `${st.elapsed}/${st.seconds} s`) + ` · ${st.counts} contagens${so}`;
    } else if (done) {
      $(fill).style.width = "100%";
      const dk = phK(done, isRcMeasure(st) ? "radiacode" : "gmc");
      $(val).textContent = done.source === "casa"
        ? `${numU(done.cpm * dk)} ± ${uS(done.errCPM, dk)} (${Math.round(done.seconds / 60)} min até ${hhmm(done.end)})`
        : `${numU(done.cpm * dk)} ± ${uS(done.errCPM, dk)} (${done.counts} contagens em ${done.seconds} s)`;
    } else {
      $(fill).style.width = "0%";
      $(val).textContent = "—";
    }
  };
  if (!st.bg && home) {
    // ainda não começou: mostra o fundo da casa que vai ser usado
    $("mBgFill").style.width = hb && hb.ok ? "100%" : "0%";
    const hk = hb && hb.ok && hb.cpm > 0 && hb.usv > 0 ? hb.usv / hb.cpm : devK(measureDevOpts().device);
    $("mBgVal").textContent = !hb ? "…" : hb.ok ? `${numU(hb.cpm * hk)} ± ${uS(hb.errCPM, hk)} (${hb.minutes} min até ${hhmm(hb.to)})` : hb.why;
  } else prog("fundo", "mBgFill", "mBgVal", st.bg);
  prog("objeto", "mObjFill", "mObjVal", st.obj);
  // os dois aparelhos ao vivo
  renderMeasureDuo(st, running);
  renderSideResult($("mOtherRes"), st.phase === "pronto" ? st : null);
  // resultado parcial ao vivo
  const lv = st.phase === "objeto" ? st.live : null;
  $("mObjLive").hidden = !lv;
  if (lv) {
    const sig = lv.sigma;
    let what = sig >= 3 ? "parece radioativo" : sig >= 2 ? "talvez um pouco acima" : "ainda não dá para dizer";
    let when = "";
    if (st.auto) when = st.elapsed < 20 ? " · juntando contagens…" : st.eta > 0 ? ` · ~${fmtMin(st.eta)} para decidir` : " · pode ir até o tempo máximo";
    $("mObjLive").innerHTML = `Até agora: <b>${(lv.netUSv >= 0 ? "+" : "−") + numU(Math.abs(lv.netUSv))} ± ${uS(lv.netErr, k)}</b> sobre o fundo (${fmt(sig, 1)}σ) · ${what}${when}`;
  }
  const r = st.result;
  $("mResult").hidden = !r;
  $("mSaved").hidden = !st.savedId;
  if (st.savedId) state.mLastSaved = st.savedId;
  const rp = $("mReport");
  rp.hidden = !st.savedId;
  if (st.savedId) {
    $("mReportText").textContent = st.reportPath ? "Relatório salvo: " + st.reportPath.split(/[\\/]/).pop()
      : st.reportError ? "Relatório não saiu: " + st.reportError : state.settings.noMeasureReport ? "" : "Gerando o relatório…";
    $("btnMReportOpen").hidden = !st.reportPath;
    $("btnMReportMake").hidden = !!st.reportPath;
    rp.dataset.id = st.savedId;
    rp.dataset.path = st.reportPath || "";
  }
  if (!r) return;
  $("mResult").className = "m-result " + r.level;
  $("mVerdict").textContent = r.verdict.charAt(0).toUpperCase() + r.verdict.slice(1) + ".";
  $("mStats").replaceChildren(measureStats({ bg: st.bg, obj: st.obj, result: r }));
  const sign = r.netCPM >= 0 ? "+" : "−";
  const ok = phK(st.obj, rc ? "radiacode" : "gmc");
  $("mNumbers").textContent = `${(r.netUSv >= 0 ? "+" : "−") + numU(Math.abs(r.netUSv))} ± ${uS(r.netErr, ok)} sobre o fundo ${rc ? "no detector" : "no tubo"} · ${fmt(r.ratio, 1)}× o fundo · ${fmt(r.sigma, 1)}σ`;
  $("mExplain").textContent = measureExplain(r, st.obj, auto, ok);
  renderMeasSpec($("mSpec"), st.spectrum);
}

// ---------- segundo aparelho (os dois perto, ou um com você) ----------

const devLabel = (d) => (d === "radiacode" ? "RadiaCode" : "GMC-800");
const DEV_COLOR = { radiacode: COR.rc, gmc: COR.accent };

// os dois aparelhos ao vivo, um embaixo do outro (tudo em µSv/h)
function renderMeasureDuo(st, running) {
  const box = $("mDuo"), o = st.other;
  box.hidden = !(running && o);
  if (box.hidden) { box.textContent = ""; return; }
  const pDev = isRcMeasure(st) ? "radiacode" : "gmc";
  const last = (state.demo.active ? state.lastReal : state.last) || {};
  const pNow = pDev === "radiacode" ? (state.rc.st || {}).usvh : last.usv;
  const k = mK(st);
  const rows = [];
  // aparelho principal (no objeto)
  {
    const parts = [`agora ${pNow != null ? usvVal(pNow) : "—"}`, `${cntTxt(st.counts)} nesta fase`];
    if (st.elapsed > 0) parts.push(`média ${uS(st.counts * 60 / st.elapsed, k)}`);
    if (st.phase === "objeto" && st.live) parts.push(`${sgnUS(st.live.netUSv)} sobre o fundo (${fmt(st.live.sigma, 1)}σ)`);
    rows.push([`${devLabel(pDev)} · no objeto`, parts.join(" · ")]);
  }
  // segundo aparelho
  {
    const parts = [`agora ${o.nowUSv ? usvVal(o.nowUSv) : "—"}`];
    if (o.role === "voce") {
      if (st.phase === "objeto") {
        parts.push(cntTxt(o.counts), `sua dose ${fmt(o.dose || 0, 4)} µSv`);
        if (o.bgUSv) parts.push(`nível de fundo ${usvVal(o.bgUSv)}`);
      } else parts.push("a sua dose começa a contar quando o objeto for medido");
    } else {
      parts.push(`${cntTxt(o.counts)} nesta fase`);
      const ok = phK(o.bg, o.device);
      if (o.elapsed > 0) parts.push(`média ${uS(o.counts * 60 / o.elapsed, ok)}`);
      if (st.phase === "objeto" && o.live) parts.push(`${sgnUS(o.live.netUSv)} sobre o fundo (${fmt(o.live.sigma, 1)}σ)`);
    }
    rows.push([`${devLabel(o.device)} · ${o.role === "voce" ? "com você" : "no objeto"}`, parts.join(" · ")]);
  }
  box.textContent = "";
  for (const [title, text] of rows) {
    const d = document.createElement("div");
    d.className = "m-duo-row";
    d.innerHTML = "<strong></strong><span class=\"mono\"></span>";
    d.children[0].textContent = title;
    d.children[1].textContent = text;
    box.appendChild(d);
  }
}

// resultado do segundo aparelho (medição terminada ou salva)
function renderSideResult(box, m) {
  const o = m && m.other;
  box.hidden = !o;
  if (!o) { box.textContent = ""; return; }
  const name = devLabel(o.device);
  box.textContent = "";
  const add = (cls, text) => { const e = document.createElement(cls === "strong" ? "strong" : "span"); if (cls !== "strong") e.className = cls; e.textContent = text; box.appendChild(e); return e; };
  if (o.role === "voce") {
    add("strong", `Com você (${name})`);
    const secs = m.obj ? m.obj.seconds : 0;
    add("mono", `${fmt(o.dose, 4)} µSv em ${fmtMin(secs)} · média ${usvVal(o.avgUSv)} · máx ${usvVal(o.maxUSv)}`);
    if (o.bgUSv) add("small", o.excess > 0
      ? `Acima do nível de fundo (${usvVal(o.bgUSv)}): +${fmt(o.excess, 4)} µSv, o que o objeto somou na sua dose nesse tempo.`
      : `Igual ao nível de fundo (${usvVal(o.bgUSv)}): o objeto não somou nada que dê para ver na sua dose.`);
    add("small muted", "Mostra quanto você recebeu enquanto o objeto era medido, na posição em que o aparelho estava (bolso, mão…).");
    return;
  }
  add("strong", `${name} (também perto do objeto)`);
  const r = o.result;
  if (r) add("mono", `${sgnUS(r.netUSv)} ± ${uS(r.netErr, phK(o.obj, o.device))} sobre o fundo · ${fmt(r.sigma, 1)}σ · ${fmt(r.ratio, 1)}× o fundo`);
  else add("small muted", "Sem leituras suficientes desse aparelho.");
  if (o.hint) add("small", o.hint);
  add("small muted", "O GMC vê beta e gama; o RadiaCode, quase só gama. Vale como pista: o tubo e o cristal ficam em lugares diferentes do objeto.");
}

// ---------- espectro da medição (RadiaCode) ----------

const kevTxt = (k) => k.toLocaleString(LOCALE, { useGrouping: false, maximumFractionDigits: 1, minimumFractionDigits: 1 });
const specFWHM = (e) => 65 * Math.sqrt(Math.max(e, 1) / 661.7);
const SPEC_LINES = [
  ["Am-241", 59.5], ["U-235", 185.7], ["Pb-212", 238.6], ["Pb-214", 295.2], ["Pb-214", 351.9], ["Tl-208", 583.2], ["Bi-214", 609.3],
  ["Cs-137", 661.7], ["Ac-228", 911.2], ["Co-60", 1173.2], ["Co-60", 1332.5], ["K-40", 1460.8], ["Bi-214", 1764.5], ["Tl-208", 2614.5],
];
// faixas de meio pico (iguais às do PDF), em contagens/s
function measSpecBins(sp) {
  const en = (ch) => { const c = ch + 0.5; return sp.a[0] + sp.a[1] * c + sp.a[2] * c * c; };
  const n = sp.obj.length, lo = 20, hi = Math.min(3000, en(n - 1));
  const hasBg = !!(sp.bg && sp.bg.length === n && sp.bgSec > 0);
  const bins = [];
  for (let e = lo; e < hi;) { const w = Math.max(specFWHM(e) / 2, 2 * sp.a[1]); bins.push({ e0: e, e1: Math.min(e + w, hi), obj: 0, bg: 0 }); e += w; }
  let j = 0;
  for (let i = 0; i < n; i++) {
    const e = en(i);
    if (e < lo || e >= hi) continue;
    while (j < bins.length - 1 && e >= bins[j].e1) j++;
    bins[j].obj += sp.obj[i] / sp.objSec;
    if (hasBg) bins[j].bg += sp.bg[i] / sp.bgSec;
  }
  return { bins, hasBg };
}

const specDur = (sec) => (sec < 300 ? `${Math.round(sec)} s` : rcDur(sec) || fmtMin(sec));
function renderMeasSpec(box, sp) {
  box.hidden = !(sp && sp.obj && sp.obj.length && sp.objSec > 0 && sp.a && sp.a[1] > 0);
  if (box.hidden) { box.textContent = ""; return; }
  const { bins, hasBg } = measSpecBins(sp);
  if (!bins.length) { box.hidden = true; return; }
  const W = 640, H = 210, PL = 44, PR = 10, PT = 18, PB = 22;
  let lo = Infinity, hi = 0;
  for (const b of bins) for (const v of [b.obj, b.bg]) if (v > 0) { lo = Math.min(lo, v); hi = Math.max(hi, v); }
  if (!hi) { box.hidden = true; return; }
  const dlo = Math.floor(Math.log10(lo)), dhi = Math.max(dlo + 1, Math.ceil(Math.log10(hi)));
  const e0 = bins[0].e0, e1 = bins[bins.length - 1].e1;
  const x = (e) => PL + ((e - e0) / (e1 - e0)) * (W - PL - PR);
  const y = (v) => PT + (H - PT - PB) * (1 - ((v > 0 ? Math.max(dlo, Math.log10(v)) : dlo) - dlo) / (dhi - dlo));
  let g = "";
  for (let d = dlo; d <= dhi; d++) {
    const yy = y(10 ** d).toFixed(1);
    g += `<line x1="${PL}" x2="${W - PR}" y1="${yy}" y2="${yy}" class="grid"/><text x="${PL - 5}" y="${(+yy + 4).toFixed(1)}" text-anchor="end" class="ax">${d >= 4 || d < -3 ? "1e" + d : fmt(10 ** d, Math.max(0, -d))}</text>`;
  }
  for (let e = 500; e < e1; e += 500) g += `<line x1="${x(e).toFixed(1)}" x2="${x(e).toFixed(1)}" y1="${PT}" y2="${H - PB}" class="grid"/><text x="${x(e).toFixed(1)}" y="${H - 6}" text-anchor="middle" class="ax">${e}</text>`;
  g += `<text x="${W - PR}" y="${H - 6}" text-anchor="end" class="ax">keV</text>`;
  // linhas conhecidas: risco embaixo; as que subiram acima do fundo, tracejado com nome
  const found = sp.peaks || [];
  for (const [nuc, e] of SPEC_LINES) {
    if (e < e0 || e > e1 || found.some((p) => p.kev === e)) continue;
    g += `<line x1="${x(e).toFixed(1)}" x2="${x(e).toFixed(1)}" y1="${H - PB - 6}" y2="${H - PB}" class="tick"><title>${nuc} · ${kevTxt(e)} keV</title></line>`;
  }
  // nomes que não cabem lado a lado descem uma linha
  const rows = [];
  for (const p of [...found].sort((a, b) => a.kev - b.kev)) {
    if (p.kev < e0 || p.kev > e1) continue;
    const xx = x(p.kev), lbl = `${p.nuclide} ${kevTxt(p.kev)}`, w = lbl.length * 6.8 + 6;
    const right = xx + 3 + w > W - PR; // perto da borda: escreve à esquerda do risco
    const a0 = right ? xx - 3 - w : xx, a1 = right ? xx : xx + 3 + w;
    let r = rows.findIndex((end) => end < a0);
    if (r < 0) { r = rows.length; rows.push(0); }
    rows[r] = Math.max(rows[r], a1);
    g += `<line x1="${xx.toFixed(1)}" x2="${xx.toFixed(1)}" y1="${PT}" y2="${H - PB}" class="peak"/><text x="${(right ? xx - 3 : xx + 3).toFixed(1)}" y="${PT + 9 + r * 12}" text-anchor="${right ? "end" : "start"}" class="peak-l">${lbl}</text>`;
  }
  const path = (get) => {
    let d = "", pen = false;
    for (const b of bins) {
      const v = get(b);
      if (!(v > 0)) { pen = false; continue; }
      d += `${pen ? "L" : "M"}${x(b.e0).toFixed(1)} ${y(v).toFixed(1)}L${x(b.e1).toFixed(1)} ${y(v).toFixed(1)}`;
      pen = true;
    }
    return d;
  };
  if (hasBg) g += `<path d="${path((b) => b.bg)}" class="bg"/>`;
  g += `<path d="${path((b) => b.obj)}" class="obj"/>`;
  const bgTxt = !hasBg ? "" : sp.bgSrc === "acumulado" ? `fundo: espectro acumulado do aparelho (${rcDur(sp.bgSec) || fmtMin(sp.bgSec)}), na escala do tempo do objeto` : `fundo (${specDur(sp.bgSec)})`;
  let txt = "";
  const groups = new Map();
  if (!hasBg) txt = "Sem o espectro do fundo: não deu para separar os picos do objeto dos do ambiente.";
  else if (!found.length) txt = "Nenhuma linha gama conhecida subiu acima do fundo (3σ).";
  else {
    txt = "Linhas acima do fundo (uma pista do radionuclídeo, não uma identificação garantida):";
    for (const p of found) { if (!groups.has(p.group)) groups.set(p.group, []); groups.get(p.group).push(p.kev); }
  }
  box.innerHTML = `<h4>Espectro</h4><svg viewBox="0 0 ${W} ${H}" class="m-spec-svg" role="img" aria-label="Espectro do objeto e do fundo">${g}</svg>
    <div class="legend small muted"><span class="key"><i class="mk-obj" aria-hidden="true"></i>com o objeto (${specDur(sp.objSec)})</span>${hasBg ? `<span class="key"><i class="mk-bg" aria-hidden="true"></i>${bgTxt}</span>` : ""}<span>contagens/s em escala log</span></div>
    <p class="small m-spec-txt"></p><ul class="small m-spec-peaks"></ul>`;
  box.querySelector(".m-spec-txt").textContent = txt;
  const ul = box.querySelector(".m-spec-peaks");
  for (const [gname, ks] of groups) {
    const li = document.createElement("li");
    const b = document.createElement("b");
    b.textContent = gname;
    const k = document.createElement("span");
    k.className = "mono";
    k.textContent = ks.sort((a, b2) => a - b2).map(kevTxt).join("; ") + " keV";
    li.append(b, " ", k);
    ul.appendChild(li);
  }
}

function measureExplain(r, obj, auto, k) {
  const t = obj ? fmtMin(obj.seconds) : "";
  k = k || 1;
  if (r.stop === "achou") return `Parou em ${t}: a diferença passou de 4σ, bem acima da variação natural da contagem. O µSv/h vale para o objeto na distância medida; a 1 metro é muito menos.`;
  if (r.stop === "nada") return `Parou em ${t}, quando deu para garantir que o objeto sobe menos de ${fmt(r.pct, 0)}% do fundo (+${uS(r.threshold, k)}). Para procurar algo mais fraco, escolha “+10% do fundo”.`;
  if (r.stop === "limite") return `Chegou ao tempo máximo (${t}) sem decidir.` + (r.level === "sim" ? "" : ` Se o objeto emite algo, é menos que +${uS(r.limit, k)}; para ir além, aumente o tempo máximo.`);
  return {
    sim: "A diferença é bem maior que a variação natural da contagem. O valor em µSv/h vale para o objeto encostado no contador; a 1 metro é muito menos.",
    talvez: "A diferença está perto do limite da variação natural. Use o modo automático para medir até ter certeza.",
    nada: "Se o objeto emitir alguma coisa, está abaixo do que dá para ver nesse tempo" + (r.limit ? ` (menos que +${uS(r.limit, k)})` : "") + ". Use o modo automático para mais certeza.",
  }[r.level];
}

function wireMeasure() {
  for (const id of ["mSecs", "mBgSrc", "mPct", "mDev", "mObjDev"]) {
    const v = localGet("m-" + id);
    if (v && [...$(id).options].some((o) => o.value === v)) $(id).value = v;
    $(id).addEventListener("change", () => { localSet("m-" + id, $(id).value); if (id === "mDev" || id === "mObjDev") refreshHomeBg(true); renderMeasure(state.measureSt || {}); });
  }
  $("btnMeasureBg").addEventListener("click", async () => {
    try {
      const st = state.measureSt || {};
      // fundo de novo no meio de uma medição: recomeça com as mesmas opções
      renderMeasure(await api.StartMeasure({ ...measureOpts(), homeBg: false }));
      if (st.bg) toast("Medindo o fundo de novo.");
    } catch (e) { toast(errMsg(e), true); }
  });
  $("btnMeasureObj").addEventListener("click", async () => {
    try {
      const st = state.measureSt || {};
      if (st.bg) renderMeasure(await api.StartMeasurePhase("objeto", 0, $("mName").value));
      else renderMeasure(await api.StartMeasure(measureOpts()));
    } catch (e) { toast(errMsg(e), true); }
  });
  $("btnMeasureReset").addEventListener("click", async () => {
    renderMeasure(await api.CancelMeasure());
    refreshHomeBg(true);
  });
}

// ---------- medições salvas ----------

const measureAt = (ms) => {
  for (const m of state.measurements || []) {
    if (ms >= m.obj.start - 30e3 && ms <= m.obj.end + 30e3) return [m, "com o objeto"];
    if (m.bg.source !== "casa" && ms >= m.bg.start - 30e3 && ms <= m.bg.end + 30e3) return [m, "fundo"];
  }
  return null;
};
const measureLabel = (ms) => { const x = measureAt(ms); return x ? ` · medição: ${x[0].name} (${x[1]})` : ""; };

// faixas da medição: cinza no fundo, verde-água com o objeto
function measureBands(t0, t1, xt) {
  let out = "";
  for (const m of state.measurements || []) {
    for (const [ph, color, op] of [[m.bg, COR.muted, 0.14], [m.obj, COR.obj, 0.24]]) {
      if (ph.source === "casa" || !(ph.end > t0 && ph.start < t1)) continue;
      const x1 = xt(Math.max(ph.start, t0)), x2 = xt(Math.min(ph.end, t1));
      out += `<rect x="${x1.toFixed(1)}" y="0" width="${Math.max(2, x2 - x1).toFixed(1)}" height="${H}" fill="${color}" fill-opacity="${op}"/>`;
    }
    if (m.obj.start > t0 && m.obj.start < t1) {
      const x = xt(m.obj.start).toFixed(1);
      out += `<line x1="${x}" y1="0" x2="${x}" y2="${H}" stroke="${COR.obj}" stroke-width="1" stroke-dasharray="3 3" vector-effect="non-scaling-stroke"/>`;
    }
  }
  return out;
}

const verdictWord = { sim: "radioativo", talvez: "talvez", nada: "sem diferença" };

// ---------- medições salvas: lista à esquerda, detalhes à direita ----------

function setMeasureMode(mode) {
  state.mMode = mode === "salvas" ? "salvas" : "nova";
  localSet("mMode", state.mMode);
  const saved = state.mMode === "salvas";
  document.querySelectorAll("#mModeSeg button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.mmode === state.mMode)));
  $("mNewPane").hidden = saved;
  $("mSavedPane").hidden = !saved;
  $(saved ? "mLiveSlotSaved" : "mLiveSlotNew").prepend($("mLive"));
  if (saved) {
    const list = state.measurements || [];
    if (!list.some((m) => m.id === state.mSel)) state.mSel = list.length ? list[list.length - 1].id : null;
  } else {
    state.mSel = null;
  }
  renderMeasureHistory();
  drawMeasureChart();
}

function renderMeasureHistory() {
  const ul = $("mHistList");
  const all = [...(state.measurements || [])].reverse();
  $("mHistCount").textContent = all.length ? `(${all.length})` : "";
  const q = ($("mSearch").value || "").trim().toLowerCase();
  const list = q ? all.filter((m) => (m.name + " " + (m.note || "")).toLowerCase().includes(q)) : all;
  ul.textContent = "";
  if (!all.length) ul.innerHTML = '<li class="muted small">Nenhuma medição salva ainda.</li>';
  else if (!list.length) ul.innerHTML = '<li class="muted small">Nenhuma medição com esse nome.</li>';
  for (const m of list) {
    const li = document.createElement("li");
    const b = document.createElement("button");
    b.type = "button";
    b.className = "m-item" + (state.mSel === m.id ? " sel" : "");
    b.dataset.id = m.id;
    b.innerHTML = '<span class="mh-dot"></span><span class="m-item-main"><strong></strong><span class="mono small muted"></span></span><span class="mono small m-item-net"></span>';
    b.querySelector(".mh-dot").classList.add(m.result.level);
    b.querySelector("strong").textContent = m.name;
    b.querySelector(".m-item-main .muted").textContent = `${whenLabel(m.time)} · ${fmtDur(m.obj.end - m.obj.start)}` + ((m.attachments || []).length ? ` · ${m.attachments.length} anexo${m.attachments.length > 1 ? "s" : ""}` : "");
    if (m.notComparable) { const tg = document.createElement("span"); tg.className = "nc-tag"; tg.textContent = "não comparável"; b.querySelector("strong").appendChild(tg); }
    const net = b.querySelector(".m-item-net");
    net.textContent = `${m.result.netUSv >= 0 ? "+" : "−"}${numU(Math.abs(m.result.netUSv))}`;
    net.title = "µSv/h acima do fundo";
    b.addEventListener("click", () => { state.mSel = m.id; renderMeasureHistory(); drawMeasureChart(); });
    li.appendChild(b);
    ul.appendChild(li);
  }
  renderMeasureDetail();
}

// limite de detecção (Currie) e tempo para uma precisão, como no PDF
// Currie com tempos diferentes (igual ao measurereport.go)
const mDetLimit = (bg, objSec) => {
  if (!(bg.seconds > 0)) return 0;
  const tb = bg.seconds / 60, to = (objSec || bg.seconds) / 60, rb = bg.counts / tb;
  return 2.71 / to + 3.29 * Math.sqrt(rb / to + rb / tb);
};
// fundo fixo (da casa): segundos com o objeto para o líquido ter precisão rel
const mObjSecsFor = (bg, obj, rel) => {
  const net = obj.cpm - bg.cpm, q = (rel * net) ** 2 - bg.errCPM ** 2;
  return net > 0 && q > 0 ? (obj.cpm / q) * 60 : Infinity;
};
const mSecsFor = (bg, obj, rel) => {
  const rb = bg.cpm / 60, ro = obj.cpm / 60, net = ro - rb;
  return net > 0 ? (rb + ro) / (rel * rel * net * net) : Infinity;
};
const mDurTxt = (s) => (!isFinite(s) || s > 30 * 864e2 ? "—" : s < 90 ? `${Math.ceil(s)} s` : s < 5400 ? `${Math.ceil(s / 60)} min` : `${fmt(s / 3600, 1)} h`);

function renderMeasureDetail() {
  const m = (state.measurements || []).find((x) => x.id === state.mSel);
  $("mDetailEmpty").hidden = !!m;
  $("mDetailBody").hidden = !m;
  if (!m) return;
  const r = m.result;
  $("mdName").textContent = m.name;
  const home = m.bg.source === "casa";
  const rc = isRcMeasure(m);
  $("mdWhen").textContent = `${whenLabel(m.time)} · ` + (home ? `fundo da casa (${Math.round(m.bg.seconds / 60)} min) + objeto ${fmtMin(m.obj.seconds)}` : `fundo ${m.bg.seconds} s + objeto ${m.obj.seconds} s`) + (m.distance ? ` · ${m.distance}` : "") +
    (m.mode === "ambos" ? " · os dois aparelhos" : m.mode === "voce" ? ` · ${rc ? "RadiaCode" : "GMC-800"} no objeto, ${rc ? "GMC-800" : "RadiaCode"} com você` : rc ? " · RadiaCode" : "");
  const v = $("mdVerdict");
  v.className = "m-verdict " + r.level;
  v.querySelector("strong").textContent = r.verdict.charAt(0).toUpperCase() + r.verdict.slice(1) + ".";
  v.querySelector("span").textContent = `${fmt(r.sigma, 1)}σ: até ~2σ é a variação natural da contagem; de 3σ para cima a diferença é real.`;
  const t = $("mdTiles");
  t.textContent = "";
  const dv = rc ? "radiacode" : "gmc", bk = phK(m.bg, dv), ok = phK(m.obj, dv);
  sumTile(t, home ? "Fundo da casa" : "Fundo (objeto longe)", uS(m.bg.cpm, bk), `± ${uS(m.bg.errCPM, bk)} · ${m.bg.counts.toLocaleString(LOCALE)} contagens`);
  sumTile(t, "Com o objeto", uS(m.obj.cpm, ok), `± ${uS(m.obj.errCPM, ok)} · ${m.obj.counts.toLocaleString(LOCALE)} contagens`);
  sumTile(t, "Líquido", sgnUS(r.netUSv), `± ${uS(r.netErr, ok)} ` + (rc ? "no detector" : "no tubo"), r.level === "sim" ? "warn" : "");
  sumTile(t, "Objeto ÷ fundo", `${fmt(r.ratio, 2)}×`, `${fmt(r.sigma, 1)}σ acima do acaso`);
  $("mdStats").replaceChildren(measureStats(m));
  renderMeasSpec($("mdSpec"), m.spectrum);
  renderSideResult($("mdOther"), m);
  $("mdNote").textContent = m.note || "Não anotada. Use Editar para guardar o lado encostado no tubo, a embalagem etc.";
  $("mdNote").classList.toggle("muted", !m.note);
  let lim = `Limite de detecção: ${uS(mDetLimit(m.bg, m.obj.seconds), ok)} acima do fundo.`;
  if (r.limit) lim += ` Se o objeto emite algo, é menos que +${uS(r.limit, ok)}.`;
  if (r.netCPM > 0) lim += home
    ? ` Precisão do líquido: ±${fmt((r.netErr / r.netCPM) * 100, 0)}%. Para ±10%: ${mDurTxt(mObjSecsFor(m.bg, m.obj, 0.1))} com o objeto; para ±5%: ${mDurTxt(mObjSecsFor(m.bg, m.obj, 0.05))}.`
    : ` Precisão do líquido: ±${fmt((r.netErr / r.netCPM) * 100, 0)}%. Para ±10%: ${mDurTxt(mSecsFor(m.bg, m.obj, 0.1))} em cada fase; para ±5%: ${mDurTxt(mSecsFor(m.bg, m.obj, 0.05))}.`;
  if (r.stop) lim = measureExplain(r, m.obj, true, ok) + " " + lim;
  $("mdLimits").textContent = lim;
  $("mdEditName").value = m.name;
  $("mdEditNote").value = m.note || "";
  const del = $("btnMdDel");
  del.dataset.sure = "";
  del.textContent = "Apagar";
  renderMeasureExtra(m);
}

// 2.7: comparabilidade, configuração da época, outras medições e anexos
let mCtxSeq = 0;
async function renderMeasureExtra(m) {
  $("mdComp").textContent = m.notComparable
    ? `Marcada como não comparável${m.ncReason ? ": " + m.ncReason : ""}. Ela continua salva, mas não deve ser comparada com as outras medições deste objeto.`
    : "Comparável com as outras medições do mesmo objeto (mesmo nome), se a distância e a configuração forem as mesmas.";
  $("btnMdComp").textContent = m.notComparable ? "Desmarcar" : "Marcar como não comparável";
  $("mdCompForm").hidden = true;
  renderAttachments(m);
  const my = ++mCtxSeq;
  let ctx = null;
  if (api.GetMeasurementContext) { try { ctx = await api.GetMeasurementContext(m.id); } catch (e) { ctx = null; } }
  if (my !== mCtxSeq) return;
  const cfg = $("mdCfg");
  if (isRcMeasure(m)) cfg.textContent = "Medição do RadiaCode: o µSv/h vem da calibração do próprio aparelho.";
  else if (!ctx || !ctx.cfgKnown) cfg.textContent = "Sem registro: o histórico de configurações começa na versão 2.1.";
  else {
    cfg.textContent = "";
    for (const c of ctx.cfg.filter((x) => x.then)) { const sp = document.createElement("span"); sp.textContent = `${c.field}: ${c.then}`; cfg.append(sp, document.createElement("br")); }
    const tail = document.createElement("span");
    tail.textContent = ctx.changed.length ? `Mudou desde então: ${ctx.changed.join(", ")}.` : "Igual à de hoje.";
    tail.className = ctx.changed.length ? "" : "muted";
    cfg.appendChild(tail);
    if (ctx.changed.length) { const t2 = document.createElement("span"); t2.textContent = " O resultado guardado usa a configuração da época."; cfg.appendChild(t2); }
  }
  const rel = $("mdRelated");
  rel.textContent = "";
  const list = (ctx && ctx.related) || [];
  if (!list.length) { rel.innerHTML = '<span class="muted">Nenhuma outra medição com este nome. Dê o mesmo nome para medir o mesmo objeto de novo e acompanhar.</span>'; return; }
  const tb = document.createElement("table");
  tb.className = "m-rel";
  const row = (id, when, net, dist, status, cls) => {
    const tr = document.createElement("tr");
    tr.className = cls;
    const c = [when, net, dist, status].map((x) => { const td = document.createElement("td"); td.textContent = x; return td; });
    if (id) { const b = document.createElement("button"); b.type = "button"; b.textContent = when; b.addEventListener("click", () => { state.mSel = id; renderMeasureHistory(); drawMeasureChart(); }); c[0].textContent = ""; c[0].appendChild(b); }
    tr.append(...c);
    tb.appendChild(tr);
  };
  const netTxt = (v, e) => `${v >= 0 ? "+" : "−"}${numU(Math.abs(v))}` + (e ? ` ± ${numU(Math.abs(e))}` : "");
  row(0, `${ddmm(m.time)} (esta)`, netTxt(m.result.netUSv, m.result.netCPM ? (m.result.netErr * m.result.netUSv) / m.result.netCPM : 0), m.distance || "—", m.notComparable ? "não comparável" : "", "cur");
  for (const r of list) row(r.id, ddmm(r.time), netTxt(r.netUSv, r.netErrUSv), r.distance || "—", r.diff.length ? r.diff.join(", ") : "comparável", r.diff.length ? "nc" : "");
  rel.appendChild(tb);
}

async function renderAttachments(m) {
  const box = $("mdAttach");
  box.textContent = "";
  const list = m.attachments || [];
  if (!list.length) { box.innerHTML = '<span class="small muted">Nenhum anexo. Fotos do objeto e da posição no tubo ajudam a repetir a medição do mesmo jeito.</span>'; return; }
  for (const at of list) {
    const card = document.createElement("div");
    card.className = "m-att";
    card.innerHTML = '<div class="thumb"></div><div class="meta"><span class="nm"></span><span class="muted"></span><span class="acts"><button type="button" data-act="open">Abrir</button><button type="button" data-act="rm">Remover</button></span></div>';
    card.querySelector(".nm").textContent = at.name;
    card.querySelector(".nm").title = at.name;
    card.querySelector(".meta .muted").textContent = `${fmt(at.size / 1024, at.size < 10240 ? 1 : 0)} KB`;
    const th = card.querySelector(".thumb");
    const ext = (at.name.split(".").pop() || "").toUpperCase().slice(0, 4);
    th.innerHTML = `<span>${escHtml(ext || "?")}</span>`;
    if ((at.type || "").startsWith("image/") && at.size < 12 * 1048576 && api.GetMeasurementAttachment) {
      api.GetMeasurementAttachment(m.id, at.file).then((u) => { const img = document.createElement("img"); img.src = u; img.alt = at.name; th.replaceChildren(img); }).catch(() => {});
    }
    const open = () => call(api.OpenMeasurementAttachment, m.id, at.file);
    th.addEventListener("click", open);
    card.querySelector('[data-act="open"]').addEventListener("click", open);
    const rm = card.querySelector('[data-act="rm"]');
    rm.addEventListener("click", async () => {
      if (rm.dataset.sure !== "1") { rm.dataset.sure = "1"; rm.textContent = "Confirmar"; return; }
      await call(api.RemoveMeasurementAttachment, m.id, at.file);
    });
    box.appendChild(card);
  }
}

function wireMeasureSaved() {
  document.querySelectorAll("#mModeSeg button").forEach((b) => b.addEventListener("click", () => setMeasureMode(b.dataset.mmode)));
  $("mSearch").addEventListener("input", renderMeasureHistory);
  const cur = () => (state.measurements || []).find((x) => x.id === state.mSel);
  $("btnMOpenSaved").addEventListener("click", () => { const id = state.mLastSaved; setMeasureMode("salvas"); if (id) { state.mSel = id; renderMeasureHistory(); drawMeasureChart(); } });
  $("btnMdReport").addEventListener("click", (e) => { const m = cur(); if (m) exportMeasureReport(m.id, e.currentTarget); });
  $("btnMdChart").addEventListener("click", () => { const m = cur(); if (m) showRange((m.bg.source === "casa" ? m.obj.start : m.bg.start) - 120e3, m.obj.end + 120e3); });
  $("btnMdEdit").addEventListener("click", () => { const f = $("mdEditForm"); f.hidden = !f.hidden; if (!f.hidden) $("mdEditName").focus(); });
  $("btnMdSave").addEventListener("click", async () => {
    const m = cur();
    if (!m) return;
    await call(api.UpdateMeasurement, m.id, $("mdEditName").value, $("mdEditNote").value);
    $("mdEditForm").hidden = true;
  });
  $("btnMdComp").addEventListener("click", async () => {
    const m = cur();
    if (!m) return;
    if (m.notComparable) { await call(api.SetMeasurementComparable, m.id, true, ""); return; }
    const f = $("mdCompForm"); f.hidden = !f.hidden; if (!f.hidden) $("mdCompReason").focus();
  });
  $("btnMdCompSave").addEventListener("click", async () => {
    const m = cur();
    if (!m) return;
    await call(api.SetMeasurementComparable, m.id, false, $("mdCompReason").value);
    $("mdCompReason").value = "";
  });
  $("btnMdAttach").addEventListener("click", async () => {
    const m = cur();
    if (!m || !api.AddMeasurementAttachment) return;
    try { await call(api.AddMeasurementAttachment, m.id); } catch (e) { /* erro já mostrado */ }
  });
  $("btnMdDel").addEventListener("click", async () => {
    const m = cur(), del = $("btnMdDel");
    if (!m) return;
    if (del.dataset.sure !== "1") { del.dataset.sure = "1"; del.textContent = "Confirmar"; return; }
    state.mSel = null;
    await call(api.DeleteMeasurement, m.id);
  });
}

async function refreshMeasurements() {
  try { state.measurements = (await api.GetMeasurements()) || []; } catch (e) { state.measurements = []; }
  renderMeasureHistory();
}
