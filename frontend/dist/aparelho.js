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
// sem leitura); labels: [[valor, nome] × 3] nos vãos; center: mostrador
// grande (forma maior). No centro fica só a forma do nível, no centro exato
// do anel; o nome do nível vai embaixo do anel, fora do desenho.
// conn: "off" (desconectado: arcos cinza, sem parte acesa nem ponto, traço no
// meio) ou "wait" (conectando: arcos âmbar pulsando, anel tracejado no meio)
const DL_CONN_ICON = {
  off: (sw) => `<path d="M3.5 8h9" fill="none" stroke="currentColor" stroke-width="${sw * 1.4}" stroke-linecap="round"/>`,
  wait: (sw) => `<circle cx="8" cy="8" r="5.6" fill="none" stroke="currentColor" stroke-width="${sw}" stroke-linecap="round" stroke-dasharray="2.6 2.2"/>`,
};
function devDialSvg(pos, lv, o = {}) {
  const w = o.w || 3.4, conn = DL_CONN_ICON[o.conn] ? o.conn : "";
  const per = !conn && pos != null && pos >= 1;
  const tf = o.center ? "translate(15.2 15.2) scale(1.1)" : "translate(17.6 17.6) scale(.8)";
  let s = "";
  if (conn) {
    let arcs = "";
    for (let i = 0; i < 3; i++) arcs += `<path class="dl-base dl-${conn}" d="${devSeg(i, 1)}" stroke-width="${w}"/>`;
    s += conn === "wait" ? `<g class="dl-sweep">${arcs}</g>` : arcs;
  } else {
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
  }
  if (o.labels) {
    [-150, 90, -30].forEach((g, i) => {
      const a = g * Math.PI / 180, x = 24 + 25.4 * Math.cos(a), y = 24 - 25.4 * Math.sin(a);
      const anchor = i === 0 ? "end" : i === 2 ? "start" : "middle";
      const [v, name] = o.labels[i] || ["", ""];
      s += `<text class="dl-lv${conn ? " dim" : ""}" x="${x.toFixed(2)}" y="${(y + (i === 1 ? -0.3 : 1.2)).toFixed(2)}" text-anchor="${anchor}">${escHtml(v)}</text>`;
      if (!conn) s += `<text class="dl-ln" x="${x.toFixed(2)}" y="${(y + (i === 1 ? 3.2 : 4.8)).toFixed(2)}" text-anchor="${anchor}">${escHtml(name)}</text>`;
    });
  }
  if (conn) s += `<g class="dl-c dl-c-${conn}" transform="${o.center ? "translate(15.2 15.2) scale(1.1)" : "translate(18.4 18.4) scale(.7)"}">${DL_CONN_ICON[conn](o.center ? 1.5 : 1.9)}</g>`;
  else if (LV_PATH[lv]) s += `<g class="dl-c lv-ico ${lv}" fill="currentColor" transform="${tf}">${LV_PATH[lv]}</g>`;
  return s;
}

// estado de conexão de cada aparelho para a tela: "" (conectado ou demo),
// "wait" (conectando) ou "off" (desconectado, erro ou procurando)
function gmcConn() {
  if (state.demo && state.demo.active) return "";
  const st = (state.status || {}).state;
  return st === "conectado" ? "" : st === "conectando" ? "wait" : "off";
}
function rcConn(st) {
  st = st || (state.rc && state.rc.st) || {};
  return st.state === "conectado" ? "" : st.state === "conectando" ? "wait" : "off";
}
// "há 3 min", "há 2 h"
function agoTxt(t) {
  const m = Math.max(0, Math.round((Date.now() - t) / 60000));
  return m < 1 ? "menos de 1 min" : m < 60 ? `${m} min` : m < 48 * 60 ? `${Math.round(m / 60)} h` : `${Math.round(m / 1440)} d`;
}
function atTxt(t) { return isToday(t) ? hhmm(t) : whenLabel(t); }

