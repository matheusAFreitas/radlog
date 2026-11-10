// Tela do aparelho (ajuste da 2.26.0, opção B aprovada pelo ACE em 10/11).
//
// Na página do GMC-800 e na do RadiaCode o valor aparece uma vez só: o
// cartão grande do aparelho tem o mostrador fazendo o papel da régua (as três
// faixas sempre à vista, apagadas, e acesas até a leitura, com um ponto na
// leitura e os limites escritos nos vãos do anel). O quadro do próprio
// aparelho sai da fileira de cima; o outro aparelho fica compacto ao lado dos
// alertas e abre a tela dele no clique. A tela Agora não muda.
//
// O mesmo desenho serve para o mostrador grande e o pequeno: cada arco do
// logo é uma faixa. Verde e amarelo são lineares entre os limites; o laranja
// vai do limite de alarme até o de perigo em escala log, senão o ponto quase
// não anda. No perigo o anel inteiro fica vermelho.

const DEV_PAGES = { gmc: "g", radiacode: "r" };

// posição no anel (0 a 1; ≥ 1 = perigo) de um valor, com os três limites
// t = [atenção, alarme, perigo] na mesma unidade
function devDialPos(v, t) {
  if (v == null || !isFinite(v) || !(t[0] > 0) || !(t[1] > t[0]) || !(t[2] > t[1])) return null;
  if (v >= t[2]) return 1.01;
  if (v < t[0]) return Math.max(0, v / t[0] / 3);
  if (v < t[1]) return (1 + (v - t[0]) / (t[1] - t[0])) / 3;
  return (2 + Math.log(v / t[1]) / Math.log(t[2] / t[1])) / 3;
}

function devSeg(i, f) {
  const p = DIAL_SEG[i];
  const a0 = Math.atan2(24 - p[1], p[0] - 24);
  let a1 = Math.atan2(24 - p[3], p[2] - 24);
  if (a1 > a0) a1 -= 2 * Math.PI;
  const ae = a0 + (a1 - a0) * f;
  const P = (a) => `${(24 + 19 * Math.cos(a)).toFixed(2)} ${(24 - 19 * Math.sin(a)).toFixed(2)}`;
  return `M${P(a0)}A19 19 0 0 1 ${P(ae)}`;
}
function devAng(i, f) {
  const p = DIAL_SEG[i];
  const a0 = Math.atan2(24 - p[1], p[0] - 24);
  let a1 = Math.atan2(24 - p[3], p[2] - 24);
  if (a1 > a0) a1 -= 2 * Math.PI;
  return a0 + (a1 - a0) * f;
}

// o desenho do mostrador. lv: nível (cor do centro); pos: devDialPos (null =
// sem leitura); labels: [[valor, nome] × 3] nos vãos; center: forma + nome do
// nível no meio (grande) ou só a forma (pequeno). A forma fica no centro
// exato do anel; o nome do nível vai embaixo dela, acima do arco de baixo
function devDialSvg(pos, lv, o = {}) {
  const w = o.w || 3.4, per = pos != null && pos >= 1;
  let s = "";
  for (let i = 0; i < 3; i++) {
    const cls = per ? "dl-perigo" : "dl-b" + i;
    s += `<path class="dl-base ${cls}${per ? " on" : ""}" d="${devSeg(i, 1)}" stroke-width="${w}"/>`;
    const f = pos == null ? 0 : Math.max(0, Math.min(1, pos * 3 - i));
    if (!per && f > 0.002) s += `<path class="dl-lit ${cls}" d="${devSeg(i, f)}" stroke-width="${w}"/>`;
  }
  if (pos != null && !per) {
    const b = Math.min(2, Math.floor(pos * 3)), f = Math.max(0, Math.min(1, pos * 3 - b)), a = devAng(b, f);
    s += `<circle class="dl-pt" cx="${(24 + 19 * Math.cos(a)).toFixed(2)}" cy="${(24 - 19 * Math.sin(a)).toFixed(2)}" r="${(w * 0.62).toFixed(2)}" stroke-width="${(w * 0.28).toFixed(2)}"/>`;
  }
  if (o.labels) {
    [-150, 90, -30].forEach((g, i) => {
      const a = g * Math.PI / 180, x = 24 + 25.4 * Math.cos(a), y = 24 - 25.4 * Math.sin(a);
      const anchor = i === 0 ? "end" : i === 2 ? "start" : "middle";
      const [v, name] = o.labels[i] || ["", ""];
      s += `<text class="dl-lv" x="${x.toFixed(2)}" y="${(y + (i === 1 ? -0.3 : 1.2)).toFixed(2)}" text-anchor="${anchor}">${escHtml(v)}</text>` +
        `<text class="dl-ln" x="${x.toFixed(2)}" y="${(y + (i === 1 ? 3.2 : 4.8)).toFixed(2)}" text-anchor="${anchor}">${escHtml(name)}</text>`;
    });
  }
  const shape = LV_PATH[lv];
  if (shape) {
    if (o.center) {
      s += `<g class="dl-c lv-ico ${lv}" fill="currentColor" transform="translate(18.4 18.4) scale(.7)">${shape}</g>`;
      if (o.name) s += `<text class="dl-name ${lv}" x="24" y="33.8" text-anchor="middle">${escHtml(o.name)}</text>`;
    } else {
      s += `<g class="dl-c lv-ico ${lv}" fill="currentColor" transform="translate(17.6 17.6) scale(.8)">${shape}</g>`;
    }
  } else if (o.center && o.name) {
    s += `<text class="dl-name" x="24" y="25.6" text-anchor="middle">${escHtml(o.name)}</text>`;
  }
  return s;
}

