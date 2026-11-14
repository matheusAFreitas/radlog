// Nuvem no Google Drive (2.79.0; spec ux/nuvem.html; Go em cloud.go). Remover
// aparelho, recomeçar no formato novo e PC copiado: 2.80.0, ux/nuvem-remover.html.
//
// O cartão #cloudBox em Configurações › Dados e backup, os diálogos do código
// de recuperação (primeiro aparelho e outro aparelho) e o indicador no menu
// das Configurações e no rodapé da barra lateral. O Go avisa as mudanças pelo
// evento "cloud". Uma cópia compilada sem a chave do Google diz ready=false:
// o botão do Google fica desligado com um aviso. No modo demo a tela inteira
// funciona com dados falsos (localStorage gmc.cloudDemo = "outro" faz o login
// cair no "essa conta já tem uma nuvem").
//
// O e-mail da conta só aparece aqui (data-no-tr) e nunca vai para log.

let cloud = { state: "off" };

async function refreshCloud() {
  if (!api.GetCloud) return;
  try { cloud = (await api.GetCloud()) || { state: "off" }; } catch { cloud = { state: "off" }; }
  renderCloud();
}

const CLOUD_MON_PT = ["jan", "fev", "mar", "abr", "mai", "jun", "jul", "ago", "set", "out", "nov", "dez"];
const CLOUD_MON_EN = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
// "2026-11" → "nov/2026"
function cloudMonth(k) {
  const [y, m] = String(k).split("-").map(Number);
  if (!y || !m) return k;
  return (LANG === "en" ? CLOUD_MON_EN : CLOUD_MON_PT)[m - 1] + "/" + y;
}
const cloudQueue = (q) => (q && q.length ? q.map(cloudMonth).join(", ") : "nada");
const cloudWhen = (ms) => (ms ? whenLabel(ms) : "ainda não");
function cloudSize(b) {
  if (!b) return "0 MB";
  return b >= 1e9 ? fmt(b / 1e9, 1) + " GB" : fmt(Math.max(1, Math.round(b / 1e6)), 0) + " MB";
}
// "há 8 h", "há 2 dias"
function cloudAgo(ms) {
  const h = (Date.now() - ms) / 36e5;
  return h < 1 ? "há pouco" : h < 48 ? `há ${Math.round(h)} h` : `há ${Math.round(h / 24)} dias`;
}

// ---------- o que aparece fora das Configurações ----------

// texto e cor do indicador: amarelo só quando precisa de você
function cloudBadge(c) {
  c = c || cloud;
  if (!c || c.state === "off" || c.state === "wait") return null;
  const stale = c.lastSync && Date.now() - c.lastSync > 864e5;
  switch (c.state) {
    case "login": return { text: "nuvem: entrar de novo", att: true };
    case "clock": return { text: "nuvem: relógio errado", att: true };
    case "dup": return { text: "nuvem: confirme o aparelho", att: true };
    case "old": return { text: "nuvem: recomeçar", att: true };
    case "removed": return { text: "nuvem: removido", att: true };
    case "err": return { text: "nuvem: deu erro", att: true };
    case "send": return { text: "nuvem enviando…" };
    case "offline": return stale ? { text: "nuvem: sem sincronizar há mais de 1 dia", att: true } : { text: "nuvem · sem internet" };
    default: return stale ? { text: "nuvem: sem sincronizar há mais de 1 dia", att: true } : { text: `nuvem em dia · ${hhmm(c.lastSync || Date.now())}` };
  }
}

function renderCloudBadges() {
  const b = cloudBadge();
  const side = $("sideCloud");
  if (side) {
    side.hidden = !b;
    if (b) {
      side.classList.toggle("att", !!b.att);
      side.firstChild.className = "dot " + (b.att ? "wait" : "on");
      side.lastChild.textContent = b.text;
    }
  }
  if (typeof renderSetNavStatus === "function") renderSetNavStatus();
}

// ---------- o cartão ----------

