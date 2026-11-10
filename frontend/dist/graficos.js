// Página Gráficos, aba Dose: o gráfico de CPM (com zoom por arraste) e a
// dose acumulada dos dois aparelhos. Saiu do app.js na 2.21, sem mudar nada
// na tela.
//
// Usa os globais do app.js (state, api, $, fmt, hhmm...). Este arquivo é
// carregado antes do app.js e, na carga, só define funções e constantes e
// registra um clique no documento. O estado da página (state.doseSrc,
// state.doseShow) é criado no app.js.

// ---------- gráfico ----------

const W = 600, H = 200;

// teto "redondo" para o eixo (1, 2, 2,5 ou 5 vezes uma potência de 10)
function niceTop(v) {
  if (!(v > 0)) return 1;
  const mag = Math.pow(10, Math.floor(Math.log10(v)));
  for (const m of [1, 2, 2.5, 5, 10]) if (m * mag >= v) return m * mag;
  return 10 * mag;
}

// formato de hora conforme o passo e o tamanho do período
function chartTimeFmt(c) {
  const crossesDay = new Date(c.from).toDateString() !== new Date((c.to || c.from) - 1).toDateString();
  if (crossesDay) return (t) => `${ddmm(t)} ${hhmm(t)}`;
  if ((c.stepMs || 60000) < 60000) return hhmmss;
  return hhmm;
}

let chartSeq = 0, doseSeq = 0; // descarta respostas atrasadas (zoom trocou no meio)

