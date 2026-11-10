// Fora de casa: trechos em que algum aparelho não estava no lugar de sempre
// (onde está cada aparelho, onde.js), linha do tempo, resumo de cada um e o
// editor. Carregado antes do app.js; só define funções e constantes.

// ---------- fora de casa ----------

// o trecho aberto (to = 0: até agora) vira "até agora" para as contas da tela
const normPeriods = (list) => (list || []).map((p) => (p.to ? p : { ...p, open: true, to: Date.now() }));

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

// números de um período: RadiaCode (você) e GMC-800 no mesmo tempo (state.periodSum fica no app.js)
function periodSummary(p) {
  const to = Math.min(p.to, Date.now());
  const key = `${p.from}-${to}`;
  // período que ainda está acontecendo: refaz a conta a cada abertura
  if (!state.periodSum[key] || p.to > Date.now()) state.periodSum[key] = api.GetSummary(p.from, to).catch(() => null);
  return state.periodSum[key];
}

function periodStatsEl(p) {
  const box = document.createElement("div");
  box.className = "p-stats mono";
  box.textContent = "…";
  periodSummary(p).then((s) => {
    box.textContent = "";
    if (!s) { box.textContent = "não deu para calcular"; return; }
    const line = (cls, label, text, warn) => {
      const d = document.createElement("div");
      d.className = cls;
      const b = document.createElement("b");
      b.textContent = label;
      const t = document.createElement("span");
      t.textContent = text;
      if (warn) t.className = "warn";
      d.append(b, t);
      box.appendChild(d);
    };
    const rc = s.rc && s.rc.minutes ? s.rc : null;
    if (rc) {
      const dg = (v) => fmt(v, usvDigits(v));
      const vs = rc.bgUSv ? (Math.abs(rc.vsBgPct) < 5 ? " (igual ao seu normal)" : ` (${rc.vsBgPct > 0 ? "+" : "−"}${fmt(Math.abs(rc.vsBgPct), 0)}% do seu normal)`) : "";
      const parts = [`${fmt(rc.dose, rc.dose < 10 ? 2 : 1)} µSv em ${hm(rc.minutes)}`, `média ${dg(rc.avg)} µSv/h${vs}`, `máx ${dg(rc.max)} µSv/h`];
      if (rc.coverage < 0.95) parts.push(`${fmt(rc.coverage * 100, 0)}% medido`);
      if (rc.danger || rc.alarm || rc.attention) parts.push(`alertas: ${alertsText(rc)}`);
      line("p-rc", "RadiaCode", parts.join(" · "), rc.bgUSv && rc.vsBgPct >= 20);
    } else {
      line("p-rc", "RadiaCode", "sem leitura no PC nesse período (com o app do celular conectado, os dados ficam no celular)");
    }
    if (s.minutes) {
      const parts = [`média ${fmt(s.avg, 1)} CPM`, `máx ${fmt(s.max, 0)} CPM`];
      if (s.coverage < 0.95) parts.push(`${fmt(s.coverage * 100, 0)}% medido`);
      if (s.danger || s.alarm || s.attention) parts.push(`alertas: ${alertsText(s)}`);
      line("p-gmc", "GMC-800", parts.join(" · "));
    } else line("p-gmc", "GMC-800", "sem leitura");
  });
  return box;
}

// total do RadiaCode fora de casa (todos os períodos)
async function renderPeriodTotal() {
  const el = $("periodTotal");
  const list = state.periods || [];
  if (!list.length) { el.hidden = true; return; }
  const sums = await Promise.all(list.map(periodSummary));
  let dose = 0, min = 0;
  for (const s of sums) if (s && s.rc && s.rc.minutes) { dose += s.rc.dose; min += s.rc.minutes; }
  el.hidden = !min;
  if (min) el.textContent = `RadiaCode fora de casa, somando os períodos: ${fmt(dose, dose < 10 ? 2 : 1)} µSv em ${hm(min)} medidos.`;
}

const WH_NAME = { gmc: "GMC-800", radiacode: "RadiaCode" };
const WH_NORMAL = { gmc: "casa", radiacode: "comigo" };
const whOf = (p) => ({ gmc: p.gmc || "comigo", radiacode: p.radiacode || "comigo" });
const WH_LOCTXT = { casa: "em casa", comigo: "comigo", fora: "fora do cálculo" };
const whKind = (w) => (w.gmc === "fora" && w.radiacode === "fora" ? "Os dois fora do cálculo"
  : w.gmc === "fora" ? "GMC-800 fora do cálculo"
  : w.radiacode === "fora" ? "RadiaCode fora do cálculo"
  : w.gmc === "comigo" && w.radiacode === "comigo" ? "Levei os dois"
  : w.gmc === "comigo" ? "Levei o GMC-800, deixei o RadiaCode"
  : w.radiacode === "casa" ? "Deixei os dois em casa" : "normal");