function cloudEl(tag, cls, text) {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  if (text != null) e.textContent = text;
  return e;
}
function cloudBtn(text, cls, fn) {
  const b = cloudEl("button", "btn" + (cls ? " " + cls : ""), text);
  b.type = "button";
  if (fn) b.addEventListener("click", fn);
  return b;
}
function cloudKV(rows) {
  const dl = cloudEl("dl", "cloud-kv");
  for (const [k, v, noTr] of rows) {
    dl.append(cloudEl("dt", "", k));
    const dd = cloudEl("dd", "mono", v);
    if (noTr) dd.dataset.noTr = "";
    dl.append(dd);
  }
  return dl;
}
function cloudNote(kind, bold, rest) {
  const n = cloudEl("div", "cloud-note " + kind);
  if (bold) n.append(cloudEl("b", "", bold), rest ? " " + rest : "");
  else n.textContent = rest;
  return n;
}
function cloudDevices(c) {
  const box = cloudEl("div", "cloud-devs");
  if (!c.devices || !c.devices.length) return box;
  box.append(cloudEl("div", "small muted", "Aparelhos nesta nuvem"));
  for (const d of c.devices) {
    const row = cloudEl("div", "cloud-dev");
    const nm = cloudEl("b", "", d.me ? "Este PC" : "");
    if (!d.me) { nm.textContent = d.name; nm.dataset.noTr = ""; }
    const right = cloudEl("span", "mono small muted", d.lastUp ? `enviou ${whenLabel(d.lastUp)}` : "ainda não enviou");
    row.append(nm, right);
    if (!d.me) {
      const idle = d.lastUp ? Math.floor((Date.now() - d.lastUp) / 864e5) : 0;
      if (idle > 30) nm.after(cloudEl("span", "chip cloud-st att", `parado há ${idle} dias`));
      const rm = cloudEl("button", "link small", "remover");
      rm.type = "button";
      rm.addEventListener("click", () => cloudRemoveDevice(d));
      right.append(" ", rm);
    }
    if (d.me) {
      const ed = cloudEl("button", "link small", "renomear");
      ed.type = "button";
      ed.title = `Nome deste aparelho na nuvem: ${asIs(d.name)}`;
      ed.addEventListener("click", () => cloudRename(d.name));
      nm.append(" ", cloudEl("span", "small muted", `(${asIs(d.name)})`), " ", ed);
    }
    box.append(row);
  }
  return box;
}

const CLOUD_TAG = { err: ["com erro", "att"], ok: ["em dia", "ok"], send: ["enviando", "ok"], offline: ["sem internet", ""], login: ["entrar de novo", "att"], clock: ["relógio", "att"], dup: ["pausada", "att"], old: ["recomeçar", "att"], removed: ["removido", "att"] };

