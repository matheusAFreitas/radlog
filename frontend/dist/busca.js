// ---------- busca rápida (Ctrl+K) e botão do logo (2.48) ----------
//
// Spec ux/busca.html. Ctrl+K (ou "Buscar" no menu) abre uma busca que acha
// ações, telas, ajustes (os cartões das Configurações) e medições salvas;
// "14:30" abre os gráficos de hoje nesse horário. O logo no canto inferior
// direito abre três ações: anotar agora (N), saí de casa / voltei e medir um
// objeto. A lista é traduzida aqui mesmo (tr) e marcada data-no-tr, porque o
// destaque <mark> quebra o texto em pedaços. Carregado antes do app.js.

const QK = { sel: 0, items: [], recent: [], last: null };

const QK_IC = {
  note: '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M10.5 2.5l3 3L6 13H3v-3z"/></svg>',
  out: '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M9 3H3v10h6M7 8h7M11.5 5.5 14 8l-2.5 2.5"/></svg>',
  back: '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3 7l5-4 5 4v6H3z"/></svg>',
  obj: '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true"><circle cx="8" cy="8" r="5.5"/><circle cx="8" cy="8" r="1.8" fill="currentColor"/></svg>',
  page: '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true"><rect x="2.5" y="2.5" width="11" height="11" rx="2"/><path d="M2.5 6h11"/></svg>',
  set: '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" aria-hidden="true"><path d="M3 4h10M3 8h10M3 12h10"/><circle cx="6" cy="4" r="1.4" fill="var(--surface)"/><circle cx="10" cy="8" r="1.4" fill="var(--surface)"/><circle cx="5" cy="12" r="1.4" fill="var(--surface)"/></svg>',
  pdf: '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round" aria-hidden="true"><path d="M4 1.5h5.5L12.5 4.5V14.5H4z"/><path d="M9.5 1.5v3h3"/></svg>',
  bell: '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round" aria-hidden="true"><path d="M4 11V7a4 4 0 0 1 8 0v4l1 1.5H3z"/><path d="M6.5 14h3"/></svg>',
  mute: '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3 6.2h2.2L8 4v8L5.2 9.8H3Z"/><path d="M10.5 6l3 4M13.5 6l-3 4"/></svg>',
  clock: '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" aria-hidden="true"><circle cx="8" cy="8" r="6"/><path d="M8 4.5V8l2.5 1.5"/></svg>',
};

// apelidos dos ajustes (pelo título do cartão, sem acento)
const QK_ALIAS = {
  som: "alarme sirene volume barulho", relatorios: "pdf", sincronizacao: "outro pc notebook rede firewall",
  idioma: "ingles english lingua", backup: "exportar importar", atualizacoes: "versao update", alarme: "limite aviso",
};

const qkNorm = (s) => String(s || "").normalize("NFD").replace(/[̀-ͯ]/g, "").toLowerCase();
const qkTr = (s) => (typeof tr === "function" ? tr(s) : s);
const qkHM = () => new Date().toLocaleTimeString(LOCALE, { hour: "2-digit", minute: "2-digit" });

// fora de casa agora? (o "Voltei" aparece no lugar do "Saí de casa")
function qkAway() {
  const w = state.where;
  return !!(w && (w.notHome || w.gmc === "comigo" || (w.gmc === "casa" && w.radiacode === "casa" && w.open)));
}

function qkAlarmOn() {
  const b = $("alarmBar");
  return !!(b && !b.hidden);
}

function qkMeasuring() {
  const st = state.measureSt;
  return !!(st && (st.phase === "fundo" || st.phase === "fundo-pronto" || st.phase === "objeto"));
}

