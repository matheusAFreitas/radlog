"use strict";

// ---------- saída programada (2.75) ----------
//
// Spec ux/saida-programada.html. Marca antes que vai sair (para onde, por
// volta de que horas, quais aparelhos vão). As programadas aparecem no topo
// do Fora de casa; quando o aparelho que foi volta a conectar, o cartão do
// Agora pergunta "Você saiu mesmo?" (renderPlannedAsk, chamado pelo
// rcaway.js). Usa os globais do app.js, fora.js e rcaway.js.

let plEditing = null; // null = sem formulário; {id: 0} = nova
const plBackUse = {}; // id da pergunta → usar a volta da dica

async function refreshPlanned() {
  if (!api.GetPlannedTrips) return;
  try { state.planned = (await api.GetPlannedTrips()) || []; } catch (e) { state.planned = []; }
  renderPlanned();
}

function onPlanned(list) {
  state.planned = list || [];
  renderPlanned();
}

const plDay = (ms) => {
  const d = new Date(ms);
  return [d.toLocaleDateString(LOCALE, { weekday: "short" }).replace(".", ""), String(d.getDate())];
};
const plDate = (ms) => `${plDay(ms)[0]} ${ddmm(ms)}`;
function plWho(gmc, rc) {
  if (gmc && rc) return "vão o RC102 e o GMC-800";
  if (gmc) return "vai o GMC-800, o RC102 fica";
  return "vai o RC102, o GMC-800 fica";
}
function plWent(gmc, rc) {
  if (gmc && rc) return "foram o RC102 e o GMC-800";
  if (gmc) return "foi o GMC-800, o RC102 ficou em casa";
  return "foi o RC102, o GMC-800 ficou em casa";
}
function plWhere(gmc, rc) {
  if (gmc && rc) return "RC102 e GMC-800 fora";
  if (gmc) return "GMC-800 fora, RC102 em casa";
  return "RC102 fora, GMC-800 em casa";
}
const plDev = (d) => (d === "radiacode" ? "RC102" : "GMC-800");

// abre o formulário (de Fora de casa, do Ctrl+K e do "Saí de casa")
function openPlanForm(p) {
  plEditing = p ? { ...p } : { id: 0 };
  if (state.view !== "fora") showView("fora", true);
  renderPlanned();
  const f = $("plannedForm");
  if (f) f.scrollIntoView({ behavior: "smooth", block: "nearest" });
}

function renderPlanned() {
  const form = $("plannedForm"), box = $("plannedBox");
  if (!form || !box) return;
  form.textContent = "";
  form.hidden = !plEditing;
  if (plEditing) form.appendChild(planForm(plEditing));
  const btn = $("btnPlanAdd");
  if (btn) btn.hidden = !!plEditing;
  const list = state.planned || [];
  box.textContent = "";
  box.hidden = !list.length;
  if (!list.length) return;
  const h = document.createElement("h3");
  h.textContent = "Programadas";
  box.appendChild(h);
  const ul = document.createElement("ul");
  ul.className = "pl-list";
  for (const p of list) {
    const li = document.createElement("li");
    li.className = "pl-row";
    const [wd, dd] = plDay(p.de);
    const day = document.createElement("div");
    day.className = "pl-day";
    day.append(Object.assign(document.createElement("span"), { textContent: wd }), Object.assign(document.createElement("b"), { className: "mono", textContent: dd }));
    const t = document.createElement("div");
    t.className = "pl-t";
    const b = document.createElement("b");
    b.textContent = p.nome;
    b.dataset.noTr = "";
    const s = document.createElement("span");
    s.className = "small muted";
    s.textContent = `sai ~${hhmm(p.de)} · ${plWho(p.gmc, p.rc)}`;
    t.append(b, s);
    if (p.saiu) {
      const c = document.createElement("span");
      c.className = "small pl-started";
      c.textContent = `Começou: o ${plDev(p.saiuAparelho)} desconectou às ${hhmm(p.saiu)}. A volta fica aberta até ele conectar de novo.`;
      t.appendChild(c);
    }
    const acts = document.createElement("div");
    acts.className = "pl-acts";
    const link = (label, fn) => {
      const x = document.createElement("button");
      x.type = "button"; x.className = "link"; x.textContent = label;
      x.addEventListener("click", fn);
      acts.appendChild(x);
    };
    if (p.saiu) link("Não estou saindo", () => call(api.NotLeaving, p.id).then(refreshPlanned));
    link("editar", () => openPlanForm(p));
    link("cancelar", async () => {
      if (!(await askConfirm({ title: "Cancelar a saída programada?", text: `${asIs(p.nome)}, ${plDate(p.de)} por volta de ${hhmm(p.de)}.`, ok: "Cancelar a saída", danger: true }))) return;
      try { await api.DeletePlannedTrip(p.id); } catch (e) { toast(errMsg(e), true); }
      refreshPlanned();
    });
    li.append(day, t, acts);
    ul.appendChild(li);
  }
  box.appendChild(ul);
}

