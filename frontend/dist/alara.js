// ---------- ALARA (2.43) ----------
//
// "Tão baixo quanto razoavelmente possível". A tela mostra de onde vem a sua
// dose (GetDoseOrigin) e deixa testar os três jeitos de baixar: tempo (conta
// na tela), distância (quatro pontos, ajuste k/r²) e blindagem (antes ×
// depois). Os testes usam o motor do Medir objeto com o fundo da casa: cada
// distância, ou cada fase da blindagem, é uma fase do objeto
// (StartMeasure + MeasureAgainAt); as contas vêm em derived.dist e
// derived.shield (alara.go). Usa os globais do app.js; carregado antes dele.

const AL = { test: "", dists: [], step: 0, err: "", dev: "radiacode", busy: false };
const alNum = (v, d) => fmt(v, d == null ? usvDigits(Math.abs(v)) : d);
const alEsc = (t) => String(t).replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c]);

function alDev() {
  const v = localGet("al-dev", "radiacode");
  return v === "gmc" ? "gmc" : "radiacode";
}

// ---------- de onde vem a sua dose ----------

async function refreshAlOrigin() {
  if (!api.GetDoseOrigin) return;
  let o;
  try { o = await api.GetDoseOrigin(7); } catch (e) { return; }
  const dev = { radiacode: "RC102", gmc: "GMC-800", misto: "RC102 e GMC-800" }[o.device] || "";
  $("alOrSub").textContent = dev ? `Últimos 7 dias · ${dev} com você` : "Últimos 7 dias · nenhum aparelho com você";
  const [tv, tu] = agSv(o.minutes ? o.total : null);
  $("alOrTot").innerHTML = "";
  $("alOrTot").append(tv, " ", Object.assign(document.createElement("small"), { textContent: tu ? `${tu} na semana` : "" }));
  const parts = [["Fundo em casa", o.homeBg, "var(--accent)"], ["Fundo fora de casa", o.awayBg, "var(--away)"], ["Acima do fundo", o.above, "var(--obj)"]];
  const tot = parts.reduce((a, p) => a + p[1], 0) || 1;
  let x = 0, h = "";
  parts.forEach(([t, v, c], i) => {
    const w = (v / tot) * 600;
    if (v > 0) h += `<rect x="${x.toFixed(1)}" y="4" width="${Math.max(2, w - (i < 2 ? 2 : 0)).toFixed(1)}" height="28" rx="4" fill="${c}"><title>${alEsc(t)}: ${alNum(v, 2)} µSv</title></rect>`;
    x += w;
  });
  $("alOrBar").innerHTML = h;
  const leg = $("alOrLeg");
  leg.textContent = "";
  for (const [t, v, c] of parts) {
    const s = document.createElement("span");
    s.innerHTML = `<i style="background:${c}"></i>`;
    const b = document.createElement("b");
    b.className = "mono";
    b.textContent = `${alNum(v, v < 10 ? 2 : 1)} µSv`;
    s.append(Object.assign(document.createElement("span"), { textContent: t }), " · ", b, ` (${fmt((v / tot) * 100, 0)}%)`);
    leg.appendChild(s);
  }
  const tb = $("alOrTbl").tBodies[0];
  tb.textContent = "";
  if (!o.items.length) {
    const tr = tb.insertRow();
    const td = tr.insertCell();
    td.colSpan = 3;
    td.className = "muted";
    td.textContent = "Nenhum trecho acima do fundo nesses dias: tudo foi fundo natural.";
  }
  for (const it of o.items) {
    const tr = tb.insertRow();
    tr.className = "al-click";
    tr.title = "abrir nos gráficos";
    tr.insertCell().textContent = it.label;
    const wd = new Date(it.from).toLocaleDateString(LOCALE, { weekday: "short" }).replace(".", "");
    tr.insertCell().textContent = `${wd} ${hhmm(it.from)} · ${fmtDur(it.to - it.from)}`;
    const d = tr.insertCell();
    d.className = "n";
    d.textContent = `${alNum(it.dose, it.dose < 0.1 ? 3 : 2)} µSv`;
    tr.addEventListener("click", () => openGapOnChart(it));
  }
  AL.bg = o.homeBgRate || homeBgUSv();
  renderAlTempo();
}

// ---------- tempo ----------

