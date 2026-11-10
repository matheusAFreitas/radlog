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
  let lv = "seguro", title, text, calm = false;
  const parts = [];
  // quem está em casa e quem está com você (onde.js)
  const wh = state.where || {};
  const gHome = (wh.gmc || "casa") === "casa", rHome = wh.radiacode === "casa";
  const rd = [];
  // aparelho fora do cálculo não entra na frase
  if (g.live && g.usv != null && wh.gmc !== "fora") rd.push([gHome, `${fmt(g.usv, 3)} µSv/h pelo GMC-800`]);
  if (r.live && r.usv != null && wh.radiacode !== "fora") rd.push([rHome, `${fmt(r.usv, 3)} µSv/h pelo RC102`]);
  const moved = !gHome || rHome;
  for (const [home, v] of rd) {
    if (!moved) parts.push(home ? `Casa em ${v.replace(" pelo GMC-800", "")}` : `você em ${v.replace(" pelo RC102", "")}`);
    else parts.push(home ? `Casa em ${v}` : `Com você em ${v}`);
  }
  if (top) {
    lv = { alarme: "alarme", perigo: "perigo" }[lvlCls(top)] || "atencao";
    title = `${lvlName(top)} no ${isRcEv(top) ? "RC102" : "GMC-800"} agora.`;
    text = parts.concat("Veja os detalhes em Alertas.").join(" · ");
  } else if (!g.live && !r.live) {
    lv = "off";
    title = "Nenhum aparelho lendo agora.";
    text = "A dose abaixo é a que ficou gravada. Conecte o GMC-800 ou o RC102 para acompanhar ao vivo.";
  } else {
    // saúde e lugar de cada aparelho (onde.js): falta medição num papel, ou algum fora do normal
    const cw = whCur(), L = whLines(cw);
    const lacking = [L.hr, L.pr].some((x) => !x.src && x.why === "sem dado");
    const odd = ["gmc", "radiacode"].some((dv) => cw[dv] === "fora" || (cw[dv] !== "fora" && ["saturado", "sem contagens"].includes(whLive(dv).state)));
    if (lacking) {
      lv = "atencao"; title = "Falta medição agora.";
      text = [L.home, L.pers].filter((x) => x.cls === "warnb" && !x.sat).map((x) => x.d).join(" ");
    } else {
      title = odd ? "Medindo, com um aparelho fora do normal." : "Tudo normal agora.";
      // tudo normal: a faixa some (só aparece quando tem algo a dizer)
      calm = !odd;
      if (g.live && gHome && cw.gmc === "casa" && whMeasuring("gmc") && g.sigma != null) parts.push(g.sigma < 2 ? "no nível de fundo habitual" : `${fmt(g.sigma, 1)} σ acima do nível de fundo`);
      text = parts.join(" · ");
    }
  }
  box.hidden = calm;
  box.className = "card ag-say lv-" + lv;
  lvLabel($("agSayTitle"), lv, title, 20);
  $("agSayText").textContent = text;
  agBgChip();
}

