// Ajustes gravados no RadiaCode (Configurações › RadiaCode).
//
// Mostra o que foi lido do aparelho agora: alarmes de taxa, de dose e de
// contagem, som e vibração (e o que faz cada um disparar), tela e unidades.
// Nada vai para o aparelho sem o clique em "Gravar no RadiaCode", e o
// confirm lista exatamente o que muda. O Go guarda a cópia de antes, grava só
// os registros que mudaram, lê de volta e registra no histórico.

const RC_CTRL = [
  { bit: 1 << 2, name: "Alarme 1 de taxa" },
  { bit: 1 << 3, name: "Alarme 2 de taxa" },
  { bit: 1 << 4, name: "Taxa fora da escala" },
  { bit: 1 << 5, name: "Alarme 1 de dose" },
  { bit: 1 << 6, name: "Alarme 2 de dose" },
  { bit: 1 << 7, name: "Dose fora da escala" },
  { bit: 1 << 0, name: "Botões" },
  { bit: 1 << 1, name: "Cliques", soundOnly: true },
];
const RC_DOSE_OFF = 9e6; // µSv: acima disso o alarme de dose está desligado
const RC_DIRS = ["automática", "virada para a direita", "virada para a esquerda"];

let rcCfg = null, rcCfgRead = false, rcCfgBusy = false;

const rcOnlineReal = () => { const st = state.rc && state.rc.st; return !!(st && st.state === "conectado" && !st.demo); };

function rcDoseTxt(v) {
  if (v >= RC_DOSE_OFF) return "desligado";
  return v >= 1000 ? `${fmt(v / 1000, v % 1000 ? 3 : 0)} mSv` : `${fmt(v, v < 10 ? 2 : 0)} µSv`;
}

function rcCtrlRows() {
  const tb = $("rcCtrlRows");
  if (tb.children.length) return;
  tb.innerHTML = RC_CTRL.map((c) => `<tr><th scope="row">${c.name}</th>
    <td><input type="checkbox" data-snd="${c.bit}" aria-label="Som: ${c.name}"></td>
    <td>${c.soundOnly ? '<span class="muted">—</span>' : `<input type="checkbox" data-vib="${c.bit}" aria-label="Vibração: ${c.name}">`}</td></tr>`).join("");
}

function setRcCfgEnabled(on) {
  qsa("#rcCfg fieldset.dc-edit").forEach((f) => { f.disabled = !on; });
  $("rcCfgSave").disabled = !on;
  $("rcResetDose").disabled = !on;
  $("rcUndo").disabled = !on || !(rcCfg && rcCfg.canUndo);
  rcDoseOffSync();
}

function rcDoseOffSync() {
  for (const n of ["1", "2"]) {
    const off = $("rcDS" + n + "Off").checked;
    $("rcDS" + n).disabled = off || $("rcCfgSave").disabled;
  }
}

function rcHints() {
  const st = (state.rc && state.rc.st) || {};
  const bg = st.stats && st.stats.bgUSv;
  const a1 = parseFloat($("rcDR1").value);
  // uma frase por <span>: cada uma é traduzida sozinha
  const parts = [bg ? `Nível de fundo em casa pelo RadiaCode: ${fmt(bg, 3)} µSv/h.` : "Nível de fundo típico dentro de casa: perto de 0,1 µSv/h."];
  if (bg && a1 > 0) parts.push(`O alarme 1 está em ${fmt(a1 / bg, 1)}× o nível de fundo.`);
  parts.push(`De fábrica: ${fmt(0.4, 2)} e ${fmt(1.2, 2)} µSv/h.`);
  const h = $("rcDRHint");
  h.textContent = "";
  parts.forEach((x, i) => { const sp = document.createElement("span"); sp.textContent = (i ? " " : "") + x; h.appendChild(sp); });
  const dose = st.doseLive || st.dose;
  $("rcDSHint").textContent = dose != null && isFinite(dose) && dose > 0
    ? `Dose acumulada no aparelho agora: ${rcDoseTxt(dose)}.`
    : "";
  $("rcBrtOut").textContent = $("rcBrt").value;
}

