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
const isToday = (ms) => new Date(ms).toDateString() === new Date().toDateString();
const isYesterday = (ms) => { const y = new Date(); y.setDate(y.getDate() - 1); return new Date(ms).toDateString() === y.toDateString(); };
const usvDigits = (v) => (v < 1 ? 3 : v < 10 ? 2 : 1);
const $ = (id) => document.getElementById(id);

// fator µSv/h por CPM no fundo: o do contador (ou o último lido dele); o
// 0,0101 é o de fábrica do GMC-800, só se o app ainda não leu nada
const calK = () => (state.status && (state.status.usvPerCPM || (state.status.device || {}).usvPerCPM)) || 0.0101;

// ---------- estado ----------

const state = {
  unit: localGet("unit", "cpm"),
  range: localGet("range", "1h"),
  zoom: null,      // { from, to } quando arrastou no gráfico
  zoomStack: [],   // zooms anteriores (botão Voltar)
  periods: [],     // fora de casa
  measurements: [], // medições de objeto salvas
  doseZoom: null,
  doseZoomStack: [],
  chartUnit: localGet("chartUnit", "cpm"),
  mMode: localGet("mMode", "nova"),
  heatDays: parseInt(localGet("heatDays", "14"), 10) || 14,
  heatMonthMode: localGet("heatMode", "dias") === "mes",
  heatMonth: (() => { const d = new Date(); return { y: d.getFullYear(), m: d.getMonth() + 1 }; })(),
  heatDataStart: 0, // início do mês da leitura mais antiga (ms)
  dosePeriod: localGet("dosePeriod", "24h"),
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
  const range = `seu normal: ${lo} a ${hi} CPM`;
  let text, cls = "";
  if (z >= 3) { text = `Acima do normal (${range}) — dificilmente é só variação`; cls = "high"; }
  else if (z >= 2) { text = `Um pouco acima do normal (${range}) — pode ser só variação`; cls = "mid"; }
  else if (z <= -2) { text = `Abaixo do normal (${range}) — sem problema`; }
  else { text = `Dentro do normal — seu fundo costuma ficar entre ${lo} e ${hi} CPM`; }
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
    <p>O seu fundo é <b class="mono">${fmt(bg, 1)} CPM</b> (mediana dos últimos 30 dias), então <b class="mono">1σ = √${fmt(bg, 1)} ≈ ${fmt(sd, 1)} CPM</b>. Uma leitura "em σ" é quantos desses passos ela está acima ou abaixo do fundo: (leitura − fundo) ÷ σ.</p>
    ${now}
    <h3>Quão raro é, pelo acaso, num minuto</h3>
    <div class="sg-tbl-wrap"><table class="m-stats mono sg-tbl"><thead><tr><th>acima de</th><th>com o seu fundo</th><th>chance por minuto</th><th>só por acaso</th></tr></thead><tbody>${rows}</tbody></table></div>
    <p class="small muted">Calculado com a distribuição de Poisson do seu fundo. Por isso até 3σ acontece todo dia: são 1.440 minutos. Um pico de 4σ ou mais já pede uma olhada (chuva? algo perto do contador?).</p>
    <h3>Onde mais aparece</h3>
    <ul class="sg-list">
      <li><b>± na medição de objeto</b>: a incerteza de 1σ de cada parte. 284,2 ± 7,5 CPM quer dizer que o valor de verdade está a ±7,5 em ~68% das vezes, ±15 em ~95%.</li>
      <li><b>A diferença em σ</b>: diferença ÷ incerteza combinada. 244,6 ± 8,0 = 30σ: na prática, impossível ser acaso. Abaixo de 2σ, pode ser só variação.</li>
      <li><b>Médias têm σ menor</b>: a média de 60 minutos varia √60 ≈ 7,7 vezes menos que 1 minuto (${fmt(Math.sqrt(bg / 60), 2)} CPM com o seu fundo). Por isso, no padrão por hora, +1,8 CPM numa hora já pode ser 2σ ou mais.</li>
      <li><b>Mais tempo, menos incerteza</b>: a incerteza relativa é 1 ÷ √contagens. 100 contagens dão ±10%; 10.000 dão ±1%.</li>
    </ul>`;
  $("sigmaDlg").showModal();
}
document.addEventListener("click", (e) => { if (e.target.closest("#btnSigmaHelp, .rc-sigma-help")) openSigmaHelp(); });

function renderReading() {
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
    $("levelName").className = "level-name";
    $("levelMarker").hidden = true;
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

  const names = { seguro: "Seguro", atencao: "Atenção", alerta: "Alerta", perigo: "Perigo" };
  $("levelName").textContent = names[r.level] || "—";
  $("levelName").className = "level-name " + r.level;
  // 0 CPM por minutos seguidos: defeito, não "seguro"
  if (r.noCount && !r.saturated) {
    const el = $("readSigma");
    el.textContent = `Sem contagens há ${fmtDur(Date.now() - r.noCountSince)}: o tubo pode estar com defeito. Confira o contador.`;
    el.className = "sigma high";
    el.hidden = false;
    $("levelName").textContent = "Sem contagens";
    $("levelName").className = "level-name nocount";
  }
  if (r.noCount && !state.noCountToast && !r.saturated) toast("Sem contagens há 2 min: confira o contador", true);
  state.noCountToast = !!r.noCount;
  const m = $("levelMarker");
  m.hidden = false;
  m.style.left = Math.min(r.cpm, 150) / 150 * 100 + "%";
  $("hero").classList.toggle("alarm", !!(r.alarm || r.danger));
}

// ---------- conexão ----------

function renderStatus() {
  const s = state.status;
  const d = s.device || {};
  const dot = $("statusDot");
  dot.className = "dot" + (s.state === "conectado" ? " on" : s.state === "conectando" ? " wait" : s.state === "erro" ? " err" : "");
  let text = "desconectado";
  if (s.state === "conectado") text = `${d.version || "contador"} · ${d.port} · conectado`;
  else if (s.state === "conectando") text = s.message || "conectando…";
  else if (s.state === "desconectado" && s.message) text = "desconectado · " + s.message;
  else if (s.state === "erro") text = s.message || "erro";
  $("statusText").textContent = text;
  const retrying = state.settings.autoConnect && s.state !== "conectado" && s.state !== "conectando" && s.message;
  $("statusPill").title = retrying ? text + " — o app tenta de novo a cada 10 s" : text;

  const connected = s.state === "conectado";
  $("btnHistory").disabled = !connected || s.downloading;
  $("clockChip").disabled = !connected || s.downloading;
  $("emptyHint").hidden = connected || s.state === "conectando" || state.demo.active;

  $("iModel").textContent = connected ? d.version : "—";
  $("iSerial").textContent = connected && d.serial ? d.serial : "—";
  $("iPort").textContent = connected ? d.port : "—";
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
  $("tAvg").textContent = fmt(s.avg1h, 1);
  $("tAvgSub").textContent = s.avg1h == null ? "CPM" : `≈ ${fmt(s.avg1h * cal, 3)} µSv/h`;
  $("tMin").textContent = fmt(s.min24h, 0);
  $("tMax").textContent = fmt(s.max24h, 0);
  $("tMaxSub").textContent = s.max24h == null ? "CPM" : `CPM às ${hhmm(s.max24hAt)}`;
  // as mesmas medidas dentro do quadro do GMC (quando está lado a lado com o RadiaCode)
  $("hAvg1h").textContent = fmt(s.avg1h, 1);
  $("hMin24").textContent = fmt(s.min24h, 0);
  $("hMax24").textContent = fmt(s.max24h, 0);
  $("hMax24Sub").textContent = s.max24h == null ? "CPM" : `CPM às ${hhmm(s.max24hAt)}`;
  $("tDose").textContent = s.minutesToday ? fmt(s.doseToday, 2) : "—";
  $("tDoseSub").textContent = s.minutesToday ? `${s.minutesToday} min medidos` : "acumulados";
  $("heroDose").title = s.minutesToday ? `Hoje: ${doseCompare(s.doseToday)}` : "";
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
      d.className = "dot-a " + (/Alarme/.test(e.text) ? "alarm" : "att");
      d.setAttribute("aria-hidden", "true");
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

// ---------- alertas ----------
// Card: alertas ativos + os 3 últimos. Modal: histórico completo.

const lvlCls = (e) => e.level || (/^(\[demo\] )?Perigo/.test(e.text) ? "perigo" : /^(\[demo\] )?Alarme/.test(e.text) ? "alarme" : "atencao");
const lvlName = (e) => ({ perigo: "Perigo", alarme: "Alarme" }[lvlCls(e)] || "Atenção");

function fmtDur(ms) {
  const m = Math.round(ms / 60000);
  if (m < 1) return "menos de 1 min";
  if (m < 60) return m + " min";
  const h = Math.floor(m / 60), r = m % 60;
  if (h < 48) return r ? `${h} h ${pad(r)} min` : `${h} h`;
  return Math.round(h / 24) + " dias";
}

function whenLabel(ms) {
  return isToday(ms) ? "hoje " + hhmm(ms) : isYesterday(ms) ? "ontem " + hhmm(ms) : ddmm(ms) + " " + hhmm(ms);
}

function alertTitle(e) {
  const demo = e.demo || /^\[demo\] /.test(e.text) ? " (demo)" : "";
  if (e.members) return `${lvlName(e)}: pico de ${fmt(e.peak, 0)} CPM${demo}`;
  if (e.retro) return `${lvlName(e)}: pico de ${fmt(e.peak, 0)} CPM${demo}`;
  // alertas antigos diziam "(acima de 50)" mesmo com 50: o certo é "a partir de"
  return e.text.replace(/^\[demo\] /, "").replace(/\(acima de (\d+)\)/, "(a partir de $1)") + demo;
}

function alertSub(e) {
  const parts = [];
  if (e.startedAs) parts.push(`${e.startedAs.level === "alarme" ? "alarme" : "atenção"} às ${hhmm(e.startedAs.time)} → ${lvlName(e).toLowerCase()} às ${hhmm(e.upAt)}`);
  if (e.end) parts.push(`${hhmm(e.time)}–${hhmm(e.end)} · durou ${fmtDur(e.end - e.time)}`);
  else if (!e.active && e.level) parts.push("fim não registrado");
  if (e.peak && (e.retro || e.peak > (e.cpm || 0))) parts.push(`pico ${fmt(e.peak, 0)} CPM${sigTxt(e.peak)} às ${(e.retro ? hhmm : hhmmss)(e.peakAt || e.time)}`);
  return parts.join(" · ");
}

// ---------- episódios: atenção que virou alarme/perigo vira uma linha só ----------
// Os alertas continuam gravados separados (cada um com o seu gráfico); só a
// lista junta os que se encostam no tempo. A linha mostra o nível mais alto,
// o começo do primeiro, o fim do último e o maior pico.
const LVL_RANK = { atencao: 1, alarme: 2, perigo: 3 };
function groupEpisodes(list) {
  const asc = (list || []).slice().sort((x, y) => x.time - y.time);
  const now = Date.now();
  const eps = [];
  let cur = null;
  for (const e of asc) {
    const end = e.active ? now : (e.end || e.time);
    if (cur && !!e.demo === cur.demo && !!e.retro === cur.retro && e.time <= cur.until + 60000) {
      cur.members.push(e);
      cur.until = Math.max(cur.until, end);
      continue;
    }
    cur = { members: [e], until: end, demo: !!e.demo, retro: !!e.retro };
    eps.push(cur);
  }
  return eps.map((g) => {
    const ms = g.members;
    if (ms.length === 1) return ms[0];
    const top = ms.reduce((b, x) => ((LVL_RANK[lvlCls(x)] || 0) > (LVL_RANK[lvlCls(b)] || 0) ? x : b), ms[0]);
    const pk = ms.reduce((b, x) => ((x.peak || 0) > (b.peak || 0) ? x : b), ms[0]);
    const first = ms[0];
    const active = ms.some((x) => x.active);
    const ended = ms.every((x) => x.end || x.active);
    return Object.assign({}, top, {
      time: first.time,
      end: active || !ended ? 0 : Math.max(...ms.map((x) => x.end)),
      active,
      peak: pk.peak, peakAt: pk.peakAt,
      startedAs: lvlCls(first) !== lvlCls(top) ? { level: lvlCls(first), time: first.time } : null,
      upAt: top.time,
      members: ms,
    });
  }).sort((x, y) => y.time - x.time);
}

function alertLi(e, withDate) {
  const li = document.createElement("li");
  const d = document.createElement("span");
  d.className = "dot-a " + ({ perigo: "danger", alarme: "alarm" }[lvlCls(e)] || "att");
  d.setAttribute("aria-hidden", "true");
  const t = document.createElement("time");
  t.dateTime = new Date(e.time).toISOString();
  t.textContent = withDate ? whenLabel(e.time) : hhmm(e.time);
  const box = document.createElement("span");
  box.className = "a-text";
  const ttl = document.createElement("span");
  ttl.textContent = alertTitle(e);
  const ap = periodAt(e.time);
  if (ap) {
    const tag = document.createElement("span");
    tag.className = "tag-retro tag-away";
    tag.textContent = ap.name;
    tag.title = "Aconteceu num período marcado como fora de casa";
    ttl.appendChild(tag);
  }
  if (e.retro) {
    const tag = document.createElement("span");
    tag.className = "tag-retro";
    tag.textContent = "do contador";
    tag.title = "Achado no histórico baixado do contador";
    ttl.appendChild(tag);
  }
  box.appendChild(ttl);
  const sub = alertSub(e);
  if (sub) {
    const sp = document.createElement("span");
    sp.className = "a-sub";
    sp.textContent = sub;
    box.appendChild(sp);
  }
  li.append(d, t, box);
  return li;
}

state.activeAlerts = [];
state.alertFilter = "";

function renderActive() {
  const ul = $("activeAlerts");
  const list = state.activeAlerts || [];
  ul.textContent = "";
  if (!list.length) {
    ul.innerHTML = '<li class="none"><span class="dot-ok" aria-hidden="true"></span>Nenhum alerta agora.</li>';
    return;
  }
  const now = Date.now();
  // num episódio os níveis de baixo vêm junto: mostra só o mais alto de cada
  // fonte (real/demo), com a hora em que os de baixo começaram
  const rank = { atencao: 1, alarme: 2, perigo: 3 };
  const top = (demo) => Math.max(0, ...list.filter((x) => !!x.demo === demo).map((x) => rank[x.level] || 1));
  const shown = list.filter((e) => (rank[e.level] || 1) === top(!!e.demo));
  for (const e of shown) {
    const li = document.createElement("li");
    li.className = "on " + lvlCls(e);
    const demo = e.demo ? " (demo)" : "";
    const lim = e.level === "alarme" ? (e.text.match(/limite (\d+)/) || [])[1] : e.level === "perigo" ? String(dangerCPM()) : null;
    li.innerHTML = `<span class="a-title"><span class="pulse" aria-hidden="true"></span></span><span class="a-meta"></span>`;
    li.querySelector(".a-title").append(`${lvlName(e)} agora${demo}`);
    li.querySelector(".a-meta").textContent =
      `desde ${hhmm(e.time)} (${fmtDur(now - e.time)}) · pico ${fmt(e.peak, 0)} CPM${sigTxt(e.peak)}` +
      (lim ? ` · limite ${fmt(+lim, 0)}` : "");
    for (const lower of list.filter((x) => !!x.demo === !!e.demo && (rank[x.level] || 1) < (rank[e.level] || 1)).sort((a, b) => a.time - b.time)) {
      if (lower.time < e.time) li.querySelector(".a-meta").textContent += ` · ${lvlName(lower).toLowerCase()} desde ${hhmm(lower.time)}`;
    }
    ul.appendChild(li);
  }
}

let lastActiveTick = 0;
function tickActive() {
  if (!(state.activeAlerts || []).length) return;
  const n = Date.now();
  if (n - lastActiveTick < 15000) return;
  lastActiveTick = n;
  renderActive();
}

function renderAlertHistory(all) {
  const box = $("alertHistList");
  const f = state.alertFilter;
  document.querySelectorAll("#alertFilter button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.af === f)));
  const list = groupEpisodes(all).filter((e) => !f || (f === "retro" ? e.retro : f === "live" ? !e.retro : lvlCls(e) !== "atencao"));
  $("alertCount").textContent = list.length === 1 ? "1 alerta" : `${list.length} alertas`;
  box.textContent = "";
  if (!list.length) { box.innerHTML = '<p class="muted small">Nenhum alerta aqui.</p>'; return; }
  let day = "", ul = null;
  for (const e of list) {
    const dt = new Date(e.time);
    const key = dt.toDateString();
    if (key !== day) {
      day = key;
      const h = document.createElement("h3");
      h.textContent = isToday(e.time) ? "Hoje" : isYesterday(e.time) ? "Ontem" :
        dt.toLocaleDateString(LOCALE, { weekday: "long", day: "2-digit", month: "2-digit", year: "numeric" });
      ul = document.createElement("ul");
      ul.className = "events alerts";
      box.append(h, ul);
    }
    const li = alertLi(e, false);
    if (e.active) li.querySelector(".a-text > span").append(" — ativo agora");
    li.classList.add("pick");
    li.classList.toggle("sel", e.id === state.alertSel);
    li.tabIndex = 0;
    li.addEventListener("click", () => selectAlert(e.id));
    li.addEventListener("keydown", (ev) => { if (ev.key === "Enter" || ev.key === " ") { ev.preventDefault(); selectAlert(e.id); } });
    li.dataset.id = e.id;
    ul.appendChild(li);
  }
  if (!list.some((e) => e.id === state.alertSel)) selectAlert(list[0].id);
  else if (!state.ad || state.ad.event.id !== state.alertSel || state.ad.event.active || list.find((e) => e.id === state.alertSel).active) loadAlertDetail();
}

// ---------- gráfico detalhado do alerta (tela Alertas) ----------

state.alertSel = null;
const LVL_COLOR = { atencao: "#F5B84A", alarme: "#F28C38", perigo: "#F0656A" };

function selectAlert(id) {
  state.alertSel = id;
  document.querySelectorAll("#alertHistList li.pick").forEach((li) => li.classList.toggle("sel", +li.dataset.id === id));
  loadAlertDetail();
}

let adSeq = 0;
async function loadAlertDetail() {
  if (state.alertSel == null) return;
  const my = ++adSeq;
  let d;
  try { d = await api.GetAlertDetail(state.alertSel); } catch (e) { d = null; }
  if (my !== adSeq) return;
  drawAlertDetail(d);
}

function drawAlertDetail(d) {
  state.ad = d;
  const svg = $("adChart");
  $("adPeak").hidden = true;
  $("btnAdOpen").hidden = !d;
  if (!d) {
    svg.innerHTML = "";
    $("adTitle").textContent = "Detalhe do alerta";
    $("adSub").textContent = "Não achei esse alerta.";
    $("adNote").hidden = true;
    return;
  }
  const e = d.event, lv = lvlCls(e), color = LVL_COLOR[lv] || LVL_COLOR.atencao;
  $("adTitle").textContent = `${lvlName(e)} · ${whenLabel(e.time)}` + (e.active ? " · ativo agora" : "");
  const sub = [];
  sub.push(e.active ? `desde ${hhmm(e.time)} · ${fmtDur(d.end - e.time)}` : `${hhmmss(e.time)}–${hhmmss(d.end)} · durou ${fmtDur(d.end - e.time)}`);
  if (e.peak) sub.push(`pico ${fmt(e.peak, 0)} CPM às ${hhmmss(e.peakAt || e.time)}` + (bgNow() ? ` (${fmtSig(sigmaOf(e.peak, bgNow()))} acima do fundo de ${fmt(bgNow(), 0)})` : ""));
  sub.push(`limite ${fmt(d.limit, 0)} CPM`);
  $("adSub").textContent = sub.join(" · ");

  const secs = d.seconds || [], mins = d.minutes || [];
  const t0 = d.from, t1 = Math.max(d.to, t0 + 60e3);
  let top = Math.max(d.limit || 0, e.peak || 0);
  for (const p of secs) top = Math.max(top, p.cpm);
  for (const p of mins) top = Math.max(top, p.cpm);
  top = niceTop(top * 1.12);
  const x = (t) => Math.max(0, Math.min(W, ((t - t0) / (t1 - t0)) * W));
  const y = (v) => H - (Math.min(top, Math.max(0, v)) / top) * H;
  const ys = $("adY").children;
  for (let i = 0; i < 4; i++) ys[i].textContent = fmt(top - (top / 3) * i, 0);
  let out = [0, 1, 2, 3].map((i) => {
    const gy = i === 3 ? H - 0.5 : i === 0 ? 0.5 : (H / 3) * i;
    return `<line x1="0" y1="${gy}" x2="${W}" y2="${gy}" stroke="#2A3136" stroke-width="1" vector-effect="non-scaling-stroke"/>`;
  }).join("");
  // faixa do alerta
  const xa = x(e.time), xb = x(d.end);
  out += `<rect x="${xa.toFixed(1)}" y="0" width="${Math.max(1.5, xb - xa).toFixed(1)}" height="${H}" fill="${color}" fill-opacity="0.13"/>`;
  // limite
  const yl = y(d.limit).toFixed(1);
  out += `<line x1="0" y1="${yl}" x2="${W}" y2="${yl}" stroke="${color}" stroke-width="1.5" stroke-dasharray="6 4" vector-effect="non-scaling-stroke"/>`;
  // por minuto (degraus); mais apagado quando há os segundos
  if (mins.length) {
    const d1 = mins.map((p, i) => {
      const a = x(Math.max(p.t, t0)), b = x(p.t + 60e3), yy = y(p.cpm).toFixed(1);
      const cont = i && mins[i - 1].t + 60e3 === p.t;
      return `${cont ? "L" : "M"}${a.toFixed(1)} ${yy}L${b.toFixed(1)} ${yy}`;
    }).join("");
    out += secs.length
      ? `<path d="${d1}" fill="none" stroke="#A3ADB4" stroke-opacity="0.55" stroke-width="1.5" vector-effect="non-scaling-stroke"/>`
      : `<path d="${d1}" fill="none" stroke="#6EE7A0" stroke-width="2" vector-effect="non-scaling-stroke"/>`;
  }
  if (secs.length > 1) {
    const d2 = secs.map((p, i) => (i && p.t - secs[i - 1].t <= 10e3 ? "L" : "M") + `${x(p.t).toFixed(1)} ${y(p.cpm).toFixed(1)}`).join("");
    out += `<path d="${d2}" fill="none" stroke="#6EE7A0" stroke-width="1.6" stroke-linejoin="round" vector-effect="non-scaling-stroke"/>`;
  }
  svg.innerHTML = out;
  // pico (bolinha em HTML para não esticar)
  if (e.peak && e.peakAt) {
    const pk = $("adPeak");
    pk.hidden = false;
    pk.style.left = (x(e.peakAt) / W) * 100 + "%";
    pk.style.top = (y(e.peak) / H) * 100 + "%";
    pk.style.background = color;
    pk.title = `pico ${fmt(e.peak, 0)} CPM às ${hhmmss(e.peakAt)}`;
  }
  const xl = t1 - t0 <= 20 * 60e3 ? hhmmss : hhmm;
  const xs = $("adX").children;
  xs[0].textContent = xl(t0); xs[1].textContent = xl(t0 + (t1 - t0) / 2); xs[2].textContent = xl(t1);
  $("adLegend").innerHTML =
    (secs.length ? '<span class="key"><i style="border-color:#6EE7A0"></i>leitura a cada segundo</span><span class="key"><i style="border-color:#A3ADB4"></i>valor do minuto (gráfico principal)</span>'
      : '<span class="key"><i style="border-color:#6EE7A0"></i>valor do minuto (gráfico principal)</span>') +
    `<span class="key"><i style="border-color:${color};border-top-style:dashed"></i>limite</span>` +
    `<span class="key"><b class="sw" style="background:${color}"></b>alerta</span>`;
  const note = $("adNote");
  note.hidden = !!(secs.length && d.detailed);
  note.textContent = e.retro ? "Alerta achado no histórico do contador: só há o valor de cada minuto."
    : !secs.length ? "Só há o valor de cada minuto. O detalhe por segundo é guardado nos alertas ao vivo a partir da versão 0.22.0."
    : "O detalhe por segundo não cobre o alerta inteiro (o app foi fechado ou reaberto no meio).";
}

function setupAlertDetail() {
  const plot = $("adPlot");
  plot.addEventListener("mousemove", (ev) => {
    const d = state.ad;
    if (!d) return;
    const r = plot.getBoundingClientRect();
    const t0 = d.from, t1 = Math.max(d.to, t0 + 60e3);
    const t = t0 + ((ev.clientX - r.left) / r.width) * (t1 - t0);
    const src = (d.seconds || []).length ? d.seconds : (d.minutes || []).map((p) => ({ t: p.t + 30e3, cpm: p.cpm, m: p.t }));
    if (!src.length) return;
    const p = src.reduce((a, q) => (Math.abs(q.t - t) < Math.abs(a.t - t) ? q : a), src[0]);
    $("adCursor").hidden = false;
    $("adCursor").style.left = ((p.t - t0) / (t1 - t0)) * 100 + "%";
    const tip = $("adTip");
    tip.hidden = false;
    tip.textContent = p.m != null ? `${hhmm(p.m)} · ${fmt(p.cpm, 0)} CPM (minuto)` : `${hhmmss(p.t)} · ${fmt(p.cpm, 0)} CPM`;
    tip.style.left = Math.max(0, Math.min(ev.clientX - r.left + 12, r.width - tip.offsetWidth - 4)) + "px";
  });
  plot.addEventListener("mouseleave", () => { $("adCursor").hidden = true; $("adTip").hidden = true; });
  $("btnAdOpen").addEventListener("click", () => { const d = state.ad; if (d) showRange(d.from, Math.max(d.to, d.from + 2 * 60e3)); });
  // alerta ativo: atualiza o gráfico a cada 5 s enquanto a tela Alertas estiver aberta
  setInterval(() => { if (state.view === "alertas" && state.ad && state.ad.event.active) loadAlertDetail(); }, 5000);
}

async function refreshAlerts() {
  const [all, active] = await Promise.all([api.GetEvents("alerta"), api.GetActiveAlerts()]);
  state.activeAlerts = active || [];
  renderActive();
  const ul = $("alerts");
  ul.textContent = "";
  const recent = groupEpisodes(all || []).filter((e) => !e.active).slice(0, 3);
  if (!recent.length) ul.innerHTML = '<li class="muted">Nenhum alerta ainda.</li>';
  else recent.forEach((e) => {
    const li = alertLi(e, true);
    li.classList.add("pick");
    li.title = "Ver o gráfico deste alerta";
    li.addEventListener("click", () => { state.alertSel = e.id; showView("alertas"); });
    ul.appendChild(li);
  });
  if (state.view === "alertas") renderAlertHistory(all || []);
}

async function openAlertHistory() {
  renderAlertHistory(await api.GetEvents("alerta"));
  showView("alertas", true);
}

function wireAlertHistory() {
  wireBackup();
  wireLogs();
  wireReports();
  wireNotes();
  setupAlertDetail();
  setupRainImpact();
  setupSummary();
  $("btnEvents").addEventListener("click", openEvents);
  document.querySelectorAll("#alertFilter button").forEach((b) => b.addEventListener("click", async () => {
    state.alertFilter = b.dataset.af;
    renderAlertHistory(await api.GetEvents("alerta"));
  }));
}

async function refreshEventsList() { renderList($("eventsList"), await api.GetEvents("evento"), 1000, "Nada por enquanto.", false); }
async function refreshEvents() {
  await refreshAlerts();
  if ($("eventsDlg").open) await refreshEventsList();
}
function openEvents() { refreshEventsList(); showEvTab(state.evTab || "log"); $("eventsDlg").showModal(); }

// ---------- visor do log técnico ----------
// Níveis, componente, busca, histograma empilhado por nível (clicar numa
// barra dá zoom nela) e "ao vivo" (atualiza a cada 3 s com a janela aberta).

const LG_COLORS = { DEBUG: "#5F6A71", INFO: "#7CB7FF", WARN: "#F5B84A", ERROR: "#F0656A" };
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


// ---------- telas da coluna da direita ----------
// Gráficos (padrão), Medir objeto, Alertas e Fora de casa ocupam o
// lugar dos gráficos, com espaço de sobra; a leitura ao vivo continua à
// esquerda.

state.view = "graficos";
const VIEW_IDS = { resumo: "viewSummary", medir: "viewMeasure", alertas: "viewAlerts", fora: "viewPeriods", chuva: "viewRain", radiacode: "viewRadiacode" };

async function showView(v, skipRender) {
  if (!VIEW_IDS[v]) v = "graficos";
  state.view = v;
  document.querySelectorAll(".col-b .view-graficos").forEach((el) => { el.hidden = v !== "graficos"; });
  for (const [k, id] of Object.entries(VIEW_IDS)) $(id).hidden = k !== v;
  document.querySelectorAll("#viewTabs button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.view === v)));
  if (v === "graficos") { refreshChart(); refreshDose(); refreshHeat(); }
  if (window.rcDemo) { if (v === "radiacode") window.rcDemo.show(); else window.rcDemo.hide(); }
  if (skipRender) return;
  if (v === "alertas") renderAlertHistory(await api.GetEvents("alerta"));
  if (v === "fora") { renderPeriods(); editPeriod(null); }
  if (v === "chuva") refreshRainImpact(true);
  if (v === "resumo") refreshSummary();
  if (v === "medir") { renderMeasure(await api.GetMeasure()); setMeasureMode(state.mMode); }
  if (v === "radiacode") { refreshRcCompare(); if (api.GetRadiacode) renderRc(await api.GetRadiacode()); }
}

document.addEventListener("click", (e) => {
  const b = e.target.closest("[data-view]");
  if (b) showView(b.dataset.view);
});

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
  if (state.settings.rainEnabled) {
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
  $("chartTitle").textContent = usvMode ? "µSv/h ao longo do tempo" : "CPM ao longo do tempo";
  const vals = (usvMode ? c.usv : c.values) || [];
  const thrV = usvMode ? c.attentionUSv : c.attentionCPM || 50;
  let max = 0;
  for (const v of vals) if (v != null && v > max) max = v;
  // picos dos alertas (leitura de segundo): entram na escala e viram ponto
  const kU = c.alarmCPM ? c.alarmUSv / c.alarmCPM : 0;
  const peakV = (p) => (usvMode ? p.cpm * kU : p.cpm);
  for (const p of c.peaks || []) max = Math.max(max, peakV(p));
  // RadiaCode: só no gráfico em µSv/h (é a unidade que os dois têm em comum)
  const rcVals = usvMode && c.rc ? c.rc : null;
  if (rcVals) for (const v of rcVals) if (v != null && v > max) max = v;
  // durante um alarme, mostra as linhas do seu limite e do perigo do app
  const ringing = !!alarmCtl.kind;
  const alarmV = usvMode ? c.alarmUSv : c.alarmCPM;
  const dangerV = usvMode ? c.dangerUSv : c.dangerCPM;
  const testDanger = alarmCtl.kind === "test" && alarmCtl.sev === "perigo";
  // o perigo só entra na escala se a leitura estiver perto dele; se não,
  // aparece na legenda como "acima da escala" (para não achatar o gráfico)
  const dangerInScale = ringing && dangerV && (alarmCtl.sev === "perigo" || max >= dangerV * 0.4) && !testDanger;
  let want = max;
  if (ringing && alarmV) want = Math.max(want, alarmV);
  if (dangerInScale) want = Math.max(want, dangerV);
  const top = usvMode ? niceTop(Math.max(thrV * 1.2, want * 1.15)) : Math.max(60, Math.ceil((want * 1.15) / 20) * 20);
  c.top = top;
  const yd = usvMode ? (top < 0.3 ? 3 : top < 10 ? 2 : 1) : 0;
  const ys = $("yAxis").children;
  for (let i = 0; i < 4; i++) ys[i].textContent = fmt(top - (top / 3) * i, i === 3 ? 0 : yd);

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
  let rcLine = "";
  if (rcVals) {
    let pts = [];
    const out = () => { if (pts.length) rcLine += "M" + pts.join("L") + (pts.length === 1 ? "l0.01 0" : ""); pts = []; };
    rcVals.forEach((v, i) => { if (v == null) out(); else pts.push(x(i).toFixed(1) + " " + y(v).toFixed(1)); });
    out();
  }
  $("rcKey").hidden = !c.rc;
  $("rcKey").classList.toggle("off", !usvMode);
  $("rcKeyText").textContent = usvMode ? "RadiaCode" : "RadiaCode (só em µSv/h)";

  const grid = [0, 1, 2, 3].map((i) => {
    const gy = i === 3 ? H - 0.5 : i === 0 ? 0.5 : (H / 3) * i;
    return `<line x1="0" y1="${gy}" x2="${W}" y2="${gy}" stroke="#2A3136" stroke-width="1" vector-effect="non-scaling-stroke"/>`;
  }).join("");
  const hline = (v, color, width) => `<line x1="0" y1="${y(v).toFixed(1)}" x2="${W}" y2="${y(v).toFixed(1)}" stroke="${color}" stroke-width="${width}" stroke-dasharray="8 5" vector-effect="non-scaling-stroke"/>`;
  let thr = top >= thrV ? hline(thrV, "#F5B84A", 1) : "";
  const showAlarm = ringing && alarmV && alarmV <= top;
  const showDanger = ringing && dangerV && dangerV <= top;
  if (showAlarm && !(dangerV && alarmV >= dangerV)) thr += hline(alarmV, "#F28C38", 2);
  if (showDanger) thr += hline(dangerV, "#F0656A", 2);
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
  const rainBars = rainy.map((p) => bar(p.time, p.time + 3600e3, Math.max(4, Math.min(1, p.mm / 5) * H * 0.3), "#7CC7FF", 0.3)).join("") +
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
    const line = `<line x1="${nx.toFixed(1)}" y1="0" x2="${nx.toFixed(1)}" y2="${H}" stroke="#B794F6" stroke-width="1.2" stroke-dasharray="3 3" vector-effect="non-scaling-stroke"/>`;
    if (!nt.end) return line;
    const ex = noteX(Math.min(nt.end, tEnd));
    return `<rect x="${nx.toFixed(1)}" y="0" width="${Math.max(1, ex - nx).toFixed(1)}" height="${H}" fill="#B794F6" fill-opacity="0.10"/>` + line +
      `<line x1="${ex.toFixed(1)}" y1="0" x2="${ex.toFixed(1)}" y2="${H}" stroke="#B794F6" stroke-opacity="0.5" stroke-width="1" stroke-dasharray="3 3" vector-effect="non-scaling-stroke"/>`;
  }).join("");
  $("noteKey").hidden = !(c.notes || []).length;
  // ponto de cada pico, em cima do ponto do gráfico daquele trecho
  const stepMs = c.stepMs || 60000;
  $("peakDots").innerHTML = (c.peaks || []).map((p) => {
    const i = Math.floor((p.t - c.from) / stepMs);
    if (i < 0 || i >= n) return "";
    const col = { perigo: "#F0656A", alarme: "#F28C38" }[p.level] || "#F5B84A";
    return `<i style="left:${((x(i) / W) * 100).toFixed(2)}%;top:${((y(peakV(p)) / H) * 100).toFixed(2)}%;background:${col}"></i>`;
  }).join("");
  $("chart").innerHTML = grid + away + meas + rainBars + thr + noteLines +
    `<path d="${area}" fill="#6EE7A0" fill-opacity="0.12"/>` +
    `<path d="${line}" fill="none" stroke="#6EE7A0" stroke-width="2" stroke-linejoin="round" stroke-linecap="round" vector-effect="non-scaling-stroke"/>` +
    (rcLine ? `<path d="${rcLine}" fill="none" stroke="#F472B6" stroke-width="1.6" stroke-linejoin="round" stroke-linecap="round" vector-effect="non-scaling-stroke"/>` : "");

  if (state.zoom) {
    const f = chartTimeFmt(c);
    const t0 = c.times && c.times.length ? c.times[0] : c.from;
    $("x0").textContent = f(t0);
    $("x1").textContent = f((t0 + tEnd) / 2);
    $("x2").textContent = f(tEnd);
  } else {
    const lbl = state.range === "7d" ? ddmm : hhmm;
    $("x0").textContent = lbl(c.from);
    $("x1").textContent = lbl((c.from + c.to) / 2);
    $("x2").textContent = state.range === "7d" ? "hoje" : "agora";
  }
  // nos períodos curtos o gráfico usa leituras por segundo
  $("chartStats").dataset.short = String(isShortRange());
  const attnC = c.attentionCPM || 50;
  $("legendText").textContent = usvMode
    ? `atenção: ${fmt(c.attentionUSv, 3)} µSv/h`
    : `atenção: ${fmt(attnC, 0)} CPM`;
  $("chartStats").textContent = c.avg == null
    ? "sem dados no período"
    : usvMode
      ? `média ${fmt(c.avgUSv, 3)} · mín ${fmt(c.minUSv, 3)} · máx ${fmt(c.maxUSv, 3)} µSv/h` + (c.rcAvg != null ? ` · RadiaCode média ${fmt(c.rcAvg, 3)}` : "")
      : `média ${fmt(c.avg, 1)} · mín ${fmt(c.min, 0)} · máx ${fmt(c.max, 0)}`;
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
    const when = state.zoom ? chartTimeFmt(c)(t) : state.range === "7d" ? `${ddmm(t)} ${hhmm(t)}` : isShortRange() ? hhmmss(t) : hhmm(t);
    const rr = (c.radar || []).find((p) => t >= p.time && t < p.time + RADAR_STEP);
    const pk = (c.peaks || []).filter((p) => p.t >= t && p.t < t + (c.stepMs || 60000)).sort((a, b) => b.cpm - a.cpm)[0];
    const kU = c.alarmCPM ? c.alarmUSv / c.alarmCPM : 0;
    const pkText = pk && !(pk.retro && v != null && Math.round(pk.cpm) === Math.round(usvMode ? v / kU : v))
      ? ` · pico ${usvMode ? fmt(pk.cpm * kU, usvDigits(pk.cpm * kU)) + " µSv/h" : fmt(pk.cpm, 0) + " CPM"} às ${hhmmss(pk.t)}` : "";
    const rh = rr ? null : (c.rain || []).find((p) => t >= p.time && t < p.time + 3600e3 && p.mm >= 0.1);
    const half = Math.max(60e3, c.stepMs || 60000);
    const nt = (c.notes || []).filter((n) => (n.end ? t >= n.time - half && t <= n.end : Math.abs(n.time - t) <= half)).map((n) => n.text);
    const ntText = nt.length ? ` · nota: ${nt.join("; ")}` : "";
    const rcv = c.rc ? c.rc[i] : null;
    const rcText = rcv != null ? ` · RadiaCode ${fmt(rcv, usvDigits(rcv))} µSv/h` : "";
    tip.textContent = (v == null ? `${when} · sem dados`
      : usvMode ? `${when} · ${fmt(v, usvDigits(v))} µSv/h`
      : `${when} · ${fmt(v, (c.stepMs || 0) >= 300000 ? 1 : 0)} CPM`) + pkText + (rr && rr.mmh > 0 ? ` · chuva ${rainWord(rr.mmh)} (radar)` : rh ? ` · chuva ${fmt(rh.mm, 1)} mm (modelo)` : "") + rcText + awayLabel(t) + measureLabel(t) + ntText;
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

async function refreshDose() {
  document.querySelectorAll("#doseSeg button").forEach((b) => b.setAttribute("aria-pressed", String(!state.doseZoom && b.dataset.dperiod === state.dosePeriod)));
  $("doseLimit").value = state.doseLimit;
  const my = ++doseSeq;
  const c = state.doseZoom
    ? await api.GetDoseRange(state.doseZoom.from, state.doseZoom.to, state.doseLimit)
    : await api.GetDose(state.dosePeriod, state.doseLimit);
  if (my !== doseSeq) return;
  state.dose = c;
  const dz = $("doseZoomBar");
  dz.hidden = !state.doseZoom;
  if (state.doseZoom) {
    $("doseZoomRange").textContent = zoomRangeText(c.from, c.to);
    $("doseZoomStats").textContent = state.zoomLinked
      ? `· mesmo trecho do gráfico de CPM · dose só nesse trecho, começando do zero`
      : `· dose acumulada só nesse trecho, começando do zero · um ponto a cada ${stepText(c.stepMs)}`;
  }
  renderZoomButtons();

  $("doseNote").textContent =
    "Os limites da CNEN e da ICRP não contam a radiação natural nem exames médicos; por isso o teto é o seu fundo medido mais o limite anual, proporcional ao tempo medido. " +
    "O contador só mede radiação externa (gama e beta): radônio inalado e outras fontes internas ficam de fora. Minutos sem leitura não entram em nenhuma das linhas.";

  if (!c.measuredMinutes) {
    $("doseTotal").textContent = "—";
    $("doseCmp").hidden = true;
    $("doseSub").textContent = "sem dados no período";
    $("doseStatus").textContent = "";
    $("doseChart").innerHTML = "";
    ["dx0", "dx1", "dx2"].forEach((id) => { $(id).textContent = ""; });
    return;
  }

  const [tv, tu] = fmtDose(c.total);
  const cmp = doseCompare(c.total);
  $("doseCmpTxt").textContent = cmp;
  $("doseCmp").hidden = !cmp;
  $("doseTotal").textContent = tv;
  $("doseTotal").nextElementSibling.textContent = tu;
  const cov = c.coverage * 100;
  const perYear = c.backgroundRate * 8766 / 1000; // mSv por ano no ritmo do fundo
  $("doseSub").textContent = `fundo ${fmt(c.backgroundRate, 3)} µSv/h (${c.backgroundSource}) · ≈ ${fmt(perYear, perYear < 10 ? 2 : 1)} mSv/ano nesse ritmo · ${fmt(cov, cov < 10 ? 1 : 0)}% do período medido`;

  const st = $("doseStatus");
  const excess = Math.max(0, c.excess);
  const pct = c.allowance > 0 ? (excess / c.allowance) * 100 : 0;
  const over = c.excess > c.allowance;
  st.className = "dose-status" + (over ? " over" : "");
  let line;
  if (c.excess <= 0) line = "<strong>Dentro do teto.</strong> A dose ficou no nível do seu fundo natural.";
  else if (!over) line = `<strong>Dentro do teto.</strong> Acima do fundo: ${doseTxt(excess)} de ${doseTxt(c.allowance)} permitidos no período (${fmt(pct, pct < 10 ? 1 : 0)}%).`;
  else line = `<strong>Acima do teto.</strong> Acima do fundo: ${doseTxt(excess)}; o permitido no período era ${doseTxt(c.allowance)}.`;
  st.innerHTML = `<span>${line}</span><div class="meter" aria-hidden="true"><div style="width:${Math.min(100, pct)}%"></div></div>`;

  // gráfico
  const n = c.times.length;
  const top0 = Math.max(c.total, c.ceilingTotal) * 1.1 || 1;
  const mag = Math.pow(10, Math.floor(Math.log10(top0)));
  const top = Math.ceil(top0 / mag * 2) / 2 * mag;
  const x = (i) => (n <= 1 ? 0 : (i / (n - 1)) * W);
  const y = (v) => H - (v / top) * H;
  const path = (arr) => arr.map((v, i) => (i ? "L" : "M") + x(i).toFixed(1) + " " + y(v).toFixed(1)).join("");
  const dosePath = path(c.dose);
  const grid = [0, 1, 2, 3].map((i) => {
    const gy = i === 3 ? H - 0.5 : i === 0 ? 0.5 : (H / 3) * i;
    return `<line x1="0" y1="${gy}" x2="${W}" y2="${gy}" stroke="#2A3136" stroke-width="1" vector-effect="non-scaling-stroke"/>`;
  }).join("");
  const dt0 = c.times[0], dt1 = c.times[n - 1];
  const dxt = (t) => Math.max(0, Math.min(W, ((t - dt0) / Math.max(1, dt1 - dt0)) * W));
  const dAway = awayBands(dt0, dt1, dxt);
  $("doseAwayKey").hidden = !dAway;
  $("doseChart").innerHTML = grid + dAway +
    `<path d="${dosePath}L${W} ${H}L0 ${H}Z" fill="#6EE7A0" fill-opacity="0.10"/>` +
    `<path d="${path(c.background)}" fill="none" stroke="#A3ADB4" stroke-width="1.5" stroke-dasharray="5 5" vector-effect="non-scaling-stroke"/>` +
    `<path d="${path(c.ceiling)}" fill="none" stroke="#F0656A" stroke-width="2" vector-effect="non-scaling-stroke"/>` +
    `<path d="${dosePath}" fill="none" stroke="#6EE7A0" stroke-width="2" stroke-linejoin="round" vector-effect="non-scaling-stroke"/>`;
  const ys = $("doseY").children;
  for (let i = 0; i < 3; i++) ys[i].textContent = fmtDose(top - (top / 3) * i)[0];
  ys[3].textContent = "0";

  if (state.doseZoom) {
    const f = chartTimeFmt(c);
    $("dx0").textContent = f(c.from);
    $("dx1").textContent = f((c.from + c.to) / 2);
    $("dx2").textContent = f(c.to);
    return;
  }
  const hourly = ["3h", "6h", "12h", "24h"].includes(state.dosePeriod);
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
    const when = ["3h", "6h", "12h", "24h"].includes(state.dosePeriod) ? hhmm(t) : `${ddmm(t)} ${hhmm(t)}`;
    tip.textContent = `${when} · dose ${doseTxt(c.dose[i])} · teto ${doseTxt(c.ceiling[i])}`;
    const left = (ev.clientX - r.left) + 12;
    tip.style.left = Math.max(0, Math.min(left, r.width - tip.offsetWidth - 4)) + "px";
  });
  plot.addEventListener("mouseleave", () => { $("doseCursor").hidden = true; $("doseTip").hidden = true; });
}

function wireDose() {
  document.querySelectorAll("#doseSeg button").forEach((b) => b.addEventListener("click", () => {
    state.dosePeriod = b.dataset.dperiod; localSet("dosePeriod", state.dosePeriod); state.doseZoom = null; state.doseZoomStack = []; unlinkZoom(); refreshDose();
  }));
  $("doseLimit").addEventListener("change", () => {
    state.doseLimit = $("doseLimit").value; localSet("doseLimit", state.doseLimit); refreshDose();
  });
  setupDoseHover();
  setupDoseZoom();
}

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
  }
  renderStatus();
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
  let h;
  if (mm) {
    if (api.GetDataStart) state.heatDataStart = (await api.GetDataStart()) || 0;
    const hm = state.heatMonth, now = new Date();
    const label = new Date(hm.y, hm.m - 1, 1).toLocaleDateString(LOCALE, { month: "long", year: "numeric" });
    $("heatMonthLabel").textContent = label.charAt(0).toUpperCase() + label.slice(1);
    $("heatPrev").disabled = !!state.heatDataStart && heatMonthStart(hm) <= state.heatDataStart;
    $("heatNext").disabled = hm.y > now.getFullYear() || (hm.y === now.getFullYear() && hm.m >= now.getMonth() + 1);
    h = await api.GetHeatmapMonth(hm.y, hm.m);
  } else {
    h = await api.GetHeatmap(state.heatDays);
  }
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
        c.style.background = `rgba(110,231,160,${(0.15 + 0.85 * ((v - mn) / span)).toFixed(3)})`;
      }
    }
  }
  markHeatPick();
  $("heatMin").textContent = mn == null ? "—" : fmt(mn, 0);
  $("heatMax").textContent = mx == null ? "—" : fmt(mx, 0);
  const daysWith = h.values.filter((row) => row.some((v) => v != null)).length;
  if (h.peakHour < 0) $("heatSub").textContent = "sem dados ainda";
  else if (daysWith < 2) $("heatSub").textContent = "com mais dias de dados dá para ver se o fundo muda com a hora";
  else {
    const p = h.hourAvg[h.peakHour], l = h.hourAvg[h.lowHour];
    $("heatSub").textContent = `mais alto por volta das ${h.peakHour}h (${fmt(p, 1)} CPM) · mais baixo por volta das ${h.lowHour}h (${fmt(l, 1)} CPM)`;
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
    line(`${fmt(v, 1)} CPM`, "ht-val");
    const u = h.usv && h.usv[i] ? h.usv[i][hr] : null;
    if (u != null) line(`≈ ${fmt(u, usvDigits(u))} µSv/h`);
    const avg = h.hourAvg[hr];
    const n = h.counts && h.counts[i] ? h.counts[i][hr] : 0;
    if (avg != null && h.days.length > 1) {
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
    line(`fora de casa: ${ac} min${av != null ? ` · ${fmt(av, 1)} CPM` : ""} (fora do padrão)`, "ht-away");
  }
  // o mais perigoso primeiro (perigo, alarme, atenção); no mesmo nível, o maior pico
  const rank = { perigo: 0, alarme: 1 };
  const al = [...((h.alerts || {})[`${i}-${hr}`] || [])].sort((x, y) =>
    (rank[x.level] ?? 2) - (rank[y.level] ?? 2) || (y.peak || 0) - (x.peak || 0) || x.time - y.time);
  for (const a of al.slice(0, 4)) {
    const nm = { perigo: "Perigo", alarme: "Alarme" }[a.level] || "Atenção";
    line(`${nm} às ${hhmm(a.time)}${a.peak ? " · pico " + fmt(a.peak, 0) + " CPM" : ""}${a.retro ? " (do contador)" : ""}`, { perigo: "ht-danger", alarme: "ht-alarm" }[a.level] || "ht-att");
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
  if (!p || !state.heat || !state.zoom || state.zoom.from !== p.from || state.zoom.to !== p.to) return;
  const i = state.heat.days.indexOf(p.day);
  if (i < 0) return;
  document.querySelectorAll(`#heat .c[data-i="${i}"]`).forEach((c) => {
    const h = +c.dataset.h;
    if (h >= p.h0 && h <= p.h1) c.classList.add("pick");
  });
}

