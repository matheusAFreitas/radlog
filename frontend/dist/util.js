// Funções pequenas usadas por todas as telas. Carrega antes dos outros
// scripts (index.html).

// escHtml escapa um texto para entrar em innerHTML. É a única cópia: antes
// havia cinco iguais, uma por arquivo (auditoria de 12/11).
function escHtml(s) {
  return String(s).replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c]);
}
