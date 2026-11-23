"use strict";

// GQ RadonScan (2.105.0): radônio no ar. Página própria em Aparelhos e um
// cartão pequeno no Agora. O aparelho manda pCi/L; a tela mostra Bq/m³ (SI)
// com pCi/L pequeno do lado. Não entra na dose e não tem alarme: é o ar que
// se respira, não radiação gama no corpo.

const RN_BQ = 37; // 1 pCi/L = 37 Bq/m³
// referências: OMS 100 Bq/m³; EPA (EUA) 4 pCi/L = 148 Bq/m³; 300 = teto da OMS e da UE
const RN_REF = [{ v: 100, t: "referência OMS" }, { v: 148, t: "EPA" }, { v: 300, t: "alto" }];
const RN_MAX = 400; // fim da régua
const RN_PLACES = [["sala", "Sala"], ["quarto", "Quarto"], ["outro", "Outro"]];
const RN_DAY = 864e5;


const rnBq = (pci) => pci * RN_BQ;
const rnNum = (bq) => Math.round(bq).toLocaleString(LOCALE);
// 2.105.2: o aparelho só manda pCi/L com 2 casas (0,01 pCi/L = 0,37 Bq/m³);
// abaixo de 100 Bq/m³ uma casa decimal fica mais perto da tela dele
const rnBqTxt = (bq) => bq < 100 ? bq.toLocaleString(LOCALE, { minimumFractionDigits: 1, maximumFractionDigits: 1 }) : rnNum(bq);
const rnPci = (pci) => pci.toLocaleString(LOCALE, { minimumFractionDigits: 2, maximumFractionDigits: 2 });
function rnLevel(bq) {
  if (bq >= 300) return { cls: "rn-alto", word: "Alto" };
  if (bq >= 100) return { cls: "rn-mod", word: "Moderado" };
  return { cls: "rn-baixo", word: "Baixo" };
}
const rnPlaceName = (p) => (RN_PLACES.find((x) => x[0] === p) || [p, p || "—"])[1];

function rnHours(h) {
  if (h < 1) return `${Math.max(1, Math.round(h * 60))} min`;
  if (h < 48) return `${Math.round(h)} h`;
  const d = Math.floor(h / 24), r = Math.round(h % 24);
  return r ? `${d} d ${r} h` : `${d} d`;
}

// "pronta em": a média só fecha o período dela depois desse tempo medindo
function rnTile(label, pci, span, first) {
  const left = span ? first + span - Date.now() : 0;
  const wait = span && first && left > 0;
  const sub = wait ? `pronta em ${rnHours(left / 36e5)}` : `${rnPci(pci)} pCi/L`;
  const prog = wait ? `<span class="rn-prog"><i style="width:${Math.max(2, Math.min(100, 100 * (1 - left / span))).toFixed(0)}%"></i></span>` : "";
  return `<div class="rn-tile${wait ? " rn-wait" : ""}"><span class="tile-label">${label}</span><span class="mono rn-tv">${rnBqTxt(rnBq(pci))}</span><span class="tile-sub">${sub}</span>${prog}</div>`;
}

function rnBar(bq) {
  const pos = (v) => (100 * Math.min(v, RN_MAX) / RN_MAX).toFixed(1);
  const ticks = RN_REF.map((r) => `<span style="left:${pos(r.v)}%"><i class="rn-tw-t">${r.t}</i> <b class="mono">${r.v}</b></span>`).join("");
  return `<div class="rn-bar" aria-hidden="true"><span class="rn-z0" style="width:${pos(100)}%"></span><span class="rn-z1" style="width:${(pos(300) - pos(100)).toFixed(1)}%"></span><span class="rn-z2"></span><i class="rn-pin" style="left:${pos(bq)}%"></i></div><div class="rn-ticks">${ticks}</div>`;
}

