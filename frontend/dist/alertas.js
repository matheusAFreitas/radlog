// Página Alertas: títulos e textos dos alertas, a lista (com os episódios
// que se encostam juntos numa linha) e o gráfico detalhado de cada alerta.
// Saiu do app.js na 2.20, sem mudar nada na tela.
//
// Usa os globais do app.js (state, api, $, fmt, hhmm...). Este arquivo é
// carregado antes do app.js e nada aqui roda na carga: só funções e
// constantes que as funções usam depois. O estado da página (state.alertSel,
// state.alertFilter, state.activeAlerts) é criado no app.js.

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

// alertas do RadiaCode: valores em µSv/h
const isRcEv = (e) => !!e && e.device === "radiacode";
const usvVal = (v) => `${fmt(v, usvDigits(v))} µSv/h`;
const evVal = (e, v) => (isRcEv(e) ? usvVal(v) : `${fmt(v, 0)} CPM`);
const evSig = (e, v) => (isRcEv(e) ? "" : sigTxt(v));

function alertTitle(e) {
  const demo = e.demo || /^\[demo\] /.test(e.text) ? " (demo)" : "";
  if (e.members && isRcEv(e)) return `${lvlName(e)} no RadiaCode: pico de ${evVal(e, e.peak)}${demo}`;
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
  if (e.peak && (e.retro || e.peak > (e.cpm || 0))) parts.push(`pico ${evVal(e, e.peak)}${evSig(e, e.peak)} às ${(e.retro ? hhmm : hhmmss)(e.peakAt || e.time)}`);
  if (e.breaks && e.breaks.length) parts.push(e.breaks.length === 1 ? `sem leitura por ${fmtDur(e.breaks[0].to - e.breaks[0].from)} no meio` : `${e.breaks.length} trechos sem leitura no meio`);
  if (e.endReason && !e.active) parts.push(`terminou sem leitura (${e.endReason})`);
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
    // desconectou com o alerta aberto e voltou alto em até 15 min: mesmo episódio (2.5)
    const gap = cur && cur.endReason && cur.endReason !== "app fechado" ? 15 * 60e3 : 60e3;
    if (cur && !!e.demo === cur.demo && !!e.retro === cur.retro && (e.device || "") === cur.device && e.time <= cur.until + gap) {
      cur.members.push(e);
      if (cur.endReason && e.time > cur.until + 60e3) cur.breaks.push({ from: cur.until, to: e.time, reason: cur.endReason });
      if (end >= cur.until) cur.endReason = e.endReason || "";
      cur.until = Math.max(cur.until, end);
      continue;
    }
    cur = { members: [e], until: end, demo: !!e.demo, retro: !!e.retro, device: e.device || "", endReason: e.endReason || "", breaks: [] };
    eps.push(cur);
  }
  return eps.map((g) => {
    const ms = g.members;
    if (ms.length === 1) return ms[0];
    const extra = { breaks: g.breaks, endReason: g.endReason };
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
    }, extra);
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
  if (isRcEv(e) && e.members) {
    const tag = document.createElement("span");
    tag.className = "tag-retro tag-rc";
    tag.textContent = "RadiaCode";
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


function renderActive() {
  const ul = $("activeAlerts");
  const list = state.activeAlerts || [];
  renderATile();
  ul.textContent = "";
  if (!list.length) {
    ul.innerHTML = '<li class="none"><span class="dot-ok" aria-hidden="true"></span>Nenhum alerta agora.</li>';
    return;
  }
  const now = Date.now();
  // num episódio os níveis de baixo vêm junto: mostra só o mais alto de cada
  // fonte (real/demo), com a hora em que os de baixo começaram
  const rank = { atencao: 1, alarme: 2, perigo: 3 };
  const src = (x) => (x.demo ? "demo" : isRcEv(x) ? "rc" : "gmc");
  const top = (s) => Math.max(0, ...list.filter((x) => src(x) === s).map((x) => rank[x.level] || 1));
  const shown = list.filter((e) => (rank[e.level] || 1) === top(src(e)));
  for (const e of shown) {
    const li = document.createElement("li");
    li.className = "on " + lvlCls(e);
    const demo = e.demo ? " (demo)" : "";
    const rc = isRcEv(e), rl = (state.rc.st && state.rc.st.limits) || {};
    const lim = rc ? ({ atencao: rl.attn, alarme: rl.alarm, perigo: rl.danger }[lvlCls(e)] || null)
      : e.level === "alarme" ? (e.text.match(/limite (\d+)/) || [])[1] : e.level === "perigo" ? String(dangerCPM()) : null;
    li.innerHTML = `<span class="a-title"><span class="pulse" aria-hidden="true"></span></span><span class="a-meta"></span>`;
    li.querySelector(".a-title").append(`${lvlName(e)} agora${rc ? " no RadiaCode" : ""}${demo}`);
    li.querySelector(".a-meta").textContent =
      `desde ${hhmm(e.time)} (${fmtDur(now - e.time)}) · pico ${evVal(e, e.peak)}${evSig(e, e.peak)}` +
      (lim ? ` · limite ${rc ? usvVal(+lim) : fmt(+lim, 0)}` : "");
    for (const lower of list.filter((x) => src(x) === src(e) && (rank[x.level] || 1) < (rank[e.level] || 1)).sort((a, b) => a.time - b.time)) {
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
  const dv = state.alertDev || "";
  document.querySelectorAll("#alertDevFilter button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.adev === dv)));
  const list = groupEpisodes(all).filter((e) => (!f || (f === "retro" ? e.retro : f === "live" ? !e.retro : lvlCls(e) !== "atencao"))
    && (!dv || (dv === "radiacode") === isRcEv(e)));
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
  const rc = d.device === "radiacode";
  const val = (v) => (rc ? usvVal(v) : `${fmt(v, 0)} CPM`);
  $("adTitle").textContent = `${lvlName(e)}${rc ? " no RadiaCode" : ""} · ${whenLabel(e.time)}` + (e.active ? " · ativo agora" : "");
  const sub = [];
  sub.push(e.active ? `desde ${hhmm(e.time)} · ${fmtDur(d.end - e.time)}` : `${hhmmss(e.time)}–${hhmmss(d.end)} · durou ${fmtDur(d.end - e.time)}`);
  if (e.peak) sub.push(`pico ${val(e.peak)} às ${hhmmss(e.peakAt || e.time)}` + (!rc && bgNow() ? ` (${fmtSig(sigmaOf(e.peak, bgNow()))} acima do fundo de ${fmt(bgNow(), 0)})` : ""));
  sub.push(`limite ${val(d.limit)}`);
  const ep = d.episode;
  if (ep && ep.n) sub.push(`mín ${val(ep.min)} · média ${val(ep.mean)} (${fmt(ep.n, 0)} ${ep.src === "segundos" ? "segundos" : "minutos"})`);
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
  for (let i = 0; i < 4; i++) { const v = top - (top / 3) * i; ys[i].textContent = rc ? fmt(v, v === 0 ? 0 : usvDigits(top)) : fmt(v, 0); }
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
      : `<path d="${d1}" fill="none" stroke="${rc ? "#F472B6" : "#6EE7A0"}" stroke-width="2" vector-effect="non-scaling-stroke"/>`;
  }
  if (secs.length > 1) {
    const d2 = secs.map((p, i) => (i && p.t - secs[i - 1].t <= 10e3 ? "L" : "M") + `${x(p.t).toFixed(1)} ${y(p.cpm).toFixed(1)}`).join("");
    out += `<path d="${d2}" fill="none" stroke="${rc ? "#F472B6" : "#6EE7A0"}" stroke-width="1.6" stroke-linejoin="round" vector-effect="non-scaling-stroke"/>`;
  }
  svg.innerHTML = out;
  // pico (bolinha em HTML para não esticar)
  if (e.peak && e.peakAt) {
    const pk = $("adPeak");
    pk.hidden = false;
    pk.style.left = (x(e.peakAt) / W) * 100 + "%";
    pk.style.top = (y(e.peak) / H) * 100 + "%";
    pk.style.background = color;
    pk.title = `pico ${val(e.peak)} às ${hhmmss(e.peakAt)}`;
  }
  const xl = t1 - t0 <= 20 * 60e3 ? hhmmss : hhmm;
  const xs = $("adX").children;
  xs[0].textContent = xl(t0); xs[1].textContent = xl(t0 + (t1 - t0) / 2); xs[2].textContent = xl(t1);
  $("adLegend").innerHTML =
    (rc ? (secs.length ? '<span class="key"><i style="border-color:#F472B6"></i>leitura ao vivo do RadiaCode (µSv/h)</span><span class="key"><i style="border-color:#A3ADB4"></i>valor do minuto</span>'
      : '<span class="key"><i style="border-color:#F472B6"></i>valor do minuto do RadiaCode (µSv/h)</span>')
      : secs.length ? '<span class="key"><i style="border-color:#6EE7A0"></i>leitura a cada segundo</span><span class="key"><i style="border-color:#A3ADB4"></i>valor do minuto (gráfico principal)</span>'
      : '<span class="key"><i style="border-color:#6EE7A0"></i>valor do minuto (gráfico principal)</span>') +
    `<span class="key"><i style="border-color:${color};border-top-style:dashed"></i>limite</span>` +
    `<span class="key"><b class="sw" style="background:${color}"></b>alerta</span>`;
  const note = $("adNote");
  const notes = [];
  if (!(secs.length && d.detailed)) notes.push(e.retro ? "Alerta achado no histórico do contador: só há o valor de cada minuto."
    : !secs.length && rc ? "Só há o valor de cada minuto do RadiaCode."
    : !secs.length ? "Só há o valor de cada minuto. O detalhe por segundo é guardado nos alertas ao vivo a partir da versão 0.22.0."
    : "O detalhe por segundo não cobre o alerta inteiro (o app foi fechado ou reaberto no meio).");
  if (ep && ep.alerts > 1) notes.push(`Faz parte de um episódio de ${ep.alerts} alertas, ${hhmm(ep.from)}–${hhmm(ep.to)} (nível mais alto: ${({ perigo: "perigo", alarme: "alarme" })[ep.level] || "atenção"}); o mínimo e a média são do episódio inteiro.`);
  for (const b of (ep && ep.breaks) || []) notes.push(`Sem leitura de ${hhmm(b.from)} a ${hhmm(b.to)} (${b.reason}). Ao voltar, a leitura ainda estava alta: o alerta seguiu no mesmo episódio. O tempo sem leitura não entra no mínimo nem na média.`);
  if (ep && ep.endReason && !e.active) notes.push(`O episódio terminou porque parou de chegar leitura (${ep.endReason}) às ${hhmm(ep.to)}, não porque a contagem voltou ao normal. Se baixar o histórico do contador, os minutos depois disso aparecem no gráfico.`);
  note.hidden = !notes.length;
  note.textContent = notes.join(" ");
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
    const v = d.device === "radiacode" ? usvVal(p.cpm) : `${fmt(p.cpm, 0)} CPM`;
    tip.textContent = p.m != null ? `${hhmm(p.m)} · ${v} (minuto)` : `${hhmmss(p.t)} · ${v}`;
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
  state.lastAlert = recent[0] || null;
  renderATile();
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
  document.querySelectorAll("#alertDevFilter button").forEach((b) => b.addEventListener("click", async () => {
    state.alertDev = b.dataset.adev;
    renderAlertHistory(await api.GetEvents("alerta"));
  }));
}

async function refreshEventsList() { renderList($("eventsList"), await api.GetEvents("evento"), 1000, "Nada por enquanto.", false); }
async function refreshEvents() {
  await refreshAlerts();
  if ($("eventsDlg").open) await refreshEventsList();
}
function openEvents() { refreshEventsList(); showEvTab(state.evTab || "log"); $("eventsDlg").showModal(); }