function renderCloud() {
  renderCloudBadges();
  cloudRestartedToast(cloud);
  const body = $("cloudBody");
  if (!body) return;
  const c = cloud || { state: "off" };
  const tag = $("cloudTag");
  const t = CLOUD_TAG[c.state];
  tag.hidden = !t;
  if (t) { tag.textContent = t[0]; tag.className = "chip cloud-st " + t[1]; }
  body.replaceChildren();
  const row = cloudEl("div", "row");
  const acct = c.email ? [["Conta", c.email, true]] : [];
  switch (c.state) {
    case "wait":
      body.append(cloudEl("div", "cloud-spin"), cloudEl("p", "", ""));
      body.lastChild.append(cloudEl("b", "", "Termine o login no navegador que abriu."));
      body.append(cloudEl("p", "small muted", "Escolha a conta e aceite. Quando o Google mostrar \"pode fechar esta aba\", volte para cá. Se o navegador não abriu, use o botão abaixo."));
      row.append(cloudBtn("Abrir o navegador de novo", "", () => cloudCall("CloudLogin")), cloudBtn("Cancelar", "", () => cloudCall("CloudCancel")));
      body.append(row);
      return;
    case "err":
    case "ok":
      body.append(cloudKV([...acct, ["Última sincronização", cloudWhen(c.lastSync)], ["Na fila", cloudQueue(c.queue)], ["Espaço usado no Drive", cloudSize(c.used)]]));
      if (c.err) body.append(cloudNote("att", "Não deu certo da última vez:", c.err));
      body.append(cloudDevices(c));
      body.append(cloudAutoToggle(c));
      row.append(cloudBtn("Sincronizar agora", "", () => cloudCall("CloudSyncNow")), cloudBtn("Ver código de recuperação", "", cloudShowCode), cloudBtn("Desconectar do Google", "", cloudDisconnect));
      body.append(row);
      break;
    case "send": {
      const now = c.restart ? `subindo tudo de novo · ${c.sendI} de ${c.sendN}` : `enviando ${cloudMonth(c.sendNow)} · ${c.sendI} de ${c.sendN}`;
      body.append(cloudKV([...acct, ["Agora", now], ["Última sincronização", cloudWhen(c.lastSync)]]));
      const bar = cloudEl("div", "cloud-bar");
      const fill = cloudEl("i");
      fill.style.width = Math.round(100 * Math.max(0, (c.sendI - 0.5) / Math.max(1, c.sendN))) + "%";
      bar.append(fill);
      body.append(bar, cloudEl("p", "small muted", c.restart
        ? "Nuvem nova criada; a antiga já está na lixeira do Drive. Pode fechar o app: o envio continua de onde parou."
        : "A primeira vez sobe todo o histórico, um arquivo por dia; pode levar alguns minutos. O app continua gravando normalmente."));
      row.append(cloudBtn("Pausar", "", () => cloudCall("CloudPause")));
      body.append(row);
      break;
    }
    case "offline":
      body.append(cloudKV([...acct, ["Última sincronização", c.lastSync ? `${whenLabel(c.lastSync)} (${cloudAgo(c.lastSync)})` : "ainda não"], ["Na fila", cloudQueue(c.queue)]]));
      body.append(cloudNote("info", "", "Sem internet agora. Tudo continua gravado aqui; o que está na fila sobe quando a conexão voltar."), cloudDevices(c));
      row.append(cloudBtn("Tentar agora", "", () => cloudCall("CloudSyncNow")));
      body.append(row);
      break;
    case "login":
      body.append(cloudNote("att", "O Google pediu para entrar de novo.", "Isso acontece quando o acesso é revogado na conta Google ou fica muito tempo sem uso. Nada foi perdido: os dados estão aqui e a cópia da nuvem continua lá."));
      body.append(cloudKV([["Última sincronização", cloudWhen(c.lastSync)], ["Na fila", cloudQueue(c.queue)]]));
      row.append(cloudBtn("Entrar com Google", "accent", () => cloudCall("CloudLogin")), cloudBtn("Desconectar", "", cloudDisconnect));
      body.append(row);
      break;
    case "clock": {
      const min = Math.max(1, Math.round(Math.abs(c.skewSec || 0) / 60));
      body.append(cloudNote("att", `O relógio deste PC está ${min} min ${(c.skewSec || 0) >= 0 ? "adiantado" : "atrasado"}`, "em relação ao Google. A sincronização continua, mas as horas das anotações editadas podem ficar trocadas entre os aparelhos. Acerte a hora do Windows (Configurações › Hora e idioma › Sincronizar agora)."));
      body.append(cloudKV([["Última sincronização", cloudWhen(c.lastSync)]]), cloudDevices(c));
      break;
    }
    case "dup": {
      const other = c.dupOf || "outro aparelho";
      const n = cloudNote("att", "", "");
      n.append(cloudEl("b", "", `Parece que este PC é uma cópia de "${asIs(other)}".`), " Os dois estão usando a mesma faixa da nuvem (isso acontece quando a pasta do RadLog é copiada de um PC para outro). A nuvem está pausada aqui até você escolher.");
      body.append(n);
      row.append(cloudBtn("Este é um aparelho novo", "accent", () => cloudCall("CloudDupAnswer", true)), cloudBtn(`Este é o ${asIs(other)} (o outro foi desligado)`, "", () => cloudCall("CloudDupAnswer", false)));
      body.append(row);
      body.append(cloudEl("p", "small muted", "Aparelho novo: este PC ganha uma linha própria na lista e sobe os dados dele."));
      body.append(cloudEl("p", "small muted", `O mesmo: este PC continua a linha do ${asIs(other)}. Se o outro ligar de novo, ele é que vai cair nesta pergunta.`));
      break;
    }
    case "old": {
      const n = cloudNote("att", "A nuvem precisa recomeçar uma vez.", "Esta versão mudou o jeito de guardar os arquivos no Drive, e a nuvem de antes não serve mais. Nada some deste PC.");
      body.append(n);
      const ul = cloudEl("ul", "cloud-pts small");
      for (const t of ["O código de recuperação continua o mesmo.", "O app cria a pasta nova e só depois manda a antiga para a lixeira do Drive, onde ela fica 30 dias.", "Este PC sobe tudo de novo.", "Os outros aparelhos só precisam atualizar o RadLog."]) ul.append(cloudEl("li", "", t));
      body.append(ul);
      if (c.err) body.append(cloudNote("att", "Não deu certo da última vez:", c.err));
      row.append(cloudBtn("Recomeçar a nuvem", "accent", cloudRestart), cloudBtn("Agora não", "", () => toast("A nuvem fica pausada até você recomeçar ou desconectar.")));
      body.append(row);
      body.append(cloudEl("p", "small muted", "\"Agora não\" deixa a nuvem pausada. O aviso continua aqui até você recomeçar ou desconectar."));
      break;
    }
    case "removed": {
      body.append(cloudNote("att", "", `Outro aparelho tirou este PC da nuvem em ${cloudDay(c.removed)}. Os dados daqui continuam todos aqui.`));
      row.append(cloudBtn("Entrar de novo", "accent", cloudRejoin), cloudBtn("Deixar desligada", "", () => cloudCall("CloudDisconnect")));
      body.append(row);
      body.append(cloudEl("p", "small muted", "\"Entrar de novo\" pede o código de recuperação e sobe tudo deste PC outra vez."));
      break;
    }
    default: {
      body.append(cloudEl("p", "small muted", "Guarda uma cópia cifrada dos seus dados numa pasta do RadLog no seu próprio Google Drive, para outro PC (e, depois, o celular) ter o mesmo histórico."));
      const ul = cloudEl("ul", "cloud-pts small");
      for (const s of ["O Google guarda os arquivos mas não consegue ler: a chave fica nos seus aparelhos e num código que só você tem.", "O app só enxerga a pasta que ele mesmo criou, não o resto do Drive.", "Lugar, coordenadas e metadados de foto nunca sobem.", "Sem internet, nada muda: o app grava aqui e sobe depois."]) ul.append(cloudEl("li", "", s));
      body.append(ul);
      const go = cloudBtn("Entrar com Google", "accent", () => cloudCall("CloudLogin"));
      if (!c.ready) go.disabled = true;
      row.append(go);
      body.append(row);
      if (!c.ready) body.append(cloudNote("info", "", "Esta cópia do app foi compilada sem a chave do Google. A nuvem funciona na versão baixada do GitHub."));
    }
  }
  const how = cloudEl("details", "how");
  how.append(cloudEl("summary", "", "Como funciona"));
  for (const s of ["Cada aparelho sobe os próprios dados para a pasta do RadLog no seu Drive, um arquivo por dia (e cada anexo uma vez só), cifrado com a chave do código de recuperação. Os outros aparelhos baixam e juntam só o que falta, como no \"Importar backup\". O que você apaga ou edita num aparelho vale nos outros.", "O e-mail da conta aparece só aqui: ele não vai para o registro, o backup, os PDFs nem a nuvem.", "A sincronização pela rede (abaixo) continua funcionando como antes."]) how.append(cloudEl("p", "small muted", s));
  body.append(how);
}