// gráfico dos últimos 7 dias: a média de 1 hora e, em cima, onde estava
function rnChart(rows) {
  if (!rows || rows.length < 2) return `<p class="small muted">O gráfico aparece depois de algumas horas de leitura.</p>`;
  const W = 760, H = 200, L = 40, R = 10, T = 28, B = 26;
  const t0 = rows[0].t, t1 = Math.max(rows[rows.length - 1].t, t0 + 36e5);
  const ys = rows.map((r) => rnBq(r.hour));
  const top = Math.max(40, Math.ceil(Math.max(...ys) / 40) * 40);
  const x = (t) => L + (W - L - R) * (t - t0) / (t1 - t0);
  const y = (v) => T + (H - T - B) * (1 - v / top);
  let s = "";
  for (let k = 0; k <= 4; k++) {
    const v = top * k / 4;
    s += `<line x1="${L}" x2="${W - R}" y1="${y(v).toFixed(1)}" y2="${y(v).toFixed(1)}" class="rn-grid"/><text x="${L - 6}" y="${(y(v) + 4).toFixed(1)}" text-anchor="end">${rnNum(v)}</text>`;
  }
  // faixas de cômodo
  let i = 0;
  while (i < rows.length) {
    let j = i;
    while (j + 1 < rows.length && rows[j + 1].place === rows[i].place) j++;
    const a = x(rows[i].t), b = j + 1 < rows.length ? x(rows[j + 1].t) : x(t1);
    if (rows[i].place) s += `<rect x="${a.toFixed(1)}" y="6" width="${Math.max(1, b - a - 2).toFixed(1)}" height="12" rx="3" class="rn-band rn-band-${rows[i].place}"/>` + (b - a > 46 ? `<text x="${(a + 5).toFixed(1)}" y="15.5" class="rn-band-t">${rnPlaceName(rows[i].place)}</text>` : "");
    i = j + 1;
  }
  // linha (quebra nos buracos de mais de 2 h)
  let d = "";
  rows.forEach((r, k) => { d += (k && r.t - rows[k - 1].t <= 2 * 36e5 ? "L" : "M") + x(r.t).toFixed(1) + " " + y(rnBq(r.hour)).toFixed(1); });
  s += `<path d="${d}" class="rn-line"/>`;
  // dias embaixo
  const day0 = new Date(t0); day0.setHours(24, 0, 0, 0);
  for (let t = day0.getTime(); t < t1; t += RN_DAY) {
    s += `<line x1="${x(t).toFixed(1)}" x2="${x(t).toFixed(1)}" y1="${T}" y2="${H - B}" class="rn-grid"/><text x="${(x(t) + 3).toFixed(1)}" y="${H - 8}">${isToday(t) ? "hoje" : isYesterday(t) ? "ontem" : ddmm(t)}</text>`;
  }
  s += `<text x="${L - 6}" y="${T - 10}" text-anchor="end" class="rn-u">Bq/m³</text>`;
  return `<div class="rn-chart"><svg viewBox="0 0 ${W} ${H}" width="100%" role="img" aria-label="Radônio por hora nos últimos 7 dias">${s}</svg></div>`;
}

function rnPlacesTable(places) {
  const rows = (places || []).filter((p) => p.hours > 0);
  if (!rows.length) return `<p class="small muted">Escolha onde o RadonScan está (em cima, à direita). Com pelo menos um dia em cada cômodo, o radônio de cada um aparece aqui junto do fundo do GMC-800 no mesmo período.</p>`;
  const body = rows.map((p) => `<tr><td>${rnPlaceName(p.place)}</td><td class="mono">${rnHours(p.hours)}</td><td class="mono">${rnBqTxt(rnBq(p.pciL))} Bq/m³</td><td class="mono">${p.gmcMin ? rnNum(p.gmcCpm) + " cpm" : "—"}</td></tr>`).join("");
  const short = rows.some((p) => p.hours < 24);
  return `<div class="rn-tw"><table class="rn-table"><thead><tr><th>Cômodo</th><th>Tempo</th><th>Radônio médio</th><th>Fundo GMC-800</th></tr></thead><tbody>${body}</tbody></table></div>` +
    `<p class="small muted">${short ? "<span>Menos de um dia num cômodo ainda é pouco: o radônio muda bastante entre o dia e a noite.</span> " : ""}<span>O fundo do GMC-800 conta só os minutos em que ele estava em casa, fora das medições de objeto.</span></p>`;
}