function openHeatRange(i, h0, h1) {
  const from = heatCellTime(i, h0), to = heatCellTime(i, h1) + 3600e3;
  state.heatPick = { day: state.heat.days[i], h0, h1, from, to };
  state.zoom = { from, to };
  state.zoomStack = [];
  state.doseZoom = { from, to };
  state.doseZoomStack = [];
  state.zoomLinked = true;
  refreshChart().then(markHeatPick);
  refreshDose();
  const card = $("cpmCard");
  const r = card.getBoundingClientRect();
  if (r.top < 0 || r.bottom > window.innerHeight) card.scrollIntoView({ behavior: "smooth", block: "start" });
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

// ---------- medir um objeto ----------

// ---------- gráfico durante a medição ----------
// CPM ao vivo (linha fina) e a média acumulada de cada fase com a margem de
// erro (faixa), segundo a segundo.

const mlog = { start: 0, live: [], ph: [], demo: false };

function measureLogStep(st) {
  const active = st.phase === "fundo" || st.phase === "objeto";
  if (!st.phase) { mlog.start = 0; mlog.live = []; mlog.ph = []; return; }
  const fresh = st.elapsed <= 1 && (st.phase === "fundo" || (st.phase === "objeto" && st.bg && st.bg.source === "casa"));
  if (fresh) { mlog.start = Date.now(); mlog.live = []; mlog.ph = []; mlog.demo = !!st.demo; state.mSel = null; if (state.mMode === "salvas") setMeasureMode("nova"); }
  if (!mlog.start) mlog.start = Date.now();
  if (active && st.elapsed > 0) {
    const m = st.elapsed / 60;
    // ao recomeçar a fase do objeto, tira o que tinha dela
    if (st.phase === "objeto" && st.elapsed <= 1) mlog.ph = mlog.ph.filter((p) => p.phase !== "objeto");
    mlog.ph.push({ t: Date.now(), phase: st.phase, avg: st.counts / m, err: Math.sqrt(Math.max(st.counts, 1)) / m });
  }
}

function measureLogReading(r) {
  const st = state.measureSt;
  if (!st || (st.phase !== "fundo" && st.phase !== "objeto") || !!r.demo !== !!st.demo) return;
  mlog.live.push({ t: Date.now(), cpm: r.cpm });
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
  return { start: mlog.start, live: mlog.live, ph: mlog.ph, seconds: st.seconds || 60, expectSec, bgFlat: home ? st.bg : null };
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
  return { start, live, ph, seconds: m.bg.seconds || 60, done: true, bgFlat: home ? m.bg : null };
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
  const tAll = [...src.live.map((p) => p.t), ...src.ph.map((p) => p.t)];
  // o eixo vai até o fim previsto da medição (fundo + objeto), para ir enchendo
  const expected = src.done ? 0 : (src.expectSec || src.seconds * 2 + 15) * 1000;
  const t1 = Math.max(t0 + expected, t0 + 10e3, ...tAll);
  src.t1 = t1;
  let top = 0;
  for (const p of src.live) top = Math.max(top, p.cpm);
  // a média dos primeiros segundos pula muito (1 contagem em 1 s = 60 CPM):
  // ela não entra na escala; a faixa de erro também pode passar do topo
  const first = {};
  for (const p of src.ph) { if (!(p.phase in first)) first[p.phase] = p.t; if (p.t - first[p.phase] >= 5000) top = Math.max(top, p.avg); }
  if (src.bgFlat) top = Math.max(top, src.bgFlat.cpm + src.bgFlat.errCPM);
  top = Math.max(10, Math.ceil((top * 1.15) / 10) * 10);
  const x = (t) => ((t - t0) / (t1 - t0)) * W;
  const y = (v) => H - (Math.min(top, Math.max(0, v)) / top) * H;
  const ys = $("mY").children;
  for (let i = 0; i < 4; i++) ys[i].textContent = fmt(top - (top / 3) * i, 0);
  let out = [0, 1, 2, 3].map((i) => {
    const gy = i === 3 ? H - 0.5 : i === 0 ? 0.5 : (H / 3) * i;
    return `<line x1="0" y1="${gy}" x2="${W}" y2="${gy}" stroke="#2A3136" stroke-width="1" vector-effect="non-scaling-stroke"/>`;
  }).join("");
  if (src.bgFlat) {
    // fundo da casa: faixa reta (média ± σ) na medição toda
    const b = src.bgFlat, y1 = y(b.cpm + b.errCPM), y2 = y(b.cpm - b.errCPM);
    out += `<rect x="0" y="${y1.toFixed(1)}" width="${W}" height="${Math.max(1, y2 - y1).toFixed(1)}" fill="#A3ADB4" fill-opacity="0.18"/>`;
    out += `<line x1="0" y1="${y(b.cpm).toFixed(1)}" x2="${W}" y2="${y(b.cpm).toFixed(1)}" stroke="#A3ADB4" stroke-width="2" stroke-dasharray="6 4" vector-effect="non-scaling-stroke"/>`;
  }
  for (const [phase, color] of [["fundo", "#A3ADB4"], ["objeto", "#5EEAD4"]]) {
    const pts = src.ph.filter((p) => p.phase === phase);
    if (!pts.length) continue;
    const up = pts.map((p) => `${x(p.t).toFixed(1)} ${y(p.avg + p.err).toFixed(1)}`);
    const dn = pts.slice().reverse().map((p) => `${x(p.t).toFixed(1)} ${y(p.avg - p.err).toFixed(1)}`);
    out += `<path d="M${up.join("L")}L${dn.join("L")}Z" fill="${color}" fill-opacity="0.18"/>`;
    out += `<path d="M${pts.map((p) => `${x(p.t).toFixed(1)} ${y(p.avg).toFixed(1)}`).join("L")}" fill="none" stroke="${color}" stroke-width="2.5" vector-effect="non-scaling-stroke"/>`;
    if (phase === "objeto") {
      const xs = x(pts[0].t).toFixed(1);
      out += `<line x1="${xs}" y1="0" x2="${xs}" y2="${H}" stroke="#5EEAD4" stroke-width="1" stroke-dasharray="3 3" vector-effect="non-scaling-stroke"/>`;
    }
  }
  if (src.live.length > 1) {
    // quebra a linha nos intervalos (ex.: entre o fundo e o objeto)
    const d = src.live.map((p, i) => (i && p.t - src.live[i - 1].t <= 3000 ? "L" : "M") + `${x(p.t).toFixed(1)} ${y(p.cpm).toFixed(1)}`).join("");
    out += `<path d="${d}" fill="none" stroke="#6EE7A0" stroke-opacity="0.55" stroke-width="1.2" vector-effect="non-scaling-stroke"/>`;
  }
  svg.innerHTML = out;
  const last = (ph) => { const l = src.ph.filter((p) => p.phase === ph); return l[l.length - 1]; };
  const b = last("fundo"), o = last("objeto");
  const parts = [];
  if (b) parts.push(`fundo ${fmt(b.avg, 1)} ± ${fmt(b.err, 1)}`);
  else if (src.bgFlat) parts.push(`fundo da casa ${fmt(src.bgFlat.cpm, 1)} ± ${fmt(src.bgFlat.errCPM, 1)}`);
  if (o) parts.push(`objeto ${fmt(o.avg, 1)} ± ${fmt(o.err, 1)}`);
  $("mLiveSub").textContent = parts.length ? "— " + parts.join(" · ") + " CPM" : "";
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
    tip.textContent = `${Math.round((ref.t - t0) / 1000)} s` + (lv ? ` · ao vivo ${fmt(lv.cpm, 0)} CPM` : "") + (ph ? ` · média (${ph.phase}) ${fmt(ph.avg, 1)} ± ${fmt(ph.err, 1)}` : "");
    tip.style.left = Math.min(ev.clientX - r.left + 12, r.width - tip.offsetWidth - 4) + "px";
  });
  plot.addEventListener("mouseleave", () => { $("mCursor").hidden = true; $("mTip").hidden = true; });
  $("btnMLiveBack").addEventListener("click", () => setMeasureMode("nova"));
}

