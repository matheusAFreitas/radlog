"use strict";

// ---------- uso do app (2.72) ----------
//
// Spec ux/uso-do-app.html. Conta, só neste computador, quantas vezes cada
// página, aba, botão, "Como funciona" e seletor foi aberto, e o tempo
// olhando cada página (só com a janela em primeiro plano e mexendo no mouse
// ou teclado nos últimos 2 min). Junta aqui e manda ao Go a cada minuto
// (RecordUsage, internal/app/usage.go), que guarda só totais por dia.
// Desligado por padrão: liga em Configurações › Dados › Uso do app.
//
// Chaves: "p:pagina", "a:pagina/aba", "c:pagina/botao", "h:pagina/cartao",
// "s:pagina/seletor=valor" e "c:busca/ctrlk" (Ctrl+K).

const USO = { buf: {}, input: 0, days: 30, met: "s", page: "", info: null, info30: null };
const USO_IDLE_MS = 120e3;

function usoOn() { return !!(typeof state !== "undefined" && state.settings && state.settings.usageOn); }

function usoAdd(k, n, s) {
  if (!usoOn()) return;
  const c = USO.buf[k] || (USO.buf[k] = { n: 0, s: 0 });
  c.n += n || 0;
  c.s += s || 0;
}

// página que abriu (e a aba dela, se tem abas)
function usoView(v) {
  usoAdd("p:" + v, 1);
  if (SUBS[v]) usoAdd("a:" + v + "/" + state.sub[v], 1);
}

// aba que abriu na página que está na tela
function usoSub(pg, sub) { if (state.view === pg) usoAdd("a:" + pg + "/" + sub, 1); }

function usoFlush() {
  const b = USO.buf;
  if (!Object.keys(b).length || !api.RecordUsage) return Promise.resolve();
  USO.buf = {};
  return api.RecordUsage(b).catch(() => {});
}

function usoSegVal(btn) {
  const v = Object.values(btn.dataset)[0];
  if (v) return v;
  return String([...btn.parentElement.children].indexOf(btn));
}

function usoClick(e) {
  if (!usoOn()) return;
  const t = e.target.closest && e.target.closest("button, summary");
  if (!t || t.closest("#usoBox")) return;
  const pg = state.view;
  if (t.tagName === "SUMMARY") {
    const d = t.parentElement;
    if (!d || !d.classList.contains("how") || d.open) return; // só quando abre
    const box = d.id ? d : d.closest("[id]");
    if (box) usoAdd("h:" + pg + "/" + box.id, 1);
    return;
  }
  if (t.closest("#sideNav") || t.dataset.view || t.dataset.sub) return; // contados como página e aba
  const seg = t.closest(".seg");
  if (seg && seg.id) { usoAdd("s:" + pg + "/" + seg.id + "=" + usoSegVal(t), 1); return; }
  if (t.id) usoAdd("c:" + pg + "/" + t.id, 1);
}

function usoInit() {
  const mark = () => { USO.input = Date.now(); };
  for (const ev of ["mousemove", "mousedown", "keydown", "wheel", "touchstart"]) window.addEventListener(ev, mark, { capture: true, passive: true });
  document.addEventListener("click", usoClick, true);
  setInterval(() => {
    if (!usoOn() || document.visibilityState !== "visible" || !document.hasFocus() || Date.now() - USO.input > USO_IDLE_MS) return;
    const v = state.view;
    usoAdd("p:" + v, 0, 1);
    if (SUBS[v]) usoAdd("a:" + v + "/" + state.sub[v], 0, 1);
  }, 1000);
  setInterval(usoFlush, 60e3);
  window.addEventListener("beforeunload", usoFlush);
  usoView(state.view);
  $("chkUsage").addEventListener("change", async (e) => {
    const on = e.target.checked;
    if (!on) await usoFlush();
    await call(api.SetUsageOn, on);
    state.settings.usageOn = on;
    if (on) usoView(state.view);
    refreshUso();
  });
  $("usoDaysSeg").addEventListener("click", (e) => {
    const b = e.target.closest("button[data-d]");
    if (!b) return;
    USO.days = +b.dataset.d;
    refreshUso();
  });
  $("usoMetSeg").addEventListener("click", (e) => {
    const b = e.target.closest("button[data-m]");
    if (!b) return;
    USO.met = b.dataset.m;
    renderUso();
  });
  $("usoPages").addEventListener("click", (e) => {
    const b = e.target.closest("button[data-pg]");
    if (!b) return;
    USO.page = b.dataset.pg;
    renderUso();
  });
  $("btnUsoReset").addEventListener("click", async () => {
    if (!(await askConfirm({ title: "Zerar a contagem de uso?", text: "Apaga tudo o que foi contado até agora neste computador. Se a contagem estiver ligada, ela recomeça do zero.", ok: "Zerar", danger: true }))) return;
    USO.buf = {};
    await call(api.ResetUsage);
    refreshUso();
  });
  $("btnUsoCopy").addEventListener("click", async () => {
    try { await navigator.clipboard.writeText(usoSummaryText()); toast(tr("Resumo copiado")); }
    catch (e) { toast(tr("Não consegui copiar"), true); }
  });
}

