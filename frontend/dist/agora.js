// Tela "Agora" (2.17): a página inicial. Responde "está tudo bem?" numa frase,
// mostra a Minha dose (hoje, semana, mês e ano) e o dia numa linha do tempo.
//
// Usa os globais do app.js (state, api, $, fmt, hhmm...). Este arquivo é
// carregado antes do app.js e as funções só rodam depois que ele terminou.

const AG_KEYS = { dia: "Hoje", semana: "Semana", mes: "Mês", ano: "Ano" };
let agDose = null, agSeq = 0, agAt = 0;

function agDev() {
  const pref = localGet("agDev", "");
  if (pref === "pessoal" || pref === "casa") return pref;
  return (agDose && agDose.default) || "pessoal";
}

// µSv com casas que façam sentido; de 1000 µSv em diante vira mSv
function agSv(usv) {
  if (usv == null || !isFinite(usv)) return ["—", ""];
  if (Math.abs(usv) >= 1000) return [fmt(usv / 1000, 2), "mSv"];
  return [fmt(usv, usv < 10 ? 2 : usv < 100 ? 1 : 0), "µSv"];
}

// ---------- frase do topo ----------

function agoraNow() {
  if (state.view !== "agora") return;
  const g = state.gNow || {}, r = state.rNow || {};
  const list = state.activeAlerts || [];
  const rank = { atencao: 1, alarme: 2, perigo: 3 };
  const top = list.slice().sort((a, b) => (rank[lvlCls(b)] || 1) - (rank[lvlCls(a)] || 1))[0];
  const box = $("agSay");
  let lv = "seguro", title, text;
  const parts = [];
  if (g.live && g.usv != null) parts.push(`Casa em ${fmt(g.usv, 3)} µSv/h`);
  if (r.live && r.usv != null) parts.push(`você em ${fmt(r.usv, 3)} µSv/h`);
  if (top) {
    lv = { alarme: "alerta", perigo: "perigo" }[lvlCls(top)] || "atencao";
    title = `${lvlName(top)} no ${isRcEv(top) ? "RadiaCode" : "GMC-800"} agora.`;
    text = parts.concat("Veja os detalhes em Alertas.").join(" · ");
  } else if (!g.live && !r.live) {
    lv = "off";
    title = "Nenhum aparelho lendo agora.";
    text = "A dose abaixo é a que ficou gravada. Conecte o GMC-800 ou o RadiaCode para acompanhar ao vivo.";
  } else {
    title = "Tudo normal agora.";
    if (g.live && g.sigma != null) parts.push(g.sigma < 2 ? "dentro do seu fundo" : `${fmt(g.sigma, 1)} σ acima do seu fundo`);
    text = parts.join(" · ");
  }
  box.className = "card ag-say lv-" + lv;
  $("agSayTitle").textContent = title;
  $("agSayText").textContent = text;
  const d = agDose && (agDev() === "pessoal" ? agDose.personal : agDose.home);
  const chip = $("agSayBg");
  chip.hidden = !(d && d.backgroundRate > 0);
  if (!chip.hidden) chip.textContent = `fundo de casa · ${fmt(d.backgroundRate, 3)} µSv/h`;
}

// ---------- Minha dose ----------

