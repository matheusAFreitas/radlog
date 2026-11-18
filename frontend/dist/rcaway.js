// "O Radiacode estava fora de casa?" (2.38.0): quando o Radiacode volta
// depois de ficar sem conexão, o cartão no topo do Agora pergunta onde ele
// estava, com o que a memória do aparelho trouxe e uma dica pela razão de
// contagens com o GMC. As perguntas sem resposta também aparecem em Fora de
// casa como "a responder".
//
// A pergunta da saída programada (2.75) é desenhada pelo programada.js. A dos
// trechos novos do histórico do GMC-800 (2.83.0, "gmc" e "ambos") fica no
// fim deste arquivo.
//
// Usa os globais do app.js e do fora.js (state, api, $, fmt, hhmm, hm,
// toLocalInput, fromLocalInput, zoomRangeText, toast, errMsg). Carregado
// antes do app.js.

let rcaEditing = 0, rcaDone = null, rcaDoneTimer = 0;
const awLater = new Set(); // "Responder depois" (2.83.0): fica só em Fora de casa até reabrir o app
let awFocus = "from";

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
const rcaUSv = (v) => (v > 0 ? rateNum(v, usvDigits(v)) : "—"); // na unidade da tela (2.84.0)

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

// perguntas abertas com "Ver detalhes" (o resto fica numa linha)
const rcaOpen = new Set();