// nome do nível embaixo do mostrador grande, na cor do nível (ou o estado da
// conexão)
function devDialState(el, conn, lv, word) {
  if (!el) return;
  el.className = "hero-dial-state " + (conn ? "conn-" + conn : lv || "");
  el.textContent = conn === "off" ? "Desconectado" : conn === "wait" ? "Conectando…" : word || "—";
}

// aviso no topo do cartão quando o aparelho não está conectado: a frase do
// erro (sem endereço), as ações e, em "detalhes", endereço e porta
function devLost(box, o) {
  if (!box) return;
  if (!o.conn) { box.hidden = true; box.innerHTML = ""; return; }
  box.hidden = false;
  box.className = "dev-lost" + (o.conn === "wait" ? " wait" : "");
  const head = o.conn === "wait" ? `Conectando ao ${o.name}…` : o.since ? `${o.name} desconectado desde ${atTxt(o.since)}` : `${o.name} desconectado`;
  let why = o.conn === "wait" ? (o.since ? `A conexão caiu às ${atTxt(o.since)}. O app tenta de novo sozinho.` : "") : o.why || "";
  why = why.charAt(0).toUpperCase() + why.slice(1);
  const acts = (o.conn === "wait" ? [o.acts[1]] : o.acts).map((a, i) =>
    `<button type="button" class="btn small${o.conn !== "wait" && i === 0 ? " pri" : ""}" data-act="${a[0]}">${escHtml(a[1])}</button>`).join("");
  box.innerHTML = `<div class="dl-h"><span class="dl-dot" aria-hidden="true"></span><span>${escHtml(head)}</span></div>` +
    (why ? `<p class="dl-why">${escHtml(why)}</p>` : "") + `<div class="dl-acts">${acts}</div>` +
    (o.conn !== "wait" && o.tech ? `<details class="dl-tech"><summary>detalhes</summary><span class="mono">${escHtml(o.tech)}</span></details>` : "");
}
// tira endereço Bluetooth (AA:BB:…) e porta (COM3, /dev/tty…) da frase
function devPlainWhy(t) {
  return String(t || "").replace(/\s*\(?\b(?:[0-9A-F]{2}[:-]){5}[0-9A-F]{2}\b\)?/gi, "").replace(/\s*\(?\b(?:COM\d+|\/dev\/\S+)\)?/g, "").replace(/\s{2,}/g, " ").trim();
}

// leitura grande quando desconectado: "Última leitura às HH:MM: valor unidade"
function devLastLine(el, t, val, unit) {
  if (!el) return;
  el.innerHTML = t && val ? `Última leitura às ${escHtml(atTxt(t))}: <b class="mono">${escHtml(val)}</b> ${escHtml(unit)}` : "Sem leituras ainda";
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
  const u = state.unit, t = [50, 100, dangerCPM()], conn = gmcConn();
  const labels = t.map((c, i) => [fmtLim(gmcInUnit(c, u), u), ["atenção", "alarme", "perigo"][i]]);
  const pos = r ? (lvl === "perigo" ? 1.01 : devDialPos(r.cpm, t)) : null;
  el.innerHTML = devDialSvg(pos, lvl, { w: 3.4, labels, center: true, conn });
  el.setAttribute("aria-label", conn ? (conn === "wait" ? "Conectando…" : "Desconectado") : word ? `Nível: ${word}` : "Mostrador");
  devDialState($("gHeroState"), conn, lvl, word);
  renderGmcConn();
}

