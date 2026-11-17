// Roteiro do teste de ponta a ponta da demonstração (2.82). Quem chama é o
// TestDemoScreenE2E (internal/app/demo_e2e_test.go), que serve a página com o
// App de verdade:
//
//   RADLOG_E2E=1 go test -tags webkit2_41 -run TestDemoScreenE2E ./internal/app/
//
// Liga a demonstração (a tela recarrega com o mundo simulado), passa pelos
// cenários e pelas páginas, tenta exportar (recusado), confere que nenhum
// arquivo do app mudou e sai.
let pw;
try { pw = require("playwright"); } catch { pw = require("playwright-core"); }

const url = process.argv[2];
const fail = (m) => { console.error("FALHOU: " + m); process.exit(1); };

(async () => {
  const b = await pw.chromium.launch(process.env.RADLOG_CHROME ? { channel: process.env.RADLOG_CHROME } : {});
  const ctx = await b.newContext({ viewport: { width: 1280, height: 900 }, locale: "pt-BR" });
  const p = await ctx.newPage();
  const errs = [];
  p.on("pageerror", (e) => errs.push(e.message + (process.env.E2E_STACK ? " " + e.stack : "")));
  const goCall = (name, ...args) => p.evaluate(([n, a]) => window.go.app.App[n](...a), [name, args]);
  await p.goto(url);
  await p.waitForTimeout(1500);
  if (!(await p.evaluate(() => !!(window.go && window.go.app)))) fail("a página não achou a ponte");
  if (await p.evaluate(() => inDemo)) fail("abriu já na demonstração");

  // 1) liga: a tela recarrega com o mundo simulado
  // 2.89.0: o botão mora em Configurações › Atualizações e sobre
  await p.evaluate(() => openSettings("btnDemo"));
  await p.waitForTimeout(500);
  await p.click("#btnDemo");
  await p.locator("#demoStripe").waitFor({ state: "visible", timeout: 10000 });
  await p.waitForTimeout(1000);
  if (!(await p.evaluate(() => inDemo && typeof demoApi === "object"))) fail("ligou sem trocar a ponte");
  if ((await goCall("GetDemo")).scenario !== "casa") fail("o Go não guardou o cenário");
  if ((await (await p.request.get(url + "__snap")).text()) !== "ok") fail("snap");
  // o GMC-800 de verdade não está conectado: a faixa avisa (2.83.1)
  const warn = await p.locator("#demoStripeWarn").textContent();
  if (!(await p.locator("#demoStripeWarn").isVisible()) || !warn.includes("desconectado")) fail("a faixa não avisou do GMC-800 desconectado: " + warn);

  // 2) cenários: o Go guarda, a tela mostra
  for (const sc of ["perigo", "fonte", "picorc", "saturado", "saida", "cs137", "desconecta", "casa"]) {
    await p.click(`#demoScen button[data-sc="${sc}"]`);
    await p.waitForTimeout(sc === "perigo" ? 2500 : 1200);
    if ((await goCall("GetDemo")).scenario !== sc) fail("cenário no Go: " + sc);
    if (sc === "perigo" && (await p.locator("#alarmBar").isHidden())) fail("o perigo da demonstração não apareceu");
  }
  await p.click('#demoSound button[data-v="1"]');
  await p.click('#demoSpeed button[data-v="10"]');
  await p.waitForTimeout(400);
  const d = await goCall("GetDemo");
  if (!d.sound || d.speed !== 10) fail("som e velocidade: " + JSON.stringify(d));

  // 3) páginas com o mundo simulado
  for (const v of ["graficos", "fundo", "resumo", "alertas", "medir", "fora", "gmc", "radiacode", "agora"]) {
    await p.click(`#sideNav [data-view="${v}"]`);
    await p.waitForTimeout(700);
  }

  // 4) exportar é recusado
  await p.evaluate(() => document.getElementById("btnExport").click()); // fica nas Configurações › Dados e backup
  await p.waitForTimeout(800);
  const toasts = await p.locator("#toasts").textContent();
  if (!toasts.includes("saia da demonstração para usar")) fail("exportar não foi recusado: " + toasts);
  // ação no aparelho: finge e avisa
  await p.evaluate(() => api.SyncClock());
  await p.waitForTimeout(300);
  if (!(await p.locator("#toasts").textContent()).includes("não muda o aparelho de verdade")) fail("acertar o relógio não avisou");

  // 5) nada mudou nos arquivos e sai
  const chk = await (await p.request.get(url + "__check")).text();
  if (chk !== "ok") fail("a demonstração mexeu nos arquivos do app");
  await p.click("#demoExit");
  await p.waitForTimeout(2500);
  if (await p.evaluate(() => inDemo)) fail("não saiu da demonstração");
  if (await p.locator("#demoStripe").isVisible()) fail("a faixa ficou depois de sair");
  if ((await goCall("GetDemo")).active) fail("o Go ficou na demonstração");

  // a recusa do exportar sobe como erro do clique (o toast já mostrou)
  const bad = errs.filter((m) => !m.startsWith("saia da demonstração para usar"));
  if (bad.length) fail("erro de script: " + bad.join(" | "));
  console.log("demonstração ok: ligar, cenários, páginas, exportar recusado, arquivos iguais e sair");
  await b.close();
})().catch((e) => fail(e.message));
