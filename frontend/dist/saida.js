// Resumo da saída (2.67.0, spec ux/saida.html): quando você volta para casa,
// tudo o que o RC102 viu enquanto você estava fora. Abre num painel grande
// pelo aviso "Saída fechada", pela faixa lilás do Resumo ou em Fora de casa.
// Nunca GPS: o lugar é o nome da saída, editável aqui mesmo.
// Carregado antes do app.js; só define funções.

// saída = trecho em que o RC102 foi junto (é ele que mede a saída)
const isTripP = (p) => (p.radiacode || "comigo") === "comigo";
const tripDefaultName = (p) => !p.name || p.name === "Fora de casa" || p.name === whKind(whOf(p));
const tripDay = (t) => `${WEEKDAY[new Date(t).getDay()]} ${ddmm(t)}`;
const tripTitle = (p) => (tripDefaultName(p) ? `Saída de ${tripDay(p.from)}` : p.name);
const tripDose = (rc) => (rc.devOK ? rc.devDose : rc.dose);
const tripUSv = (v) => fmt(v, usvDigits(v));

// ---------- aviso "Saída fechada" ----------

function tripToastShow(p) {
  const el = document.createElement("div");
  el.className = "toast trip-toast";
  el.innerHTML = '<b></b><span class="small muted"></span><div class="acts"></div>';
  el.children[0].textContent = `Saída fechada: ${hhmm(p.from)} → ${hhmm(p.to)}`;
  const sub = el.children[1];
  const b1 = document.createElement("button");
  b1.type = "button"; b1.className = "btn small away-btn"; b1.textContent = "Ver o resumo da saída";
  const b2 = document.createElement("button");
  b2.type = "button"; b2.className = "btn small"; b2.textContent = "Depois";
  el.children[2].append(b1, b2);
  const close = () => el.remove();
  b1.addEventListener("click", () => { close(); openTrip(p.id); });
  b2.addEventListener("click", close);
  $("toasts").appendChild(el);
  setTimeout(close, 30000);
  api.GetSummary(p.from, p.to).then((s) => {
    const rc = s && s.rc && s.rc.minutes ? s.rc : null;
    if (rc) sub.textContent = `Dose ${fmt(tripDose(rc), 2)} µSv · pico ${tripUSv(rc.max)} µSv/h às ${hhmm(rc.maxAt)}`;
  }).catch(() => {});
}

// chamado quando a lista de trechos muda: uma saída que estava aberta fechou
function tripCheckClosed(prev, list) {
  for (const q of list || []) {
    if (!q.to || !isTripP(q)) continue;
    const old = (prev || []).find((p) => p.id === q.id);
    if (old && old.open) tripToastShow(q);
  }
}

// ---------- faixa lilás no Resumo ----------

let sumTripSeq = 0;
function renderSumTrips(per) {
  const box = $("sumTrips");
  const my = ++sumTripSeq;
  box.textContent = "";
  const list = per ? (state.periods || []).filter((p) => isTripP(p) && pTo(p) > per[0] && p.from < per[1]).sort((a, b) => a.from - b.from) : [];
  box.hidden = !list.length;
  for (const p of list) {
    const row = document.createElement("div");
    row.className = "sum-trip";
    row.innerHTML = `<span class="ic" aria-hidden="true">${WH_ICON_ME}</span><div class="t"><b></b><span class="small muted"></span></div><div class="nums mono"></div>`;
    row.querySelector("b").textContent = tripTitle(p);
    if (!tripDefaultName(p)) row.querySelector("b").dataset.noTr = "";
    const to = pTo(p);
    row.querySelector(".t .muted").textContent = `${hhmm(p.from)} → ${p.open ? "agora" : hhmm(to)} · ${whDur(p.from, Math.min(to, Date.now()))} com o RC102`;
    const btn = document.createElement("button");
    btn.type = "button"; btn.className = "btn small away-btn"; btn.textContent = "Abrir resumo da saída";
    btn.addEventListener("click", () => openTrip(p.id));
    row.appendChild(btn);
    box.appendChild(row);
    periodSummary(p).then((s) => {
      if (my !== sumTripSeq) return;
      const rc = s && s.rc && s.rc.minutes ? s.rc : null;
      const nums = row.querySelector(".nums");
      if (!rc) { nums.innerHTML = '<span class="small muted"></span>'; nums.firstChild.textContent = "sem leitura do RC102"; return; }
      for (const [k, v] of [["dose", `${fmt(tripDose(rc), 2)} µSv`], ["média", tripUSv(rc.avg)], ["pico", tripUSv(rc.max)]]) {
        const d = document.createElement("span");
        d.innerHTML = "<i></i><b></b>";
        d.firstChild.textContent = k; d.lastChild.textContent = v;
        nums.appendChild(d);
      }
    });
  }
}

