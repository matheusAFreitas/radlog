"use strict";

// ---------- pulso da contagem (ideia 14, 2.54) ----------
//
// O anel do meio do mostrador pisca a cada contagem do aparelho, sem som:
// uma piscada = uma contagem, sempre (decisão do ACE). O logo pisca junto,
// com o aparelho do ícone da bandeja. Spec: ux/pulso.html.
//
// GMC-800: as N contagens do GETCPS (r.cps, a cada segundo) em instantes
// sorteados dentro do segundo seguinte. RC102: o aparelho manda a taxa
// (cps), então as contagens saem de um processo de Poisson com ela.
// Várias contagens no mesmo quadro viram uma piscada só. A piscada dura
// clamp(900 ms / taxa; 70; 400) e mexe só no stroke-opacity (0,35 → 1 → 0,35).
//
// Desliga com Configurações › Janela e tela, com o Windows sem efeitos de
// animação (prefers-reduced-motion) e com a janela escondida na bandeja.
// A fita "Os últimos 10 segundos" (opção própria, desligada) só aparece com
// o pulso ligado.
//
// Usa os globais do app.js (state, $, gmcConn, rcConn, TILE_LV). Carregado
// antes do app.js: só define funções.

const PU = {
  queue: [], last: 0, raf: 0, ticks: { gmc: [], radiacode: [] },
  rate: { gmc: 0, radiacode: 0 }, lastFlash: { gmc: 0, radiacode: 0 },
  reduced: false, preview: null,
};
const PU_TAPE_MS = 10000;
const PU_LV_VAR = { seguro: "--accent", atencao: "--att", alarme: "--alarm", perigo: "--warn" };

// as opções (a prévia do que está marcado em Configurações vale antes de salvar)
function pulseOn() {
  const p = PU.preview, s = state.settings || {};
  return !PU.reduced && !(p ? p.off : s.pulseOff);
}
function pulseTapeOn() {
  const p = PU.preview, s = state.settings || {};
  return pulseOn() && !!(p ? p.strip : s.pulseStrip);
}

// o aparelho está mandando contagens agora?
function pulseLive(dev) {
  if (dev === "gmc") {
    const r = state.last;
    return !!r && (r.demo || gmcConn() === "") && Date.now() - (r.time || 0) < 5000;
  }
  const st = (state.rc && state.rc.st) || {};
  return rcConn(st) === "" && st.cps > 0 && Date.now() - (st.time || 0) < 15000;
}
function pulseLevel(dev) {
  const el = $(dev === "gmc" ? "gTile" : "rTile");
  if (el) for (const l of ["perigo", "alarme", "atencao", "seguro"]) if (el.classList.contains("lv-" + l)) return l;
  return "seguro";
}
function pulseColor(dev) {
  return getComputedStyle(document.documentElement).getPropertyValue(PU_LV_VAR[pulseLevel(dev)] || "--accent").trim() || "#6EE7A0";
}
// aparelho do logo: o escolhido em Configurações (2.55.1) ou, em "igual
// à bandeja", o mesmo do ícone da bandeja (automático: o GMC-800, a
// não ser que o RC102 esteja num nível mais grave ou o GMC não esteja lendo)
function pulseLogoDev() {
  const p = PU.preview, s = state.settings || {};
  const own = p ? p.logo : s.pulseLogo || "";
  if (own === "nenhum") return "";
  const pick = own || s.trayDevice;
  if (pick === "gmc" || pick === "radiacode") return pulseLive(pick) ? pick : "";
  const g = pulseLive("gmc"), r = pulseLive("radiacode");
  if (g && r) {
    const rank = { seguro: 0, atencao: 1, alarme: 2, perigo: 3 };
    return rank[pulseLevel("radiacode")] > rank[pulseLevel("gmc")] ? "radiacode" : "gmc";
  }
  return g ? "gmc" : r ? "radiacode" : "";
}

// GMC-800: chamada a cada leitura (1 s), com as contagens do segundo
function pulseGmc(r) {
  if (!r) return;
  const n = Math.max(0, Math.round(r.cps || 0));
  PU.rate.gmc = n > 0 ? n : PU.rate.gmc * 0.5;
  if (!pulseOn() || document.hidden) { PU.queue.length = 0; return; }
  const now = performance.now();
  // muita contagem: não precisa de um instante para cada (vira uma piscada
  // por quadro); guarda no máximo uma por milissegundo
  const m = Math.min(n, 1000);
  for (let i = 0; i < m; i++) PU.queue.push({ t: now + Math.random() * 1000, w: n / m });
  PU.queue.sort((a, b) => a.t - b.t);
  pulseKick();
}

