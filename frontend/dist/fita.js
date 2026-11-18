// ---------- fita do dia (2.42) ----------
//
// O dia inteiro numa faixa: cada pedacinho é 10 minutos, pintado pelo nível
// da média do aparelho que estava em cada papel (GetDayStrip, daystrip.go).
// Em cima, as marcas (alertas com a forma do nível, anotações com o lápis);
// embaixo, fora de casa (lilás) e chuva (azul). Sem dado fica cinza escuro,
// nunca verde. Na semana, cada linha é um dia e junta casa e você (vale o
// nível mais alto dos dois). Passar o mouse mostra o trecho; clicar abre os
// Gráficos centrados nele. No Agora (no lugar do gráfico "Hoje") e no Resumo.
// Usa os globais do app.js; carregado antes dele.

const FT_N = 144; // pedaços de 10 min num dia
const FT_COL = { seguro: "var(--ok)", atencao: "var(--att)", alarme: "var(--alarm)", perigo: "var(--warn)" };
const FT_PEN = '<svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="var(--text)" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M10.5 2.5l3 3L6 13H3v-3z"/></svg>';
const FT_RANK = { seguro: 1, atencao: 2, alarme: 3, perigo: 4 };
const FT_DEV = { gmc: "GMC-800", radiacode: "RC102" };

const ftShape = (lv, size) => `<svg width="${size}" height="${size}" viewBox="0 0 16 16" fill="${FT_COL[lv]}" aria-hidden="true">${LV_PATH[lv]}</svg>`;

// pedaços de um dia (do começo do dia), na posição certa; o resto é null
function ftDay(strip, day0) {
  const out = new Array(FT_N).fill(null);
  for (const s of (strip && strip.slots) || []) {
    const i = Math.floor((s.t - day0) / 6e5);
    if (i >= 0 && i < FT_N) out[i] = s;
  }
  return out;
}

// faixa em SVG: um retângulo por sequência do mesmo nível; o futuro fica apagado
function ftStripSvg(levels, futureFrom) {
  let s = "", i = 0;
  while (i < FT_N) {
    let j = i;
    while (j < FT_N && levels[j] === levels[i]) j++;
    const lv = levels[i];
    if (FT_COL[lv]) s += `<rect x="${i}" y="0" width="${j - i}" height="10" fill="${FT_COL[lv]}"${lv === "seguro" ? ' fill-opacity=".55"' : ""}/>`;
    i = j;
  }
  if (futureFrom != null && futureFrom < FT_N) s += `<rect class="ft-future" x="${futureFrom}" y="0" width="${FT_N - futureFrom}" height="10"/>`;
  return `<svg viewBox="0 0 ${FT_N} 10" preserveAspectRatio="none" aria-hidden="true">${s}</svg>`;
}

function ftBandSvg(flags, color) {
  let s = "", i = 0;
  while (i < FT_N) {
    if (!flags[i]) { i++; continue; }
    let j = i;
    while (j < FT_N && flags[j]) j++;
    s += `<rect x="${i}" y="0" width="${j - i}" height="10" rx=".8" fill="${color}"/>`;
    i = j;
  }
  return `<svg viewBox="0 0 ${FT_N} 10" preserveAspectRatio="none" aria-hidden="true">${s}</svg>`;
}

// a forma do nível em cima de cada trecho acima do seguro
function ftShapesOver(levels) {
  let s = "", i = 0;
  while (i < FT_N) {
    let j = i;
    while (j < FT_N && levels[j] === levels[i]) j++;
    if (FT_RANK[levels[i]] > 1) s += `<span class="ft-mk" style="left:${((i + j) / 2 / FT_N) * 100}%">${ftShape(levels[i], 10)}</span>`;
    i = j;
  }
  return s;
}

const ftHm = (i) => { const m = i * 10; return pad(Math.floor(m / 60)) + ":" + pad(m % 60); };
// papel de cada aparelho (roles.go)
const FT_ROLE = { casa: "em casa", comigo: "com você", fora: "ignorado" };
// em tela estreita a faixa do papel usa o nome curto (2.83.10)
const FT_ROLE_SHORT = { casa: "casa", comigo: "você", fora: "fora" };