async function refreshUso() {
  if (!api.GetUsage) { $("usoBox").hidden = true; return; }
  await usoFlush();
  try {
    USO.info = await api.GetUsage(USO.days);
    USO.info30 = USO.days === 30 ? USO.info : await api.GetUsage(30);
  } catch (e) { return; }
  renderUso();
}

// ---------- nomes ----------

const usoTxt = (el) => (el ? el.textContent.replace(/\s+/g, " ").trim() : "");

function usoPageName(v) {
  const b = document.querySelector('#sideNav [data-view="' + v + '"] span') || document.querySelector('[data-view="' + v + '"]');
  return usoTxt(b) || v;
}

function usoTabName(pg, sub) {
  const page = $(PAGES[pg]);
  return usoTxt(page && page.querySelector('.subtabs [data-sub="' + sub + '"]')) || sub;
}

function usoKeyName(k) {
  const kind = k[0];
  const rest = k.slice(2);
  const pg = rest.split("/")[0];
  const id = rest.slice(pg.length + 1);
  if (kind === "c" && k === "c:busca/ctrlk") return tr("Busca (Ctrl+K)");
  if (kind === "c") {
    const el = document.getElementById(id);
    return (usoTxt(el) || (el && (el.getAttribute("aria-label") || el.title)) || id).slice(0, 48);
  }
  if (kind === "h") {
    const el = document.getElementById(id);
    let title = "";
    if (el) {
      const box = el.tagName === "DETAILS" ? (el.parentElement.closest("fieldset, section, article, [id]") || el) : el;
      title = usoTxt(box.querySelector("legend, h2, h3, h4, .card-title")).slice(0, 40);
    }
    return title ? tr(`Como funciona · ${title}`) : tr("Como funciona");
  }
  if (kind === "s") {
    const [sid, val] = id.split("=");
    const seg = document.getElementById(sid);
    let b = null;
    if (seg) {
      b = [...seg.querySelectorAll("button")].find((x) => usoSegVal(x) === val);
    }
    return usoTxt(b) || val;
  }
  return k;
}

// ---------- números ----------

function usoDur(s) {
  if (!s) return "0";
  if (s < 60) return "<1 min";
  if (s < 3600) return Math.round(s / 60) + " min";
  const h = s / 3600;
  return (h < 10 ? (Math.round(h * 10) / 10).toLocaleString(LOCALE) : String(Math.round(h))) + " h";
}

const usoTimes = (n) => tr(n === 1 ? "1 vez" : `${n} vezes`);
const usoClicks = (n) => tr(n === 1 ? "1 clique" : `${n} cliques`);
const usoOpens = (n) => tr(n === 1 ? "1 abertura" : `${n} aberturas`);

function usoGet(info, k) { return (info && info.totals && info.totals[k]) || { n: 0, s: 0 }; }

function usoVal(c) { return USO.met === "s" ? c.s || 0 : c.n || 0; }

function usoHeat(v, max) {
  if (!v || !max) return 0;
  return Math.min(4, Math.max(1, Math.ceil(4 * Math.sqrt(v / max))));
}

function usoPages(info) {
  return Object.keys(PAGES).map((k) => ({ k, name: usoPageName(k), c: usoGet(info, "p:" + k) }))
    .sort((a, b) => usoVal(b.c) - usoVal(a.c) || (b.c.n || 0) - (a.c.n || 0));
}

function usoNever() {
  const i = USO.info30;
  const out = [];
  for (const k of Object.keys(PAGES)) if (!usoGet(i, "p:" + k).n) out.push(usoPageName(k));
  for (const [pg, subs] of Object.entries(SUBS)) {
    if (!usoGet(i, "p:" + pg).n) continue; // a página inteira já está na lista
    for (const s of subs) if (!usoGet(i, "a:" + pg + "/" + s).n) out.push(usoPageName(pg) + " › " + usoTabName(pg, s));
  }
  if (!usoGet(i, "c:busca/ctrlk").n) out.push(tr("Busca (Ctrl+K)"));
  return out;
}

// ---------- tela ----------

function usoLi(name, val, cls) {
  const li = document.createElement("li");
  if (cls) li.className = cls;
  const a = document.createElement("span");
  a.textContent = name;
  const b = document.createElement("span");
  b.className = "mono";
  b.textContent = val;
  li.append(a, b);
  return li;
}