function planForm(p) {
  const box = document.createElement("div");
  box.className = "wh-edit pl-form";
  const h = document.createElement("b");
  h.textContent = p.id ? "Editar saída programada" : "Programar saída";
  const field = (label, input) => {
    const f = document.createElement("div");
    f.className = "wh-field";
    const l = document.createElement("label");
    l.textContent = label;
    l.htmlFor = input.id = "pl" + Math.random().toString(36).slice(2, 8);
    f.append(l, input);
    return f;
  };
  const name = document.createElement("input");
  name.type = "text"; name.maxLength = 60; name.value = p.nome || ""; name.placeholder = "ex.: Escritório";
  // quando: hoje, amanhã ou outro dia (abre a data)
  const now = new Date();
  const dayStr = (d) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
  const today = dayStr(now), tomorrow = dayStr(new Date(now.getFullYear(), now.getMonth(), now.getDate() + 1));
  const start = p.de ? new Date(p.de) : null;
  let day = start ? dayStr(start) : tomorrow;
  const date = document.createElement("input");
  date.type = "date"; date.value = day; date.min = today;
  const seg = document.createElement("div");
  seg.className = "seg";
  seg.setAttribute("role", "group");
  seg.setAttribute("aria-label", "Quando");
  const opts = [["hoje", today], ["amanhã", tomorrow], ["outro dia…", ""]];
  const syncSeg = () => {
    const k = day === today ? 0 : day === tomorrow ? 1 : 2;
    seg.querySelectorAll("button").forEach((b, i) => b.setAttribute("aria-pressed", String(i === k)));
    date.hidden = k !== 2;
  };
  opts.forEach(([label, v]) => {
    const b = document.createElement("button");
    b.type = "button"; b.textContent = label;
    b.addEventListener("click", () => {
      if (v) day = v;
      else { day = day === today || day === tomorrow ? dayStr(new Date(now.getFullYear(), now.getMonth(), now.getDate() + 2)) : day; date.value = day; }
      syncSeg();
      if (!v) date.focus();
    });
    seg.appendChild(b);
  });
  date.addEventListener("change", () => { if (date.value) day = date.value; syncSeg(); });
  const whenBox = document.createElement("div");
  whenBox.className = "wh-field";
  const whenLab = Object.assign(document.createElement("span"), { className: "lab", textContent: "Quando" });
  const whenRow = document.createElement("div");
  whenRow.className = "pl-when";
  whenRow.append(seg, date);
  whenBox.append(whenLab, whenRow);
  const time = document.createElement("input");
  time.type = "time"; time.value = start ? `${pad(start.getHours())}:${pad(start.getMinutes())}` : "05:30";
  syncSeg();
  // quais aparelhos vão
  const goes = { rc: p.id ? !!p.rc : true, gmc: p.id ? !!p.gmc : false };
  const devs = document.createElement("div");
  devs.className = "pl-devs";
  const devCard = (k, label, on, off) => {
    const l = document.createElement("label");
    l.className = "pl-dev";
    const c = Object.assign(document.createElement("input"), { type: "checkbox", checked: goes[k] });
    const tt = document.createElement("span");
    const b = Object.assign(document.createElement("b"), { textContent: label });
    const s = Object.assign(document.createElement("span"), { className: "small muted" });
    const upd = () => { goes[k] = c.checked; l.classList.toggle("on", c.checked); s.textContent = c.checked ? on : off; };
    c.addEventListener("change", upd);
    upd();
    tt.append(b, s);
    l.append(c, tt);
    return l;
  };
  devs.append(
    devCard("rc", "RC102 vai", "conta a dose da saída", "fica em casa medindo o fundo"),
    devCard("gmc", "GMC-800 vai", "conta a dose da saída", "fica em casa medindo o fundo"));
  const devBox = document.createElement("div");
  devBox.className = "wh-field";
  devBox.append(Object.assign(document.createElement("span"), { className: "lab", textContent: "Quais aparelhos vão comigo" }), devs);
  const note = document.createElement("p");
  note.className = "small muted";
  note.textContent = "Não precisa dizer a volta: ela é a hora em que o aparelho que foi conectar de novo no PC. O horário da saída não precisa ser exato: o app acerta pelo que os aparelhos mediram. Pode desligar o PC.";
  const err = document.createElement("div");
  err.className = "wh-err"; err.setAttribute("role", "alert"); err.hidden = true;
  const btns = document.createElement("div");
  btns.className = "row-btns";
  const cancel = Object.assign(document.createElement("button"), { type: "button", className: "btn small", textContent: "Cancelar" });
  const save = Object.assign(document.createElement("button"), { type: "button", className: "btn small accent", textContent: p.id ? "Salvar" : "Programar" });
  btns.append(cancel, save);
  const g2 = document.createElement("div");
  g2.className = "pl-g";
  g2.append(field("Para onde (vira o título da saída)", name), whenBox, field("Saio por volta de", time));
  box.append(h, g2, devBox, note, err, btns);
  cancel.addEventListener("click", () => { plEditing = null; renderPlanned(); });
  save.addEventListener("click", async () => {
    const at = new Date(`${day}T${time.value || "00:00"}`).getTime();
    if (!goes.rc && !goes.gmc) { err.textContent = "Escolha pelo menos um aparelho que vai com você."; err.hidden = false; return; }
    if (!(at > Date.now() - 3600e3)) { err.textContent = "Esse horário já passou. Para uma saída que já aconteceu, use \"Adicionar trecho passado\"."; err.hidden = false; return; }
    try {
      const r = await api.SavePlannedTrip({ id: p.id || 0, nome: name.value, de: at, gmc: goes.gmc, rc: goes.rc });
      plEditing = null;
      toast(`Saída programada: ${asIs(r.nome)}, ${plDate(r.de)} por volta de ${hhmm(r.de)}`);
      refreshPlanned();
    } catch (e) { err.textContent = errMsg(e); err.hidden = false; }
  });
  setTimeout(() => name.focus(), 0);
  return box;
}