const whDur = (a, b) => hm(Math.max(1, Math.round((b - a) / 60000)));

// faixa de cada aparelho nos últimos 7 dias: lilás fora do lugar de sempre
// marcas de saúde (sem dado, saturando, sem contagens) da semana, buscadas uma vez por abertura
let whMarks = null, whMarksAt = 0;
async function loadMarks(t0, t1) {
  if (!api.GetDeviceMarks || (whMarks && Date.now() - whMarksAt < 60e3)) return;
  whMarksAt = Date.now();
  try { whMarks = (await api.GetDeviceMarks(t0, t1)) || []; } catch (e) { whMarks = []; }
  renderTimeline();
}
const WH_MARK = { "sem dado": "var(--nosig)", saturando: "var(--alarm)", "sem contagens": "var(--alarm)" };

function renderTimeline() {
  const box = $("whTimeline");
  const now = Date.now();
  const t1 = new Date(); t1.setHours(24, 0, 0, 0);
  const t0 = t1.getTime() - 7 * 864e5, span = t1.getTime() - t0;
  const pct = (t) => Math.max(0, Math.min(100, ((t - t0) / span) * 100));
  $("whTlRange").textContent = `${dayMonth(t0)} → ${dayMonth(now)}`;
  box.textContent = "";
  loadMarks(t0, now);
  const lane = (cls, label) => {
    const l = document.createElement("div");
    l.className = "wh-lane" + (cls ? " " + cls : "");
    const lb = document.createElement("span");
    if (label) lb.innerHTML = `<span class="dot ${label === "gmc" ? "gmc" : "rc"}"></span> `;
    if (label) lb.append(WH_NAME[label]);
    const bar = document.createElement("div");
    bar.className = "bar";
    l.append(lb, bar);
    box.appendChild(l);
    return bar;
  };
  const axis = lane("axis", "");
  for (let d = 0; d < 7; d++) {
    const s = document.createElement("span");
    const mid = t0 + d * 864e5 + 432e5;
    s.style.left = pct(mid) + "%";
    s.textContent = dayMonth(mid);
    axis.appendChild(s);
  }
  for (const dev of ["gmc", "radiacode"]) {
    const bar = lane("", dev);
    for (const p of state.periods || []) {
      const at = whOf(p)[dev];
      if (at === WH_NORMAL[dev] || p.to <= t0) continue;
      const a = pct(p.from), b = pct(p.to);
      const i = document.createElement("i");
      i.title = `${p.name} · ${zoomRangeText(p.from, p.to)}`;
      i.style.cssText = `left:${a}%;width:${Math.max(0.6, b - a)}%;background:var(${at === "fora" ? "--outc" : "--swap"})`;
      bar.appendChild(i);
    }
    // por cima: minutos em que o aparelho não mediu direito
    for (const m of whMarks || []) {
      if (m.device !== dev || m.to <= t0) continue;
      const a = pct(m.from), b = pct(m.to);
      const i = document.createElement("i");
      i.className = "mk";
      i.title = `${m.kind} · ${zoomRangeText(m.from, m.to)}`;
      i.style.cssText = `left:${a}%;width:${Math.max(0.4, b - a)}%;background:${WH_MARK[m.kind] || "var(--nosig)"}`;
      bar.appendChild(i);
    }
    const n = document.createElement("i");
    n.style.cssText = `left:${pct(now)}%;width:2px;background:var(--text)`;
    bar.appendChild(n);
  }
}
const dayMonth = (ms) => { const d = new Date(ms); return `${pad(d.getDate())}/${pad(d.getMonth() + 1)}`; };

let periodConfirm = 0, periodAdding = null;

// seletor casa/comigo/fora do cálculo do formulário
function whSeg(dev, value, disabled) {
  const f = document.createElement("div");
  f.className = "wh-field";
  const lab = document.createElement("span");
  lab.className = "lab";
  lab.textContent = WH_NAME[dev];
  const seg = document.createElement("div");
  seg.className = "seg loc";
  seg.setAttribute("role", "group");
  seg.setAttribute("aria-label", "Onde estava o " + WH_NAME[dev]);
  seg.dataset.dev = dev;
  for (const [v, t] of [["casa", "em casa"], ["comigo", "comigo"], ["fora", "fora do cálculo"]]) {
    const b = document.createElement("button");
    b.type = "button"; b.dataset.v = v; b.textContent = t; b.disabled = !!disabled;
    b.setAttribute("aria-pressed", String(v === value));
    b.classList.toggle("off", v !== WH_NORMAL[dev] && v !== "fora");
    b.classList.toggle("out", v === "fora");
    b.addEventListener("click", () => seg.querySelectorAll("button").forEach((x) => x.setAttribute("aria-pressed", String(x === b))));
    seg.appendChild(b);
  }
  f.append(lab, seg);
  return f;
}