function qkActions() {
  const a = [
    { g: "Ações", ic: QK_IC.note, t: "Anotar agora", s: "nota presa a este minuto", k: "N", kw: "nota anotacao escrever", run: qkNote },
    qkAway()
      ? { g: "Ações", ic: QK_IC.back, t: "Voltei", s: "encerra o período fora de casa", kw: "voltar casa chegar fora", run: qkBack, c: "away" }
      : { g: "Ações", ic: QK_IC.out, t: "Saí de casa", s: "pergunta quem vai junto", kw: "sair fora rua viagem", run: qkAskAway, c: "away" },
    { g: "Ações", ic: QK_IC.obj, t: "Medir um objeto", s: "abre o Medir objeto no passo 1", kw: "medir objeto amostra", run: qkMeasure, c: "obj" },
    { g: "Ações", ic: QK_IC.pdf, t: "Relatório em PDF do Resumo", s: "do período escolhido no Resumo", kw: "relatorio pdf imprimir", run: qkReport },
    { g: "Ações", ic: QK_IC.bell, t: "Testar alarme (5 s)", s: "Configurações › Alarme", kw: "testar alarme som sirene", run: () => $("btnTestAlarm").click() },
  ];
  if (qkAlarmOn()) a.splice(3, 0, { g: "Ações", ic: QK_IC.mute, t: "Silenciar o alarme", s: "a faixa continua até o nível baixar", kw: "silenciar alarme som parar", run: () => $("btnSilence").click() });
  return a;
}

function qkViews() {
  const out = [];
  document.querySelectorAll("#sideNav [data-view]").forEach((b) => {
    const s = b.querySelector("span");
    out.push([b.dataset.view, (s ? s.textContent : b.textContent).trim()]);
  });
  out.push(["config", "Configurações"]);
  return out;
}

function qkSettings() {
  const out = [];
  if (typeof setLay !== "function") return out;
  for (const f of document.querySelectorAll("#settings .set-sec fieldset")) {
    if (f.parentElement.closest("fieldset")) continue;
    const leg = f.querySelector(":scope > legend");
    const sec = f.closest(".set-sec");
    if (!leg || !sec) continue;
    const L = setLay(sec.dataset.sec), title = leg.textContent.trim();
    out.push({
      g: "Ajustes", ic: QK_IC.set, t: title, s: "Configurações › " + L.name,
      kw: (QK_ALIAS[qkNorm(title)] || "") + " " + L.name,
      run: async () => {
        if (!f.id) f.id = "set-card-" + Math.random().toString(36).slice(2, 8);
        await openSettings();
        showSetFor(f.id, true);
      },
    });
  }
  return out;
}

function qkMeasurements() {
  return [...(state.measurements || [])].reverse().map((m) => ({
    g: "Medições salvas", ic: QK_IC.obj, t: m.name, raw: true,
    s: whenLabel(m.time) + (m.result ? ` · ${m.result.netUSv >= 0 ? "+" : "−"}${fmt(Math.abs(m.result.netUSv), 3)} µSv/h` : ""),
    kw: "medicao objeto " + (m.note || ""),
    run: async () => {
      await showView("medir");
      setMeasureMode("salvas");
      state.mSel = m.id;
      renderMeasureHistory();
      drawMeasureChart();
    },
  }));
}

function qkResults(q) {
  const nq = qkNorm(q.trim());
  if (!nq) {
    const r = qkActions().slice(0, 3);
    const names = Object.fromEntries(qkViews());
    for (const v of QK.recent) if (v !== state.view && names[v]) r.push({ g: "Abertas há pouco", ic: QK_IC.page, t: names[v], run: () => qkGo(v) });
    return r;
  }
  const out = [];
  const m = nq.match(/^([01]?\d|2[0-3])[:h]([0-5]\d)$/);
  if (m) {
    const hm = m[1].padStart(2, "0") + ":" + m[2];
    out.push({ g: "Ir para um horário", ic: QK_IC.clock, t: `Abrir os gráficos em ${hm}`, s: "hoje, centrado nesse minuto", run: () => qkAt(+m[1], +m[2]) });
  }
  const all = [
    ...qkActions(),
    ...qkViews().map(([v, n]) => ({ g: "Telas", ic: QK_IC.page, t: n, kw: v, run: () => qkGo(v) })),
    ...qkSettings(),
    ...qkMeasurements(),
  ];
  const words = nq.split(/\s+/);
  const hits = [];
  for (const it of all) {
    // acha pelo português e pela tradução; quem tem a busca no título vem antes
    const hay = qkNorm([it.t, it.s, it.kw, qkTr(it.t), it.s ? qkTr(it.s) : ""].join(" "));
    if (!words.every((w) => hay.includes(w))) continue;
    const title = " " + qkNorm(it.t + " " + qkTr(it.t));
    it.score = words.every((w) => title.includes(" " + w)) ? 0 : words.every((w) => title.includes(w)) ? 1 : 2;
    hits.push(it);
  }
  // grupos na ordem do melhor resultado de cada um
  const best = {};
  hits.forEach((it) => { best[it.g] = Math.min(best[it.g] ?? 9, it.score); });
  const order = [...new Set(hits.map((it) => it.g))].sort((x, y) => best[x] - best[y]);
  for (const g of order) out.push(...hits.filter((it) => it.g === g).sort((x, y) => x.score - y.score));
  return out.slice(0, 14);
}