function pulsePoisson(l) {
  if (l > 30) return Math.max(0, Math.round(l + Math.sqrt(l) * Math.sqrt(-2 * Math.log(Math.random() + 1e-12)) * Math.cos(2 * Math.PI * Math.random())));
  const L = Math.exp(-l);
  let k = 0, p = 1;
  do { k++; p *= Math.random(); } while (p > L);
  return k - 1;
}

const puDur = (r) => Math.round(Math.max(70, Math.min(400, 900 / Math.max(r, 0.01))));

function pulseTargets(dev) {
  const out = [];
  const tile = $(dev === "gmc" ? "gTileDial" : "rTileDial");
  if (tile) { const c = tile.querySelector(".dial-ring"); if (c) out.push(c); }
  const hero = $(dev === "gmc" ? "gHeroDial" : "rHeroDial");
  if (hero) { const c = hero.querySelector(".dl-pulse"); if (c) out.push(c); }
  if (pulseLogoDev() === dev) document.querySelectorAll(".brand-o .brand-ring").forEach((c) => out.push(c));
  return out;
}

function pulseFlash(dev, now) {
  const d = puDur(PU.rate[dev]);
  for (const el of pulseTargets(dev)) {
    if (!el.animate || el.getClientRects().length === 0) continue;
    el.animate([{ strokeOpacity: 0.35 }, { strokeOpacity: 1, offset: 0.15 }, { strokeOpacity: 0.35 }], { duration: d, easing: "ease-out" });
  }
  PU.lastFlash[dev] = now;
}

function pulseKick() {
  if (!PU.raf && pulseOn() && !document.hidden) { PU.last = performance.now(); PU.raf = requestAnimationFrame(pulseLoop); }
}

function pulseLoop(now) {
  PU.raf = 0;
  if (!pulseOn() || document.hidden) { PU.queue.length = 0; pulseApply(); return; }
  const dt = Math.min(250, Math.max(0, now - PU.last));
  PU.last = now;
  const wall = Date.now();
  // GMC: as contagens sorteadas que já chegaram
  let g = 0;
  while (PU.queue.length && PU.queue[0].t <= now) g += PU.queue.shift().w;
  if (!pulseLive("gmc")) { g = 0; PU.queue.length = 0; }
  // RC102: Poisson com a taxa do aparelho
  const st = (state.rc && state.rc.st) || {};
  let r = 0;
  if (pulseLive("radiacode")) { PU.rate.radiacode = st.cps; r = pulsePoisson(st.cps * dt / 1000); }
  if (g > 0) { pulseFlash("gmc", now); PU.ticks.gmc.push([wall, g]); }
  if (r > 0) { pulseFlash("radiacode", now); PU.ticks.radiacode.push([wall, r]); }
  for (const k of ["gmc", "radiacode"]) {
    const a = PU.ticks[k];
    while (a.length && wall - a[0][0] > PU_TAPE_MS) a.shift();
  }
  if (pulseTapeOn()) pulseTapes(wall);
  if (pulseLive("gmc") || pulseLive("radiacode") || PU.queue.length) PU.raf = requestAnimationFrame(pulseLoop);
  else pulseApply();
}

// ---------- fita dos últimos 10 segundos ----------

const PU_TAPE_HOSTS = [["gTile", "gmc"], ["rTile", "radiacode"], ["hero", "gmc"], ["rcMain", "radiacode"]];

function pulseTapeEl(host, dev) {
  const h = $(host);
  if (!h) return null;
  let el = h.querySelector(":scope > .pu-tape");
  if (!el) {
    el = document.createElement("div");
    el.className = "pu-tape";
    el.dataset.dev = dev;
    el.title = dev === "gmc" ? "No GMC-800 cada piscada é uma contagem de verdade; só o instante dentro do segundo é sorteado."
      : "O RC102 manda a taxa, não cada contagem: o ritmo é o real, as piscadas são sorteadas a partir dele.";
    el.innerHTML = `<div class="pu-head small"><span>Os últimos 10 segundos</span><span class="muted">cada risco é uma contagem</span></div>` +
      `<canvas aria-hidden="true"></canvas><div class="pu-ax mono"><span>−10 s</span><span>−5 s</span><span>agora</span></div>`;
    h.appendChild(el);
  }
  return el;
}