function rcaCompact(box, q, list) {
  box.className = "card rca rca-1";
  const rc = !q.kind || q.kind === "rc";
  const dots = document.createElement("span");
  dots.className = "rca1-dots";
  dots.innerHTML = q.kind === "ambos" ? '<span class="dot gmc"></span><span class="dot rc"></span>' : `<span class="dot ${rc ? "rc" : "gmc"}"></span>`;
  dots.setAttribute("aria-hidden", "true");
  const t = document.createElement("div");
  t.className = "rca1-t";
  const hint = rc ? rcaHintText(q)[0] : (awHintText(q) || [""])[0];
  t.append(rc ? `O RC102 ficou ${hm(rcaMin(q))} sem conexão.` : "Você estava fora de casa?");
  if (hint) { const s = document.createElement("span"); s.className = "rca1-hint"; s.textContent = " " + hint; t.appendChild(s); }
  const sm = document.createElement("small");
  const parts = [rc ? zoomRangeText(q.from, q.to) : awSub(q)];
  if (rc && q.memMinutes && q.avgUSv > 0) parts.push(`média ${rateTxt(q.avgUSv)}`);
  if (!rc && q.gmcMinutes && q.gmcUSv > 0) parts.push(`média ${rateTxt(q.gmcUSv)}`);
  if (rc && q.gmcUSv > 0) parts.push(`GMC-800 em casa ${rateTxt(q.gmcUSv)}`);
  if (list.length > 1) parts.push(list.length === 2 ? "mais 1 a responder" : `mais ${list.length - 1} a responder`);
  sm.textContent = parts.join(" · ");
  t.appendChild(sm);
  const row = document.createElement("div");
  row.className = "rca1-btns";
  const btn = (label, cls, fn) => {
    const b = document.createElement("button");
    b.type = "button"; b.className = "btn" + (cls ? " " + cls : ""); b.textContent = label;
    b.addEventListener("click", fn);
    row.appendChild(b);
  };
  if (rc) {
    btn("Comigo, fora de casa", q.hint === "fora" ? "rca-out sug" : "rca-out", () => rcaAnswer(q, "fora"));
    btn("Em casa", q.hint === "casa" ? "sug" : "", () => rcaAnswer(q, "casa"));
  } else {
    btn("Estava em casa", q.hint === "casa" ? "sug" : "", () => awAnswer(q, "casa"));
    btn("Saí de casa…", "rca-out" + (q.hint === "fora" ? " sug" : ""), () => { rcaEditing = q.id; awFocus = "name"; renderRcAway(); });
  }
  const more = document.createElement("button");
  more.type = "button"; more.className = "link"; more.textContent = "Ver detalhes";
  more.addEventListener("click", () => { rcaOpen.add(q.id); renderRcAway(); });
  row.appendChild(more);
  // 2.90.0 (avaliação A3): a pergunta pode ser fechada; continua em Fora de
  // casa como "a responder" e volta quando o app abrir de novo
  const x = document.createElement("button");
  x.type = "button"; x.className = "rca-x"; x.textContent = "×";
  x.setAttribute("aria-label", "Fechar a pergunta");
  x.title = "Fechar: a pergunta continua em Fora de casa";
  x.addEventListener("click", () => { awLater.add(q.id); renderRcAway(); });
  row.appendChild(x);
  box.append(dots, t, row);
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
    if (rcaDone.ans === "awsai" || rcaDone.ans === "awcasa") {
      awDoneText(box, rcaDone);
      const u = document.createElement("button");
      u.type = "button"; u.className = "link"; u.textContent = "Desfazer";
      u.addEventListener("click", rcaUndo);
      box.appendChild(u);
      return;
    }
    if (rcaDone.ans === "programada" || rcaDone.ans === "naosai") {
      plDoneText(box, rcaDone);
      const u = document.createElement("button");
      u.type = "button"; u.className = "link"; u.textContent = "Desfazer";
      u.addEventListener("click", rcaUndo);
      box.appendChild(u);
      return;
    }
    if (rcaDone.ans === "voltei" && p && p.id) t.textContent = `Ok, marquei a volta às ${hhmm(p.to)}. O trecho "${pNameTxt(p.name)}" foi de ${zoomRangeText(p.from, p.to)}.`;
    else if (rcaDone.ans === "aindafora") t.textContent = "Ok, o trecho continua aberto. Quando voltar, use \"Voltei\".";
    else if (rcaDone.ans === "fora" && p && p.id) {
      const c = document.createElement("span");
      c.className = "chip off"; c.textContent = "fora de casa";
      t.append(c, ` Marcado: ${pNameTxt(p.name)} · RC102 com você, GMC-800 em casa · ${hhmm(p.from)}–${hhmm(p.to)}`);
    } else t.textContent = rcaDone.ans === "casa" ? "Ok, ficou como em casa." : "Ok, ficou sem marcar.";
    const u = document.createElement("button");
    u.type = "button"; u.className = "link"; u.textContent = "Desfazer";
    u.addEventListener("click", rcaUndo);
    box.append(t, u);
    return;
  }
  const vis = list.filter((x) => !awLater.has(x.id)); // "Responder depois" some do Agora
  if (!vis.length) { box.hidden = true; box.className = "card rca"; return; }
  box.hidden = false;
  box.className = "card rca";
  const q = vis[vis.length - 1]; // a mais recente primeiro
  // 2.84.0 (revisão da interface): fechada, a pergunta é uma linha só
  const kindRc = !q.kind || q.kind === "rc";
  if ((kindRc || q.kind === "gmc" || q.kind === "ambos") && !rcaOpen.has(q.id) && rcaEditing !== q.id) { rcaCompact(box, q, list); return; }
  if (q.kind === "gmc" || q.kind === "ambos") { renderAwayAsk(box, q, list); return; }
  if (q.kind === "volta") { renderRcBack(box, q, list); return; }
  if (q.kind === "programada") { renderPlannedAsk(box, q, list); return; }
  const head = document.createElement("div");
  head.className = "rca-head";
  head.innerHTML = '<span class="dot rc" aria-hidden="true"></span>';
  const h = document.createElement("h2");
  h.textContent = `O RC102 ficou ${hm(rcaMin(q))} sem conexão`;
  const tag = document.createElement("span");
  tag.className = "chip off"; tag.textContent = "a responder";
  head.append(h, tag);
  const sub = document.createElement("div");
  sub.className = "mono small muted rca-sub";
  sub.textContent = `${zoomRangeText(q.from, q.to)} · ` + (q.memMinutes ? `a memória do aparelho trouxe ${hm(q.memMinutes)}` : "a memória do aparelho não trouxe leituras");
  box.append(head, sub);

  if (q.memMinutes) {
    const g = document.createElement("div");
    g.className = "rca-tiles";
    g.append(
      rcaTile("média do RC102", rcaUSv(q.avgUSv), rateLbl()),
      rcaTile("máximo", rcaUSv(q.maxUSv), q.maxAt ? `às ${hhmm(q.maxAt)}` : ""),
      rcaTile("dose no trecho", doseNum(q.doseUSv), doseLbl()),
      rcaTile("GMC-800 em casa, mesmo horário", rcaUSv(q.gmcUSv), rateLbl(), "gmc"));
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
  sub.textContent = `"${pNameTxt(q.name || "Saí de casa")}" está aberto desde ${whenLabel(q.since || q.from)}. O RC102 voltou a conectar no PC às ${hhmm(q.to)}, depois de ${hm(rcaMin(q))} sem conexão.`;
  box.append(head, sub);
  if (q.memMinutes) {
    const g = document.createElement("div");
    g.className = "rca-tiles";
    g.append(
      rcaTile("média do RC102", rcaUSv(q.avgUSv), rateLbl()),
      rcaTile("máximo", rcaUSv(q.maxUSv), q.maxAt ? `às ${hhmm(q.maxAt)}` : ""),
      rcaTile("dose sem conexão", doseNum(q.doseUSv), doseLbl()),
      rcaTile("GMC-800 em casa, mesmo horário", rcaUSv(q.gmcUSv), rateLbl(), "gmc"));
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
    what.textContent = q.kind === "gmc" || q.kind === "ambos" ? "Você estava fora de casa?"
      : q.kind === "volta" ? `RC102 voltou a conectar às ${hhmm(q.to)}: voltou para casa?`
      : q.kind === "programada" ? `Saída programada "${plNameTxt(q.name)}": você saiu mesmo?`
      : q.memMinutes ? `A memória do aparelho trouxe ${hm(q.memMinutes)}.` : "Sem leituras da memória do aparelho.";
    const tt = document.createElement("b");
    tt.textContent = q.kind === "gmc" ? "GMC-800 sem conexão" : q.kind === "ambos" ? "Os dois sem conexão"
      : q.kind === "volta" ? "RC102 voltou" : q.kind === "programada" ? "Saída programada" : "RC102 sem conexão";
    t.append(tt, " ", c, when, what);
    const acts = document.createElement("div");
    acts.className = "acts";
    const go = document.createElement("button");
    go.type = "button"; go.className = "btn small"; go.textContent = "Responder";
    go.addEventListener("click", () => {
      // a escolhida vai para o fim: é a que o cartão do Agora mostra
      state.rcAway = (state.rcAway || []).filter((x) => x.id !== q.id).concat(q);
      awLater.delete(q.id);
      rcaOpen.add(q.id);
      showView("agora");
      renderRcAway();
    });
    acts.appendChild(go);
    li.append(t, acts);
    ul.appendChild(li);
  }
}