function cloudAutoToggle(c) {
  const l = cloudEl("label", "toggle");
  const cb = document.createElement("input");
  cb.type = "checkbox";
  cb.checked = c.auto !== false;
  cb.addEventListener("change", () => cloudCall("SetCloudAuto", cb.checked));
  l.append(cloudEl("span", "", "Sincronizar sozinho a cada 15 min"), cb);
  return l;
}

// chama o Go (ou o demo)
async function cloudCall(fn, ...args) {
  if (!api[fn]) return;
  try {
    const r = await api[fn](...args);
    if (r && r.state) cloud = r;
    renderCloud();
    if (cloud.state === "wait" && fn === "CloudLogin" && api.CloudLoginWait) {
      const done = await api.CloudLoginWait();
      if (!done) return;
      cloud = done.info || cloud;
      renderCloud();
      if (done.next === "new") cloudFirstDevice();
      else if (done.next === "existing") cloudOtherDevice(done);
      else if (done.next === "old") cloudOldLogin();
    }
  } catch (e) {
    toast(String(e && e.message || e), true);
  }
}

async function cloudDisconnect() {
  const ok = await askConfirm({
    title: "Desconectar do Google?",
    text: ["Este PC para de sincronizar com a nuvem. Os dados daqui não mudam, e a cópia da nuvem continua no seu Drive.", "Para ligar de novo, entre com o Google e digite o código de recuperação."],
    ok: "Desconectar", danger: true,
  });
  if (ok) cloudCall("CloudDisconnect");
}

