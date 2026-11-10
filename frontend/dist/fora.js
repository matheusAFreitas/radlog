// Fora de casa: períodos fora, resumo de cada um e o editor.
// Carregado antes do app.js; só define funções e constantes.

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

function renderPeriods() {
  const ul = $("periodList");
  ul.textContent = "";
  if (!state.periods.length) { ul.innerHTML = '<li class="muted small">Nenhum período marcado.</li>'; renderPeriodTotal(); return; }
  renderPeriodTotal();
  for (const p of [...state.periods].reverse()) {
    const li = document.createElement("li");
    li.className = "p-row";
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
    rep.addEventListener("click", (e) => exportReport(p.from, Math.min(p.to, Date.now()), "Fora de casa: " + p.name, e.currentTarget));
    const head = document.createElement("div");
    head.className = "p-head";
    head.append(txt, see, rep, ed, del);
    li.append(head, periodStatsEl(p));
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
  setSub("graficos", "tempo");
  if (state.view !== "graficos") showView("graficos", true);
  state.zoom = { from, to }; state.zoomStack = [];
  state.doseZoom = { from, to }; state.doseZoomStack = [];
  state.zoomLinked = true;
  refreshChart(); refreshDose();
  $("cpmCard").scrollIntoView({ behavior: "smooth", block: "start" });
}

async function refreshPeriods() {
  try { state.periods = (await api.GetPeriods()) || []; } catch (e) { state.periods = []; }
  state.periodSum = {};
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
