// "O Radiacode estava fora de casa?" (2.38.0): quando o Radiacode volta
// depois de ficar sem conexão, o cartão no topo do Agora pergunta onde ele
// estava, com o que a memória do aparelho trouxe e uma dica pela razão de
// contagens com o GMC. As perguntas sem resposta também aparecem em Fora de
// casa como "a responder".
//
// Usa os globais do app.js e do fora.js (state, api, $, fmt, hhmm, hm,
// toLocalInput, fromLocalInput, zoomRangeText, toast, errMsg). Carregado
// antes do app.js.

let rcaEditing = 0, rcaDone = null, rcaDoneTimer = 0;

async function refreshRcAway() {
  if (!api.GetRcAwayAsks) return;
  try { state.rcAway = (await api.GetRcAwayAsks()) || []; } catch (e) { state.rcAway = []; }
  renderRcAway();
}

function onRcAway(list) {
  state.rcAway = list || [];
  renderRcAway();
  if (state.view === "fora" && typeof renderPeriods === "function") renderPeriods();
}

const rcaMin = (q) => Math.max(1, Math.round((q.to - q.from) / 60000));
const rcaUSv = (v) => (v > 0 ? fmt(v, usvDigits(v)) : "—");

function rcaTile(label, value, unit, cls) {
  const d = document.createElement("div");
  d.className = "rca-tile";
  const l = document.createElement("span");
  l.className = "small muted";
  l.textContent = label;
  const v = document.createElement("span");
  v.className = "mono rca-num" + (cls ? " " + cls : "");
  v.textContent = value;
  if (unit) { const u = document.createElement("span"); u.className = "small muted"; u.textContent = " " + unit; v.appendChild(u); }
  d.append(l, v);
  return d;
}

function rcaHintText(q) {
  if (q.hint === "fora") return ["Parece que ele saiu de casa.", `Com o RC102 em casa, ele conta umas ${fmt(q.normalRatio, 0)} vezes o que o GMC conta. Nesse trecho foram ${fmt(q.ratio, 0)} vezes.`];
  if (q.hint === "casa") return ["Parece que ele ficou em casa.", `A razão de contagens com o GMC ficou perto da normal (${fmt(q.ratio, 0)} contra ${fmt(q.normalRatio, 0)}).`];
  if (!q.memMinutes) return ["", "A memória do aparelho não trouxe leituras desse trecho, então o app não tem dica."];
  if (!q.ratio || !q.normalRatio) return ["", "Poucos minutos com os dois aparelhos para comparar, então o app não tem dica."];
  return ["", `A razão de contagens com o GMC ficou entre a normal e a de fora (${fmt(q.ratio, 0)} contra ${fmt(q.normalRatio, 0)}): sem dica.`];
}

async function rcaAnswer(q, ans, from, to, name) {
  try {
    const p = await api.AnswerRcAway(q.id, ans, from || 0, to || 0, name || "");
    rcaEditing = 0;
    clearTimeout(rcaDoneTimer);
    rcaDone = { ask: { id: q.id, from: q.from, to: q.to, kind: q.kind || "", period: q.period || 0 }, ans, period: p };
    rcaDoneTimer = setTimeout(() => { rcaDone = null; renderRcAway(); }, 10000);
    await refreshRcAway();
  } catch (e) { toast(errMsg(e), true); }
}

async function rcaUndo() {
  const d = rcaDone;
  if (!d) return;
  rcaDone = null;
  clearTimeout(rcaDoneTimer);
  try { await api.UndoRcAway(d.ask, (d.period && d.period.id) || 0); } catch (e) { toast(errMsg(e), true); }
  await refreshRcAway();
}