function fillRcCfg(c) {
  rcCfg = c;
  rcCtrlRows();
  $("rcDR1").value = c.doseRate1;
  $("rcDR2").value = c.doseRate2;
  for (const n of ["1", "2"]) {
    const v = c["dose" + n], off = v >= RC_DOSE_OFF;
    $("rcDS" + n + "Off").checked = off;
    $("rcDS" + n).value = off ? "" : v;
  }
  $("rcCR1").value = c.countRate1;
  $("rcCR2").value = c.countRate2;
  $("rcSoundOn").checked = !!c.soundOn;
  $("rcVibroOn").checked = !!c.vibroOn;
  qsa("#rcCtrlRows [data-snd]").forEach((el) => { el.checked = (c.soundCtrl & +el.dataset.snd) !== 0; });
  qsa("#rcCtrlRows [data-vib]").forEach((el) => { el.checked = (c.vibroCtrl & +el.dataset.vib) !== 0; });
  $("rcBrt").value = c.brightness;
  const offs = c.offTimes && c.offTimes.length ? c.offTimes : ["5 s", "10 s", "15 s", "30 s"];
  $("rcOff").innerHTML = offs.map((o, i) => `<option value="${i}">${o}</option>`).join("");
  $("rcOff").value = String(c.offTime);
  $("rcDir").value = String(c.direction);
  $("rcUnitDose").value = c.unitSv ? "1" : "0";
  $("rcUnitCnt").value = c.unitCPM ? "1" : "0";
  $("rcEnergy").textContent = `E(c) = ${fmt(c.energyA0, 2)} + ${fmt(c.energyA1, 3)}·c + ${fmt(c.energyA2, 6)}·c² keV`;
  const st = (state.rc && state.rc.st) || {};
  $("rcCfgFw").textContent = [st.model, st.firmware].filter(Boolean).join(" · ") || "—";
  $("rcIntro").textContent = "Valores lidos do RadiaCode agora. Mude o que quiser e clique em \"Gravar no RadiaCode\".";
  rcHints();
  setRcCfgEnabled(true);
}

function readRcCfgForm() {
  const n = (id) => parseFloat($(id).value);
  const s = Object.assign({}, rcCfg);
  s.doseRate1 = n("rcDR1"); s.doseRate2 = n("rcDR2");
  for (const k of ["1", "2"]) s["dose" + k] = $("rcDS" + k + "Off").checked ? 1e7 : n("rcDS" + k);
  s.countRate1 = n("rcCR1"); s.countRate2 = n("rcCR2");
  s.soundOn = $("rcSoundOn").checked;
  s.vibroOn = $("rcVibroOn").checked;
  let snd = rcCfg.soundCtrl & ~0xff, vib = rcCfg.vibroCtrl & ~0xfd;
  qsa("#rcCtrlRows [data-snd]").forEach((el) => { if (el.checked) snd |= +el.dataset.snd; });
  qsa("#rcCtrlRows [data-vib]").forEach((el) => { if (el.checked) vib |= +el.dataset.vib; });
  s.soundCtrl = snd >>> 0; s.vibroCtrl = vib >>> 0;
  s.brightness = parseInt($("rcBrt").value, 10);
  s.offTime = parseInt($("rcOff").value, 10);
  s.direction = parseInt($("rcDir").value, 10);
  s.unitSv = $("rcUnitDose").value === "1";
  s.unitCPM = $("rcUnitCnt").value === "1";
  return s;
}

