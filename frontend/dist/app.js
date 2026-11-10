"use strict";

// ---------- ponte com o Go (ou modo demonstração no navegador) ----------

// a Wails expõe os métodos em window.go.<pacote Go>.App (o pacote é "app"
// desde a 1.13.4; antes era "main")
const goApp = window.go && ((window.go.app && window.go.app.App) || (window.go.main && window.go.main.App));
const hasGo = !!goApp;
const api = hasGo ? goApp : makeDemoApi();
const on = (name, fn) => {
  if (hasGo && window.runtime) window.runtime.EventsOn(name, fn);
  else demoBus[name] = (demoBus[name] || []).concat(fn);
};

// ---------- formatação ----------

const nf = (d) => new Intl.NumberFormat(LOCALE, { minimumFractionDigits: d, maximumFractionDigits: d });
const fmt = (v, d) => (v == null || !isFinite(v) ? "—" : nf(d).format(v));
const pad = (n) => String(n).padStart(2, "0");
const hhmm = (ms) => { const t = new Date(ms); return pad(t.getHours()) + ":" + pad(t.getMinutes()); };
const ddmm = (ms) => { const t = new Date(ms); return pad(t.getDate()) + "/" + pad(t.getMonth() + 1); };
const hhmmss = (ms) => { const t = new Date(ms); return hhmm(ms) + ":" + pad(t.getSeconds()); };
const isShortRange = () => state.range === "15m" || state.range === "30m";
const isLongRange = () => state.range === "7d" || state.range === "30d" || state.range === "1y";
const HOURLY = ["15m", "30m", "1h", "3h", "6h", "12h", "24h"];
const isToday = (ms) => new Date(ms).toDateString() === new Date().toDateString();
const isYesterday = (ms) => { const y = new Date(); y.setDate(y.getDate() - 1); return new Date(ms).toDateString() === y.toDateString(); };
const usvDigits = (v) => (v < 1 ? 3 : v < 10 ? 2 : 1);
const $ = (id) => document.getElementById(id);

// fator µSv/h por CPM no fundo: o do contador (ou o último lido dele); o
// 0,0101 é o de fábrica do GMC-800, só se o app ainda não leu nada
const calK = () => (state.status && (state.status.usvPerCPM || (state.status.device || {}).usvPerCPM)) || 0.0101;

// ---------- estado ----------

// 2.8: os dois aparelhos abrem em µSv/h; quem ainda estava no padrão antigo
// (CPM) passa para µSv/h uma vez, depois vale o que escolher
if (!localGet("unit28", "")) { localSet("unit", "usv"); localSet("unit28", "1"); }
const state = {
  unit: localGet("unit", "usv"),
  range: localGet("range", "1h"),
  zoom: null,      // { from, to } quando arrastou no gráfico
  zoomStack: [],   // zooms anteriores (botão Voltar)
  periods: [],     // fora de casa
  where: null,     // onde está cada aparelho (onde.js)
  measurements: [], // medições de objeto salvas
  doseZoom: null,
  doseZoomStack: [],
  // µSv/h é o gráfico principal desde a 2.29 (o de CPM ficou só de análise):
  // quem estava no CPM volta uma vez para o µSv/h
  chartUnit: localGet("unitUSv229", "") ? localGet("chartUnit", "usv") : (localSet("unitUSv229", "1"), localSet("chartUnit", "usv"), "usv"),
  mMode: localGet("mMode", "nova"),
  heatDays: parseInt(localGet("heatDays", "14"), 10) || 14,
  heatMonthMode: localGet("heatMode", "dias") === "mes",
  heatMonth: (() => { const d = new Date(); return { y: d.getFullYear(), m: d.getMonth() + 1 }; })(),
  heatDataStart: 0, // início do mês da leitura mais antiga (ms)
  dosePeriod: localGet("range", "1h"), // 2.12: o mesmo período da taxa
  doseLimit: localGet("doseLimit", "publico"),
  status: { state: "desconectado", message: "", device: {}, downloading: false, dataDir: "" },
  settings: {},
  last: null,
  alarm: false,
  demo: { active: false, cpm: 20, realistic: false },
  realAlarm: false,
  chart: null,
};

function localGet(k, def) { try { return localStorage.getItem("gmc." + k) || def; } catch (e) { return def; } }
function localSet(k, v) { try { localStorage.setItem("gmc." + k, v); } catch (e) { /* sem armazenamento */ } }

// ---------- avisos ----------

function toast(msg, err) {
  const el = document.createElement("div");
  el.className = "toast" + (err ? " err" : "");
  el.textContent = msg;
  $("toasts").appendChild(el);
  setTimeout(() => el.remove(), err ? 7000 : 4000);
}

async function call(fn, ...args) {
  try { return await fn(...args); }
  catch (e) { toast(String(e && e.message ? e.message : e), true); throw e; }
}

// ---------- leitura atual ----------

// Quão incomum é a leitura: (CPM − fundo) / √fundo, a variação natural de
// uma contagem de Poisson.
// Compara a leitura com o fundo de costume, em palavras. A contagem do Geiger
// varia sozinha: num fundo de N CPM, quase todas as leituras (95%) ficam entre
// N − 2√N e N + 2√N. Acima de N + 3√N dificilmente é acaso.
// σ (sigma): o tamanho típico da variação natural de uma contagem de
// Poisson. Num fundo de N CPM, 1σ = √N CPM.
const bgNow = () => (state.last && state.last.bgCPM) || state.bgCPM || null;
const sigmaOf = (cpm, bg) => (bg > 0 ? (cpm - bg) / Math.sqrt(bg) : null);
const fmtSig = (z) => (z == null ? "" : `${z >= 0 ? "+" : "−"}${fmt(Math.abs(z), 1)}σ`);
const sigTxt = (cpm) => { const z = sigmaOf(cpm, bgNow()); return z == null ? "" : ` (${fmtSig(z)})`; };

function renderSigma(r) {
  const el = $("readSigma");
  const num = $("readSigmaNum");
  if (!r || r.sigma == null || !r.bgCPM) { el.hidden = true; num.hidden = true; return; }
  $("readSigmaVal").textContent = `${fmtSig(r.sigma)} · fundo ${fmt(r.bgCPM, 0)} CPM · 1σ = ${fmt(Math.sqrt(r.bgCPM), 1)} CPM`;
  num.hidden = false;
  const z = r.sigma, bg = r.bgCPM, sd = Math.sqrt(bg);
  const lo = Math.max(0, Math.round(bg - 2 * sd)), hi = Math.round(bg + 2 * sd), far = Math.round(bg + 3 * sd);
  // a faixa em µSv/h (a conta do σ continua em contagens, no detalhe)
  const k = calK(), uu = (c) => fmt(c * k, usvDigits(hi * k));
  const range = `seu normal: ${uu(lo)} a ${uu(hi)} µSv/h`;
  let text, cls = "";
  if (z >= 3) { text = `Acima do normal (${range}) — dificilmente é só variação`; cls = "high"; }
  else if (z >= 2) { text = `Um pouco acima do normal (${range}) — pode ser só variação`; cls = "mid"; }
  else if (z <= -2) { text = `Abaixo do normal (${range}) — sem problema`; }
  else { text = `Dentro do normal — o nível de fundo costuma ficar entre ${uu(lo)} e ${uu(hi)} µSv/h`; }
  el.textContent = text;
  el.className = "sigma " + cls;
  el.title = `A contagem do Geiger varia sozinha de um minuto para outro. Pelas suas leituras dos últimos 30 dias, o fundo fica em ${fmt(bg, 0)} CPM e quase todas as leituras (95%) caem entre ${lo} e ${hi} CPM. Um pouco acima disso ainda pode ser acaso; acima de ${far} CPM dificilmente é.`;
  el.hidden = false;
}

// probabilidade de uma contagem de Poisson de média m dar k ou mais
function lgam(x) { // Lanczos
  const g = 7, c = [0.99999999999980993, 676.5203681218851, -1259.1392167224028, 771.32342877765313, -176.61502916214059, 12.507343278686905, -0.13857109526572012, 9.9843695780195716e-6, 1.5056327351493116e-7];
  if (x < 0.5) return Math.log(Math.PI / Math.sin(Math.PI * x)) - lgam(1 - x);
  x -= 1; let a = c[0]; const t = x + g + 0.5;
  for (let i = 1; i < 9; i++) a += c[i] / (x + i);
  return 0.5 * Math.log(2 * Math.PI) + (x + 0.5) * Math.log(t) - t + Math.log(a);
}
function poissonTail(k, m) { let s = 0; for (let j = 0; j < k; j++) s += Math.exp(j * Math.log(m) - m - lgam(j + 1)); return Math.max(0, 1 - s); }
function everyText(p) { // p por minuto → "N vezes por dia" ou "1 vez a cada X"
  const perDay = p * 1440;
  if (perDay >= 1.5) return `~${fmt(perDay, 0)} vezes por dia`;
  if (perDay >= 0.7) return "~1 vez por dia";
  const days = 1 / perDay;
  if (days < 60) return `~1 vez a cada ${fmt(days, 0)} dias`;
  if (days < 730) return `~1 vez a cada ${fmt(days / 30, 0)} meses`;
  return `~1 vez a cada ${fmt(days / 365, 0)} anos`;
}
function openSigmaHelp() {
  const bg = bgNow() || 28, sd = Math.sqrt(bg), r = state.last;
  const box = $("sigmaBody");
  const rows = [1, 2, 3, 4, 5].map((k) => {
    const thr = Math.ceil(bg + k * sd), p = poissonTail(thr, bg);
    return `<tr><td>+${k}σ</td><td>${fmt(thr, 0)} CPM ou mais</td><td>${p * 100 >= 0.1 ? fmt(p * 100, 1) : Number((p * 100).toPrecision(2)).toLocaleString(LOCALE, { maximumFractionDigits: 10 })}%</td><td>${everyText(p)}</td></tr>`;
  }).join("");
  const now = r && r.cpm != null ? `<p>Agora: <b class="mono">${fmt(r.cpm, 0)} CPM</b> → (${fmt(r.cpm, 0)} − ${fmt(bg, 0)}) ÷ ${fmt(sd, 1)} = <b class="mono">${fmtSig(sigmaOf(r.cpm, bg))}</b>.</p>` : "";
  box.innerHTML = `
    <p><b>σ (sigma)</b> é o desvio padrão: o tamanho típico da variação natural da contagem. Os decaimentos acontecem ao acaso, então o número de pulsos por minuto varia sozinho, mesmo sem nada mudar. Numa contagem assim (de Poisson), <b>1σ = raiz da média</b>.</p>
    <p>O nível de fundo aqui é <b class="mono">${fmt(bg, 1)} CPM</b> (mediana dos últimos 30 dias), então <b class="mono">1σ = √${fmt(bg, 1)} ≈ ${fmt(sd, 1)} CPM</b>. Uma leitura "em σ" é quantos desses passos ela está acima ou abaixo do fundo: (leitura − fundo) ÷ σ.</p>
    ${now}
    <h3>Quão raro é, pelo acaso, num minuto</h3>
    <div class="sg-tbl-wrap"><table class="m-stats mono sg-tbl"><thead><tr><th>acima de</th><th>com esse nível de fundo</th><th>chance por minuto</th><th>só por acaso</th></tr></thead><tbody>${rows}</tbody></table></div>
    <p class="small muted">Calculado com a distribuição de Poisson desse nível de fundo. Por isso até 3σ acontece todo dia: são 1.440 minutos. Um pico de 4σ ou mais já pede uma olhada (chuva? algo perto do contador?).</p>
    <h3>Onde mais aparece</h3>
    <ul class="sg-list">
      <li><b>± na medição de objeto</b>: a incerteza de 1σ de cada parte. 284,2 ± 7,5 CPM quer dizer que o valor de verdade está a ±7,5 em ~68% das vezes, ±15 em ~95%.</li>
      <li><b>A diferença em σ</b>: diferença ÷ incerteza combinada. 244,6 ± 8,0 = 30σ: na prática, impossível ser acaso. Abaixo de 2σ, pode ser só variação.</li>
      <li><b>Médias têm σ menor</b>: a média de 60 minutos varia √60 ≈ 7,7 vezes menos que 1 minuto (${fmt(Math.sqrt(bg / 60), 2)} CPM com esse nível de fundo). Por isso, no padrão por hora, +1,8 CPM numa hora já pode ser 2σ ou mais.</li>
      <li><b>Mais tempo, menos incerteza</b>: a incerteza relativa é 1 ÷ √contagens. 100 contagens dão ±10%; 10.000 dão ±1%.</li>
    </ul>`;
  $("sigmaDlg").showModal();
}
document.addEventListener("click", (e) => { if (e.target.closest("#btnSigmaHelp, .rc-sigma-help")) openSigmaHelp(); });

function renderReading() {
  renderGTile();
  const r = state.last;
  const connected = state.status.state === "conectado";
  const u = state.unit;
  const units = { cpm: "CPM", usv: "µSv/h", mrh: "mrem/h", cps: "CPS" };
  $("readUnit").textContent = units[u];
  document.querySelectorAll("#unitSeg button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.unit === u)));

  $("demoTag").hidden = !state.demo.active;
  if (!r || (!connected && !state.demo.active)) {
    $("readValue").textContent = "—";
    $("readAlt").textContent = connected ? "aguardando a primeira leitura…" : "sem contador conectado";
    $("readSigma").hidden = true;
    $("readSigmaNum").hidden = true;
    $("levelName").textContent = "—";
    $("levelName").className = "sr-only level-name";
    renderGmcDial(null, "", "");
    $("hero").classList.remove("alarm");
    return;
  }
  const vals = {
    cpm: fmt(r.cpm, 0),
    usv: fmt(r.usv, usvDigits(r.usv)),
    mrh: fmt(r.mrh, usvDigits(r.mrh * 10) + 1),
    cps: fmt(r.cps, 0),
  };
  $("readValue").textContent = vals[u];
  const alt = {
    cpm: `≈ ${vals.usv} µSv/h · ${vals.cps} CPS`,
    usv: `${vals.cpm} CPM · ${vals.mrh} mrem/h`,
    mrh: `${vals.cpm} CPM · ${vals.usv} µSv/h`,
    cps: `último segundo · ${vals.cpm} CPM`,
  };
  $("readAlt").textContent = alt[u] + (r.dtPct >= 0.1 ? ` · tempo morto +${fmt(r.dtPct, r.dtPct < 10 ? 1 : 0)}% (contador: ${fmt(r.rawCPM, 0)} CPM)` : "");
  renderSigma(r);
  if (r.saturated) {
    const el = $("readSigma");
    el.textContent = r.satReason === "queda"
      ? "Geiger saturado: caiu de repente depois de um valor extremo; o real pode ser bem maior"
      : "Geiger saturado: acima do que o aparelho mede; o real pode ser maior";
    el.className = "sigma high";
    el.hidden = false;
  }

  const names = { seguro: "Seguro", atencao: "Atenção", alarme: "Alarme", perigo: "Perigo" };
  let dLv = r.level, dWord = names[r.level] || "—";
  lvLabel($("levelName"), r.level, dWord, 16);
  $("levelName").className = "sr-only level-name " + r.level;
  // 0 CPM por minutos seguidos: defeito, não "seguro"
  if (r.noCount && !r.saturated) {
    const el = $("readSigma");
    el.textContent = `Sem contagens há ${fmtDur(Date.now() - r.noCountSince)}: o tubo pode estar com defeito. Confira o contador.`;
    el.className = "sigma high";
    el.hidden = false;
    lvLabel($("levelName"), "nocount", "Sem contagens", 16);
    $("levelName").className = "sr-only level-name nocount";
    dLv = "nocount"; dWord = "Sem contagens";
  }
  if (r.saturated) dWord = "Saturado";
  if (r.noCount && !state.noCountToast && !r.saturated) toast("Sem contagens há 2 min: confira o contador", true);
  state.noCountToast = !!r.noCount;
  renderGmcDial(r, dLv, dWord);
  $("hero").classList.toggle("alarm", !!(r.alarm || r.danger));
}

// ---------- painel dos aparelhos (2.8) ----------
// Um quadro por aparelho, sempre em cima: o valor na unidade escolhida
// (µSv/h de padrão), CPM e CPS embaixo, a última hora e a régua com só a
// faixa atual acesa. Em atenção/alarme/perigo a borda e a palavra do estado
// mudam de cor. O terceiro quadro resume os alertas.

const UNIT_TXT = { cpm: "CPM", usv: "µSv/h", mrh: "mrem/h", cps: "CPS" };
const LV_WORD = { seguro: "Seguro", atencao: "Atenção", alarme: "Alarme", perigo: "Perigo" };
const TILE_LV = ["seguro", "atencao", "alarme", "perigo", "nocount"];

// valores em cada unidade; o CPS é a média do minuto (CPM ÷ 60)
function tileVals(usv, cpm, cps) {
  return {
    usv: fmt(usv, usvDigits(usv)),
    mrh: fmt(usv / 10, usvDigits(usv) + 1),
    cpm: fmt(cpm, 0),
    cps: fmt(cps == null ? cpm / 60 : cps, 2),
  };
}

// rodapé: o que não está no número grande (CPM e CPS; µSv/h se o grande for
// CPM ou CPS) e a dose de hoje
// lost: aparelho desconectado ou conectando ({t} = hora da última leitura):
// no lugar das outras unidades vai "última leitura às HH:MM" em cinza, e a
// dose diz "até HH:MM", como no cartão do aparelho
function tileFoot(el, u, v, dose, doseSub, lost) {
  const keys = u === "cpm" ? ["usv", "cps"] : u === "cps" ? ["usv", "cpm"] : ["cpm", "cps"];
  el.textContent = "";
  const num = (t) => { const b = document.createElement("b"); b.className = "mono"; b.textContent = t; return b; };
  if (lost) {
    const sp = document.createElement("span");
    sp.className = "inst-sub";
    sp.textContent = lost.t ? `última leitura às ${atTxt(lost.t)}` : "sem leituras ainda";
    el.appendChild(sp);
    if (lost.t) doseSub = (doseSub ? doseSub + " · " : "") + `até ${atTxt(lost.t)}`;
  }
  for (const k of lost ? [] : keys) {
    const sp = document.createElement("span");
    sp.append(num(v ? v[k] : "—"), " " + UNIT_TXT[k]);
    el.appendChild(sp);
  }
  const sp = document.createElement("span");
  sp.append("hoje", " ", num(dose == null ? "—" : fmt(dose, 2)), " µSv");
  if (doseSub) { const s2 = document.createElement("span"); s2.className = "inst-sub"; s2.textContent = doseSub; sp.append(" ", s2); }
  el.appendChild(sp);
}

// mostrador (ideia 1): o arco de fora do logo enche pelos três pedaços, um por
// faixa (seguro, atenção, alarme); p1/p2 são onde começam a 2ª e a 3ª faixa
// (0 a 1) e pos é a leitura. No perigo o anel fica inteiro (cor pela classe).
const DIAL_SEG = [[34.07, 40.11, 13.93, 40.11], [5.01, 24.66, 15.08, 7.22], [32.92, 7.22, 42.99, 24.66]];
function dialPath(frac) {
  let d = "", rem = Math.max(0, Math.min(1, frac)) * 3;
  for (let i = 0; i < 3 && rem > 0; i++) {
    const p = DIAL_SEG[i], f = Math.min(1, rem);
    rem -= f;
    const a0 = Math.atan2(24 - p[1], p[0] - 24);
    let a1 = Math.atan2(24 - p[3], p[2] - 24);
    if (a1 > a0) a1 -= 2 * Math.PI;
    const ae = a0 + (a1 - a0) * f;
    d += `M${(24 + 19 * Math.cos(a0)).toFixed(2)} ${(24 - 19 * Math.sin(a0)).toFixed(2)}A19 19 0 0 1 ${(24 + 19 * Math.cos(ae)).toFixed(2)} ${(24 - 19 * Math.sin(ae)).toFixed(2)}`;
  }
  return d;
}
function tileScale(pfx, p1, p2, pos, t1, t2) {
  let f = null;
  if (pos != null) f = pos < p1 ? pos / p1 / 3 : pos < p2 ? 1 / 3 + (pos - p1) / (p2 - p1) / 3 : 2 / 3 + (pos - p2) / (1 - p2) / 3;
  $(pfx + "TileArc").setAttribute("d", f == null ? "" : dialPath($(pfx + "Tile").classList.contains("lv-perigo") ? 1 : Math.max(f, 0.02)));
  $(pfx + "TileDial").querySelector("title")?.remove();
  $(pfx + "TileDial").insertAdjacentHTML("afterbegin", `<title>atenção a partir de ${t1} · alarme a partir de ${t2}</title>`);
}

// frase do nível ao lado do mostrador (só nos quadros dos aparelhos)
const LV_SAY = {
  seguro: "no nível de fundo habitual",
  atencao: "acima do limite de atenção",
  alarme: "passou o limite de alarme",
  perigo: "acima do limite de perigo",
  nocount: "o aparelho não está mandando contagens",
};
function tileState(pfx, lvl, word, say) {
  const el = $(pfx + "Tile");
  TILE_LV.forEach((l) => el.classList.toggle("lv-" + l, lvl === l));
  lvLabel($(pfx + "TileState"), lvl, word, pfx === "a" ? 14 : 18);
  const sayEl = $(pfx + "TileSay");
  if (sayEl) sayEl.textContent = say != null ? say : LV_SAY[lvl] || "";
}
// número dentro do anel: encolhe quando não cabe
function dialVal(pfx, txt) {
  const el = $(pfx + "TileVal");
  el.textContent = txt;
  el.setAttribute("font-size", txt.length > 6 ? "3.4" : txt.length > 4 ? "4.2" : "5.2");
}

// faixa compacta: o mesmo valor, o número secundário (CPM, ou µSv/h se o
// grande já for CPM/CPS) e o estado na cor do nível
function miniDev(pfx, u, v, lvl, word, dotCls, stale) {
  const el = $("mini" + pfx);
  TILE_LV.forEach((l) => el.classList.toggle("lv-" + l, lvl === l));
  el.classList.toggle("stale", !!stale);
  $("mini" + pfx + "Dot").className = "dot" + dotCls;
  $("mini" + pfx + "Val").textContent = v ? v[u] : "—";
  $("mini" + pfx + "Unit").textContent = UNIT_TXT[u];
  const k2 = u === "cpm" || u === "cps" ? "usv" : "cpm";
  $("mini" + pfx + "Sec").textContent = v ? `${v[k2]} ${UNIT_TXT[k2]}` : "";
  lvLabel($("mini" + pfx + "State"), lvl, word, 12);
}

function renderGTile() {
  const r = state.last, s = state.status || {};
  const connected = s.state === "conectado";
  const live = !!r && (connected || state.demo.active);
  const u = state.unit;
  $("gTileUnit").textContent = UNIT_TXT[u];
  $("gTileDemo").hidden = !state.demo.active;
  $("gTile").classList.toggle("stale", !live);
  const gc = gmcConn();
  $("gTile").classList.toggle("conn-off", gc === "off");
  $("gTile").classList.toggle("conn-wait", gc === "wait");
  const v = live ? tileVals(r.usv, r.cpm) : null;
  dialVal("g", v ? v[u] : "—");
  // os níveis do app (os mesmos que disparam os avisos), régua até 1,5× o alarme
  const k = calK(), [tA, tB] = gmcThrCPM(), top = tB * 1.5;
  const tick = (cpm) => (u === "cpm" ? fmt(cpm, 0) : u === "cps" ? fmt(cpm / 60, 2) : u === "mrh" ? fmt(cpm * k / 10, 3) : fmt(cpm * k, 2));
  let lvl = "", word;
  if (live) {
    lvl = r.level;
    word = LV_WORD[r.level] || "—";
    if (r.saturated) word = "Saturado";
    else if (r.noCount) { lvl = "nocount"; word = "Sem contagens"; }
  } else word = connected ? "Aguardando…" : s.state === "conectando" ? "Conectando…" : "Desconectado";
  tileState("g", lvl, word, live && r.saturated ? "o tubo saturou; a taxa real é maior" : null);
  tileScale("g", tA / top, tB / top, live ? Math.min(r.cpm, top) / top : null, tick(tA), tick(tB));
  state.gNow = { live, lvl, usv: live ? r.usv : null, sigma: live && r.sigma != null ? r.sigma : null };
  agoraNow();
  const gl = state.gLastSeen;
  tileFoot($("gTileFoot"), u, v, state.gDose, "", gc ? { t: (gl && gl.time) || state.gOffSince || 0 } : null);
  renderDevMini();
  miniDev("G", u, v, lvl, word, s.state === "conectado" ? " on" : s.state === "conectando" ? " wait" : s.state === "erro" ? " err" : "", !live);
}

function renderRTile(st, o) {
  const on = st.state === "conectado";
  const via = st.demo ? "simulado" : st.via === "bluetooth" ? "Bluetooth" : "USB";
  $("rTileDemo").hidden = !st.demo;
  $("rTileDot").className = "dot" + (on ? " on" : st.state === "conectando" ? " wait" : st.state === "erro" ? " err" : "");
  $("rTileConnText").textContent = on
    ? `Radiacode 102 · ${via}` + (st.hasRare ? ` · ${fmt(st.battery, 0)}%` : "")
    : st.state === "conectando" ? "Radiacode 102 · conectando…" : "Radiacode 102 · desconectado";
  $("rTileConn").title = on ? `Radiacode 102 conectado pelo ${via}` : st.err || "Radiacode 102";
  const u = state.rcUnit;
  $("rTileUnit").textContent = UNIT_TXT[u];
  $("rTile").classList.toggle("stale", !o.live);
  const rcc = rcConn(st);
  $("rTile").classList.toggle("conn-off", rcc === "off");
  $("rTile").classList.toggle("conn-wait", rcc === "wait");
  const v = o.usv == null ? null : tileVals(o.usv, o.cps * 60, o.cps);
  dialVal("r", v ? v[u] : "—");
  $("rTileVal").closest(".inst-read").title = !o.live && st.lastT ? `última leitura: ${whenLabel(st.lastT)}` : "";
  // marcas da régua na unidade do número (CPM/CPS pela relação de agora)
  const perUSv = o.usv > 0 && o.cps > 0 ? o.cps / o.usv : null;
  const tick = (x) => (u === "mrh" ? fmt(x / 10, 3) : u === "cps" && perUSv ? fmt(x * perUSv, 1) : u === "cpm" && perUSv ? fmt(x * perUSv * 60, 0) : fmt(x, 2));
  const names = o.own ? { seguro: "Seguro", atencao: "Alarme 1", alarme: "Alarme 2" } : LV_WORD;
  const ownSay = { atencao: "passou o 1º limite do aparelho", alarme: "passou o 2º limite do aparelho" };
  if (o.live) tileState("r", o.lvl, names[o.lvl] || "—", o.own ? ownSay[o.lvl] : null);
  else tileState("r", "", st.state === "conectando" ? "Conectando…" : on ? "Aguardando…" : "Desconectado");
  tileScale("r", o.attn / o.top, o.alarm / o.top, o.usv == null ? null : Math.min(o.usv, o.top) / o.top, tick(o.attn), tick(o.alarm));
  $("miniR").hidden = false;
  miniDev("R", u, v, o.live ? o.lvl : "", $("rTileState").textContent, $("rTileDot").className.replace(/^dot/, ""), !o.live);
  const sx = st.stats || {};
  const dose = st.todayDevOK ? st.todayDev : sx.todayMinutes || st.today ? st.today : null;
  tileFoot($("rTileFoot"), u, v, dose, st.todayDevOK ? "do aparelho" : "", rcc ? { t: st.lastT || 0 } : null);
}

// alertas: o ativo mais alto (borda na cor do nível) ou o último que passou
function renderATile() {
  const list = state.activeAlerts || [];
  const rank = { atencao: 1, alarme: 2, perigo: 3 };
  const dev = (e) => (isRcEv(e) ? "RC102" : "GMC-800");
  const top = list.slice().sort((a, b) => (rank[lvlCls(b)] || 1) - (rank[lvlCls(a)] || 1) || a.time - b.time)[0];
  const srcs = new Set(list.map((e) => dev(e) + (e.demo ? " demo" : "")));
  const lv = top ? { alarme: "alarme", perigo: "perigo" }[lvlCls(top)] || "atencao" : "";
  tileState("a", lv, srcs.size > 1 ? `${srcs.size} aparelhos` : "");
  const ma = $("miniAlert");
  ma.hidden = !top;
  ma.className = "mini-alert" + (lv ? " lv-" + lv : "");
  ma.querySelector("i").innerHTML = lvIcon(lv, 12);
  $("miniAlertText").textContent = top ? (list.length > 1 && srcs.size > 1 ? `${srcs.size} alertas ativos` : `${lvlName(top)} no ${dev(top)}`) : "";
  $("mini").classList.toggle("alerting", !!top);
  ["atencao", "alarme", "perigo"].forEach((l) => $("mini").classList.toggle("lv-" + l, lv === l));
  agoraNow();
  const nav = $("navAlerts");
  nav.hidden = !top;
  nav.textContent = String(srcs.size);
  if (lv) nav.insertAdjacentHTML("afterbegin", lvIcon(lv, 10, "Nível: " + LV_WORD[lv]));
  nav.className = "side-badge mono" + (lv ? " lv-" + lv : "");
  if (top) {
    $("aTileTitle").textContent = `${lvlName(top)} no ${dev(top)}` + (top.demo ? " (demo)" : "");
    $("aTileText").textContent = `desde ${hhmm(top.time)} (${fmtDur(Date.now() - top.time)}) · pico ${evVal(top, top.peak)}`;
    return;
  }
  $("aTileTitle").textContent = "Nenhum alerta ativo";
  const e = state.lastAlert;
  $("aTileText").textContent = e
    ? `Último: ${whenLabel(e.time)} · ${lvlName(e)} no ${dev(e)}` + (e.end ? ` · ${fmtDur(e.end - e.time)}` : "") + (e.peak ? ` · pico ${evVal(e, e.peak)}` : "")
    : "Nenhum alerta registrado ainda.";
}

// a faixa de atenção/alarme fica presa no topo: o menu começa embaixo dela
(() => {
  const bars = document.querySelector(".bars");
  if (!bars || !window.ResizeObserver) return;
  new ResizeObserver(() => document.documentElement.style.setProperty("--barsH", bars.offsetHeight + "px")).observe(bars);
})();

// a faixa aparece quando os quadros grandes saem da tela
(() => {
  const mini = $("mini"), inst = $("instruments");
  if (!window.IntersectionObserver) return;
  // aparece quando os valores dos quadros já saíram por cima (mais da
  // metade do painel escondida), não só quando ele some inteiro: páginas
  // curtas rolam pouco e o painel ficava cortado sem a faixa
  let on = false;
  const set = (gone) => {
    if (gone === on) return;
    on = gone;
    mini.hidden = !gone;
    requestAnimationFrame(() => mini.classList.toggle("in", gone));
  };
  new IntersectionObserver(([e]) => {
    const top = parseFloat(getComputedStyle(document.documentElement).getPropertyValue("--barsH")) || 0;
    set(e.boundingClientRect.top < top && e.intersectionRatio < 0.55);
  }, { threshold: [0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.55, 0.6, 0.7, 0.8, 0.9, 1] }).observe(inst);
  const toTop = () => window.scrollTo({ top: 0, behavior: "smooth" });
  ["miniG", "miniR", "miniTop"].forEach((id) => $(id).addEventListener("click", toTop));
})();

// a última hora de cada aparelho, desenhada pequena no quadro
let sparkSeq = 0;
async function refreshSpark() {
  if (!api.GetChart) return;
  const my = ++sparkSeq;
  let c;
  try { c = await api.GetChart("1h"); } catch (e) { return; }
  if (my !== sparkSeq || !c) return;
  drawSpark("gTileSpark", c.values);
  drawSpark("rTileSpark", c.rc);
  drawSpark("gHeroSpark", c.values);
  drawSpark("rHeroSpark", c.rc);
}
function drawSpark(id, vals) {
  const pl = $(id).querySelector("polyline");
  const pts = (vals || []).map((v, i) => [i, v]).filter((p) => p[1] != null && isFinite(p[1]));
  if (pts.length < 2) { pl.setAttribute("points", ""); return; }
  const n = Math.max(1, vals.length - 1);
  // média de 3 pontos: a variação natural da contagem não vira serra, um
  // aumento de verdade continua aparecendo
  const sm = pts.map((p, k) => { const w = pts.slice(Math.max(0, k - 1), k + 2); return [p[0], w.reduce((a, q) => a + q[1], 0) / w.length]; });
  pts.splice(0, pts.length, ...sm);
  const ys = pts.map((p) => p[1]);
  const lo0 = Math.min(...ys), hi0 = Math.max(...ys), mid = (lo0 + hi0) / 2;
  // altura mínima de 60% do valor: o fundo parado fica quase reto
  const span = Math.max(hi0 - lo0, Math.abs(mid) * 0.6, 1e-6), lo = mid - span / 2;
  pl.setAttribute("points", pts.map(([i, v]) => `${(i / n * 160).toFixed(1)},${(37 - (v - lo) / span * 34).toFixed(1)}`).join(" "));
}

// ---------- conexão ----------

function renderStatus() {
  const s = state.status;
  const d = s.device || {};
  const dot = $("statusDot");
  dot.className = "dot" + (s.state === "conectado" ? " on" : s.state === "conectando" ? " wait" : s.state === "erro" ? " err" : "");
  let text = "desconectado";
  // no quadro do topo: modelo e porta (o "conectado" já é a bolinha verde)
  if (s.state === "conectado") text = `GQ GMC-800 · ${d.port}`;
  else if (s.state === "conectando") text = s.message || "conectando…";
  else if (s.state === "desconectado" && s.message) text = "desconectado · " + s.message;
  else if (s.state === "erro") text = s.message || "erro";
  $("statusText").textContent = text;
  const retrying = state.settings.autoConnect && s.state !== "conectado" && s.state !== "conectando" && s.message;
  $("statusPill").title = retrying ? text + " — o app tenta de novo a cada 10 s" : s.state === "conectado" ? `${d.version || "contador"} · ${d.port} · conectado` : text;

  const connected = s.state === "conectado";
  $("btnHistory").disabled = !connected || s.downloading;
  if ($("btnSyncDl")) $("btnSyncDl").disabled = !connected || s.downloading;
  $("clockChip").disabled = !connected || s.downloading;
  $("emptyHint").hidden = connected || s.state === "conectando" || state.demo.active;

  $("iModel").textContent = connected ? d.version : "—";
  $("iSerial").textContent = connected && d.serial ? d.serial : "—";
  $("iPort").textContent = connected ? d.port : "—";
  renderSetNavStatus();
  $("clockChip").hidden = !connected;
  if (connected) {
    const drift = d.clockDrift || 0;
    const clock = Math.abs(drift) <= 2 ? "certo" : `${fmt(Math.abs(drift), 0)} s ${drift > 0 ? "adiantado" : "atrasado"}`;
    $("iClock").textContent = clock;
    $("clockText").textContent = "relógio " + clock;
    $("clockChip").classList.toggle("off", Math.abs(drift) > 2);
    const cal = d.calibration || {};
    $("iCal").textContent = `${cal.source || "padrão"} · ${fmt(d.usvPerCPM, 4)} µSv/h por CPM`;
  } else {
    $("iClock").textContent = "—";
    $("iCal").textContent = "—";
  }
  $("progress").hidden = !s.downloading;
  $("dataDir").textContent = s.dataDir || "—";
  $("connMsg").textContent = s.state === "erro" ? s.message : s.state === "conectado" ? "conectado em " + d.port : "";
  $("btnConnect").textContent = connected ? "Desconectar" : "Conectar";
  renderHeroVia();
  renderReading();
}

// ---------- cartões, tabela e eventos ----------

async function refreshStats() {
  const s = await api.GetStats();
  state.bgCPM = s.bgCPM || null;
  const cal = calK();
  // em µSv/h desde a 2.29 (a última hora e o último dia: calibração de agora)
  const u = (v) => (v == null ? "—" : fmt(v * cal, usvDigits(v * cal)));
  $("tAvg").textContent = u(s.avg1h);
  $("tAvgSub").textContent = s.avg1h == null ? "µSv/h" : `µSv/h · ${fmt(s.avg1h, 1)} CPM`;
  $("tMin").textContent = u(s.min24h);
  $("tMax").textContent = u(s.max24h);
  $("tMaxSub").textContent = s.max24h == null ? "µSv/h" : `µSv/h às ${hhmm(s.max24hAt)}`;
  // as mesmas medidas dentro do quadro do GMC (quando está lado a lado com o Radiacode)
  $("hAvg1h").textContent = u(s.avg1h);
  $("hMin24").textContent = u(s.min24h);
  $("hMax24").textContent = u(s.max24h);
  $("hMax24Sub").textContent = s.max24h == null ? "µSv/h" : `µSv/h às ${hhmm(s.max24hAt)}`;
  state.gDose = s.minutesToday ? s.doseToday : null;
  renderGTile();
  $("tDose").textContent = s.minutesToday ? fmt(s.doseToday, 2) : "—";
  $("tDoseSub").textContent = s.minutesToday ? `${s.minutesToday} min medidos` : "acumulados";
  if (gmcConn()) renderGmcConn(); // desconectado: "até HH:MM"
  $("heroDose").title = s.minutesToday ? `Hoje: ${doseCompare(s.doseToday)} · calculada pelo app (o GMC não informa a dose que soma)` : "";
}

async function refreshDaily() {
  const days = await api.GetDaily(7);
  const tb = $("days");
  tb.textContent = "";
  if (!days || !days.length) {
    tb.innerHTML = '<tr><td colspan="3" class="muted">Sem leituras guardadas.</td></tr>';
    return;
  }
  for (const d of days) {
    const tr = document.createElement("tr");
    const label = isToday(d.day) ? "hoje" : isYesterday(d.day) ? "ontem" : ddmm(d.day);
    for (const [txt, cls] of [[label, ""], [fmt(d.cpm, 1), "num"], [fmt(d.usv, 3), "num"]]) {
      const td = document.createElement("td");
      td.textContent = txt;
      if (cls) td.className = cls;
      tr.appendChild(td);
    }
    tr.title = `${d.minutes} minutos medidos`;
    tb.appendChild(tr);
  }
}

function renderList(ul, list, max, empty, alerts) {
  ul.textContent = "";
  if (!list || !list.length) {
    ul.innerHTML = `<li class="muted">${empty}</li>`;
    return;
  }
  for (const e of list.slice(0, max)) {
    const li = document.createElement("li");
    if (alerts) {
      const d = document.createElement("span");
      d.className = "dot-a";
      d.innerHTML = lvIcon(/Perigo/.test(e.text) ? "perigo" : /Alarme/.test(e.text) ? "alarme" : "atencao", 12);
      li.appendChild(d);
    }
    const t = document.createElement("time");
    t.dateTime = new Date(e.time).toISOString();
    t.textContent = isToday(e.time) ? hhmm(e.time) : isYesterday(e.time) ? "ontem " + hhmm(e.time) : ddmm(e.time) + " " + hhmm(e.time);
    const s = document.createElement("span");
    s.textContent = e.text;
    li.append(t, s);
    ul.appendChild(li);
  }
}

// ---------- alertas (a página está em alertas.js) ----------

state.activeAlerts = [];
state.alertFilter = "";
state.alertSel = null;

// ---------- visor do log técnico ----------
// Níveis, componente, busca, histograma empilhado por nível (clicar numa
// barra dá zoom nela) e "ao vivo" (atualiza a cada 3 s com a janela aberta).

const LG_COLORS = { DEBUG: "#5F6A71", INFO: "#7CB7FF", WARN: COR.att, ERROR: COR.warn };
const LG_RANGES = { "15m": 15 * 60e3, "1h": 3600e3, "24h": 86400e3, "7d": 7 * 86400e3, all: 0 };
state.lg = { range: localGet("lgRange", "24h"), levels: (localGet("lgLevels", "INFO,WARN,ERROR")).split(",").filter(Boolean), comp: "", text: "", zoom: null, res: null };
state.evTab = "log";

function showEvTab(t) {
  state.evTab = t;
  document.querySelectorAll("#evTabs button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.evtab === t)));
  $("logPane").hidden = t !== "log";
  $("evPane").hidden = t !== "eventos";
  if (t === "log") refreshLogs();
}

function lgQuery() {
  const g = state.lg, now = Date.now();
  let from = 0, to = 0;
  if (g.zoom) { from = g.zoom.from; to = g.zoom.to; }
  else if (LG_RANGES[g.range]) { from = now - LG_RANGES[g.range]; }
  return { from, to, levels: g.levels, comp: g.comp, text: g.text, limit: 1500 };
}

let lgSeq = 0;
async function refreshLogs() {
  if (!$("eventsDlg").open && lgSeq) return;
  const my = ++lgSeq;
  let r;
  try { r = await api.GetLogs(lgQuery()); } catch (e) { $("lgFoot").textContent = "Não consegui ler o log: " + errMsg(e); return; }
  if (my !== lgSeq) return;
  state.lg.res = r;
  renderLogs(r);
}

const lgTime = (t) => { const d = new Date(t); return `${ddmm(t)} ${hhmmss(t)}.${String(d.getMilliseconds()).padStart(3, "0")}`; };
const escHtml = (x) => x.replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]));