function renderRcAway() {
  const box = $("rcAwayBox");
  if (!box) return;
  box.textContent = "";
  const list = state.rcAway || [];
  if (rcaDone) {
    box.hidden = false;
    box.className = "card rca rca-done";
    const t = document.createElement("span");
    const p = rcaDone.period;
    if (rcaDone.ans === "voltei" && p && p.id) t.textContent = `Ok, marquei a volta às ${hhmm(p.to)}. O trecho "${p.name}" foi de ${zoomRangeText(p.from, p.to)}.`;
    else if (rcaDone.ans === "aindafora") t.textContent = "Ok, o trecho continua aberto. Quando voltar, use \"Voltei\".";
    else if (rcaDone.ans === "fora" && p && p.id) {
      const c = document.createElement("span");
      c.className = "chip off"; c.textContent = "fora de casa";
      t.append(c, ` Marcado: ${p.name} · RC102 com você, GMC-800 em casa · ${hhmm(p.from)}–${hhmm(p.to)}`);
    } else t.textContent = rcaDone.ans === "casa" ? "Ok, ficou como em casa." : "Ok, ficou sem marcar.";
    const u = document.createElement("button");
    u.type = "button"; u.className = "link"; u.textContent = "Desfazer";
    u.addEventListener("click", rcaUndo);
    box.append(t, u);
    return;
  }
  if (!list.length) { box.hidden = true; box.className = "card rca"; return; }
  box.hidden = false;
  box.className = "card rca";
  const q = list[list.length - 1]; // a mais recente primeiro
  if (q.kind === "volta") { renderRcBack(box, q, list); return; }
  const head = document.createElement("div");
  head.className = "rca-head";
  head.innerHTML = '<span class="dot rc" aria-hidden="true"></span>';
  const h = document.createElement("h2");
  h.textContent = `O RC102 ficou ${hm(rcaMin(q))} sem conexão`;
  const tag = document.createElement("span");
  tag.className = "chip off"; tag.textContent = "a responder";
  head.append(h, tag);
  const sub = document.createElement("div");
  sub.className = "mono small muted";
  sub.textContent = `${zoomRangeText(q.from, q.to)} · ` + (q.memMinutes ? `a memória do aparelho trouxe ${hm(q.memMinutes)}` : "a memória do aparelho não trouxe leituras");
  box.append(head, sub);

  if (q.memMinutes) {
    const g = document.createElement("div");
    g.className = "rca-tiles";
    g.append(
      rcaTile("média do RC102", rcaUSv(q.avgUSv), "µSv/h"),
      rcaTile("máximo", rcaUSv(q.maxUSv), q.maxAt ? `às ${hhmm(q.maxAt)}` : ""),
      rcaTile("dose no trecho", fmt(q.doseUSv, 2), "µSv"),
      rcaTile("GMC-800 em casa, mesmo horário", rcaUSv(q.gmcUSv), "µSv/h", "gmc"));
    box.appendChild(g);
  }
  const [hb, ht] = rcaHintText(q);
  const hint = document.createElement("div");
  hint.className = "rca-hint small";
  if (hb) { const b = document.createElement("strong"); b.textContent = hb; hint.append(b, " "); }
  const hs = document.createElement("span");
  hs.className = "muted"; hs.textContent = ht;
  hint.appendChild(hs);
  box.appendChild(hint);

  if (rcaEditing === q.id) { box.appendChild(rcaForm(q)); return; }
  const ask = document.createElement("div");
  ask.className = "rca-ask";
  const lab = document.createElement("b");
  lab.textContent = "Onde ele estava?";
  const row = document.createElement("div");
  row.className = "row-btns";
  const btn = (label, cls, fn) => {
    const b = document.createElement("button");
    b.type = "button"; b.className = "btn" + (cls ? " " + cls : ""); b.textContent = label;
    b.addEventListener("click", fn);
    row.appendChild(b);
    return b;
  };
  btn("Comigo, fora de casa", q.hint === "fora" ? "rca-out sug" : "rca-out", () => rcaAnswer(q, "fora"));
  btn("Em casa", q.hint === "casa" ? "sug" : "", () => rcaAnswer(q, "casa"));
  btn("Não sei", "", () => rcaAnswer(q, "naosei"));
  const adj = document.createElement("button");
  adj.type = "button"; adj.className = "link"; adj.textContent = "Ajustar horário";
  adj.addEventListener("click", () => { rcaEditing = q.id; renderRcAway(); });
  row.appendChild(adj);
  const note = document.createElement("span");
  note.className = "small muted";
  note.textContent = "\"Comigo, fora de casa\": essas medições saem do fundo da casa e do padrão por hora, e continuam na sua dose.";
  ask.append(lab, row, note);
  if (list.length > 1) {
    const more = document.createElement("span");
    more.className = "small muted";
    more.textContent = list.length === 2 ? "Mais 1 trecho a responder em Fora de casa." : `Mais ${list.length - 1} trechos a responder em Fora de casa.`;
    ask.appendChild(more);
  }
  box.appendChild(ask);
}