// taxa inicial: a última medição de objeto que achou algo, ou a leitura de agora
function alRateDefault() {
  const ms = [...(state.measurements || [])].reverse().find((m) => m.result && m.result.level === "sim" && m.obj && m.obj.usv > 0);
  if (ms) return [ms.obj.usv, `taxa da medição "${ms.name}" (${ddmm(ms.time)})`];
  const r = (state.rNow && state.rNow.live && state.rNow.usv) || (state.gNow && state.gNow.live && state.gNow.usv);
  if (r > 0) return [r, "taxa da leitura de agora"];
  return [0.5, "taxa de exemplo"];
}

function renderAlTempo() {
  const inp = $("alRate");
  if (!inp.dataset.set) {
    const [r, from] = alRateDefault();
    inp.value = String(Math.round(r * 1000) / 1000);
    $("alRateFrom").textContent = from;
  }
  const r = Math.max(0, parseFloat(String(inp.value).replace(",", ".")) || 0);
  const m = Math.max(1, Math.min(6000, parseFloat(String($("alMin").value).replace(",", ".")) || 1));
  const bg = AL.bg || homeBgUSv();
  const d = (r * m) / 60;
  const [dv, du] = agSv(d);
  $("alTDose").innerHTML = "";
  $("alTDose").append(dv, Object.assign(document.createElement("small"), { textContent: " " + du }));
  const eq = $("alTEq");
  eq.textContent = "";
  for (const l of doseKnown(d, bg)) eq.appendChild(Object.assign(document.createElement("span"), { textContent: l }));
  const alt = $("alTAlt");
  alt.textContent = "";
  if (bg > 0 && r > bg) {
    const net = r - bg, d2 = ((bg + net / 4) * m) / 60, d4 = ((bg + net / 16) * m) / 60;
    const b = document.createElement("b");
    b.append(Object.assign(document.createElement("span"), { textContent: `Se a fonte for pequena e você ficar ao dobro da distância: ${alNum(d2, d2 < 1 ? 3 : 2)} µSv.` }), " ", Object.assign(document.createElement("span"), { textContent: `Ao quádruplo: ${alNum(d4, d4 < 1 ? 3 : 2)} µSv.` }));
    const p = document.createElement("span");
    p.append(Object.assign(document.createElement("span"), { textContent: "Pelo inverso do quadrado, só a parte acima do fundo cai." }), " ", Object.assign(document.createElement("span"), { textContent: "O fundo continua o mesmo onde você estiver na casa." }));
    alt.append(b, p);
  } else if (bg > 0) {
    alt.appendChild(Object.assign(document.createElement("span"), { textContent: `Essa taxa está no nível do fundo da sua casa (${alNum(bg)} µSv/h): não há fonte para afastar.` }));
  }
}

// ---------- testes (distância e blindagem) ----------

function alDists() {
  const v = String($("alDists").value || "").split(/[;,\s]+/).map((x) => parseFloat(x.replace(",", "."))).filter((x) => x > 0 && x <= 500);
  return v.length >= 2 ? v.slice(0, 8) : [5, 10, 20, 40];
}

async function alStart(test) {
  AL.err = "";
  const dev = alDev();
  try {
    if (test === "distancia") {
      AL.dists = alDists();
      const d = AL.dists[0];
      await api.StartMeasure({ name: "Teste de distância", distance: `${fmt(d, 0)} cm`, distCm: d, size: "pequeno", homeBg: true, auto: false, seconds: 300, relErr: 0.1, test, device: dev, mode: "" });
    } else {
      const mat = String($("alMat").value || "").trim() || "material";
      const min = Math.max(0.5, Math.min(30, parseFloat(String($("alBMin").value).replace(",", ".")) || 2));
      await api.StartMeasure({ name: `Blindagem: ${mat}`, distance: "", distCm: 0, homeBg: true, auto: false, seconds: Math.round(min * 60), test, material: mat, device: dev, mode: "" });
    }
    AL.test = test;
  } catch (e) {
    AL.err = errMsg(e);
    AL.errTest = test;
  }
  alTick();
}

async function alNext() {
  const st = await api.GetMeasure();
  try {
    if (st.test === "distancia") {
      const n = (st.prev || []).length + 1;
      const d = AL.dists[n] || AL.dists[AL.dists.length - 1];
      await api.MeasureAgainAt(`${fmt(d, 0)} cm`, d);
    } else await api.MeasureAgainAt("", 0);
  } catch (e) { AL.err = errMsg(e); AL.errTest = st.test; }
  alTick();
}

