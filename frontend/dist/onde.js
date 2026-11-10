// Onde está cada aparelho (2.25): o cartão da tela Agora com um seletor por
// aparelho (em casa / comigo / fora do cálculo) e o estado ao vivo de cada
// um (conectado, desconectado, saturando, sem contagens). Mudar um seletor
// não vale na hora: abre uma linha com o que vai mudar, nome (ou motivo) e
// Confirmar. Os trechos ficam na página Fora de casa (fora.js). Carregado
// antes do app.js.

let whPending = null; // { gmc, radiacode } esperando o Confirmar
let whWhen = "now";   // tirar do cálculo: "now" ou "last" (última leitura boa)

const WH_ICON_HOME = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3 11l9-7 9 7"/><path d="M5 10v10h14V10"/></svg>';
const WH_ICON_ME = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" aria-hidden="true"><circle cx="12" cy="6" r="2.6"/><path d="M8 21l2-7-2-3 4-2 4 2-2 3 2 7"/></svg>';

const whCur = () => { const w = state.where || {}; return { gmc: w.gmc || "casa", radiacode: w.radiacode || "comigo", notHome: !!w.notHome }; };
const whIsNormal = (w) => w.gmc === "casa" && w.radiacode === "comigo";
// o estado que vem do app (GetWhereabouts) é conferido a cada 30 s; a conexão
// em si vem na hora pelos eventos de status, e manda: desconectado não fica
// "conectado" só porque o último minuto gravado ainda é recente
function whConn(dev) { return dev === "gmc" ? gmcConn() : rcConn(); }
function whLive(dev) {
  const l = ((state.where && state.where.live) || {})[dev] || { connected: false, state: "sem dado" };
  const c = whConn(dev);
  if (c === "wait") return { ...l, connected: false, state: "conectando" };
  if (c === "off" && l.state !== "sem dado") return { ...l, connected: false, state: "sem dado" };
  return l;
}
// quando a conexão de um aparelho muda, o quadro é refeito na hora
let whConnKey = null;
function whConnSync() {
  const k = whConn("gmc") + "|" + whConn("radiacode");
  if (k === whConnKey) return;
  const first = whConnKey === null;
  whConnKey = k;
  if (!first && !whPending) refreshWhere();
}
// aparelhos que estão saindo do cálculo agora (pedem motivo e "desde quando")
const whNewOff = (p) => { const c = whCur(); return ["gmc", "radiacode"].filter((d) => p[d] === "fora" && c[d] !== "fora"); };

// estado ao vivo, para a linha embaixo do nome
function whHealth(dev, w) {
  if (w[dev] === "fora") return { cls: "out", t: "fora do cálculo" };
  const l = whLive(dev);
  if (l.state === "sem contagens") return { cls: "zero", t: `sem contagens há ${hm(Math.max(1, Math.round((Date.now() - l.since) / 6e4)))}` };
  if (l.state === "saturado") return { cls: "sat", t: "saturando" };
  if (l.state === "lendo") return { cls: "", t: "conectado" };
  if (l.state === "conectando") return { cls: "wait", t: "conectando…" };
  return { cls: "nosig", t: l.last ? `desconectado desde ${hhmm(l.last)}` : "desconectado" };
}
const whMeasuring = (dev) => { const s = whLive(dev).state; return s === "lendo" || s === "saturado"; };

// quem faz um papel ("casa" ou "comigo"), olhando lugar e saúde
function whRole(place, w) {
  const pref = place === "casa" ? ["gmc", "radiacode"] : ["radiacode", "gmc"];
  const here = pref.filter((d) => w[d] === place);
  // os dois em casa: você também estava, a dose pessoal usa a leitura de casa
  if (place === "comigo" && !here.length && w.gmc === "casa" && w.radiacode === "casa" && !w.notHome) {
    const h = whRole("casa", w);
    if (h.src) return { ...h, homeRead: true };
  }
  if (!here.length) return { src: "", why: "ninguém" };
  const m = here.filter(whMeasuring);
  if (m.length) return { src: m[0], both: place === "casa" && m.length === 2, sat: whLive(m[0]).state === "saturado", covers: m[0] !== here[0] ? here[0] : null };
  // o Radiacode longe do PC continua somando a dose sozinho
  if (place === "comigo" && here.includes("radiacode") && ["sem dado", "conectando"].includes(whLive("radiacode").state)) return { src: "radiacode", curve: true };
  return { src: "", why: "sem dado", dev: here[0] };
}