// tabela com as contas da medição: contagens, tempo, taxa média e a
// incerteza de Poisson (1σ = raiz das contagens, levada para CPM)
const plusMinus = (v) => (v >= 0 ? "+" : "−") + fmt(Math.abs(v), 1);
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
  thead.appendChild(row(["", "contagens", "tempo", "CPM", "± 1σ"]));
  const tb = document.createElement("tbody");
  for (const [nm, x] of [["Fundo", m.bg], ["Com o objeto", m.obj]]) {
    if (!x) continue;
    tb.appendChild(row([nm === "Com o objeto" ? "Objeto" : x.source === "casa" ? "Fundo (casa)" : nm, x.counts.toLocaleString(LOCALE), mDurTxt(x.seconds), fmt(x.cpm, 1), `±${fmt(x.errCPM, 1)}`]));
  }
  const r = m.result;
  if (r) {
    tb.appendChild(row(["Diferença", "", "", plusMinus(r.netCPM), `±${fmt(r.netErr, 1)}`], "m-diff"));
  }
  tbl.append(thead, tb);
  const box = document.createElement("div");
  box.className = "m-stats-box";
  box.appendChild(tbl);
  if (r) {
    const p = document.createElement("p");
    p.className = "small muted";
    const sig = Math.abs(r.sigma);
    p.textContent = `A incerteza de cada linha é a de Poisson: raiz das contagens, convertida para CPM. A da diferença junta as duas (raiz da soma dos quadrados). ` +
      `A diferença vale ${fmt(sig, 1)} vezes a incerteza (${fmt(sig, 1)}σ)` +
      (r.sigma >= 3 ? ": é real, não variação da contagem." : r.sigma >= 2 ? ": provavelmente real, mas vale repetir." : r.sigma <= -3 ? ": o objeto deu abaixo do fundo, então o fundo provavelmente mudou (outro lugar, chuva)." : ": cabe na variação natural da contagem.");
    box.appendChild(p);
  }
  return box;
}

