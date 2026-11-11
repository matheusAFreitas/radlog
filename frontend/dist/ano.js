"use strict";

// ---------- calendário do ano (Gráficos › Ano, 2.53) ----------
//
// Um quadradinho por dia, pintado pelo dia comparado com a sua média do ano
// (fundo de casa do GMC-800 ou a dose do RC102). Forma do nível no canto
// dos dias com alarme, risquinha azul nos dias de chuva, borda lilás quando
// o RC102 saiu de casa. Spec: ux/calendario.html.
//
// Usa os globais do app.js (state, api, $, fmt, localGet, localSet, lvIcon,
// showRange). Carregado antes do app.js: só define funções.

const YC = {
  mode: "bg", year: 0, rain: true, alr: true, sel: null, data: null, at: 0, seq: 0,
  CELL: 12, GAP: 2, L: 30, T: 18,
  RAMP: ["#17271F", "#1F4030", "#2C6447", "#43986A", "#6EE7A0"],
  LV: { atencao: "#F5B84A", alarme: "#F28C38", perigo: "#F0656A" },
  LVN: { atencao: "atenção", alarme: "alarme", perigo: "perigo" },
};

function ycInit() {
  YC.mode = localGet("yc.mode", "bg") === "dose" ? "dose" : "bg";
  YC.rain = localGet("yc.rain", "1") !== "0";
  YC.alr = localGet("yc.alr", "1") !== "0";
  $("ycRain").checked = YC.rain;
  $("ycAlr").checked = YC.alr;
  ycSeg();
  $("ycMode").addEventListener("click", (e) => {
    const b = e.target.closest("button[data-v]");
    if (!b || b.dataset.v === YC.mode) return;
    YC.mode = b.dataset.v; localSet("yc.mode", YC.mode); ycSeg(); ycRender();
  });
  $("ycRain").addEventListener("change", (e) => { YC.rain = e.target.checked; localSet("yc.rain", YC.rain ? "1" : "0"); ycRender(); });
  $("ycAlr").addEventListener("change", (e) => { YC.alr = e.target.checked; localSet("yc.alr", YC.alr ? "1" : "0"); ycRender(); });
  $("ycPrev").addEventListener("click", () => ycGo(YC.year - 1));
  $("ycNext").addEventListener("click", () => ycGo(YC.year + 1));
  $("ycTblBtn").addEventListener("click", (e) => {
    const t = $("ycTbl"); t.hidden = !t.hidden;
    e.currentTarget.setAttribute("aria-expanded", String(!t.hidden));
    e.currentTarget.textContent = t.hidden ? "Ver como tabela" : "Esconder tabela";
  });
  const svg = $("ycCal"), tip = $("ycTip");
  svg.addEventListener("mousemove", (e) => {
    const r = e.target.closest(".yc-c");
    if (!r || !YC.data) { tip.hidden = true; return; }
    tip.innerHTML = ycDayHtml(YC.days[+r.dataset.i]).join("");
    tip.hidden = false;
    const box = $("ycCard").getBoundingClientRect();
    let x = e.clientX - box.left + 14;
    if (e.clientX + 14 + tip.offsetWidth > window.innerWidth - 8) x = e.clientX - box.left - 14 - tip.offsetWidth;
    tip.style.left = x + "px"; tip.style.top = (e.clientY - box.top + 14) + "px";
  });
  svg.addEventListener("mouseleave", () => { tip.hidden = true; });
  svg.addEventListener("click", (e) => {
    const r = e.target.closest(".yc-c");
    if (!r) return;
    YC.sel = +r.dataset.i; ycRender();
  });
  svg.addEventListener("keydown", (e) => {
    if (!YC.days) return;
    const n = YC.days.length, mv = { ArrowLeft: -7, ArrowRight: 7, ArrowUp: -1, ArrowDown: 1 }[e.key];
    if (mv != null) {
      e.preventDefault();
      YC.sel = Math.max(0, Math.min(n - 1, (YC.sel == null ? n - 1 : YC.sel + mv)));
      ycRender(); svg.focus();
    } else if (e.key === "Enter" && YC.sel != null) {
      e.preventDefault(); ycOpenDay(YC.days[YC.sel]);
    } else if (e.key === "Escape") { YC.sel = null; ycRender(); }
  });
  $("ycDay").addEventListener("click", (e) => {
    if (e.target.closest("#ycOpenDay") && YC.sel != null) ycOpenDay(YC.days[YC.sel]);
  });
  $("ycSay").addEventListener("click", (e) => {
    const b = e.target.closest("[data-go]");
    if (b) { YC.sel = +b.dataset.go; ycRender(); $("ycDay").scrollIntoView({ behavior: "smooth", block: "nearest" }); }
  });
}