// texto do trecho, para a dica: os dois aparelhos e o papel de cada um (2.83.9)
function ftSlotTip(s, i, showRc) {
  const parts = [`${ftHm(i)}–${ftHm(i + 1)}`];
  if (!s) { parts.push("sem dado"); return parts.join(" · "); }
  const dev = (name, lv, v, role) => {
    const r = FT_ROLE[role] ? ` (${FT_ROLE[role]})` : "";
    parts.push(lv ? `${name}: ${fmt(v, usvDigits(v))} µSv/h${r}` : `${name}: sem dado${r}`);
  };
  dev("GMC-800", s.gmc, s.gmcUSv, s.gmcRole);
  if (showRc) dev("RC102", s.rc, s.rcUSv, s.rcRole);
  if (s.away) parts.push("fora de casa");
  if (s.rain) parts.push(`chuva ${fmt(s.rain, 1)} mm/h`);
  return parts.join(" · ");
}

// 2.101.1: cada pedaço traduzido antes de juntar (a dica inteira não está na
// tradução); a anotação da pessoa fica como ela escreveu
const ftMarkTip = (m) => `${hhmm(m.t)} · ${m.kind === "nota" ? tr("anotação") + ": " + asIs(m.text) : tr(m.text)}`;

// trechos seguidos de um sinal (ex.: fora de casa) como [início, fim]
function ftRuns(flags) {
  const out = [];
  let i = 0;
  while (i < FT_N) {
    if (!flags[i]) { i++; continue; }
    let j = i;
    while (j < FT_N && flags[j]) j++;
    out.push([i, j]);
    i = j;
  }
  return out;
}

// frase do topo: tranquilo ou quantos avisos, e quando você saiu.
// Uma frase por span (a tradução é por frase inteira).
function ftSay(el, day, marks, label, nowIdx) {
  el.textContent = "";
  const alerts = marks.filter((m) => m.kind === "alerta").length;
  const above = day.filter((s) => s && (FT_RANK[s.home] > 1 || FT_RANK[s.pers] > 1)).length;
  const has = day.some((s) => s && (s.home || s.pers));
  const add = (t) => { const s = document.createElement("span"); s.textContent = t; el.appendChild(s); el.append(" "); };
  if (!has) add(`${label}: sem leitura ainda.`);
  else if (!alerts && !above) add(`${label}: dia tranquilo, tudo no nível seguro.`);
  else if (alerts === 1) add(`${label}: 1 alerta.`);
  else if (alerts) add(`${label}: ${alerts} alertas.`);
  else add(`${label}: ${above * 10} min acima do seguro, sem alerta.`);
  const away = ftRuns(day.map((s) => !!(s && s.away)));
  if (away.length) {
    const t = away.map(([a, b]) => `${ftHm(a)}–${b >= nowIdx && nowIdx < FT_N ? hhmm(Date.now()) : ftHm(b)}`).join(", ");
    add(`Fora de casa: ${t}.`);
  }
}

// liga a dica e o clique de uma fita (fn(i, el) devolve o texto do pedaço)
function ftHover(box, strips, tipOf, slotT) {
  // 2.101.8: um jogo de ouvintes por caixa (cada desenho tira os do anterior)
  if (box._ftAbort) box._ftAbort.abort();
  const ac = new AbortController(), sig = { signal: ac.signal };
  box._ftAbort = ac;
  const tip = document.createElement("div"), cur = document.createElement("div");
  tip.className = "ft-tip"; tip.hidden = true;
  cur.className = "ft-cursor"; cur.hidden = true;
  box.append(tip, cur);
  const at = (e) => {
    const st = e.target.closest(".ft-strip") || strips[0];
    if (!st) return null;
    const r = st.getBoundingClientRect(), x = e.clientX - r.left;
    if (x < 0 || x > r.width) return null;
    return { st, i: Math.min(FT_N - 1, Math.floor((x / r.width) * FT_N)) };
  };
  box.addEventListener("mousemove", (e) => {
    const mk = e.target.closest("[data-tip]");
    const p = at(e);
    if (!p && !mk) { tip.hidden = cur.hidden = true; return; }
    tip.textContent = mk ? mk.dataset.tip : tipOf(p.i, p.st);
    tip.hidden = cur.hidden = false;
    const fr = box.getBoundingClientRect();
    cur.style.left = e.clientX - fr.left + "px";
    tip.style.left = Math.min(fr.width - tip.offsetWidth, Math.max(0, e.clientX - fr.left - tip.offsetWidth / 2)) + "px";
    tip.style.top = e.clientY - fr.top + 16 + "px";
  }, sig);
  box.addEventListener("mouseleave", () => { tip.hidden = cur.hidden = true; }, sig);
  box.addEventListener("click", (e) => {
    const mk = e.target.closest("[data-t]");
    const p = at(e);
    const t = mk ? +mk.dataset.t : p ? slotT(p.i, p.st) : null;
    if (t == null) return;
    openGapOnChart({ from: t, to: t + 6e5 });
  }, sig);
}

