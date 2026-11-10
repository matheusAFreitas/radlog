// Página Resumo: o resumo de um período, a comparação de dois e o que fica
// em "detalhes" (lacunas, anotações, histórico disponível). Saiu do app.js na
// 2.19, sem mudar nada na tela.
//
// Usa os globais do app.js (state, api, $, fmt, hhmm, DAY_MS, dayLabel...).
// Este arquivo é carregado antes do app.js e nada aqui roda na carga: só
// funções e constantes que as funções usam depois.

// ---------- cobertura, lacunas e anotações ----------

// cobertura com o denominador (2.3): "n de N min", até agora; se o período
// começa antes do primeiro dado, conta desde ele e mostra também o período
// inteiro
function covText(r) {
  const hist = r.histMinutes != null ? r.histMinutes : r.spanMinutes;
  if (!hist) return `${hm(r.minutes)} com leitura`;
  let s = `${fmt(r.minutes, 0)} de ${fmt(hist, 0)} min` + (r.ongoing ? " até agora" : "");
  if (r.spanMinutes > hist && r.histFrom) {
    s += ` · desde o primeiro dado (${dayLabel(r.histFrom)} ${hhmm(r.histFrom)})`;
    s += ` · ${fmt((r.coverageAll || 0) * 100, 0)}% do período inteiro`;
  }
  return s;
}

// lacunas do período no Resumo (clique abre o trecho no gráfico)
function renderSumGaps(r) {
  const box = $("sumGaps");
  const list = (r && r.gaps) || [];
  const pre = r && r.preMinutes > 0 && r.histMinutes > 0;
  box.hidden = !list.length && !pre;
  box.textContent = "";
  if (!list.length && !pre) return;
  const h = document.createElement("h3");
  h.textContent = `Lacunas de 10 min ou mais (${r.gapCount}` + (r.gapMinutes ? ` · ${fmtDur(r.gapMinutes * 6e4)} sem leitura` + (pre ? " desde o primeiro dado" : " no período") : "") + ")";
  box.appendChild(h);
  if (pre) {
    const p = document.createElement("p");
    p.className = "small muted";
    p.textContent = `O período começa ${fmtDur(r.preMinutes * 6e4)} antes do primeiro dado do GMC (${dayLabel(r.histFrom)} ${hhmm(r.histFrom)}); esse trecho não conta como lacuna.`;
    box.appendChild(p);
  }
  const long = r.to - r.from > DAY_MS * 1.1;
  const at = (t) => (long ? `${dayLabel(t)} ${hhmm(t)}` : hhmm(t));
  for (const g of list) {
    const b = document.createElement("button");
    b.type = "button";
    b.className = "sum-note";
    b.innerHTML = '<span class="mono muted"></span><span></span>';
    b.children[0].textContent = `${at(g.from)} → ${g.open ? "agora" : long && dayLabel(g.to) !== dayLabel(g.from) ? at(g.to) : hhmm(g.to)}`;
    b.children[1].textContent = `${hm(g.minutes)} · ${g.cause}`;
    b.title = "Ver o trecho no gráfico";
    b.addEventListener("click", () => openGapOnChart(g));
    box.appendChild(b);
  }
  if (r.gapCount > list.length) {
    const more = document.createElement("p");
    more.className = "small muted";
    more.textContent = `Mostrando as ${list.length} mais recentes.`;
    box.appendChild(more);
  }
}

async function openGapOnChart(g) {
  const pad = Math.max(10 * 6e4, (g.to - g.from) * 0.3);
  const from = g.from - pad, to = Math.min(Date.now(), g.to + pad);
  await showView("graficos");
  state.zoom = { from, to };
  state.zoomStack = [];
  state.doseZoom = { from, to };
  state.doseZoomStack = [];
  state.zoomLinked = true;
  refreshChart();
  refreshDose();
}

async function renderSumNotes(from, to) {
  const box = $("sumNotes");
  let list = [];
  try { list = (await api.GetNotes(from, to)) || []; } catch (e) { list = []; }
  box.hidden = !list.length;
  box.textContent = "";
  if (!list.length) return;
  const h = document.createElement("h3");
  h.textContent = "Anotações";
  box.appendChild(h);
  const long = to - from > DAY_MS * 1.1;
  for (const n of list) {
    const b = document.createElement("button");
    b.type = "button";
    b.className = "sum-note";
    b.innerHTML = '<span class="mono muted"></span><span></span>';
    const at = (t) => (long ? `${dayLabel(t)} ${hhmm(t)}` : hhmm(t));
    b.children[0].textContent = n.end ? `${at(n.time)}–${long && dayLabel(n.end) !== dayLabel(n.time) ? at(n.end) : hhmm(n.end)}` : at(n.time);
    b.children[1].textContent = n.text;
    b.title = "Editar";
    b.addEventListener("click", () => openNote(n));
    box.appendChild(b);
  }
}

