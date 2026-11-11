"use strict";

// ---------- subida lenta do fundo (2.58) ----------
//
// Cartão "Fundo nas últimas semanas" no topo de Gráficos › Tendência do
// fundo (GetBgSlow, bgslow.go): veredito, três números, gráfico de 120 dias
// em % da referência e "O que conferir". Aviso informativo no topo de
// Alertas enquanto der "vale olhar" ou "degrau" (some sozinho, sem
// histórico). "Mudei o aparelho de lugar" fica no menu do logo (busca.js) e
// como atalho no cartão quando der degrau. Spec: ux/fundo.html.
//
// Usa os globais do app.js (api, $, fmt, toast, errMsg, askConfirm) e o
// tbChecked do tubo.js. Carregado antes do app.js: só define funções.

const BS = { b: null, busy: false, days: [] };

const bsPct = (v) => (v >= 0 ? "+" : "−") + fmt(Math.abs(v * 100), 0) + "%";
function bsDay(ms) {
  const d = new Date(ms), p = (n) => String(n).padStart(2, "0");
  return `${p(d.getDate())}/${p(d.getMonth() + 1)}`;
}
function bsMed(a) {
  if (!a.length) return 0;
  const b = a.slice().sort((x, y) => x - y), m = b.length >> 1;
  return b.length % 2 ? b[m] : (b[m - 1] + b[m]) / 2;
}

// veredito: [ícone, título, frase]
function bsVerdict(b) {
  if (b.state === "na") {
    return b.moved
      ? ["na", `Nova referência desde ${bsDay(b.moved)}: faltam ${b.left} dias`, "Você marcou que mudou o GMC-800 de lugar. O app junta 30 dias válidos no lugar novo antes de comparar de novo."]
      : ["na", `Ainda medindo: faltam ${b.left} dias`, "Para comparar, o app precisa de uns 30 dias válidos de fundo em casa como referência, mais 2 semanas recentes. Até lá o cartão só mostra o gráfico."];
  }
  if (b.state === "ok") return ["ok", "O fundo de casa está estável", `Últimas 2 semanas ${bsPct(b.rise)} em relação à referência. Isso é o vai e vem normal do radônio e do tempo.`];
  if (b.state === "step") {
    const rc = !b.rcOk ? "Não há RC102 em casa nas duas épocas para comparar." : b.env ? "O RC102 em casa subiu junto." : "O RC102 em casa não mudou, então foi algo perto do GMC-800.";
    return ["warn", `O fundo mudou de patamar em ${bsDay(b.stepDay)}`, `Subiu de uma vez (${bsPct(b.stepPct)}) e ficou. ${rc} Mudou o aparelho de lugar ou chegou algo novo perto dele?`];
  }
  if (!b.rcOk) return ["warn", `O fundo de casa subiu devagar: ${bsPct(b.rise)} em ${b.weeks} semanas`, "Sem o RC102 em casa nas duas épocas, não dá para saber se foi o ambiente ou o aparelho. Ainda é fundo baixo, mas vale descobrir o motivo."];
  if (b.env) return ["warn", `O fundo de casa subiu devagar: ${bsPct(b.rise)} em ${b.weeks} semanas`, `O RC102 em casa subiu junto (${bsPct(b.rcRise)}), então é o ambiente, não o aparelho. Ainda é fundo baixo, mas vale descobrir o motivo.`];
  return ["warn", `Só o GMC-800 subiu: ${bsPct(b.rise)} em ${b.weeks} semanas`, `O RC102 em casa ficou igual (${bsPct(b.rcRise)}). Isso aponta para o aparelho, não para o ambiente.`];
}

const BS_TODO = {
  env: [
    ["Casa mais fechada:", "o radônio do chão e das paredes acumula quando a casa fica fechada. Janelas abertas umas horas por dia costumam baixar o fundo."],
    ["Algo novo em casa:", "bancada ou piso de granito, revestimento cerâmico, areia, adubo com potássio, luminária ou relógio antigo. Dá para usar o \"Medir objeto\" com o RC102 para achar."],
    ["Época do ano:", "se a referência é de meses mais ventilados, um pouco de subida é esperado. O Calendário do ano mostra se aconteceu no ano passado."],
  ],
  dev: [
    ["Saúde do tubo:", "veja se a sensibilidade ou a dispersão mudaram no cartão da tela do GMC-800."],
    ["Calibração:", "a conta é em contagens, então trocar o fator não muda nada aqui. Se mudou, foi o tubo contando diferente."],
    ["Interferência:", "fonte, carregador ou aparelho novo ligado perto do GMC-800 pode somar pulsos falsos."],
  ],
  step: [
    ["Mudou o GMC-800 de lugar?", "Perto de parede, piso ou janela diferente o fundo muda. Se mudou, marque abaixo: o app começa uma referência nova a partir desse dia."],
    ["Chegou algo novo perto dele?", "Granito, cerâmica, luminária antiga. Afaste e veja se volta."],
  ],
};

