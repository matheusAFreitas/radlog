"use strict";

// ---------- unidades da tela (2.84.0, revisão da interface) ----------
//
// Duas escolhas em Configurações › Janela e tela › Unidades, para o app todo:
// a taxa (µSv/h ou mrem/h; a dose acompanha, µSv ou mrem) e a contagem (CPM
// ou CPS). O app guarda e calcula tudo em µSv/h e CPM; só muda o que mostra.
// O que é do tubo (calibração, parte técnica) continua em CPM.
//
// Os textos montados com número e unidade juntos ("Alarme: 0,52 µSv/h") são
// trocados na tela pelo mesmo observador da tradução (i18n.js), com remText.
// Número e unidade separados (um <b> com o número e o texto da unidade ao
// lado) não são trocados: esses lugares usam rateNum/rateLbl. As regras são
// as mesmas de internal/app/unidades.go (remText).
// Trocar a unidade recarrega a tela (como o idioma); a escolha fica também no
// localStorage para a tela já abrir certa.

const UNITS = (() => {
  const get = (k, d) => { try { return localStorage.getItem(k) || d; } catch (e) { return d; } };
  return { rate: get("rateUnit", "usv") === "mrh" ? "mrh" : "usv", count: get("countUnit", "cps") === "cpm" ? "cpm" : "cps" };
})();
const RATE_MREM = UNITS.rate === "mrh";

// 2.97.0: formato da data, nos dois idiomas. O app escreve sempre dia/mês
// ("16/11", "16/11/2026"); com "iso" o observador da tela (i18n.js) troca
// para "11-16" e "2026-11-16". Mês/dia não existe, de propósito.
const DATE_ISO = (() => { try { return localStorage.getItem("dateFmt") === "iso"; } catch (e) { return false; } })();
const dateText = (() => {
  if (!DATE_ISO) return (s) => s;
  const re = /(?<![\d/.,:-])(\d{2})\/(\d{2})(?:\/(\d{4}))?(?![\d/])/g;
  return (s) => {
    if (typeof s !== "string" || s.indexOf("/") < 0) return s;
    return s.replace(re, (m, d, mo, y) => (+d < 1 || +d > 31 || +mo < 1 || +mo > 12 ? m : y ? `${y}-${mo}-${d}` : `${mo}-${d}`));
  };
})();

// fator da taxa e da dose na tela (µSv → mrem = ÷10)
const rateK = () => (RATE_MREM ? 0.1 : 1);
const rateLbl = () => (RATE_MREM ? "mrem/h" : "µSv/h");
const doseLbl = () => (RATE_MREM ? "mrem" : "µSv");
const countLbl = () => (UNITS.count === "cpm" ? "CPM" : "CPS");
// casas decimais para um valor em µSv/h (ou µSv) já na unidade da tela
const rateDigits = (usv) => { const v = Math.abs(usv); const d = v < 1 ? 3 : v < 10 ? 2 : 1; return RATE_MREM ? d + 1 : d; };
// número na unidade da tela (sem a unidade); d = casas para o valor em µSv
const rateNum = (usv, d) => (usv == null || !isFinite(usv) ? "—" : fmt(usv * rateK(), d == null ? rateDigits(usv) : RATE_MREM ? d + 1 : d));
const rateTxt = (usv, d) => `${rateNum(usv, d)} ${rateLbl()}`;
const doseNum = (usv, d) => (usv == null || !isFinite(usv) ? "—" : fmt(usv * rateK(), (d == null ? 2 : d) + (RATE_MREM ? 1 : 0)));
// contagem: o CPS é a média do minuto (CPM ÷ 60), a não ser que venha o do segundo
const countNum = (cpm, cps) => (cpm == null || !isFinite(cpm) ? "—" : UNITS.count === "cpm" ? fmt(cpm, 0) : fmt(cps != null ? cps : cpm / 60, 2));
const countTxt = (cpm, cps) => `${countNum(cpm, cps)} ${countLbl()}`;
// número de eixo de gráfico de taxa (sem unidade); d = casas para µSv/h
const axRate = (usv, d) => fmt(usv * rateK(), RATE_MREM ? d + 1 : d);
// campos digitados na unidade da tela (os limites dos alarmes do app)
const rateIn = (v) => { const x = parseFloat(v); return x > 0 ? x / rateK() : NaN; };
const rateOut = (usv) => (usv > 0 ? +(usv * rateK()).toPrecision(4) : "");

