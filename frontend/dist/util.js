// Funções pequenas usadas por todas as telas. Carrega antes dos outros
// scripts (index.html).

// escHtml escapa um texto para entrar em innerHTML. É a única cópia: antes
// havia cinco iguais, uma por arquivo (auditoria de 12/11).
function escHtml(s) {
  return String(s).replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c]);
}

// versão de teste da CSP: mostra na tela o que a política bloquear
document.addEventListener("securitypolicyviolation", (e) => {
  const msg = `CSP bloqueou: ${e.violatedDirective} ${e.blockedURI || ""} ${e.sourceFile ? e.sourceFile.split("/").pop() + ":" + e.lineNumber : ""}`;
  console.warn(msg);
  const el = document.createElement("div");
  el.className = "toast err";
  el.textContent = msg;
  (document.getElementById("toasts") || document.body).appendChild(el);
});