// opções da tela de medir (lembradas entre as vezes)
function measureOpts() {
  const t = $("mSecs").value, auto = t.startsWith("auto");
  return {
    name: $("mName").value, distance: $("mDist").value, auto, homeBg: $("mBgSrc").value === "casa",
    seconds: auto ? 0 : parseInt(t, 10), maxSec: auto ? parseInt(t.slice(4), 10) : 0, pct: parseFloat($("mPct").value),
  };
}

async function refreshHomeBg(force) {
  if (!force && state.homeBgAt && Date.now() - state.homeBgAt < 30e3) return;
  state.homeBgAt = Date.now();
  try { state.homeBg = await api.GetHomeBackground(); } catch (e) { state.homeBg = { ok: false, why: errMsg(e) }; }
  if (state.measureSt) renderMeasure(state.measureSt);
}

const fmtMin = (sec) => (sec < 90 ? `${Math.max(1, Math.round(sec))} s` : `${Math.round(sec / 60)} min`);

function renderMeasure(st) {
  state.measureSt = st;
  measureLogStep(st);
  drawMeasureChart();
  const running = st.phase === "fundo" || st.phase === "objeto";
  const started = running || !!st.bg;
  $("measureTitle").textContent = "Medir um objeto" + (st.demo ? " (demo)" : "");
  for (const id of ["mSecs", "mBgSrc", "mPct", "mDist"]) $(id).disabled = started;
  const o = measureOpts();
  const home = started ? !!(st.bg && st.bg.source === "casa") : o.homeBg;
  const auto = started ? !!st.auto : o.auto;
  $("mPctBox").style.visibility = o.auto ? "" : "hidden";
  if (!started) refreshHomeBg(false);
  const hb = state.homeBg;
  // dica embaixo das opções
  $("mOptHint").textContent = !started ? [
    o.homeBg ? "Fundo da casa: meça o objeto no lugar de sempre do contador (o fundo muda de um canto para outro)." : "O fundo é medido primeiro, com o objeto longe (2 m ou mais)" + (o.auto ? ", por 5 min." : ", pelo mesmo tempo do objeto."),
    o.auto ? `Para quando o objeto estiver claramente acima do fundo, ou quando der para garantir que ele sobe menos de ${o.pct}% do fundo.` : "",
  ].filter(Boolean).join(" ") : "";
  // passo 1: fundo
  $("mBgTitle").textContent = home ? "1. Fundo da casa" : "1. Fundo";
  $("btnMeasureBg").hidden = home;
  $("btnMeasureBg").disabled = running;
  $("btnMeasureBg").textContent = st.bg ? "Medir o fundo de novo" : "Medir o fundo";
  const objReady = !running && (st.bg ? true : home && hb && hb.ok);
  $("btnMeasureObj").disabled = !objReady;
  $("btnMeasureObj").textContent = st.obj ? "Medir de novo com o objeto" : "Medir com o objeto";
  const prog = (phase, fill, val, done) => {
    if (st.phase === phase) {
      const auto = phase === "objeto" && st.auto;
      const total = auto ? (st.eta >= 0 ? st.elapsed + st.eta : st.seconds) : st.seconds;
      $(fill).style.width = Math.min(100, (st.elapsed / Math.max(1, total)) * 100) + "%";
      // até agora: taxa e incerteza com as contagens já feitas
      const so = st.elapsed > 0 ? ` · ${fmt(st.counts * 60 / st.elapsed, 1)} ± ${fmt(Math.sqrt(Math.max(st.counts, 1)) * 60 / st.elapsed, 1)} CPM` : "";
      $(val).textContent = (auto ? `${Math.floor(st.elapsed / 60)}:${pad(st.elapsed % 60)}` : `${st.elapsed}/${st.seconds} s`) + ` · ${st.counts} contagens${so}`;
    } else if (done) {
      $(fill).style.width = "100%";
      $(val).textContent = done.source === "casa"
        ? `${fmt(done.cpm, 1)} ± ${fmt(done.errCPM, 1)} CPM (${Math.round(done.seconds / 60)} min até ${hhmm(done.end)})`
        : `${fmt(done.cpm, 1)} ± ${fmt(done.errCPM, 1)} CPM (${done.counts} em ${done.seconds} s)`;
    } else {
      $(fill).style.width = "0%";
      $(val).textContent = "—";
    }
  };
  if (!st.bg && home) {
    // ainda não começou: mostra o fundo da casa que vai ser usado
    $("mBgFill").style.width = hb && hb.ok ? "100%" : "0%";
    $("mBgVal").textContent = !hb ? "…" : hb.ok ? `${fmt(hb.cpm, 1)} ± ${fmt(hb.errCPM, 1)} CPM (${hb.minutes} min até ${hhmm(hb.to)})` : hb.why;
  } else prog("fundo", "mBgFill", "mBgVal", st.bg);
  prog("objeto", "mObjFill", "mObjVal", st.obj);
  // resultado parcial ao vivo
  const lv = st.phase === "objeto" ? st.live : null;
  $("mObjLive").hidden = !lv;
  if (lv) {
    const sig = lv.sigma;
    let what = sig >= 3 ? "parece radioativo" : sig >= 2 ? "talvez um pouco acima" : "ainda não dá para dizer";
    let when = "";
    if (st.auto) when = st.elapsed < 20 ? " · juntando contagens…" : st.eta > 0 ? ` · ~${fmtMin(st.eta)} para decidir` : " · pode ir até o tempo máximo";
    $("mObjLive").innerHTML = `Até agora: <b>${plusMinus(lv.netCPM)} ± ${fmt(lv.netErr, 1)} CPM</b> sobre o fundo (${fmt(sig, 1)}σ) · ${what}${when}`;
  }
  const r = st.result;
  $("mResult").hidden = !r;
  $("mSaved").hidden = !st.savedId;
  if (st.savedId) state.mLastSaved = st.savedId;
  const rp = $("mReport");
  rp.hidden = !st.savedId;
  if (st.savedId) {
    $("mReportText").textContent = st.reportPath ? "Relatório salvo: " + st.reportPath.split(/[\\/]/).pop()
      : st.reportError ? "Relatório não saiu: " + st.reportError : state.settings.noMeasureReport ? "" : "Gerando o relatório…";
    $("btnMReportOpen").hidden = !st.reportPath;
    $("btnMReportMake").hidden = !!st.reportPath;
    rp.dataset.id = st.savedId;
    rp.dataset.path = st.reportPath || "";
  }
  if (!r) return;
  $("mResult").className = "m-result " + r.level;
  $("mVerdict").textContent = r.verdict.charAt(0).toUpperCase() + r.verdict.slice(1) + ".";
  $("mStats").replaceChildren(measureStats({ bg: st.bg, obj: st.obj, result: r }));
  const sign = r.netCPM >= 0 ? "+" : "−";
  $("mNumbers").textContent = `${sign}${fmt(Math.abs(r.netCPM), 1)} ± ${fmt(r.netErr, 1)} CPM sobre o fundo · ${fmt(r.ratio, 1)}× o fundo · ≈ ${r.netUSv >= 0 ? "+" : "−"}${fmt(Math.abs(r.netUSv), usvDigits(Math.abs(r.netUSv)))} µSv/h no tubo`;
  $("mExplain").textContent = measureExplain(r, st.obj, auto);
}

function measureExplain(r, obj, auto) {
  const t = obj ? fmtMin(obj.seconds) : "";
  if (r.stop === "achou") return `Parou em ${t}: a diferença passou de 4σ, bem acima da variação natural da contagem. O µSv/h vale para o objeto na distância medida; a 1 metro é muito menos.`;
  if (r.stop === "nada") return `Parou em ${t}, quando deu para garantir que o objeto sobe menos de ${fmt(r.pct, 0)}% do fundo (+${fmt(r.threshold, 1)} CPM). Para procurar algo mais fraco, escolha “+10% do fundo”.`;
  if (r.stop === "limite") return `Chegou ao tempo máximo (${t}) sem decidir.` + (r.level === "sim" ? "" : ` Se o objeto emite algo, é menos que +${fmt(r.limit, 1)} CPM; para ir além, aumente o tempo máximo.`);
  return {
    sim: "A diferença é bem maior que a variação natural da contagem. O valor em µSv/h vale para o objeto encostado no contador; a 1 metro é muito menos.",
    talvez: "A diferença está perto do limite da variação natural. Use o modo automático para medir até ter certeza.",
    nada: "Se o objeto emitir alguma coisa, está abaixo do que dá para ver nesse tempo" + (r.limit ? ` (menos que +${fmt(r.limit, 1)} CPM)` : "") + ". Use o modo automático para mais certeza.",
  }[r.level];
}