function qkMark(text, nq) {
  const span = document.createElement("span");
  const i = nq ? qkNorm(text).indexOf(nq) : -1;
  if (i < 0) { span.textContent = text; return span; }
  const mk = document.createElement("mark");
  mk.textContent = text.slice(i, i + nq.length);
  span.append(text.slice(0, i), mk, text.slice(i + nq.length));
  return span;
}

function qkList() {
  const ul = $("qkList"), q = $("qkInput").value;
  QK.items = qkResults(q);
  QK.sel = Math.min(QK.sel, Math.max(0, QK.items.length - 1));
  ul.textContent = "";
  if (!QK.items.length) {
    const li = document.createElement("li");
    li.className = "qk-none";
    li.textContent = qkTr(`Nada encontrado para "${q.trim()}".`) + " " + qkTr('Tente o nome de uma tela, "nota" ou "alarme".');
    ul.appendChild(li);
    $("qkInput").removeAttribute("aria-activedescendant");
    return;
  }
  const nq = qkNorm(q.trim());
  let last = "";
  QK.items.forEach((it, i) => {
    if (it.g !== last) {
      const g = document.createElement("li");
      g.className = "qk-grp";
      g.setAttribute("role", "presentation");
      g.textContent = qkTr(it.g);
      ul.appendChild(g);
      last = it.g;
    }
    const li = document.createElement("li");
    li.className = "qk-it";
    li.id = "qk" + i;
    li.setAttribute("role", "option");
    li.setAttribute("aria-selected", String(i === QK.sel));
    li.dataset.k = i;
    const ic = document.createElement("span");
    ic.className = "qk-ic";
    ic.innerHTML = it.ic;
    const t = document.createElement("span");
    t.className = "qk-t";
    t.appendChild(qkMark(it.raw ? it.t : qkTr(it.t), nq));
    if (it.s) t.appendChild(Object.assign(document.createElement("small"), { textContent: it.raw ? it.s : qkTr(it.s) }));
    li.append(ic, t);
    if (it.k) li.appendChild(Object.assign(document.createElement("kbd"), { textContent: it.k }));
    else {
      const go = it.g === "Telas" || it.g === "Abertas há pouco" ? "abrir" : it.g === "Ajustes" ? "ajuste" : "";
      if (go) li.appendChild(Object.assign(document.createElement("span"), { className: "qk-go", textContent: qkTr(go) }));
    }
    ul.appendChild(li);
  });
  $("qkInput").setAttribute("aria-activedescendant", "qk" + QK.sel);
  const el = $("qk" + QK.sel);
  if (el) el.scrollIntoView({ block: "nearest" });
}

function qkRun(i) {
  const it = QK.items[i];
  if (!it) return;
  qkClose();
  it.run();
}

function qkOpen(q) {
  qkFab(false);
  qkSheetClose();
  const d = $("qkDlg");
  QK.last = document.activeElement;
  $("qkInput").value = q || "";
  QK.sel = 0;
  if (!d.open) d.showModal();
  $("qkInput").focus();
  qkList();
  // a lista das medições salvas pode não ter sido carregada ainda
  if (!state.measurements && api.GetMeasurements) api.GetMeasurements().then((m) => { state.measurements = m || []; if (d.open) qkList(); }).catch(() => {});
}

function qkClose() {
  const d = $("qkDlg");
  if (d.open) d.close();
}