// cartão do GMC quando não está conectado: aviso no topo, a última leitura no
// lugar do número grande e a dose "até HH:MM"
function renderGmcConn() {
  const hero = $("hero");
  if (!hero) return;
  const conn = gmcConn(), s = state.status || {}, d = s.device || {};
  hero.classList.toggle("conn-off", conn === "off");
  hero.classList.toggle("conn-wait", conn === "wait");
  const last = state.gLastSeen, since = state.gOffSince || (last && last.time) || 0;
  const port = d.port || (state.settings && state.settings.port) || "";
  const m = devPlainWhy(s.message);
  devLost($("gLost"), {
    conn, name: "GMC-800", since,
    why: s.state === "erro" && m ? m.charAt(0).toUpperCase() + m.slice(1) : "O GMC-800 não responde. O cabo USB está ligado?",
    tech: ["GMC-800", port, s.message].filter(Boolean).join(" · "),
    acts: [["gmc-retry", "Procurar de novo"], ["gmc-port", "Escolher porta"]],
  });
  if (!conn) return;
  const u = state.unit;
  devLastLine($("gLast"), last && last.time, last ? tileVals(last.usv, last.cpm, last.cps)[u] : "", UNIT_TXT[u]);
  $("emptyHint").hidden = true;
  if (since) $("tDoseSub").textContent = `até ${atTxt(since)}`;
  const via = $("heroVia");
  via.textContent = conn === "wait" ? "· conectando…" : since ? `· desconectado há ${agoTxt(since)}` : "· desconectado";
  via.className = "rc-main-via gmc-via mono conn-" + conn;
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
  const t = rcDialLimits(st, o.attn, o.alarm, o.own), conn = rcConn(st);
  const u = state.rcUnit, perUSv = o.usv > 0 && o.cps > 0 ? o.cps / o.usv : null;
  const names = o.own ? ["alarme 1", "alarme 2", "perigo"] : ["atenção", "alarme", "perigo"];
  const labels = t.map((x, i) => { const v = rcInUnit(x, u, perUSv); return [v == null ? fmtLim(x, "usv") : fmtLim(v, u), names[i]]; });
  const word = o.lvl ? (o.own ? { seguro: "Seguro", atencao: "Alarme 1", alarme: "Alarme 2", perigo: "Perigo" } : LV_WORD)[o.lvl] : "";
  const pos = o.usv == null ? null : o.lvl === "perigo" ? 1.01 : devDialPos(o.usv, t);
  el.innerHTML = devDialSvg(pos, o.lvl, { w: 3.4, labels, center: true, conn });
  el.setAttribute("aria-label", conn ? (conn === "wait" ? "Conectando…" : "Desconectado") : word ? `Nível: ${word}` : "Mostrador");
  devDialState($("rHeroState"), conn, o.lvl, word);
  renderRcConn(st);
}

// cartão do RadiaCode quando não está conectado (como o do GMC)
function renderRcConn(st) {
  const main = $("rcMain");
  if (!main) return;
  const conn = rcConn(st), since = st.lastT || 0;
  main.classList.toggle("conn-off", conn === "off");
  main.classList.toggle("conn-wait", conn === "wait");
  const via = st.via === "bluetooth" ? "Bluetooth" : st.via === "usb" ? "USB" : "";
  const why = st.state === "erro" && st.err ? devPlainWhy(st.err)
    : st.state === "off" ? "A procura está desligada em Configurações → RadiaCode."
    : st.ble ? "Não apareceu no USB nem no Bluetooth. Ele está ligado e perto? O celular não está conectado nele?"
    : "Não apareceu no USB. O cabo está ligado?";
  devLost($("rLost"), {
    conn, name: "RadiaCode", since, why,
    tech: ["RadiaCode", st.ble, via, since ? `último contato ${atTxt(since)}` : ""].filter(Boolean).join(" · "),
    acts: [["rc-retry", "Procurar de novo"], ["rc-cfg", "Configurar conexão"]],
  });
  if (!conn) return;
  const u = state.rcUnit, usv = st.lastUSv, cps = st.lastCPS;
  devLastLine($("rLast"), since, since && usv != null ? tileVals(usv, cps * 60, cps)[u] : "", UNIT_TXT[u]);
  if (since && !st.todayDevOK) $("rcDoseTodaySub").textContent = `até ${atTxt(since)}`;
  else if (since) $("rcDoseTodaySub").textContent = `do aparelho · até ${atTxt(since)}`;
  $("rcMainVia").textContent = conn === "wait" ? "· conectando…" : since ? `· desconectado há ${agoTxt(since)}` : "· desconectado";
  $("rcMainVia").className = "rc-main-via mono conn-" + conn;
}