// as duas frases: de onde vem a casa e a sua dose
function whLines(w) {
  const hr = whRole("casa", w), pr = whRole("comigo", w);
  const both = w.gmc === "comigo" && w.radiacode === "comigo";
  const satTxt = "Saturando: conta como pelo menos o valor lido (≥).";
  const home = !hr.src
    ? (hr.why === "ninguém" ? { cls: "blank", t: "Casa sem medição", d: "Nenhum aparelho em casa. Fundo e dose da casa ficam em branco." }
      : { cls: "warnb", t: "Casa sem dado", d: `O ${WH_NAME[hr.dev]} está em casa, mas não manda leitura. Esses minutos ficam em branco.` })
    : { cls: hr.sat ? "warnb" : "", t: hr.both ? "Casa pelos dois" : `Casa pelo ${WH_NAME[hr.src]}`,
      d: (hr.both ? "O GMC-800 mede a casa e o RC102 preenche se ele falhar." : hr.covers ? `Ele assume enquanto o ${WH_NAME[hr.covers]} não mede.` : hr.src === "gmc" ? "Como sempre." : "Ele entra no fundo da casa enquanto estiver lá."), sat: hr.sat ? satTxt : "" };
  const pers = !pr.src
    ? (pr.why === "ninguém" ? { cls: "blank", t: "Sem dosímetro com você", d: w.notHome ? "Você marcou que não estava em casa: sua dose fica em branco neste trecho." : "Sua dose fica em branco neste trecho." }
      : { cls: "warnb", t: "Sua dose sem dado", d: `O ${WH_NAME[pr.dev]} está com você, mas não manda leitura.` })
    : pr.homeRead ? { cls: pr.sat ? "warnb" : "", t: "Sua dose pela leitura de casa", d: "Os dois ficaram em casa, então você também estava. Se saiu e esqueceu de levar, marque \"eu não estava em casa\" no trecho e ele fica em branco.", sat: pr.sat ? satTxt : "" }
    : pr.curve ? { cls: "", t: "Sua dose pelo RC102", d: "Desconectado, mas ele continua somando a dose sozinho. Só a curva minuto a minuto fica faltando até reconectar." }
    : { cls: pr.sat ? "warnb" : "", t: `Sua dose pelo ${WH_NAME[pr.src]}`,
      d: (pr.covers ? `Ele assume enquanto o ${WH_NAME[pr.covers]} não mede.` : both ? "O GMC-800 vai junto e preenche se o RC102 falhar." : pr.src === "radiacode" ? "Como sempre." : "Ele sai do fundo da casa enquanto estiver com você."), sat: pr.sat ? satTxt : "" };
  return { hr, pr, home, pers };
}