// valor de CPM em cada unidade (o GMC converte pelo fator de calibração)
function gmcInUnit(cpm, u) {
  const k = calK();
  return u === "cpm" ? cpm : u === "cps" ? cpm / 60 : u === "mrh" ? cpm * k / 10 : cpm * k;
}
function fmtLim(x, u) {
  if (u === "cpm") return fmt(x, 0);
  if (u === "cps") return fmt(x, x < 10 ? 2 : 1);
  if (u === "mrh") return fmt(x, x < 1 ? 3 : 2);
  return fmt(x, x < 1 ? 2 : 1);
}

// mostrador grande do GMC-800 (faixas do manual: 50 / 100 CPM; perigo no
// limite do app)
function renderGmcDial(r, lvl, word) {
  const el = $("gHeroDial");
  if (!el) return;
  const u = state.unit, t = [50, 100, dangerCPM()];
  const labels = t.map((c, i) => [fmtLim(gmcInUnit(c, u), u), ["atenção", "alarme", "perigo"][i]]);
  const pos = r ? (lvl === "perigo" ? 1.01 : devDialPos(r.cpm, t)) : null;
  el.innerHTML = devDialSvg(pos, lvl, { w: 3.4, labels, center: true, name: word });
  el.setAttribute("aria-label", word ? `Nível: ${word}` : "Mostrador");
}

// mostrador grande do RadiaCode (alarmes do aparelho; perigo no limite do
// app ou a 10× o alarme 2 enquanto não houver um limite próprio)
function rcDialLimits(st, attn, alarm, own) {
  const lim = st.limits || {};
  const danger = lim.danger > alarm ? lim.danger : st.dangerUSv > alarm && !own ? st.dangerUSv : alarm * 10;
  return [attn, alarm, danger];
}
function rcInUnit(usv, u, perUSv) {
  return u === "mrh" ? usv / 10 : u === "cps" ? (perUSv ? usv * perUSv : null) : u === "cpm" ? (perUSv ? usv * perUSv * 60 : null) : usv;
}
function renderRcDial(st, o) {
  const el = $("rHeroDial");
  if (!el) return;
  const t = rcDialLimits(st, o.attn, o.alarm, o.own);
  const u = state.rcUnit, perUSv = o.usv > 0 && o.cps > 0 ? o.cps / o.usv : null;
  const names = o.own ? ["alarme 1", "alarme 2", "perigo"] : ["atenção", "alarme", "perigo"];
  const labels = t.map((x, i) => { const v = rcInUnit(x, u, perUSv); return [v == null ? fmtLim(x, "usv") : fmtLim(v, u), names[i]]; });
  const word = o.lvl ? (o.own ? { seguro: "Seguro", atencao: "Alarme 1", alarme: "Alarme 2", perigo: "Perigo" } : LV_WORD)[o.lvl] : "";
  const pos = o.usv == null ? null : o.lvl === "perigo" ? 1.01 : devDialPos(o.usv, t);
  el.innerHTML = devDialSvg(pos, o.lvl, { w: 3.4, labels, center: true, name: word || "—" });
  el.setAttribute("aria-label", word ? `Nível: ${word}` : "Mostrador");
}

