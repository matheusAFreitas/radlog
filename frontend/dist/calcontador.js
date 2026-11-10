// Calibrar o contador com o fator do app (2.37.0).
//
// No cartão "Pontos gravados no contador" (Configurações › GMC-800 ›
// Calibração): mostra os pontos de hoje ao lado dos sugeridos com o fator em
// uso no app (intercalibração), grava no GMC-800 e guarda a calibração de
// fábrica (fixa) e a anterior (a que estava antes da última mudança), com
// botão para voltar a cada uma. Só os 6 pontos mudam; o resto da
// configuração do contador fica como está.

const CC_SRC = { "intercalibração": "intercalibração com o RadiaCode", "você": "informado à mão", "contador": "curva do contador" };

function ccDay(t) {
  return new Date(t).toLocaleDateString(LOCALE, { day: "2-digit", month: "2-digit" });
}

function ccList(pts) {
  return (pts || []).map((p) => fmt(p.cpm, 0)).join(" / ");
}

// pontos de antes e depois, lado a lado (para a confirmação)
function ccDiff(a, b) {
  a = a || []; b = b || [];
  const n = Math.max(a.length, b.length), out = [];
  for (let i = 0; i < n; i++) out.push([`Ponto ${i + 1}`, a[i] ? `${fmt(a[i].cpm, 0)} CPM` : "—", b[i] ? `${fmt(b[i].cpm, 0)} CPM` : "—"]);
  return out;
}

function ccSavedRow(label, s, key) {
  if (!s) return "";
  const what = key === "fabrica" ? (s.fabrica ? "a calibração de fábrica do GMC-800" : "a que estava no contador antes do app mexer") : "a que estava antes da última mudança";
  return `<div class="cc-saved"><dt>${label}</dt><dd><span class="mono">${ccList(s.pontos)} CPM</span><br><span class="muted">${what}</span> <span class="muted">guardada em ${ccDay(s.em)}</span></dd>` +
    `<dd><button type="button" class="btn small" data-cc="${key}">${key === "fabrica" ? "Voltar à de fábrica" : "Voltar à anterior"}</button></dd></div>`;
}

let ccView = null;

async function renderCounterCal() {
  const box = $("ccBox");
  if (!box || !api.GetCounterCal) return;
  const pts = (devCfg && devCfg.calibration && devCfg.calibration.points) || [];
  try { ccView = await api.GetCounterCal(pts); } catch (e) { box.hidden = true; return; }
  const v = ccView;
  const src = CC_SRC[v.source] || "calibração do app";
  let h = "";
  if (v.same) {
    h = `<p class="small cc-done">${lvIcon("seguro", 12)} Contador e app usam o mesmo fator: ${fmt(v.appK, 5)} µSv/h por CPM. Os dois mostram o mesmo µSv/h.</p>`;
  } else if (v.suggested && v.suggested.length) {
    const pct = v.appK > 0 ? (v.devK / v.appK - 1) * 100 : 0;
    const n = fmt(Math.abs(pct), 0);
    const since = v.since ? ` Em uso desde ${ccDay(v.since)}.` : "";
    const diff = pct >= 0 ? `Por isso a tela dele mostra uns ${n}% a mais que o app.` : `Por isso a tela dele mostra uns ${n}% a menos que o app.`;
    h = `<h3 class="cc-title">Calibrar o contador com a intercalibração</h3>` +
      `<p class="small muted">O app usa ${fmt(v.appK, 5)} µSv/h por CPM (${src}).${since} O contador usa ${fmt(v.devK, 5)}. ${diff} Gravando os pontos abaixo, os dois passam a mostrar o mesmo µSv/h.</p>` +
      `<div class="table-box"><table class="cc-cmp"><thead><tr><th scope="col">Ponto</th><th scope="col" class="num">µSv/h</th><th scope="col" class="num cc-grp">CPM hoje</th><th scope="col" class="num">CPM por µSv/h</th><th scope="col" class="num cc-grp">CPM sugerido</th><th scope="col" class="num">CPM por µSv/h</th></tr></thead><tbody>` +
      v.suggested.map((p, i) => {
        const o = v.device[i] || {};
        return `<tr><td>${i + 1}</td><td class="num mono">${fmt(p.usv, p.usv < 10 ? 2 : 0)}</td><td class="num mono cc-old cc-grp">${fmt(o.cpm, 0)}</td><td class="num mono cc-old">${fmt(o.cpm / o.usv, 1)}</td>` +
          `<td class="num mono cc-new cc-grp">${fmt(p.cpm, 0)}</td><td class="num mono cc-new">${fmt(p.cpm / p.usv, 1)}</td></tr>`;
      }).join("") + `</tbody></table></div>` +
      `<p class="small muted">O µSv/h de cada ponto fica igual e o CPM é multiplicado por ${fmt(v.devK / v.appK, 3)}. A forma da curva continua, inclusive a compensação de tempo morto nos pontos de cima. A intercalibração mede só o fundo da casa; os pontos de cima seguem a forma da curva atual, não foram medidos.</p>` +
      `<div class="row"><button type="button" class="btn accent" data-cc="gravar">Gravar no contador…</button></div>`;
  }
  if (v.factory || v.previous) {
    h += `<dl class="cc-keep">${ccSavedRow("De fábrica", v.factory, "fabrica")}${ccSavedRow("Anterior", v.previous, "anterior")}</dl>`;
  }
  box.innerHTML = h;
  box.hidden = !h;
  qsa("#ccBox button").forEach((b) => { b.disabled = !(devCfg && devCfg.calibrationEditable) || $("dcSaveBtn").disabled; });
}