// o que muda de a para b: [ajuste, agora, depois] (para a confirmação)
function rcCfgDiff(a, b) {
  const out = [];
  const f = (name, x, y) => { if (x !== y) out.push([name, x, y]); };
  const u = (v) => `${fmt(v, v < 10 ? 2 : 1)} µSv/h`;
  f("Alarme 1 de taxa", u(a.doseRate1), u(b.doseRate1));
  f("Alarme 2 de taxa", u(a.doseRate2), u(b.doseRate2));
  f("Alarme 1 de dose", rcDoseTxt(a.dose1), rcDoseTxt(b.dose1));
  f("Alarme 2 de dose", rcDoseTxt(a.dose2), rcDoseTxt(b.dose2));
  f("Alarme 1 de contagem", `${fmt(a.countRate1, 1)} cps`, `${fmt(b.countRate1, 1)} cps`);
  f("Alarme 2 de contagem", `${fmt(a.countRate2, 1)} cps`, `${fmt(b.countRate2, 1)} cps`);
  const lig = (v) => (v ? "ligado" : "desligado");
  f("Som", lig(a.soundOn), lig(b.soundOn));
  f("Vibração", lig(a.vibroOn), lig(b.vibroOn));
  for (const c of RC_CTRL) {
    f(`Som em "${c.name}"`, lig(a.soundCtrl & c.bit), lig(b.soundCtrl & c.bit));
    if (!c.soundOnly) f(`Vibração em "${c.name}"`, lig(a.vibroCtrl & c.bit), lig(b.vibroCtrl & c.bit));
  }
  f("Brilho", String(a.brightness), String(b.brightness));
  const offs = a.offTimes || [];
  f("Apagar a tela", offs[a.offTime] || String(a.offTime), offs[b.offTime] || String(b.offTime));
  f("Orientação", RC_DIRS[a.direction] || String(a.direction), RC_DIRS[b.direction] || String(b.direction));
  f("Unidade de dose", a.unitSv ? "Sv" : "R", b.unitSv ? "Sv" : "R");
  f("Unidade de contagem", a.unitCPM ? "cpm" : "cps", b.unitCPM ? "cpm" : "cps");
  return out;
}

function rcCfgCheck(s) {
  const bad = (v) => !(v > 0) || !isFinite(v);
  if (bad(s.doseRate1) || bad(s.doseRate2) || s.doseRate1 < 0.01 || s.doseRate2 > 1000) return "Os alarmes de taxa de dose precisam ficar entre 0,01 e 1.000 µSv/h.";
  if (s.doseRate2 <= s.doseRate1) return "O alarme 2 de taxa precisa ser maior que o alarme 1.";
  if (bad(s.dose1) || bad(s.dose2) || s.dose1 < 0.01) return "Preencha os alarmes de dose (a partir de 0,01 µSv) ou marque \"desligado\".";
  if (s.dose2 < s.dose1) return "O alarme 2 de dose não pode ser menor que o alarme 1.";
  if (bad(s.countRate1) || bad(s.countRate2) || s.countRate1 < 0.1 || s.countRate2 > 50000) return "Os alarmes de contagem precisam ficar entre 0,1 e 50.000 cps.";
  if (s.countRate2 <= s.countRate1) return "O alarme 2 de contagem precisa ser maior que o alarme 1.";
  return "";
}

// mudou algum ajuste do RadiaCode sem gravar?
function rcCfgDirty() {
  if (!rcCfg || $("rcCfgSave").disabled) return false;
  try { return rcCfgDiff(rcCfg, readRcCfgForm()).length > 0; } catch (e) { return false; }
}

async function openRcCfg(force) {
  if (rcCfgBusy || (rcCfgRead && !force)) return;
  if (!rcOnlineReal()) {
    rcCfg = null;
    setRcCfgEnabled(false);
    $("rcIntro").textContent = "Conecte o RadiaCode para ver e mudar os ajustes do aparelho.";
    return;
  }
  rcCfgRead = true;
  setRcCfgEnabled(false);
  $("rcIntro").textContent = "Lendo os ajustes do RadiaCode…";
  try {
    fillRcCfg(await api.GetRcSettings());
  } catch (e) {
    rcCfgRead = false;
    $("rcIntro").textContent = "Não consegui ler os ajustes: " + errMsg(e);
  }
}

async function rcCfgRun(label, fn, okMsg) {
  rcCfgBusy = true;
  setRcCfgEnabled(false);
  const b = $("rcCfgSave"), txt = b.textContent;
  b.textContent = label;
  try {
    const c = await fn();
    if (c) fillRcCfg(c);
    if (okMsg) toast(okMsg);
  } catch (e) {
    toast(errMsg(e), true);
    try { fillRcCfg(await api.GetRcSettings()); } catch (e2) { setRcCfgEnabled(!!rcCfg); }
  } finally {
    b.textContent = txt;
    rcCfgBusy = false;
  }
}

