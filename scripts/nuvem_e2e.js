// Roteiro do teste de ponta a ponta da tela da nuvem (auditoria 2.79.0, T2).
// Quem chama é o TestCloudScreenE2E (internal/app/cloud_e2e_test.go), que
// serve a página com o App de verdade e o Drive de mentira:
//
//   RADLOG_E2E=1 go test -tags webkit2_41 -run TestCloudScreenE2E ./internal/app/
//
// Liga a nuvem como o primeiro aparelho (ver, guardar e conferir o código),
// sincroniza, abre "Ver código", confere que é o mesmo e desconecta.
let pw;
try { pw = require("playwright"); } catch { pw = require("playwright-core"); }

const url = process.argv[2];
const fail = (m) => { console.error("FALHOU: " + m); process.exit(1); };

(async () => {
  const b = await pw.chromium.launch(process.env.RADLOG_CHROME ? { channel: process.env.RADLOG_CHROME } : {});
  const ctx = await b.newContext({ viewport: { width: 1280, height: 900 }, locale: "pt-BR" });
  const p = await ctx.newPage();
  const errs = [];
  p.on("pageerror", (e) => errs.push(e.message));
  await p.goto(url);
  await p.waitForTimeout(1500);
  // sem a ponte a página cai no modo demonstração e o teste não testaria nada
  if (!(await p.evaluate(() => !!(window.go && window.go.app)))) fail("a página não achou a ponte (modo demonstração)");
  await p.evaluate(() => openSettings("cloudBox"));
  await p.waitForTimeout(500);
  const box = p.locator("#cloudBox");
  const btn = (scope, text) => scope.locator("button", { hasText: text }).first();
  const dlg = p.locator("#cloudDlg");

  // 1) entrar com Google (o "navegador" do teste volta sozinho com o código)
  await btn(box, "Entrar com Google").click();
  await dlg.locator(".cloud-code").waitFor({ timeout: 10000 });
  const groups = await dlg.locator(".cloud-code span").evaluateAll((ss) => ss.map((s) => s.firstChild.textContent.trim()));
  if (groups.length !== 6) fail("código com " + groups.length + " grupos");
  const code = groups.join("-");

  // 2) guardei, confirmo e confiro dois grupos
  await btn(dlg, "Guardei, continuar").click();
  await dlg.locator("input[type=checkbox]").check();
  await btn(dlg, "Conferir").click();
  const labels = await dlg.locator(".cloud-inputs label").evaluateAll((ls) => ls.map((l) => l.textContent));
  const inputs = dlg.locator(".cloud-inputs input");
  // primeiro um errado: a tela recusa
  await inputs.nth(0).fill("ZZZZ");
  await inputs.nth(1).fill("ZZZZ");
  await btn(dlg, "Ligar a nuvem").click();
  if (!(await dlg.locator(".cloud-err").isVisible())) fail("aceitou o código errado");
  for (let k = 0; k < 2; k++) {
    const n = parseInt(labels[k].match(/grupo (\d)/)[1], 10);
    await inputs.nth(k).fill(groups[n - 1].toLowerCase());
  }
  await btn(dlg, "Ligar a nuvem").click();

  // 3) ligada: sincroniza e mostra "em dia"
  await btn(box, "Sincronizar agora").waitFor({ timeout: 10000 });
  await btn(box, "Sincronizar agora").click();
  await p.waitForTimeout(800);
  await p.evaluate(() => cloudCall("GetCloud"));
  const tag = (await p.locator("#cloudTag").textContent()).trim();
  if (tag !== "em dia") fail("estado depois de sincronizar: " + tag + " / " + (await box.textContent()));
  if (!(await box.textContent()).includes("Este PC")) fail("sem a lista de aparelhos");

  // 4) ver código: o mesmo
  await btn(box, "Ver código de recuperação").click();
  await btn(p.locator("dialog[open]"), "Mostrar").click();
  await dlg.locator(".cloud-code").waitFor({ timeout: 5000 });
  const again = await dlg.locator(".cloud-code span").evaluateAll((ss) => ss.map((s) => s.firstChild.textContent.trim()).join("-"));
  if (again !== code) fail("ver código mostrou outro: " + again);
  await btn(dlg, "Fechar").click();

  // 5) desconectar
  await btn(box, "Desconectar do Google").click();
  await btn(p.locator("dialog[open]"), "Desconectar").click();
  await btn(box, "Entrar com Google").waitFor({ timeout: 5000 });

  if (errs.length) fail("erro de script: " + errs.join(" | "));
  console.log("nuvem ok: ligar, conferir o código, sincronizar, ver código e desconectar");
  await b.close();
})().catch((e) => fail(e.message));
