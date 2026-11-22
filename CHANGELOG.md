# Versões

## 2.104.1
- Resumo, Comparar: a coluna "A − B" agora se chama "diferença" e diz o
  sentido com os nomes dos dois períodos (por exemplo, Hoje − Ontem).

**English**
- Summary, Compare: the "A − B" column is now called "difference" and shows
  the direction with the names of the two periods (for example, Today −
  Yesterday).

## 2.104.0
Atividade (Bq): calibração por montagem.
- A calibração com KCl agora é de uma montagem (o pote, o material e a
  posição do RC102). Escolha o pote (pote de 1 L com tubo, béquer ou outra),
  o material (KCl de adubo, KCl puro ou sal light) e o fundo (pote igual com
  sal de cozinha comum, ou sem nada com o KCl a 3 m). Cada montagem ganha um
  nome e pode ser renomeada ou apagada.
- Fundo e KCl têm sempre o mesmo tempo. Uma rodada curta não se perde: dá
  para somar outra rodada na mesma montagem e o app junta as duas (a soma
  das áreas sobre a soma do esperado, inclusive rodadas em que o pico não
  passou do fundo).
- O ± da montagem separa a contagem dos erros fixos (o teor do material e,
  sem branco, o fundo sem sal). A montagem só fica pronta para medir com
  ±20% ou menos no total.
- Conferências durante o KCl: com 10 min, se a faixa de 300 a 1.400 keV
  subiu (o RC102 está vendo o KCl); com 30 min, o pico do K-40, se o sinal
  combina com a faixa e a previsão de ± no fim do tempo marcado.
- Na medida de amostra, escolha a montagem; a conta usa a eficiência e a
  escala de energia dela. A calibração feita depois da correção do pico
  (2.103.0) vira a primeira montagem sozinha. Sem montagem, a amostra sai
  com o chute (±100%), não mais com a calibração única de antes.
- Durante o KCl, a previsão mostra o ± total da montagem no fim (as rodadas
  somadas mais o teor do material), e o tempo recomendado já conta com ele.

**English**
Activity (Bq): calibration per setup.
- The KCl calibration now belongs to a setup (the pot, the material and the
  RC102 position). Pick the pot (1 L pot with tube, beaker or other), the
  material (fertilizer KCl, pure KCl or light salt) and the background (same
  pot with regular table salt, or nothing with the KCl 3 m away). Each setup
  gets a name and can be renamed or deleted.
- Background and KCl always take the same time. A short round is not
  wasted: you can add another round to the same setup and the app combines
  them (sum of areas over sum of expected, including rounds where the peak
  did not rise above the background).
- The setup's ± separates counting from the fixed errors (the material's
  content and, without a blank, the salt-free background). A setup is only
  ready to measure at ±20% or less in total.
- Checks during the KCl phase: at 10 min, whether the 300 to 1,400 keV band
  rose (the RC102 sees the KCl); at 30 min, the K-40 peak, whether the
  signal matches the band and the ± forecast at the end of the chosen time.
- When measuring a sample, pick the setup; the calculation uses its
  efficiency and energy scale. The calibration made after the peak fix
  (2.103.0) becomes the first setup on its own. Without a setup, a sample
  uses the guess (±100%), no longer the single calibration from before.