// ---------- "Você estava fora de casa?" (2.83.0) ----------
//
// Spec ux/historico-automatico. O histórico do GMC-800 (baixado sozinho ao
// conectar ou pelo botão) trouxe um trecho que o app não tinha visto: "gmc"
// quando só ele ficou sem conexão, "ambos" quando o RC102 também. A pergunta
// aparece sempre; a dica pela conta RC102 ÷ GMC-800 só destaca um botão.

function awSub(q) {
  const why = q.kind === "ambos" ? "o GMC-800 e o RC102 ficaram sem conexão"
    : q.rcHome ? "o GMC-800 ficou sem conexão, o RC102 estava no PC"
    : "o GMC-800 ficou sem conexão com o app";
  return `${zoomRangeText(q.from, q.to)} · ${why}`;
}

function awHintText(q) {
  if (q.hint === "casa") return [q.kind === "ambos" ? "Parece que os dois ficaram em casa." : "Parece que o GMC-800 ficou em casa.", `A conta RC102 ÷ GMC-800 ficou perto da normal (${fmt(q.ratio, 0)} contra ${fmt(q.normalRatio, 0)}).`];
  if (q.hint === "fora") return ["Parece que algum aparelho saiu de casa.", `A conta RC102 ÷ GMC-800 mudou: ${fmt(q.ratio, 0)} contra ${fmt(q.normalRatio, 0)} em casa.`];
  return null;
}

async function awAnswer(q, ans, from, to, name, gmc, rc) {
  try {
    const p = await api.AnswerAway(q.id, ans, from || 0, to || 0, name || "", !!gmc, !!rc);
    rcaEditing = 0;
    clearTimeout(rcaDoneTimer);
    rcaDone = { ask: { id: q.id, from: q.from, to: q.to, kind: q.kind, rcHome: !!q.rcHome }, ans: ans === "sai" ? "awsai" : "awcasa", period: p };
    rcaDoneTimer = setTimeout(() => { rcaDone = null; renderRcAway(); }, 10000);
    await refreshRcAway();
  } catch (e) { toast(errMsg(e), true); }
}

function awDoneText(box, d) {
  const t = document.createElement("span");
  const p = d.period;
  if (d.ans === "awsai" && p && p.id) {
    const c = document.createElement("span");
    c.className = "chip off"; c.textContent = "fora de casa";
    t.append(c, ` Marcado: ${pNameTxt(p.name)} · ${plWhere(p.gmc === "comigo", p.radiacode === "comigo")} · ${hhmm(p.from)}–${hhmm(p.to)}`);
  } else t.textContent = "Ok, ficou como em casa.";
  box.appendChild(t);
}