// botões do aviso de conexão
document.addEventListener("click", async (e) => {
  const b = e.target.closest(".dev-lost [data-act]");
  if (!b) return;
  const act = b.dataset.act;
  if (act === "gmc-port") openSettings("selPort");
  else if (act === "rc-cfg") openSettings("chkRcOn");
  else if (act === "rc-retry") { try { renderRc(await call(api.ReconnectRadiacode)); } catch (_) { /* call já avisou */ } }
  else if (act === "gmc-retry") {
    let port = state.settings && state.settings.port;
    b.disabled = true;
    try {
      if (!port) port = await api.DetectPort();
      await api.Connect(port);
    } catch (err) {
      toast(String(err && err.message ? err.message : err), true);
    } finally { b.disabled = false; }
  }
});

// o outro aparelho, compacto ao lado dos alertas
function renderDevMini() {
  if (typeof whConnSync === "function") whConnSync();
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
  let pos = null, lv = "", word = "—", val = "—", name, via, dose, conn, lastT = 0, lastV = null;
  if (other === "g") {
    const g = state.gNow || {}, r = state.last;
    name = "GMC-800";
    conn = gmcConn();
    via = (state.status && state.status.port) || "";
    if (g.live && r) {
      lv = g.lvl;
      word = r.noCount ? "Sem contagens" : LV_WORD[g.lvl] || "—";
      pos = g.lvl === "perigo" ? 1.01 : devDialPos(r.cpm, [50, 100, dangerCPM()]);
      val = fmt(r.usv, usvDigits(r.usv));
    } else word = $("gTileState").textContent || "—";
    dose = state.gDose;
    const ls = state.gLastSeen;
    lastT = state.gOffSince || (ls && ls.time) || 0;
    lastV = ls ? ls.usv : null;
  } else {
    const n = state.rNow || {}, st = (state.rc && state.rc.st) || {};
    name = "RadiaCode";
    conn = rcConn(st);
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
    lastT = st.lastT || 0;
    lastV = st.lastUSv;
  }
  if (conn) { lv = ""; via = conn === "wait" ? "conectando…" : lastT ? `desconectado há ${agoTxt(lastT)}` : "desconectado"; }
  box.className = "card dev-mini " + (other === "g" ? "dm-gmc" : "dm-rc") + (lv ? " lv-" + lv : "") + (conn ? " conn-" + conn : "");
  const valHtml = conn
    ? (lastT && lastV != null ? `<span class="dm-val dm-last mono">última ${fmt(lastV, usvDigits(lastV))}<small>µSv/h às ${escHtml(atTxt(lastT))}</small></span>` : `<span class="dm-val dm-last mono">—</span>`)
    : `<span class="dm-val mono">${val}<small>µSv/h</small></span>`;
  box.innerHTML = `<svg class="dm-dial" viewBox="2 2 44 44" aria-hidden="true">${devDialSvg(pos, lv, { w: 4.2, conn })}</svg>` +
    `<span class="dm-name mono">${escHtml(name)}${via ? ` · <span class="dm-via">${escHtml(via)}</span>` : ""}</span>` + valHtml +
    `<span class="dm-sub">${conn ? "" : `<span class="dm-state"></span>${dose != null ? `<span>hoje ${fmt(dose, 2)} µSv</span>` : ""}`}<span class="dm-go">abrir</span></span>`;
  if (!conn) lvLabel(box.querySelector(".dm-state"), lv, word, 12);
  box.setAttribute("aria-label", `Abrir ${name}`);
}