// o outro aparelho, compacto ao lado dos alertas
function renderDevMini() {
  const box = $("devMini"), page = state.view;
  if (!box) return;
  const inst = $("instruments");
  if (DEV_PAGES[page]) inst.dataset.page = page; else delete inst.dataset.page;
  // o aviso do relógio do GMC mora no quadro dele; na página do GMC o quadro
  // some, então o aviso vai para o cartão grande
  const clk = $("clockChip"), home = page === "gmc" ? $("heroClock") : $("gTileSync").parentNode;
  if (clk && home && clk.parentNode !== home) home.appendChild(clk);
  if (!DEV_PAGES[page]) { box.hidden = true; return; }
  const other = page === "gmc" ? "r" : "g";
  if (other === "r" && $("rTile").hidden) { box.hidden = true; return; }
  box.hidden = false;
  box.dataset.view = other === "g" ? "gmc" : "radiacode";
  let pos = null, lv = "", word = "—", val = "—", name, via, dose;
  if (other === "g") {
    const g = state.gNow || {}, r = state.last;
    name = "GMC-800";
    via = (state.status && state.status.port) || "";
    if (g.live && r) {
      lv = g.lvl;
      word = r.noCount ? "Sem contagens" : LV_WORD[g.lvl] || "—";
      pos = g.lvl === "perigo" ? 1.01 : devDialPos(r.cpm, [50, 100, dangerCPM()]);
      val = fmt(r.usv, usvDigits(r.usv));
    } else word = $("gTileState").textContent || "—";
    dose = state.gDose;
  } else {
    const n = state.rNow || {}, st = (state.rc && state.rc.st) || {};
    name = "RadiaCode";
    via = st.state === "conectado" ? (st.via === "bluetooth" ? "Bluetooth" : "USB") + (st.hasRare ? ` · ${fmt(st.battery, 0)}%` : "") : "";
    if (n.live && n.usv != null) {
      const al = st.alarms || {}, own = al.DoseRate1 > 0 && al.DoseRate2 > al.DoseRate1;
      const attn = own ? al.DoseRate1 : st.attnUSv || 0.505, alarm = own ? al.DoseRate2 : st.alarmUSv || 1.01;
      lv = n.lvl;
      word = (own ? { seguro: "Seguro", atencao: "Alarme 1", alarme: "Alarme 2", perigo: "Perigo" } : LV_WORD)[n.lvl] || "—";
      pos = n.lvl === "perigo" ? 1.01 : devDialPos(n.usv, rcDialLimits(st, attn, alarm, own));
      val = fmt(n.usv, usvDigits(n.usv));
    } else word = $("rTileState").textContent || "—";
    const sx = st.stats || {};
    dose = st.todayDevOK ? st.todayDev : sx.todayMinutes || st.today ? st.today : null;
  }
  box.className = "card dev-mini " + (other === "g" ? "dm-gmc" : "dm-rc") + (lv ? " lv-" + lv : "");
  box.innerHTML = `<svg class="dm-dial" viewBox="2 2 44 44" aria-hidden="true">${devDialSvg(pos, lv, { w: 4.2 })}</svg>` +
    `<span class="dm-name mono">${escHtml(name)}${via ? " · " + escHtml(via) : ""}</span>` +
    `<span class="dm-val mono">${val}<small>µSv/h</small></span>` +
    `<span class="dm-sub"><span class="dm-state"></span>${dose != null ? `<span>hoje ${fmt(dose, 2)} µSv</span>` : ""}<span class="dm-go">abrir</span></span>`;
  lvLabel(box.querySelector(".dm-state"), lv, word, 12);
  box.setAttribute("aria-label", `Abrir ${name}`);
}
