"use strict";

// ---------- confirmação no estilo do app ----------
//
// Troca o confirm() nativo (a caixa "wails.localhost says" com OK/Cancel em
// inglês) por um diálogo do próprio app. Devolve uma Promise<boolean>.
//
//   askConfirm({
//     title: "Gravar no Radiacode?",
//     text: ["parágrafo", …],           // ou uma string
//     rows: [["Hoje", "986 · 8546 CPM"]], // nome → valor (mono)
//     diff: [["Brilho", "5", "7"]],      // nome, agora, depois
//     diffHead: ["Ponto", "Hoje", "Novo"], // títulos das colunas
//     note: "aviso em destaque",
//     ok: "Gravar", danger: true,        // danger = ação sem volta
//   })
//
// O foco começa no Cancelar; Esc e o X cancelam. O texto entra em
// português e o MutationObserver do i18n.js traduz.

let cfmDlg = null, cfmDone = null;

function cfmBuild() {
  const d = document.createElement("dialog");
  d.id = "confirmDlg";
  d.className = "cfm-dlg";
  d.setAttribute("aria-labelledby", "cfmTitle");
  d.setAttribute("aria-describedby", "cfmBody");
  d.innerHTML =
    '<form method="dialog" class="dlg">' +
      '<div class="dlg-head"><h2 id="cfmTitle"></h2>' +
        '<button type="submit" class="btn icon" aria-label="Fechar" value="no">' +
          '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><path d="M6 6l12 12M18 6L6 18"/></svg>' +
        "</button></div>" +
      '<div class="cfm-body" id="cfmBody"></div>' +
      '<div class="dlg-foot">' +
        '<button type="submit" class="btn" value="no" id="cfmNo">Cancelar</button>' +
        '<button type="submit" class="btn accent" value="yes" id="cfmYes"></button>' +
      "</div>" +
    "</form>";
  document.body.appendChild(d);
  d.addEventListener("close", () => {
    const ok = d.returnValue === "yes";
    d.returnValue = "";
    const f = cfmDone;
    cfmDone = null;
    if (f) f(ok);
  });
  return d;
}

function askConfirm(o) {
  if (!cfmDlg) cfmDlg = cfmBuild();
  // outra confirmação ainda aberta: a antiga conta como cancelada
  if (cfmDone) { const f = cfmDone; cfmDone = null; f(false); }
  const el = (tag, cls, txt) => { const e = document.createElement(tag); if (cls) e.className = cls; if (txt != null) e.textContent = txt; return e; };
  const body = cfmDlg.querySelector("#cfmBody");
  body.replaceChildren();
  cfmDlg.querySelector("#cfmTitle").textContent = o.title || "Continuar?";
  const paras = o.text == null ? [] : Array.isArray(o.text) ? o.text : [o.text];
  const lead = paras.slice(0, o.textAfter ? paras.length - o.textAfter : paras.length);
  const tail = o.textAfter ? paras.slice(-o.textAfter) : [];
  for (const p of lead) body.appendChild(el("p", "cfm-p", p));
  if (o.rows && o.rows.length) {
    const dl = el("dl", "cfm-rows");
    for (const [k, v] of o.rows) { dl.appendChild(el("dt", "", k)); dl.appendChild(el("dd", "mono", v)); }
    body.appendChild(dl);
  }
  if (o.diff && o.diff.length) {
    const t = el("div", "cfm-diff");
    t.setAttribute("role", "table");
    const head = el("div", "cfm-dh");
    head.setAttribute("role", "row");
    for (const h of o.diffHead || ["Ajuste", "Agora", "Depois"]) { const c = el("span", "", h); c.setAttribute("role", "columnheader"); head.appendChild(c); }
    t.appendChild(head);
    for (const [k, a, b] of o.diff) {
      const r = el("div", "cfm-dr");
      r.setAttribute("role", "row");
      const cells = [el("span", "", k), el("span", "mono cfm-was", a), el("span", "mono cfm-new", b)];
      cells.forEach((c) => { c.setAttribute("role", "cell"); r.appendChild(c); });
      t.appendChild(r);
    }
    body.appendChild(t);
  }
  for (const p of tail) body.appendChild(el("p", "cfm-p", p));
  if (o.note) body.appendChild(el("p", "cfm-note" + (o.danger ? " danger" : ""), o.note));
  const yes = cfmDlg.querySelector("#cfmYes");
  yes.textContent = o.ok || "Continuar";
  yes.classList.toggle("accent", !o.danger);
  yes.classList.toggle("danger", !!o.danger);
  cfmDlg.returnValue = "";
  return new Promise((res) => {
    cfmDone = res;
    if (!cfmDlg.open) cfmDlg.showModal();
    cfmDlg.querySelector("#cfmNo").focus();
  });
}