// "Voltou para casa?" (2.66.0): havia um "Saí de casa" aberto com o RC102
// junto e ele voltou a conectar no PC (ACE, 12/11)
function renderRcBack(box, q, list) {
  const head = document.createElement("div");
  head.className = "rca-head";
  head.innerHTML = '<span class="dot rc" aria-hidden="true"></span>';
  const h = document.createElement("h2");
  h.textContent = "Voltou para casa?";
  const tag = document.createElement("span");
  tag.className = "chip off"; tag.textContent = "a responder";
  head.append(h, tag);
  const sub = document.createElement("div");
  sub.className = "small muted";
  sub.textContent = `"${q.name || "Saí de casa"}" está aberto desde ${whenLabel(q.since || q.from)}. O RC102 voltou a conectar no PC às ${hhmm(q.to)}, depois de ${hm(rcaMin(q))} sem conexão.`;
  box.append(head, sub);
  if (q.memMinutes) {
    const g = document.createElement("div");
    g.className = "rca-tiles";
    g.append(
      rcaTile("média do RC102", rcaUSv(q.avgUSv), "µSv/h"),
      rcaTile("máximo", rcaUSv(q.maxUSv), q.maxAt ? `às ${hhmm(q.maxAt)}` : ""),
      rcaTile("dose sem conexão", fmt(q.doseUSv, 2), "µSv"),
      rcaTile("GMC-800 em casa, mesmo horário", rcaUSv(q.gmcUSv), "µSv/h", "gmc"));
    box.appendChild(g);
  }
  if (rcaEditing === q.id) { box.appendChild(rcaBackForm(q)); return; }
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
  btn(`Voltei às ${hhmm(q.to)}`, "accent", () => rcaAnswer(q, "voltei"));
  btn("Ainda estou fora", "", () => rcaAnswer(q, "aindafora"));
  const adj = document.createElement("button");
  adj.type = "button"; adj.className = "link"; adj.textContent = "Ajustar horário";
  adj.addEventListener("click", () => { rcaEditing = q.id; renderRcAway(); });
  row.appendChild(adj);
  const note = document.createElement("span");
  note.className = "small muted";
  note.textContent = "\"Voltei\" fecha o trecho fora de casa nesse horário e tudo volta ao normal.";
  ask.append(row, note);
  if (list.length > 1) {
    const more = document.createElement("span");
    more.className = "small muted";
    more.textContent = list.length === 2 ? "Mais 1 trecho a responder em Fora de casa." : `Mais ${list.length - 1} trechos a responder em Fora de casa.`;
    ask.appendChild(more);
  }
  box.appendChild(ask);
}

function rcaBackForm(q) {
  const f = document.createElement("div");
  f.className = "rca-form";
  const g = document.createElement("div");
  g.className = "row-btns rca-fields";
  const l = document.createElement("label");
  l.className = "small muted";
  const to = document.createElement("input");
  to.type = "datetime-local"; to.value = toLocalInput(q.to);
  l.append("Voltei em", to);
  const err = document.createElement("div");
  err.className = "wh-err"; err.setAttribute("role", "alert"); err.hidden = true;
  const save = document.createElement("button");
  save.type = "button"; save.className = "btn accent"; save.textContent = "Salvar";
  save.addEventListener("click", () => {
    const b = fromLocalInput(to.value);
    if (!(b > (q.since || 0)) || b > Date.now()) { err.textContent = "A volta precisa ser depois da saída e não pode estar no futuro."; err.hidden = false; return; }
    rcaAnswer(q, "voltei", 0, b);
  });
  const back = document.createElement("button");
  back.type = "button"; back.className = "btn"; back.textContent = "Cancelar";
  back.addEventListener("click", () => { rcaEditing = 0; renderRcAway(); });
  g.append(l, save, back);
  f.append(g, err);
  setTimeout(() => to.focus(), 0);
  return f;
}