function bsTodo(b) {
  let items = [];
  if (b.state === "step") items = BS_TODO.step;
  else if (b.state === "warn") items = !b.rcOk ? [...BS_TODO.env, ...BS_TODO.dev] : b.env ? BS_TODO.env : BS_TODO.dev;
  if (!items.length) return "";
  return `<div class="bs-todo"><h3>O que conferir</h3><ul>${items.map(([k, t]) => `<li><b>${k}</b> <span>${t}</span></li>`).join("")}</ul></div>`;
}

// mediana de 7 dias de uma série (só dias válidos), em % de ref
function bsRoll(days, key, ref) {
  const out = [];
  if (!(ref > 0)) return out;
  for (let i = 0; i < days.length; i++) {
    const w = [];
    for (let j = Math.max(0, i - 6); j <= i; j++) if (days[j].valid && days[j][key] > 0) w.push(days[j][key]);
    if (w.length >= 4) out.push([i, bsMed(w) / ref * 100]);
  }
  return out;
}

function bsChart(b, W) {
  const days = b.days, N = days.length, H = 220, L = 40, T = 10, B = 22, pw = W - L - 8, ph = H - T - B, ref = b.refGm;
  const vals = days.filter((d) => d.gm > 0).map((d) => d.gm / ref * 100);
  const lo = Math.min(85, Math.min(...vals, 100) - 3), hi = Math.max(120, Math.max(...vals, 100) + 3);
  const x = (i) => L + (i + 0.5) / N * pw, y = (v) => T + (hi - v) / (hi - lo) * ph, f = (v) => v.toFixed(1);
  let s = "";
  for (let t = Math.ceil(lo / 10) * 10; t <= hi; t += 10) s += `<line class="bs-grid" x1="${L}" x2="${L + pw}" y1="${f(y(t))}" y2="${f(y(t))}"/><text class="bs-yl" x="${L - 6}" y="${f(y(t) + 3.5)}" text-anchor="end">${t}%</text>`;
  s += `<rect class="bs-band" x="${L}" y="${f(y(105))}" width="${pw}" height="${f(y(95) - y(105))}"/>`;
  s += `<line class="bs-ref" x1="${L}" x2="${L + pw}" y1="${f(y(100))}" y2="${f(y(100))}"/>`;
  s += `<line class="bs-lim" x1="${L}" x2="${L + pw}" y1="${f(y(110))}" y2="${f(y(110))}"/><text class="bs-liml" x="${L + 6}" y="${f(y(110) - 4)}">+10%: vale olhar</text>`;
  // janelas: referência e últimas 2 semanas
  const mi = b.moved ? days.findIndex((d) => d.day >= b.moved) : -1;
  const rs = mi >= 0 ? mi : 0, re = mi >= 0 ? N - 14 : N - 30;
  if (b.refN > 0 && re > rs) s += `<rect class="bs-wref" x="${f(L + rs / N * pw)}" y="${T + ph + 3}" width="${f((re - rs) / N * pw)}" height="3"/>`;
  s += `<rect class="bs-wrec" x="${f(L + (N - 14) / N * pw)}" y="${T + ph + 3}" width="${f(14 / N * pw)}" height="3"/>`;
  if (mi >= 0) s += `<line class="bs-mv" x1="${f(x(mi))}" x2="${f(x(mi))}" y1="${T}" y2="${T + ph}"/><text class="bs-mvl" x="${f(x(mi) + 4)}" y="${T + 11}">mudou de lugar</text>`;
  days.forEach((d, i) => {
    if (!(d.gm > 0)) return;
    const v = d.gm / ref * 100;
    s += d.rain ? `<circle class="bs-rain" cx="${f(x(i))}" cy="${f(y(v))}" r="2.6"/>` : `<circle class="bs-dot" cx="${f(x(i))}" cy="${f(y(v))}" r="2.1"/>`;
  });
  const line = (pts, cls) => (pts.length > 1 ? `<polyline class="${cls}" points="${pts.map(([i, v]) => f(x(i)) + "," + f(y(v))).join(" ")}"/>` : "");
  if (b.refRc > 0) s += line(bsRoll(days, "rc", b.refRc), "bs-rc");
  s += line(bsRoll(days, "gm", ref), "bs-gm");
  s += `<line class="bs-hv" id="bsHv" x1="0" x2="0" y1="${T}" y2="${T + ph}" visibility="hidden"/>`;
  const ax = [0, 30, 60, 90, N - 1].filter((i) => i < N).map((i) => `<span style="left:${((L + (i + 0.5) / N * pw) / W * 100).toFixed(2)}%">${i === N - 1 ? "hoje" : bsDay(days[i].day)}</span>`).join("");
  BS.geom = { L, pw, N };
  return `<svg viewBox="0 0 ${W} ${H}" width="${W}" height="${H}" role="img" aria-label="Fundo de casa em porcentagem da referência, 120 dias">${s}</svg>` +
    `<div class="bs-tip mono" id="bsTip" hidden></div></div><div class="bs-ax mono">${ax}</div>`;
}

