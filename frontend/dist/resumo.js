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

// período do Resumo (2.64): {tipo: dia | semana | mes, âncora}. A âncora é o
// começo do período; 0 é o período atual.
function sumStart(kind, t) {
  return kind === "semana" ? startWeek(t) : kind === "mes" ? startMonth(t, 0) : startDay(t);
}
const sumNext = (kind, t0) => (kind === "semana" ? addDays(t0, 7) : kind === "mes" ? startMonth(t0, 1) : addDays(t0, 1));
const sumPrev = (kind, t0) => (kind === "semana" ? addDays(t0, -7) : kind === "mes" ? startMonth(t0, -1) : addDays(t0, -1));

function sumStep(dir) {
  const s = state.sum, cur = sumStart(s.kind, Date.now());
  const t0 = s.anchor || cur;
  const t = dir < 0 ? sumPrev(s.kind, t0) : sumNext(s.kind, t0);
  if (t > cur) return;
  s.anchor = t >= cur ? 0 : t;
  refreshSummary();
}

// [rótulo da tela, rótulo do PDF] do período: "Hoje, qua 11/11", "Esta
// semana, 09–15/11", "Novembro"
function sumLabels(kind, t0) {
  const now = Date.now(), wd = (t) => new Date(t).toLocaleDateString(LOCALE, { weekday: "short" }).replace(".", "");
  if (kind === "semana") {
    const end = addDays(t0, 6), cur = t0 === startWeek(now), last = t0 === addDays(startWeek(now), -7);
    const span = `${pad(new Date(t0).getDate())}${new Date(t0).getMonth() === new Date(end).getMonth() ? "" : "/" + pad(new Date(t0).getMonth() + 1)}–${ddmm(end)}`;
    return [cur ? `Esta semana, ${span}` : last ? `Semana passada, ${span}` : span, cur ? "esta semana" : last ? "semana passada" : `semana de ${ddmm(t0)}`];
  }
  if (kind === "mes") {
    const d = new Date(t0), m = d.toLocaleDateString(LOCALE, { month: "long" });
    const txt = m.charAt(0).toUpperCase() + m.slice(1) + (d.getFullYear() === new Date(now).getFullYear() ? "" : " " + d.getFullYear());
    return [txt, t0 === startMonth(now, 0) ? "este mês" : t0 === startMonth(now, -1) ? "mês passado" : txt];
  }
  const d0 = startDay(now), dd = `${wd(t0)} ${ddmm(t0)}`;
  if (t0 === d0) return [`Hoje, ${dd}`, "hoje"];
  if (t0 === addDays(d0, -1)) return [`Ontem, ${dd}`, "ontem"];
  return [dd, dayLabel(t0)];
}