function renderUso() {
  const info = USO.info;
  if (!info) return;
  const has = Object.keys(info.totals || {}).length > 0 || (USO.info30 && Object.keys(USO.info30.totals || {}).length > 0);
  $("chkUsage").checked = !!info.on;
  $("usoBody").hidden = !info.on && !has;
  document.querySelectorAll("#usoDaysSeg button").forEach((b) => b.setAttribute("aria-pressed", String(+b.dataset.d === USO.days)));
  document.querySelectorAll("#usoMetSeg button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.m === USO.met)));

  const pages = usoPages(info);
  if (!PAGES[USO.page]) USO.page = pages[0].k;
  const max = Math.max(...pages.map((p) => usoVal(p.c)));
  const ul = $("usoPages");
  ul.textContent = "";
  for (const p of pages) {
    const li = document.createElement("li");
    const lvl = usoHeat(usoVal(p.c), max);
    if (lvl) li.className = "uso-h" + lvl;
    if (p.k === USO.page) li.setAttribute("aria-current", "true");
    const b = document.createElement("button");
    b.type = "button";
    b.className = "uso-row";
    b.dataset.pg = p.k;
    const a = document.createElement("span");
    a.textContent = p.name;
    const v = document.createElement("span");
    v.className = "mono";
    v.textContent = USO.met === "s" ? usoDur(p.c.s) : String(p.c.n || 0);
    b.append(a, v);
    li.append(b);
    ul.append(li);
  }

  // abas da página escolhida
  const pg = USO.page;
  const tabs = $("usoTabs");
  tabs.textContent = "";
  $("usoTabsH").hidden = tabs.hidden = !SUBS[pg];
  if (SUBS[pg]) {
    $("usoTabsH").textContent = tr(`Abas de ${usoPageName(pg)}`);
    const list = SUBS[pg].map((s) => ({ name: usoTabName(pg, s), c: usoGet(info, "a:" + pg + "/" + s) }))
      .sort((a, b) => usoVal(b.c) - usoVal(a.c));
    const tmax = Math.max(...list.map((x) => usoVal(x.c)));
    for (const x of list) {
      const val = x.c.n ? usoDur(x.c.s) + " · " + usoTimes(x.c.n) : tr("0 · nunca");
      const lvl = usoHeat(usoVal(x.c), tmax);
      tabs.append(usoLi(x.name, val, lvl ? "uso-h" + lvl : ""));
    }
  }

  // botões, "Como funciona" e seletores da página
  const pre = [":" + pg + "/"];
  const clicks = Object.entries(info.totals || {})
    .filter(([k, c]) => "chs".includes(k[0]) && k.slice(1).startsWith(pre[0]) && c.n)
    .sort((a, b) => b[1].n - a[1].n).slice(0, 8);
  const cl = $("usoClicks");
  cl.textContent = "";
  if (!clicks.length) cl.append(usoLi(tr("Nada contado ainda"), ""));
  for (const [k, c] of clicks) cl.append(usoLi(usoKeyName(k), k[0] === "h" ? usoOpens(c.n) : usoClicks(c.n)));

  const nv = $("usoNever");
  nv.textContent = "";
  const never = usoNever();
  for (const n of never) { const s = document.createElement("span"); s.textContent = n; nv.append(s); }
  if (!never.length) { const s = document.createElement("span"); s.textContent = tr("tudo foi usado"); nv.append(s); }

  const since = info.since ? new Date(info.since).toLocaleDateString(LOCALE) : "—";
  $("usoFoot").textContent = info.on
    ? tr(`Contado só neste computador · desde ${since}`)
    : tr("Contagem desligada · o que já foi contado fica guardado até zerar");
}

// texto curto para colar na conversa
function usoSummaryText() {
  const info = USO.info;
  if (!info) return "";
  const per = USO.days ? `${USO.days} dias` : "tudo";
  const since = info.since ? new Date(info.since).toLocaleDateString(LOCALE) : "—";
  const L = [tr(`Uso do RadLog (${tr(per)}, contando desde ${since})`)];
  L.push(tr("Páginas:"));
  for (const p of usoPages(info)) L.push("- " + p.name + ": " + usoDur(p.c.s) + ", " + usoTimes(p.c.n || 0));
  for (const [pg, subs] of Object.entries(SUBS)) {
    L.push(tr(`Abas de ${usoPageName(pg)}`) + ":");
    for (const s of subs) { const c = usoGet(info, "a:" + pg + "/" + s); L.push("- " + usoTabName(pg, s) + ": " + usoDur(c.s) + ", " + usoTimes(c.n || 0)); }
  }
  const top = Object.entries(info.totals || {}).filter(([k, c]) => "chs".includes(k[0]) && c.n).sort((a, b) => b[1].n - a[1].n).slice(0, 15);
  if (top.length) {
    L.push(tr("Botões mais usados:"));
    for (const [k, c] of top) {
      const pg = k.slice(2).split("/")[0];
      L.push("- " + (PAGES[pg] ? usoPageName(pg) + " › " : "") + usoKeyName(k) + ": " + (k[0] === "h" ? usoOpens(c.n) : usoClicks(c.n)));
    }
  }
  const never = usoNever();
  L.push(tr("Nunca usado em 30 dias:") + " " + (never.length ? never.join(", ") : tr("nada")));
  return L.join("\n");
}