// ---------- o painel ----------

let tripSeq = 0, tripOpenId = 0;
async function openTrip(id) {
  const dlg = $("tripDlg");
  const my = ++tripSeq;
  tripOpenId = id;
  $("tripBody").innerHTML = '<p class="muted">Calculando…</p>';
  $("tripTitle").textContent = "Resumo da saída";
  $("tripWhen").textContent = "";
  if (!dlg.open) dlg.showModal();
  let s;
  try { s = await api.GetTripSummary(id); } catch (e) { toast(errMsg(e), true); dlg.close(); return; }
  if (my !== tripSeq) return;
  renderTrip(s);
}

function tripCard(cls, title, sub) {
  const c = document.createElement("section");
  c.className = "trip-card " + (cls || "");
  c.innerHTML = '<div class="trip-card-h"><h3></h3><span class="small muted"></span></div>';
  c.querySelector("h3").textContent = title;
  c.querySelector("span").textContent = sub || "";
  return c;
}

function renderTripTitle(s) {
  const p = s.period;
  const h = $("tripTitle");
  h.textContent = "";
  const name = document.createElement("span");
  if (!s.defaultName) name.dataset.noTr = "";
  name.textContent = s.defaultName ? `Saída de ${tripDay(s.from)}` : p.name;
  const ed = document.createElement("button");
  ed.type = "button"; ed.className = "link small-link trip-edit"; ed.textContent = "✎ editar título";
  ed.title = "Onde você foi: o nome da saída em Fora de casa";
  ed.addEventListener("click", () => {
    const inp = document.createElement("input");
    inp.type = "text"; inp.maxLength = 60; inp.value = s.defaultName ? "" : p.name;
    inp.placeholder = "Onde você foi"; inp.setAttribute("aria-label", "Título da saída");
    const save = async () => {
      const v = inp.value.trim();
      try {
        const q = await api.SavePeriod({ ...p, name: v });
        s.period = q; s.defaultName = !v;
      } catch (e) { toast(errMsg(e), true); }
      renderTripTitle(s);
    };
    inp.addEventListener("keydown", (e) => {
      if (e.key === "Enter") { e.preventDefault(); save(); }
      if (e.key === "Escape") { e.preventDefault(); e.stopPropagation(); renderTripTitle(s); }
    });
    inp.addEventListener("blur", save, { once: true });
    h.textContent = "";
    h.appendChild(inp);
    inp.focus();
  });
  h.append(name, " ", ed);
}

