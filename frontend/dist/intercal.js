// Intercalibração com o RadiaCode e vigia semanal (ideia 6).
//
// Configurações › Calibração do GMC ganha o bloco da intercalibração: o botão
// começa uma coleta de 24 h com os dois aparelhos encostados; no fim aparece
// o resultado com a mesma conta da análise de 09/11 (core/intercal) e o botão
// para usar o fator novo. A vigia compara, toda semana, a razão de contagens
// dos dois em casa com o normal deles; a frase aparece na Calibração e na
// página do GMC-800, e o aviso de "se afastando" vai para Alertas como
// manutenção (não é alerta de radiação).

const IC_WATCH_CLS = { concordam: "ok", "uma-semana": "warn", afastando: "bad" };

function icWhen(t) {
  const d = new Date(t);
  return d.toLocaleDateString(LOCALE, { day: "2-digit", month: "2-digit" }) + " " + d.toLocaleTimeString(LOCALE, { hour: "2-digit", minute: "2-digit" });
}

// fator de cada hora com a margem, e a faixa do fator novo (±95%)
function icHoursSvg(r) {
  const hs = r.hours || [];
  if (hs.length < 3) return "";
  const W = 560, H = 150, L = 52, R = 8, T = 10, B = 22;
  const ys = hs.flatMap((h) => [h.k - h.err, h.k + h.err]).concat([r.lo, r.hi]);
  let y0 = Math.min(...ys), y1 = Math.max(...ys);
  const pad = (y1 - y0) * 0.08 || r.k * 0.05;
  y0 -= pad; y1 += pad;
  const t0 = new Date(hs[0].t).getTime(), t1 = new Date(hs[hs.length - 1].t).getTime() || t0 + 1;
  const x = (t) => L + ((new Date(t).getTime() - t0) / Math.max(1, t1 - t0)) * (W - L - R);
  const y = (v) => T + (1 - (v - y0) / (y1 - y0)) * (H - T - B);
  const ticks = [y0 + pad, r.k, y1 - pad];
  let s = `<svg class="ic-chart" viewBox="0 0 ${W} ${H}" role="img" aria-label="Fator de cada hora da coleta">`;
  s += `<rect class="ic-band" x="${L}" y="${y(r.hi)}" width="${W - L - R}" height="${Math.max(1, y(r.lo) - y(r.hi))}"/>`;
  s += `<line class="ic-mid" x1="${L}" x2="${W - R}" y1="${y(r.k)}" y2="${y(r.k)}"/>`;
  for (const v of ticks) s += `<text class="ic-tick" x="${L - 6}" y="${y(v) + 3}" text-anchor="end">${fmt(v, 5)}</text>`;
  for (const h of hs) {
    const cx = x(h.t);
    s += `<line class="ic-err" x1="${cx}" x2="${cx}" y1="${y(h.k + h.err)}" y2="${y(h.k - h.err)}"/><circle class="ic-dot" cx="${cx}" cy="${y(h.k)}" r="3"><title>${icWhen(h.t)} · ${fmt(h.k, 5)} ± ${fmt(h.err, 5)}</title></circle>`;
  }
  const lab = (t) => new Date(t).toLocaleTimeString(LOCALE, { hour: "2-digit", minute: "2-digit" });
  s += `<text class="ic-tick" x="${L}" y="${H - 6}">${lab(hs[0].t)}</text><text class="ic-tick" x="${W - R}" y="${H - 6}" text-anchor="end">${lab(hs[hs.length - 1].t)}</text>`;
  return s + "</svg>";
}