async function cloudRename(cur) {
  cloudDialog((box, close) => {
    box.append(cloudEl("h3", "", "Nome deste aparelho na nuvem"));
    box.append(cloudEl("p", "small muted", "É como os outros aparelhos veem este PC na lista. Vai cifrado junto com os dados."));
    const inp = cloudInput("nome", cur);
    inp.maxLength = 40;
    box.append(inp.parentNode);
    const row = cloudEl("div", "row cloud-foot");
    row.append(cloudBtn("Cancelar", "", close), cloudBtn("Salvar", "accent", async () => {
      const v = inp.value.trim();
      if (!v) { inp.focus(); return; }
      close();
      await cloudCall("SetCloudName", v);
    }));
    box.append(row);
    setTimeout(() => inp.select(), 30);
  });
}

// "14/11"
function cloudDay(ms) {
  const d = new Date(ms || Date.now());
  return String(d.getDate()).padStart(2, "0") + "/" + String(d.getMonth() + 1).padStart(2, "0");
}

async function cloudRemoveDevice(d) {
  const ok = await askConfirm({
    title: `Remover "${asIs(d.name)}" da nuvem?`,
    text: [
      d.lastUp ? `Último envio em ${cloudDay(d.lastUp)}.` : "Ele ainda não enviou nada.",
      "Os arquivos que ele mandou vão para a lixeira do seu Drive e ficam lá 30 dias.",
      "O que já chegou neste PC continua aqui, e este PC sobe de novo o que tinha recebido dele. Os outros aparelhos não perdem nada.",
      "Se ele ainda estiver ligado, para de sincronizar e mostra que foi removido. Dá para entrar de novo com o código.",
      "Perdeu ou roubaram o aparelho? Remover não basta, porque ele ainda sabe o código de recuperação. Nesse caso, depois de remover, use \"Começar uma nuvem nova\" para trocar o código.",
    ],
    ok: "Remover", danger: true,
  });
  if (ok) cloudCall("CloudRemoveDevice", d.id);
}

async function cloudRestart() {
  await cloudCall("CloudRestart");
}

// o toast do fim do recomeço aparece uma vez
function cloudRestartedToast(c) {
  if (!c || !c.restarted || Date.now() - c.restarted > 6e5) return;
  let seen = "";
  try { seen = localStorage.getItem("gmc.cloudRestarted") || ""; } catch {}
  if (seen === String(c.restarted)) return;
  try { localStorage.setItem("gmc.cloudRestarted", String(c.restarted)); } catch {}
  toast(`Nuvem recomeçada. Tudo deste PC subiu no formato novo, com o mesmo código. A pasta antiga fica na lixeira do Drive até ${cloudDay(c.restarted + 30 * 864e5)}.`);
}

// "Entrar de novo" depois de removido: confere o código guardado
function cloudRejoin() {
  cloudDialog((box, close) => {
    box.append(cloudEl("h3", "", "Entrar de novo na nuvem"));
    box.append(cloudEl("p", "small muted", "Digite o código de recuperação. Este PC volta como o mesmo aparelho e sobe tudo outra vez."));
    const inp = cloudInput("código de recuperação (24 letras e números; pode colar)");
    inp.classList.add("mono");
    inp.placeholder = "____-____-____-____-____-____";
    inp.parentNode.classList.add("big");
    const err = cloudEl("p", "cloud-err", "");
    err.hidden = true;
    box.append(inp.parentNode, err);
    const f = cloudEl("div", "row cloud-foot");
    f.append(cloudBtn("Cancelar", "", close), cloudBtn("Entrar de novo", "accent", async () => {
      err.hidden = true;
      try {
        const r = await api.CloudRejoin(inp.value);
        cloud = r || cloud;
      } catch (e) {
        err.hidden = false;
        err.textContent = String(e && e.message || e);
        return;
      }
      close();
      renderCloud();
    }));
    box.append(f);
    setTimeout(() => inp.focus(), 30);
  });
}