function renderTrip(s) {
  renderTripTitle(s);
  const rc = s.rc && s.rc.minutes ? s.rc : null;
  const to = s.to;
  $("tripWhen").textContent = `${tripDay(s.from)} · ${hhmm(s.from)} → ${s.open ? "agora" : hhmm(to)}`;
  const body = $("tripBody");
  body.textContent = "";
  const meta = document.createElement("p");
  meta.className = "small muted trip-meta";
  const parts = [whDur(s.from, to), "RC102"];
  if (rc) parts.push(`${fmt(rc.coverageAll * 100, 0)}% dos minutos`);
  meta.textContent = parts.join(" · ");
  const acts = document.createElement("div");
  acts.className = "trip-acts";
  const pdf = document.createElement("button");
  pdf.type = "button"; pdf.className = "btn small"; pdf.textContent = "Relatório da saída (PDF)";
  pdf.addEventListener("click", (e) => exportReport(s.from, to, "Fora de casa: " + (s.defaultName ? `Saída de ${tripDay(s.from)}` : s.period.name), e.currentTarget));
  const see = document.createElement("button");
  see.type = "button"; see.className = "btn small"; see.textContent = "Ver nos gráficos";
  see.addEventListener("click", () => { $("tripDlg").close(); showRange(s.from, to); });
  acts.append(pdf, see);
  const top = document.createElement("div");
  top.className = "trip-top";
  top.append(meta, acts);
  body.appendChild(top);
  const other = !!(s.spec || s.hasBat || s.hasTemp || (s.events || []).length);
  if (!rc) {
    const p = document.createElement("p");
    p.className = "muted";
    p.textContent = other ? "Sem os minutos de dose do RC102 nessa saída: o histórico dele chega quando ele conecta no PC." : "Sem leitura do RC102 nessa saída. Se ele ainda não voltou a conectar no PC, o histórico dele chega quando conectar.";
    body.appendChild(p);
    if (!other) return;
  } else {
    // os quatro números
    const k = document.createElement("div");
    k.className = "trip-kpis";
    const kpi = (label, value, sub, cls) => {
      const d = sumKpi(k, label, value, sub, cls);
      return d;
    };
    const dose = tripDose(rc);
    const kd = kpi("Dose na saída", `${fmt(dose, 2)} µSv`, "", "rc");
    const kn = doseKnownEl(dose, s.homeUSv > 0 ? s.homeUSv : null, "sum-known", doseXray);
    if (kn) kd.appendChild(kn);
    kpi("Média", `${tripUSv(rc.avg)} µSv/h`, s.homeUSv > 0 ? `${fmt(rc.avg / s.homeUSv, 1)}× casa (${tripUSv(s.homeUSv)})` : "", "rc");
    kpi("Pico", `${tripUSv(rc.max)} µSv/h`, `às ${hhmm(rc.maxAt)}`, "rc");
    const al = rc.danger + rc.alarm + rc.attention;
    kpi("Alertas", al ? alertsText(rc) : "nenhum", al ? `pico ${tripUSv(rc.peakAlert)} µSv/h` : "sempre no nível seguro", al ? "up rc" : "rc");
    body.appendChild(k);
    // o gráfico
    const ch = tripCard("trip-chart", "A saída, minuto a minuto", "RC102 em µSv/h" + (s.homeUSv > 0 ? " · tracejado: fundo de casa" : ""));
    ch.appendChild(tripChartSvg(s, rc));
    body.appendChild(ch);
  }
  const row = document.createElement("div");
  row.className = "trip-row";
  row.append(tripSpecCard(s), tripDevCard(s));
  body.appendChild(row);
}