function pulseTapes(wall) {
  for (const [host, dev] of PU_TAPE_HOSTS) {
    const el = pulseTapeEl(host, dev);
    if (!el || el.hidden || el.getClientRects().length === 0) continue;
    // o canvas tem exatamente pixels inteiros da tela (com escala de 125%
    // do Windows, 100% de largura dava meio pixel e o navegador esticava o
    // desenho, deixando riscos de larguras diferentes)
    const cv = el.querySelector("canvas"), dpr = window.devicePixelRatio || 1;
    const box = el.clientWidth || cv.parentNode.clientWidth;
    const w = Math.max(1, Math.floor(box * dpr)), hh = Math.max(1, Math.round(36 * dpr));
    if (cv.width !== w || cv.height !== hh) {
      cv.width = w; cv.height = hh;
      cv.style.width = w / dpr + "px"; cv.style.height = hh / dpr + "px";
    }
    const g = cv.getContext("2d");
    g.clearRect(0, 0, w, hh);
    g.fillStyle = getComputedStyle(document.documentElement).getPropertyValue("--border").trim() || "#20272C";
    g.fillRect(0, hh - Math.round(dpr), w, Math.round(dpr));
    // cada risco anda suave, de pixel em pixel da tela, sempre com a mesma
    // largura (posição arredondada para um pixel inteiro). A altura é a
    // mesma para todos; só numa taxa alta, com várias contagens na mesma
    // casa (risco + vão, presa ao relógio), ela passa a contar quantas foram.
    const lw = Math.max(1, Math.round(1.5 * dpr)), cell = lw + Math.max(1, Math.round(dpr));
    const pxMs = w / PU_TAPE_MS, cellMs = cell / pxMs;
    const cnt = new Map();
    let mx = 1;
    for (const [t, k] of PU.ticks[dev]) {
      const c = Math.floor(t / cellMs), v = (cnt.get(c) || 0) + k;
      cnt.set(c, v);
      if (v > mx) mx = v;
    }
    g.fillStyle = pulseColor(dev);
    const base = hh - Math.round(dpr), top = Math.round(4 * dpr), full = base - top;
    for (const [t, k] of PU.ticks[dev]) {
      const x = Math.round(w - (wall - t) * pxMs) - lw;
      if (x < 0 || x > w - lw) continue;
      const f = mx >= 4 ? 0.25 + 0.75 * Math.log1p(cnt.get(Math.floor(t / cellMs))) / Math.log1p(mx) : 1;
      const hgt = Math.max(Math.round(2 * dpr), Math.round(full * f));
      g.fillRect(x, base - hgt, lw, hgt);
    }
  }
}

// liga e desliga as classes (anel a 35% em repouso, fita à vista)
function pulseApply() {
  const on = pulseOn(), tape = pulseTapeOn();
  document.documentElement.classList.toggle("pulse-on", on);
  for (const dev of ["gmc", "radiacode"]) {
    const k = dev === "gmc" ? "g" : "r", live = on && pulseLive(dev);
    document.documentElement.classList.toggle("pulse-" + k, live);
    if (live) document.documentElement.style.setProperty("--pu-" + k, pulseColor(dev));
  }
  const logo = on ? pulseLogoDev() : "";
  document.querySelectorAll(".brand-o").forEach((el) => {
    el.classList.toggle("pulsing", !!logo);
    if (logo) el.style.setProperty("--pu-c", pulseColor(logo)); else el.style.removeProperty("--pu-c");
  });
  for (const [host, dev] of PU_TAPE_HOSTS) {
    const show = tape && pulseLive(dev);
    const el = show ? pulseTapeEl(host, dev) : $(host) && $(host).querySelector(":scope > .pu-tape");
    if (el) el.hidden = !show;
  }
  const strip = $("chkPulseStrip");
  if (strip) { strip.disabled = !on; strip.closest("label").classList.toggle("off", !on); }
  const sl = $("selPulseLogo");
  if (sl) sl.disabled = !on;
  if (on) pulseKick();
}

function pulseInit() {
  const mq = window.matchMedia ? matchMedia("(prefers-reduced-motion: reduce)") : null;
  PU.reduced = !!(mq && mq.matches);
  if (mq && mq.addEventListener) mq.addEventListener("change", (e) => { PU.reduced = e.matches; pulseApply(); });
  // o anel do meio do logo ganha uma classe para piscar
  document.querySelectorAll(".brand-o circle").forEach((c) => c.classList.add("brand-ring"));
  document.addEventListener("visibilitychange", () => { PU.queue.length = 0; pulseApply(); });
  const prev = () => {
    PU.preview = { off: !$("chkPulse").checked, strip: $("chkPulseStrip").checked, logo: $("selPulseLogo").value };
    pulseApply();
  };
  $("chkPulse").addEventListener("change", prev);
  $("chkPulseStrip").addEventListener("change", prev);
  $("selPulseLogo").addEventListener("change", prev);
  setInterval(pulseApply, 2000); // conectou, caiu, mudou de nível
}
document.addEventListener("DOMContentLoaded", pulseInit);
