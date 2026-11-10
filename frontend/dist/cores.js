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
  });
})();