function bsRender() {
  const b = BS.b, card = $("slowCard");
  if (!b || !card) return;
  const [ic, t, p] = bsVerdict(b);
  const hasRef = b.refGm > 0;
  let s = `<div class="tb-head"><h2>Fundo nas últimas semanas</h2><span class="small muted">GQ GMC-800 em casa · conferido ${tbChecked(b.at)}</span></div>`;
  s += `<div class="tb-sum${ic === "warn" ? " bs-warn" : ""}">${TB_ICO[ic]}<div><b>${t}</b><p class="small muted">${p}</p></div></div>`;
  if (b.state !== "na" || b.moved) {
    const na = b.state === "na";
    s += `<div class="bs-nums">` +
      `<div class="bs-num"><span class="k">${b.moved ? "Referência (lugar novo)" : "Referência"}</span><span class="v mono">${hasRef && b.refUSv > 0 ? fmt(b.refUSv, 3) + " µSv/h" : "—"}</span><span class="d">${b.moved ? `desde ${bsDay(b.moved)}` : "mediana de 120 a 30 dias atrás"} · ${b.refN} dias</span></div>` +
      `<div class="bs-num"><span class="k">Últimas 2 semanas</span><span class="v mono">${na ? "—" : fmt(b.recUSv, 3) + " µSv/h"}</span><span class="d">${na ? "esperando a referência" : `${bsPct(b.rise)} · ${b.above} de ${b.recN} dias acima de +5%`}</span></div>` +
      `<div class="bs-num"><span class="k">RC102 em casa</span><span class="v mono">${na || !b.rcOk ? "—" : bsPct(b.rcRise)}</span><span class="d">${b.rcOk || na ? "mesma conta, com a contagem do RC102" : "sem o RC102 em casa nas duas épocas"}</span></div></div>`;
  }
  if (hasRef && b.days.some((d) => d.gm > 0)) {
    const W = Math.max(320, Math.round((card.clientWidth || 940) - 48));
    s += `<div class="bs-chart" id="bsChart">${bsChart(b, W)}`;
    s += `<div class="bs-leg small"><span><i class="sw bs-sw-gm"></i>GMC-800 (mediana de 7 dias)</span><span><i class="sw bs-sw-dot"></i>cada dia</span>${b.refRc > 0 ? '<span><i class="sw bs-sw-rc"></i>RC102 em casa</span>' : ""}<span><i class="sw bs-sw-rain"></i>chuva (fora da conta)</span><span><i class="sw bs-sw-ref"></i>referência</span><span><i class="sw bs-sw-rec"></i>últimas 2 semanas</span></div>`;
  } else {
    s += `<p class="small muted">Ainda não há nenhum dia com 12 h do GMC-800 em casa.</p>`;
  }
  s += bsTodo(b);
  s += `<div class="tb-foot">${b.state === "step" ? `<button type="button" class="btn small sug" id="btnBsStep">Mudei o aparelho de lugar em ${bsDay(b.stepDay)}</button>` : ""}${b.moved ? '<button type="button" class="btn small" id="btnBsUndo">Desfazer a marca</button>' : ""}<span class="small muted">Só dias em casa, sem chuva e sem medir objeto. Recalcula todo dia às 06:00.</span></div>`;
  card.innerHTML = s;
  // aviso informativo em Alertas, enquanto durar
  const n = $("slowNotice");
  if (n) {
    const on = b.state === "warn" || b.state === "step";
    n.hidden = !on;
    if (on) {
      const why = b.state === "step" ? "Subiu de uma vez e ficou." : !b.rcOk ? "Sem o RC102 em casa para comparar." : b.env ? "O RC102 subiu junto: é o ambiente." : "O RC102 não subiu: veja a Saúde do tubo.";
      n.innerHTML = `<strong>${t}</strong>. <span>${why}</span> <span>Não é alarme.</span> <button type="button" class="link small-link" data-slowopen="1">Abrir em Gráficos › Tendência do fundo</button>`;
    }
  }
}

