// ---------- régua de radiação: comparações para dar noção de grandeza ----------
//
// Duas escadas (taxa em µSv/h e dose em µSv), em ordem, com a leitura de
// agora e a dose do período encaixadas no meio. As referências são valores
// típicos, arredondados: servem para ter ideia da ordem de grandeza.
// Usa as funções do app.js ($, fmt, state…), que já existem quando é chamada.

const REGUA_TAXA = [
  { v: 0.09, t: "Estrada, dentro do carro", d: "mapa do RadiaCode na Via Dutra: a lataria e a distância do chão cortam parte" },
  { v: 4, t: "Avião em altitude de cruzeiro", d: "raios cósmicos; varia de ~2 a 6 com a altitude e a rota" },
  { v: 10, t: "Média permitida a quem trabalha com radiação", d: "20 mSv por ano ÷ 2.000 horas de trabalho" },
  { v: 1000, t: "1 mSv/h", d: "em 1 hora, o limite anual do público (fora o fundo natural)" },
  { v: 2900, t: "Bússola com tinta de rádio, encostada", d: "leitura do GMC-600 Pro no vídeo; boa parte é beta e o valor real no corpo é menor" },
  { v: 1e6, t: "1 Sv/h", d: "em ~1 hora, começam os efeitos agudos (enjoo de radiação)" },
  // acidentes (Gy ≈ Sv para gama; 1 R ≈ 10 mSv)
  { v: 4.56e6, t: "Goiânia, 1987: a cápsula de césio-137, a 1 metro", d: "4,56 Gy/h (relatório da AIEA); 50,9 TBq de cloreto de césio", acc: true },
  { v: 1e8, t: "Chernobyl, 1986: o \"pé de elefante\"", d: "~10.000 R/h medidos em dez/1986 na lava do reator (≈ 100 Sv/h); hoje caiu para ~1% disso", acc: true },
  { v: 5.3e8, t: "Fukushima: dentro da contenção do reator 2", d: "~530 Sv/h estimados pela TEPCO em 2017 (±30%), perto do combustível derretido", acc: true },
];

const REGUA_DOSE = [
  { v: 0.1, t: "Comer uma banana", d: "potássio-40 da banana; a \"dose de banana\" é uma unidade informal" },
  { v: 20, t: "Raio-X de tórax", d: "um exame comum, de frente" },
  { v: 1000, t: "Limite anual do público", d: "1 mSv por ano, além do fundo natural e de exames médicos" },
  { v: 7000, t: "Tomografia de tórax", d: "~7 mSv; varia com o aparelho e o protocolo" },
  { v: 20000, t: "Limite anual de quem trabalha com radiação", d: "20 mSv por ano (média de 5 anos)" },
  { v: 1e6, t: "1 Sv de uma vez", d: "começam os efeitos agudos (enjoo de radiação)" },
  // acidentes (doses altas se medem em Gy; para gama, Gy ≈ Sv)
  { v: 6.79e5, t: "Fukushima: a maior dose de um trabalhador", d: "678,8 mSv (funcionário da TEPCO, 2011); a média dos trabalhadores ficou em ~12 mSv", acc: true },
  { v: 6e6, t: "Goiânia, 1987: as 4 vítimas fatais", d: "4,5 a 6 Gy de dose estimada; a maior foi a de Leide das Neves, 6 anos", acc: true },
  { v: 1.6e7, t: "Chernobyl, 1986: bombeiros e operadores", d: "134 com síndrome aguda da radiação, com doses de menos de 2,1 até 16 Gy; 28 morreram em até 3 meses", acc: true },
];

