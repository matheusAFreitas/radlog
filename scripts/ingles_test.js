// Teste do inglês (2.101.2): falha se sobrar português na tela em inglês.
//
// Parte 1 (sem navegador): todo texto em português do código (o catálogo do
// scripts/i18n_extract.py: frases fixas e moldes com partes variáveis do JS,
// do HTML e do Go) passa pelo tr() do app; os moldes são preenchidos com
// valores de exemplo (números, horas, aparelhos, "Hoje"…). Sobrou acento ou
// palavra comum do português no resultado: falha.
// Parte 2 (com navegador, RADLOG_INGLES_TELAS=1): abre o demo em inglês, liga
// cada cenário da demonstração (seguro, atenção, alarme, perigo, saída,
// saturado, desconectado, Cs-137…) e passa por todas as páginas, sub-abas,
// detalhes abertos, janelas e hover nos gráficos, lendo o texto da tela e os
// atributos title/aria-label/placeholder.
//
// Uso: node scripts/ingles_test.js            (parte 1)
//      RADLOG_INGLES_TELAS=1 node scripts/ingles_test.js   (as duas)
"use strict";
const fs = require("fs"), vm = require("vm"), path = require("path"), cp = require("child_process");
const ROOT = path.resolve(__dirname, "..");
const D = path.join(ROOT, "frontend/dist/");

// palavras e letras que só o português tem (o inglês não usa nenhuma delas)
const PT = /[ãõçáéíóúâêôà]|\b(não|de|da|das|dos|na|nas|nos|em|que|ou|com|para|sem|uma|pelo|pela|tudo|nada|dia|dias|hora|horas|minutos|segundos|semana|casa|fundo|aparelho|aparelhos|contador|hoje|ontem|seguro|nível|até|mais|menos|ainda|agora|aqui|desde|depois|antes|medição|medir|objeto|pico|picos|alarme|atenção|perigo|chuva|leitura|contagens|ligado|desligado|conectado|desconectado|aberto|fechado|nenhum|nenhuma|vezes|sim|muito|pouco)\b/i;
// o que pode ficar como está: nomes próprios, unidades, o seletor de idioma
const OK = [
  /^Português$/, /^Automático \(idioma do sistema\) · Automatic$/, /^Automatic \(system language\) · Automático$/, // seletor de idioma: os dois de propósito
  /Eletrodo de tungstênio toriado/, // nome de exemplo que a pessoa deu
  /Goiânia|Leide das Neves|Rio de Janeiro|Matheus Alves de Freitas/, // nomes próprios
  /Documents\/RadLog\/Relatórios/, // nome da pasta no disco
];
// endereços (github.com, voce@gmail.com) não contam: o "com" é do domínio
const noUrl = (s) => s.replace(/\S+\.(com|org|net|io|br)\b\S*/g, "");
const okText = (s) => OK.some((r) => r.test(s)) || !PT.test(noUrl(s));

function loadTr() {
  const i = fs.readFileSync(D + "i18n.js", "utf8").split("\n");
  const src = fs.readFileSync(D + "i18n-en.js", "utf8") + "\n" + i.slice(0, i.indexOf("})();") + 1).join("\n") + ";globalThis.__tr = tr;";
  const ctx = { localStorage: { getItem: () => "en" }, navigator: { language: "en" }, document: { documentElement: {} } };
  vm.createContext(ctx); vm.runInContext(src, ctx);
  return ctx.__tr;
}

function part1() {
  const tr = loadTr();
  const cat = cp.execFileSync("python3", [path.join(ROOT, "scripts/i18n_extract.py")], { encoding: "utf8", maxBuffer: 64 << 20 });
  const skip = new Set(fs.readFileSync(path.join(ROOT, "scripts/i18n_pendentes.txt"), "utf8").split("\n").map((s) => s.trim()).filter(Boolean));
  const FILL = [["3", "14:05", "GMC-800", "0,12"], ["Hoje", "RC102", "12/11", "2 h"]];
  const fails = [];
  let n = 0;
  for (const ln of cat.split("\n")) {
    const [kind, s] = [ln.slice(0, 1), ln.slice(2)];
    if (!s || skip.has(s)) continue;
    const samples = kind === "P" ? FILL.map((f) => s.replace(/\{(\d+)\}/g, (_, i) => f[+i % f.length])) : [s];
    for (const x of samples) {
      n++;
      // o valor de exemplo "Hoje" no lugar de uma palavra (linha, linhas…)
      // fica como está: o que conta é o texto fixo
      const out = tr(x), chk = kind === "P" ? out.replace(/\bHoje\b/g, "") : out;
      if (PT.test(chk) && !okText(out)) fails.push(`${x}\n      → ${out}`);
    }
  }
  return { n, fails };
}