async function alCancel() {
  try { await api.CancelMeasure(); } catch (e) { /* já parou */ }
  AL.test = "";
  alTick();
}

let alTickBusy = false;
async function alTick() {
  if (state.view !== "alara" || alTickBusy) return;
  alTickBusy = true;
  try {
    const st = await api.GetMeasure();
    renderAlTest("distancia", st);
    renderAlTest("blindagem", st);
  } finally { alTickBusy = false; }
}

// a parte de cada teste: botões, andamento, gráfico e resultado
function renderAlTest(test, st) {
  const P = test === "distancia" ? "alD" : "alB";
  const mine = st && st.test === test && st.phase;
  const other = st && st.phase && st.phase !== "pronto" && !mine;
  const running = mine && (st.phase === "objeto" || st.phase === "fundo");
  const done = mine && st.phase === "pronto";
  const nPrev = mine ? (st.prev || []).length : 0;
  const total = test === "distancia" ? (AL.dists.length || alDists().length) : 2;
  const more = done && nPrev + 1 < total;
  $(P + "Start").hidden = running || more;
  $(P + "Start").textContent = done ? "Medir de novo" : "Começar";
  $(P + "Start").disabled = !!other;
  $(P + "Next").hidden = !more;
  $(P + "Cancel").hidden = !(running || more);
  $(P + "Stop").hidden = !running;
  // passos
  const steps = $(P + "Steps");
  steps.textContent = "";
  const labels = test === "distancia" ? (AL.dists.length ? AL.dists : alDists()).map((d) => `${fmt(d, 0)} cm`) : ["1. sem nada", "2. com o material no meio"];
  labels.concat(test === "distancia" ? ["resultado"] : ["3. resultado"]).forEach((l, i) => {
    const s = document.createElement("span");
    s.textContent = l;
    const idx = !mine ? -1 : done ? nPrev + 1 : nPrev;
    if (mine && (i < idx || (i === idx && running))) s.className = "on";
    if (mine && i === idx && running) s.classList.add("now");
    if (done && !more && i === labels.length) s.className = "on";
    steps.appendChild(s);
  });
  // andamento
  const msg = $(P + "Msg");
  msg.className = "al-msg small";
  if (AL.err && AL.errTest === test) { msg.textContent = AL.err; msg.classList.add("warn"); }
  else if (other && st.test) msg.textContent = test === "distancia" ? "O teste de blindagem está em andamento. Termine ou cancele ele antes de começar este." : "O teste de distância está em andamento. Termine ou cancele ele antes de começar este.";
  else if (other) msg.textContent = "Outra medição está em andamento (Medir objeto). Termine ou cancele por lá antes de começar o teste.";
  else if (running) {
    const lv = st.live, rel = lv && lv.netCPM > 0 ? (lv.netErr / lv.netCPM) * 100 : null;
    const head = test === "distancia" ? `Medindo a ${st.distance}: ${fmtMin(st.elapsed)} de no máximo ${fmtMin(st.seconds)}`
      : `${nPrev ? "Com o material no meio" : "Sem nada no meio"}: ${fmtMin(st.elapsed)} de no máximo ${fmtMin(st.seconds)}`;
    msg.textContent = head +
      (test === "distancia" ? (rel != null ? ` · incerteza ${fmt(rel, 0)}% (para em 10%)` : " · esperando o sinal subir acima do fundo") : "");
  } else if (more) {
    msg.textContent = test === "distancia"
      ? `Pronto. Ponha a fonte a ${fmt(AL.dists[nPrev + 1], 0)} cm do centro do cristal, na mesma linha, e clique no botão abaixo.`
      : "Pronto. Ponha o material entre a fonte e o aparelho sem mexer em mais nada, e clique no botão abaixo.";
    $(P + "Next").textContent = test === "distancia" ? `Medir a ${fmt(AL.dists[nPrev + 1], 0)} cm` : "Medir com o material";
  } else if (done) msg.textContent = "Teste salvo em Medições salvas.";
  else msg.textContent = test === "distancia" ? "Use uma fonte pequena (pedra, relógio, a camisa de lampião). O fundo é o da sua casa na última hora." : "Escolha um material e deixe a fonte e o aparelho fixos nas duas fases.";
  // resultado
  const d = done && st.derived ? (test === "distancia" ? st.derived.dist : st.derived.shield) : null;
  const ver = $(P + "Ver");
  ver.hidden = !(d && d.title && !more);
  if (!ver.hidden) {
    ver.className = "al-verdict" + ((test === "distancia" ? d.verdict === "seguiu" : d.seen && d.cut > 0) ? " ok" : " meh");
    ver.textContent = "";
    const txt = document.createElement("span");
    String(d.text || "").split(/(?<=\.)\s+(?=[A-ZÀ-Ú])/).forEach((x, i) => { if (i) txt.append(" "); txt.append(Object.assign(document.createElement("span"), { textContent: x })); });
    ver.append(Object.assign(document.createElement("b"), { textContent: d.title }), txt);
  }
  if (test === "distancia") renderAlDistChart(mine ? st : null);
  else renderAlShieldChart(d && !more ? d : null, mine ? st : null);
}