// valores da casa e das medições salvas (o que é seu entra na escada também)
function reguaTaxaRefs() {
  const out = [...REGUA_TAXA];
  const cal = calK();
  const bg = (state.last && state.last.bgCPM) || state.bgCPM;
  if (bg) out.push({ v: bg * cal, t: "Fundo da sua casa (GMC-800)", d: "mediana dos últimos 30 dias", mine: true });
  const rs = (state.rc && state.rc.st && state.rc.st.stats) || {};
  if (rs.bgUSv > 0) out.push({ v: rs.bgUSv, t: "Fundo da sua casa (RadiaCode)", d: "mediana dos minutos em casa dos últimos 7 dias", mine: true, rc: true });
  const seen = new Set();
  for (const m of [...(state.measurements || [])].reverse()) {
    if (!m.result || m.result.level !== "sim" || !m.obj || !(m.obj.usv > 0)) continue;
    const key = m.name.toLowerCase();
    if (seen.has(key) || seen.size >= 4) continue;
    seen.add(key);
    out.push({ v: m.obj.usv, t: `${m.name}, encostado`, d: `sua medição de ${ddmm(m.time)}`, mine: true });
  }
  return out;
}

function reguaDoseRefs() {
  const out = [...REGUA_DOSE];
  const cal = calK();
  const bg = (state.last && state.last.bgCPM) || state.bgCPM;
  if (bg) {
    const h = bg * cal;
    out.push({ v: h * 24, t: "Um dia de fundo na sua casa", d: `${fmt(h, 3)} µSv/h × 24 h`, mine: true });
    out.push({ v: h * 8766, t: "Um ano de fundo na sua casa", d: `${fmt(h, 3)} µSv/h × 1 ano`, mine: true });
  }
  return out;
}

const reguaFmt = (v, unit) => {
  const per = unit === "µSv/h" ? "/h" : "";
  if (v >= 1e6) return `${fmt(v / 1e6, v >= 1e7 ? 0 : 1)} Sv${per}`;
  if (v >= 1000) return `${fmt(v / 1000, v >= 10000 ? 0 : 1)} mSv${per}`;
  return `${fmt(v, v < 0.1 ? 3 : v < 1 ? 2 : v < 10 ? 1 : 0)} ${unit}`;
};

// "≈ 3× isso" ou "≈ 1/5 disso" (relação com o item marcado)
const bigX = (x) => (x >= 1e9 ? `${fmt(x / 1e9, x >= 1e10 ? 0 : 1)} bi` : x >= 1e6 ? `${fmt(x / 1e6, x >= 1e7 ? 0 : 1)} mi` : x >= 1e4 ? `${fmt(x / 1000, 0)} mil` : fmt(x, x < 10 ? 1 : 0));
function reguaRel(v, here) {
  if (!(here > 0)) return "";
  const r = v / here;
  if (r >= 0.9 && r <= 1.1) return "≈ igual";
  if (r > 1) return `${bigX(r)}× mais`;
  return `${bigX(1 / r)}× menos`;
}

// escada: itens em ordem, com a barra em escala log e as linhas "agora" de
// cada aparelho (GMC e RadiaCode) no meio. As relações ("3× mais") são com a
// primeira delas.
function reguaLadder(el, refs, heres, unit) {
  const rows = refs.map((r) => ({ ...r })).sort((a, b) => a.v - b.v);
  heres = (heres || []).filter((h) => h.v > 0);
  const here = heres.length ? heres[0].v : null;
  for (const h of heres) rows.push({ v: h.v, t: h.t, here: true, rc: h.rc, tag: h.tag });
  rows.sort((a, b) => a.v - b.v || (a.here ? 1 : -1));
  const lo = Math.log10(Math.min(...rows.map((r) => r.v)) / 2), hi = Math.log10(Math.max(...rows.map((r) => r.v)) * 2);
  const pos = (v) => ((Math.log10(v) - lo) / (hi - lo)) * 100;
  el.textContent = "";
  for (const r of rows) {
    const li = document.createElement("li");
    li.className = "rg-row" + (r.here ? " here" : "") + (r.mine ? " mine" : "") + (r.acc ? " acc" : "") + (r.rc ? " rc" : "");
    li.innerHTML = '<span class="rg-val mono"></span><span class="rg-txt"><span class="rg-t"></span><span class="rg-d small"></span></span><span class="rg-bar" aria-hidden="true"><i></i></span><span class="rg-rel mono small"></span>';
    li.querySelector(".rg-val").textContent = reguaFmt(r.v, unit);
    li.querySelector(".rg-t").textContent = r.t;
    li.querySelector(".rg-d").textContent = r.here ? "" : r.d || "";
    li.querySelector(".rg-bar i").style.width = Math.max(2, pos(r.v)) + "%";
    li.querySelector(".rg-rel").textContent = r.here ? r.tag || "você" : reguaRel(r.v, here);
    el.appendChild(li);
  }
}