// ---------- resumo e comparação ----------

// períodos [de, até, rótulo] conforme o que está escolhido
function sumPeriods() {
  const now = Date.now(), s = state.sum, d0 = startDay(now), w0 = startWeek(now);
  if (s.mode === "resumo") {
    if (s.day) return [[s.day, addDays(s.day, 1), dayLabel(s.day)]];
    return [{
      hoje: [d0, now, "hoje"],
      ontem: [addDays(d0, -1), d0, "ontem"],
      semana: [w0, now, "esta semana"],
      "semana-passada": [addDays(w0, -7), w0, "semana passada"],
      mes: [startMonth(now, 0), now, "este mês"],
      "mes-passado": [startMonth(now, -1), startMonth(now, 0), "mês passado"],
    }[s.preset]];
  }
  if (s.cmp === "7d") return [[now - 7 * DAY_MS, now, "últimos 7 dias"], [now - 14 * DAY_MS, now - 7 * DAY_MS, "7 dias antes"]];
  if (s.cmp === "dias") {
    const a = s.dayA || addDays(d0, -1), b = s.dayB || addDays(d0, -2);
    return [[a, addDays(a, 1), dayLabel(a)], [b, addDays(b, 1), dayLabel(b)]];
  }
  // hoje até agora × ontem até a mesma hora (comparação justa)
  return [[d0, now, "hoje (até agora)"], [addDays(d0, -1), addDays(d0, -1) + (now - d0), "ontem (até a mesma hora)"]];
}