async function refreshChart() {
  const my = ++chartSeq;
  const c = state.zoom ? await api.GetChartRange(state.zoom.from, state.zoom.to) : await api.GetChart(state.range);
  if (my !== chartSeq) return;
  state.chart = c;
  c.rain = [];
  c.radar = [];
  c.rainErr = "";
  try { c.notes = (await api.GetNotes(c.from, c.to + 60e3)) || []; } catch (e) { c.notes = []; }
  if (my !== chartSeq) return;
  if (state.settings.rainEnabled && c.to - c.from <= 8 * 864e5) { // chuva: até ~1 semana
    const [rain, radar] = await Promise.all([
      api.GetRain(c.from, c.to).catch((e) => { c.rainErr = errMsg(e); return []; }),
      api.GetRainRadar(c.from, c.to).catch(() => []),
    ]);
    if (my !== chartSeq) return;
    c.rain = rain || [];
    c.radar = radar || [];
  }
  document.querySelectorAll("#rangeSeg button").forEach((b) => b.setAttribute("aria-pressed", String(!state.zoom && b.dataset.range === state.range)));
  renderZoomBar(c);

  const usvMode = state.chartUnit === "usv";
  document.querySelectorAll("#chartUnitSeg button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.cunit === state.chartUnit)));
  // o gráfico em µSv/h é o principal (limites, alarmes e picos); o de CPM é
  // só de análise: as contagens de cada aparelho, sem nenhum alarme (2.29)
  $("chartTitle").textContent = usvMode ? "µSv/h ao longo do tempo" : "CPM ao longo do tempo (análise)";
  const vals = (usvMode ? c.usv : c.values) || [];
  const thrV = usvMode ? c.attentionUSv : c.attentionCPM || 50;
  let max = 0;
  for (const v of vals) if (v != null && v > max) max = v;
  // picos dos alertas (leitura de segundo): entram na escala e viram ponto
  const kU = c.alarmCPM ? c.alarmUSv / c.alarmCPM : 0;
  const peakV = (p) => (usvMode ? p.cpm * kU : p.cpm);
  for (const p of c.peaks || []) max = Math.max(max, peakV(p)); // o "máx" já os inclui
  // Radiacode: no µSv/h, na mesma escala do GMC; no CPM, com escala própria
  // no eixo da direita (conta umas 28 vezes mais que o GMC, então na mesma
  // escala a linha do GMC sumiria no chão do gráfico)
  const rcVals = usvMode ? c.rc || null : c.rcCPM || null;
  let rcMax = 0;
  if (rcVals) for (const v of rcVals) if (v != null && v > rcMax) rcMax = v;
  if (usvMode) max = Math.max(max, rcMax);
  // durante um alarme, mostra as linhas do seu limite e do perigo do app
  // (só no µSv/h: o gráfico de CPM não mostra alarme)
  const ringing = usvMode && !!alarmCtl.kind;
  const alarmV = usvMode ? c.alarmUSv : c.alarmCPM;
  const dangerV = usvMode ? c.dangerUSv : c.dangerCPM;
  const testDanger = alarmCtl.kind === "test" && alarmCtl.sev === "perigo";
  // o perigo só entra na escala se a leitura estiver perto dele; se não,
  // aparece na legenda como "acima da escala" (para não achatar o gráfico)
  const dangerInScale = ringing && dangerV && (alarmCtl.sev === "perigo" || max >= dangerV * 0.4) && !testDanger;
  let want = max;
  if (ringing && alarmV) want = Math.max(want, alarmV);
  if (dangerInScale) want = Math.max(want, dangerV);
  const top = usvMode ? niceTop(Math.max(thrV * 1.2, want * 1.15)) : Math.max(30, Math.ceil((want * 1.15) / 15) * 15);
  c.top = top;
  const yd = usvMode ? (top < 0.3 ? 3 : top < 10 ? 2 : 1) : 0;
  const ys = $("yAxis").children;
  for (let i = 0; i < 4; i++) ys[i].textContent = fmt(top - (top / 3) * i, i === 3 ? 0 : yd);
  // eixo da direita: CPM do Radiacode
  const rcAxis = !usvMode && rcMax > 0;
  const rcTop = rcAxis ? Math.max(300, Math.ceil((rcMax * 1.15) / 300) * 300) : top;
  $("yAxisR").hidden = !rcAxis;
  if (rcAxis) { const yr = $("yAxisR").children; for (let i = 0; i < 4; i++) yr[i].textContent = fmt(rcTop - (rcTop / 3) * i, 0); }

  const n = vals.length;
  const x = (i) => (n <= 1 ? 0 : (i / (n - 1)) * W);
  const y = (v) => H - (v / top) * H;

  let line = "", area = "", seg = [];
  const flush = () => {
    if (!seg.length) return;
    line += "M" + seg.map((p) => p[0].toFixed(1) + " " + p[1].toFixed(1)).join("L");
    if (seg.length === 1) line += "l0.01 0";
    area += `M${seg[0][0].toFixed(1)} ${H}L` + seg.map((p) => p[0].toFixed(1) + " " + p[1].toFixed(1)).join("L") + `L${seg[seg.length - 1][0].toFixed(1)} ${H}Z`;
    seg = [];
  };
  vals.forEach((v, i) => { if (v == null) flush(); else seg.push([x(i), y(v)]); });
  flush();
  let rcLine = "", rcArea = "";
  if (rcVals) {
    let pts = [];
    const out = () => {
      if (pts.length) {
        rcLine += "M" + pts.map((p) => p.join(" ")).join("L") + (pts.length === 1 ? "l0.01 0" : "");
        rcArea += `M${pts[0][0]} ${H}L` + pts.map((p) => p.join(" ")).join("L") + `L${pts[pts.length - 1][0]} ${H}Z`;
      }
      pts = [];
    };
    const yr = (v) => H - (v / rcTop) * H;
    rcVals.forEach((v, i) => { if (v == null) out(); else pts.push([x(i).toFixed(1), (usvMode ? y(v) : yr(v)).toFixed(1)]); });
    out();
  }
  $("rcKey").hidden = !rcVals;
  $("rcKeyText").textContent = usvMode ? "RC102" : "RC102 (eixo da direita)";

  const grid = [0, 1, 2, 3].map((i) => {
    const gy = i === 3 ? H - 0.5 : i === 0 ? 0.5 : (H / 3) * i;
    return `<line x1="0" y1="${gy}" x2="${W}" y2="${gy}" stroke="${COR.grid}" stroke-width="1" vector-effect="non-scaling-stroke"/>`;
  }).join("");
  const hline = (v, color, width) => `<line x1="0" y1="${y(v).toFixed(1)}" x2="${W}" y2="${y(v).toFixed(1)}" stroke="${color}" stroke-width="${width}" stroke-dasharray="8 5" vector-effect="non-scaling-stroke"/>`;
  let thr = usvMode && top >= thrV ? hline(thrV, COR.att, 1) : "";
  const showAlarm = ringing && alarmV && alarmV <= top;
  const showDanger = ringing && dangerV && dangerV <= top;
  if (showAlarm && !(dangerV && alarmV >= dangerV)) thr += hline(alarmV, COR.alarm, 2);
  if (showDanger) thr += hline(dangerV, COR.warn, 2);
  // forma do nível na ponta direita de cada linha de limite (ideia 12)
  const mark = (v, lv) => `<span style="top:${((y(v) / H) * 100).toFixed(2)}%">${lvIcon(lv, 10)}</span>`;
  $("thrMarks").innerHTML = (usvMode && top >= thrV ? mark(thrV, "atencao") : "") +
    (showAlarm && !(dangerV && alarmV >= dangerV) ? mark(alarmV, "alarme") : "") + (showDanger ? mark(dangerV, "perigo") : "");
  const fmtV = (v) => (usvMode ? `${fmt(v, usvDigits(v))} µSv/h` : `${fmt(v, 0)} CPM`);
  $("legAlarm").hidden = !ringing || !alarmV || (dangerV && alarmV >= dangerV);
  $("legAlarm").lastChild.textContent = `seu alarme: ${fmtV(alarmV)}`;
  $("legDanger").hidden = !ringing || !dangerV;
  $("legDanger").lastChild.textContent = `perigo do app: ${fmtV(dangerV)}` + (showDanger ? "" : " (acima da escala)");
  $("legDanger").classList.toggle("off-scale", !showDanger);
  const tEnd = c.times && c.times.length ? c.times[c.times.length - 1] : c.to;
  const xt = (t) => Math.max(0, Math.min(W, ((t - c.from) / Math.max(1, tEnd - c.from)) * W));
  // radar (medido, a cada 10 min) tem prioridade; o modelo só preenche as
  // horas em que não há leitura do radar
  const radarWet = c.radar.filter((p) => p.mmh > 0);
  const radarHour = (t) => c.radar.some((p) => p.time >= t && p.time < t + 3600e3);
  const rainy = c.rain.filter((p) => p.mm >= 0.1 && !radarHour(p.time));
  const bar = (t1, t2, h, fill, op) => {
    const x1 = xt(t1), x2 = xt(t2);
    return `<rect x="${x1.toFixed(1)}" y="${(H - h).toFixed(1)}" width="${Math.max(1, x2 - x1).toFixed(1)}" height="${h.toFixed(1)}" fill="${fill}" fill-opacity="${op}"/>`;
  };
  const rainBars = rainy.map((p) => bar(p.time, p.time + 3600e3, Math.max(4, Math.min(1, p.mm / 5) * H * 0.3), COR.info, 0.3)).join("") +
    radarWet.map((p) => bar(p.time, p.time + RADAR_STEP, Math.max(4, Math.min(1, p.mmh / 10) * H * 0.3), "#3DA5FF", 0.6)).join("");
  $("rainKey").hidden = !rainy.length && !radarWet.length;
  $("rainKey").lastChild.textContent = radarWet.length ? (rainy.length ? "chuva (radar · modelo)" : "chuva (radar)") : "chuva (modelo)";
  const away = awayBands(c.from, tEnd, xt);
  $("awayKey").hidden = !away;
  const meas = measureBands(c.from, tEnd, xt);
  $("measKey").hidden = !meas;
  // anotações: linha tracejada roxa e uma marca clicável no topo
  const noteX = (t) => xt(t);
  $("noteMarks").innerHTML = "";
  const noteLines = (c.notes || []).map((nt) => {
    const nx = noteX(Math.max(nt.time, c.from));
    const m = document.createElement("button");
    m.type = "button";
    m.className = "note-mark";
    m.style.left = ((nx / W) * 100).toFixed(2) + "%";
    m.title = `${nt.end ? hhmm(nt.time) + "–" + hhmm(nt.end) : hhmm(nt.time)} · ${nt.text} (clique para editar)`;
    m.addEventListener("mousedown", (e) => e.stopPropagation());
    m.addEventListener("click", (e) => { e.stopPropagation(); openNote(nt); });
    $("noteMarks").appendChild(m);
    const line = `<line x1="${nx.toFixed(1)}" y1="0" x2="${nx.toFixed(1)}" y2="${H}" stroke="${COR.note}" stroke-width="1.2" stroke-dasharray="3 3" vector-effect="non-scaling-stroke"/>`;
    if (!nt.end) return line;
    const ex = noteX(Math.min(nt.end, tEnd));
    return `<rect x="${nx.toFixed(1)}" y="0" width="${Math.max(1, ex - nx).toFixed(1)}" height="${H}" fill="${COR.note}" fill-opacity="0.10"/>` + line +
      `<line x1="${ex.toFixed(1)}" y1="0" x2="${ex.toFixed(1)}" y2="${H}" stroke="${COR.note}" stroke-opacity="0.5" stroke-width="1" stroke-dasharray="3 3" vector-effect="non-scaling-stroke"/>`;
  }).join("");
  $("noteKey").hidden = !(c.notes || []).length;
  // ponto de cada pico, em cima do ponto do gráfico daquele trecho
  const stepMs = c.stepMs || 60000;
  $("peakDots").innerHTML = (usvMode ? c.peaks || [] : []).map((p) => {
    const i = Math.floor((p.t - c.from) / stepMs);
    if (i < 0 || i >= n) return "";
    const col = { perigo: COR.warn, alarme: COR.alarm }[p.level] || COR.att;
    return `<i style="left:${((x(i) / W) * 100).toFixed(2)}%;top:${((y(peakV(p)) / H) * 100).toFixed(2)}%;background:${col}"></i>`;
  }).join("");
  // com o Radiacode (aparelho principal), ele é a curva cheia e o GMC (casa)
  // vira uma linha de referência
  $("chart").innerHTML = grid + away + meas + rainBars + thr + noteLines + (rcLine
    ? `<path d="${rcArea}" fill="${COR.rc}" fill-opacity="0.12"/>` +
      `<path d="${line}" fill="none" stroke="${COR.accent}" stroke-opacity="0.75" stroke-width="1.4" stroke-linejoin="round" stroke-linecap="round" vector-effect="non-scaling-stroke"/>` +
      `<path d="${rcLine}" fill="none" stroke="${COR.rc}" stroke-width="2" stroke-linejoin="round" stroke-linecap="round" vector-effect="non-scaling-stroke"/>`
    : `<path d="${area}" fill="${COR.accent}" fill-opacity="0.12"/>` +
      `<path d="${line}" fill="none" stroke="${COR.accent}" stroke-width="2" stroke-linejoin="round" stroke-linecap="round" vector-effect="non-scaling-stroke"/>`);
  $("gmcKey").hidden = !rcLine;

  if (state.zoom) {
    const f = chartTimeFmt(c);
    const t0 = c.times && c.times.length ? c.times[0] : c.from;
    $("x0").textContent = f(t0);
    $("x1").textContent = f((t0 + tEnd) / 2);
    $("x2").textContent = f(tEnd);
  } else {
    const lbl = state.range === "1y" ? (ms) => { const t = new Date(ms); return pad(t.getMonth() + 1) + "/" + String(t.getFullYear()).slice(2); } : isLongRange() ? ddmm : hhmm;
    $("x0").textContent = lbl(c.from);
    $("x1").textContent = lbl((c.from + c.to) / 2);
    $("x2").textContent = isLongRange() ? "hoje" : "agora";
  }
  // nos períodos curtos o gráfico usa leituras por segundo
  $("chartStats").dataset.short = String(isShortRange());
  $("legendText").textContent = usvMode
    ? `atenção: ${fmt(c.attentionUSv, usvDigits(c.attentionUSv))} µSv/h`
    : "análise: sem limites nem alarmes (eles ficam no µSv/h)";
  $("legDash").hidden = !usvMode;
  $("chartStats").textContent = c.avg == null
    ? "sem dados no período"
    : usvMode
      ? (c.rcAvg != null ? `RC102 média ${fmt(c.rcAvg, 3)} · GMC-800 média ${fmt(c.avgUSv, 3)} · mín ${fmt(c.minUSv, 3)} · máx ${fmt(c.maxUSv, 3)} µSv/h` : `média ${fmt(c.avgUSv, 3)} · mín ${fmt(c.minUSv, 3)} · máx ${fmt(c.maxUSv, 3)} µSv/h`)
      : `GMC-800 média ${fmt(c.avg, 1)} · mín ${fmt(c.min, 0)} · máx ${fmt(c.max, 0)} CPM` + (c.rcAvgCPM != null ? ` · RC102 média ${fmt(c.rcAvgCPM, 0)} CPM` : "");
}

function setupChartHover() {
  const plot = $("plot");
  plot.addEventListener("mousemove", (ev) => {
    const c = state.chart;
    if (!c || !c.values || !c.values.length) return;
    const r = plot.getBoundingClientRect();
    const f = Math.min(1, Math.max(0, (ev.clientX - r.left) / r.width));
    const i = Math.round(f * (c.values.length - 1));
    const usvMode = state.chartUnit === "usv";
    const v = (usvMode ? c.usv : c.values)[i];
    const t = c.times[i];
    const cur = $("cursor"), tip = $("tip");
    cur.hidden = false;
    cur.style.left = (i / Math.max(1, c.values.length - 1)) * 100 + "%";
    tip.hidden = false;
    const when = state.zoom ? chartTimeFmt(c)(t) : isLongRange() ? `${ddmm(t)} ${hhmm(t)}` : isShortRange() ? hhmmss(t) : hhmm(t);
    const rr = (c.radar || []).find((p) => t >= p.time && t < p.time + RADAR_STEP);
    const pk = (c.peaks || []).filter((p) => p.t >= t && p.t < t + (c.stepMs || 60000)).sort((a, b) => b.cpm - a.cpm)[0];
    const kU = c.alarmCPM ? c.alarmUSv / c.alarmCPM : 0;
    const pkText = usvMode && pk && !(pk.retro && v != null && Math.round(pk.cpm) === Math.round(usvMode ? v / kU : v))
      ? ` · pico ${usvMode ? fmt(pk.cpm * kU, usvDigits(pk.cpm * kU)) + " µSv/h" : fmt(pk.cpm, 0) + " CPM"} às ${hhmmss(pk.t)}` : "";
    const rh = rr ? null : (c.rain || []).find((p) => t >= p.time && t < p.time + 3600e3 && p.mm >= 0.1);
    const half = Math.max(60e3, c.stepMs || 60000);
    const nt = (c.notes || []).filter((n) => (n.end ? t >= n.time - half && t <= n.end : Math.abs(n.time - t) <= half)).map((n) => n.text);
    const ntText = nt.length ? ` · nota: ${nt.join("; ")}` : "";
    const rcv = usvMode ? (c.rc ? c.rc[i] : null) : c.rcCPM ? c.rcCPM[i] : null;
    const rcText = rcv == null ? "" : usvMode ? ` · RC102 ${fmt(rcv, usvDigits(rcv))} µSv/h` : ` · RC102 ${fmt(rcv, 0)} CPM`;
    tip.textContent = (v == null ? `${when} · sem dados`
      : usvMode ? `${when} · ${fmt(v, usvDigits(v))} µSv/h`
      : `${when} · GMC-800 ${fmt(v, (c.stepMs || 0) >= 300000 ? 1 : 0)} CPM`) + pkText + (rr && rr.mmh > 0 ? ` · chuva ${rainWord(rr.mmh)} (radar)` : rh ? ` · chuva ${fmt(rh.mm, 1)} mm (modelo)` : "") + rcText + awayLabel(t) + measureLabel(t) + ntText;
    const left = (ev.clientX - r.left) + 12;
    tip.style.left = Math.min(left, r.width - tip.offsetWidth - 4) + "px";
  });
  plot.addEventListener("mouseleave", () => { $("cursor").hidden = true; $("tip").hidden = true; });
  // dois cliques: anotar naquele momento
  plot.addEventListener("dblclick", (ev) => {
    const c = state.chart;
    if (!c || !c.times || !c.times.length) return;
    const r = plot.getBoundingClientRect();
    const f = Math.min(1, Math.max(0, (ev.clientX - r.left) / r.width));
    const i = Math.round(f * (c.times.length - 1));
    openNote({ id: 0, time: Math.min(Date.now(), c.times[i]), text: "" });
  });
  setupChartZoom();
}

// ---------- zoom: arrastar no gráfico ----------

function chartTimeAt(c, frac) {
  const t0 = c.times && c.times.length ? c.times[0] : c.from;
  const t1 = c.times && c.times.length ? c.times[c.times.length - 1] : c.to;
  return t0 + Math.min(1, Math.max(0, frac)) * (t1 - t0);
}

// arrastar num gráfico para escolher um trecho (usado no CPM e na dose)
function attachDragZoom(plot, sel, getChart, onZoom) {
  let x0 = null;
  const frac = (ev) => { const r = plot.getBoundingClientRect(); return { f: (ev.clientX - r.left) / r.width, w: r.width }; };
  plot.addEventListener("mousedown", (ev) => {
    const c = getChart();
    if (ev.button !== 0 || !c || !c.times || !c.times.length) return;
    ev.preventDefault();
    x0 = frac(ev).f;
    sel.style.left = x0 * 100 + "%";
    sel.style.width = "0";
    sel.hidden = false;
  });
  window.addEventListener("mousemove", (ev) => {
    if (x0 == null) return;
    const f = Math.min(1, Math.max(0, frac(ev).f));
    sel.style.left = Math.min(x0, f) * 100 + "%";
    sel.style.width = Math.abs(f - x0) * 100 + "%";
  });
  window.addEventListener("mouseup", (ev) => {
    if (x0 == null) return;
    const { f, w } = frac(ev);
    const lo = Math.min(x0, f), hi = Math.max(x0, f);
    x0 = null;
    sel.hidden = true;
    if ((hi - lo) * w < 8) return; // foi só um clique
    const c = getChart();
    // a dica do mouse era do gráfico antigo
    plot.querySelectorAll(".tip, .cursor").forEach((el) => { el.hidden = true; });
    onZoom(Math.round(chartTimeAt(c, lo)), Math.round(chartTimeAt(c, hi)));
  });
}

function zoomRangeText(t0, t1) {
  const sameDay = new Date(t0).toDateString() === new Date(t1 - 1).toDateString();
  const day = isToday(t0) ? "hoje" : isYesterday(t0) ? "ontem" : ddmm(t0);
  return sameDay
    ? `${day} ${hhmm(t0)}–${hhmm(t1)} (${fmtDur(t1 - t0)})`
    : `${ddmm(t0)} ${hhmm(t0)} – ${ddmm(t1)} ${hhmm(t1)} (${fmtDur(t1 - t0)})`;
}

const stepText = (ms) => (ms < 60000 ? `${Math.round(ms / 1000)} s` : fmtDur(ms));

function setupChartZoom() {
  attachDragZoom($("plot"), $("zoomSel"), () => state.chart, (from, to) => {
    if (state.zoom) state.zoomStack.push(state.zoom);
    else state.zoomStack = [];
    state.zoom = { from, to };
    unlinkZoom();
    refreshChart();
  });
  $("btnZoomBack").addEventListener("click", () => { state.zoom = state.zoomStack.pop() || null; unlinkZoom(); refreshChart(); });
  $("btnZoomOut").addEventListener("click", resetAllZoom);
}

// Um só "Sair do zoom": tira o zoom dos dois gráficos (e a seleção do mapa).
// Ele aparece no primeiro gráfico com zoom; o outro mostra só o "Voltar".
function resetAllZoom() {
  state.zoom = null; state.zoomStack = [];
  state.doseZoom = null; state.doseZoomStack = [];
  state.zoomLinked = false;
  state.heatPick = null;
  refreshChart();
  refreshDose();
  markHeatPick();
}

// Zoom vindo do mapa de calor deixa os dois gráficos no mesmo trecho; mexer
// no zoom de um deles separa os dois.
function unlinkZoom() {
  if (!state.zoomLinked) return;
  state.zoomLinked = false;
  refreshChart();
  refreshDose();
}

function renderZoomButtons() {
  $("btnZoomOut").hidden = !state.zoom;
  $("btnDoseZoomOut").hidden = !state.doseZoom || !!state.zoom;
  $("btnZoomBack").hidden = !state.zoomStack.length;
  $("btnDoseZoomBack").hidden = !state.doseZoomStack.length;
}

function setupDoseZoom() {
  attachDragZoom($("dosePlot"), $("doseZoomSel"), () => state.dose, (from, to) => {
    if (state.doseZoom) state.doseZoomStack.push(state.doseZoom);
    else state.doseZoomStack = [];
    state.doseZoom = { from, to };
    unlinkZoom();
    refreshDose();
  });
  $("btnDoseZoomBack").addEventListener("click", () => { state.doseZoom = state.doseZoomStack.pop() || null; unlinkZoom(); refreshDose(); });
  $("btnDoseZoomOut").addEventListener("click", resetAllZoom);
}

function renderZoomBar(c) {
  markHeatPick();
  const bar = $("zoomBar");
  bar.hidden = !state.zoom;
  if (!state.zoom) return;
  const t0 = c.times && c.times.length ? c.times[0] : c.from;
  const t1 = c.to;
  $("zoomRange").textContent = zoomRangeText(t0, t1);
  const n = (c.values || []).filter((v) => v != null).length;
  const hours = (n * (c.stepMs || 60000)) / 3600e3;
  const dose = c.avgUSv != null ? c.avgUSv * hours : null;
  const step = stepText(c.stepMs || 60000);
  $("zoomStats").textContent = c.avg == null ? "· sem dados nesse trecho"
    : `· dose ≈ ${fmt(dose, dose < 1 ? 3 : 2)} µSv · um ponto a cada ${step}` + (c.fineAlertsOnly ? " (por segundo só perto dos alertas)" : "");
  renderZoomButtons();
}

// ---------- dose acumulada (dosímetro) ----------

function fmtDose(v) {
  if (v >= 1000) return [fmt(v / 1000, v >= 10000 ? 1 : 2), "mSv"];
  return [fmt(v, v < 10 ? 2 : v < 100 ? 1 : 0), "µSv"];
}
const doseTxt = (v) => fmtDose(v).join(" ");

// o Radiacode já mandou algum dado (está conectado ou tem minuto gravado)
function hasRcData() {
  const st = state.rc && state.rc.st;
  return !!(st && (st.state === "conectado" || st.lastT > 0));
}

// dose de um aparelho no período (ou no trecho)
function fetchDose(src, range) {
  return range
    ? (api.GetDoseRangeSrc ? api.GetDoseRangeSrc(range.from, range.to, state.doseLimit, src) : api.GetDoseRange(range.from, range.to, state.doseLimit))
    : (api.GetDoseSrc ? api.GetDoseSrc(state.dosePeriod, state.doseLimit, src) : api.GetDose(state.dosePeriod, state.doseLimit));
}

// quadros da dose de cada aparelho: total, média e quanto do teto já foi
function renderDevBoxes(box, list) {
  box.textContent = "";
  for (const d of list) {
    const c = d.c, el = document.createElement(list.length > 1 ? "button" : "div");
    el.className = "dd-box " + d.key;
    el.dataset.key = d.key;
    if (list.length > 1) {
      el.type = "button";
      const on = !!state.doseShow[d.key];
      el.classList.toggle("off", !on);
      el.setAttribute("aria-pressed", String(on));
      el.title = "Mostrar ou esconder no gráfico";
    }
    const has = c && c.measuredMinutes > 0;
    const [tv, tu] = has ? fmtDose(c.total) : ["—", "µSv"];
    const hours = has ? c.measuredMinutes / 60 : 0;
    const rate = has && hours ? c.total / hours : null;
    // quanto do limite já foi: só o que passou do fundo natural conta (o
    // limite não inclui a radiação natural), cada aparelho com esse nível de fundo
    const excess = has ? Math.max(0, c.excess) : 0;
    const pct = has && c.allowance > 0 ? (excess / c.allowance) * 100 : 0;
    const src = !has ? "sem dados no período" : d.key === "rc" ? (c.doseSource === "aparelho" ? "medida pelo aparelho no período" : "calculada pelo app") : "calculada pelo app";
    el.innerHTML = '<span class="dd-name"><i aria-hidden="true"></i><span></span></span><span class="dd-val mono"><b></b> <span></span></span><span class="dd-sub small"></span><span class="meter" aria-hidden="true"><span></span></span><span class="dd-pct small"></span>';
    el.querySelector(".dd-name span").textContent = d.name;
    el.querySelector(".dd-val b").textContent = tv;
    el.querySelector(".dd-val > span").textContent = tu;
    el.querySelector(".dd-sub").textContent = rate == null ? src : `${fmt(rate, usvDigits(rate))} µSv/h em média · ${src}`;
    el.querySelector(".meter span").style.width = Math.min(100, pct) + "%";
    el.querySelector(".meter").classList.toggle("over", pct > 100);
    el.querySelector(".dd-pct").textContent = has && c.allowance > 0
      ? `${fmt(pct, pct < 10 ? 1 : 0)}% do limite · ${doseTxt(excess)} acima do fundo, de ${doseTxt(c.allowance)} permitidos (${(c.limitName || "").replace(/^\p{Lu}(?!\p{Lu})/u, (m) => m.toLowerCase())})`
      : "";
    if (list.length > 1) {
      const s = document.createElement("span");
      s.className = "dd-state";
      s.textContent = state.doseShow[d.key] ? "no gráfico · clique para esconder" : "escondido do gráfico · clique para mostrar";
      el.appendChild(s);
    }
    box.appendChild(el);
  }
}

// aparelhos à vista nos gráficos de dose (clicar no quadro do aparelho liga
// e desliga); pelo menos um fica sempre visível
const devKey = (c) => (c && c.device === "radiacode" ? "rc" : "gmc");
const devName = (k) => (k === "rc" ? "RC102" : "GMC-800");
const devColor = (k) => (k === "rc" ? COR.rc : COR.accent);
function toggleDoseDev(k) {
  const s = state.doseShow;
  s[k] = !s[k];
  if (!s.gmc && !s.rc) s[k === "rc" ? "gmc" : "rc"] = true; // nunca os dois apagados
  localSet("doseShow", JSON.stringify(s));
  document.querySelectorAll("button.dd-box").forEach((b) => {
    const on = !!s[b.dataset.key];
    b.classList.toggle("off", !on);
    b.setAttribute("aria-pressed", String(on));
    const st = b.querySelector(".dd-state");
    if (st) st.textContent = on ? "no gráfico · clique para esconder" : "escondido do gráfico · clique para mostrar";
  });
  drawDoseChart();
  if (state.heatPick && state.ht) drawHeatTrecho();
}
document.addEventListener("click", (e) => { const b = e.target.closest(".dd-box"); if (b) toggleDoseDev(b.dataset.key); });

function drawDoseChart() {
  const c = state.dose, c2raw = state.dose2;
  if (!c || !c.times || !c.times.length) return;
  const k1 = devKey(c), c2 = c2raw && c2raw.measuredMinutes ? c2raw : null, k2 = c2 ? devKey(c2) : null;
  // cada um com o seu teto; o do principal é a linha cheia, o do outro
  // pontilhado (se o principal estiver escondido, o do outro vira cheio)
  const show1 = !!(c.measuredMinutes && (state.doseShow[k1] || !c2)), show2 = !!(c2 && state.doseShow[k2]);
  const n = c.times.length;
  const vis = [show1 ? Math.max(c.total, c.ceilingTotal) : 0, show2 ? Math.max(c2.total, c2.ceilingTotal) : 0];
  const top0 = Math.max(...vis) * 1.1 || 1;
  const solid2 = show2 && !show1;
  $("doseKeyCeil").textContent = show1 && c2 ? `teto do ${devName(k1)} (fundo + limite)` : solid2 ? `teto do ${devName(k2)} (fundo + limite)` : "teto (fundo + limite)";
  $("doseKeyCeil2").hidden = !(show1 && show2);
  $("doseKeyCeil2Txt").textContent = show1 && show2 ? `teto do ${devName(k2)}` : "";
  $("doseKeyRc").hidden = !((k1 === "rc" && show1) || (k2 === "rc" && show2));
  document.querySelector("#doseCard .dose-legend .sw-gmc").parentElement.hidden = !((k1 === "gmc" && show1) || (k2 === "gmc" && show2));
  const mag = Math.pow(10, Math.floor(Math.log10(top0)));
  const top = Math.ceil(top0 / mag * 2) / 2 * mag;
  const y = (v) => H - (v / top) * H;
  const path = (arr) => arr.map((v, i) => (i ? "L" : "M") + ((arr.length <= 1 ? 0 : (i / (arr.length - 1)) * W)).toFixed(1) + " " + y(v).toFixed(1)).join("");
  const grid = [0, 1, 2, 3].map((i) => {
    const gy = i === 3 ? H - 0.5 : i === 0 ? 0.5 : (H / 3) * i;
    return `<line x1="0" y1="${gy}" x2="${W}" y2="${gy}" stroke="${COR.grid}" stroke-width="1" vector-effect="non-scaling-stroke"/>`;
  }).join("");
  const dt0 = c.times[0], dt1 = c.times[n - 1];
  const dxt = (t) => Math.max(0, Math.min(W, ((t - dt0) / Math.max(1, dt1 - dt0)) * W));
  const dAway = awayBands(dt0, dt1, dxt);
  $("doseAwayKey").hidden = !dAway;
  const fillOf = (cc, k) => `<path d="${path(cc.dose)}L${W} ${H}L0 ${H}Z" fill="${devColor(k)}" fill-opacity="0.10"/>`;
  const lineOf = (arr, color, w, dash, op) => `<path d="${path(arr)}" fill="none" stroke="${color}" stroke-width="${w}"${dash ? ` stroke-dasharray="${dash}"` : ""}${op ? ` stroke-opacity="${op}"` : ""} stroke-linejoin="round" vector-effect="non-scaling-stroke"/>`;
  $("doseCard").classList.toggle("rc", k1 === "rc");
  $("doseChart").innerHTML = grid + dAway +
    (show1 ? fillOf(c, k1) + lineOf(c.background, COR.muted, 1.5, "5 5") + lineOf(c.ceiling, COR.warn, 2) + lineOf(c.dose, devColor(k1), 2) : "") +
    (show2 ? (solid2 ? fillOf(c2, k2) + lineOf(c2.background, COR.muted, 1.5, "5 5") + lineOf(c2.ceiling, COR.warn, 2) : lineOf(c2.ceiling, COR.warn, 1.5, "2 4", ".75")) + lineOf(c2.dose, devColor(k2), 2) : "");
  const ys = $("doseY").children;
  for (let i = 0; i < 3; i++) ys[i].textContent = fmtDose(top - (top / 3) * i)[0];
  ys[3].textContent = "0";
}

async function refreshDose() {
  $("doseLimit").value = state.doseLimit;
  const my = ++doseSeq;
  const rcOn0 = hasRcData();
  // o teto e o fundo seguem o Radiacode quando ele tem dado (a sua dose); a
  // curva do outro aparelho vai junto, no mesmo eixo
  const src = rcOn0 ? "radiacode" : "gmc";
  const [cA, cB] = await Promise.all([fetchDose(src, state.doseZoom), rcOn0 ? fetchDose("gmc", state.doseZoom).catch(() => null) : null]);
  if (my !== doseSeq) return;
  // trecho sem Radiacode (ex.: antes de ele existir): o GMC vira o principal
  // (fundo, teto e eixo), senão o gráfico ficava vazio com o GMC tendo dado
  const swap = rcOn0 && !(cA && cA.measuredMinutes) && cB && cB.measuredMinutes;
  const c = swap ? cB : cA, c2 = swap ? cA : cB;
  state.dose = c;
  state.dose2 = c2;
  renderDevBoxes($("doseDevs"), rcOn0 ? [{ key: "gmc", name: "GMC-800", c: swap ? c : c2 }, { key: "rc", name: "RC102", c: swap ? c2 : c }] : [{ key: "gmc", name: "GMC-800", c }]);
  placeRcDevLines();
  $("doseKeyRc").hidden = !rcOn0;
  const dz = $("doseZoomBar");
  dz.hidden = !state.doseZoom;
  if (state.doseZoom) {
    $("doseZoomRange").textContent = zoomRangeText(c.from, c.to);
    $("doseZoomStats").textContent = state.zoomLinked
      ? `· mesmo trecho do gráfico de CPM · dose só nesse trecho, começando do zero`
      : `· dose acumulada só nesse trecho, começando do zero · um ponto a cada ${stepText(c.stepMs)}`;
  }
  renderZoomButtons();

  const isRc = c.device === "radiacode";
  const rcOn = hasRcData();
  $("doseTitle").textContent = "Dose acumulada";
  const otherName = isRc ? "GMC-800 (casa)" : "RC102 (você)";
  renderDoseDevice();
  $("doseOther").hidden = true;
  $("doseOther").textContent = c.otherMinutes > 0 ? `${otherName} no mesmo período: ${doseTxt(c.otherTotal)} em ${fmtDur(c.otherMinutes * 60e3)} medidos` : "";
  $("doseKeyTxt").textContent = c.doseSource === "aparelho" ? "dose do aparelho" : "dose calculada pelo app";
  $("doseNote").textContent =
    "Os limites da CNEN e da ICRP não contam a radiação natural nem exames médicos; por isso o teto é a radiação de fundo medida mais o limite anual, proporcional ao tempo medido. " +
    (isRc
      ? (c.doseSource === "aparelho"
        ? "A curva é a dose que o próprio RC102 soma: ele continua contando desconectado, e o que somou longe do PC aparece quando ele volta (no gráfico, distribuído pelo tempo em que esteve fora). O fundo e o teto crescem com o tempo em que ele contou. Ele mede a radiação externa (gama) onde você estiver; radônio inalado e outras fontes internas ficam de fora."
        : "O RC102 mede a radiação externa (gama) onde você estiver, em casa ou fora; radônio inalado e outras fontes internas ficam de fora. Minutos sem leitura (desconectado do PC) não entram em nenhuma das linhas.")
      : "O contador só mede radiação externa (gama e beta): radônio inalado e outras fontes internas ficam de fora. Minutos sem leitura não entram em nenhuma das linhas. " +
        "O GMC não informa a dose que soma na tela: esta é calculada pelo app, minuto a minuto, pela calibração que valia em cada minuto.");
  if (!isRc && c.calNote) {
    const a = document.createElement("span"), b = document.createElement("span");
    a.textContent = " Calibração:";
    b.textContent = " " + c.calNote;
    $("doseNote").append(a, b);
  }

  if (!c.measuredMinutes) {
    $("doseTotal").textContent = "—";
    $("doseCmp").hidden = true;
    $("doseSub").textContent = "sem dados no período";
    $("doseStatus").textContent = "";
    $("doseChart").innerHTML = "";
    ["dx0", "dx1", "dx2"].forEach((id) => { $(id).textContent = ""; });
    renderDoseDevice();
    return;
  }

  const [tv, tu] = fmtDose(c.total);
  const cmp = doseCompare(c.total);
  $("doseCmpTxt").textContent = cmp;
  $("doseCmp").hidden = !cmp;
  $("doseTotal").textContent = tv;
  $("doseTotal").nextElementSibling.textContent = tu;
  renderDoseDevice();
  const cov = c.coverage * 100;
  const perYear = c.backgroundRate * 8766 / 1000; // mSv por ano no ritmo do fundo
  $("doseSub").textContent = `fundo ${fmt(c.backgroundRate, 3)} µSv/h (${c.backgroundSource}) · ≈ ${fmt(perYear, perYear < 10 ? 2 : 1)} mSv/ano nesse ritmo · ${fmt(cov, cov < 10 ? 1 : 0)}% ` + (c.histFrom ? `medido desde o primeiro dado (${dayLabel(c.histFrom)})` : "do período medido");

  const st = $("doseStatus");
  const excess = Math.max(0, c.excess);
  const pct = c.allowance > 0 ? (excess / c.allowance) * 100 : 0;
  const over = c.excess > c.allowance;
  st.className = "dose-status" + (over ? " over" : "");
  let line;
  if (c.excess <= 0) line = "<strong>Dentro do teto.</strong> A dose ficou no nível da radiação de fundo natural.";
  else if (!over) line = `<strong>Dentro do teto.</strong> Acima do fundo: ${doseTxt(excess)} de ${doseTxt(c.allowance)} permitidos no período (${fmt(pct, pct < 10 ? 1 : 0)}%).`;
  else line = `<strong>Acima do teto.</strong> Acima do fundo: ${doseTxt(excess)}; o permitido no período era ${doseTxt(c.allowance)}.`;
  st.innerHTML = `<span>${line}</span>`;

  drawDoseChart();

  if (state.doseZoom) {
    const f = chartTimeFmt(c);
    $("dx0").textContent = f(c.from);
    $("dx1").textContent = f((c.from + c.to) / 2);
    $("dx2").textContent = f(c.to);
    return;
  }
  const hourly = HOURLY.includes(state.dosePeriod);
  const lbl = hourly ? hhmm : state.dosePeriod === "1y"
    ? (ms) => { const t = new Date(ms); return pad(t.getMonth() + 1) + "/" + String(t.getFullYear()).slice(2); }
    : ddmm;
  $("dx0").textContent = lbl(c.from);
  $("dx1").textContent = lbl((c.from + c.to) / 2);
  $("dx2").textContent = hourly ? "agora" : "hoje";
}

function setupDoseHover() {
  const plot = $("dosePlot");
  plot.addEventListener("mousemove", (ev) => {
    const c = state.dose;
    if (!c || !c.times || !c.times.length || !c.measuredMinutes) return;
    const r = plot.getBoundingClientRect();
    const f = Math.min(1, Math.max(0, (ev.clientX - r.left) / r.width));
    const i = Math.round(f * (c.times.length - 1));
    const t = c.times[i];
    const cur = $("doseCursor"), tip = $("doseTip");
    cur.hidden = false;
    cur.style.left = (i / Math.max(1, c.times.length - 1)) * 100 + "%";
    tip.hidden = false;
    const when = HOURLY.includes(state.dosePeriod) && !state.doseZoom ? hhmm(t) : `${ddmm(t)} ${hhmm(t)}`;
    const c2 = state.dose2 && state.dose2.measuredMinutes && state.doseShow[devKey(state.dose2)] ? state.dose2 : null, j2 = c2 && c2.dose && c2.dose.length ? Math.round(f * (c2.dose.length - 1)) : -1;
    if (!state.doseShow[devKey(c)] && c2 && j2 >= 0) {
      tip.textContent = `${when} · ${devName(devKey(c2))} ${doseTxt(c2.dose[j2])} · teto ${doseTxt(c2.ceiling[j2])}`;
    } else tip.textContent = c2 && j2 >= 0
      ? (c.device === "radiacode"
        ? `${when} · RC102 ${doseTxt(c.dose[i])} · GMC-800 ${doseTxt(c2.dose[j2])} · teto ${doseTxt(c.ceiling[i])}`
        : `${when} · GMC-800 ${doseTxt(c.dose[i])} · RC102 ${doseTxt(c2.dose[j2])} · teto ${doseTxt(c.ceiling[i])}`)
      : `${when} · dose ${doseTxt(c.dose[i])} · teto ${doseTxt(c.ceiling[i])}`;
    const left = (ev.clientX - r.left) + 12;
    tip.style.left = Math.max(0, Math.min(left, r.width - tip.offsetWidth - 4)) + "px";
  });
  plot.addEventListener("mouseleave", () => { $("doseCursor").hidden = true; $("doseTip").hidden = true; });
}

function wireDose() {
  $("doseLimit").addEventListener("change", () => {
    state.doseLimit = $("doseLimit").value; localSet("doseLimit", state.doseLimit); refreshDose(); agAt = 0; // o teto da Minha dose muda junto
  });
  setupDoseHover();
  setupDoseZoom();
}