function renderAwayAsk(box, q, list) {
  const head = document.createElement("div");
  head.className = "rca-head";
  head.innerHTML = '<span class="dot gmc" aria-hidden="true"></span>';
  const h = document.createElement("h2");
  h.textContent = "Você estava fora de casa?";
  const tag = document.createElement("span");
  tag.className = "chip off"; tag.textContent = "a responder";
  head.append(h, tag);
  const sub = document.createElement("div");
  sub.className = "mono small muted rca-sub";
  sub.textContent = awSub(q);
  box.append(head, sub);
  if (q.gmcMinutes) {
    const g = document.createElement("div");
    g.className = "rca-tiles";
    g.append(
      rcaTile("média do GMC-800", rcaUSv(q.gmcUSv), rateLbl(), "gmc"),
      rcaTile("máximo", rcaUSv(q.gmcMaxUSv), q.gmcMaxAt ? `às ${hhmm(q.gmcMaxAt)}` : ""),
      rcaTile("dose no trecho", doseNum(q.gmcDoseUSv), doseLbl()),
      q.kind === "ambos" && q.memMinutes ? rcaTile("média do RC102", rcaUSv(q.avgUSv), rateLbl()) : rcaTile("alertas", String(q.alerts || 0), ""));
    box.appendChild(g);
  }
  const ht = awHintText(q);
  if (ht) {
    const hint = document.createElement("div");
    hint.className = "rca-hint small";
    const b = document.createElement("strong");
    b.textContent = ht[0];
    const hs = document.createElement("span");
    hs.className = "muted"; hs.textContent = " " + ht[1];
    hint.append(b, hs);
    box.appendChild(hint);
  }
  if (rcaEditing === q.id) { box.appendChild(awForm(q)); return; }
  const ask = document.createElement("div");
  ask.className = "rca-ask";
  const row = document.createElement("div");
  row.className = "row-btns";
  const btn = (label, cls, fn) => {
    const b = document.createElement("button");
    b.type = "button"; b.className = cls; b.textContent = label;
    b.addEventListener("click", fn);
    row.appendChild(b);
  };
  btn("Estava em casa", "btn" + (q.hint === "casa" ? " sug" : ""), () => awAnswer(q, "casa"));
  btn("Saí de casa…", "btn rca-out" + (q.hint === "fora" ? " sug" : ""), () => { rcaEditing = q.id; awFocus = "name"; renderRcAway(); });
  btn("Ajustar horário", "link", () => { rcaEditing = q.id; awFocus = "from"; renderRcAway(); });
  btn("Responder depois", "link", () => { awLater.add(q.id); renderRcAway(); });
  const note = document.createElement("span");
  note.className = "small muted";
  note.textContent = "\"Saí de casa\": as medições do aparelho que foi com você saem do fundo da casa e continuam na sua dose.";
  ask.append(row, note);
  if (list.length > 1) {
    const more = document.createElement("span");
    more.className = "small muted";
    more.textContent = list.length === 2 ? "Mais 1 trecho a responder em Fora de casa." : `Mais ${list.length - 1} trechos a responder em Fora de casa.`;
    ask.appendChild(more);
  }
  box.appendChild(ask);
}

// o mesmo formulário da saída programada: horário, nome e quem foi
function awForm(q) {
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
  const to = Object.assign(document.createElement("input"), { type: "datetime-local", value: toLocalInput(q.to) });
  const name = Object.assign(document.createElement("input"), { type: "text", maxLength: 60, placeholder: "Fora de casa" });
  g.append(field("Saí em", from), field("Voltei em", to), field("Para onde (opcional)", name));
  const devs = document.createElement("div");
  devs.className = "row-btns";
  const pick = {};
  for (const [k, label, on] of [["rc", "RC102 foi comigo", q.kind === "ambos"], ["gmc", "GMC-800 foi comigo", true]]) {
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
    awAnswer(q, "sai", a, b, name.value, pick.gmc.checked, pick.rc.checked);
  });
  back.addEventListener("click", () => { rcaEditing = 0; renderRcAway(); });
  btns.append(save, back);
  const note = document.createElement("span");
  note.className = "small muted";
  note.textContent = "O tempo sem conexão fora desse horário fica como em casa (o normal).";
  f.append(g, devs, err, btns, note);
  setTimeout(() => (awFocus === "name" ? name : from).focus(), 0);
  return f;
}