function renderRadon(st) {
  if (st) state.radon = st;
  st = state.radon;
  const nav = $("navRadon");
  if (nav) {
    nav.hidden = !(st && st.seen);
    $("navRadonDot").hidden = !(st && st.state === "erro");
  }
  renderRadonAgora();
  const box = $("rnMain");
  if (!box || !st) return;
  const on = st.state === "conectado";
  const conn = on ? `<span class="rn-via mono">· ${escHtml(st.port)}</span>` : `<span class="rn-off">${st.state === "erro" ? "erro" : "procurando no USB…"}</span>`;
  const seg = RN_PLACES.map(([k, t]) => `<button type="button" data-rn-place="${k}" aria-pressed="${st.place === k}">${t}</button>`).join("");
  let html = `<div class="rn-head"><span class="eyebrow">GQ RadonScan</span>${conn}<span class="rn-fill"></span><span class="small muted">onde está</span><div class="seg rn-seg" role="group" aria-label="Onde está o RadonScan">${seg}</div></div>`;
  const l = st.last;
  if (!l) {
    html += `<div class="empty rn-empty"><span>${on ? "Esperando a primeira leitura do RadonScan…" : "Ligue o RadonScan no USB do PC. O app acha ele sozinho."}</span></div>`;
    box.innerHTML = html;
    return;
  }
  const bq = rnBq(l.hour), lv = rnLevel(bq);
  const when = st.readAt && on ? `atualizado ${hhmm(st.readAt)}` : `última leitura ${whenLabel(l.t)}`;
  html += `<div class="value-row"><span class="value">${rnBqTxt(bq)}</span><span class="unit">Bq/m³</span><span class="hero-lv ${lv.cls}">${lv.word}</span></div>` +
    `<p class="rn-alt mono">${rnPci(l.hour)} pCi/L · média da última hora · ${when}</p>` + rnBar(bq) +
    `<p class="small muted">Ar de dentro de casa costuma ficar entre 10 e 50 Bq/m³. Acima de 100 vale ventilar mais; acima de 300 é alto.</p>` +
    `<div class="rn-tiles">${rnTile("1 hora", l.hour, 0, 0)}${rnTile("1 dia", l.day, RN_DAY, st.first)}${rnTile("1 semana", l.week, 7 * RN_DAY, st.first)}${rnTile("Longo prazo", l.long, 0, 0)}</div>` +
    `<p class="small muted">O RadonScan recalcula o radônio de hora em hora. "Pronta em" conta desde que o app começou a ler o aparelho.</p>`;
  box.innerHTML = html;
  $("rnWeek").innerHTML = `<h2 class="rn-h">Última semana, média de 1 hora</h2>` + rnChart(st.rows);
  $("rnPlaces").innerHTML = `<h2 class="rn-h">Radônio e fundo por cômodo</h2>` + rnPlacesTable(st.places);
  $("rnWeek").hidden = $("rnPlaces").hidden = false;
}

function renderRadonAgora() {
  const card = $("agRadon");
  if (!card) return;
  const st = state.radon, l = st && st.last;
  card.hidden = !(st && st.seen && l);
  if (card.hidden) return;
  const bq = rnBq(l.hour), lv = rnLevel(bq);
  const where = st.place ? ` · ${rnPlaceName(st.place)}` : "";
  const on = st.state === "conectado";
  const when = on && st.readAt ? `média 1 h · atualizado ${hhmm(st.readAt)}` : `desconectado · última ${whenLabel(l.t)}`;
  card.innerHTML = `<button type="button" class="rn-card" data-view="radonscan"><span class="rn-card-n"><span class="dot rn-dot${on ? "" : " off"}" aria-hidden="true"></span>GQ RadonScan${where}</span>` +
    `<span class="mono rn-card-v">${rnBqTxt(bq)}</span><span class="rn-card-u">Bq/m³</span><span class="hero-lv ${lv.cls}">${lv.word}</span><span class="rn-card-w small muted">${when}</span></button>`;
}

async function refreshRadon() {
  if (!api.GetRadonScan) return;
  try { renderRadon(await api.GetRadonScan()); } catch (e) { /* sem RadonScan */ }
}

function wireRadon() {
  on("radonscan", renderRadon);
  document.addEventListener("click", async (e) => {
    const b = e.target.closest("[data-rn-place]");
    if (!b || !api.SetRadonPlace) return;
    const p = b.dataset.rnPlace === (state.radon && state.radon.place) ? "" : b.dataset.rnPlace;
    try { renderRadon(await api.SetRadonPlace(p)); } catch (err) { toast(String(err.message || err), true); }
  });
  refreshRadon();
  setInterval(refreshRadon, 5 * 60e3); // o gráfico e o "pronta em" andam
}
