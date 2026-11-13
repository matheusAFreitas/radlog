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
  seguro: "", // "seguro" já diz tudo (ACE, 11/11)
  atencao: "acima do limite de atenção",
  alarme: "passou o limite de alarme",
  perigo: "acima do limite de perigo",
  nocount: "o aparelho não está mandando contagens",
};
function tileState(pfx, lvl, word, say) {
  const el = $(pfx + "Tile");
  TILE_LV.forEach((l) => el.classList.toggle("lv-" + l, lvl === l));
  lvLabel($(pfx + "TileState"), lvl, word, 18);
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
  // mrem/h pequeno embaixo do mostrador (ACE 11/11); só quando o grande é µSv/h
  $("gTileAlt").textContent = v && u === "usv" ? `${v.mrh} mrem/h` : "";
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
  // mrem/h pequeno embaixo do mostrador, igual ao do GMC-800 (ACE 11/11)
  $("rTileAlt").textContent = v && u === "usv" ? `${v.mrh} mrem/h` : "";
  $("rTileVal").closest(".inst-read").title = !o.live && st.lastT ? `última leitura: ${whenLabel(st.lastT)}` : "";
  // marcas da régua na unidade do número (CPM/CPS pela relação de agora)
  const perUSv = o.usv > 0 && o.cps > 0 ? o.cps / o.usv : null;
  const tick = (x) => (u === "mrh" ? fmt(x / 10, 3) : u === "cps" && perUSv ? fmt(x * perUSv, 1) : u === "cpm" && perUSv ? fmt(x * perUSv * 60, 0) : fmt(x, 2));
  const names = o.own ? { seguro: "Seguro", atencao: "Alarme 1", alarme: "Alarme 2", perigo: "Perigo" } : LV_WORD;
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

// alertas ativos: a marca no menu (#navAlerts) e no topo compacto (#miniAlert);
// o quadro "Alertas" do topo saiu na 2.73 (a faixa de cima já avisa)
function renderAlertMarks() {
  const list = state.activeAlerts || [];
  const rank = { atencao: 1, alarme: 2, perigo: 3 };
  const dev = (e) => (isRcEv(e) ? "RC102" : "GMC-800");
  const top = list.slice().sort((a, b) => (rank[lvlCls(b)] || 1) - (rank[lvlCls(a)] || 1) || a.time - b.time)[0];
  const srcs = new Set(list.map((e) => dev(e) + (e.demo ? " demo" : "")));
  const lv = top ? { alarme: "alarme", perigo: "perigo" }[lvlCls(top)] || "atencao" : "";
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
  if (!$(id)) return;
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
  $("heroDose").title = s.minutesToday ? `Hoje: ${doseKnown(s.doseToday, homeBgUSv()).join(" · ")} · calculada pelo app (o GMC não informa a dose que soma)` : "";
}

// Fundo por dia (Gráficos › Tendência do fundo, 2.65): 14 dias e "Ver mais"
let dailyN = 14;
async function refreshDaily() {
  // GetDaily conta dias do calendário (até 60): pede um a mais para saber se tem mais
  const days = await api.GetDaily(Math.min(60, dailyN + 1));
  const tb = $("days");
  tb.textContent = "";
  $("btnDaysMore").hidden = dailyN >= 60 || !days || !days.some((d) => d.day < startDay(Date.now()) - (dailyN - 1) * 864e5);
  if (!days || !days.length) {
    tb.innerHTML = '<tr><td colspan="3" class="muted">Sem leituras guardadas.</td></tr>';
    return;
  }
  for (const d of days.slice(0, dailyN)) {
    const tr = document.createElement("tr");
    const label = isToday(d.day) ? "hoje" : isYesterday(d.day) ? "ontem" : ddmm(d.day);
    for (const [txt, cls] of [[label, ""], [fmt(d.usv, 3), "num"], [fmt(d.cpm, 1), "num"]]) {
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
const PAGES = { agora: "pageAgora", graficos: "pageGraficos", resumo: "pageResumo", alara: "pageAlara", alertas: "pageAlertas", medir: "pageMedir", fora: "pageFora", chuva: "pageChuva", gmc: "pageGmc", radiacode: "pageRadiacode", config: "pageConfig" };

async function showView(v, skipRender) {
  if (v === "dose") { setSub("graficos", "tempo"); v = "graficos"; } // a Dose virou sub-aba de Gráficos
  if (!PAGES[v]) v = "graficos";
  if (v !== "alertas") state.alertRange = null; // o filtro de período vale só na ida pelo Resumo
  const changed = state.view !== v;
  state.view = v;
  qkTrack(v);
  if (changed) usoView(v);
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
  if (v === "fora") { renderPeriods(); refreshPlanned(); }
  if (v === "chuva") refreshRainImpact(true);
  if (v === "resumo") refreshSummary();
  if (v === "alara") refreshAlara();
  if (v === "medir") { renderMeasure(await api.GetMeasure()); setMeasureMode(state.mMode); }
  if (v === "radiacode") { if (api.GetRadiacode) renderRc(await api.GetRadiacode()); refreshRc24(); }
  if (v === "config") openSettings();
  if (v === "agora") refreshAgora();
}

// sub-abas: Gráficos e Radiacode mostram uma parte de cada vez (a última
// escolhida volta quando abre o app)
const SUBS = { graficos: ["tempo", "hora", "padroes", "tendencia", "ano"], radiacode: ["agora", "espectro", "busca", "atividade", "log"], gmc: ["agora", "tubo"] };
state.sub = {};
// 2.51: o espectrograma virou "Ao longo do tempo" dentro do Espectro
if (localGet("sub.radiacode", "") === "espectrograma") { localSet("sub.radiacode", "espectro"); localSet("rcSpecView", "tempo"); }
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
  if (changed) usoSub(pg, sub);
  if (changed && state.view === pg) refreshSub(pg);
  // fora da aba Agora a fileira de cima volta a mostrar os dois aparelhos (2.65)
  if (DEV_PAGES[pg]) renderDevMini();
}

function refreshSub(pg) {
  const s = state.sub[pg];
  if (pg === "graficos") {
    if (s === "tempo") { refreshChart(); refreshDose(); }
    else if (s === "hora") refreshHeat();
    else if (s === "padroes") refreshPatterns();
    else if (s === "ano") refreshYearCal();
    else refreshBgTrend();
  }
  // os gráficos do Radiacode são canvas: redesenha com o tamanho de agora
  if (pg === "radiacode" && window.rcDemo && window.rcDemo.redraw) requestAnimationFrame(() => window.rcDemo.redraw());
  if (pg === "radiacode" && s === "agora") refreshRc24();
  if (pg === "gmc" && s === "tubo" && typeof tbLoad === "function" && !TB.h) tbLoad();
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
  if (!b) return;
  // o quadro do aparelho da própria página (fora da aba Agora) volta para a aba Agora
  if (b.dataset.view === state.view && SUBS[state.view]) setSub(state.view, "agora");
  showView(b.dataset.view);
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
  // o próprio aparelho avisou "fora da escala", ou passou do fim da escala configurado
  const rcSat = kind === "rc" && (!!rcs.offscale || rcs.usvh >= lv().rcSatUSv);
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
  if (L.id === "data") renderMaturity();
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
// Princípios de proteção radiológica: tempo, distância e blindagem. Contato
// da ANSN, que assumiu a fiscalização da CNEN (gov.br/ansn, "Emergência
// radiológica", modificada em 21/08/2026): plantão 24 h (21) 99304-9453,
// coordenador do plantão (21) 99299-2620. Em inglês a faixa diz "no Brasil"
// e dá os números gerais de emergência (auditoria de 12/11, R3 e R4).

// Textos curtos: dá para ler em segundos, no meio de um alarme.
const PHONES = "ANSN 24 h: (21) 99304-9453 · Bombeiros: 193";

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
    let r;
    try { r = await api.ImportBackup(); if (r && (r.from || r.created)) toast(`Backup de ${r.from || "outro PC"}: ${mergeTextJS(r)}`); } catch (e) { toast(errMsg(e), true); return; }
    if (r && r.settings > 0 && api.ApplyBackupSettings) askBackupSettings(r);
  });
}

// configurações do backup (2.74): o backup traz as que valem em qualquer
// PC; só mudam aqui se a pessoa escolher
async function askBackupSettings(r) {
  const n = r.settings, from = r.from || "outro PC";
  const text = [
    n === 1 ? `O backup de ${from} tem 1 ajuste diferente do daqui.` : `O backup de ${from} tem ${n} ajustes diferentes dos daqui.`,
    "Vêm do backup: alarme, níveis, som, bandeja, pulso, idioma, atualização, conexão automática e medição. O lugar da chuva e o que é só deste PC (porta, Bluetooth, pastas, tamanho da tela, início com o Windows) continuam como estão.",
  ];
  if (!(await askConfirm({ title: "Usar as configurações do backup?", text, ok: "Usar as do backup" }))) return;
  try {
    state.settings = await api.ApplyBackupSettings();
    toast("Configurações do backup aplicadas");
    syncLang(state.settings.lang);
    $("chkAlarm").checked = !!state.settings.alarmEnabled;
    renderAlarmLabel();
    if (state.view === "config") openSettings("btnImportBackup");
  } catch (e) { toast(errMsg(e), true); }
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
  refreshUso();
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
  uiSegSet(s.uiScale || 1, s.uiDensity || "");
  applyUi(s.uiScale, s.uiDensity);
  $("chkTrayNum").checked = !s.trayPlain;
  $("selTrayDev").value = s.trayDevice || "";
  $("selTrayDev").disabled = !!s.trayPlain;
  $("chkPulse").checked = !s.pulseOff;
  $("chkPulseStrip").checked = !!s.pulseStrip;
  $("selPulseLogo").value = s.pulseLogo || "";
  PU.preview = null; pulseApply();
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
const SET_SAVE_IDS = () => ["chkAuto", "numAlarm", "selSound", "chkRaise", "chkAttn", "chkTray", "chkAutostart", "chkTrayNum", "selTrayDev", "chkPulse", "chkPulseStrip", "selPulseLogo", "valUiScale", "valUiDensity", "chkUpdCheck", "chkCleanOld", "selLang", "chkMeasReport", "numAwayAsk", "lvDTOn", ...Object.keys(LV_FIELDS)];
// nomes curtos na barra de mudanças
const SET_FIELD_NAMES = { chkAuto: "conectar sozinho", numAlarm: "limite do alarme", selSound: "som do alarme", chkRaise: "trazer a janela para a frente", chkAttn: "aviso de atenção",
  chkTray: "bandeja ao fechar", chkAutostart: "iniciar com o Windows", chkTrayNum: "leitura no ícone", valUiScale: "tamanho", valUiDensity: "espaço", selTrayDev: "aparelho no ícone", chkPulse: "piscar a cada contagem", chkPulseStrip: "fita dos últimos 10 segundos", selPulseLogo: "aparelho do logo", chkUpdCheck: "procurar versão nova", chkCleanOld: "apagar versões antigas", selLang: "idioma",
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
  $("chkTrayNum").addEventListener("change", () => { $("selTrayDev").disabled = !$("chkTrayNum").checked; });
  wireUiScale();
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
      trayPlain: !$("chkTrayNum").checked,
      uiScale: parseFloat($("valUiScale").value) || 1,
      uiDensity: $("valUiDensity").value,
      trayDevice: $("selTrayDev").value,
      pulseOff: !$("chkPulse").checked,
      pulseStrip: $("chkPulseStrip").checked,
      pulseLogo: $("selPulseLogo").value,
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
  wirePlanned();
  wireTrip();
  wireReportDlg();
  wireWhere();
  wireBusca();
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
    pulseGmc(r);
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
  on("periods", (list) => { const prev = state.periods; state.periods = normPeriods(list); tripCheckClosed(prev, state.periods); renderPeriods(); refreshAll(); });
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
  refreshPressure(force);
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
  // o aviso de conexão fica no cartão de cima; aqui (sem o cartão) só o tipo e o último contato
  $("rcState").textContent = on ? `conectado · ${via}` : [st.via ? via : "", st.lastT ? `último contato ${isToday(st.lastT) ? hhmm(st.lastT) : whenLabel(st.lastT)}` : st.state === "conectando" ? "conectando…" : st.state === "procurando" ? "procurando…" : "desconectado"].filter(Boolean).join(" · ");
  $("rcState").className = "rc-state mono small " + (on ? "ok" : "");
  $("rcEyebrow").title = on ? [st.model, st.serial && "série " + st.serial, st.firmware && "firmware " + st.firmware].filter(Boolean).join(" · ") : "";
  $("rcHelp").textContent = on ? "" : st.state === "erro" ? st.err
    : st.state === "off" ? "A procura está desligada em Configurações → Radiacode 102."
    : "Ligue o RC102 no USB do PC" + (st.bleOK ? (st.ble ? ", ou deixe-o perto com o Bluetooth ligado (e o app do celular desconectado)." : ", ou configure o endereço Bluetooth.") : ". No Windows, por enquanto, a conexão é só pelo USB.");
  $("rcHelp").className = "small muted";
  // com o cartão de cima à vista, o motivo e as ações estão no aviso de lá
  if (!on && (st.state === "conectando" || st.lastT > 0)) $("rcHelp").textContent = "";
  if (on && (!prev || prev.state !== "conectado")) { refreshRcCompare(); refreshChart(); refreshDose(); refreshRc24(); }
  // Radiacode como aparelho principal: na primeira vez, o gráfico vai para µSv/h
  if (hasRcData() && !localGet("rcPrimaryUnit", "")) {
    localSet("rcPrimaryUnit", "1");
    if (state.chartUnit !== "usv") { state.chartUnit = "usv"; localSet("chartUnit", "usv"); refreshChart(); }
  }
  if (hasRcData() && prev && !(prev.state === "conectado" || prev.lastT > 0)) refreshDose();
  renderRcMain(st);
  // configurações
  $("chkRcOn").checked = !st.off;
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
// desde a 2.51 o RC102 mostra só µSv/h em destaque (CPM e cps na linha de baixo)
state.rcUnit = "usv";

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
  $("rcNone").hidden = show;
  $("rc24Card").hidden = !show;
  $("rTile").hidden = !show;
  document.body.classList.toggle("has-rc", show);
  renderHeroVia();
  if (!show) return;
  const via = st.demo ? "simulado" : st.via === "bluetooth" ? "Bluetooth" : "USB";
  $("rcMainVia").textContent = st.state === "conectado" ? "" : st.state === "conectando" ? "· conectando…" : "· desconectado";
  $("rcMainVia").className = "rc-main-via mono" + (st.state === "conectado" ? " on" : "");
  renderRcChips(st, via);
  $("rcMain").classList.toggle("stale", !live);
  const sx = st.stats || {};
  // valor (ao vivo ou o último gravado, apagado)
  const usv = live ? st.usvh : st.lastT ? st.lastUSv : null;
  const cps = live ? st.cps : st.lastT ? st.lastCPS : null;
  const u = state.rcUnit;
  $("rcMainUnit").textContent = { usv: "µSv/h", mrh: "mrem/h", cps: "CPS", cpm: "CPM" }[u];
  const v = usv == null ? null : { usv, mrh: usv / 10, cps, cpm: cps * 60 }[u];
  $("rcMainVal").textContent = v == null ? "—" : u === "usv" ? fmt(v, usvDigits(v)) : u === "mrh" ? fmt(v, usvDigits(v * 10) + 1) : u === "cps" ? fmt(v, 2) : fmt(v, 0);
  const mrem = u === "usv" && usv != null ? `${fmt(usv / 10, usvDigits(usv) + 1)} mrem/h · ` : "";
  $("rcMainAlt").textContent = live ? `${mrem}${fmt(cps * 60, 0)} CPM · ${fmt(cps, 1)} cps · ± ${fmt(st.usvhErr, 0)}%` : st.lastT ? `última leitura ${isToday(st.lastT) ? "às " + hhmm(st.lastT) : "(" + whenLabel(st.lastT) + ")"}` : "sem leituras ainda";
  // dose de hoje
  if (st.todayDevOK) {
    $("rcDoseToday").textContent = fmt(st.todayDev, 2);
    rcParts($("rcDoseTodaySub"), ["desde 00:00", rcAwayToday(), sx.todayMinutes || st.today ? `app ${fmt(st.today, 2)}` : ""]);
    const chk = $("rcDoseTodaySub").lastElementChild;
    if (chk && (sx.todayMinutes || st.today)) chk.className = "dose-chk";
    $("rcDoseToday").title = `Somada pelo próprio RC102, também longe do PC. Pelo app (só os minutos gravados): ${fmt(st.today, 2)} µSv`;
    $("rcDoseTodaySub").title = "O número grande é o do RC102. \"app\" é a soma que o app fez só com os minutos gravados, para conferir: costuma ficar um pouco abaixo, porque o app não vê o tempo longe do PC.";
  } else {
    $("rcDoseToday").textContent = sx.todayMinutes || st.today ? fmt(st.today, 2) : "—";
    rcParts($("rcDoseTodaySub"), [sx.todayMinutes ? `${sx.todayMinutes} min medidos` : "desde 00:00", rcAwayToday()]);
    $("rcDoseToday").title = "Calculada pelo app: só os minutos que o PC gravou";
    $("rcDoseTodaySub").title = "";
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
  const lvl = usv == null ? "" : st.dangerUSv && usv >= st.dangerUSv ? "perigo" : usv >= alarm ? "alarme" : usv >= attn ? "atencao" : "seguro";
  lvLabel($("rcLevelName"), lvl, (own ? { seguro: "Seguro", atencao: "Alarme 1", alarme: "Alarme 2", perigo: "Perigo" } : { seguro: "Seguro", atencao: "Atenção", alarme: "Alarme", perigo: "Perigo" })[lvl] || "—", 16);
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
  // dose que o próprio aparelho soma desde que foi zerado
  if (st.doseLiveAt) {
    const mSv = st.doseLive >= 1000, v = mSv ? st.doseLive / 1000 : st.doseLive;
    $("rcDevDose").textContent = fmt(v, v < 10 ? (mSv ? 2 : 3) : 2);
    $("rcDevDoseSub").textContent = `${mSv ? "mSv" : "µSv"} desde que foi zerada`;
    $("rcDevDose").title = st.doseLiveSec ? `em ${rcDur(st.doseLiveSec)} contando` + (st.doseLiveOn ? "" : ` · às ${hhmm(st.doseLiveAt)}`) : "";
  } else { $("rcDevDose").textContent = "—"; $("rcDevDoseSub").textContent = "desde que foi zerada"; }
}

// etiquetas ao lado do nome: conexão, onde está, bateria e temperatura
function renderRcChips(st, via) {
  const box = $("rcChips"), on = st.state === "conectado";
  const where = { casa: "em casa", comigo: "comigo", fora: "fora do cálculo" }[((state.where || {}).radiacode) || "comigo"];
  const chips = [];
  if (on) chips.push(["on", via]);
  chips.push(["", where]);
  if (on && st.hasRare) chips.push(["", `bateria ${fmt(st.battery, 0)}%`], ["", `${fmt(st.tempC, 0)} °C`]);
  const key = chips.map((c) => c.join(":")).join("|");
  if (box.dataset.k === key) return;
  box.dataset.k = key;
  box.textContent = "";
  for (const [cls, text] of chips) {
    const s = document.createElement("span");
    s.className = "chip rc-chip" + (cls ? " " + cls : "");
    s.textContent = text;
    box.appendChild(s);
  }
}

// cada parte num span (a tradução não atravessa o " · ")
function rcParts(el, parts) {
  el.textContent = "";
  parts.filter(Boolean).forEach((t, i) => {
    if (i) el.append(" · ");
    const s = document.createElement("span");
    s.textContent = t;
    el.appendChild(s);
  });
}

// quanto tempo o RC102 passou fora de casa hoje ("1 h 20 fora de casa")
function rcAwayToday() {
  const t0 = startDay(Date.now()), now = Date.now();
  let ms = 0;
  for (const p of state.periods || []) {
    const a = Math.max(p.from, t0), b = Math.min(pTo(p), now);
    if (b > a) ms += b - a;
  }
  const min = Math.round(ms / 6e4);
  if (min < 1) return "";
  return `${min < 60 ? min + " min" : Math.floor(min / 60) + " h" + (min % 60 ? " " + pad(min % 60) : "")} fora de casa`;
}

// gráfico das últimas horas do RC102, com os trechos fora de casa
state.rc24 = { h: +localGet("rc24h", "24") || 24, seq: 0 };
async function refreshRc24() {
  if (!api.GetChartRange || state.view !== "radiacode" || state.sub.radiacode !== "agora" || $("rc24Card").hidden) return;
  const my = ++state.rc24.seq, h = state.rc24.h, now = Date.now();
  let c;
  try { c = await api.GetChartRange(now - h * 3600e3, now); } catch (e) { return; }
  if (my !== state.rc24.seq || !c) return;
  drawRc24(c, now - h * 3600e3, now);
}
function drawRc24(c, t0, t1) {
  const box = $("rc24Chart"), h = state.rc24.h;
  $("rc24Title").textContent = h === 168 ? "Últimos 7 dias" : `Últimas ${h} horas`;
  const W = 1000, H = 150, PT = 8, PB = 18;
  const vals = (c.rc || []).map((v, i) => [c.times[i], v]).filter((p) => p[1] != null && isFinite(p[1]));
  let top = 0;
  for (const [, v] of vals) top = Math.max(top, v);
  top = Math.max(0.25, top * 1.15);
  const step = top <= 0.5 ? 0.1 : top <= 1.2 ? 0.2 : top <= 3 ? 0.5 : Math.pow(10, Math.floor(Math.log10(top / 2)));
  const x = (t) => ((t - t0) / (t1 - t0)) * W, y = (v) => PT + (1 - v / top) * (H - PT - PB);
  let svg = `<svg viewBox="0 0 ${W} ${H}" preserveAspectRatio="none" class="rc-24-svg" aria-hidden="true">`;
  let away = false;
  for (const p of state.periods || []) {
    const a = Math.max(p.from, t0), b = Math.min(pTo(p), t1);
    if (b <= a) continue;
    away = true;
    svg += `<rect x="${x(a).toFixed(1)}" y="0" width="${Math.max(1, x(b) - x(a)).toFixed(1)}" height="${H - PB}" class="away"/>`;
  }
  const lines = [];
  for (let v = step; v < top; v += step) { lines.push(v); svg += `<line x1="0" x2="${W}" y1="${y(v).toFixed(1)}" y2="${y(v).toFixed(1)}" class="grid"/>`; }
  // linha com buracos onde não há leitura (mais de 3 passos sem dado)
  const gap = (c.stepMs || 60e3) * 3;
  let d = "", prev = 0;
  for (const [t, v] of vals) { d += `${d && t - prev <= gap ? "L" : "M"}${x(t).toFixed(1)},${y(v).toFixed(1)}`; prev = t; }
  svg += `<path d="${d}" class="rc"/></svg>`;
  box.innerHTML = vals.length ? svg : `<div class="rc-empty small muted">Sem leituras do RC102 nesse período.</div>`;
  $("rc24AwayKey").hidden = !away;
  const ls = lines.slice(0, 2).map((v) => fmt(v, step < 0.1 ? 2 : 1));
  rcParts($("rc24Info"), [ls.length === 2 ? `linhas em ${ls[0]} e ${ls[1]} µSv/h` : ls.length ? `linhas em ${ls[0]} µSv/h` : "", h === 168 ? "−7 dias … agora" : `−${h} h … agora`]);
}

async function refreshRcCompare() {
  // o cartão mora em Configurações › GQ GMC-800 › Calibração (2.51)
  if (!api.GetRcCompare || !$("rcCmpCard").offsetParent) return;
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
  for (const id of ["btnRcRetry", "btnRcRetrySet"]) $(id).addEventListener("click", async () => renderRc(await call(api.ReconnectRadiacode)));
  for (const id of ["btnRcSettings", "btnRcConn"]) $(id).addEventListener("click", () => openSettings("chkRcOn"));
  $("rc24Range").addEventListener("click", (e) => {
    const b = e.target.closest("button[data-h]");
    if (!b) return;
    state.rc24.h = +b.dataset.h;
    localSet("rc24h", String(state.rc24.h));
    $("rc24Range").querySelectorAll("button").forEach((x) => x.setAttribute("aria-pressed", String(x === b)));
    refreshRc24();
  });
  $("rc24Range").querySelectorAll("button").forEach((x) => x.setAttribute("aria-pressed", String(+x.dataset.h === state.rc24.h)));
  // o lado a lado (agora nas Configurações) atualiza quando aparece
  new IntersectionObserver((es) => { if (es.some((e) => e.isIntersecting)) refreshRcCompare(); }).observe($("rcCmpCard"));
  $("chkRcOn").addEventListener("change", async (e) => renderRc(await call(api.SetRadiacode, e.target.checked, $("rcBLE").value)));
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
  setInterval(() => { if (state.rc.st && state.rc.st.state === "conectado") { refreshRcCompare(); refreshRc24(); } }, 60e3);
  if (api.GetRadiacode) api.GetRadiacode().then(renderRc).catch(() => {});
}

// ---------- resumo e comparação ----------

state.sum = { mode: "resumo", kind: "dia", anchor: 0, cmp: "hoje-ontem", dayA: 0, dayB: 0 };
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

// Configurações › GQ GMC-800 › Contador: duas linhas curtas (Memória e
// Download) e a explicação embaixo (2.73, U1 da auditoria; spec
// ux/topo-sem-alertas.html, item 6). Cada frase num span: juntas, a tradução
// não achava o molde (U3).
function renderMemInfo(cs, pct) {
  const mem = $("iMem");
  mem.textContent = !cs.estPct ? "—" : cs.fullIn > 0 ? `${pct}% · enche em ${fmtDuration(cs.fullIn)}` : `${pct}%`;
  mem.classList.toggle("mem-att", cs.memLevel === "atencao");
  mem.classList.toggle("mem-full", cs.memLevel === "cheia");
  const when = cs.at ? `${ddmm(cs.at)} ${hhmm(cs.at)}` : "";
  const dl = { "em-dia": `em dia · ${when}`, pendente: `falta ${fmtDur((cs.pendingMinutes || 0) * 60e3)}`, "sem-gravacao": "contador não grava", nunca: "nunca" }[cs.state] || "—";
  $("iSync").textContent = cs.known && !cs.complete ? `${dl} · incompleto` : dl;
  const note = [];
  if (cs.state === "em-dia") note.push("Em dia: nenhum trecho sem leitura no app desde o último download.");
  else if (cs.state === "sem-gravacao") note.push("O contador não está gravando na memória: minutos sem o app não podem ser recuperados.");
  else if (cs.text) note.push(cs.text);
  if (cs.known && !cs.complete && cs.state !== "nunca" && !note.some((t) => t.includes("interrompido"))) note.push("O último download foi interrompido antes do fim da memória.");
  if (cs.estPct) note.push(`A porcentagem é estimada a partir do download, no modo de gravação "${cs.saveMode}".`);
  const p = $("iSyncNote");
  p.replaceChildren();
  note.forEach((t, i) => { if (i) p.append(" "); p.append(Object.assign(document.createElement("span"), { textContent: t })); });
  p.hidden = !note.length;
}

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
  renderMemInfo(cs, pct);
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
  // em µSv/h pela calibração de agora (2.65.2; o servidor manda em CPM)
  const svg = $("bgChart"), k = calK(), u = (v) => (v == null ? null : v * k);
  let lo = Infinity, hi = -Infinity;
  for (const p of pts) for (const v of [u(p.mean), u(p.median)]) if (v != null) { lo = Math.min(lo, v); hi = Math.max(hi, v); }
  for (const d of daily) if (d.full) { lo = Math.min(lo, u(d.trimmed)); hi = Math.max(hi, u(d.trimmed)); }
  const ys = $("bgY").children;
  if (!isFinite(lo)) { svg.innerHTML = ""; for (const e of ys) e.textContent = ""; $("bgLegend").textContent = ""; $("bgDisp").innerHTML = ""; for (const e of $("bgX").children) e.textContent = ""; return; }
  const pad = Math.max(k, (hi - lo) * 0.15), q = 0.005;
  lo = Math.max(0, Math.floor((lo - pad) / q) * q); hi = Math.ceil((hi + pad) / q) * q;
  const x = (t) => ((t - tr.from) / (tr.to - tr.from)) * W;
  const y = (v) => H - ((v - lo) / (hi - lo)) * H;
  for (let i = 0; i < 4; i++) ys[i].textContent = fmt(hi - ((hi - lo) / 3) * i, 3);
  let out = [0, 1, 2, 3].map((i) => { const gy = i === 3 ? H - 0.5 : i === 0 ? 0.5 : (H / 3) * i; return `<line x1="0" y1="${gy}" x2="${W}" y2="${gy}" stroke="${COR.grid}" stroke-width="1" vector-effect="non-scaling-stroke"/>`; }).join("");
  if (forming) out += `<rect x="0" y="0" width="${Math.max(0, x(tr.first)).toFixed(1)}" height="${H}" fill="${COR.muted}" fill-opacity="0.06"/>`;
  const path = (val) => { let d = "", pen = "M"; for (const p of pts) { const v = u(val(p)); if (v == null) { pen = "M"; continue; } d += `${pen}${x(p.t).toFixed(1)} ${y(v).toFixed(1)}`; pen = "L"; } return d; };
  out += `<path d="${path((p) => p.mean)}" fill="none" stroke="${COR.accent}" stroke-width="2" stroke-linejoin="round" vector-effect="non-scaling-stroke"/>`;
  out += `<path d="${path((p) => p.median)}" fill="none" stroke="#7DD3FC" stroke-width="1.4" stroke-dasharray="5 3" stroke-linejoin="round" vector-effect="non-scaling-stroke"/>`;
  for (const d of daily) if (d.full) out += `<circle cx="${x(d.day + DAY_MS / 2).toFixed(1)}" cy="${y(u(d.trimmed)).toFixed(1)}" r="2.6" fill="${COR.text}" fill-opacity="0.75"/>`;
  if (f.state === "subindo" || f.state === "descendo") {
    out += `<line x1="${x(f.fitT0 + DAY_MS / 2).toFixed(1)}" y1="${y(u(f.fitFrom)).toFixed(1)}" x2="${x(f.fitT1 + DAY_MS / 2).toFixed(1)}" y2="${y(u(f.fitTo)).toFixed(1)}" stroke="${COR.att}" stroke-width="1.6" stroke-dasharray="6 4" vector-effect="non-scaling-stroke"/>`;
  } else if (f.state === "degrau") {
    out += `<path d="M${x(f.fitT0).toFixed(1)} ${y(u(f.stepFrom)).toFixed(1)}H${x(f.stepAt).toFixed(1)}V${y(u(f.stepTo)).toFixed(1)}H${x(f.fitT1 + DAY_MS).toFixed(1)}" fill="none" stroke="${COR.att}" stroke-width="1.6" stroke-dasharray="6 4" vector-effect="non-scaling-stroke"/>`;
  }
  svg.innerHTML = out;
  const lab = (t) => `${WEEKDAY[new Date(t).getDay()]} ${ddmm(t)}`;
  const xs = $("bgX").children;
  xs[0].textContent = lab(tr.from); xs[1].textContent = lab((tr.from + tr.to) / 2); xs[2].textContent = "agora";
  $("bgLegend").innerHTML = '<span class="key"><i style="border-color:' + COR.accent + '"></i>média móvel de 24 h</span><span class="key"><i style="border-color:#7DD3FC;border-top-style:dashed"></i>mediana móvel de 24 h</span>' +
    '<span class="key"><i class="dot-k"></i>média de cada dia (sem os 5% extremos)</span>' +
    (f.state === "subindo" || f.state === "descendo" || f.state === "degrau" ? '<span class="key"><i style="border-color:' + COR.att + ';border-top-style:dashed"></i>tendência</span>' : "") +
    (forming ? '<span class="key"><i class="sw" style="background:' + COR.muted + '"></i>antes do primeiro dado</span>' : "") + " (µSv/h)";

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
    tip.textContent = `24 h até ${dayLabel(best.t - 1)} ${hhmm(best.t)} · ` + (best.mean == null ? `poucos dados (${fmt(best.n, 0)} min)` : `média ${fmt(best.mean * calK(), 3)} · mediana ${fmt(best.median * calK(), 3)} µSv/h · n = ${fmt(best.n, 0)} min`);
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

// Relatório (PDF) (2.68): o diálogo pergunta completo × só o resumo, as
// anotações e o anexo das configurações; o app lembra a última escolha
let repPending = null;
function exportReport(from, to, label, btn) {
  if (!api.ExportReportWith) return runReport(btn, () => api.ExportReport(Math.round(from), Math.round(to), label));
  repPending = { from, to, label, btn };
  const st = state.settings || {};
  const mode = st.reportMode === "resumo" ? "resumo" : "completo";
  document.querySelectorAll("#repModes .rep-mode").forEach((b) => b.setAttribute("aria-checked", String(b.dataset.mode === mode)));
  $("repNotes").checked = !!st.reportNotes;
  $("repCfg").checked = !st.reportNoCfg;
  $("repSub").textContent = `${label} · ${dayLabel(from)} ${hhmm(from)} → ${dayLabel(Math.min(to, Date.now()) - 1)} ${hhmm(Math.min(to, Date.now()))}`;
  $("repDlg").showModal();
}
function wireReportDlg() {
  document.querySelectorAll("#repModes .rep-mode").forEach((b) => b.addEventListener("click", () => {
    document.querySelectorAll("#repModes .rep-mode").forEach((x) => x.setAttribute("aria-checked", String(x === b)));
  }));
  $("btnRepGo").addEventListener("click", () => {
    const q = repPending;
    if (!q) return;
    const mode = document.querySelector('#repModes .rep-mode[aria-checked="true"]').dataset.mode;
    const notes = $("repNotes").checked, cfg = $("repCfg").checked;
    Object.assign(state.settings, { reportMode: mode, reportNotes: notes, reportNoCfg: !cfg });
    $("repDlg").close();
    repPending = null;
    runReport(q.btn, () => api.ExportReportWith(Math.round(q.from), Math.round(q.to), q.label, mode, notes, cfg));
  });
  $("btnRepCancel").addEventListener("click", () => { repPending = null; $("repDlg").close(); });
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
  $("btnSumCmp").addEventListener("click", () => { state.sum.mode = "comparar"; refreshSummary(); });
  $("btnSumBack").addEventListener("click", () => { state.sum.mode = "resumo"; refreshSummary(); });
  $("btnDaysMore").addEventListener("click", () => { dailyN = 60; refreshDaily(); });
  // "Mais números" do Resumo lembra se estava aberto
  $("sumDetails").open = localGet("sumDet", "") === "1";
  $("sumDetails").addEventListener("toggle", (e) => localSet("sumDet", e.target.open ? "1" : ""));
  document.querySelectorAll("#sumKindSeg button").forEach((b) => b.addEventListener("click", () => { state.sum.kind = b.dataset.sk; state.sum.anchor = 0; refreshSummary(); }));
  document.querySelectorAll("#sumCmpSeg button").forEach((b) => b.addEventListener("click", () => { state.sum.cmp = b.dataset.sc; refreshSummary(); }));
  $("sumPrev").addEventListener("click", () => sumStep(-1));
  $("sumNext").addEventListener("click", () => sumStep(1));
  $("sumWhen").addEventListener("click", () => { const i = $("sumDay"); try { i.showPicker(); } catch (e) { i.focus(); i.click(); } });
  $("sumDay").addEventListener("change", (e) => { if (e.target.value) { state.sum.anchor = sumStart(state.sum.kind, parseDateVal(e.target.value)); refreshSummary(); } });
  // dia no futuro não tem o que comparar: volta para hoje
  const cmpDay = (v) => Math.min(parseDateVal(v), startDay(Date.now()));
  $("sumDayA").addEventListener("change", (e) => { if (e.target.value) { state.sum.dayA = cmpDay(e.target.value); refreshSummary(); } });
  $("sumDayB").addEventListener("change", (e) => { if (e.target.value) { state.sum.dayB = cmpDay(e.target.value); refreshSummary(); } });
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
    applyUi(state.settings.uiScale, state.settings.uiDensity);
    if (hasGo) { syncLang(state.settings.lang); if (api.SetUILang) api.SetUILang(LANG).catch(() => {}); }
    else state.settings.lang = LANG_PREF; // modo navegador: vale o que está no navegador
    state.status = await api.GetStatus();
    state.demo = await api.GetDemo();
  } catch (e) { /* segue com o padrão */ }
  renderDemo();
  usoInit();
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
  refreshPlanned();
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


// ---------- licenças de terceiros (auditoria de 12/11, D2) ----------
// o texto é gerado por scripts/licenses e só é lido quando abre
{
  const d = $("licDetails");
  if (d) d.addEventListener("toggle", async () => {
    const pre = $("licText");
    if (!d.open || pre.textContent) return;
    try { pre.textContent = await (await fetch("licencas.txt")).text(); }
    catch (e) { pre.textContent = tr("Não consegui abrir o arquivo das licenças."); }
  });
}

// ---------- tamanho e espaço da janela (2.49) ----------
// --ui-scale multiplica os tokens de fonte e --ui-density os de espaço
// (style.css). A prévia é ao vivo; "Salvar" guarda. Ctrl +/−/0 muda e já
// salva.
const UI_STEPS = [0.9, 1, 1.08, 1.15, 1.3, 1.5];
const UI_DENS = { compacto: 0.8, confortavel: 1.2 };
function applyUi(scale, dens) {
  const r = document.documentElement.style;
  r.setProperty("--ui-scale", String(scale > 0 ? scale : 1));
  r.setProperty("--ui-density", String(UI_DENS[dens] || 1));
}
function uiSegSet(scale, dens) {
  $("valUiScale").value = String(scale);
  $("valUiDensity").value = dens;
  document.querySelectorAll("#segUiScale button").forEach((b) => b.setAttribute("aria-pressed", String(+b.dataset.v === +scale)));
  document.querySelectorAll("#segUiDensity button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.v === dens)));
}
function wireUiScale() {
  const pick = () => {
    uiSegSet(parseFloat($("valUiScale").value) || 1, $("valUiDensity").value);
    applyUi(parseFloat($("valUiScale").value), $("valUiDensity").value);
    $("valUiScale").dispatchEvent(new Event("change", { bubbles: true }));
  };
  document.querySelectorAll("#segUiScale button").forEach((b) => b.addEventListener("click", () => { $("valUiScale").value = b.dataset.v; pick(); }));
  document.querySelectorAll("#segUiDensity button").forEach((b) => b.addEventListener("click", () => { $("valUiDensity").value = b.dataset.v; pick(); }));
  document.addEventListener("keydown", async (e) => {
    if (!(e.ctrlKey || e.metaKey) || e.altKey) return;
    const k = e.key;
    if (!["+", "=", "-", "_", "0"].includes(k)) return;
    e.preventDefault();
    const cur = state.settings.uiScale || 1;
    let i = UI_STEPS.findIndex((s) => s >= cur - 1e-6);
    if (i < 0) i = 1;
    const next = k === "0" ? 1 : UI_STEPS[Math.max(0, Math.min(UI_STEPS.length - 1, i + (k === "-" || k === "_" ? -1 : 1)))];
    if (next === cur) return;
    applyUi(next, state.settings.uiDensity);
    try { await saveSettings({ uiScale: next }); } catch (err) { return; }
    if (state.setSnap) { uiSegSet(next, $("valUiDensity").value); state.setSnap.valUiScale = String(next); renderSetBar(); }
    document.querySelectorAll("#toasts .toast.ui-size").forEach((t) => t.remove()); // um aviso só
    toast(`Tamanho: ${Math.round(next * 100)}%`);
    const t = $("toasts").lastElementChild;
    if (t) t.classList.add("ui-size");
  });
}

// ---------- Configurações em duas colunas (2.68.4) ----------
// O último cartão visível de cada coluna estica até o fim, para as duas
// colunas terminarem juntas. É marcado aqui (e não com :last-child) porque
// cartões aparecem e somem (aparelho desconectado, seção escondida).
(function setColsLast() {
  const root = document.getElementById("settings");
  if (!root) return;
  let pending = false;
  const mark = () => {
    pending = false;
    for (const col of root.querySelectorAll(".set-col")) {
      let last = null;
      for (const f of col.children) {
        if (f.tagName === "FIELDSET" && !f.hidden && getComputedStyle(f).display !== "none") last = f;
      }
      for (const f of col.children) {
        const want = f === last;
        if (f.classList.contains("is-last") !== want) f.classList.toggle("is-last", want);
      }
    }
  };
  const later = () => { if (!pending) { pending = true; requestAnimationFrame(mark); } };
  new MutationObserver((recs) => {
    if (recs.every((r) => r.attributeName === "class" && r.target.classList && r.target.parentElement && r.target.parentElement.classList.contains("set-col") && r.oldValue != null && r.oldValue.replace(/\s*is-last\s*/g, " ").trim() === r.target.className.replace(/\s*is-last\s*/g, " ").trim())) return;
    later();
  }).observe(root, { attributes: true, attributeOldValue: true, attributeFilter: ["hidden", "class", "style"], subtree: true, childList: true });
  later();
})();
