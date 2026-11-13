# Versões

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