async function part2() {
  let pw;
  try { pw = require("playwright"); } catch { pw = require("playwright-core"); }
  const VIEWS = ["agora", "resumo", "gmc", "radiacode", "graficos", "fundo", "alertas", "fora", "medir", "chuva", "config"];
  const SCEN = ["", "casa", "atencao", "alarme", "perigo", "fonte", "picorc", "saturado", "saida", "cs137", "desconecta"];
  const b = await pw.chromium.launch(process.env.RADLOG_CHROME ? { channel: process.env.RADLOG_CHROME } : {});
  const found = new Map(), errs = new Set();
  let scans = 0;
  for (const sc of SCEN) {
    const ctx = await b.newContext({ viewport: { width: 1400, height: 950 }, locale: "en-US" });
    await ctx.addInitScript(() => { try { localStorage.setItem("lang", "en"); localStorage.setItem("rcFake", "1"); } catch {} });
    const p = await ctx.newPage();
    p.on("pageerror", (e) => errs.add(e.message));
    await p.goto("file://" + D + "index.html");
    await p.waitForTimeout(1500);
    if (sc) { await p.evaluate((s) => api.SetDemo({ active: true, scenario: s, speed: 10 }).then((d) => { state.demo = d; if (typeof renderDemo === "function") renderDemo(); }), sc); await p.waitForTimeout(2500); }
    const scan = async (where) => {
      scans++;
      const r = await p.evaluate((src) => {
        const re = new RegExp(src, "i"), out = [];
        const vis = (el) => { for (let e = el; e; e = e.parentElement) { if (e.hidden) return false; const cs = getComputedStyle(e); if (cs.display === "none" || cs.visibility === "hidden") return false; } return true; };
        const w = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT); let nd;
        while ((nd = w.nextNode())) {
          const el = nd.parentElement; if (!el || el.closest("script,style,[data-no-tr]")) continue;
          const t = nd.textContent.replace(/[⁨⁩]/g, "").replace(/\s+/g, " ").trim();
          if (t.length > 1 && re.test(t) && vis(el)) out.push(t.slice(0, 200));
        }
        for (const el of document.querySelectorAll("[title],[aria-label],[placeholder]")) for (const a of ["title", "aria-label", "placeholder"]) {
          const t = el.getAttribute(a); if (t && re.test(t) && !el.closest("[data-no-tr]")) out.push("@" + a + ": " + t.replace(/\s+/g, " ").slice(0, 200));
        }
        for (const el of document.querySelectorAll("option")) { const t = el.textContent.trim(); if (re.test(t)) out.push(t.slice(0, 200)); }
        return out;
      }, PT.source);
      for (const t of r) if (!found.has(t) && !okText(t.replace(/^@[\w-]+: /, ""))) found.set(t, `${sc || "normal"}/${where}`);
    };
    const hover = async (where) => {
      const boxes = await p.evaluate(() => [...document.querySelectorAll("svg, canvas, .ft-row, [data-tip]")].map((e) => e.getBoundingClientRect()).filter((r) => r.width > 100 && r.height > 20 && r.bottom > 0 && r.top < innerHeight).map((r) => [r.x, r.y, r.width, r.height]));
      for (const [x, y, w, h] of boxes.slice(0, 10)) for (let i = 1; i < 10; i++) { await p.mouse.move(x + (w * i) / 10, y + h / 2); await p.waitForTimeout(40); await scan(where + "/hover"); }
      await p.mouse.move(2, 2);
    };
    // no cenário normal passa também pelos botões de cada página
    const BAD = /apagar|excluir|delete|remove|reset|zerar|clear|quit|sair do app|disconnect|desconectar|stop|parar|export|download|baixar|choose|escolher|folder|pasta|sign|login|logout|install|instalar|update now|restore|restaurar|import/i;
    for (const v of VIEWS) {
      await p.evaluate((x) => showView(x), v); await p.waitForTimeout(500);
      const tabs = await p.locator(".page:not([hidden]) [role=tab][data-sub]").count();
      for (let ti = 0; ti < Math.max(1, tabs); ti++) {
        if (tabs) { try { await p.locator(".page:not([hidden]) [role=tab][data-sub]").nth(ti).click({ timeout: 800 }); await p.waitForTimeout(400); } catch { continue; } }
        const w = `${v}/${ti}`;
        await scan(w);
        await p.evaluate(() => document.querySelectorAll(".page:not([hidden]) details:not([open])").forEach((d) => (d.open = true)));
        await p.waitForTimeout(150); await scan(w + "/detalhes");
        await hover(w);
        if (sc) continue;
        const btns = p.locator(".page:not([hidden]) :is(button, summary, [role=button], a):visible");
        const nb = Math.min(await btns.count(), 90);
        for (let j = 0; j < nb; j++) {
          let txt = "";
          try { txt = (await btns.nth(j).innerText({ timeout: 300 })) + " " + ((await btns.nth(j).getAttribute("aria-label")) || ""); } catch { continue; }
          if (BAD.test(txt)) continue;
          try { await btns.nth(j).click({ timeout: 500 }); await p.waitForTimeout(200); await scan(`${w}/${txt.trim().slice(0, 24)}`); } catch {}
          await p.keyboard.press("Escape").catch(() => {});
          if ((await p.evaluate(() => state.view)) !== v) { await p.evaluate((x) => showView(x), v); await p.waitForTimeout(250); }
        }
      }
    }
    await ctx.close();
  }
  await b.close();
  return { scans, found, errs };
}

(async () => {
  let bad = 0;
  const a = part1();
  for (const f of a.fails) console.log("  " + f);
  console.log(`parte 1: ${a.n} textos, ${a.fails.length} com português sobrando`);
  bad += a.fails.length;
  if (process.env.RADLOG_INGLES_TELAS) {
    const r = await part2();
    for (const [t, w] of r.found) console.log(`  ${w}\t${t}`);
    for (const e of r.errs) console.log("  erro de script: " + e);
    console.log(`parte 2: ${r.scans} leituras da tela, ${r.found.size} textos com português, ${r.errs.size} erros`);
    bad += r.found.size + r.errs.size;
  }
  process.exit(bad ? 1 : 0);
})();
