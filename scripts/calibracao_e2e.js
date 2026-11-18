// Roteiro do teste de ponta a ponta da calibração do KCl (2.101.3). Quem chama
// é o TestActivityCalScreenE2E (internal/app/actcal_e2e_test.go), que serve a
// página com o App de verdade e um RC102 de mentira:
//
//   RADLOG_E2E=1 go test -tags webkit2_41 -run TestActivityCalScreenE2E ./internal/app/
//
// Digita 250 g, 51% e a posição, mexe em tudo que antes trocava a massa pelo
// padrão (Sal light, KCl puro, os cartões, voltar e Calibrar de novo), começa,
// passa pelas fases e confere o resultado na tela. O Go confere o que ficou
// salvo.
let pw;
try { pw = require("playwright"); } catch { pw = require("playwright-core"); }

const url = process.argv[2];
const fail = (m) => { console.error("FALHOU: " + m); process.exit(1); };

(async () => {
  const b = await pw.chromium.launch(process.env.RADLOG_CHROME ? { channel: process.env.RADLOG_CHROME } : {});
  const ctx = await b.newContext({ viewport: { width: 1400, height: 950 }, locale: "pt-BR" });
  const p = await ctx.newPage();
  const errs = [];
  p.on("pageerror", (e) => errs.push(e.message));
  await p.goto(url);
  await p.waitForTimeout(1500);
  if (!(await p.evaluate(() => !!(window.go && window.go.app)))) fail("a página não achou a ponte");
  await p.evaluate(() => showView("radiacode"));
  await p.click('[role=tab][data-sub="atividade"]');
  await p.waitForTimeout(800);
  await p.click('#rcActCal [data-cal="kcl"]');
  await p.waitForTimeout(300);
  const mass = () => p.inputValue("#rcActCalMass");
  const want = async (v, depois) => { const m = await mass(); if (m !== v) fail(`massa ${m} depois de ${depois}, queria ${v}`); };
  await want("500", "abrir");
  await p.fill("#rcActCalMass", "250");
  await p.click('#rcActKclType button[data-v="light"]'); await want("250", "Sal light");
  await p.click('#rcActKclType button[data-v="100"]'); await want("250", "KCl puro");
  await p.click('#rcActKclType button[data-v="light"]'); await want("250", "Sal light de novo");
  await p.click('#rcActCalSteps [data-k="kcl"]'); await want("250", "cartão 1 · KCl");
  await p.click('#rcActCalSteps [data-k="torio"]'); await want("40", "cartão 2 · Tório");
  await p.fill("#rcActCalMass", "35");
  await p.click('#rcActCalSteps [data-k="kcl"]'); await want("250", "voltar ao KCl");
  await p.click('#rcActCalSteps [data-k="torio"]'); await want("35", "voltar ao tório");
  await p.click('#rcActCalSteps [data-k="kcl"]');
  await p.click("#rcActCalBack");
  await p.click('#rcActCal [data-cal="kcl"]'); await want("250", "voltar e Calibrar de novo");
  await p.fill("#rcActPct", "51");
  await p.fill("#rcActCalGeom", "Becker Marinelli");
  await p.click("#rcActCalStart");
  await p.waitForTimeout(800);
  const err = (await p.textContent("#rcActCalErr")).trim();
  if (err) fail("não começou: " + err);
  const what = (await p.textContent("#rcActWhat")).replace(/\s+/g, " ").trim();
  if (!/KCl \(51%\)/.test(what) || !/250,0 g/.test(what) || !/Becker Marinelli/.test(what)) fail("começou com outra coisa: " + what);
  const go = await p.evaluate(() => window.go.app.App.GetActivity());
  if (!go.opts || go.opts.massG !== 250 || go.opts.pct !== 51 || go.opts.geometry !== "Becker Marinelli" || go.phase !== "fundo") fail("o Go recebeu outra coisa: " + JSON.stringify(go.opts));
  // fundo de 2 h → colocar o sal → 2 h de sal → resultado
  const next = async (ph) => {
    await p.click("#rcActNext");
    for (let i = 0; i < 30; i++) { await p.waitForTimeout(200); if ((await p.evaluate(() => window.go.app.App.GetActivity())).phase === ph) return; }
    fail("não passou para " + ph);
  };
  await (await p.request.get(url + "__fundo")).text();
  await next("trocar");
  await next("amostra");
  // 2.101.4: 10 min sem o RC102 ver o sal: avisa na hora, sem esperar as 2 h
  const liveTxt = async (re) => { let t = ""; for (let i = 0; i < 40; i++) { t = (await p.textContent("#rcActLive")).replace(/\s+/g, " "); if (re.test(t)) break; await p.waitForTimeout(200); } return t; };
  await (await p.request.get(url + "__semsal")).text();
  let live = await liveTxt(/não está vendo o sal/);
  if (!/não está vendo o sal/.test(live)) fail("não avisou com o sal fora do aparelho: " + live.slice(0, 300));
  // com o sal visto, a contagem total sobe e o aviso some
  await (await p.request.get(url + "__sal")).text();
  live = await liveTxt(/\(\+([1-9]\d|\d{3,})%\)/);
  if (!/contagem total: [\d.,]+ cps · fundo [\d.,]+ cps \(\+\d+%\)/.test(live)) fail("sem a contagem total durante o sal: " + live.slice(0, 300));
  if (/não está vendo o sal/.test(live)) fail("avisou com o sal visto: " + live.slice(0, 300));
  await next("pronto");
  await p.waitForTimeout(800);
  const res = (await p.textContent("#rcActLive")).replace(/\s+/g, " ").trim();
  if (!/KCl \(51%\)/.test(res) || !/250,0 g/.test(res)) fail("resultado na tela: " + res.slice(0, 300));
  if (errs.length) fail("erros na página: " + errs.join(" | "));
  console.log("ok: 250 g, 51% e a posição chegaram até o resultado");
  await b.close();
})().catch((e) => fail(e.stack || String(e)));