function ycSeg() {
  $("ycMode").querySelectorAll("button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.v === YC.mode)));
}

function ycGo(y) {
  if (!YC.data || y < YC.data.first || y > new Date().getFullYear()) return;
  YC.year = y; YC.sel = null; YC.data = null; refreshYearCal();
}

async function refreshYearCal() {
  if (!api.GetYearCal || state.view !== "graficos" || state.sub.graficos !== "ano") return;
  const y = YC.year || new Date().getFullYear();
  if (YC.data && YC.data.year === y && Date.now() - YC.at < 60e3) { ycRender(); return; }
  const my = ++YC.seq;
  let c;
  try { c = await api.GetYearCal(y); } catch (e) { return; }
  if (my !== YC.seq || !c) return;
  YC.data = c; YC.year = c.year; YC.at = Date.now();
  ycRender();
}

// ---------- contas ----------

const ycMed = (a) => {
  const v = a.slice().sort((x, y) => x - y), n = v.length;
  return n ? (n % 2 ? v[(n - 1) / 2] : (v[n / 2 - 1] + v[n / 2]) / 2) : NaN;
};
const ycKey = (d) => (YC.mode === "bg" ? d.bg : d.dose);
const ycMin = (d) => (YC.mode === "bg" ? d.n : d.doseN || 0);
const ycNone = (d) => !d || d.future || !(ycMin(d) > 0) || !(ycKey(d) > 0);
const ycFull = (d) => !ycNone(d) && ycMin(d) >= 720;
const ycPct = (d, ref) => (ycKey(d) / ref - 1) * 100;
function ycBin(p) {
  const b = YC.mode === "bg" ? [-6, -2, 2, 6] : [-30, -10, 10, 30];
  return p < b[0] ? 0 : p < b[1] ? 1 : p <= b[2] ? 2 : p <= b[3] ? 3 : 4;
}
const ycDate = (d) => {
  const s = new Date(d.day).toLocaleDateString(LOCALE, { weekday: "short", day: "2-digit", month: "2-digit", year: "numeric" });
  return s.replace(/\./g, "");
};
const ycMonthName = (m) => new Date(2000, m, 1).toLocaleDateString(LOCALE, { month: "short" }).replace(/\./g, "");
const ycUSv = (v) => fmt(v, v < 10 ? 2 : 1);

// o ano inteiro (dias que ainda não chegaram entram vazios)
function ycDays(c) {
  const out = [], have = new Map(c.days.map((d) => [d.day, d]));
  for (let t = new Date(c.year, 0, 1); t.getFullYear() === c.year; t.setDate(t.getDate() + 1)) {
    const k = t.getTime();
    out.push(have.get(k) || { day: k, n: 0, future: k > c.today });
  }
  return out;
}

// ---------- tela ----------

function ycRender() {
  const c = YC.data;
  if (!c) return;
  const days = YC.days = ycDays(c);
  const thisYear = new Date().getFullYear();
  $("ycYear").textContent = c.year;
  $("ycPrev").disabled = c.year <= c.first;
  $("ycNext").disabled = c.year >= thisYear;
  $("ycTitle").textContent = YC.mode === "bg" ? "Fundo de casa, dia a dia" : "Minha dose, dia a dia";
  const ok = days.filter(ycFull), ref = ycMed(ok.map(ycKey)), withData = days.filter((d) => !ycNone(d)).length;
  YC.ref = ref;
  const sub = $("ycSub");
  if (!withData) sub.textContent = YC.mode === "bg" ? "sem dados do GMC-800 neste ano" : "sem dados do RC102 neste ano";
  else {
    const a = isFinite(ref) ? (YC.mode === "bg" ? `sua média do ano: ${fmt(ref, 3)} µSv/h` : `sua média do ano: ${fmt(ref, 2)} µSv por dia`) : "sem um dia com 12 h de dado ainda";
    sub.innerHTML = `<span></span> · <span></span>`;
    sub.children[0].textContent = a;
    sub.children[1].textContent = withData === 1 ? "1 dia com dado" : `${withData} dias com dado`;
  }
  ycDraw(days, ref);
  ycLegend();
  ycMonths(days);
  ycSay(days, ref, ok);
  ycTable(days);
  ycShowDay(days);
}

function ycDraw(days, ref) {
  const { CELL, GAP, L, T, RAMP } = YC, svg = $("ycCal");
  const jan1 = new Date(YC.data.year, 0, 1), off = (jan1.getDay() + 6) % 7;
  let h = "";
  for (let m = 0; m < 12; m++) {
    const i = Math.round((new Date(YC.data.year, m, 1) - jan1) / 864e5) + off;
    h += `<text class="yc-lbl" x="${L + Math.floor(i / 7) * (CELL + GAP)}" y="11">${ycMonthName(m)}</text>`;
  }
  // seg, qua, sex
  [0, 2, 4].forEach((r) => {
    const wd = new Date(2024, 0, 1 + r).toLocaleDateString(LOCALE, { weekday: "short" }).replace(/\./g, "");
    h += `<text class="yc-lbl" x="0" y="${T + r * (CELL + GAP) + 10}">${wd}</text>`;
  });
  days.forEach((d, idx) => {
    const i = idx + off, x = L + Math.floor(i / 7) * (CELL + GAP), y = T + (i % 7) * (CELL + GAP);
    const sel = YC.sel === idx ? " sel" : "";
    if (d.future) { h += `<rect x="${x + .5}" y="${y + .5}" width="${CELL - 1}" height="${CELL - 1}" rx="2.5" class="yc-fut"/>`; return; }
    if (ycNone(d) || !isFinite(ref)) {
      h += `<rect class="yc-c yc-none${sel}" data-i="${idx}" x="${x + .5}" y="${y + .5}" width="${CELL - 1}" height="${CELL - 1}" rx="2.5"/>`;
    } else {
      const b = ycBin(ycPct(d, ref)), fill = RAMP[Math.max(b, ycFull(d) ? 0 : 1)], op = ycFull(d) ? 1 : .5;
      const away = YC.mode === "dose" && d.away;
      const g = away ? .75 : 0;
      h += `<rect class="yc-c${away ? " yc-away" : ""}${sel}" data-i="${idx}" x="${x + g}" y="${y + g}" width="${CELL - 2 * g}" height="${CELL - 2 * g}" rx="3" fill="${fill}" fill-opacity="${op}"${op < 1 && !away ? ` stroke="${fill}" stroke-width="1"` : ""}/>`;
    }
    if (YC.rain && d.rain) h += `<rect x="${x + 2}" y="${y + CELL - 2.5}" width="${CELL - 4}" height="1.6" rx=".8" class="yc-rain"/>`;
    if (YC.alr && d.lvl && LV_PATH[d.lvl]) {
      h += `<g transform="translate(${x + 1.5},${y + 1.5}) scale(.56)" class="yc-lv" fill="${YC.LV[d.lvl]}" stroke="#0B0E10" stroke-width="2.2" paint-order="stroke">${LV_PATH[d.lvl]}</g>`;
    }
  });
  const cols = Math.ceil((days.length + off) / 7), w = L + cols * (CELL + GAP), hh = T + 7 * (CELL + GAP);
  svg.setAttribute("viewBox", `0 0 ${w} ${hh}`);
  svg.style.minWidth = w + "px";
  svg.innerHTML = h;
}

function ycLegend() {
  const sw = (fill, extra = "") => `<svg width="12" height="12" aria-hidden="true">${fill}${extra}</svg>`;
  let r = `<span class="yc-it"><span>bem abaixo</span><span class="yc-ramp" aria-hidden="true">${YC.RAMP.map((c) => `<i style="background:${c}"></i>`).join("")}</span><span>bem acima da sua média</span></span>`;
  r += `<span class="yc-it">${sw('<rect x=".5" y=".5" width="11" height="11" rx="2.5" class="yc-none"/>')}<span>sem dado</span></span>`;
  r += `<span class="yc-it">${sw('<rect width="12" height="12" rx="3" fill="#43986A" fill-opacity=".45"/>')}<span>menos de 12 h de dado</span></span>`;
  if (YC.rain) r += `<span class="yc-it">${sw('<rect width="12" height="12" rx="3" fill="#1F4030"/>', '<rect x="2" y="9.5" width="8" height="1.6" rx=".8" class="yc-rain"/>')}<span>choveu</span></span>`;
  if (YC.alr) r += `<span class="yc-it">${lvIcon("atencao", 12)}${lvIcon("alarme", 12)}${lvIcon("perigo", 12)}<span>nível mais grave do dia</span></span>`;
  if (YC.mode === "dose") r += `<span class="yc-it">${sw('<rect x=".75" y=".75" width="10.5" height="10.5" rx="3" fill="#2C6447" class="yc-away"/>')}<span>RC102 fora de casa</span></span>`;
  $("ycLegend").innerHTML = r;
}

function ycByMonth(days) {
  const a = [];
  for (let m = 0; m < 12; m++) a.push(days.filter((d) => new Date(d.day).getMonth() === m && !ycNone(d)));
  return a;
}

function ycMonths(days) {
  const bm = ycByMonth(days), vals = bm.map((a) => (a.length ? ycMed(a.map(ycKey)) : null));
  const have = vals.filter((v) => v != null);
  const el = $("ycMonths");
  if (!have.length) { el.innerHTML = ""; return; }
  const lo = Math.min(...have) * .97, hi = Math.max(...have);
  const unit = YC.mode === "bg" ? "µSv/h" : "µSv/dia", dec = YC.mode === "bg" ? 3 : 2;
  el.innerHTML = vals.map((v, m) => {
    const name = ycMonthName(m);
    if (v == null) return `<div class="yc-m none" title="${name}: ${tr("sem dado")}"><div class="bar"></div><span>${name}</span></div>`;
    const hgt = hi > lo ? Math.max(6, (v - lo) / (hi - lo) * 70) : 40;
    const t = `${name}: ${tr(`mediana ${fmt(v, dec)} ${unit}`)} · ${tr(bm[m].length === 1 ? "1 dia com dado" : `${bm[m].length} dias com dado`)}`;
    return `<div class="yc-m" title="${t}"><div class="bar" style="height:${hgt}px"></div><span>${name}</span></div>`;
  }).join("");
}

function ycSay(days, ref, ok) {
  const el = $("ycSay"), p = [];
  const nM = ycByMonth(days).filter((a) => a.length).length;
  const pc = (a, b) => fmt(Math.abs((a / b - 1) * 100), 1);
  const mo = (d) => new Date(d.day).getMonth();
  if (ok.length && isFinite(ref)) {
    if (YC.mode === "bg") {
      const cold = ok.filter((d) => mo(d) >= 5 && mo(d) <= 7), hot = ok.filter((d) => mo(d) <= 2 || mo(d) >= 10);
      if (nM >= 6 && cold.length >= 10 && hot.length >= 10) {
        const a = ycMed(cold.map(ycKey)), b = ycMed(hot.map(ycKey));
        p.push(a >= b ? `Junho a agosto ficaram ${pc(a, b)}% acima de novembro a março. Com a casa mais fechada no frio, o radônio se acumula mais.`
          : `Junho a agosto ficaram ${pc(a, b)}% abaixo de novembro a março.`);
      }
      const wet = ok.filter((d) => d.rain), dry = ok.filter((d) => !d.rain);
      if (wet.length >= 5 && dry.length >= 5) {
        const a = ycMed(wet.map(ycKey)), b = ycMed(dry.map(ycKey));
        p.push(a >= b ? `Nos dias de chuva o fundo ficou ${pc(a, b)}% acima dos dias secos. É a chuva trazendo para o chão os produtos do radônio do ar.`
          : `Nos dias de chuva o fundo ficou ${pc(a, b)}% abaixo dos dias secos.`);
      }
      const top = ok.reduce((a, b) => (ycKey(b) > ycKey(a) ? b : a));
      p.push({ t: `O dia mais alto foi ${ycDate(top)}, ${fmt(ycPct(top, ref), 1)}% acima da sua média.`, go: days.indexOf(top) });
    } else {
      const tot = days.filter((d) => !ycNone(d)).reduce((s, d) => s + d.dose, 0);
      p.push(`Somando os dias com dado, o RC102 marcou ${fmt(tot / 1000, 2)} mSv no ano até agora.`);
      const aw = ok.filter((d) => d.away), hm = ok.filter((d) => !d.away);
      if (aw.length >= 5 && hm.length >= 5) {
        const a = ycMed(aw.map(ycKey)), b = ycMed(hm.map(ycKey));
        p.push(a < b ? `Nos dias com saída de casa a dose ficou ${fmt(Math.abs((a / b - 1) * 100), 0)}% abaixo dos dias só em casa. Fora, o fundo costuma ser mais baixo que dentro de casa, por causa do radônio.`
          : `Nos dias com saída de casa a dose ficou ${fmt(Math.abs((a / b - 1) * 100), 0)}% acima dos dias só em casa.`);
      }
      const top = ok.reduce((a, b) => (ycKey(b) > ycKey(a) ? b : a));
      p.push({ t: `O dia de maior dose foi ${ycDate(top)}: ${fmt(top.dose, 1)} µSv.`, go: days.indexOf(top) });
    }
  }
  if (nM && nM < 12) p.push({ t: nM === 1 ? "Ainda é 1 mês. O calendário fica mais útil depois de um ano." : `Ainda são ${nM} meses. O calendário fica mais útil depois de um ano.`, small: true });
  el.innerHTML = p.length ? `<h3 class="sub-head">O que o ano mostrou</h3>` : "";
  for (const x of p) {
    const e = document.createElement("p");
    e.className = "small" + (x.small ? " muted" : "");
    e.textContent = typeof x === "string" ? x : x.t;
    el.appendChild(e);
    if (x.go != null) {
      const b = document.createElement("button");
      b.type = "button"; b.className = "link small-link"; b.dataset.go = x.go; b.textContent = "ver este dia";
      e.append(" "); e.appendChild(b);
    }
  }
}

function ycTable(days) {
  const bm = ycByMonth(days), unit = YC.mode === "bg" ? "µSv/h" : "µSv/dia", dec = YC.mode === "bg" ? 3 : 2;
  let h = `<table class="yc-table"><thead><tr><th>Mês</th><th class="n">Mediana (${unit})</th><th class="n">Dia mais baixo</th><th class="n">Dia mais alto</th><th class="n">Dias com alarme</th><th class="n">Dias de chuva</th><th class="n">Dias com dado</th></tr></thead><tbody>`;
  bm.forEach((a, m) => {
    if (!a.length) { h += `<tr><td>${ycMonthName(m)}</td><td colspan="6" class="muted">sem dado</td></tr>`; return; }
    const v = a.map(ycKey);
    h += `<tr><td>${ycMonthName(m)}</td><td class="n mono">${fmt(ycMed(v), dec)}</td><td class="n mono">${fmt(Math.min(...v), dec)}</td><td class="n mono">${fmt(Math.max(...v), dec)}</td>` +
      `<td class="n mono">${a.filter((d) => d.lvl).length}</td><td class="n mono">${a.filter((d) => d.rain).length}</td><td class="n mono">${a.length}</td></tr>`;
  });
  $("ycTbl").innerHTML = h + "</tbody></table>";
}

// as linhas do dia (cada uma num elemento, para a tradução pegar inteira)
function ycDayHtml(d) {
  const L = [`<b data-no-tr>${ycDate(d)}</b>`];
  const line = (t, cls = "") => `<div${cls ? ` class="${cls}"` : ""}>${t}</div>`;
  if (ycNone(d)) {
    L[0] = line(L[0]);
    L.push(line(d.future ? "ainda não chegou" : YC.mode === "bg" ? "sem dado do GMC-800: o app estava fechado ou o contador desligado" : "sem dado do RC102 neste dia", "muted"));
  } else {
    L[0] = line(L[0]);
    L.push(line(YC.mode === "bg" ? `fundo ${fmt(d.bg, 3)} µSv/h (mediana do dia)` : `dose ${fmt(d.dose, 2)} µSv no RC102`));
    if (isFinite(YC.ref)) {
      const p = ycPct(d, YC.ref);
      L.push(line(Math.abs(p) < .05 ? "na sua média" : p > 0 ? `${fmt(p, 1)}% acima da sua média` : `${fmt(-p, 1)}% abaixo da sua média`, "muted"));
    }
    if (!ycFull(d)) L.push(line(`só ${fmt(ycMin(d) / 60, 1)} h de dado`, "muted"));
  }
  if (d.mm > 0) L.push(line(`choveu ${fmt(d.mm, 1)} mm`, "muted"));
  if (d.lvl) {
    const at = d.peakAt ? hhmm(d.peakAt) : "";
    const pk = d.peak > 0 ? (at ? `pico ${ycUSv(d.peak)} µSv/h às ${at}` : `pico ${ycUSv(d.peak)} µSv/h`) : "";
    L.push(`<div class="yc-lvl" style="color:${YC.LV[d.lvl]}">${lvIcon(d.lvl, 12)}<span>${YC.LVN[d.lvl]}</span>${pk ? `<span class="yc-dot">·</span><span>${pk}</span>` : ""}${d.peakRC ? `<span class="muted">(RC102)</span>` : ""}</div>`);
  }
  if (d.away && YC.mode === "dose") L.push(line("RC102 saiu de casa", "yc-away-t"));
  return L;
}

function ycShowDay(days) {
  const el = $("ycDay");
  if (YC.sel == null || !days[YC.sel]) { el.hidden = true; el.innerHTML = ""; return; }
  const d = days[YC.sel];
  el.hidden = false;
  el.innerHTML = `<div class="yc-day-lines">${ycDayHtml(d).join("")}</div>` +
    (d.future ? "" : `<button type="button" class="btn small" id="ycOpenDay">Abrir este dia nos gráficos</button>`);
}

function ycOpenDay(d) {
  if (!d || d.future) return;
  const end = new Date(d.day); end.setDate(end.getDate() + 1);
  showRange(d.day, Math.min(end.getTime(), Date.now()));
}
document.addEventListener("DOMContentLoaded", ycInit);
