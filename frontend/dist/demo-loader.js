"use strict";

// O modo demonstração (demo.js, ~1.500 linhas) só serve no navegador, sem o
// Go por trás. Dentro do app ele não é carregado (auditoria 2.76.0, P3).
// document.write mantém a ordem: o demo.js roda antes do app.js.
if (!(window.go && (window.go.app || window.go.main))) {
  document.write('<script src="demo.js"><\/script>');
}