function renderAgDose() {
  if (!agDose) return;
  const dev = agDev();
  document.querySelectorAll("#agDevSeg button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.dev === dev)));
  const d = dev === "pessoal" ? agDose.personal : agDose.home;
  $("agDose").classList.toggle("rc", dev === "pessoal");
  $("agDevNote").textContent = dev === "pessoal"
    ? (d.hasData ? "Dose pessoal, pelo RadiaCode (anda com você)." : "O RadiaCode ainda não tem dado neste ano. Troque para \"casa\" para ver a do GMC-800.")
    : (d.hasData ? "Dose da casa, pelo GMC-800 (fica parado)." : "O GMC-800 ainda não tem dado neste ano.");
  const box = $("agCells");
  box.textContent = "";
  const periods = d.periods && d.periods.length ? d.periods : Object.keys(AG_KEYS).map((key) => ({ key, total: 0, measuredMinutes: 0 }));
  for (const p of periods) {
    const cell = document.createElement("div");
    cell.className = "ag-cell" + (p.key === "dia" ? " now" : "");
    const k = document.createElement("span");
    k.className = "k";
    k.textContent = AG_KEYS[p.key] || p.key;
    const v = document.createElement("span");
    v.className = "v";
    const [num, unit] = agSv(p.measuredMinutes || p.total ? p.total : null);
    v.textContent = num;
    if (unit) { const u = document.createElement("small"); u.textContent = unit; v.appendChild(u); }
    const n = document.createElement("span");
    n.className = "n";
    if (!p.measuredMinutes && !p.total) n.textContent = "sem dado";
    else if (p.key === "ano" && d.yearProjection > 0) {
      const [pn, pu] = agSv(d.yearProjection);
      n.textContent = `no ano todo, nesse ritmo: ${pn} ${pu}`;
    } else if (p.background > 0) {
      const pct = Math.round(p.above * 100);
      const [bn, bu] = agSv(p.background);
      n.textContent = `o fundo daria ${bn} ${bu}` + (pct >= 1 ? ` · +${pct}%` : pct <= -1 ? ` · ${pct}%` : " · igual");
      if (pct >= 10) n.classList.add("up");
    } else n.textContent = `${fmt(p.coverage * 100, 0)}% medido`;
    cell.append(k, v, n);
    box.appendChild(cell);
  }
  // barra do ano: o que já foi, o que só o fundo daria no ano inteiro, e o teto
  const year = periods.find((p) => p.key === "ano");
  const top = Math.max(d.yearCeiling || 0, d.yearProjection || 0, year ? year.total : 0) * 1.06 || 1;
  const bar = $("agBar");
  bar.querySelector(".ag-me").style.width = `${year ? Math.min(100, (year.total / top) * 100) : 0}%`;
  bar.querySelector(".ag-nat").style.width = `${Math.min(100, ((d.yearBackground || 0) / top) * 100)}%`;
  bar.querySelector(".ag-cap").style.left = `${Math.min(99.6, ((d.yearCeiling || 0) / top) * 100)}%`;
  const leg = $("agLeg");
  leg.textContent = "";
  const item = (color, label, val) => {
    const s = document.createElement("span");
    const sw = document.createElement("i");
    sw.className = "ag-sw";
    sw.style.background = color;
    const b = document.createElement("b");
    b.textContent = label;
    const v = document.createElement("span");
    v.textContent = val;
    s.append(sw, b, v);
    leg.appendChild(s);
  };
  const [yn, yu] = agSv(year ? year.total : null);
  const [bn, bu] = agSv(d.yearBackground);
  const [cn, cu] = agSv(d.yearCeiling);
  item(dev === "pessoal" ? COR.rc : "var(--accent)", "neste ano", `${yn} ${yu}`);
  item("rgba(163, 173, 180, .5)", "só o fundo, no ano todo", `${bn} ${bu}`);
  item("var(--warn)", "teto", `${cn} ${cu} (fundo + ${fmt(agDose.limitMSvYear, 0)} mSv/ano)`);
  agoraNow();
}

async function refreshAgDose(force) {
  // a dose do ano relê meses de dados: refaz a cada 5 min (o dia, a cada 1 min)
  if (!force && agDose && Date.now() - agAt < 5 * 60e3) { renderAgDose(); return; }
  const my = ++agSeq;
  let d;
  try { d = api.GetMyDose ? await api.GetMyDose(state.doseLimit) : null; } catch (e) { d = null; }
  if (my !== agSeq || !d) return;
  agDose = d;
  agAt = Date.now();
  renderAgDose();
}

// ---------- o dia ----------

function agLine(vals, x, y, color) {
  let d = "", pen = false;
  vals.forEach((v, i) => {
    if (v == null) { pen = false; return; }
    d += (pen ? "L" : "M") + x(i).toFixed(1) + "," + y(v).toFixed(1);
    pen = true;
  });
  if (!d) return null;
  const p = document.createElementNS("http://www.w3.org/2000/svg", "path");
  p.setAttribute("d", d);
  p.setAttribute("fill", "none");
  p.setAttribute("stroke", color);
  p.setAttribute("stroke-width", "1.6");
  p.setAttribute("vector-effect", "non-scaling-stroke");
  return p;
}

async function refreshAgDay() {
  const svg = $("agChart");
  let c;
  try { c = await api.GetChart("24h"); } catch (e) { return; }
  const ns = "http://www.w3.org/2000/svg";
  svg.textContent = "";
  const W = 800, H = 120, n = (c.times || []).length;
  if (!n) return;
  const g = c.usv || [], rc = c.rc || [];
  const all = g.concat(rc).filter((v) => v != null);
  const attn = c.attentionUSv || 0;
  const max = Math.max(attn * 1.1, ...all, 0.05);
  const x = (i) => (n > 1 ? (i / (n - 1)) * W : 0);
  const y = (v) => H - 8 - (v / max) * (H - 16);
  // fora de casa hoje
  const t0 = c.times[0], t1 = c.times[n - 1];
  const xt = (t) => ((t - t0) / Math.max(1, t1 - t0)) * W;
  for (const p of state.periods || []) {
    if (p.to < t0 || p.from > t1) continue;
    const r = document.createElementNS(ns, "rect");
    const a = xt(Math.max(p.from, t0)), b = xt(Math.min(p.to, t1));
    r.setAttribute("x", a); r.setAttribute("y", 0); r.setAttribute("width", Math.max(1, b - a)); r.setAttribute("height", H);
    r.setAttribute("fill", "rgba(138, 180, 248, .10)");
    svg.appendChild(r);
  }
  if (attn > 0 && attn < max) {
    const l = document.createElementNS(ns, "line");
    l.setAttribute("x1", 0); l.setAttribute("x2", W); l.setAttribute("y1", y(attn)); l.setAttribute("y2", y(attn));
    l.setAttribute("stroke", "rgba(245, 184, 74, .55)"); l.setAttribute("stroke-dasharray", "4 4");
    l.setAttribute("vector-effect", "non-scaling-stroke");
    svg.appendChild(l);
  }
  const lg = agLine(g, x, y, COR.accent);
  const lr = agLine(rc, x, y, COR.rc);
  if (lg) svg.appendChild(lg);
  if (lr) svg.appendChild(lr);
  $("agAx0").textContent = hhmm(t0);
  $("agAx1").textContent = hhmm(t0 + (t1 - t0) / 2);

  // o que aconteceu hoje: alertas, anotações e fora de casa
  const day0 = new Date(); day0.setHours(0, 0, 0, 0);
  const from = day0.getTime(), now = Date.now();
  const items = [];
  try {
    for (const e of (await api.GetEvents("alerta")) || []) {
      if (e.time < from) continue;
      const dur = e.end ? ` · ${Math.max(1, Math.round((e.end - e.time) / 6e4))} min` : e.active ? " · agora" : "";
      items.push([e.time, `${lvlName(e)} no ${isRcEv(e) ? "RadiaCode" : "GMC-800"}${dur}`]);
    }
  } catch (e) { /* sem alertas */ }
  try {
    for (const nt of (await api.GetNotes(from, now)) || []) items.push([nt.time, nt.text]);
  } catch (e) { /* sem anotações */ }
  for (const p of state.periods || []) {
    if (p.to < from || p.from > now) continue;
    items.push([Math.max(p.from, from), `fora de casa até ${hhmm(p.to)}` + (p.name ? ` · ${p.name}` : "")]);
  }
  items.sort((a, b) => a[0] - b[0]);
  const ul = $("agItems");
  ul.textContent = "";
  if (!items.length) {
    const li = document.createElement("li");
    li.textContent = "Nada marcado hoje: nenhum alerta, anotação ou saída de casa.";
    ul.appendChild(li);
  }
  for (const [t, txt] of items.slice(-12)) {
    const li = document.createElement("li");
    const s = document.createElement("span");
    s.className = "mono";
    s.textContent = hhmm(t);
    li.append(s, document.createTextNode(txt));
    ul.appendChild(li);
  }
}

async function refreshAgora(force) {
  if (state.view !== "agora") return;
  agoraNow();
  await Promise.allSettled([refreshAgDose(force), refreshAgDay()]);
}

document.addEventListener("click", (e) => {
  const b = e.target.closest("#agDevSeg button");
  if (!b) return;
  localSet("agDev", b.dataset.dev);
  renderAgDose();
});
setInterval(() => { if (state.view === "agora") refreshAgora(); }, 60e3);
