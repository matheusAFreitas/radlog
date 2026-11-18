// Tela "Agora" (2.17): a página inicial. Responde "está tudo bem?" numa frase,
// mostra a Minha dose (hoje, semana, mês e ano) e o dia na fita (fita.js).
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
const AG_OF = { dia: "do dia", semana: "da semana", mes: "do mês", ano: "do ano" };
function agSv(usv) {
  if (usv == null || !isFinite(usv)) return ["—", ""];
  if (RATE_MREM) return fmtDose(usv); // mrem (2.84.0)
  if (Math.abs(usv) >= 1000) return [fmt(usv / 1000, 2), "mSv"];
  return [fmt(usv, usv < 10 ? 2 : usv < 100 ? 1 : 0), "µSv"];
}

// 2.90.0 (A1): na Minha dose tudo numa unidade só (µSv, ou mrem com a
// taxa em mrem/h), sem trocar para mSv no meio da linha
function agU() { return RATE_MREM ? "mrem" : "µSv"; }
function agN(usv) {
  if (usv == null || !isFinite(usv)) return "—";
  const v = RATE_MREM ? usv / 10 : usv;
  return fmt(v, v < 10 ? 2 : v < 100 ? 1 : 0);
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
      if (g.live && gHome && cw.gmc === "casa" && whMeasuring("gmc") && g.sigma != null) parts.push(g.sigma < 2 ? "no nível de fundo habitual" : g.sigma < 3 ? `um pouco acima do fundo, dentro do acaso (${fmt(g.sigma, 1)} σ)` : `bem acima do nível de fundo (${fmt(g.sigma, 1)} σ)`);
      text = parts.join(" · ");
    }
  }
  box.hidden = calm;
  box.className = "card ag-say lv-" + lv;
  lvLabel($("agSayTitle"), lv, title, 20);
  $("agSayText").textContent = text;
  // fundo acima do normal sem alerta: "Por que?" (porque.js)
  const why = $("agWhy");
  if (why) why.hidden = !(api.GetWhyRise && !top && g.live && gHome && g.sigma != null && g.sigma >= 2);
  agBgChip();
}

// selo do fundo de casa no cabeçalho da Minha dose: saiu na 2.90.0 (A1), a
// frase de hoje já compara com o fundo
function agBgChip() {
  const chip = $("agSayBg");
  if (chip) chip.hidden = true;
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
  const why = outs.length === 2 ? `os dois ignorados${since}` : outs.length ? `${outs[0]} ignorado${since}` : null;
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
  // 2.90.0 (A1): a frase de quem mede só aparece quando foge do de sempre
  const usual = ["Dose pessoal, pelo RC102 (anda com você).", "Dose da casa, pelo GMC-800 (fica parado)."];
  $("agDevNote").hidden = usual.includes($("agDevNote").textContent);
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
    else if (!meas && nod) { tag.classList.add("nodata"); tag.textContent = "sem dado"; cell.classList.add("blankv"); }
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
      n.textContent = `o fundo daria ${bn} ${bu}` + (pct >= 1 ? ` · +${pct}%` : pct <= -1 ? ` · −${-pct}%` : " · " + tr("igual"));
      if (pct >= 10) n.classList.add("up");
    } else n.textContent = `${fmt(p.coverage * 100, 0)}% medido`;
    cell.append(k, v);
    if (swap || gap || (nod && meas)) {
      const tot = meas + gap + nod || 1;
      const bar = document.createElement("div");
      bar.className = "wh-mix";
      bar.setAttribute("aria-hidden", "true");
      const col = (x) => (x === "radiacode" ? "var(--rc)" : "var(--gmc)");
      for (const [w, bg] of [[meas - swap, col(main)], [swap, col(other)], [gap, "var(--gap)"], [nod, "var(--nodata)"]]) {
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
      // 2.84.0: sem dado quase sempre é o aparelho desligado; texto neutro
      if (nod) parts.push(`${devName[main]} desligado em ${pc(nod)} ${AG_OF[p.key] || "do período"}`);
      n.textContent = parts.join(" · ");
      n.classList.remove("up");
    }
    cell.append(n);
    // em coisas conhecidas: tempo de fundo da casa e voo, raio-X
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
  // 2.90.0 (A1): o número de hoje, uma frase só, e os outros períodos
  // numa linha, tudo na mesma unidade
  const per = (k) => periods.find((p) => p.key === k);
  const has = (p) => p && (p.measuredMinutes || p.total);
  const today = per("dia");
  const u = agU();
  $("agToday").textContent = has(today) ? (today.satMinutes && today.measuredMinutes ? "≥ " : "") + agN(today.total) : "—";
  $("agTodayU").textContent = u;
  const cmp = $("agCmp");
  cmp.className = "ag-cmp";
  if (!has(today)) cmp.textContent = "sem dado hoje";
  else if (today.background > 0) {
    const pct = Math.round(today.above * 100);
    cmp.textContent = pct >= 10 ? `${pct}% acima do fundo da sua casa` : pct <= -10 ? `${-pct}% abaixo do fundo da sua casa` : pct >= 1 || pct <= -1 ? `perto do fundo da sua casa (${pct > 0 ? "+" : "−"}${Math.abs(pct)}%)` : "igual ao fundo da sua casa"; // 2.101.6: o mesmo % da célula
    if (pct >= 10) cmp.classList.add("up");
  } else cmp.textContent = "";
  const line = $("agLine");
  line.textContent = "";
  [["semana", "semana"], ["mes", "mês"], ["ano", "ano"]].forEach(([k, label], i) => {
    if (i) line.append(" · ");
    const p = per(k), b = document.createElement("b");
    b.className = "mono";
    b.textContent = has(p) ? agN(p.total) : "—";
    line.append(Object.assign(document.createElement("span"), { textContent: label }), " ", b);
  });
  line.append(" " + u);
  $("agYr").textContent = d.yearCeiling > 0 ? `ano: ${agN(year ? year.total : 0)} ${u} de um teto de ${agN(d.yearCeiling)} ${u}` : "";
  $("agYr").title = `Teto: o fundo da casa no ano todo (${agN(d.yearBackground)} ${u}) mais ${fmt(agDose.limitMSvYear, 0)} mSv/ano.`;
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

// ---------- o dia: a fita (fita.js, 2.42) ----------

async function refreshAgora(force) {
  if (state.view !== "agora") return;
  agoraNow();
  await Promise.allSettled([refreshAgDose(force), refreshAgFita()]);
}

document.addEventListener("click", (e) => {
  const b = e.target.closest("#agDevSeg button");
  if (!b) return;
  localSet("agDev", b.dataset.dev);
  renderAgDose();
});
setInterval(() => { if (state.view === "agora") refreshAgora(); }, 60e3);