// ---------- "Você saiu mesmo?" (cartão do Agora) ----------

const plTo = (q) => (plBackUse[q.id] && q.backHint ? q.backHint : q.to);

function plFromText(q) {
  const plan = `programado ~${hhmm(q.planAt)}`;
  if (q.source === "desconexao") return `Saída ${hhmm(q.from)} (o ${plDev(q.leftDev)} desconectou do PC; ${plan})`;
  if (q.source === "razao") return `Saída ${hhmm(q.from)} (achada pela conta RC102 ÷ GMC-800; ${plan})`;
  return `Saída ~${hhmm(q.from)} (horário estimado: nada nos aparelhos deu certeza)`;
}

async function plAnswer(q, ans, from, to, name, gmc, rc) {
  try {
    const p = await api.AnswerPlanned(q.id, ans, from || 0, to || 0, name || "", gmc == null ? !!q.gmcGoes : gmc, rc == null ? !!q.rcGoes : rc);
    rcaEditing = 0;
    clearTimeout(rcaDoneTimer);
    rcaDone = { ask: { ...q }, ans: ans === "sai" ? "programada" : "naosai", period: p };
    rcaDoneTimer = setTimeout(() => { rcaDone = null; renderRcAway(); }, 15000);
    await refreshRcAway();
  } catch (e) { toast(errMsg(e), true); }
}