function bsHover(e) {
  const sv = e.target.closest && e.target.closest("#bsChart svg"), tip = $("bsTip"), hv = $("bsHv");
  if (!tip || !hv) return;
  const b = BS.b, g = BS.geom;
  if (!sv || !b || !g) { tip.hidden = true; hv.setAttribute("visibility", "hidden"); return; }
  const r = sv.getBoundingClientRect(), fx = (e.clientX - r.left) / r.width * sv.viewBox.baseVal.width;
  const i = Math.floor((fx - g.L) / g.pw * g.N), d = b.days[i];
  if (!d) { tip.hidden = true; hv.setAttribute("visibility", "hidden"); return; }
  const xx = g.L + (i + 0.5) / g.N * g.pw;
  hv.setAttribute("x1", xx); hv.setAttribute("x2", xx); hv.setAttribute("visibility", "visible");
  let t = bsDay(d.day) + " · ";
  if (!(d.gm > 0)) t += "sem dado";
  else {
    t += `GMC-800 ${fmt(b.refUSv * d.gm / b.refGm, 3)} µSv/h (${fmt(d.gm / b.refGm * 100, 0)}%)`;
    if (d.rc > 0 && b.refRc > 0) t += ` · RC102 ${fmt(d.rc / b.refRc * 100, 0)}%`;
    if (d.rain) t += " · chuva, fora da conta";
  }
  tip.textContent = t;
  tip.hidden = false;
  const px = e.clientX - r.left;
  tip.style.left = Math.max(0, Math.min(r.width - tip.offsetWidth - 4, px + 12)) + "px";
  tip.style.top = "6px";
}

async function bsLoad(force) {
  if (!api.GetBgSlow || BS.busy) return;
  BS.busy = true;
  try { BS.b = await api.GetBgSlow(!!force); } catch (e) { /* fica o anterior */ }
  BS.busy = false;
  bsRender();
}

async function bsSet(fn) {
  try { BS.b = await fn(); bsRender(); } catch (e) { toast(errMsg(e), true); }
}

// menu do logo e busca: "Mudei o aparelho de lugar"
async function bsMarkMoved() {
  if (!api.MarkBgMoved) return;
  if (!(await askConfirm({ title: "Mudou o GMC-800 de lugar?", text: "A referência do fundo recomeça hoje. O app junta 30 dias no lugar novo antes de comparar de novo e deixa uma nota no gráfico.", ok: "Mudei de lugar" }))) return;
  await bsSet(() => api.MarkBgMoved(0));
  toast("Referência do fundo recomeça hoje");
}

function slowInit() {
  if (!api.GetBgSlow) return;
  document.addEventListener("click", async (e) => {
    if (e.target.closest("#btnBsStep") && BS.b) {
      const d = BS.b.stepDay;
      if (await askConfirm({ title: `Mudou o GMC-800 de lugar em ${bsDay(d)}?`, text: "A referência do fundo recomeça nesse dia e fica uma nota no gráfico.", ok: "Mudei de lugar" })) bsSet(() => api.MarkBgMoved(d));
    } else if (e.target.closest("#btnBsUndo")) bsSet(() => api.ClearBgMoved());
    else if (e.target.closest("[data-slowopen]")) {
      await showView("graficos");
      setSub("graficos", "tendencia");
      setTimeout(() => { const c = $("slowCard"); if (c) c.scrollIntoView({ block: "start" }); }, 50);
    }
  });
  document.addEventListener("mousemove", bsHover);
  let rz = 0;
  window.addEventListener("resize", () => { clearTimeout(rz); rz = setTimeout(bsRender, 150); });
  // o cartão nasce escondido: desenha de novo quando a subaba abre (largura certa)
  const c = $("slowCard");
  if (c && window.ResizeObserver) {
    let w = 0;
    new ResizeObserver(() => { if (c.clientWidth && Math.abs(c.clientWidth - w) > 8) { w = c.clientWidth; bsRender(); } }).observe(c);
  }
  bsLoad(false);
  setInterval(() => bsLoad(false), 10 * 60000);
}
document.addEventListener("DOMContentLoaded", slowInit);

