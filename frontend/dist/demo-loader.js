"use strict";

// O modo demonstração (demo.js, ~1.500 linhas) serve no navegador, sem o
// Go por trás. Dentro do app ele só é carregado com a demonstração ligada
// (auditoria 2.76.0, P3).
// document.write mantém a ordem: o demo.js roda antes do app.js.
// 2.82: dentro do app, a demonstração ligada também usa o demo.js (o mundo
// simulado); a tela recarrega ao ligar e ao sair.
{
  const wanted = (() => { try { return sessionStorage.getItem("radlogDemo") === "1"; } catch (e) { return false; } })();
  if (!(window.go && (window.go.app || window.go.main)) || wanted) {
    document.write('<script src="demo.js"><\/script>');
  }
}