function renderAlDistChart(st) {
  const svg = $("alDChart"), tb = $("alDTbl").tBodies[0];
  const f = st && st.derived && st.derived.dist;
  let pts = f ? f.points : [];
  if (st && !f) pts = (st.prev || []).map((p) => ({ cm: p.distCm, netUSv: p.netUSv, errUSv: p.errUSv, seconds: p.seconds }));
  const bg = (st && st.bg && st.bg.usv) || AL.bg || homeBgUSv();
  tb.textContent = "";
  $("alDBox").hidden = !pts.length;
  if (!pts.length) { svg.innerHTML = ""; return; }
  const W = 640, H = 280, L = 56, R = 16, T = 18, B = 34;
  const xmax = Math.max(...pts.map((p) => p.cm), ...(AL.dists.length ? AL.dists : [40])) * 1.12;
  const k = f && f.k > 0 ? f.k : 0;
  const ymax = Math.max(...pts.map((p) => p.netUSv + p.errUSv), k ? k / Math.min(...pts.map((p) => p.cm)) ** 2 : 0, bg) * 1.12 || 1;
  const X = (r) => L + (r / xmax) * (W - L - R), Y = (v) => T + (1 - Math.max(0, v) / ymax) * (H - T - B);
  let h = "";
  for (let i = 0; i <= 4; i++) {
    const v = (ymax * i) / 4, y = Y(v);
    h += `<line x1="${L}" x2="${W - R}" y1="${y.toFixed(1)}" y2="${y.toFixed(1)}" stroke="var(--chart-grid)"/><text x="${L - 8}" y="${(y + 4).toFixed(1)}" class="al-ax" text-anchor="end">${alNum(v, v < 1 ? 2 : 1)}</text>`;
  }
  const step = xmax > 100 ? 50 : xmax > 50 ? 20 : 10;
  for (let r = 0; r <= xmax; r += step) h += `<text x="${X(r).toFixed(1)}" y="${H - B + 18}" class="al-ax" text-anchor="middle">${r} cm</text>`;
  h += `<line x1="${L}" x2="${W - R}" y1="${Y(bg).toFixed(1)}" y2="${Y(bg).toFixed(1)}" stroke="var(--dim)" stroke-width="1.5" stroke-dasharray="5 5"/>`;
  if (k) {
    let c = "";
    for (let r = Math.max(1, Math.min(...pts.map((p) => p.cm)) * 0.8); r <= xmax; r += xmax / 200) c += (c ? "L" : "M") + X(r).toFixed(1) + " " + Y(Math.min(ymax * 1.5, k / (r * r))).toFixed(1);
    h += `<clipPath id="alClip"><rect x="${L}" y="${T}" width="${W - L - R}" height="${H - T - B}"/></clipPath><path d="${c}" fill="none" stroke="var(--muted)" stroke-width="2" clip-path="url(#alClip)"/>`;
  }
  for (const p of pts) {
    const x = X(p.cm).toFixed(1);
    h += `<line x1="${x}" x2="${x}" y1="${Y(p.netUSv + p.errUSv).toFixed(1)}" y2="${Y(p.netUSv - p.errUSv).toFixed(1)}" stroke="var(--rc)" stroke-width="2"/><circle cx="${x}" cy="${Y(p.netUSv).toFixed(1)}" r="5" fill="var(--rc)" stroke="var(--surface)" stroke-width="2"><title>${fmt(p.cm, 0)} cm · ${alNum(p.netUSv, 3)} ± ${alNum(p.errUSv, 3)} µSv/h</title></circle>`;
  }
  svg.innerHTML = h;
  for (const p of pts) {
    const tr = tb.insertRow();
    tr.insertCell().textContent = `${fmt(p.cm, 0)} cm`;
    const a = tr.insertCell(); a.className = "n"; a.textContent = `${alNum(p.netUSv, 3)} ± ${alNum(p.errUSv, 3)}` + (p.lost ? " · sumiu no fundo" : "");
    const b = tr.insertCell(); b.className = "n"; b.textContent = p.predict != null ? alNum(p.predict, 3) : "—";
    const c = tr.insertCell(); c.className = "n"; c.textContent = fmtMin(p.seconds || 0);
  }
}