// ajustar o horário: parte do tempo sem conexão pode ter sido em casa
function rcaForm(q) {
  const f = document.createElement("div");
  f.className = "rca-form";
  const t = document.createElement("b");
  t.textContent = "Quando você saiu e voltou com o RC102?";
  const bar = document.createElement("div");
  bar.className = "rca-bar";
  const sel = document.createElement("i");
  bar.appendChild(sel);
  const ends = document.createElement("div");
  ends.className = "rca-ends mono small muted";
  ends.innerHTML = "<span></span><span></span>";
  ends.firstChild.textContent = `${hhmm(q.from)} desconectou`;
  ends.lastChild.textContent = `${hhmm(q.to)} voltou`;
  const g = document.createElement("div");
  g.className = "row-btns rca-fields";
  const field = (label, input) => {
    const l = document.createElement("label");
    l.className = "small muted";
    l.append(label, input);
    return l;
  };
  const from = document.createElement("input");
  from.type = "datetime-local"; from.value = toLocalInput(q.from);
  const to = document.createElement("input");
  to.type = "datetime-local"; to.value = toLocalInput(q.to);
  const name = document.createElement("input");
  name.type = "text"; name.maxLength = 60; name.placeholder = "Saí com o RC102";
  const sync = () => {
    const a = fromLocalInput(from.value), b = fromLocalInput(to.value), span = q.to - q.from;
    const pa = Math.max(0, Math.min(100, ((a - q.from) / span) * 100)), pb = Math.max(0, Math.min(100, ((b - q.from) / span) * 100));
    sel.style.left = pa + "%"; sel.style.width = Math.max(1, pb - pa) + "%";
  };
  from.addEventListener("input", sync); to.addEventListener("input", sync);
  sync();
  const err = document.createElement("div");
  err.className = "wh-err"; err.setAttribute("role", "alert"); err.hidden = true;
  const save = document.createElement("button");
  save.type = "button"; save.className = "btn accent"; save.textContent = "Salvar";
  save.addEventListener("click", () => {
    const a = fromLocalInput(from.value), b = fromLocalInput(to.value);
    if (!(a < b)) { err.textContent = "A volta precisa ser depois da saída."; err.hidden = false; return; }
    rcaAnswer(q, "fora", a, b, name.value);
  });
  const back = document.createElement("button");
  back.type = "button"; back.className = "btn"; back.textContent = "Voltar";
  back.addEventListener("click", () => { rcaEditing = 0; renderRcAway(); });
  g.append(field("Saí em", from), field("Voltei em", to), field("Nome (opcional)", name), save, back);
  const note = document.createElement("span");
  note.className = "small muted";
  note.textContent = "O tempo sem conexão fora desse horário fica como em casa (o normal).";
  f.append(t, bar, ends, g, err, note);
  setTimeout(() => from.focus(), 0);
  return f;
}

// linhas "a responder" no topo da lista de Fora de casa
function rcAwayRows(ul) {
  for (const q of [...(state.rcAway || [])].reverse()) {
    const li = document.createElement("li");
    li.className = "wh-trow rca-row";
    const t = document.createElement("div");
    t.className = "t";
    const c = document.createElement("span");
    c.className = "chip off rca-pending"; c.textContent = "a responder";
    const when = document.createElement("div");
    when.className = "when";
    when.textContent = zoomRangeText(q.from, q.to);
    const what = document.createElement("div");
    what.className = "small muted";
    what.textContent = q.kind === "volta" ? `RC102 voltou a conectar às ${hhmm(q.to)}: voltou para casa?`
      : "RC102 sem conexão, " + (q.memMinutes ? `a memória trouxe ${hm(q.memMinutes)}` : "sem leituras da memória");
    t.append(c, when, what);
    const acts = document.createElement("div");
    acts.className = "acts";
    const go = document.createElement("button");
    go.type = "button"; go.className = "btn small"; go.textContent = "Responder";
    go.addEventListener("click", () => {
      // a escolhida vai para o fim: é a que o cartão do Agora mostra
      state.rcAway = (state.rcAway || []).filter((x) => x.id !== q.id).concat(q);
      showView("agora");
      renderRcAway();
    });
    acts.appendChild(go);
    li.append(t, acts);
    ul.appendChild(li);
  }
}