function renderLogs(r) {
  const g = state.lg;
  document.querySelectorAll("#lgRange button").forEach((b) => b.setAttribute("aria-pressed", String(!g.zoom && b.dataset.lr === g.range)));
  document.querySelectorAll("#lgLevels button").forEach((b) => {
    b.setAttribute("aria-pressed", String(g.levels.includes(b.dataset.lv)));
    b.querySelector("span").textContent = fmt((r.counts || {})[b.dataset.lv] || 0, 0);
  });
  // componentes
  const sel = $("lgComp");
  const cur = g.comp;
  sel.innerHTML = '<option value="">todos os componentes</option>';
  for (const c of r.comps || []) {
    const o = document.createElement("option");
    o.value = c.name; o.textContent = `${c.name} (${fmt(c.count, 0)})`;
    sel.appendChild(o);
  }
  if (cur && !(r.comps || []).some((c) => c.name === cur)) { const o = document.createElement("option"); o.value = cur; o.textContent = cur + " (0)"; sel.appendChild(o); }
  sel.value = cur;
  $("lgZoomOut").hidden = !g.zoom;
  drawLogHist(r);
  // tabela
  const box = $("lgTable");
  const q = g.text.trim().toLowerCase();
  const mark = (txt) => {
    const h = escHtml(txt);
    if (!q) return h;
    const i = txt.toLowerCase().indexOf(q);
    if (i < 0) return h;
    return escHtml(txt.slice(0, i)) + "<mark>" + escHtml(txt.slice(i, i + q.length)) + "</mark>" + escHtml(txt.slice(i + q.length));
  };
  const rows = ['<div class="lg-row head"><span>hora</span><span>nível</span><span>componente</span><span>mensagem</span></div>'];
  for (const e of r.entries || []) {
    const msg = e.comp !== "app" && e.msg.startsWith(e.comp) ? e.msg.slice(e.msg.indexOf(": ") + 2) : e.msg;
    rows.push(`<div class="lg-row ${e.level}"><span class="t">${lgTime(e.t)}</span><span class="lg-badge ${e.level}">${e.level}</span><span class="c" data-comp="${escHtml(e.comp)}" title="filtrar por ${escHtml(e.comp)}">${escHtml(e.comp)}</span><span class="m">${mark(msg)}</span></div>`);
  }
  if (!(r.entries || []).length) rows.push('<div class="lg-row"><span></span><span></span><span></span><span class="muted">Nenhuma linha com esses filtros.</span></div>');
  box.innerHTML = rows.join("");
  const shown = (r.entries || []).length;
  $("lgFoot").textContent = `${fmt(r.total || 0, 0)} linhas com esses filtros` + (shown < (r.total || 0) ? ` · mostrando as ${fmt(shown, 0)} mais recentes` : "") +
    ` · ${lgTime(r.from)} → ${lgTime(r.to)}` + (g.levels.includes("DEBUG") ? "" : " · DEBUG escondido (tentativas de reconexão com o cabo fora)");
}

function drawLogHist(r) {
  const svg = $("lgChart"), b = r.buckets || [];
  const W = 1000, H = 120, n = b.length || 1, bw = W / n;
  const lv = state.lg.levels.length ? state.lg.levels : Object.keys(LG_COLORS);
  const order = ["ERROR", "WARN", "INFO", "DEBUG"].filter((x) => lv.includes(x)).reverse();
  let max = 1;
  for (const x of b) max = Math.max(max, order.reduce((s, l) => s + (x.counts[l] || 0), 0));
  const parts = [];
  for (let i = 0; i < 4; i++) parts.push(`<line x1="0" x2="${W}" y1="${(H / 4) * i}" y2="${(H / 4) * i}" stroke="var(--border)" stroke-width="1" vector-effect="non-scaling-stroke"/>`);
  b.forEach((x, i) => {
    let y = H;
    for (const l of order) {
      const c = x.counts[l] || 0;
      if (!c) continue;
      const h = Math.max(1.5, (c / max) * (H - 4));
      y -= h;
      parts.push(`<rect x="${i * bw + bw * 0.12}" y="${y}" width="${bw * 0.76}" height="${h}" fill="${LG_COLORS[l]}" opacity=".9"/>`);
    }
    parts.push(`<rect class="hit" data-i="${i}" x="${i * bw}" y="0" width="${bw}" height="${H}"/>`);
  });
  svg.innerHTML = parts.join("");
  const span = (r.to || 0) - (r.from || 0);
  const lab = (t) => span <= 2 * 86400e3 ? hhmm(t) : ddmm(t) + " " + hhmm(t);
  const xs = $("lgX").children;
  if (b.length) { xs[0].textContent = lab(b[0].t); xs[1].textContent = lab(b[Math.floor(b.length / 2)].t); xs[2].textContent = lab(b[b.length - 1].t); }
  $("lgHistTitle").textContent = `linhas por ${fmtDur(r.bucketMs || 60000)} · máx ${fmt(max, 0)}`;
}

function wireLogs() {
  document.querySelectorAll("#evTabs button").forEach((b) => b.addEventListener("click", () => showEvTab(b.dataset.evtab)));
  document.querySelectorAll("#lgRange button").forEach((b) => b.addEventListener("click", () => { state.lg.range = b.dataset.lr; state.lg.zoom = null; localSet("lgRange", b.dataset.lr); refreshLogs(); }));
  document.querySelectorAll("#lgLevels button").forEach((b) => b.addEventListener("click", () => {
    const g = state.lg, l = b.dataset.lv;
    g.levels = g.levels.includes(l) ? g.levels.filter((x) => x !== l) : g.levels.concat(l);
    localSet("lgLevels", g.levels.join(","));
    refreshLogs();
  }));
  $("lgComp").addEventListener("change", (e) => { state.lg.comp = e.target.value; refreshLogs(); });
  let tmr = 0;
  $("lgText").addEventListener("input", (e) => { clearTimeout(tmr); tmr = setTimeout(() => { state.lg.text = e.target.value; refreshLogs(); }, 250); });
  $("lgTable").addEventListener("click", (e) => { const c = e.target.closest("[data-comp]"); if (c) { state.lg.comp = c.dataset.comp; refreshLogs(); } });
  $("lgZoomOut").addEventListener("click", () => { state.lg.zoom = null; refreshLogs(); });
  const svg = $("lgChart"), tip = $("lgTip");
  svg.addEventListener("click", (e) => {
    const h = e.target.closest(".hit"), r = state.lg.res;
    if (!h || !r) return;
    const x = r.buckets[+h.dataset.i];
    state.lg.zoom = { from: x.t, to: x.t + r.bucketMs };
    refreshLogs();
  });
  svg.addEventListener("mousemove", (e) => {
    const h = e.target.closest(".hit"), r = state.lg.res;
    if (!h || !r) { tip.hidden = true; return; }
    const x = r.buckets[+h.dataset.i];
    const lines = Object.keys(LG_COLORS).filter((l) => x.counts[l]).map((l) => `${l} ${x.counts[l]}`);
    tip.textContent = `${lgTime(x.t).slice(0, 11)}–${hhmm(x.t + r.bucketMs)} · ${lines.join(" · ") || "nada"} · clique para ver`;
    const box = svg.getBoundingClientRect(), hb = $("lgChart").parentElement.getBoundingClientRect();
    tip.style.left = Math.min(e.clientX - hb.left + 10, hb.width - 260) + "px";
    tip.hidden = false;
  });
  svg.addEventListener("mouseleave", () => { tip.hidden = true; });
  setInterval(() => { if ($("eventsDlg").open && state.evTab === "log" && $("lgLive").checked && !state.lg.zoom) refreshLogs(); }, 3000);
}


// ---------- páginas (2.8) ----------
// O menu da esquerda troca a página; o painel dos aparelhos fica sempre em
// cima. Gráficos é a página de abertura.

state.view = "agora"; // 2.17: a tela inicial é a "Agora"
const PAGES = { agora: "pageAgora", graficos: "pageGraficos", resumo: "pageResumo", alertas: "pageAlertas", medir: "pageMedir", fora: "pageFora", chuva: "pageChuva", gmc: "pageGmc", radiacode: "pageRadiacode", config: "pageConfig" };

async function showView(v, skipRender) {
  if (v === "dose") { setSub("graficos", "tempo"); v = "graficos"; } // a Dose virou sub-aba de Gráficos
  if (!PAGES[v]) v = "graficos";
  const changed = state.view !== v;
  state.view = v;
  for (const [k, id] of Object.entries(PAGES)) $(id).hidden = k !== v;
  document.querySelectorAll("#sideNav [data-view], #btnSettings").forEach((b) => {
    if (b.dataset.view === v) b.setAttribute("aria-current", "page"); else b.removeAttribute("aria-current");
  });
  // trocou de página com a tela rolada: volta para o começo da página
  if (changed && !skipRender) {
    const top = $("pages").getBoundingClientRect().top + window.scrollY - 12;
    if (window.scrollY > top) window.scrollTo({ top: Math.max(0, top) });
  }
  if (v === "graficos") refreshSub("graficos");
  if (window.rcDemo) { if (v === "radiacode") window.rcDemo.show(); else window.rcDemo.hide(); }
  renderDevMini();
  if (skipRender) return;
  if (v === "alertas") renderAlertHistory(await api.GetEvents("alerta"));
  if (v === "fora") renderPeriods();
  if (v === "chuva") refreshRainImpact(true);
  if (v === "resumo") refreshSummary();
  if (v === "medir") { renderMeasure(await api.GetMeasure()); setMeasureMode(state.mMode); }
  if (v === "radiacode") { refreshRcCompare(); if (api.GetRadiacode) renderRc(await api.GetRadiacode()); }
  if (v === "config") openSettings();
  if (v === "agora") refreshAgora();
}

// sub-abas: Gráficos e Radiacode mostram uma parte de cada vez (a última
// escolhida volta quando abre o app)
const SUBS = { graficos: ["tempo", "hora", "padroes", "tendencia"], radiacode: ["agora", "espectro", "espectrograma", "log", "busca", "atividade"] };
state.sub = {};
for (const pg of Object.keys(SUBS)) { const v = localGet("sub." + pg, ""); state.sub[pg] = SUBS[pg].includes(v) ? v : SUBS[pg][0]; }

function setSub(pg, sub, focus) {
  if (!SUBS[pg].includes(sub)) sub = SUBS[pg][0];
  const changed = state.sub[pg] !== sub;
  state.sub[pg] = sub;
  localSet("sub." + pg, sub);
  const page = $(PAGES[pg]);
  page.querySelectorAll(".subtabs [data-sub]").forEach((b) => {
    const on = b.dataset.sub === sub;
    b.setAttribute("aria-selected", String(on));
    b.tabIndex = on ? 0 : -1;
    if (on && focus) b.focus();
  });
  page.querySelectorAll("[data-subpane]").forEach((el) => { el.hidden = el.dataset.subpane !== sub; });
  if (pg === "graficos") $("heatTrecho").hidden = sub !== "hora" || !state.heatPick;
  if (changed && state.view === pg) refreshSub(pg);
}

function refreshSub(pg) {
  const s = state.sub[pg];
  if (pg === "graficos") {
    if (s === "tempo") { refreshChart(); refreshDose(); }
    else if (s === "hora") refreshHeat();
    else if (s === "padroes") refreshPatterns();
    else refreshBgTrend();
  }
  // os gráficos do Radiacode são canvas: redesenha com o tamanho de agora
  if (pg === "radiacode" && window.rcDemo && window.rcDemo.redraw) requestAnimationFrame(() => window.rcDemo.redraw());
}

for (const pg of Object.keys(SUBS)) setSub(pg, state.sub[pg]);
document.addEventListener("click", (e) => {
  const b = e.target.closest(".subtabs [data-sub]");
  if (b) setSub(b.closest(".subtabs").dataset.page, b.dataset.sub);
});
// setas e Home/End entre as sub-abas (padrão de abas)
document.querySelectorAll(".subtabs").forEach((nav) => nav.addEventListener("keydown", (e) => {
  const pg = nav.dataset.page, list = SUBS[pg], i = list.indexOf(state.sub[pg]);
  const j = e.key === "ArrowRight" ? (i + 1) % list.length : e.key === "ArrowLeft" ? (i - 1 + list.length) % list.length
    : e.key === "Home" ? 0 : e.key === "End" ? list.length - 1 : -1;
  if (j < 0) return;
  e.preventDefault();
  setSub(pg, list[j], true);
}));

document.addEventListener("click", (e) => {
  const b = e.target.closest("[data-view]");
  if (b) showView(b.dataset.view);
});

// ---------- gráficos (a página está em graficos.js) ----------

state.doseSrc = localGet("doseSrc", "radiacode");
state.doseShow = (() => { try { return Object.assign({ gmc: true, rc: true }, JSON.parse(localGet("doseShow", "{}"))); } catch (e) { return { gmc: true, rc: true }; } })();

// ---------- modo demo ----------

const sliderToCPM = (v) => Math.min(999999, Math.max(0, Math.round(Math.pow(10, (v / 100) * 6) - 1)));
const cpmToSlider = (c) => Math.round((Math.log10(Math.max(0, c) + 1) / 6) * 100);
let demoTimer = null;

function renderDemo() {
  const d = state.demo;
  $("demoCard").hidden = !d.active;
  $("btnDemo").setAttribute("aria-pressed", String(!!d.active));
  $("btnDemoText").textContent = d.active ? "Demo ligado" : "Modo demo";
  if (d.active) {
    if (document.activeElement !== $("demoCPM")) $("demoCPM").value = Math.round(d.cpm);
    if (document.activeElement !== $("demoSlider")) $("demoSlider").value = cpmToSlider(d.cpm);
    $("demoReal").checked = !!d.realistic;
    $("demoRcOn").checked = !!d.rc;
    $("demoRcCtl").hidden = !d.rc;
    if (d.rc) {
      if (document.activeElement !== $("demoRcUSv")) $("demoRcUSv").value = +(d.rcUSv || 0).toFixed(d.rcUSv < 1 ? 3 : 2);
      if (document.activeElement !== $("demoRcSlider")) $("demoRcSlider").value = usvToSlider(d.rcUSv || 0);
    }
    renderDemoRcPresets();
  }
  renderStatus();
}

// Radiacode simulado: µSv/h de 0,01 a 10.000 na escala logarítmica
const sliderToUSv = (v) => Math.round(Math.pow(10, (v / 100) * 6 - 2) * 1000) / 1000;
const usvToSlider = (u) => Math.round(((Math.log10(Math.max(0.01, u)) + 2) / 6) * 100);
function renderDemoRcPresets() {
  const box = $("demoRcPresets");
  const st = (state.rc && state.rc.st) || {};
  const l = st.limits || {};
  const bg = (st.stats && st.stats.bgUSv) || 0.12;
  const list = [[bg, "fundo"]];
  if (l.attn) list.push([l.attn * 1.15, "atenção"]);
  if (l.alarm) list.push([l.alarm * 1.15, "alarme"]);
  if (l.danger) list.push([l.danger * 1.15, "perigo"]);
  const key = list.map((x) => x[0].toFixed(3)).join("|");
  if (box.dataset.key === key) return;
  box.dataset.key = key;
  box.textContent = "";
  for (const [v, name] of list) {
    const b = document.createElement("button");
    b.type = "button";
    b.className = "btn small";
    const r = v < 1 ? Math.round(v * 1000) / 1000 : v < 10 ? Math.round(v * 100) / 100 : Math.round(v * 10) / 10;
    b.dataset.demoRc = r;
    b.textContent = `${fmt(r, usvDigits(r))} · ${name}`;
    box.appendChild(b);
  }
}
let demoRcTimer = null;
function sendDemoRc(on, usv) {
  clearTimeout(demoRcTimer);
  demoRcTimer = setTimeout(() => {
    if (!api.SetDemoRc) return;
    call(api.SetDemoRc, on, usv).then((st) => { state.demo = st; renderDemo(); });
  }, 200);
}

function sendDemo(cpm) {
  clearTimeout(demoTimer);
  demoTimer = setTimeout(() => {
    call(api.StartDemo, cpm, $("demoReal").checked).then((st) => { state.demo = st; renderDemo(); });
  }, 200);
}

function wireDemo() {
  $("btnDemo").addEventListener("click", async () => {
    if (state.demo.active) {
      state.demo = await call(api.StopDemo);
    } else {
      const start = state.last && !state.last.demo ? state.last.cpm : 20;
      state.demo = await call(api.StartDemo, start, false);
    }
    state.last = null;
    renderDemo();
    refreshAll();
  });
  $("btnDemoStop").addEventListener("click", () => $("btnDemo").click());
  $("demoRcOn").addEventListener("change", () => {
    const st = (state.rc && state.rc.st) || {};
    const v = parseFloat($("demoRcUSv").value) || state.demo.rcUSv || (st.stats && st.stats.bgUSv) || 0.12;
    sendDemoRc($("demoRcOn").checked, v);
  });
  $("demoRcUSv").addEventListener("input", () => {
    const v = Math.min(10000, Math.max(0, parseFloat(String($("demoRcUSv").value).replace(",", ".")) || 0));
    $("demoRcSlider").value = usvToSlider(v);
    sendDemoRc(true, v);
  });
  $("demoRcSlider").addEventListener("input", () => {
    const v = sliderToUSv(parseInt($("demoRcSlider").value, 10));
    $("demoRcUSv").value = v;
    sendDemoRc(true, v);
  });
  $("demoRcPresets").addEventListener("click", (e) => {
    const b = e.target.closest("[data-demo-rc]");
    if (!b) return;
    const v = parseFloat(b.dataset.demoRc);
    $("demoRcUSv").value = v;
    $("demoRcSlider").value = usvToSlider(v);
    sendDemoRc(true, v);
  });
  $("demoCPM").addEventListener("input", () => {
    const v = Math.min(999999, Math.max(0, parseInt($("demoCPM").value, 10) || 0));
    $("demoSlider").value = cpmToSlider(v);
    sendDemo(v);
  });
  $("demoSlider").addEventListener("input", () => {
    const v = sliderToCPM(parseInt($("demoSlider").value, 10));
    $("demoCPM").value = v;
    sendDemo(v);
  });
  document.querySelectorAll("[data-demo]").forEach((b) => b.addEventListener("click", () => {
    const v = parseInt(b.dataset.demo, 10);
    $("demoCPM").value = v;
    $("demoSlider").value = cpmToSlider(v);
    sendDemo(v);
  }));
  $("demoReal").addEventListener("change", () => sendDemo(parseInt($("demoCPM").value, 10) || 0));
}

// ---------- padrão por hora do dia (mapa de calor) ----------

function heatMonthStart(hm) { return new Date(hm.y, hm.m - 1, 1).getTime(); }

