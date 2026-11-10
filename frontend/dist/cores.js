// Cores do guia de estilo para o JS (gráficos em SVG e canvas).
// A fonte é o :root do style.css; os valores abaixo só valem se a variável
// não puder ser lida. Carregado antes dos outros scripts da interface.
const COR = (() => {
  const css = getComputedStyle(document.documentElement);
  const v = (name, fallback) => css.getPropertyValue("--" + name).trim() || fallback;
  return Object.freeze({
    accent: v("accent", "#6EE7A0"), // GMC-800 e destaque
    rc: v("rc", "#F472B6"), // RadiaCode
    att: v("att", "#F5B84A"), // atenção
    alarm: v("alarm", "#F28C38"), // alarme
    warn: v("warn", "#F0656A"), // perigo
    text: v("text", "#E9EDEF"),
    muted: v("muted", "#A3ADB4"),
    grid: v("chart-grid", "#2A3136"), // linhas de grade dos gráficos
    away: v("away", "#B79CFF"), // fora de casa / fora do lugar
    obj: v("obj", "#5EEAD4"), // Medir objeto
    info: v("info", "#7CC7FF"), // chuva, informação
    note: v("note", "#A3ADB4"), // anotação (cinza)
  });
})();

// Forma de cada nível (ideia 12): o nível aparece com forma e cor em todo
// lugar, para não depender só da cor. Desenhos numa caixa 16 × 16,
// preenchidos com currentColor; a cor vem da classe .lv-ico.<nível>.
const LV_PATH = {
  seguro: '<circle cx="8" cy="8" r="6.5"/>',
  atencao: '<path d="M8 1.2 15.2 14.4H.8Z"/>',
  alarme: '<path d="M8 .6 15.4 8 8 15.4.6 8Z"/>',
  perigo: '<path fill-rule="evenodd" d="M5.1 1h5.8L15 5.1v5.8L10.9 15H5.1L1 10.9V5.1ZM4 6.9v2.2h8V6.9Z"/>',
  nocount: '<path d="M8 2.6 14 13.6H2Z" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/>',
};

// SVG do nível. Sem label é decorativo (a palavra já está ao lado); com
// label vira role="img" (quando a forma aparece sozinha).
function lvIcon(level, size = 12, label = "") {
  const p = LV_PATH[level];
  if (!p) return "";
  const esc = (s) => String(s).replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c]);
  return `<svg class="lv-ico ${level}" width="${size}" height="${size}" viewBox="0 0 16 16" fill="currentColor"` +
    (label ? ` role="img" aria-label="${esc(label)}"` : ' aria-hidden="true"') + `>${p}</svg>`;
}

// Põe a forma antes do texto num elemento (substitui o conteúdo).
// Sem texto, a forma vai sozinha e ganha nome ("Nível: Alarme").
const LV_NAME = { seguro: "Seguro", atencao: "Atenção", alarme: "Alarme", perigo: "Perigo", nocount: "Sem contagens" };
function lvLabel(el, level, text, size = 12) {
  el.textContent = text;
  if (LV_PATH[level]) el.insertAdjacentHTML("afterbegin", lvIcon(level, size, text ? "" : "Nível: " + LV_NAME[level]));
}