// formulário de um trecho (novo: p.id = 0); o aberto só muda o nome
function periodForm(p, onDone) {
  const box = document.createElement("div");
  box.className = "wh-edit";
  const open = !!p.open;
  const g3 = document.createElement("div");
  g3.className = "g3";
  const field = (label, input) => {
    const f = document.createElement("div");
    f.className = "wh-field";
    const l = document.createElement("label");
    l.innerHTML = label;
    l.htmlFor = input.id = "pf" + Math.random().toString(36).slice(2, 8);
    f.append(l, input);
    return f;
  };
  const name = document.createElement("input");
  name.type = "text"; name.maxLength = 60; name.value = p.name || ""; name.placeholder = "ex.: viagem";
  const from = document.createElement("input");
  from.type = "datetime-local"; from.value = toLocalInput(p.from); from.disabled = open;
  const to = document.createElement("input");
  to.type = "datetime-local"; to.value = open ? "" : toLocalInput(p.to); to.disabled = open;
  if (open) to.placeholder = "até agora";
  g3.append(field("Nome ou motivo <em>opcional</em>", name), field("Início", from), field("Fim", to));
  const w = whOf(p);
  const pick = (seg) => seg.querySelector('[aria-pressed="true"]').dataset.v;
  const g2 = document.createElement("div");
  g2.className = "g2";
  const segG = whSeg("gmc", w.gmc, open), segR = whSeg("radiacode", w.radiacode, open);
  g2.append(segG, segR);
  // os dois em casa: "eu não estava em casa" deixa a dose pessoal em branco
  const nh = document.createElement("label");
  nh.className = "wh-nothome small";
  const nhBox = document.createElement("input");
  nhBox.type = "checkbox"; nhBox.checked = !!p.notHome;
  nh.append(nhBox, " eu não estava em casa (esqueci de levar os dois)");
  const both = () => pick(segG) === "casa" && pick(segR) === "casa";
  const syncNh = () => { nh.hidden = !both(); };
  [segG, segR].forEach((x) => x.addEventListener("click", () => setTimeout(syncNh, 0)));
  const err = document.createElement("div");
  err.className = "wh-err"; err.setAttribute("role", "alert"); err.hidden = true;
  const btns = document.createElement("div");
  btns.className = "row-btns";
  const save = document.createElement("button");
  save.type = "button"; save.className = "btn small accent"; save.textContent = p.id ? "Salvar" : "Adicionar trecho";
  const cancel = document.createElement("button");
  cancel.type = "button"; cancel.className = "btn small"; cancel.textContent = "Cancelar";
  btns.append(save, cancel);
  box.append(g3, g2, nh, err, btns);
  syncNh();
  cancel.addEventListener("click", () => onDone(false));
  save.addEventListener("click", async () => {
    const f = open ? p.from : fromLocalInput(from.value), t = open ? 0 : fromLocalInput(to.value);
    if (!open && !(f < t)) { err.textContent = "O fim precisa ser depois do início."; err.hidden = false; return; }
    try {
      await api.SavePeriod({ id: p.id || 0, from: f, to: t, name: name.value, gmc: pick(segG), radiacode: pick(segR), notHome: both() && nhBox.checked });
      toast(p.id ? "Trecho salvo" : "Trecho adicionado");
      onDone(true);
    } catch (e) { err.textContent = errMsg(e); err.hidden = false; }
  });
  setTimeout(() => name.focus(), 0);
  return box;
}