function wireMeasure() {
  for (const id of ["mSecs", "mBgSrc", "mPct"]) {
    const v = localGet("m-" + id);
    if (v && [...$(id).options].some((o) => o.value === v)) $(id).value = v;
    $(id).addEventListener("change", () => { localSet("m-" + id, $(id).value); renderMeasure(state.measureSt || {}); });
  }
  $("btnMeasureBg").addEventListener("click", async () => {
    try {
      const st = state.measureSt || {};
      // fundo de novo no meio de uma medição: recomeça com as mesmas opções
      renderMeasure(await api.StartMeasure({ ...measureOpts(), homeBg: false }));
      if (st.bg) toast("Medindo o fundo de novo.");
    } catch (e) { toast(errMsg(e), true); }
  });
  $("btnMeasureObj").addEventListener("click", async () => {
    try {
      const st = state.measureSt || {};
      if (st.bg) renderMeasure(await api.StartMeasurePhase("objeto", 0, $("mName").value));
      else renderMeasure(await api.StartMeasure(measureOpts()));
    } catch (e) { toast(errMsg(e), true); }
  });
  $("btnMeasureReset").addEventListener("click", async () => {
    renderMeasure(await api.CancelMeasure());
    refreshHomeBg(true);
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
    for (const [ph, color, op] of [[m.bg, "#A3ADB4", 0.14], [m.obj, "#5EEAD4", 0.24]]) {
      if (ph.source === "casa" || !(ph.end > t0 && ph.start < t1)) continue;
      const x1 = xt(Math.max(ph.start, t0)), x2 = xt(Math.min(ph.end, t1));
      out += `<rect x="${x1.toFixed(1)}" y="0" width="${Math.max(2, x2 - x1).toFixed(1)}" height="${H}" fill="${color}" fill-opacity="${op}"/>`;
    }
    if (m.obj.start > t0 && m.obj.start < t1) {
      const x = xt(m.obj.start).toFixed(1);
      out += `<line x1="${x}" y1="0" x2="${x}" y2="${H}" stroke="#5EEAD4" stroke-width="1" stroke-dasharray="3 3" vector-effect="non-scaling-stroke"/>`;
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
  $(saved ? "mLiveSlotSaved" : "mLiveSlotNew").appendChild($("mLive"));
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
    b.querySelector(".m-item-main .muted").textContent = `${whenLabel(m.time)} · ${fmtDur(m.obj.end - m.obj.start)}`;
    const net = b.querySelector(".m-item-net");
    net.textContent = `${m.result.netCPM >= 0 ? "+" : "−"}${fmt(Math.abs(m.result.netCPM), 1)}`;
    net.title = "CPM acima do fundo";
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
  $("mdWhen").textContent = `${whenLabel(m.time)} · ` + (home ? `fundo da casa (${Math.round(m.bg.seconds / 60)} min) + objeto ${fmtMin(m.obj.seconds)}` : `fundo ${m.bg.seconds} s + objeto ${m.obj.seconds} s`) + (m.distance ? ` · ${m.distance}` : "");
  const v = $("mdVerdict");
  v.className = "m-verdict " + r.level;
  v.querySelector("strong").textContent = r.verdict.charAt(0).toUpperCase() + r.verdict.slice(1) + ".";
  v.querySelector("span").textContent = `${fmt(r.sigma, 1)}σ: até ~2σ é a variação natural da contagem; de 3σ para cima a diferença é real.`;
  const t = $("mdTiles");
  t.textContent = "";
  sumTile(t, home ? "Fundo da casa" : "Fundo (objeto longe)", `${fmt(m.bg.cpm, 1)} CPM`, `± ${fmt(m.bg.errCPM, 1)} · ${m.bg.counts} contagens`);
  sumTile(t, "Com o objeto", `${fmt(m.obj.cpm, 1)} CPM`, `± ${fmt(m.obj.errCPM, 1)} · ${m.obj.counts} contagens`);
  sumTile(t, "Líquido", `${r.netCPM >= 0 ? "+" : "−"}${fmt(Math.abs(r.netCPM), 1)} CPM`, `± ${fmt(r.netErr, 1)} · ≈ ${fmt(r.netUSv, usvDigits(Math.abs(r.netUSv)))} µSv/h no tubo`, r.level === "sim" ? "warn" : "");
  sumTile(t, "Objeto ÷ fundo", `${fmt(r.ratio, 2)}×`, `${fmt(r.sigma, 1)}σ acima do acaso`);
  $("mdStats").replaceChildren(measureStats(m));
  $("mdNote").textContent = m.note || "Não anotada. Use Editar para guardar o lado encostado no tubo, a embalagem etc.";
  $("mdNote").classList.toggle("muted", !m.note);
  let lim = `Limite de detecção: ${fmt(mDetLimit(m.bg, m.obj.seconds), 1)} CPM acima do fundo.`;
  if (r.limit) lim += ` Se o objeto emite algo, é menos que +${fmt(r.limit, 1)} CPM.`;
  if (r.netCPM > 0) lim += home
    ? ` Precisão do líquido: ±${fmt((r.netErr / r.netCPM) * 100, 0)}%. Para ±10%: ${mDurTxt(mObjSecsFor(m.bg, m.obj, 0.1))} com o objeto; para ±5%: ${mDurTxt(mObjSecsFor(m.bg, m.obj, 0.05))}.`
    : ` Precisão do líquido: ±${fmt((r.netErr / r.netCPM) * 100, 0)}%. Para ±10%: ${mDurTxt(mSecsFor(m.bg, m.obj, 0.1))} em cada fase; para ±5%: ${mDurTxt(mSecsFor(m.bg, m.obj, 0.05))}.`;
  if (r.stop) lim = measureExplain(r, m.obj, true) + " " + lim;
  $("mdLimits").textContent = lim;
  $("mdEditName").value = m.name;
  $("mdEditNote").value = m.note || "";
  const del = $("btnMdDel");
  del.dataset.sure = "";
  del.textContent = "Apagar";
}

function wireMeasureSaved() {
  document.querySelectorAll("#mModeSeg button").forEach((b) => b.addEventListener("click", () => setMeasureMode(b.dataset.mmode)));
  $("mSearch").addEventListener("input", renderMeasureHistory);
  const cur = () => (state.measurements || []).find((x) => x.id === state.mSel);
  $("btnMOpenSaved").addEventListener("click", () => { const id = state.mLastSaved; setMeasureMode("salvas"); if (id) { state.mSel = id; renderMeasureHistory(); drawMeasureChart(); } });
  $("btnMdReport").addEventListener("click", () => { const m = cur(); if (m) exportMeasureReport(m.id); });
  $("btnMdChart").addEventListener("click", () => { const m = cur(); if (m) showRange((m.bg.source === "casa" ? m.obj.start : m.bg.start) - 120e3, m.obj.end + 120e3); });
  $("btnMdEdit").addEventListener("click", () => { const f = $("mdEditForm"); f.hidden = !f.hidden; if (!f.hidden) $("mdEditName").focus(); });
  $("btnMdSave").addEventListener("click", async () => {
    const m = cur();
    if (!m) return;
    await call(api.UpdateMeasurement, m.id, $("mdEditName").value, $("mdEditNote").value);
    $("mdEditForm").hidden = true;
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

// ---------- fora de casa ----------

const periodAt = (ms) => (state.periods || []).find((p) => ms >= p.from && ms < p.to) || null;
const awayLabel = (ms) => { const p = periodAt(ms); return p ? ` · ${p.name}` : ""; };

// faixas lilás no gráfico para os períodos fora de casa
function awayBands(t0, t1, xt) {
  return (state.periods || []).filter((p) => p.to > t0 && p.from < t1).map((p) => {
    const x1 = xt(Math.max(p.from, t0)), x2 = xt(Math.min(p.to, t1));
    return `<rect x="${x1.toFixed(1)}" y="0" width="${Math.max(1, x2 - x1).toFixed(1)}" height="${H}" fill="#B79CFF" fill-opacity="0.13"/>`;
  }).join("");
}

const toLocalInput = (ms) => { const d = new Date(ms); return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`; };
const fromLocalInput = (s) => (s ? new Date(s).getTime() : NaN);

let periodEditing = 0;

function renderPeriods() {
  const ul = $("periodList");
  ul.textContent = "";
  if (!state.periods.length) { ul.innerHTML = '<li class="muted small">Nenhum período marcado.</li>'; return; }
  for (const p of [...state.periods].reverse()) {
    const li = document.createElement("li");
    const txt = document.createElement("span");
    txt.innerHTML = `<strong></strong> <span class="mono small muted"></span>`;
    txt.firstChild.textContent = p.name;
    txt.lastChild.textContent = zoomRangeText(p.from, p.to);
    const ed = document.createElement("button");
    ed.type = "button"; ed.className = "btn small"; ed.textContent = "Editar";
    ed.addEventListener("click", () => editPeriod(p));
    const see = document.createElement("button");
    see.type = "button"; see.className = "btn small"; see.textContent = "Ver";
    see.addEventListener("click", () => showRange(p.from, p.to));
    const del = document.createElement("button");
    del.type = "button"; del.className = "btn small danger-btn"; del.textContent = "Apagar";
    del.addEventListener("click", async () => {
      if (del.dataset.sure !== "1") { del.dataset.sure = "1"; del.textContent = "Confirmar"; return; }
      await call(api.DeletePeriod, p.id);
    });
    const rep = document.createElement("button");
    rep.type = "button"; rep.className = "btn small"; rep.textContent = "Relatório (PDF)";
    rep.title = "Relatório em PDF só desse trecho fora de casa";
    rep.addEventListener("click", () => exportReport(p.from, Math.min(p.to, Date.now()), "Fora de casa: " + p.name));
    li.append(txt, see, rep, ed, del);
    ul.appendChild(li);
  }
}

function editPeriod(p) {
  periodEditing = p ? p.id || 0 : 0;
  $("periodFormTitle").textContent = periodEditing ? "Editar período" : "Novo período";
  $("btnPeriodNew").hidden = !periodEditing;
  $("pName").value = p ? p.name || "" : "";
  const now = Date.now();
  $("pFrom").value = toLocalInput(p && p.from ? p.from : now - 3 * 3600e3);
  $("pTo").value = toLocalInput(p && p.to ? p.to : now);
  $("pErr").hidden = true;
}

function openPeriods(prefill) {
  renderPeriods();
  editPeriod(prefill || null);
  showView("fora", true);
  if (prefill) $("pName").focus();
}

// abre um trecho nos dois gráficos (mesmo jeito do mapa de calor)
function showRange(from, to) {
  if (state.view !== "graficos") showView("graficos", true);
  state.zoom = { from, to }; state.zoomStack = [];
  state.doseZoom = { from, to }; state.doseZoomStack = [];
  state.zoomLinked = true;
  refreshChart(); refreshDose();
  $("cpmCard").scrollIntoView({ behavior: "smooth", block: "start" });
}

async function refreshPeriods() {
  try { state.periods = (await api.GetPeriods()) || []; } catch (e) { state.periods = []; }
  renderPeriods();
}

// Depois de baixar o histórico: pergunta se os trechos novos foram fora de casa
function askNewRanges(ranges) {
  const ul = $("newRangeList");
  ul.textContent = "";
  const bg = state.last && state.last.bgCPM ? state.last.bgCPM : null;
  for (const r of ranges) {
    const li = document.createElement("li");
    li.className = "nr";
    const txt = document.createElement("span");
    txt.innerHTML = "<strong></strong> <span class=\"mono small muted\"></span>";
    txt.firstChild.textContent = zoomRangeText(r.from, r.to);
    txt.lastChild.textContent = `média ${fmt(r.avgCPM, 1)} CPM` + (bg ? ` · seu normal em casa: ${fmt(bg, 0)}` : "");
    // resumo do trecho: máximo, dose, alertas, chuva
    const sum = document.createElement("span");
    sum.className = "nr-sum small mono muted";
    txt.appendChild(sum);
    api.GetSummary(r.from, r.to).then((s) => {
      const parts = [`máx ${fmt(s.max, 0)} CPM ${atText(s, s.maxAt)}`, `mín ${fmt(s.min, 0)}`, `dose ${fmt(s.dose, 2)} µSv`, `alertas: ${alertsText(s)}`];
      if (s.rainKnown) parts.push(s.rainHours ? `choveu ${fmt(s.rainMM, 1)} mm` : "sem chuva");
      sum.textContent = parts.join(" · ");
    }).catch(() => {});
    const home = document.createElement("button");
    home.type = "button"; home.className = "btn small"; home.textContent = "Em casa";
    const out = document.createElement("button");
    out.type = "button"; out.className = "btn small away-btn"; out.textContent = "Fora de casa";
    const form = document.createElement("div");
    form.className = "nr-form";
    form.hidden = true;
    const name = document.createElement("input");
    name.type = "text"; name.maxLength = 60; name.placeholder = "Fora de casa"; name.setAttribute("aria-label", "Nome do período");
    const save = document.createElement("button");
    save.type = "button"; save.className = "btn small accent"; save.textContent = "Salvar";
    form.append(name, save);
    const done = (msg) => { li.classList.add("answered"); home.hidden = out.hidden = true; form.hidden = true; const s = document.createElement("span"); s.className = "small muted"; s.textContent = msg; li.appendChild(s); };
    home.addEventListener("click", () => done("em casa"));
    out.addEventListener("click", () => { form.hidden = false; name.focus(); });
    const doSave = async () => {
      try {
        const p = await api.SavePeriod({ id: 0, from: r.from, to: r.to, name: name.value });
        done(`marcado: ${p.name}`);
      } catch (e) { toast(errMsg(e), true); }
    };
    save.addEventListener("click", doSave);
    name.addEventListener("keydown", (e) => { if (e.key === "Enter") { e.preventDefault(); doSave(); } });
    li.append(txt, home, out, form);
    ul.appendChild(li);
  }
  $("newDataDlg").showModal();
}

function wirePeriods() {
  $("btnZoomAway").addEventListener("click", () => state.zoom && openPeriods({ from: state.zoom.from, to: state.zoom.to }));
  $("btnZoomNote").addEventListener("click", () => state.zoom && openNote({ id: 0, time: state.zoom.from, end: Math.min(Date.now(), state.zoom.to), text: "" }));
  $("btnDoseZoomAway").addEventListener("click", () => state.doseZoom && openPeriods({ from: state.doseZoom.from, to: state.doseZoom.to }));
  $("btnPeriodNew").addEventListener("click", () => editPeriod(null));
  $("btnPeriodSave").addEventListener("click", async () => {
    const from = fromLocalInput($("pFrom").value), to = fromLocalInput($("pTo").value);
    const err = $("pErr");
    if (!(from < to)) { err.textContent = "O fim precisa ser depois do início."; err.hidden = false; return; }
    try {
      await api.SavePeriod({ id: periodEditing, from, to, name: $("pName").value });
      toast("Período salvo");
      editPeriod(null);
    } catch (e) { err.textContent = errMsg(e); err.hidden = false; }
  });
}

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
const LV_DEFAULT = { attentionCPM: 50, dangerCPM: 1000, snoozeMin: 5, satMaxUSv: 2000, satDropUSv: 1000, satHoldSec: 60, deadTimeOn: false, deadTimeUs: 200 };
function lv() {
  const out = Object.assign({}, LV_DEFAULT);
  const s = (state.settings && state.settings.levels) || {};
  for (const k of Object.keys(out)) if (s[k] > 0) out[k] = s[k];
  return out;
}
const dangerCPM = () => lv().dangerCPM;
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
  const demoOn = alarmCtl.demo || alarmCtl.demoDanger;
  const kind = realOn ? "real" : demoOn ? "demo" : testing ? "test" : null;
  const r = kind === "real" ? (state.demo.active ? state.lastReal : state.last) : kind === "demo" ? state.last : null;
  const sev = kind === "real" ? (alarmCtl.realDanger ? "perigo" : "alarme")
    : kind === "demo" ? (alarmCtl.demoDanger ? "perigo" : "alarme")
    : kind === "test" ? alarmCtl.testSev : null;
  const limit = state.settings.alarmCPM || 100;
  const cpm = kind === "test" ? (sev === "perigo" ? dangerCPM() * 1.2 : limit * 2.5) : r ? r.cpm : 0;
  const fast = sev === "alarme" && cpm >= limit * 2;

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
    } else if (!was || kind === "real") {
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
  updateAttention();
  const bar = $("alarmBar");
  bar.hidden = !on;
  bar.classList.toggle("demo", kind === "demo" || kind === "test");
  bar.classList.toggle("perigo", sev === "perigo");
  bar.classList.toggle("muted", sev === "perigo" ? snoozed : alarmCtl.muted);
  if (!on) return;

  const usv = kind === "test" ? cpm * (calK()) : r ? r.usv : 0;
  const tag = { real: state.demo.active ? " REAL" : "", demo: " (demo)", test: " (teste)" }[kind];
  const note = $("alarmNote");
  const sat = r && r.saturated ? r.satReason || "limite" : null;
  renderRise(sev === "perigo" && r ? r.rise : null, usv);
  if (sev === "perigo") {
    if (sat) {
      $("alarmTitle").textContent = "GEIGER SATURADO" + tag;
      $("alarmDetail").textContent = sat === "limite"
        ? `acima do que o aparelho mede · o real pode ser maior que ${fmt(usv, usv >= 100 ? 0 : usvDigits(usv))} µSv/h`
        : `caiu de repente após valor extremo · o tubo pode ter travado`;
    } else {
      $("alarmTitle").textContent = "PERIGO" + tag;
      $("alarmDetail").textContent = `${fmt(cpm, 0)} CPM · ${fmt(usv, usvDigits(usv))} µSv/h · perigo a partir de ${fmt(dangerCPM(), 0)}`;
    }
    note.textContent = PHONES;
    note.hidden = false;
    const g = guidance("perigo", sat ? 1000 : usv);
    g.steps = g.steps.filter((t) => t !== PHONES); // os telefones já ficam à vista na faixa
    fillGuide("alarmGuide", "alarmHint", g);
    if (sat) $("alarmHint").textContent = "Não confie no número. Afaste-se agora.";
    $("btnSilence").textContent = snoozed ? `Silenciado até ${hhmm(alarmCtl.snoozeUntil)}` : "Silenciar 5 min";
    $("btnSilence").disabled = snoozed;
  } else {
    $("alarmTitle").textContent = "ALARME" + tag;
    $("alarmDetail").textContent = `${fmt(cpm, 0)} CPM · ${fmt(usv, usvDigits(usv))} µSv/h · seu limite: ${fmt(limit, 0)}`;
    note.hidden = true;
    fillGuide("alarmGuide", "alarmHint", guidance("alarme", usv));
    $("btnSilence").textContent = alarmCtl.muted ? "Som desligado" : "Silenciar";
    $("btnSilence").disabled = alarmCtl.muted;
  }
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
  const v = parseInt($("numAlarm").value, 10) || 0;
  const el = $("dangerNote");
  const d = parseInt($("lvDanger").value, 10) || dangerCPM();
  if (v >= d) {
    el.className = "danger-note warn";
    el.textContent = `Seu limite (${fmt(v, 0)}) está acima do perigo do app (${fmt(d, 0)} CPM): quem toca é o alarme de perigo.`;
  } else {
    el.className = "danger-note";
    el.textContent = `A partir de ${fmt(d, 0)} CPM toca o alarme de perigo do app (vermelho), com prioridade sobre o seu. Ajuste em "Níveis do app".`;
  }
}

// ---------- aba "Níveis do app" ----------

const LV_FIELDS = { lvAttn: "attentionCPM", lvDanger: "dangerCPM", lvSnooze: "snoozeMin", lvSatMax: "satMaxUSv", lvSatDrop: "satDropUSv", lvSatHold: "satHoldSec", lvDTUs: "deadTimeUs" };

function fillLevels(l) {
  for (const [id, k] of Object.entries(LV_FIELDS)) $(id).value = l[k] ?? LV_DEFAULT[k];
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
  out.deadTimeOn = $("lvDTOn").checked;
  return out;
}

// mostra quanto o CPM dá em µSv/h com a calibração do contador
function renderLevelEq() {
  const k = calK();
  document.querySelectorAll(".lv-eq").forEach((el) => {
    const v = parseFloat($(el.dataset.eq).value) || 0;
    el.textContent = v ? `≈ ${fmt(v * k, usvDigits(v * k))} µSv/h com a calibração atual` : "";
  });
}

// ---------- configurações: seções na lateral, uma de cada vez ----------

const SET_GROUPS = [["Geral", "setGeneral"], ["Níveis do app", "setLevels"]];
const setSections = () => [...document.querySelectorAll("#settings .set-panel > fieldset")].filter((f) => !f.hidden);
const slug = (t) => t.normalize("NFD").replace(/[\u0300-\u036f]/g, "").toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "");

function buildSetNav() {
  const nav = $("setNav");
  nav.textContent = "";
  for (const [name, pid] of SET_GROUPS) {
    const fs = [...$(pid).querySelectorAll(":scope > fieldset")].filter((f) => !f.hidden);
    if (!fs.length) continue;
    const h = document.createElement("h4");
    h.textContent = name;
    nav.appendChild(h);
    for (const f of fs) {
      const title = f.querySelector(":scope > legend").textContent.trim();
      if (!f.id) f.id = "sec-" + slug(title);
      const b = document.createElement("button");
      b.type = "button";
      b.textContent = title;
      b.dataset.sec = f.id;
      b.addEventListener("click", () => showSetSection(f.id));
      nav.appendChild(b);
    }
  }
}

function showSetSection(id) {
  const all = setSections();
  const f = all.find((x) => x.id === id) || all[0];
  if (!f) return;
  for (const x of all) x.classList.toggle("sec-off", x !== f);
  const lvOn = !!f.closest("#setLevels");
  $("setGeneral").hidden = lvOn;
  $("setLevels").hidden = !lvOn;
  $("btnLevelsReset").hidden = !lvOn;
  document.querySelectorAll("#setNav button").forEach((b) => b.setAttribute("aria-current", String(b.dataset.sec === f.id)));
  state.setSec = f.id;
  $("setContent").scrollTop = 0;
  if (f.querySelector("#devcfg")) openDevCfg();
}

// abre a seção onde está um campo (ex.: "txtPlace" → Chuva)
function showSetFor(elId) {
  const f = $(elId) && $(elId).closest("#settings .set-panel > fieldset");
  if (f) showSetSection(f.id);
}

function showSetTab(which) { // compatível com o jeito antigo (abas)
  const all = setSections();
  const f = which === "levels" ? all.find((x) => x.closest("#setLevels")) : all[0];
  if (f) showSetSection(f.id);
}

document.addEventListener("click", async (e) => {
  if (!e.target.closest("#btnFixAutostart")) return;
  try { await api.FixAutostart(); $("chkAutostart").checked = true; toast("Início com o Windows corrigido"); }
  catch (err) { toast("Não consegui corrigir: " + errMsg(err), true); }
  refreshAutostart();
});

function wireLevels() {
  for (const id of Object.keys(LV_FIELDS)) $(id).addEventListener("input", () => { renderLevelEq(); renderDangerNote(); renderAttnLabel(); renderDeadTime(); });
  $("lvDTOn").addEventListener("change", renderDeadTime);
  $("btnLevelsReset").addEventListener("click", () => { fillLevels(LV_DEFAULT); renderDangerNote(); renderAttnLabel(); });
}

function renderAttnLabel() {
  const v = parseInt($("lvAttn").value, 10) || lv().attentionCPM;
  $("attnToggleLabel").textContent = `Aviso de atenção a partir de ${fmt(v, 0)} CPM (amarelo e 3 bipes)`;
}

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
  const kind = attnCtl.real ? "real" : attnCtl.demo ? "demo" : null;
  attnCtl.kind = kind;
  const show = !!kind && !attnCtl.dismissed && !alarmCtl.kind;
  $("attnBar").hidden = !show;
  document.body.classList.toggle("attention", show || (!!kind && !alarmCtl.kind));
  if (!show) return;
  const r = kind === "real" && state.demo.active ? state.lastReal : state.last;
  $("attnTitle").textContent = kind === "demo" ? "Atenção (demo)" : state.demo.active ? "Atenção (real)" : "Atenção";
  $("attnDetail").textContent = r
    ? `${fmt(r.cpm, 0)} CPM · ${fmt(r.usv, usvDigits(r.usv))} µSv/h · atenção a partir de ${fmt(lv().attentionCPM, 0)}`
    : `atenção a partir de ${fmt(lv().attentionCPM, 0)} CPM`;
  fillGuide("attnGuide", "attnHint", guidance("atencao", r ? r.usv : 0));
}

function silenceAlarm() {
  if (!alarmCtl.kind) return;
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
  $("numAlarm").addEventListener("input", renderDangerNote);
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
  $("alarmLabel").textContent = "Alarme ligado";
}

// ---------- backup ----------

function mergeTextJS(r) {
  const p = [];
  const add = (n, one, many) => { if (n === 1) p.push(`1 ${one}`); else if (n > 1) p.push(`${n} ${many}`); };
  add(r.minutes, "minuto de leitura", "minutos de leitura");
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
  devCfgRead = false;
  refreshReportDir();
  refreshAutostart();
  const s = state.settings;
  $("chkAuto").checked = !!s.autoConnect;
  $("numAlarm").value = s.alarmCPM || 100;
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
  buildSetNav();
  // (o botão da engrenagem chama com o evento do clique: só vale texto)
  if (typeof focusId === "string") showSetFor(focusId); else showSetSection(state.setSec);
  renderDangerNote();
  renderRainSettings();
  renderStatus();
  $("settings").showModal();
  await fillPorts();
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
  qsa("#devcfg fieldset.dc-edit").forEach((f) => { f.disabled = !on; });
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
    let msg = "O app vai apagar e regravar a configuração do contador (leva alguns segundos). Não desconecte o cabo nesse tempo.";
    if (calChanged(next)) msg += "\n\nA CALIBRAÇÃO vai mudar: isso altera o µSv/h mostrado pelo contador e pelo app.";
    if (!confirm(msg + "\n\nContinuar?")) return;
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
    if (!confirm("Escolha uma cópia salva para gravar de volta no contador. A configuração atual também é salva antes. Continuar?")) return;
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
    state.range = b.dataset.range; localSet("range", state.range); state.zoom = null; state.zoomStack = []; unlinkZoom(); refreshChart(); markHeatPick();
  }));

  $("statusPill").addEventListener("click", () => openSettings("selPort"));
  $("btnSettings").addEventListener("click", openSettings);
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
      $("settings").close();
      toast("Conectado em " + port);
    } catch (e) {
      $("connMsg").textContent = String(e && e.message ? e.message : e);
    } finally {
      $("btnConnect").disabled = false;
    }
  });
  $("btnSave").addEventListener("click", async () => {
    if (devCfgDirty()) toast('Os ajustes do aparelho não foram para o contador: use "Gravar no contador" na seção Contador.', true);
    const alarm = parseInt($("numAlarm").value, 10);
    await saveSettings({
      autoConnect: $("chkAuto").checked,
      alarmCPM: alarm > 0 ? alarm : 100,
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
    $("settings").close();
    toast("Configurações salvas");
  });
  $("btnOpenData").addEventListener("click", () => call(api.OpenDataFolder));
  $("btnOpenLog").addEventListener("click", () => call(api.OpenLogFile));
  $("footData").addEventListener("click", () => call(api.OpenDataFolder));

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
}

// ---------- eventos vindos do Go ----------

function listen() {
  on("reading", (r) => {
    state.last = r;
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
  on("periods", (list) => { state.periods = list || []; renderPeriods(); refreshAll(); });
  on("status", (s) => {
    const was = state.status.state;
    state.status = s;
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
    line.textContent = `Leitura real: ${fmt(r.cpm, 0)} CPM · ${fmt(r.usv, usvDigits(r.usv))} µSv/h` + (r.danger ? " · PERIGO REAL" : r.alarm ? " · ALARME REAL" : "");
    line.classList.toggle("alarm", !!(r.alarm || r.danger));
    state.lastReal = r;
    state.realAlarm = !!(r.alarm || r.danger);
    alarmCtl.real = !!r.alarm;
    alarmCtl.realDanger = !!r.danger;
    updateAlarm();
    setAttention("real", !!r.attention);
  });
  on("stored", () => refreshAll());
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
  await Promise.allSettled([refreshStats(), refreshChart(), refreshDose(), refreshHeat(), refreshDaily(), refreshEvents(), refreshRainImpact()]);
}

// ---------- chuva × radiação ----------

let riAt = 0;
async function refreshRainImpact(force) {
  if (state.view !== "chuva") return; // só calcula com a aba aberta
  if (!state.settings.rainEnabled) { renderRainOff(); return; }
  if (!force && Date.now() - riAt < 9 * 60e3 && state.ri) return;
  riAt = Date.now();
  let r;
  try { r = await api.GetRainImpact(); } catch (e) { return; }
  state.ri = r;
  renderRainImpact(r);
}

const sgn = (v, d) => (v >= 0 ? "+" : "−") + fmt(Math.abs(v), d);
const durMin = (m) => (m < 60 ? `${m} min` : `${fmt(m / 60, m % 60 ? 1 : 0)} h`);

function renderRainOff() {
  state.ri = null;
  $("riSub").textContent = "chuva desligada";
  $("riChartBox").hidden = true;
  $("riList").textContent = "";
  $("riTiles").innerHTML = '<p class="small muted">Para medir o efeito da chuva no CPM, ligue "Mostrar chuva no gráfico" e escolha a cidade em Configurações → Geral → Chuva no gráfico.</p>';
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
  if (n.hasBase && n.hasNow) cmp = `Antes da chuva: ${fmt(n.baseline, 1)} CPM · últimos 15 min: ${fmt(n.nowCPM, 1)} (${sgn(n.nowCPM - n.baseline, 1)})`;
  box.className = "ri-now " + cls;
  box.innerHTML = '<strong></strong><span class="small muted"></span><span class="small"></span><span class="small mono muted"></span>';
  box.children[0].textContent = since;
  box.children[1].textContent = what;
  box.children[2].textContent = st;
  box.children[3].textContent = cmp;
  box.children[3].hidden = !cmp;
}

function renderRainImpact(r) {
  renderRainNow(r.now);
  const tiles = $("riTiles"), list = $("riList");
  tiles.textContent = ""; list.textContent = "";
  $("riChartBox").hidden = !r.n;
  if (!r.n) {
    $("riSub").textContent = r.skippedNoData ? `${r.skippedNoData} chuva(s) sem dados suficientes do contador` : "nenhuma chuva medida ainda";
    tiles.innerHTML = '<p class="small muted">A cada chuva com o contador gravando (app aberto ou histórico baixado), o app mede quanto o CPM subiu depois e compara com horários sem chuva.</p>';
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
  tile("Efeito da chuva", `${sgn(eff, 1)} CPM`, r.dryN ? `sobe ${sgn(r.avgRise, 1)}; sem chuva, ${sgn(r.dryRise, 1)}` : "sem horários secos para comparar ainda");
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
    li.children[1].children[1].textContent = `${fmt(ep.baseline, 1)} → ${fmt(ep.peak, 1)} CPM (${sgn(ep.rise, 1)}) em ${durMin(Math.round((ep.peakAt - ep.start) / 60e3))} · ${back}`;
    li.addEventListener("click", () => showRange(ep.start - 2 * 36e5, ep.start + 4 * 36e5));
    list.appendChild(li);
  }
}

function drawRainCurve(r) {
  const pts = r.curve || [];
  let lo = 0, hi = 1;
  for (const p of pts) for (const v of [p.rain, p.dry]) if (v != null) { lo = Math.min(lo, v); hi = Math.max(hi, v); }
  lo = Math.floor(lo - 0.5);
  hi = lo + 3 * Math.ceil((Math.ceil(hi + 0.5) - lo) / 3); // 4 marcas inteiras
  const x = (m) => ((m + 120) / 360) * W;
  const y = (v) => H - ((v - lo) / (hi - lo)) * H;
  const ys = $("riY").children;
  for (let i = 0; i < 4; i++) { const v = Math.round(hi - ((hi - lo) / 3) * i); ys[i].textContent = v === 0 ? "0" : sgn(v, 0); }
  const line = (key, color, w) => {
    let d = "", pen = "M";
    for (const p of pts) { if (p[key] == null) { pen = "M"; continue; } d += `${pen}${x(p.min).toFixed(1)} ${y(p[key]).toFixed(1)}`; pen = "L"; }
    return d ? `<path d="${d}" fill="none" stroke="${color}" stroke-width="${w}" stroke-linejoin="round" vector-effect="non-scaling-stroke"/>` : "";
  };
  const x0 = x(0).toFixed(1), y0 = y(0).toFixed(1);
  $("riChart").innerHTML =
    `<line x1="0" y1="${y0}" x2="${W}" y2="${y0}" stroke="#2A3136" stroke-width="1" vector-effect="non-scaling-stroke"/>` +
    `<rect x="${x0}" y="0" width="${(x(240) - x(0)).toFixed(1)}" height="${H}" fill="#3DA5FF" fill-opacity="0.05"/>` +
    `<line x1="${x0}" y1="0" x2="${x0}" y2="${H}" stroke="#3DA5FF" stroke-width="1" stroke-dasharray="4 4" vector-effect="non-scaling-stroke"/>` +
    line("dry", "#A3ADB4", 1.5) + line("rain", "#3DA5FF", 2.5);
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

// lacunas do período no Resumo (clique abre o trecho no gráfico)
function renderSumGaps(r) {
  const box = $("sumGaps");
  const list = (r && r.gaps) || [];
  box.hidden = !list.length;
  box.textContent = "";
  if (!list.length) return;
  const h = document.createElement("h3");
  h.textContent = `Lacunas de 10 min ou mais (${r.gapCount}` + (r.gapMinutes ? ` · ${hm(r.gapMinutes)} sem leitura no período` : "") + ")";
  box.appendChild(h);
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

// ---------- RadiaCode (aparelho de verdade) ----------

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
  const on = st.state === "conectado";
  const via = st.via === "bluetooth" ? "Bluetooth" : "USB";
  // pílula no topo: só aparece quando tem algo a dizer
  const pill = $("rcPill");
  pill.hidden = st.state === "off" || st.state === "procurando";
  $("rcDot").className = "dot " + (on ? "on" : st.state === "erro" ? "err" : "wait");
  $("rcPillText").textContent = on
    ? "RadiaCode" + (st.hasRare ? ` · ${fmt(st.battery, 0)}%` : "")
    : st.state === "conectando" ? "RadiaCode: conectando…" : "RadiaCode: erro";
  pill.title = on ? `RadiaCode conectado pelo ${via}` : st.err || "RadiaCode";
  // aba
  $("rcSub").textContent = on ? `conectado pelo ${via}` : { off: "procura desligada", procurando: "procurando no USB" + (st.ble ? " e no Bluetooth" : "") + "…", conectando: "conectando…", erro: "erro na conexão" }[st.state] || st.state;
  $("rcState").textContent = on ? `conectado · ${via}` : st.state;
  $("rcState").className = "rc-state mono small " + (on ? "ok" : st.state === "erro" ? "err" : "");
  $("rcNums").hidden = !on;
  $("rcSpecCard").hidden = !on;
  if (on) {
    $("rcUsv").textContent = st.time ? `${fmt(st.usvh, usvDigits(st.usvh))} µSv/h` : "—";
    $("rcUsvErr").textContent = st.time ? `± ${fmt(st.usvhErr, 0)}%` : "";
    $("rcCps").textContent = st.time ? `${fmt(st.cps, 2)} cps` : "—";
    $("rcCpsErr").textContent = st.time ? `± ${fmt(st.cpsErr, 0)}% · ${fmt(st.cps * 60, 0)} CPM` : "";
    $("rcDose").textContent = st.hasRare ? `${fmt(st.dose, st.dose < 10 ? 2 : 1)} µSv` : "—";
    $("rcDoseSince").textContent = st.hasRare && st.doseSec ? `em ${rcDur(st.doseSec)}` : "";
    $("rcBat").textContent = st.hasRare ? `${fmt(st.battery, 0)}%` : "—";
    $("rcTemp").textContent = st.hasRare ? `${fmt(st.tempC, 1)} °C` : "";
  }
  $("rcDevInfo").textContent = on ? [st.model, st.serial && "série " + st.serial, st.firmware && "firmware " + st.firmware].filter(Boolean).join(" · ") : "";
  $("rcHelp").textContent = on ? "" : st.state === "erro" ? st.err
    : st.state === "off" ? "A procura está desligada em Configurações → RadiaCode."
    : "Ligue o RadiaCode no USB do PC" + (st.bleOK ? (st.ble ? ", ou deixe-o perto com o Bluetooth ligado (e o app do celular desconectado)." : ", ou configure o endereço Bluetooth.") : ". No Windows, por enquanto, a conexão é só pelo USB.");
  $("rcHelp").className = "small " + (st.state === "erro" ? "warn-text" : "muted");
  if (on && (!prev || prev.state !== "conectado")) { refreshRcCompare(); refreshChart(); }
  renderRcMain(st);
  // configurações
  $("chkRcOn").checked = !st.off;
  $("chkRcRaw").checked = st.raw !== false;
  if (document.activeElement !== $("rcBLE")) $("rcBLE").value = st.ble || "";
  $("btnRcBLEScan").disabled = !st.bleOK;
  $("rcBLEInfo").textContent = st.bleOK ? "" : "No Windows, a conexão Bluetooth ainda não está pronta: use o cabo USB.";
}

// quadro do RadiaCode na tela principal: cheio quando conectado, uma linha
// com o último dado quando não
state.rcUnit = localGet("rcUnit", "usv");

// quadro do RadiaCode, com as mesmas informações do GMC: valor, dose de
// hoje, normal/σ, régua e medidas
// conexão do GMC no canto do quadro, como a do RadiaCode (só lado a lado)
function renderHeroVia() {
  const el = $("heroVia");
  const s = state.status || {}, d = s.device || {};
  const on = s.state === "conectado";
  el.hidden = !document.body.classList.contains("has-rc");
  el.textContent = on ? `· ${d.port || ""}` : state.demo && state.demo.active ? "" : "· desconectado";
  el.className = "rc-main-via gmc-via mono" + (on ? " on" : "");
}

function renderRcMain(st) {
  const live = st.state === "conectado" && st.time;
  const show = st.state === "conectado" || st.state === "conectando" || st.lastT > 0;
  $("rcMain").hidden = !show;
  $("duo").classList.toggle("on", show);
  document.body.classList.toggle("has-rc", show);
  $("heroEyebrow").textContent = show ? "GMC-800" : "Leitura atual";
  renderHeroVia();
  if (!show) return;
  const via = st.via === "bluetooth" ? "Bluetooth" : "USB";
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
  $("rcDoseToday").textContent = sx.todayMinutes || st.today ? fmt(st.today, 2) : "—";
  $("rcDoseTodaySub").textContent = sx.todayMinutes ? `${sx.todayMinutes} min medidos` : "acumulados";
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
    else { text = `Dentro do normal — seu fundo costuma ficar entre ${fmt(sx.bgLo, d)} e ${fmt(sx.bgHi, d)} µSv/h`; }
    sig.textContent = text;
    sig.className = "sigma " + cls;
    sig.title = `Pelos minutos em casa dos últimos 7 dias, o RadiaCode costuma marcar ${fmt(sx.bgUSv, d)} µSv/h (80% entre ${range}).`;
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
  // régua: os alarmes do próprio RadiaCode (como a do GMC usa as faixas do
  // manual dele); sem eles, as faixas do app em µSv/h
  const al = st.alarms || {};
  const own = al.DoseRate1 > 0 && al.DoseRate2 > al.DoseRate1;
  const attn = own ? al.DoseRate1 : st.attnUSv || 0.505, alarm = own ? al.DoseRate2 : st.alarmUSv || 1.01, top = alarm * 1.5;
  $("rcLevelHint").textContent = own ? "alarmes do RadiaCode" : "faixas do app em µSv/h";
  $("rcLevelScale").innerHTML = own
    ? `<span>0</span><span>${fmt(attn, 2)} · alarme 1</span><span>${fmt(alarm, 2)} · alarme 2</span>`
    : `<span>0</span><span>${fmt(attn, 2)} · atenção</span><span>${fmt(alarm, 2)} · alerta</span>`;
  const lvl = usv == null ? "" : !own && st.dangerUSv && usv >= st.dangerUSv ? "perigo" : usv >= alarm ? "alerta" : usv >= attn ? "atencao" : "seguro";
  $("rcLevelName").textContent = (own ? { seguro: "Seguro", atencao: "Alarme 1", alerta: "Alarme 2" } : { seguro: "Seguro", atencao: "Atenção", alerta: "Alerta", perigo: "Perigo" })[lvl] || "—";
  $("rcLevelName").className = "level-name " + lvl;
  ["seguro", "atencao", "alerta", "perigo"].forEach((l) => $("rcMain").classList.toggle("lv-" + l, live && lvl === l));
  $("rcLevelMarker").hidden = usv == null;
  if (usv != null) $("rcLevelMarker").style.left = Math.min(usv, top) / top * 100 + "%";
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
    $("rcCmpStats").textContent = c && c.rcOnly ? `${c.rcOnly} min só do RadiaCode (GMC desconectado?)` : "";
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
  box.innerHTML = svg + `<div class="rc-legend small"><span class="k gmc"></span>GMC-800 (µSv/h, média de 10 min) <span class="k rc"></span>RadiaCode</div>`;
  const parts = [`${n} min juntos`];
  if (c.ratio) parts.push(`RadiaCode ÷ GMC = ${fmt(c.ratio, 2)}${c.ratioErr ? ` ± ${fmt(c.ratioErr, 2)}` : ""}`);
  if (c.corr) parts.push(`correlação ${fmt(c.corr, 2)}`);
  if (n < 24 * 60) parts.push(`para o fator valer, junte 24 h (faltam ${rcDur((24 * 60 - n) * 60)})`);
  $("rcCmpStats").textContent = parts.join(" · ");
}

async function readRcSpectrum() {
  try {
    state.rc.spec = await api.GetRcSpectrum(state.rc.accum);
    drawRcSpectrum();
  } catch (e) { toast(errMsg(e), true); }
}

function drawRcSpectrum() {
  const sp = state.rc.spec, box = $("rcRealSpec");
  if (!sp || !sp.counts || !sp.counts.length) { box.textContent = ""; return; }
  const W = 1000, H = 260, PL = 8, PB = 22, n = sp.counts.length;
  const E = (c) => sp.a[0] + sp.a[1] * c + sp.a[2] * c * c;
  const eMax = E(n - 1);
  let top = 1, total = 0;
  for (let i = 0; i < n; i++) { total += sp.counts[i]; if (i > 8) top = Math.max(top, sp.counts[i]); }
  const yv = (v) => (state.rc.log ? Math.log10(1 + v) / Math.log10(1 + top) : v / top);
  const x = (c) => PL + (Math.max(0, E(c)) / eMax) * (W - PL - 8), y = (v) => 8 + (1 - Math.min(1, yv(v))) * (H - PB - 8);
  let d = "";
  for (let i = 0; i < n; i++) d += `${i ? "L" : "M"}${x(i).toFixed(1)},${y(sp.counts[i]).toFixed(1)}`;
  let svg = `<svg viewBox="0 0 ${W} ${H}" preserveAspectRatio="none" class="rc-cmp-svg">`;
  for (const e of [100, 500, 1000, 1500, 2000, 2500, 3000]) {
    if (e > eMax) break;
    const xx = PL + (e / eMax) * (W - PL - 8);
    svg += `<line x1="${xx}" x2="${xx}" y1="8" y2="${H - PB}" class="grid"/><text x="${xx}" y="${H - 4}" text-anchor="middle" class="ax">${e} keV</text>`;
  }
  svg += `<path d="${d}" class="rc"/></svg>`;
  box.innerHTML = svg;
  $("rcRealSpecInfo").textContent = `${(total).toLocaleString(LOCALE)} contagens em ${rcDur(sp.seconds) || sp.seconds + " s"} · ${n} canais`;
}

function wireRc() {
  on("radiacode", renderRc);
  $("rcPill").addEventListener("click", () => showView("radiacode"));
  $("rcUnitSeg").addEventListener("click", (e) => {
    const b = e.target.closest("button[data-unit]");
    if (!b) return;
    state.rcUnit = b.dataset.unit;
    localSet("rcUnit", state.rcUnit);
    if (state.rc.st) renderRcMain(state.rc.st);
  });
  $("rcKey").addEventListener("click", () => {
    if (state.chartUnit === "usv") return;
    state.chartUnit = "usv";
    localSet("chartUnit", "usv");
    refreshChart();
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
    info.textContent = list.length ? "" : "Nenhum RadiaCode apareceu. O Bluetooth dele está ligado? O app do celular está desconectado?";
    for (const d of list) {
      const li = document.createElement("li");
      const g = document.createElement("span");
      g.className = "grow";
      g.innerHTML = "<strong data-no-tr></strong><span class='small mono muted' data-no-tr></span>";
      g.firstChild.textContent = d.name;
      g.lastChild.textContent = d.address + (d.rssi ? ` · ${d.rssi} dBm` : "");
      const b = document.createElement("button");
      b.type = "button"; b.className = "btn small"; b.textContent = "Usar este";
      b.addEventListener("click", async () => { $("rcBLE").value = d.address; renderRc(await call(api.SetRadiacode, true, d.address)); ul.textContent = ""; });
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
  $("rcSpecSrc").addEventListener("click", (e) => {
    const b = e.target.closest("button[data-acc]");
    if (!b) return;
    state.rc.accum = b.dataset.acc === "1";
    $("rcSpecSrc").querySelectorAll("button").forEach((x) => x.setAttribute("aria-pressed", String(x === b)));
    readRcSpectrum();
  });
  $("btnRcSpec").addEventListener("click", readRcSpectrum);
  $("btnRcSpecLog").addEventListener("click", () => {
    state.rc.log = !state.rc.log;
    $("btnRcSpecLog").textContent = state.rc.log ? "Escala: log" : "Escala: linear";
    drawRcSpectrum();
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
}

function bgVerdict(r) {
  if (!r.bgOK || r.avg == null) return "fundo ainda não calculado";
  const p = r.vsBgPct;
  if (Math.abs(p) < 5) return `igual ao seu normal (${fmt(r.bgCPM, 0)})`;
  return `${fmt(Math.abs(p), 0)}% ${p > 0 ? "acima" : "abaixo"} do seu normal (${fmt(r.bgCPM, 0)})`;
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

function renderSummary(r, per) {
  $("sumSub").textContent = `${per[2]} · ${dayLabel(per[0])} ${hhmm(per[0])} → ${per[1] >= Date.now() - 6e4 ? "agora" : dayLabel(per[1] - 1) + " " + hhmm(per[1])}`;
  const t = $("sumTiles");
  t.textContent = "";
  $("sumTable").hidden = true;
  if (!r.minutes) {
    t.innerHTML = '<p class="small muted">Sem leituras nesse período.</p>';
    drawSumChart([]);
    return;
  }
  sumTile(t, "Média", `${fmt(r.avg, 1)} CPM`, bgVerdict(r), r.bgOK && r.vsBgPct >= 20 ? "warn" : "");
  sumTile(t, "Máximo", `${fmt(r.max, 0)} CPM`, atText(r, r.maxAt));
  sumTile(t, "Mínimo", `${fmt(r.min, 0)} CPM`, atText(r, r.minAt));
  sumTile(t, "Dose", `${fmt(r.dose, r.dose < 10 ? 2 : 1)} µSv`, `≈ ${fmt(r.avgUSv, 3)} µSv/h em média`);
  const cov = r.spanMinutes ? `${fmt(r.minutes, 0)} de ${fmt(r.spanMinutes, 0)} min` : `${hm(r.minutes)} com leitura`;
  const days = r.spanMinutes > 1440 && r.daysData ? ` · ${r.daysData} ${r.daysData === 1 ? "dia" : "dias"} com dado` : "";
  sumTile(t, "Medido", `${fmt(r.coverage * 100, 0)}%`, cov + days + (r.awayMinutes ? ` · ${hm(r.awayMinutes)} fora de casa` : ""));
  if (r.dispersion != null) sumTile(t, "Dispersão", fmt(r.dispersion, 2), `σ real ${fmt(r.sd, 1)} ÷ acaso ${fmt(Math.sqrt(r.avg), 1)} · ${r.dispersion >= 1.2 ? "variou além do acaso" : "perto de 1: normal"}`, r.dispersion >= 1.2 ? "warn" : "");
  sumTile(t, "Alertas", alertsText(r), r.peakAlert ? `pico ao vivo ${fmt(r.peakAlert, 0)} CPM` : "", r.danger || r.alarm ? "warn" : "");
  sumTile(t, "Chuva", rainText(r), r.rainHours ? `${r.rainHours} h com chuva` : "");
  if (r.measurements) sumTile(t, "Medições", String(r.measurements), "de objetos");
  drawSumChart([{ r, color: "#6EE7A0", label: per[2] }]);
}

function renderCompare(res, periods) {
  const [a, b] = res;
  $("sumSub").textContent = `${periods[0][2]} × ${periods[1][2]}`;
  $("sumTiles").textContent = "";
  const tb = $("sumTable");
  tb.hidden = false;
  const pctTxt = (x, y) => { const p = ((x - y) / y) * 100; return Math.abs(p) < 0.5 ? " (igual)" : ` (${sgn(p, 0)}%)`; };
  const diff = (x, y, d, unit) => (x == null || y == null ? "—" : Math.abs(x - y) < Math.pow(10, -d) / 2 ? "igual" : `${sgn(x - y, d)}${unit}` + (y ? pctTxt(x, y) : ""));
  const rows = [
    ["Média", a.avg, b.avg, 1, " CPM"],
    ["Máximo", a.max, b.max, 0, " CPM"],
    ["Mínimo", a.min, b.min, 0, " CPM"],
    ["Dose", a.dose, b.dose, 2, " µSv"],
  ];
  let html = `<thead><tr><th></th><th><i class="sw" style="background:#6EE7A0"></i></th><th><i class="sw" style="background:#A3ADB4"></i></th><th>A − B</th></tr></thead><tbody>`;
  tb.innerHTML = html + "</tbody>";
  const head = tb.querySelectorAll("th");
  head[1].append(" " + periods[0][2]); head[2].append(" " + periods[1][2]);
  const body = tb.querySelector("tbody");
  const row = (cells) => { const tr = document.createElement("tr"); cells.forEach((c, i) => { const td = document.createElement(i ? "td" : "th"); td.textContent = c; tr.appendChild(td); }); body.appendChild(tr); };
  for (const [n, x, y, d, u] of rows) row([n, x == null ? "—" : fmt(x, d) + u, y == null ? "—" : fmt(y, d) + u, diff(x, y, d, u)]);
  row(["Medido", hm(a.minutes), hm(b.minutes), Math.abs(a.minutes - b.minutes) > 0.1 * Math.max(a.minutes, b.minutes) ? "tempos bem diferentes: compare pela média" : ""]);
  row(["Alertas", alertsText(a), alertsText(b), ""]);
  row(["Chuva", rainText(a), rainText(b), ""]);
  drawSumChart([{ r: a, color: "#6EE7A0", label: periods[0][2] }, { r: b, color: "#A3ADB4", label: periods[1][2] }]);
}

function drawSumChart(series) {
  state.sumSeries = series;
  const svg = $("sumChart");
  if (!series.length) { svg.innerHTML = ""; $("sumLegend").textContent = ""; return; }
  const n = Math.max(...series.map((s) => s.r.curve.length));
  let top = 10;
  for (const s of series) for (const p of s.r.curve) if (p.cpm != null) top = Math.max(top, p.cpm);
  top = niceTop(top * 1.15);
  const x = (i) => (n <= 1 ? 0 : (i / (n - 1)) * W);
  const y = (v) => H - (Math.min(top, v) / top) * H;
  const ys = $("sumY").children;
  for (let i = 0; i < 4; i++) ys[i].textContent = fmt(top - (top / 3) * i, 0);
  let out = [0, 1, 2, 3].map((i) => { const gy = i === 3 ? H - 0.5 : i === 0 ? 0.5 : (H / 3) * i; return `<line x1="0" y1="${gy}" x2="${W}" y2="${gy}" stroke="#2A3136" stroke-width="1" vector-effect="non-scaling-stroke"/>`; }).join("");
  const bg = series[0].r.bgOK ? series[0].r.bgCPM : 0;
  if (bg) out += `<line x1="0" y1="${y(bg).toFixed(1)}" x2="${W}" y2="${y(bg).toFixed(1)}" stroke="#6EE7A0" stroke-opacity="0.35" stroke-dasharray="4 4" vector-effect="non-scaling-stroke"/>`;
  for (const s of series.slice().reverse()) {
    let d = "", pen = "M";
    s.r.curve.forEach((p, i) => { if (p.cpm == null) { pen = "M"; return; } d += `${pen}${x(i).toFixed(1)} ${y(p.cpm).toFixed(1)}`; pen = "L"; });
    out += `<path d="${d}" fill="none" stroke="${s.color}" stroke-width="${s === series[0] ? 2.2 : 1.6}" stroke-linejoin="round" vector-effect="non-scaling-stroke"/>`;
  }
  svg.innerHTML = out;
  const c0 = series[0].r.curve, long = series[0].r.to - series[0].r.from > DAY_MS * 1.1;
  const lab = (t) => (long ? `${WEEKDAY[new Date(t).getDay()]} ${ddmm(t)}` : hhmm(t));
  const xs = $("sumX").children;
  xs[0].textContent = lab(c0[0].t); xs[1].textContent = lab(c0[Math.floor(c0.length / 2)].t); xs[2].textContent = lab(series[0].r.to);
  $("sumLegend").innerHTML = series.map((s) => `<span class="key"><i style="border-color:${s.color}"></i></span>`).join("") + (bg ? '<span class="key"><i style="border-color:#6EE7A0;border-top-style:dashed;opacity:.5"></i>seu normal</span>' : "");
  series.forEach((s, i) => $("sumLegend").children[i].append(s.label));
}

// ---------- relatórios em PDF ----------

async function exportMeasureReport(id) {
  try {
    const f = await api.ExportMeasureReport(id);
    if (f) toast("Relatório salvo: " + f.split(/[\\/]/).pop());
  } catch (e) { toast(errMsg(e), true); }
}

async function exportReport(from, to, label) {
  try {
    const f = await api.ExportReport(Math.round(from), Math.round(to), label);
    if (f) toast("Relatório salvo: " + f.split(/[\\/]/).pop());
  } catch (e) { toast(errMsg(e), true); }
}

function renderReportDir(d) {
  d = d || {};
  $("reportDir").textContent = (d.dir || "—") + (d.default ? "  (padrão)" : "");
  $("btnResetReportDir").hidden = !!d.default;
}
async function refreshReportDir() { try { renderReportDir(await api.GetReportDir()); } catch (e) { /* segue */ } }

function wireReports() {
  $("btnMReportOpen").addEventListener("click", () => call(api.OpenReportFile, $("mReport").dataset.path));
  $("btnMReportMake").addEventListener("click", () => exportMeasureReport(+$("mReport").dataset.id));
  $("btnReportDir").addEventListener("click", async () => { try { renderReportDir(await api.ChooseReportDir()); } catch (e) { toast(errMsg(e), true); } });
  $("btnOpenReportDir").addEventListener("click", () => call(api.OpenReportDir));
  $("btnResetReportDir").addEventListener("click", async () => renderReportDir(await api.ResetReportDir()));
  $("btnZoomReport").addEventListener("click", () => {
    if (!state.zoom) return;
    const p = state.heatPick && state.heatPick.from === state.zoom.from && state.heatPick.to === state.zoom.to;
    exportReport(state.zoom.from, state.zoom.to, p ? "padrão por hora" : "trecho selecionado");
  });
}

function setupSummary() {
  $("btnReport").addEventListener("click", () => { const [from, to, label] = sumPeriods()[0]; exportReport(from, to, label); });
  document.querySelectorAll("#sumModeSeg button").forEach((b) => b.addEventListener("click", () => { state.sum.mode = b.dataset.smode; refreshSummary(); }));
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
      return `${ser.length > 1 ? s.label + " " : ""}${when}: ${p.cpm == null ? "sem dado" : fmt(p.cpm, 1) + " CPM"}`;
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
    tip.textContent = `${when} · chuva ${p.rain == null ? "—" : sgn(p.rain, 1)} · sem chuva ${p.dry == null ? "—" : sgn(p.dry, 1)} CPM`;
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
  try { $("appVersion").textContent = "v" + (await api.GetVersion()) + " · "; } catch (e) { /* sem versão */ }
  try {
    state.platform = api.GetPlatform ? await api.GetPlatform() : "windows";
    // bandeja e iniciar com o sistema só existem no Windows
    if (state.platform !== "windows") $("chkTray").closest("fieldset").hidden = true;
  } catch (e) { /* segue */ }
  await refreshRainStatus();
  await refreshPeriods();
  await refreshMeasurements();
  await refreshAll();
  if (!hasGo) toast("Modo demonstração: abra pelo app para usar o contador de verdade.");
  // Garante que o gráfico de 7 dias e as datas "hoje/ontem" virem o dia.
  setInterval(refreshAll, 5 * 60 * 1000);
  // os gráficos de 15 e 30 min andam junto com as leituras por segundo
  setInterval(() => { if (isShortRange() && !state.zoom) refreshChart(); }, 5000);
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
  let status = { state: "conectado", message: "", device: dev, downloading: false, dataDir: "C:\\Users\\voce\\AppData\\Roaming\\GMC Monitor" };
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
  let periods = [{ id: 1, name: "Passeio no parque", from: Math.floor((now - 2 * 864e5) / 36e5) * 36e5 + 2 * 36e5, to: Math.floor((now - 2 * 864e5) / 36e5) * 36e5 + 5.5 * 36e5 }];
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
      emit("reading", { time: Date.now(), cpm: c, cps: Math.round(c / 60), usv: c * 0.0101, mrh: c * 0.00101, level: c >= dl ? "perigo" : c >= 100 ? "alerta" : c >= 50 ? "atencao" : "seguro", alarm: settings.alarmEnabled && c >= settings.alarmCPM, danger: c >= dl, saturated: c * 0.0101 >= 2000, satReason: "limite", rise: c >= dl ? { fromUSv: 0.13, seconds: 3 } : null, demo: true, attention: c >= al, bgCPM: 20, sigma: (c - 20) / Math.sqrt(20) });
      feed(Math.max(0, Math.round(c / 60 + (Math.random() - 0.5) * 2)));
      emit("realReading", { time: Date.now(), cpm: 21, cps: 0, usv: 0.137, mrh: 0.0137, level: "seguro", alarm: false });
      return;
    }
    if (status.state !== "conectado") return;
    const cpm = Math.round(20 + (Math.random() - 0.5) * 12);
    emit("reading", { time: Date.now(), cpm, cps: Math.round(cpm / 60), usv: cpm * 0.0101, mrh: cpm * 0.00101, level: cpm >= 100 ? "alerta" : cpm >= 50 ? "atencao" : "seguro", alarm: false, attention: cpm >= 50, bgCPM: 20, sigma: (cpm - 20) / Math.sqrt(20) });
    feed(Math.random() < 0.33 ? 1 : 0);
  }, 1000);
  const range = (from, to) => pts.filter((p) => p.t >= from && p.t < to);
  let meas = { phase: "", seconds: 0, elapsed: 0, counts: 0, name: "", demo: false, bg: null, obj: null, result: null };
  const phaseOf = (counts, secs) => { const m = secs / 60, cpm = counts / m; return { counts, seconds: secs, cpm, errCPM: Math.sqrt(Math.max(counts, 1)) / m, usv: cpm * 0.0101 }; };
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
  function feed(cps) {
    if (meas.phase !== "fundo" && meas.phase !== "objeto") return;
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
      if (meas.phase === "fundo") { meas.bg = ph; meas.phase = "fundo-pronto"; }
      else {
        meas.obj = ph; meas.phase = "pronto";
        const r = compare(meas.bg, ph);
        r.stop = stop;
        if (r.level !== "sim") r.limit = Math.max(r.netCPM, 0) + 2 * r.netErr;
        if (stop === "nada") { r.level = "nada"; r.verdict = `igual ao fundo: se o objeto emite algo, é menos que +${fmt(r.limit, 1)} CPM`; }
        if (stop === "limite" && r.level !== "sim") r.verdict += " (chegou ao tempo máximo sem decidir)";
        if (meas.auto) { r.pct = meas.pct; r.threshold = thrOf(meas.bg, meas.pct); }
        meas.result = r; meas.live = null; meas.eta = -1;
      }
    }
    emit("measure", { ...meas });
  }
  const demoRcCfg = { off: false, ble: "", raw: true };
  // localStorage rcFake=1: finge um RadiaCode conectado (para ver a tela no demo)
  const demoRcFake = () => { try { return localStorage.getItem("rcFake") === "1"; } catch { return false; } };
  const demoRc = () => demoRcFake() && !demoRcCfg.off
    ? { today: 0.84, lastT: Date.now() - 60e3, lastUSv: 0.098, lastCPS: 3.2, attnUSv: 0.505, alarmUSv: 1.01, dangerUSv: 10.1,
        alarms: { CountRate1: 20, CountRate2: 60, DoseRate1: 0.4, DoseRate2: 1.2, Dose1: 99999.99, Dose2: 99999.99, ShowSv: true, ShowCPM: true },
        stats: { bgUSv: 0.097, bgLo: 0.085, bgHi: 0.11, bgMinutes: 4000, avg1h: 0.099, n1h: 60, min24h: 0.071, max24h: 0.142, max24hAt: Date.now() - 5 * 3600e3, n24h: 1300, todayMinutes: 862 }, state: localStorage.getItem("rcFake2") ? "procurando" : "conectado", via: "usb", model: "RadiaCode-102", serial: "RC-102-001234", firmware: "4.14 (Sep 10 2026)", time: Date.now(), cps: 3.1 + Math.random() * 0.3, cpsErr: 9, usvh: 0.095 + Math.random() * 0.01, usvhErr: 11, dose: 2.41, doseSec: 93600, battery: 87, tempC: 24.5, hasRare: true, ble: demoRcCfg.ble, bleOK: true, off: false }
    : { state: demoRcCfg.off ? "off" : "procurando", ble: demoRcCfg.ble, bleOK: true, off: demoRcCfg.off, raw: demoRcCfg.raw };
  const demoHomeBg = () => ({ ok: true, cpm: 20.4, errCPM: 0.6, minutes: 58, counts: 1183, from: Date.now() - 3600e3, to: Date.now() - 60e3, demo: true });
  const chartPreview = (period) => api._unused(period);
  let api;
  api = {
    GetSettings: async () => settings,
    GetAutostartStatus: async () => ({ available: true, on: !!settings.startWithWindows, path: "C:\\Users\\voce\\Downloads\\GMC-Monitor-v1.6.0.exe", exists: false, current: false, disabled: false, started: false }),
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
    GetReportDir: async () => ({ dir: settings.reportDir || "C:\\Users\\voce\\Documents\\GMC Monitor\\Relatórios", default: !settings.reportDir }),
    ChooseReportDir: async () => { settings.reportDir = "D:\\Radiação\\Relatórios"; return api.GetReportDir(); },
    ResetReportDir: async () => { settings.reportDir = ""; return api.GetReportDir(); },
    OpenReportDir: async () => {},
    GetEvents: async (kind) => events.concat(demoAlerts).filter((e) => !kind || e.kind === kind).sort((x, y) => y.time - x.time),
    GetAlertDetail: async (id) => {
      const e = events.concat(demoAlerts).find((x) => x.id === id);
      if (!e) throw new Error("alerta não encontrado");
      const end = e.active || !e.end ? Date.now() : e.end;
      const lim = +((e.text.match(/(?:a partir de|limite|acima de) (\d+)/) || [])[1] || 50);
      const live = !e.retro && Date.now() - e.time < 7 * 864e5;
      const pad = live ? Math.min(Math.max((end - e.time) / 4, 120e3), 18e5) : Math.min(Math.max(end - e.time, 6e5), 72e5);
      const from = e.time - pad, to = Math.min(Date.now(), end + pad);
      const bump = (t) => (t >= e.time && t <= end ? lim + (e.peak - lim) * Math.exp(-Math.abs(t - (e.peakAt || e.time)) / Math.max(60e3, (end - e.time) / 3)) : 22 + 4 * Math.sin(t / 7e4));
      const seconds = [], minutes = [];
      if (live) for (let t = Math.ceil(from / 1e3) * 1e3; t < to; t += 2e3) seconds.push({ t, cpm: Math.max(5, Math.round(bump(t) + (Math.random() - 0.5) * 6)) });
      for (let t = Math.floor(from / 6e4) * 6e4; t < to; t += 6e4) minutes.push({ t, cpm: Math.round(bump(t + 3e4)) });
      return { event: { ...e, active: !!e.active }, from, to, end, seconds, minutes, limit: lim, attention: 50, alarm: settings.alarmCPM, danger: 1000, detailed: live };
    },
    GetActiveAlerts: async () => { const o = { perigo: 0, alarme: 1, atencao: 2 }; return demoAlerts.filter((e) => e.active).sort((x, y) => o[x.level] - o[y.level]); },
    ListPorts: async () => [{ name: "COM5", description: "USB-SERIAL CH340", isUSB: true }, { name: "COM1", description: "", isUSB: false }],
    DetectPort: async () => "COM5",
    Connect: async (p) => { status = { ...status, state: "conectado", device: { ...dev, port: p } }; emit("status", status); return status.device; },
    Disconnect: async () => { status = { ...status, state: "desconectado" }; emit("status", status); },
    SyncClock: async () => { dev.clockDrift = 0; emit("status", status); return 0; },
    DownloadHistory: async () => {
      const d = new Date(); d.setHours(0, 0, 0, 0);
      return { records: 900, added: 720, newRanges: [
        { from: d.getTime() + 1 * 36e5, to: d.getTime() + 7.5 * 36e5, minutes: 390, avgCPM: 19.6 },
        { from: d.getTime() - 864e5 + 13 * 36e5, to: d.getTime() - 864e5 + 17.25 * 36e5, minutes: 255, avgCPM: 31.4 },
      ] };
    },
    CancelDownload: async () => {},
    ExportCSV: async () => "",
    OpenDataFolder: async () => {},
    OpenConfigBackups: async () => {},
    RestoreDeviceConfig: async () => "",
    SnapshotDeviceConfig: async () => {},
    GetDemo: async () => demoSt,
    StartDemo: async (cpm, realistic) => (demoSt = { active: true, cpm, realistic }),
    StopDemo: async () => { demoAlerts = []; demoSt = { active: false, cpm: 20, realistic: false }; emit("event", { kind: "alerta" }); emit("demo", demoSt); return demoSt; },
    GetVersion: async () => "navegador",
    ExportMeasureReport: async (id) => { await new Promise((r) => setTimeout(r, 300)); return `C:\\Users\\voce\\Documents\\GMC Monitor\\Relatórios\\gmc-medicao-${id}.pdf`; },
    OpenReportFile: async () => {},
    GetVersionsDir: async () => "C:\\Users\\voce\\Documents\\GMC Monitor\\Versões",
    OpenVersionsDir: async () => {},
    GetSync: async () => demoSync,
    GetRadiacode: async () => demoRc(),
    SetRadiacode: async (on, ble) => { demoRcCfg.off = !on; demoRcCfg.ble = (ble || "").toUpperCase(); return demoRc(); },
    ReconnectRadiacode: async () => demoRc(),
    SetRcRaw: async (on) => { demoRcCfg.raw = on; return demoRc(); },
    ScanRadiacodeBLE: async () => { await new Promise((r) => setTimeout(r, 600)); return [{ address: "52:43:06:50:01:D7", name: "RadiaCode-102#RC-102-001234", rssi: -62 }]; },
    GetRcCompare: async (min) => {
      if (!demoRcFake()) return { minutes: 0, rcOnly: 0 };
      const n = Math.min(min, 600), t = [], gmc = [], rc = [];
      for (let i = 0; i < n; i++) { const g = 0.1 + 0.02 * Math.sin(i / 60) + (Math.random() - 0.5) * 0.04; t.push(Date.now() - (n - i) * 60e3); gmc.push(g); rc.push(g * 0.92 + (Math.random() - 0.5) * 0.02); }
      return { t, gmc, rc, gmcCPM: gmc.map((v) => v * 150), rcCPS: rc.map((v) => v * 30), ratio: 0.92, ratioErr: 0.01, corr: 0.81, minutes: n, rcOnly: 0 };
    },
    GetRcSpectrum: async () => {
      if (!demoRcFake()) throw new Error("RadiaCode não conectado");
      const counts = []; for (let i = 0; i < 1024; i++) counts.push(Math.round(4000 * Math.exp(-i / 60) + 300 * Math.exp(-0.5 * ((i - 250) / 9) ** 2) + Math.random() * 5));
      return { seconds: 3600, a: [-6, 2.45, 0.00042], counts, accum: false };
    },
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
    StartMeasure: async (o) => {
      const maxSec = o.maxSec || 1800, pct = o.pct || 20;
      meas = { phase: "fundo", seconds: o.auto ? 300 : o.seconds || 60, elapsed: 0, counts: 0, name: (o.name || "").trim(), distance: (o.distance || "").trim(), demo: demoSt.active,
        auto: !!o.auto, maxSec, pct, bg: null, obj: null, result: null, live: null, eta: -1, phaseStart: Date.now() };
      if (o.homeBg) {
        const h = demoHomeBg();
        meas.bg = { counts: h.counts, seconds: h.minutes * 60, cpm: h.cpm, errCPM: h.errCPM, usv: h.cpm * 0.0101, start: h.from, end: h.to, source: "casa" };
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
        r.avg = sum / p.length; r.max = mx.cpm; r.maxAt = mx.t; r.min = mn.cpm; r.minAt = mn.t; r.avgUSv = r.avg * 0.0101; r.vsBgPct = (r.avg - 20) / 20 * 100;
      }
      for (let t = from; t < to; t += step) { const q = range(t, t + step); r.curve.push({ t, cpm: q.length ? q.reduce((a, b) => a + b.cpm, 0) / q.length : null }); }
      for (const e of events.concat(demoAlerts)) if (e.kind === "alerta" && e.time >= from && e.time < to) { r[e.level === "perigo" ? "danger" : e.level === "alarme" ? "alarm" : "attention"]++; r.peakAlert = Math.max(r.peakAlert, e.peak || 0); }
      if (settings.rainEnabled && span > 20 * 36e5) { r.rainMM = 3.4; r.rainHours = 2; }
      const effTo = Math.min(to, Date.now());
      r.spanMinutes = Math.round((effTo - from) / 6e4);
      r.daysData = new Set(p.map((q) => new Date(q.t).toDateString())).size;
      r.gapMinutes = Math.max(0, r.spanMinutes - p.length);
      r.gaps = []; r.gapCount = 0;
      for (let i = 1; i < p.length; i++) if (p[i].t - p[i - 1].t >= 10 * 6e4) { r.gapCount++; r.gaps.unshift({ from: p[i - 1].t + 6e4, to: p[i].t, minutes: Math.round((p[i].t - p[i - 1].t - 6e4) / 6e4), cause: "contador desconectado", open: false }); }
      if (span >= 6 * 36e5) { r.gapCount++; r.gaps.unshift({ from: from + 3 * 36e5, to: from + 3 * 36e5 + 42 * 6e4, minutes: 42, cause: "app fechado e sem histórico do contador", open: false }); }
      if (p.length > 30) { const m = r.avg; r.sd = Math.sqrt(m) * 1.04; r.dispersion = 1.04; }
      return r;
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
    GetMeasurements: async () => measurements.slice(),
    UpdateMeasurement: async (id, name, note) => { measurements = measurements.map((m) => (m.id === id ? { ...m, name: name.trim() || m.name, note: note.trim() } : m)); setTimeout(() => emit("measurements", measurements.slice()), 0); },
    DeleteMeasurement: async (id) => { measurements = measurements.filter((m) => m.id !== id); setTimeout(() => emit("measurements", measurements.slice()), 0); },
    SavePeriod: async (p) => {
      if (!(p.to - p.from >= 6e4)) throw new Error("o período precisa ter pelo menos 1 minuto");
      p = { ...p, name: (p.name || "").trim() || "Fora de casa" };
      if (p.id) periods = periods.map((q) => (q.id === p.id ? p : q));
      else { p.id = Math.max(0, ...periods.map((q) => q.id)) + 1; periods.push(p); }
      periods.sort((a, b) => a.from - b.from);
      setTimeout(() => emit("periods", periods.slice()), 0);
      return p;
    },
    DeletePeriod: async (id) => { periods = periods.filter((q) => q.id !== id); setTimeout(() => emit("periods", periods.slice()), 0); },
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
      const [span, step] = { "3h": [108e5, 6e4], "6h": [216e5, 6e4], "12h": [432e5, 12e4], "24h": [864e5, 3e5], "7d": [6048e5, 36e5], "30d": [2592e6, 216e5], "1y": [31536e6, 864e5] }[period] || [864e5, 3e5];
      const to = Math.floor(Date.now() / 6e4) * 6e4 + 6e4;
      return api._dose(to - span, to, step, limit);
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
      return { from, to, stepMs: step, times, dose, background: bg, ceiling: ceil, total: d, backgroundTotal: b, ceilingTotal: b + al,
        allowance: al, excess: d - b, backgroundRate: rate, backgroundSource: "mediana dos últimos 30 dias", limitMSvYear: limY,
        measuredMinutes: p.length, coverage: Math.min(1, p.length / (span / 6e4)) };
    },
    GetChartRange: async (from, to) => withPeaks(await chartRangePreview(from, to)),
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
        avgUSv: avg == null ? null : avg * 0.0101, minUSv: null, maxUSv: null,
        attentionCPM: 50, attentionUSv: 50 * 0.0101, alarmCPM: settings.alarmCPM || 100, alarmUSv: (settings.alarmCPM || 100) * 0.0101, dangerCPM: 1000, dangerUSv: 10.1 };
    },
    GetChart: async (period) => {
      const c = await chartPreview(period);
      const u = (v) => (v == null ? null : v * 0.0101);
      withPeaks(c);
      if (demoRcFake()) { // RadiaCode fingido: segue o GMC um pouco abaixo
        c.rc = (c.usv || []).map((v, i) => (v == null || i < c.usv.length * 0.3 ? null : v * 0.9 + (Math.sin(i / 3) * 0.004)));
        const ok = c.rc.filter((v) => v != null);
        c.rcAvg = ok.length ? ok.reduce((a, b) => a + b, 0) / ok.length : null;
      }
      return Object.assign(c, { avgUSv: u(c.avg), minUSv: u(c.min), maxUSv: u(c.max) });
    },
    _unused: async (period) => {
      const [span, step] = { "15m": [9e5, 5e3], "30m": [18e5, 1e4], "24h": [864e5, 3e5], "7d": [6048e5, 18e5] }[period] || [3600e3, 6e4];
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