function renderWhere() {
  if (typeof renderMiniPlace === "function") renderMiniPlace();
  const card = $("whCard");
  if (!card) return;
  const c = whCur(), off = !whIsNormal(c), w = state.where || {};
  const show = whPending || c;
  card.classList.toggle("off", off);
  const chip = $("whChip");
  chip.hidden = !off;
  if (off && w.since) chip.textContent = `fora do normal desde ${hhmm(w.since)}` + (w.open && w.open.name ? ` · ${w.open.name}` : "");
  for (const dev of ["gmc", "radiacode"]) {
    card.querySelectorAll(`.seg.loc[data-dev="${dev}"] button`).forEach((b) => {
      b.setAttribute("aria-pressed", String(b.dataset.v === show[dev]));
      b.classList.toggle("off", b.dataset.v !== WH_NORMAL[dev] && b.dataset.v !== "fora");
      b.classList.toggle("out", b.dataset.v === "fora");
    });
    const g = dev === "gmc";
    $(g ? "whGmcSince" : "whRcSince").textContent = c[dev] === WH_NORMAL[dev]
      ? "no lugar de sempre"
      : `${{ casa: "em casa", comigo: "com você", fora: "fora do cálculo" }[c[dev]]} desde ${hhmm(w.since)}`;
    const h = whHealth(dev, c), hs = $(g ? "whGmcH" : "whRcH");
    hs.className = "hstat " + h.cls;
    hs.innerHTML = '<span class="hd"></span>';
    hs.append(h.t + (c[dev] === "fora" && w.open && w.open.name ? ` · ${w.open.name}` : ""));
    hs.closest(".wh-row").classList.toggle("dead", c[dev] === "fora");
  }
  // GMC sem contagens: sugere tirar do cálculo
  const sug = $("whSuggest");
  sug.textContent = "";
  const gl = whLive("gmc");
  if (gl.state === "sem contagens" && c.gmc !== "fora" && !(whPending && whPending.gmc === "fora")) {
    const d = document.createElement("div");
    d.className = "wh-suggest";
    const s = document.createElement("span");
    const b = document.createElement("b");
    b.textContent = `O GMC-800 está sem contagens há ${hm(Math.max(1, Math.round((Date.now() - gl.since) / 6e4)))}.`;
    s.append(b, " O tubo pode estar com defeito. Se estiver, tire do cálculo para não puxar o fundo para baixo.");
    const btn = document.createElement("button");
    btn.type = "button"; btn.className = "btn small"; btn.textContent = "Tirar do cálculo…";
    btn.addEventListener("click", () => { whPending = { ...c, gmc: "fora" }; whWhen = "last"; renderWhere(); $("whName").focus(); });
    d.append(s, btn);
    sug.appendChild(d);
  }
  $("whSheet").hidden = !whPending;
  if (whPending) {
    const nf = whNewOff(whPending);
    $("whReasons").hidden = $("whWhenRow").hidden = !nf.length;
    $("whNameLab").innerHTML = nf.length ? "Motivo <em>escolha acima ou escreva</em>" : "Nome <em>opcional</em>";
    $("whName").placeholder = nf.length ? "ex.: quebrado" : "ex.: viagem, trilha";
    const lastGood = nf.length ? Math.max(...nf.map((d) => whLive(d).lastGood || 0)) : 0;
    const lastBtn = $("whWhenLast");
    lastBtn.hidden = !lastGood;
    lastBtn.textContent = lastGood ? `a última leitura boa (${hhmm(lastGood)})` : "";
    if (!lastGood) whWhen = "now";
    $("whWhenSeg").querySelectorAll("button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.w === whWhen)));
    const txt = $("whSheetTxt");
    txt.textContent = "";
    const b = document.createElement("b");
    const at = document.createElement("span");
    at.className = "mono";
    const fromLast = nf.length && whWhen === "last" && lastGood;
    at.textContent = hhmm(fromLast ? lastGood : Date.now());
    if (whIsNormal(whPending)) {
      b.textContent = "Voltar ao normal";
      txt.append(b, " a partir de agora (", at, "): GMC-800 em casa, RC102 com você.");
    } else {
      const l = whLines(whPending);
      b.textContent = whKind(whPending);
      txt.append(b, fromLast ? ", a partir de " : ", a partir de agora (", at, fromLast ? ". " : "). ");
      // uma frase por nó de texto: a tradução acha cada uma no dicionário
      for (const x of [l.home, l.pers]) txt.append(x.t + ". ", x.d + " ", ...(x.sat ? [x.sat + " "] : []));
    }
  }
  const L = whLines(c);
  const box = $("whConseq");
  box.textContent = "";
  for (const [icon, line] of [[WH_ICON_HOME, L.home], [WH_ICON_ME, L.pers]]) {
    const d = document.createElement("div");
    d.className = "wh-cq" + (line.cls ? " " + line.cls : "");
    d.innerHTML = icon;
    const s = document.createElement("span");
    const b = document.createElement("b");
    b.textContent = line.t;
    s.append(b, line.d);
    if (line.sat) s.append(" ", line.sat);
    d.appendChild(s);
    box.appendChild(d);
  }
  const back = $("whBack");
  back.textContent = "";
  if (off && !whPending) {
    const b = document.createElement("button");
    b.type = "button"; b.className = "btn accent"; b.textContent = "Voltar ao normal";
    b.addEventListener("click", () => whSet({ gmc: "casa", radiacode: "comigo" }, ""));
    back.appendChild(b);
    // os dois em casa: saída para "esqueci de levar" (a dose pessoal fica em branco)
    if (c.gmc === "casa" && c.radiacode === "casa" && w.open && api.SavePeriod) {
      const nh = document.createElement("button");
      nh.type = "button"; nh.className = "btn";
      nh.textContent = c.notHome ? "Eu estava em casa" : "Eu não estava em casa";
      nh.addEventListener("click", async () => {
        try { await api.SavePeriod({ ...w.open, notHome: !c.notHome }); await refreshWhere(); } catch (e) { toast(errMsg(e), true); }
      });
      back.appendChild(nh);
    }
  }
}

async function whSet(w, name) {
  try {
    const nf = whNewOff(w);
    const lastGood = nf.length ? Math.max(...nf.map((d) => whLive(d).lastGood || 0)) : 0;
    state.where = whWhen === "last" && lastGood && api.SetWhereAt
      ? await api.SetWhereAt(w.gmc, w.radiacode, name || "", lastGood)
      : await api.SetWhere(w.gmc, w.radiacode, name || "");
    whPending = null; whWhen = "now";
    $("whName").value = "";
    await refreshWhere();
  } catch (e) { toast(errMsg(e), true); }
}

async function refreshWhere() {
  try { state.where = api.GetWhereabouts ? await api.GetWhereabouts() : null; } catch (e) { state.where = null; }
  renderWhere();
}

function wireWhere() {
  const card = $("whCard");
  if (!card) return;
  card.querySelectorAll(".seg.loc button").forEach((b) => b.addEventListener("click", () => {
    const dev = b.parentElement.dataset.dev;
    const next = { ...(whPending || whCur()), [dev]: b.dataset.v };
    const c = whCur();
    whPending = next.gmc === c.gmc && next.radiacode === c.radiacode ? null : next;
    if (!whPending || !whNewOff(whPending).length) whWhen = "now";
    renderWhere();
    if (whPending) $("whName").focus();
  }));
  $("whReasons").querySelectorAll("button").forEach((b) => b.addEventListener("click", () => { $("whName").value = b.textContent.trim(); }));
  $("whWhenSeg").querySelectorAll("button").forEach((b) => b.addEventListener("click", () => { whWhen = b.dataset.w; renderWhere(); }));
  $("whOk").addEventListener("click", () => whPending && whSet(whPending, $("whName").value));
  $("whNo").addEventListener("click", () => { whPending = null; whWhen = "now"; renderWhere(); });
  $("whName").addEventListener("keydown", (e) => { if (e.key === "Enter" && whPending) { e.preventDefault(); whSet(whPending, $("whName").value); } });
  // o estado ao vivo muda sozinho: confere a cada 30 s na tela Agora
  setInterval(() => { if (state.view === "agora" && !whPending) refreshWhere(); }, 30e3);
}