// 2.90.0 (avaliação A4): a legenda só traz o que aparece na fita
// (used: conjunto de chaves); sem used, mostra tudo
function ftLegend(el, used) {
  const sw = (bg, op) => `<i class="ft-sw" style="background:${bg}${op ? ";opacity:" + op : ""}"></i>`;
  const items = [["seguro", `${sw("var(--ok)", ".55")}seguro`], ["atencao", `${ftShape("atencao", 11)}atenção`], ["alarme", `${ftShape("alarme", 11)}alarme`],
    ["perigo", `${ftShape("perigo", 11)}perigo`], ["none", `${sw("var(--ft-none)")}sem leitura`], ["away", `${sw("var(--away)")}fora de casa`], ["rain", `${sw("var(--info)")}chuva`], ["nota", `${FT_PEN}anotação`]];
  el.innerHTML = items.filter(([k]) => !used || used.has(k)).map(([, h]) => `<span>${h}</span>`).join("");
}
// o que aparece na fita, para a legenda
function ftUsed(levelsList, nowIdx, marks, away, rain) {
  const u = new Set();
  for (const lv of levelsList) for (let i = 0; i < Math.min(nowIdx, FT_N); i++) u.add(lv[i] && FT_COL[lv[i]] ? lv[i] : "none");
  for (const m of marks || []) u.add(m.kind === "nota" ? "nota" : LV_PATH[m.level] ? m.level : "atencao");
  if (away && away.some(Boolean)) u.add("away");
  if (rain && rain.some(Boolean)) u.add("rain");
  return u;
}
// falha escrita (A4): trechos de 30 min ou mais sem leitura de um aparelho,
// até agora ("GMC-800 sem leitura das 10:00 às 11:20")
function ftGapLines(name, levels, nowIdx) {
  const end = Math.min(nowIdx, FT_N), out = [];
  const first = levels.findIndex(Boolean);
  if (first < 0) return out;
  let i = first;
  while (i < end) {
    if (levels[i]) { i++; continue; }
    let j = i;
    while (j < end && !levels[j]) j++;
    if (j - i >= 3) out.push(j >= nowIdx ? `${name} sem leitura desde ${ftHm(i)}.` : `${name} sem leitura das ${ftHm(i)} às ${ftHm(j)}.`);
    i = j;
  }
  return out.slice(-3);
}

const ftHours = (marks) => `<div class="ft-hours"><span></span><div>${marks.map((h) => `<span>${pad(h)}</span>`).join("")}</div></div>`;

// linha de um aparelho: nome com a bolinha, papel embaixo (ou "papel mudou"
// e a faixinha de papel sob a fita), marcas de nível em cima
function ftDevLane(name, color, levels, roles, nowIdx, lane) {
  const runs = [];
  let i = 0;
  while (i < FT_N) {
    let j = i;
    while (j < FT_N && roles[j] === roles[i]) j++;
    if (roles[i]) runs.push([i, j, roles[i]]);
    i = j;
  }
  const kinds = new Set(runs.map((r) => r[2]));
  const changed = kinds.size > 1;
  const sub = changed ? "papel mudou" : FT_ROLE[[...kinds][0]] || "";
  const band = changed ? `<div class="ft-roles">${runs.map(([a, b, r]) =>
    `<span class="${r}" style="left:${(a / FT_N) * 100}%;width:${((b - a) / FT_N) * 100}%" title="${FT_ROLE[r] || r}"><b class="ft-rl">${FT_ROLE[r] || r}</b><b class="ft-rs">${FT_ROLE_SHORT[r] || r}</b></span>`).join("")}</div>` : "";
  return `<div class="ft-lane"><span class="ft-lb ft-dev"><b><i style="background:${color}"></i>${name}</b>${sub ? `<small>${sub}</small>` : ""}</span>` +
    `<div><div class="ft-marks ft-over">${ftShapesOver(levels)}</div><div class="ft-strip" data-lane="${lane}">${ftStripSvg(levels, nowIdx)}</div>${band}</div></div>`;
}