// login numa conta com nuvem do formato antigo
function cloudOldLogin() {
  cloudDialog((box, close) => {
    box.append(cloudEl("h3", "", "Achamos uma nuvem de uma versão antiga"));
    box.append(cloudEl("p", "small muted", "Esta conta tem uma pasta do RadLog no formato de antes, que esta versão não lê."));
    const ul = cloudEl("ul", "cloud-pts small");
    for (const t of ["A pasta antiga vai para a lixeira do Drive e fica lá 30 dias.", "Começa uma nuvem nova, com código de recuperação novo (o próximo passo é o de sempre: ver, confirmar e conferir o código).", "Os dados deste PC sobem de novo."]) ul.append(cloudEl("li", "", t));
    box.append(ul);
    const f = cloudEl("div", "row cloud-foot");
    f.append(cloudBtn("Cancelar", "", () => { close(); cloudCall("CloudCancel"); }), cloudBtn("Continuar", "accent", async () => {
      close();
      try { await api.CloudReset(); } catch (e) { toast(String(e && e.message || e), true); return; }
      cloudFirstDevice();
    }));
    box.append(f, cloudEl("p", "small muted", "Cancelar sai do login sem mexer no Drive."));
  });
}

// ---------- diálogos ----------

// abre #cloudDlg com o conteúdo que build monta; close fecha
function cloudDialog(build) {
  const d = $("cloudDlg");
  const box = $("cloudDlgBody");
  box.replaceChildren();
  const close = () => { if (d.open) d.close(); };
  build(box, close);
  if (!d.open) d.showModal();
  return close;
}
function cloudSteps(n) {
  const s = cloudEl("div", "cloud-steps");
  for (let i = 1; i <= 3; i++) s.append(cloudEl("i", i <= n ? "f" : ""));
  return s;
}
function cloudCodeGrid(code) {
  const g = cloudEl("div", "cloud-code");
  g.dataset.noTr = "";
  code.split("-").forEach((p, i) => {
    const s = cloudEl("span", "mono", p);
    s.append(cloudEl("small", "", String(i + 1)));
    g.append(s);
  });
  return g;
}
function cloudInput(label, value) {
  const wrap = cloudEl("label", "cloud-inp");
  wrap.append(cloudEl("span", "", label));
  const i = document.createElement("input");
  i.type = "text";
  i.autocomplete = "off";
  i.spellcheck = false;
  if (value) i.value = value;
  wrap.append(i);
  return i;
}
async function cloudCopy(code) {
  try { await navigator.clipboard.writeText(code); toast("Código copiado"); } catch { toast("Não deu para copiar; selecione e copie à mão.", true); }
}
async function cloudSave(code) {
  try {
    const p = await api.SaveCloudCode(code);
    if (p) toast("Código salvo em " + p.split(/[\\/]/).pop());
  } catch (e) { toast(String(e && e.message || e), true); }
}