function tripChartSvg(s, rc) {
  const W = 760, H = 190, PL = 44, PR = 10, PT = 10, PB = 22;
  const pts = (rc.curve || []).filter((q) => q.usv != null);
  const wrap = document.createElement("div");
  if (!pts.length) { wrap.className = "small muted"; wrap.textContent = "sem pontos no intervalo"; return wrap; }
  const t0 = s.from, t1 = Math.max(s.to, t0 + 6e4);
  let hi = Math.max(...pts.map((q) => q.usv), s.homeUSv || 0) * 1.15;
  const st = niceStep(hi / 3);
  hi = Math.ceil(hi / st) * st;
  const x = (t) => PL + ((t - t0) / (t1 - t0)) * (W - PL - PR);
  const y = (v) => PT + (H - PT - PB) * (1 - v / hi);
  let g = "";
  for (let v = 0; v <= hi + st / 2; v += st) g += `<line x1="${PL}" x2="${W - PR}" y1="${y(v).toFixed(1)}" y2="${y(v).toFixed(1)}" class="grid"/><text x="${PL - 5}" y="${(y(v) + 4).toFixed(1)}" text-anchor="end" class="ax">${fmt(v, st < 0.01 ? 3 : 2)}</text>`;
  const hr = 36e5, span = t1 - t0, tick = span <= 3 * hr ? hr / 2 : span <= 12 * hr ? hr : span <= 48 * hr ? 6 * hr : 24 * hr;
  const first = Math.ceil(t0 / tick) * tick;
  for (let t = first; t <= t1; t += tick) if (x(t) - PL > 30 && W - PR - x(t) > 30) g += `<text x="${x(t).toFixed(1)}" y="${H - 6}" text-anchor="middle" class="ax">${hhmm(t)}</text>`;
  g += `<text x="${PL}" y="${H - 6}" text-anchor="start" class="ax">${hhmm(t0)}</text><text x="${W - PR}" y="${H - 6}" text-anchor="end" class="ax">${hhmm(t1)}</text>`;
  if (s.homeUSv > 0) g += `<line x1="${PL}" x2="${W - PR}" y1="${y(s.homeUSv).toFixed(1)}" y2="${y(s.homeUSv).toFixed(1)}" class="home"/>`;
  let d = "", prev = null;
  const stp = s.stepMs || 6e4;
  for (const q of pts) {
    const xx = x(q.t + stp / 2).toFixed(1), yy = y(q.usv).toFixed(1);
    d += `${prev != null && q.t - prev <= stp * 1.5 ? "L" : "M"}${xx} ${yy}`;
    prev = q.t;
  }
  g += `<path d="${d}" class="line"/>`;
  wrap.innerHTML = `<svg viewBox="0 0 ${W} ${H}" class="trip-svg" role="img" aria-label="RC102 durante a saída">${g}</svg>`;
  return wrap;
}

function niceStep(x) {
  if (!(x > 0)) return 0.05;
  const p = 10 ** Math.floor(Math.log10(x)), m = x / p;
  return (m <= 1 ? 1 : m <= 2 ? 2 : m <= 5 ? 5 : 10) * p;
}

// por que uma linha subiu (só as naturais; nunca é diagnóstico)
const TRIP_WHY = {
  "K-40": "K-40 mais alto é comum perto de granito, tijolo, cimento e solo.",
  "Bi-214": "Bi-214 e Pb-214 vêm do radônio: sobem em lugar fechado, subsolo e metrô.",
  "Pb-214": "Bi-214 e Pb-214 vêm do radônio: sobem em lugar fechado, subsolo e metrô.",
  "Tl-208": "Tl-208, Ac-228 e Pb-212 vêm do tório: comum em granito, areia e tijolo.",
  "Ac-228": "Tl-208, Ac-228 e Pb-212 vêm do tório: comum em granito, areia e tijolo.",
  "Pb-212": "Tl-208, Ac-228 e Pb-212 vêm do tório: comum em granito, areia e tijolo.",
};