// um dia: marcas, uma linha por aparelho (2.83.9), fora de casa e chuva
function renderFitaDay(box, strip, day0, opt = {}) {
  const day = ftDay(strip, day0);
  const now = Date.now();
  const nowIdx = now < day0 + 864e5 ? Math.max(0, Math.ceil((now - day0) / 6e5)) : FT_N;
  const marks = (strip && strip.marks) || [];
  const gmc = day.map((s) => (s && s.gmc) || null), rc = day.map((s) => (s && s.rc) || null);
  const gmcRole = day.map((s) => (s && s.gmcRole) || null), rcRole = day.map((s) => (s && s.rcRole) || null);
  // o RC102 só aparece se ele é usado (ou mediu neste dia)
  const showRc = rc.some(Boolean) || (typeof hasRcData === "function" && hasRcData());
  const away = day.map((s) => !!(s && s.away)), rain = day.map((s) => !!(s && s.rain));
  if (opt.say) ftSay(opt.say, day, marks, opt.label || "Hoje", nowIdx);
  const mk = marks.map((m) => {
    const i = Math.floor((m.t - day0) / 6e5);
    if (i < 0 || i >= FT_N) return "";
    const ico = m.kind === "nota" ? FT_PEN : LV_PATH[m.level] ? ftShape(m.level, 13) : ftShape("atencao", 13);
    return `<span class="ft-mk ft-mark" style="left:${((i + 0.5) / FT_N) * 100}%" data-t="${m.t}" data-tip="${escHtml(ftMarkTip(m))}">${ico}</span>`;
  }).join("");
  let h = "";
  if (mk) h += `<div class="ft-lane"><span class="ft-lb">marcas</span><div class="ft-marks">${mk}</div></div>`;
  h += ftDevLane("GQ GMC-800", "var(--gmc)", gmc, gmcRole, nowIdx, "gmc");
  if (showRc) h += ftDevLane("Radiacode 102", "var(--rc)", rc, rcRole, nowIdx, "rc");
  if (away.some(Boolean)) h += `<div class="ft-lane"><span class="ft-lb">fora de casa</span><div class="ft-strip thin">${ftBandSvg(away, "var(--away)")}</div></div>`;
  if (rain.some(Boolean)) h += `<div class="ft-lane"><span class="ft-lb">chuva</span><div class="ft-strip thin">${ftBandSvg(rain, "var(--info)")}</div></div>`;
  h += ftHours([0, 4, 8, 12, 16, 20, 24]);
  const gaps = [...ftGapLines("GMC-800", gmc, nowIdx), ...(showRc ? ftGapLines("RC102", rc, nowIdx) : [])];
  if (gaps.length) h += `<p class="ft-fail small">${gaps.map((g) => `<span>${escHtml(g)}</span>`).join(" ")}</p>`;
  box.innerHTML = h;
  ftHover(box, [...box.querySelectorAll(".ft-strip")], (i) => ftSlotTip(day[i], i, showRc), (i) => (i < nowIdx ? day0 + i * 6e5 : null));
  return ftUsed(showRc ? [gmc, rc] : [gmc], nowIdx, marks, away, rain);
}