function icResultHtml(v) {
  const r = v.result, k0 = v.k0 || v.curK, pct = v.pct;
  const dir = pct >= 0 ? "acima" : "abaixo";
  const tubeOk = Math.abs(r.dispersion - 1) <= Math.max(0.1, 3 * r.dispersionErr);
  const k0Usv = r.gmcCpm * k0, kUsv = r.gmcCpm * r.k;
  let h = `<div class="ic-head"><span class="ic-big mono">${pct >= 0 ? "+" : "−"}${fmt(Math.abs(pct), 0)}%</span>` +
    `<span>O GMC-800 estava lendo ${fmt(Math.abs(pct), 0)}% ${dir} do RadiaCode</span></div>` +
    `<p class="small muted">Nas ${fmt(r.minutes / 60, 0)} h com os dois lado a lado, o mesmo fundo deu ${fmt(k0Usv, 3)} µSv/h no GMC-800 e ${fmt(r.rcUsvh, 3)} µSv/h no RadiaCode. O RadiaCode pesa cada contagem pela energia; o GMC usa uma curva de fábrica.</p>` +
    `<dl class="info ic-facts">` +
    `<dt>fator novo</dt><dd class="mono">${fmt(r.k, 5)} µSv/h por CPM · hoje ${fmt(k0, 5)}</dd>` +
    `<dt>margem (95%)</dt><dd class="mono">entre ${fmt(r.lo, 5)} e ${fmt(r.hi, 5)} (±${fmt(r.rel95 * 100, 1)} %)</dd>` +
    `<dt>coleta</dt><dd class="mono">${fmt(r.minutes, 0)} de ${fmt(v.need, 0)} minutos com os dois${r.outliers ? ` · ${fmt(r.outliers, 0)} fora da curva ficaram de fora` : ""}</dd>` +
    `<dt>tubo do GMC</dt><dd class="mono">${fmt(r.dispersion, 2)} variância/média · ${tubeOk ? "saudável (≈ 1)" : "fora do esperado (≈ 1)"}</dd>` +
    `</dl>`;
  const svg = icHoursSvg(r);
  if (svg) h += `<h4 class="ic-sub">Hora a hora</h4><p class="small muted">Cada ponto é o fator de uma hora, com a margem dele. A faixa é o fator novo, tirado da coleta inteira.</p>${svg}`;
  h += `<h4 class="ic-sub">O que muda se aplicar</h4><table class="cfg-hist ic-cmp"><thead><tr><th></th><th>GMC-800 hoje</th><th>com o fator novo</th></tr></thead><tbody>` +
    `<tr><td>fundo da casa</td><td class="num mono">${fmt(k0Usv, 3)} µSv/h</td><td class="num mono">${fmt(kUsv, 3)} µSv/h</td></tr>` +
    `<tr><td>dose do GMC por dia</td><td class="num mono">${fmt(k0Usv * 24, 1)} µSv</td><td class="num mono">${fmt(kUsv * 24, 1)} µSv</td></tr></tbody></table>` +
    `<p class="small muted">O contador não é mexido: o fator vale só no RadLog. Vira uma versão nova na tabela de calibração; dá para desfazer, e recalcular o histórico é opcional.</p>`;
  if (v.applied) {
    h += `<p class="small ic-done">${lvIcon("seguro", 12)} Em uso a partir da aplicação.</p><div class="row"><button type="button" class="btn small" data-ic="again">Intercalibrar de novo</button></div>`;
  } else {
    h += `<div class="row"><button type="button" class="btn accent" data-ic="apply">Usar ${fmt(r.k, 5)} a partir de agora</button>` +
      (v.skipped ? `<button type="button" class="btn" data-ic="forget">Esquecer este resultado</button>` : `<button type="button" class="btn" data-ic="skip">Agora não</button>`) + `</div>`;
  }
  return h;
}

async function renderIntercal() {
  const box = $("icBox");
  if (!box || !api.GetIntercal) return;
  let v;
  try { v = await api.GetIntercal(); } catch (e) { return; }
  state.intercal = v;
  if (v.state === "coletando") {
    const pc = Math.min(100, (v.minutes / v.need) * 100);
    box.innerHTML = `<p><strong>Coletando.</strong> ${fmt(v.minutes, 0)} de ${fmt(v.need, 0)} minutos com os dois · termina em ${icWhen(v.end)}.</p>` +
      `<div class="ic-bar" role="progressbar" aria-valuemin="0" aria-valuemax="${v.need}" aria-valuenow="${v.minutes}"><i style="width:${pc}%"></i></div>` +
      `<p class="small muted">Deixe os dois aparelhos encostados, na mesma altura, sem um blindar o outro. Se mexer em um deles, cancele e comece de novo.</p>` +
      `<div class="row"><button type="button" class="btn small" data-ic="cancel">Cancelar a coleta</button></div>`;
  } else if (v.state === "poucos") {
    box.innerHTML = `<p>A coleta terminou com só ${fmt(v.minutes, 0)} minutos com os dois aparelhos medindo (precisa de pelo menos 60). Confira se o RadiaCode ficou conectado.</p>` +
      `<div class="row"><button type="button" class="btn small" data-ic="again">Começar de novo</button><button type="button" class="btn small" data-ic="forget">Fechar</button></div>`;
  } else if (v.state === "pronta" && v.result) {
    box.innerHTML = icResultHtml(v);
  } else {
    box.innerHTML = `<p class="small muted">Deixe os dois aparelhos encostados, na mesma altura, sem um blindar o outro, por 24 h. O app faz a conta e sugere o fator novo do GMC, com o RadiaCode como referência. Vale repetir a cada seis meses, ou quando mudar o GMC de lugar.</p>` +
      `<div class="row"><button type="button" class="btn" data-ic="start">Intercalibrar com o RadiaCode</button></div>`;
  }
}