function renderAlShieldChart(s, st) {
  const svg = $("alBChart");
  const rows = [];
  if (s) {
    rows.push(["sem nada", s.beforeUSv, s.beforeErr, "var(--rc)"], [`com ${s.material || "o material"}`, s.afterUSv, s.afterErr, "var(--obj)"]);
  } else if (st && (st.prev || []).length) {
    const a = st.prev[st.prev.length - 1];
    rows.push(["sem nada", a.netUSv, a.errUSv, "var(--rc)"]);
  }
  $("alBBox").hidden = !rows.length;
  if (!rows.length) { svg.innerHTML = ""; return; }
  const W = 640, L = 150, R = 70, rh = 34, H = rows.length * (rh + 16) + 14;
  const max = Math.max(...rows.map((r) => r[1] + r[2])) * 1.1 || 1;
  const X = (v) => L + (Math.max(0, v) / max) * (W - L - R);
  let h = "";
  rows.forEach(([t, v, e, c], i) => {
    const y = 8 + i * (rh + 16);
    h += `<text x="${L - 10}" y="${y + rh / 2 + 4}" class="al-lb" text-anchor="end">${alEsc(t)}</text>`;
    h += `<rect x="${L}" y="${y}" width="${Math.max(2, X(v) - L).toFixed(1)}" height="${rh}" rx="4" fill="${c}" fill-opacity=".75"/>`;
    h += `<line x1="${X(v - e).toFixed(1)}" x2="${X(v + e).toFixed(1)}" y1="${y + rh / 2}" y2="${y + rh / 2}" stroke="var(--text)" stroke-width="2"/>`;
    h += `<text x="${(X(v + e) + 8).toFixed(1)}" y="${y + rh / 2 + 4}" class="al-ax">${alNum(v, 3)}</text>`;
  });
  svg.setAttribute("viewBox", `0 0 ${W} ${H}`);
  svg.innerHTML = h;
}

// ---------- página ----------

async function refreshAlara() {
  if (state.view !== "alara") return;
  const dev = alDev();
  document.querySelectorAll("#alDevSeg button, #alBDevSeg button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.aldev === dev)));
  await Promise.allSettled([refreshAlOrigin(), alTick()]);
}

function wireAlara() {
  if (!$("pageAlara")) return;
  $("alRate").addEventListener("input", () => { $("alRate").dataset.set = "1"; $("alRateFrom").textContent = "taxa escrita à mão"; renderAlTempo(); });
  $("alMin").addEventListener("input", renderAlTempo);
  $("alDStart").addEventListener("click", () => alStart("distancia"));
  $("alBStart").addEventListener("click", () => alStart("blindagem"));
  $("alDNext").addEventListener("click", alNext);
  $("alBNext").addEventListener("click", alNext);
  $("alDCancel").addEventListener("click", alCancel);
  $("alBCancel").addEventListener("click", alCancel);
  $("alDStop").addEventListener("click", async () => { await api.StopMeasure(); alTick(); });
  $("alBStop").addEventListener("click", async () => { await api.StopMeasure(); alTick(); });
  document.addEventListener("click", (e) => {
    const b = e.target.closest("#alDevSeg button, #alBDevSeg button");
    if (!b) return;
    localSet("al-dev", b.dataset.aldev);
    refreshAlara();
  });
  setInterval(alTick, 1000);
}
document.addEventListener("DOMContentLoaded", wireAlara);