function tripSpecCard(s) {
  const sp = s.spec;
  const c = tripCard("trip-spec", "Espectro da saída", sp ? `somado de ${rcDur(sp.objSec) || fmtMin(sp.objSec)} · por segundo, para comparar` : "");
  if (!sp || !sp.obj || !sp.obj.length) {
    const p = document.createElement("p");
    p.className = "small muted";
    p.textContent = "Sem espectro desta saída";
    c.appendChild(p);
    return c;
  }
  const box = document.createElement("div");
  box.className = "m-spec";
  c.appendChild(box);
  renderMeasSpec(box, sp, { title: "", objLabel: "na saída", bgLabel: "em casa (mesmo tempo)" });
  const h4 = box.querySelector("h4");
  if (h4) h4.remove();
  box.querySelector(".m-spec-txt").remove();
  box.querySelector(".m-spec-peaks").remove();
  const lines = s.lines || [];
  if (lines.length) {
    const t = document.createElement("p");
    t.className = "small trip-lines-h";
    t.textContent = sp.bgSec > 0 ? "O que apareceu mais que em casa" : "Linhas que apareceram";
    const ul = document.createElement("div");
    ul.className = "trip-lines";
    for (const l of lines) {
      const ch = document.createElement("span");
      ch.className = "chip mono" + (l.more ? " more" : "");
      const v = !l.hasHome ? "" : l.more ? ` · ${l.pct >= 0 ? "+" : "−"}${fmt(Math.abs(l.pct), 0)}%` : " · igual";
      ch.textContent = `${l.nuclide} · ${fmt(l.kev, 0)} keV${v}`;
      ch.title = l.group;
      ul.appendChild(ch);
    }
    c.append(t, ul);
    const why = [...new Set(lines.filter((l) => l.more && TRIP_WHY[l.nuclide]).map((l) => TRIP_WHY[l.nuclide]))];
    if (lines.some((l) => l.more)) {
      const p = document.createElement("p");
      p.className = "small muted";
      p.textContent = (why.length ? why.join(" ") + " " : "") + "Não é perigo: só mostra de onde veio a diferença.";
      c.appendChild(p);
    }
  } else {
    const p = document.createElement("p");
    p.className = "small muted";
    p.textContent = "Nenhuma linha gama conhecida se destacou na saída.";
    c.appendChild(p);
  }
  return c;
}

function tripDevCard(s) {
  const c = tripCard("trip-dev", "Como o RC102 estava", "");
  const dl = document.createElement("dl");
  const row = (k, v, sub, extra) => {
    const dt = document.createElement("dt");
    dt.textContent = k;
    const dd = document.createElement("dd");
    const b = document.createElement("span");
    b.className = "mono";
    b.textContent = v;
    dd.appendChild(b);
    if (extra) dd.appendChild(extra);
    if (sub) { const n = document.createElement("span"); n.className = "small muted"; n.textContent = sub; dd.appendChild(n); }
    dl.append(dt, dd);
  };
  const dur = whDur(s.from, s.to);
  if (s.hasBat) {
    const bar = document.createElement("i");
    bar.className = "trip-bat";
    bar.innerHTML = "<i></i>";
    bar.firstChild.style.width = Math.max(0, Math.min(100, s.bat1)) + "%";
    const used = s.bat0 - s.bat1;
    row("Bateria", `${fmt(s.bat0, 0)}% → ${fmt(s.bat1, 0)}%`, used > 0 ? `gastou ${fmt(used, 0)}% em ${dur}` : used < 0 ? "carregou no caminho" : "", bar);
  } else row("Bateria", "sem registro", "");
  if (s.hasTemp) row("Temperatura", `${fmt(s.tempMin, 0)} → ${fmt(s.tempMax, 0)} °C`, `máx. às ${hhmm(s.tempMaxAt)}`);
  else row("Temperatura", "sem registro", "");
  const ev = s.events || [];
  if (ev.length) {
    const names = [...new Set(ev.map((e) => e.event))];
    row("Avisos do aparelho", names.join(" · "), `${ev.length} ${ev.length === 1 ? "aviso" : "avisos"}, o primeiro às ${hhmm(ev[0].t)}`);
  } else row("Avisos do aparelho", "nenhum", "alarmes 1 e 2, temperatura, fora de escala");
  c.appendChild(dl);
  if (!s.hasBat && !s.hasTemp) {
    const p = document.createElement("p");
    p.className = "small muted";
    p.textContent = "Bateria e temperatura vêm do histórico do próprio RC102, que chega quando ele conecta no PC.";
    c.appendChild(p);
  }
  return c;
}

function wireTrip() {
  $("tripDlg").addEventListener("close", () => { tripSeq++; tripOpenId = 0; });
}
