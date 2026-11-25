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
  // faixas de cômodo; 2.106.1: troca de cômodo que durou menos de 10 min
  // (clique por engano, desfeito logo) não parte a faixa em duas
  const pl = [];
  rows.forEach((r, k) => {
    const next = k + 1 < rows.length ? rows[k + 1].t : t1;
    pl.push(k && r.place !== pl[k - 1] && next - r.t < 10 * 6e4 ? pl[k - 1] : r.place);
  });
  let i = 0;
  while (i < rows.length) {
    let j = i;
    while (j + 1 < rows.length && pl[j + 1] === pl[i]) j++;
    const a = x(rows[i].t), b = j + 1 < rows.length ? x(rows[j + 1].t) : x(t1);
    if (pl[i]) s += `<rect x="${a.toFixed(1)}" y="6" width="${Math.max(1, b - a - 2).toFixed(1)}" height="12" rx="3" class="rn-band rn-band-${pl[i]}"/>` + (b - a > 46 ? `<text x="${(a + 5).toFixed(1)}" y="15.5" class="rn-band-t">${rnPlaceName(pl[i])}</text>` : "");
    i = j + 1;
  }
  // linha (quebra nos buracos de mais de 2 h); trecho que toca hora da
  // memória do aparelho sai tracejado (2.106.0)
  let d = "", dm = "";
  rows.forEach((r, k) => {
    if (!k || r.t - rows[k - 1].t > 2 * 36e5) return;
    const p = rows[k - 1], seg = `M${x(p.t).toFixed(1)} ${y(rnBq(p.hour)).toFixed(1)}L${x(r.t).toFixed(1)} ${y(rnBq(r.hour)).toFixed(1)}`;
    if (r.mem || p.mem) dm += seg; else d += seg;
  });
  s += `<path d="${d}" class="rn-line"/>`;
  if (dm) s += `<path d="${dm}" class="rn-line rn-line-mem"/>`;
  rows.forEach((r) => {
    if (!r.mem) return;
    const tip = `${ddmm(r.t - 36e5)} ${hhmm(r.t - 36e5)}–${hhmm(r.t)} · ${rnBqTxt(rnBq(r.hour))} Bq/m³ · ${r.counts === 1 ? "1 contagem" : `${r.counts || 0} contagens`}`;
    s += `<circle cx="${x(r.t).toFixed(1)}" cy="${y(rnBq(r.hour)).toFixed(1)}" r="3" class="rn-mem-pt"><title>${tip}</title></circle>`;
  });
  // dias embaixo
  const day0 = new Date(t0); day0.setHours(24, 0, 0, 0);
  for (let t = day0.getTime(); t < t1; t += RN_DAY) {
    s += `<line x1="${x(t).toFixed(1)}" x2="${x(t).toFixed(1)}" y1="${T}" y2="${H - B}" class="rn-grid"/><text x="${(x(t) + 3).toFixed(1)}" y="${H - 8}">${isToday(t) ? "hoje" : isYesterday(t) ? "ontem" : ddmm(t)}</text>`;
  }
  s += `<text x="${L - 6}" y="${T - 10}" text-anchor="end" class="rn-u">Bq/m³</text>`;
  const legend = rows.some((r) => r.mem) ? `<div class="rn-legend small muted"><span><i class="rn-lg"></i>lido ao vivo</span><span><i class="rn-lg rn-lg-mem"></i>da memória do aparelho (uma média por hora)</span></div>` : "";
  return `<div class="rn-chart"><svg viewBox="0 0 ${W} ${H}" width="100%" role="img" aria-label="Radônio por hora nos últimos 7 dias">${s}</svg></div>${legend}`;
}

// 2.106.0: botão da memória do aparelho, aviso do buraco e resultado
let rnMemBusy = false;
// início e fim de um trecho ("24/11 01:04" e "10:04"; o dia só se mudar)
const rnEnds = (a, b) => [`${ddmm(a)} ${hhmm(a)}`, `${ddmm(b) === ddmm(a) ? "" : ddmm(b) + " "}${hhmm(b)}`];
function rnMemBox(st) {
  const on = st.state === "conectado";
  const btn = `<button type="button" class="btn${st.gaps && st.gaps.length && on ? " accent" : ""}" data-rn-mem ${on && !rnMemBusy ? "" : "disabled"}><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 4v11"/><path d="m7 10 5 5 5-5"/><path d="M5 20h14"/></svg><span>${rnMemBusy ? "Baixando…" : "Baixar da memória do aparelho"}</span></button>`;
  const m = st.memLast, gap = st.gaps && st.gaps.length ? st.gaps[st.gaps.length - 1] : null;
  let note = "";
  if (m && m.hours > 0) {
    const [ra, rb] = rnEnds(m.from, m.to);
    note = `<p class="rn-mem-ok"><span>${m.hours === 1 ? `1 hora trazida da memória (${ra} a ${rb}).` : `${m.hours} horas trazidas da memória (${ra} a ${rb}).`}</span> <span>Média dessas horas: <b class="mono">${rnBqTxt(rnBq(m.mean))} Bq/m³</b>.</span></p>`;
  } else if (gap && on && !(m && m.at >= gap.to)) {
    const h = (gap.to - gap.from) / 36e5, [ga, gb] = rnEnds(gap.from, gap.to);
    note = `<p class="notice rn-mem-gap"><span>${rnHours(h)} sem leitura (${ga} a ${gb}).</span> <span>O RadonScan guarda a contagem de cada hora: dá pra trazer.</span></p>`;
  } else if (m && m.err) {
    note = `<p class="small muted">${escHtml(m.err)}</p>`;
  } else if (m) {
    note = `<p class="small muted">A memória do aparelho não tinha horas novas pra trazer.</p>`;
  }
  return { btn, note };
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
  const mb = rnMemBox(st);
  $("rnWeek").innerHTML = `<div class="rn-wh"><h2 class="rn-h">Última semana, média de 1 hora</h2>${mb.btn}</div>${mb.note}` + rnChart(st.rows) +
    `<p class="small muted">Só lê, não muda nada no aparelho. Quando ele reconecta depois de um buraco, o app já baixa sozinho.</p>`;
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
    const mbtn = e.target.closest("[data-rn-mem]");
    if (mbtn && api.DownloadRadonMemory && !rnMemBusy) {
      rnMemBusy = true;
      renderRadon();
      try { rnMemBusy = false; renderRadon(await api.DownloadRadonMemory()); } catch (err) { rnMemBusy = false; renderRadon(); toast(String(err.message || err), true); }
      return;
    }
    const b = e.target.closest("[data-rn-place]");
    if (!b || !api.SetRadonPlace) return;
    const p = b.dataset.rnPlace === (state.radon && state.radon.place) ? "" : b.dataset.rnPlace;
    try { renderRadon(await api.SetRadonPlace(p)); } catch (err) { toast(String(err.message || err), true); }
  });
  refreshRadon();
  setInterval(refreshRadon, 5 * 60e3); // o gráfico e o "pronta em" andam
}
