// Teste de tela: abre o app no modo demonstração do navegador (sem Go),
// passa pelas 11 páginas em 1280 px e em 412 px, nos dois idiomas, abre as
// sub-abas de cada página e as janelas (<dialog>), e falha se aparecer erro
// de script, rolagem horizontal ou janela saindo da tela.
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

async function checkOver(p, label, fails) {
  const over = await p.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  if (over > 1) fails.push(`${label}: rolagem horizontal de ${over}px`);
}

let subs = 0, dlgs = 0;
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
        await checkOver(p, `${lang} ${vp.width}px ${v}`, fails);
        // sub-abas da página (Gráficos, RC102, GMC-800…)
        const tabs = p.locator(`[role="tab"][data-sub]:visible, [data-tab]:visible`);
        const nt = await tabs.count();
        for (let i = 0; i < nt; i++) {
          const t = tabs.nth(i);
          const name = (await t.getAttribute("data-sub")) || (await t.getAttribute("data-tab"));
          where = `${v} › ${name}`;
          try { await t.click({ timeout: 2000 }); } catch { continue; }
          await p.waitForTimeout(300);
          await checkOver(p, `${lang} ${vp.width}px ${where}`, fails);
          subs++;
        }
      }
      // janelas: abre cada <dialog> e confere se cabe na tela
      for (const id of await p.evaluate(() => [...document.querySelectorAll("dialog[id]")].map((d) => d.id))) {
        where = "janela " + id;
        const r = await p.evaluate((id) => {
          const d = document.getElementById(id);
          try { if (!d.open) d.showModal(); } catch (e) { return { err: e.message }; }
          const b = d.getBoundingClientRect();
          const out = { right: Math.round(b.right - innerWidth), inner: d.scrollWidth - d.clientWidth };
          d.close();
          return out;
        }, id);
        if (r.err) fails.push(`${lang} ${vp.width}px ${where}: ${r.err}`);
        else {
          if (r.right > 1) fails.push(`${lang} ${vp.width}px ${where}: sai ${r.right}px da tela`);
          if (r.inner > 1) fails.push(`${lang} ${vp.width}px ${where}: rolagem horizontal de ${r.inner}px`);
        }
        dlgs++;
      }
      await ctx.close();
    }
  }
  await b.close();
  if (fails.length) {
    console.log(fails.join("\n"));
    process.exit(1);
  }
  console.log(`telas ok: ${VIEWS.length} páginas, ${subs} sub-abas e ${dlgs} janelas abertas em ${SIZES.length} larguras × 2 idiomas`);
})().catch((e) => { console.error(e); process.exit(1); });