// vários dias empilhados (semana no Agora; dias do período no Resumo)
function renderFitaWeek(box, strips, days, opt = {}) {
  let h = "";
  const rows = days.map((d0) => {
    const day = ftDay(strips, d0);
    const lv = day.map((s) => {
      if (!s) return null;
      const a = FT_RANK[s.home] || 0, b = FT_RANK[s.pers] || 0;
      return a || b ? (a >= b ? s.home : s.pers) : null;
    });
    return { d0, day, lv };
  });
  const now = Date.now();
  let alerts = 0, above = 0;
  for (const r of rows) {
    const nowIdx = now < r.d0 + 864e5 ? Math.max(0, Math.ceil((now - r.d0) / 6e5)) : FT_N;
    const wd = new Date(r.d0).toLocaleDateString(LOCALE, { weekday: "short" }).replace(".", "");
    h += `<div class="ft-lane"><span class="ft-lb">${wd} ${ddmm(r.d0)}</span><div class="ft-strip" data-d0="${r.d0}">${ftStripSvg(r.lv, nowIdx)}</div></div>`;
    const n = ((strips && strips.marks) || []).filter((m) => m.kind === "alerta" && m.t >= r.d0 && m.t < r.d0 + 864e5).length;
    alerts += n;
    if (r.lv.some((x) => FT_RANK[x] > 1)) above++;
  }
  h += ftHours([0, 6, 12, 18, 24]);
  h += '<p class="ft-note small">Cada linha junta casa e você: vale o nível mais alto dos dois.</p>';
  box.innerHTML = h;
  if (opt.say) {
    opt.say.textContent = "";
    const s = document.createElement("span");
    s.textContent = !alerts && !above ? `${opt.label || "Semana"}: tudo no nível seguro.`
      : !above && alerts === 1 ? `${opt.label || "Semana"}: 1 alerta curto, nenhum trecho de 10 min acima do seguro.`
      : !above ? `${opt.label || "Semana"}: ${alerts} alertas curtos, nenhum trecho de 10 min acima do seguro.`
      : alerts === 1 && above === 1 ? `${opt.label || "Semana"}: 1 alerta, 1 dia com trecho acima do seguro.`
      : alerts === 1 ? `${opt.label || "Semana"}: 1 alerta, ${above} dias com trecho acima do seguro.`
      : above === 1 ? `${opt.label || "Semana"}: ${alerts} alertas, 1 dia com trecho acima do seguro.`
      : `${opt.label || "Semana"}: ${alerts} alertas, ${above} dias com trecho acima do seguro.`;
    opt.say.appendChild(s);
  }
  const used = new Set();
  for (const r of rows) {
    const nowIdx = now < r.d0 + 864e5 ? Math.max(0, Math.ceil((now - r.d0) / 6e5)) : FT_N;
    ftUsed([r.lv], nowIdx).forEach((k) => used.add(k));
  }
  const byD0 = new Map(rows.map((r) => [r.d0, r]));
  ftHover(box, [...box.querySelectorAll(".ft-strip")], (i, st) => {
    const r = byD0.get(+st.dataset.d0);
    return `${ddmm(r.d0)} ${ftSlotTip(r.day[i], i)}`;
  }, (i, st) => {
    const t = +st.dataset.d0 + i * 6e5;
    return t < now ? t : null;
  });
  return used;
}

// junta várias respostas de GetDayStrip (de até 8 dias cada)
async function ftFetch(from, to) {
  const out = { slots: [], marks: [] };
  for (let f = from; f < to; f += 7 * 864e5) {
    const s = await api.GetDayStrip(f, Math.min(to, f + 7 * 864e5));
    if (!s) continue;
    out.slots.push(...(s.slots || []));
    out.marks.push(...(s.marks || []));
  }
  return out;
}

// ---------- no Agora ----------

let ftSeq = 0;
const ftMode = () => (localGet("ftMode", "dia") === "semana" ? "semana" : "dia");

async function refreshAgFita() {
  if (!api.GetDayStrip) return;
  const my = ++ftSeq, mode = ftMode();
  document.querySelectorAll("#ftSeg button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.ft === mode)));
  const d0 = startDay(Date.now());
  const from = mode === "semana" ? addDays(d0, -6) : d0;
  let s;
  try { s = await ftFetch(from, Date.now()); } catch (e) { return; }
  if (my !== ftSeq) return;
  const box = $("ftBox");
  let used;
  if (mode === "semana") {
    const days = [];
    for (let i = 0; i < 7; i++) days.push(addDays(from, i));
    used = renderFitaWeek(box, s, days, { say: $("ftSay"), label: "Semana" });
  } else used = renderFitaDay(box, s, d0, { say: $("ftSay"), label: "Hoje" });
  ftLegend($("ftLeg"), used);
}

// ---------- no Resumo ----------

let ftSumSeq = 0;
async function renderSumFita(per) {
  const box = $("sumFita"), say = $("sumFitaSay");
  if (!box || !api.GetDayStrip) return;
  const my = ++ftSumSeq;
  const d0 = startDay(per[0]);
  const days = [];
  for (let d = d0; d < per[1] && days.length < 31; d = addDays(d, 1)) days.push(d);
  if (!days.length) { box.parentElement.hidden = true; return; }
  let s;
  try { s = await ftFetch(d0, Math.min(per[1], addDays(days[days.length - 1], 1))); } catch (e) { return; }
  if (my !== ftSumSeq) return;
  box.parentElement.hidden = false;
  const label = per[2].charAt(0).toUpperCase() + per[2].slice(1);
  const used = days.length === 1 ? renderFitaDay(box, s, d0, { say, label }) : renderFitaWeek(box, s, days, { say, label });
  ftLegend($("sumFitaLeg"), used);
}

document.addEventListener("click", (e) => {
  const b = e.target.closest("#ftSeg button");
  if (!b) return;
  localSet("ftMode", b.dataset.ft);
  refreshAgFita();
});