async function renderConcord() {
  if (!api.GetConcord) return;
  let c;
  try { c = await api.GetConcord(); } catch (e) { return; }
  state.concord = c;
  const cls = "ic-watch " + (IC_WATCH_CLS[c.state] || "dim");
  for (const id of ["icWatch", "gmcConcord"]) {
    const el = $(id);
    if (!el) continue;
    el.className = cls;
    el.textContent = c.text || "";
    el.hidden = !c.text;
  }
  const m = $("icMaint");
  if (m) {
    m.hidden = c.state !== "afastando";
    if (!m.hidden) {
      m.innerHTML = `<strong>Manutenção.</strong> <span></span> <button type="button" class="link small-link" data-icopen="1">Abrir a calibração</button>`;
      m.querySelector("span").textContent = c.text;
    }
  }
}

function initIntercal() {
  const box = $("icBox");
  if (!box) return;
  box.addEventListener("click", async (e) => {
    const b = e.target.closest("button[data-ic]");
    if (!b) return;
    const k = b.dataset.ic;
    try {
      if (k === "start" || k === "again") {
        if (k === "again") await call(api.CancelIntercal);
        await call(api.StartIntercal);
        toast("Coleta de 24 h começou");
      } else if (k === "cancel" || k === "forget") {
        await call(api.CancelIntercal);
      } else if (k === "skip") {
        await call(api.ApplyIntercal, true);
      } else if (k === "apply") {
        await call(api.ApplyIntercal, false);
        toast("Calibração nova a partir de agora");
        await calHistChanged();
      }
    } catch (err) { return; }
    renderIntercal();
    renderConcord();
  });
  // a coleta anda sozinha: atualiza a cada minuto enquanto a tela está aberta
  setInterval(() => { if (state.view === "config" && state.intercal && state.intercal.state === "coletando") renderIntercal(); }, 60000);
  const m = $("icMaint");
  if (m) m.addEventListener("click", (e) => { if (e.target.closest("[data-icopen]")) openSettings("icBox"); });
  renderConcord();
  setInterval(renderConcord, 30 * 60000);
}

// modo navegador: a coleta termina na hora, com os números de 09/11
function demoIntercal() {
  let d = { state: "parado" };
  const res = () => {
    const t0 = Date.now() - 864e5, hours = [];
    for (let i = 0; i < 24; i++) {
      const k = 0.00564 * (1 + 0.045 * Math.sin((i - 6) / 24 * 2 * Math.PI) * -1 + 0.01 * Math.sin(i * 1.7));
      hours.push({ t: new Date(t0 + i * 36e5).toISOString(), min: 60, k, err: k * 0.028 });
    }
    return { k: 0.00564, rel: 0.0075, rel95: 0.015, lo: 0.00555, hi: 0.00573, minutes: 1440, outliers: 0, gmcCpm: 26.8, rcUsvh: 0.151, dispersion: 1.02, dispersionErr: 0.037, countRatio: 28.1, hours };
  };
  const view = () => ({ need: 1440, curK: 0.01014, k0: 0.01014, start: d.start || 0, end: d.end || 0, minutes: d.state === "pronta" ? 1440 : 0, pct: d.state === "pronta" ? (0.01014 / 0.00564 - 1) * 100 : 0, ...d });
  return {
    GetIntercal: async () => view(),
    StartIntercal: async () => { d = { state: "pronta", start: Date.now() - 864e5, end: Date.now(), result: res() }; return view(); },
    CancelIntercal: async () => { d = { state: "parado" }; },
    ApplyIntercal: async (skip) => { if (skip) d.skipped = true; else { d.applied = true; await api.SetGMCFactor(0.00564, "com o RadiaCode, coleta de " + new Date(d.start).toLocaleDateString(LOCALE, { day: "2-digit", month: "2-digit" }), "intercalibração"); } return view(); },
    GetConcord: async () => ({ state: "aprendendo", learnedMin: 1260, text: "Aprendendo o normal dos dois em casa: 21 h juntos até agora; a vigia começa em " + new Date(Date.now() + 27 * 864e5).toLocaleDateString(LOCALE, { day: "2-digit", month: "2-digit" }) + "." }),
  };
}