// ---------- demonstração (navegador sem o app) ----------
// cenário pelo localStorage "fundoDemo": est, amb (padrão), apar, degrau, pouco
function bsDemo(sc, moved) {
  let seed = 7;
  const r = () => { seed = (seed * 16807) % 2147483647; return seed / 2147483647; };
  const g = () => (r() + r() + r() + r() - 2) * 1.2;
  const N = 120, today = new Date(); today.setHours(0, 0, 0, 0);
  const days = [];
  for (let i = 0; i < N; i++) {
    const rain = r() < 0.14;
    let gm = 1 + g() * 0.025, rc = 1 + g() * 0.03;
    if (sc === "amb" && i >= 84) { const f = (i - 84) / 35 * 0.15; gm += f; rc += f * 0.95; }
    if (sc === "apar" && i >= 80) gm += (i - 80) / 39 * 0.13;
    if (sc === "degrau" && i >= 92) gm += 0.095;
    if (rain) { gm += 0.08 + r() * 0.12; rc += 0.08 + r() * 0.12; }
    const day = new Date(today); day.setDate(day.getDate() - (N - 1 - i));
    const no = sc === "pouco" && i < N - 25;
    days.push({ day: day.getTime(), gm: no ? 0 : gm * 20, rc: no ? 0 : rc * 560, rain, valid: !no && !rain });
  }
  const at = new Date(); at.setHours(6, 0, 0, 0);
  const o = { at: at.getTime(), today: today.getTime(), moved: moved || 0, days, state: "na", left: 0, refN: 0, recN: 0, refGm: 0, recGm: 0, refRc: 0, recRc: 0, rise: 0, rcRise: 0, rcOk: false, above: 0, env: false, weeks: 0, stepDay: 0, stepPct: 0 };
  // a mesma conta do bgslow.go (bsAnalyze)
  const rec0 = N - 14, inRef = (i) => (o.moved ? days[i].day >= o.moved && i < rec0 : i < N - 30);
  const rg = [], rr = [], cg = [], cr = [];
  let valid = 0;
  days.forEach((d, i) => {
    if (!d.valid) return;
    valid++;
    if (i >= rec0) { cg.push(d.gm); if (d.rc) cr.push(d.rc); } else if (inRef(i)) { rg.push(d.gm); if (d.rc) rr.push(d.rc); }
  });
  Object.assign(o, { refN: rg.length, recN: cg.length, refGm: bsMed(rg), recGm: bsMed(cg), refRc: bsMed(rr), recRc: bsMed(cr) });
  const usv = (c) => c * 0.00564;
  if (o.refN < 30 || o.recN < 10) {
    o.left = Math.max(30 - o.refN, 10 - o.recN, 1);
    if (!o.moved && o.refN < 30) o.left = Math.max(o.left, 60 - valid);
    if (!(o.refGm > 0)) {
      const ok = days.filter((d) => d.valid && (!o.moved || d.day >= o.moved));
      o.refGm = bsMed(ok.map((d) => d.gm)); o.refRc = bsMed(ok.map((d) => d.rc));
    }
    o.refUSv = usv(o.refGm); o.recUSv = usv(o.recGm);
    return o;
  }
  o.rise = o.recGm / o.refGm - 1;
  o.rcOk = rr.length >= 7 && cr.length >= 5;
  if (o.rcOk) o.rcRise = o.recRc / o.refRc - 1;
  o.above = days.slice(rec0).filter((d) => d.valid && d.gm / o.refGm - 1 > 0.05).length;
  const win = (a, b) => days.slice(Math.max(a, 0), Math.min(b, N)).filter((d) => d.valid).map((d) => d.gm);
  const mean = (v) => v.reduce((s, x) => s + x, 0) / v.length;
  let best = 0, bestI = -1, bestM = 0;
  for (let i = Math.max(N - 60, 7); i < N - 7; i++) {
    const x = win(i - 7, i), y = win(i, i + 7);
    if (x.length < 4 || y.length < 4) continue;
    const j = bsMed(y) / bsMed(x) - 1, m = mean(y) / mean(x) - 1;
    if (j >= 0.08 && m > bestM) { best = j; bestI = i; bestM = m; }
  }
  o.state = o.above >= 10 && o.rise >= 0.1 ? "warn" : "ok";
  if (bestI >= 0 && o.above >= 10 && o.rise >= 0.072 && best >= 0.75 * o.rise) { o.state = "step"; o.stepDay = days[bestI].day; o.stepPct = best; }
  o.env = o.rcOk && o.rcRise >= 0.6 * o.rise;
  o.weeks = 2;
  for (let k = Math.max(N - 60, 0); k < N; k++) {
    const w = win(k - 6, k + 1);
    if (w.length >= 4 && bsMed(w) / o.refGm - 1 > 0.05) { o.weeks = Math.max(2, Math.round((N - k) / 7)); break; }
  }
  o.refUSv = usv(o.refGm); o.recUSv = usv(o.recGm);
  return o;
}