function renderPeriods() {
  renderTimeline();
  const add = $("periodAddForm");
  add.textContent = "";
  $("btnPeriodAdd").hidden = !!periodAdding;
  if (periodAdding) add.appendChild(periodForm(periodAdding, () => { periodAdding = null; renderPeriods(); }));
  const ul = $("periodList");
  ul.textContent = "";
  if (!state.periods.length) {
    ul.innerHTML = '<li class="muted small">Nenhum trecho. Quando levar ou deixar um aparelho fora do lugar de sempre, marque na tela Agora ou adicione aqui.</li>';
    renderPeriodTotal();
    return;
  }
  renderPeriodTotal();
  const list = [...state.periods].sort((a, b) => (b.open ? 1 : 0) - (a.open ? 1 : 0) || b.from - a.from);
  for (const p of list) {
    const li = document.createElement("li");
    li.className = "wh-trow" + (p.open ? " open" : "");
    const t = document.createElement("div");
    t.className = "t";
    const b = document.createElement("b");
    b.textContent = p.name;
    t.appendChild(b);
    if (p.open) { const c = document.createElement("span"); c.className = "chip off"; c.textContent = "valendo agora"; t.append(" ", c); }
    const w = whOf(p);
    const kind = document.createElement("div");
    kind.className = "wh-kind";
    kind.textContent = whKind(w);
    kind.hidden = p.name === whKind(w);
    const when = document.createElement("div");
    when.className = "when";
    when.textContent = p.open ? `desde ${zoomRangeText(p.from, p.from).split(" ")[0]} ${hhmm(p.from)} · ${whDur(p.from, p.to)} até agora` : zoomRangeText(p.from, p.to);
    const who = document.createElement("div");
    who.className = "who";
    for (const dev of ["gmc", "radiacode"]) {
      const c = document.createElement("span");
      c.className = "chip" + (w[dev] === "fora" ? " dead" : w[dev] !== WH_NORMAL[dev] ? " off" : "");
      c.innerHTML = `<span class="dot ${dev === "gmc" ? "gmc" : "rc"}"></span>`;
      c.append(`${WH_NAME[dev]} ${WH_LOCTXT[w[dev]]}`);
      who.appendChild(c);
    }
    if (p.notHome) { const c = document.createElement("span"); c.className = "chip"; c.textContent = "eu não estava em casa"; who.appendChild(c); }
    t.append(kind, when, who);
    const acts = document.createElement("div");
    acts.className = "acts";
    const btn = (label, cls, fn, title) => {
      const x = document.createElement("button");
      x.type = "button"; x.className = "btn small" + (cls ? " " + cls : ""); x.textContent = label;
      if (title) x.title = title;
      x.addEventListener("click", fn);
      acts.appendChild(x);
      return x;
    };
    if (p.open) btn("Voltar ao normal", "accent", () => call(api.SetWhere, "casa", "comigo", ""));
    btn("Ver", "", () => showRange(p.from, p.to));
    btn("Relatório (PDF)", "", (e) => exportReport(p.from, Math.min(p.to, Date.now()), "Fora de casa: " + p.name, e.currentTarget), "Relatório em PDF só desse trecho");
    btn(p.open ? "Renomear" : "Editar", "", () => { periodEditing = periodEditing === p.id ? 0 : p.id; periodConfirm = 0; renderPeriods(); });
    btn("Apagar…", "danger-btn", () => { periodConfirm = p.id; periodEditing = 0; renderPeriods(); });
    li.append(t, acts);
    if (periodConfirm === p.id) {
      const c = document.createElement("div");
      c.className = "wh-confirm";
      const msg = document.createElement("span");
      msg.textContent = "Apagar este trecho? Os minutos voltam a contar como normais na dose e no fundo.";
      const sp = document.createElement("span");
      sp.style.flex = "1";
      const ok = document.createElement("button");
      ok.type = "button"; ok.className = "btn small danger-btn"; ok.textContent = "Apagar";
      ok.addEventListener("click", async () => { periodConfirm = 0; await call(api.DeletePeriod, p.id); });
      const no = document.createElement("button");
      no.type = "button"; no.className = "btn small"; no.textContent = "Cancelar";
      no.addEventListener("click", () => { periodConfirm = 0; renderPeriods(); });
      c.append(msg, sp, ok, no);
      li.appendChild(c);
    }
    if (periodEditing === p.id) li.appendChild(periodForm(p, () => { periodEditing = 0; renderPeriods(); }));
    li.appendChild(periodStatsEl(p));
    ul.appendChild(li);
  }
}

// trecho passado novo (prefill: de/até, vindos do zoom de um gráfico)
function editPeriod(prefill) {
  const now = Date.now();
  periodEditing = 0;
  periodAdding = { id: 0, name: "", from: prefill && prefill.from ? prefill.from : now - 3 * 3600e3, to: prefill && prefill.to ? Math.min(prefill.to, now) : now, gmc: "comigo", radiacode: "comigo" };
  renderPeriods();
}

function openPeriods(prefill) {
  periodAdding = null;
  showView("fora", true);
  if (prefill) editPeriod(prefill);
  else renderPeriods();
}

// abre um trecho nos dois gráficos (mesmo jeito do mapa de calor)
function showRange(from, to) {
  setSub("graficos", "tempo");
  if (state.view !== "graficos") showView("graficos", true);
  state.zoom = { from, to }; state.zoomStack = [];
  state.doseZoom = { from, to }; state.doseZoomStack = [];
  state.zoomLinked = true;
  refreshChart(); refreshDose();
  $("cpmCard").scrollIntoView({ behavior: "smooth", block: "start" });
}

async function refreshPeriods() {
  try { state.periods = normPeriods(await api.GetPeriods()); } catch (e) { state.periods = []; }
  state.periodSum = {};
  whMarks = null;
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
  $("btnPeriodAdd").addEventListener("click", () => editPeriod(null));
}
