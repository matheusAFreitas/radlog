// ---------- Clima: pressão × radônio (2.44) ----------
//
// Na tela Clima (antiga Chuva), embaixo da chuva: a pressão do ar e o fundo
// em casa da última semana, em dois gráficos empilhados (sem eixo duplo),
// com o mesmo cursor, e a frase da correlação (GetPressureRadon, pressure.go).
// Usa o aparelho escolhido no seletor da chuva. Carregado antes do app.js.

const PR = { data: null, busy: false, at: 0, dev: "" };

async function refreshPressure(force) {
  if (state.view !== "chuva" || !api.GetPressureRadon || PR.busy) return;
  const box = $("clPress");
  if (!state.settings.rainEnabled) { box.hidden = true; return; }
  box.hidden = false;
  const dev = riDev();
  if (!force && PR.data && PR.dev === dev && Date.now() - PR.at < 9 * 60e3) return;
  PR.busy = true;
  PR.at = Date.now();
  PR.dev = dev;
  let r;
  try { r = await api.GetPressureRadon(dev); } catch (e) { r = null; } finally { PR.busy = false; }
  if (!r || dev !== riDev()) return;
  PR.data = r;
  renderPressure(r);
}

function renderPressure(r) {
  const devTxt = r.device === "radiacode" ? "RC102" : "GMC-800";
  $("prSub").textContent = `Últimos 7 dias · em casa · ${devTxt}`;
  const hasP = r.hours.some((h) => h.hPa != null), hasB = r.hours.some((h) => h.usv != null);
  $("prBox").hidden = !(hasP || hasB);
  if (hasP || hasB) drawPressCharts(r);
  const ver = $("prVer");
  ver.hidden = !r.title;
  ver.className = "al-verdict" + (r.status === "pista" || r.status === "relacao" ? (Math.abs(r.r) >= 0.2 ? " ok" : " meh") : " meh");
  ver.textContent = "";
  const txt = document.createElement("span");
  String(r.text || "").split(/(?<=\.)\s+(?=[A-ZÀ-Ú])/).forEach((x, i) => { if (i) txt.append(" "); txt.append(Object.assign(document.createElement("span"), { textContent: x })); });
  ver.append(Object.assign(document.createElement("b"), { textContent: r.title }), txt);
}

function drawPressCharts(r) {
  const H = r.hours, N = H.length;
  const W = 640, L = 56, R = 12;
  const X = (i) => L + (i / Math.max(1, N - 1)) * (W - L - R);
  const wd = (t) => new Date(t).toLocaleDateString(LOCALE, { weekday: "short" }).replace(".", "");
  const one = (id, key, top, lab, col, dec) => {
    const svg = $(id), Ht = top ? 150 : 176, T = 20, B = top ? 12 : 30;
    const vals = H.map((h) => h[key]).filter((v) => v != null);
    let mn = Math.min(...vals), mx = Math.max(...vals);
    if (!vals.length) { mn = 0; mx = 1; }
    if (mx - mn < 1e-9) { mn -= 1; mx += 1; }
    const pad = (mx - mn) * 0.12; mn -= pad; mx += pad;
    const Y = (v) => T + (1 - (v - mn) / (mx - mn)) * (Ht - T - B);
    let s = `<text x="${L}" y="13" class="al-lb" style="font-size:12px">${lab}</text>`;
    for (let k = 0; k <= 3; k++) {
      const v = mn + ((mx - mn) * k) / 3, y = Y(v).toFixed(1);
      s += `<line x1="${L}" x2="${W - R}" y1="${y}" y2="${y}" stroke="var(--chart-grid)"/><text x="${L - 8}" y="${(+y + 4).toFixed(1)}" class="al-ax" text-anchor="end">${dec ? fmt(v, dec) : String(Math.round(v))}</text>`;
    }
    // meia-noite de cada dia
    H.forEach((h, i) => {
      const d = new Date(h.t);
      if (d.getHours() !== 0) return;
      const x = X(i).toFixed(1);
      s += `<line x1="${x}" x2="${x}" y1="${T}" y2="${Ht - B}" stroke="var(--chart-grid)" stroke-dasharray="2 4"/>`;
      if (!top && i + 12 < N) s += `<text x="${X(i + 12).toFixed(1)}" y="${Ht - 10}" class="al-ax" text-anchor="middle">${wd(h.t)}</text>`;
    });
    let d = "", pen = false;
    H.forEach((h, i) => {
      if (h[key] == null) { pen = false; return; }
      d += (pen ? "L" : "M") + X(i).toFixed(1) + " " + Y(h[key]).toFixed(1);
      pen = true;
    });
    s += `<path d="${d}" fill="none" stroke="${col}" stroke-width="2" stroke-linejoin="round"/>`;
    if (!vals.length) s += `<text x="${(L + W - R) / 2}" y="${(T + Ht - B) / 2}" class="al-lb" text-anchor="middle">sem dado</text>`;
    s += `<line class="pr-cx" x1="0" x2="0" y1="${T}" y2="${Ht - B}" stroke="var(--muted)" visibility="hidden"/><circle class="pr-cd" r="4" fill="${col}" stroke="var(--surface)" stroke-width="2" visibility="hidden"/>`;
    s += `<rect class="pr-hit" x="${L}" y="0" width="${W - L - R}" height="${Ht}" fill="transparent"/>`;
    svg.innerHTML = s;
    return { svg, Y, key };
  };
  const unit = r.device === "radiacode" ? "RC102" : "GMC-800";
  const a = one("prPChart", "hPa", true, "Pressão do ar (hPa)", "var(--info, #7CC7FF)", 0);
  const b = one("prRChart", "usv", false, `Fundo em casa pelo ${unit} (µSv/h, mediana de 1 h)`, "var(--accent)", 3);
  const tip = $("prTip");
  const move = (ev) => {
    const bb = ev.currentTarget.ownerSVGElement.getBoundingClientRect(), sx = W / bb.width;
    const i = Math.max(0, Math.min(N - 1, Math.round((((ev.clientX - bb.left) * sx - L) / (W - L - R)) * (N - 1))));
    const h = H[i], x = X(i);
    for (const c of [a, b]) {
      const cx = c.svg.querySelector(".pr-cx"), cd = c.svg.querySelector(".pr-cd");
      cx.setAttribute("x1", x); cx.setAttribute("x2", x); cx.setAttribute("visibility", "visible");
      if (h[c.key] != null) { cd.setAttribute("cx", x); cd.setAttribute("cy", c.Y(h[c.key])); cd.setAttribute("visibility", "visible"); } else cd.setAttribute("visibility", "hidden");
    }
    tip.hidden = false;
    tip.textContent = `${wd(h.t)} ${hhmm(h.t)} · ` + (h.hPa != null ? `${fmt(h.hPa, 1)} hPa` : "sem pressão") + " · " + (h.usv != null ? `${fmt(h.usv, 3)} µSv/h` : "sem dado");
    const wrap = $("prBox").getBoundingClientRect();
    tip.style.left = Math.max(0, Math.min(wrap.width - tip.offsetWidth - 4, x / sx + 10)) + "px";
    tip.style.top = (a.svg.getBoundingClientRect().height - 10) + "px";
  };
  const out = () => {
    tip.hidden = true;
    for (const c of [a, b]) c.svg.querySelectorAll(".pr-cx,.pr-cd").forEach((e) => e.setAttribute("visibility", "hidden"));
  };
  for (const c of [a, b]) {
    const hit = c.svg.querySelector(".pr-hit");
    hit.addEventListener("mousemove", move);
    hit.addEventListener("mouseleave", out);
  }
}