// frase curta para a dose: "≈ 1/3 de um raio-X de tórax"
function doseCompare(uSv) {
  if (!(uSv > 0)) return "";
  const parts = [];
  const xr = uSv / 20;
  if (xr >= 0.001 && xr < 1) parts.push(`≈ 1/${fmt(Math.round(1 / xr), 0)} de um raio-X de tórax`);
  else if (xr >= 1 && xr < 350) parts.push(`≈ ${fmt(xr, xr < 10 ? 1 : 0)} raios-X de tórax`);
  const tc = uSv / 7000;
  if (uSv >= 700) parts.push(`≈ ${fmt(tc, tc < 10 ? (tc < 1 ? 2 : 1) : 0)} ${tc < 1.5 ? "tomografia" : "tomografias"} de tórax`);
  return parts.slice(0, 2).join(" · ");
}

function openRegua(which) {
  const cal = calK();
  const r = state.last;
  const nowUSv = r && r.usv > 0 ? r.usv : null;
  const rc = state.rc && state.rc.st;
  const rcNow = rc && rc.state === "conectado" && rc.time && rc.usvh > 0 ? rc.usvh : null;
  reguaLadder($("rgTaxa"), reguaTaxaRefs(), [
    { v: nowUSv, t: nowUSv ? "Agora no GMC-800" : "", tag: "GMC-800" },
    { v: rcNow, t: "Agora no RadiaCode", tag: "RadiaCode", rc: true },
  ], "µSv/h");
  const d = state.dose;
  const doseHere = d && d.measuredMinutes ? d.total : null;
  const per = state.doseZoom ? "trecho do zoom" : ($("doseSeg").querySelector('[aria-pressed="true"]')?.textContent.trim() || "período");
  const lbl = d && d.measuredMinutes ? `Dose no contador (${per})` : "";
  // o cartão de dose pode estar no RadiaCode (sua dose) ou no GMC (casa):
  // entram os dois, no mesmo período
  const dRc = d && d.device === "radiacode";
  const gmcDose = dRc ? (d.otherMinutes ? d.otherTotal : null) : doseHere;
  const rcDose = dRc ? doseHere : d && d.otherMinutes ? d.otherTotal : rc && rc.today > 0 ? rc.today : null;
  const rcLbl = dRc || (d && d.otherMinutes) ? `Sua dose no RadiaCode (${per})` : "Dose hoje no RadiaCode";
  reguaLadder($("rgDose"), reguaDoseRefs(), [
    { v: gmcDose, t: `Dose no GMC-800 (${per})`, tag: "GMC-800" },
    { v: rcDose, t: rcLbl, tag: "RadiaCode", rc: true },
  ], "µSv");
  $("rgTaxaSub").textContent = nowUSv || rcNow
    ? [nowUSv ? `GMC-800 agora: ${reguaFmt(nowUSv, "µSv/h")} (calibração ${fmt(cal, 4)} µSv/h por CPM)` : "", rcNow ? `RadiaCode agora: ${reguaFmt(rcNow, "µSv/h")}` : ""].filter(Boolean).join(" · ") + "."
    : "Sem leitura agora: a escada mostra só as referências.";
  $("rgDoseSub").textContent = doseHere ? `Dose medida no período escolhido no cartão de dose: ${reguaFmt(doseHere, "µSv")}.` : "Sem dose medida no período: a escada mostra só as referências.";
  showReguaTab(which === "dose" ? "dose" : "taxa");
  $("reguaDlg").showModal();
}

function showReguaTab(t) {
  $("rgPaneTaxa").hidden = t !== "taxa";
  $("rgPaneDose").hidden = t !== "dose";
  document.querySelectorAll("#rgTabs button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.rg === t)));
}

document.addEventListener("click", (e) => {
  const b = e.target.closest("[data-regua]");
  if (b) { e.preventDefault(); openRegua(b.dataset.regua); return; }
  const t = e.target.closest("#rgTabs button");
  if (t) showReguaTab(t.dataset.rg);
});