// primeiro aparelho: ver, guardar e conferir (a nuvem só liga no fim)
async function cloudFirstDevice() {
  let code;
  try { code = await api.NewCloudCode(); } catch (e) { toast(String(e && e.message || e), true); return; }
  const groups = code.split("-");
  const step1 = () => cloudDialog((box, close) => {
    box.append(cloudSteps(1), cloudEl("h3", "", "Guarde este código"));
    box.append(cloudEl("p", "small muted", "É a chave da sua nuvem. Com ele você liga outro PC ou o celular, ou recupera tudo se este PC quebrar. Ninguém mais tem esse código: nem o Google, nem o RadLog. Se perder o código e todos os aparelhos, a cópia da nuvem fica ilegível (os dados deste PC continuam intactos)."));
    box.append(cloudCodeGrid(code));
    const r = cloudEl("div", "row");
    r.append(cloudBtn("Copiar", "", () => cloudCopy(code)), cloudBtn("Salvar em arquivo", "", () => cloudSave(code)));
    box.append(r, cloudEl("p", "small muted", "Guarde onde só você acessa, como o gerenciador de senhas ou junto do backup."));
    const f = cloudEl("div", "row cloud-foot");
    f.append(cloudBtn("Cancelar", "", () => { close(); cloudCall("CloudCancel"); }), cloudBtn("Guardei, continuar", "accent", step2));
    box.append(f);
  });
  const step2 = () => cloudDialog((box) => {
    box.append(cloudSteps(2), cloudEl("h3", "", "Você guardou o código?"));
    box.append(cloudEl("p", "small muted", "Depois desta tela ele só aparece de novo em \"Ver código de recuperação\", neste PC."));
    const l = cloudEl("label", "cloud-check");
    const cb = document.createElement("input");
    cb.type = "checkbox";
    l.append(cb, cloudEl("span", "", "Guardei o código num lugar seguro, fora deste PC."));
    box.append(l);
    const f = cloudEl("div", "row cloud-foot");
    const go = cloudBtn("Conferir", "accent", step3);
    go.disabled = true;
    cb.addEventListener("change", () => { go.disabled = !cb.checked; });
    f.append(cloudBtn("Voltar e ver de novo", "", step1), go);
    box.append(f);
  });
  const step3 = () => cloudDialog((box, close) => {
    // dois grupos sorteados
    const a = Math.floor(Math.random() * 6);
    let b = Math.floor(Math.random() * 5);
    if (b >= a) b++;
    const ask = [a, b].sort((x, y) => x - y);
    box.append(cloudSteps(3), cloudEl("h3", "", "Confira dois pedaços do código"));
    box.append(cloudEl("p", "small muted", "Para ter certeza de que o que você guardou está certo. Sem olhar nesta tela (ela não mostra mais o código)."));
    const grid = cloudEl("div", "cloud-inputs");
    const ins = ask.map((k) => {
      const i = cloudInput(`grupo ${k + 1}`);
      i.maxLength = 4;
      i.classList.add("mono");
      grid.append(i.parentNode);
      return i;
    });
    const err = cloudEl("p", "cloud-err", "");
    err.hidden = true;
    box.append(grid, err);
    const f = cloudEl("div", "row cloud-foot");
    f.append(cloudBtn("Voltar e ver de novo", "", step1), cloudBtn("Ligar a nuvem", "accent", async () => {
      const norm = (s) => s.toUpperCase().replace(/O/g, "0").replace(/[IL]/g, "1").replace(/[^0-9A-Z]/g, "");
      const ok = ins.every((i, k) => norm(i.value) === groups[ask[k]]);
      if (!ok) {
        err.hidden = false;
        err.textContent = "Não confere. Veja o código que você guardou, ou volte e veja de novo.";
        return;
      }
      close();
      await cloudCall("CloudFinishSetup", code);
    }));
    box.append(f);
    setTimeout(() => ins[0].focus(), 30);
  });
  step1();
}

// outro aparelho: a conta já tem uma nuvem, então pede o código
function cloudOtherDevice(info) {
  cloudDialog((box, close) => {
    box.append(cloudEl("h3", "", "Essa conta já tem uma nuvem do RadLog"));
    box.append(cloudEl("p", "small muted", `Ela foi criada em outro aparelho (último envio ${info && info.lastUp ? whenLabel(info.lastUp) : "—"}; os nomes dos aparelhos só aparecem depois do código, porque também vão cifrados). Para abrir os arquivos, digite o código de recuperação que você guardou.`));
    const inp = cloudInput("código de recuperação (24 letras e números; pode colar)");
    inp.classList.add("mono");
    inp.placeholder = "____-____-____-____-____-____";
    inp.parentNode.classList.add("big");
    const err = cloudEl("p", "cloud-err", "");
    err.hidden = true;
    box.append(inp.parentNode, err);
    const f = cloudEl("div", "row cloud-foot");
    f.append(cloudBtn("Cancelar", "", () => { close(); cloudCall("CloudCancel"); }), cloudBtn("Abrir a nuvem", "accent", async () => {
      err.hidden = true;
      inp.parentNode.classList.remove("bad");
      let code;
      try { code = await api.CheckCloudCode(inp.value); } catch (e) {
        err.hidden = false;
        err.textContent = String(e && e.message || e);
        inp.parentNode.classList.add("bad");
        return;
      }
      inp.value = code;
      try {
        const r = await api.CloudOpen(code);
        cloud = r || cloud;
      } catch {
        err.hidden = false;
        err.textContent = "Esse código não abre os arquivos desta nuvem. Confira letra por letra (o código não tem O, I nem L; parecidos com 0 e 1).";
        inp.parentNode.classList.add("bad");
        return;
      }
      cloudOpened(close);
    }));
    box.append(f);
    const lost = cloudEl("details", "cloud-lost");
    lost.append(cloudEl("summary", "small muted", "Perdi o código"));
    lost.append(cloudEl("p", "small muted", "Pegue o código num aparelho que já está ligado na nuvem: Configurações › Dados e backup › Nuvem › Ver código de recuperação. Se nenhum aparelho tem mais, dá para começar uma nuvem nova; a antiga fica ilegível, e os dados de cada aparelho sobem de novo."));
    lost.append(cloudBtn("Começar uma nuvem nova", "danger", () => cloudNewCloud(close)));
    box.append(lost);
    setTimeout(() => inp.focus(), 30);
  });
}