async function ccWrite(fn, okMsg) {
  setDevCfgEnabled(false);
  qsa("#ccBox button").forEach((b) => { b.disabled = true; });
  try {
    devCfg = await call(fn);
    fillDevCfg(devCfg);
    toast(okMsg);
    state.status = await api.GetStatus();
    renderStatus();
    if (typeof calHistChanged === "function") await calHistChanged();
  } catch (e) {
    setDevCfgEnabled(!!(devCfg && devCfg.editable));
    renderCounterCal();
  }
}

function initCounterCal() {
  const box = $("ccBox");
  if (!box) return;
  box.addEventListener("click", async (e) => {
    const b = e.target.closest("button[data-cc]");
    if (!b || !ccView) return;
    const k = b.dataset.cc;
    if (k === "gravar") {
      const s = ccView.suggested, d = ccView.device;
      const text = [
        "O µSv/h de cada ponto fica igual.",
        ccView.factory ? "A de fábrica já está guardada e continua guardada. A que está no contador agora vira a \"anterior\"." : "A calibração que está no contador agora fica guardada como a de fábrica.",
        "Os outros ajustes do contador não mudam.",
      ];
      if (!(await askConfirm({ title: "Gravar a calibração nova no GMC-800?", diff: ccDiff(d, s), diffHead: ["Ponto", "Hoje", "Novo"], text, textAfter: text.length, note: "Não desconecte o cabo enquanto grava.", ok: "Gravar" }))) return;
      ccWrite(api.CalibrateCounter, "Contador calibrado com o fator do app");
    } else if (k === "fabrica" || k === "anterior") {
      const s = k === "fabrica" ? ccView.factory : ccView.previous;
      const q = k === "fabrica" ? "Voltar à calibração de fábrica?" : "Voltar à calibração anterior?";
      if (!(await askConfirm({ title: q, diff: ccDiff(ccView.device, s.pontos), diffHead: ["Ponto", "Agora", "Depois"], text: "Só os 6 pontos mudam. A que está no contador agora fica guardada como a \"anterior\".", textAfter: 1, note: "Não desconecte o cabo enquanto grava.", ok: "Voltar" }))) return;
      ccWrite(() => api.RestoreCounterCal(k), k === "fabrica" ? "Contador de volta à calibração de fábrica" : "Contador de volta à calibração anterior");
    }
  });
}

// modo navegador: guarda as cópias na memória
function demoCounterCal() {
  const keep = { fabrica: null, anterior: null };
  const same = (a, b) => a.length === b.length && a.every((p, i) => p.cpm === b[i].cpm && Math.abs(p.usv - b[i].usv) < 1e-6);
  const fab = [[986, 10], [8546, 100], [16408, 200], [37970, 500], [76010, 1000], [139671, 2000]].map(([cpm, usv]) => ({ cpm, usv }));
  const appK = () => 0.00564;
  const rotate = (before) => {
    const s = { pontos: before.map((p) => ({ ...p })), em: Date.now(), fabrica: same(before, fab) };
    if (!keep.fabrica) keep.fabrica = s;
    else if (!same(keep.fabrica.pontos, before)) keep.anterior = s;
  };
  const write = async (pts) => {
    const cfg = await api.GetDeviceConfig();
    rotate(cfg.calibration.points);
    cfg.calibration.points = pts;
    return api.SetDeviceConfig(cfg);
  };
  return {
    GetCounterCal: async (pts) => {
      const devK = pts[0] ? pts[0].usv / pts[0].cpm : 0, k = appK(), r = devK / k;
      return { appK: k, devK, source: "intercalibração", since: Date.now() - 15 * 3600e3, same: Math.abs(devK / k - 1) < 0.005, device: pts,
        suggested: pts.map((p) => ({ cpm: Math.round(p.cpm * r), usv: p.usv })), factory: keep.fabrica, previous: keep.anterior };
    },
    CalibrateCounter: async () => {
      const cfg = await api.GetDeviceConfig(), r = (cfg.calibration.points[0].usv / cfg.calibration.points[0].cpm) / appK();
      return write(cfg.calibration.points.map((p) => ({ cpm: Math.round(p.cpm * r), usv: p.usv })));
    },
    RestoreCounterCal: async (which) => write((which === "fabrica" ? keep.fabrica : keep.anterior).pontos.map((p) => ({ ...p }))),
  };
}
