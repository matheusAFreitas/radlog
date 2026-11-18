// Fora de casa: trechos em que algum aparelho não estava no lugar de sempre
// (onde está cada aparelho, onde.js), linha do tempo, resumo de cada um e o
// editor. Carregado antes do app.js; só define funções e constantes.

// ---------- fora de casa ----------

// o trecho aberto (to = 0: até agora) vira "até agora" para as contas da tela
const normPeriods = (list) => (list || []).map((p) => (p.to ? p : { ...p, open: true, to: Date.now() }));
// fim de um trecho agora (o aberto anda junto com o relógio, não para na hora em que a lista chegou)
const pTo = (p) => (p.open ? Math.max(p.to, Date.now()) : p.to);

const periodAt = (ms) => (state.periods || []).find((p) => ms >= p.from && ms < pTo(p)) || null;
// 2.101.9: nome padrão de trecho é do app e traduz; o que a pessoa escreveu fica como está
const P_DEF_NAMES = new Set(["", "Fora de casa", "Saída programada", "Saí de casa", "Saí com o RC102", "Os dois fora do cálculo", "GMC-800 fora do cálculo", "RC102 fora do cálculo", "Levei os dois", "Levei o GMC-800, deixei o RC102", "Deixei os dois em casa"]);
// os motivos prontos do chip (WH_REASONS, mais abaixo) também são do app
const pNameTxt = (n) => (P_DEF_NAMES.has(n || "") || WH_REASONS.includes(n) ? n || "" : asIs(n));
const awayLabel = (ms) => { const p = periodAt(ms); return p ? ` · ${pNameTxt(p.name)}` : ""; };

// faixas lilás no gráfico para os períodos fora de casa
function awayBands(t0, t1, xt) {
  return (state.periods || []).filter((p) => pTo(p) > t0 && p.from < t1).map((p) => {
    const x1 = xt(Math.max(p.from, t0)), x2 = xt(Math.min(pTo(p), t1));
    return `<rect x="${x1.toFixed(1)}" y="0" width="${Math.max(1, x2 - x1).toFixed(1)}" height="${H}" fill="${COR.away}" fill-opacity="0.13"/>`;
  }).join("");
}

