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
  if (state.mBusca && !!r.demo === !!state.demo.active) buscaFeed("gmc", (r.cps || 0) * 60 * calK());
  const st = state.measureSt;
  if (!st || (st.phase !== "fundo" && st.phase !== "objeto")) return;
  // GMC como segundo aparelho: vai para a linha dele (µSv/h)
  if (st.other && st.other.device === "gmc" && !r.demo) { mlog.other.push({ t: Date.now(), usv: r.usv, phase: st.phase }); return; }
  if (!!r.demo !== !!st.demo || isRcMeasure(st)) return;
  mlog.live.push({ t: Date.now(), cpm: r.cpm });
}

// dose acumulada do próprio Radiacode (desde que foi zerado), no cartão de
// dose: anda ao vivo com ele conectado
// dose do próprio Radiacode (desde que foi zerado) e a conta do app, em duas
// linhas curtas dentro do quadro do Radiacode
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

// alertas do Radiacode: a faixa de alarme/atenção e o som seguem o nível que
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

// Radiacode medindo: a leitura ao vivo é o CPS do aparelho × 60
function measureLogRc(rs) {
  if (state.mBusca && rs && rs.state === "conectado") buscaFeed("radiacode", rs.usvh);
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
// µSv/h. k = µSv/h por CPM: o Radiacode tem a dose dele (k da fase), o GMC
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

// ---------- medir objeto em etapas (2.40) ----------
// Preparar → procurar o ponto mais forte → fundo → objeto → resultado. As
// escolhas ficam lembradas entre as vezes (mesmas chaves de antes da 2.40).

const M_DEVS = ["radiacode", "gmc", "ambos", "voce"];
const mOpt = (key, def, ok) => { const v = localGet("m-" + key); return v != null && (!ok || ok(v)) ? v : def; };
const mSetOpt = (key, v) => localSet("m-" + key, String(v));

function measureOpts() {
  const t = $("mSecs").value, auto = t.startsWith("auto");
  const cm = parseFloat(String($("mDistCm").value).replace(",", "."));
  const has = isFinite(cm) && cm >= 0;
  return {
    name: $("mName").value, distance: !has ? "" : cm === 0 ? "encostado" : `${fmt(cm, cm % 1 ? 1 : 0)} cm`, distCm: has ? cm : 0,
    size: mOpt("size", ""), auto, homeBg: mOpt("mBgSrc", "casa") === "casa", ...measureDevOpts(),
    seconds: auto ? 0 : parseInt(t, 10), maxSec: auto ? parseInt(t.slice(4), 10) : 0, pct: parseFloat(mOpt("mPct", "20")),
  };
}

// aparelho no objeto e modo (os dois perto, ou um com você)
function measureDevOpts() {
  const v = mOpt("mDev", "", (x) => M_DEVS.includes(x)) || (rcOnline() ? "radiacode" : "gmc");
  if (v === "ambos") return { device: "radiacode", mode: "ambos" };
  if (v === "voce") return { device: mOpt("mObjDev", "gmc", (x) => x === "gmc" || x === "radiacode"), mode: "voce" };
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

const rcOnline = () => !!(state.rc.st && state.rc.st.state === "conectado");
const gmcOnline = () => state.status.state === "conectado" || !!state.demo.active;
const isRcMeasure = (x) => (x && x.device) === "radiacode";
const fmtMin = (sec) => (sec < 90 ? `${Math.max(1, Math.round(sec))} s` : `${Math.round(sec / 60)} min`);
const mClock = (s) => (s < 60 ? `${s} s` : `${Math.floor(s / 60)} min ${s % 60} s`);
const cmTxt = (cm) => `${fmt(cm, cm < 10 && cm % 1 ? 1 : 0)} cm`;

// etapa da tela: vem da fase do Go; antes de começar, da escolha local
function measureStep(st) {
  if (st.phase === "fundo" || st.phase === "fundo-pronto") return "fundo";
  if (st.phase === "objeto") return "objeto";
  if (st.phase === "pronto") return "result";
  return state.mBusca ? "busca" : "prep";
}

function renderMeasure(st) {
  st = st || {};
  state.measureSt = st;
  measureLogStep(st);
  drawMeasureChart();
  const step = measureStep(st);
  if (step !== "busca" && state.mBusca && st.phase) state.mBusca = false;
  $("measureTitle").textContent = "Medir objeto" + (st.demo ? " (demo)" : "");
  for (const [id, s] of [["mPanePrep", "prep"], ["mPaneBusca", "busca"], ["mPaneFundo", "fundo"], ["mPaneObj", "objeto"], ["mPaneResult", "result"]]) $(id).hidden = step !== s;
  renderMeasureSteps(st, step);
  if (step === "prep") renderMeasurePrep();
  else if (step === "busca") renderMeasureBusca();
  else if (step === "fundo") renderMeasureFundo(st);
  else if (step === "objeto") renderMeasureObj(st);
  else renderMeasureResult(st);
  mLivePoll(st);
}

function renderMeasureSteps(st, step) {
  const order = ["prep", "busca", "fundo", "objeto", "result"], at = order.indexOf(step);
  const home = !!(st.bg && st.bg.source === "casa");
  document.querySelectorAll("#mSteps li").forEach((li, i) => {
    const s = li.dataset.step;
    const skip = (s === "fundo" && home && at > 2) || (s === "busca" && state.mBuscaSkip && at > 1);
    li.className = (i === at ? "cur" : i < at ? "done" : "") + (skip ? " skip" : "");
    li.querySelector(".m-step-n").textContent = i < at && !skip ? "✓" : skip ? "–" : String(i + 1);
    if (i === at) li.setAttribute("aria-current", "step"); else li.removeAttribute("aria-current");
  });
}

// ---------- 1. preparar ----------

function renderMeasurePrep() {
  const o = measureOpts(), dv = mOpt("mDev", "", (x) => M_DEVS.includes(x)) || (rcOnline() ? "radiacode" : "gmc");
  document.querySelectorAll("#mDevs .m-dev").forEach((b) => b.setAttribute("aria-checked", String(b.dataset.dev === dv)));
  $("mObjDevBox").hidden = dv !== "voce";
  document.querySelectorAll("#mObjDevSeg button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.objdev === o.device)));
  $("mObjDevHint").textContent = dv !== "voce" ? "" : o.device === "radiacode" ? "Com o RC102 no objeto, você ganha o espectro e os isótopos." : "Com o RC102 com você, você vê o espectro do que chega até você.";
  const cm = $("mDistCm").value;
  document.querySelectorAll("#mDistSeg button").forEach((b) => b.setAttribute("aria-pressed", String(cm !== "" && +b.dataset.dist === +cm)));
  document.querySelectorAll("#mSizeSeg button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.size === o.size)));
  document.querySelectorAll("#mBgSeg button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.bg === (o.homeBg ? "casa" : "medir"))));
  document.querySelectorAll("#mPctSeg button").forEach((b) => b.setAttribute("aria-pressed", String(+b.dataset.pct === o.pct)));
  $("mPctBox").hidden = !o.auto;
  refreshHomeBg(false);
  const hb = state.homeBg, rc = o.device === "radiacode";
  $("mHomeBgLine").textContent = !hb ? "" : hb.ok
    ? `Fundo da casa: ${usvVal(hb.usv)} (${hb.minutes} min até ${hhmm(hb.to)}). ` + (rc ? "Só vale se o RC102 passou a última hora parado no lugar onde vai medir." : "Meça no lugar de sempre do contador.")
    : `Fundo da casa indisponível: ${hb.why}. O app mede o fundo na hora.`;
  const warn = [
    (rc || o.mode) && !rcOnline() ? "O RC102 não está conectado." : "",
    (!rc || o.mode) && !gmcOnline() ? "O GMC-800 não está conectado." : "",
    o.mode === "voce" ? `Deixe o ${devLabel(o.device)} no objeto e o ${devLabel(o.device === "radiacode" ? "gmc" : "radiacode")} com você (no bolso ou na mão).` : "",
  ].filter(Boolean).join(" ");
  $("mOptHint").textContent = warn;
  $("mOptHint").hidden = !warn;
}

// ---------- 2. procurar o ponto mais forte ----------
// Média curta (5 s) do aparelho que fica no objeto, a seta de subindo ou
// descendo e o pico. Nada disso vai para o Go: é só para posicionar.

const mBusca = { dev: "", buf: [], vals: [], peak: 0, prev: null, t0: 0 };
function buscaDev() { const o = measureDevOpts(); return o.mode === "ambos" ? "radiacode" : o.device; }
function buscaReset() { Object.assign(mBusca, { dev: buscaDev(), buf: [], vals: [], peak: 0, prev: null, t0: Date.now() }); }
function buscaFeed(dev, usv) {
  if (!state.mBusca || dev !== mBusca.dev || !(usv >= 0)) return;
  const now = Date.now();
  mBusca.buf.push({ t: now, v: usv });
  mBusca.buf = mBusca.buf.filter((x) => now - x.t <= 5000);
  const v = mBusca.buf.reduce((a, x) => a + x.v, 0) / mBusca.buf.length;
  mBusca.vals.push({ t: now, v });
  mBusca.vals = mBusca.vals.filter((x) => now - x.t <= 60e3);
  if (mBusca.buf.length >= 2) mBusca.peak = Math.max(mBusca.peak, v);
  if (state.view === "medir") renderMeasureBusca();
}
function renderMeasureBusca() {
  const dev = mBusca.dev || buscaDev(), on = dev === "radiacode" ? rcOnline() : gmcOnline();
  const last = mBusca.vals[mBusca.vals.length - 1];
  $("mBuscaVal").textContent = last ? numU(last.v) : "—";
  const old = mBusca.vals.find((x) => last && last.t - x.t <= 3000);
  let trend = !on ? `${devLabel(dev)} desconectado` : !last ? "esperando a leitura" : "estável", cls = "";
  if (last && old && last !== old) {
    const d = last.v - old.v, rel = Math.abs(d) / Math.max(last.v, 1e-6);
    if (rel > 0.06) { trend = d > 0 ? "▲ subindo" : "▼ descendo"; cls = d > 0 ? "up" : "down"; }
  }
  const tEl = $("mBuscaTrend");
  tEl.textContent = trend;
  tEl.className = "m-busca-trend " + cls;
  const hb = state.homeBg && state.homeBg.device === dev && state.homeBg.ok ? state.homeBg.usv : 0;
  const top = Math.max(mBusca.peak, last ? last.v : 0, hb * 2, 1e-3) * 1.15;
  const pos = (v) => Math.max(0, Math.min(100, (v / top) * 100)) + "%";
  $("mBuscaFill").style.width = last ? pos(last.v) : "0%";
  $("mBuscaBgMk").style.left = pos(hb);
  $("mBuscaBgMk").hidden = !hb;
  $("mBuscaPkMk").style.left = pos(mBusca.peak);
  $("mBuscaPkMk").hidden = !mBusca.peak;
  $("mBuscaBgTxt").textContent = hb ? `fundo ${numU(hb)}` : "fundo";
  $("mBuscaPkTxt").textContent = mBusca.peak ? `pico ${usvVal(mBusca.peak)}` : "pico —";
  const pts = mBusca.vals, sp = $("mBuscaSpark").querySelector("polyline");
  if (pts.length > 1) {
    const now = Date.now(), mx = Math.max(...pts.map((p) => p.v), 1e-6) * 1.1;
    sp.setAttribute("points", pts.map((p) => `${(240 - ((now - p.t) / 60e3) * 240).toFixed(1)},${(78 - (p.v / mx) * 74).toFixed(1)}`).join(" "));
  } else sp.setAttribute("points", "");
  $("mBuscaDev").textContent = `Lendo o ${devLabel(dev)} (média dos últimos 5 s). Vale também para achar o lado mais forte de um objeto grande.`;
}

// começa a medição de verdade (depois de procurar ou pular)
async function measureBegin(homeBg) {
  const o = measureOpts();
  if (homeBg != null) o.homeBg = homeBg;
  // fundo da casa escolhido, mas indisponível agora: mede na hora
  else if (o.homeBg && state.homeBg && state.homeBg.device === o.device && !state.homeBg.ok) o.homeBg = false;
  try {
    state.mBusca = false;
    renderMeasure(await api.StartMeasure(o));
  } catch (e) {
    toast(errMsg(e), true);
    state.mBusca = false;
    renderMeasure(state.measureSt || {});
  }
}

// ---------- 3. fundo ----------

function renderMeasureFundo(st) {
  const rc = isRcMeasure(st), k = mK(st), done = st.phase === "fundo-pronto";
  const hb = state.homeBg;
  $("mFundoTitle").textContent = done ? `Fundo pronto. Agora o objeto na frente do aparelho${st.distance ? ", " + st.distance : ""}` : "Afaste o objeto pelo menos 2 metros";
  $("mFundoText").textContent = done ? "Coloque o objeto no ponto mais forte e comece. O fundo medido agora é descontado no resultado."
    : "O app mede o fundo do lugar para descontar depois." + (rc || (st.other && st.other.device === "radiacode") ? " Com o RC102, guarda também o espectro do fundo." : "");
  const el = done ? (st.bg ? st.bg.seconds : 0) : st.elapsed || 0, total = done ? el : st.seconds || 60;
  $("mBgFill").style.width = Math.min(100, (el / Math.max(1, total)) * 100) + "%";
  $("mBgElapsed").textContent = mClock(el);
  $("mBgTotal").textContent = fmtMin(total);
  const ph = done ? st.bg : st.elapsed > 0 ? { cpm: (st.counts * 60) / st.elapsed, errCPM: (Math.sqrt(Math.max(st.counts, 1)) * 60) / st.elapsed } : null;
  $("mBgValK").textContent = done ? "fundo medido" : "fundo até agora";
  $("mBgVal").textContent = ph ? `${numU(ph.cpm * (done ? phK(st.bg, rc ? "radiacode" : "gmc") : k))} ± ${uS(ph.errCPM, done ? phK(st.bg, rc ? "radiacode" : "gmc") : k)}` : "—";
  const o = st.other;
  $("mBgOtherRow").hidden = !(o && o.role === "objeto");
  if (o && o.role === "objeto") {
    $("mBgOtherK").textContent = `fundo do ${devLabel(o.device)}`;
    const ok = phK(o.bg, o.device);
    $("mBgOtherV").textContent = o.bg ? `${numU(o.bg.cpm * ok)} ± ${uS(o.bg.errCPM, ok)}` : o.elapsed > 0 ? `${uS((o.counts * 60) / o.elapsed, ok)}` : "—";
  }
  $("btnMObjGo").hidden = !done;
  $("btnMUseHome").hidden = done || !(hb && hb.ok && hb.device === (rc ? "radiacode" : "gmc"));
  $("btnMBgCancel").textContent = done ? "Recomeçar" : "Cancelar";
  $("mFundoHint").textContent = !done && st.auto ? "No automático, o fundo é medido por 5 min. Com o fundo da casa, pula esta etapa." : "";
}

// ---------- 4. objeto ----------

function measKV(box, rows) {
  box.textContent = "";
  for (const [k, v, cls] of rows) {
    const d = document.createElement("div");
    d.className = "m-kv" + (cls ? " " + cls : "");
    d.innerHTML = '<span></span><span class="mono"></span>';
    d.children[0].textContent = k;
    d.children[1].textContent = v;
    box.appendChild(d);
  }
}

function renderMeasureObj(st) {
  const rc = isRcMeasure(st), k = mK(st), o = st.other;
  $("mObjTitle").textContent = `Objeto na frente do aparelho${st.distance ? ", " + st.distance : ""}`;
  const lv = st.live, okk = st.elapsed > 0 ? k : k;
  const objUSv = st.elapsed > 0 ? ((st.counts * 60) / st.elapsed) * okk : null;
  const objErr = st.elapsed > 0 ? ((Math.sqrt(Math.max(st.counts, 1)) * 60) / st.elapsed) * okk : null;
  const bk = phK(st.bg, rc ? "radiacode" : "gmc");
  const rows = [];
  const pName = devLabel(rc ? "radiacode" : "gmc");
  if (o) rows.push([pName + (o.role === "voce" ? " · no objeto" : ""), "", "m-kv-head"]);
  rows.push(["com o objeto", objUSv != null ? `${numU(objUSv)} ± ${usvVal(objErr)}` : "—"]);
  rows.push(["fundo", st.bg ? usvVal(st.bg.cpm * bk) : "—"]);
  rows.push(["diferença líquida", lv ? `${(lv.netUSv >= 0 ? "+" : "−") + numU(Math.abs(lv.netUSv))} ± ${uS(lv.netErr, k)}` : "—", "m-kv-net"]);
  measKV($("mObjMain"), rows);
  const sig = lv ? lv.sigma : 0;
  $("mSigFill").style.width = Math.max(0, Math.min(100, (sig / 5) * 100)) + "%";
  $("mSigFill").className = "m-sig-fill" + (sig >= 3 ? " real" : "");
  $("mObjTime").textContent = mClock(st.elapsed || 0) + (st.auto && st.eta > 0 ? ` · ~${fmtMin(st.eta)} para decidir` : "");
  // segundo aparelho
  const orows = [];
  if (o && o.role === "voce") {
    orows.push(["com você · " + devLabel(o.device), "", "m-kv-head"]);
    orows.push(["taxa agora", o.nowUSv ? usvVal(o.nowUSv) : "—"]);
    orows.push(["sua dose nesta medição", `${fmt(o.dose || 0, 4)} µSv`]);
    orows.push(["extra por estar perto", o.bgUSv ? `${(o.excess || 0) >= 0 ? "+" : "−"}${fmt(Math.abs(o.excess || 0), 4)} µSv` : "—"]);
  } else if (o) {
    const ok = phK(o.bg, o.device);
    orows.push([devLabel(o.device), "", "m-kv-head"]);
    orows.push(["com o objeto", o.elapsed > 0 ? uS((o.counts * 60) / o.elapsed, ok) : "—"]);
    orows.push(["fundo", o.bg ? uS(o.bg.cpm, ok) : "—"]);
    orows.push(["certeza", o.live ? `${fmt(o.live.sigma, 1)}σ` : "—"]);
  }
  measKV($("mObjOther"), orows);
  let say = "Para sozinho quando já der pra responder.";
  if (!st.auto) say = `Tempo fixo: ${fmtMin(st.seconds)}.`;
  else if (lv && st.elapsed >= 20) say = sig >= 3 ? "Já dá pra dizer que tem algo. Mais um pouco para afinar o número." : sig >= 2 ? "Talvez um pouco acima. Esperando mais contagens para ter certeza." : "Ainda não dá pra dizer. Esperando mais contagens.";
  else if (st.auto) say = "Juntando contagens…";
  $("mObjSay").textContent = say;
  // gráficos: espectro (RC102), espectrograma e o µSv/h ao vivo
  const rcIn = rc || (o && o.device === "radiacode");
  $("mLiveSpecBox").hidden = !rcIn || !mLive.spec;
  $("mSgBox").hidden = !rcIn || mLive.rows.length < 2;
  $("mLiveSlotNew").hidden = rc && !o; // só o RC102: o espectro conta mais que a linha do tempo
}

// ---------- espectro e espectrograma ao vivo (RC102) ----------

const mLive = { timer: 0, busy: false, at: 0, phase: "", start: 0, spec: null, last: null, rows: [], sgMax: 3000 };
function mLivePoll(st) {
  const run = (st.phase === "fundo" || st.phase === "objeto") && (isRcMeasure(st) || (st.other && st.other.device === "radiacode"));
  if (!run || !api.GetMeasureLive) {
    if (mLive.timer) { clearInterval(mLive.timer); mLive.timer = 0; }
    if (!st.phase) Object.assign(mLive, { spec: null, last: null, rows: [], phase: "", start: 0 });
    return;
  }
  if (mLive.phase !== st.phase || mLive.start !== st.phaseStart) Object.assign(mLive, { phase: st.phase, start: st.phaseStart, spec: null, last: null, rows: [], at: 0 });
  if (!mLive.timer) mLive.timer = setInterval(mLiveTick, 1000);
}
async function mLiveTick() {
  const st = state.measureSt || {};
  if (mLive.busy || state.view !== "medir") return;
  const every = (state.rc.st || {}).via === "bluetooth" ? 8000 : 4000;
  if (Date.now() - mLive.at < every) return;
  mLive.at = Date.now();
  mLive.busy = true;
  let r = null;
  try { r = await api.GetMeasureLive(); } catch (e) { r = null; }
  mLive.busy = false;
  if (!r || !r.spectrum || r.phase !== st.phase || mLive.start !== st.phaseStart) return;
  const sp = r.spectrum;
  mLiveRow(sp);
  mLive.spec = sp;
  if (st.phase === "objeto") {
    const voce = sp.role === "voce";
    renderMeasSpec($("mLiveSpecBox"), sp, { title: voce ? "Espectro · RC102 com você" : "Espectro", bgLabel: "fundo", objLabel: voce ? "com você enquanto mexe no objeto" : "com o objeto" });
    $("mSgTitle").textContent = voce ? "Espectrograma · com você" : "Espectrograma";
    drawMeasSg();
  }
  renderMeasure(st);
}
// uma linha do espectrograma: o que entrou desde a leitura anterior, por faixa de energia
function mLiveRow(sp) {
  const L = mLive.last;
  mLive.last = { obj: sp.obj.slice(), sec: sp.objSec };
  if (!L || L.obj.length !== sp.obj.length || sp.objSec <= L.sec) return;
  const dt = sp.objSec - L.sec, N = 160, en = (ch) => { const c = ch + 0.5; return sp.a[0] + sp.a[1] * c + sp.a[2] * c * c; };
  const emax = Math.min(3000, en(sp.obj.length - 1));
  mLive.sgMax = emax;
  const row = new Float64Array(N);
  for (let i = 0; i < sp.obj.length; i++) {
    const e = en(i);
    if (e < 0 || e >= emax) continue;
    const d = sp.obj[i] - L.obj[i];
    if (d > 0) row[Math.min(N - 1, Math.floor((e / emax) * N))] += d / dt;
  }
  mLive.rows.push(row);
  if (mLive.rows.length > 90) mLive.rows.shift();
}
function drawMeasSg() {
  const cv = $("mSg");
  if (!cv || !mLive.rows.length) return;
  const w = Math.max(200, Math.round(cv.clientWidth || 600)), h = 120;
  if (cv.width !== w) cv.width = w;
  cv.height = h;
  const g = cv.getContext("2d");
  g.clearRect(0, 0, w, h);
  const rows = mLive.rows, N = rows[0].length, rh = h / Math.max(rows.length, 30);
  let mx = 0;
  for (const r of rows) for (const v of r) mx = Math.max(mx, v);
  if (!mx) return;
  const lmx = Math.log10(1 + mx * 10);
  const col = getComputedStyle(document.documentElement).getPropertyValue("--obj").trim() || "#5EEAD4";
  g.fillStyle = col;
  rows.forEach((r, j) => {
    const y = h - (rows.length - j) * rh;
    for (let i = 0; i < N; i++) {
      if (!(r[i] > 0)) continue;
      g.globalAlpha = Math.min(1, 0.12 + (Math.log10(1 + r[i] * 10) / lmx) * 0.88);
      g.fillRect((i / N) * w, y, w / N + 0.5, rh + 0.5);
    }
  });
  g.globalAlpha = 1;
  $("mSgMax").textContent = `${Math.round(mLive.sgMax)} keV`;
}

// ---------- 5. resultado ----------

function measBannerTitle(st, r) {
  const nm = (st.name || "").trim();
  if (r.level === "sim") return nm ? `${nm.charAt(0).toUpperCase() + nm.slice(1)} emite radiação acima do fundo` : "O objeto emite radiação acima do fundo";
  if (r.level === "talvez") return nm ? `${nm.charAt(0).toUpperCase() + nm.slice(1)} talvez emita um pouco acima do fundo` : "O objeto talvez emita um pouco acima do fundo";
  return nm ? `${nm.charAt(0).toUpperCase() + nm.slice(1)}: nenhuma diferença que dê para ver` : "Nenhuma diferença do fundo que dê para ver";
}

function renderMeasureResult(st) {
  const r = st.result;
  if (!r) return;
  const rc = isRcMeasure(st), dv = rc ? "radiacode" : "gmc", ok = phK(st.obj, dv), bk = phK(st.bg, dv), d = st.derived || {};
  if (st.savedId) state.mLastSaved = st.savedId;
  const auto = !!st.auto;
  // faixa de cima
  const ban = $("mBanner");
  ban.className = "m-banner " + r.level;
  ban.querySelector(".m-banner-ico").textContent = r.level === "nada" ? "✓" : "!";
  $("mVerdict").textContent = measBannerTitle(st, r);
  const objU = st.obj.cpm * ok;
  // duas frases em dois pedaços (cada um é traduzido inteiro)
  const bt = $("mBannerText"), s1 = document.createElement("span"), s2 = document.createElement("span");
  s1.textContent = r.level === "nada" ? r.verdict.charAt(0).toUpperCase() + r.verdict.slice(1) + "." : `Ficou ${fmt(r.ratio, 1)}× o fundo (${usvVal(objU)}), com ${fmt(r.sigma, 1)}σ de certeza.`;
  s2.textContent = measureExplain(r, st.obj, auto, ok);
  bt.replaceChildren(s1, " ", s2);
  // quatro números
  const t = $("mTilesNew");
  t.textContent = "";
  sumTile(t, "com o objeto", usvVal(objU), `fundo ${usvVal(st.bg.cpm * bk)} · ${fmt(r.ratio, 1)}× o fundo`);
  sumTile(t, "certeza", `${fmt(r.sigma, 1)}σ`, r.sigma >= 3 ? "acima de 3σ: diferença real" : r.sigma >= 2 ? "entre 2σ e 3σ: talvez" : "abaixo de 2σ: cabe no acaso");
  sumTile(t, "taxa extra", sgnUS(r.netUSv), `± ${uS(r.netErr, ok)}` + (st.distance ? ` · ${st.distance === "encostado" ? "encostado" : "a " + st.distance}` : ""), r.level === "sim" ? "warn" : "");
  const sf = d.safe || {};
  sumTile(t, "distância segura", sf.ok ? (sf.below ? `≤ ${cmTxt(sf.cm)}` : `${sf.approx ? "~" : ""}${cmTxt(sf.cm)}`) : "—",
    sf.ok ? (sf.below ? "já aqui o extra fica abaixo de 10% do fundo" : "onde o extra cai a 10% do fundo" + (sf.approx ? " (aproximada)" : "")) : sf.why || "");
  // cartões: isótopos e Bq (RC102 no objeto), os dois lado a lado, com você
  const cards = $("mRCards");
  cards.textContent = "";
  const card = (title) => { const c = document.createElement("div"); c.className = "m-viz m-rcard"; const h = document.createElement("h4"); h.textContent = title; c.appendChild(h); cards.appendChild(c); return c; };
  const p = (box, text, cls) => { const e = document.createElement("p"); e.className = "small" + (cls ? " " + cls : ""); e.textContent = text; box.appendChild(e); return e; };
  if (rc && st.spectrum && st.spectrum.role !== "voce") {
    const c = card("Isótopos prováveis");
    const iso = d.isotopes || [];
    if (!iso.length) p(c, st.spectrum.bg ? "Nenhuma linha gama conhecida subiu acima do fundo (3σ)." : "Sem o espectro do fundo, não deu para separar os picos do objeto dos do ambiente.", "muted");
    for (const g of iso) {
      const row = document.createElement("div");
      row.className = "m-iso";
      row.innerHTML = '<div><strong></strong><span class="small muted"></span></div><div class="m-conf"><div class="m-conf-bar"><i></i></div><span class="mono small muted"></span></div>';
      row.querySelector("strong").textContent = g.group;
      row.querySelector(".small.muted").textContent = `${g.nuclides.join(", ")} · ${g.kev.map((x) => Math.round(x)).join(", ")} keV · ${g.lines} de ${g.of} ${g.of === 1 ? "linha" : "linhas"}`;
      row.querySelector("i").style.width = Math.round(g.conf * 100) + "%";
      row.querySelector(".mono").textContent = `confiança ${Math.round(g.conf * 100)}%`;
      c.appendChild(row);
    }
    p(c, "Pista pelo espectro, não identificação garantida. Mais tempo de medição deixa a confiança maior.", "muted");
    const bq = d.bq;
    if (bq) {
      const c2 = card("Atividade (Bq)");
      if (bq.ok) {
        const v = p(c2, "");
        v.className = "m-bq mono";
        v.textContent = `${bqTxt(bq.lo)} a ${bqTxt(bq.hi)}`;
        p(c2, `Estimativa de ordem de grandeza para ${bq.group}, pela taxa de dose a ${st.distance} (Γ = ${fmt(bq.gamma, bq.gamma < 0.01 ? 3 : 3)} µSv·m²/h por MBq).`);
      } else {
        const e = p(c2, "");
        const b = document.createElement("b");
        b.textContent = "Não dá pra estimar com segurança nesta medição.";
        e.append(b, " O aparelho mede taxa de dose; passar para Bq exige saber o isótopo, a distância e que a fonte seja pequena.");
      }
      const ul = document.createElement("ul");
      ul.className = "m-checks small";
      for (const ck of bq.checks || []) { const li = document.createElement("li"); li.className = ck.ok ? "ok" : "no"; li.textContent = ck.text; ul.appendChild(li); }
      c2.appendChild(ul);
    }
  }
  const o = st.other;
  if (o && o.role === "objeto") {
    const c = card("Os dois aparelhos, lado a lado");
    const tb = document.createElement("table");
    tb.className = "m-stats mono";
    tb.innerHTML = "<thead><tr><th></th><th>RC102</th><th>GMC-800</th></tr></thead><tbody></tbody>";
    const add = (k, a, b) => { const tr = document.createElement("tr"); [k, a, b].forEach((x, i) => { const td = document.createElement(i ? "td" : "th"); td.textContent = x; tr.appendChild(td); }); tb.tBodies[0].appendChild(tr); };
    const rcr = rc ? r : o.result, gr = rc ? o.result : r;
    if (rcr && gr) {
      add("taxa extra", sgnUS(rcr.netUSv), sgnUS(gr.netUSv));
      add("× o fundo", `${fmt(rcr.ratio, 1)}×`, `${fmt(gr.ratio, 1)}×`);
      add("certeza", `${fmt(rcr.sigma, 1)}σ`, `${fmt(gr.sigma, 1)}σ`);
    }
    c.appendChild(tb);
    const agree = rcr && gr && (rcr.sigma >= 3) === (gr.sigma >= 3);
    const q = p(c, "");
    const b = document.createElement("b");
    b.textContent = agree ? "Concordam? Sim." : "Concordam? Não totalmente.";
    q.append(b, " " + (o.hint || ""));
    p(c, "O GMC vê beta e gama; o RC102, quase só gama. O tubo e o cristal ficam em lugares diferentes do objeto, então vale como pista.", "muted");
  } else if (o && o.role === "voce") {
    const c = card("O que chegou até você");
    const secs = st.obj ? st.obj.seconds : 0;
    measKV(c.appendChild(document.createElement("div")), [
      ["aparelho com você", devLabel(o.device)],
      ["sua dose nesta medição", `${fmt(o.dose, 4)} µSv em ${fmtMin(secs)}`],
      ["média · máximo", `${usvVal(o.avgUSv)} · ${usvVal(o.maxUSv)}`],
      ["extra por estar perto", o.bgUSv ? `${o.excess > 0 ? "+" : "−"}${fmt(Math.abs(o.excess), 4)} µSv` : "—"],
    ]);
    // o extra em coisas conhecidas: "≈ 3 h do fundo da sua casa · ≈ meia hora de voo"
    const kn = o.bgUSv && o.excess > 0 ? doseKnownEl(o.excess, o.bgUSv, "m-known") : null;
    if (kn) { kn.prepend(Object.assign(document.createElement("b"), { textContent: "dose extra" })); c.appendChild(kn); }
    p(c, o.bgUSv ? (o.excess > 0 ? `Acima do nível de fundo (${usvVal(o.bgUSv)}): é o que o objeto somou na sua dose nesse tempo.` : `Igual ao nível de fundo (${usvVal(o.bgUSv)}): o objeto não somou nada que dê para ver na sua dose.`) : "Sem o fundo da casa desse aparelho, só a dose total.", "muted");
  }
  if (d.invSq) { const c = card("Duas distâncias"); p(c, d.invSq.text); }
  // espectros
  const sp = st.spectrum;
  renderNetSpec($("mNetSpecBox"), sp);
  // sem o espectro do fundo não dá para descontar: mostra o espectro inteiro
  renderMeasSpec($("mSpec"), $("mNetSpecBox").hidden ? sp : null, sp && sp.role === "voce" ? { title: "Espectro · RC102 com você", objLabel: "com você enquanto mexe no objeto" } : {});
  $("mNetSpecTitle").textContent = sp && sp.role === "voce" ? "Espectro do que chegou até você, fundo descontado" : "Espectro do objeto, fundo descontado";
  // detalhes técnicos
  const tech = [];
  tech.push(["contagens com o objeto", `${st.obj.counts.toLocaleString(LOCALE)} em ${st.obj.seconds} s (${fmt(st.obj.cpm / 60, 2)} CPS · ${fmt(st.obj.cpm, 1)} CPM)`]);
  tech.push(["contagens do fundo", `${st.bg.counts.toLocaleString(LOCALE)} em ${st.bg.seconds} s (${fmt(st.bg.cpm / 60, 2)} CPS · ${fmt(st.bg.cpm, 1)} CPM)` + (st.bg.source === "casa" ? " · fundo da casa" : "")]);
  tech.push(["aparelho", st.mode === "ambos" ? "RC102 + GMC-800 no objeto" : st.mode === "voce" ? `${devLabel(dv)} no objeto, ${devLabel(rc ? "gmc" : "radiacode")} com você` : devLabel(dv)]);
  tech.push(["σ da diferença", `${fmt(r.netErr / 60, 3)} CPS = ${uS(r.netErr, ok)}`]);
  tech.push(["limite “tem algo”", auto ? `+${fmt(st.pct, 0)}% do fundo e ≥ 3σ (para sozinho com 4σ)` : "≥ 3σ (tempo fixo)"]);
  tech.push(["conversão para dose", rc ? `${fmt(ok * 60, 4)} µSv/h por CPS (média do próprio RC102)` : `${fmt(ok, 5)} µSv/h por CPM (calibração do GMC-800)`]);
  tech.push(["distância", st.distance || "não informada"]);
  if (st.size) tech.push(["tamanho", st.size === "pequeno" ? "pequeno (fonte pontual)" : "espalhado"]);
  const dl = $("mTech");
  dl.textContent = "";
  for (const [k, v] of tech) { const dt = document.createElement("dt"); dt.textContent = k; const dd = document.createElement("dd"); dd.className = "mono"; dd.textContent = v; dl.append(dt, dd); }
  $("mStats").replaceChildren(measureStats({ bg: st.bg, obj: st.obj, result: r, device: st.device }));
  $("mExplain").textContent = "";
  // salvar e relatório
  $("btnMOpenSaved").hidden = !st.savedId;
  const rp = $("mReport");
  rp.hidden = !st.savedId;
  if (st.savedId) {
    const a = document.createElement("span"), b2 = document.createElement("span");
    a.textContent = "Salva e marcada no gráfico de CPM.";
    b2.textContent = st.reportPath ? "Relatório salvo: " + st.reportPath.split(/[\\/]/).pop()
      : st.reportError ? "Relatório não saiu: " + st.reportError : state.settings.noMeasureReport ? "" : "Gerando o relatório…";
    $("mReportText").replaceChildren(a, " ", b2);
    rp.dataset.id = st.savedId;
    rp.dataset.path = st.reportPath || "";
  } else if (st.demo) {
    rp.hidden = false;
    $("mReportText").textContent = "Medição do modo demo: não fica salva.";
  }
  $("btnMReportOpen").hidden = !st.reportPath;
  $("btnMReportMake").hidden = !st.savedId || !!st.reportPath;
}

const bqTxt = (v) => (v >= 1e6 ? `${fmt(v / 1e6, 1)} MBq` : v >= 1e3 ? `${fmt(v / 1e3, 1)} kBq` : `${fmt(v, 0)} Bq`);

// espectro líquido (objeto − fundo no mesmo tempo), em contagens/s, com os
// picos marcados
function renderNetSpec(box, sp) {
  const ok = sp && sp.obj && sp.obj.length && sp.objSec > 0 && sp.a && sp.a[1] > 0 && sp.bg && sp.bg.length === sp.obj.length && sp.bgSec > 0;
  box.hidden = !ok;
  const host = $("mNetSpec");
  if (!ok) { host.textContent = ""; return; }
  const { bins } = measSpecBins(sp);
  const net = bins.map((b) => Math.max(0, b.obj - b.bg));
  const mx = Math.max(...net, 0);
  if (!(mx > 0)) { box.hidden = true; host.textContent = ""; return; }
  const W = 640, H = 190, PL = 44, PR = 10, PT = 20, PB = 22;
  const e0 = bins[0].e0, e1 = bins[bins.length - 1].e1;
  const x = (e) => PL + ((e - e0) / (e1 - e0)) * (W - PL - PR);
  const y = (v) => PT + (H - PT - PB) * (1 - v / (mx * 1.08));
  let g = "";
  for (const f of [0.25, 0.5, 0.75, 1]) { const yy = y(mx * f).toFixed(1); g += `<line x1="${PL}" x2="${W - PR}" y1="${yy}" y2="${yy}" class="grid"/><text x="${PL - 5}" y="${(+yy + 4).toFixed(1)}" text-anchor="end" class="ax">${fmt(mx * f, mx < 1 ? 2 : mx < 10 ? 1 : 0)}</text>`; }
  for (let e = 500; e < e1; e += 500) g += `<text x="${x(e).toFixed(1)}" y="${H - 6}" text-anchor="middle" class="ax">${e}</text>`;
  g += `<text x="${W - PR}" y="${H - 6}" text-anchor="end" class="ax">keV</text>`;
  bins.forEach((b, i) => { if (net[i] > 0) g += `<rect x="${x(b.e0).toFixed(1)}" y="${y(net[i]).toFixed(1)}" width="${Math.max(0.8, x(b.e1) - x(b.e0) - 0.6).toFixed(1)}" height="${(H - PB - y(net[i])).toFixed(1)}" class="net"/>`; });
  const rows = [];
  for (const pk of [...(sp.peaks || [])].sort((a, b) => a.kev - b.kev)) {
    if (pk.kev < e0 || pk.kev > e1) continue;
    const xx = x(pk.kev), lbl = `${pk.nuclide} ${Math.round(pk.kev)}`, w = lbl.length * 6.8 + 6, right = xx + 3 + w > W - PR;
    const a0 = right ? xx - 3 - w : xx, a1 = right ? xx : xx + 3 + w;
    let rr = rows.findIndex((end) => end < a0);
    if (rr < 0) { rr = rows.length; rows.push(0); }
    rows[rr] = Math.max(rows[rr], a1);
    g += `<line x1="${xx.toFixed(1)}" x2="${xx.toFixed(1)}" y1="${PT}" y2="${H - PB}" class="peak"/><text x="${(right ? xx - 3 : xx + 3).toFixed(1)}" y="${PT - 6 + 11 + rr * 12}" text-anchor="${right ? "end" : "start"}" class="peak-l">${escHtml(lbl)}</text>`;
  }
  host.innerHTML = `<svg viewBox="0 0 ${W} ${H}" class="m-spec-svg" role="img" aria-label="Espectro líquido do objeto">${g}</svg><div class="legend small muted"><span>contagens/s acima do fundo, por faixa de energia</span></div>`;
}

// ---------- segundo aparelho (os dois perto, ou um com você) ----------

const devLabel = (d) => (d === "radiacode" ? "RC102" : "GMC-800");
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
  add("small muted", "O GMC vê beta e gama; o RC102, quase só gama. Vale como pista: o tubo e o cristal ficam em lugares diferentes do objeto.");
}

// ---------- espectro da medição (Radiacode) ----------

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
function renderMeasSpec(box, sp, opt) {
  opt = opt || {};
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
  const bgTxt = !hasBg ? "" : opt.bgLabel ? opt.bgLabel : sp.bgSrc === "acumulado" ? `fundo: espectro acumulado do aparelho (${rcDur(sp.bgSec) || fmtMin(sp.bgSec)}), na escala do tempo do objeto` : `fundo (${specDur(sp.bgSec)})`;
  let txt = "";
  const groups = new Map();
  if (!hasBg) txt = "Sem o espectro do fundo: não deu para separar os picos do objeto dos do ambiente.";
  else if (!found.length) txt = "Nenhuma linha gama conhecida subiu acima do fundo (3σ).";
  else {
    txt = "Linhas acima do fundo (uma pista do radionuclídeo, não uma identificação garantida):";
    for (const p of found) { if (!groups.has(p.group)) groups.set(p.group, []); groups.get(p.group).push(p.kev); }
  }
  box.innerHTML = `<h4>${escHtml(opt.title || "Espectro")}</h4><svg viewBox="0 0 ${W} ${H}" class="m-spec-svg" role="img" aria-label="Espectro do objeto e do fundo">${g}</svg>
    <div class="legend small muted"><span class="key"><i class="mk-obj" aria-hidden="true"></i>${escHtml(opt.objLabel || "com o objeto")} (${specDur(sp.objSec)})</span>${hasBg ? `<span class="key"><i class="mk-bg" aria-hidden="true"></i>${bgTxt}</span>` : ""}<span>contagens/s em escala log</span></div>
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
    sim: "A diferença é bem maior que a variação natural da contagem. O valor em µSv/h vale para o objeto na distância medida; a 1 metro é muito menos.",
    talvez: "A diferença está perto do limite da variação natural. Use o modo automático para medir até ter certeza.",
    nada: "Se o objeto emitir alguma coisa, está abaixo do que dá para ver nesse tempo" + (r.limit ? ` (menos que +${uS(r.limit, k)})` : "") + ". Use o modo automático para mais certeza.",
  }[r.level];
}

function wireMeasure() {
  // escolhas lembradas
  const sv = localGet("m-mSecs");
  if (sv && [...$("mSecs").options].some((o) => o.value === sv)) $("mSecs").value = sv;
  $("mSecs").addEventListener("change", () => { mSetOpt("mSecs", $("mSecs").value); renderMeasure(state.measureSt || {}); });
  const dist = localGet("m-distCm");
  if (dist != null && dist !== "") $("mDistCm").value = dist;
  const rr = () => renderMeasure(state.measureSt || {});
  $("mDistCm").addEventListener("input", () => { mSetOpt("distCm", $("mDistCm").value); rr(); });
  const segs = [["#mDistSeg", "dist", (v) => { $("mDistCm").value = v; mSetOpt("distCm", v); }], ["#mSizeSeg", "size", (v) => mSetOpt("size", v)],
    ["#mBgSeg", "bg", (v) => mSetOpt("mBgSrc", v)], ["#mPctSeg", "pct", (v) => mSetOpt("mPct", v)],
    ["#mObjDevSeg", "objdev", (v) => { mSetOpt("mObjDev", v); refreshHomeBg(true); }]];
  for (const [sel, key, fn] of segs) document.querySelectorAll(sel + " button").forEach((b) => b.addEventListener("click", () => { fn(b.dataset[key]); rr(); }));
  document.querySelectorAll("#mDevs .m-dev").forEach((b) => b.addEventListener("click", () => { mSetOpt("mDev", b.dataset.dev); refreshHomeBg(true); rr(); }));
  // setas no grupo de aparelhos (radiogroup)
  $("mDevs").addEventListener("keydown", (e) => {
    if (!["ArrowRight", "ArrowLeft", "ArrowDown", "ArrowUp"].includes(e.key)) return;
    const bs = [...document.querySelectorAll("#mDevs .m-dev")], i = bs.findIndex((b) => b.getAttribute("aria-checked") === "true");
    const n = bs[(i + (e.key === "ArrowRight" || e.key === "ArrowDown" ? 1 : bs.length - 1)) % bs.length];
    e.preventDefault();
    n.click();
    n.focus();
  });
  $("btnMStart").addEventListener("click", () => { state.mBusca = true; state.mBuscaSkip = false; buscaReset(); rr(); });
  $("btnMBuscaBack").addEventListener("click", () => { state.mBusca = false; rr(); });
  $("btnMBuscaSkip").addEventListener("click", () => { state.mBuscaSkip = true; measureBegin(); });
  $("btnMBuscaOk").addEventListener("click", () => { state.mBuscaSkip = false; measureBegin(); });
  $("btnMUseHome").addEventListener("click", async () => { await api.CancelMeasure(); measureBegin(true); });
  $("btnMBgCancel").addEventListener("click", async () => { state.mBusca = false; renderMeasure(await api.CancelMeasure()); refreshHomeBg(true); });
  $("btnMObjGo").addEventListener("click", async () => {
    try { renderMeasure(await api.StartMeasurePhase("objeto", 0, $("mName").value)); } catch (e) { toast(errMsg(e), true); }
  });
  $("btnMStop").addEventListener("click", async () => {
    const b = $("btnMStop");
    try { await api.StopMeasure(); b.textContent = "Parando…"; b.disabled = true; setTimeout(() => { b.textContent = "Parar e ver o resultado"; b.disabled = false; }, 6000); } catch (e) { toast(errMsg(e), true); }
  });
  $("btnMeasureReset").addEventListener("click", async () => {
    state.mBusca = false;
    $("mAgainBox").hidden = true;
    renderMeasure(await api.CancelMeasure());
    refreshHomeBg(true);
  });
  // medir em outra distância: repete só a etapa do objeto
  $("btnMAgain").addEventListener("click", () => {
    const box = $("mAgainBox");
    box.hidden = !box.hidden;
    if (!box.hidden) { const st = state.measureSt || {}; const cm = st.distCm > 0 ? st.distCm * 2 : 10; $("mAgainCm").value = cm; mAgainSeg(); $("mAgainCm").focus(); }
  });
  const mAgainSeg = () => document.querySelectorAll("#mAgainSeg button").forEach((b) => b.setAttribute("aria-pressed", String(+b.dataset.dist === +$("mAgainCm").value)));
  $("mAgainCm").addEventListener("input", mAgainSeg);
  document.querySelectorAll("#mAgainSeg button").forEach((b) => b.addEventListener("click", () => { $("mAgainCm").value = b.dataset.dist; mAgainSeg(); }));
  $("btnMAgainGo").addEventListener("click", async () => {
    const cm = parseFloat(String($("mAgainCm").value).replace(",", "."));
    if (!(cm > 0)) { toast("Escreva a nova distância em cm.", true); return; }
    try { $("mAgainBox").hidden = true; renderMeasure(await api.MeasureAgainAt(`${fmt(cm, cm % 1 ? 1 : 0)} cm`, cm)); } catch (e) { toast(errMsg(e), true); }
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
    (m.mode === "ambos" ? " · os dois aparelhos" : m.mode === "voce" ? ` · ${rc ? "RC102" : "GMC-800"} no objeto, ${rc ? "GMC-800" : "RC102"} com você` : rc ? " · RC102" : "");
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
  if (isRcMeasure(m)) cfg.textContent = "Medição do RC102: o µSv/h vem da calibração do próprio aparelho.";
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