// a resposta dada (no lugar do cartão, com Desfazer)
function plDoneText(box, d) {
  const t = document.createElement("span");
  const p = d.period;
  if (d.ans === "programada" && p && p.id) {
    const c = document.createElement("span");
    c.className = "chip off"; c.textContent = "saída marcada";
    t.append(c, ` ${asIs(p.name)} · ${plDate(p.from)} · ${hhmm(p.from)}–${hhmm(p.to)} · ${plWhere(p.gmc === "comigo", p.radiacode === "comigo")}`);
    box.appendChild(t);
    if (typeof openTrip === "function" && isTripP(p)) {
      const v = document.createElement("button");
      v.type = "button"; v.className = "link"; v.textContent = "Ver o resumo da saída";
      v.addEventListener("click", () => openTrip(p.id));
      box.appendChild(v);
    }
    return;
  }
  t.textContent = "Ok, nada foi marcado.";
  box.appendChild(t);
}

function renderPlannedAsk(box, q, list) {
  const head = document.createElement("div");
  head.className = "rca-head";
  head.innerHTML = q.missed ? '<span class="dot pl-miss" aria-hidden="true"></span>' : '<span class="dot rc" aria-hidden="true"></span>';
  const h = document.createElement("h2");
  h.textContent = q.missed ? `A saída programada de ${plDate(q.planAt)} não aconteceu?` : `Você saiu mesmo? ${asIs(q.name)}, ${plDate(q.planAt)}`;
  const tag = document.createElement("span");
  tag.className = "chip off"; tag.textContent = "a responder";
  head.append(h, tag);
  const sub = document.createElement("div");
  sub.className = "small muted";
  if (q.missed) sub.textContent = `O ${plDev(q.rcGoes ? "radiacode" : "gmc")} ficou com o mesmo jeito de casa o tempo todo (a conta RC102 ÷ GMC-800 não mudou). Nada foi marcado ainda.`;
  else sub.textContent = `${plFromText(q)} · volta ${hhmm(plTo(q))} (o ${plDev(q.dev)} conectou${plBackUse[q.id] ? ", pela dica" : ""}) · ${plWent(q.gmcGoes, q.rcGoes)}`;
  box.append(head, sub);
  if (q.backHint && !q.missed) {
    const hint = document.createElement("div");
    hint.className = "rca-hint small";
    const b = document.createElement("strong");
    b.textContent = `Parece que você chegou por volta das ${hhmm(q.backHint)}.`;
    const s = document.createElement("span");
    s.className = "muted";
    s.textContent = ` A conta RC102 ÷ GMC-800 voltou ao normal bem antes de o ${plDev(q.dev)} conectar às ${hhmm(q.to)}. `;
    const u = document.createElement("button");
    u.type = "button"; u.className = "link";
    u.textContent = plBackUse[q.id] ? `Usar ${hhmm(q.to)} (a conexão)` : `Usar ${hhmm(q.backHint)}`;
    u.addEventListener("click", () => { plBackUse[q.id] = !plBackUse[q.id]; renderRcAway(); });
    hint.append(b, s, u);
    box.appendChild(hint);
  }
  if (rcaEditing === q.id) { box.appendChild(plForm(q)); return; }
  const ask = document.createElement("div");
  ask.className = "rca-ask";
  const row = document.createElement("div");
  row.className = "row-btns";
  const btn = (label, cls, fn) => {
    const b = document.createElement("button");
    b.type = "button"; b.className = "btn" + (cls ? " " + cls : ""); b.textContent = label;
    b.addEventListener("click", fn);
    row.appendChild(b);
  };
  if (q.missed) {
    btn("Não fui, pode apagar", "", () => plAnswer(q, "naosai"));
    btn(`Fui sim, marcar ${hhmm(q.from)} até a conexão`, "rca-out", () => plAnswer(q, "sai", q.from, q.to));
  } else {
    btn("Saí, marcar", "rca-out sug", () => plAnswer(q, "sai", q.from, plTo(q)));
    btn("Ajustar horário ou aparelhos", "", () => { rcaEditing = q.id; renderRcAway(); });
    btn("Não saí", "", () => plAnswer(q, "naosai"));
  }
  ask.appendChild(row);
  if (list.length > 1) {
    const more = document.createElement("span");
    more.className = "small muted";
    more.textContent = list.length === 2 ? "Mais 1 trecho a responder em Fora de casa." : `Mais ${list.length - 1} trechos a responder em Fora de casa.`;
    ask.appendChild(more);
  }
  box.appendChild(ask);
}

