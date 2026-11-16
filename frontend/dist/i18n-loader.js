"use strict";

// A tabela de tradução (i18n-en.js, ~700 KB) só é carregada com o app em
// inglês (2.83.10, auditoria 2.79.0 P3). A escolha do idioma é a mesma do
// i18n.js; trocar o idioma recarrega a tela.
// document.write mantém a ordem: a tabela roda antes do i18n.js.
{
  const pref = (() => { try { return localStorage.getItem("lang") || "auto"; } catch (e) { return "auto"; } })();
  const lang = pref === "auto" ? ((navigator.language || "pt").toLowerCase().startsWith("pt") ? "pt" : "en") : pref;
  if (lang === "en") document.write('<script src="i18n-en.js"><\/script>');
}