function cloudOpened(close) {
  cloudDialog((box) => {
    box.append(cloudEl("h3", "", "Nuvem aberta"));
    box.append(cloudEl("p", "small muted", "Este aparelho agora faz parte da nuvem. Ele vai baixar o histórico dos outros aparelhos (pode levar alguns minutos) e subir o dele."));
    const me = (cloud.devices || []).find((d) => d.me);
    const inp = cloudInput("nome deste aparelho na nuvem", me ? me.name : "");
    inp.maxLength = 40;
    box.append(inp.parentNode);
    const f = cloudEl("div", "row cloud-foot");
    f.append(cloudBtn("Começar", "accent", async () => {
      close();
      const v = inp.value.trim();
      if (v && (!me || v !== me.name)) await cloudCall("SetCloudName", v);
      else renderCloud();
    }));
    box.append(f);
  });
}

function cloudNewCloud(close) {
  cloudDialog((box) => {
    box.append(cloudEl("h3", "", "Começar uma nuvem nova?"));
    box.append(cloudEl("p", "small muted", "A cópia que está no Drive fica ilegível para sempre (sem o código, ninguém abre). Os dados deste PC continuam aqui e sobem de novo com um código novo. Os outros aparelhos vão pedir o código novo."));
    const inp = cloudInput("para confirmar, digite NOVA");
    box.append(inp.parentNode);
    const f = cloudEl("div", "row cloud-foot");
    const go = cloudBtn("Começar uma nuvem nova", "danger", async () => {
      close();
      await cloudCall("CloudReset");
      cloudFirstDevice();
    });
    go.disabled = true;
    inp.addEventListener("input", () => { go.disabled = inp.value.trim().toUpperCase() !== "NOVA"; });
    f.append(cloudBtn("Voltar", "", () => cloudOtherDevice(cloud)), go);
    box.append(f);
    setTimeout(() => inp.focus(), 30);
  });
}

// "Ver código de recuperação": confirmação simples e mostra de novo
async function cloudShowCode() {
  const ok = await askConfirm({ title: "Mostrar o código de recuperação?", text: "Confira se ninguém está olhando a tela. Com o código, dá para abrir a cópia da nuvem.", ok: "Mostrar" });
  if (!ok || !api.CloudCode) return;
  let code;
  try { code = await api.CloudCode(); } catch (e) { toast(String(e && e.message || e), true); return; }
  cloudDialog((box, close) => {
    box.append(cloudEl("h3", "", "Código de recuperação"));
    box.append(cloudEl("p", "small muted", "É a chave da sua nuvem. Com ele você liga outro PC ou o celular."));
    box.append(cloudCodeGrid(code));
    const f = cloudEl("div", "row cloud-foot");
    f.append(cloudBtn("Copiar", "", () => cloudCopy(code)), cloudBtn("Salvar em arquivo", "", () => cloudSave(code)), cloudBtn("Fechar", "accent", close));
    box.append(f);
  });
}

function wireCloud() {
  if (!$("cloudBody")) return;
  on("cloud", (c) => { if (c) { cloud = c; renderCloud(); } });
  const d = $("cloudDlg");
  d.addEventListener("cancel", () => { if (cloud.state === "wait" && api.CloudCancel) cloudCall("CloudCancel"); });
  const side = $("sideCloud");
  if (side) side.addEventListener("click", () => { if (typeof openSettings === "function") openSettings("cloudBox"); });
  refreshCloud();
  // o "em dia · 17:42" e o amarelo de >24 h mudam com o tempo
  setInterval(renderCloudBadges, 60e3);
}