function plForm(q) {
  const f = document.createElement("div");
  f.className = "rca-form";
  const g = document.createElement("div");
  g.className = "row-btns rca-fields";
  const field = (label, input) => {
    const l = document.createElement("label");
    l.className = "small muted";
    l.append(label, input);
    return l;
  };
  const from = Object.assign(document.createElement("input"), { type: "datetime-local", value: toLocalInput(q.from) });
  const to = Object.assign(document.createElement("input"), { type: "datetime-local", value: toLocalInput(plTo(q)) });
  const name = Object.assign(document.createElement("input"), { type: "text", maxLength: 60, value: q.name || "" });
  g.append(field("Saí em", from), field("Voltei em", to), field("Nome", name));
  const devs = document.createElement("div");
  devs.className = "row-btns";
  const pick = {};
  for (const [k, label, on] of [["rc", "RC102 foi comigo", q.rcGoes], ["gmc", "GMC-800 foi comigo", q.gmcGoes]]) {
    const l = document.createElement("label");
    l.className = "qk-go-row small";
    const c = Object.assign(document.createElement("input"), { type: "checkbox", checked: !!on });
    pick[k] = c;
    l.append(c, Object.assign(document.createElement("span"), { textContent: label }));
    devs.appendChild(l);
  }
  const err = document.createElement("div");
  err.className = "wh-err"; err.setAttribute("role", "alert"); err.hidden = true;
  const btns = document.createElement("div");
  btns.className = "row-btns";
  const save = Object.assign(document.createElement("button"), { type: "button", className: "btn accent", textContent: "Salvar" });
  const back = Object.assign(document.createElement("button"), { type: "button", className: "btn", textContent: "Voltar" });
  save.addEventListener("click", () => {
    const a = fromLocalInput(from.value), b = fromLocalInput(to.value);
    if (!(a < b)) { err.textContent = "A volta precisa ser depois da saída."; err.hidden = false; return; }
    if (!pick.rc.checked && !pick.gmc.checked) { err.textContent = "Escolha pelo menos um aparelho que foi com você."; err.hidden = false; return; }
    plAnswer(q, "sai", a, b, name.value, pick.gmc.checked, pick.rc.checked);
  });
  back.addEventListener("click", () => { rcaEditing = 0; renderRcAway(); });
  btns.append(save, back);
  f.append(g, devs, err, btns);
  setTimeout(() => from.focus(), 0);
  return f;
}

function wirePlanned() {
  const b = $("btnPlanAdd");
  if (b) b.addEventListener("click", () => openPlanForm(null));
  on("planned", onPlanned);
}