function wireRcCfg() {
  rcCtrlRows();
  setRcCfgEnabled(false);
  ["rcDR1", "rcBrt", "rcDS1", "rcDS2"].forEach((id) => $(id).addEventListener("input", rcHints));
  ["rcDS1Off", "rcDS2Off"].forEach((id) => $(id).addEventListener("change", () => {
    const n = id[4], inp = $("rcDS" + n);
    if (!$(id).checked && !inp.value) inp.value = n === "1" ? 1000 : 5000;
    rcDoseOffSync();
  }));
  $("btnRcAdjust").addEventListener("click", () => openSettings("rcCfg"));
  $("rcBackups").addEventListener("click", () => call(api.OpenRcConfigBackups));

  $("rcCfgSave").addEventListener("click", async () => {
    const next = readRcCfgForm();
    const err = rcCfgCheck(next);
    if (err) { toast(err, true); return; }
    const diff = rcCfgDiff(rcCfg, next);
    if (!diff.length) { toast("Nada mudou: o aparelho já está assim."); return; }
    const text = ["O app salva uma cópia do que está agora antes de gravar."];
    if (next.doseRate1 !== rcCfg.doseRate1 || next.doseRate2 !== rcCfg.doseRate2) text.push("Os alertas do RadiaCode no app passam a usar os alarmes de taxa novos.");
    if (!(await askConfirm({ title: "Gravar no RadiaCode?", diff, text, textAfter: text.length, ok: "Gravar" }))) return;
    rcCfgRun("Gravando…", () => api.SetRcSettings(next), "Ajustes gravados no RadiaCode");
  });

  $("rcUndo").addEventListener("click", async () => {
    if (!(await askConfirm({ title: "Desfazer a última gravação?", text: "O app grava de volta no RadiaCode os ajustes de antes da última gravação.", ok: "Desfazer" }))) return;
    rcCfgRun("Desfazendo…", () => api.UndoRcSettings(), "Ajustes de antes gravados de volta");
  });

  $("rcResetDose").addEventListener("click", async () => {
    const st = (state.rc && state.rc.st) || {};
    const d = st.doseLive || st.dose;
    if (!(await askConfirm({ title: "Zerar a dose acumulada do RadiaCode?", rows: d > 0 ? [["Dose acumulada agora", rcDoseTxt(d)]] : null, note: "Não tem volta: o aparelho recomeça do zero.", ok: "Zerar a dose", danger: true }))) return;
    rcCfgRun("Zerando…", async () => { await api.ResetRcDose(); return null; }, "Dose do RadiaCode zerada");
  });

  // conectou ou caiu com a seção aberta: lê de novo ou trava
  on("radiacode", () => {
    if (state.view !== "config" || !setShown($("rcCfg"))) return;
    if (rcOnlineReal() && !rcCfgRead) openRcCfg();
    else if (!rcOnlineReal() && rcCfg) { rcCfgRead = false; openRcCfg(); }
  });
}

// modo navegador (sem o Go): aparelho simulado com os valores de fábrica
function demoRcAjustes() {
  let d = {
    doseRate1: 0.4, doseRate2: 1.2, dose1: 9999999.99, dose2: 9999999.99, countRate1: 20, countRate2: 60,
    soundOn: true, soundCtrl: 0x1ffc, vibroOn: true, vibroCtrl: 0x1cfe, brightness: 1, offTime: 1,
    offTimes: ["5 s", "10 s", "15 s", "30 s", "2 min", "5 min"], direction: 0, unitSv: true, unitCPM: true,
    energyA0: 5.43, energyA1: 2.351, energyA2: 0.000381,
  };
  let prev = null;
  const wait = (ms) => new Promise((r) => setTimeout(r, ms));
  const out = () => Object.assign(JSON.parse(JSON.stringify(d)), { canUndo: !!prev });
  return {
    GetRcSettings: async () => { await wait(400); return out(); },
    SetRcSettings: async (s) => { await wait(900); prev = d; d = Object.assign({}, d, s); delete d.canUndo; return out(); },
    UndoRcSettings: async () => { await wait(900); if (!prev) throw new Error("ainda não há gravação para desfazer"); d = prev; prev = null; return out(); },
    ResetRcDose: async () => { await wait(500); },
    OpenRcConfigBackups: async () => {},
  };
}