// períodos [de, até, rótulo do PDF, rótulo da tela] conforme o que está escolhido
function sumPeriods() {
  const now = Date.now(), s = state.sum, d0 = startDay(now);
  if (s.mode === "resumo") {
    const t0 = s.anchor || sumStart(s.kind, now);
    const [txt, pdf] = sumLabels(s.kind, t0);
    return [[t0, Math.min(now, sumNext(s.kind, t0)), pdf, txt]];
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
  const res0 = s.mode === "resumo";
  document.querySelectorAll("#sumKindSeg button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.sk === s.kind)));
  document.querySelectorAll("#sumCmpSeg button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.sc === s.cmp)));
  $("sumKindSeg").hidden = $("sumNav").hidden = !res0;
  $("btnSumCmp").hidden = !res0;
  $("btnSumBack").hidden = res0;
  $("sumCmpSeg").hidden = res0;
  $("btnReport").hidden = !res0;
  $("sumDays").hidden = !(s.mode === "comparar" && s.cmp === "dias");
  $("sumSay").hidden = $("sumTop").hidden = !res0;
  if (!res0) renderSumTrips(null);
  $("sumDetails").hidden = !res0;
  const periods = sumPeriods();
  if (res0) {
    $("sumWhen").textContent = periods[0][3];
    $("sumNext").disabled = !s.anchor;
    $("sumDay").value = dateVal(periods[0][0]);
    $("sumDay").max = dateVal(Date.now());
  }
  if (!res0 && s.cmp === "dias") { $("sumDayA").value = dateVal(periods[0][0]); $("sumDayB").value = dateVal(periods[1][0]); $("sumDayA").max = $("sumDayB").max = dateVal(Date.now()); }
  const my = ++sumSeq;
  let res;
  try { res = await Promise.all(periods.map((p) => api.GetSummary(p[0], p[1]))); } catch (e) { toast(errMsg(e), true); return; }
  if (my !== sumSeq) return;
  state.sumRes = { periods, res };
  if (res0) renderSummary(res[0], periods[0]);
  else { $("sumPlot").closest(".sum-chart").hidden = false; renderCompare(res, periods); sumOverlayClear(); }
  renderSumGaps(res0 ? res[0] : null);
  renderSumNotes(periods[0][0], periods[0][1]);
}

// maturidade do histórico (2.3): para cada janela, se já existe histórico
// para ela inteira e quanto foi medido
const MAT_STATE = { disponivel: "disponível", formacao: "em formação", aguardando: "aguardando dados" };
async function renderMaturity() {
  const box = $("sumMature"), card = $("setMature");
  if (!api.GetMaturity) { card.hidden = true; return; }
  let m;
  try { m = await api.GetMaturity(); } catch (e) { card.hidden = true; return; }
  const winTxt = (d) => (d === 1 ? "24 h" : d === 365 ? "1 ano" : `${d} dias`);
  const cell = (w) => {
    if (!w || w.state === "aguardando") return `<td class="mat aguardando">${MAT_STATE.aguardando}</td>`;
    const st = w.state === "formacao" ? `em formação (${fmt(w.histDays, w.histDays < 10 ? 1 : 0)} de ${w.days} dias)` : MAT_STATE.disponivel;
    return `<td class="mat ${w.state}"><span>${st}</span><br><span class="muted">${fmt(w.coverage * 100, 0)}% medido · ${fmt(w.minutes, 0)} de ${fmt(w.histMinutes, 0)} min</span></td>`;
  };
  const rc = m.rc && m.rc.length;
  let html = `<p class="small muted">Para cada janela (até agora): se o histórico já cobre a janela inteira e quanto dela foi medido, contando desde o primeiro dado do aparelho. Médias de janelas "em formação" ainda não representam a janela toda.</p>`;
  html += `<table class="mat-table"><thead><tr><th>janela</th><th>GMC-800</th>${rc ? "<th>RC102</th>" : ""}</tr></thead><tbody>`;
  m.gmc.forEach((w, i) => { html += `<tr><th>${winTxt(w.days)}</th>${cell(w)}${rc ? cell(m.rc[i]) : ""}</tr>`; });
  html += "</tbody></table>";
  const first = [m.gmcFirst ? `GMC desde ${dayLabel(m.gmcFirst)} ${hhmm(m.gmcFirst)}` : "", m.rcFirst ? `RC102 desde ${dayLabel(m.rcFirst)} ${hhmm(m.rcFirst)}` : ""].filter(Boolean).join(" · ");
  if (first) html += `<p class="small muted">${first}</p>`;
  box.innerHTML = html;
  card.hidden = false;
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
  return d;
}

// raio-X de tórax (20 µSv, o mesmo da régua), sempre o mesmo no Resumo
// (ACE, 11/11): abaixo de 1, porcentagem; de 1 para cima, quantos
function doseXray(uSv) {
  if (!(uSv > 0)) return "";
  const x = uSv / 20;
  if (x < 0.01) return "menos de 1% de um raio-X de tórax";
  if (x < 1) {
    const p = x * 100, r = p <= 20 ? Math.max(1, Math.round(p)) : Math.round(p / 5) * 5;
    if (r < 100) return `≈ ${fmt(r, 0)}% de um raio-X de tórax`;
  }
  const n = knownNum(x);
  return `≈ ${knownTxt(n)} ${n === 1 ? "raio-X de tórax" : "raios-X de tórax"}`;
}

// em casa × fora, embaixo da dose: só se houve minuto fora no período
function sumAwayBar(k, rc) {
  if (!rc || !rc.awayMinutes) return;
  const tot = rc.homeDose + rc.awayDose;
  if (!(tot > 0)) return;
  const d = (v) => fmt(v, v < 10 ? 2 : 1);
  const w = document.createElement("div");
  w.className = "sum-away";
  w.innerHTML = '<div class="sum-away-bar" aria-hidden="true"><i class="home"></i><i class="away"></i></div><span class="small muted"></span>';
  const hp = (rc.homeDose / tot) * 100;
  w.querySelector(".home").style.width = hp.toFixed(1) + "%";
  w.querySelector(".away").style.width = (100 - hp).toFixed(1) + "%";
  w.lastElementChild.textContent = `em casa ${d(rc.homeDose)} · fora ${d(rc.awayDose)} µSv`;
  w.title = `Fora de casa: ${hm(rc.awayMinutes)}, ${fmt(100 - hp, 0)}% da dose`;
  k.appendChild(w);
}

// os três números: quanto (dose), comparado com o quê (fundo) e o pico
function renderSumBig(r, rc) {
  const box = $("sumBig");
  box.textContent = "";
  if (!r.minutes && !rc) return;
  const d = (v) => fmt(v, usvDigits(v));
  const k = rc ? sumKpi(box, "Sua dose", sumUSv(rc.devOK ? rc.devDose : rc.dose), rc.devOK ? "RC102 · somada pelo aparelho" : `RC102 · ${fmt(rc.coverage * 100, 0)}% medido`, "rc")
    : sumKpi(box, "Dose da casa", sumUSv(r.dose), `GMC-800 · ${fmt(r.coverage * 100, 0)}% medido`);
  // em coisas conhecidas: horas do fundo da casa e o raio-X de tórax
  const kn = doseKnownEl(rc ? (rc.devOK ? rc.devDose : rc.dose) : r.dose, rc && rc.bgUSv > 0 ? rc.bgUSv : null, "sum-known", doseXray);
  if (kn) k.appendChild(kn);
  sumAwayBar(k, rc);
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

// "Mais números" (2.64): só o que não está em cima. Média, Máximo e Dose
// saíram (repetiam o Fundo, o Pico e a Sua dose); Alertas vira link.
function renderSummary(r, per) {
  $("sumSub").textContent = `${dayLabel(per[0])} ${hhmm(per[0])} → ${per[1] >= Date.now() - 6e4 ? "agora" : dayLabel(per[1] - 1) + " " + (hhmm(per[1]) === "00:00" ? "24:00" : hhmm(per[1]))}`;
  const rc = r.rc && r.rc.minutes ? r.rc : null;
  renderSumSay(r, rc);
  renderSumTrips(per);
  renderSumBig(r, rc);
  const t = $("sumTiles");
  t.textContent = "";
  $("sumTable").hidden = true;
  const d = (v) => fmt(v, usvDigits(v));
  if (rc) sumTile(t, "Mínimo", `${d(rc.min)} µSv/h`, `RC102 · ${atText(r, rc.minAt)}`, "rc");
  else if (r.minutes) sumTile(t, "Mínimo", r.minUSv != null ? usvVal(r.minUSv) : `${fmt(r.min, 0)} CPM`, `GMC-800 · ${atText(r, r.minAt)}`);
  if (rc) sumTile(t, "Medido", `${fmt(rc.coverage * 100, 0)}%`, "RC102 · " + covText({ ...rc, ongoing: r.ongoing }), "rc");
  else if (r.minutes) {
    const days = r.spanMinutes > 1440 && r.daysData ? ` · ${r.daysData} ${r.daysData === 1 ? "dia" : "dias"} com dado` : "";
    sumTile(t, "Medido", `${fmt(r.coverage * 100, 0)}%`, "GMC-800 · " + covText(r) + days);
  }
  if (r.minutes && r.dispersion != null) sumTile(t, "Dispersão", fmt(r.dispersion, 2), `GMC-800 · σ real ${fmt(r.sd, 1)} ÷ acaso ${fmt(Math.sqrt(r.avg), 1)} · ${r.dispersion >= 1.2 ? "variou além do acaso" : "perto de 1: normal"}`, r.dispersion >= 1.2 ? "warn" : "");
  sumTile(t, "Chuva", rainText(r), r.rainHours ? `${r.rainHours} h com chuva` : "");
  const n = (k) => (r[k] || 0) + ((rc && rc[k]) || 0);
  if (n("danger") + n("alarm") + n("attention")) {
    const b = document.createElement("button");
    b.type = "button";
    b.className = "link small-link sum-alerts-link";
    b.textContent = "Ver os alertas do período";
    b.addEventListener("click", () => openAlertsRange(per[0], per[1], per[3] || per[2]));
    t.appendChild(b);
  }
  const chart = $("sumPlot").closest(".sum-chart");
  chart.hidden = !r.minutes && !rc;
  if (chart.hidden) { drawSumChart([]); sumOverlayClear(); return; }
  drawSumChart([{ r, color: COR.accent, label: "GMC-800" }]);
  renderSumOverlay(per);
}

// abre os Alertas só com os do período (o filtro some ao sair de lá)
async function openAlertsRange(from, to, label) {
  const all = await api.GetEvents("alerta");
  await showView("alertas", true);
  state.alertRange = { from, to, label };
  renderAlertHistory(all || []);
  $("alertRangeChip").scrollIntoView({ block: "center" });
}

// ---------- gráfico do Resumo: faixa de nível, marcas e fora de casa ----------
//
// Um gráfico só no lugar da fita + gráfico: a cor do nível vira uma faixa fina
// embaixo (a mesma lógica da fita, fita.js), as marcas (atenção, alarme,
// anotação) ficam em cima e o tempo fora de casa pinta o fundo de lilás.
// Dia: pedaços de 10 min. Semana e mês: um pedaço por dia (o nível mais alto).

const SUM_LV_TXT = { seguro: "seguro", atencao: "atenção", alarme: "alarme", perigo: "perigo" };
let sumOvSeq = 0;

function sumOverlayClear() {
  sumOvSeq++;
  $("sumBand").innerHTML = "";
  $("sumMarks").innerHTML = "";
  $("sumPlot").classList.remove("has-band", "has-marks");
  const aw = $("sumChart").querySelector(".sum-away-bg");
  if (aw) aw.remove();
}

async function renderSumOverlay(per) {
  const my = ++sumOvSeq;
  if (!api.GetDayStrip) return;
  const ser = state.sumSeries && state.sumSeries[0];
  if (!ser) return;
  const t0 = per[0], t1 = per[1];
  let s;
  try { s = await ftFetch(startDay(t0), t1); } catch (e) { return; }
  if (my !== sumOvSeq) return;
  // o gráfico vai do primeiro ao último ponto da curva (por índice)
  const c = ser.r.rc && ser.r.rc.minutes && ser.r.rc.curve.length >= ser.r.curve.length ? ser.r.rc.curve : ser.r.curve;
  const x0 = c.length ? c[0].t : t0, x1 = c.length > 1 ? c[c.length - 1].t : t1;
  const span = Math.max(1, x1 - x0);
  const pct = (t) => Math.max(0, Math.min(100, ((t - x0) / span) * 100));
  const lvOf = (sl) => {
    const a = FT_RANK[sl.home] || 0, b = FT_RANK[sl.pers] || 0;
    return a || b ? (a >= b ? sl.home : sl.pers) : null;
  };
  const slots = (s.slots || []).filter((sl) => sl.t >= t0 && sl.t < t1);
  const multi = t1 - t0 > DAY_MS * 1.1;
  // faixa de nível
  const runs = [];
  if (multi) {
    const byDay = new Map();
    for (const sl of slots) {
      const lv = lvOf(sl);
      if (!lv) continue;
      const d0 = startDay(sl.t), cur = byDay.get(d0);
      if (!cur || FT_RANK[lv] > FT_RANK[cur]) byDay.set(d0, lv);
    }
    for (const [d0, lv] of byDay) runs.push([d0, addDays(d0, 1), lv]);
  } else {
    for (const sl of slots) {
      const lv = lvOf(sl);
      if (!lv) continue;
      const last = runs[runs.length - 1];
      if (last && last[2] === lv && last[1] === sl.t) last[1] = sl.t + 6e5;
      else runs.push([sl.t, sl.t + 6e5, lv]);
    }
  }
  $("sumBand").innerHTML = runs.map(([a, b, lv]) => {
    const l = pct(a), w = Math.max(0.2, pct(b) - l);
    return `<i style="left:${l}%;width:${w}%;background:${FT_COL[lv]}${lv === "seguro" ? ";opacity:.55" : ""}"></i>`;
  }).join("");
  // fora de casa: fundo lilás no trecho
  const away = [];
  for (const sl of slots) {
    if (!sl.away) continue;
    const last = away[away.length - 1];
    if (last && last[1] === sl.t) last[1] = sl.t + 6e5;
    else away.push([sl.t, sl.t + 6e5]);
  }
  const svg = $("sumChart");
  const old = svg.querySelector(".sum-away-bg");
  if (old) old.remove();
  if (away.length) {
    const g = document.createElementNS("http://www.w3.org/2000/svg", "g");
    g.setAttribute("class", "sum-away-bg");
    g.innerHTML = away.map(([a, b]) => `<rect x="${((pct(a) / 100) * W).toFixed(1)}" y="0" width="${Math.max(1, ((pct(b) - pct(a)) / 100) * W).toFixed(1)}" height="${H}" fill="var(--away)" fill-opacity=".14"/>`).join("");
    svg.insertBefore(g, svg.firstChild);
  }
  // marcas em cima (alertas com a forma do nível, anotações com o lápis)
  const esc = (t) => String(t).replace(/[&<>"]/g, (ch) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[ch]);
  const marks = (s.marks || []).filter((m) => m.t >= t0 && m.t < t1);
  $("sumMarks").innerHTML = marks.map((m) => {
    const ico = m.kind === "nota" ? FT_PEN : LV_PATH[m.level] ? ftShape(m.level, 12) : ftShape("atencao", 12);
    const tip = (multi ? `${ddmm(m.t)} ` : "") + ftMarkTip(m);
    return `<span class="sum-mk" style="left:${pct(m.t)}%" title="${esc(tip)}">${ico}</span>`;
  }).join("");
  $("sumPlot").classList.toggle("has-band", runs.length > 0);
  $("sumPlot").classList.toggle("has-marks", marks.length > 0);
  sumLegendExtra(runs, away.length > 0, marks);
}

// legenda dinâmica: só o que tem no período
function sumLegendExtra(runs, away, marks) {
  const lg = $("sumLegend");
  lg.querySelectorAll(".sum-lg-x").forEach((e) => e.remove());
  const add = (html, txt) => {
    const sp = document.createElement("span");
    sp.className = "key sum-lg-x";
    sp.innerHTML = html;
    sp.append(txt);
    lg.appendChild(sp);
  };
  const lvs = new Set(runs.map((r) => r[2]));
  for (const m of marks) if (m.kind !== "nota") lvs.add(LV_PATH[m.level] ? m.level : "atencao");
  if (lvs.has("seguro")) add('<i class="ft-sw" style="background:var(--accent);opacity:.55"></i>', "seguro");
  for (const lv of ["atencao", "alarme", "perigo"]) if (lvs.has(lv)) add(ftShape(lv, 11), SUM_LV_TXT[lv]);
  if (away) add('<i class="ft-sw" style="background:var(--away);opacity:.5"></i>', "fora de casa");
  if (marks.some((m) => m.kind === "nota")) add(FT_PEN, "anotação");
}

function renderCompare(res, periods) {
  const [a, b] = res;
  const sub = $("sumSub");
  sub.textContent = "";
  const sp = (t) => { const e = document.createElement("span"); e.textContent = t; return e; };
  sub.append(sp(periods[0][2]), " × ", sp(periods[1][2]));
  $("sumTiles").textContent = "";
  $("sumBig").textContent = "";
  const tb = $("sumTable");
  tb.hidden = false;
  // tempos medidos muito diferentes: a dose não dá para comparar em %
  const fair = Math.abs(a.minutes - b.minutes) <= 0.1 * Math.max(a.minutes, b.minutes);
  const pctTxt = (x, y) => { const p = ((x - y) / y) * 100; return Math.abs(p) < 0.5 ? " (igual)" : ` (${sgn(p, 0)}%)`; };
  const diff = (x, y, d, unit, timed) => (x == null || y == null ? "—" : Math.abs(x - y) < Math.pow(10, -d) / 2 ? "igual" : `${sgn(x - y, d)}${unit}` + (y && (!timed || fair) ? pctTxt(x, y) : ""));
  const rows = [
    ["Média", a.avgUSv, b.avgUSv, 3, " µSv/h"],
    ["Máximo", a.maxUSv, b.maxUSv, 3, " µSv/h"],
    ["Mínimo", a.minUSv, b.minUSv, 3, " µSv/h"],
    ["Dose", a.dose, b.dose, 2, " µSv", true],
  ];
  let html = `<thead><tr><th></th><th><i class="sw" style="background:${COR.accent}"></i></th><th><i class="sw" style="background:${COR.muted}"></i></th><th>A − B</th></tr></thead><tbody>`;
  tb.innerHTML = html + "</tbody>";
  const head = tb.querySelectorAll("th");
  head[1].append(" " + periods[0][2]); head[2].append(" " + periods[1][2]);
  const body = tb.querySelector("tbody");
  const row = (cells) => { const tr = document.createElement("tr"); cells.forEach((c, i) => { const td = document.createElement(i ? "td" : "th"); td.textContent = c; tr.appendChild(td); }); body.appendChild(tr); };
  for (const [n, x, y, d, u, timed] of rows) row([n, x == null ? "—" : fmt(x, d) + u, y == null ? "—" : fmt(y, d) + u, diff(x, y, d, u, timed)]);
  if ((a.rc && a.rc.minutes) || (b.rc && b.rc.minutes)) {
    const ra = a.rc || {}, rb = b.rc || {};
    const v = (o, k, dd, u) => (o.minutes ? fmt(o[k], dd) + u : "—");
    const dd = (o, k, d2, u) => (o.minutes && o[k] != null ? o[k] : null);
    row(["RC102: dose", v(ra, "dose", 2, " µSv"), v(rb, "dose", 2, " µSv"), diff(dd(ra, "dose"), dd(rb, "dose"), 2, " µSv", Math.abs((ra.minutes || 0) - (rb.minutes || 0)) <= 0.1 * Math.max(ra.minutes || 0, rb.minutes || 0))]);
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
  const n = Math.max(...series.map((s) => (s.r.curve || []).length));
  if (!n || !(series[0].r.curve || []).length) { svg.innerHTML = ""; $("sumLegend").textContent = ""; [...$("sumX").children].forEach((e) => (e.textContent = "")); return; }
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
  if (!c0.length) return;
  const lab = (t) => (long ? `${WEEKDAY[new Date(t).getDay()]} ${ddmm(t)}` : hhmm(t));
  const xs = $("sumX").children;
  xs[0].textContent = lab(c0[0].t); xs[1].textContent = lab(c0[Math.floor(c0.length / 2)].t); xs[2].textContent = lab(r.to);
  $("sumLegend").innerHTML = '<span class="key"><i style="border-color:' + COR.rc + '"></i>RC102 (µSv/h)</span><span class="key"><i style="border-color:' + COR.accent + '"></i>GMC-800 (µSv/h)</span>' +
    (rc.bgUSv ? '<span class="key"><i style="border-color:' + COR.rc + ';border-top-style:dashed;opacity:.6"></i>seu normal em casa (RC102)</span>' : "");
}
