// Roteiro do teste de ponta a ponta da calibração do KCl (2.101.3). Quem chama
// é o TestActivityCalScreenE2E (internal/app/actcal_e2e_test.go), que serve a
// página com o App de verdade e um RC102 de mentira:
//
//   RADLOG_E2E=1 go test -tags webkit2_41 -run TestActivityCalScreenE2E ./internal/app/
//
// Digita 250 g, 51% e o nome da montagem, mexe em tudo que antes trocava a
// massa pelo padrão (material, pote, voltar e Calibrar de novo), começa, passa
// pelas fases (com as conferências de 10 e 30 min), soma uma segunda rodada e
// confere o resultado na tela. O Go confere o que ficou salvo.
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
  // 2.104.0: calibração por montagem (pote, material, fundo)
  await want("1000", "abrir");
  await p.fill("#rcActCalMass", "250");
  await p.click('#rcActMat button[data-v="light"]'); await want("250", "Sal light");
  await p.click('#rcActMat button[data-v="puro"]'); await want("250", "KCl puro");
  await p.click('#rcActSetup button[data-v="pote1l"]'); await want("250", "pote de 1 L");
  await p.click('#rcActSetup button[data-v="becker"]'); await want("250", "béquer");
  await p.click('#rcActMat button[data-v="light"]'); await want("250", "Sal light de novo");
  await p.click("#rcActCalBack");
  await p.click('#rcActCal [data-cal="kcl"]'); await want("250", "voltar e Calibrar de novo");
  if (await p.isHidden("#rcActPctBox")) fail("sal light sem o campo de %");
  // fundo e KCl sempre com o mesmo tempo
  const hm = await p.$$eval("#rcActCalTot b", (bs) => bs.map((x) => x.textContent));
  if (hm.length !== 2 || hm[0] !== hm[1]) fail("fundo e KCl com tempos diferentes: " + hm.join(" / "));
  await p.fill("#rcActPct", "51");
  await p.fill("#rcActMountName", "Becker Marinelli");
  await p.click("#rcActCalStart");
  await p.waitForTimeout(800);
  const err = (await p.textContent("#rcActCalErr")).trim();
  if (err) fail("não começou: " + err);
  const what = (await p.textContent("#rcActWhat")).replace(/\s+/g, " ").trim();
  if (!/250,0 g/.test(what) || !/Becker Marinelli/.test(what)) fail("começou com outra coisa: " + what);
  const go = await p.evaluate(() => window.go.app.App.GetActivity());
  const oo = go.opts || {};
  if (oo.massG !== 250 || oo.pct !== 51 || oo.setup !== "becker" || oo.material !== "light" || oo.bgMode !== "branco" || oo.mountName !== "Becker Marinelli" || oo.bgSec !== oo.smpSec || go.phase !== "fundo") fail("o Go recebeu outra coisa: " + JSON.stringify(oo));
  if (!/sal de cozinha comum/.test(await p.textContent("#rcActDo"))) fail("o fundo não pediu o pote com sal comum");
  // fundo de 2 h → colocar o sal → 2 h de sal → resultado
  const next = async (ph) => {
    await p.click("#rcActNext");
    for (let i = 0; i < 30; i++) { await p.waitForTimeout(200); if (((await p.evaluate(() => window.go.app.App.GetActivity())).phase || "") === ph) return; }
    fail("não passou para " + ph);
  };
  const liveTxt = async (re) => { let t = ""; for (let i = 0; i < 40; i++) { t = (await p.textContent("#rcActLive")).replace(/\s+/g, " "); if (re.test(t)) break; await p.waitForTimeout(200); } return t; };
  const round = async (n) => {
    await (await p.request.get(url + "__fundo")).text();
    await next("trocar");
    await next("amostra");
    // 10 min a 30 min: só a conferência da faixa (o sal fora do aparelho = quase não subiu)
    await (await p.request.get(url + "__semsal")).text();
    let live = await liveTxt(/quase não subiu/);
    if (!/quase não subiu/.test(live)) fail(`rodada ${n}: não avisou com o sal fora do aparelho: ` + live.slice(0, 300));
    if (!/aparecem com 30 min/.test(live)) fail(`rodada ${n}: mostrou pico ou previsão antes dos 30 min: ` + live.slice(0, 300));
    // com o sal visto: faixa subiu, pico e previsão
    await (await p.request.get(url + "__sal")).text();
    live = await liveTxt(/Previsão/);
    if (!/O RC102 está vendo o KCl/.test(live) || !/Previsão:/.test(live)) fail(`rodada ${n}: sem as conferências dos 30 min: ` + live.slice(0, 400));
    await next("pronto");
    await p.waitForTimeout(800);
    const res = (await p.textContent("#rcActLive")).replace(/\s+/g, " ").trim();
    if (!/Becker Marinelli/.test(res) || !/250 g/.test(res) || !/eficiência da montagem/.test(res)) fail(`rodada ${n}: resultado na tela: ` + res.slice(0, 400));
    if (n > 1 && !/somadas 2 rodadas/.test(res)) fail("a segunda rodada não somou: " + res.slice(0, 400));
    return res;
  };
  await round(1);
  // somar outra rodada na mesma montagem
  if (await p.isHidden("#rcActAgain")) fail("sem o botão Somar outra rodada");
  await p.click("#rcActAgain");
  await p.waitForTimeout(800);
  const g2 = await p.evaluate(() => window.go.app.App.GetActivity());
  if (g2.phase !== "fundo" || !g2.opts.mountId || g2.opts.massG !== 250 || g2.opts.pct !== 51) fail("a segunda rodada começou com outra coisa: " + JSON.stringify(g2.opts));
  await round(2);
  await p.click("#rcActNext"); // Nova medida: fecha e volta à lista
  await p.waitForTimeout(800);
  const list = (await p.textContent("#rcActCal")).replace(/\s+/g, " ");
  if (!/Becker Marinelli/.test(list) || !/2 rodadas/.test(list)) fail("a lista de montagens não mostra a montagem: " + list.slice(0, 300));
  if (errs.length) fail("erros na página: " + errs.join(" | "));
  console.log("ok: 250 g, 51% e a montagem chegaram até o resultado, duas rodadas somadas");
  await b.close();
})().catch((e) => fail(e.stack || String(e)));