async function refreshHeat() {
  const mm = state.heatMonthMode;
  document.querySelectorAll("#heatSeg button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.hmonth ? mm : !mm && parseInt(b.dataset.hdays, 10) === state.heatDays)));
  $("heatMonthNav").hidden = !mm;
  const rcOn = hasRcData() && !!api.GetHeatmapDev;
  $("heatDevSeg").hidden = !rcOn;
  if (!rcOn) state.heatDev = "";
  const dev = state.heatDev || "";
  document.querySelectorAll("#heatDevSeg button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.hdev === dev)));
  let h;
  if (mm) {
    if (api.GetDataStart) state.heatDataStart = (await api.GetDataStart()) || 0;
    const hm = state.heatMonth, now = new Date();
    const label = new Date(hm.y, hm.m - 1, 1).toLocaleDateString(LOCALE, { month: "long", year: "numeric" });
    $("heatMonthLabel").textContent = label.charAt(0).toUpperCase() + label.slice(1);
    $("heatPrev").disabled = !!state.heatDataStart && heatMonthStart(hm) <= state.heatDataStart;
    $("heatNext").disabled = hm.y > now.getFullYear() || (hm.y === now.getFullYear() && hm.m >= now.getMonth() + 1);
    h = api.GetHeatmapMonthDev ? await api.GetHeatmapMonthDev(hm.y, hm.m, dev) : await api.GetHeatmapMonth(hm.y, hm.m);
  } else {
    h = api.GetHeatmapDev ? await api.GetHeatmapDev(state.heatDays, dev) : await api.GetHeatmap(state.heatDays);
  }
  const rcMap = h.device === "radiacode";
  const hv = (v, d) => (rcMap ? fmt(v, usvDigits(v)) : fmt(v, d));
  $("heatUnit").textContent = rcMap ? "µSv/h médio (RC102) · cinza = sem dados" : "CPM médio · cinza = sem dados";
  state.heat = h;
  hideHeatTip();
  const box = $("heat");
  box.textContent = "";
  const cell = (cls, text) => {
    const d = document.createElement("div");
    d.className = cls;
    if (text) d.textContent = text;
    box.appendChild(d);
    return d;
  };
  cell("rl", "");
  for (let hr = 0; hr < 24; hr++) cell("hl", hr % 3 === 0 ? String(hr) : "");
  const mn = h.min, mx = h.max;
  const span = mx != null && mn != null && mx > mn ? mx - mn : 1;
  // mais recente em cima
  for (let i = h.days.length - 1; i >= 0; i--) {
    const day = h.days[i];
    cell("rl", isToday(day) ? "hoje" : ddmm(day));
    for (let hr = 0; hr < 24; hr++) {
      const v = h.values[i][hr];
      const c = cell("c", "");
      c.dataset.i = i;
      c.dataset.h = hr;
      const al = (h.alerts || {})[`${i}-${hr}`];
      const ac = h.awayCounts && h.awayCounts[i] ? h.awayCounts[i][hr] : 0;
      if (ac && !(h.counts && h.counts[i] && h.counts[i][hr]) && !(al && al.length)) c.classList.add("away"); // a hora toda fora de casa
      else if (ac) c.classList.add("away-part");
      if (al && al.length) {
        // hora com alerta: laranja (atenção) ou vermelho (alarme)
        c.classList.add(al.some((x) => x.level === "perigo") ? "danger" : al.some((x) => x.level === "alarme") ? "alarm" : "att");
      } else if (v != null) {
        c.style.background = `rgba(${rcMap ? "244,114,182" : "110,231,160"},${(0.15 + 0.85 * ((v - mn) / span)).toFixed(3)})`;
      }
    }
  }
  markHeatPick();
  $("heatMin").textContent = mn == null ? "—" : hv(mn, 0);
  $("heatMax").textContent = mx == null ? "—" : hv(mx, 0);
  document.querySelector("#heatCard .heat-ramp").classList.toggle("rc", rcMap);
  const daysWith = h.values.filter((row) => row.some((v) => v != null)).length;
  if (h.peakHour < 0) $("heatSub").textContent = "sem dados ainda";
  else if (daysWith < 2) $("heatSub").textContent = "com mais dias de dados dá para ver se o fundo muda com a hora";
  else {
    const p = h.hourAvg[h.peakHour], l = h.hourAvg[h.lowHour];
    const u = rcMap ? " µSv/h" : " CPM";
    $("heatSub").textContent = `mais alto por volta das ${h.peakHour}h (${hv(p, 1)}${u}) · mais baixo por volta das ${h.lowHour}h (${hv(l, 1)}${u})`;
  }
}

// caixinha com o valor de cada quadrado
function showHeatTip(c) {
  const h = state.heat;
  if (!h || c.dataset.i == null) return;
  const i = +c.dataset.i, hr = +c.dataset.h;
  const v = h.values[i][hr];
  const day = new Date(h.days[i]);
  const dayTxt = isToday(h.days[i]) ? "hoje" : isYesterday(h.days[i]) ? "ontem" :
    day.toLocaleDateString(LOCALE, { weekday: "short", day: "2-digit", month: "2-digit" }).replace(".", "");
  const tip = $("heatTip");
  tip.textContent = "";
  const line = (txt, cls) => { const d = document.createElement("div"); if (cls) d.className = cls; d.textContent = txt; tip.appendChild(d); };
  line(`${dayTxt} · ${pad(hr)}h–${pad((hr + 1) % 24)}h`, "ht-when");
  const onlyAway = v == null && h.awayCounts && h.awayCounts[i] && h.awayCounts[i][hr];
  if (v == null) {
    if (!onlyAway) line("sem dados", "ht-none");
  } else {
    const rcMap = h.device === "radiacode";
    line(rcMap ? `${fmt(v, usvDigits(v))} µSv/h` : `${fmt(v, 1)} CPM`, "ht-val");
    const u = h.usv && h.usv[i] ? h.usv[i][hr] : null;
    if (u != null && !rcMap) line(`≈ ${fmt(u, usvDigits(u))} µSv/h`);
    const avg = h.hourAvg[hr];
    const n = h.counts && h.counts[i] ? h.counts[i][hr] : 0;
    if (rcMap && avg != null && h.days.length > 1) {
      const d = v - avg;
      line(Math.abs(d) < avg * 0.005 ? `igual à média das ${hr}h` : `${d > 0 ? "+" : "−"}${fmt(Math.abs(d), usvDigits(avg) + 1)} da média das ${hr}h (${fmt(avg, usvDigits(avg))})`, "ht-dim");
    } else if (avg != null && h.days.length > 1) {
      const d = v - avg;
      // σ de uma média de n minutos: √(média ÷ n), bem menor que o de 1 minuto
      const sm = n > 0 ? Math.sqrt(avg / n) : 0;
      line(Math.abs(d) < 0.05 ? `igual à média das ${hr}h` : `${d > 0 ? "+" : "−"}${fmt(Math.abs(d), 1)} da média das ${hr}h (${fmt(avg, 1)})` + (sm > 0 ? ` · ${fmtSig(d / sm)}` : ""), "ht-dim");
    }
    if (n) line(n >= 60 ? "hora completa" : `${n} de 60 min medidos`, "ht-dim");
  }
  const ac = h.awayCounts && h.awayCounts[i] ? h.awayCounts[i][hr] : 0;
  if (ac) {
    const av = h.awayValues && h.awayValues[i] ? h.awayValues[i][hr] : null;
    line(`fora de casa: ${ac} min${av != null ? ` · ${h.device === "radiacode" ? fmt(av, usvDigits(av)) + " µSv/h" : fmt(av, 1) + " CPM"}` : ""} (fora do padrão)`, "ht-away");
  }
  // o mais perigoso primeiro (perigo, alarme, atenção); no mesmo nível, o maior pico
  const rank = { perigo: 0, alarme: 1 };
  const al = [...((h.alerts || {})[`${i}-${hr}`] || [])].sort((x, y) =>
    (rank[x.level] ?? 2) - (rank[y.level] ?? 2) || (y.peak || 0) - (x.peak || 0) || x.time - y.time);
  for (const a of al.slice(0, 4)) {
    const nm = { perigo: "Perigo", alarme: "Alarme" }[a.level] || "Atenção";
    line(`${nm} às ${hhmm(a.time)}${a.peak ? " · pico " + (h.device === "radiacode" ? fmt(a.peak, usvDigits(a.peak)) + " µSv/h" : fmt(a.peak, 0) + " CPM") : ""}${a.retro ? " (do contador)" : ""}`, { perigo: "ht-danger", alarme: "ht-alarm" }[a.level] || "ht-att");
  }
  if (al.length > 4) line(`+${al.length - 4} alertas nesta hora`, "ht-dim");
  tip.hidden = false;
  const r = c.getBoundingClientRect(), tr = tip.getBoundingClientRect();
  let x = r.left + r.width / 2 - tr.width / 2;
  x = Math.max(8, Math.min(window.innerWidth - tr.width - 8, x));
  let y = r.top - tr.height - 8;
  if (y < 8) y = r.bottom + 8;
  tip.style.left = x + "px";
  tip.style.top = y + "px";
  document.querySelectorAll("#heat .c.hover").forEach((e) => e.classList.remove("hover"));
  c.classList.add("hover");
}

function hideHeatTip() {
  const tip = $("heatTip");
  if (tip) tip.hidden = true;
  document.querySelectorAll("#heat .c.hover").forEach((e) => e.classList.remove("hover"));
}

// Clicar num quadrado (ou arrastar por vários na mesma linha) abre aquele
// trecho nos gráficos de CPM e de dose, como se tivesse dado zoom neles.
function heatCellTime(i, hr) {
  const d = new Date(state.heat.days[i]);
  d.setHours(hr, 0, 0, 0);
  return d.getTime();
}

function markHeatPick() {
  document.querySelectorAll("#heat .c.pick").forEach((e) => e.classList.remove("pick"));
  const p = state.heatPick;
  if (!p || !state.heat) return;
  const i = state.heat.days.indexOf(p.day);
  if (i < 0) return;
  document.querySelectorAll(`#heat .c[data-i="${i}"]`).forEach((c) => {
    const h = +c.dataset.h;
    if (h >= p.h0 && h <= p.h1) c.classList.add("pick");
  });
}

// 2.12: o trecho abre logo embaixo do mapa (taxa e dose de cada aparelho),
// sem sair da aba; "Abrir na aba Dose" leva o mesmo trecho para lá
function openHeatRange(i, h0, h1) {
  const from = heatCellTime(i, h0), to = heatCellTime(i, h1) + 3600e3;
  state.heatPick = { day: state.heat.days[i], h0, h1, from, to };
  markHeatPick();
  $("heatTrecho").hidden = false;
  renderHeatTrecho();
  const card = $("heatTrecho");
  requestAnimationFrame(() => {
    const r = card.getBoundingClientRect();
    if (r.bottom > window.innerHeight) card.scrollIntoView({ behavior: "smooth", block: "end" });
  });
}

function closeHeatTrecho() {
  state.heatPick = null;
  markHeatPick();
  $("heatTrecho").hidden = true;
}

let htSeq = 0;
async function renderHeatTrecho() {
  const p = state.heatPick;
  if (!p) return;
  const my = ++htSeq;
  const rcOn = hasRcData();
  const day = new Date(p.from);
  $("htChip").textContent = `${ddmm(p.from)} · ${hhmm(p.from)}–${p.h1 === 23 ? "24:00" : hhmm(p.to)}`;
  if (isToday(day)) $("htChip").textContent = "hoje · " + $("htChip").textContent.split(" · ")[1];
  const range = { from: p.from, to: p.to };
  const [ch, dg, dr] = await Promise.all([
    api.GetChartRange(p.from, p.to).catch(() => null),
    fetchDose("gmc", range).catch(() => null),
    rcOn ? fetchDose("radiacode", range).catch(() => null) : null,
  ]);
  if (my !== htSeq || state.heatPick !== p) return;
  state.ht = { ch, dg, dr, rcOn };
  drawHeatTrecho();
}

function drawHeatTrecho() {
  const p = state.heatPick, h = state.ht;
  if (!p || !h) return;
  const { ch, dg, dr, rcOn } = h;
  renderDevBoxes($("htDevs"), rcOn ? [{ key: "gmc", name: "GMC-800", c: dg }, { key: "rc", name: "RC102", c: dr }] : [{ key: "gmc", name: "GMC-800", c: dg }]);
  const showG = !rcOn || state.doseShow.gmc, showR = rcOn && state.doseShow.rc;
  $("htKeyRc").hidden = !showR;
  $("htKeyGmc").hidden = !showG;
  // taxa
  const VW = 600, RH = 120, DH = 100;
  const g = ch && showG ? ch.usv || [] : [], r = ch && showR ? ch.rc || [] : [];
  const vals = g.concat(r).filter((v) => v != null && isFinite(v));
  const attn = ch && ch.attentionUSv;
  const rtop = niceTop(Math.max(...vals, 0.05) * 1.15);
  const ry = (v) => RH - (v / rtop) * RH;
  const line = (arr, n, yf, color, dash) => {
    let d = "", pen = false;
    arr.forEach((v, k) => {
      if (v == null || !isFinite(v)) { pen = false; return; }
      d += (pen ? "L" : "M") + ((k / Math.max(1, n - 1)) * VW).toFixed(1) + " " + yf(v).toFixed(1);
      pen = true;
    });
    return d ? `<path d="${d}" fill="none" stroke="${color}" stroke-width="${dash ? 1.4 : 1.8}"${dash ? ` stroke-dasharray="${dash}"` : ""} stroke-linejoin="round" vector-effect="non-scaling-stroke"/>` : "";
  };
  const grid = (hh) => [0.5, hh / 2, hh - 0.5].map((gy) => `<line x1="0" y1="${gy}" x2="${VW}" y2="${gy}" stroke="${COR.grid}" vector-effect="non-scaling-stroke"/>`).join("");
  $("htRate").innerHTML = grid(RH) +
    (attn && attn < rtop ? `<line x1="0" y1="${ry(attn)}" x2="${VW}" y2="${ry(attn)}" stroke="${COR.att}" stroke-opacity=".6" stroke-dasharray="6 5" vector-effect="non-scaling-stroke"/>` : "") +
    line(g, g.length, ry, COR.accent) + line(r, r.length, ry, COR.rc);
  $("htRateY").innerHTML = `<span>${fmt(rtop, usvDigits(rtop))}</span><span>${fmt(rtop / 2, usvDigits(rtop / 2))}</span><span>0</span>`;
  // dose acumulada: cada aparelho a sua, com o seu teto (o do Radiacode cheio,
  // o do GMC pontilhado quando os dois aparecem)
  const G = showG && dg && dg.measuredMinutes ? dg : null, R = showR && dr && dr.measuredMinutes ? dr : null;
  const dtop = niceTop(Math.max(G ? Math.max(G.total, G.ceilingTotal) : 0, R ? Math.max(R.total, R.ceilingTotal) : 0) * 1.1 || 1);
  const dy = (v) => DH - (v / dtop) * DH;
  $("htDose").innerHTML = grid(DH) +
    (R ? line(R.ceiling, R.ceiling.length, dy, COR.warn) : "") +
    (G ? line(G.ceiling, G.ceiling.length, dy, COR.warn, R ? "2 4" : "") : "") +
    (G ? line(G.dose, G.dose.length, dy, COR.accent) : "") +
    (R ? line(R.dose, R.dose.length, dy, COR.rc) : "");
  $("htDoseY").innerHTML = `<span>${fmtDose(dtop)[0]}</span><span>${fmtDose(dtop / 2)[0]}</span><span>0</span>`;
  $("htX0").textContent = hhmm(p.from);
  $("htX1").textContent = hhmm((p.from + p.to) / 2);
  $("htX2").textContent = p.h1 === 23 ? "24:00" : hhmm(p.to);
}

function wireHeatTrecho() {
  $("htClose").addEventListener("click", closeHeatTrecho);
  $("htOpen").addEventListener("click", () => { const p = state.heatPick; if (p) showRange(p.from, p.to); });
  $("htAway").addEventListener("click", () => {
    const p = state.heatPick;
    if (p) openPeriods({ from: p.from, to: p.to });
  });
}

function wireHeat() {
  const grid = $("heat");
  grid.addEventListener("mouseover", (e) => { const c = e.target.closest(".c"); if (c) showHeatTip(c); });
  grid.addEventListener("mouseleave", hideHeatTip);
  // selecionar: só dentro da mesma linha (o mesmo dia)
  let drag = null;
  const paint = () => {
    document.querySelectorAll("#heat .c.picking").forEach((e) => e.classList.remove("picking"));
    if (!drag) return;
    const lo = Math.min(drag.h0, drag.h1), hi = Math.max(drag.h0, drag.h1);
    document.querySelectorAll(`#heat .c[data-i="${drag.i}"]`).forEach((c) => {
      const h = +c.dataset.h;
      if (h >= lo && h <= hi) c.classList.add("picking");
    });
  };
  grid.addEventListener("mousedown", (e) => {
    const c = e.target.closest(".c");
    if (!c || e.button !== 0 || !state.heat) return;
    e.preventDefault();
    drag = { i: +c.dataset.i, h0: +c.dataset.h, h1: +c.dataset.h };
    paint();
  });
  grid.addEventListener("mousemove", (e) => {
    if (!drag) return;
    const c = e.target.closest(".c");
    if (c && +c.dataset.i === drag.i && +c.dataset.h !== drag.h1) { drag.h1 = +c.dataset.h; paint(); }
  });
  window.addEventListener("mouseup", () => {
    if (!drag) return;
    const d = drag;
    drag = null;
    paint();
    openHeatRange(d.i, Math.min(d.h0, d.h1), Math.max(d.h0, d.h1));
  });
  window.addEventListener("scroll", hideHeatTip, { passive: true });
  wireHeatTrecho();
  wireBgTrend();
  wirePatterns();
  document.querySelectorAll("#heatDevSeg button").forEach((b) => b.addEventListener("click", () => { state.heatDev = b.dataset.hdev; refreshHeat(); }));
  document.querySelectorAll("#heatSeg button").forEach((b) => b.addEventListener("click", () => {
    if (b.dataset.hmonth) {
      state.heatMonthMode = true;
    } else {
      state.heatMonthMode = false;
      state.heatDays = parseInt(b.dataset.hdays, 10); localSet("heatDays", String(state.heatDays));
    }
    localSet("heatMode", state.heatMonthMode ? "mes" : "dias");
    refreshHeat();
  }));
  const stepMonth = (d) => {
    const hm = state.heatMonth, t = new Date(hm.y, hm.m - 1 + d, 1);
    state.heatMonth = { y: t.getFullYear(), m: t.getMonth() + 1 };
    refreshHeat();
  };
  $("heatPrev").addEventListener("click", () => stepMonth(-1));
  $("heatNext").addEventListener("click", () => stepMonth(1));
}

// ---------- chuva (configurações) ----------

function renderRainSettings() {
  const s = state.settings;
  $("chkRain").checked = !!s.rainEnabled;
  $("placeChosen").textContent = s.rainPlace ? (s.rainHidePlace ? "Local: oculto" : "Local: " + s.rainPlace) : "Nenhuma cidade escolhida.";
  $("chkRainHide").checked = !!s.rainHidePlace;
}

function wireRain() {
  document.querySelectorAll("#riDevSeg button").forEach((b) => b.addEventListener("click", () => {
    localSet("riDev", b.dataset.ridev);
    refreshRainImpact(true);
  }));
  const search = async () => {
    const ul = $("placeList");
    ul.innerHTML = '<li class="muted small">buscando…</li>';
    let list = [];
    try { list = await api.SearchPlace($("txtPlace").value); } catch (e) { ul.innerHTML = `<li class="muted small">${errMsg(e)}</li>`; return; }
    ul.textContent = "";
    if (!list.length) { ul.innerHTML = '<li class="muted small">Nenhuma cidade encontrada.</li>'; return; }
    for (const p of list) {
      const li = document.createElement("li");
      const b = document.createElement("button");
      b.type = "button";
      b.textContent = p.detail ? `${p.name} — ${p.detail}` : p.name;
      b.addEventListener("click", async () => {
        await saveSettings({ rainEnabled: true, rainLat: p.latitude, rainLon: p.longitude, rainPlace: b.textContent });
        ul.textContent = "";
        renderRainSettings();
        refreshChart();
      });
      li.appendChild(b);
      ul.appendChild(li);
    }
  };
  $("btnPlace").addEventListener("click", search);
  $("chkRainHide").addEventListener("change", async (e) => {
    await saveSettings({ rainHidePlace: e.target.checked });
    renderRainSettings();
    renderRainStatus();
  });
  $("rainStatus").addEventListener("click", async () => {
    if (state.rainStatus && state.rainStatus.enabled) {
      try { state.rainStatus = await api.RefreshRain(); } catch (e) { /* o status mostra o erro */ }
      renderRainStatus();
      refreshChart();
      return;
    }
    await openSettings("txtPlace");
    $("txtPlace").focus();
  });
  $("txtPlace").addEventListener("keydown", (e) => { if (e.key === "Enter") { e.preventDefault(); search(); } });
  $("chkRain").addEventListener("change", async (e) => {
    if (e.target.checked && !state.settings.rainPlace) {
      e.target.checked = false;
      toast("Escolha a cidade primeiro.");
      $("txtPlace").focus();
      return;
    }
    await saveSettings({ rainEnabled: e.target.checked });
    renderRainSettings();
    refreshChart();
  });
}

// ---------- status da busca de chuva ----------
// Fica sempre visível embaixo do gráfico: desligada, buscando, atualizada ou
// com erro.

state.rainStatus = { enabled: false };
const RADAR_STEP = 10 * 60e3;

function rainWord(mmh) {
  const k = mmh < 2.5 ? "fraca" : mmh < 10 ? "moderada" : mmh < 50 ? "forte" : "muito forte";
  return `${k}, ~${fmt(mmh, mmh < 10 ? 1 : 0)} mm/h`;
}

function renderRainStatus() {
  const st = state.rainStatus || {};
  const btn = $("rainStatus");
  let text, cls, title;
  const place = state.settings.rainHidePlace ? "" : st.place ? st.place.split(" — ")[0] : "";
  if (!st.enabled) {
    text = "chuva: desligada";
    cls = "off";
    title = "O app não está buscando dados de chuva. Clique para ligar nas Configurações.";
  } else if (st.busy) {
    text = `buscando chuva${place ? " em " + place : ""}…`;
    cls = "busy";
    title = "Buscando a chuva por hora no Open-Meteo.";
  } else if (st.error) {
    text = `chuva: sem acesso ao serviço de clima${st.next ? " · tenta de novo às " + hhmm(st.next) : ""}`;
    cls = "err";
    title = st.error;
  } else if (st.radarFetched && st.radarCovered !== false) {
    const now = st.radarNow;
    const fresh = now && Date.now() - now.time < 40 * 60e3;
    text = `chuva${place ? " em " + place : ""} · ` + (fresh && now.mmh > 0 ? `chovendo (${rainWord(now.mmh).split(",")[0]})` : "sem chuva") + ` · radar às ${hhmm(now ? now.time : st.radarFetched)}`;
    cls = "on";
    title = "Radar meteorológico (RainViewer), lido a cada 10 min no seu local. Onde não há leitura do radar, o gráfico usa o modelo do Open-Meteo (barras mais claras)." + (st.radarError ? " Última tentativa: " + st.radarError : "") + " Clique para atualizar agora.";
  } else if (st.fetched) {
    text = `chuva${place ? " em " + place : " configurada"} · modelo às ${hhmm(st.fetched)}` + (st.radarCovered === false ? " · sem radar aqui" : "");
    cls = "on";
    title = (st.radarCovered === false ? "O radar não cobre o seu local, então a chuva vem só do modelo do tempo (Open-Meteo), que pode não mostrar chuva bem localizada." : st.radarError ? "Radar indisponível agora (" + st.radarError + "); usando o modelo do Open-Meteo, que pode não mostrar chuva bem localizada." : "Modelo do tempo do Open-Meteo, aguardando o radar.") + " Clique para atualizar agora.";
  } else {
    text = `chuva ligada${place ? " · " + place : ""} · aguardando a primeira busca`;
    cls = "on";
    title = "";
  }
  btn.className = "rain-status " + cls;
  $("rainStatusText").textContent = text;
  btn.title = title;
  const set = $("rainStatusSet");
  const radarLine = !st.enabled ? "" : st.radarCovered === false ? " Radar: sem cobertura no seu local." : st.radarFetched ? ` Radar: lido às ${hhmm(st.radarFetched)}, a cada 10 min.` : st.radarError ? " Radar: " + st.radarError : "";
  set.textContent = st.enabled ? (st.busy ? "Buscando agora…" : st.error ? "Modelo: última tentativa falhou: " + st.error : st.fetched ? `Modelo: última busca às ${hhmm(st.fetched)}.` : "Ainda não buscou.") + radarLine : "Desligada: o app não consulta o serviço de clima.";
  set.className = "small " + (st.error ? "rs-err" : "muted");
}

async function refreshRainStatus() {
  try { state.rainStatus = await api.GetRainStatus(); } catch (e) { /* segue */ }
  renderRainStatus();
}

// ---------- fora de casa (o resto está em fora.js) ----------
state.periodSum = {};

// ---------- alarme ----------
//
// Visual: faixa fixa no topo, sem piscar (a moldura e a animação atrapalhavam
// a leitura durante o alarme). Som: sirene ou bipes em loop até o alarme
// acabar ou alguém clicar em Silenciar (Esc também silencia).

let audio = null;
// Dois níveis:
//  alarme: o limite que o usuário escolheu (laranja, som das Configurações,
//          "Silenciar" cala até o alarme acabar);
//  perigo: fixo em 1.000 CPM (vermelho, som próprio mais agressivo, não dá
//          para desligar nem trocar; "Silenciar" cala por 5 min e, se ainda
//          estiver alto, volta a tocar). O perigo tem prioridade sobre o alarme.
// níveis do app (aba "Níveis do app" nas Configurações)
const LV_DEFAULT = { attentionCPM: 50, dangerCPM: 1000, snoozeMin: 5, satMaxUSv: 2000, satDropUSv: 1000, satHoldSec: 60, rcSatUSv: 1000, deadTimeOn: false, deadTimeUs: 200 };
function lv() {
  const out = Object.assign({}, LV_DEFAULT);
  const s = (state.settings && state.settings.levels) || {};
  for (const k of Object.keys(out)) if (s[k] > 0) out[k] = s[k];
  return out;
}
const dangerCPM = () => lv().dangerCPM;
// limites do GMC em µSv/h (o que a pessoa escolhe desde a 2.29); sem o valor
// salvo, sai do CPM pela calibração atual
const usv3 = (v) => (v > 0 ? Number(v.toPrecision(3)) : 0);
const attnUSv = () => { const l = (state.settings && state.settings.levels) || {}; return l.attentionUSv > 0 ? l.attentionUSv : lv().attentionCPM * calK(); };
const dangerUSv = () => { const l = (state.settings && state.settings.levels) || {}; return l.dangerUSv > 0 ? l.dangerUSv : dangerCPM() * calK(); };
const alarmUSv = () => { const s = state.settings || {}; return s.alarmUSv > 0 ? s.alarmUSv : (s.alarmCPM || 100) * calK(); };
const usvTxt = (v) => `${fmt(v, usvDigits(v))} µSv/h`;
const snoozeMs = () => lv().snoozeMin * 60000;
const alarmCtl = {
  kind: null, sev: null, muted: false, snoozeUntil: 0, snoozeTimer: null, sound: null,
  timer: null, nodes: [], testUntil: 0, testSev: "alarme",
  real: false, demo: false, realDanger: false, demoDanger: false,
};

function audioCtx() {
  audio = audio || new (window.AudioContext || window.webkitAudioContext)();
  if (audio.state === "suspended") audio.resume();
  return audio;
}
// libera o áudio no primeiro clique, por garantia
document.addEventListener("pointerdown", () => { try { audioCtx(); } catch (e) { /* sem áudio */ } }, { once: true });

function stopSound() {
  clearInterval(alarmCtl.timer);
  alarmCtl.timer = null;
  alarmCtl.sound = null;
  for (const n of alarmCtl.nodes) {
    try { n.g.gain.setTargetAtTime(0.0001, audio.currentTime, 0.02); n.o.stop(audio.currentTime + 0.15); } catch (e) { /* já parou */ }
  }
  alarmCtl.nodes = [];
}

function startSound(mode, fast) {
  stopSound();
  alarmCtl.sound = mode + (fast ? "+" : "");
  if (mode === "mudo") return;
  let ctx;
  try { ctx = audioCtx(); } catch (e) { return; }
  if (mode === "perigo") {
    // perigo: duas ondas quadradas em trítono (o intervalo mais áspero),
    // subindo rápido de 900 a 2.600 Hz quatro vezes por segundo, mais alto
    const g = ctx.createGain();
    g.gain.value = 0.0001;
    g.connect(ctx.destination);
    const oscs = [1, Math.SQRT2].map((ratio) => {
      const o = ctx.createOscillator();
      o.type = "square";
      o.connect(g);
      o.start();
      alarmCtl.nodes.push({ o, g });
      return { o, ratio };
    });
    g.gain.exponentialRampToValueAtTime(0.42, ctx.currentTime + 0.03);
    const period = 0.25;
    const yelp = () => {
      const t = ctx.currentTime;
      for (const { o, ratio } of oscs) {
        o.frequency.cancelScheduledValues(t);
        o.frequency.setValueAtTime(900 * ratio, t);
        o.frequency.exponentialRampToValueAtTime(2600 * ratio, t + period * 0.9);
      }
      // corta o som no fim de cada subida: fica "picotado", mais urgente
      g.gain.cancelScheduledValues(t);
      g.gain.setValueAtTime(0.42, t);
      g.gain.setValueAtTime(0.42, t + period * 0.85);
      g.gain.linearRampToValueAtTime(0.02, t + period * 0.98);
    };
    yelp();
    alarmCtl.timer = setInterval(yelp, period * 1000);
    return;
  }
  if (mode === "bipes") {
    const burst = () => {
      for (let k = 0; k < 4; k++) {
        const o = ctx.createOscillator(), g = ctx.createGain();
        o.type = "square";
        o.frequency.value = 2200;
        o.connect(g); g.connect(ctx.destination);
        const t0 = ctx.currentTime + k * 0.14;
        g.gain.setValueAtTime(0.0001, t0);
        g.gain.exponentialRampToValueAtTime(0.4, t0 + 0.005);
        g.gain.setValueAtTime(0.4, t0 + 0.08);
        g.gain.exponentialRampToValueAtTime(0.0001, t0 + 0.1);
        o.start(t0); o.stop(t0 + 0.12);
      }
    };
    burst();
    alarmCtl.timer = setInterval(burst, fast ? 650 : 900);
    return;
  }
  // sirene: duas ondas dente-de-serra levemente desafinadas, varrendo 600–1500 Hz
  const g = ctx.createGain();
  g.gain.value = 0.0001;
  g.connect(ctx.destination);
  const oscs = [0, 7].map((detune) => {
    const o = ctx.createOscillator();
    o.type = "sawtooth";
    o.detune.value = detune * 10;
    o.connect(g);
    o.start();
    alarmCtl.nodes.push({ o, g });
    return o;
  });
  g.gain.exponentialRampToValueAtTime(0.28, ctx.currentTime + 0.05);
  const period = fast ? 0.5 : 1.0;
  const sweep = () => {
    const t = ctx.currentTime;
    for (const o of oscs) {
      o.frequency.cancelScheduledValues(t);
      o.frequency.setValueAtTime(600, t);
      o.frequency.linearRampToValueAtTime(1500, t + period / 2);
      o.frequency.linearRampToValueAtTime(600, t + period);
    }
  };
  sweep();
  alarmCtl.timer = setInterval(sweep, period * 1000);
}

// updateAlarm decide o que mostrar a partir das leituras mais recentes.
function updateAlarm() {
  const now = Date.now();
  const testing = alarmCtl.testUntil > now;
  const realOn = alarmCtl.real || alarmCtl.realDanger;
  const rcOn = alarmCtl.rc || alarmCtl.rcDanger;
  const demoOn = alarmCtl.demo || alarmCtl.demoDanger;
  // GMC e Radiacode juntos: mostra o mais grave (empate: o GMC)
  const kind = realOn && !(alarmCtl.rcDanger && !alarmCtl.realDanger) ? "real" : rcOn ? "rc" : demoOn ? "demo" : testing ? "test" : null;
  const r = kind === "real" ? (state.demo.active ? state.lastReal : state.last) : kind === "demo" ? state.last : null;
  const sev = kind === "real" ? (alarmCtl.realDanger ? "perigo" : "alarme")
    : kind === "rc" ? (alarmCtl.rcDanger ? "perigo" : "alarme")
    : kind === "demo" ? (alarmCtl.demoDanger ? "perigo" : "alarme")
    : kind === "test" ? alarmCtl.testSev : null;
  const limit = state.settings.alarmCPM || 100;
  const rcs = state.rc.st || {}, rcl = rcs.limits || {};
  const cpm = kind === "test" ? (sev === "perigo" ? dangerCPM() * 1.2 : limit * 2.5) : r ? r.cpm : 0;
  const fast = sev === "alarme" && (kind === "rc" ? rcs.usvh >= (rcl.alarm || Infinity) * 2 : cpm >= limit * 2);

  if (kind !== alarmCtl.kind || sev !== alarmCtl.sev) {
    setTimeout(refreshChart, 0); // mostra ou tira as linhas dos limites no gráfico
    const was = alarmCtl.kind, wasSev = alarmCtl.sev;
    alarmCtl.kind = kind;
    alarmCtl.sev = sev;
    if (!kind) {
      stopSound();
      alarmCtl.muted = false;
      clearSnooze();
      if (!$("alarmGuide").hidden) toggleGuide("btnAlarmGuide", "alarmGuide");
    } else if (sev === "perigo" && wasSev !== "perigo") {
      clearSnooze(); // perigo novo: toca mesmo que o alarme tenha sido silenciado
    } else if (sev === "alarme" && wasSev === "perigo") {
      clearSnooze(); // saiu do perigo e voltou ao alarme: segue o que estava no alarme
    } else if (!was || kind === "real" || kind === "rc") {
      alarmCtl.muted = false; // alarme novo (ou o real por cima do demo) volta a tocar
    }
  }
  // som: o que deveria estar tocando agora
  const snoozed = alarmCtl.snoozeUntil > now;
  let want = null;
  if (sev === "perigo") want = snoozed ? null : "perigo";
  else if (sev === "alarme") want = alarmCtl.muted ? null : (kind === "test" ? $("selSound").value : (state.settings.alarmSound || "sirene")) + (fast ? "+" : "");
  if (want !== alarmCtl.sound) {
    if (want) startSound(want.replace("+", ""), want.endsWith("+"));
    else stopSound();
  }

  const on = !!kind;
  document.body.classList.toggle("alarming", on);
  document.body.classList.toggle("danger", sev === "perigo");
  const usv = kind === "test" ? cpm * (calK()) : kind === "rc" ? rcs.usvh || 0 : r ? r.usv : 0;
  bandEpisode(kind, sev, usv);
  updateAttention();
  const bar = $("alarmBar");
  bar.hidden = !on;
  if (!on) return;
  const fake = kind === "demo" || kind === "test" || (kind === "rc" && !!rcs.demo);
  const perigo = sev === "perigo";
  bar.classList.toggle("demo", fake);
  bar.classList.toggle("perigo", perigo);
  bar.classList.toggle("alarme", !perigo);
  // o perigo nunca fica calmo: silenciado, só ganha a etiqueta com a hora
  bar.classList.toggle("muted", !perigo && alarmCtl.muted);

  const dev = kind === "rc" ? "RC102" : "GMC-800";
  const rcSat = kind === "rc" && rcs.usvh >= lv().rcSatUSv;
  const sat = rcSat ? "escala" : r && r.saturated ? r.satReason || "limite" : null;
  $("alarmIco").innerHTML = LV_PATH[sat ? "perigo" : sev];
  const tag = fake ? (kind === "test" ? "TESTE" : "DEMO") : kind === "real" && state.demo.active ? "REAL" : "";
  $("alarmTag").textContent = tag;
  $("alarmTag").hidden = !tag;
  $("alarmVal").textContent = (sat ? "≥ " : "") + (usv >= 100 ? `${fmt(usv, 0)} µSv/h` : usvTxt(usv));
  if (sat) {
    $("alarmTitle").textContent = `${dev} saturado`;
    $("alarmDetail").textContent = sat === "escala" ? "fora da escala do aparelho"
      : sat === "limite" ? "acima do que o aparelho mede" : "caiu de repente após valor extremo · o tubo pode ter travado";
  } else {
    $("alarmTitle").textContent = `${BAND_NAME[sev]} no ${dev}`;
    $("alarmDetail").textContent = perigo
      ? `perigo a partir de ${kind === "rc" ? usvTxt(rcl.danger || 0) : usvTxt(dangerUSv())}`
      : kind === "rc" ? (rcl.device ? `alarme 2 do aparelho: ${usvTxt(rcl.alarm || 0)}` : `limite ${usvTxt(rcl.alarm || 0)}`)
      : `seu limite: ${usvTxt(alarmUSv())}`;
  }
  // o outro aparelho: no mesmo nível, os dois números lado a lado; abaixo,
  // a linha "Também"
  bandOther("alarm", kind, sat ? null : sev, usv);
  if (sat) $("alarmBoth").hidden = $("alarmAgree").hidden = true;
  $("alarmChip").hidden = perigo || !alarmCtl.muted;
  renderRise(perigo && r ? r.rise : null, usv);
  const g = guidance(perigo ? "perigo" : "alarme", sat ? 1000 : usv);
  if (perigo) g.steps = g.steps.filter((t) => t !== PHONES); // os telefones já ficam à vista na faixa
  fillGuide("alarmGuide", "alarmHint", g);
  if (sat) $("alarmHint").textContent = "Não confie no número. Afaste-se agora.";
  const note = $("alarmNote");
  note.hidden = !perigo;
  if (perigo) {
    const key = PHONES + (snoozed ? alarmCtl.snoozeUntil : "");
    if (note.dataset.key !== key) {
      note.dataset.key = key;
      note.textContent = "";
      for (const t of PHONES.split(" · ")) { const sp = document.createElement("span"); sp.className = "phone mono"; sp.textContent = t; note.appendChild(sp); }
      if (snoozed) { const c = document.createElement("span"); c.className = "chip"; c.textContent = `som pausado até ${hhmm(alarmCtl.snoozeUntil)}`; note.appendChild(c); }
    }
  }
  const quiet = perigo ? snoozed : alarmCtl.muted;
  const btn = $("btnSilence");
  btn.textContent = quiet ? "Ligar o som" : perigo ? "Silenciar 5 min" : "Silenciar";
  btn.classList.toggle("solid", !quiet);
  btn.disabled = false;
}

// nomes dos níveis na faixa ("Alarme no GMC-800") e na linha "Também"
const BAND_NAME = { atencao: "Atenção", alarme: "Alarme", perigo: "Perigo" };
const BAND_LOW = { atencao: "atenção", alarme: "alarme", perigo: "perigo" };
const BAND_RANK = { atencao: 1, alarme: 2, perigo: 3 };

// nível e valor de agora do aparelho que não é o da faixa
function bandOtherOf(kind) {
  if (kind === "real" || kind === "demo") {
    const st = state.rc.st || {};
    return { dev: "RC102", lv: alarmCtl.rcDanger ? "perigo" : alarmCtl.rc ? "alarme" : attnCtl.rc ? "atencao" : null, usv: st.usvh || 0 };
  }
  if (kind === "rc") {
    const r = state.demo.active ? state.lastReal : state.last;
    return { dev: "GMC-800", lv: alarmCtl.realDanger ? "perigo" : alarmCtl.real ? "alarme" : attnCtl.real ? "atencao" : null, usv: r ? r.usv : 0 };
  }
  return null;
}

// pfx "alarm" ou "attn": preenche "nos dois aparelhos" ou a linha "Também"
function bandOther(pfx, kind, lv, usv) {
  const o = lv ? bandOtherOf(kind) : null;
  const same = !!o && o.lv === lv;
  const also = !!o && !!o.lv && !same && BAND_RANK[o.lv] < BAND_RANK[lv];
  const dev = kind === "rc" ? "RC102" : "GMC-800";
  $(pfx + "Both").hidden = $(pfx + "Agree").hidden = !same;
  $(pfx + "Also").hidden = !also;
  $(pfx + "Val").hidden = same;
  if (same) {
    $(pfx + "Title").textContent = `${BAND_NAME[lv]} nos dois aparelhos`;
    const box = $(pfx + "Both");
    box.textContent = "";
    [[dev, usv, false], [o.dev, o.usv, true]].forEach(([d, v, unit]) => {
      const sp = document.createElement("span"), b = document.createElement("b");
      b.className = "val";
      b.textContent = fmt(v, usvDigits(v));
      sp.append(d + " ", b);
      if (unit) sp.append(" µSv/h");
      box.appendChild(sp);
    });
  }
  if (also) {
    const box = $(pfx + "Also");
    box.innerHTML = `<svg class="lv-ico ${o.lv}" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">${LV_PATH[o.lv]}</svg>`;
    const sp = document.createElement("span"), b = document.createElement("b");
    b.textContent = fmt(o.usv, usvDigits(o.usv));
    sp.append(`Também: ${BAND_LOW[o.lv]} no ${o.dev}`, " · ", b, " µSv/h");
    box.appendChild(sp);
  }
}

// "Voltou ao normal": guarda o pico de cada episódio real e, quando acaba,
// mostra a tira verde por 60 s (não depois de demo nem de teste)
const okCtl = { ep: null, timer: null };
function bandEpisode(kind, sev, usv) {
  const real = kind === "real" || (kind === "rc" && !(state.rc.st || {}).demo);
  if (real) {
    const ep = okCtl.ep || (okCtl.ep = { from: Date.now(), peak: 0, sev, dev: "" });
    if (BAND_RANK[sev] > BAND_RANK[ep.sev]) ep.sev = sev;
    if (usv >= ep.peak) { ep.peak = usv; ep.dev = kind === "rc" ? "RC102" : "GMC-800"; }
    hideOkBar();
    return;
  }
  if (kind) { okCtl.ep = null; hideOkBar(); return; } // demo ou teste por cima: sem tira verde
  const ep = okCtl.ep;
  okCtl.ep = null;
  if (ep && ep.peak > 0 && !attnCtl.kind) showOkBar(ep);
}
function showOkBar(ep) {
  const now = Date.now(), min = Math.max(1, Math.round((now - ep.from) / 60000));
  $("okTitle").textContent = `Voltou ao normal às ${hhmm(now)}`;
  $("okDetail").textContent = `${BAND_LOW[ep.sev]} no ${ep.dev} · ${min < 60 ? min + " min" : Math.floor(min / 60) + " h " + (min % 60) + " min"} · pico ${usvTxt(ep.peak)}`;
  $("okBar").hidden = false;
  clearTimeout(okCtl.timer);
  okCtl.timer = setTimeout(hideOkBar, 60000);
}
function hideOkBar() {
  clearTimeout(okCtl.timer);
  okCtl.timer = null;
  $("okBar").hidden = true;
}

// como a leitura chegou ao perigo (ajuda a saber se foi você ou a fonte que se moveu)
function renderRise(rise, usvNow) {
  const el = $("alarmRise");
  if (!rise) { el.hidden = true; return; }
  const from = `${fmt(rise.fromUSv, usvDigits(rise.fromUSv))} µSv/h`;
  const s = rise.seconds;
  const dur = s < 90 ? `${Math.max(1, Math.round(s))} s` : `${Math.round(s / 60)} min`;
  let text;
  if (rise.longer) text = "Subiu aos poucos (mais de 3 min): você foi chegando perto de algo.";
  else if (s <= 30) text = `Subiu de ${from} em ${dur}: algo pode ter chegado perto.`;
  else if (s >= 90) text = `Subiu de ${from} em ${dur}: você foi chegando perto de algo.`;
  else text = `Subiu de ${from} em ${dur}.`;
  el.textContent = text;
  el.hidden = false;
}

// aviso nas Configurações sobre o alarme de perigo
function renderDangerNote() {
  const v = parseFloat($("numAlarm").value) || 0;
  const el = $("dangerNote");
  const d = parseFloat($("lvDanger").value) || dangerUSv();
  if (v >= d) {
    el.className = "danger-note warn";
    el.textContent = `Seu limite (${usvTxt(v)}) está acima do perigo do app (${usvTxt(d)}): quem toca é o alarme de perigo.`;
  }
  // a escada já mostra o perigo ao lado do seu alarme: o aviso só aparece
  // quando o seu limite passa do perigo
  el.hidden = !(v >= d);
}

// ---------- aba "Níveis do app" ----------

const LV_FIELDS = { lvAttn: "attentionUSv", lvDanger: "dangerUSv", lvSnooze: "snoozeMin", lvSatMax: "satMaxUSv", lvSatDrop: "satDropUSv", lvSatHold: "satHoldSec", lvRcSat: "rcSatUSv", lvDTUs: "deadTimeUs" };

function fillLevels(l) {
  for (const [id, k] of Object.entries(LV_FIELDS)) $(id).value = l[k] ?? LV_DEFAULT[k];
  // limites em µSv/h: sem o valor salvo, o equivalente do CPM antigo
  if (!(l.attentionUSv > 0)) $("lvAttn").value = usv3((l.attentionCPM || LV_DEFAULT.attentionCPM) * calK());
  if (!(l.dangerUSv > 0)) $("lvDanger").value = usv3((l.dangerCPM || LV_DEFAULT.dangerCPM) * calK());
  $("lvDTOn").checked = !!l.deadTimeOn;
  renderLevelEq();
  renderDeadTime();
}

// tabela: quanto a compensação muda cada taxa com o τ escolhido
function dtFactor(perSec, tau) { return 1 / Math.max(0.1, 1 - perSec * tau); }
function renderDeadTime() {
  const tau = (parseFloat($("lvDTUs").value) || LV_DEFAULT.deadTimeUs) * 1e-6, on = $("lvDTOn").checked;
  $("lvDTUs").disabled = !on;
  const k = calK();
  const rows = [28, 1000, 10000, 30000, 100000, 200000].map((c) => {
    const f = dtFactor(c / 60, tau), cap = f >= 9.999;
    return `<tr><td>${fmt(c, 0)}</td><td>${fmt(c * f, 0)}</td><td>${f < 1.0005 ? "+" + fmt((f - 1) * 100, 3) : "+" + fmt((f - 1) * 100, f < 1.1 ? 1 : 0)}%${cap ? " (teto)" : ""}</td><td>${fmt(c * f * k, usvDigits(c * f * k))}</td></tr>`;
  }).join("");
  $("lvDTTable").innerHTML = `<thead><tr><th>contador (CPM)</th><th>corrigido</th><th>soma</th><th>µSv/h</th></tr></thead><tbody>${rows}</tbody>`;
  $("lvDTTable").classList.toggle("off", !on);
}

function readLevels() {
  const out = {};
  for (const [id, k] of Object.entries(LV_FIELDS)) {
    const v = parseFloat($(id).value);
    out[k] = v > 0 ? (k.endsWith("USv") || k === "deadTimeUs" ? v : Math.round(v)) : LV_DEFAULT[k];
  }
  // em branco: o padrão do app, em µSv/h pela calibração atual
  if (!(out.attentionUSv > 0)) out.attentionUSv = usv3(LV_DEFAULT.attentionCPM * calK());
  if (!(out.dangerUSv > 0)) out.dangerUSv = usv3(LV_DEFAULT.dangerCPM * calK());
  out.deadTimeOn = $("lvDTOn").checked;
  return out;
}

// mostra quanto o CPM dá em µSv/h com a calibração do contador
function renderLevelEq() {
  const k = calK();
  document.querySelectorAll(".lv-eq").forEach((el) => {
    const v = parseFloat($(el.dataset.eq).value) || 0;
    el.textContent = v ? `≈ ${fmt(v / k, 0)} CPM no GMC-800 com a calibração atual` : "";
  });
}

// ---------- configurações: 6 seções na lateral, com abas nos aparelhos ----------
//
// Cada seção é um <section class="set-sec" data-sec=…> do index.html; as dos
// aparelhos têm abas (<div class="set-tab" data-tab=…>). Cartão com data-dev
// grava no aparelho (botão próprio); o resto vai pela barra "Salvar".

const SET_LAYOUT = [
  { group: "Aparelhos", id: "gmc", name: "GQ GMC-800", tabs: [["con", "Conexão"], ["dev", "No aparelho"], ["cal", "Calibração"], ["tubo", "Tubo"], ["test", "Testes"]] },
  { id: "rc", name: "Radiacode 102", tabs: [["con", "Conexão"], ["dev", "No aparelho"]] },
  { group: "Avisos", id: "alarm", name: "Alarmes e avisos" },
  { group: "App", id: "app", name: "Janela e tela" },
  { id: "data", name: "Dados e backup" },
  { id: "about", name: "Atualizações e sobre" },
];
state.setSec = "gmc";
state.setTab = { gmc: "con", rc: "con" };
const setSecEl = (id) => document.querySelector(`#settings .set-sec[data-sec="${id}"]`);
const setLay = (id) => SET_LAYOUT.find((x) => x.id === id);
// o elemento está na seção e na aba abertas agora
function setShown(el) { return !!el && !el.closest("[hidden]"); }

function buildSetNav() {
  const nav = $("setNav");
  if (nav.childElementCount) return;
  for (const L of SET_LAYOUT) {
    if (L.group) {
      const h = document.createElement("h4");
      h.textContent = L.group;
      nav.appendChild(h);
    }
    const b = document.createElement("button");
    b.type = "button";
    b.dataset.sec = L.id;
    b.innerHTML = `<span class="set-nav-nm"></span><span class="set-nav-st" id="setSt-${L.id}" hidden><span class="dot"></span><span></span></span>`;
    b.firstChild.textContent = L.name;
    b.addEventListener("click", () => showSetSection(L.id));
    nav.appendChild(b);
    if (!L.tabs) continue;
    const bar = setSecEl(L.id).querySelector(".set-tabs");
    for (const [k, name] of L.tabs) {
      const t = document.createElement("button");
      t.type = "button";
      t.setAttribute("role", "tab");
      t.dataset.tab = k;
      t.textContent = name;
      t.addEventListener("click", () => showSetSection(L.id, k));
      bar.appendChild(t);
    }
  }
  document.querySelectorAll("#lvCard .lv-name").forEach((el) => lvLabel(el, el.dataset.lv, el.textContent.trim(), 13));
  renderSetNavStatus();
}

function showSetSection(id, tab) {
  buildSetNav();
  const L = setLay(id) || SET_LAYOUT[0];
  if (L.tabs && tab && L.tabs.some((t) => t[0] === tab)) state.setTab[L.id] = tab;
  document.querySelectorAll("#settings .set-sec").forEach((x) => { x.hidden = x.dataset.sec !== L.id; });
  const sec = setSecEl(L.id);
  if (L.tabs) {
    const cur = state.setTab[L.id];
    sec.querySelectorAll(":scope > .set-tab").forEach((t) => { t.hidden = t.dataset.tab !== cur; });
    sec.querySelectorAll(".set-tabs button").forEach((b) => b.setAttribute("aria-selected", String(b.dataset.tab === cur)));
  }
  document.querySelectorAll("#setNav button").forEach((b) => b.setAttribute("aria-current", String(b.dataset.sec === L.id)));
  state.setSec = L.id;
  if (state.view === "config") {
    const top = $("settings").getBoundingClientRect().top + window.scrollY - 60;
    if (window.scrollY > top) window.scrollTo({ top: Math.max(0, top) });
  }
  if (["devcfg", "dcCalCard", "dcTube"].some((k) => setShown($(k)))) openDevCfg();
  if (setShown($("rcCfg"))) openRcCfg();
  if (L.id === "alarm") { renderLadder(); renderDevAlarms(); openDevCfg().then(renderDevAlarms); }
}

// abre a seção e a aba onde está um campo (ex.: "txtPlace" → Janela e tela)
function showSetFor(elId, flash) {
  const el = $(elId);
  const sec = el && el.closest(".set-sec");
  if (!sec) return;
  const t = el.closest(".set-tab");
  showSetSection(sec.dataset.sec, t ? t.dataset.tab : null);
  const card = el.closest("fieldset") || el;
  if (flash) {
    card.scrollIntoView({ block: "nearest" });
    card.classList.remove("set-flash");
    void card.offsetWidth;
    card.classList.add("set-flash");
  }
}

// linha de estado embaixo de cada seção no menu
function renderSetNavStatus() {
  if (!$("setSt-gmc")) return;
  const put = (id, text, dot) => {
    const el = $("setSt-" + id);
    el.hidden = !text;
    el.lastChild.textContent = text || "";
    el.firstChild.className = "dot" + (dot ? " " + dot : "");
    el.firstChild.hidden = !dot;
  };
  const s = state.status || {}, d = s.device || {};
  put("gmc", s.state === "conectado" ? `conectado · ${d.port}` : s.state === "conectando" ? "conectando…" : "desconectado",
    s.state === "conectado" ? "on" : s.state === "erro" ? "err" : s.state === "conectando" ? "wait" : "off");
  const rt = $("rcSetText") && $("rcSetText").textContent;
  put("rc", rt || "", ($("rcSetDot").className.match(/\b(on|err|wait)\b/) || [])[1] || "off");
  const set = state.settings || {};
  put("alarm", !set.alarmEnabled ? "alarme desligado" : `toca a partir de ${usvTxt(alarmUSv())}`, "");
  put("about", $("appVersion") && $("appVersion").textContent ? `versão ${$("appVersion").textContent.replace(/^v/, "")}` : "", "");
}

// escada dos níveis: a barra em cima acompanha os três números
function renderLadder() {
  const bar = $("lvBar");
  if (!bar) return;
  const a = parseFloat($("lvAttn").value) || attnUSv();
  const al = parseFloat($("numAlarm").value) || alarmUSv();
  const p = parseFloat($("lvDanger").value) || dangerUSv();
  const lo = Math.min(a, al, p) / 10, hi = Math.max(a, al, p) * 1.5;
  const pos = (v) => Math.max(0, Math.min(100, (Math.log10(v / lo) / Math.log10(hi / lo)) * 100));
  const xs = [pos(Math.min(a, al)), pos(Math.max(a, al)), pos(p)];
  // atenção acima do alarme: a faixa amarela some (o alarme já toca antes)
  const w = a <= al ? [xs[0], xs[1] - xs[0], Math.max(0, xs[2] - xs[1]), 100 - xs[2]] : [xs[0], 0, Math.max(0, xs[2] - xs[0]), 100 - xs[2]];
  [...bar.children].forEach((i, k) => { i.style.width = w[k] + "%"; });
  bar.classList.toggle("no-attn", !$("chkAttn").checked);
  bar.classList.toggle("no-alarm", !$("chkAlarm").checked);
  $("lvScale").innerHTML = "";
  for (const v of [lo, hi]) { const sp = document.createElement("span"); sp.textContent = usvTxt(v); $("lvScale").appendChild(sp); }
  $("lvScale").insertBefore(Object.assign(document.createElement("span"), { textContent: "escala logarítmica" }), $("lvScale").lastChild);
}
document.addEventListener("input", (e) => { if (e.target.closest && e.target.closest("#lvCard")) renderLadder(); });
document.addEventListener("change", (e) => { if (e.target.closest && e.target.closest("#lvCard")) renderLadder(); });

// alarmes que tocam nos próprios aparelhos, em µSv/h para comparar
function renderDevAlarms() {
  if (!$("devAlGmc")) return;
  const cpm = devCfgRead && devCfg ? parseFloat($("dcAlarm").value) : NaN;
  $("devAlGmc").textContent = cpm > 0 ? `${fmt(cpm, 0)} CPM ≈ ${usvTxt(cpm * calK())}` : "conecte o GMC-800 para ver";
  const l = ((state.rc || {}).st || {}).limits || {};
  $("devAlRc").textContent = l.device ? `${usvTxt(l.attn || 0)} e ${usvTxt(l.alarm || 0)}` : "conecte o RC102 para ver";
}

// links para fora (Sobre › Código) abrem no navegador do sistema
document.addEventListener("click", (e) => {
  const a = e.target.closest && e.target.closest("a[data-ext]");
  if (!a) return;
  e.preventDefault();
  if (window.runtime && window.runtime.BrowserOpenURL) window.runtime.BrowserOpenURL(a.href); else window.open(a.href, "_blank");
});

document.addEventListener("click", (e) => {
  const b = e.target.closest && e.target.closest("[data-setgo]");
  if (!b) return;
  const [sec, tab] = b.dataset.setgo.split("/");
  showSetSection(sec, tab);
});

document.addEventListener("click", async (e) => {
  if (!e.target.closest("#btnFixAutostart")) return;
  try { await api.FixAutostart(); $("chkAutostart").checked = true; toast("Início com o Windows corrigido"); }
  catch (err) { toast("Não consegui corrigir: " + errMsg(err), true); }
  refreshAutostart();
});

function wireLevels() {
  for (const id of Object.keys(LV_FIELDS)) $(id).addEventListener("input", () => { renderLevelEq(); renderDangerNote(); renderAttnLabel(); renderDeadTime(); });
  $("lvDTOn").addEventListener("change", renderDeadTime);
  $("btnLevelsReset").addEventListener("click", () => { fillLevels(LV_DEFAULT); renderDangerNote(); renderAttnLabel(); renderSetBar(); });
  $("btnSetDiscard").addEventListener("click", () => { state.setSnap = null; openSettings(); });
  $("setSearch").addEventListener("input", setSearch);
  $("setSearch").addEventListener("keydown", (e) => {
    if (e.key === "Enter") { const b = document.querySelector("#setResults button"); if (b) b.click(); }
    if (e.key === "Escape") { $("setSearch").value = ""; setSearch(); }
    if (e.key === "ArrowDown") { const b = document.querySelector("#setResults button"); if (b) { e.preventDefault(); b.focus(); } }
  });
  $("setResults").addEventListener("keydown", (e) => {
    if (e.key !== "ArrowDown" && e.key !== "ArrowUp" && e.key !== "Escape") return;
    e.preventDefault();
    if (e.key === "Escape") { $("setSearch").focus(); return; }
    const bs = [...$("setResults").querySelectorAll("button")], i = bs.indexOf(document.activeElement);
    const n = bs[i + (e.key === "ArrowDown" ? 1 : -1)];
    if (n) n.focus(); else if (e.key === "ArrowUp") $("setSearch").focus();
  });
  document.addEventListener("click", (e) => { if (!e.target.closest(".set-search-wrap")) $("setResults").hidden = true; });
  // o botão "Gravar no GMC-800" da Calibração é o mesmo da aba No aparelho
  $("dcCalSave").addEventListener("click", () => $("dcSaveBtn").click());
  const mirror = () => { $("dcCalSave").disabled = $("dcSaveBtn").disabled; $("dcCalSave").textContent = $("dcSaveBtn").textContent; };
  new MutationObserver(mirror).observe($("dcSaveBtn"), { attributes: true, attributeFilter: ["disabled"], childList: true, characterData: true, subtree: true });
  mirror();
}

function renderAttnLabel() { renderLadder(); } // a escada mostra o valor; o rótulo é só "Ligado"

function clearSnooze() {
  alarmCtl.snoozeUntil = 0;
  clearTimeout(alarmCtl.snoozeTimer);
  alarmCtl.snoozeTimer = null;
}

// ---------- o que fazer (orientações nos avisos) ----------
//
// Princípios de proteção radiológica: tempo, distância e blindagem. Contatos
// da CNEN (gov.br/cnen, "Emergência radiológica"): horário comercial
// (21) 2442-8539; plantão 24 h (21) 98368-0763.

// Textos curtos: dá para ler em segundos, no meio de um alarme.
const PHONES = "CNEN 24 h: (21) 98368-0763 · Bombeiros: 193";

function guidance(kind, usv) {
  if (kind === "perigo" && usv < 10) usv = 10; // perigo sempre usa as orientações fortes
  if (kind === "atencao") {
    return {
      hint: "Acima do normal, sem perigo imediato.",
      steps: [
        "Espere 1–2 min: picos curtos são normais.",
        "Afaste o contador de objetos próximos e veja se cai.",
        "Compare com outro cômodo e com o lado de fora.",
        "Choveu agora há pouco? Sobe um pouco e volta em horas.",
      ],
    };
  }
  if (usv >= 100) {
    return {
      hint: "Nível perigoso. Afaste-se agora e leve as pessoas junto.",
      steps: [
        "Andando? Volte por onde veio.",
        "Algo chegou perto? Afaste-se e ponha paredes no meio.",
        "Não toque em nada. Mantenha todos longe.",
        PHONES,
      ],
    };
  }
  if (usv >= 10) {
    return {
      hint: "Bem acima do normal. Afaste-se agora.",
      steps: [
        "Andando? Volte por onde veio.",
        "Algo chegou perto? Afaste-se e ponha paredes no meio.",
        "Dobro da distância = 1/4 da dose.",
        "Não toque em nada.",
        PHONES,
      ],
    };
  }
  return {
    hint: "Acima do seu limite. Afaste-se e procure a fonte.",
    steps: [
      "Afaste-se: dobro da distância = 1/4 da dose.",
      "Meça de perto e a 2 m para achar a fonte.",
      "Não toque nem guarde o objeto.",
      "Objeto desconhecido? " + PHONES,
    ],
  };
}

function fillGuide(listId, hintId, g) {
  $(hintId).textContent = g.hint;
  const ol = $(listId);
  const key = g.steps.join("|");
  if (ol.dataset.key === key) return;
  ol.dataset.key = key;
  ol.textContent = "";
  for (const t of g.steps) {
    const li = document.createElement("li");
    li.textContent = t;
    ol.appendChild(li);
  }
}

function toggleGuide(btnId, listId) {
  const open = $(listId).hidden;
  $(listId).hidden = !open;
  $(btnId).setAttribute("aria-expanded", String(open));
  $(btnId).textContent = open ? "Esconder" : "O que fazer";
}

// ---------- aviso de atenção (50 CPM) ----------
//
// Mais leve que o alarme: faixa amarela no topo, 3 bipes uma vez por episódio
// e um alerta anotado (o Go anota). Some sozinho quando o valor volta abaixo
// de 45 CPM, ou quando você clica em OK.

const attnCtl = { real: false, demo: false, dismissed: false, kind: null };

function threeBeeps() {
  if ((state.settings.alarmSound || "sirene") === "mudo") return;
  try {
    const ctx = audioCtx();
    for (let k = 0; k < 3; k++) {
      const o = ctx.createOscillator(), g = ctx.createGain();
      o.frequency.value = 1760;
      o.connect(g); g.connect(ctx.destination);
      const t0 = ctx.currentTime + k * 0.3;
      g.gain.setValueAtTime(0.0001, t0);
      g.gain.exponentialRampToValueAtTime(0.3, t0 + 0.01);
      g.gain.exponentialRampToValueAtTime(0.0001, t0 + 0.18);
      o.start(t0); o.stop(t0 + 0.2);
    }
  } catch (e) { /* sem áudio */ }
}

// setAttention recebe o estado de atenção de uma leitura; bipa só quando um
// episódio começa (não quando um alarme acaba e o valor ainda está acima de 50).
function setAttention(which, on) {
  const was = attnCtl[which];
  attnCtl[which] = on;
  if (on && !was) {
    attnCtl.dismissed = false;
    if (!alarmCtl.kind) threeBeeps();
  }
  updateAttention();
}

function updateAttention() {
  const kind = attnCtl.real ? "real" : attnCtl.rc ? "rc" : attnCtl.demo ? "demo" : null;
  attnCtl.kind = kind;
  const show = !!kind && !attnCtl.dismissed && !alarmCtl.kind;
  $("attnBar").hidden = !show;
  document.body.classList.toggle("attention", show || (!!kind && !alarmCtl.kind));
  if (!show) return;
  if (!$("okBar").hidden) hideOkBar();
  $("attnIco").innerHTML = LV_PATH.atencao;
  let dev, usv, lim, fake;
  if (kind === "rc") {
    const rs = state.rc.st || {}, l = rs.limits || {};
    dev = "RC102";
    usv = rs.usvh || 0;
    lim = l.device ? `alarme 1 do aparelho: ${usvTxt(l.attn || 0)}` : `atenção a partir de ${usvTxt(l.attn || 0)}`;
    fake = !!rs.demo;
  } else {
    const r = kind === "real" && state.demo.active ? state.lastReal : state.last;
    dev = "GMC-800";
    usv = r ? r.usv : 0;
    lim = `atenção a partir de ${usvTxt(attnUSv())}`;
    fake = kind === "demo";
  }
  const tag = fake ? "DEMO" : kind === "real" && state.demo.active ? "REAL" : "";
  $("attnBar").classList.toggle("demo", fake);
  $("attnTag").textContent = tag;
  $("attnTag").hidden = !tag;
  $("attnTitle").textContent = `Atenção no ${dev}`;
  $("attnVal").textContent = usvTxt(usv);
  $("attnDetail").textContent = lim;
  bandOther("attn", kind, "atencao", usv);
  fillGuide("attnGuide", "attnHint", guidance("atencao", usv));
}

function silenceAlarm() {
  if (!alarmCtl.kind) return;
  // já silenciado: o mesmo botão liga o som de novo
  if (alarmCtl.sev === "perigo" ? alarmCtl.snoozeUntil > Date.now() : alarmCtl.muted) {
    if (alarmCtl.sev === "perigo") clearSnooze();
    else alarmCtl.muted = false;
    updateAlarm();
    return;
  }
  if (alarmCtl.sev === "perigo") {
    // perigo: só por 5 min; se ainda estiver alto, volta a tocar
    alarmCtl.snoozeUntil = Date.now() + snoozeMs();
    clearTimeout(alarmCtl.snoozeTimer);
    alarmCtl.snoozeTimer = setTimeout(() => { alarmCtl.snoozeUntil = 0; updateAlarm(); }, snoozeMs() + 50);
  } else {
    alarmCtl.muted = true;
  }
  updateAlarm();
}

function wireAlarm() {
  $("btnSilence").addEventListener("click", silenceAlarm);
  $("btnAttnOk").addEventListener("click", () => { attnCtl.dismissed = true; updateAttention(); });
  $("btnAttnGuide").addEventListener("click", () => toggleGuide("btnAttnGuide", "attnGuide"));
  $("btnAlarmGuide").addEventListener("click", () => toggleGuide("btnAlarmGuide", "alarmGuide"));
  $("btnOkClose").addEventListener("click", hideOkBar);
  $("btnOkHist").addEventListener("click", () => { hideOkBar(); showView("alertas"); });
  wireAlertHistory();
  document.addEventListener("keydown", (e) => {
    if (e.key !== "Escape" || !alarmCtl.kind) return;
    if (alarmCtl.sev === "perigo" ? alarmCtl.snoozeUntil <= Date.now() : !alarmCtl.muted) silenceAlarm();
  });
  const test = (sev) => {
    alarmCtl.testSev = sev;
    alarmCtl.testUntil = Date.now() + 5000;
    updateAlarm();
    setTimeout(updateAlarm, 5050);
  };
  $("btnTestAlarm").addEventListener("click", () => test("alarme"));
  $("btnTestDanger").addEventListener("click", () => test("perigo"));
  $("numAlarm").addEventListener("input", () => { renderDangerNote(); renderLevelEq(); });
  wireLevels();
}

// ---------- configurações ----------

async function fillPorts(selectName) {
  const sel = $("selPort");
  let ports = [];
  try { ports = await api.ListPorts() || []; } catch (e) { toast("Não consegui listar as portas: " + e, true); }
  sel.textContent = "";
  const want = selectName || state.settings.port || "";
  if (!ports.length) {
    const o = document.createElement("option");
    o.value = ""; o.textContent = "nenhuma porta encontrada";
    sel.appendChild(o);
  }
  for (const p of ports) {
    const o = document.createElement("option");
    o.value = p.name;
    o.textContent = p.description ? `${p.name} — ${p.description}` : p.name;
    sel.appendChild(o);
  }
  if (want && ports.some((p) => p.name === want)) sel.value = want;
}

function renderAlarmLabel() {
  $("alarmLabel").textContent = "Ligado";
  renderSetNavStatus();
  renderLadder();
}

// ---------- backup ----------

function mergeTextJS(r) {
  const p = [];
  const add = (n, one, many) => { if (n === 1) p.push(`1 ${one}`); else if (n > 1) p.push(`${n} ${many}`); };
  add(r.minutes, "minuto de leitura", "minutos de leitura");
  add(r.rcMinutes, "registro do RC102", "registros do RC102");
  add(r.events, "evento/alerta", "eventos/alertas");
  add(r.periods, "período fora de casa", "períodos fora de casa");
  add(r.measurements, "medição", "medições");
  add(r.alertFiles, "detalhe de alerta", "detalhes de alerta");
  add(r.notes, "anotação", "anotações");
  return p.length ? "+" + p.join(", ") : "nada novo";
}

function wireBackup() {
  $("btnBackup").addEventListener("click", async () => {
    try { const f = await api.ExportBackup(); if (f) toast("Backup salvo: " + f.split(/[\\/]/).pop()); } catch (e) { toast(errMsg(e), true); }
  });
  $("btnImportBackup").addEventListener("click", async () => {
    try { const r = await api.ImportBackup(); if (r && (r.from || r.created)) toast(`Backup de ${r.from || "outro PC"}: ${mergeTextJS(r)}`); } catch (e) { toast(errMsg(e), true); }
  });
}

async function refreshAutostart() {
  let st;
  try { st = await api.GetAutostartStatus(); } catch (e) { return; }
  const box = $("autostartInfo"), btn = $("btnFixAutostart"), txt = $("autostartText");
  if (!st || !st.available || !st.on) { box.hidden = true; return; }
  let msg, fix = "", bad = true;
  if (!st.path) { msg = "Ligado no app, mas não está registrado no Windows."; fix = "Registrar"; }
  else if (!st.exists) { msg = `Registrado para abrir um arquivo que não existe mais: ${st.path}`; fix = "Usar esta versão"; }
  else if (!st.current) { msg = `Registrado para abrir outra cópia: ${st.path}`; fix = "Usar esta versão"; }
  else if (st.disabled) { msg = "O Windows desativou o início do app (Gerenciador de Tarefas → Aplicativos de inicialização)."; fix = "Reativar"; }
  else { msg = `Registrado: ${st.path}. Abre escondido na bandeja, perto do relógio (às vezes dentro da setinha ^).` + (st.started ? " Esta cópia abriu com o Windows." : ""); bad = false; }
  txt.textContent = msg;
  box.className = "as-info small " + (bad ? "as-bad" : "muted");
  btn.hidden = !fix;
  btn.textContent = fix;
  box.hidden = false;
}

async function openSettings(focusId) {
  if (state.view !== "config") { showView("config", true); }
  $("aboutVer").textContent = $("appVersion").textContent || "—";
  buildSetNav();
  if (setDirty().length) { // volta com o que estava mexendo
    if (typeof focusId === "string") showSetFor(focusId); else showSetSection(state.setSec);
    renderStatus();
    return;
  }
  devCfgRead = false;
  rcCfgRead = false;
  refreshReportDir();
  refreshAutostart();
  const s = state.settings;
  $("chkAuto").checked = !!s.autoConnect;
  $("numAlarm").value = usv3(alarmUSv());
  renderLevelEq();
  $("selSound").value = s.alarmSound || "sirene";
  $("chkRaise").checked = s.alarmRaise !== false;
  $("chkAttn").checked = s.attentionEnabled !== false;
  $("numAwayAsk").value = s.awayAskMin || 15;
  $("chkTray").checked = s.trayOnClose !== false;
  $("chkAutostart").checked = !!s.startWithWindows;
  $("chkUpdCheck").checked = !s.noUpdateCheck;
  $("chkCleanOld").checked = !s.keepOldVersions;
  if (api.GetVersionsDir) api.GetVersionsDir().then((d) => { $("updDir").textContent = d; }).catch(() => {});
  $("selLang").value = s.lang || "auto";
  $("chkMeasReport").checked = !s.noMeasureReport;
  fillLevels(lv());
  renderAttnLabel();
  // (o botão da engrenagem chama com o evento do clique: só vale texto)
  if (typeof focusId === "string") showSetFor(focusId); else showSetSection(state.setSec);
  renderDangerNote();
  renderRainSettings();
  renderStatus();
  renderCfgHist();
  renderCalHist();
  refreshCounterSync();
  await fillPorts();
  setSnap();
}

// ---------- mudanças não salvas ----------
// Os campos que só valem com "Salvar" (os outros salvam na hora ou têm botão
// próprio, como "Gravar no contador"). A barra de baixo diz quais mudaram.
const SET_SAVE_IDS = () => ["chkAuto", "numAlarm", "selSound", "chkRaise", "chkAttn", "chkTray", "chkAutostart", "chkUpdCheck", "chkCleanOld", "selLang", "chkMeasReport", "numAwayAsk", "lvDTOn", ...Object.keys(LV_FIELDS)];
// nomes curtos na barra de mudanças
const SET_FIELD_NAMES = { chkAuto: "conectar sozinho", numAlarm: "limite do alarme", selSound: "som do alarme", chkRaise: "trazer a janela para a frente", chkAttn: "aviso de atenção",
  chkTray: "bandeja ao fechar", chkAutostart: "iniciar com o Windows", chkUpdCheck: "procurar versão nova", chkCleanOld: "apagar versões antigas", selLang: "idioma",
  chkMeasReport: "relatório da medição", numAwayAsk: "pergunta de fora de casa", lvAttn: "nível de atenção", lvDanger: "nível de perigo", lvSnooze: "tempo do silenciar",
  lvSatMax: "fim da escala", lvSatDrop: "queda brusca", lvSatHold: "tempo do aviso de queda", lvRcSat: "fim da escala do RC102", lvDTOn: "compensar o tempo morto", lvDTUs: "tempo morto τ" };
const setVal = (el) => (el.type === "checkbox" ? el.checked : el.value);
state.setSnap = null;
function setSnap() {
  state.setSnap = {};
  for (const id of SET_SAVE_IDS()) if ($(id)) state.setSnap[id] = setVal($(id));
  renderSetBar();
}
function setDirty() {
  if (!state.setSnap) return [];
  return SET_SAVE_IDS().filter((id) => $(id) && id in state.setSnap && setVal($(id)) !== state.setSnap[id]);
}
function setFieldName(id) {
  if (SET_FIELD_NAMES[id]) return SET_FIELD_NAMES[id];
  const el = $(id);
  const lab = document.querySelector(`label[for="${id}"]`) || el.closest("label");
  let t = lab ? (lab.querySelector("span") || lab).textContent : "";
  t = t.replace(/\s+/g, " ").trim().replace(/\s*\(.*\)$/, "");
  const sec = el.closest("fieldset");
  const leg = sec && sec.querySelector(":scope > legend");
  return (t || (leg ? leg.textContent.trim() : id)).replace(/[:.]$/, "");
}
function renderSetBar() {
  const d = setDirty();
  $("setBar").hidden = !d.length;
  document.documentElement.classList.toggle("set-dirty", d.length > 0);
  $("btnSettings").classList.toggle("dirty", d.length > 0);
  if (!d.length) return;
  const names = [...new Set(d.map(setFieldName))];
  const bar = $("setBarText");
  bar.textContent = d.length === 1 ? "1 mudança não salva: " : `${d.length} mudanças não salvas: `;
  names.slice(0, 4).forEach((n, i) => { if (i) bar.append(", "); const b = document.createElement("b"); b.textContent = n; bar.appendChild(b); });
  if (names.length > 4) bar.append(` e mais ${names.length - 4}`);
}
document.addEventListener("input", (e) => { if (e.target.closest && e.target.closest("#settings")) renderSetBar(); });
document.addEventListener("change", (e) => { if (e.target.closest && e.target.closest("#settings")) renderSetBar(); });

// ---------- busca nas configurações ----------
const setNorm = (t) => t.normalize("NFD").replace(/[\u0300-\u036f]/g, "").toLowerCase();
function setSearch() {
  const q = setNorm($("setSearch").value.trim()).replace(/[^a-z0-9µ]+/g, " ").trim();
  const box = $("setResults");
  box.innerHTML = "";
  if (!q) { box.hidden = true; $("setSearchInfo").textContent = ""; return; }
  buildSetNav();
  const hits = [];
  // cada cartão é um resultado; o caminho diz a seção e a aba
  for (const f of document.querySelectorAll("#settings .set-sec fieldset")) {
    if (f.closest("fieldset") !== f && f.parentElement.closest("fieldset")) continue;
    if (f.hidden) continue;
    const leg = f.querySelector(":scope > legend");
    if (!leg) continue;
    const sec = f.closest(".set-sec"), tab = f.closest(".set-tab"), L = setLay(sec.dataset.sec);
    const title = setNorm(leg.textContent);
    const hay = setNorm(L.name + " " + f.textContent + " " + [...f.querySelectorAll("option")].map((o) => o.textContent).join(" "));
    if (!(" " + hay.replace(/[^a-z0-9µ]+/g, " ")).includes(" " + q)) continue; // começo de palavra
    hits.push({ f, L, tab: tab && L.tabs.find((t) => t[0] === tab.dataset.tab), score: title.includes(q) ? 0 : 1 });
  }
  hits.sort((x, y) => x.score - y.score);
  for (const h of hits.slice(0, 8)) {
    const b = document.createElement("button");
    b.type = "button";
    b.setAttribute("role", "option");
    const t = document.createElement("span");
    t.textContent = h.f.querySelector(":scope > legend").textContent.trim();
    const p = document.createElement("span");
    p.className = "set-res-path";
    p.append(Object.assign(document.createElement("span"), { textContent: h.L.name }));
    if (h.tab) { p.append(" › "); p.append(Object.assign(document.createElement("span"), { textContent: h.tab[1] })); }
    b.append(t, p);
    b.addEventListener("click", () => {
      if (!h.f.id) h.f.id = "set-card-" + Math.random().toString(36).slice(2, 8);
      box.hidden = true;
      showSetFor(h.f.id, true);
    });
    box.appendChild(b);
  }
  box.hidden = !hits.length;
  $("setSearchInfo").textContent = hits.length ? "" : "nada encontrado";
}

// versões da calibração do GMC (2.2)
const calWhen = (t) => { const d = new Date(t); return d.toLocaleDateString(LOCALE, { day: "2-digit", month: "2-digit", year: "numeric" }) + " " + d.toLocaleTimeString(LOCALE, { hour: "2-digit", minute: "2-digit" }); };
const calSrcTxt = (v) => v.origem === "intercalibração" ? "intercalibração com o RC102" : v.fator > 0 ? "informado à mão" : v.origem === "contador" ? "curva do contador" : v.origem === "padrão" ? "padrão de fábrica" : "curva do contador (início)";
async function renderCalHist() {
  if (!api.GetCalHistory) return;
  let h;
  try { h = await api.GetCalHistory(); } catch (e) { return; }
  state.calHist = h;
  const vs = h.versions || [];
  const cur = vs.find((v) => v.atual) || vs[0];
  if (cur) {
    $("calNow").textContent = `${fmt(cur.k, 5)} µSv/h por CPM · ${calSrcTxt(cur)}` + (h.retro ? " · histórico recalculado" : "");
    $("calK").value = cur.k ? cur.k.toFixed(5) : "";
    $("calKReset").hidden = !(cur.fator > 0);
  }
  $("calDev").textContent = h.devK ? `${fmt(h.devK, 5)} µSv/h por CPM` : "—";
  $("calVerBody").innerHTML = vs.map((v) => {
    const isRetro = h.retro === v.id;
    let act = "";
    if (isRetro) act = `<button type="button" class="btn small" data-retro="0">Voltar ao original</button>`;
    else if (v.desde > 0) act = `<button type="button" class="btn small" data-cmp="${v.id}">Recalcular o passado…</button>`;
    const note = v.nota ? `<br><span class="muted">${escHtml(v.nota)}</span>` : "";
    return `<tr class="${v.atual ? "cur" : ""}${isRetro ? " retro" : ""}"><td class="when">${v.desde ? calWhen(v.desde) : "o começo"}</td><td class="when">${v.ate ? calWhen(v.ate) : "em uso"}</td>` +
      `<td class="num mono">${fmt(v.k, 5)}</td><td>${escHtml(calSrcTxt(v))}${isRetro ? `<br><span class="muted">vale também antes desta data</span>` : ""}${note}</td><td class="num">${fmt(v.minutos, 0)}</td><td>${act}</td></tr>`;
  }).join("");
  $("calCmp").hidden = true;
  calSuggest();
  renderIntercal();
  renderConcord();
}

async function calSuggest() {
  const el = $("calKHint");
  el.hidden = true;
  if (!api.GetRcCompare || !hasRcData()) return;
  let c;
  try { c = await api.GetRcCompare(6 * 60); } catch (e) { return; }
  if (!c || c.minutes < 60 || !(c.suggestK > 0)) return;
  el.innerHTML = `Lado a lado com o RC102 (${fmt(c.minutes, 0)} min nas últimas 6 h): <strong class="mono">${fmt(c.suggestK, 5)}</strong> µSv/h por CPM faria o GMC bater com ele na média. <button type="button" class="btn small" id="calUseSug">Preencher</button>`;
  el.hidden = false;
  $("calUseSug").addEventListener("click", () => { $("calK").value = c.suggestK.toFixed(5); $("calK").focus(); });
}

async function calCompare(id) {
  const box = $("calCmp");
  let c;
  try { c = await call(api.CompareCal, id); } catch (e) { return; }
  const v = (state.calHist.versions || []).find((x) => x.id === id) || {};
  if (!c.minutes) {
    box.innerHTML = `Não há minutos gravados antes de ${calWhen(v.desde)} para recalcular.`;
    box.hidden = false;
    return;
  }
  const pct = c.changePct;
  box.innerHTML = `<strong>Recalcular com a versão de ${calWhen(v.desde)} (${fmt(v.k, 5)} µSv/h por CPM)</strong><br>` +
    `<span>Período: ${calWhen(c.from)} a ${calWhen(c.to)} (${fmt(c.minutes, 0)} minutos gravados)</span><br>` +
    `<span>Dose: ${fmt(c.doseOrig, 2)} → ${fmt(c.doseNew, 2)} µSv (${pct >= 0 ? "+" : "−"}${fmt(Math.abs(pct), 0)}%)</span><br>` +
    `<span>Média: ${fmt(c.avgOrig, 3)} → ${fmt(c.avgNew, 3)} µSv/h · máximo: ${fmt(c.maxOrig, 3)} → ${fmt(c.maxNew, 3)} µSv/h</span><br>` +
    `<span class="muted">Nenhuma leitura gravada muda; dá para voltar ao original depois.</span>` +
    `<div class="row"><button type="button" class="btn accent small" data-retro="${id}">Recalcular o histórico</button><button type="button" class="btn small" data-cmpclose="1">Cancelar</button></div>`;
  box.hidden = false;
}

async function calHistChanged() {
  await renderCalHist();
  renderCfgHist();
  refreshAll();
}

function initCalHist() {
  $("calKApply").addEventListener("click", async () => {
    const k = parseFloat(String($("calK").value).replace(",", "."));
    if (!(k > 0)) { toast("Informe o µSv/h por CPM", true); return; }
    try { await call(api.SetGMCFactor, k, ""); } catch (e) { return; }
    toast("Calibração nova a partir de agora");
    calHistChanged();
  });
  $("calKReset").addEventListener("click", async () => {
    try { await call(api.SetGMCFactor, 0, ""); } catch (e) { return; }
    toast("Voltou à curva do contador a partir de agora");
    calHistChanged();
  });
  const onClick = async (e) => {
    const b = e.target.closest("button");
    if (!b) return;
    if (b.dataset.cmp) calCompare(Number(b.dataset.cmp));
    else if (b.dataset.cmpclose) $("calCmp").hidden = true;
    else if (b.dataset.retro != null) {
      const id = Number(b.dataset.retro);
      try { await call(api.SetCalRetro, id); } catch (e2) { return; }
      toast(id ? "Histórico recalculado (as leituras gravadas não mudaram)" : "Histórico original de volta");
      calHistChanged();
    }
  };
  $("calVerBody").addEventListener("click", onClick);
  $("calCmp").addEventListener("click", onClick);
  $("btnGmcDiag").addEventListener("click", async () => {
    const b = $("btnGmcDiag"), out = $("gmcDiagOut");
    if (!api.DiagnoseGMCDose) return;
    b.disabled = true;
    out.textContent = "testando… (cerca de 2 min)";
    try {
      const r = await call(api.DiagnoseGMCDose);
      out.textContent = `${r.verdict}. Relatório: ${r.path}`;
    } catch (e) { out.textContent = ""; }
    b.disabled = false;
  });
  on("gmcdiag", (st) => {
    if (!st) return;
    $("gmcDiagOut").textContent = st === "esperando" ? "esperando 90 s para ler de novo…" : `testando… (${st})`;
  });
  on("calibration", () => { if (state.view === "config") renderCalHist(); });
}

// histórico de configurações (2.1)
async function renderCfgHist() {
  if (!api.GetConfigHistory) return;
  let h = [];
  try { h = (await api.GetConfigHistory()) || []; } catch (e) { h = []; }
  const when = (t) => { const d = new Date(t); return d.toLocaleDateString(LOCALE, { day: "2-digit", month: "2-digit", year: "numeric" }) + " " + d.toLocaleTimeString(LOCALE, { hour: "2-digit", minute: "2-digit" }); };
  $("cfgHistBody").innerHTML = h.map((c) => `<tr><td class="when">${when(c.t)}</td><td>${escHtml(c.campo)}</td><td class="from">${escHtml(c.de || "—")}</td><td>${escHtml(c.para)}</td><td class="from">${escHtml(c.origem)}</td></tr>`).join("");
  $("cfgHistEmpty").hidden = h.length > 0;
}

// idioma: o i18n.js lê do localStorage antes de tudo; o Go guarda a escolha
// (para os relatórios e para valer em outra instalação). Se mudar, recarrega.
function syncLang(lang) {
  const want = lang || "auto";
  if (want === LANG_PREF) return;
  try {
    localStorage.setItem("lang", want);
    if (localStorage.getItem("lang") === want) location.reload();
  } catch (e) { /* sem localStorage: fica no idioma atual */ }
}

async function saveSettings(extra) {
  const s = Object.assign({}, state.settings, extra || {});
  const rainWas = state.settings.rainEnabled + "|" + state.settings.rainLat + "|" + state.settings.rainLon;
  state.settings = await call(api.SaveSettings, s);
  syncLang(state.settings.lang);
  if (rainWas !== state.settings.rainEnabled + "|" + state.settings.rainLat + "|" + state.settings.rainLon) refreshRainImpact(true);
  $("chkAlarm").checked = !!state.settings.alarmEnabled;
  renderAlarmLabel();
}

// ---------- ajustes do contador ----------

let devCfg = null;
const errMsg = (e) => (e && e.message ? e.message : String(e));
const qsa = (sel, root) => Array.from((root || document).querySelectorAll(sel));

const saveHints = {
  0: "Não recomendado: o contador não guarda nada sozinho; só o app grava, e só enquanto estiver conectado.",
  1: "Bom só para investigar algo por pouco tempo. No fundo natural cada segundo tem 0 a 2 contagens, então o dado fica muito ruidoso, e a memória enche rápido.",
  2: "É a mesma resolução que o app grava, mostra picos de poucos minutos e cabe anos na memória.",
  3: "Cabe décadas, mas um pico de poucos minutos some dentro da média da hora.",
  4: "Só grava quando passar do limiar definido no aparelho; fora disso não fica nada.",
  5: "Só grava quando passar do limiar definido no aparelho; fora disso não fica nada.",
};
const RECOMMENDED_SAVE = 2;
// Cada leitura ocupa ~1 byte na memória (com radiação de fundo).
const secondsPerByte = { 1: 1, 2: 60, 3: 3600 };
let memInfo = null;

function fmtDuration(sec) {
  const day = 86400;
  if (sec < 2 * day) return `${fmt(sec / 3600, 0)} horas`;
  if (sec < 60 * day) return `${fmt(sec / day, 0)} dias`;
  if (sec < 2 * 365 * day) return `${fmt(sec / (30 * day), 0)} meses`;
  if (sec < 50 * 365 * day) return `${fmt(sec / (365 * day), 0)} anos`;
  return "décadas";
}

function renderSaveAdvice() {
  const mode = parseInt($("dcSave").value, 10);
  $("dcSaveHint").textContent = saveHints[mode] || "";
  $("dcRec").hidden = mode === RECOMMENDED_SAVE || $("dcSave").matches(":disabled") || isNaN(mode);
  const m = memInfo;
  const spb = secondsPerByte[mode];
  let txt = "";
  if (m && m.known) {
    const pct = (m.usedBytes / m.totalBytes) * 100;
    txt = `Memória: ${fmt(m.usedBytes / 1024, 0)} KB de ${fmt(m.totalBytes / 1048576, 0)} MB usados (${fmt(pct, pct < 10 ? 1 : 0)}%, em ${ddmm(m.at)}).`;
    if (spb) {
      const d = fmtDuration((m.totalBytes - m.usedBytes) * spb);
      txt += d === "décadas" ? " Nesse modo, o espaço livre dura décadas." : ` Nesse modo, o espaço livre dura cerca de ${d}.`;
    }
  } else if (spb) {
    const d = fmtDuration(2097152 * spb);
    txt = (d === "décadas" ? "Nesse modo a memória vazia dura décadas." : `Nesse modo a memória vazia dura cerca de ${d}.`) +
      " Baixe o histórico uma vez para o app saber quanto já está ocupado.";
  }
  $("dcMem").textContent = txt;
}

function setDevCfgEnabled(on) {
  qsa("#devcfg fieldset.dc-edit, #dcCalCard").forEach((f) => { f.disabled = !on; });
  $("dcSaveBtn").disabled = !on;
  if (on && devCfg) {
    const locked = new Set(devCfg.locked || []);
    qsa("#devcfg [data-key]").forEach((el) => { el.disabled = locked.has(el.dataset.key); });
    qsa("#calRows input, #calSens, #calApply").forEach((el) => { el.disabled = !devCfg.calibrationEditable; });
  }
  renderSaveAdvice();
}

function renderOutputs() {
  qsa("#devcfg [data-out]").forEach((o) => {
    const el = document.querySelector(`#devcfg [data-key="${o.dataset.out}"]`);
    if (el) o.textContent = el.value;
  });
  renderSaveAdvice();
}

function calRow(i, p) {
  const tr = document.createElement("tr");
  tr.innerHTML = `<td>${i + 1}</td>
    <td><input type="number" class="cal-cpm" min="1" step="1" aria-label="CPM do ponto ${i + 1}"></td>
    <td><input type="number" class="cal-usv" min="0.001" step="any" aria-label="µSv/h do ponto ${i + 1}"></td>
    <td class="num cal-s"></td>`;
  tr.querySelector(".cal-cpm").value = p.cpm;
  tr.querySelector(".cal-usv").value = p.usv;
  return tr;
}

function updateCalSens() {
  qsa("#calRows tr").forEach((tr) => {
    const c = parseFloat(tr.querySelector(".cal-cpm").value);
    const u = parseFloat(tr.querySelector(".cal-usv").value);
    const cell = tr.querySelector(".cal-s");
    const sens = c / u;
    cell.textContent = isFinite(sens) && u > 0 ? fmt(sens, 1) : "—";
    cell.classList.toggle("bad", !(sens >= 20 && sens <= 1000));
  });
}

// tema da tela: o firmware 1.23 tem 5; os antigos só "modo escuro"
function fwVersion() {
  const m = /Re(\d+)\.(\d+)/.exec(((state.status || {}).device || {}).version || "");
  return m ? +m[1] + +m[2] / 100 : 0;
}
function fillThemeOptions() {
  const names = fwVersion() >= 1.23 ? ["Light", "BRY", "BGY", "BBY", "Dark"] : ["Escuro", "Claro"];
  $("dcTheme").innerHTML = names.map((n, i) => `<option value="${i}">${n}</option>`).join("");
}

function fillDevCfg(c) {
  fillThemeOptions();
  const v = c.values || {};
  qsa("#devcfg [data-key]").forEach((el) => {
    const val = v[el.dataset.key];
    if (el.type === "checkbox") el.checked = val === 1;
    else el.value = String(val);
    const lab = el.closest("label") || document.querySelector(`label[for="${el.id}"]`);
    if (lab) {
      lab.querySelectorAll(".locked-tag").forEach((t) => t.remove());
      if ((c.locked || []).includes(el.dataset.key)) {
        const t = document.createElement("span");
        t.className = "locked-tag";
        t.textContent = `(travado: valor ${val} não reconhecido)`;
        (lab.querySelector("span") || lab).appendChild(t);
      }
    }
  });
  renderOutputs();

  const tb = $("calRows");
  tb.textContent = "";
  ((c.calibration && c.calibration.points) || []).forEach((p, i) => tb.appendChild(calRow(i, p)));
  updateCalSens();
  const p0 = (c.calibration.points || [])[0];
  $("calSens").value = p0 ? Math.round((p0.cpm / p0.usv) * 10) / 10 : "";
  $("dcTube").textContent = c.tubeVoltage;

  if (c.editable) {
    $("dcIntro").textContent = "Valores lidos do contador agora. Confira se batem com o menu do aparelho antes de mudar alguma coisa.";
  } else {
    $("dcIntro").textContent = "Valores lidos do contador agora.";
  }
  const probs = c.problems || [];
  $("dcProblems").hidden = probs.length === 0;
  $("dcProblems").textContent = c.editable
    ? "Alguns campos ficaram travados porque o valor lido não é o esperado: " + probs.join("; ") + "."
    : "Por segurança a edição ficou bloqueada: a configuração não bate com o layout que o app conhece (" + probs.join("; ") + "). Pode ser outro firmware.";
  setDevCfgEnabled(!!c.editable);
  renderCounterCal();
}

// lê os ajustes do aparelho quando a seção Contador aparece (uma vez por
// abertura das Configurações; "Gravar" e "Restaurar" leem de novo)
let devCfgRead = false;
async function openDevCfg() {
  if (devCfgRead) return;
  devCfgRead = true;
  setDevCfgEnabled(false);
  $("dcProblems").hidden = true;
  $("dcDiffOut").textContent = "";
  if (!state.status || state.status.state !== "conectado") {
    $("dcIntro").textContent = "Conecte o contador para ver e mudar os ajustes do aparelho.";
    devCfgRead = false;
    return;
  }
  $("dcIntro").textContent = "Lendo a configuração do contador…";
  try {
    try { memInfo = await api.GetMemoryInfo(); } catch (e) { memInfo = null; }
    devCfg = await api.GetDeviceConfig();
    fillDevCfg(devCfg);
  } catch (e) {
    $("dcIntro").textContent = "Não consegui ler a configuração: " + errMsg(e);
  }
}

function readDevCfgForm() {
  const values = Object.assign({}, devCfg.values);
  qsa("#devcfg [data-key]").forEach((el) => {
    values[el.dataset.key] = el.type === "checkbox" ? (el.checked ? 1 : 0) : parseInt(el.value, 10);
  });
  const points = qsa("#calRows tr").map((tr) => ({
    cpm: Math.round(parseFloat(tr.querySelector(".cal-cpm").value)),
    usv: parseFloat(tr.querySelector(".cal-usv").value),
  }));
  return Object.assign({}, devCfg, { values, calibration: Object.assign({}, devCfg.calibration, { points }) });
}

// mudou algum ajuste do aparelho sem gravar no contador?
function devCfgDirty() {
  if (!devCfg || !devCfg.editable || $("dcSaveBtn").disabled) return false;
  try {
    const next = readDevCfgForm();
    const locked = new Set(devCfg.locked || []);
    return Object.keys(next.values).some((k) => !locked.has(k) && !Number.isNaN(next.values[k]) && next.values[k] !== devCfg.values[k]) || calChanged(next);
  } catch (e) { return false; }
}

function calChanged(next) {
  const a = (devCfg.calibration.points || []), b = next.calibration.points;
  return a.length !== b.length || a.some((p, i) => p.cpm !== b[i].cpm || Math.abs(p.usv - b[i].usv) > 1e-6 * p.usv);
}

function wireDevCfg() {
  qsa("#devcfg [data-key]").forEach((el) => el.addEventListener("input", renderOutputs));
  $("calRows").addEventListener("input", updateCalSens);
  $("dcUseRec").addEventListener("click", () => { $("dcSave").value = String(RECOMMENDED_SAVE); renderSaveAdvice(); });
  $("calApply").addEventListener("click", () => {
    const s = parseFloat($("calSens").value);
    if (!(s >= 20 && s <= 1000)) { toast("Use uma sensibilidade entre 20 e 1000 CPM por µSv/h.", true); return; }
    qsa("#calRows tr").forEach((tr) => {
      const u = parseFloat(tr.querySelector(".cal-usv").value);
      tr.querySelector(".cal-cpm").value = Math.round(s * u);
    });
    updateCalSens();
  });

  $("dcSaveBtn").addEventListener("click", async () => {
    const next = readDevCfgForm();
    const a = next.values.alarmCPM;
    if (!(a >= 1 && a <= 999999)) { toast("O limite do alarme precisa ficar entre 1 e 999999 CPM.", true); return; }
    const text = ["O app vai apagar e regravar a configuração do contador (leva alguns segundos)."];
    if (calChanged(next)) text.push("A calibração vai mudar: isso altera o µSv/h mostrado pelo contador e pelo app.");
    if (!(await askConfirm({ title: "Gravar no GMC-800?", text, note: "Não desconecte o cabo nesse tempo.", ok: "Gravar" }))) return;
    setDevCfgEnabled(false);
    $("dcSaveBtn").textContent = "Gravando…";
    try {
      devCfg = await api.SetDeviceConfig(next);
      fillDevCfg(devCfg);
      toast("Configuração gravada no contador");
      state.status = await api.GetStatus();
      renderStatus();
    } catch (e) {
      toast(errMsg(e), true);
      setDevCfgEnabled(!!(devCfg && devCfg.editable));
    } finally {
      $("dcSaveBtn").textContent = "Gravar no contador";
    }
  });

  $("dcRestore").addEventListener("click", async () => {
    if (!(await askConfirm({ title: "Restaurar uma cópia no GMC-800?", text: "Escolha uma cópia salva para gravar de volta no contador. A configuração atual também é salva antes.", ok: "Escolher a cópia" }))) return;
    setDevCfgEnabled(false);
    try {
      const name = await api.RestoreDeviceConfig();
      if (name) {
        toast("Configuração restaurada de " + name);
        devCfg = await api.GetDeviceConfig();
      }
    } catch (e) {
      toast(errMsg(e), true);
    }
    if (devCfg) fillDevCfg(devCfg);
  });
  $("dcBackups").addEventListener("click", () => call(api.OpenConfigBackups));

  $("dcSnap").addEventListener("click", async () => {
    try {
      await api.SnapshotDeviceConfig();
      $("dcDiff").disabled = false;
      $("dcDiffOut").innerHTML = '<li class="muted">Foto tirada. Agora mude um ajuste no menu do contador e clique em Comparar.</li>';
    } catch (e) { toast(errMsg(e), true); }
  });
  $("dcDiff").addEventListener("click", async () => {
    try {
      const diff = await api.DiffDeviceConfig();
      const ul = $("dcDiffOut");
      ul.textContent = "";
      if (!diff.length) { ul.innerHTML = '<li class="muted">Nenhum byte mudou desde a foto.</li>'; return; }
      for (const d of diff) {
        const li = document.createElement("li");
        li.textContent = `byte ${d.index}: ${d.before} → ${d.after}  ${d.field ? "(" + d.field + ")" : "(campo desconhecido)"}`;
        ul.appendChild(li);
      }
    } catch (e) { toast(errMsg(e), true); }
  });
}

// ---------- ligações da interface ----------

function wire() {
  document.querySelectorAll("#unitSeg button").forEach((b) => b.addEventListener("click", () => {
    state.unit = b.dataset.unit; localSet("unit", state.unit); renderReading();
  }));
  document.querySelectorAll("#chartUnitSeg button").forEach((b) => b.addEventListener("click", () => {
    state.chartUnit = b.dataset.cunit; localSet("chartUnit", state.chartUnit); refreshChart();
  }));
  document.querySelectorAll("#rangeSeg button").forEach((b) => b.addEventListener("click", () => {
    state.range = b.dataset.range; localSet("range", state.range); state.zoom = null; state.zoomStack = []; unlinkZoom(); refreshChart();
    state.dosePeriod = state.range; state.doseZoom = null; state.doseZoomStack = []; refreshDose();
  }));

  $("statusPill").addEventListener("click", () => openSettings("selPort"));
  $("btnConnectHint").addEventListener("click", () => openSettings("selPort")); // direto na seção Conexão

  $("btnRefresh").addEventListener("click", () => fillPorts($("selPort").value));
  $("btnDetect").addEventListener("click", async () => {
    $("connMsg").textContent = "procurando…";
    try {
      const p = await api.DetectPort();
      await fillPorts(p);
      $("connMsg").textContent = "encontrei em " + p;
    } catch (e) {
      $("connMsg").textContent = String(e && e.message ? e.message : e);
    }
  });
  $("btnConnect").addEventListener("click", async () => {
    if (state.status.state === "conectado") { await api.Disconnect(); return; }
    const port = $("selPort").value;
    if (!port) { $("connMsg").textContent = "escolha uma porta"; return; }
    $("btnConnect").disabled = true;
    try {
      await api.Connect(port);
      state.settings.port = port;
      toast("Conectado em " + port);
    } catch (e) {
      $("connMsg").textContent = String(e && e.message ? e.message : e);
    } finally {
      $("btnConnect").disabled = false;
    }
  });
  $("btnSave").addEventListener("click", async () => {
    if (devCfgDirty()) toast('Os ajustes do aparelho não foram para o contador: use "Gravar no contador" na seção Contador.', true);
    if (rcCfgDirty()) toast('Os ajustes do RC102 não foram para o aparelho: use "Gravar no RC102" na seção RC102.', true);
    const alarm = parseFloat($("numAlarm").value);
    await saveSettings({
      autoConnect: $("chkAuto").checked,
      alarmUSv: alarm > 0 ? alarm : usv3(100 * calK()),
      alarmSound: $("selSound").value,
      alarmRaise: $("chkRaise").checked,
      attentionEnabled: $("chkAttn").checked,
      trayOnClose: $("chkTray").checked,
      startWithWindows: $("chkAutostart").checked,
      noUpdateCheck: !$("chkUpdCheck").checked,
      keepOldVersions: !$("chkCleanOld").checked,
      lang: $("selLang").value,
      noMeasureReport: !$("chkMeasReport").checked,
      awayAskMin: Math.min(240, Math.max(5, parseInt($("numAwayAsk").value, 10) || 15)),
      levels: readLevels(),
    });
    setSnap();
    toast("Configurações salvas");
  });
  $("btnOpenData").addEventListener("click", () => call(api.OpenDataFolder));
  initCalHist();
  initIntercal();
  initCounterCal();
  wireRcCfg();
  $("btnOpenLog").addEventListener("click", () => call(api.OpenLogFile));

  $("chkAlarm").addEventListener("change", (e) => saveSettings({ alarmEnabled: e.target.checked }));
;

  $("clockChip").addEventListener("click", async () => {
    await call(api.SyncClock);
    toast("Relógio do contador acertado");
  });
  $("btnExport").addEventListener("click", async () => {
    const path = await call(api.ExportCSV);
    if (path) toast("Leituras exportadas para " + path);
  });
  $("btnHistory").addEventListener("click", async () => {
    $("progressFill").style.width = "0%";
    $("progressText").textContent = "Lendo a memória do contador…";
    try {
      const r = await api.DownloadHistory();
      toast(`Histórico: ${r.records} registros, ${r.added} minutos novos`);
      if (r.newRanges && r.newRanges.length) askNewRanges(r.newRanges);
    } catch (e) {
      toast("Histórico: " + (e && e.message ? e.message : e), true);
    }
    refreshAll();
  });
  $("btnCancel").addEventListener("click", () => api.CancelDownload());

  setupChartHover();
  wireDevCfg();
  wireDose();
  wireDemo();
  wireAlarm();
  wireHeat();
  wireUpdate();
  wireSync();
  wireRc();
  wireMeasureSaved();
  wireRain();
  wireMeasure();
  setupMeasureHover();
  wirePeriods();
  wireWhere();
}

// ---------- eventos vindos do Go ----------

function listen() {
  on("reading", (r) => {
    state.last = r;
    if (!r.demo) { state.gLastSeen = r; state.gOffSince = 0; } // a última leitura de verdade, para quando desconectar
    state.alarm = r.alarm;
    if (r.demo) { alarmCtl.demo = !!r.alarm; alarmCtl.demoDanger = !!r.danger; }
    else { alarmCtl.real = !!r.alarm; alarmCtl.realDanger = !!r.danger; state.lastReal = r; }
    renderReading();
    measureLogReading(r);
    updateAlarm();
    setAttention(r.demo ? "demo" : "real", !!r.attention);
    tickActive();
  });
  on("alerts", (list) => { state.activeAlerts = list || []; renderActive(); });
  on("rain-status", (st) => { state.rainStatus = st || {}; renderRainStatus(); });
  on("notes", () => { refreshChart(); if (state.view === "resumo") refreshSummary(); });
  on("radar", () => { if (state.settings.rainEnabled) { refreshChart(); refreshRainImpact(); } });
  on("measurements", (list) => { state.measurements = list || []; renderMeasureHistory(); drawMeasureChart(); refreshChart(); });
  on("periods", (list) => { state.periods = normPeriods(list); renderPeriods(); refreshAll(); });
  on("rcaway", onRcAway);
  on("whereabouts", (w) => { state.where = w; renderWhere(); refreshAgDose(true); });
  on("status", (s) => {
    const was = state.status.state;
    state.status = s;
    if (was === "conectado" && s.state !== "conectado") state.gOffSince = Date.now();
    if (s.state !== "conectado") { state.last = null; alarmCtl.real = false; alarmCtl.realDanger = false; attnCtl.real = false; updateAlarm(); updateAttention(); }
    renderStatus();
    if (was !== s.state) refreshStats();
  });
  on("event", (e) => { refreshEvents(); if (e && e.kind === "alerta") refreshHeat(); });
  on("demo", (d) => {
    const changed = d.active !== state.demo.active;
    state.demo = d;
    if (changed) { state.last = null; state.alarm = false; alarmCtl.demo = false; alarmCtl.demoDanger = false; attnCtl.demo = false; refreshAll(); updateAlarm(); updateAttention(); }
    renderDemo();
  });
  on("realReading", (r) => {
    // leitura real enquanto o demo está ligado (o alarme real continua valendo)
    const line = $("demoRealLine");
    line.textContent = `Leitura real: ${usvTxt(r.usv)}` + (r.danger ? " · PERIGO REAL" : r.alarm ? " · ALARME REAL" : "");
    line.classList.toggle("alarm", !!(r.alarm || r.danger));
    state.lastReal = r;
    state.realAlarm = !!(r.alarm || r.danger);
    alarmCtl.real = !!r.alarm;
    alarmCtl.realDanger = !!r.danger;
    updateAlarm();
    setAttention("real", !!r.attention);
  });
  on("stored", () => refreshAll());
  on("countersync", (cs) => refreshCounterSync(cs));
  $("btnSyncDl").addEventListener("click", () => $("btnHistory").click());
  on("measure", (st) => renderMeasure(st));
  on("history-progress", (p) => {
    // A memória para de ser lida na primeira página vazia, então o total real
    // não é conhecido: mostra quanto já veio.
    const kb = p.bytes / 1024;
    $("progressText").textContent = `Lendo a memória do contador… ${fmt(kb, 0)} KB`;
    $("progressFill").style.width = Math.min(100, (p.bytes / p.max) * 100) + "%";
  });
}

async function refreshAll() {
  await Promise.allSettled([refreshStats(), refreshChart(), refreshDose(), refreshHeat(), refreshPatterns(), refreshBgTrend(), refreshDaily(), refreshEvents(), refreshRainImpact(), refreshCounterSync()]);
}

// ---------- chuva × radiação ----------

let riAt = 0;
async function refreshRainImpact(force) {
  if (state.view !== "chuva") return; // só calcula com a aba aberta
  if (!state.settings.rainEnabled) { renderRainOff(); return; }
  if (!force && Date.now() - riAt < 9 * 60e3 && state.ri) return;
  riAt = Date.now();
  let r;
  const dev = riDev();
  document.querySelectorAll("#riDevSeg button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.ridev === dev)));
  try { r = api.GetRainImpactDev ? await api.GetRainImpactDev(dev) : await api.GetRainImpact(); } catch (e) { return; }
  if (dev !== riDev()) return; // trocou no meio
  state.ri = r;
  renderRainImpact(r);
}

// aparelho da conta: o escolhido antes ou, da primeira vez, o Radiacode se ele
// já tiver gravado algo (conta mais estável: muito mais contagens por minuto)
function riDev() {
  const v = localGet("riDev");
  if (v === "gmc" || v === "radiacode") return v;
  return state.rc.st && state.rc.st.lastT ? "radiacode" : "gmc";
}

const sgn = (v, d) => (v >= 0 ? "+" : "−") + fmt(Math.abs(v), d);
const durMin = (m) => (m < 60 ? `${m} min` : `${fmt(m / 60, m % 60 ? 1 : 0)} h`);

function renderRainOff() {
  state.ri = null;
  $("riSub").textContent = "chuva desligada";
  $("riChartBox").hidden = true;
  $("riList").textContent = "";
  $("riTiles").innerHTML = '<p class="small muted">Para medir o efeito da chuva no CPM, ligue "Mostrar chuva no gráfico" e escolha a cidade em Configurações → Janela e tela → Chuva no gráfico.</p>';
}

function renderRainNow(n) {
  const box = $("riNow");
  box.hidden = !n;
  if (!n) return;
  const since = n.raining ? `Chovendo desde ${hhmm(n.start)}` : `Choveu das ${hhmm(n.start)} às ${hhmm(n.end)}`;
  const what = `${rainWord(n.maxMMh).split(",")[0]}, ${fmt(n.mm, 1)} mm até agora · ${n.source}`;
  let st, cls = n.status;
  if (n.status === "medindo") st = `Medindo: entra na conta às ${hhmm(n.resultAt)} (2 h depois do começo).`;
  else if (n.status === "contando") st = `Já entrou na conta; o resultado fecha às ${hhmm(n.resultAt)} (4 h depois do começo).`;
  else st = "Não vai entrar na conta: " + (n.prevEnd ? `choveu até as ${hhmm(n.prevEnd)}, menos de 3 h antes, então não há uma base limpa para comparar.` : n.reason + ".");
  let cmp = "";
  const rc = state.ri && state.ri.device === "radiacode", k = (state.ri && state.ri.usvPerCPM) || 0;
  if (n.hasBase && n.hasNow) cmp = rc
    ? `Antes da chuva: ${fmt(n.baseline * k, 3)} µSv/h · últimos 15 min: ${fmt(n.nowCPM * k, 3)} (${sgn((n.nowCPM / n.baseline - 1) * 100, 1)}%)`
    : `Antes da chuva: ${fmt(n.baseline, 1)} CPM · últimos 15 min: ${fmt(n.nowCPM, 1)} (${sgn(n.nowCPM - n.baseline, 1)})`;
  box.className = "ri-now " + cls;
  box.innerHTML = '<strong></strong><span class="small muted"></span><span class="small"></span><span class="small mono muted"></span>';
  box.children[0].textContent = since;
  box.children[1].textContent = what;
  box.children[2].textContent = st;
  box.children[3].textContent = cmp;
  box.children[3].hidden = !cmp;
}

function renderRainImpact(r) {
  state.ri = r;
  renderRainNow(r.now);
  const rc = r.device === "radiacode", k = r.usvPerCPM || 0, base = r.avgBase || 1;
  $("riUnit").textContent = rc ? "% acima da média das 2 h antes (RC102)" : "CPM acima da média das 2 h antes";
  const tiles = $("riTiles"), list = $("riList");
  tiles.textContent = ""; list.textContent = "";
  $("riChartBox").hidden = !r.n;
  if (!r.n) {
    $("riSub").textContent = r.skippedNoData ? `${r.skippedNoData} chuva(s) sem dados suficientes ${rc ? "do RC102" : "do contador"}` : "nenhuma chuva medida ainda";
    tiles.innerHTML = rc
      ? '<p class="small muted">A cada chuva com o RC102 conectado ao PC em casa, o app mede quanto a leitura dele subiu depois e compara com horários sem chuva.</p>'
      : '<p class="small muted">A cada chuva com o contador gravando (app aberto ou histórico baixado), o app mede quanto o CPM subiu depois e compara com horários sem chuva.</p>';
    return;
  }
  $("riSub").textContent = `${r.n} chuva${r.n > 1 ? "s" : ""} medida${r.n > 1 ? "s" : ""} nos últimos 60 dias` + (r.dryN ? ` · comparado com ${r.dryN} horários sem chuva` : "");
  const tile = (label, value, sub) => {
    const d = document.createElement("div");
    d.className = "stat ri-tile";
    d.innerHTML = '<span class="label"></span><span class="value mono"></span><span class="sub small muted"></span>';
    d.children[0].textContent = label; d.children[1].textContent = value; d.children[2].textContent = sub;
    tiles.appendChild(d);
  };
  const eff = r.dryN ? r.effect : r.avgRise;
  if (rc) {
    const pc = (v) => `${sgn((v / base) * 100, 1)}%`;
    tile("Efeito da chuva", pc(eff), `≈ ${sgn(eff * k, 3)} µSv/h · ` + (r.dryN ? `sobe ${pc(r.avgRise)}; sem chuva, ${pc(r.dryRise)}` : "sem horários secos para comparar ainda"));
  } else tile("Efeito da chuva", `${sgn(eff, 1)} CPM`, r.dryN ? `sobe ${sgn(r.avgRise, 1)}; sem chuva, ${sgn(r.dryRise, 1)}` : "sem horários secos para comparar ainda");
  tile("Pico", `~${durMin(r.peakMin)}`, "depois do começo da chuva");
  tile("Volta ao normal", r.backN ? `~${durMin(r.backMin)}` : "> 4 h", r.backN ? `em ${r.backN} de ${r.n}` : "não voltou em 4 h");
  drawRainCurve(r);
  for (const ep of r.episodes.slice(0, 8)) {
    const li = document.createElement("li");
    li.className = "pick";
    li.title = "Ver esse trecho nos gráficos";
    const back = ep.partial ? `ainda medindo (até ${hhmm(ep.start + 4 * 36e5)})`
      : ep.backAt ? `voltou em ${durMin(Math.round((ep.backAt - ep.start) / 60e3))}` : "não voltou em 4 h";
    li.innerHTML = '<span class="ri-when mono"></span><span class="ri-txt"></span>';
    li.children[0].textContent = whenLabel(ep.start);
    li.children[1].innerHTML = "<span></span><span class=\"small muted mono\"></span>";
    li.children[1].children[0].textContent = `${fmt(ep.mm, 1)} mm (até ${fmt(ep.maxMMh, 0)} mm/h) · ${ep.source}`;
    li.children[1].children[1].textContent = (rc
      ? `${fmt(ep.baseline * k, 3)} → ${fmt(ep.peak * k, 3)} µSv/h (${sgn((ep.rise / ep.baseline) * 100, 1)}%)`
      : `${fmt(ep.baseline, 1)} → ${fmt(ep.peak, 1)} CPM (${sgn(ep.rise, 1)})`) + ` em ${durMin(Math.round((ep.peakAt - ep.start) / 60e3))} · ${back}`;
    li.addEventListener("click", () => showRange(ep.start - 2 * 36e5, ep.start + 4 * 36e5));
    list.appendChild(li);
  }
}

function drawRainCurve(r) {
  // Radiacode: a curva em % da base (a escala do CPM dele é outra)
  const pc = r.device === "radiacode" && r.avgBase > 0 ? 100 / r.avgBase : 0;
  const pts = (r.curve || []).map((p) => (pc ? { min: p.min, rain: p.rain == null ? null : p.rain * pc, dry: p.dry == null ? null : p.dry * pc } : p));
  let lo = 0, hi = 1;
  for (const p of pts) for (const v of [p.rain, p.dry]) if (v != null) { lo = Math.min(lo, v); hi = Math.max(hi, v); }
  lo = Math.floor(lo - 0.5);
  hi = lo + 3 * Math.ceil((Math.ceil(hi + 0.5) - lo) / 3); // 4 marcas inteiras
  const x = (m) => ((m + 120) / 360) * W;
  const y = (v) => H - ((v - lo) / (hi - lo)) * H;
  const ys = $("riY").children;
  for (let i = 0; i < 4; i++) { const v = Math.round(hi - ((hi - lo) / 3) * i); ys[i].textContent = v === 0 ? "0" : sgn(v, 0) + (pc ? "%" : ""); }
  const line = (key, color, w) => {
    let d = "", pen = "M";
    for (const p of pts) { if (p[key] == null) { pen = "M"; continue; } d += `${pen}${x(p.min).toFixed(1)} ${y(p[key]).toFixed(1)}`; pen = "L"; }
    return d ? `<path d="${d}" fill="none" stroke="${color}" stroke-width="${w}" stroke-linejoin="round" vector-effect="non-scaling-stroke"/>` : "";
  };
  const x0 = x(0).toFixed(1), y0 = y(0).toFixed(1);
  $("riChart").innerHTML =
    `<line x1="0" y1="${y0}" x2="${W}" y2="${y0}" stroke="${COR.grid}" stroke-width="1" vector-effect="non-scaling-stroke"/>` +
    `<rect x="${x0}" y="0" width="${(x(240) - x(0)).toFixed(1)}" height="${H}" fill="#3DA5FF" fill-opacity="0.05"/>` +
    `<line x1="${x0}" y1="0" x2="${x0}" y2="${H}" stroke="#3DA5FF" stroke-width="1" stroke-dasharray="4 4" vector-effect="non-scaling-stroke"/>` +
    line("dry", COR.muted, 1.5) + line("rain", "#3DA5FF", 2.5);
}

// ---------- anotações ----------

state.noteEdit = null;

function openNote(n) {
  state.noteEdit = n;
  $("noteTitle").textContent = n.id ? "Editar anotação" : "Nova anotação";
  $("noteText").value = n.text || "";
  $("noteWhen").value = toLocalInput(n.time || Date.now());
  $("noteWhen").max = toLocalInput(Date.now());
  $("noteEnd").value = n.end ? toLocalInput(n.end) : "";
  $("noteEnd").max = toLocalInput(Date.now());
  if (!n.id) $("noteTitle").textContent = n.end ? "Anotar trecho" : "Nova anotação";
  $("btnNoteDelete").hidden = !n.id;
  $("noteDlg").showModal();
  $("noteText").focus();
}

function wireNotes() {
  $("btnAddNote").addEventListener("click", () => openNote({ id: 0, time: Date.now(), text: "" }));
  const save = async () => {
    const n = state.noteEdit || { id: 0 };
    const v = $("noteWhen").value, ve = $("noteEnd").value;
    const time = v ? new Date(v).getTime() : Date.now();
    const end = ve ? new Date(ve).getTime() : 0;
    try {
      await api.SaveNote({ id: n.id || 0, time, end, text: $("noteText").value, created: n.created || 0 });
      $("noteDlg").close();
      toast("Anotação salva");
    } catch (e) { toast(errMsg(e), true); }
  };
  $("btnNoteSave").addEventListener("click", save);
  $("noteText").addEventListener("keydown", (e) => { if (e.key === "Enter") { e.preventDefault(); save(); } });
  $("btnNoteDelete").addEventListener("click", async () => {
    const n = state.noteEdit;
    if (!n || !n.id) return;
    try { await api.DeleteNote(n.id); $("noteDlg").close(); toast("Anotação apagada"); } catch (e) { toast(errMsg(e), true); }
  });
}

// ---------- atualização (GitHub, com clique) ----------

// compara "1.18.2" com "1.18.10": >0 se a for mais nova
function verCmp(a, b) {
  const pa = String(a || "").split(".").map(Number), pb = String(b || "").split(".").map(Number);
  for (let i = 0; i < 3; i++) if ((pa[i] || 0) !== (pb[i] || 0)) return (pa[i] || 0) - (pb[i] || 0);
  return 0;
}

function renderUpdate(u) {
  state.update = u = u || state.update || {};
  const show = u.available && !u.skipped;
  const busy = u.state === "baixando" || u.state === "esperando" || u.state === "abrindo";
  $("updPill").hidden = !(show || busy);
  $("updPillText").textContent = u.state === "baixando" ? `baixando v${u.latest}… ${fmt((u.progress || 0) * 100, 0)}%`
    : u.state === "esperando" ? `v${u.latest}: troca quando o CPM normalizar`
    : `v${u.latest} disponível`;
  // configurações
  const when = u.checkedAt ? ` (olhou às ${hhmm(u.checkedAt)})` : "";
  $("updStatus").textContent = u.state === "procurando" ? "procurando…"
    : u.error && u.state !== "erro" ? u.error
    : !u.latest ? "" : u.available ? `versão ${u.latest} disponível${u.skipped ? " (pulada)" : ""}${when}`
    : verCmp(u.current, u.latest) > 0 ? `esta versão (${u.current}) é mais nova que a última publicada (${u.latest})${when}`
    : `você está na versão mais nova (${u.current})${when}`;
  // diálogo
  $("updTitle").textContent = u.latest ? `Versão ${u.latest}` : "Versão nova";
  $("updSub").textContent = `Você está na ${u.current || "?"}.` + (u.canAuto ? "" : u.manual ? ` Atualizar por aqui não dá: ${u.manual}.` : "");
  $("updNotes").textContent = (u.notes || "Sem notas.").replace(/\*\*/g, "");
  const st = $("updState");
  const msg = {
    baixando: `Baixando e conferindo… ${fmt((u.progress || 0) * 100, 0)}%`,
    esperando: `Baixado e conferido. Troca sozinho quando o CPM voltar ao normal (agora: ${u.wait}).`,
    abrindo: "Abrindo a versão nova…",
    erro: `Não deu certo: ${u.error}`,
    cancelado: "Troca cancelada. O arquivo baixado ficou na pasta do app.",
  }[u.state];
  st.hidden = !msg;
  st.textContent = msg || "";
  st.className = "small" + (u.state === "erro" ? " warn-text" : "");
  $("updBar").hidden = u.state !== "baixando";
  $("updBarFill").style.width = `${Math.round((u.progress || 0) * 100)}%`;
  $("btnUpdGo").hidden = !u.canAuto || busy;
  $("btnUpdGo").textContent = u.state === "erro" ? "Tentar de novo" : "Atualizar";
  $("btnUpdCancel").hidden = u.state !== "esperando";
  $("btnUpdLater").textContent = busy ? "Fechar" : "Agora não";
  $("btnUpdSkip").hidden = busy || !u.available;
  $("btnUpdSkip").textContent = u.skipped ? "Voltar a avisar desta versão" : "Pular esta versão";
}

function wireUpdate() {
  on("update", renderUpdate);
  $("updPill").addEventListener("click", () => { renderUpdate(); $("updDlg").showModal(); });
  $("btnUpdGo").addEventListener("click", () => call(api.StartUpdate));
  $("btnUpdCancel").addEventListener("click", () => call(api.CancelUpdate));
  $("btnUpdSkip").addEventListener("click", async () => {
    const u = state.update || {};
    await call(api.SkipUpdate, u.skipped ? "" : u.latest);
    if (!u.skipped) $("updDlg").close();
  });
  $("btnUpdPage").addEventListener("click", () => {
    const u = state.update || {};
    const url = u.url || "https://github.com/matheusAFreitas/radlog/releases";
    if (window.runtime && window.runtime.BrowserOpenURL) window.runtime.BrowserOpenURL(url); else window.open(url, "_blank");
  });
  $("btnUpdDir").addEventListener("click", () => call(api.OpenVersionsDir));
  $("btnUpdCheck").addEventListener("click", async () => {
    try {
      const u = await api.CheckUpdate();
      renderUpdate(u);
      if (u.available) $("updDlg").showModal();
    } catch (e) { renderUpdate(); }
  });
  if (api.GetUpdateInfo) api.GetUpdateInfo().then(renderUpdate).catch(() => {});
}

// ---------- sincronização entre PCs ----------

let syncTimer = null;

function renderSync(info) {
  if (!info) return;
  state.sync = info;
  $("chkSyncListen").checked = !!info.listen;
  $("chkSyncAuto").checked = !!info.auto;
  const li = $("syncListenInfo");
  if (info.listen && info.listenErr) {
    li.textContent = "Não deu para aceitar conexões: " + info.listenErr;
    li.className = "small warn-text";
  } else if (info.listening) {
    li.textContent = `Este PC: ${info.name} · ${(info.addrs || []).join(", ") || "sem rede"}`;
    li.className = "small muted";
  } else {
    li.textContent = "Desligado: este PC só conecta nos outros (no Windows não pede liberação no firewall).";
    li.className = "small muted";
  }
  $("syncCodeRow").hidden = !info.listening;
  const codeOn = !!info.code;
  $("syncCode").textContent = codeOn ? info.code.slice(0, 3) + " " + info.code.slice(3) : "";
  $("btnSyncCode").textContent = codeOn ? "Cancelar código" : "Mostrar código para parear";
  clearInterval(syncTimer);
  const left = () => {
    const s = Math.max(0, Math.round((info.codeUntil - Date.now()) / 1000));
    $("syncCodeLeft").textContent = codeOn ? `vale por mais ${Math.floor(s / 60)}:${pad(s % 60)}` : "";
    if (codeOn && s <= 0) { clearInterval(syncTimer); api.GetSync().then(renderSync).catch(() => {}); }
  };
  left();
  if (codeOn) syncTimer = setInterval(left, 1000);
  const ul = $("syncPeers");
  ul.textContent = "";
  if (!info.peers.length) ul.innerHTML = '<li class="small muted">Nenhum PC pareado ainda.</li>';
  for (const p of info.peers) {
    const item = document.createElement("li");
    const g = document.createElement("span");
    g.className = "grow";
    const name = document.createElement("strong");
    name.textContent = p.name;
    const st = document.createElement("span");
    st.className = "small " + (p.lastError ? "err" : "muted");
    st.textContent = p.lastError ? `Última tentativa ${whenLabel(p.lastTry)}: ${p.lastError}`
      : p.lastSync ? `Sincronizado ${whenLabel(p.lastSync)} · ${p.lastResult || ""}` : "Ainda não sincronizou.";
    const ad = document.createElement("span");
    ad.className = "small mono muted";
    ad.dataset.noTr = "";
    ad.textContent = [...(p.manualAddrs || []), ...(p.addrs || [])].join(", ");
    g.append(name, st, ad);
    const now = document.createElement("button");
    now.type = "button"; now.className = "btn small"; now.textContent = info.running ? "Sincronizando…" : "Sincronizar agora";
    now.disabled = !!info.running;
    now.addEventListener("click", async () => {
      try { await api.SyncNow(p.id); } catch (e) { toast(errMsg(e), true); }
      renderSync(await api.GetSync());
    });
    const addrs = document.createElement("button");
    addrs.type = "button"; addrs.className = "btn small"; addrs.textContent = "Endereços";
    addrs.title = "Endereços extras deste PC, separados por vírgula (ex.: o IP do Tailscale)";
    addrs.addEventListener("click", () => {
      const f = item.querySelector(".nr-form");
      if (f) { f.remove(); return; }
      const form = document.createElement("div");
      form.className = "nr-form sync-pair";
      form.innerHTML = '<input type="text" aria-label="Endereços extras"><button type="button" class="btn small accent">Salvar</button>';
      const inp = form.querySelector("input");
      inp.value = (p.manualAddrs || []).join(", ");
      inp.placeholder = "ex.: 100.101.102.103, notebook";
      form.querySelector("button").addEventListener("click", async () => {
        renderSync(await call(api.SetPeerAddrs, p.id, inp.value.split(/[,\s]+/).filter(Boolean)));
      });
      item.appendChild(form);
      inp.focus();
    });
    const del = document.createElement("button");
    del.type = "button"; del.className = "btn small danger-btn"; del.textContent = "Esquecer";
    del.addEventListener("click", async () => {
      if (del.dataset.sure !== "1") { del.dataset.sure = "1"; del.textContent = "Confirmar"; return; }
      renderSync(await call(api.ForgetPeer, p.id));
    });
    item.append(g, now, addrs, del);
    ul.appendChild(item);
  }
}

function wireSync() {
  on("sync", renderSync);
  $("chkSyncListen").addEventListener("change", async (e) => renderSync(await call(api.SetSyncListen, e.target.checked)));
  $("chkSyncAuto").addEventListener("change", async (e) => renderSync(await call(api.SetSyncAuto, e.target.checked)));
  $("btnSyncCode").addEventListener("click", async () => {
    try { renderSync(state.sync && state.sync.code ? await api.CancelPairCode() : await api.ShowPairCode()); }
    catch (e) { toast(errMsg(e), true); }
  });
  $("btnSyncFind").addEventListener("click", async () => {
    const msg = $("syncFindMsg"), ul = $("syncFound");
    msg.textContent = "Procurando…";
    ul.textContent = "";
    let list = [];
    try { list = await api.DiscoverPeers(); } catch (e) { msg.textContent = errMsg(e); return; }
    msg.textContent = list.length ? "" : "Ninguém respondeu. O outro PC está com \"Aceitar conexões\" ligado e na mesma rede?";
    for (const f of list) {
      const li = document.createElement("li");
      const g = document.createElement("span");
      g.className = "grow";
      g.innerHTML = "<strong></strong><span class='small mono muted' data-no-tr></span>";
      g.firstChild.textContent = f.name + (f.paired ? " (já pareado)" : "");
      g.lastChild.textContent = f.addr;
      const b = document.createElement("button");
      b.type = "button"; b.className = "btn small"; b.textContent = "Usar este";
      b.addEventListener("click", () => { $("syncAddr").value = f.addr; $("syncPairCode").focus(); });
      li.append(g, b);
      ul.appendChild(li);
    }
  });
  $("btnSyncPair").addEventListener("click", async () => {
    const msg = $("syncPairMsg");
    msg.className = "small";
    msg.textContent = "Pareando…";
    try {
      renderSync(await api.PairWith($("syncAddr").value, $("syncPairCode").value));
      msg.textContent = "Pareado. A primeira sincronização já começou.";
      $("syncPairCode").value = "";
      $("syncFound").textContent = "";
    } catch (e) {
      msg.className = "small warn-text";
      msg.textContent = errMsg(e);
    }
  });
  if (api.GetSync) api.GetSync().then(renderSync).catch(() => {});
}

// ---------- Radiacode (aparelho de verdade) ----------

state.rc = { st: null, cmpMin: 360, accum: false, log: true, spec: null };

function rcDur(sec) {
  if (!(sec > 0)) return "";
  const h = sec / 3600;
  return h < 1 ? `${Math.round(sec / 60)} min` : h < 48 ? `${fmt(h, 1)} h` : `${Math.round(h / 24)} dias`;
}

function renderRc(st) {
  if (!st) return;
  const prev = state.rc.st;
  state.rc.st = st;
  rcAlarmState(st);
  renderDoseDevice();
  const on = st.state === "conectado";
  const via = st.demo ? "simulado" : st.via === "bluetooth" ? "Bluetooth" : "USB";
  // pílula no topo: só aparece quando tem algo a dizer
  $("navRcDot").hidden = st.state !== "erro";
  // aba
  $("rcSub").textContent = on ? `conectado pelo ${via}` : { off: "procura desligada", procurando: "procurando no USB" + (st.ble ? " e no Bluetooth" : "") + "…", conectando: "conectando…", erro: "desconectado" }[st.state] || st.state;
  // o aviso de conexão fica no cartão de cima; aqui só o tipo e o último contato
  $("rcState").textContent = on ? `conectado · ${via}` : [st.via ? via : "", st.lastT ? `último contato ${isToday(st.lastT) ? hhmm(st.lastT) : whenLabel(st.lastT)}` : st.state === "conectando" ? "conectando…" : st.state === "procurando" ? "procurando…" : "desconectado"].filter(Boolean).join(" · ");
  $("rcState").className = "rc-state mono small " + (on ? "ok" : "");
  $("rcNums").hidden = !on;
  if (on) {
    $("rcUsv").textContent = st.time ? `${fmt(st.usvh, usvDigits(st.usvh))} µSv/h` : "—";
    $("rcUsvErr").textContent = st.time ? `± ${fmt(st.usvhErr, 0)}%` : "";
    $("rcCps").textContent = st.time ? `${fmt(st.cps, 2)} cps` : "—";
    $("rcCpsErr").textContent = st.time ? `± ${fmt(st.cpsErr, 0)}% · ${fmt(st.cps * 60, 0)} CPM` : "";
    $("rcDose").textContent = st.doseLiveAt ? `${fmt(st.doseLive, st.doseLive < 10 ? 3 : 2)} µSv` : "—";
    $("rcDoseSince").textContent = st.doseLiveAt && st.doseLiveSec ? `em ${rcDur(st.doseLiveSec)}` + (st.doseLiveOn ? "" : ` · às ${hhmm(st.doseLiveAt)}`) : "";
    $("rcBat").textContent = st.hasRare ? `${fmt(st.battery, 0)}%` : "—";
    $("rcTemp").textContent = st.hasRare ? `${fmt(st.tempC, 1)} °C` : "";
  }
  $("rcDevInfo").textContent = on ? [st.model, st.serial && "série " + st.serial, st.firmware && "firmware " + st.firmware].filter(Boolean).join(" · ") : "";
  $("rcHelp").textContent = on ? "" : st.state === "erro" ? st.err
    : st.state === "off" ? "A procura está desligada em Configurações → Radiacode 102."
    : "Ligue o RC102 no USB do PC" + (st.bleOK ? (st.ble ? ", ou deixe-o perto com o Bluetooth ligado (e o app do celular desconectado)." : ", ou configure o endereço Bluetooth.") : ". No Windows, por enquanto, a conexão é só pelo USB.");
  $("rcHelp").className = "small muted";
  // com o cartão de cima à vista, o motivo e as ações estão no aviso de lá
  if (!on && (st.state === "conectando" || st.lastT > 0)) $("rcHelp").textContent = "";
  if (on && (!prev || prev.state !== "conectado")) { refreshRcCompare(); refreshChart(); refreshDose(); }
  // Radiacode como aparelho principal: na primeira vez, o gráfico vai para µSv/h
  if (hasRcData() && !localGet("rcPrimaryUnit", "")) {
    localSet("rcPrimaryUnit", "1");
    if (state.chartUnit !== "usv") { state.chartUnit = "usv"; localSet("chartUnit", "usv"); refreshChart(); }
  }
  if (hasRcData() && prev && !(prev.state === "conectado" || prev.lastT > 0)) refreshDose();
  renderRcMain(st);
  // configurações
  $("chkRcOn").checked = !st.off;
  $("chkRcRaw").checked = st.raw !== false;
  if (document.activeElement !== $("rcBLE")) $("rcBLE").value = st.ble || "";
  $("btnRcBLEScan").disabled = !st.bleOK;
  renderRcSetState(st, prev);
  renderSetNavStatus();
  const noBLE = "Neste computador a conexão Bluetooth não está disponível: use o cabo USB.";
  if (!st.bleOK) $("rcBLEInfo").textContent = noBLE;
  else if ($("rcBLEInfo").textContent === noBLE) $("rcBLEInfo").textContent = "";
}

// estado da conexão nas Configurações → Radiacode (e o aviso de quando o
// endereço escolhido conectou ou deu erro)
function renderRcSetState(st, prev) {
  const via = st.demo ? "simulado" : st.via === "bluetooth" ? "Bluetooth" : "USB";
  const txt = {
    conectado: `Conectado pelo ${via}` + (st.model || st.serial ? ` · ${[st.model, st.serial].filter(Boolean).join(" ")}` : ""),
    conectando: st.via ? `Conectando pelo ${via}…` : "Conectando…",
    procurando: "Procurando" + (st.ble ? " no USB e no Bluetooth…" : " no USB…"),
    erro: "Não conectou: " + (st.err || "erro"),
    off: "Procura desligada",
  }[st.state] || st.state;
  $("rcSetText").textContent = txt;
  $("rcSetDot").className = "dot " + (st.state === "conectado" ? "on" : st.state === "erro" ? "err" : "wait");
  $("rcSetState").classList.toggle("err", st.state === "erro");
  const p = state.rc.pendingBLE;
  if (!p) return;
  const info = $("rcBLEInfo");
  if (st.state === "conectado" && st.via === "bluetooth") {
    info.textContent = "";
    toast("RC102 conectado pelo Bluetooth");
    state.rc.pendingBLE = null;
  } else if (st.state === "conectado" && st.via === "usb") {
    info.textContent = "O RC102 está no cabo USB: o app usa o USB enquanto ele estiver ligado e passa para o Bluetooth quando você tirar o cabo.";
    state.rc.pendingBLE = null;
  } else if (st.state === "erro" && prev && prev.state !== "erro") {
    info.textContent = "Não deu para conectar pelo Bluetooth. O app do celular está conectado nele? O app tenta de novo sozinho a cada 30 segundos.";
  } else if (Date.now() - p.at > 60e3) {
    state.rc.pendingBLE = null;
  }
}

// quadro do Radiacode na tela principal: cheio quando conectado, uma linha
// com o último dado quando não
state.rcUnit = localGet("rcUnit", "usv");

// quadro do Radiacode, com as mesmas informações do GMC: valor, dose de
// hoje, normal/σ, régua e medidas
// conexão do GMC no canto do quadro, como a do Radiacode (só lado a lado)
function renderHeroVia() {
  const el = $("heroVia");
  const s = state.status || {}, d = s.device || {};
  const on = s.state === "conectado";
  el.hidden = false;
  el.textContent = on ? `· ${d.port || ""}` : state.demo && state.demo.active ? "" : "· desconectado";
  el.className = "rc-main-via gmc-via mono" + (on ? " on" : "");
  if (gmcConn()) renderGmcConn();
}

function renderRcMain(st) {
  const live = st.state === "conectado" && st.time;
  const show = st.state === "conectado" || st.state === "conectando" || st.lastT > 0;
  $("rcMain").hidden = !show;
  $("rTile").hidden = !show;
  document.body.classList.toggle("has-rc", show);
  renderHeroVia();
  if (!show) return;
  const via = st.demo ? "simulado" : st.via === "bluetooth" ? "Bluetooth" : "USB";
  $("rcMainVia").textContent = st.state === "conectado" ? `· ${via}` + (st.hasRare ? ` · ${fmt(st.battery, 0)}%` : "") : st.state === "conectando" ? "· conectando…" : "· desconectado";
  $("rcMainVia").className = "rc-main-via mono" + (st.state === "conectado" ? " on" : "");
  $("rcMain").classList.toggle("stale", !live);
  const sx = st.stats || {};
  // valor (ao vivo ou o último gravado, apagado)
  const usv = live ? st.usvh : st.lastT ? st.lastUSv : null;
  const cps = live ? st.cps : st.lastT ? st.lastCPS : null;
  const u = state.rcUnit;
  document.querySelectorAll("#rcUnitSeg button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.unit === u)));
  $("rcMainUnit").textContent = { usv: "µSv/h", mrh: "mrem/h", cps: "CPS", cpm: "CPM" }[u];
  const v = usv == null ? null : { usv, mrh: usv / 10, cps, cpm: cps * 60 }[u];
  $("rcMainVal").textContent = v == null ? "—" : u === "usv" ? fmt(v, usvDigits(v)) : u === "mrh" ? fmt(v, usvDigits(v * 10) + 1) : u === "cps" ? fmt(v, 2) : fmt(v, 0);
  const vs = usv == null ? "" : `${fmt(usv, usvDigits(usv))} µSv/h`;
  // mesmas linhas do quadro do GMC em cada unidade
  const alt = usv == null ? "" : {
    cpm: `≈ ${vs} · ${fmt(cps, 1)} CPS`,
    usv: `${fmt(cps * 60, 0)} CPM · ${fmt(usv / 10, usvDigits(usv) + 1)} mrem/h`,
    mrh: `${fmt(cps * 60, 0)} CPM · ${vs}`,
    cps: `${fmt(cps * 60, 0)} CPM · ${vs}`,
  }[u];
  $("rcMainAlt").textContent = live ? `${alt} · ± ${fmt(st.usvhErr, 0)}%` : st.lastT ? `última leitura ${isToday(st.lastT) ? "às " + hhmm(st.lastT) : "(" + whenLabel(st.lastT) + ")"}` : "sem leituras ainda";
  // dose de hoje
  if (st.todayDevOK) {
    $("rcDoseToday").textContent = fmt(st.todayDev, 2);
    $("rcDoseTodaySub").textContent = "do aparelho";
    $("rcDoseToday").title = `Somada pelo próprio RC102, também longe do PC. Pelo app (só os minutos gravados): ${fmt(st.today, 2)} µSv`;
  } else {
    $("rcDoseToday").textContent = sx.todayMinutes || st.today ? fmt(st.today, 2) : "—";
    $("rcDoseTodaySub").textContent = sx.todayMinutes ? `${sx.todayMinutes} min medidos · app` : "acumulados";
    $("rcDoseToday").title = "Calculada pelo app: só os minutos que o PC gravou";
  }
  // normal / σ, pelo fundo em casa dos últimos 7 dias
  const sig = $("rcMainSigma"), num = $("rcMainSigmaNum");
  if (live && sx.bgUSv > 0) {
    const sdDev = st.usvh * (st.usvhErr || 0) / 100, sdBg = (sx.bgHi - sx.bgLo) / 2.56;
    const sd = Math.max(sdDev, sdBg, 1e-4), z = (st.usvh - sx.bgUSv) / sd, d = usvDigits(sx.bgUSv);
    const range = `${fmt(sx.bgLo, d)} a ${fmt(sx.bgHi, d)} µSv/h`;
    let text, cls = "";
    if (z >= 3) { text = `Acima do normal (seu normal: ${range}) — dificilmente é só variação`; cls = "high"; }
    else if (z >= 2) { text = `Um pouco acima do normal (seu normal: ${range}) — pode ser só variação`; cls = "mid"; }
    else if (z <= -2) { text = `Abaixo do normal (seu normal: ${range}) — sem problema`; }
    else { text = `Dentro do normal — o nível de fundo costuma ficar entre ${fmt(sx.bgLo, d)} e ${fmt(sx.bgHi, d)} µSv/h`; }
    sig.textContent = text;
    sig.className = "sigma " + cls;
    sig.title = `Pelos minutos em casa dos últimos 7 dias, o RC102 costuma marcar ${fmt(sx.bgUSv, d)} µSv/h (80% entre ${range}).`;
    sig.hidden = false;
    $("rcMainSigmaVal").textContent = `${fmtSig(z)} · fundo ${fmt(sx.bgUSv, d)} µSv/h · 1σ = ${fmt(sd, usvDigits(sd))} µSv/h`;
    num.hidden = false;
  } else if (live) {
    sig.textContent = `Fundo: juntando dados em casa (${Math.round((sx.bgMinutes || 0))} de 60 min)`;
    sig.className = "sigma";
    sig.title = "";
    sig.hidden = false;
    num.hidden = true;
  } else { sig.hidden = true; num.hidden = true; }
  // régua: os alarmes do próprio Radiacode; sem eles, as faixas do app em
  // µSv/h (as mesmas do GMC-800)
  const al = st.alarms || {};
  const own = al.DoseRate1 > 0 && al.DoseRate2 > al.DoseRate1;
  const attn = own ? al.DoseRate1 : st.attnUSv || 0.505, alarm = own ? al.DoseRate2 : st.alarmUSv || 1.01, top = alarm * 1.5;
  $("rcLevelHint").textContent = own ? "alarmes do RC102" : "faixas do app em µSv/h";
  const lvl = usv == null ? "" : !own && st.dangerUSv && usv >= st.dangerUSv ? "perigo" : usv >= alarm ? "alarme" : usv >= attn ? "atencao" : "seguro";
  lvLabel($("rcLevelName"), lvl, (own ? { seguro: "Seguro", atencao: "Alarme 1", alarme: "Alarme 2" } : { seguro: "Seguro", atencao: "Atenção", alarme: "Alarme", perigo: "Perigo" })[lvl] || "—", 16);
  $("rcLevelName").className = "sr-only level-name " + lvl;
  ["seguro", "atencao", "alarme", "perigo"].forEach((l) => $("rcMain").classList.toggle("lv-" + l, live && lvl === l));
  renderRcDial(st, { usv, cps, lvl, own, attn, alarm });
  renderRTile(st, { usv, cps, live, lvl, own, attn, alarm, top });
  state.rNow = { live, lvl, usv };
  agoraNow();
  renderDevMini();
  // medidas
  const fu = (x) => (x == null ? "—" : fmt(x, usvDigits(x)));
  $("rcAvg1h").textContent = sx.n1h ? fu(sx.avg1h) : "—";
  $("rcMin24").textContent = sx.n24h ? fu(sx.min24h) : "—";
  $("rcMax24").textContent = sx.n24h ? fu(sx.max24h) : "—";
  $("rcMax24At").textContent = sx.n24h ? `µSv/h às ${hhmm(sx.max24hAt)}` : "µSv/h";
}

async function refreshRcCompare() {
  if (!api.GetRcCompare || state.view !== "radiacode") return;
  let c;
  try { c = await api.GetRcCompare(state.rc.cmpMin); } catch (e) { $("rcCmpStats").textContent = errMsg(e); return; }
  drawRcCompare(c);
}

function drawRcCompare(c) {
  const box = $("rcCmpChart");
  if (!c || !c.minutes) {
    box.innerHTML = `<div class="rc-empty small muted">Sem minutos com os dois aparelhos nesse período.</div>`;
    $("rcCmpStats").textContent = c && c.rcOnly ? `${c.rcOnly} min só do RC102 (GMC desconectado?)` : "";
    return;
  }
  const W = 1000, H = 260, PL = 46, PB = 22, n = c.t.length;
  // média de 10 min para as curvas (o minuto sozinho é muito ruidoso)
  const avg = (arr, k) => arr.map((_, i) => { const a = Math.max(0, i - k + 1); let s = 0; for (let j = a; j <= i; j++) s += arr[j]; return s / (i - a + 1); });
  const g = avg(c.gmc, 10), r = avg(c.rc, 10);
  const t0 = c.t[0], t1 = Math.max(c.t[n - 1], t0 + 60e3);
  let top = 0;
  for (let i = 0; i < n; i++) top = Math.max(top, g[i], r[i]);
  top = top * 1.15 || 0.2;
  const x = (t) => PL + ((t - t0) / (t1 - t0)) * (W - PL - 8), y = (v) => 8 + (1 - v / top) * (H - PB - 8);
  const path = (arr) => arr.map((v, i) => `${i ? "L" : "M"}${x(c.t[i]).toFixed(1)},${y(v).toFixed(1)}`).join("");
  let svg = `<svg viewBox="0 0 ${W} ${H}" preserveAspectRatio="none" class="rc-cmp-svg">`;
  for (let k = 0; k <= 4; k++) {
    const v = (top / 4) * k, yy = y(v);
    svg += `<line x1="${PL}" x2="${W - 8}" y1="${yy}" y2="${yy}" class="grid"/><text x="${PL - 6}" y="${yy + 4}" text-anchor="end" class="ax">${fmt(v, 2)}</text>`;
  }
  svg += `<text x="${PL}" y="${H - 4}" class="ax">${hhmm(t0)}</text><text x="${W - 8}" y="${H - 4}" text-anchor="end" class="ax">${hhmm(t1)}</text>`;
  svg += `<path d="${path(g)}" class="gmc"/><path d="${path(r)}" class="rc"/></svg>`;
  box.innerHTML = svg + `<div class="rc-legend small"><span class="k gmc"></span>GMC-800 (µSv/h, média de 10 min) <span class="k rc"></span>RC102</div>`;
  const parts = [`${n} min juntos`];
  if (c.ratio) parts.push(`RC102 ÷ GMC = ${fmt(c.ratio, 2)}${c.ratioErr ? ` ± ${fmt(c.ratioErr, 2)}` : ""}`);
  if (c.corr) parts.push(`correlação ${fmt(c.corr, 2)}`);
  if (n < 24 * 60) parts.push(`para o fator valer, junte 24 h (faltam ${rcDur((24 * 60 - n) * 60)})`);
  $("rcCmpStats").textContent = parts.join(" · ");
}

function wireRc() {
  on("radiacode", (st) => {
    renderRc(st);
    measureLogRc(st);
    // a tela de medir muda o aparelho padrão e o aviso quando o Radiacode conecta/cai
    if (rcOnline() !== state.rcWasOnline) { state.rcWasOnline = rcOnline(); if (state.measureSt) renderMeasure(state.measureSt); }
  });
  $("rcUnitSeg").addEventListener("click", (e) => {
    const b = e.target.closest("button[data-unit]");
    if (!b) return;
    state.rcUnit = b.dataset.unit;
    localSet("rcUnit", state.rcUnit);
    if (state.rc.st) renderRcMain(state.rc.st);
  });
  $("btnRcRetry").addEventListener("click", async () => renderRc(await call(api.ReconnectRadiacode)));
  $("btnRcSettings").addEventListener("click", () => openSettings("chkRcOn"));
  $("chkRcOn").addEventListener("change", async (e) => renderRc(await call(api.SetRadiacode, e.target.checked, $("rcBLE").value)));
  $("chkRcRaw").addEventListener("change", async (e) => renderRc(await call(api.SetRcRaw, e.target.checked)));
  $("rcBLE").addEventListener("change", async () => renderRc(await call(api.SetRadiacode, $("chkRcOn").checked, $("rcBLE").value)));
  $("btnRcBLEScan").addEventListener("click", async () => {
    const ul = $("rcBLEFound"), info = $("rcBLEInfo");
    ul.textContent = "";
    info.textContent = "Procurando (uns 6 segundos)…";
    let list = [];
    try { list = await api.ScanRadiacodeBLE(); } catch (e) { info.textContent = errMsg(e); return; }
    info.textContent = list.length ? "" : "Nenhum RC102 apareceu. O Bluetooth dele está ligado? O app do celular está desconectado?";
    for (const d of list) {
      const li = document.createElement("li");
      const g = document.createElement("span");
      g.className = "grow";
      g.innerHTML = "<strong data-no-tr></strong><span class='small mono muted' data-no-tr></span>";
      g.firstChild.textContent = d.name;
      g.lastChild.textContent = d.address + (d.rssi ? ` · ${d.rssi} dBm` : "");
      const b = document.createElement("button");
      b.type = "button"; b.className = "btn small"; b.textContent = "Usar este";
      b.addEventListener("click", async () => {
        $("rcBLE").value = d.address;
        state.rc.pendingBLE = { address: d.address, name: d.name, at: Date.now() };
        ul.textContent = "";
        info.textContent = `Conectando a ${d.name}… leva alguns segundos. Deixe o app do celular desconectado.`;
        renderRc(await call(api.SetRadiacode, true, d.address));
      });
      li.append(g, b);
      ul.appendChild(li);
    }
  });
  $("rcCmpRange").addEventListener("click", (e) => {
    const b = e.target.closest("button[data-min]");
    if (!b) return;
    state.rc.cmpMin = +b.dataset.min;
    $("rcCmpRange").querySelectorAll("button").forEach((x) => x.setAttribute("aria-pressed", String(x === b)));
    refreshRcCompare();
  });
  setInterval(() => { if (state.view === "radiacode" && state.rc.st && state.rc.st.state === "conectado") refreshRcCompare(); }, 60e3);
  if (api.GetRadiacode) api.GetRadiacode().then(renderRc).catch(() => {});
}

// ---------- resumo e comparação ----------

state.sum = { mode: "resumo", preset: "hoje", cmp: "hoje-ontem", day: 0, dayA: 0, dayB: 0 };
const DAY_MS = 864e5;
const startDay = (t) => { const d = new Date(t); d.setHours(0, 0, 0, 0); return d.getTime(); };
const addDays = (t, n) => { const d = new Date(t); d.setDate(d.getDate() + n); return d.getTime(); };
const startWeek = (t) => addDays(startDay(t), -((new Date(t).getDay() + 6) % 7)); // segunda-feira
const dateVal = (t) => { const d = new Date(t); return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`; };
const parseDateVal = (v) => { const [y, m, d] = v.split("-").map(Number); return new Date(y, m - 1, d).getTime(); };
const WEEKDAY = ["dom", "seg", "ter", "qua", "qui", "sex", "sáb"];
const dayLabel = (t) => isToday(t) ? "hoje" : isYesterday(t) ? "ontem" : `${WEEKDAY[new Date(t).getDay()]} ${ddmm(t)}`;
const hm = (min) => (min < 60 ? `${min} min` : `${Math.floor(min / 60)} h${min % 60 ? " " + pad(min % 60) + " min" : ""}`);

const startMonth = (t, add) => { const d = new Date(t); return new Date(d.getFullYear(), d.getMonth() + add, 1).getTime(); };

// ---------- memória e sincronização do contador (2.7) ----------

async function refreshCounterSync(cs) {
  if (!cs) {
    if (!api.GetCounterSync) return;
    try { cs = await api.GetCounterSync(); } catch (e) { return; }
  }
  state.counterSync = cs;
  const pct = fmt(cs.estPct, cs.estPct < 10 ? 1 : 0);
  const memTxt = !cs.estPct ? "" : cs.fullIn > 0
    ? `A memória está cerca de ${pct}% ocupada (estimado desde o download) e enche em cerca de ${fmtDuration(cs.fullIn)} no modo "${cs.saveMode}".`
    : `A memória está cerca de ${pct}% ocupada (estimado desde o download).`;
  $("iSync").textContent = cs.text + (memTxt ? " " + memTxt : "");
  const warnMem = cs.memLevel === "atencao" || cs.memLevel === "cheia";
  const show = cs.state === "pendente" || warnMem;
  const card = $("syncCard");
  card.hidden = !show;
  $("navGmcDot").hidden = !show;
  const gs = $("gTileSync");
  gs.hidden = !show;
  gs.textContent = warnMem ? (cs.memLevel === "cheia" ? "memória cheia" : `memória em ${pct}%`) : "minutos para baixar";
  gs.classList.toggle("full", cs.memLevel === "cheia");
  gs.title = cs.text;
  card.classList.toggle("cheia", cs.memLevel === "cheia");
  $("btnHistory").classList.toggle("pending", show);
  $("btnHistory").title = cs.text;
  if (!show) return;
  $("syncTitle").textContent = warnMem ? (cs.memLevel === "cheia" ? "Memória do contador praticamente cheia" : "Memória do contador perto de encher") : "Há minutos para buscar no contador";
  $("syncText").textContent = warnMem
    ? `${memTxt} Quando enche, dados novos podem não ser gravados ou podem sobrescrever os antigos. Baixe o histórico para guardar tudo no app; depois, se quiser liberar espaço, apague a memória pelo menu do contador.` + (cs.state === "pendente" ? " " + cs.text : "")
    : cs.text;
  $("btnSyncDl").disabled = $("btnHistory").disabled;
}

// ---------- padrões do fundo (2.5) ----------

let patSeq = 0;
async function refreshPatterns() {
  if (!api.GetPatterns || state.view !== "graficos" || state.sub.graficos !== "padroes") return;
  const rcOn = hasRcData();
  $("patDevSeg").hidden = !rcOn;
  if (!rcOn) state.patDev = "";
  const dev = state.patDev || "", days = state.patDays || 90;
  document.querySelectorAll("#patDevSeg button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.pdev === dev)));
  document.querySelectorAll("#patSeg button").forEach((b) => b.setAttribute("aria-pressed", String(+b.dataset.pdays === days)));
  const my = ++patSeq;
  let p;
  try { p = await api.GetPatterns(days, dev); } catch (e) { return; }
  if (my !== patSeq) return;
  state.pat = p;
  drawPatterns(p);
}

function patVal(p, v) { return p.unit === "CPM" ? fmt(v, 1) : fmt(v, usvDigits(v)); }

function drawPatterns(p) {
  const rc = p.device === "radiacode", color = rc ? COR.rc : COR.accent;
  const hist = p.first ? Math.max(0, (p.to - Math.max(p.from, p.first)) / DAY_MS) : 0;
  $("patSub").textContent = !p.minutes ? "sem minutos em casa nessa janela"
    : `${fmt(p.minutes, 0)} min em casa (${p.unit})` + (p.first && p.first > p.from ? ` · janela em formação (${fmt(hist, hist < 10 ? 1 : 0)} de ${p.days} dias)` : "") +
      (p.peakHour >= 0 ? ` · mais alto por volta das ${p.peakHour}h, mais baixo por volta das ${p.lowHour}h` : "");
  // hora do dia
  const svg = $("patChart"), PH = 160;
  const hs = p.hours || [];
  let lo = Infinity, hi = -Infinity;
  for (const h of hs) if (h.n >= 30) { lo = Math.min(lo, h.p25, h.mean); hi = Math.max(hi, h.p75, h.mean); }
  const ys = $("patY").children;
  if (!isFinite(lo)) {
    svg.innerHTML = ""; for (const e of ys) e.textContent = ""; $("patLegend").textContent = "";
  } else {
    const pad = (hi - lo) * 0.1 || hi * 0.05 || 1;
    lo = Math.max(0, lo - pad); hi += pad;
    const x = (h) => (h / 23) * W;
    const y = (v) => PH - ((v - lo) / (hi - lo)) * PH;
    for (let i = 0; i < 4; i++) ys[i].textContent = patVal(p, hi - ((hi - lo) / 3) * i);
    let out = [0, 1, 2, 3].map((i) => { const gy = i === 3 ? PH - 0.5 : i === 0 ? 0.5 : (PH / 3) * i; return `<line x1="0" y1="${gy}" x2="${W}" y2="${gy}" stroke="${COR.grid}" stroke-width="1" vector-effect="non-scaling-stroke"/>`; }).join("");
    // noite (18h–6h) com fundo levemente mais escuro
    out += `<rect x="0" y="0" width="${x(6).toFixed(1)}" height="${PH}" fill="#000" fill-opacity="0.18"/><rect x="${x(18).toFixed(1)}" y="0" width="${(W - x(18)).toFixed(1)}" height="${PH}" fill="#000" fill-opacity="0.18"/>`;
    const ok = hs.map((h, i) => ({ h, i })).filter((q) => q.h.n >= 30);
    if (ok.length > 1) {
      const top = ok.map((q) => `${x(q.i).toFixed(1)} ${y(q.h.p75).toFixed(1)}`).join(" L");
      const bot = ok.slice().reverse().map((q) => `${x(q.i).toFixed(1)} ${y(q.h.p25).toFixed(1)}`).join(" L");
      out += `<path d="M${top} L${bot} Z" fill="${color}" fill-opacity="0.15"/>`;
      out += `<path d="M${ok.map((q) => `${x(q.i).toFixed(1)} ${y(q.h.median).toFixed(1)}`).join(" L")}" fill="none" stroke="${color}" stroke-width="2" vector-effect="non-scaling-stroke"/>`;
      out += `<path d="M${ok.map((q) => `${x(q.i).toFixed(1)} ${y(q.h.mean).toFixed(1)}`).join(" L")}" fill="none" stroke="${COR.text}" stroke-opacity="0.6" stroke-width="1.2" stroke-dasharray="4 3" vector-effect="non-scaling-stroke"/>`;
    }
    svg.innerHTML = out;
    $("patLegend").innerHTML = `<span class="key"><i style="border-color:${color}"></i>mediana</span><span class="key"><b class="sw" style="background:${color}"></b>1º a 3º quartil</span><span class="key"><i style="border-color:${COR.text};border-top-style:dashed;opacity:.6"></i>média</span><span class="key"><b class="sw" style="background:#000;opacity:.5"></b>noite (18h–6h)</span>`;
  }
  // dia × noite
  const dn = $("patDN");
  dn.innerHTML = "";
  for (const [lab, st] of [["Dia (6h–18h)", p.day], ["Noite (18h–6h)", p.night]]) {
    const d = document.createElement("div");
    d.className = "pat-dn-v";
    d.innerHTML = "<span class='small muted'></span><b></b><span class='small muted'></span>";
    d.children[0].textContent = lab;
    d.children[1].textContent = st && st.n ? `${patVal(p, st.median)} ${p.unit}` : "—";
    d.children[2].textContent = st && st.n ? `mediana · média ${patVal(p, st.mean)} · ${fmt(st.n, 0)} min` : "sem dados";
    dn.appendChild(d);
  }
  $("patDNText").textContent = p.minutes ? p.dnText : "";
  // barras
  const bars = (box, rows) => {
    box.textContent = "";
    box.classList.toggle("rc", rc);
    const vals = rows.filter((r) => r.st && r.st.n >= 60).map((r) => r.st.median);
    if (!vals.length) { box.innerHTML = '<span class="small muted">sem dados</span>'; return; }
    const mn = Math.min(...vals), mx = Math.max(...vals), base = Math.max(0, mn - (mx - mn) * 0.6 - mx * 0.02);
    for (const r of rows) {
      const l = document.createElement("span"); l.className = "lab"; l.textContent = r.label;
      const wrap = document.createElement("span"); wrap.className = "track";
      const v = document.createElement("span"); v.className = "v";
      if (r.st && r.st.n >= 60) {
        const b = document.createElement("span"); b.className = "bar";
        b.style.width = `${Math.max(2, ((r.st.median - base) / (mx - base)) * 100).toFixed(1)}%`;
        wrap.appendChild(b);
        v.textContent = `${patVal(p, r.st.median)} · ${r.st.days} ${r.st.days === 1 ? "dia" : "dias"}`;
      } else v.textContent = "—";
      box.append(l, wrap, v);
    }
  };
  const WD = ["domingo", "segunda", "terça", "quarta", "quinta", "sexta", "sábado"];
  bars($("patWd"), [1, 2, 3, 4, 5, 6, 0].map((i) => ({ label: WD[i], st: (p.weekdays || [])[i] })));
  bars($("patMonths"), (p.months || []).map((m) => ({ label: new Date(m.month).toLocaleDateString(LOCALE, { month: "short", year: "2-digit" }), st: m })));
}

function wirePatterns() {
  state.patDays = parseInt(localGet("patDays") || "90", 10) || 90;
  document.querySelectorAll("#patSeg button").forEach((b) => b.addEventListener("click", () => { state.patDays = +b.dataset.pdays; localSet("patDays", String(state.patDays)); refreshPatterns(); }));
  document.querySelectorAll("#patDevSeg button").forEach((b) => b.addEventListener("click", () => { state.patDev = b.dataset.pdev; refreshPatterns(); }));
  const plot = $("patPlot");
  plot.addEventListener("mousemove", (ev) => {
    const p = state.pat;
    if (!p || !p.hours) return;
    const r = plot.getBoundingClientRect();
    const h = Math.max(0, Math.min(23, Math.round(((ev.clientX - r.left) / r.width) * 23)));
    const st = p.hours[h];
    $("patCursor").hidden = false;
    $("patCursor").style.left = (h / 23) * 100 + "%";
    const tip = $("patTip");
    tip.hidden = false;
    tip.textContent = st.n ? `${pad(h)}h · mediana ${patVal(p, st.median)} · quartis ${patVal(p, st.p25)}–${patVal(p, st.p75)} · média ${patVal(p, st.mean)} ${p.unit} · ${fmt(st.n, 0)} min em ${st.days} ${st.days === 1 ? "dia" : "dias"}` : `${pad(h)}h · sem dados`;
    const left = (h / 23) * r.width;
    tip.style.left = Math.min(Math.max(0, left - tip.offsetWidth / 2), r.width - tip.offsetWidth) + "px";
  });
  plot.addEventListener("mouseleave", () => { $("patCursor").hidden = true; $("patTip").hidden = true; });
}

// ---------- fundo ao longo do tempo (2.4) ----------

const BG_STATE = { estavel: "Estável", subindo: "Tendência de alta", descendo: "Tendência de queda", degrau: "Mudança brusca", insuficiente: "Ainda sem tendência" };
let bgSeq = 0;
async function refreshBgTrend() {
  if (!api.GetBgTrend || state.view !== "graficos" || state.sub.graficos !== "tendencia") return;
  const days = state.bgDays || 30;
  document.querySelectorAll("#bgSeg button").forEach((b) => b.setAttribute("aria-pressed", String(+b.dataset.bgdays === days)));
  const my = ++bgSeq;
  let tr;
  try { tr = await api.GetBgTrend(days); } catch (e) { return; }
  if (my !== bgSeq) return;
  state.bgTrend = tr;
  drawBgTrend(tr);
}

function drawBgTrend(tr) {
  const pts = tr.pts || [], daily = tr.daily || [];
  const hist = tr.first ? Math.max(0, (tr.to - Math.max(tr.from, tr.first)) / DAY_MS) : 0;
  const forming = tr.first && tr.first > tr.from;
  $("bgSub").textContent = !tr.minutes ? "sem minutos em casa nessa janela"
    : `${fmt(tr.minutes, 0)} min em casa · ${daily.length} ${daily.length === 1 ? "dia" : "dias"} com dado` +
      (forming ? ` · janela em formação (${fmt(hist, hist < 10 ? 1 : 0)} de ${tr.days} dias)` : "");
  const fit = $("bgFit");
  const f = tr.fit || {};
  fit.hidden = !tr.minutes;
  fit.className = "bg-fit" + (f.state === "subindo" || f.state === "descendo" || f.state === "degrau" ? " warn" : "");
  fit.innerHTML = "<strong></strong><span></span>";
  fit.children[0].textContent = BG_STATE[f.state] || "";
  fit.children[1].textContent = f.text || "";

  // gráfico: média e mediana móveis, a média aparada de cada dia (pontos) e
  // a reta ou o degrau quando há aviso
  const svg = $("bgChart");
  let lo = Infinity, hi = -Infinity;
  for (const p of pts) for (const v of [p.mean, p.median]) if (v != null) { lo = Math.min(lo, v); hi = Math.max(hi, v); }
  for (const d of daily) if (d.full) { lo = Math.min(lo, d.trimmed); hi = Math.max(hi, d.trimmed); }
  const ys = $("bgY").children;
  if (!isFinite(lo)) { svg.innerHTML = ""; for (const e of ys) e.textContent = ""; $("bgLegend").textContent = ""; $("bgDisp").innerHTML = ""; for (const e of $("bgX").children) e.textContent = ""; return; }
  const pad = Math.max(1, (hi - lo) * 0.15);
  lo = Math.max(0, Math.floor(lo - pad)); hi = Math.ceil(hi + pad);
  const x = (t) => ((t - tr.from) / (tr.to - tr.from)) * W;
  const y = (v) => H - ((v - lo) / (hi - lo)) * H;
  for (let i = 0; i < 4; i++) ys[i].textContent = fmt(hi - ((hi - lo) / 3) * i, hi - lo < 6 ? 1 : 0);
  let out = [0, 1, 2, 3].map((i) => { const gy = i === 3 ? H - 0.5 : i === 0 ? 0.5 : (H / 3) * i; return `<line x1="0" y1="${gy}" x2="${W}" y2="${gy}" stroke="${COR.grid}" stroke-width="1" vector-effect="non-scaling-stroke"/>`; }).join("");
  if (forming) out += `<rect x="0" y="0" width="${Math.max(0, x(tr.first)).toFixed(1)}" height="${H}" fill="${COR.muted}" fill-opacity="0.06"/>`;
  const path = (val) => { let d = "", pen = "M"; for (const p of pts) { const v = val(p); if (v == null) { pen = "M"; continue; } d += `${pen}${x(p.t).toFixed(1)} ${y(v).toFixed(1)}`; pen = "L"; } return d; };
  out += `<path d="${path((p) => p.mean)}" fill="none" stroke="${COR.accent}" stroke-width="2" stroke-linejoin="round" vector-effect="non-scaling-stroke"/>`;
  out += `<path d="${path((p) => p.median)}" fill="none" stroke="#7DD3FC" stroke-width="1.4" stroke-dasharray="5 3" stroke-linejoin="round" vector-effect="non-scaling-stroke"/>`;
  for (const d of daily) if (d.full) out += `<circle cx="${x(d.day + DAY_MS / 2).toFixed(1)}" cy="${y(d.trimmed).toFixed(1)}" r="2.6" fill="${COR.text}" fill-opacity="0.75"/>`;
  if (f.state === "subindo" || f.state === "descendo") {
    out += `<line x1="${x(f.fitT0 + DAY_MS / 2).toFixed(1)}" y1="${y(f.fitFrom).toFixed(1)}" x2="${x(f.fitT1 + DAY_MS / 2).toFixed(1)}" y2="${y(f.fitTo).toFixed(1)}" stroke="${COR.att}" stroke-width="1.6" stroke-dasharray="6 4" vector-effect="non-scaling-stroke"/>`;
  } else if (f.state === "degrau") {
    out += `<path d="M${x(f.fitT0).toFixed(1)} ${y(f.stepFrom).toFixed(1)}H${x(f.stepAt).toFixed(1)}V${y(f.stepTo).toFixed(1)}H${x(f.fitT1 + DAY_MS).toFixed(1)}" fill="none" stroke="${COR.att}" stroke-width="1.6" stroke-dasharray="6 4" vector-effect="non-scaling-stroke"/>`;
  }
  svg.innerHTML = out;
  const lab = (t) => `${WEEKDAY[new Date(t).getDay()]} ${ddmm(t)}`;
  const xs = $("bgX").children;
  xs[0].textContent = lab(tr.from); xs[1].textContent = lab((tr.from + tr.to) / 2); xs[2].textContent = "agora";
  $("bgLegend").innerHTML = '<span class="key"><i style="border-color:' + COR.accent + '"></i>média móvel de 24 h</span><span class="key"><i style="border-color:#7DD3FC;border-top-style:dashed"></i>mediana móvel de 24 h</span>' +
    '<span class="key"><i class="dot-k"></i>média de cada dia (sem os 5% extremos)</span>' +
    (f.state === "subindo" || f.state === "descendo" || f.state === "degrau" ? '<span class="key"><i style="border-color:' + COR.att + ';border-top-style:dashed"></i>tendência</span>' : "") +
    (forming ? '<span class="key"><i class="sw" style="background:' + COR.muted + '"></i>antes do primeiro dado</span>' : "") + " (CPM)";

  // dispersão de cada dia
  const dsvg = $("bgDisp"), DH = 80;
  let dtop = 2;
  for (const d of daily) dtop = Math.max(dtop, Math.ceil(d.dispersion * 2) / 2);
  const dys = $("bgDY").children;
  dys[0].textContent = fmt(dtop, 1); dys[1].textContent = fmt(dtop / 2, 1); dys[2].textContent = "0";
  const bw = Math.max(2, (DAY_MS / (tr.to - tr.from)) * W * 0.7);
  let dout = `<line x1="0" y1="${(DH - DH / dtop).toFixed(1)}" x2="${W}" y2="${(DH - DH / dtop).toFixed(1)}" stroke="${COR.muted}" stroke-opacity="0.5" stroke-dasharray="3 3" vector-effect="non-scaling-stroke"/>`;
  for (const d of daily) {
    const h = (Math.min(dtop, d.dispersion) / dtop) * DH, cx = x(d.day + DAY_MS / 2);
    const col = d.dispersion >= 1.2 ? COR.att : COR.accent;
    dout += `<rect x="${(cx - bw / 2).toFixed(1)}" y="${(DH - h).toFixed(1)}" width="${bw.toFixed(1)}" height="${h.toFixed(1)}" fill="${col}" fill-opacity="${d.full ? 0.7 : 0.3}"/>`;
  }
  dsvg.innerHTML = dout;
}

function wireBgTrend() {
  state.bgDays = parseInt(localGet("bgDays") || "30", 10) || 30;
  document.querySelectorAll("#bgSeg button").forEach((b) => b.addEventListener("click", () => {
    state.bgDays = +b.dataset.bgdays; localSet("bgDays", String(state.bgDays)); refreshBgTrend();
  }));
  const plot = $("bgPlot");
  plot.addEventListener("mousemove", (ev) => {
    const tr = state.bgTrend;
    if (!tr || !tr.pts || !tr.pts.length) return;
    const b = plot.getBoundingClientRect(), f = (ev.clientX - b.left) / b.width;
    const t = tr.from + f * (tr.to - tr.from);
    let best = tr.pts[0];
    for (const p of tr.pts) if (Math.abs(p.t - t) < Math.abs(best.t - t)) best = p;
    $("bgCursor").hidden = false;
    $("bgCursor").style.left = (((best.t - tr.from) / (tr.to - tr.from)) * 100) + "%";
    const tip = $("bgTip");
    tip.hidden = false;
    tip.textContent = `24 h até ${dayLabel(best.t - 1)} ${hhmm(best.t)} · ` + (best.mean == null ? `poucos dados (${fmt(best.n, 0)} min)` : `média ${fmt(best.mean, 1)} · mediana ${fmt(best.median, 1)} CPM · n = ${fmt(best.n, 0)} min`);
    const left = ((best.t - tr.from) / (tr.to - tr.from)) * b.width;
    tip.style.left = Math.min(Math.max(0, left - tip.offsetWidth / 2), b.width - tip.offsetWidth) + "px";
  });
  plot.addEventListener("mouseleave", () => { $("bgCursor").hidden = true; $("bgTip").hidden = true; });
  const dplot = $("bgDPlot");
  dplot.addEventListener("mousemove", (ev) => {
    const tr = state.bgTrend;
    if (!tr || !tr.daily || !tr.daily.length) return;
    const b = dplot.getBoundingClientRect(), t = tr.from + ((ev.clientX - b.left) / b.width) * (tr.to - tr.from);
    const d = tr.daily.find((q) => t >= q.day && t < q.day + DAY_MS);
    const tip = $("bgDTip");
    if (!d) { tip.hidden = true; return; }
    tip.hidden = false;
    tip.textContent = `${dayLabel(d.day)} · dispersão ${fmt(d.dispersion, 2)} · σ real ${fmt(d.sd, 1)} ÷ acaso ${fmt(d.poissonSD, 1)} · mediana ${fmt(d.median, 1)} CPM · n = ${fmt(d.n, 0)} min`;
    const left = ((d.day + DAY_MS / 2 - tr.from) / (tr.to - tr.from)) * b.width;
    tip.style.left = Math.min(Math.max(0, left - tip.offsetWidth / 2), b.width - tip.offsetWidth) + "px";
  });
  dplot.addEventListener("mouseleave", () => { $("bgDTip").hidden = true; });
}

// ---------- relatórios em PDF ----------

// Enquanto o PDF é gerado, os botões de relatório mostram "Gerando…" e
// ficam travados: sem isso o clique parecia não ter feito nada, e cada
// clique a mais virava outro PDF.
const REPORT_BTNS = ["btnReport", "btnZoomReport", "btnMReportMake", "btnMdReport"];
let reportBusy = false;
function setReportBusy(on, btn) {
  reportBusy = on;
  const btns = REPORT_BTNS.map((id) => $(id)).concat(btn && !REPORT_BTNS.includes(btn.id) ? [btn] : []).filter(Boolean);
  for (const b of btns) {
    if (on) { b.dataset.idleText = b.textContent; b.textContent = "Gerando…"; }
    else if (b.dataset.idleText) { b.textContent = b.dataset.idleText; delete b.dataset.idleText; }
    b.disabled = on;
    b.classList.toggle("is-busy", on);
    b.setAttribute("aria-busy", on ? "true" : "false");
  }
}
async function runReport(btn, make) {
  if (reportBusy) return;
  setReportBusy(true, btn);
  try {
    const f = await make();
    if (f) toast("Relatório salvo: " + f.split(/[\\/]/).pop());
  } catch (e) { toast(errMsg(e), true); }
  finally { setReportBusy(false, btn); }
}

function exportMeasureReport(id, btn) {
  return runReport(btn, () => api.ExportMeasureReport(id));
}

function exportReport(from, to, label, btn) {
  return runReport(btn, () => api.ExportReport(Math.round(from), Math.round(to), label));
}

function renderReportDir(d) {
  d = d || {};
  $("reportDir").textContent = (d.dir || "—") + (d.default ? "  (padrão)" : "");
  $("btnResetReportDir").hidden = !!d.default;
}
async function refreshReportDir() { try { renderReportDir(await api.GetReportDir()); } catch (e) { /* segue */ } }

function wireReports() {
  $("btnMReportOpen").addEventListener("click", () => call(api.OpenReportFile, $("mReport").dataset.path));
  $("btnMReportMake").addEventListener("click", (e) => exportMeasureReport(+$("mReport").dataset.id, e.currentTarget));
  $("btnReportDir").addEventListener("click", async () => { try { renderReportDir(await api.ChooseReportDir()); } catch (e) { toast(errMsg(e), true); } });
  $("btnOpenReportDir").addEventListener("click", () => call(api.OpenReportDir));
  $("btnResetReportDir").addEventListener("click", async () => renderReportDir(await api.ResetReportDir()));
  $("btnZoomReport").addEventListener("click", (e) => {
    if (!state.zoom) return;
    const p = state.heatPick && state.heatPick.from === state.zoom.from && state.heatPick.to === state.zoom.to;
    exportReport(state.zoom.from, state.zoom.to, p ? "padrão por hora" : "trecho selecionado", e.currentTarget);
  });
}

function setupSummary() {
  $("btnReport").addEventListener("click", (e) => { const [from, to, label] = sumPeriods()[0]; exportReport(from, to, label, e.currentTarget); });
  document.querySelectorAll("#sumModeSeg button").forEach((b) => b.addEventListener("click", () => { state.sum.mode = b.dataset.smode; refreshSummary(); }));
  // "detalhes" do Resumo lembra se estava aberto
  $("sumDetails").open = localGet("sumDet", "") === "1";
  $("sumDetails").addEventListener("toggle", (e) => localSet("sumDet", e.target.open ? "1" : ""));
  document.querySelectorAll("#sumPresetSeg button").forEach((b) => b.addEventListener("click", () => { state.sum.preset = b.dataset.sp; state.sum.day = 0; refreshSummary(); }));
  document.querySelectorAll("#sumCmpSeg button").forEach((b) => b.addEventListener("click", () => { state.sum.cmp = b.dataset.sc; refreshSummary(); }));
  $("sumDay").addEventListener("change", (e) => { state.sum.day = e.target.value ? parseDateVal(e.target.value) : 0; refreshSummary(); });
  $("sumDayA").addEventListener("change", (e) => { if (e.target.value) { state.sum.dayA = parseDateVal(e.target.value); refreshSummary(); } });
  $("sumDayB").addEventListener("change", (e) => { if (e.target.value) { state.sum.dayB = parseDateVal(e.target.value); refreshSummary(); } });
  const plot = $("sumPlot");
  plot.addEventListener("mousemove", (ev) => {
    const ser = state.sumSeries;
    if (!ser || !ser.length) return;
    const n = Math.max(...ser.map((s) => s.r.curve.length));
    const b = plot.getBoundingClientRect();
    const i = Math.max(0, Math.min(n - 1, Math.round(((ev.clientX - b.left) / b.width) * (n - 1))));
    $("sumCursor").hidden = false;
    $("sumCursor").style.left = (i / Math.max(1, n - 1)) * 100 + "%";
    const long = ser[0].r.to - ser[0].r.from > DAY_MS * 1.1;
    const parts = ser.map((s) => {
      const p = s.r.curve[i];
      if (!p) return "";
      const when = long ? `${WEEKDAY[new Date(p.t).getDay()]} ${ddmm(p.t)} ${hhmm(p.t)}` : hhmm(p.t);
      const rc = ser.length === 1 && s.r.rc && s.r.rc.minutes ? s.r.rc.curve[i] : null;
      const gu = p.usv != null ? p.usv : p.cpm != null ? p.cpm * calK() : null;
      if (rc) return `${when}: RC102 ${rc.usv == null ? "sem dado" : usvTxt(rc.usv)} · GMC-800 ${gu == null ? "sem dado" : usvTxt(gu)}`;
      return `${ser.length > 1 ? s.label + " " : ""}${when}: ${gu == null ? "sem dado" : usvTxt(gu)}`;
    }).filter(Boolean);
    const tip = $("sumTip");
    tip.hidden = false;
    tip.textContent = parts.join(" · ");
    tip.style.left = Math.max(0, Math.min(ev.clientX - b.left + 12, b.width - tip.offsetWidth - 4)) + "px";
  });
  plot.addEventListener("mouseleave", () => { $("sumCursor").hidden = true; $("sumTip").hidden = true; });
}

function setupRainImpact() {
  const plot = $("riPlot");
  plot.addEventListener("mousemove", (ev) => {
    const r = state.ri;
    if (!r || !r.curve || !r.curve.length) return;
    const b = plot.getBoundingClientRect();
    const m = ((ev.clientX - b.left) / b.width) * 360 - 120;
    const p = r.curve.reduce((a, q) => (Math.abs(q.min - m) < Math.abs(a.min - m) ? q : a), r.curve[0]);
    $("riCursor").hidden = false;
    $("riCursor").style.left = ((p.min + 120) / 360) * 100 + "%";
    const tip = $("riTip");
    tip.hidden = false;
    const when = p.min < 0 ? `${durMin(-p.min)} antes` : `${durMin(p.min)} depois`;
    const pc = r.device === "radiacode" && r.avgBase > 0 ? 100 / r.avgBase : 0;
    const v = (x) => (x == null ? "—" : pc ? sgn(x * pc, 1) + "%" : sgn(x, 1));
    tip.textContent = `${when} · chuva ${v(p.rain)} · sem chuva ${v(p.dry)}` + (pc ? "" : " CPM");
    tip.style.left = Math.max(0, Math.min(ev.clientX - b.left + 12, b.width - tip.offsetWidth - 4)) + "px";
  });
  plot.addEventListener("mouseleave", () => { $("riCursor").hidden = true; $("riTip").hidden = true; });
}

async function init() {
  wire();
  listen();
  try {
    state.settings = await api.GetSettings();
    if (hasGo) { syncLang(state.settings.lang); if (api.SetUILang) api.SetUILang(LANG).catch(() => {}); }
    else state.settings.lang = LANG_PREF; // modo navegador: vale o que está no navegador
    state.status = await api.GetStatus();
    state.demo = await api.GetDemo();
  } catch (e) { /* segue com o padrão */ }
  renderDemo();
  $("chkAlarm").checked = !!state.settings.alarmEnabled;
  renderAlarmLabel();
  renderStatus();
  try { const v = await api.GetVersion(); $("appVersion").textContent = /^\d/.test(v) ? "v" + v : v; } catch (e) { /* sem versão */ }
  try {
    state.platform = api.GetPlatform ? await api.GetPlatform() : "windows";
    // bandeja e iniciar com o sistema só existem no Windows
    if (state.platform !== "windows") $("chkTray").closest("fieldset").hidden = true;
  } catch (e) { /* segue */ }
  await refreshRainStatus();
  await refreshPeriods();
  await refreshRcAway();
  await refreshWhere();
  await refreshMeasurements();
  await refreshAll();
  refreshAgora(true);
  if (!hasGo) toast("Modo demonstração: abra pelo app para usar o contador de verdade.");
  // Garante que o gráfico de 7 dias e as datas "hoje/ontem" virem o dia.
  setInterval(refreshAll, 5 * 60 * 1000);
  // os gráficos de 15 e 30 min andam junto com as leituras por segundo
  setInterval(() => { if (isShortRange() && !state.zoom) refreshChart(); }, 5000);
  refreshSpark();
  setInterval(refreshSpark, 30e3);
}

init();

// ---------- modo demonstração (só no navegador, sem o Go) ----------

var demoBus;
function makeDemoApi() {
  let demoSync = { id: "a", name: "ACENULL", listen: false, listening: false, auto: true, peers: [
    { id: "b", name: "parrot", addrs: ["192.168.0.20:47862", "100.88.1.7:47862"], manualAddrs: [], lastSync: Date.now() - 4 * 6e4, lastTry: Date.now() - 4 * 6e4, lastResult: "trouxe: 12 minutos · levou: 1.440 minutos, 2 alertas" }] };
  let demoUpd = { current: "1.17.1", latest: "1.18.0", available: true, canAuto: true, url: "https://github.com/matheusAFreitas/radlog/releases",
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
  let settings = { levels: {}, port: "COM5", autoConnect: true, alarmEnabled: true, alarmCPM: 100, alarmSound: "sirene", alarmRaise: true };
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
    const nh = !!(o && o.notHome && g === "casa" && r === "casa");
    const pers = r === "comigo" ? "radiacode" : g === "comigo" ? "gmc" : g === "casa" && r === "casa" && !nh ? "gmc" : "", home = g === "casa" ? "gmc" : r === "casa" ? "radiacode" : "";
    return { gmc: g, radiacode: r, normal: g === "casa" && r === "comigo", personal: pers, home, notHome: nh, since: o ? o.from : 0, open: o && !o.to ? o : null, live: mockLive() };
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
  let periods = [{ id: 1, name: "Passeio no parque", from: Math.floor((now - 2 * 864e5) / 36e5) * 36e5 + 2 * 36e5, to: Math.floor((now - 2 * 864e5) / 36e5) * 36e5 + 5.5 * 36e5 }];
  const rcY = Math.floor((now - 864e5) / 6e4) * 6e4 - 3 * 36e5;
  const rcAsksAll = [{ id: rcY, from: rcY, to: rcY + 130 * 6e4, memMinutes: 128, avgUSv: 0.142, maxUSv: 0.231, maxAt: rcY + 57 * 6e4, doseUSv: 0.30, gmcUSv: 0.098, ratio: 41, normalRatio: 28, hint: "fora" }];
  let rcAsks = rcAsksAll.slice();
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
      emit("reading", { time: Date.now(), cpm: c, cps: Math.round(c / 60), usv: c * 0.0101, mrh: c * 0.00101, level: c >= dl ? "perigo" : c >= 100 ? "alarme" : c >= 50 ? "atencao" : "seguro", alarm: settings.alarmEnabled && c >= settings.alarmCPM, danger: c >= dl, saturated: c * 0.0101 >= 2000, satReason: "limite", rise: c >= dl ? { fromUSv: 0.13, seconds: 3 } : null, demo: true, attention: c >= al, bgCPM: 20, sigma: (c - 20) / Math.sqrt(20) });
      feed(Math.max(0, Math.round(c / 60 + (Math.random() - 0.5) * 2)));
      emit("realReading", { time: Date.now(), cpm: 21, cps: 0, usv: 0.137, mrh: 0.0137, level: "seguro", alarm: false });
      return;
    }
    if (status.state !== "conectado") return;
    const cpm = Math.round(20 + (Math.random() - 0.5) * 12);
    emit("reading", { time: Date.now(), cpm, cps: Math.round(cpm / 60), usv: cpm * 0.0101, mrh: cpm * 0.00101, level: cpm >= 100 ? "alarme" : cpm >= 50 ? "atencao" : "seguro", alarm: false, attention: cpm >= 50, bgCPM: 20, sigma: (cpm - 20) / Math.sqrt(20) });
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
      if (meas.auto) {
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
        if (stop === "nada") { r.level = "nada"; r.verdict = `igual ao fundo: se o objeto emite algo, é menos que +${fmt(r.limit, 1)} CPM`; }
        if (stop === "limite" && r.level !== "sim") r.verdict += " (chegou ao tempo máximo sem decidir)";
        if (meas.auto) { r.pct = meas.pct; r.threshold = thrOf(meas.bg, meas.pct); }
        meas.result = r; meas.live = null; meas.eta = -1;
        if (meas.device === "radiacode") meas.spectrum = demoMeasSpec(meas.elapsed, meas.bg.source === "casa");
        if (endedPh === "objeto") endSide("objeto");
      }
    }
    emit("measure", { ...meas });
  }
  // Radiacode medindo no modo navegador: fundo ~12 cps, objeto ~19 cps
  setInterval(() => {
    const rcIn = meas.device === "radiacode" || (meas.other && meas.other.device === "radiacode");
    if (!rcIn || !demoRcFake() || (meas.phase !== "fundo" && meas.phase !== "objeto")) return;
    const lam = meas.other && meas.other.device === "radiacode" && meas.other.role === "voce" ? 14 : meas.phase === "objeto" ? 13.5 : 12;
    let k = 0, p = 1; const L = Math.exp(-lam); for (;;) { p *= Math.random(); if (p <= L) break; k++; }
    emit("radiacode", { ...demoRc(), cps: lam + (Math.random() - 0.5) * 2 });
    feed(k, "radiacode");
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
  const demoRcCfg = { off: false, ble: "", raw: true };
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
    : { state: demoRcCfg.off ? "off" : "procurando", ble: demoRcCfg.ble, bleOK: true, off: demoRcCfg.off, raw: demoRcCfg.raw };
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
    SetRcRaw: async (on) => { demoRcCfg.raw = on; return demoRc(); },
    ScanRadiacodeBLE: async () => { await new Promise((r) => setTimeout(r, 600)); return [{ address: "52:43:06:50:01:D7", name: "RadiaCode-102#RC-102-001234", rssi: -62 }]; },
    GetRcCompare: async (min) => {
      if (!demoRcFake()) return { minutes: 0, rcOnly: 0 };
      const n = Math.min(min, 600), t = [], gmc = [], rc = [];
      for (let i = 0; i < n; i++) { const g = 0.1 + 0.02 * Math.sin(i / 60) + (Math.random() - 0.5) * 0.04; t.push(Date.now() - (n - i) * 60e3); gmc.push(g); rc.push(g * 0.92 + (Math.random() - 0.5) * 0.02); }
      return { t, gmc, rc, gmcCPM: gmc.map((v) => v * 150), rcCPS: rc.map((v) => v * 30), ratio: 0.92, ratioErr: 0.01, corr: 0.81, minutes: n, rcOnly: 0, suggestK: 0.0055, curK: 0.01014 };
    },
    GetRcSpectrum: async (accum) => {
      if (!demoRcFake()) throw new Error("RC102 não conectado");
      // espectro de mentira: fundo (acumulado, 7 h) e o atual, que vai
      // juntando com um eletrodo de tório encostado
      const A = [5.43, 2.351, 0.000381], En = (i) => A[0] + A[1] * (i + 0.5) + A[2] * (i + 0.5) ** 2;
      const rate = (i, th) => {
        const e = En(i); if (e < 20) return 0;
        const pk = (pe, r) => { const sg = 65 * Math.sqrt(pe / 661.7) / 2.355; return r * 2.35 / (sg * 2.5066) * Math.exp(-(((e - pe) / sg) ** 2) / 2); };
        let v = 0.035 * Math.exp(-e / 350) + pk(1460.8, 0.08) + pk(609.3, 0.06) + pk(351.9, 0.06);
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
    AnalyzeSpectrum: async (sp) => (sp.objSec > 20 ? [
      { group: "Tório-232 (série)", nuclide: "Pb-212", kev: 238.6, net: 300, sigma: 12 }, { group: "Tório-232 (série)", nuclide: "Tl-208", kev: 583.2, net: 90, sigma: 6 },
      { group: "Tório-232 (série)", nuclide: "Tl-208", kev: 2614.5, net: 30, sigma: 4 }] : []),
    SetSyncListen: async (on) => (demoSync = { ...demoSync, listen: on, listening: on, addrs: on ? ["192.168.0.10:47862", "100.101.102.103:47862"] : [] }),
    SetSyncAuto: async (on) => (demoSync = { ...demoSync, auto: on }),
    ShowPairCode: async () => (demoSync = { ...demoSync, code: "482913", codeUntil: Date.now() + 300e3 }),
    CancelPairCode: async () => (demoSync = { ...demoSync, code: "" }),
    DiscoverPeers: async () => { await new Promise((r) => setTimeout(r, 800)); return [{ id: "b", name: "parrot", addr: "192.168.0.20:47862", paired: false }]; },
    PairWith: async () => { throw new Error("no modo navegador não dá para parear; abra pelo app"); },
    SyncNow: async () => ({}),
    SetPeerAddrs: async () => demoSync,
    ForgetPeer: async () => demoSync,
    GetUpdateInfo: async () => demoUpd,
    CheckUpdate: async () => { demoUpd = { ...demoUpd, latest: "1.18.0", available: true, checkedAt: Date.now() }; emit("update", demoUpd); return demoUpd; },
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
    StartMeasurePhase: async (phase, secs, name) => {
      if (phase === "fundo") meas = { phase, seconds: secs || 60, elapsed: 0, counts: 0, name, demo: demoSt.active, bg: null, obj: null, result: null, eta: -1 };
      else { if (!meas.bg) throw new Error("meça o fundo primeiro"); Object.assign(meas, { phase, elapsed: 0, counts: 0, obj: null, result: null, live: null, eta: -1, name: name || meas.name }); if (meas.auto) meas.seconds = meas.maxSec; }
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
    ImportBackup: async () => ({ from: "LINUX-PC", created: "", minutes: 340, events: 3, periods: 1, measurements: 0, alertFiles: 1 }),
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
      if (ans === "fora") p = await api.SavePeriod({ id: 0, from: from || q.from, to: to || q.to, name, gmc: "casa", radiacode: "comigo", notHome: true });
      rcAsks = rcAsks.filter((x) => x.id !== id);
      setTimeout(() => emit("rcaway", rcAsks.slice()), 0);
      return p;
    },
    UndoRcAway: async (ask, pid) => {
      if (pid) await api.DeletePeriod(pid);
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
    SetWhere: async (g, r, name) => {
      const now = Math.floor(Date.now() / 6e4) * 6e4;
      const cur = mockWhere();
      if (cur.gmc === g && cur.radiacode === r) return cur;
      periods = periods.filter((q) => !(q.to === 0 && now - q.from < 6e4)).map((q) => (q.to === 0 ? { ...q, to: now } : q));
      if (!(g === "casa" && r === "comigo")) periods.push({ id: Math.max(0, ...periods.map((q) => q.id)) + 1, from: now, to: 0, name: (name || "").trim() || whKind({ gmc: g, radiacode: r }), gmc: g, radiacode: r });
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
