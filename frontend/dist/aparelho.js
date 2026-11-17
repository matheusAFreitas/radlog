// Tela do aparelho (ajuste da 2.26.0, opção B aprovada pelo ACE em 10/11).
//
// Na página do GMC-800 e na do Radiacode o valor aparece uma vez só: o
// cartão grande do aparelho tem o número, o nível e a régua dos limites
// (2.88.0: a régua tomou o lugar do mostrador redondo, que ficou só no
// Agora). O quadro do próprio aparelho sai da fileira de cima; o outro
// aparelho fica na faixa compacta.
//
// A escala é a mesma do mostrador: um terço para cada faixa. Verde e amarelo
// são lineares entre os limites; o laranja vai do limite de alarme até o de
// perigo em escala log, senão o ponto quase não anda.

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

// nome do nível ao lado do número grande, na cor do nível (ou o estado da
// conexão)
function devDialState(el, conn, lv, word) {
  if (!el) return;
  el.className = "hero-lv " + (conn ? "conn-" + conn : lv || "");
  el.textContent = conn === "off" ? "Desconectado" : conn === "wait" ? "Conectando…" : word || "—";
}

// 2.88.0 (avaliação A2/E2): na página do aparelho a régua dos níveis fica
// embaixo do número, no lugar do mostrador redondo. Mesma escala do
// mostrador (devDialPos): um terço para cada faixa até o perigo, que fica com
// um pedaço curto no fim. labels: [[valor, nome] × 3]; next: frase de quanto
// falta para o próximo limite.
const RG_W = 93; // % da barra até o limite de perigo
function devReguaHtml(pos, lv, labels, o = {}) {
  const conn = o.conn || "";
  const lit = conn || pos == null ? -1 : pos >= 1 ? 3 : Math.min(2, Math.floor(pos * 3));
  const segs = ["seguro", "atencao", "alarme", "perigo"].map((c, i) =>
    `<i class="dr-s dr-${c}${i === lit ? " on" : ""}" style="flex:${i < 3 ? RG_W / 3 : 100 - RG_W}"></i>`).join("");
  const x = conn || pos == null ? null : pos >= 1 ? RG_W + (100 - RG_W) / 2 : pos * RG_W;
  const pt = x == null ? "" : `<b class="dr-pt" style="left:${x.toFixed(2)}%"></b>`;
  const lab = (labels || []).map(([v, name], i) =>
    `<span class="dr-l dr-l${i}" style="left:${((i + 1) * RG_W / 3).toFixed(2)}%">${escHtml(name)} <b class="mono">${escHtml(v)}</b></span>`).join("");
  return `<div class="dr-bar${conn ? " off" : ""}">${segs}${pt}</div><div class="dr-labs">${lab}</div>` +
    (o.next && !conn ? `<p class="dr-next">${escHtml(o.next)}</p>` : "");
}
// "falta 0,28 µSv/h para a atenção": t e v na mesma unidade (linear)
function devNext(v, t, f) {
  if (v == null || !isFinite(v) || !(t[0] > 0)) return "";
  if (v < t[0]) return `falta ${f(t[0] - v)} para a atenção`;
  if (v < t[1]) return `falta ${f(t[1] - v)} para o alarme`;
  if (v < t[2]) return `falta ${f(t[2] - v)} para o perigo`;
  return "";
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

// os três níveis do app (Configurações › Níveis e alarmes) em CPM pela
// calibração atual: são eles que disparam os avisos do GMC-800
function gmcThrCPM() {
  const k = calK();
  return [attnUSv() / k, alarmUSv() / k, dangerUSv() / k];
}
// mostrador grande do GMC-800, com os níveis do app
function renderGmcDial(r, lvl, word) {
  const el = $("gHeroRegua");
  if (!el) return;
  const u = state.unit, t = gmcThrCPM(), conn = gmcConn();
  const labels = t.map((c, i) => [fmtLim(gmcInUnit(c, u), u), ["atenção", "alarme", "perigo"][i]]);
  const pos = r ? (lvl === "perigo" ? 1.01 : devDialPos(r.cpm, t)) : null;
  const next = r ? devNext(r.cpm, t, (d) => `${fmtLim(gmcInUnit(d, u), u)} ${UNIT_TXT[u]}`) : "";
  el.innerHTML = devReguaHtml(pos, lvl, labels, { conn, next });
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
// mostrador grande do Radiacode (alarmes do aparelho; perigo no limite do
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
  const el = $("rHeroRegua");
  if (!el) return;
  const t = rcDialLimits(st, o.attn, o.alarm, o.own), conn = rcConn(st);
  const u = state.rcUnit, perUSv = o.usv > 0 && o.cps > 0 ? o.cps / o.usv : null;
  // 2.88.0 (R2): os mesmos nomes do GMC-800; os alarmes 1 e 2 do aparelho
  // fazem o papel de atenção e alarme (a legenda embaixo diz de onde vêm)
  const inU = (x) => { const v = rcInUnit(x, u, perUSv); return v == null ? `${fmtLim(x, "usv")} µSv/h` : `${fmtLim(v, u)} ${UNIT_TXT[u]}`; };
  const labels = t.map((x, i) => { const v = rcInUnit(x, u, perUSv); return [v == null ? fmtLim(x, "usv") : fmtLim(v, u), ["atenção", "alarme", "perigo"][i]]; });
  const word = o.lvl ? LV_WORD[o.lvl] : "";
  const pos = o.usv == null ? null : o.lvl === "perigo" ? 1.01 : devDialPos(o.usv, t);
  const next = o.usv == null ? "" : devNext(o.usv, t, inU);
  el.innerHTML = devReguaHtml(pos, o.lvl, labels, { conn, next });
  devDialState($("rHeroState"), conn, o.lvl, word);
  renderRcConn(st);
}

// cartão do Radiacode quando não está conectado (como o do GMC)
function renderRcConn(st) {
  const main = $("rcMain");
  if (!main) return;
  const conn = rcConn(st), since = st.lastT || 0;
  main.classList.toggle("conn-off", conn === "off");
  main.classList.toggle("conn-wait", conn === "wait");
  const via = st.via === "bluetooth" ? "Bluetooth" : st.via === "usb" ? "USB" : "";
  const why = st.state === "erro" && st.err ? devPlainWhy(st.err)
    : st.state === "off" ? "A procura está desligada em Configurações → Radiacode 102."
    : st.ble ? "Não apareceu no USB nem no Bluetooth. Ele está ligado e perto? O celular não está conectado nele?"
    : "Não apareceu no USB. O cabo está ligado?";
  devLost($("rLost"), {
    conn, name: "RC102", since, why,
    tech: ["RC102", st.ble, via, since ? `último contato ${atTxt(since)}` : ""].filter(Boolean).join(" · "),
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

// abre e fecha um quadro da faixa compacta
function instOpen(tile, on) {
  tile.classList.toggle("open", on);
  const b = tile.querySelector(".inst-open");
  if (b) { b.setAttribute("aria-expanded", String(on)); b.setAttribute("aria-label", on ? "fechar o quadro" : "abrir o quadro"); }
}
// na faixa, um clique no quadro fechado abre ele ali mesmo (não leva para a
// página do aparelho); os botões de dentro continuam com a ação deles
document.addEventListener("click", (e) => {
  const inst = $("instruments");
  if (!inst || !("strip" in inst.dataset)) return;
  const tile = e.target.closest(".inst");
  if (!tile || !inst.contains(tile)) return;
  const ob = e.target.closest(".inst-open");
  if (ob) { e.stopPropagation(); instOpen(tile, !tile.classList.contains("open")); return; }
  if (tile.classList.contains("open") || e.target.closest("button, a, summary, input")) return;
  e.stopPropagation();
  instOpen(tile, true);
}, true);

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

// fileira de cima nas telas dos aparelhos
function renderDevMini() {
  if (typeof whConnSync === "function") whConnSync();
  const page = state.view;
  // 2.89.0 (E6): o Clima só aparece na barra com o clima ligado
  const nc = $("navChuva");
  if (nc) nc.hidden = !(state.settings && state.settings.rainEnabled) && page !== "chuva";
  const inst = $("instruments");
  if (!inst) return;
  // na tela de um aparelho, na aba Agora, a fileira de cima é a mesma do
  // Agora, só sem o quadro do aparelho aberto (o valor dele já está no cartão
  // grande); nas outras abas mostra os três quadros (ACE, 11/11)
  const own = DEV_PAGES[page] && (!state.sub || state.sub[page] === "agora");
  if (own) inst.dataset.page = page; else delete inst.dataset.page;
  inst.dataset.devpage = DEV_PAGES[page] ? page : "";
  // 2.88.0 (avaliação E1): fora do Agora os quadros viram a faixa compacta;
  // trocar de página fecha o que estava aberto
  const strip = page !== "agora";
  if (strip !== ("strip" in inst.dataset) || inst._page !== page) {
    inst._page = page;
    if (strip) inst.dataset.strip = ""; else delete inst.dataset.strip;
    inst.querySelectorAll(".inst.open").forEach((t) => instOpen(t, false));
  }
  // o aviso do relógio do GMC mora no quadro dele; na página do GMC o quadro
  // some, então o aviso vai para o cartão grande
  const clk = $("clockChip"), home = own && page === "gmc" ? $("heroClock") : $("gTileSync").parentNode;
  if (clk && home && clk.parentNode !== home) home.appendChild(clk);
}
