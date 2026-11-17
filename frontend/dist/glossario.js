// ---------- glossário (2.96.0, avaliação T2) ----------
//
// A frase simples vem na frente; o termo técnico fica sublinhado e abre a
// explicação ao clicar. Um glossário só para o app todo: qualquer elemento
// com data-gl="<termo>" vira clicável. O texto é posto com textContent e o
// tradutor da tela (i18n.js) troca para o inglês. Carregado antes do app.js.

const GLOSS = {
  sigma: ["σ (sigma)", "O tamanho do vai e vem natural da contagem. Até 2σ é acaso. De 3σ para cima, a mudança é real."],
  fundo: ["Fundo", "A radiação natural de sempre no lugar: solo, paredes, radônio do ar e raios cósmicos. Não dá para zerar, e é normal."],
  mediana: ["Mediana", "O valor do meio: metade das leituras fica abaixo, metade acima. Um pico curto não mexe nela, por isso serve para medir o fundo."],
  quartil: ["Quartis", "Cortam as leituras em quatro partes iguais. Entre o 1º e o 3º quartil ficam as leituras do meio, as mais comuns."],
  dispersao: ["Dispersão", "Quanto as leituras variam comparado com o que o acaso explica. Perto de 1 é normal. Bem acima de 1, alguma coisa mexeu de verdade (radônio, chuva, o aparelho)."],
  poisson: ["Poisson", "A conta do acaso para coisas que acontecem soltas no tempo, como os decaimentos. Diz quanto a contagem varia sozinha."],
  dureza: ["Dureza", "Quanto do espectro está nas energias altas. Sobe com fontes mais fortes em gama alta (tório, rádio) e atrás de blindagem, que segura mais as energias baixas."],
  janela: ["Janela em formação", "O app ainda está juntando dias para saber o seu normal. Até completar, as comparações são só uma pista."],
  saturando: ["Saturando", "Radiação tão alta que o tubo não consegue contar tudo: o número fica abaixo do real. Afaste-se e confie no RC102."],
  curva: ["Curva do contador", "A tabela de fábrica do GMC-800 que transforma CPM em µSv/h. A intercalibração com o RC102 pode trocar o fator."],
  cpm: ["CPM e CPS", "Contagens por minuto e por segundo: quantos pulsos o aparelho viu. É o que ele mede de verdade; o µSv/h é uma conta feita em cima disso."],
  incerteza: ["Incerteza (±)", "Quanto o número pode estar fora por causa do acaso. Medir mais tempo diminui a incerteza."],
};

function glClose() {
  const p = $("glPop");
  if (p) p.hidden = true;
  document.querySelectorAll("[data-gl][aria-expanded=true]").forEach((x) => x.setAttribute("aria-expanded", "false"));
}

function glOpen(el) {
  const g = GLOSS[el.dataset.gl];
  if (!g) return;
  let p = $("glPop");
  if (!p) {
    p = document.createElement("div");
    p.id = "glPop";
    p.className = "gl-pop";
    p.setAttribute("role", "dialog");
    p.innerHTML = '<b class="gl-t"></b><p class="gl-x"></p><button type="button" class="link small-link" id="glAll">Todos os termos</button>';
    document.body.appendChild(p);
    $("glAll").addEventListener("click", () => { glClose(); glOpenAll(); });
  }
  glClose();
  p.querySelector(".gl-t").textContent = g[0];
  p.querySelector(".gl-x").textContent = g[1];
  p.setAttribute("aria-label", g[0]);
  p.hidden = false;
  el.setAttribute("aria-expanded", "true");
  const r = el.getBoundingClientRect(), w = p.offsetWidth, h = p.offsetHeight;
  const left = Math.max(8, Math.min(window.innerWidth - w - 8, r.left));
  const below = r.bottom + 6 + h < window.innerHeight;
  p.style.left = left + "px";
  p.style.top = (below ? r.bottom + 6 : Math.max(8, r.top - h - 6)) + "px";
}

// a lista inteira, num diálogo
function glOpenAll() {
  const d = $("glDlg");
  const box = $("glList");
  box.innerHTML = "";
  for (const k of Object.keys(GLOSS)) {
    const dt = document.createElement("dt"), dd = document.createElement("dd");
    dt.textContent = GLOSS[k][0];
    dd.textContent = GLOSS[k][1];
    box.append(dt, dd);
  }
  d.showModal();
}

// marca um elemento como termo do glossário (para o que é montado no JS)
function glMark(el, key) {
  if (!el || !GLOSS[key]) return el;
  el.dataset.gl = key;
  el.setAttribute("role", "button");
  el.setAttribute("tabindex", "0");
  el.setAttribute("aria-haspopup", "dialog");
  return el;
}

// põe " · <termo>" no fim de um texto já escrito, com o termo clicável
function glAppend(el, text, key) {
  el.append(" · ");
  el.appendChild(glMark(Object.assign(document.createElement("span"), { textContent: text }), key));
}

document.addEventListener("click", (e) => {
  const t = e.target.closest && e.target.closest("[data-gl]");
  if (t) { e.preventDefault(); e.stopPropagation(); glOpen(t); return; }
  const p = $("glPop");
  if (p && !p.hidden && !p.contains(e.target)) glClose();
}, true);
document.addEventListener("keydown", (e) => {
  if (e.key === "Escape") { glClose(); return; }
  const t = e.target.closest && e.target.closest("[data-gl]");
  if (t && (e.key === "Enter" || e.key === " ")) { e.preventDefault(); glOpen(t); }
});
window.addEventListener("scroll", glClose, true);
// o que vem escrito no index.html com data-gl ganha o mesmo jeito de botão
document.addEventListener("DOMContentLoaded", () => document.querySelectorAll("[data-gl]").forEach((el) => glMark(el, el.dataset.gl)));