// ---------- ações ----------

function qkGo(v) {
  if (v === "config") openSettings();
  else showView(v);
}

function qkAt(h, m) {
  const t = new Date();
  t.setHours(h, m, 0, 0);
  const c = Math.min(t.getTime(), Date.now());
  showRange(c - 30 * 6e4, Math.min(Date.now(), c + 30 * 6e4));
}

function qkReport() {
  showView("resumo");
  const [from, to, label] = sumPeriods()[0];
  exportReport(from, to, label, $("btnReport"));
}

async function qkMeasure() {
  await showView("medir");
  setMeasureMode("nova");
  if (!qkMeasuring()) { state.mBusca = false; if (state.measureSt) renderMeasure(state.measureSt); }
}

function qkNote() {
  qkSheetClose();
  openNote({ id: 0, time: Date.now(), text: "" });
}

function qkGoFor() {
  try { return Object.assign({ gmc: false, rc: true }, JSON.parse(localStorage.getItem("gmc.awayGo") || "{}")); } catch (e) { return { gmc: false, rc: true }; }
}

function qkSheetClose() {
  const s = $("qkSheet");
  if (s) s.hidden = true;
}

function qkAskAway() {
  qkFab(false);
  const s = $("qkSheet"), go = qkGoFor();
  s.textContent = "";
  const b = document.createElement("b");
  b.textContent = `Saí de casa às ${qkHM()}`;
  s.appendChild(b);
  s.appendChild(Object.assign(document.createElement("span"), { className: "small muted", textContent: "Quem vai com você?" }));
  for (const [k, name] of [["gmc", "GMC-800"], ["rc", "RC102"]]) {
    const l = document.createElement("label");
    l.className = "qk-go-row";
    const c = Object.assign(document.createElement("input"), { type: "checkbox", checked: go[k] });
    c.dataset.go = k;
    const st = Object.assign(document.createElement("span"), { className: "small muted" });
    const upd = () => { st.textContent = c.checked ? "vai comigo" : "fica em casa"; };
    c.addEventListener("change", upd);
    upd();
    l.append(c, Object.assign(document.createElement("span"), { textContent: name }), st);
    s.appendChild(l);
  }
  s.appendChild(Object.assign(document.createElement("span"), { className: "small muted", textContent: "Já vem marcado como da última vez." }));
  const acts = document.createElement("div");
  acts.className = "qk-acts";
  const no = Object.assign(document.createElement("button"), { type: "button", className: "btn", textContent: "Cancelar" });
  const ok = Object.assign(document.createElement("button"), { type: "button", className: "btn qk-away-ok", textContent: "Saí de casa" });
  no.addEventListener("click", qkSheetClose);
  ok.addEventListener("click", async () => {
    const pick = {};
    s.querySelectorAll("[data-go]").forEach((c) => { pick[c.dataset.go] = c.checked; });
    try { localStorage.setItem("gmc.awayGo", JSON.stringify(pick)); } catch (e) { /* sem armazenamento */ }
    qkSheetClose();
    try {
      state.where = await api.LeaveHome(!!pick.gmc, !!pick.rc);
      const who = [pick.gmc && "GMC-800", pick.rc && "RC102"].filter(Boolean);
      toast(who.length ? `Fora de casa desde ${qkHM()} · ${who.join(" e ")} com você` : `Fora de casa desde ${qkHM()} · nenhum aparelho com você`);
      if (typeof refreshWhere === "function") refreshWhere();
      if (typeof refreshPeriods === "function" && state.view === "fora") refreshPeriods();
    } catch (e) { toast(errMsg(e), true); }
  });
  acts.append(no, ok);
  s.appendChild(acts);
  s.hidden = false;
  ok.focus();
}

async function qkBack() {
  qkSheetClose();
  try {
    state.where = await api.SetWhere("casa", "comigo", "");
    toast(`Bem-vindo de volta · período fora de casa encerrado às ${qkHM()}`);
    if (typeof refreshWhere === "function") refreshWhere();
    if (typeof refreshPeriods === "function" && state.view === "fora") refreshPeriods();
  } catch (e) { toast(errMsg(e), true); }
}

// ---------- botão do logo ----------