// selo do fundo de casa no cabeçalho da Minha dose (acompanha pessoal/casa)
function agBgChip() {
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
  agBgChip();
  $("agDose").classList.toggle("rc", dev === "pessoal");
  // o aparelho com esse papel agora (onde.js); "" = nenhum
  const now = d.device === undefined ? (dev === "pessoal" ? "radiacode" : "gmc") : d.device;
  const since = state.where && state.where.since ? ` desde ${hhmm(state.where.since)}` : "";
  const devName = { radiacode: "RC102", gmc: "GMC-800" };
  // por que ninguém faz esse papel agora
  const wc = whCur(), outs = ["gmc", "radiacode"].filter((x) => wc[x] === "fora").map((x) => devName[x]);
  const why = outs.length ? `${outs.join(" e ")} fora do cálculo${since}` : null;
  $("agDevNote").textContent = dev === "pessoal"
    ? (!now ? `Dose pessoal: nenhum aparelho está com você agora (${why || "os dois em casa" + since}).`
      : now === "gmc" && wc.gmc === "casa" ? `Dose pessoal pela leitura de casa${since} (os dois ficaram em casa, e você também).`
      : now === "gmc" ? `Dose pessoal, pelo aparelho que está com você: o GMC-800${since}.`
      : d.hasData ? "Dose pessoal, pelo RC102 (anda com você)." : "O RC102 ainda não tem dado neste ano. Troque para \"casa\" para ver a do GMC-800.")
    : (!now ? `Dose da casa: nenhum aparelho em casa agora (${why || "os dois com você" + since}).`
      : now === "radiacode" ? `Dose da casa, pelo aparelho que fica parado: o RC102${since}.`
      : d.hasData ? "Dose da casa, pelo GMC-800 (fica parado)." : "O GMC-800 ainda não tem dado neste ano.");
  // Radiacode com você mas longe do PC: a dose segue no aparelho, a curva não
  const rl = whLive("radiacode");
  if (dev === "pessoal" && whRole("comigo", whCur()).curve && rl.last) $("agDevNote").textContent += ` Curva incompleta desde ${hhmm(rl.last)}.`;
  const box = $("agCells");
  box.textContent = "";
  const periods = d.periods && d.periods.length ? d.periods : Object.keys(AG_KEYS).map((key) => ({ key, total: 0, measuredMinutes: 0 }));
  for (const p of periods) {
    const cell = document.createElement("div");
    cell.className = "ag-cell" + (p.key === "dia" ? " now" : "");
    const k = document.createElement("span");
    k.className = "k";
    const kl = document.createElement("span");
    kl.textContent = AG_KEYS[p.key] || p.key;
    k.appendChild(kl);
    // quem mediu (onde está cada aparelho): o aparelho, misto ou ninguém
    const main = dev === "pessoal" ? "radiacode" : "gmc", other = main === "radiacode" ? "gmc" : "radiacode";
    const swap = p.swapMinutes || 0, gap = p.gapMinutes || 0, meas = p.measuredMinutes || 0;
    const nod = p.noDataMinutes || 0, sat = p.satMinutes || 0;
    const tag = document.createElement("span");
    tag.className = "wh-src";
    if (swap && swap < meas) { tag.classList.add("mixed"); tag.innerHTML = '<span class="dot" style="background:var(--swap)"></span>'; tag.append("misto"); }
    else if (!meas && gap) { tag.classList.add("mixed"); tag.textContent = "sem medição"; cell.classList.add("blankv"); }
    else if (!meas && nod) { tag.classList.add("mixed"); tag.textContent = "sem dado"; cell.classList.add("blankv"); }
    else { const who = swap ? other : main; tag.innerHTML = `<span class="dot ${who === "gmc" ? "gmc" : "rc"}"></span>`; tag.append(devName[who]); }
    k.appendChild(tag);
    const v = document.createElement("span");
    v.className = "v";
    const [num, unit] = agSv(p.measuredMinutes || p.total ? p.total : null);
    // minutos saturados contam pelo valor lido: a dose real é pelo menos isso
    v.textContent = (sat && meas ? "≥ " : "") + num;
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
    cell.append(k, v);
    if (swap || gap || (nod && meas)) {
      const tot = meas + gap + nod || 1;
      const bar = document.createElement("div");
      bar.className = "wh-mix";
      bar.setAttribute("aria-hidden", "true");
      const col = (x) => (x === "radiacode" ? "var(--rc)" : "var(--accent)");
      for (const [w, bg] of [[meas - swap, col(main)], [swap, col(other)], [gap, "var(--gap)"], [nod, "var(--nosig)"]]) {
        const i = document.createElement("i");
        i.style.width = `${(w / tot) * 100}%`;
        i.style.background = bg;
        bar.appendChild(i);
      }
      cell.appendChild(bar);
      const parts = [];
      const pc = (x) => { const v = (x / tot) * 100; return v < 1 ? "<1%" : `${Math.round(v)}%`; };
      if (swap) parts.push(`${pc(swap)} pelo ${devName[other]}`);
      if (gap) parts.push(`${pc(gap)} sem medição`);
      if (nod) parts.push(`${pc(nod)} sem dado`);
      n.textContent = parts.join(" · ");
      n.classList.remove("up");
    }
    cell.append(n);
    // em coisas conhecidas: tempo de fundo da casa e banana, voo, raio-X
    if (p.measuredMinutes || p.total) {
      const kn = doseKnownEl(p.total, d.backgroundRate || null, "ag-known");
      if (kn) cell.append(kn);
    }
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
    if (pTo(p) < t0 || p.from > t1) continue;
    const r = document.createElementNS(ns, "rect");
    const a = xt(Math.max(p.from, t0)), b = xt(Math.min(pTo(p), t1));
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
      items.push([e.time, `${lvlName(e)} no ${isRcEv(e) ? "RC102" : "GMC-800"}${dur}`]);
    }
  } catch (e) { /* sem alertas */ }
  try {
    for (const nt of (await api.GetNotes(from, now)) || []) items.push([nt.time, nt.text]);
  } catch (e) { /* sem anotações */ }
  for (const p of state.periods || []) {
    if (pTo(p) < from || p.from > now) continue;
    items.push([Math.max(p.from, from), (p.open ? "fora de casa até agora" : `fora de casa até ${hhmm(p.to)}`) + (p.name ? ` · ${p.name}` : "")]);
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
