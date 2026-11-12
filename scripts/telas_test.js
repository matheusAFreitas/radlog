// Teste de tela: abre o app no modo demonstração do navegador (sem Go),
// passa pelas 11 páginas em 1280 px e em 412 px, nos dois idiomas, e falha
// se aparecer erro de script ou rolagem horizontal.
//
// Uso: node scripts/telas_test.js
// Precisa do playwright (ou playwright-core com o Chrome do sistema:
// RADLOG_CHROME=chrome node scripts/telas_test.js).
let pw;
try { pw = require("playwright"); } catch { pw = require("playwright-core"); }
const path = require("path");

const VIEWS = ["agora", "resumo", "gmc", "radiacode", "graficos", "alertas", "fora", "medir", "alara", "chuva", "config"];
const SIZES = [{ width: 1280, height: 900 }, { width: 412, height: 915 }];
const page0 = "file://" + path.resolve(__dirname, "../frontend/dist/index.html");

(async () => {
  const b = await pw.chromium.launch(process.env.RADLOG_CHROME ? { channel: process.env.RADLOG_CHROME } : {});
  const fails = [];
  for (const lang of ["pt", "en"]) {
    for (const vp of SIZES) {
      const ctx = await b.newContext({ viewport: vp, locale: "pt-BR" });
      await ctx.addInitScript((l) => { try { localStorage.setItem("lang", l); } catch {} }, lang);
      const p = await ctx.newPage();
      let where = "abrir";
      p.on("pageerror", (e) => fails.push(`${lang} ${vp.width}px ${where}: ${e.message}`));
      await p.goto(page0);
      await p.waitForTimeout(2000);
      for (const v of VIEWS) {
        where = v;
        const nav = p.locator(`[data-view="${v}"]:visible`).first();
        if (!(await nav.count())) { fails.push(`${lang} ${vp.width}px: sem botão para a página ${v}`); continue; }
        await nav.click();
        await p.waitForTimeout(700);
        const over = await p.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
        if (over > 1) fails.push(`${lang} ${vp.width}px ${v}: rolagem horizontal de ${over}px`);
      }
      await ctx.close();
    }
  }
  await b.close();
  if (fails.length) {
    console.log(fails.join("\n"));
    process.exit(1);
  }
  console.log(`telas ok: ${VIEWS.length} páginas × ${SIZES.length} larguras × 2 idiomas`);
})().catch((e) => { console.error(e); process.exit(1); });