let sumSeq = 0;
async function refreshSummary() {
  if (state.view !== "resumo") return;
  const s = state.sum;
  document.querySelectorAll("#sumModeSeg button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.smode === s.mode)));
  document.querySelectorAll("#sumPresetSeg button").forEach((b) => b.setAttribute("aria-pressed", String(s.mode === "resumo" && !s.day && b.dataset.sp === s.preset)));
  document.querySelectorAll("#sumCmpSeg button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.sc === s.cmp)));
  $("sumPresetSeg").hidden = s.mode !== "resumo";
  $("sumDay").parentElement.hidden = s.mode !== "resumo";
  $("sumCmpSeg").hidden = s.mode !== "comparar";
  $("btnReport").hidden = s.mode !== "resumo";
  $("sumDays").hidden = !(s.mode === "comparar" && s.cmp === "dias");
  $("sumSay").hidden = $("sumTop").hidden = s.mode !== "resumo";
  $("sumDay").value = s.day ? dateVal(s.day) : "";
  const periods = sumPeriods();
  if (s.cmp === "dias") { $("sumDayA").value = dateVal(periods[0][0]); $("sumDayB").value = dateVal(periods[1][0]); }
  const my = ++sumSeq;
  let res;
  try { res = await Promise.all(periods.map((p) => api.GetSummary(p[0], p[1]))); } catch (e) { toast(errMsg(e), true); return; }
  if (my !== sumSeq) return;
  state.sumRes = { periods, res };
  if (s.mode === "resumo") renderSummary(res[0], periods[0]);
  else renderCompare(res, periods);
  renderSumGaps(s.mode === "resumo" ? res[0] : null);
  renderSumNotes(periods[0][0], periods[0][1]);
  renderMaturity();
}

// maturidade do histórico (2.3): para cada janela, se já existe histórico
// para ela inteira e quanto foi medido
const MAT_STATE = { disponivel: "disponível", formacao: "em formação", aguardando: "aguardando dados" };
async function renderMaturity() {
  const box = $("sumMature");
  if (!api.GetMaturity) { box.hidden = true; return; }
  let m;
  try { m = await api.GetMaturity(); } catch (e) { box.hidden = true; return; }
  const winTxt = (d) => (d === 1 ? "24 h" : d === 365 ? "1 ano" : `${d} dias`);
  const cell = (w) => {
    if (!w || w.state === "aguardando") return `<td class="mat aguardando">${MAT_STATE.aguardando}</td>`;
    const st = w.state === "formacao" ? `em formação (${fmt(w.histDays, w.histDays < 10 ? 1 : 0)} de ${w.days} dias)` : MAT_STATE.disponivel;
    return `<td class="mat ${w.state}"><span>${st}</span><br><span class="muted">${fmt(w.coverage * 100, 0)}% medido · ${fmt(w.minutes, 0)} de ${fmt(w.histMinutes, 0)} min</span></td>`;
  };
  const rc = m.rc && m.rc.length;
  let html = `<h3>Histórico disponível</h3><p class="small muted">Para cada janela (até agora): se o histórico já cobre a janela inteira e quanto dela foi medido, contando desde o primeiro dado do aparelho. Médias de janelas "em formação" ainda não representam a janela toda.</p>`;
  html += `<table class="mat-table"><thead><tr><th>janela</th><th>GMC-800</th>${rc ? "<th>RC102</th>" : ""}</tr></thead><tbody>`;
  m.gmc.forEach((w, i) => { html += `<tr><th>${winTxt(w.days)}</th>${cell(w)}${rc ? cell(m.rc[i]) : ""}</tr>`; });
  html += "</tbody></table>";
  const first = [m.gmcFirst ? `GMC desde ${dayLabel(m.gmcFirst)} ${hhmm(m.gmcFirst)}` : "", m.rcFirst ? `RC102 desde ${dayLabel(m.rcFirst)} ${hhmm(m.rcFirst)}` : ""].filter(Boolean).join(" · ");
  if (first) html += `<p class="small muted">${first}</p>`;
  box.innerHTML = html;
  box.hidden = false;
}

function bgVerdict(r) {
  if (!r.bgOK || r.avg == null) return "fundo ainda não calculado";
  const p = r.vsBgPct;
  const bg = r.bgUSv ? usvVal(r.bgUSv) : `${fmt(r.bgCPM, 0)} CPM`;
  if (Math.abs(p) < 5) return `igual ao seu normal (${bg})`;
  return `${fmt(Math.abs(p), 0)}% ${p > 0 ? "acima" : "abaixo"} do seu normal (${bg})`;
}
const alertsText = (r) => {
  const parts = [];
  if (r.danger) parts.push(`${r.danger} perigo`);
  if (r.alarm) parts.push(`${r.alarm} alarme${r.alarm > 1 ? "s" : ""}`);
  if (r.attention) parts.push(`${r.attention} atenção`);
  return parts.length ? parts.join(" · ") : "nenhum";
};
const rainText = (r) => (!r.rainKnown ? "sem dado" : r.rainHours ? `${fmt(r.rainMM, 1)} mm` : "sem chuva");
const atText = (r, t) => (r.to - r.from > DAY_MS * 1.1 ? `${dayLabel(t)} ${hhmm(t)}` : `às ${t % 60000 ? hhmmss(t) : hhmm(t)}`);

function sumTile(box, label, value, sub, cls) {
  const d = document.createElement("div");
  d.className = "stat ri-tile" + (cls ? " " + cls : "");
  d.innerHTML = '<span class="label"></span><span class="value mono"></span><span class="sub small muted"></span>';
  d.children[0].textContent = label; d.children[1].textContent = value; d.children[2].textContent = sub || "";
  box.appendChild(d);
}

function sumHead(box, text, cls) {
  const h = document.createElement("h3");
  h.className = "sum-head " + (cls || "");
  h.textContent = text;
  box.appendChild(h);
}

// bloco do Radiacode (seu dosímetro): dose, média, máximo e mínimo, e o tempo
// em casa e fora
function renderRcSummary(t, rc, r) {
  const d = (v) => fmt(v, usvDigits(v));
  const dose = (v) => `${fmt(v, v < 10 ? 2 : 1)} µSv`;
  sumHead(t, "Radiacode 102", "rc");
  if (rc.devOK) sumTile(t, "Dose do aparelho", dose(rc.devDose), `somada pelo RC102, também longe do PC · pelo app: ${dose(rc.dose)}`, "rc");
  else sumTile(t, "Dose (app)", dose(rc.dose), `≈ ${d(rc.avg)} µSv/h em média · só os minutos gravados`, "rc");
  const p = rc.vsBgPct;
  const vs = !rc.bgUSv ? "fundo em casa ainda não calculado" : Math.abs(p) < 5 ? `igual ao seu normal em casa (${d(rc.bgUSv)})` : `${fmt(Math.abs(p), 0)}% ${p > 0 ? "acima" : "abaixo"} do seu normal em casa (${d(rc.bgUSv)})`;
  sumTile(t, "Média", `${d(rc.avg)} µSv/h`, vs, rc.bgUSv && p >= 20 ? "warn rc" : "rc");
  sumTile(t, "Máximo", `${d(rc.max)} µSv/h`, atText(r, rc.maxAt), "rc");
  sumTile(t, "Mínimo", `${d(rc.min)} µSv/h`, atText(r, rc.minAt), "rc");
  sumTile(t, "Em casa", dose(rc.homeDose), `${hm(rc.homeMinutes)} medidos`, "rc");
  sumTile(t, "Fora de casa", rc.awayMinutes ? dose(rc.awayDose) : "—",
    rc.awayMinutes ? `${hm(rc.awayMinutes)} · média ${d(rc.awayAvg)} · máx ${d(rc.awayMax)} µSv/h` : "nenhum minuto fora de casa", "rc");
  sumTile(t, "Medido", `${fmt(rc.coverage * 100, 0)}%`, covText({ ...rc, ongoing: r.ongoing }), "rc");
  sumTile(t, "Alertas", alertsText(rc), rc.peakAlert ? `pico ao vivo ${d(rc.peakAlert)} µSv/h` : "alarmes 1 e 2 do aparelho", rc.danger || rc.alarm ? "warn rc" : "rc");
}

// ---------- Resumo repaginado (2.18): frase, três números, casa × pessoal ----------

const sumUSv = (v) => { const [n, u] = agSv(v); return `${n} ${u}`; };
// lead vai junto na mesma frase para a tradução achar o texto inteiro
const sumPct = (p, lead) => (lead ? lead + " " : "") + (Math.abs(p) < 5 ? "igual ao normal" : p > 0 ? `${fmt(p, 0)}% acima do normal` : `${fmt(-p, 0)}% abaixo do normal`);

// a frase do topo: o período foi normal?
function renderSumSay(r, rc) {
  const c = (k) => (r[k] || 0) + ((rc && rc[k]) || 0);
  const n = { danger: c("danger"), alarm: c("alarm"), attention: c("attention") };
  const gAbove = r.minutes && r.bgOK && r.vsBgPct >= 20, rAbove = rc && rc.bgUSv && rc.vsBgPct >= 20;
  let lv = "seguro", title;
  const parts = [];
  if (!r.minutes && !rc) {
    lv = "off";
    title = "Sem leituras nesse período.";
    parts.push("Escolha outro período ou confira se os aparelhos estão gravando.");
  } else {
    if (n.danger) { lv = "perigo"; title = "Teve perigo no período."; }
    else if (n.alarm) { lv = "alarme"; title = "Teve alarme no período."; }
    else if (gAbove || rAbove) { lv = "atencao"; title = "Fundo acima do normal no período."; }
    else if (n.attention) { lv = "atencao"; title = "Período normal, com alertas de atenção."; }
    else title = "Período normal.";
    if (r.minutes && r.bgOK) parts.push(sumPct(r.vsBgPct, "Casa"));
    if (rc && rc.bgUSv) parts.push(sumPct(rc.vsBgPct, "RC102"));
    const cov = rc ? rc.coverage : r.coverage;
    parts.push(`${fmt(cov * 100, 0)}% do tempo medido`);
    if (rc && rc.awayMinutes) parts.push(`fora de casa: ${hm(rc.awayMinutes)}`);
  }
  $("sumSay").className = "sum-say ag-say lv-" + lv;
  lvLabel($("sumSayTitle"), lv, title, 20);
  $("sumSayText").textContent = parts.join(" · ");
  const chip = $("sumSayChip");
  chip.hidden = lv === "off";
  chip.textContent = n.danger || n.alarm || n.attention ? alertsText(n) : "nenhum alerta";
}

function sumKpi(box, label, value, sub, cls) {
  const d = document.createElement("div");
  d.className = "sum-kpi" + (cls ? " " + cls : "");
  d.innerHTML = '<span class="k"></span><span class="v"></span><span class="n"></span>';
  d.children[0].textContent = label; d.children[1].textContent = value; d.children[2].textContent = sub || "";
  box.appendChild(d);
}

// os três números: quanto (dose), comparado com o quê (fundo) e o pico
function renderSumBig(r, rc) {
  const box = $("sumBig");
  box.textContent = "";
  if (!r.minutes && !rc) return;
  const d = (v) => fmt(v, usvDigits(v));
  if (rc) sumKpi(box, "Sua dose", sumUSv(rc.devOK ? rc.devDose : rc.dose), rc.devOK ? "RC102 · somada pelo aparelho" : `RC102 · ${fmt(rc.coverage * 100, 0)}% medido`, "rc");
  else sumKpi(box, "Dose da casa", sumUSv(r.dose), `GMC-800 · ${fmt(r.coverage * 100, 0)}% medido`);
  if (r.minutes) {
    sumKpi(box, "Fundo da casa", usvVal(r.avgUSv), "GMC-800 · " + (r.bgOK ? sumPct(r.vsBgPct) : "normal ainda não calculado"), r.bgOK && r.vsBgPct >= 20 ? "up" : "");
  } else if (rc.homeMinutes) {
    const home = (rc.homeDose * 60) / rc.homeMinutes;
    const p = rc.bgUSv ? ((home - rc.bgUSv) / rc.bgUSv) * 100 : 0;
    sumKpi(box, "Fundo da casa", `${d(home)} µSv/h`, "pelo RC102 em casa" + (rc.bgUSv ? " · " + sumPct(p) : ""), rc.bgUSv && p >= 20 ? "up rc" : "rc");
  }
  if (rc) sumKpi(box, "Pico", `${d(rc.max)} µSv/h`, `RC102 · ${atText(r, rc.maxAt)}`, "rc");
  else sumKpi(box, "Pico", r.maxUSv != null ? usvVal(r.maxUSv) : `${fmt(r.max, 0)} CPM`, `GMC-800 · ${atText(r, r.maxAt)}`);
}

// casa × pessoal: quanto da dose do Radiacode foi em casa e fora, e quanto o
// Radiacode marcou em relação ao GMC-800 (que fica parado em casa)
function renderSumSplit(r, rc) {
  const box = $("sumSplit");
  box.hidden = !rc || !(rc.homeMinutes + rc.awayMinutes);
  if (box.hidden) return;
  const tot = rc.homeDose + rc.awayDose || 1, mins = rc.homeMinutes + rc.awayMinutes;
  const hp = (rc.homeDose / tot) * 100, ap = 100 - hp;
  const st = $("sumStack");
  st.textContent = "";
  const seg = (cls, pct, label) => {
    if (pct <= 0) return;
    const e = document.createElement("div");
    e.className = cls;
    e.style.width = pct.toFixed(1) + "%";
    if (pct >= 14) e.textContent = label;
    st.appendChild(e);
  };
  seg("home", hp, `em casa ${fmt(rc.homeDose, rc.homeDose < 10 ? 2 : 1)}`);
  seg("away", rc.awayMinutes ? ap : 0, `fora ${fmt(rc.awayDose, rc.awayDose < 10 ? 2 : 1)}`);
  const parts = [];
  parts.push(rc.awayMinutes
    ? `Fora de casa foi ${fmt((rc.awayMinutes / mins) * 100, 0)}% do tempo e ${fmt(ap, 0)}% da dose (µSv).`
    : "Nenhum minuto fora de casa no período (µSv).");
  if (r.minutes && r.avgUSv > 0 && rc.avg > 0) parts.push(`Razão pessoal ÷ casa: ${fmt(rc.avg / r.avgUSv, 2)} (RC102 ÷ GMC-800 em µSv/h).`);
  $("sumSplitText").textContent = parts.join(" ");
}

function renderSummary(r, per) {
  $("sumSub").textContent = `${per[2]} · ${dayLabel(per[0])} ${hhmm(per[0])} → ${per[1] >= Date.now() - 6e4 ? "agora" : dayLabel(per[1] - 1) + " " + hhmm(per[1])}`;
  const rc0 = r.rc && r.rc.minutes ? r.rc : null;
  renderSumSay(r, rc0);
  renderSumBig(r, rc0);
  renderSumSplit(r, rc0);
  const t = $("sumTiles");
  t.textContent = "";
  $("sumTable").hidden = true;
  const rc = r.rc && r.rc.minutes ? r.rc : null;
  if (rc) renderRcSummary(t, rc, r);
  if (!r.minutes) {
    if (rc) { sumHead(t, "GQ GMC-800 (casa)"); t.insertAdjacentHTML("beforeend", '<p class="small muted sum-none">Sem leituras do GMC nesse período.</p>'); drawSumChart([{ r, color: COR.accent, label: "GMC-800" }]); return; }
    t.innerHTML = '<p class="small muted">Sem leituras nesse período.</p>';
    drawSumChart([]);
    return;
  }
  if (rc) sumHead(t, "GQ GMC-800 (casa)");
  sumTile(t, "Média", usvVal(r.avgUSv), bgVerdict(r), r.bgOK && r.vsBgPct >= 20 ? "warn" : "");
  sumTile(t, "Máximo", r.maxUSv != null ? usvVal(r.maxUSv) : `${fmt(r.max, 0)} CPM`, atText(r, r.maxAt));
  sumTile(t, "Mínimo", r.minUSv != null ? usvVal(r.minUSv) : `${fmt(r.min, 0)} CPM`, atText(r, r.minAt));
  sumTile(t, "Dose (app)", `${fmt(r.dose, r.dose < 10 ? 2 : 1)} µSv`, `≈ ${fmt(r.avgUSv, 3)} µSv/h em média · calculada pelo app`);
  const days = r.spanMinutes > 1440 && r.daysData ? ` · ${r.daysData} ${r.daysData === 1 ? "dia" : "dias"} com dado` : "";
  sumTile(t, "Medido", `${fmt(r.coverage * 100, 0)}%`, covText(r) + days + (r.awayMinutes ? ` · ${hm(r.awayMinutes)} fora de casa` : ""));
  if (r.dispersion != null) sumTile(t, "Dispersão", fmt(r.dispersion, 2), `σ real ${fmt(r.sd, 1)} ÷ acaso ${fmt(Math.sqrt(r.avg), 1)} · ${r.dispersion >= 1.2 ? "variou além do acaso" : "perto de 1: normal"}`, r.dispersion >= 1.2 ? "warn" : "");
  sumTile(t, "Alertas", alertsText(r), r.peakAlert ? `pico ao vivo ${r.peakAlertUSv ? usvVal(r.peakAlertUSv) : fmt(r.peakAlert, 0) + " CPM"}` : "", r.danger || r.alarm ? "warn" : "");
  sumTile(t, "Chuva", rainText(r), r.rainHours ? `${r.rainHours} h com chuva` : "");
  if (r.measurements) sumTile(t, "Medições", String(r.measurements), "de objetos");
  drawSumChart([{ r, color: COR.accent, label: rc ? "GMC-800" : per[2] }]);
}

function renderCompare(res, periods) {
  const [a, b] = res;
  $("sumSub").textContent = `${periods[0][2]} × ${periods[1][2]}`;
  $("sumTiles").textContent = "";
  $("sumBig").textContent = "";
  $("sumSplit").hidden = true;
  const tb = $("sumTable");
  tb.hidden = false;
  const pctTxt = (x, y) => { const p = ((x - y) / y) * 100; return Math.abs(p) < 0.5 ? " (igual)" : ` (${sgn(p, 0)}%)`; };
  const diff = (x, y, d, unit) => (x == null || y == null ? "—" : Math.abs(x - y) < Math.pow(10, -d) / 2 ? "igual" : `${sgn(x - y, d)}${unit}` + (y ? pctTxt(x, y) : ""));
  const rows = [
    ["Média", a.avgUSv, b.avgUSv, 3, " µSv/h"],
    ["Máximo", a.maxUSv, b.maxUSv, 3, " µSv/h"],
    ["Mínimo", a.minUSv, b.minUSv, 3, " µSv/h"],
    ["Dose", a.dose, b.dose, 2, " µSv"],
  ];
  let html = `<thead><tr><th></th><th><i class="sw" style="background:${COR.accent}"></i></th><th><i class="sw" style="background:${COR.muted}"></i></th><th>A − B</th></tr></thead><tbody>`;
  tb.innerHTML = html + "</tbody>";
  const head = tb.querySelectorAll("th");
  head[1].append(" " + periods[0][2]); head[2].append(" " + periods[1][2]);
  const body = tb.querySelector("tbody");
  const row = (cells) => { const tr = document.createElement("tr"); cells.forEach((c, i) => { const td = document.createElement(i ? "td" : "th"); td.textContent = c; tr.appendChild(td); }); body.appendChild(tr); };
  for (const [n, x, y, d, u] of rows) row([n, x == null ? "—" : fmt(x, d) + u, y == null ? "—" : fmt(y, d) + u, diff(x, y, d, u)]);
  if ((a.rc && a.rc.minutes) || (b.rc && b.rc.minutes)) {
    const ra = a.rc || {}, rb = b.rc || {};
    const v = (o, k, dd, u) => (o.minutes ? fmt(o[k], dd) + u : "—");
    const dd = (o, k, d2, u) => (o.minutes && o[k] != null ? o[k] : null);
    row(["RC102: dose", v(ra, "dose", 2, " µSv"), v(rb, "dose", 2, " µSv"), diff(dd(ra, "dose"), dd(rb, "dose"), 2, " µSv")]);
    row(["RC102: média", v(ra, "avg", 3, " µSv/h"), v(rb, "avg", 3, " µSv/h"), diff(dd(ra, "avg"), dd(rb, "avg"), 3, " µSv/h")]);
    row(["RC102: fora de casa", ra.minutes ? `${fmt(ra.awayDose, 2)} µSv em ${hm(ra.awayMinutes)}` : "—", rb.minutes ? `${fmt(rb.awayDose, 2)} µSv em ${hm(rb.awayMinutes)}` : "—", ""]);
  }
  row(["Medido", hm(a.minutes), hm(b.minutes), Math.abs(a.minutes - b.minutes) > 0.1 * Math.max(a.minutes, b.minutes) ? "tempos bem diferentes: compare pela média" : ""]);
  row(["Alertas", alertsText(a), alertsText(b), ""]);
  row(["Chuva", rainText(a), rainText(b), ""]);
  drawSumChart([{ r: a, color: COR.accent, label: periods[0][2] }, { r: b, color: COR.muted, label: periods[1][2] }]);
}

function drawSumChart(series) {
  state.sumSeries = series;
  const svg = $("sumChart");
  if (!series.length) { svg.innerHTML = ""; $("sumLegend").textContent = ""; return; }
  const rc0 = series.length === 1 && series[0].r.rc && series[0].r.rc.minutes ? series[0].r.rc : null;
  if (rc0) return drawSumChartUSv(series[0].r, rc0);
  const n = Math.max(...series.map((s) => s.r.curve.length));
  // GMC em µSv/h (2.29), cada ponto pela calibração do seu minuto
  const k = calK();
  const pu = (p) => (p.usv != null ? p.usv : p.cpm != null ? p.cpm * k : null);
  let top = 0.05;
  for (const s of series) for (const p of s.r.curve) if (pu(p) != null) top = Math.max(top, pu(p));
  top = niceTop(top * 1.15);
  const x = (i) => (n <= 1 ? 0 : (i / (n - 1)) * W);
  const y = (v) => H - (Math.min(top, v) / top) * H;
  const ys = $("sumY").children;
  for (let i = 0; i < 4; i++) ys[i].textContent = fmt(top - (top / 3) * i, i === 3 ? 0 : top < 0.3 ? 3 : 2);
  let out = [0, 1, 2, 3].map((i) => { const gy = i === 3 ? H - 0.5 : i === 0 ? 0.5 : (H / 3) * i; return `<line x1="0" y1="${gy}" x2="${W}" y2="${gy}" stroke="${COR.grid}" stroke-width="1" vector-effect="non-scaling-stroke"/>`; }).join("");
  const bg = series[0].r.bgOK ? series[0].r.bgUSv || series[0].r.bgCPM * k : 0;
  if (bg) out += `<line x1="0" y1="${y(bg).toFixed(1)}" x2="${W}" y2="${y(bg).toFixed(1)}" stroke="${COR.accent}" stroke-opacity="0.35" stroke-dasharray="4 4" vector-effect="non-scaling-stroke"/>`;
  for (const s of series.slice().reverse()) {
    let d = "", pen = "M";
    s.r.curve.forEach((p, i) => { const v = pu(p); if (v == null) { pen = "M"; return; } d += `${pen}${x(i).toFixed(1)} ${y(v).toFixed(1)}`; pen = "L"; });
    out += `<path d="${d}" fill="none" stroke="${s.color}" stroke-width="${s === series[0] ? 2.2 : 1.6}" stroke-linejoin="round" vector-effect="non-scaling-stroke"/>`;
  }
  svg.innerHTML = out;
  const c0 = series[0].r.curve, long = series[0].r.to - series[0].r.from > DAY_MS * 1.1;
  const lab = (t) => (long ? `${WEEKDAY[new Date(t).getDay()]} ${ddmm(t)}` : hhmm(t));
  const xs = $("sumX").children;
  xs[0].textContent = lab(c0[0].t); xs[1].textContent = lab(c0[Math.floor(c0.length / 2)].t); xs[2].textContent = lab(series[0].r.to);
  $("sumLegend").innerHTML = series.map((s) => `<span class="key"><i style="border-color:${s.color}"></i></span>`).join("") + (bg ? '<span class="key"><i style="border-color:' + COR.accent + ';border-top-style:dashed;opacity:.5"></i>seu normal</span>' : "");
  series.forEach((s, i) => $("sumLegend").children[i].append(s.label));
}

// resumo com o Radiacode: as duas curvas em µSv/h (o GMC pela calibração dele)
function drawSumChartUSv(r, rc) {
  const svg = $("sumChart");
  const k = calK();
  const n = Math.max(r.curve.length, rc.curve.length);
  let top = 0.05;
  for (const p of rc.curve) if (p.usv != null) top = Math.max(top, p.usv);
  for (const p of r.curve) if (p.cpm != null) top = Math.max(top, p.usv != null ? p.usv : p.cpm * k);
  top = niceTop(top * 1.15);
  const x = (i) => (n <= 1 ? 0 : (i / (n - 1)) * W);
  const y = (v) => H - (Math.min(top, v) / top) * H;
  const ys = $("sumY").children;
  for (let i = 0; i < 4; i++) ys[i].textContent = fmt(top - (top / 3) * i, i === 3 ? 0 : top < 0.3 ? 3 : 2);
  let out = [0, 1, 2, 3].map((i) => { const gy = i === 3 ? H - 0.5 : i === 0 ? 0.5 : (H / 3) * i; return `<line x1="0" y1="${gy}" x2="${W}" y2="${gy}" stroke="${COR.grid}" stroke-width="1" vector-effect="non-scaling-stroke"/>`; }).join("");
  if (rc.bgUSv) out += `<line x1="0" y1="${y(rc.bgUSv).toFixed(1)}" x2="${W}" y2="${y(rc.bgUSv).toFixed(1)}" stroke="${COR.rc}" stroke-opacity="0.45" stroke-dasharray="4 4" vector-effect="non-scaling-stroke"/>`;
  const path = (pts, val) => { let d = "", pen = "M"; pts.forEach((p, i) => { const v = val(p); if (v == null) { pen = "M"; return; } d += `${pen}${x(i).toFixed(1)} ${y(v).toFixed(1)}`; pen = "L"; }); return d; };
  out += `<path d="${path(r.curve, (p) => (p.usv != null ? p.usv : p.cpm == null ? null : p.cpm * k))}" fill="none" stroke="${COR.accent}" stroke-opacity="0.8" stroke-width="1.4" stroke-linejoin="round" vector-effect="non-scaling-stroke"/>`;
  out += `<path d="${path(rc.curve, (p) => p.usv)}" fill="none" stroke="${COR.rc}" stroke-width="2.2" stroke-linejoin="round" vector-effect="non-scaling-stroke"/>`;
  svg.innerHTML = out;
  const c0 = rc.curve.length >= r.curve.length ? rc.curve : r.curve, long = r.to - r.from > DAY_MS * 1.1;
  const lab = (t) => (long ? `${WEEKDAY[new Date(t).getDay()]} ${ddmm(t)}` : hhmm(t));
  const xs = $("sumX").children;
  xs[0].textContent = lab(c0[0].t); xs[1].textContent = lab(c0[Math.floor(c0.length / 2)].t); xs[2].textContent = lab(r.to);
  $("sumLegend").innerHTML = '<span class="key"><i style="border-color:' + COR.rc + '"></i>RC102 (µSv/h)</span><span class="key"><i style="border-color:' + COR.accent + '"></i>GMC-800 (µSv/h)</span>' +
    (rc.bgUSv ? '<span class="key"><i style="border-color:' + COR.rc + ';border-top-style:dashed;opacity:.6"></i>seu normal em casa (RC102)</span>' : "");
}