function qkFab(open) {
  const f = $("qkFab");
  if (!f) return;
  f.classList.toggle("open", open);
  $("qkFabMain").setAttribute("aria-expanded", String(open));
  if (!open) return;
  qkSheetClose();
  const menu = $("qkFabMenu");
  menu.textContent = "";
  qkActions().slice(0, 3).forEach((it) => {
    const b = document.createElement("button");
    b.type = "button";
    b.append(it.t);
    if (it.k) b.append(" ", Object.assign(document.createElement("kbd"), { textContent: it.k }));
    const ic = document.createElement("span");
    ic.className = "qk-fic" + (it.c ? " " + it.c : "");
    ic.innerHTML = it.ic;
    b.appendChild(ic);
    b.addEventListener("click", () => { qkFab(false); it.run(); });
    menu.appendChild(b);
  });
  const first = menu.querySelector("button");
  if (first) first.focus();
}

// some durante uma medição em andamento (o "Parar" fica no canto) e com o
// alarme tocando (a faixa tem prioridade)
function qkFabSync() {
  const f = $("qkFab");
  if (!f) return;
  const hide = qkMeasuring() || qkAlarmOn();
  if (hide && f.classList.contains("open")) qkFab(false);
  f.hidden = hide;
}

function qkTrack(v) {
  QK.recent = [v, ...QK.recent.filter((x) => x !== v)].slice(0, 4);
}

function wireBusca() {
  $("qkInput").addEventListener("input", () => { QK.sel = 0; qkList(); });
  $("qkInput").addEventListener("keydown", (e) => {
    if (e.key === "ArrowDown") { QK.sel = Math.min(QK.items.length - 1, QK.sel + 1); qkList(); e.preventDefault(); }
    else if (e.key === "ArrowUp") { QK.sel = Math.max(0, QK.sel - 1); qkList(); e.preventDefault(); }
    else if (e.key === "Enter") { e.preventDefault(); qkRun(QK.sel); }
  });
  $("qkList").addEventListener("click", (e) => {
    const li = e.target.closest(".qk-it");
    if (li) qkRun(+li.dataset.k);
  });
  $("qkList").addEventListener("mousemove", (e) => {
    const li = e.target.closest(".qk-it");
    if (li && +li.dataset.k !== QK.sel) { QK.sel = +li.dataset.k; $("qkList").querySelectorAll(".qk-it").forEach((x) => x.setAttribute("aria-selected", String(x === li))); $("qkInput").setAttribute("aria-activedescendant", li.id); }
  });
  $("qkDlg").addEventListener("mousedown", (e) => { if (e.target === $("qkDlg")) qkClose(); });
  $("qkDlg").addEventListener("close", () => { if (QK.last && QK.last.focus) QK.last.focus(); });
  $("btnQuick").addEventListener("click", () => qkOpen(""));
  $("qkFabMain").addEventListener("click", () => qkFab(!$("qkFab").classList.contains("open")));
  document.addEventListener("keydown", (e) => {
    if ((e.ctrlKey || e.metaKey) && !e.altKey && e.key.toLowerCase() === "k") {
      e.preventDefault();
      if ($("qkDlg").open) qkClose(); else qkOpen("");
      return;
    }
    if (e.key === "Escape") {
      if ($("qkFab").classList.contains("open")) { qkFab(false); $("qkFabMain").focus(); }
      qkSheetClose();
      return;
    }
    // N: anotar agora, só fora de campos de texto e sem outro diálogo aberto
    const el = document.activeElement || {};
    const typing = /^(input|textarea|select)$/i.test(el.tagName || "") || el.isContentEditable;
    if (!typing && !e.ctrlKey && !e.metaKey && !e.altKey && e.key.toLowerCase() === "n" && !document.querySelector("dialog[open]")) {
      e.preventDefault();
      qkFab(false);
      qkNote();
    }
  });
  document.addEventListener("click", (e) => {
    if (!e.target.closest("#qkFab")) qkFab(false);
    if (!e.target.closest("#qkSheet, #qkFab")) qkSheetClose();
  });
  setInterval(qkFabSync, 1000);
  qkFabSync();
}