- During the KCl phase, the forecast shows the setup's total ± at the end
  (rounds summed plus the material's content), and the recommended time
  already accounts for it.

## 2.103.0
Atividade (Bq): o pico do K-40 achado onde ele está (avaliação independente
de 21/11):
- A escala de energia que o RC102 manda pode estar uns % fora; no aparelho
  do ACE o K-40 aparece em ~1.415 keV, não em 1.461. O app procurava o pico
  no lugar fixo e pegava só 40–60% dele: as calibrações saíam ~40% baixas.
  Agora cada medida acha o K-40, corrige a escala e mostra "K-40 achado em
  X keV". Calibrações feitas antes disso aparecem como "feita antes da
  correção do pico: calibre de novo".
- O ajuste dos picos pesa cada canal pela média dos vizinhos, não pela
  contagem dele sozinho: a área não sai mais 20–30% baixa em medidas curtas
  e o "detectado" falso em pote vazio volta a ~5%.
- Na calibração com KCl, o ajuste usa só o K-40 (mais o contínuo): as linhas
  vizinhas não existem no sal − fundo e só tiravam ou inventavam área.
- O fundo é posto na mesma escala de energia da amostra antes da conta.
- Os dois espectros de cada medida (amostra e fundo) ficam guardados, para
  refazer a conta quando o programa melhorar.

**English**
Activity (Bq): the K-40 peak found where it really is (independent review of
11/21):
- The energy scale the RC102 sends can be a few % off; on ACE's device K-40
  shows at ~1,415 keV, not 1,461. The app looked for the peak at a fixed
  place and caught only 40–60% of it: calibrations came out ~40% low. Now
  each measurement finds K-40, corrects the scale and shows "K-40 found at
  X keV". Calibrations made before this show as "made before the K-40 peak
  fix: calibrate again".
- The peak fit weights each channel by the average of its neighbours, not
  by its own count: the area no longer comes out 20–30% low in short
  measurements, and false "detected" in an empty jar is back to ~5%.
- In the KCl calibration the fit uses only K-40 (plus the continuum): the
  neighbouring lines do not exist in salt − background and only took away or
  invented area.
- The background is put on the same energy scale as the sample before the
  calculation.
- Both spectra of each measurement (sample and background) are kept, so the
  calculation can be redone when the program improves.

## 2.102.2
Calibração do KCl:
- "eficiência até agora": o tempo estimado agora é para ±15%, o que a
  calibração mira (antes ±3%, que o RC102 não alcança: dava "~1.676 h"). E
  a linha não some mais quando o pico ainda está abaixo do fundo (normal no
  começo): diz isso.
- Histórico: a calibração salva com "Salvar mesmo assim" aparecia duas vezes
  (a nuvem trazia de volta a linha antiga, "não guardada"). Agora fica uma
  linha por medida, a usada.

Uso do app (o resumo que o ACE copia), conferido pelo QA:
- Abas do Fundo da casa sem o resumo grudado no nome; Gráficos sem a
  sub-aba única; o OK da janela de confirmação não conta mais como botão da
  página; "<1 min" quando abriu sem tempo e "nunca" quando nunca abriu; o
  seletor do Espectro do RC102 separa Espectro, Tela e Acumulado e não conta
  o clique feito por código.

**English**
KCl calibration:
- "efficiency so far": the time estimate is now for ±15%, what the
  calibration aims for (it was ±3%, which the RC102 cannot reach: it gave
  "~1,676 h"). And the line no longer disappears while the peak is still
  below the background (normal at the start): it says so.
- History: a calibration saved with "Save anyway" showed up twice (the cloud
  brought back the old "not saved" line). Now there is one line per
  measurement, the used one.

App usage (the summary ACE copies), checked by QA:
- Home background tabs without the summary stuck to the name; Charts
  without its single sub-tab; the confirmation dialog's OK no longer counts
  as a page button; "<1 min" when opened without time and "never" when never
  opened; the RC102 Spectrum selector tells Spectrum, Device screen and
  Accumulated apart and ignores clicks made by code.

## 2.102.1
Espectro: análise dos picos mais honesta (o mesmo critério do celular):
- Com pouco tempo de espectro, o modo "agora" marcava quase tudo como
  "certo". Agora cada linha é graduada como certo, provável ou fraco pelo σ,
  por quanto o pico sobe acima do que tem embaixo (pelo menos 30%, ou 60% se
  estiver sozinho) e se outra linha da mesma série apareceu junto. O detalhe
  "Por que" mostra essa conta. No Acumulado do aparelho continua valendo só
  o σ (lá essa conta depende do formato do espectro, não do tempo).
- Linha sozinha que sobe menos de 10% acima do que tem embaixo some da tela
  ao vivo (é ruído ou deriva, não isótopo).
- 186 keV agora é "Ra-226 (ou U-235)": o rádio solta quase a mesma energia e
  está em tudo que tem urânio. Cada um aparece com a sua energia (Ra-226
  186,2 keV, U-235 185,7 keV).
- Inglês: frases já traduzidas coladas uma na outra não são mais traduzidas
  de novo ("has no other" virava "has on the other").
- Dose acumulada, "Tudo": o quadro do RC102 dizia "faltam 10 dias sem dado"
  contando os dias antes de existir o RC102 (o "Tudo" começa no primeiro dado
  do GMC-800). Agora a falta conta desde o primeiro dado do RC102.
- Atividade (Bq): o aviso de "fundo bem acima do normal da casa" acusava o
  sal à toa (o ACE, com nada perto: 15,8 × 12,4 cps). Agora não culpa o sal
  e diz que minério no mesmo cômodo também sobe o fundo: se ele ficar no
  mesmo lugar até o fim, pode seguir. O aviso nunca travou a medição.
- Radiacode 102: sem o aparelho conectado, Espectro, Busca, Espectrograma e
  Dureza não mostram mais os exemplos simulados (pedido do ACE); cada cartão
  diz só o que falta. Os exemplos ficam na demonstração.
- Calibração do KCl: quando a incerteza passa um pouco do limite de ±20%
  (até ±30%), aparece "Salvar mesmo assim", também no histórico (o ACE tinha
  ficado em ±20,3%). O aviso agora diz a incerteza em vez de "o pico quase
  não apareceu". O tempo sugerido de cada fase passou a usar o que as duas
  calibrações do ACE mostraram (250 g de sal light: ~11 h por fase).
- O último canal (onde caem as contagens que estouram a escala) não é mais
  desenhado, então sumiu o falso pico perto de 3000 keV.

**English**
Spectrum: more honest peak analysis (the same criterion as the phone):
- With little spectrum time, the "now" mode marked almost everything as
  "sure". Each line is now graded sure, likely or weak by σ, by how much the
  peak rises above what is underneath it (at least 30%, or 60% if it is
  alone) and by whether another line from the same series showed up too. The
  "Why" detail shows the math. In the device's Accumulated mode only σ still
  counts (there this math depends on the spectrum's shape, not on time).
- A lone line rising less than 10% above what is underneath disappears from
  the live screen (it is noise or drift, not an isotope).
- 186 keV is now "Ra-226 (or U-235)": radium gives off almost the same energy
  and is in everything that has uranium. Each one shows its own energy
  (Ra-226 186.2 keV, U-235 185.7 keV).
- English: sentences already translated and joined together are no longer
  translated again ("has no other" became "has on the other").
- Accumulated dose, "All": the RC102 box said "no data for 10 days",
  counting the days before the RC102 existed ("All" starts at the GMC-800's
  first data). Missing time now counts from the RC102's first data.
- Activity (Bq): the "background well above the home normal" warning blamed
  the salt for nothing (ACE, with nothing nearby: 15.8 × 12.4 cps). It no
  longer blames the salt and says ore in the same room also raises the
  background: if it stays in place until the end, you can go on. The warning
  never blocked the measurement.
- Radiacode 102: with the device disconnected, Spectrum, Search, Spectrogram
  and Hardness no longer show the simulated examples (ACE's request); each
  card just says what is missing. The examples stay in the demo.
- KCl calibration: when the uncertainty is a bit over the ±20% limit (up to
  ±30%), "Save anyway" shows up, in the history too (ACE's came out at
  ±20.3%). The warning now states the uncertainty instead of "the peak barely
  showed". The suggested time for each phase now uses what ACE's two
  calibrations showed (250 g of lite salt: ~11 h per phase).
- The last channel (where counts beyond the scale land) is no longer drawn,
  so the false peak near 3000 keV is gone.

## 2.102.0
Zoom dos gráficos (pedido do ACE, com o design aprovado):
- O zoom vale só enquanto você está nos Gráficos. Saiu da página, ele some,
  e na volta pela barra lateral os gráficos abrem no período de sempre, ao
  vivo. Quem chega por um "Ver no gráfico" continua vendo o trecho.
- A barra do zoom diz de onde o trecho veio ("aberto pelo Fora de casa", com
  o nome do trecho) e tem o link de volta ("← Fora de casa"). "Sair do zoom"
  virou "Voltar para 1 h" (o período guardado, que fica marcado de leve), e a
  barra avisa que o gráfico está parado no trecho. Anotar, Fora de casa,
  Relatório e Voltar ao zoom anterior foram para o "⋯".
- Fita do dia: o clique abre um balão com o horário e o que aconteceu, e um
  botão "Abrir 04:15–05:15 nos gráficos" (1 h em volta do ponto). Não troca
  mais de tela sozinho. Esc ou clique fora fecha.
- Arrastar no gráfico só vira zoom depois de uns 24 px (antes eram 8), e
  mostra o horário do trecho enquanto arrasta. Esc cancela.

**English**
Chart zoom (ACE's request, with the approved design):
- Zoom only lasts while you are on Charts. Leave the page and it goes away;
  coming back from the sidebar, the charts open on the usual period, live.
  Arriving from a "See on the chart" still shows the stretch.
- The zoom bar says where the stretch came from ("opened from Away from
  home", with the stretch's name) and has a link back ("← Away from home").
  "Exit zoom" became "Back to 1 h" (the saved period, lightly outlined), and
  the bar says the chart is frozen on the stretch. Note, Away from home,
  Report and Back to the previous zoom moved into the "⋯".
- Day strip: a click opens a balloon with the time and what happened, and a
  button "Open 04:15–05:15 on the charts" (1 h around the point). It no
  longer switches screens by itself. Esc or a click outside closes it.
- Dragging on the chart only zooms after about 24 px (it was 8), and shows
  the stretch's time while dragging. Esc cancels.

## 2.101.16
- Dose acumulada › a conta, em inglês: o limite saía pela metade em
  português ("público: 1 mSv/ano above background"). Agora sai inteiro no
  idioma da tela. A demonstração passou a usar o mesmo nome do limite que o
  app, e o teste do inglês pega esse caso.

**English**
- Accumulated dose › the math, in English: the limit came out half in
  Portuguese ("público: 1 mSv/ano above background"). It now comes out whole
  in the screen's language. The demo now uses the same limit name as the app,
  and the English test catches this case.

## 2.101.15
Itens 133 a 138 da varredura (achados do QA):
- Medir objeto com o RC102: o ± e o σ do líquido, o "Objeto ÷ fundo" e a
  precisão vêm todos da dose (na tela, no PDF e nos Eventos), e não mais
  metade da contagem.
- Medir objeto, parada no tempo máximo: o "menos que +X" sai uma vez só, e
  quando quem limita é o fundo medido na hora, a dica diz para medir o fundo
  por mais tempo ou usar o fundo da casa. Na tela ao vivo, a frase não
  repete mais "chegou ao tempo máximo".
- Atividade (Bq): numa pedra solta, as medidas anteriores dizem "pedra
  solta" em vez da Posição do pote da calibração, e a Posição escrita por
  você não é mais traduzida pela metade em inglês.
- Demonstração: a calibração nova grava a Posição nova, como no app.

**English**
Items 133 to 138 from the sweep (QA findings):
- Measure object with the RC102: the net ± and σ, "Object ÷ background" and
  the precision all come from the dose (on screen, in the PDF and in
  Events), no longer half from the count.
- Measure object, stop at the time limit: the "less than +X" shows only once,
  and when the background measured on the spot is what limits, the tip says
  to measure the background longer or use the home background. On the live
  screen, the sentence no longer repeats "reached the time limit".
- Activity (Bq): for a loose stone, past measurements say "loose stone"
  instead of the calibration jar's Position, and a Position you typed is no
  longer half translated in English.
- Demo: a new calibration saves the new Position, as in the app.

## 2.101.14
Itens 17, 35 e 68 da varredura (ok do ACE):
- Medir objeto com o RC102 no automático: a dose também decide. Ele só diz
  "igual ao fundo" quando a contagem e a dose ficam abaixo do limiar (depois
  de 5 min), e para dizendo que a dose subiu quando ela passa de 4σ. A
  medição salva, os Eventos e o PDF não dizem mais "igual ao fundo" quando a
  dose subiu, e o "menos que +X" tem um número só em todo lugar (no RC102,
  pela dose).
- Atividade (Bq): uma calibração nova pior que a que está em uso não troca
  sozinha. O app mostra as duas e oferece "Usar a nova"; a de antes fica
  guardada e dá para voltar em detalhes.
- Atividade (Bq) › Mais opções: "Como está a amostra: no pote | pedra
  solta" para qualquer amostra (pedra solta deixa a massa opcional).

**English**
Items 17, 35 and 68 from the sweep (ACE's ok):
- Measure object with the RC102 in automatic: the dose also decides. It only
  says "same as background" when both count and dose stay below the
  threshold (after 5 min), and stops saying the dose rose when it passes 4σ.
  The saved measurement, Events and PDF no longer say "same as background"
  when the dose rose, and the "less than +X" has a single number everywhere
  (on the RC102, from the dose).
- Activity (Bq): a new calibration worse than the one in use no longer
  replaces it by itself. The app shows both and offers "Use the new one";
  the previous one is kept and you can go back to it in details.
- Activity (Bq) › More options: "How the sample is: in the jar | loose
  stone" for any sample (loose stone makes the mass optional).

## 2.101.13
Itens 21, 22, 62, 66, 67, 79, 92 e 125 da varredura (ok do ACE), e o 132:
- Configurações: valor fora da faixa já avisa no campo, antes de salvar
  ("O mínimo é 5 min. Vai gravar 5."); depois de salvar aparece corrigido e
  o aviso diz o que ficou ("Fora da faixa, ficou: …").
- "Restaurar padrões dos níveis" volta também o Alarme e o "Ligado" da
  Atenção e do alarme (o alarme religa no Salvar), e só mexe na seção
  Níveis e alarmes.
- Saída programada que passou do horário (começada ou não): no lugar de
  "editar" aparece "corrigir aparelhos", que salva a correção; a começada
  continua começada se o aparelho que saiu continua indo.
- Agora: quando o fundo de casa fica acima do normal sem alerta (pela média
  dos últimos 10 min: aparece com 3σ, some abaixo de 2σ), a frase do topo
  diz isso e mostra o "Por que?". Os segundos com o GMC-800 num objeto
  (Medir objeto ou "Estou testando") ficam fora dessa média.
- Configurações › Níveis e alarmes e GQ GMC-800 › Tubo: a tabela não some
  mais quando aparece um aviso embaixo de um campo (item 132).
- Aviso de memória no quadro do GQ GMC-800: clicar abre a Memória do
  contador.
- Janela estreita: o menu de páginas mostra botões redondos ‹ › e degradê
  quando há mais páginas, e a roda do mouse rola o menu para os lados.
- "Cancelar a saída programada?": os botões viram "Manter a saída" e
  "Cancelar a saída".
- Demonstração em inglês: os nomes de exemplo (medições, saídas, notas e
  nuvem) saem em inglês.

**English**
Items 21, 22, 62, 66, 67, 79, 92 and 125 from the sweep (ACE's ok), and 132:
- Settings: an out-of-range value warns on the field before saving ("The
  minimum is 5 min. It will save 5."); after saving it shows corrected and
  the saved message says what was kept ("Out of range, set to: …").
- "Restore level defaults" also restores the Alarm and the "On" boxes (the
  alarm turns back on at Save), and only touches the Levels and alarms
  section.
- Planned trip past its time (started or not): instead of "edit" there is
  "fix devices", which saves the fix; a started trip stays started while the
  device that left still goes.
- Now: when the home background stays above normal without an alert (by the
  last 10 min average: shows at 3σ, hides below 2σ), the top sentence says
  so and shows "Why?". Seconds with the GMC-800 on an object (Measure object
  or "Testing an object") stay out of that average.
- Settings › Levels and alarms and GQ GMC-800 › Tube: the table no longer
  disappears when a warning shows under a field (item 132).
- Memory warning on the GQ GMC-800 card: clicking opens the counter memory.
- Narrow window: the page menu shows round ‹ › buttons and a fade when
  there are more pages, and the mouse wheel scrolls the menu sideways.
- "Cancel the planned trip?": the buttons are now "Keep the trip" and
  "Cancel the trip".
- English demo: sample names (measurements, trips, notes and cloud) are in
  English.

## 2.101.12
Itens 130 e 131 da varredura do QA:
- App em inglês: os nomes que o próprio app dá saem traduzidos ("amostra"
  da Atividade sem nome, "objeto" da medição sem nome, "Saída programada",
  "Levei os dois", cartões prontos e calibração nos Eventos); a pedra solta
  sem massa aparece como "pedra solta" nos Eventos, não "0 g".
- Busca (Ctrl+K) › Medições salvas: a linha de baixo segue a unidade e o
  formato de data escolhidos e o idioma.

**English**
Items 130 and 131 from QA's sweep:
- English app: names the app itself gives are translated ("sample" for an
  unnamed Activity, "object" for an unnamed measurement, "Planned trip",
  "Took both", ready cards and calibration in Events); a loose stone without
  mass shows as "loose stone" in Events, not "0 g".
- Search (Ctrl+K) › Saved measurements: the second line follows the chosen
  unit, date format and language.

## 2.101.11
Calibração do KCl (Atividade, RC102) com tempo honesto:
- O chute de eficiência sem calibração cai de 0,1% para 0,005% em 1.460 keV,
  que é o que o cristal de 1 cm³ do RC102 deu com 250 g de sal light em volta.
  Os Bq sem calibração mudam de acordo (seguem com ±100%).
- O tempo da calibração sai da massa e do % de KCl: o fundo e o sal medem o
  mesmo tempo, o suficiente para a eficiência sair com ~15% de dúvida (500 g
  de sal light: ~4 h; 500 g de KCl puro: ~2 h). Se passar de 24 h, o app
  avisa antes de começar.
- O aviso "não está vendo o sal" olha a faixa de 300 a 1.400 keV aos 20 min
  (o sal em volta blinda o fundo de baixa energia, e o total pode até cair) e
  o mínimo plausível no pico passa a 0,001%.

**English**
KCl calibration (Activity, RC102) with an honest time:
- The efficiency guess without calibration drops from 0.1% to 0.005% at
  1,460 keV, which is what the RC102's 1 cm³ crystal gave with 250 g of lite
  salt around it. Bq without calibration change accordingly (still ±100%).
- The calibration time comes from the mass and the KCl %: background and
  salt measure the same time, enough for the efficiency to come out at ~15%
  uncertainty (500 g of lite salt: ~4 h; 500 g of pure KCl: ~2 h). Over 24 h,
  the app warns before starting.
- The "not seeing the salt" warning looks at 300 to 1,400 keV at 20 min (the
  salt around the device shields the low-energy background, and the total can
  even drop) and the plausible minimum at the peak becomes 0.001%.

## 2.101.10
Sobras da conferência do QA na 2.101.9:
- Resumo, "De onde vem a sua dose": o nome da medição, da anotação e do
  trecho fica como você escreveu no app em inglês ("Relógio" não vira
  "Clock").
- Na mesma tabela, só saída de verdade (GMC-800 junto ou "eu não estava em
  casa") vira "Fora de casa"; GMC-800 ignorado ou os dois em casa não.
- Por que subiu?: a anotação perto da subida fica como você escreveu.
- Atividade (Bq): o nome que você deu à amostra não é traduzido no
  histórico, no resultado e nos detalhes; os cartões prontos continuam
  traduzidos (também durante a medida, nas medidas antigas e no aviso de
  tirar a amostra); Uraninita, Uranocircita e Euxenita ganham nome em inglês.
- Gráficos, Resumo, Fundo e Padrões: a dica perto da borda direita não passa
  mais da janela em inglês nem em mrem/h (era medida antes de traduzir e
  converter). Na dica do Fundo da casa > Outra conta a média também sai em mrem/h.

**English**
Leftovers from QA's check of 2.101.9:
- Summary, "Where your dose comes from": the measurement, note and stretch
  names stay as you wrote them in the English app ("Relógio" no longer
  becomes "Clock").
- In the same table, only a real trip (GMC-800 with you or "I wasn't home")
  becomes "Away from home"; an ignored GMC-800 or both left at home does not.
- Why did it rise?: the note near the rise stays as you wrote it.
- Activity (Bq): the name you gave the sample is not translated in the
  history, the result and the details; ready cards are still translated
  (also during the measurement, in older measurements and in the remove
  notice); Uraninite, Uranocircite and Euxenite get English names.
- Charts, Summary, Background and Patterns: the tooltip near the right edge
  no longer runs past the window in English or in mrem/h (it was measured
  before translating and converting). In the Home background > Another calculation
  tooltip the average is also shown in mrem/h.

## 2.101.9
Achados da conferência do QA na 2.101.8:
- O nome que você deu a uma medição ou a um trecho não é mais traduzido no
  app em inglês (a medição "Sal light" não vira "Lite salt", o trecho
  "Escritório" não vira "Office"), em todo lugar: lista e detalhe da
  medição, dica e marcadores do gráfico, anotações e notas, faixa do trecho,
  Onde do Agora, etiqueta do alerta, régua, avisos do RC102 fora, evento
  "Medição de …", janela e PDF. Os nomes que o app dá (testes de Distância e Blindagem, "Saída de …")
  continuam traduzidos.
- Relatório de Fora de casa em inglês: o rodapé das páginas sai em inglês.
- PDF da medição em inglês: "Configuração na época" traduz cada ajuste e a
  origem da curva; o rótulo "Mudou desde a medição" cabe na coluna.
- Janela do relatório: um dia fechado termina em "24:00" do mesmo dia (não
  "00:00").

**English**
Findings from QA's check of 2.101.8:
- The name you gave a measurement or a stretch is no longer translated in
  the English app ("Sal light" no longer becomes "Lite salt", "Escritório" no
  longer becomes "Office"), everywhere: measurement list and detail, chart
  tooltip and markers, notes, stretch strip, Where on Now, alert tag, ruler,
  RC102-away notices, the "Measurement of …" event, the window and the PDF.
  Names the app gives (Distance and Shielding tests, "Trip on …") are still
  translated.
- Away from home report in English: the page footer is in English.
- Measurement PDF in English: "Settings at the time" translates each setting
  and the curve source; the "Changed since then" label fits its column.
- Report window: a closed day ends at "24:00" of that day (not "00:00").

## 2.101.8
Correções da varredura do QA na 2.101.2 (defeitos de gravidade média, parte 3,
e as sobras da conferência da 2.101.6):
- Saída de casa tem uma regra só: o GMC-800 foi junto ou o trecho diz "eu
  não estava em casa". Ignorar o GMC-800 (tubo quebrado) ou deixar os dois em
  casa não vira saída: sem "Resumo da saída", sem aviso "Saída fechada", fora
  do total "RC102 fora de casa", sem "fora de casa" na Fita e fora das saídas
  do PDF; o fundo desses trechos conta como fundo em casa. Com o GMC-800
  ignorado também dá para marcar "eu não estava em casa".
- O aviso "Saída fechada" não aparece mais com a saída ainda aberta (ao
  renomear, adicionar trecho ou responder o cartão do RC102).
- Gráficos em CPS: os andares dizem "· CPS" e o eixo mostra 0,25 (não 0,3).
- Espectrograma e log do RC102: trocar o período ou o filtro durante a
  leitura não perde mais o pedido novo. O log sem nenhum tipo marcado diz
  isso (não "Ainda sem registros").
- Som da Busca só toca com a aba Busca aberta.
- Telas estreitas e letra grande: tabela dos níveis, setinha da barra
  lateral, valor do RC102 e da Busca em mrem/h, "← Voltar" da Distância e
  Blindagem, tabela do Comparar, "Ver no gráfico" sem ficar sob a faixa dos
  aparelhos, tabela do tempo morto, caminho das versões baixadas, horas do
  detalhe do alerta, seletores do Agora e dos Alertas, tabela das duas fases
  e o número do mostrador da Busca.
- "Máx 24 h" diz "ontem às 03:46" quando o máximo foi ontem.
- Padrões: diferença pequena entre dia e noite, mas fora do acaso, diz
  "pequena, mas real" (não "sem diferença clara").
- A Fita do dia conta um alerta novo na hora e não acumula ouvintes.
- PDF: a linha "Unidade" do anexo (e a "Configuração na época" da medição)
  não sai mais cortada.
- Plural certo em "1 registro", "1 minuto novo", "1 linha ignorada".
- Atividade (Bq): "Guardar como minha" de um cartão pronto leva o subtítulo e
  o preparo de novo (só o "Outra coisa" vai sem); a lista de distâncias
  aceita "2,5, 5, 10" e "2,5 5 10".
- Calibrar: o cartão do KCl diz ~3 h com sal light e ~2 h com KCl puro.
- Inglês: saída programada sem nome, notas do Ra-226/Ra-228, "your
  reference" depois de "Save as mine", "Download history", janelas de
  arquivo do Windows, "· same" da dose por período, janela e PDF do relatório
  ("this week", "selected stretch", "Percentiles", "Gap", "Program", dias da
  semana), chip de motivo, Nome e Posição em Atividade, "no mass", filtro do
  log, eventos com o nome da pessoa, textos do espectrograma, nota e chuva na
  dica dos Gráficos, "Place:", "what is σ?" e "(2 days)" do zoom.
- Modo demonstração: "fica calado" para o Perigo do outro aparelho vale.
- Sobras da 2.101.7: o PDF da medição compara o fator do fundo de então com
  o de hoje (acima da curva não dá mais "hoje" falso); a Hora do dia diz os
  dias com dado (não "em 90 dias" com 8 dias de histórico); o ± do subtítulo
  do gráfico da medição com um algarismo; "−9%" com sinal de menos na dose
  por período.

**English**
Fixes from QA's sweep of 2.101.2 (medium-severity defects, part 3, and the
leftovers from the 2.101.6 check):
- Leaving home has a single rule: the GMC-800 went with you or the stretch
  says "I wasn't home". Ignoring the GMC-800 (broken tube) or leaving both at
  home is no longer a trip: no "Trip summary", no "Trip closed" notice, left
  out of the "RC102 away from home" total, no "away from home" on the day
  strip and out of the PDF trips; the background of those stretches counts as
  home background. With the GMC-800 ignored you can also mark "I wasn't
  home".
- The "Trip closed" notice no longer shows while the trip is still open
  (when renaming, adding a stretch or answering the RC102 card).
- Charts in CPS: the stacked panels say "· CPS" and the axis shows 0.25 (not
  0.3).
- RC102 spectrogram and log: changing the period or filter during a load no
  longer loses the new request. The log with no type selected says so (not
  "No records yet").
- The Search sound only plays with the Search tab open.
- Narrow windows and large text: level table, side bar arrow, RC102 and
  Search value in mrem/h, "← Back" in Distance and Shielding, Compare table,
  "Show on chart" no longer under the device strip, dead-time table, download
  folder path, alert detail times, Now and Alerts selectors, two-phase table
  and the Search dial number.
- "Max 24 h" says "yesterday at 03:46" when the maximum was yesterday.
- Patterns: a small but real day/night difference says "small, but real"
  (not "no clear difference").
- The day strip counts a new alert right away and no longer piles up
  listeners.
- PDF: the appendix "Unit" row (and the measurement "Settings at the time")
  are no longer cut off.
- Correct singular in "1 record", "1 new minute", "1 line skipped".
- Activity (Bq): "Save as mine" from a ready card keeps the subtitle and
  preparation again (only "Something else" goes without); the distance list
  accepts "2,5, 5, 10" and "2,5 5 10".
- Calibrate: the KCl card says ~3 h with lite salt and ~2 h with pure KCl.
- English: unnamed planned trip, Ra-226/Ra-228 notes, "your reference" after
  "Save as mine", "Download history", Windows file dialogs, "· same" in dose
  by period, report window and PDF ("this week", "selected stretch",
  "Percentiles", "Gap", "Program", weekdays), reason chip, Name and Position
  in Activity, "no mass", log filter, events with your own names,
  spectrogram texts, note and rain in the chart tooltip, "Place:", "what is
  σ?" and the zoom "(2 days)".
- Demo mode: "stays quiet" for the other device's Danger now applies.
- Leftovers from 2.101.7: the measurement PDF compares the background
  factor then with today's (no false "today" above the curve); Time of day
  says the days with data (not "over 90 days" with 8 days of history); the ±
  in the measurement chart subtitle has at least one digit; "−9%" with a
  minus sign in dose by period.

## 2.101.7
Correções da varredura do QA na 2.101.2 (defeitos de gravidade média, parte 2,
e as sobras da conferência da 2.101.5):
- Medir objeto no RC102 com "a contagem não mudou, mas a dose subiu": a
  certeza, o "Objeto ÷ fundo", a precisão, o texto da tabela, os Eventos e o
  PDF passam a falar da dose (sem "menos que +X" pela contagem); a distância
  segura e os "Os dois aparelhos" contam a dose que subiu como "viu".
- PDF de uma medição salva mostra o fator que valeu nos números ("hoje: …"
  quando mudou) e o "Mudou desde a medição" numa linha própria.
- Fundo de casa: o RC102 que subiu um pouco "subiu menos" (não "ficou
  igual"); "Por que subiu?" diz "o RC102 também subiu, menos" em vez de
  "ficou parado"; as duas frases da Hora do dia dizem de quantos dias são.
- Minha dose: "perto do fundo da sua casa (+8%)" com o mesmo % da célula.
- Resumo › Comparar: "igual" só com os números mostrados iguais, sem
  "(igual)" ao lado de uma diferença, e o % da dose do RC102 só com tempos
  parecidos.
- mrem/h: eixo do mapa de calor, "≈ CPM" dos níveis, coluna do tempo morto,
  ajustes do RC102 (alarmes de taxa e de dose, dicas e janelas), Resumo da
  saída, "Por que subiu?", título da janela no alarme, tabelas do PDF e dicas
  de padrão ("Padrão 100 mrem/h"); o fator de calibração fica sempre em
  µSv/h por CPM; "RC102" não é mais lido como número.
- Resumo da saída: Esc no "editar título" desiste (e o Enter grava uma vez).
- Ajuda do "RC102 saturado" sem o "RC102 102".
- ± pequeno com pelo menos um algarismo (não mais "± 0,000").

**English**
Fixes from QA's sweep of 2.101.2 (medium-severity defects, part 2, and the
leftovers from the 2.101.5 check):
- Measure object with the RC102 when "the count didn't change, but the dose
  rose": certainty, "Object ÷ background", precision, the table text, Events
  and the PDF now talk about the dose (no "less than +X" from the count); the
  safe distance and "Both devices" count the rising dose as "seen".
- The PDF of a saved measurement shows the factor used in its numbers
  ("today: …" when it changed) and "Changed since the measurement" on its own
  line.
- Home background: an RC102 that rose a bit "rose less" (not "stayed the
  same"); "Why did it rise?" says "the RC102 also rose, less" instead of
  "stayed flat"; both Time of day sentences say how many days they cover.
- My dose: "close to your home background (+8%)" with the cell's same %.
- Summary › Compare: "same" only when the shown numbers are equal, no
  "(same)" next to a difference, and the RC102 dose % only with similar times.
- mrem/h: heat map axis, the levels' "≈ CPM", the dead time column, RC102
  settings (rate and dose alarms, hints and dialogs), trip summary, "Why did
  it rise?", the window title on alarm, PDF tables and default hints
  ("Default 100 mrem/h"); the calibration factor always stays in µSv/h per
  CPM; "RC102" is no longer read as a number.
- Trip summary: Esc in "edit title" cancels (and Enter saves once).
- "RC102 saturated" help without "RC102 102".
- Small ± values keep at least one digit (no more "± 0.000").

## 2.101.6
Correções da varredura do QA na 2.101.2 (defeitos de gravidade média, parte 1):
- RC102: dá para ligar só o alarme 2 de dose (com o 1 desligado, o app não
  diz mais que o 2 é menor que o 1).
- RC102 › Desfazer a última gravação: com o aparelho já igual à cópia, avisa
  que nada foi gravado e não perde o passo anterior do Desfazer.
- Atividade (Bq): "até eu parar" guardado em Suas amostras volta como "até eu
  parar" (não como 1 h); guardar de novo atualiza a mesma amostra (sem cartões
  repetidos e sem os textos do cartão de modelo), e o botão vira "Atualizar
  minha amostra"; a Posição segue a da calibração enquanto você não mexe nela
  e não volta sozinha com o cursor no campo; "dentro do típico" segue a marca
  (com a ressalva "mas a incerteza é grande"); "falta para ±10%" conta a
  incerteza da calibração e diz quando não chega.
- Importar backup com uma mudança ainda não salva: avisa que ela será
  descartada e mostra os valores do backup (o Salvar seguinte não desfaz mais
  o backup).
- Calibração › Avançado: o fator digitado e ainda não usado não volta sozinho
  ao fechar e abrir.
- Fator da intercalibração com o RC102 aparece como tal no relatório e na
  origem da calibração (não como "informado à mão").
- Depois de mudar o fator do GMC-800, o alarme do aparelho e os "≈ CPM" dos
  níveis são refeitos na hora.
- Os avisos do Salvar e o botão "Gravar no GMC-800" usam os nomes certos.
- Medir objeto: objeto bem abaixo do fundo (−3σ ou menos) diz que o fundo
  provavelmente mudou, em vez de "nenhuma diferença, cabe no acaso".
- Medir objeto › Distância: "2,5; 5; 10; 20" vira 2,5, 5, 10 e 20 cm (com ";"
  a vírgula é decimal).

**English**
Fixes from QA's sweep of 2.101.2 (medium-severity defects, part 1):
- RC102: you can turn on only dose alarm 2 (with alarm 1 off, the app no
  longer says 2 is lower than 1).
- RC102 › Undo the last write: with the device already matching the copy, it
  says nothing was written and doesn't lose the previous Undo step.
- Activity (Bq): "until I stop" saved in Your samples comes back as "until I
  stop" (not 1 h); saving again updates the same sample (no duplicate cards
  and no template texts), and the button becomes "Update my sample"; the
  Position follows the calibration's while you haven't touched it and no
  longer refills itself with the cursor in the field; "within typical"
  follows the mark (with "but the uncertainty is large"); "left for ±10%"
  includes the calibration uncertainty and says when it can't get there.
- Importing a backup with an unsaved change: warns it will be discarded and
  shows the backup's values (the next Save no longer undoes the backup).
- Calibration › Advanced: a typed factor not yet used no longer resets when
  closing and opening it.
- The RC102 intercalibration factor shows as such in the report and the
  calibration source (not as "entered by hand").
- After changing the GMC-800 factor, the device alarm and the levels'
  "≈ CPM" are redone right away.
- The Save warnings and the "Write to the GMC-800" button use the right names.
- Measure object: an object well below the background (−3σ or less) says the
  background probably changed, instead of "no difference, within chance".
- Measure object › Distance: "2,5; 5; 10; 20" becomes 2.5, 5, 10 and 20 cm
  (with ";" the comma is decimal).

## 2.101.5
Correções da varredura do QA na 2.101.2 (os defeitos de gravidade alta):
- Medir objeto: se o aparelho da medição desconecta, a medição não fica mais
  presa calando a atenção e o alarme do outro aparelho; depois de 2 min sem
  leitura o silêncio acaba.
- Níveis de Atenção e Perigo: o valor salvo reaparece igual na tela, e salvar
  de novo não troca o número.
- Configurações: alarmes e ajustes dos aparelhos digitados e não gravados não
  somem ao sair e voltar; a barra de salvar mostra que há ajuste pendente.
- Fotos .jfif, .jpe, .pjpeg e parecidas têm o GPS tirado antes do backup e da
  nuvem (o app olha o conteúdo do arquivo, não só a extensão).
- RC102: uma linha cortada no fim do CSV não gruda mais na próxima (o minuto
  falso de 5,3 µSv/h some). Com o CSV do mês travado (Excel, antivírus,
  OneDrive), os minutos esperam na memória e entram quando o arquivo libera,
  com aviso nos Eventos.
- "Saí de casa" e "Voltei" não devolvem ao cálculo o aparelho que estava fora
  dele (tubo com defeito, por exemplo).
- Calibrar o GMC-800 lado a lado com o RC102 ignora os minutos fora de casa e
  os de medição de objeto.
- Em mrem/h: frases com vários números convertem todos (e o "800" do
  "GMC-800" fica como está); o Medir objeto, o Resumo (Sua dose), a
  Blindagem e a estatística dos Gráficos saem todos na unidade escolhida.
- Gráficos: a "dose ≈" do zoom é a do aparelho escolhido.
- Medir objeto no RC102: "com o objeto" usa a taxa do próprio RC102 na fase
  (ao vivo, no gráfico salvo e no PDF), em vez da conversão do fundo, que
  escondia a subida do sal (K-40).
- Medições salvas › Editar e os formulários do Fora de casa (Programar saída,
  Adicionar e Editar trecho) guardam o que foi digitado ao trocar de tela.
- Atividade (Bq): clicar de novo no cartão já escolhido não apaga mais massa,
  nome, radionuclídeos e tempo.
- Também: desfazer "Mudei o aparelho de lugar" tira a anotação; editar um
  trecho com nome automático troca o nome junto com os aparelhos; "Estou
  testando um objeto" no RC102 não entra na dose; o backup leva as unidades e
  o formato da data; a porta USB escolhida não se perde ao salvar.
- Alertas: um alerta que começava no mesmo instante em que outro terminava
  (dois aparelhos) podia sumir do alertas.csv e só existir até fechar o app.

**English**
Fixes from QA's sweep of 2.101.2 (the high-severity defects):
- Measure object: if the measuring device disconnects, the measurement no
  longer stays stuck muting the other device's attention and alarm; after
  2 min without readings the mute ends.
- Attention and Danger levels: the saved value shows up unchanged, and saving
  again no longer changes the number.
- Settings: device alarms and settings typed but not written no longer vanish
  when leaving and coming back; the save bar shows a pending device setting.
- .jfif, .jpe, .pjpeg and similar photos have their GPS removed before backup
  and cloud (the app checks the file content, not only the extension).
- RC102: a cut line at the end of the CSV no longer sticks to the next one
  (the fake 5.3 µSv/h minute is gone). With the month's CSV locked (Excel,
  antivirus, OneDrive), minutes wait in memory and go in once the file is
  free, with a note in Events.
- "Left home" and "Back home" no longer put back into the calculation a
  device that was left out (a faulty tube, for example).
- Calibrating the GMC-800 side by side with the RC102 skips the minutes away
  from home and the ones measuring an object.
- In mrem/h: sentences with several numbers convert all of them (and the
  "800" in "GMC-800" stays as is); Measure object, Summary (Your dose),
  Shielding and the Charts statistics all show the chosen unit.
- Charts: the zoom's "dose ≈" is the chosen device's.
- Measure object with the RC102: "with the object" uses the RC102's own rate
  in that phase (live, in the saved chart and in the PDF) instead of the
  background's conversion, which hid the salt's rise (K-40).
- Saved measurements › Edit and the Away forms (Schedule a trip, Add and Edit
  a stretch) keep what was typed when switching screens.
- Activity (Bq): clicking the already chosen card again no longer clears
  mass, name, radionuclides and time.
- Also: undoing "I moved the device" removes its note; editing a stretch with
  an automatic name updates the name along with the devices; "I'm testing an
  object" on the RC102 stays out of the dose; the backup carries the units
  and the date format; the chosen USB port is no longer lost on saving.
- Alerts: an alert starting at the same moment another one ended (two
  devices) could vanish from alertas.csv and only last until the app closed.

## 2.101.4
- Calibração do KCl: quando o pico do K-40 quase não aparece acima do fundo
  (incerteza da eficiência acima de ±20%), a calibração não é mais guardada.
  Antes, qualquer pico positivo virava calibração, e uma medida ruim (o sal
  longe do cristal, por exemplo) deixava todos os Bq seguintes dezenas de
  vezes altos demais. O resultado mostra a eficiência medida e avisa que não
  foi guardada; a calibração anterior continua valendo.
- Aviso cedo (pedido do ACE, depois de 3 h perdidas numa calibração sem
  pico): durante o sal, a tela mostra a contagem total com o sal e a do fundo
  (+X%). Se aos 5 min ela quase não subiu (menos de 5%), ou se aos 30 min o
  pico do K-40 continua abaixo do mínimo plausível, aparece "O RC102 não está
  vendo o sal", com o que conferir, em vez de deixar rodar 2 h.
- Durante o fundo, a tela compara a contagem com o normal da casa (mediana
  dos 7 dias antes) e avisa se ela está mais de 25% acima: o sal ficou perto
  e a calibração sairia errada.

**English**
- KCl calibration: when the K-40 peak barely shows above the background
  (efficiency uncertainty above ±20%), the calibration is no longer saved.
  Before, any positive peak became the calibration, and a bad measurement
  (the salt away from the crystal, for example) made every later Bq tens of
  times too high. The result shows the measured efficiency and says it was
  not saved; the previous calibration stays.
- Early warning (ACE's request, after losing 3 h on a calibration with no
  peak): during the salt phase, the screen shows the total count rate with
  the salt and the background's (+X%). If after 5 min it barely rose (less
  than 5%), or after 30 min the K-40 peak is still below the lowest plausible
  value, "The RC102 isn't seeing the salt" appears with what to check,
  instead of running for 2 h.
- During the background, the screen compares the count rate with the home
  normal (median of the previous 7 days) and warns if it is more than 25%
  higher: the salt was nearby and the calibration would come out wrong.

## 2.101.3
- Calibração do KCl: a massa digitada voltava para 500 g quando se clicava
  em "Sal light", "KCl puro" ou no cartão "1 · KCl" depois de digitar (pedido
  do ACE: 250 g viraram 500 g). Agora a massa digitada fica, e cada referência
  (KCl e tório) guarda a sua; o padrão só entra num campo que ainda não foi
  mexido.
- Teste novo de ponta a ponta (roda no CI): pela tela, com o App de verdade e
  um RC102 de mentira, digita 250 g, 51% e a posição, clica em tudo que antes
  trocava a massa, passa pelo fundo e pelo sal e confere o resultado na tela,
  a calibração, o histórico e o que ficou salvo no disco. Com a versão
  anterior ele falha em "massa 500 depois de Sal light".

**English**
- KCl calibration: the typed mass went back to 500 g when you clicked "Lite
  salt", "Pure KCl" or the "1 · KCl" card after typing it (reported by ACE:
  250 g became 500 g). Now the typed mass stays, and each reference (KCl and
  thorium) keeps its own; the default only fills a field you haven't touched.
- New end-to-end test (runs in CI): through the screen, with the real App and
  a fake RC102, it types 250 g, 51% and the position, clicks everything that
  used to reset the mass, goes through the background and the salt, and
  checks the result on screen, the calibration, the history and what was
  saved on disk. With the previous version it fails at "mass 500 after Lite
  salt".

## 2.101.2
- Inglês sem meia tradução (pedido do ACE): a Fita do dia dizia "Today: dia
  tranquilo, all on the nível seguro". O tradutor pegava o primeiro molde que
  servia ("Hoje: {0}") e o resto saía meio traduzido. Agora, na tela e no Go,
  vale o molde com mais texto fixo, e um resultado que ainda tem português é
  recusado enquanto houver outro jeito (frases, pedaços, molde genérico).
- Teste novo (scripts/ingles_test.js, roda no CI): todo texto em português do
  código (5.076, frases fixas e moldes preenchidos com valores de exemplo)
  passa pelo tradutor, e o demo em inglês é aberto em todos os cenários
  (seguro, atenção, alarme, perigo, saída, saturado, desconectado, Cs-137…),
  em todas as páginas, sub-abas, detalhes, janelas e hover. Sobrou
  português: o CI falha.
- Depois de "rótulo:", o número sai no formato do inglês (0.12, não 0,12).
- Notas da atualização: em inglês, só a parte em inglês (antes vinham as
  duas, com o português primeiro).
- O nome que você dá (saída, medição, amostra) fica protegido antes da
  tradução: um "em" ou "de" dentro dele não é mais lido como parte da frase.
  "Marcado: …" do Fora de casa traduzido; "sem leitura" não ganha maiúscula
  no meio da frase (achados do QA).

**English**
- English without half translations (ACE's request): the Day strip said
  "Today: dia tranquilo, all on the nível seguro". The translator took the
  first template that fit ("Hoje: {0}") and the rest came out half
  translated. Now, on screen and in Go, the template with the most fixed text
  wins, and a result that still has Portuguese is rejected while there is
  another way (sentences, pieces, generic template).
- New test (scripts/ingles_test.js, runs in CI): every Portuguese text in the
  code (5,076, fixed sentences and templates filled with sample values) goes
  through the translator, and the English demo is opened in every scenario
  (safe, attention, alarm, danger, trip, saturated, disconnected, Cs-137…), on
  every page, sub-tab, details, dialog and hover. Any Portuguese left: CI
  fails.
- After "label:", the number comes out in the English format (0.12, not
  0,12).
- Update notes: in English, only the English part (before, both came, with
  the Portuguese first).
- The name you give (trip, measurement, sample) is protected before
  translation: an "em" or "de" inside it is no longer read as part of the
  sentence. "Marked: …" in Away from home translated; "no reading" no longer
  gets a capital letter mid-sentence (found by QA).

## 2.101.1
- Faixa dos aparelhos presa no topo (fora do Agora): atrás da bolha do
  aparelho aparecia um retângulo de outra cor, de cantos retos. A faixa agora
  usa o mesmo fundo da página (pedido do ACE).
- Inglês, varredura completa (pedido do ACE): a dica do espectro mostrava os
  isótopos em português ("actínio · série do tório"); as marcas da fita, a
  lista de uso do app ("Gráficos › tempo") e várias mensagens montadas com um
  valor no fim ("Backup salvo: …", "Conexão perdida: …", "Não consegui ler o
  log: …", erros da atualização, avisos do GMC-800) também. Agora um rótulo
  conhecido seguido de ": valor" e um texto entre parênteses são traduzidos em
  pedaços, na tela e no PDF.
- Relatório da medição: a anotação que você escreveu fica como está (antes,
  em inglês, saía meio traduzida).
- Medir objeto no RC102: a incerteza do µSv/h agora sai das médias de blocos
  de 30 s. O µSv/h do aparelho vem suavizado, e contar cada segundo como
  independente deixava a incerteza pequena demais; o "a dose subiu" podia
  aparecer à toa (achado do QA).
- Gráfico da dose: o eixo não mostra mais zeros inúteis ("60" em vez de
  "60,0"); em inglês, "no data for 6 days" no lugar de "6 days missing
  without data".
- Testes novos: todo PDF gerado em inglês nos testes e a bandeja em todos os
  estados não podem ter texto em português.

**English**
- Device strip pinned to the top (outside Now): a rectangle of a different
  color, with square corners, showed behind the device bubble. The strip now
  uses the same background as the page (ACE's request).
- English, full sweep (ACE's request): the spectrum tip showed the isotopes
  in Portuguese ("actínio · série do tório"); the strip marks, the app usage
  list ("Charts › tempo") and several messages built with a value at the end
  ("Backup saved: …", "Connection lost: …", "Couldn't read the log: …", update
  errors, GMC-800 notices) too. Now a known label followed by ": value" and a
  text in parentheses are translated piece by piece, on screen and in the PDF.
- Measurement report: the note you wrote stays as written (before, in
  English, it came out half translated).
- Measure object on the RC102: the µSv/h uncertainty now comes from the means
  of 30 s blocks. The device's µSv/h is smoothed, and counting each second as
  independent made the uncertainty too small; "the dose went up" could show
  up for nothing (found by QA).
- Dose chart: the axis no longer shows useless zeros ("60" instead of
  "60.0"); in English, "no data for 6 days" instead of "6 days missing
  without data".
- New tests: every PDF generated in English in the tests and the tray in every
  state must have no Portuguese text.

## 2.101.0
- Gráficos, dose por período (pedido do ACE): um trecho só para a página.
  Arrastar no gráfico de cima ou no da dose marca o mesmo trecho nos dois, e
  um "Sair do zoom" tira dos dois.
- Dose acumulada com "Do gráfico de cima" (padrão) ou "Tudo": do primeiro dado
  gravado até agora, os dois aparelhos, com o teto proporcional ao tempo
  medido. A escolha fica guardada; marcar um trecho volta para "Do gráfico de
  cima".
- O quadro do RC102 na dose tem um número só: a dose do período, com os
  buracos preenchidos pelo histórico do aparelho. Saem as linhas "Contador do
  aparelho" e "Pelo app"; se faltar mais de 5% do período, aparece quanto
  falta. O contador do aparelho continua na página do RC102, intocado.
- Gráfico de cima com o seletor "Todos | GMC-800 | RC102" (guardado), ao lado
  de µSv/h e CPM. Com um aparelho só, aparece só a linha dele, com o eixo
  ajustado a ele; no CPM fica um eixo só. Sem dado do RC102, o seletor some.
- Botão "Analisar nos gráficos" no gráfico de 24 h do RC102 e no quadro do
  GMC-800: abre os Gráficos já naquele aparelho e no mesmo período.
- Inglês: um texto já traduzido não é traduzido de novo (a legenda do CPM
  dizia "on the limits" em vez de "no limits").

**English**
- Charts, dose by period (ACE's request): one stretch for the whole page.
  Dragging on the chart above or on the dose chart marks the same stretch on
  both, and one "Exit zoom" clears both.
- Accumulated dose with "From the chart above" (default) or "All": from the
  first recorded data point until now, both devices, with the ceiling
  proportional to the measured time. The choice is remembered; marking a
  stretch goes back to "From the chart above".
- The RC102 tile in the dose has a single number: the period's dose, with
  gaps filled from the device's history. The "Device counter" and "By the app"
  lines are gone; if more than 5% of the period is missing, it says how much.
  The device counter stays on the RC102 page, untouched.
- The chart above has an "All | GMC-800 | RC102" selector (remembered), next
  to µSv/h and CPM. With a single device, only its line shows, with the axis
  fitted to it; in CPM there is a single axis. Without RC102 data, the
  selector goes away.
- "Analyze in the charts" button on the RC102 24 h chart and on the GMC-800
  card: opens Charts on that device and the same period.
- English: text that is already translated is not translated again (the CPM
  legend said "on the limits" instead of "no limits").

## 2.100.2
- Medir objeto: atenção e alarme dos dois aparelhos continuam em silêncio
  durante a medição, mas o Perigo (e a saturação) do aparelho que não está no
  objeto volta a valer: ali seria radiação de verdade. O aparelho no objeto
  segue calado por inteiro até o resultado.
- No Preparar, a opção "Perigo do outro aparelho durante a medição": toca
  (padrão) ou fica calado (pedido do ACE). A escolha fica guardada.
- O aviso na tela diz isso e fica até 1 min depois do resultado, o mesmo
  tempo em que os alertas continuam calados.
- Medir objeto com o RC102 e fundo da casa: o espectro de fundo passa a ser o
  dos espectros gravados em casa nos últimos 7 dias, sem os minutos de pedra
  ou fonte perto. O acumulado do aparelho juntava tudo o que ele já mediu
  (pedras inclusive) e ficava 2 a 3 vezes acima em todas as energias. O mesmo
  filtro vale para o "Parece" da busca e do espectrograma.
- Medir objeto com o RC102: quando a contagem não muda mas a taxa de dose
  sobe (gama de energia alta, como o K-40 do sal light), o resumo diz "a
  contagem não mudou, mas a dose subiu", com quantos σ a dose subiu e a linha
  que apareceu no espectro. O ± do µSv/h do RC102 passa a vir do espalhamento
  da própria dose, e não mais do √N das contagens (também no PDF).
- Faixa dos aparelhos: o "do aparelho" da dose de hoje do RC102 virou dica.
  Na janela estreita ele quebrava a linha e deixava o quadro com o dobro da
  altura do quadro do GMC-800.
- Página do RC102: os botões "Ajustes do aparelho" e "Conexão" saíram do
  quadro principal. Os dois continuam em Configurações.
- Inglês: textos que ainda apareciam em português foram traduzidos (a dica
  "nenhum pico conhecido aqui" do espectro e outros 80). A conferência das
  traduções agora pega mais frases em português.

**English**
- Measure object: attention and alarm on both devices stay silent during the
  measurement, but Danger (and saturation) on the device that is not on the
  object counts again, since that would be real radiation. The device on the
  object stays fully silent until the result.
- In Prepare, the option "Danger on the other device during the measurement":
  sounds (default) or stays silent (ACE's request). The choice is remembered.
- The on-screen notice says so and stays until 1 min after the result, the
  same time the alerts stay silent.
- Measure object with the RC102 and home background: the background spectrum
  is now the spectra recorded at home over the last 7 days, without the
  minutes of rocks or nearby sources. The device's accumulated spectrum held
  everything it ever measured (rocks included) and sat 2 to 3 times higher at
  every energy. The same filter applies to the "Looks like" in the search
  and in the spectrogram.
- Measure object with the RC102: when the count doesn't change but the dose
  rate goes up (high-energy gamma, like the K-40 in lite salt), the summary
  says "the count didn't change, but the dose went up", with how many σ the
  dose rose and the line that showed up in the spectrum. The ± of the RC102
  µSv/h now comes from the spread of the dose itself, no longer from the √N
  of the counts (also in the PDF).
- Device strip: the RC102's "from the device" next to today's dose became a
  tooltip. In a narrow window it broke the line and made the tile twice as
  tall as the GMC-800's.
- RC102 page: the "Device settings" and "Connection" buttons left the main
  tile. Both are still in Settings.
- English: texts still showing in Portuguese were translated (the spectrum's
  "no known peak here" tooltip and about 80 others). The translation check
  now catches more Portuguese phrases.

## 2.100.1
- Correção importante no Medir objeto: desde a 2.94.0 a tela não mandava
  para a medição o aparelho escolhido, o modo automático nem o fundo da casa.
  Toda medição rodava no GMC-800, com tempo fixo e medindo o fundo na hora,
  mesmo com o RC102 marcado. Agora vale o que está marcado. As medições
  salvas desde a 2.94.0 dizem o aparelho em que rodaram de verdade.
- Durante a medição, os alertas do app dos dois aparelhos (GMC-800 e RC102)
  ficam em silêncio até o resultado, não só os do aparelho no objeto. O aviso
  diz isso e mostra em qual aparelho a medição está rodando.
- Modo busca no Bluetooth mais leve: o espectro é lido a cada 5 s, e se uma
  leitura demorar mais que 2 s o app espaça as próximas (até 15 s) para não
  atrasar a taxa e os alarmes. Quando isso acontece, fica uma linha no
  registro dizendo quanto levou.
- O espectrograma da busca começa quando a aba abre. O trecho de antes agora
  diz "juntando desde HH:MM", em vez de ficar só vazio.
- "Parece": as linhas em ordem de energia, sem parênteses em sequência (ex.:
  "Rádio-226 (série do urânio) · 295, 352, 609 keV").
- Celular: "Janela" e "Cores" não se separam dos botões ao quebrar a linha; o
  título "Minha dose" do Agora fica numa linha só.
- Textos do normal e do máximo da busca em fonte de texto (números seguem
  iguais).

**English**

- Important Measure object fix: since 2.94.0 the screen didn't pass the
  chosen device, automatic mode or home background to the measurement. Every
  measurement ran on the GMC-800, with a fixed time and measuring the
  background on the spot, even with the RC102 selected. Now what is selected
  is used. Measurements saved since 2.94.0 show the device they really ran on.
- During a measurement, the app alerts of both devices (GMC-800 and RC102)
  stay silent until the result, not only those of the device on the object.
  The notice says so and shows which device the measurement runs on.
- Lighter search mode over Bluetooth: the spectrum is read every 5 s, and if
  a read takes more than 2 s the app spaces the next ones (up to 15 s) so the
  rate and alarms are not delayed. When that happens, a line in the log says
  how long it took.
- The search spectrogram starts when the tab opens. The part before that now
  says "collecting since HH:MM" instead of just being empty.
- "Looks like": lines in energy order, without back-to-back parentheses (e.g.
  "Radium-226 (uranium series) · 295, 352, 609 keV").
- Phone: "Window" and "Colors" stay with their buttons when the line wraps;
  the "My dose" title on Now stays on one line.
- The search's normal and max texts in the text font.

## 2.100.0
- Modo busca do RC102 com espectrograma ao vivo: embaixo da linha da taxa,
  no mesmo eixo de tempo, aparece de que energia chega o que chega (0 a
  1500 keV). Perto de uma fonte, as linhas dela acendem e o app escreve com o
  que se parece (ex.: "Parece: Rádio-226 (série do urânio)"); só fundo, não
  escreve nada.
- Janela de 1, 2 (padrão) ou 5 min, guardada. As cores são as mesmas da aba
  Espectrograma (original ou contínua), e trocar num lugar troca no outro.
- Com a aba Busca aberta, o espectro é lido a cada 2 s no USB e a cada 3,5 s
  no Bluetooth. Fechou a aba, volta ao normal. Nada a mais é gravado.

**English**

- RC102 search mode with a live spectrogram: below the rate line, on the same
  time axis, it shows at which energy things arrive (0 to 1500 keV). Near a
  source, its lines light up and the app says what it looks like (e.g. "Looks
  like: Radium-226 (uranium series)"); with only background, it says nothing.
- Window of 1, 2 (default) or 5 min, remembered. Colors are the same as the
  Spectrogram tab (original or continuous), and changing one changes the
  other.
- With the Search tab open, the spectrum is read every 2 s over USB and every
  3.5 s over Bluetooth. Close the tab and it goes back to normal. Nothing
  extra is saved.

## 2.99.1
- Medir objeto: a caixa "Ajustes da medição" saiu (no tempo fixo ela abria
  vazia, e no automático também, porque a opção de dentro nunca aparecia). O
  "Conta como tem algo" (+10%, +20%, +50% do fundo) agora fica logo abaixo
  do tempo e aparece no automático, que é onde ele vale.

**English**

- Measure object: the "Measurement settings" box is gone (with a fixed time
  it opened empty, and in automatic too, because the option inside never
  showed). "Counts as something there" (+10%, +20%, +50% of background) now
  sits right below the time and shows in automatic, which is where it
  applies.

## 2.99.0
- Medir objeto: antes de começar, a escolha fica à vista em "Quanto tempo
  medir": Automático (para quando tiver certeza, até 10 min, 30 min ou 1 h)
  ou Tempo fixo (de 30 s a 1 h, mede o tempo todo). Cada modo lembra o último
  tempo usado. Os Ajustes ficam só com quando conta como "tem algo".
- No celular, a faixa dos aparelhos presa no topo fica com uma linha por
  aparelho (nome, número e nível), ocupando metade da altura.
- Dose acumulada: o nome do aparelho é o botão para o teclado e o leitor de
  tela; clicar no único aparelho aceso não troca mais para o outro.
- Em inglês, a frase de alertas da Fita do dia e da semana sai traduzida
  inteira.

**English**

- Measure object: before starting, the choice is in plain sight under "How
  long to measure": Automatic (stops once it is sure, up to 10 min, 30 min or
  1 h) or Fixed time (30 s to 1 h, measures the whole time). Each mode
  remembers the last time used. Settings keep only when it counts as
  "something there".
- On the phone, the device strip pinned to the top shows one line per device
  (name, number and level), at half the height.
- Accumulated dose: the device name is the button for the keyboard and screen
  readers; clicking the only device that is on no longer switches to the
  other one.
- In English, the alerts sentence of the day and week tapes is fully
  translated.

## 2.98.1
- Medir objeto: o tempo fixo vai até 1 h (novas opções de 10 min, 30 min e
  1 h). Serve para fontes fracas, como sal light, que pedem meia hora ou
  mais para a diferença aparecer.
- Medições salvas: o "Como funciona: tempo, distância e blindagem" ganhou a
  própria linha e não fica mais por cima dos botões Relatório, Ver no
  gráfico, Editar e Apagar. Na lista, a data e a duração não quebram no meio.

**English**

- Measure object: the fixed time now goes up to 1 h (new 10 min, 30 min and
  1 h options). Useful for weak sources, like light salt, which need half an
  hour or more for the difference to show.
- Saved measurements: "How it works: time, distance and shielding" got its
  own line and no longer sits on top of the Report, See on the chart, Edit
  and Delete buttons. In the list, the date and duration no longer break in
  the middle.

## 2.98.0
- A faixa dos aparelhos virou a barra compacta: fica presa no topo ao rolar,
  com o nível e a dose de hoje de cada um. No Agora ela aparece quando os
  quadros grandes saem da tela; um clique volta para eles. A barra antiga
  ("voltar ao topo") saiu.
- Com os dois quadros abertos na faixa, eles ficam lado a lado, sem empilhar.
  Com um só aberto, a faixa rola junto para não cobrir a tela.
- Dose acumulada: clicar no quadro do aparelho volta a mostrar ou esconder a
  linha dele no gráfico (no lugar da caixa "mostrar no gráfico"). Escondido,
  o quadro fica apagado.

**English**

- The device strip became the compact bar: it stays pinned to the top while
  scrolling, with each device's level and today's dose. On Now it shows up
  once the big cards leave the screen; a click goes back to them. The old bar
  ("back to top") is gone.
- With both cards open on the strip, they sit side by side instead of
  stacking. With only one open, the strip scrolls along so it doesn't cover
  the screen.
- Accumulated dose: clicking a device's card shows or hides its line on the
  chart again (instead of the "show on chart" box). Hidden, the card is
  dimmed.

## 2.97.1
- Faixa dos aparelhos (fora do Agora): abrir um quadro não deixa mais um
  buraco embaixo do outro. O aberto ganha a linha inteira, e o fechado fica
  numa linha fina embaixo.
- A forma do nível voltou junto da palavra: no número grande da página de
  cada aparelho e na faixa fechada ("● Seguro").

**English**

- Device strip (outside Now): opening one card no longer leaves a hole under
  the other. The open one takes the whole row and the closed one sits in a
  thin row below.
- The level shape is back next to the word: on each device page's big
  number and on the closed strip ("● Safe").

## 2.97.0
- Formato da data à escolha, em Configurações › Janela e tela › Datas:
  16/11/2026 (dia, mês e ano) ou 2026-11-16 (ano, mês e dia). Vale para a
  tela e o relatório em PDF, nos dois idiomas. Em inglês o app não usa mais
  mês/dia em lugar nenhum.
- Página do RC102: a linha do σ saiu da fonte de código, como a do GMC-800.
- Configurações em tela estreita: as abas quebram linha, e a última ("Testes"
  no GMC-800) aparece sem rolar de lado.

**English**

- Choose the date format in Settings › Window and display › Dates:
  16/11/2026 (day, month, year) or 2026-11-16 (year, month, day). It applies
  to the screen and the PDF report, in both languages. In English the app no
  longer uses month/day anywhere.
- RC102 page: the σ line left the code font, like the GMC-800's.
- Settings on narrow screens: the tabs wrap, and the last one ("Tests" on the
  GMC-800) shows without sideways scrolling.

## 2.96.1
- "Os termos do app" entrou nas Ações rápidas (Ctrl+K).
- Glossário pelo teclado: o quadro recebe o foco, Tab chega em "Todos os
  termos" e Esc volta para o termo. Rolar a página não fecha mais o quadro.
- Alertas: o "?" do Como funciona não fica mais por cima do primeiro alerta.
- Níveis e alarmes em tela estreita: cada nível vira um bloco, com App,
  GMC-800 e RC102 um embaixo do outro, sem rolar de lado. O "Ligado" da dose
  do RC102 fica junto da caixa.
- Mais frases saíram da fonte de código (relógio, calibração e memória do
  GMC-800, data do alerta, linha do σ, total do período).
- Lista das Configurações: o RC102 aparece como "conectado · USB", igual ao
  GMC-800.
- Inglês: relógio "on time", download "… to download", "same as your
  normal" no Fora de casa e "click to open" no Medir objeto.

**English**

- "The app's terms" is now in Quick actions (Ctrl+K).
- Glossary by keyboard: the box takes focus, Tab reaches "All terms" and Esc
  goes back to the term. Scrolling no longer closes it.
- Alerts: the "How it works" "?" no longer sits on top of the first alert.
- Levels and alarms on narrow screens: each level becomes a block, with App,
  GMC-800 and RC102 stacked, no sideways scrolling. The RC102 dose "On" sits
  next to its box.
- More sentences left the code font (GMC-800 clock, calibration and memory,
  alert date, σ line, period total).
- Settings list: the RC102 shows "connected · USB", like the GMC-800.
- English: clock "on time", download "… to download", "same as your
  normal" in Away from home and "click to open" in Measure object.

## 2.96.0
- Cores com um emprego só: verde é "seguro" (e "tudo certo"), azul é o
  GMC-800, rosa continua o RC102, e o destaque e o botão principal ficam em
  branco. A chuva passou para um azul acinzentado.
- "Como funciona" num formato só: um "?" no canto de cima do cartão, que abre
  o texto flutuando (Esc fecha). As caixas "Por que isso importa" entraram
  nele.
- Termos técnicos clicáveis: σ, mediana, quartis, dispersão, dureza e outros
  abrem uma explicação curta. "Os termos do app" (Ctrl+K) mostra todos. A
  frase simples vem antes do número ("bem acima do nível de fundo (4,2 σ)").
- Texto um pouco maior (legendas e rótulos) e fonte de código só nos
  números, não nas frases (em teste, fácil de voltar).
- Acessibilidade: o leitor de tela ouve a última hora de cada aparelho e a
  conclusão do espectro; os gráficos do RC102 têm nome; o anel de foco da
  barra lateral não é mais cortado.
- "Baixar histórico": o resultado fica escrito ao lado do botão por 30 min,
  em vez de só num aviso que some.

**English**

- One job per colour: green is "safe" (and "all good"), blue is the GMC-800,
  pink stays the RC102, and highlights and the main button are white. Rain
  moved to a greyish blue.
- "How it works" in a single format: a "?" in the card's top corner that
  opens the text floating (Esc closes). The "Why it matters" boxes went into
  it.
- Clickable technical terms: σ, median, quartiles, dispersion, hardness and
  others open a short explanation. "The app's terms" (Ctrl+K) lists them
  all. The plain sentence comes before the number ("well above the
  background level (4.2 σ)").
- Slightly larger text (captions and labels) and code font only on numbers,
  not on sentences (being tested, easy to undo).
- Accessibility: screen readers hear each device's last hour and the
  spectrum's conclusion; the RC102 charts have names; the sidebar focus ring
  is no longer cut off.
- "Download history": the result stays written next to the button for 30
  min, instead of only in a notice that goes away.

## 2.95.0
- Níveis e alarmes virou uma tela só: o app, o GMC-800 e o RC102 lado a
  lado, cada nível com a forma, o nome e o valor. Os valores dos aparelhos
  dá para mudar ali mesmo e vão com "Gravar no GMC-800" e "Gravar no
  RC102", como antes. Os alarmes de dose do RC102 também estão ali.
- RC102: os alarmes de dose agora têm o interruptor "ligado" (antes era uma
  caixa "desligado", que ficava marcada com o alarme desligado).
- Calibração (em teste, fácil de voltar): na frente só o fator em uso, de
  onde veio e o botão de intercalibrar. Fator à mão, versões, lado a lado
  com o RC102 e pontos do contador ficam em "Avançado". Antes de gravar no
  contador, a confirmação mostra o que muda, ponto por ponto.
- Janela e tela: cartões na altura do conteúdo, tamanho num controle
  deslizante, menos texto (o resto em "Como funciona"). A chuva ganhou
  seção própria: Configurações › Chuva no gráfico.
- Botões em duas alturas e uma caixa de marcar só no app todo.
- Sobras da sincronização tiradas dos textos da demonstração e do registro.

**English**

- Levels and alarms is now a single screen: the app, the GMC-800 and the
  RC102 side by side, each level with its shape, name and value. The
  devices' values can be changed right there and go with "Write to the
  GMC-800" and "Write to the RC102", as before. The RC102 dose alarms are
  there too.
- RC102: dose alarms now have an "on" switch (it used to be an "off" box
  that was ticked when the alarm was off).
- Calibration (being tested, easy to undo): up front only the factor in use,
  where it came from and the cross-calibrate button. Factor by hand,
  versions, side by side with the RC102 and the counter's points are under
  "Advanced". Before writing to the counter, the confirmation shows what
  changes, point by point.
- Window and display: cards as tall as their content, size on a slider, less
  text (the rest under "How it works"). Rain got its own section: Settings ›
  Rain on the chart.
- Buttons in two heights and one checkbox style across the app.
- Leftovers from sync removed from the demo and log texts.

## 2.94.0
- Medir objeto: "tem algo" agora usa a cor do Medir objeto (turquesa); o
  vermelho fica só quando a leitura com o objeto passa do nível de perigo. O
  resultado diz a certeza em palavras ("Certeza: muito alta") e compara a
  leitura com os níveis do app ("0,31 µSv/h com o objeto, abaixo do nível de
  atenção"). O σ fica nos detalhes.
- Passo 1 mais limpo: "(opcional)" separado, tamanho "pequeno" já marcado,
  textos dos aparelhos sem fonte de código e as etapas como marcação, não
  como botões. As quatro opções de aparelho continuam.
- Distância e Blindagem saíram das abas: aparecem no fim da medição ("Quer
  ver na prática?") e têm um link para voltar.
- Fora de casa: a lista vem primeiro, com a dose de cada saída em destaque;
  a linha do tempo fica recolhida. Três palavras para o lugar: em casa,
  comigo e ignorar ("fora do cálculo" virou "ignorado").

**English**

- Measure object: "something there" now uses the Measure object colour
  (teal); red only when the reading with the object goes over the danger
  level. The result states the certainty in words ("Certainty: very high")
  and compares the reading with the app's levels ("0.31 µSv/h with the
  object, below the attention level"). σ stays in the details.
- Cleaner step 1: "(optional)" set apart, size "small" already selected,
  device texts without code font, and the steps as markers, not buttons.
  The four device options stay.
- Distance and Shielding left the tabs: they show up at the end of a
  measurement ("Want to see it in practice?") and have a link back.
- Away from home: the list comes first, with each trip's dose up front; the
  timeline is folded. Three words for the place: at home, with me and
  ignore ("out of the calculation" became "ignored").

## 2.93.0
- Espectro: o gráfico cresce com a janela e tem botão de tela cheia (Esc
  sai). A conclusão aparece em destaque acima do gráfico ("Parece: …", com
  os três mais prováveis e "+N"); a lista completa dos isótopos fica
  embaixo. Os controles ficam numa linha só, a escala acompanha a curva mais
  alta (inclusive a do fundo) e as marcas do eixo são redondas.
- Espectro direto na barra lateral (em teste, fácil de voltar), quando o
  RC102 já foi conectado. Dentro dele, um seletor só: Tela do aparelho,
  Acumulado do aparelho e Ao longo do tempo.
- Log do RC102: os filtros ganharam nome ao lado do ícone, e os valores
  ficam em cor neutra.
- Atividade (Bq): saiu a nota de projeto do rodapé; "talvez só 'abaixo de'"
  virou "pode ficar abaixo do que o RC102 consegue medir"; o título "O que
  vai medir" sem caixa alta.
- Modo busca: o ponteiro e a linha têm a cor do nível agora (verde quando
  seguro). Espectrograma: escala de cor contínua como opção ao lado da
  original.

**English**

- Spectrum: the chart grows with the window and has a full screen button
  (Esc leaves). The conclusion shows up front above the chart ("Looks like:
  …", with the three most likely and "+N"); the full isotope list stays
  below. Controls fit in one row, the scale follows the highest curve
  (background included) and the axis ticks are round.
- Spectrum straight in the sidebar (on trial, easy to undo), once the RC102
  has been connected. Inside it, a single selector: Device screen, Device's
  accumulated and Over time.
- RC102 log: the filters got a name next to the icon, and values are in a
  neutral colour.
- Activity (Bq): the project note in the footer is gone; "maybe only
  'below'" became "may stay below what the RC102 can measure"; the "What
  you're measuring" title is no longer upper case.
- Search mode: the needle and the line take the current level colour (green
  when safe). Spectrogram: a continuous colour scale as an option next to
  the original.

## 2.92.0
- Gráficos: as marcas do eixo saem em passos redondos (1,5 / 1,0 / 0,5) e
  a unidade fica escrita embaixo do eixo.
- Gráficos › Dose (em teste, fácil de voltar): o quadro de cada aparelho diz
  só "dentro do teto"; a conta fica em "a conta". Mostrar ou esconder a
  linha virou uma caixa "mostrar no gráfico". A legenda tira "chuva:
  desligada" e encurta "medição".
- Resumo: "Sua dose" usa o mesmo total do "De onde vem a sua dose", com a
  conta à vista (fundo + acima do fundo). O pico inclui os alertas do
  período e diz quando foi durante um. Com alerta no período, a frase da
  casa diz "Fora dos alertas, a casa ficou…".
- Palavras: "aviso" sai. Alertas marcados "do contador" agora dizem "achado
  depois"; o filtro ganhou o rótulo "Mostrar"; nos limites do RC102 aparece
  "alarme 1 (atenção)" e "alarme 2 (alarme)"; a linha do pico diz "bem acima
  do fundo" em vez do σ. Configurações › "Alarmes e avisos" virou "Níveis e
  alarmes".

**English**

- Charts: axis ticks now use round steps (1.5 / 1.0 / 0.5) and the unit is
  written under the axis.
- Charts › Dose (on trial, easy to undo): each device box only says "within
  the ceiling"; the math is under "the math". Showing or hiding the line is
  now a "show on chart" checkbox. The legend drops "rain: off" and shortens
  "measurement".
- Summary: "Your dose" uses the same total as "Where your dose comes from",
  with the math in view (background + above background). The peak includes
  the period's alerts and says when it happened during one. With an alert
  in the period, the home sentence says "Outside the alerts, home was…".
- Words: "warning" is gone. Alerts tagged "from the counter" now say "found
  later"; the filter got a "Show" label; RC102 limits show "alarm 1
  (attention)" and "alarm 2 (alarm)"; the peak line says "well above
  background" instead of σ. Settings › "Alarms and warnings" became "Levels
  and alarms".

## 2.91.0
- Página nova "Fundo de casa" na barra lateral. No topo, uma frase diz como
  está o fundo da casa, e quatro quadros dão a resposta de cada recorte:
  últimas semanas, hora do dia, dia da semana e ano. Clicar num quadro abre
  o gráfico daquele recorte embaixo. Gráficos fica só com a leitura e a dose.
- Últimas semanas: um veredito só. A outra conta (a reta dos últimos dias) e
  a tabela "Fundo por dia" ficam recolhidas embaixo, com o nome de cada uma.
- Dia da semana e mês: cada barra mostra a diferença para a média, em %,
  saindo do meio. Os cartões dia × noite ganharam uma casa decimal, para
  baterem com a frase.

**English**

- New "Home background" page in the sidebar. At the top, one sentence says
  how the home background is doing, and four boxes give the answer for each
  cut: last weeks, hour of day, day of week and year. Clicking a box opens
  that chart below. Charts keeps only the reading and the dose.
- Last weeks: a single verdict. The other calculation (the line over the
  last days) and the "Background by day" table are folded below, each with
  its name.
- Day of week and month: each bar shows the difference from the average, in
  %, starting from the middle. The day × night cards got one more decimal
  place, to match the sentence.

## 2.90.0
- Agora, Minha dose: um número só (hoje), uma frase comparando com o fundo
  da casa e uma linha com semana, mês e ano, tudo em µSv. A barra do ano
  diz quanto já foi de um teto, na mesma unidade. Os quatro cartões de antes
  ficam em "por período".
- O lugar de cada aparelho (em casa, comigo) virou um selo no quadro dele.
  O cartão "Onde estão" só aparece quando tem algo a dizer ou quando você
  abre pelo selo.
- A pergunta "O RC102 ficou sem conexão" pode ser fechada com o ×; ela
  continua em Fora de casa como "a responder".
- Fita do dia: a legenda só mostra o que aparece na fita. Trechos de 30 min
  ou mais sem leitura ganham uma linha escrita embaixo. "Avisos" virou
  "alertas", como no resto do app.

**English**

- Now, My dose: a single number (today), one sentence comparing it with the
  home background, and one line with week, month and year, all in µSv. The
  year bar says how much of a ceiling has gone, in the same unit. The four
  cards from before are under "by period".
- Where each device is (at home, with me) became a badge on its card. The
  "Where they are" card only shows up when it has something to say or when
  you open it from the badge.
- The "The RC102 was disconnected" question can be closed with ×; it stays
  in Away from home as "to answer".
- Day strip: the legend only shows what appears on the strip. Stretches of
  30 min or more with no reading get a written line below. "Warnings"
  became "alerts", as in the rest of the app.

## 2.89.0
- Barra lateral: a busca saiu. Ctrl+K abre as Ações rápidas, que agora
  têm o que se faz no dia a dia: anotar, medir um objeto, "Estou testando
  um objeto", saí de casa, programar saída, relatório em PDF, baixar
  histórico e mudei o aparelho de lugar. A busca de ajustes continua em
  Configurações.
- "Estou testando um objeto": cala atenção e alarme do aparelho escolhido
  por 10 min, sem abrir o Medir objeto. O perigo continua tocando. Um aviso
  embaixo dos quadros mostra até quando e tem o botão "Parar".
- Demonstração foi para Configurações › Atualizações e sobre. "Baixar
  histórico" saiu do pé da barra e fica no aviso de memória e nas Ações.
  O ponto laranja do GQ GMC-800 diz o que é ao passar o mouse.
- Clima só aparece na barra quando o clima está ligado.
- No alarme e no perigo, o Agora esconde a frase que repete a faixa e as
  perguntas pendentes; elas voltam quando o alarme passar.
- Em janela baixa, uma seta no pé da lista avisa que há mais itens.

**English**

- Sidebar: search is gone. Ctrl+K opens Quick actions, which now have the
  everyday tasks: note, measure an object, "I'm testing an object", left
  home, plan a trip, PDF report, download history and moved the device.
  The settings search stays in Settings.
- "I'm testing an object": silences attention and alarm on the chosen
  device for 10 min, without opening Measure object. Danger still sounds.
  A notice under the cards shows until when and has a "Stop" button.
- Demo moved to Settings › Updates and about. "Download history" left the
  bottom of the sidebar; it lives in the memory notice and in Quick actions.
  The orange dot on GQ GMC-800 says what it means on hover.
- Weather only shows in the sidebar when weather is turned on.
- During an alarm or danger, Now hides the sentence that repeats the band
  and the pending questions; they come back when the alarm ends.
- In a short window, an arrow at the bottom of the list shows there are
  more items.

## 2.88.0
- Fora do Agora, os quadros dos aparelhos viram uma faixa de uma linha com
  nome, número, nível e a dose de hoje. Um clique abre o quadro inteiro ali
  mesmo; "fechar" volta para a faixa. Gráficos, Espectro e as outras telas
  ganham quase 200 px de altura.
- Página de cada aparelho: na aba Agora o cartão grande é o do próprio
  aparelho; nas outras abas ele vem primeiro na faixa. O mostrador redondo
  do cartão deu lugar a uma régua com atenção, alarme e perigo marcados e
  quanto falta para o próximo limite. O trifólio continua no Agora.
- Os dois aparelhos usam os mesmos nomes de nível (atenção e alarme também
  no RC102, que usa os alarmes 1 e 2 dele) e o número no mesmo tamanho.
- "detalhes" do Agora virou uma tabela só, embaixo dos dois quadros, sem
  deixar buraco. A linha do σ do cartão grande foi para "detalhes".

**English**

- Outside Now, the device cards become a one-line strip with name, number,
  level and today's dose. One click opens the full card right there;
  "close" goes back to the strip. Charts, Spectrum and the other screens
  gain almost 200 px of height.
- Each device page: on the Now tab the big card is the device's own; on
  the other tabs it comes first in the strip. The round dial on the card
  gave way to a ruler with attention, alarm and danger marked and how far
  it is to the next limit. The trefoil stays on Now.
- Both devices use the same level names (attention and alarm on the RC102
  too, which uses its alarms 1 and 2) and the number in the same size.
- "details" on Now became a single table under both cards, with no empty
  space. The σ line on the big card moved to "details".

## 2.87.2
- Ações rápidas: o menu abre num painel com fundo, em vez de bolhas soltas.
  Os itens da barra lateral não aparecem mais entre elas, e o texto fica
  fácil de ler.

**English**

- Quick actions: the menu opens in a panel with a background instead of
  loose bubbles. The sidebar items no longer show between them, and the
  text is easy to read.

## 2.87.1
- Agora: "detalhes" abre e fecha nos dois quadros juntos (GMC-800 e RC102).
  Abrir só um deixava o outro quadro esticado, com um espaço vazio.

**English**

- Now: "details" opens and closes on both cards together (GMC-800 and
  RC102). Opening just one left the other card stretched with an empty
  space.

## 2.87.0
- Agora: o trifólio voltou para o quadro do GMC-800 e do RC102, à esquerda
  do número grande e do tamanho de antes. No meio dele fica a forma do
  nível (círculo, triângulo, losango, octógono) na cor do nível, e o anel
  em volta pisca a cada contagem. A palavra do nível continua em cima do
  gráfico, sem o pontinho que piscava ao lado dela.
- "detalhes" fica só com CPM, CPS, a outra unidade de taxa e a fita dos
  últimos 10 segundos.

**English**

- Now: the trefoil is back on the GMC-800 and RC102 cards, left of the big
  number and at its old size. Inside it sits the level shape (circle,
  triangle, diamond, octagon) in the level color, and the ring around it
  blinks on every count. The level word stays above the chart, without the
  blinking dot next to it.
- "details" keeps only CPM, CPS, the other rate unit and the last 10
  seconds strip.

## 2.86.1
- Atividade (Bq) em inglês: o valor típico da castanha, da areia e do
  granito aparece traduzido; o preparo da pedra solta não sai mais com uma
  frase trocada; o nome e a posição na linha da medida também são
  traduzidos.
- Na medida de uma pedra solta, a linha de cima diz "pedra solta" em vez da
  posição do pote.
- "Guardar como minha" a partir de um cartão da lista grava o nome no
  idioma da tela.

**English**

- Activity (Bq) in English: the typical value for Brazil nuts, sand and
  granite is translated; the loose-stone preparation no longer comes out
  with a garbled sentence; the name and position in the measurement line
  are translated too.
- When measuring a loose stone, the top line says "loose stone" instead of
  the jar position.
- "Save as mine" from a list card saves the name in the screen's language.

## 2.86.0
- Atividade (Bq) do RC102 repaginada, na ordem em que as coisas acontecem.
  No topo, uma linha diz se falta calibrar ou com o que está calibrado; a
  tabela de eficiência foi para "detalhes".
- Calibrar virou um passo guiado e separado: KCl (sal light ou KCl puro)
  primeiro e o tório, opcional, depois. O tempo total aparece antes de
  começar.
- O que medir é escolhido por cartões, com o valor típico, o tempo
  recomendado (já marcado) e o preparo de cada um. Nome, posição, fundo e
  radionuclídeos foram para "Mais opções", e o tempo total fica ao lado do
  Começar.
- "Suas amostras": cartões guardados por você, começando com uraninita,
  uranocircita e euxenita. A referência de cada uma vem da sua última
  medição de objeto com o mesmo nome. Qualquer medida pode virar "minha".
- Pedra solta: a massa é opcional e o resultado sai em Bq da peça (e em
  Bq/g, se tiver a massa), com o aviso de que a forma é diferente da do
  pote da calibração.
- Durante a medida, os passos (fundo, colocar a amostra, medindo,
  resultado). No resultado, a comparação com o valor típico quando a
  amostra tem uma faixa conhecida, "Medir de novo" e as linhas usadas
  recolhidas. O texto dos limites foi para o "Como funciona".

**English**

- The RC102 Activity (Bq) tab was redone, in the order things happen. At
  the top, one line says whether calibration is missing or what it was
  calibrated with; the efficiency table moved to "details".
- Calibrating is now a separate guided step: KCl (lite salt or pure KCl)
  first, then the optional thorium. The total time shows before starting.
- What to measure is picked with cards, each with its typical value,
  recommended time (already selected) and preparation. Name, position,
  background and radionuclides moved to "More options", and the total time
  sits next to Start.
- "Your samples": cards you save, starting with uraninite, uranocircite
  and euxenite. Each one's reference comes from your latest object
  measurement with the same name. Any measurement can become "mine".
- Loose stone: the mass is optional and the result is in Bq for the piece
  (and Bq/g if the mass is given), with a note that the shape differs from
  the calibration jar.
- While measuring, the steps (background, place the sample, measuring,
  result). The result compares with the typical value when the sample has
  a known range, has "Measure again", and keeps the lines used collapsed.
  The limits text moved to "How it works".

## 2.85.1
- Unidades: rótulos que ainda diziam µSv/h com a tela em mrem/h agora
  seguem a unidade escolhida (a dica embaixo da régua do GMC-800 e do
  RC102, as legendas do Resumo, do detalhe do alerta e da tendência do
  fundo, a frase dos alarmes dos aparelhos e a da bandeja). A tabela, o
  eixo e o resultado dos testes de distância e de blindagem também.
- Na página do GMC-800 em mrem/h, a linha embaixo do número volta a mostrar
  o µSv/h (estava sendo convertida para mrem/h de novo).

**English**

- Units: labels that still said µSv/h with the screen in mrem/h now follow
  the chosen unit (the hint under the GMC-800 and RC102 scale, the legends
  of the Summary, the alert detail and the background trend, the device
  alarms sentence and the tray one). So do the table, axis and result of
  the distance and shielding tests.
- On the GMC-800 page in mrem/h, the line under the number shows µSv/h
  again (it was being converted to mrem/h twice).

## 2.85.0
- Ações rápidas saíram do botão redondo do canto e foram para o pé da barra
  lateral ("＋ Ações rápidas", junto de Baixar histórico e Configurações).
  O menu abre para cima e some nas mesmas horas de antes (medição em
  andamento, alarme tocando).
- Fora de casa: cada saída mostra "Resumo da saída" e "Ver no gráfico";
  Relatório (PDF), Editar e Apagar ficam no "⋯". Os itens "a responder"
  ganharam título ("RC102 sem conexão", "GMC-800 sem conexão", "Os dois sem
  conexão").
- A tela ALARA saiu e as partes foram para onde fazem sentido: "De onde vem
  a sua dose" virou um quadro do Resumo, no período escolhido lá (dia,
  semana ou mês); os testes de distância e de blindagem viraram abas do
  Medir objeto (Objeto · Distância · Blindagem); o resultado de cada
  medição mostra quanto custa ficar perto ("ficar 30 min perto = X acima do
  fundo"); e o "Como funciona" do Medir objeto explica os três jeitos.
- Clima desligado: uma linha do que a tela mostra e o botão "Ligar". O
  efeito da chuva no GMC-800 sai na unidade da tela, como no RC102.
- Aviso de memória do GMC-800 curto: título, uma linha e o botão.
- Configurações › Dados e backup em abas: Backup e arquivos · Nuvem · Uso
  do app · Histórico.
- A sincronização pela rede local entre PCs (pareamento por código) saiu do
  app: a nuvem substitui. PCs que estavam pareados param de trocar dados por
  ela; quem usa a nuvem não muda nada. O arquivo sincronizacao.json pode ser
  apagado da pasta de dados.

**English**

- Quick actions left the round corner button and moved to the foot of the
  sidebar ("＋ Quick actions", next to Download history and Settings). The
  menu opens upwards and hides at the same times as before (measurement in
  progress, alarm ringing).
- Away from home: each trip shows "Trip summary" and "See on the chart";
  Report (PDF), Edit and Delete are in the "⋯". The "to answer" items got a
  title ("RC102 disconnected", "GMC-800 disconnected", "Both
  disconnected").
- The ALARA screen is gone and its parts moved where they make sense:
  "Where your dose comes from" is now a Summary tile, for the period chosen
  there (day, week or month); the distance and shielding tests became tabs
  of Measure object (Object · Distance · Shielding); each measurement's
  result shows how much staying close costs ("stay close 30 min = X above
  background"); and Measure object's "How it works" explains the three
  ways.
- Weather off: one line about what the screen shows and a "Turn on"
  button. The rain effect on the GMC-800 uses the screen unit, like the
  RC102.
- Short GMC-800 memory notice: title, one line and the button.
- Settings › Data and backup in tabs: Backup and files · Cloud · App usage
  · History.
- Sync over the local network between PCs (pairing by code) was removed:
  the cloud replaces it. PCs that were paired stop exchanging data through
  it; nothing changes for those using the cloud. The sincronizacao.json
  file can be deleted from the data folder.

## 2.84.0
- Unidades da tela para o app todo, em Configurações › Janela e tela ›
  Unidades: taxa em µSv/h ou mrem/h e contagem em CPM ou CPS (dá para
  misturar, por exemplo µSv/h com CPS). A dose acompanha a taxa (µSv ou
  mrem), inclusive nos alarmes e no relatório. O RC102 segue a mesma
  escolha. Os botões de unidade da página do GQ GMC-800 saíram.
- A calibração, o tubo e as partes técnicas continuam em CPM, que é o que
  o tubo mede. Os gráficos dentro do PDF continuam em µSv/h; o texto do
  relatório sai na unidade escolhida.
- Alarme do GMC-800 nas configurações do aparelho: você digita na unidade
  da tela e o app grava em CPM, pela calibração do app ("grava N CPM no
  aparelho"). Se o aparelho estiver com o alarme em µSv/h ou mR/h no menu
  dele, esse campo ainda não é lido; fica para uma próxima versão.
- Quadro do Agora: o número grande na unidade escolhida e, embaixo, uma
  linha com a contagem e a dose de hoje. O mostrador com o pulso foi para
  "detalhes" (fechado por padrão, o app lembra), com CPM, CPS, a outra
  unidade de taxa e a fita dos últimos 10 segundos. O ícone do nível pisca
  a cada contagem.
- Pergunta "a responder" do Agora numa linha só: frase, dica, os dois
  botões mais prováveis e "Ver detalhes" para abrir o cartão inteiro.
- "Onde está cada aparelho" virou uma linha ("GMC-800 em casa · RC102 com
  você · mudar"); "mudar" abre os seletores. Os cartões "Casa pelo GMC-800"
  e "Sua dose pelo RC102" saíram; só aparece aviso quando algo fica em
  branco ou satura. RC102 longe do PC mostra "continua somando a dose
  sozinho" embaixo dele.
- "Sem dado" neutro: a barra fica cinza e o texto diz "RC102 desligado em
  X% do dia".
- Fonte mono só nos números; rótulos, botões e etiquetas na fonte normal.

**English**

- Screen units for the whole app, in Settings › Window and display ›
  Units: rate in µSv/h or mrem/h and count in CPM or CPS (they can be
  mixed, for example µSv/h with CPS). Dose follows the rate (µSv or mrem),
  including alarms and the report. The RC102 follows the same choice. The
  unit buttons on the GQ GMC-800 page are gone.
- Calibration, the tube and the technical parts stay in CPM, which is what
  the tube measures. Charts inside the PDF stay in µSv/h; the report text
  uses the chosen unit.
- GMC-800 alarm in the device settings: you type it in the screen unit and
  the app writes it in CPM, using the app's calibration ("writes N CPM to
  the device"). If the device has its alarm set to µSv/h or mR/h in its own
  menu, that field isn't read yet; that's for a later version.
- Now tile: the big number in the chosen unit and, below it, one line with
  the count and today's dose. The dial with the pulse moved into "details"
  (closed by default, the app remembers), with CPM, CPS, the other rate
  unit and the last-10-seconds strip. The level icon blinks with each
  count.
- The "to answer" question on Now fits on one line: sentence, hint, the
  two likeliest buttons and "See details" to open the full card.
- "Where each device is" became one line ("GMC-800 at home · RC102 with
  you · change"); "change" opens the selectors. The "Home by the GMC-800"
  and "Your dose by the RC102" cards are gone; a notice only shows when
  something goes blank or saturates. An RC102 away from the PC shows
  "keeps adding up the dose on its own" under it.
- Neutral "no data": the bar turns gray and the text says "RC102 off for
  X% of the day".
- Mono font only for numbers; labels, buttons and tags use the regular
  font.

## 2.83.10
- Abertura mais leve: a tabela de tradução para o inglês (~700 KB) só
  carrega com o app em inglês, e os gráficos da aba Gráficos (tempo, dose,
  hora do dia, últimos dias) só são buscados e desenhados com a sub-aba
  aberta, em vez de todos na abertura e a cada 5 minutos.
- O log guarda o tempo de abertura da tela ("abertura: tela pronta em …"),
  para comparar antes e depois.
- Fita do dia: em tela estreita, a faixa do papel usa nomes curtos ("casa",
  "você", "fora") e mostra o nome inteiro ao passar o mouse.
- Teste novo que trava a lista de funções que o app do celular usa, para
  uma mudança no PC não quebrar o celular sem aviso.

**English**

- Lighter startup: the English translation table (~700 KB) only loads with
  the app in English, and the charts on the Charts tab (time, dose, hour of
  day, recent days) are only fetched and drawn with their sub-tab open,
  instead of all of them at startup and every 5 minutes.
- The log records how long the screen took to open ("abertura: tela pronta
  em …"), to compare before and after.
- Day strip: on a narrow screen, the role band uses short names ("home",
  "you", "out") and shows the full name on hover.
- New test that locks the list of functions the phone app uses, so a change
  on the PC can't break the phone without notice.

## 2.83.9
- Fita do dia: as linhas "com você" e "casa" viraram uma linha por
  aparelho, "GQ GMC-800" em cima e "Radiacode 102" embaixo, com o papel de
  cada um embaixo do nome. Se o papel mudou no dia, uma faixinha embaixo
  da linha mostra quando (com "fora do cálculo" listrado). A dica do mouse
  mostra os dois aparelhos e o papel de cada um.

**English**

- Day strip: the "with you" and "home" rows became one row per device,
  "GQ GMC-800" on top and "Radiacode 102" below, with each one's role under
  the name. If the role changed during the day, a thin band under the row
  shows when (with "out of the calculation" striped). The tooltip shows
  both devices and each one's role.

## 2.83.8
- Gráficos › Hora do dia e Padrões do fundo: na 2.83.7, o µSv/h do
  GMC-800 usava a calibração de hoje para todos os dias. Agora cada minuto
  usa a calibração que valia nele, como no resto do app. Vale também para
  o "fora de casa" e o pico dos alertas na caixinha de cada hora.
- Tendência do fundo: a caixinha de cada dia não mistura mais CPM com
  µSv/h (a dispersão é uma razão, sem unidade).

**English**

- Charts › Time of day and Background patterns: in 2.83.7 the GMC-800
  µSv/h used today's calibration for every day. Now each minute uses the
  calibration in force at that time, like the rest of the app. This also
  applies to "away from home" and the alert peak in each hour's box.
- Background trend: each day's box no longer mixes CPM with µSv/h (the
  dispersion is a ratio, with no unit).

## 2.83.7
- Gráficos › Hora do dia: o padrão por hora do GMC-800 aparecia em CPM
  (subtítulo, escala e caixinha de cada hora). Agora fica em µSv/h, pela
  calibração atual, como no resto do app.
- Gráficos › Padrões do fundo: o GMC-800 também passa para µSv/h (horas,
  dia × noite, dias da semana e meses). Na Tendência do fundo, a mediana
  da caixinha de cada dia também.
- Barra lateral: um degradê no pé da lista mostra que há mais páginas
  embaixo quando a janela é baixa.
- Ctrl+K: "Exportar leituras" e "Eventos e log" aparecem uma vez só (em
  Ações).

**English**

- Charts › Time of day: the GMC-800 hourly pattern showed CPM (subtitle,
  scale and the box for each hour). It now shows µSv/h, with the current
  calibration, like the rest of the app.
- Charts › Background patterns: the GMC-800 also moves to µSv/h (hours, day
  vs. night, weekdays and months). In Background trend, the median in each
  day's box too.
- Sidebar: a fade at the bottom of the list shows there are more pages
  below when the window is short.
- Ctrl+K: "Export readings" and "Events and log" show up only once (under
  Actions).

## 2.83.6
- Barra lateral: com a janela baixa, a barra inteira rolava e a logo
  sumia. Agora só a lista de páginas rola; a logo, a nuvem e a busca ficam
  presas em cima, e Baixar histórico, Demonstração e Configurações ficam
  presas embaixo. Uma linha fina embaixo da busca mostra que a lista está
  rolada.
- A linha da nuvem subiu para logo abaixo da logo, com o ícone de nuvem.
- "Eventos" e "Exportar CSV" saíram da barra: agora são os cartões "Eventos
  e log" (Configurações › Atualizações e sobre) e "Exportar leituras"
  (Configurações › Dados e backup). Os dois continuam no Ctrl+K.

**English**

- Sidebar: in a short window the whole sidebar scrolled and the logo went
  away. Now only the page list scrolls; the logo, cloud and search stay
  pinned at the top, and Download history, Demo and Settings stay pinned at
  the bottom. A thin line under the search shows the list is scrolled.
- The cloud line moved right below the logo, with a cloud icon.
- "Events" and "Export CSV" left the sidebar: they are now the "Events and
  log" card (Settings › Updates and about) and the "Export readings" card
  (Settings › Data and backup). Both are still in Ctrl+K.

## 2.83.5
- Linux: ao atualizar pelo app, o AppImage novo passa a se chamar sempre
  "radlog" na pasta das versões (o anterior vira radlog.old e é apagado
  depois), como o RadLog.exe no Windows. Assim um atalho no menu ou na
  área de trabalho continua valendo depois de cada atualização.

**English**

- Linux: when updating from the app, the new AppImage is now always named
  "radlog" in the versions folder (the previous one becomes radlog.old and
  is deleted later), like RadLog.exe on Windows. A shortcut in the menu or
  on the desktop keeps working after every update.

## 2.83.4
- Linux: os títulos dos quadros (Configurações e outros) apareciam em cima
  da borda. Agora ficam dentro do quadro, como no Windows.
- Linux: a rolagem e a tela em geral estavam lentas porque o app abria
  sem aceleração de vídeo (tudo desenhado pelo processador). Agora a
  aceleração é usada quando a página pede. Se a janela ficar em branco em
  alguma placa de vídeo, abrir com RADLOG_GPU=0 volta ao modo anterior.

**English**

- Linux: box titles (Settings and others) showed on top of the border. They
  now sit inside the box, as on Windows.
- Linux: scrolling and the screen in general were slow because the app ran
  without graphics acceleration (everything drawn by the processor). Now
  acceleration is used when the page asks for it. If the window turns blank
  on some graphics card, starting it with RADLOG_GPU=0 goes back to the old
  mode.

## 2.83.3
- Os botões da nuvem ficam desligados enquanto a ação anda: um segundo
  clique não chega mais ao app antes da resposta do primeiro.

**English**

- Cloud buttons stay disabled while the action runs: a second click no
  longer reaches the app before the first one answers.

## 2.83.2
- Correção: clicar duas vezes seguidas em "Recomeçar a nuvem" criava duas
  nuvens com o mesmo código. O PC ficava numa e os outros aparelhos
  entravam na outra, e os dados deles não apareciam no PC. Agora o segundo
  clique é ignorado enquanto o recomeço anda, e também depois que a nuvem
  já recomeçou.

**English**

- Fix: clicking "Restart the cloud" twice in a row created two clouds with
  the same code. The PC stayed in one and the other devices joined the
  other, so their data didn't show up on the PC. Now the second click is
  ignored while the restart runs, and also once the cloud has already
  restarted.

## 2.83.1
- Com a demonstração ligada, a faixa azul avisa o que acontece de verdade
  e não é alarme: o GMC-800 desconectado, saturado ou sem contagens, e a
  memória dele passando de 80% ou 95%.
- Na demonstração, gravar a configuração do GMC-800, acertar o relógio,
  ligar ou desligar o RC102 e zerar o espectro avisam que isso não muda o
  aparelho de verdade.

**English**

- With the demo on, the blue stripe shows what is really happening that
  isn't an alarm: the GMC-800 disconnected, saturated or without counts,
  and its memory going over 80% or 95%.
- In the demo, writing the GMC-800 settings, setting its clock, turning the
  RC102 on or off and resetting the spectrum now say that this doesn't
  change the real device.

## 2.83.0
- O app baixa a memória do GMC-800 sozinho toda vez que ele conecta (ou
  quando o app abre com ele conectado), depois de 10 s de conexão estável.
  Espera a gravação da configuração ou outro download terminar, deixa pelo
  menos 2 min entre dois automáticos e, se você cancelar, só tenta de novo
  na próxima conexão. No fim aparece um aviso com o tempo novo e os
  alertas; se der erro, o aviso tem "Tentar de novo" e o que chegou fica
  guardado. Dá para desligar em Configurações › GQ GMC-800 › Conexão.
- A pergunta "Você estava fora de casa?" dos trechos novos do histórico
  virou um cartão no Agora, em µSv/h, que fica até ser respondido (no
  lugar da janela em CPM). Se o RC102 também ficou sem conexão no mesmo
  trecho, é uma pergunta só para os dois. A conta RC102 ÷ GMC-800 destaca
  "Estava em casa" quando parece que nada saiu, mas a pergunta aparece
  sempre. "Saí de casa…" abre o mesmo formulário da saída programada
  (horário, nome e quais aparelhos foram), e "Responder depois" deixa o
  trecho só em Fora de casa.
- Uma saída programada no mesmo horário continua ganhando, trecho menor
  que o mínimo entra sem pergunta e, com a leitura interrompida, o último
  trecho só é perguntado quando o download seguinte terminar.
- O "Perguntar se foi fora de casa a partir de" saiu de Alertas e foi para
  Configurações › GQ GMC-800 › Conexão, junto da opção nova.

**English**

- The app downloads the GMC-800 memory by itself every time it connects
  (or when the app opens with it connected), after 10 s of stable
  connection. It waits for a config write or another download to finish,
  keeps at least 2 min between two automatic downloads and, if you cancel,
  only tries again on the next connection. At the end a notice shows the
  new time and the alerts; on an error the notice has "Try again" and what
  arrived is kept. It can be turned off in Settings › GQ GMC-800 ›
  Connection.
- The "Were you away from home?" question for new history stretches is now
  a card on Now, in µSv/h, that stays until answered (instead of the
  window in CPM). If the RC102 was also offline in the same stretch, it is
  a single question for both. The RC102 ÷ GMC-800 ratio highlights "I was
  home" when nothing seems to have left, but the question always shows.
  "I left home…" opens the same form as a planned trip (times, name and
  which devices went), and "Answer later" keeps the stretch only in Away
  from home.
- A planned trip at the same time still wins, a stretch shorter than the
  minimum is merged without a question and, when a download is
  interrupted, the last stretch is only asked about after the next
  download finishes.
- "Ask if you were away from home from" moved from Alerts to Settings ›
  GQ GMC-800 › Connection, next to the new option.

## 2.82.0
- Demonstração nova: um controle só move os dois aparelhos juntos, cada um
  com os próprios limites e sempre em µSv/h. Dez cenários prontos: fundo
  de casa, atenção, alarme, perigo, fonte chegando perto, pico só no
  RC102, GMC-800 saturado, saída de casa, espectro com Cs-137 e aparelho
  desconecta. O "Valor agora" dá o mesmo µSv/h aos dois, a velocidade 10×
  passa os cenários que andam no tempo em segundos, e o som do alarme fica
  mudo, com a opção de tocar.
- Com a demonstração ligada, o app todo mostra um mundo simulado: gráficos,
  resumo, ALARA, calendário, dose, alertas (marcados "demo"), saídas e
  perguntas "fora de casa?", medições, espectro, espectrograma e
  confirmação cruzada. É o mesmo mundo do modo navegador (prints e testes),
  então os dois não ficam mais fora de sincronia. Ao sair, tudo volta para
  os dados reais como estavam.
- Uma faixa azul listrada "DEMONSTRAÇÃO" fica no topo de todas as telas,
  com "Abrir controles" e "Sair". No menu, o item vira "Demonstração
  ligada" em azul, e o Ctrl+K liga e desliga. Azul, e não amarelo, porque
  amarelo é o nível "atenção".
- Exportar CSV, relatório em PDF, backup, nuvem e sincronização ficam
  desligados durante a demonstração ("saia da demonstração para usar"),
  para nunca sair um arquivo com dado de mentira. Nada vai para os arquivos
  do app; os aparelhos de verdade continuam sendo lidos e gravados por
  baixo, e um alarme de verdade tira da demonstração na hora.

**English**

- New demo: one control moves both devices together, each with its own
  limits and always in µSv/h. Ten ready scenarios: home background,
  attention, alarm, danger, source coming closer, spike on the RC102 only,
  GMC-800 saturated, leaving home, spectrum with Cs-137 and device
  disconnects. "Value now" gives both the same µSv/h, the 10× speed plays
  the time-based scenarios in seconds, and the alarm sound is muted, with
  an option to play it.
- With the demo on, the whole app shows a simulated world: charts,
  summary, ALARA, calendar, dose, alerts (marked "demo"), trips and "away
  from home?" questions, measurements, spectrum, spectrogram and cross
  check. It is the same world as the browser mode (screenshots and tests),
  so the two no longer drift apart. When you leave, everything goes back to
  the real data as it was.
- A striped blue "DEMO" band stays at the top of every screen, with "Open
  controls" and "Leave". In the menu the item turns into "Demo on" in blue,
  and Ctrl+K turns it on and off. Blue, not yellow, because yellow is the
  "attention" level.
- CSV export, PDF report, backup, cloud and sync are off during the demo
  ("leave the demo to use this"), so a file with made-up data never leaves
  the app. Nothing goes to the app's files; the real devices keep being read
  and recorded underneath, and a real alarm ends the demo right away.

## 2.81.0
- Medir objeto: o fundo vem antes de procurar o ponto. A ordem agora é
  Preparar, Fundo, Procurar, Objeto e Resultado: primeiro o fundo sem o
  objeto (ou o fundo da casa, escolhido na própria etapa, com o valor à
  vista), depois você traz o objeto, acha o ponto mais forte e a medição
  começa ali, sem mexer mais. Antes era preciso afastar o objeto depois de
  achar o ponto e tentar achar o mesmo ponto de novo.
- Procurar: mostra quantas vezes o valor está acima do fundo desta
  medição, com o fundo marcado na linha; só diz "estável" depois de 3 s de
  leitura e o pico aparece desde a primeira leitura.
- Alarmes durante a medição: do começo até o resultado (e 1 minuto depois,
  para tirar o objeto de perto), o aparelho que está no objeto não abre
  alerta, não toca e não pisca a janela. O valor fica salvo na medição. O
  aparelho que fica com você continua com alarme normal, e se a medição
  ficar parada esperando o ponto por 30 min os alarmes voltam sozinhos.
  Encostado numa pedra, o GQ GMC-800 conta também beta e mostrava "Perigo"
  com valores 4 a 6 vezes maiores que o Radiacode 102.
- Gráfico de CPM: com os dois aparelhos, vira dois andares (GMC-800 em
  cima, RC102 embaixo), cada um com a sua escala, nome e cor, no mesmo
  horário. A escala do GMC-800 não estica mais por causa de um pico de 1
  segundo que nem aparece nesse gráfico, e o "máx" mostra o maior ponto do
  gráfico. Buracos curtos na leitura aparecem hachurados ("sem leitura").
- Gráficos de µSv/h e de CPM: as medições de objeto viram uma linha fina
  com um número no topo (a mesma pedra medida em seguida vira um número
  só), com a lista embaixo do gráfico. Clicar no número ou na lista abre a
  medição salva. Antes cada medição pintava faixas de fundo e de objeto que
  se sobrepunham.

**English**

- Measure object: the background now comes before searching for the spot.
  The order is Prepare, Background, Search, Object and Result: first the
  background without the object (or the home background, chosen in that
  step with its value shown), then you bring the object, find the strongest
  spot and the measurement starts right there, without moving it again.
  Before, you had to move the object away after finding the spot and try to
  find the same spot again.
- Search: shows how many times the reading is above this measurement's
  background, with the background marked on the line; it only says
  "steady" after 3 s of reading, and the peak shows from the first reading.
- Alarms during a measurement: from the start until the result (and 1
  minute after, to move the object away), the device on the object opens no
  alert, plays no sound and doesn't flash the window. The value is saved in
  the measurement. The device that stays with you keeps its normal alarm,
  and if the measurement waits 30 min for the spot, alarms come back on
  their own. Touching a stone, the GQ GMC-800 also counts beta and showed
  "Danger" with values 4 to 6 times higher than the Radiacode 102.
- CPM chart: with both devices it becomes two stacked charts (GMC-800 on
  top, RC102 below), each with its own scale, name and color, on the same
  time axis. The GMC-800 scale no longer stretches because of a 1-second
  peak that isn't even drawn on this chart, and "max" shows the highest
  point on the chart. Short gaps in the readings are hatched ("no reading").
- µSv/h and CPM charts: object measurements become a thin line with a
  number at the top (the same stone measured in a row becomes one number),
  with the list below the chart. Clicking the number or the list opens the
  saved measurement. Before, each measurement painted background and
  object bands that overlapped.

## 2.80.11
- Histórico de configurações: anota a troca do jeito de calcular o minuto do
  GQ GMC-800 (média das contagens de cada segundo desde a 2.80.9), para
  marcar a data em comparações de fundo e de períodos. Nos seus dados, a
  média antiga ficava uns 2% abaixo da contagem por minuto da memória do
  contador; a nova deve ficar mais perto dela.

**English**

- Settings history: records the change in how the GQ GMC-800 minute is
  computed (average of the counts of each second since 2.80.9), to mark the
  date when comparing background and periods. In real data the old average
  was about 2% below the per-minute count from the counter's memory; the new
  one should be closer to it.

## 2.80.10
- Detalhe do alerta do GQ GMC-800: o "Limite" mostrava 0,056 µSv/h quando o
  alerta dizia "a partir de 10,1": o app lia o 10,1 µSv/h do texto como se
  fosse 10 CPM e convertia de novo. Agora mostra o limite certo. Também
  saiu o "1 minutos".
- Espectros: o app só fala em tório (Th-232) quando aparece o pico de
  911 keV ou o de 2.614 keV. O pico de 583 keV fica junto do 609 keV do
  urânio (e o de 238,6 junto do 242), e o RC102 não separa os dois: numa
  pedra de urânio o app chegava a marcar "Tório-232" sem tório nenhum.

**English**

- GQ GMC-800 alert detail: "Limit" showed 0.056 µSv/h when the alert said
  "from 10.1": the app read the 10.1 µSv/h in the text as if it were 10 CPM
  and converted it again. It now shows the right limit. "1 minutes" is gone
  too.
- Spectra: the app only mentions thorium (Th-232) when the 911 keV or the
  2,614 keV peak shows up. The 583 keV peak sits next to uranium's 609 keV
  (and 238.6 next to 242), and the RC102 cannot tell them apart: on a
  uranium rock the app could label "Thorium-232" with no thorium at all.

## 2.80.9
- GQ GMC-800: o valor gravado de cada minuto agora é a média das contagens
  de cada segundo. Antes era só a última leitura do minuto, e o GMC-800 manda
  uma estimativa que sobe e desce em poucos segundos: um pico curto (o
  contador encostado numa pedra por 20 s) sumia do gráfico e da dose. Num
  minuto de 14/11 o valor gravado foi 101 CPM, quando a média segundo a
  segundo foi 793. Os minutos já gravados não mudam.

**English**

- GQ GMC-800: each minute is now saved as the average of the counts of each
  second. Before, it was only the last reading of the minute, and the
  GMC-800 sends an estimate that rises and falls within a few seconds: a
  short peak (the counter held against a rock for 20 s) vanished from the
  chart and from the dose. In one minute on 14/11 the saved value was
  101 CPM while the second-by-second average was 793. Minutes already saved
  do not change.

## 2.80.8
- Nuvem no Linux: quando o cofre do sistema pede a senha, a tela da nuvem e
  o resto do app não ficam parados esperando. Se o pedido for fechado ou
  passar do tempo, o cartão mostra "cofre bloqueado" com "Tentar de novo",
  em vez de pedir o código de recuperação. O código só é pedido quando a
  chave não está mais no cofre.

**English**

- Cloud on Linux: when the system keyring asks for the password, the cloud
  screen and the rest of the app no longer freeze while waiting. If the
  prompt is closed or times out, the card shows "keyring locked" with "Try
  again" instead of asking for the recovery code. The code is only asked for
  when the key is no longer in the keyring.

## 2.80.7
- Nuvem no Linux: o acesso do Google e o código de recuperação são cifrados
  com uma chave guardada no cofre do sistema (GNOME Keyring ou KWallet),
  como já era no Windows. Sem cofre, ficam como antes, num arquivo que só o
  seu usuário lê. Desconectar tira a chave do cofre.
- Nuvem: quando o Google pede para esperar (limite de pedidos), o envio
  espera e continua, em vez de desistir. Um pacote acima do limite de
  download não sobe e o erro aparece no cartão do PC que enviou.

**English**

- Cloud on Linux: the Google sign-in and the recovery code are encrypted
  with a key kept in the system keyring (GNOME Keyring or KWallet), as on
  Windows already. Without a keyring, they stay as before, in a file only
  your user can read. Disconnecting removes the key from the keyring.
- Cloud: when Google asks to wait (rate limit), the upload waits and
  continues instead of giving up. A package above the download limit is not
  uploaded, and the error shows on the card of the PC that sent it.

## 2.80.6
- Anexos de tipos que podem levar a localização dentro e que o app não sabe
  limpar (vídeos, fotos TIFF e RAW) ficam só neste PC: não vão para o
  backup, a nuvem nem a sincronização pela rede. O app avisa ao anexar.
- Nuvem: dois aparelhos criando a nuvem no mesmo minuto agora terminam
  sempre com uma nuvem só, mesmo quando os dois acham a mesma pasta vazia.
- Política de privacidade: a lista do que sobe para a nuvem ficou completa
  (saídas de casa com os nomes delas, espectros, históricos de configuração
  e calibração, nome do aparelho). A página de segurança fala da nuvem.

**English**

- Attachments of types that can carry a location inside and that the app
  cannot clean (videos, TIFF and RAW photos) stay only on this PC: they do
  not go to the backup, the cloud or the local network sync. The app says
  so when attaching.
- Cloud: two devices creating the cloud in the same minute now always end
  up with a single cloud, even when both find the same empty folder.
- Privacy policy: the list of what goes up to the cloud is now complete
  (times away from home with their names, spectra, settings and
  calibration histories, device name). The security page covers the cloud.

## 2.80.5
- Nuvem: arquivo grande (mais de 8 MB, como uma foto anexada) sobe em
  pedaços. Se a conexão cair no meio, o envio continua do último pedaço
  que chegou, em vez de recomeçar do zero.
- Nuvem: o download tem um limite de tamanho. Um arquivo da nuvem grande
  demais é pulado, sem travar o resto.
- Nuvem no Windows: o código de recuperação copiado não entra no histórico
  da área de transferência (Win+V) nem sobe para a nuvem da Microsoft.

**English**

- Cloud: large files (over 8 MB, such as an attached photo) upload in
  pieces. If the connection drops halfway, the upload continues from the
  last piece that arrived, instead of starting over.
- Cloud: downloads have a size limit. A cloud file that is too large is
  skipped without blocking the rest.
- Cloud on Windows: the copied recovery code does not go into the
  clipboard history (Win+V) or up to Microsoft's cloud clipboard.

## 2.80.4
- Nuvem: no "Entrar com Google" de novo, outra conta Google é recusada com
  um aviso, em vez de dar erro depois.
- Nuvem: desconectar o único aparelho da nuvem também tira o RadLog da
  lista de apps da conta Google. Com outros aparelhos ligados, não tira,
  para não desconectar os outros.
- Nuvem: a pasta do RadLog no Drive ganha uma marca. Um aparelho novo acha
  a nuvem mesmo que a pasta mude de nome, e dois aparelhos criando a nuvem
  ao mesmo tempo não viram duas nuvens.

**English**

- Cloud: when signing in with Google again, a different Google account is
  refused with a notice, instead of failing later.
- Cloud: disconnecting the only device in the cloud also removes RadLog
  from the Google account's app list. With other devices connected it does
  not, so the others stay connected.
- Cloud: the RadLog folder in Drive gets a mark. A new device finds the
  cloud even if the folder is renamed, and two devices creating the cloud
  at the same time no longer end up with two clouds.

## 2.80.3
- Nuvem: "Começar uma nuvem nova" só manda a nuvem antiga para a lixeira do
  Drive depois de a nova estar criada. Quem desiste no meio continua com a
  antiga.
- Nuvem: o código de recuperação copiado sai da área de transferência
  depois de 1 minuto.
- Nuvem: com o relógio errado ou sem internet, o cartão também mostra "Ver
  código de recuperação" e "Desconectar do Google". "Pausar" avisa que o
  envio continua na próxima sincronização.
- Nuvem em inglês: "(há 1 h)", a palavra para confirmar a nuvem nova (agora
  "NEW") e a página do fim do login.
- A versão não é publicada se faltar a chave do app no Google.

**English**

- Cloud: "Start a new cloud" only sends the old cloud to the Drive trash
  after the new one is created. If you give up halfway, you keep the old
  one.
- Cloud: the copied recovery code is cleared from the clipboard after 1
  minute.
- Cloud: with a wrong clock or no internet, the card also shows "Show
  recovery code" and "Disconnect from Google". "Pause" says the upload
  continues at the next sync.
- Cloud in English: "(1 h ago)", the word to confirm a new cloud (now
  "NEW") and the page at the end of the login.
- A release is not published if the app's Google key is missing.

## 2.80.2
- Nuvem: quem já tinha a nuvem ligada na 2.79 agora vê o aviso "Recomeçar
  a nuvem". Na 2.80.0 e na 2.80.1 o app não relia o arquivo da nuvem e
  ficava "em dia" no formato antigo.

**English**

- Cloud: if the cloud was already on in 2.79, the "Restart the cloud"
  notice now shows. In 2.80.0 and 2.80.1 the app did not read the cloud
  file again and stayed "up to date" in the old format.

## 2.80.1
- Importar backup traz de volta uma anotação, medição ou saída de casa
  apagada sem querer, e ela volta também nos outros aparelhos da nuvem.
  Antes, a marca de apagado impedia que ela voltasse.

**English**

- Import backup brings back a note, measurement or time away from home
  that was deleted by mistake, and it comes back on the other cloud
  devices too. Before, the deletion mark kept it from coming back.

## 2.80.0
- Nuvem no formato novo. Na primeira vez, o cartão da nuvem pede para
  "Recomeçar a nuvem": o código de recuperação continua o mesmo, o app cria
  a pasta nova e só depois manda a antiga para a lixeira do Drive (fica lá
  30 dias). Os outros aparelhos só precisam atualizar o RadLog.
- Nuvem: apagar ou editar uma anotação, medição ou saída de casa agora vale
  nos outros aparelhos, e o que foi apagado não volta.
- Nuvem: sobe só o dia que mudou, em vez do mês inteiro. Os espectros vão
  por mês, e cada anexo sobe uma vez só (sem os metadados da foto).
- Nuvem: o mesmo PC continua o mesmo aparelho depois de desconectar e
  entrar de novo.
- Nuvem: "remover" em cada aparelho da lista. Os arquivos dele vão para a
  lixeira do Drive, e quem estiver parado há mais de 30 dias ganha a
  etiqueta "parado há N dias". Um aparelho removido que ainda está ligado
  avisa e pode entrar de novo com o código.
- Nuvem: se a pasta do RadLog for copiada para outro PC, a nuvem pausa e
  pergunta se é um aparelho novo ou o mesmo.
- Nuvem: arquivo de uma versão mais nova do RadLog fica para depois de
  atualizar, em vez de ser ignorado para sempre. O formato está descrito
  em docs/nuvem-formato.md.

**English**

- Cloud in the new format. The first time, the cloud card asks you to
  "Restart the cloud": the recovery code stays the same, the app creates
  the new folder and only then sends the old one to Drive trash (it stays
  there for 30 days). The other devices only need to update RadLog.
- Cloud: deleting or editing a note, measurement or time away from home now
  applies on the other devices, and what was deleted does not come back.
- Cloud: only the day that changed is uploaded, instead of the whole month.
  Spectra go by month, and each attachment is uploaded only once (without
  photo metadata).
- Cloud: the same PC stays the same device after disconnecting and signing
  in again.
- Cloud: "remove" on each device in the list. Its files go to Drive trash,
  and a device idle for more than 30 days gets an "idle for N days" tag. A
  removed device that is still on says so and can join again with the code.
- Cloud: if the RadLog folder is copied to another PC, the cloud pauses and
  asks whether it is a new device or the same one.
- Cloud: a file from a newer RadLog version waits until you update, instead
  of being skipped forever. The format is described in
  docs/nuvem-formato.md.

## 2.79.1
- Nuvem: as fotos e outros anexos das medições agora chegam no outro
  aparelho (antes chegava só a medição). Quem já tinha a nuvem ligada baixa
  e junta de novo uma vez, sozinho.
- Nuvem: um pacote de outro aparelho que não entra não trava mais a
  sincronização; o resto segue e ele é tentado de novo.
- Nuvem: se a chave guardada no PC ficar ilegível (pasta copiada, Windows
  reinstalado), o app pede para entrar e digitar o código de recuperação, e
  o PC continua o mesmo aparelho na lista.
- Nuvem: erro que se repete (Drive cheio, por exemplo) aparece em amarelo na
  hora, com uma explicação em português, em vez de "em dia".
- Desconectar no instante em que um envio termina não trava mais a tela da
  nuvem.

**English**

- Cloud: photos and other measurement attachments now reach the other
  device (before only the measurement did). If the cloud was already on, it
  downloads and merges again once, on its own.
- Cloud: a package from another device that does not go in no longer stops
  syncing; the rest goes on and it is tried again.
- Cloud: if the key saved on the PC becomes unreadable (copied folder,
  reinstalled Windows), the app asks you to sign in and type the recovery
  code, and the PC stays the same device in the list.
- Cloud: an error that keeps happening (a full Drive, for example) shows in
  yellow right away, with an explanation, instead of "up to date".
- Disconnecting right when an upload finishes no longer freezes the cloud
  screen.

## 2.79.0
- Nuvem no Google Drive (experimental, desligada até você ligar):
  Configurações › Dados e backup › Nuvem. Entra com o Google, guarda um
  código de recuperação e sobe uma cópia cifrada dos dados (um arquivo por
  mês) numa pasta do RadLog no seu próprio Drive. Outro PC entra com a mesma
  conta e o código, baixa o histórico e junta só o que falta, como no
  "Importar backup". Sincroniza sozinha a cada 15 minutos.
- O app só enxerga a pasta que ele mesmo criou no Drive. O Google guarda os
  arquivos mas não consegue ler. Lugar, coordenadas e metadados de foto
  nunca sobem, e o e-mail da conta aparece só na tela.
- Aviso no menu lateral e em Configurações quando a nuvem precisa de você
  (entrar de novo, relógio do PC errado ou mais de 1 dia sem sincronizar).
- Política de privacidade nova no repositório (PRIVACY.md).

**English**

- Google Drive cloud (experimental, off until you turn it on): Settings ›
  Data and backup › Cloud. Sign in with Google, save a recovery code and the
  app uploads an encrypted copy of your data (one file per month) to a
  RadLog folder in your own Drive. Another PC signs in with the same account
  and the code, downloads the history and merges only what is missing, like
  "Import backup". It syncs on its own every 15 minutes.
- The app only sees the folder it created in Drive. Google stores the files
  but can't read them. Location, coordinates and photo metadata are never
  uploaded, and the account email only shows on screen.
- Notice in the side menu and in Settings when the cloud needs you (sign in
  again, wrong PC clock or more than 1 day without syncing).
- New privacy policy in the repository (PRIVACY.md).

## 2.78.3
- Na comparação de linhas do espectro de uma saída com casa, a incerteza
  agora conta o pedaço de casa que foi tirado da leitura da volta. Antes ela
  saía menor do que é, e podiam aparecer linhas "mais que em casa" ou "menos
  que em casa" que não eram de verdade.

**English**

- When comparing a trip's spectrum lines with home, the uncertainty now
  includes the home part removed from the return reading. Before it came out
  smaller than it is, and lines could show as "more than at home" or "less
  than at home" when they weren't.

## 2.78.2
- O espectro de uma saída não mistura mais horas de casa. Quando o RC102
  para de mandar espectro um tempo antes de sair, a primeira leitura da
  volta cobre também esse tempo em casa; agora esse pedaço sai pelo espectro
  de casa, e o tempo mostrado é o da saída (antes aparecia 14 h numa saída
  de 10 h).
- O resumo do relatório diz quanto a dose ficou abaixo da de casa ("32%
  abaixo") em vez de "perto" quando a diferença passa de 10%.
- Uma saída programada que já passou do horário avisa que a pergunta vem na
  tela Agora uns 2 minutos depois que o aparelho conectar de novo.

**English**

- A trip's spectrum no longer mixes in hours at home. When the RC102 stops
  sending spectra a while before leaving, the first reading after the return
  also covers that time at home; that part is now removed using the home
  spectrum, and the time shown is the trip's (before it showed 14 h for a
  10 h trip).
- The report summary says how far the dose was below staying home ("32%
  below") instead of "close" when the difference is over 10%.
- A scheduled trip past its time says the question comes on the Now screen
  about 2 minutes after the device connects again.

## 2.78.1
- Uma mudança de configuração feita bem no instante em que a gravação
  anterior terminava podia ficar parada na fila e se perder ao fechar o
  app. Agora ela sempre vai para o disco.
- O registro de erro de gravação diz o arquivo certo (configurações ou uso).

**English**

- A settings change made right as the previous write was finishing could
  sit in the queue and be lost on exit. It now always reaches the disk.
- The write error log names the right file (settings or usage).

## 2.78.0
O resto da auditoria de 13/11, numa versão só: segurança, gráficos, dados,
velocidade, testes e textos.

- O espectro do RC102 para em 4.096 canais. Uma resposta malformada (ou de
  um aparelho falso por perto) virava milhões de canais e gigabytes de
  memória.
- O radar só busca imagens no RainViewer, por https, e as respostas do
  serviço de clima e do radar são lidas até 4 MB.
- O erro da busca de cidade entra na tela como texto.
- O SECURITY.md tem o passo a passo certo para a chave de assinatura perdida
  e o caso de uma chave vazar, com um teste que ensaia a troca.
- O botão Reconectar do RC102 espera a leitura em andamento antes de fechar
  o USB.
- Nos gráficos de 1 ano (taxa e dose) cada dia vai de meia-noite a
  meia-noite. Antes ia das 21:00 às 21:00.
- Só as 5 cópias mais novas da memória do GMC-800 ficam guardadas.
- O backup não leva mais o nome do computador, nem dentro do arquivo nem no
  nome sugerido.
- A Minha dose abre bem mais rápido: o ano de leituras é lido uma vez por
  conta, não uma vez por gráfico (de 3,5 s para 1,3 s num ano cheio).
- Configurações e contagem de uso gravam no disco em segundo plano, juntando
  várias mudanças seguidas numa gravação só. Ao fechar, o que falta é
  gravado.
- O app não carrega mais o código do modo demonstração quando roda de
  verdade.
- Minutos que ficaram na pasta temporária quando a gravação falhou voltam
  para o histórico ao abrir o app.
- O CSV exportado confere se foi gravado inteiro antes de dizer que deu
  certo, e a mudança de pasta da 2.0 tenta de novo quando o Windows segura
  um arquivo.
- Atenção: depois de abrir esta versão (ou a 2.76.2 em diante), uma versão
  anterior à 2.76.2 não mostra mais os alertas, que agora ficam em
  alertas.csv. Voltar para uma versão antiga não é recomendado.
- A tradução para o inglês cobre quase todos os textos que faltavam
  (gráficos, Medir objeto, relatórios, mensagens do RC102, modo
  demonstração). A busca rápida também acha as ações por palavras em inglês.
- A data no Resumo tem nome para leitor de tela, e o texto do alerta e do
  passo do RC102 não aparece vazio antes de carregar.
- O modo demonstração mostra a versão atual e uma "versão nova" depois dela.
- O README tem a seção "O que vai para a internet": checagem de versão,
  clima e radar, e nada mais. A sincronização fica na rede de casa.
- Testes: busca aleatória (fuzz) toda semana, teste das sub-abas e janelas
  nos dois idiomas, e checagem de que toda função chamada pela tela existe.

**English**

- The RC102 spectrum stops at 4,096 channels. A malformed reply (or one from
  a fake device nearby) turned into millions of channels and gigabytes of
  memory.
- The radar only fetches images from RainViewer over https, and replies from
  the weather and radar services are read up to 4 MB.
- The city search error goes on screen as text.
- SECURITY.md has the right steps for a lost signing key and the case of a
  leaked key, with a test that rehearses the switch.
- The RC102 Reconnect button waits for the current read before closing USB.
- On the 1-year charts (rate and dose) each day runs midnight to midnight.
  Before it ran from 21:00 to 21:00.
- Only the 5 newest copies of the GMC-800 memory are kept.
- Backups no longer carry the computer's name, neither inside the file nor
  in the suggested file name.
- My dose opens much faster: the year of readings is read once per
  calculation, not once per chart (from 3.5 s to 1.3 s on a full year).
- Settings and usage counts are written to disk in the background, folding
  several changes in a row into one write. On exit, whatever is left is
  written.
- The app no longer loads the demo mode code when running for real.
- Minutes left in the temp folder when saving failed come back into the
  history when the app opens.
- The exported CSV checks it was written in full before reporting success,
  and the 2.0 folder move retries when Windows holds a file.
- Note: after opening this version (or 2.76.2 onwards), versions before
  2.76.2 no longer show alerts, which now live in alertas.csv. Going back to
  an old version is not recommended.
- The English translation covers almost all the texts that were missing
  (charts, Measure object, reports, RC102 messages, demo mode). Quick search
  also finds actions by English words.
- The date on Summary has a name for screen readers, and the alert and RC102
  step texts no longer show up empty before loading.
- Demo mode shows the current version and a "new version" after it.
- The README has a "What goes to the internet" section: version check,
  weather and radar, nothing else. Sync stays on the home network.
- Tests: weekly fuzzing, a test of sub-tabs and dialogs in both languages,
  and a check that every function the screen calls exists.

## 2.77.0
Bloco 3 da auditoria de 13/11: relógio, porta do contador, versões e fotos.

- O relógio do GMC-800 é acertado sozinho ao conectar quando está mais de
  3 s fora do relógio do PC, com uma linha em Eventos. Era a origem do
  histórico com hora errada.
- A reconexão automática procura o GMC-800 pelo USB, não só pelo número da
  porta: se ele mudar de COM, o app acha; se outro aparelho pegar aquele
  número, o app não manda comandos para ele. O botão Detectar testa primeiro
  as portas com o chip USB do contador.
- Abrir uma versão mais antiga com a nova aberta não troca mais: a nova
  continua e avisa. Para usar a antiga, feche o app e abra ela.
- Fotos anexadas às medições perdem os metadados (localização do celular,
  modelo da câmera, data) ao anexar; a rotação fica. As anexadas antes saem
  limpas no backup e na sincronização. JPEG, PNG, WebP e HEIC; uma foto que
  não dá para limpar não é anexada.
- Anexo com nome comprido anexado duas vezes não some mais da lista (QA).
- Minutos esperando o arquivo do mês liberar são tentados de novo a cada
  minuto, mesmo com o contador desconectado, e se o app fechar antes ficam
  guardados e entram no mês na próxima abertura (QA).

**English**

- The GMC-800 clock is set automatically on connect when it is more than
  3 s off the PC clock, with a line in Events. It was the source of history
  with the wrong time.
- Automatic reconnect looks for the GMC-800 by USB, not only by port number:
  if it moves to another COM, the app finds it; if another device takes that
  number, the app doesn't send it commands. The Detect button tries the
  ports with the counter's USB chip first.
- Opening an older version while the new one runs no longer switches: the
  new one stays and warns. To use the older one, close the app and open it.
- Photos attached to measurements lose their metadata (phone location,
  camera model, date) when attached; rotation is kept. Photos attached
  before go out clean in backups and sync. JPEG, PNG, WebP and HEIC; a photo
  that can't be cleaned isn't attached.
- An attachment with a long name attached twice no longer disappears from
  the list (QA).
- Minutes waiting for the month's file to be free are retried every minute,
  even with the counter disconnected, and if the app closes first they are
  kept and go into the month on the next start (QA).

## 2.76.2
Bloco 2 da auditoria de 13/11: arquivos que podiam ficar vazios ou sumir.

- Todo arquivo que o app grava agora passa pela gravação segura: escreve num
  temporário, força o disco e só então troca. Antes 22 lugares gravavam sem
  forçar o disco ou direto por cima, e numa queda de energia o arquivo podia
  ficar vazio (versões da calibração, pareamentos, intercalibração, minutos e
  dose do RC102, entre outros). Um teste impede que volte.
- Versões da calibração, intercalibração, calibração do contador,
  sincronização, perguntas do RC102 e dose do RC102 estragados não começam
  mais vazios em silêncio: ficam guardados em .ilegivel, com aviso no log.
- No Windows, trocar um arquivo que outra parte do app está lendo (baixar o
  histórico com um gráfico de 1 ano aberto) tenta de novo por até 1 s em vez
  de falhar com "acesso negado" e deixar um .tmp.
- Alertas não somem mais: ficam num arquivo próprio (alertas.csv), sem
  limite. Só os eventos comuns continuam com o limite de 5.000. Na primeira
  abertura o app separa os alertas que estavam no eventos.csv.
- Os arquivos do RC102 guardam em .ilegivel as linhas que não dá para ler,
  em vez de descartá-las ao regravar, e uma linha enorme não esconde mais o
  resto do mês.
- Se a pasta de leituras não abrir, a tela Agora mostra um aviso fixo
  enquanto durar. Se nem a pasta temporária abrir, o app segue com as
  leituras na memória em vez de quebrar.

**English**

- Every file the app writes now goes through safe saving: it writes a
  temporary file, flushes it to disk and only then swaps it in. Before, 22
  places saved without flushing or straight over the old file, and a power
  cut could leave it empty (calibration versions, pairings, cross-calibration,
  RC102 minutes and dose, among others). A test keeps it from coming back.
- Damaged calibration versions, cross-calibration, counter calibration, sync,
  RC102 questions and RC102 dose no longer start empty silently: they are
  kept as .ilegivel, with a note in the log.
- On Windows, replacing a file another part of the app is reading
  (downloading history with a 1-year chart open) retries for up to 1 s
  instead of failing with "access denied" and leaving a .tmp behind.
- Alerts no longer disappear: they live in their own file (alertas.csv),
  with no limit. Only ordinary events keep the 5,000 limit. On first start
  the app moves the alerts out of eventos.csv.
- RC102 files keep unreadable lines in .ilegivel instead of dropping them
  when rewriting, and a huge line no longer hides the rest of the month.
- If the readings folder doesn't open, the Now screen shows a fixed notice
  for as long as it lasts. If not even the temporary folder opens, the app
  keeps the readings in memory instead of crashing.

## 2.76.1
Bloco 1 da auditoria de 13/11: o que fazia perder leitura ou arquivo.

- Histórico do GMC-800 com o relógio adiantado não trava mais a gravação ao
  vivo. Antes, os minutos que caíam no futuro faziam o app descartar cada
  minuto novo, sem aviso, até a hora real passar deles. Agora o que vem do
  histórico com hora no futuro ou antes de 2020 fica de fora, com um aviso em
  Eventos para conferir o relógio.
- Com o arquivo do mês aberto no Excel (ou em outro programa que trava o
  arquivo), os minutos não se perdem mais: ficam na memória e nos gráficos e
  são gravados assim que o arquivo liberar. A tela Agora mostra um aviso
  enquanto isso.
- Leituras impossíveis (NaN, infinito, negativas) são recusadas na leitura
  dos arquivos e na entrada ao vivo do RC102. Antes uma delas parava a tela.
- Um backup de outra pessoa não consegue mais apontar anexos para fora da
  pasta do app: nomes assim são descartados ao importar e ao abrir o app.
  "Abrir" um anexo entrega ao sistema só foto, PDF e texto; o resto abre a
  pasta.
- Mais seis tarefas internas ganharam a proteção contra erro interno, e um
  teste impede que entre outra sem ela.

**English**

- GMC-800 history with the clock running ahead no longer stops live
  recording. Before, minutes that landed in the future made the app drop
  every new minute, silently, until real time caught up. Now history with a
  time in the future or before 2020 is left out, with a note in Events to
  check the clock.
- With the month's file open in Excel (or another program that locks it),
  minutes are no longer lost: they stay in memory and on the charts and are
  saved as soon as the file is free. The Now screen shows a notice meanwhile.
- Impossible readings (NaN, infinity, negative) are rejected when reading
  files and in the RC102 live input. Before, one of them froze the screen.
- Someone else's backup can no longer point attachments outside the app
  folder: such names are dropped on import and when the app opens. "Open" on
  an attachment hands only photos, PDF and text to the system; anything else
  opens the folder.
- Six more internal tasks got the crash protection, and a test keeps new ones
  from coming in without it.

## 2.76.0
- Configurações › Dados e backup ganhou o cartão "Nuvem (Google Drive)",
  acima da Sincronização. Por enquanto é só a tela, marcada como WIP e
  desligada: nada nele é clicável. O login com o Google e o envio cifrado
  para o Drive entram numa próxima versão.

**English**

- Settings › Data and backup has a new "Cloud (Google Drive)" card, above
  Sync. For now it's only the screen, marked WIP and turned off: nothing in it
  can be clicked. Google sign-in and the encrypted upload to Drive come in a
  later version.

## 2.75.3
- Barra lateral com texto grande (Configurações › Janela, acima de 100%):
  ela cresce junto com o texto, e a pílula "vX disponível" e a versão ao
  lado do logo não cortam mais. Os avisos longos da atualização ("baixando…",
  "troca quando o CPM normalizar") quebram em duas linhas dentro da barra.

**English**

- Sidebar with large text (Settings › Window, above 100%): it grows with the
  text, and the "vX available" pill and the version next to the logo are no
  longer cut off. Long update notices ("downloading…", "switches when the
  CPM settles") wrap inside the bar.

## 2.75.2
- O arquivo de leituras volta a recusar datas que não existem (31/04, 29/02
  fora de ano bissexto): a leitura rápida da 2.75.1 as aceitava e jogava o
  minuto para o mês seguinte. A linha vai de novo para o .ilegivel.
- Tirado do repositório um binário de teste que entrou por engano.

**English**

- The readings file again rejects dates that don't exist (April 31,
  February 29 outside a leap year): the fast reader in 2.75.1 accepted them
  and moved the minute to the next month. The line goes to .ilegivel again.
- Removed a test binary that was committed by mistake.

## 2.75.1
Item P1 da auditoria de 12/11 e acertos da Saída programada.

- Gráficos, relatórios e tendências de períodos longos ficaram cerca de 2 a
  3 vezes mais rápidos e usam bem menos memória. Num ano de minutos, a
  leitura caiu de ~0,7 s e ~740 MB alocados para ~0,25 s e ~200 MB.
- Enquanto uma consulta longa lê os meses antigos, a gravação do minuto não
  fica mais esperando: os dias recentes vêm da memória e os antigos são lidos
  sem segurar a trava.
- Saída programada em inglês: o nome que você deu à saída não é mais
  traduzido (ficava "Office" em parte das telas).
- Fora de casa em tela estreita: os botões de cada trecho descem para baixo
  do texto e o eixo dos últimos 7 dias mostra dia sim, dia não, sem os
  rótulos se encostarem.
- "Saí de casa" mostra a hora em 24 h também em inglês.

**English**

- Charts, reports and trends over long periods got about 2 to 3 times faster
  and use much less memory. For a year of minutes, reading went from ~0.7 s
  and ~740 MB allocated to ~0.25 s and ~200 MB.
- While a long query reads old months, saving the current minute no longer
  waits: recent days come from memory and old ones are read without holding
  the lock.
- Planned trip in English: the name you gave the trip is no longer
  translated (part of the screens showed "Office").
- Away from home on a narrow window: each stretch's buttons move below the
  text and the last-7-days axis shows every other day, so labels no longer
  touch.
- "I left home" shows the time in 24 h in English too.

## 2.75.0
- Saída programada: em Fora de casa, "Programar saída" marca
  para onde vai, por volta de que horas e quais aparelhos vão. Também no
  Ctrl+K e no "Saí de casa" ("É depois? Programar a saída"). Pode desligar o
  PC.
- Quando o aparelho que foi conecta de novo, o app pergunta "Você saiu
  mesmo?" no lugar do "onde estava o RC102?". A saída vem da desconexão do
  aparelho (PC ligado, até 1 h do programado), senão da conta RC102 ÷
  GMC-800, senão do horário programado (marcado como "horário estimado"). A
  volta é a hora em que ele conectou, com uma dica se a conta mostra que você
  chegou mais de 1 h antes.
- Se a conta ficou com o jeito de casa o tempo todo, a pergunta vira "A saída
  programada não aconteceu?". Uma queda curta do Bluetooth não conta como
  saída. Um trecho marcado à mão no mesmo horário ganha.
- Na lista de Fora de casa, a saída marcada ganha "programada" e, quando for
  o caso, "horário estimado". As programadas vão no backup.
- Textos do QA na pergunta das configurações do backup.

**English**

- Planned trip: in Away from home, "Plan a trip" sets where
  you're going, around what time and which devices go. Also in Ctrl+K and in
  "I left home" ("Later? Plan the trip"). You can turn the PC off.
- When the device that went connects again, the app asks "Did you really
  leave?" instead of "where was the RC102?". The departure comes from the
  device disconnecting (PC on, up to 1 h from the plan), else from the RC102 ÷
  GMC-800 count, else from the planned time (marked "estimated time"). The
  return is when it connected, with a hint if the count shows you got back
  more than 1 h earlier.
- If the count looked like home the whole time, the question becomes "Didn't
  the planned trip happen?". A short Bluetooth drop doesn't count as leaving.
  A stretch marked by hand at the same time wins.
- In the Away from home list, the marked trip gets "planned" and, when it
  applies, "estimated time". Planned trips go into the backup.
- QA wording fixes in the backup settings question.

## 2.74.0
Item B5 da auditoria de 12/11.

- O backup passa a levar as configurações: alarme, níveis, som, bandeja,
  pulso, idioma e atualização. Não leva o lugar da chuva nem o que é só do
  PC (porta, Bluetooth, pastas, tamanho da tela, início com o Windows).
- Ao importar um backup com configurações diferentes das daqui, o app
  pergunta se você quer usar as do backup. Se disser que não, nada muda. A
  troca fica no histórico de configurações, com origem "backup".

**English**

- Backups now carry the settings: alarm, levels, sound, tray, pulse,
  language and updates. They don't carry the rain location or what belongs
  only to the PC (port, Bluetooth, folders, screen size, starting with
  Windows).
- When importing a backup whose settings differ from the ones here, the app
  asks whether you want to use the backup's. If you say no, nothing changes.
  The change shows in the settings history, with source "backup".

## 2.73.2
- A tela passa a ter uma política de conteúdo (CSP, item S5 da auditoria):
  ela só carrega os arquivos do próprio app. É uma segunda barreira caso
  algum texto vindo de fora (como o nome de um aparelho Bluetooth) escape
  da proteção que já existe. Testado no Windows antes de sair.

**English**

- The screen now has a content security policy (CSP, audit item S5): it
  only loads the app's own files. It is a second barrier in case some outside
  text (such as a Bluetooth device name) slips past the existing protection.
  Tested on Windows before release.

## 2.73.1
Itens soltos da auditoria de 12/11 (B4, B7, S6, U5).

- Sincronização entre PCs: cada mensagem tem no máximo 128 MB (eram 512) e
  o PC que recebe tem 5 minutos para ler o pedido inteiro. A procura na rede
  ("Procurar na rede") só acha este PC enquanto o código de pareamento está
  na tela: fora disso, ninguém na rede fica sabendo o nome dele.
- Radar da chuva: a imagem tem limite de tamanho (8 MB e 4096 px de lado)
  antes de ser lida.
- Alertas: as legendas do gráfico da confirmação cruzada e do gráfico do
  fundo saem com 11 px no mínimo (estavam com 10,5).
- Topo: em janelas com menos de 680 px, os quadros dos dois aparelhos ficam
  um embaixo do outro (lado a lado ficavam estreitos demais).

**English**

Loose items from the 12 Nov audit (B4, B7, S6, U5).

- Sync between PCs: each message is at most 128 MB (it was 512) and the
  receiving PC has 5 minutes to read the whole request. "Search the network"
  only finds this PC while the pairing code is on screen: otherwise, nobody
  on the network learns its name.
- Rain radar: the image has a size limit (8 MB and 4096 px per side) before
  it is read.
- Alerts: the labels of the cross-check chart and the background chart are
  at least 11 px (they were 10.5).
- Top row: in windows narrower than 680 px, the two device boxes stack (side
  by side they were too narrow).

## 2.73.0
Topo sem o quadro de alertas e memória do GMC-800 em duas linhas (U1 e U2 da
auditoria; spec ux/topo-sem-alertas.html).

- O quadro "Alertas" sai do topo de todas as telas: quando há alerta, a faixa
  de cima já avisa, e o menu e o topo compacto continuam com a marca. O
  último alerta e o histórico ficam na tela Alertas. Os quadros dos aparelhos
  dividem a largura; quando aparece um só (um aparelho conectado, ou as
  páginas GQ GMC-800 e Radiacode 102), ele ocupa a largura toda.
- Configurações › GQ GMC-800 › Conexão: o campo Memória vira duas linhas
  curtas, "Memória" (quanto está ocupado e quando enche) e "Download" (em dia,
  quanto falta, contador não grava ou nunca), com a explicação embaixo.
- Uso do app: a data do "desde" sai como dd/mm/aaaa também em inglês, e no
  modo demonstração "Zerar contagem" zera o exemplo.
- Prints do README refeitos sem o quadro de alertas.

**English**

Top row without the alerts box and GMC-800 memory in two lines (audit items
U1 and U2; spec ux/topo-sem-alertas.html).

- The "Alerts" box leaves the top of every screen: when there is an alert,
  the bar at the top already says so, and the menu and the compact top still
  show the mark. The last alert and the history are on the Alerts screen. The
  device boxes share the width; when only one shows (one device connected,
  or the GQ GMC-800 and Radiacode 102 pages), it takes the full width.
- Settings › GQ GMC-800 › Connection: the Memory field becomes two short
  lines, "Memory" (how full it is and when it fills up) and "Download" (up to
  date, how much is missing, counter isn't saving, or never), with the
  explanation below.
- App usage: the "since" date shows as dd/mm/yyyy in English too, and in
  demo mode "Reset count" resets the example.
- README screenshots redone without the alerts box.

## 2.72.0
Novo "Uso do app" (spec ux/uso-do-app.html).

- Configurações › Dados e backup › Uso do app: com a chave ligada, o app
  conta, só neste computador, quantas vezes cada página, aba, botão,
  "Como funciona" e seletor foi aberto, e o tempo olhando cada página (só
  com a janela em primeiro plano e mexendo no mouse ou teclado; parado mais
  de 2 minutos, não conta). A tela mostra as páginas com cores de pouco a
  muito, as abas e os botões mais usados da página escolhida, o que nunca
  foi usado em 30 dias, e tem "Copiar resumo" e "Zerar contagem".
- Vem desligado. Guarda só totais por dia, sem horário, no `uso.json` da
  pasta de dados, que fica fora do backup, da sincronização e do PDF.
- Atualização: entra a chave de assinatura de reserva (a pública está no
  `SECURITY.md`). Se a chave principal se perder, as versões novas ainda
  podem ser assinadas e instaladas pelo app.

**English**

New "App usage" (spec ux/uso-do-app.html).

- Settings › Data and backup › App usage: with the switch on, the app
  counts, only on this computer, how many times each page, tab, button,
  "How it works" and selector was opened, and the time spent looking at
  each page (only with the window in front and the mouse or keyboard in
  use; idle for more than 2 minutes doesn't count). The screen shows the
  pages colored from a little to a lot, the most used tabs and buttons of
  the chosen page, what was never used in 30 days, and has "Copy summary"
  and "Reset count".
- It starts off. It keeps only daily totals, with no times, in `uso.json`
  in the data folder, which stays out of the backup, the sync and the PDF.
- Updates: the reserve signing key is in (its public half is in
  `SECURITY.md`). If the main key is ever lost, new versions can still be
  signed and installed by the app.

## 2.71.0
Bloco 4 da auditoria de 12/11: limpeza e documentação.

- A gravação bruta do RC102 (pasta `radiacode/bruto`) sai. Ela servia para
  descobrir o protocolo do aparelho, que hoje já é conhecido, e crescia
  para sempre. O app não grava mais nada ali, não leva a pasta no backup nem
  na sincronização, e o interruptor "Dados brutos" sai das Configurações do
  RC102. Os arquivos antigos ficam onde estão; se quiser o espaço de volta,
  pode apagar a pasta à mão.
- Configurações › Atualizações e sobre: novo "Licenças de terceiros", com os
  avisos das bibliotecas e fontes que vão dentro do app (o arquivo é gerado
  no build e o CI confere se está em dia).
- O app passa a aceitar uma segunda chave de assinatura, de reserva, para o
  caso de a principal se perder (a chave entra numa próxima versão).
- Novo `SECURITY.md` (como avisar de uma falha, a chave pública e o plano se
  a chave se perder). O `CONTRIBUTING.md` ganha as conferências de antes do
  PR e as regras de versão; o README lista as pastas que faltavam e avisa
  para não compartilhar o `sincronizacao.json`. As versões 1.x do CHANGELOG
  vão para o `CHANGELOG-1.x.md`.
- Arrumação interna: o modo demonstração sai do `app.js` para o `demo.js`,
  sem nomes de computador reais; uma função de escape de HTML só no lugar de
  cinco cópias; saem da raiz o `flow.png` e os `.syso` (gerados no build); o
  `build.bat` não roda mais `go mod tidy` e gera o ícone e a versão do
  `.exe`.

**English**

Block 4 of the 12 Nov audit: cleanup and documentation.

- RC102 raw recording (the `radiacode/bruto` folder) is gone. It was there
  to work out the device protocol, which is now known, and it grew forever.
  The app no longer writes anything there, doesn't put the folder in backups
  or sync, and the "Raw data" switch leaves the RC102 settings. Old files
  stay where they are; delete the folder by hand to get the space back.
- Settings › Updates and about: new "Third-party licenses", with the notices
  of the libraries and fonts inside the app (the file is generated at build
  time and CI checks that it is up to date).
- The app now accepts a second, reserve signing key, in case the main one is
  lost (the key itself comes in a later version).
- New `SECURITY.md` (how to report a problem, the public key and the plan if
  the key is lost). `CONTRIBUTING.md` gets the pre-PR checks and the version
  rules; the README lists the missing folders and warns not to share
  `sincronizacao.json`. The 1.x versions of the CHANGELOG move to
  `CHANGELOG-1.x.md`.
- Internal cleanup: demo mode moves out of `app.js` into `demo.js`, without
  real computer names; a single HTML escape function instead of five copies;
  `flow.png` and the `.syso` files (generated at build) leave the root;
  `build.bat` no longer runs `go mod tidy` and generates the `.exe` icon and
  version.

## 2.70.2
Blocos 2 e 3 da auditoria de 12/11: ferramentas e textos.

- Radioproteção: "Minha dose" ganha um "Como funciona" e o PDF uma frase
  na primeira página dizendo que o µSv/h dos aparelhos é uma estimativa
  (calibrados para o césio-137, medindo o ambiente): bom para comparar dias
  e lugares, não é uma dose oficial.
- A banana sai das comparações de dose ("≈ 29 bananas"). Fica só na régua,
  em "Dose recebida", com a ressalva de que o corpo mantém o potássio
  constante e comer banana não soma dose.
- Emergência: o telefone 24 h passa a ser o da ANSN, (21) 99304-9453
  (gov.br/ansn, página de emergência radiológica). Em inglês, a faixa
  mostra o número geral de emergência (911 nos EUA, 112 na Europa) e diz
  "no Brasil" antes dos números daqui.
- Isótopos: o Pb-212 é "descendente" do tório-232 (era "neto") e o Ac-228
  tem uma meia-vida só, 6,15 horas.
- Em inglês, a frase da memória do GMC-800 sai traduzida inteira (saía
  misturada). O modo demonstração não anuncia mais a "v1.18.0".
- Dados: o arquivo do mês das leituras vai para o disco antes de trocar de
  nome. As saídas de casa, as anotações e as medições ganham a mesma
  proteção do config.json: gravação por arquivo temporário e, se o arquivo
  estiver estragado, cópia em `.ilegivel` e aviso em Eventos, em vez de
  começar vazio e sobrescrever. As medições de atividade do RC102 também
  passam a ser gravadas por arquivo temporário.
- Ferramentas: Go 1.27 (o 1.24 saiu de suporte) e Wails 2.17.1, fora da
  faixa do aviso de segurança CVE-2026-34157, com as dependências
  atualizadas. As ações do GitHub ficam presas pelo commit.
- CI: teste de tela (as 11 páginas no modo demonstração, em 1280 e 412 px,
  nos dois idiomas), testes no Windows, staticcheck, govulncheck e uma
  conferência que reprova frase nova sem tradução. Código morto removido.

**English**

Blocks 2 and 3 of the 12 Nov audit: tools and texts.

- Radiation protection: "My dose" gets a "How it works" and the PDF a line
  on its first page saying that the devices' µSv/h is an estimate
  (calibrated for caesium-137, measuring the surroundings): good for
  comparing days and places, not an official dose.
- The banana leaves the dose comparisons ("≈ 29 bananas"). It stays only on
  the scale, under "Dose received", with the note that the body keeps its
  potassium constant and eating a banana adds no dose.
- Emergency: the 24 h number is now ANSN's, (21) 99304-9453 (gov.br/ansn,
  radiological emergency page). In English the banner shows the general
  emergency number (911 in the US, 112 in Europe) and says "in Brazil"
  before the Brazilian numbers.
- Isotopes: Pb-212 is a "descendant" of thorium-232 (it said "grandchild")
  and Ac-228 has a single half-life, 6.15 hours.
- In English, the GMC-800 memory sentence is translated whole (it came out
  mixed). Demo mode no longer announces "v1.18.0".
- Data: the readings month file reaches the disk before it is renamed.
  Trips away from home, notes and measurements get the same protection as
  config.json: written through a temporary file and, if the file is
  damaged, a copy in `.ilegivel` and a notice in Events instead of starting
  empty and overwriting it. RC102 activity measurements are also written
  through a temporary file.
- Tools: Go 1.27 (1.24 is out of support) and Wails 2.17.1, outside the
  range of security advisory CVE-2026-34157, with dependencies updated.
  GitHub actions are pinned by commit.
- CI: screen test (the 11 pages in demo mode, at 1280 and 412 px, in both
  languages), tests on Windows, staticcheck, govulncheck and a check that
  fails on new text without a translation. Dead code removed.

## 2.70.1
Primeiro bloco da auditoria de 12/11: dados e segurança. Nada muda na tela.

- Leituras: uma linha estragada no meio do arquivo do mês (por exemplo, de
  uma gravação cortada por queda de energia) não esconde mais as linhas
  seguintes. Antes o resto do mês sumia da tela e, na importação seguinte
  (histórico do contador, backup ou sincronização), era apagado do arquivo.
  Agora cada linha é lida sozinha, e as que não dá para ler vão para um
  arquivo ao lado (`AAAA-MM.csv.ilegivel`) antes de o mês ser regravado.
- Configurações: o `config.json` é gravado num arquivo temporário e só
  depois troca de nome, então uma queda de energia não deixa mais o arquivo
  pela metade. Se mesmo assim ele não puder ser lido, o app volta ao padrão,
  guarda o arquivo ruim como `config.json.ilegivel` e avisa em Eventos, em
  vez de zerar tudo em silêncio. A chuva por dia ganhou a mesma gravação.
- Atualização: o app só troca de versão sozinho se o nome do arquivo da
  release trouxer a versão anunciada. A assinatura já garantia quem
  publicou; isto impede que um arquivo antigo assinado seja oferecido como
  versão nova.
- Erro interno: o relatório automático da medição, a primeira sincronização
  depois de parear e o ícone da bandeja passam a rodar protegidos, como as
  outras tarefas de fundo. Um erro neles vira uma linha em Eventos em vez de
  fechar o app.
- Release: o `appimagetool` e o runtime do AppImage passam a ser baixados em
  versão fixa, com o SHA-256 conferido antes de rodar (antes vinha do canal
  "continuous", sem conferência, no mesmo job que assina).
- O `.exe` volta a trazer a versão certa no campo fixo do arquivo (estava
  parado em 2.28.0.0). O CI agora confere que `version.go`, `winres.json` e
  o topo deste arquivo dizem a mesma versão.

**English**

First block of the 12 Nov audit: data and security. Nothing changes on
screen.

- Readings: a damaged line in the middle of a month file (for example from
  a write cut short by a power loss) no longer hides the lines after it.
  Before, the rest of the month vanished from the screen and was deleted
  from the file on the next import (counter history, backup or sync). Each
  line is now read on its own, and unreadable ones are saved to a file next
  to it (`YYYY-MM.csv.ilegivel`) before the month is rewritten.
- Settings: `config.json` is written to a temporary file and then renamed,
  so a power loss can no longer leave it half written. If it still can't be
  read, the app goes back to the defaults, keeps the bad file as
  `config.json.ilegivel` and says so in Events instead of resetting
  silently. The rain-per-day file is written the same way.
- Update: the app only switches versions by itself when the release file
  name carries the announced version. The signature already proved who
  published; this stops an old signed file from being offered as a new
  version.
- Internal errors: the automatic measurement report, the first sync after
  pairing and the tray icon now run protected like the other background
  tasks. An error there becomes a line in Events instead of closing the
  app.
- Release: `appimagetool` and the AppImage runtime are now downloaded at a
  fixed version, with the SHA-256 checked before running.
- The `.exe` carries the right version in its fixed version field again (it
  was stuck at 2.28.0.0). CI now checks that `version.go`, `winres.json`
  and the top of this file agree.

## 2.70.0
- "De onde vem" (detalhe do isótopo na aba Espectro) ganha:
  - a linha do decaimento: a série ou cadeia do isótopo, do pai até o
    estável, com o que decai por α ou β e a meia-vida de cada passo. O
    isótopo aberto fica em rosa e os que fazem pico no RC102 ganham o γ.
    Séries longas aparecem resumidas, com "ver a série inteira"; o ramo do
    Bi-212 (64% β, 36% α) aparece lado a lado. Dados conferidos na tabela
    da SUNY Potsdam / ENSDF (spec ux/isotopos.html, item 2c);
  - a linha "Uso" nos isótopos artificiais e no Ra-226 e U-235: medicina
    (em azul), indústria, casa, energia, antigo (item 2d).

## 2.69.1
- Resumo da saída: o "Espectro da saída" compara em casa × fora de casa.
  Cada isótopo tem duas barras (fora em rosa, casa em branco), com as
  contagens por minuto só no pico, e diz se ficou mais, menos ou igual a
  casa (só muda acima de 3σ). Antes a conta usava a janela inteira, com o
  contínuo, e um pico que subiu podia aparecer com "-20%".
- A frase de baixo cita só os isótopos que subiram. No PDF entra também
  "Menos que em casa".
- Detalhe do isótopo: o "Pode ser também" vira uma ressalva neutra (•)
  quando a identidade já foi confirmada, e o cartão diz "Sim, junto com X"
  quando o vizinho também foi achado (os dois devem somar no pico).

## 2.69.0
- Isótopos explicados na aba Radiacode 102 › Espectro (spec ux/isotopos.html):
  - a etiqueta mostra o que o isótopo emite (α β γ rX), com o que fez o pico
    em laranja e uma dica em cada símbolo;
  - "certo" agora quer dizer duas coisas: o pico é forte (10 σ) e a energia
    é mesmo desse isótopo (vizinhos descartados ou outro pico da mesma
    família achado). Senão, fica "provável";
  - clicar no isótopo abre "Por que X": o trecho com a mesma base que o app
    usou para achar o pico, o que sobra acima dela, os números da detecção e
    a lista do que foi conferido;
  - "De onde vem", recolhido: nome por extenso, natural ou artificial,
    meia-vida e onde costuma aparecer. "Outros da série" mostra o que mais
    da família apareceu;
  - "Como funciona" do espectro explica pico, σ e os selos.
- Corrigido: no Acumulado do aparelho o zoom do Pb-212 dizia "Sem pico claro"
  e listava o próprio Pb-212 como "outro pico". Agora usa o contínuo da
  detecção (o SNIP) em vez de uma reta.
- Corrigido: os nomes dos picos não ficam mais um em cima do outro no
  gráfico do espectro e no zoom de um trecho arrastado. Num trecho largo, o
  zoom pede para arrastar em volta de um pico só.
- O cursor no espectro diz de onde vêm os picos que não são isótopo (raio X
  do chumbo e do tungstênio, aniquilação em 511 keV).

## 2.68.5
- O mrem/h agora aparece também no RC102: pequeno embaixo do mostrador no
  Agora (e no topo das outras telas) e na leitura da aba do Radiacode 102,
  igual ao GMC-800. Só quando a unidade é µSv/h.

## 2.68.4
- Configurações sem buracos: em cada aba os cartões ficam em duas colunas
  que empilham sozinhas, e o último cartão de cada coluna estica até o fim,
  então as duas terminam juntas. Os cartões foram rearrumados por seção
  (spec ux/config-colunas.html). Abaixo de 1100 px vira uma coluna só.
- O cartão "Dados" virou "Pasta dos dados".

## 2.68.3
- README: a logo do título não troca mais pela preferência do sistema, e sim
  pelo tema do GitHub. No tema claro com o Windows escuro, aparecia a versão
  clara da logo no fundo branco.
- PDF: o período no subtítulo usa "–" entre as datas, que serve nas duas
  línguas (no PDF em inglês aparecia "a").
- Saiu o docs/ROADMAP.md, parado desde a 1.31: o planejamento fica no
  quadro do projeto.

## 2.68.2
- README refeito para o público: em inglês (README.md) e em português
  (README.pt-BR.md), com a logo no título, o que entrou da 2.57 à 2.68, a
  dose, o PDF novo e prints do modo demonstração nas duas línguas.

## 2.68.1
- PDF: saída de casa levando só o GMC-800 (sem o RC102) agora aparece em
  "Saídas de casa", com a média e o pico do GMC-800, e na faixa lilás do
  gráfico. A tabela ganhou a coluna "Levou".
- PDF: "Fundo da casa" (página 1 e "Em resumo") e a coluna "Casa" do Por
  dia usam só os minutos com o GMC-800 em casa.
- PDF: sem dados do RC102, o título do gráfico fica só "GMC-800 (casa)".
- PDF: no A1, "Pico ao vivo (60 s)" não passa mais por cima do valor.

## 2.68.0
- Relatório PDF novo (spec aprovada pelo ACE). Ao clicar em "Relatório
  (PDF)" o app pergunta: "Completo" ou "Só o resumo", incluir as anotações e
  incluir o anexo das configurações vigentes. Lembra a última escolha.
- Página 1, o resumo: a frase do período, 4 números (sua dose com o
  veredito Normal / Acima do normal / Alto comparado a ficar em casa e o
  ritmo em mSv/ano, fundo da casa, pico e alertas), um gráfico com o
  GMC-800 e o RC102 e as saídas de casa em lilás, a fita dos dias e "Em
  resumo".
- Página 2: por dia (casa, sua dose, pico e alertas), os alertas dos dois
  aparelhos com a confirmação do outro, as saídas de casa, as medições de
  objeto, a chuva e as anotações.
- Página 3: o espectro do RC102 no período contra o de casa e um de cada
  saída.
- Anexo técnico com sumário (A1 a A8): tudo o que o relatório tinha antes.
  O rodapé diz "página N de M".
- Resumo da saída: sem os minutos de dose do RC102, o painel avisa isso sem
  contradizer o espectro e o estado do aparelho.
- Resumo: o fim do dia aparece como 00:00 do dia seguinte (em inglês saía
  metade traduzida).

## 2.67.0
- Resumo da saída: quando uma saída com o RC102 fecha ("Voltei" ou "Voltar
  ao normal"), aparece o aviso "Saída fechada" com "Ver o resumo da saída".
  O resumo junta o que o RC102 viu fora: dose (com horas do fundo de casa e
  raio-X de tórax), média comparada com o fundo de casa do dia, pico,
  alertas, o gráfico minuto a minuto, o espectro da saída inteira contra o de
  casa no mesmo tempo (com as linhas que subiram mais que em casa) e como o
  aparelho estava (bateria, temperatura e avisos).
- O título é o nome da saída em Fora de casa, editável ali mesmo pelo lápis;
  com o nome padrão, aparece "Saída de ter 10/11".
- Resumo: uma faixa lilás para cada saída do período, logo abaixo da frase,
  com dose, média e pico e o botão "Abrir resumo da saída". Em Fora de casa,
  o botão "Resumo da saída" abre o mesmo painel.
- Resumo: o fim do dia aparece como 24:00 em vez de 00:00.

## 2.66.1
- Fora de casa: cada trecho mostra só os aparelhos que foram junto (o que
  ficou em casa não aparece nos chips nem nos números).
- Fora de casa: os números do GMC-800 agora estão em µSv/h, com a
  calibração de cada minuto, como no relatório.
- Fora de casa: aparelho marcado como "comigo" com menos de 10% do trecho
  medido mostra "quase sem leitura nesse trecho" e o botão "Não estava
  comigo", que tira o aparelho do cálculo naquele trecho.

## 2.66.0
- Fora de casa: com um "Saí de casa" aberto e o RC102 junto, quando o RC102
  volta a conectar no PC depois do tempo mínimo sem conexão (15 min por
  padrão), o Agora pergunta "Voltou para casa?". "Voltei" fecha o trecho na
  hora em que ele reconectou (dá para ajustar o horário); "Ainda estou fora"
  deixa o trecho aberto. Dá para desfazer.

## 2.65.2
- Tendência do fundo: o gráfico "Fundo ao longo do tempo" e o texto da
  tendência agora estão em µSv/h, pela calibração de agora, como o resto do
  GMC-800. A dispersão continua em contagens, porque é técnica.

## 2.65.1
- Fundo por dia: o µSv/h de todos os dias usa a calibração de agora. Antes
  cada minuto usava a da época, e a troca de calibração de 10/11 parecia
  uma queda de 45% no fundo.
- Alertas: a contagem diz "+ 1 confirmado" no singular.

## 2.65.0
- Alertas repaginado: uma lista só, separada por dia, com o detalhe do
  alerta escolhido ao lado (pico, duração e limite em destaque, gráfico,
  conferência com o outro aparelho, "Por que subiu?" e "Ver nos gráficos").
  Em cima fica só o que está acontecendo agora; os filtros ficam numa linha
  e a explicação foi para "Como funciona".
- Alertas: no filtro de um aparelho entram também os alertas do outro que
  ele confirmou, um pouco mais apagados ("do GMC-800 · o RC102 confirmou" e
  o contrário). Se o alerta aberto sai da lista, abre o primeiro; se a lista
  fica vazia, o detalhe diz "Nenhum alerta neste filtro".
- GQ GMC-800 ganha abas, Agora e Saúde do tubo, como a do Radiacode 102.
- O fundo diário virou "Fundo por dia" em Gráficos › Tendência do fundo:
  µSv/h antes do CPM, 14 dias e "Ver mais".
- Na tela de um aparelho, fora da aba Agora, a fileira de cima mostra os
  dois aparelhos e os Alertas. Clicar no quadro do aparelho da página volta
  para a aba Agora.

## 2.64.2
- Histórico de alertas: quando o filtro esvazia a lista (ex.: RC102 sem
  alertas), o detalhe do alerta escolhido antes some junto.
- Histórico de alertas: no filtro RC102 entram também os alertas do GMC-800
  que o RC102 confirmou, com a marca "GMC-800".
- ALARA: a dose em coisas conhecidas usa o raio-X de tórax no lugar da
  banana, como no Resumo.
- Quadro dos aparelhos: sai a frase "no nível de fundo habitual" ao lado do
  "Seguro", e a fita do pulso perde as legendas "Os últimos 10 segundos" e
  "cada risco é uma contagem".
- Quadro do GMC-800: o valor em mrem/h aparece pequeno embaixo do mostrador.
- Resumo: voltar da comparação de dois dias não deixa mais a tabela e o
  gráfico da comparação na tela.
- ALARA: "Por que isso importa" agora abre e fecha, e as explicações mais
  longas foram para "Como funciona", como na aba do Radiacode 102.

## 2.64.1
- "O RC102 estava fora de casa?": agora pergunta também depois de viagens
  longas, de até 40 dias sem conexão (antes, só até 14 dias). É mais ou menos
  o que a memória do aparelho guarda (~1000 h). Vale também com o PC
  desligado o tempo todo.
- Espectrograma do Radiacode 102: a lupa 4×4 saiu; ficam "sem lupa" e a
  lupa 5×5 (quem estava com a 4×4 passa para a 5×5).
- Comparação do Resumo (achados da QA): dia no futuro não quebra mais o
  gráfico; sem porcentagem na dose quando os tempos medidos são muito
  diferentes; subtítulo, "(igual)" e "nenhum" traduzidos no inglês.
- "Ver os alertas do período" já leva até o filtro do período.

## 2.64.0
- Resumo mais enxuto: uma frase, três números e um gráfico na primeira vista.
- Período por Dia, Semana ou Mês, com as setas ‹ › para andar ("Hoje, qua
  11/11", "Esta semana, 09–15/11", "Novembro"). Clicar na data abre o
  calendário. "Comparar" virou um botão ao lado do PDF.
- Sua dose em coisas conhecidas: as horas do fundo da casa e o raio-X de
  tórax ("≈ 13% de um raio-X de tórax"). A banana continua na régua.
- Quando você saiu de casa, uma barrinha embaixo da dose mostra quanto foi em
  casa e quanto foi fora.
- Um gráfico só no lugar da fita + gráfico: a cor do nível vira uma faixa fina
  embaixo, os alertas e as anotações ficam em cima e o tempo fora de casa
  pinta o fundo de lilás. A legenda mostra só o que tem no período.
- "Mais números" (fechado, lembra o último estado): Mínimo, Medido, Dispersão,
  Chuva, um link para os alertas do período, lacunas e anotações. Média,
  Máximo e Dose saíram, porque repetiam os números de cima.
- "Histórico disponível" foi para Configurações › Dados. O PDF não muda.

## 2.63.0
- "Minha dose hoje" do Radiacode 102: o número grande continua sendo o
  contador do próprio aparelho, e agora a soma que o app fez com os minutos
  gravados aparece pequena ao lado ("app 0,52"), para conferir. Ela costuma
  ficar um pouco abaixo, porque o app não vê o tempo longe do PC.
- Isótopos no "Acumulado do aparelho": o retroespalhamento de uma fonte
  forte (o morro largo em 184 keV do Cs-137, 218 do K-40) não vira mais
  U-235 ou Pb-212 falso. Uma linha nesse ponto só fica se tiver pelo menos
  30% das contagens do pico que a explicaria.

## 2.62.1
- Isótopos no "Acumulado do aparelho": além de passar do contínuo, o pico
  tem que subir acima das duas janelas vizinhas. Assim, o platô Compton de
  uma fonte (Cs-137, ou o K-40 do sal light) não vira rádio, tório ou urânio
  que não estão lá.
- No "Zoom no pico" (e no cursor), o nome do elemento aparece sempre junto
  da origem: "bismuto · filho do radônio", "tálio · série do tório", como o
  K-40 já aparecia como potássio.

## 2.62.0
- Espectro do Radiacode 102: o "Acumulado do aparelho" agora mostra os
  isótopos. Como ele não tem fundo para subtrair e pode ter um contínuo
  enorme (como o beta do teste com tório), o app tira esse contínuo com o
  SNIP, técnica padrão de espectroscopia, e só marca o que sobra como pico
  (pelo menos 5σ e 1,5% acima do contínuo, acima de 100 keV).
- No "Acumulado do aparelho" o botão Pausar também sai: é o total do
  aparelho, e ele continua se atualizando sozinho.

## 2.61.2
- Isótopos do Radiacode 102: o texto ao passar o mouse diz de onde o
  isótopo vem, por exemplo "vem do Rádio-226 (série do urânio)" no Pb-214.

## 2.61.1
- Espectro do Radiacode 102: sai o "Zerar o acumulado…". O "Acumulado do
  aparelho" volta a mostrar o valor real que o RC102 guarda, sem zerar no
  app (o zerado no RadLog de antes deixa de valer).

## 2.61.0
- Espectro do Radiacode 102: o "Recomeçar" agora zera o espectro da tela do
  próprio aparelho (igual a segurar ▼ ou ao Restart do app oficial), e a
  opção "Tela do aparelho" (antes "Desde o Recomeçar") mostra exatamente o
  que está na tela do RC102. As contagens não se perdem: vão para o
  Acumulado do aparelho.
- "Zerar o acumulado…" zera direto no RadLog. O firmware do aparelho não tem
  comando que apague o espectro acumulado, e o "Recomeçar" soma o espectro da
  tela dentro dele. A tentativa da 2.60.1 zerava de tabela o espectro da tela;
  não faz mais isso. O aviso agora diz "Zerado no RadLog (o RC102 não deixa
  apagar o dele)".

## 2.60.3
- Aba do Radiacode 102: os textos de explicação de todos os cartões
  (Espectrograma, Dureza, Fonte simulada, Atividade, Log) também ficam em
  "Como funciona", como no Espectro.
- Os botões dentro do Espectro passam a se chamar "Espectro" e
  "Espectrograma" (antes "Acumulado" e "Ao longo do tempo").

## 2.60.2
- Aba Espectro do Radiacode 102: o texto grande de explicação foi para
  "Como funciona", que abre ao clicar, como nas Configurações.
- O texto passa a dizer que "Desde o Recomeçar" só marca isótopos acima
  do fundo da casa; os do próprio fundo aparecem no "Acumulado do
  aparelho" depois de algumas horas.

## 2.60.1
- "Zerar o acumulado…": no teste com o aparelho real, o RC102 recusou o
  primeiro jeito de zerar. Agora o app tenta mais dois (pelo SPEC_RESET do
  aparelho) e só dá por zerado se o tempo do acumulado lido depois caiu.
  O log diz qual jeito funcionou. Se nenhum funcionar, continua zerando
  só no app, como na 2.60.0.

## 2.60.0
- **Zerar o espectro acumulado do RC102.** O Radiacode 102 guarda dois
  espectros: o atual, que aparece na tela e zera pelo menu, e o acumulado,
  de longo prazo, que ele não mostra nem deixa zerar. No modo "Acumulado
  do aparelho" da aba Espectro, o botão Recomeçar vira "Zerar o
  acumulado…" e manda o comando ao aparelho, com confirmação. Se o
  aparelho recusar, o app zera só na tela: guarda uma foto daquela hora e
  mostra só o que veio depois.
- O acumulado é relido a cada 30 s nesse modo, e na hora em que você entra
  nele. Antes era relido a cada 10 min, então o tempo ficava parado e um
  reset demorava a aparecer.
- Sem isótopo achado, a dica embaixo do espectro fala só do arrastar.
- O texto da aba Espectro explica o morro grande abaixo de uns 200 keV:
  são fótons espalhados, não um isótopo.

## 2.59.0
- **Confirmação cruzada.** Quando o GQ GMC-800 ou o Radiacode 102 entra em
  atenção ou alarme e os dois estão no mesmo lugar (os dois em casa ou os
  dois com você), o app confere no outro aparelho. A comparação é pela
  subida sobre o fundo de cada um, em %, e não pelo µSv/h, então a
  calibração não interfere.
- Uma linha nova na faixa diz o resultado, atualizada a cada 5 s:
  - "Confirmado pelo RC102": o outro subiu acima de 3σ e pelo menos 40% do
    que o primeiro subiu;
  - "Conferindo": o outro ainda não juntou contagens para ver uma subida
    desse tamanho, com o tempo que falta (no máximo 15 min);
  - "não viu": já daria para ver com folga e o outro ficou abaixo de 2σ.
    Na faixa de atenção, aparece também o botão "Por que subiu?";
  - "Sem como conferir": os aparelhos estão em lugares diferentes, o outro
    está desconectado ou tem uma medição de objeto em andamento.
- No perigo a faixa não ganha a linha, mas o resultado fica no alerta. Se o
  outro aparelho também passou do limite, continua a linha "Também" de
  sempre. A confirmação nunca cala nem atrasa o alarme.
- Em Alertas, cada alerta ganhou um selo com o resultado ("confirmado pelo
  RC102", "só no GMC-800"...). O alerta aberto mostra um gráfico de 10 min
  antes a 5 min depois, com a subida dos dois aparelhos sobre o fundo de
  cada um.
- O "Por que subiu?" usa o mesmo resultado na causa "Os dois aparelhos
  viram".

## 2.58.2
- A mediana do fundo do GQ GMC-800 agora é interpolada. Com umas 20
  contagens por minuto, a mediana comum só andava de 1 em 1 contagem, em
  degraus de ~5%, e era esse arredondamento que decidia os limiares de +10%
  e +5% da subida lenta do fundo. A mediana nova anda suave e continua
  resistente a picos. Ela entra em três lugares:
  - na subida lenta do fundo (mediana de cada dia);
  - no Calendário do ano (cor de cada dia em Fundo de casa);
  - no "Por que subiu?" (o normal e a mediana de cada hora). Antes, o
    "subiu X%" já saía uns 2% acima.

## 2.58.1
- Tela do GQ GMC-800 a 130% e 150% (Configurações › Janela) com a janela
  estreita: o cartão grande não passa mais da largura. A "Dose hoje" desce
  para baixo do gráfico da última hora quando não cabe ao lado.
- "Por que subiu?": na linha da pressão, quando ela não apoia, o texto
  mostra a variação real nas 12 horas, com o sentido ("subiu", "caiu" ou
  "ficou estável"). Antes, com a pressão subindo, aparecia "mudou 0,0 hPa".
- Saúde do tubo: a hora que se repete nas rajadas não troca mais entre um
  recálculo e outro quando duas horas empatam (fica a menor).

## 2.58.0
- **Subida lenta do fundo.** Cartão novo "Fundo nas últimas semanas" no
  topo de Gráficos › Tendência do fundo. Ele compara a mediana das
  últimas 2 semanas com a dos meses antes (120 a 30 dias atrás) e avisa
  quando o fundo de casa subiu de verdade, coisa que o alarme não pega
  porque nenhum dia parece diferente do anterior.
- Só entram dias com o GMC-800 em casa, pelo menos 12 h medidas, sem chuva
  no dia, sem medir objeto e sem minutos de alarme ou interferência. A conta
  é em contagens: trocar a calibração não vira subida.
- "Vale olhar" quando as 2 semanas ficam 10% ou mais acima da referência e
  10 dos últimos 14 dias passam de +5%. O cartão diz se foi o ambiente ou o
  aparelho conferindo com a contagem do RC102 nos minutos com os dois em
  casa, e separa a subida aos poucos de um degrau (pulo de uma semana para a
  outra que fica).
- Gráfico de 120 dias em % da referência: pontos por dia e mediana de 7 dias
  do GMC-800, mediana do RC102, dias de chuva em círculo azul, faixa de ±5%
  e linha de +10%. Passar o mouse mostra o dia em µSv/h.
- "O que conferir" aparece quando dá "vale olhar" ou degrau, e um aviso
  informativo fica no topo de Alertas enquanto durar (não é alarme, some
  sozinho e não vai para o histórico).
- "Mudei o aparelho de lugar" entrou no menu do logo e na busca (Ctrl+K), e
  como atalho no cartão quando dá degrau: a referência recomeça nesse dia e
  fica uma nota no gráfico. "Desfazer a marca" volta a referência antiga.
- Recalcula uma vez por dia, depois das 06:00, junto da Saúde do tubo.

## 2.57.0
- **Saúde do tubo.** Cartão novo na tela do GQ GMC-800 que confere, com o
  que o app já gravou, se o tubo Geiger está contando direito. São quatro
  conferências:
  - conta sem falhar (nenhum trecho de 2 min ou mais em 0 CPM em 30 dias);
  - varia só pelo acaso (dispersão de cada dia até 1,2, com o histograma
    dos minutos contra a curva de Poisson);
  - sem rajadas estranhas (até 3 em 30 dias);
  - sensibilidade estável (razão de contagens RC102 ÷ GMC-800 com os dois
    em casa dentro de ±5% em 90 dias).
- Cada conferência diz "tudo certo", "vale olhar" ou "sem dado", sem usar
  os níveis de radiação, e explica o que mede. O cartão recalcula uma vez
  por dia, depois das 06:00, ou no botão "Conferir agora". Nada é escrito
  no aparelho.
- Enquanto alguma conferência estiver em "vale olhar", aparece um aviso
  informativo no topo de Alertas. Ele não é alarme, some sozinho quando
  volta a "tudo certo" e não vai para o histórico.
- Em Configurações › GQ GMC-800 › Testar o contador, uma linha resume a
  saúde do tubo.
- **Fita dos últimos 10 segundos mais estável.**
  - Quando 3 ou 4 contagens caíam juntas por acaso, a fita trocava de jeito
    de desenhar (riscos de alturas diferentes × todos cheios). Agora a
    altura só passa a contar com taxa alta de verdade.
  - As contagens do mesmo quadro saem espalhadas no tempo, e não mais
    grudadas num ponto só.
  - Depois que o Windows segura a animação (por exemplo com a ferramenta
    de captura aberta), o RC102 repõe o trecho em vez de deixar um buraco.

## 2.56.2
- **README novo.** A página do projeto no GitHub agora descreve o app como
  ele é hoje: os dois aparelhos, as telas, os alarmes, a atualização
  assinada, onde ficam os dados, como compilar e a estrutura do código.

## 2.56.1
- **Fita dos últimos 10 segundos andando suave.** Na 2.56.0 a fita andava
  aos saltos, de casa em casa, e parecia travada. Agora cada risco anda de
  pixel em pixel a cada quadro, sem perder a largura certa da 2.56.0.
- **Símbolo de perigo no Radiacode 102.** Com os alarmes do próprio
  aparelho na régua, o mostrador e o cartão do RC102 paravam em "Alarme 2"
  (losango) mesmo acima do limite de perigo, enquanto a faixa do topo e os
  alertas já diziam perigo. Agora os três concordam.

## 2.56.0
- **Fita dos últimos 10 segundos sem riscos tortos.** Com a escala de 125%
  do Windows, a fita ficava com meio pixel esticado e os riscos saíam com
  larguras e alturas diferentes. Agora o desenho bate pixel com pixel com
  a tela, cada risco anda de casa em casa sem mudar de largura e todos têm
  a mesma altura (só numa taxa alta, com várias contagens na mesma casa,
  a altura passa a contar quantas foram).
- **Escolher o aparelho do logo.** Em Configurações › Janela e tela ›
  Pulso da contagem, "O logo pisca com": o mesmo aparelho do ícone da
  bandeja (como antes), GQ GMC-800, Radiacode 102 ou nenhum.

## 2.55.0
- **"Por que subiu?"** Quando a leitura sobe, um painel lateral confere as
  causas comuns com os seus próprios dados. Para cada causa ele diz se os
  dados **apoiam**, **não apoiam** ou se está **sem dado**, mostra a
  evidência e explica por que aquilo faz subir. As causas conferidas:
  - acaso (a subida em σ);
  - os dois aparelhos (o outro subiu junto?);
  - chuva nas 4 h antes;
  - radônio da hora do dia;
  - pressão caindo;
  - objeto ou anotação perto;
  - saída de casa;
  - coisa do aparelho.
- **No topo**, uma frase com o mais provável. Embaixo, dois gráficos
  empilhados das 12 h antes, cada um com a sua escala (nunca menor que ±25%
  do normal), com a chuva, fora de casa e notas marcadas.
- **Onde fica o botão:**
  - na faixa de alarme, ao lado do Silenciar;
  - em cada alerta da lista (olha a hora daquele alerta);
  - no Agora, quando o fundo está acima do normal ("Por que?").
- Não é diagnóstico: é um atalho para não cruzar quatro telas.

## 2.54.0
- **Pulso da contagem.** O anel do meio do mostrador pisca de leve a cada
  contagem do aparelho, como o clique de um Geiger, só que sem som. Uma
  piscada é sempre uma contagem.
  - Com o fundo de casa, o GMC-800 dá uma piscada a cada 3 segundos, mais
    ou menos. Numa subida, as piscadas aceleram.
  - Quanto mais contagem, mais curta a piscada: de 0,4 s até 70 ms.
- **O logo pisca junto,** com o aparelho do ícone da bandeja. Só o anel
  pisca; o número e o resto da tela ficam parados.
- **De onde vêm as contagens:**
  - no GMC-800 elas vêm de verdade do GETCPS, e só o instante dentro do
    segundo é sorteado;
  - o RC102 manda a taxa, então as piscadas são sorteadas a partir dela.
- **Fita "Os últimos 10 segundos"** (opcional): um risco por contagem,
  embaixo do cartão de cada aparelho no Agora e na tela do aparelho.
- **Configurações › Janela e tela › Pulso da contagem:**
  - "Piscar o mostrador a cada contagem" (vem ligado);
  - "Mostrar a fita dos últimos 10 segundos" (vem desligado; só vale com o
    pulso ligado).

  O pulso desliga sozinho com o Windows sem efeitos de animação e com a
  janela escondida na bandeja.

## 2.53.0
- **Calendário do ano** (Gráficos › Ano). Mostra o ano inteiro numa tela,
  um quadradinho por dia, pintado pelo dia comparado com a **sua média do
  ano**: do mais escuro (bem abaixo) ao verde forte (bem acima).
- **Fundo de casa ou Minha dose** no mesmo botão:
  - o fundo é a mediana dos minutos em casa do GMC-800;
  - a dose é a do RC102 no dia, com borda lilás nos dias em que ele saiu de
    casa.
- **Marcas nos dias:**
  - a forma do nível mais grave nos dias com alarme;
  - uma risquinha azul nos dias de chuva;
  - vazio tracejado nos dias sem dado;
  - cor mais fraca com menos de 12 h de dado.
- **Ao passar o mouse**, aparece o dia em texto, com o pico do alarme em
  µSv/h e a hora. **Clicar** fixa o dia embaixo, com "Abrir este dia nos
  gráficos". Pelo teclado, as setas andam entre os dias e Enter abre.
- **Embaixo do calendário:**
  - "Mês a mês", com as medianas;
  - "O que o ano mostrou", com frases tiradas dos seus dados (meses frios ×
    quentes, chuva × seco, o dia mais alto);
  - a tabela por mês.
- O resumo de cada dia é guardado por mês e só é refeito quando algo
  daquele mês muda. A chuva por dia já é guardada para sempre desde a
  2.43.1.

## 2.52.0
- **Aba do Radiacode 102 repaginada (2ª parte).** Em cima do espectro
  aparece uma fileira com os **isótopos mais detectados**. Ela vai do mais
  forte para o mais fraco, mostra as linhas achadas e tem um selo de certeza
  (certo, provável, fraco). Cada um fica marcado no gráfico. Também vale no
  acumulado do aparelho.
- **Zoom na crista:**
  - Clicar num isótopo, ou arrastar no espectro, abre os canais daquele
    trecho com a linha de base e a curva ajustada.
  - Ao lado ficam a crista em keV, a largura (FWHM e resolução em %), as
    contagens do pico com a incerteza e quantos σ acima do fundo.
  - Avisa quando tem outro pico perto.
- **Lupa no espectrograma:** passando o mouse, aparece um quadrado de 5×5
  ou 4×4 pixels ampliado. Ele mostra as contagens de cada pixel, o tempo e
  a faixa de energia do pixel do meio.

## 2.51.0
- **Aba do Radiacode 102 repaginada (1ª parte).** São 5 subabas: Agora,
  Espectro, Busca, Atividade (Bq) e Log.
- **Agora:**
  - A leitura aparece uma vez só, em µSv/h; CPM e cps ficam na linha de baixo.
  - Conexão, onde ele está, bateria e temperatura viram etiquetas ao lado do
    nome.
  - A dose de hoje mostra quanto tempo ele passou fora de casa.
  - A dose somada no aparelho entra junto da média e do mínimo e máximo de
    24 h.
  - Um gráfico novo mostra as últimas 6 h, 24 h ou 7 dias, com os trechos
    fora de casa.
  - "Procurar de novo" e "Configurar conexão" viram um botão só, "Conexão".
- **Espectro:** tem "Acumulado" e "Ao longo do tempo". Este segundo junta o
  espectrograma (com a cor de sempre) e a dureza, agora em dois gráficos
  empilhados: contagem em cima e dureza embaixo, sem dois eixos.
- Saem os textos "Para você" e os números dos blocos.
- A calibração da Atividade vira uma tabela pequena.
- "Lado a lado com o RC102" mudou para Configurações › GQ GMC-800 ›
  Calibração.

## 2.50.0
- **A barra compacta volta a ser como antes da 2.45.0.** Os quadros grandes
  aparecem de novo em todas as telas, e a faixa de cima só surge ao rolar a
  página, com os dois aparelhos, o alerta e o "voltar ao topo". O resto que
  veio depois (bandeja, busca, tamanho da janela) continua igual.

## 2.49.0
- **Tamanho e espaço da janela.** Em Configurações › Janela e tela dá para
  escolher o tamanho de tudo (90%, 100%, 108%, 115%, 130% ou 150%: texto,
  números e gráficos juntos) e o espaço entre as coisas (confortável, normal
  ou compacto). A mudança aparece na hora e fica guardada neste PC.
- Atalhos: Ctrl + e Ctrl − mudam o tamanho, e Ctrl 0 volta a 100%.

## 2.48.0
- **Busca rápida (Ctrl+K).** Abre em qualquer tela, ou pelo "Buscar" no
  menu. Acha ações, telas, cartões das Configurações e medições salvas, sem
  ligar para acentos e também por apelido ("som" acha o Som do alarme, "pdf"
  acha o relatório). Digitar um horário, como "14:30", abre os gráficos de
  hoje nesse ponto. As setas escolhem, Enter abre e Esc fecha.
- **Botão do logo no canto.** Um clique abre três ações: anotar agora, saí
  de casa / voltei e medir um objeto. "Saí de casa" pergunta quem vai junto,
  já marcado como da última vez. O botão some durante uma medição de objeto
  e com o alarme tocando.
- **Atalho N** para anotar agora, quando o foco não está num campo de texto.

## 2.47.0
- **A leitura no ícone da bandeja.** Com a janela fechada, o ícone perto do
  relógio mostra o número em µSv/h, em até 3 caracteres (,12 · 1,4 · 48 ·
  1k). Quando está tudo seguro, o número sai na cor do aparelho (verde do
  GMC-800, rosa do RC102). Em atenção, alarme e perigo, o quadradinho ganha
  a cor do nível. Saturado mostra MAX. Sem contagens mostra 0 em laranja.
  Conectando aparece como um anel tracejado e desconectado como um tracinho
  cinza. O ícone é desenhado pixel a pixel em 16, 20, 24 e 32 px, para
  ficar nítido em qualquer escala da tela.
- No automático, o ícone mostra o GMC-800, a não ser que o RC102 esteja num
  nível mais grave. Em Configurações › Janela dá para fixar o aparelho ou
  voltar ao logo de antes.
- A dica do ícone agora fica em µSv/h (antes estava em CPM) e mostra os dois
  aparelhos, uma linha cada.

## 2.46.0
- **Medir objeto não vira dose sua.** Quando o RC102 está encostado num
  objeto em Medir objeto, aqueles minutos passam a contar só como fundo na
  Minha dose, na fita do dia, no ALARA e no total do aparelho. Antes, a
  leitura alta do objeto entrava como se fosse dose recebida por você.
- **Pressão × radônio mais rigorosa.** O app tira o ciclo do dia do fundo e
  da pressão antes de comparar, porque os dois sobem e descem com a hora e
  isso fabricava relação. Depois faz um teste de acaso: desloca o fundo no
  tempo várias vezes e vê quantas vezes uma relação daquele tamanho aparece
  sozinha. Só chama de relação com 28 dias ou mais e passando no teste; se
  não passar, diz que pode ser coincidência.
- **ALARA: Tempo mostra o que passa do fundo.** O número grande agora é a
  dose a mais por ficar no lugar, e a linha de baixo mostra o total e quanto
  disso é fundo. As contas de 2× e 4× da distância também mostram só o extra.

## 2.45.0
- **Barra compacta.** Os quadros grandes dos aparelhos ficam só no Agora.
  Nas outras telas, uma faixa fina fica presa no topo desde o começo, com a
  leitura dos dois aparelhos, o nível e a última hora em miniatura. Clicar
  num aparelho abre a página dele.
- A faixa mostra sempre os alertas: "Sem alertas" quando está tudo calmo,
  ou o nível, o aparelho e há quanto tempo. Clicar leva a Alertas.
- Quando a memória do GMC-800 está perto de encher ou o relógio dele está
  fora, aparece um aviso na faixa que leva à página do contador.
- Se um aparelho não está no lugar de sempre, a faixa mostra onde ele está.
- Em janelas estreitas a faixa encurta os nomes (GMC, RC) e os avisos.

## 2.44.1
- **Nada do lugar no backup.** Na 2.44.0, a coordenada da chuva virava uma
  marca no backup. Só que as cidades possíveis são poucas, então dava para
  testar todas e descobrir qual era. Agora o backup não leva nada do lugar:
  nem coordenada, nem marca. Ao importar, o app reconhece se a chuva é da
  mesma cidade comparando a pressão das horas que os dois já têm, porque a
  pressão na superfície muda de um lugar para outro. Backups antigos
  continuam sendo reconhecidos como antes.
- A mensagem de erro do radar e do serviço de clima não guarda mais o
  endereço pedido, que trazia a coordenada ou o pedaço do mapa. Antes ela ia
  junto no backup quando o radar falhava.
- Clima: o gráfico da semana converte cada hora pela calibração que valia
  naquela hora, como os Gráficos. Antes, as horas anteriores à troca do
  fator saíam com o fator novo.
- ALARA: "De onde vem a sua dose" diz "pela leitura de casa (GMC-800)"
  quando a dose vem do GMC-800, em vez de "GMC-800 com você". No card Tempo,
  as doses a 2× e 4× da distância saem com as mesmas casas que o número
  grande. A dica "Abrir nos gráficos" das linhas da tabela agora é
  traduzida no inglês.

## 2.44.0
- **A tela Chuva virou "Clima"** e ganhou **Pressão × radônio**. São dois
  gráficos empilhados, com o mesmo cursor: a pressão do ar e o fundo em
  casa (mediana de 1 h, fora de alertas) dos últimos 7 dias. Embaixo vem
  uma frase que diz se o fundo sobe quando a pressão cai e quantas horas
  depois. A conta é a correlação com 0 a 12 h de atraso, sobre até 90
  dias. Com menos de 28 dias o resultado aparece como "pista", e a partir
  daí como "relação". Funciona com o GMC-800 ou com o RC102, pelo mesmo
  seletor da chuva.
- A pressão vem do mesmo serviço da chuva (Open-Meteo, sem chave) e fica
  guardada por um ano. Na primeira busca o app pega os últimos 92 dias.
- **O backup não leva mais o lugar.** A latitude e a longitude da chuva e
  do radar viram uma marca que não dá para reverter. Ela só serve para
  saber, ao importar, se a chuva é do mesmo lugar. Backups antigos
  continuam entrando.

## 2.43.1
- A chuva de cada dia passa a ficar guardada para sempre, em
  chuva-dias.json (só a data e os mm, sem o lugar). A chuva por hora
  continua só 60 dias. É a base do calendário do ano, que vem depois.
  Entra no backup.

## 2.43.0
- **Tela nova "ALARA"** em Análise (ícone de escudo). ALARA é "tão baixo
  quanto razoavelmente possível": a tela mostra de onde vem a sua dose e
  deixa testar os três jeitos de baixar com os seus aparelhos.
- **De onde vem a sua dose**: os últimos 7 dias divididos em fundo em casa,
  fundo fora de casa e acima do fundo. Cada trecho acima do fundo ganha um
  nome (medição de objeto, alarme, saída de casa, anotação) e abre nos
  Gráficos com um clique.
- **Tempo**: taxa × minutos dá a dose, com a comparação com coisas
  conhecidas e o que sobraria ao dobro e ao quádruplo da distância.
- **Teste de distância**: mede a fonte em 5, 10, 20 e 40 cm (dá para mudar),
  cada ponto até a incerteza ficar abaixo de 10% (no máximo 5 min), tira o
  fundo da casa e ajusta a curva do inverso do quadrado. Diz se seguiu a
  regra, caiu mais devagar (fonte grande) ou não bateu, e avisa o ponto que
  sumiu no fundo.
- **Teste de blindagem**: sem nada × com o material no meio, com quanto
  cortou e a incerteza. Se a diferença for menor que duas vezes a
  incerteza, diz que não deu para ver.
- Os dois testes usam o motor do Medir objeto e ficam salvos em Medições
  salvas.

## 2.42.0
- **Fita do dia no Agora**, no lugar do gráfico "Hoje" (que continua nos
  Gráficos). O dia inteiro numa faixa, em pedaços de 10 minutos pintados
  pelo nível da média: uma linha para a casa e outra para você, cada uma
  pelo aparelho que estava nesse papel (onde está cada aparelho). Trecho
  acima do seguro ganha a forma do nível em cima. Sem dado fica cinza
  escuro, nunca verde.
- Embaixo vêm os trechos de fora de casa (lilás) e a chuva (azul). Em cima
  ficam as marcas: alertas com a forma do nível e anotações com o lápis.
  Passar o mouse mostra o trecho (µSv/h de casa e de você, e qual
  aparelho). Clicar abre os Gráficos centrados nele.
- "Semana" empilha os últimos 7 dias, um por linha, com o nível mais alto
  entre casa e você. Uma frase no topo resume o dia ou a semana.
- No Resumo a fita mostra o dia escolhido, ou os dias do período um embaixo
  do outro. No relatório em PDF entra a "Fita dos dias", logo depois do
  gráfico.

## 2.41.0
- **Dose em coisas conhecidas.** "2,9 µSv" não diz muito, então cada dose
  ganha duas linhas que traduzem o número. A primeira é quanto tempo do
  fundo da sua casa aquilo dá ("≈ 22 h do fundo da sua casa", "≈ 3
  semanas…"), que é a comparação mais honesta. A segunda é uma coisa
  conhecida da Régua de radiação que dê um número entre 1 e 30: bananas,
  horas de voo, raios-X ou tomografias de tórax. Se nenhuma der, não
  aparece. Nunca compara com acidentes, e o número fica com no máximo dois
  dígitos, porque é ordem de grandeza.
- Aparece nas células da Minha dose (hoje, semana, mês e ano), no cartão de
  dose dos Gráficos, no Resumo, na "dose extra" do Medir objeto com um
  aparelho com você e no relatório em PDF. Clicar abre a Régua de radiação
  na escada da dose.

## 2.40.1
- **Ícone da bandeja fica onde você deixou depois de atualizar.** O Windows
  lembra se o ícone fica fixo na barra ou escondido na setinha pelo caminho
  do .exe, e cada versão vinha com um nome novo (RadLog-vX.Y.Z.exe). Por
  isso, a cada atualização ele achava que era outro programa e escondia o
  ícone. Agora o atualizador põe a versão nova sempre como `RadLog.exe`, na
  pasta das versões (Documentos\RadLog\Versões). A anterior vira
  `RadLog.old.exe` e é apagada junto com as versões antigas.
- O mesmo vale para a pergunta do Firewall do Windows ao aceitar conexões
  da sincronização: a regra é pelo caminho do .exe, então ela para de
  aparecer a cada versão.
- Na primeira atualização depois desta, o caminho muda uma última vez (de
  RadLog-vX.Y.Z.exe para RadLog.exe). Então fixe o ícone e responda ao
  firewall mais uma vez. Daí em diante o caminho não muda mais.
## 2.40.0
- Medir objeto virou um caminho em cinco etapas: preparar, procurar o ponto
  mais forte, fundo, objeto e resultado. Na preparação ficam só o nome, a
  distância (em cm, com atalhos), o aparelho (quatro cartões) e o tamanho do
  objeto; fundo, tempo e o "tem algo" foram para "Ajustes da medição", com
  os mesmos padrões de antes.
- Procurar: leitura rápida (média de 5 s) do aparelho que fica no objeto,
  com a seta de subindo ou descendo, o fundo e o pico. Dá para pular.
- Durante a medição: os números (com o objeto, fundo, diferença líquida),
  a barra de certeza até 5σ e o botão "Parar e ver o resultado". Com o RC102,
  o espectro vai se formando por cima do fundo e o espectrograma mostra se
  algo mudou no meio. Com o GMC-800, o gráfico em µSv/h ao vivo.
- Resultado com uma frase no topo e quatro números (com o objeto, certeza,
  taxa extra e distância segura, onde o extra cai a 10% do fundo). Com o
  RC102 no objeto: isótopos prováveis com a confiança (quantas linhas da
  série bateram) e a atividade em Bq como ordem de grandeza (faixa ÷2 a ×2),
  só quando dá: isótopo identificado com constante conhecida, distância
  informada e objeto pequeno; senão o cartão mostra o que faltou. O
  espectro líquido (fundo descontado) com os picos marcados e uma tabela
  técnica (contagens, CPS e CPM, σ, conversão usada).
- Os dois no objeto: lado a lado, com "Concordam?". Um no objeto e outro com
  você: "O que chegou até você", e com o RC102 com você, o espectro do que
  chegou.
- "Medir em outra distância" repete só a etapa do objeto com o mesmo fundo.
  Com duas distâncias, o app confere se a radiação caiu pelo inverso do
  quadrado; se caiu bem menos, avisa que o objeto é espalhado, a distância
  segura fica aproximada e o Bq não sai.
- A tabela de linhas do espectro ganhou o urânio natural (Th-234 em 63 e
  93 keV, Pa-234m em 1001 keV). O relatório em PDF da medição traz a
  distância segura, o Bq estimado e a conta das duas distâncias quando houver.

## 2.39.0
- O app passa a ouvir o aviso de "fora da escala" do próprio Radiacode 102
  (taxa de dose ou contagem acima do que ele mede). Com o aviso, a faixa de
  alarme mostra "RC102 saturado · fora da escala do aparelho" e o "Onde está
  cada aparelho" mostra "saturando", mesmo abaixo do fim da escala ajustado
  em Configurações. O aviso vale até a leitura cair para menos da metade do
  valor em que ele chegou. O fim da escala do app continua valendo como
  segunda trava.

## 2.38.6
- Nomes dos aparelhos padronizados. Nos títulos (menu, quadros do Agora,
  cartão grande e título de cada aparelho, "Onde está cada aparelho",
  Configurações e o cabeçalho do relatório em PDF) aparece o nome completo:
  "GQ GMC-800" e "Radiacode 102". No meio de frases, alertas, legendas,
  tabelas e na barra compacta fica o abreviado: "GMC-800" e "RC102". A marca
  passa a ser escrita "Radiacode". Textos antigos já gravados com "RadiaCode"
  continuam traduzidos no inglês.

## 2.38.5
- No Resumo, o símbolo do nível no "Período normal." (e nos outros vereditos)
  ganhou espaço até o texto, como no resto do app.

## 2.38.4
- O cartão grande das telas GMC-800 e RadiaCode ganhou o gráfico da última
  hora, o mesmo do quadro do Agora, no espaço vazio entre a leitura e a
  "Dose hoje". No RadiaCode, a "Dose hoje" vai para o canto de cima, como no
  GMC-800.
- No Agora, a faixa "Tudo normal agora" some quando está tudo normal. Ela
  continua aparecendo com alerta ativo, sem aparelho lendo, quando falta
  medição ou com um aparelho fora do normal. O selo "fundo de casa" foi para
  o cabeçalho da Minha dose, de onde vem esse número.

## 2.38.3
- Cada item do menu lateral ganhou um ícone, no mesmo traço dos de baixo:
  mostrador no Agora, linha nos Gráficos, caderno no Resumo, sino nos
  Alertas, mira no Medir objeto, casa com seta no Fora de casa, nuvem na
  Chuva e os dois aparelhos. O ícone do item aberto fica na cor de destaque.
- As confirmações antes de gravar nos aparelhos saem da caixa do sistema
  ("wails.localhost says", com OK/Cancel em inglês) e viram uma janela do
  próprio app, com botões em português. Gravar no RadiaCode mostra uma tabela
  com cada ajuste, o valor de agora e o de depois. A calibração do GMC-800
  mostra os 6 pontos de hoje e os novos. Zerar a dose tem o botão em vermelho.
  O foco começa no Cancelar, e Esc também cancela.

## 2.38.2
- **Configurações: explicações recolhidas em "Como funciona".** Em 23
  cartões, o texto que só explica (como funciona, de onde vem o número,
  quando usar) fica fechado no pé do cartão; um clique abre e mostra o texto
  inteiro. Os campos sobem para o topo. Continuam abertos os avisos que
  mudam, os valores medidos, as notas de uma linha, a escada de alarmes e os
  avisos de "não tem volta". A busca continua achando o que está dentro.

## 2.38.1
- Na tela de um aparelho, a fileira de cima volta a ser a mesma do Agora:
  o quadro completo do outro aparelho (conexão, mostrador, nível, gráfico,
  CPM/CPS e dose de hoje) ao lado dos alertas, só sem o quadro do aparelho
  aberto. O cartão compacto da 2.37.1 saiu. Clicar no valor ou na conexão do
  quadro abre a tela do outro aparelho, como no Agora.

## 2.38.0
- **"O RadiaCode estava fora de casa?"** Quando o RadiaCode volta a conectar
  depois de ficar sem conexão pelo menos o tempo da pergunta de fora de casa
  (Configurações, padrão 15 min), o app espera a memória do aparelho chegar e
  pergunta num cartão no topo do Agora onde ele estava. O cartão mostra o que a
  memória trouxe (média, máximo e dose do trecho), o GMC-800 em casa no mesmo
  horário e uma dica pela razão de contagens entre os dois: longe da normal,
  "parece que ele saiu de casa"; perto, "parece que ele ficou em casa".
- Respostas: **Comigo, fora de casa** marca o trecho (GMC-800 em casa,
  RadiaCode com você, você fora); as leituras do RadiaCode saem do fundo da
  casa e do padrão por hora e continuam na sua dose. **Em casa** não marca
  nada (é o normal). **Não sei** some sem marcar. "Ajustar horário" muda a
  saída e a volta, porque parte do tempo sem conexão pode ter sido em casa. Dá
  para desfazer logo depois.
- Pergunta sem resposta fica no cartão e aparece em Fora de casa como "a
  responder". Trecho que já está marcado não é perguntado.
- Fora de casa: o trecho "GMC-800 em casa, RadiaCode comigo" agora pode ser
  marcado à mão com "eu não estava em casa (saí com o RadiaCode)".
- **Correção:** ao marcar "Fora de casa" num trecho que veio da memória do
  GMC-800, o app marcava os dois aparelhos como "com você". Agora ele pergunta
  onde estava o RadiaCode, já sugerindo "em casa" se ele estava conectado ao
  PC nesse trecho e "comigo" se não estava.
## 2.37.1
- Na tela de um aparelho, o cartão pequeno do outro aparelho usa o mesmo
  mostrador dos blocos do Agora: o anel com o valor e "µSv/h" dentro e o
  arco embaixo, com as mesmas cores, só que menor. O número que aparecia ao
  lado do mostrador saiu, para não repetir. Desconectado, o centro mostra um
  traço cinza, com "última … às HH:MM" ao lado; conectando, o anel fica
  tracejado em âmbar.

## 2.37.0
- **Calibrar o contador com a intercalibração.** No cartão "Pontos gravados
  no contador" (Configurações › GMC-800 › Calibração), o app mostra os seis
  pontos de hoje ao lado dos sugeridos com o fator em uso no app (por
  exemplo, 0,00564 µSv/h por CPM da intercalibração com o RadiaCode) e grava
  no GMC-800 com um botão. O µSv/h de cada ponto fica igual e o CPM é
  escalado, então a forma da curva de fábrica continua. Depois disso a tela
  do contador mostra o mesmo µSv/h que o app; no app nada muda.
- Toda gravação que muda a calibração do contador guarda duas cópias dos
  pontos: a de fábrica (a primeira que o app viu, nunca trocada) e a
  anterior (a de antes da última mudança). "Voltar à de fábrica" e "Voltar à
  anterior" gravam só os seis pontos; o resto da configuração fica como
  está. A cópia completa da configuração de antes de gravar continua.

## 2.36.0
- **Relatório em PDF em µSv/h, igual à tela.** O relatório do período passa
  a mostrar o GMC-800 em µSv/h. Cada minuto, pico e limite é convertido pela
  calibração que valia naquela hora, então trocar o fator não muda o passado.
  Isso vale para:
  - os cartões de média, mínimo e máximo, fundo da casa e pico ao vivo;
  - o gráfico do período;
  - as tabelas por dia, do fundo, dos episódios de alerta e do tempo acima
    dos limites;
  - os gráficos de 30 minutos antes e depois de cada episódio.
- O CPM fica onde é a medida do aparelho: a estatística da contagem (σ,
  dispersão, histograma), uma coluna de CPM médio por dia, a mediana do
  fundo e a calibração.
- **Limites da época certos.** Desde a 2.29 o histórico de configurações
  grava os limites em µSv/h, e o relatório só entendia os antigos, em CPM:
  marcava "valor atual" (*) em limites que tinham registro. Agora lê os dois.
  Os limites em CPM continuam comparados em CPM, como tocavam, e aparecem
  convertidos.
- "Configurações e método" ganha a linha "Unidade", que explica a conversão.

## 2.35.3
- **Histórico do RadiaCode sem o buraco do fim.** No segundo teste, a
  memória do aparelho voltou inteira, mas os últimos 2 minutos antes da
  reconexão ficavam vazios ou pela metade. Isso acontecia por dois motivos.
  O último ponto de 96 s chega uns 20 s depois de conectar, e o minuto em
  que ele caía já tinha uma parte gravada, então ele era ignorado. E o ponto
  seguinte cobre o começo do minuto da conexão, mas era tratado como leitura
  ao vivo. Agora os minutos pela metade são completados, pesando pelos
  segundos de cada parte, sem passar de 60 s. E o pedaço de antes da
  conexão entra também no minuto em que o RadiaCode foi conectado.
## 2.35.2
- **"Ver na régua" voltou a abrir.** Nas telas do GMC-800 e do RadiaCode o
  link do mostrador não fazia nada: a régua ainda procurava o seletor de
  período da dose que saiu na 2.12. Agora usa o período dos gráficos.
- Em inglês, o título dos alertas do GMC em µSv/h ("Alarme: pico de 1,35
  µSv/h") ficava em português; agora sai traduzido.
- O bloco "Aparelho" do RadiaCode diz "procurando…" enquanto o app procura,
  em vez de "desconectado", igual à linha de cima.

## 2.35.1
- **O mostrador do GMC-800 segue os níveis das Configurações.** Os avisos
  do GMC já disparavam pelos níveis de Configurações › Alarmes e avisos, mas
  o mostrador, o bloco do Agora e a palavra do nível ("Seguro", "Atenção"…)
  ainda usavam as faixas fixas do manual (50 e 100 CPM). Agora os três
  usam os mesmos limites que tocam, como no RadiaCode, e a legenda diz
  "faixas do app em µSv/h".

## 2.35.0
- **Faixas de alarme novas.** Atenção, alarme e perigo agora são a mesma
  peça, com a forma de cada nível (triângulo, losango, octógono) e na mesma
  ordem nos três: nível e aparelho, número, limite; embaixo, o que fazer; à
  direita, os botões. A faixa cresce com a gravidade: a atenção é fina e
  âmbar, o alarme é laranja cheio e o perigo é vermelho, alto e com o número
  grande.
- Silenciado aparece de outro jeito. No alarme, a faixa escurece e mostra
  "som desligado". No perigo, ela continua vermelha e ganha "som pausado até
  HH:MM". Nos dois casos o botão vira "Ligar o som".
- Os telefones de emergência viram etiquetas que dá para selecionar e
  copiar.
- Demo e teste ganham listras e a etiqueta DEMO ou TESTE. Uma leitura real
  com o demo ligado leva a etiqueta REAL.
- **Os dois aparelhos tocando ao mesmo tempo** continuam numa faixa só:
  manda o nível mais grave (no empate, o GMC), e o outro aparece na linha
  "Também: …". Se os dois estão no mesmo nível, a faixa diz "… nos dois
  aparelhos", mostra os dois números e avisa que os dois concordam, ou seja,
  não é defeito de um aparelho só. Continua sendo um som só.
- **RadiaCode saturado:** a partir do fim da escala do aparelho (padrão
  1.000 µSv/h, o fim da faixa do 102 pela especificação do fabricante;
  ajuste em Configurações › RadiaCode › Conexão), a faixa mostra "≥" e "Não
  confie no número", como já fazia no GMC.
- **Voltou ao normal (novo):** quando um alarme real acaba, aparece por
  1 min uma tira verde com a hora, o nível, quanto durou, o pico e o link
  para o histórico. Não aparece depois de demo nem de teste.

## 2.34.2
- **"Onde está cada aparelho" mostra a conexão de verdade.** Com o
  RadiaCode desconectado, o quadro do Agora ainda dizia "conectado" com o
  ponto verde por até 3 minutos, porque olhava só se o último minuto gravado
  era recente. Agora ele segue o mesmo estado do cartão do aparelho, muda na
  hora em que a conexão cai ou volta, e mostra "conectando…" em âmbar
  enquanto tenta.
- Nos blocos do Agora, um aparelho desconectado não mostra mais CPM e CPS
  como se fossem de agora: no lugar vai "última leitura às HH:MM", em cinza,
  e a dose de hoje diz "até HH:MM", como no cartão do aparelho.

## 2.34.1
- **O fim do período desconectado do RadiaCode não se perde mais.** Ao
  reconectar, o aparelho manda o que guardou na memória e também o que
  ainda estava no buffer recente, e esse buffer vem sem a marca de "dado
  antigo". No teste de 10/11, de 12:39 a 13:12 veio marcado e foi para a
  hora certa, mas de 13:14 a 13:46 entrou como leitura ao vivo, na hora da
  reconexão, e o gráfico ficou com o buraco. Agora tudo o que foi medido
  antes do começo da conexão vai para a hora em que foi medido, com a marca
  ou sem.
- Na primeira vez que abre, o app relê os dados brutos das reconexões já
  gravadas, preenche os minutos que ficaram vazios desse jeito e tira do
  log do aparelho as linhas que tinham caído na hora da reconexão.

## 2.34.0
- **Configurações repaginadas.** O menu da esquerda cai de 20 itens para 6:
  GMC-800, RadiaCode, Alarmes e avisos, Janela e tela, Dados e backup, e
  Atualizações e sobre. Cada item mostra uma linha de estado (conectado,
  procurando, a partir de quanto o alarme toca, versão). Nenhum ajuste saiu
  do app; só mudaram de lugar.
- **Um lugar por aparelho, com abas.** GMC-800: Conexão, No aparelho,
  Calibração, Tubo e Testes. RadiaCode: Conexão e No aparelho. A Calibração
  do GMC junta o fator que o app usa (com a intercalibração e as versões) e
  os 6 pontos gravados no contador, que antes ficavam em lugares separados.
  Tempo morto e tubo saturado foram para a aba Tubo; "Testar o contador",
  "Descobrir onde fica um ajuste" e a tensão do tubo, para Testes.
- **RadiaCode sem o "Avançado".** Alarmes de contagem, calibração de energia,
  firmware e "Zerar a dose acumulada" ficam junto dos outros ajustes, em No
  aparelho.
- **Selo do aparelho.** Os cartões que gravam no aparelho têm o selo
  "GMC-800" (verde) ou "RadiaCode" (rosa) e o botão de gravar dele. O resto
  fica no app e continua indo pela barra "Salvar".
- **Alarmes e avisos numa escada só.** Atenção, alarme e perigo lado a lado,
  com a forma e a cor de cada nível e uma barra que acompanha os números. Os
  alarmes que tocam nos próprios aparelhos aparecem embaixo, em µSv/h, com
  atalho para mudar. "Fora de casa" também fica aqui.
- **Busca lista os ajustes.** Digitar mostra os cartões encontrados com o
  caminho (ex.: "GMC-800 › Tubo") e abre direto na aba certa.
- **Sobre** com o link do projeto no GitHub e o autor.
## 2.33.0
- **Mostrador: só o símbolo no meio.** No mostrador grande da tela do
  aparelho, o centro mostra só a forma do nível, maior. "Seguro", "Atenção",
  "Alarme" e "Perigo" ficam escritos embaixo do anel, na cor do nível.
- **Três estados de conexão, no mostrador e nos cartões:**
  - conectado: a forma do nível, como antes;
  - conectando: anel tracejado âmbar, com os arcos âmbar pulsando (parados
    com "reduzir movimento"), e "Conectando…" embaixo;
  - desconectado: um traço cinza, com os arcos cinza, sem parte acesa nem
    ponto, e "Desconectado" embaixo.
  Vale para o cartão grande, para o compacto do outro aparelho e para os
  quadros da tela Agora.
- **Cartão grande sem conexão:** o número grande vira "Última leitura às
  HH:MM", em cinza, e as unidades, o σ e o "dentro do normal" somem. No topo
  entra um aviso com o motivo (sem o endereço do aparelho; endereço e porta
  ficam em "detalhes"), com "Procurar de novo" e "Configurar conexão" (no
  GMC, "Escolher porta"). A dose de hoje mostra "até HH:MM".
- O cartão "Aparelho" da aba RadiaCode perde a linha vermelha de erro e o
  "erro na conexão" do título. Agora mostra o tipo de conexão, o último
  contato e os botões.
## 2.32.1
- Mostrador grande da tela do aparelho: a forma do nível volta para o centro
  exato do anel (estava um pouco acima). O nome do nível fica embaixo dela.
- Dose acumulada: os nomes "GMC-800" e "RadiaCode" nos quadros voltam ao
  tamanho normal. O estilo do texto do mostrador novo tinha o mesmo nome de
  classe e encolhia esses nomes.
## 2.32.0
- **Ajustes do RadiaCode pelo app.** Em Configurações › RadiaCode, o bloco
  novo "Ajustes gravados no aparelho" mostra o que está no RadiaCode agora e
  deixa mudar: alarmes 1 e 2 de taxa de dose (µSv/h), de dose acumulada
  (µSv, ou desligado) e, no Avançado, de contagem (cps); som e vibração, e o
  que faz cada um disparar (alarmes, fora da escala, botões, cliques); brilho,
  tempo para apagar a tela e orientação; unidades no visor (Sv/R, cps/cpm).
  Cada grupo explica para que serve. Na aba RadiaCode, o botão "Ajustes do
  aparelho" leva direto para lá.
- Nada vai para o aparelho sem clicar em "Gravar no RadiaCode", e a
  confirmação lista o que muda. Antes de gravar, o app salva uma cópia (pasta
  radiacode/configuracao), grava só o que mudou, lê de volta para conferir e
  registra no histórico de configurações e no log. "Desfazer a última
  gravação" põe de volta o que estava.
- Os alertas do RadiaCode no app continuam usando os alarmes 1 e 2 de taxa do
  aparelho, então mudar esses dois muda os alertas também (a confirmação
  avisa).
- **Zerar a dose acumulada** do RadiaCode pelo app, com confirmação (não tem
  volta).
- Só leitura: a calibração de energia (keV por canal) e o firmware.
- Correção: as unidades do aparelho (Sv/R e cps/cpm) eram lidas no byte
  errado e saíam sempre como R e cps.

## 2.31.0
- **Tela do aparelho: o valor aparece uma vez só.** Na página do GMC-800 e
  na do RadiaCode, o quadro do próprio aparelho sai da fileira de cima. O
  outro aparelho fica compacto ao lado dos alertas, com um mostrador pequeno,
  e abre a tela dele no clique. A tela Agora não muda.
- **O mostrador grande faz o papel da régua.** A barra das faixas saiu. As
  três faixas ficam sempre à vista, apagadas, e acendem até a leitura, com um
  ponto branco nela. Os limites ficam escritos nos vãos do anel, na unidade
  escolhida. O verde e o amarelo são lineares entre os limites; o laranja vai
  do limite de alarme até o de perigo em escala log. No perigo o anel inteiro
  fica vermelho.
  - GMC-800: 50 / 100 CPM do manual e o limite de perigo do app.
  - RadiaCode: alarme 1 e alarme 2 do aparelho; perigo no limite do app ou a
    10× o alarme 2 enquanto não houver um limite próprio.
- O aviso de relógio do GMC fora da hora vai para o cartão grande na página
  dele.
- Textos do nível de fundo mais claros: "dentro do fundo de sempre" virou
  "no nível de fundo habitual", e "seu fundo" virou "nível de fundo" nas
  frases do Agora, do σ, da Medição e do relatório.
## 2.30.0
- **O RadiaCode não perde mais o que mediu com o PC desligado.** Sozinho, o
  aparelho guarda na memória as últimas ~1000 h (a taxa de 96 em 96 s, a
  dose, temperatura, bateria e eventos) e manda tudo quando conecta, marcado
  como "dado antigo". O app jogava esses lotes fora. Agora cada registro vai
  para a hora em que foi medido, com a hora do próprio aparelho:
  - os minutos sem leitura ganham a taxa da memória (marcados como
    `memoria`, com os segundos cobertos), e o que o app gravou ao vivo nunca
    é trocado;
  - o log do aparelho ganha as linhas, com a etiqueta "memória do aparelho";
  - a dose acumulada entra no histórico de dose.
  No fim vem um aviso no histórico: quantos registros, de quando a quando e
  quantos minutos foram preenchidos.
- O aparelho apaga da memória o que manda. Se o app do celular conectar
  antes, é ele que leva; aí o caminho é importar o log do celular.
- Alarmes do RadiaCode (que trazem a taxa junto) e o contador de dose não
  embaralham mais a leitura do resto do pacote.
## 2.29.0
- **GMC-800 em µSv/h.** Os limites do GMC (alarme, aviso de atenção e alarme
  de perigo) agora são escolhidos em µSv/h nas Configurações; o app acha o
  CPM equivalente pela calibração em uso. Na atualização os limites antigos
  viram µSv/h sem mudar o ponto em que tocam. Se a calibração mudar, o que
  fica fixo é o µSv/h.
- Alertas (lista, detalhe e título da janela), Resumo e os números do Agora
  mostram o GMC em µSv/h, cada valor pela calibração da hora; os alertas
  antigos gravados em CPM também aparecem convertidos.
- **Gráfico principal em µSv/h**: é o único com as linhas de limite, os
  alarmes e os picos, e passa a abrir por padrão.
- **"CPM (análise)"**: o gráfico de CPM fica só para análise, sem limites nem
  alarmes, e ganha o RadiaCode com escala própria no eixo da direita (ele
  conta umas 28 vezes mais que o GMC; na mesma escala a linha do GMC
  sumiria).
- CPM e CPS continuam onde são medida do aparelho: configuração do contador,
  calibração, tempo morto, explicação do σ e mapa de padrões.

## 2.28.0
- **Intercalibrar com o RadiaCode, de dentro do app.** Em Configurações ›
  Calibração do GMC, o botão novo começa uma coleta de 24 h com os dois
  aparelhos encostados. No fim aparece o resultado com a mesma conta da
  análise de 09/11: quanto o GMC lia acima ou abaixo do RadiaCode, o fator
  novo com a margem de 95%, o fator hora a hora, a saúde do tubo e o que
  muda no fundo e na dose. "Usar … a partir de agora" vira uma versão da
  calibração com origem "intercalibração com o RadiaCode" (dá para
  desfazer). O contador não é mexido.
- **Vigia semanal dos dois aparelhos.** Nos minutos com os dois em casa, o
  app compara a razão de contagens (cps do RadiaCode ÷ CPM do GMC) com o
  normal das 4 primeiras semanas da calibração em uso. A frase aparece na
  Calibração e na página do GMC-800; se a razão sair mais de 5% do normal
  por duas semanas seguidas, entra um aviso de manutenção em Alertas.
- A conta fica em `internal/core/intercal`, com teste; a coleta entra no
  backup.

## 2.27.0
- **Fundação do estilo, passo 6: espaço.** Os espaçamentos internos
  (padding, margin e gap) passam a usar a régua do guia: 4, 8, 12, 16, 20,
  24 e 32 px (`--space-*`). Os valores fora da régua sobem para o degrau
  mais próximo (6 vira 8, 10 vira 12, 14 vira 16, 18 vira 20, 22 vira 24),
  então o app fica um pouco mais arejado. Ajustes finos de 1 a 3 px e
  recuos grandes continuam como estavam.

## 2.26.0
- **Cada nível tem forma, não só cor.** Seguro é círculo, Atenção é
  triângulo, Alarme é losango e Perigo é octógono com uma faixa; "sem
  contagens" é um triângulo vazado. A forma aparece antes do nome do nível
  no Agora, no Resumo, nos quadros dos aparelhos, no selo de Alertas da barra
  lateral, na lista de alertas, na legenda e nas linhas de limite dos
  gráficos, no ícone da bandeja e na coluna de nível do relatório PDF. Dá pra
  ler o nível sem distinguir cores.
- **Mostrador no lugar do número e da régua.** Nos quadros do GMC-800 e do
  RadiaCode, o número grande e a régua de três faixas viraram o mostrador do
  logo: o valor fica dentro do anel, o arco de fora enche conforme a leitura
  (um terço por faixa, no sentido do relógio) e tudo pega a cor do nível. Ao
  lado, o nível em letra grande e uma frase curta ("dentro do fundo de
  sempre", "passou o limite de alarme"...). Passando o mouse no mostrador
  aparecem os limites.
- **Ponto de conexão vira anel.** O ponto ao lado do nome do aparelho agora é
  um anel, para não confundir com o círculo do nível Seguro.

## 2.25.1
- **Trecho aberto na mescla de backup.** Um backup feito em outro PC
  enquanto você ainda estava fora traz o trecho "até agora". Se aqui ele já
  tinha sido fechado, a mescla colocava o aberto de novo, por cima, e o
  aparelho ficava "fora" para sempre. Agora um trecho que chega cruzando com
  um que já existe aqui fica de fora (o daqui vale).
- **Trecho aberto acompanha o relógio.** Com o app aberto por horas, o
  trecho "até agora" parava na hora em que a lista foi carregada: a faixa
  lilás nos gráficos e no Agora encurtava, o relatório e o "Ver" do trecho
  cortavam o fim. Agora vai até o minuto atual, e o Agora diz "fora de casa
  até agora" em vez de uma hora parada.
- Cabeçalho da tabela de calibração e do histórico de configuração com o
  fundo do cartão (usava uma cor que não existe no guia).

## 2.25.0
- **Fundação do estilo, passo 5: "Alerta" vira "Alarme" no segundo nível.**
  Os níveis agora são Seguro, Atenção, Alarme e Perigo. "Alerta" continua
  sendo o aviso ou o episódio (página Alertas, "Nenhum alerta ativo").
  - O botão do modo demo e a régua do RadiaCode dizem "150 · alarme" e
    "· alarme".
  - O segundo nível fica laranja em todo lugar: no quadro, no menu, na
    faixa compacta e na frase do Agora (que estava com fundo vermelho). O
    valor do RadiaCode em Perigo passa a vermelho (estava laranja).
  - Por dentro: o núcleo (`internal/core/levels`) devolve "alarme", as
    classes `lv-alerta` viraram `lv-alarme` e saiu a tradução de uma chave
    para a outra.

## 2.24.0
- **Fundação do estilo, passo 4: cores restantes.**
  - **Anotação em cinza.** A marca de anotação nos gráficos sai do roxo,
    que se confundia com o lilás de "fora de casa", e fica cinza, com o
    tracejado de sempre. O lilás passa a ser só "fora de casa / fora do
    lugar".
  - As cores que ainda estavam escritas à mão viraram tokens no style.css
    (`--away`, `--obj`, `--info`, as faixas sólidas do alarme e do modo
    demo, os trilhos e as faixas do quadro de nível) e no JS dos gráficos
    (`COR.away`, `COR.obj`, `COR.info`, `COR.note`). Fora do `:root` só
    sobra o degradê do espectro, que é cor de dado.
  - As transparências agora derivam do token com `color-mix` e ficam em
    poucas intensidades (6%, 12%, 24% e 50%). Alguns fundos e contornos
    ficam um pouco mais claros ou mais escuros.
  - Corrigidas duas cores de reserva erradas (um azul no lugar do verde e
    um fundo antigo).

## 2.23.0
- **Fundação do estilo, passo 3: raios.** Os 19 arredondamentos de canto
  que o app usava viraram os 5 do guia, em 153 lugares: `--radius-xs` (4)
  para barrinhas, células e etiquetas pequenas, `--radius-sm` (8) para
  botão, campo e aba, `--radius-md` (10) para segmentado e caixas internas,
  `--radius-lg` (14) para cartões e `--radius-pill` para chips. Só sobram
  `50%` (pontos redondos) e `0`. Os cantos mudam 1 ou 2 px em alguns
  lugares.

## 2.22.0
- **Fundação do estilo, passo 2: tipo.** Os 29 tamanhos de fonte que o app
  usava viraram os 11 degraus do guia (`--fs-micro` a `--fs-reading-xl`),
  em todos os 220 lugares do style.css. A tela muda pouco:
  - 10 px sobe para 11 (as etiquetas "ao vivo" e "simulado" ficam legíveis);
  - 14 px vira 15 no texto corrido e 13 em botões, tabelas e etiquetas;
  - 17 vira 16; 18 e 20 viram 19; 24 e 26 viram 28 (números de quadro);
  - as leituras grandes de 34 a 44 viram 40; 64 vira 56; 68 e 76 viram 88
    (e 56 em tela estreita).
  Sobra só um `font-size: 0`, que esconde o texto do menu recolhido.

## 2.21.2
- **Fundação do estilo, passo 1: tokens e limpeza.** Arrumação interna do
  style.css, sem mudança na tela (conferido comparando o estilo calculado
  de cada elemento em todas as telas do modo demo, antes e depois).
  - O `:root` ganhou os tokens do guia que faltavam: os 11 degraus de tipo
    (`--fs-*`), os 5 raios (`--radius-*`), a régua de espaço (`--space-*`),
    a sombra de elemento flutuante e as cores que ainda estavam soltas
    (fora de casa, anotação, Medir objeto, informação e as faixas sólidas
    do alarme). Os próximos passos passam a usar esses nomes.
  - Regras do mesmo seletor escritas duas ou três vezes foram juntadas num
    lugar só (de 79 para 34; as que sobraram dependem da ordem em relação a
    outra regra e ficam para os passos seguintes).
  - `!important` caiu de 19 para 3 (ficam `[hidden]` e os dois do modo
    duo); os outros viraram seletores um pouco mais específicos.

## 2.21.1
- **Atualização automática volta a achar a versão mais nova.** A 2.20.5 e a
  2.21.0 saíram com dois minutos de diferença, e a 2.20.5 terminou de
  publicar por último: o GitHub marcou ela como a mais recente e o app
  (que pergunta só pela mais recente) não oferecia a 2.21.0. Agora as
  releases saem uma por vez, na ordem, e a última publicada é marcada como
  a mais recente. Nada muda no app em si.

## 2.21.0
- **Onde está cada aparelho.** Na tela Agora, um cartão novo diz onde está o
  GMC-800 e onde está o RadiaCode: em casa, com você ou fora do cálculo. O
  normal continua sendo o GMC-800 em casa e o RadiaCode com você. Mudar
  abre uma linha com o que vai acontecer, um nome opcional e Confirmar.
  - **Dose pessoal:** vem de quem está com você. Com os dois com você, o
    RadiaCode mede e o GMC-800 preenche as falhas dele. Com os dois em casa,
    vale a leitura de casa (você também estava em casa); se saiu e esqueceu
    os dois, marque "eu não estava em casa" e o trecho fica em branco.
  - **Casa e fundo:** vêm de quem está em casa. Com os dois em casa, o
    GMC-800 mede e o RadiaCode preenche. Aparelho com você não entra no
    fundo de casa.
  - **Fora do cálculo:** para aparelho quebrado, em manutenção ou com
    leitura não confiável. Dá para valer desde a última leitura boa. Com o
    GMC-800 sem nenhuma contagem por uns minutos, o app sugere tirar do
    cálculo para o tubo com defeito não puxar o fundo para baixo.
  - **Estado ao vivo** de cada aparelho: conectado, desconectado desde
    HH:MM, saturando ou sem contagens. A frase do topo diz quando falta
    medição. Minutos saturados contam pelo valor lido, com "≥" na dose.
  - **Minha dose** mostra quem mediu cada período (o aparelho, "misto" ou
    "sem medição") e quanto foi de cada um.
- **Fora de casa virou a lista de trechos.** Linha do tempo dos últimos 7
  dias por aparelho, com as falhas marcadas (sem dado, saturando, sem
  contagens), e um formulário para trecho passado com onde estava cada um.
  Os trechos antigos continuam valendo como "Levei os dois".

## 2.20.5
- **Relatório em PDF de novo rápido, e com aviso enquanto gera.** O
  relatório de uma semana levava cerca de 40 s, sem nenhum sinal na tela:
  a tabela "Configurações no período" cortava o texto comprido uma letra
  por vez, medindo tudo de novo a cada letra. Agora sai em menos de 0,1 s,
  igual ao de antes. Enquanto gera, o botão mostra "Gerando…" e fica
  travado; cliques a mais não viram outros PDFs, e relatórios gerados ao
  mesmo tempo não gravam mais um por cima do outro.
- **Log sem enxurrada.** O RadiaCode 102 manda "Error: S1/S2 Data frame
  corrupted!" a cada ~100 s, e cada uma ia para o log.txt (16 mil linhas
  em dois dias, o log girava em horas). Agora a mesma mensagem entra no
  máximo uma vez por hora, com a contagem das repetições, e reconectar não
  repete as que já entraram. "Sincronização com … não respondeu" entra uma
  vez, e de novo só quando o erro muda ou o outro PC volta a responder.
- **Dose acumulada do GMC-800 não "estoura" mais ao trocar o fator.** Com
  um fator novo valendo só daqui em diante, a dose do período continuava no
  fator antigo, mas o fundo de referência já vinha no novo: 111% do limite
  sem a radiação ter mudado. Agora o fundo usa, minuto a minuto, a mesma
  calibração que a dose.
- No modo navegador, o zoom do gráfico em µSv/h mostrava "mín — · máx —".

## 2.20.4
- **Cores num lugar só, sem mudança na tela.** As cores da interface agora
  saem das variáveis do guia de estilo, no começo do `style.css`.
  - O `style.css` tinha dois blocos `:root`, e o segundo (da repaginação
    1.10) trocava fundo, superfícies e borda. Ficou um bloco só, com os
    valores que já estavam valendo, mais os que faltavam: `--rc`, `--alarm`,
    `--accent-hover`, `--surface-hover`, `--border-hover`, `--attn-bg`,
    `--attn-ink` e `--chart-grid`.
  - Cerca de 45 cores escritas à mão no CSS viraram `var(--…)`.
  - Os gráficos (SVG e canvas) pegam as cores de `frontend/dist/cores.js`,
    que lê essas mesmas variáveis. Foram cerca de 110 trocas nos arquivos
    JS. Cores próprias de um gráfico só (o roxo e o ciano das séries extras,
    por exemplo) ficaram como estavam.

## 2.20.3
- **Mais arrumação por dentro, sem mudança na tela.** A página Fora de casa
  (a lista de períodos, os números de cada período e o editor) saiu do
  `app.js` para `frontend/dist/fora.js`. O `app.js` ficou com cerca de 210
  linhas a menos; desde a 2.18 já são cerca de 2.600.

## 2.20.2
- **Mais arrumação por dentro, sem mudança na tela.** A página Medir objeto
  (a medição com fundo e objeto, o gráfico durante a medição, o segundo
  aparelho, o espectro do RadiaCode e as medições salvas) saiu do `app.js`
  para `frontend/dist/medir.js`. O `app.js` ficou com cerca de 930 linhas a
  menos; desde a 2.18 já são cerca de 2.400.

## 2.20.1
- **Mais arrumação por dentro, sem mudança na tela.** A aba Dose dos
  Gráficos (o gráfico de CPM com o zoom por arraste e a dose acumulada dos
  dois aparelhos) saiu do `app.js` para `frontend/dist/graficos.js`. O
  `app.js` ficou com cerca de 630 linhas a menos; desde a 2.18 já são
  cerca de 1.500.

## 2.20.0
- **Mais arrumação por dentro, sem mudança na tela.**
  - A busca dos episódios de alerta nos minutos do histórico do contador
    (entrada no nível de atenção ou no alarme, se for menor, saída em 90%,
    corte depois de 70 min sem dado) e o nível de cada episódio foram para
    `internal/core/alerts`, com testes próprios. É o terceiro pacote do
    núcleo que o app Android vai reaproveitar, junto com `levels` e `stats`.
  - A página Alertas (títulos, lista com os episódios juntos e o gráfico de
    cada alerta) saiu do `app.js` para `frontend/dist/alertas.js`. O
    `app.js` ficou com cerca de 400 linhas a menos.

## 2.19.0
- **Arrumação por dentro, sem mudança na tela.** Preparação para o app
  Android e para mexer no app com menos risco:
  - os níveis (atenção, alerta, perigo), a correção de tempo morto e a
    entrada e saída dos episódios de atenção e perigo foram para
    `internal/core/levels`; a mediana e o desvio de Poisson, para
    `internal/core/stats`. São pacotes sem interface nem Wails, com testes
    próprios, que o Android vai reaproveitar junto com `internal/core/dose`;
  - duas cópias da mediana que existiam no meio do código passaram a usar a
    mesma função;
  - a página Resumo saiu do `app.js` para um arquivo próprio
    (`frontend/dist/resumo.js`), como a tela Agora. O `app.js` ficou com
    cerca de 450 linhas a menos; as outras páginas saem uma por versão.

## 2.18.0
- **Resumo repaginado.** O período e a comparação continuam iguais; muda a
  ordem do que aparece:
  - **Uma frase no topo** diz se o período foi normal, se teve alerta
    (atenção, alarme ou perigo, somando os dois aparelhos) ou se o fundo
    ficou acima do normal, com a casa e o RadiaCode em relação ao normal de
    cada um, quanto do tempo foi medido e quanto ficou fora de casa.
  - **Três números**: a sua dose (RadiaCode; sem ele, a da casa pelo
    GMC-800), o fundo da casa comparado com o seu normal e o pico.
  - **Casa × pessoal**: quanto da dose do RadiaCode foi em casa e quanto
    fora, a parte do tempo e da dose fora de casa e a razão entre o
    RadiaCode e o GMC-800 em µSv/h (quanto você recebe em relação à casa).
  - **Detalhes**: todos os números de antes, as lacunas, as anotações e o
    histórico disponível ficam recolhidos e abrem com um clique. O app
    lembra se estava aberto.
- **Gráficos com um só seletor de período**, acima dos gráficos de CPM e de
  dose. Os dois já andavam juntos; agora o botão repetido no gráfico da dose
  saiu.

## 2.17.0
- **Tela "Agora", a nova página inicial.** O app abre nela, e ela fica no
  topo da barra lateral.
  - **Uma frase no topo** responde se está tudo bem: "Tudo normal agora",
    com a taxa da casa e a sua, e se a leitura está dentro do seu fundo; ou
    qual alerta está ativo e em qual aparelho; ou que nenhum aparelho está
    lendo.
  - **Minha dose**: a dose de hoje, desta semana (desde segunda), deste mês
    e deste ano, com quanto o fundo natural sozinho daria nos mesmos
    minutos. Escolha entre "pessoal" (RadiaCode, que anda com você) e
    "casa" (GMC-800); o app começa pelo RadiaCode quando ele tem dado no
    ano. No ano, mostra quanto daria o ano inteiro nesse ritmo e uma barra
    com a dose do ano, o fundo do ano inteiro e o teto (fundo + limite
    escolhido na Dose acumulada).
  - **Hoje**: as duas curvas das últimas 24 h, com o nível de atenção e os
    trechos fora de casa, e a lista do dia (alertas, anotações e saídas).
- As contas da Minha dose seguem as da Dose acumulada (só os minutos
  medidos entram). Os períodos do calendário e a projeção do ano estão em
  `internal/core/dose`, o primeiro pacote do núcleo sem interface, que o app
  Android vai reaproveitar.

## 2.16.0
- **Atualização assinada.** O app só instala uma versão nova se a
  assinatura do `SHA256SUMS.txt` conferir com a chave das releases do
  RadLog (Ed25519, embutida no app). Antes, o app só conferia se o arquivo
  tinha chegado inteiro, pela soma que vem na própria release; quem
  conseguisse publicar uma release falsa publicaria a soma junto. Agora:
  - cada release traz o `SHA256SUMS.txt.sig`, gerado no GitHub com a chave
    privada guardada como segredo;
  - sem assinatura, ou com uma assinatura que não confere, o app não instala
    e pede para baixar pela página;
  - o job de release não publica nada se a chave faltar ou não for a do app.
- Para conferir uma release à mão: `go run ./scripts/sign verify
  SHA256SUMS.txt` (com o `.sig` na mesma pasta).

## 2.15.0
- **Um erro interno não fecha mais o app.** As tarefas de fundo (leitura do
  GMC, laço do RadiaCode, sincronização, chuva, atualização, alertas) agora
  se protegem contra erro interno (panic). Antes, um pacote estranho do
  Bluetooth, por exemplo, podia fechar o RadLog inteiro e a gravação parava
  até alguém abrir de novo. Agora:
  - o erro vai para o log técnico, com os detalhes para diagnóstico, e
    aparece nos eventos como "Erro interno no app";
  - a tarefa volta sozinha depois de 5 s, 10 s, 20 s… (até 5 min); se der
    erro 10 vezes na mesma hora, para e avisa para fechar e abrir o app;
  - na leitura do GMC, o erro solta a porta e reconecta como numa conexão
    perdida; no RadiaCode, o USB ou o Bluetooth é liberado para a próxima
    conexão.
- **Testes de fuzz do protocolo do RadiaCode.** O DATA_BUF, o espectro e a
  montagem das respostas em pedaços são testados com milhões de entradas
  aleatórias (nenhuma derrubou o decodificador).
- **Build com dependências travadas.** O `go.sum` passa a ficar no
  repositório e o CI confere as somas (`go mod verify`) e falha se faltar
  `go mod tidy`, em vez de baixar o que estiver lá na hora. O CI também roda
  os testes com o detector de corrida, e o `go-winres` da release tem
  versão fixa.

## 2.14.1
- **Fica claro que o número grande do RadiaCode é do aparelho.** No quadro
  da dose acumulada, o valor em destaque já era a dose medida pelo próprio
  RadiaCode no período (a mesma da curva no gráfico), mas o rótulo curto
  "do aparelho" e a linha "No aparelho:" logo abaixo davam a entender que o
  destaque era a conta do app. Agora o destaque diz "medida pelo aparelho no
  período" e a linha do total desde que foi zerado virou "Contador do
  aparelho:"; a conta do app continua em "Pelo app:".

## 2.14.0
- **Modo demo com RadiaCode.** No quadro do modo demo há agora "Simular o
  RadiaCode": liga um RadiaCode de mentira com o µSv/h que você escolher
  (campo, barra em escala logarítmica de 0,01 a 10.000 µSv/h e atalhos para
  o fundo, a atenção, o alerta e o perigo, com os limites do próprio
  RadiaCode).
  - O quadro do RadiaCode, a faixa compacta, o quadro completo e o gráfico
    ao longo do tempo usam o valor simulado, marcados como "demo" e
    "simulado".
  - Os alertas do RadiaCode (atenção, alarme e perigo), a faixa no topo e o
    som funcionam como de verdade, com "(demo)" no título.
  - "Variar como um contador de verdade" também vale para ele (a contagem
    oscila como a do aparelho).
  - Nada simulado vai para o disco: o RadiaCode de verdade, se estiver
    conectado, continua sendo lido e gravado, e ao sair do demo tudo some.

## 2.13.3
- **Dose do aparelho dentro do quadro do RadiaCode.** As duas linhas soltas
  embaixo dos quadros ("no aparelho, desde que foi zerado…" e "Calculado
  pelo app…") viraram duas linhas curtas dentro do quadro do RadiaCode:
  "No aparelho: 4,990 µSv desde que foi zerado (33,8 h) · ao vivo" e "Pelo
  app: 3,20 µSv (13% do período gravado)".

## 2.13.2
- **Uma barra só por aparelho na dose.** As barras dos quadros do GMC-800 e
  do RadiaCode comparavam a dose total com o teto (fundo + limite), e a
  barra embaixo de "Dentro do teto" mostrava outra conta (o que passou do
  fundo sobre o limite), com números bem diferentes. Agora a barra de cada
  quadro mostra quanto do limite aquele aparelho já usou: só o que passou do
  fundo natural dele conta, como no limite de verdade (ex.: "2,6% do limite
  · 0,10 µSv acima do fundo, de 3,79 µSv permitidos"). A terceira barra
  saiu; a frase "Dentro do teto" continua.

## 2.13.1
- **Dose do RadiaCode não aparece mais como "desconectado" com ele
  conectado.** O aparelho só manda o total de dose a cada vários minutos, e a
  cada reconexão (inclusive quando o app se atualiza) o app esperava o
  próximo envio; até lá, a linha dizia "desconectado". Agora:
  - ao fechar ou perder a conexão, o app guarda o total estimado (o último
    do aparelho mais o que somou ao vivo) e, se reconectar em até 3 min,
    continua dele;
  - conectado e esperando o próximo total, a linha diz "esperando o próximo
    total do aparelho".
- **Bluetooth sem insistir a cada 30 s.** Quando o RadiaCode está no USB de
  outro PC (ou com o celular), o app que procura pelo Bluetooth conectava,
  caía em poucos segundos e tentava de novo a cada 30 s; o log do aparelho
  mostrava "BLE: client connected / disconnected" o tempo todo. Cada
  tentativa pode levar dados que iriam para quem está com ele no USB (como o
  total de dose). Agora, a cada tentativa que não segura a conexão, o
  intervalo dobra (30 s, 1, 2, 4, 8 até 16 min) e volta ao normal quando uma
  conexão dura mais de 2 min.

## 2.13.0
- **Período também na dose acumulada.** O seletor de período (15 min a
  1 ano) aparece no quadro da dose acumulada, além do gráfico da taxa; os
  dois continuam ligados: trocar num troca no outro.
- **Mostrar ou esconder cada aparelho no gráfico da dose.** Clicar no
  quadro do GMC-800 ou do RadiaCode tira a curva dele do gráfico (e o teto e
  o fundo dele); clicar de novo volta. Dá para deixar os dois, só o
  RadiaCode ou só o GMC-800; um deles fica sempre visível (esconder o único
  que sobrou troca para o outro). Vale também para o trecho do mapa de
  calor, e a escolha fica guardada.

## 2.12.2
- **Dose do RadiaCode não fica mais parada.** A curva "do aparelho" usa os
  totais que o RadiaCode manda a cada ~4 min; se ele para de mandar (ou a
  conexão cai e volta), a curva e o teto ficavam retos depois do último
  total, mesmo com o app gravando os minutos. Agora, depois do último total,
  a curva segue com os minutos que o app gravou, e o fundo e o teto
  continuam crescendo junto.
- **Cada aparelho com o seu teto.** No gráfico da dose acumulada, o teto do
  aparelho principal é a linha vermelha e o do outro aparece pontilhado;
  antes só havia o teto de um, e a curva do outro parecia passar do limite
  sem passar.

## 2.12.1
- **Corrigido o gráfico de dose vazio num trecho sem RadiaCode.** Com o
  RadiaCode já usado alguma vez, a dose usava ele como aparelho principal
  (fundo, teto e eixo); num trecho de antes dele existir, ou em que ele
  estava desligado, o gráfico ficava em branco com "sem dados no período"
  mesmo com o GMC-800 tendo medido (acontecia ao abrir um trecho do mapa de
  calor na aba Dose). Agora o GMC vira o principal quando o RadiaCode não
  tem dado no trecho.
- "mSv" no teto não sai mais como "msv".

## 2.12.0
- **Dose virou a primeira sub-aba de Gráficos.** "Ao longo do tempo" agora
  se chama Dose e junta os dois gráficos: em cima a taxa de dose, embaixo a
  dose acumulada. A página Dose saiu do menu.
  - **Um período só** para os dois: 15 min, 30 min, 1 h, 6 h, 24 h, 7 dias,
    30 dias e 1 ano (o gráfico da taxa ganhou 6 h, 30 dias e 1 ano; a dose
    ganhou 15 min, 30 min e 1 h). O zoom num gráfico continua valendo para
    o outro.
  - **Dose de cada aparelho:** um quadro para o GMC-800 e outro para o
    RadiaCode, com o total, a média em µSv/h, de onde vem a dose (do
    aparelho ou calculada pelo app) e quanto do teto do período já foi. No
    gráfico, uma curva para cada; o fundo e o teto seguem o RadiaCode
    quando ele tem dado. Saiu o seletor RadiaCode/GMC-800 da dose.
- **Trecho do mapa de calor sem trocar de aba.** Arrastar numa linha do
  "Hora do dia" abre logo embaixo um quadro "Trecho selecionado" com a dose
  de cada aparelho naquele trecho, a taxa de dose e a dose acumulada dos
  dois. O × limpa; "Abrir na aba Dose" leva o mesmo trecho para os gráficos
  completos e "Marcar como fora de casa" abre o período já preenchido.
- A chuva no gráfico só é buscada em períodos de até uma semana.

## 2.11.1
- **Faixa compacta em todas as páginas.** Ela só aparecia quando os quadros
  dos aparelhos saíam inteiros da tela; em páginas que rolam pouco (como o
  Espectro do RadiaCode numa tela alta) os quadros ficavam cortados em cima
  e a faixa não vinha. Agora ela aparece assim que mais da metade do painel
  sai por cima.

## 2.11.0
- **Configurações viraram uma página.** Saiu a janela por cima do app:
  Configurações é um item do menu, com os quadros dos aparelhos em cima
  como nas outras páginas. As seções ficam numa coluna em grupos:
  - Aparelhos: GMC-800, Conexão, RadiaCode.
  - Medição: Calibração do GMC, Tempo morto do tubo, Geiger saturado.
  - Avisos: Alarme, Aviso de atenção, Alarme de perigo, Fora de casa.
  - App: Chuva no gráfico, Janela, Idioma, Atualizações.
  - Dados: Sincronização, Backup, Relatórios, Dados, Histórico de
    configurações.
  - Sobre: versão, leitura e protocolo do GMC-800.
- **Busca.** O campo ao lado do título filtra as seções pelo que está
  escrito nelas (nomes dos ajustes, explicações e opções) e abre a primeira
  que bate. Enter abre, Esc limpa.
- **Barra de mudanças não salvas.** Ao mexer num ajuste que precisa de
  Salvar, aparece uma barra fixa embaixo dizendo quantos e quais mudaram,
  com Descartar e Salvar; a engrenagem do menu ganha uma bolinha. Dá para
  sair da página e voltar sem perder o que estava mexendo. Salvar não fecha
  mais nada: você continua onde estava.
- Em janela estreita (menu em cima), as páginas compridas não cortam mais o
  fim embaixo.

## 2.10.0
- **Faixa compacta dos aparelhos ao rolar.** Quando os quadros do topo saem
  da tela, uma faixa fina fica presa em cima com cada aparelho numa linha:
  conexão, valor, CPM e o estado na cor do nível (Seguro, Atenção, Alerta,
  Perigo). Com alerta ativo aparece um selo com o nível e o aparelho (clicar
  abre a página Alertas) e a borda da faixa muda de cor. Clicar num aparelho
  ou em "voltar ao topo" volta aos quadros completos. Em janela estreita a
  faixa mostra só o essencial.

## 2.9.1
- **Sem rodapé.** A versão foi para o lado do nome, no topo do menu. "Leitura
  a cada 1 s" e "Protocolo GQ-RFC1201" ficam em Configurações, no quadro do
  contador, e a pasta de dados continua em Configurações → Dados → Abrir
  pasta.

## 2.9.0
- **Gráficos e RadiaCode em sub-abas.** As duas páginas mais longas agora
  mostram uma parte de cada vez, numa barra de abas no topo, sem precisar
  rolar por tudo:
  - Gráficos: Ao longo do tempo, Hora do dia, Padrões do fundo e Tendência
    do fundo.
  - RadiaCode: Agora (o quadro completo, o aparelho e o lado a lado com o
    GMC), Espectro, Espectrograma (com a dureza), Log, Modo busca e
    Atividade (Bq).

  A última aba escolhida em cada página volta quando o app abre. Clicar num
  alerta ou no mapa de calor abre o trecho em "Ao longo do tempo". As abas
  também andam pelas setas do teclado. Padrões e Tendência só são calculados
  com a aba aberta.
- Em página curta, o rodapé vai para o fim da janela.

## 2.8.1
- **Marca nova.** O símbolo virou um trifólio de ondas: um anel com três
  arcos na posição das pás do símbolo de radiação. No topo do menu ele
  entra no lugar do "o" de RadLog; sozinho, é o ícone do programa, da barra
  de tarefas e da bandeja (com a bolinha amarela, laranja ou vermelha em
  atenção, alarme ou perigo, como antes). Nos tamanhos pequenos o traço é
  mais grosso para continuar legível.

## 2.8.0
- **Tela nova: menu na lateral e os aparelhos sempre em cima.** As abas e
  a coluna da esquerda deram lugar a um menu lateral (Análise: Gráficos,
  Dose, Resumo; Registros: Alertas, Medir objeto, Fora de casa, Chuva;
  Aparelhos: GMC-800, RadiaCode) e cada item abre uma página só dele, sem
  precisar rolar por tudo. Baixar histórico, Modo demo, Eventos, Exportar
  CSV e Configurações ficam no pé do menu. Em janela estreita o menu vira
  uma faixa em cima.
- **Painel dos aparelhos.** No topo de todas as páginas há um quadro para o
  GMC-800, um para o RadiaCode (quando ele está ou já esteve conectado) e
  um dos alertas. Cada quadro do aparelho mostra o valor em µSv/h, CPM e
  CPS embaixo, a dose de hoje, a última hora num gráfico pequeno e a régua
  das faixas. Clicar no valor abre a página do aparelho, com o quadro
  completo (σ, faixas, médias), a memória do contador e o fundo diário.
- **µSv/h como padrão nos dois aparelhos.** Quem estava em CPM passa para
  µSv/h uma vez; depois vale a unidade que escolher na página do aparelho.
- **Cor do nível só onde importa.** Na régua acende só a faixa em que a
  leitura está (seguro, atenção ou alerta); as outras ficam apagadas. Em
  atenção, alerta ou perigo a borda do quadro e a palavra do estado ficam
  amarelas, laranja ou vermelhas, e o quadro dos alertas mostra o nível, o
  aparelho, desde quando e o pico. O menu mostra quantos alertas estão
  ativos e uma bolinha no GMC-800 quando a memória do contador pede
  atenção.

## 2.7.3
- **Importar log do RadiaCode: novo formato aceito.** A exportação com a
  unidade no cabeçalho ("Time;Timestamp;Event;Count Rate, cps;Dose rate,
  Sv/h;…;Dose, Sv;Temperature") dava "nenhum registro reconhecido". Agora o
  app acha as colunas pelos dois jeitos de nomear ("Real time"/"Time",
  "Temp."/"Temperature", "Duration of d.a."/"Duration of dose
  accumulation, sec."), usa a unidade do cabeçalho quando o valor vem só
  com o número (Sv/h e Sv viram µSv/h e µSv), lê a hora com milissegundos e
  aceita o Timestamp em decimal.

## 2.7.2
- **Corrigido o degrau falso no dosímetro do RadiaCode.** A curva da dose
  do aparelho tratava qualquer queda do total como "aparelho zerado" e
  somava o total inteiro de novo (e o tempo contando inteiro, o que também
  subia o fundo e o teto). Uma queda pequena acontece quando dois registros
  do mesmo aparelho discordam por pouco (por exemplo, um total estimado num
  PC e o lido logo depois). Agora só conta como zerado quando o tempo
  contando e a dose caem para menos da metade; quedas pequenas não somam
  nada. Vale para o gráfico, o total do período, o fundo, o teto e a "Dose
  hoje" do aparelho.

## 2.7.1
- **Sincronização entre PCs agora leva o RadiaCode.** Até aqui ela só
  levava os minutos do GMC, os alertas, os períodos, as medições e as
  anotações; os dados do RadiaCode ficavam só no PC em que ele estava
  conectado. Agora vão também:
  - os minutos, a dose e o log do aparelho, comparando um resumo por dia de
    cada arquivo (só os dias diferentes são enviados);
  - os espectros por minuto e os dados brutos já fechados (o arquivo que
    ainda está sendo gravado vai na sincronização seguinte, depois de
    fechado);
  - a atividade (Bq) e a dose acumulada do aparelho;
  - o histórico de configurações e as versões da calibração do GMC (um PC
    que nunca leu o contador passa a usar a curva lida pelo outro);
  - os anexos das medições.
- O resultado da sincronização mostra quantos registros do RadiaCode
  entraram. Os dois PCs precisam estar na 2.7.1; com um PC em versão
  anterior, a sincronização continua só com o que já levava.

## 2.7.0
- **Fotos e anexos nas medições de objeto:** em Medir objeto → Salvas,
  "Anexar foto ou arquivo" guarda fotos (do objeto, da posição no tubo) e
  documentos junto da medição, em `medicoes-anexos/`. As fotos aparecem em
  miniatura; qualquer anexo abre no programa padrão. Entram no backup (e
  vão para a medição certa no outro PC) e são listados no PDF da medição.
  Apagar a medição apaga os anexos.
- **Marcar como não comparável**, com o motivo (outro recipiente, outra
  posição…). A marca aparece na lista, no detalhe e no PDF.
- **Outras medições deste objeto:** as medições com o mesmo nome aparecem
  juntas, com o líquido ± incerteza, e o app diz quais não dá para comparar
  direto (marcadas, outro aparelho, outra distância ou outra configuração).
- **Configuração do contador na época:** cada medição do GMC mostra a
  calibração e o tempo morto que valiam quando foi feita (pelo histórico de
  configurações) e avisa se mudaram desde então; vai também para o PDF.
- **Sincronização contador × app:** o app guarda cada download da memória
  (`contador-sincronizacao.json`) e mostra um estado claro: em dia,
  pendente (o app ficou sem leitura depois do último download e a memória
  do contador pode ter esses minutos), sem gravação ou nunca baixado. Fica
  em Configurações → Contador, no botão "Baixar histórico" (marcado quando
  há o que buscar) e num aviso na tela principal.
- **Aviso de memória quase cheia:** o uso da memória é estimado a partir do
  último download e do modo de gravação; a partir de 80% (e de novo em 95%)
  o app avisa uma vez, com quanto falta para encher.

## 2.6.0
- **Relatório: configurações vigentes no período.** Nova tabela com o valor
  de cada ajuste no início do período e as mudanças dentro dele (data, de →
  para, origem), tirada do histórico de configurações. Ajustes sem registro
  da época aparecem em "Configurações e método" marcados como "valor atual".
- **Limites da época:** o limite de cada episódio e o "tempo acima dos
  limites" usam o valor vigente em cada momento (com "*" quando não há
  registro e foi usado o valor atual; quando o limite mudou no período, a
  tabela mostra "≥ 40 → 60 CPM").
- **Medido × calculado em cada número:** marcas M (medido pelo contador), C
  (calculado pelo app), R (informado pelo RadiaCode) e E (estimativa ou
  extrapolação) nos cartões, na estatística e nos cabeçalhos das tabelas,
  com a legenda logo abaixo dos cartões. No RadiaCode, a dose acumulada pelo
  aparelho e a calculada pelo app aparecem separadas.
- **Regra dos episódios** escrita no relatório (histerese de 90%, alertas que
  se encostam em até 1 min, continuação após desconexão de até 15 min,
  episódios do histórico do contador), e marca (d) nos episódios com
  interrupção de leitura.
- **Histórico disponível (maturidade):** tabela com 24 h, 7, 30, 90 dias e
  1 ano até o fim do período, para o GMC e o RadiaCode.
- Textos do relatório revistos para um tom técnico e formal (método,
  limitações, dispersão, lacunas).

## 2.5.0
- **Padrões do fundo** (novo quadro em Gráficos): mediana por hora do dia,
  com a faixa do 1º ao 3º quartil e a média; dia da semana; mês; e dia ×
  noite. Janelas de 30, 90 dias e 1 ano, para o GMC (CPM) ou o RadiaCode
  (µSv/h). Só os minutos em casa, sem objeto encostado e sem minutos
  saturados ou com interferência.
- **Dia × noite pareado:** cada dia é comparado com a própria noite (6 h ou
  mais de cada metade), e só diz que a noite fica acima (ou abaixo) quando a
  diferença passa de 3 vezes a incerteza e de 2%.
- **Mapa de calor do RadiaCode:** o "Padrão por hora do dia" ganha a escolha
  GMC-800 / RadiaCode, com os alertas de cada aparelho.
- **Mínimo e média do episódio:** o detalhe do alerta mostra mínimo, média
  (e mediana) das leituras de dentro do episódio inteiro, com quantos
  segundos ou minutos entraram.
- **Alerta com o aparelho desconectado:** o alerta agora guarda por que
  terminou ("contador desconectado", "RadiaCode desconectado", "app
  fechado"; nova coluna no `eventos.csv`). Se o aparelho volta em até 15 min
  e a leitura ainda está alta, o alerta novo continua o mesmo episódio, com
  o trecho sem leitura marcado (ele não entra no mínimo nem na média). Se
  não volta, o episódio fica marcado como "terminou sem leitura", sem
  fingir que a contagem voltou ao normal. Fechar o app não emenda episódios.

## 2.4.0
- **Fundo ao longo do tempo** (novo quadro em Gráficos, abaixo do padrão por
  hora): média e mediana móveis de 24 h do fundo em casa, em janelas de 7,
  30, 90 dias e 1 ano, com a média de cada dia (sem os 5% extremos) em
  pontos. Usa só os minutos em casa, sem medição de objeto e sem minutos
  saturados ou com interferência; a janela móvel só aparece com pelo menos
  6 h medidas nas 24 h. O trecho antes do primeiro dado fica marcado e a
  janela aparece como "em formação".
- **Variação dentro de cada dia:** barras com a dispersão de cada dia (DP
  real ÷ √média), o "sigma ao longo do tempo"; acima de 1,2 o dia variou
  além do acaso.
- **Aviso de tendência, sem conclusão automática:** com 7 ou mais dias
  completos (12 h medidas), procura rampa (reta pela média de cada dia) e
  degrau (dois níveis). Só avisa quando a mudança passa de 5% do nível e é
  grande perto da variação entre os dias (3σ na rampa, 4σ no degrau); senão
  diz "estável", com a inclinação e a incerteza. O texto lembra que estação,
  chuva, ventilação e o lugar do contador também mudam o fundo.

## 2.3.0
- **Cobertura com denominador claro.** A % e o "n de N min" agora usam o
  mesmo denominador e vão só até agora (num período em andamento a % ficava
  menor que n ÷ N). O tempo antes do primeiro dado do aparelho não conta mais
  como lacuna: "Este mês" com 7 dias de histórico mostra a cobertura desses 7
  dias, e ao lado aparece também a % do período inteiro. Vale para o Resumo
  (GMC e RadiaCode), o dosímetro e o relatório em PDF.
- Lacunas: o trecho antes do primeiro dado sai da lista e ganha uma linha
  própria ("o período começa X antes do primeiro dado; esse trecho não conta
  como lacuna").
- **Histórico disponível** (no fim do Resumo): para 24 h, 7, 30, 90 dias e
  1 ano, se o histórico já cobre a janela inteira ("disponível"), se ainda
  está "em formação" (quantos dias de quantos) ou "aguardando dados", e
  quanto foi medido, para o GMC e o RadiaCode.

## 2.2.0
- **Calibração do GMC com versões** (Configurações → Calibração do GMC).
  Cada mudança vira uma versão com a data em que passou a valer, guardada em
  `calibracao-versoes.json` (entra no backup). O µSv/h e a dose de cada
  minuto saem da versão que valia naquele minuto: mudar a calibração hoje não
  muda o que já foi medido. Gráficos, resumo, mapa de calor, dosímetro,
  relatório e exportação seguem essa regra.
- **Fator informado à mão:** um µSv/h por CPM no fundo (por exemplo, o que
  bate com o RadiaCode) escala a curva do contador a partir de agora. Com o
  RadiaCode ligado, a tela sugere o fator das últimas 6 h lado a lado.
  "Voltar à curva do contador" cria outra versão, também a partir de agora.
  Se a curva lida do contador mudar, vira uma versão nova (o fator manual é
  mantido).
- **Recálculo retroativo opcional:** aplica uma versão também antes da data
  dela. Antes de aplicar, mostra a comparação (dose, média e máximo antes e
  depois). Nenhuma leitura gravada muda e dá para voltar ao original.
- **Origem da dose sempre indicada.** GMC: "dose calculada pelo app", com a
  calibração do período na nota do dosímetro e no relatório. RadiaCode:
  "Dose hoje" e o resumo passam a usar a dose somada pelo próprio aparelho
  (inclui o tempo longe do PC) quando o registro dos totais cobre o período;
  senão, a do app, marcada como tal.
- CSV exportado: nova coluna `calibracao` (versão e fator de cada minuto,
  com `manual` e `recalculado` quando for o caso).
- **Teste da dose do GMC** (Configurações → Calibração do GMC → Testar o
  contador): só leitura. Lê a configuração duas vezes com 90 s de intervalo e
  tenta comandos `<GET…>>` candidatos; grava
  `diagnostico-dose-gmc-….txt` na pasta de dados.

## 2.1.0
- **Cada minuto do GMC guarda de onde veio.** O arquivo de leituras ganha
  colunas: `cpm_bruto` (o que o contador mandou), `cpm_corrigido` (do tempo
  morto), `tau_us`, `tm` (correção ligada ou desligada) e `obs` (marcas). A
  coluna `cpm` continua sendo o valor usado, então versões antigas continuam
  lendo os arquivos. Os minutos gravados antes ficam como estavam (até a 2.0
  a correção nunca foi ligada: o gravado é o bruto).
- **Marcas no minuto:** `seg` (veio de gravações por segundo da memória do
  contador), `sat` (o contador saturou) e `interf` (segundo suspeito de
  interferência). O CSV exportado traz as mesmas colunas.
- **Histórico de configurações** (Configurações → Histórico de
  configurações): limites, tempo morto, calibração do GMC, saturação, modo
  de gravação da memória e mudanças feitas no contador pelo app, com data, o
  valor antes e depois e quem mudou. Na primeira abertura grava o estado
  atual. Fica em `configuracoes-historico.jsonl` e entra no backup (e na
  junção de backups, sem repetir).
- **Segundos dos alertas do GMC:** agora a partir de 10 min antes do disparo
  (eram 2) até a contagem voltar ao fundo (dentro de 2σ por 5 min seguidos),
  no máximo 30 min depois do fim (eram 2). A gravação acontece de minuto em
  minuto enquanto o alerta está aberto, então alertas longos não perdem o
  começo.
- Relatório em PDF: nova linha "Registro de cada minuto" em Configurações e
  método (quantos minutos têm bruto e corrigido, quantos são de antes da 2.1
  e as marcas).

## 2.0.0
- **GMC Monitor agora é RadLog.** Nenhum cálculo muda nesta versão: é só o
  nome e a mudança de pasta, para um problema de migração não se misturar com
  um problema de número.
- **Pasta de dados:** na primeira abertura, sem a versão antiga aberta, o app
  copia `%AppData%\GMC Monitor` (no Linux, `~/.config/GMC Monitor`) para
  `RadLog`, confere arquivo por arquivo, grava um backup completo da pasta
  antiga em `RadLog\migracao\` e só então apaga a antiga. O que foi feito
  fica em `migrado-de-gmc-monitor.json` e no log. Se algo falhar, nada é
  apagado: o app segue na pasta antiga e tenta de novo na próxima abertura.
  Caches da tela (que se refazem) não são copiados; o idioma escolhido vem
  junto.
- **Relatórios:** `Documentos\GMC Monitor\Relatórios` vira
  `Documentos\RadLog\Relatórios` (os PDFs vão junto, nada é sobrescrito).
  Versões novas passam a ser baixadas em `Documentos\RadLog\Versões`; as
  versões antigas (GMC-Monitor-v…) saem sozinhas como antes.
- **Iniciar com o Windows:** o item "GMC Monitor" é trocado por "RadLog"
  (mantendo o desativado do Gerenciador de Tarefas, se for o caso).
- **Backup:** passa a se chamar `radlog-backup-….zip`; os backups da 1.x
  continuam sendo aceitos. Um backup da 2.0 importado numa 1.x é recusado.
- **Arquivos da release:** `RadLog-vX.exe`, `RadLog-vX-x86_64.AppImage` e
  `radlog-vX-linux-x86_64.tar.gz` (o `install.sh` tira o GMC Monitor instalado
  antes). A 1.x se atualiza sozinha para a 2.0.

## 1.x
As versões 1.x (quando o app se chamava GMC Monitor) estão em
[CHANGELOG-1.x.md](CHANGELOG-1.x.md).