// textos com número: µSv → mrem (o mesmo de unidades.go)
const remText = (() => {
  if (!RATE_MREM) return (s) => s;
  const re = /([+±−-]?\d[\d.,]*)((?:\s?(?:–|-|±|a|e|até|ou|to|and|or)\s?[+±−-]?\d[\d.,]*)*)(\s| )?(µSv|mSv)(\/h|\/ano|\/dia|\/year|\/day|\b|(?=[^A-Za-z]|$))/g;
  const paren = /(\d\s?)µSv\/h \(([^()]{0,40}?\s)(\d[\d.,]*\d|\d)\)/g;
  const num = (n, k) => {
    const sep = typeof LANG !== "undefined" && LANG === "en" ? "." : ",", th = sep === "." ? "," : ".";
    let sign = "";
    if (/^[+±−-]/.test(n)) { sign = n[0]; n = n.slice(1); }
    n = n.replace(/[.,]+$/, "");
    const clean = n.split(th).join("");
    const i = clean.indexOf(sep);
    let dec = i >= 0 ? clean.length - i - 1 : 0;
    const v = parseFloat(clean.replace(sep, "."));
    if (!isFinite(v)) return sign + n;
    dec = k < 1 ? dec + 1 : Math.max(0, dec - 2);
    let out = (v * k).toFixed(dec);
    if (sep !== ".") out = out.replace(".", sep);
    return sign + out;
  };
  return (s) => {
    if (typeof s !== "string" || s.indexOf("Sv") < 0) return s;
    return s.replace(paren, "$1µSv/h ($2$3 µSv/h)").replace(re, (m, a, b, sp, u, suf) => {
      const k = u === "mSv" ? 100 : 0.1;
      return (a + b).replace(/[+±−-]?\d[\d.,]*/g, (n) => num(n, k)) + (sp || "") + "mrem" + suf;
    });
  };
})();

// rótulos de unidade sem número no HTML: o elemento marcado com
// data-u="rate" (µSv/h), "dose" (µSv) ou "count" (CPM) troca a unidade no
// texto dele. Roda no observador da tradução (i18n.js), depois dela, para a
// frase inteira continuar sendo traduzida.
const UNIT_WORDS = !RATE_MREM && UNITS.count === "cpm" ? null : {
  rate: (s) => (RATE_MREM ? s.replace(/µSv\/h/g, "mrem/h") : s),
  dose: (s) => (RATE_MREM ? s.replace(/µSv(?!\/)/g, "mrem") : s),
  count: (s) => (UNITS.count === "cps" ? s.replace(/\bCPM\b/g, "CPS") : s),
};
function unitWord(s, el) {
  const u = UNIT_WORDS && el && el.closest && el.closest("[data-u]");
  const f = u && UNIT_WORDS[u.dataset.u];
  return f ? f(s) : s;
}

// guarda a escolha (vinda das Configurações) e recarrega se mudou
function syncUnits(rate, count, date) {
  rate = rate === "mrh" ? "mrh" : "usv";
  count = count === "cpm" ? "cpm" : "cps";
  const iso = date === "iso";
  if (rate === UNITS.rate && count === UNITS.count && iso === DATE_ISO) return false;
  try {
    localStorage.setItem("rateUnit", rate);
    localStorage.setItem("countUnit", count);
    localStorage.setItem("dateFmt", iso ? "iso" : "");
    if (localStorage.getItem("rateUnit") !== rate) return false; // sem localStorage: não recarrega em volta
  } catch (e) { return false; }
  location.reload();
  return true;
}