const toLocalInput = (ms) => { const d = new Date(ms); return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`; };
const fromLocalInput = (s) => (s ? new Date(s).getTime() : NaN);

let periodEditing = 0;

// números de um período: Radiacode (você) e GMC-800 no mesmo tempo (state.periodSum fica no app.js)
function periodSummary(p) {
  const to = Math.min(pTo(p), Date.now());
  const key = `${p.from}-${to}`;
  // período que ainda está acontecendo: refaz a conta a cada abertura
  if (!state.periodSum[key] || p.open) state.periodSum[key] = api.GetSummary(p.from, to).catch(() => null);
  return state.periodSum[key];
}

// só os aparelhos que foram junto (ACE, 12/11): o que ficou em casa mede a casa
const whWent = (p) => { const w = whOf(p); return ["radiacode", "gmc"].filter((d) => w[d] === "comigo"); };
// menos que isso do trecho medido: o aparelho provavelmente não foi junto
const P_FEW = 0.1;

function periodStatsEl(p) {
  const box = document.createElement("div");
  box.className = "p-stats";
  const went = whWent(p);
  if (!went.length) { box.hidden = true; return box; }
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
      return d;
    };
    // marcado "comigo" mas quase sem leitura: oferece tirar do cálculo
    const few = (dev, cls) => {
      const d = line(cls, WH_NAME[dev], `quase sem leitura nesse trecho`);
      if (p.open) return;
      const b = document.createElement("button");
      b.type = "button"; b.className = "link small-link"; b.textContent = "Não estava comigo";
      b.title = `Marca o ${WH_NAME[dev]} como fora do cálculo neste trecho`;
      b.addEventListener("click", async () => {
        const w = whOf(p), nw = { ...w, [dev]: "fora" };
        const name = p.name === whKind(w) ? "" : p.name;
        try { await api.SavePeriod({ id: p.id, from: p.from, to: p.to, name, gmc: nw.gmc, radiacode: nw.radiacode, notHome: p.notHome }); }
        catch (e) { toast(errMsg(e), true); }
      });
      d.append(" ", b);
    };
    const dg = (v) => fmt(v, usvDigits(v));
    if (went.includes("radiacode")) {
      const rc = s.rc && s.rc.minutes ? s.rc : null;
      if (!rc || rc.coverageAll < P_FEW) few("radiacode", "p-rc");
      else {
        const vs = rc.bgUSv ? (Math.abs(rc.vsBgPct) < 5 ? " (igual ao seu normal)" : ` (${rc.vsBgPct > 0 ? "+" : "−"}${fmt(Math.abs(rc.vsBgPct), 0)}% do seu normal)`) : "";
        // 2.94 (M5): a dose da saída em destaque; o resto na linha de baixo
        const ds = document.createElement("div");
        ds.className = "p-dose";
        ds.innerHTML = '<b class="mono"></b> <span class="small muted"></span>';
        ds.children[0].textContent = `${fmt(rc.dose, rc.dose < 10 ? 2 : 1)} µSv`;
        ds.children[1].textContent = `de dose em ${hm(rc.minutes)}, pelo RC102`;
        box.appendChild(ds);
        const parts = [`média ${dg(rc.avg)} µSv/h${vs}`, `máx ${dg(rc.max)} µSv/h`];
        if (rc.coverage < 0.95) parts.push(`${fmt(rc.coverage * 100, 0)}% medido`);
        if (rc.danger || rc.alarm || rc.attention) parts.push(`alertas: ${alertsText(rc)}`);
        line("p-rc", "RC102", parts.map((x) => tr(x)).join(" · "), rc.bgUSv && rc.vsBgPct >= 20);
      }
    }
    if (went.includes("gmc")) {
      if (!s.minutes || s.coverageAll < P_FEW) few("gmc", "p-gmc");
      else {
        // em µSv/h com a calibração de cada minuto, como no relatório
        const parts = [`média ${usvVal(s.avgUSv)}`, `máx ${s.maxUSv != null ? usvVal(s.maxUSv) : fmt(s.max, 0) + " CPM"}`];
        if (s.coverage < 0.95) parts.push(`${fmt(s.coverage * 100, 0)}% medido`);
        if (s.danger || s.alarm || s.attention) parts.push(`alertas: ${alertsText(s)}`);
        line("p-gmc", "GMC-800", parts.join(" · "));
      }
    }
  });
  return box;
}

// total do Radiacode fora de casa (só as saídas: 2.101.8)
async function renderPeriodTotal() {
  const el = $("periodTotal");
  const list = (state.periods || []).filter(isTripP);
  if (!list.length) { el.hidden = true; return; }
  const sums = await Promise.all(list.map(periodSummary));
  let dose = 0, min = 0;
  for (const s of sums) if (s && s.rc && s.rc.minutes) { dose += s.rc.dose; min += s.rc.minutes; }
  el.hidden = !min;
  if (min) el.textContent = `RC102 fora de casa, somando os períodos: ${fmt(dose, dose < 10 ? 2 : 1)} µSv em ${hm(min)} medidos.`;
}

const WH_NAME = { gmc: "GMC-800", radiacode: "RC102" };
const WH_NORMAL = { gmc: "casa", radiacode: "comigo" };
const whOf = (p) => ({ gmc: p.gmc || "comigo", radiacode: p.radiacode || "comigo", notHome: !!p.notHome });
// saí com o Radiacode e deixei o GMC (2.38.0): o Radiacode fora do fundo da casa
const whOut = (w) => (w.gmc === "casa" || w.gmc === "fora") && w.radiacode === "comigo" && !!w.notHome;
const WH_LOCTXT = { casa: "em casa", comigo: "comigo", fora: "ignorado" };
// 2.94 (M5): três palavras (em casa, comigo, ignorar). Os nomes guardados
// continuam os de antes ("… fora do cálculo"); só a tela troca
// motivos prontos do chip (2.101.8): gravados em português, mostrados no idioma da tela
const WH_REASONS = ["quebrado", "em manutenção", "desligado de propósito", "leitura não confiável"];
const whWord = (s) => ({ "Os dois fora do cálculo": "Os dois ignorados", "GMC-800 fora do cálculo": "GMC-800 ignorado", "RC102 fora do cálculo": "RC102 ignorado" })[s] || s;
const whKind = (w) => (w.gmc === "fora" && w.radiacode === "fora" ? "Os dois fora do cálculo"
  : w.gmc === "fora" ? "GMC-800 fora do cálculo"
  : w.radiacode === "fora" ? "RC102 fora do cálculo"
  : w.gmc === "comigo" && w.radiacode === "comigo" ? "Levei os dois"
  : w.gmc === "comigo" ? "Levei o GMC-800, deixei o RC102"
  : w.radiacode === "casa" ? "Deixei os dois em casa"
  : whOut(w) ? "Saí com o RC102" : "normal");
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
    if (d % 2) s.className = "alt"; // some em tela estreita: os rótulos se encostavam
    const mid = t0 + d * 864e5 + 432e5;
    s.style.left = pct(mid) + "%";
    s.textContent = dayMonth(mid);
    axis.appendChild(s);
  }
  for (const dev of ["gmc", "radiacode"]) {
    const bar = lane("", dev);
    for (const p of state.periods || []) {
      const at = whOf(p)[dev];
      if ((at === WH_NORMAL[dev] && !(dev === "radiacode" && whOut(whOf(p)))) || pTo(p) <= t0) continue;
      const a = pct(p.from), b = pct(pTo(p));
      const i = document.createElement("i");
      i.title = `${pNameTxt(p.name)} · ${zoomRangeText(p.from, p.to)}`;
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
  seg.setAttribute("aria-label", `Onde estava o ${WH_NAME[dev]}`);
  seg.dataset.dev = dev;
  for (const [v, t] of [["casa", "em casa"], ["comigo", "comigo"], ["fora", "ignorar"]]) {
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
  // 2.101.5: nome automático fica vazio (segue os aparelhos ao salvar) e aparece como dica
  const autoName = p.name && typeof whOf === "function" && p.name === whKind(whOf(p));
  name.type = "text"; name.maxLength = 60; name.value = autoName ? "" : p.name || ""; name.placeholder = autoName ? tr(p.name) : "ex.: viagem";
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
  const nhTxt = document.createElement("span");
  nh.append(nhBox, nhTxt);
  // os dois em casa: esqueci de levar; GMC em casa e Radiacode comigo: saí com ele (2.38.0)
  // 2.101.8: com o GMC-800 ignorado (tubo quebrado) também dá para sair com o RC102
  const both = () => (pick(segG) === "casa" || pick(segG) === "fora") && (pick(segR) === "casa" || pick(segR) === "comigo");
  const syncNh = () => {
    nh.hidden = !both();
    nhTxt.textContent = pick(segR) === "comigo" ? " eu não estava em casa (saí com o RC102)" : " eu não estava em casa (esqueci de levar os dois)";
  };
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

// menu "⋯": <details> com a lista embaixo; fecha ao escolher, com Esc ou ao
// clicar fora
function moreMenu(label, items) {
  const d = document.createElement("details");
  d.className = "more-menu";
  const sm = document.createElement("summary");
  sm.className = "btn small icon";
  sm.setAttribute("aria-label", label);
  sm.title = label;
  sm.textContent = "⋯";
  const box = document.createElement("div");
  box.className = "more-list";
  box.setAttribute("role", "menu");
  for (const [t, cls, fn] of items) {
    const b = document.createElement("button");
    b.type = "button"; b.setAttribute("role", "menuitem"); b.textContent = t;
    if (cls) b.className = cls;
    b.addEventListener("click", () => { d.open = false; fn(); });
    box.appendChild(b);
  }
  d.append(sm, box);
  return d;
}
document.addEventListener("click", (e) => {
  document.querySelectorAll("details.more-menu[open]").forEach((d) => { if (!d.contains(e.target)) d.open = false; });
});
document.addEventListener("keydown", (e) => {
  if (e.key !== "Escape") return;
  document.querySelectorAll("details.more-menu[open]").forEach((d) => { d.open = false; d.querySelector("summary").focus(); });
});

// 2.101.5: redesenhar a lista (troca de tela, evento) não apaga o que se
// digitou num formulário aberto: copia os campos do velho para o novo
let keepSeq = 0;
const keepKey = (o) => (typeof o !== "object" ? String(o) : String(o.__keep || (Object.defineProperty(o, "__keep", { value: ++keepSeq }), o.__keep)));
function keepInputs(oldRoot, newRoot, obj) {
  if (!newRoot) return;
  // só o formulário da mesma coisa (outro trecho aberto não herda nada)
  if (obj) newRoot.dataset.keep = keepKey(obj);
  if (!oldRoot || (obj && oldRoot.dataset.keep !== newRoot.dataset.keep)) return;
  const a = [...oldRoot.querySelectorAll("input,select,textarea")], b = [...newRoot.querySelectorAll("input,select,textarea")];
  if (a.length !== b.length) return;
  a.forEach((x, i) => {
    const y = b[i];
    if (x.tagName !== y.tagName || x.type !== y.type) return;
    if (x.type === "checkbox" || x.type === "radio") y.checked = x.checked; else y.value = x.value;
  });
}

function renderPeriods() {
  renderTimeline();
  const add = $("periodAddForm");
  const oldAdd = add.firstElementChild, oldEdit = document.querySelector("#periodList .wh-edit");
  add.textContent = "";
  $("btnPeriodAdd").hidden = !!periodAdding;
  if (periodAdding) { const f = periodForm(periodAdding, () => { periodAdding = null; renderPeriods(); }); add.appendChild(f); keepInputs(oldAdd, f, periodAdding); }
  const ul = $("periodList");
  ul.textContent = "";
  if (typeof rcAwayRows === "function") rcAwayRows(ul);
  if (!state.periods.length) {
    if (!ul.children.length) ul.innerHTML = '<li class="muted small">Nenhum trecho. Quando levar ou deixar um aparelho fora do lugar de sempre, marque na tela Agora ou adicione aqui.</li>';
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
    b.textContent = whWord(p.name);
    if (p.name !== whKind(whOf(p)) && !WH_REASONS.includes(p.name)) b.dataset.noTr = "";
    t.appendChild(b);
    if (p.open) { const c = document.createElement("span"); c.className = "chip off"; c.textContent = "valendo agora"; t.append(" ", c); }
    const w = whOf(p);
    const kind = document.createElement("div");
    kind.className = "wh-kind";
    kind.textContent = whWord(whKind(w));
    kind.hidden = p.name === whKind(w);
    const when = document.createElement("div");
    when.className = "when";
    when.textContent = p.open ? `desde ${zoomRangeText(p.from, p.from).split(" ")[0]} ${hhmm(p.from)} · ${whDur(p.from, pTo(p))} até agora` : zoomRangeText(p.from, p.to);
    const who = document.createElement("div");
    who.className = "who";
    for (const dev of ["gmc", "radiacode"]) {
      // só o que foi junto (ou saiu do cálculo); o que ficou em casa não aparece
      if (w[dev] === "casa") continue;
      const c = document.createElement("span");
      c.className = "chip" + (w[dev] === "fora" ? " dead" : w[dev] !== WH_NORMAL[dev] || (dev === "radiacode" && whOut(w)) ? " off" : "");
      c.innerHTML = `<span class="dot ${dev === "gmc" ? "gmc" : "rc"}"></span>`;
      c.append(`${WH_NAME[dev]} ${WH_LOCTXT[w[dev]]}`);
      who.appendChild(c);
    }
    if (p.notHome) { const c = document.createElement("span"); c.className = "wh-note"; c.textContent = "você não estava em casa"; who.appendChild(c); }
    if (p.planned) { const c = document.createElement("span"); c.className = "wh-note"; c.textContent = "programada"; who.appendChild(c); }
    if (p.estimated) { const c = document.createElement("span"); c.className = "chip off"; c.textContent = "horário estimado"; c.title = "A saída ficou no horário programado: nada nos aparelhos deu certeza."; who.appendChild(c); }
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
    if (isTripP(p)) btn("Resumo da saída", "away-btn", () => openTrip(p.id), "Dose, gráfico, espectro e como o RC102 estava");
    btn("Ver no gráfico", "", () => showRange(p.from, pTo(p)));
    // o resto fica no "⋯" (2.85)
    const more = moreMenu("Mais ações deste trecho", [
      ["Relatório (PDF)", "", () => exportReport(p.from, Math.min(pTo(p), Date.now()), "Fora de casa: " + asIs(p.name), more.querySelector("summary"))],
      [p.open ? "Renomear" : "Editar", "", () => { periodEditing = periodEditing === p.id ? 0 : p.id; periodConfirm = 0; renderPeriods(); }],
      ["Apagar…", "danger", () => { periodConfirm = p.id; periodEditing = 0; renderPeriods(); }],
    ]);
    acts.appendChild(more);
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
    if (periodEditing === p.id) { const f = periodForm(p, () => { periodEditing = 0; renderPeriods(); }); li.appendChild(f); keepInputs(oldEdit, f, "trecho:" + p.id); }
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

function wirePeriods() {
  $("btnZoomAway").addEventListener("click", () => state.zoom && openPeriods({ from: state.zoom.from, to: state.zoom.to }));
  $("btnZoomNote").addEventListener("click", () => state.zoom && openNote({ id: 0, time: state.zoom.from, end: Math.min(Date.now(), state.zoom.to), text: "" }));
  $("btnDoseZoomAway").addEventListener("click", () => state.doseZoom && openPeriods({ from: state.doseZoom.from, to: state.doseZoom.to }));
  $("btnPeriodAdd").addEventListener("click", () => editPeriod(null));
}
