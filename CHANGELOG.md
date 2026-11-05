# Versões

## 1.18.2
- **Releases automáticas**: toda versão nova que chega ao GitHub vira release
  sozinha (com o .exe, o AppImage, o .tar.gz e o SHA256SUMS.txt), e o app
  avisa da atualização.
- Em Configurações → Atualizações, quando esta versão é mais nova que a
  última publicada, o texto diz isso em vez de "você está na versão mais nova".

## 1.18.1
- **Conectar vai direto para a Conexão**: o botão "Conectar" da tela inicial
  e o indicador de conexão no topo abrem as Configurações já na seção
  Conexão (porta), em vez da última seção aberta.

## 1.18.0
- **Atualização pelo app, com clique**: o app olha as releases do GitHub ao
  abrir e uma vez por dia. Com versão nova, aparece "vX disponível" no topo;
  clicando, você vê as notas e escolhe **Atualizar**, **Agora não** ou **Pular
  esta versão**.
- Ao atualizar, ele baixa o .exe (ou o AppImage) para a mesma pasta, confere o
  SHA-256 com o SHA256SUMS.txt da release (arquivo que não confere é apagado e
  não abre) e abre a versão nova, que assume no lugar desta como quando você
  abre um .exe novo à mão.
- **Só troca com o CPM normal**: com alerta ativo, Geiger saturado, medição de
  objeto ou histórico baixando, ele baixa e espera; troca sozinho quando tudo
  voltar ao normal. Dá para cancelar a troca.
- Em Configurações → Atualizações: ligar/desligar a procura e "Procurar agora".
- No Linux funciona com o AppImage; instalado pelo .tar.gz, ele avisa e manda
  para a página.
- As releases passam a trazer o SHA256SUMS.txt.

## 1.17.1
- **Alarme e perigo não abrem mais juntos**: quando o CPM pula vários níveis
  no mesmo segundo (ex.: de 20 para 1.000), só o nível mais alto é aberto. Se
  o valor descer só até um nível de baixo, ele abre nessa hora (perigo →
  alarme → atenção).
- **Alertas ativos com um quadro só**: numa subida aos poucos, o painel mostra
  só o nível mais alto, com a hora em que os de baixo começaram ("atenção
  desde 00:11 · alarme desde 00:16").

## 1.17.0
- **Lacunas no Resumo**: lista dos trechos de 10 min ou mais sem leitura no
  período, com a duração e a causa provável (contador desconectado, porta
  travou, app fechado e sem histórico do contador, ou sem leitura até agora).
  Clicar abre o trecho no gráfico.
- **Cobertura**: o quadro "Medido" mostra "X de Y min" e, em períodos de mais
  de um dia, quantos dias tiveram dado.
- **Dispersão no Resumo**: desvio-padrão real dos minutos ÷ o que o acaso
  explica (√média), só com o fundo da casa (sem fora de casa nem medição de
  objeto). Perto de 1 é normal; de ~1,2 para cima, algo variou além do acaso.
- **Pico suspeito de interferência**: um segundo com contagens quase
  impossíveis pelo acaso enquanto os 20 s em volta estão normais vira um aviso
  no registro (nível WARN). Radiação de verdade sobe vários segundos seguidos;
  ruído elétrico (celular encostado, fonte, motor) costuma ser um segundo
  isolado. Se um alerta começou por causa desse pico, ele ganha a anotação
  "possível interferência". No máximo um aviso a cada 5 min (os outros somam).
- O quadro Alertas do Resumo chama o pico de "pico ao vivo", como o relatório.

## 1.16.0
Relatório em PDF, a partir da revisão de um relatório antigo:
- **Máximo com uma definição só**: no resumo e na tabela por dia é o maior
  valor visto, igual ao app, contando o pico ao vivo dos alertas. Quando o
  pico ao vivo passa do maior minuto gravado, o valor vem marcado com * e o
  maior minuto aparece junto. Antes o resumo mostrava 124 e a tabela 114. A
  estatística continua só com os minutos gravados e diz isso no rótulo.
- **Pico ao vivo** explicado em "Configurações e método": o contador soma os
  últimos 60 s, o app lê a cada segundo, e essa janela não coincide com o
  minuto do relógio que fica gravado.
- **Episódios**: duração exata (ex.: 4 min 37 s; os do histórico do contador
  ficam "≈", porque são de minuto a minuto) e um **gráfico de 30 min antes e
  depois** de cada um (até 12), com os minutos gravados, a leitura ao vivo
  por segundo quando o app gravou, o limite, o fundo e o pico.
- **Fundo da casa: agora × 7 dias × 30 dias**: mediana da última hora, de 7 e
  de 30 dias, com µSv/h, minutos usados e a média do período comparada a cada
  uma.
- **Medições de objeto** completas: contagens e tempo do fundo usado e do
  objeto, líquido com objeto ÷ fundo e σ, e a anotação (para a geometria). A
  dica do campo de anotação na aba Medir sugere o que escrever.
- O título de uma seção não fica mais sozinho no pé da página.

## 1.15.0
- **Padrão por hora por mês**: novo botão "Mês" no mapa de calor, com ‹ › para
  andar entre os meses. Dá para ver qualquer mês desde a primeira leitura
  guardada (antes só ia até 30 dias para trás). O mês atual vai até hoje, e
  selecionar um trecho do mês abre no gráfico e no relatório como antes.

## 1.14.2
- **Tema da tela do firmware 1.23**: em Configurações → Contador, o antigo
  "modo escuro" virou um seletor de tema. No Re1.23 são 5 opções (Light,
  BRY, BGY, BBY, Dark), gravadas no byte 129 da configuração; firmwares
  antigos continuam com Escuro/Claro.
- A dica do campo Série (USB) agora explica que é esse o número que a GQ usa
  para gerar a atualização de firmware.

## 1.14.1
- **Calibração de fábrica certa**: a reserva do app (usada só se ele nunca
  leu a do contador) passa a ser a do GMC-800 formatado, 986 CPM = 10 µSv/h
  no 1º ponto (≈ 0,0101 µSv/h por CPM). Antes era 1538 CPM = 10 µSv/h
  (0,0065), de um firmware antigo.
- **Tempo morto sem compensar duas vezes**: a curva de fábrica já compensa
  as contagens perdidas em taxa alta. Com a correção de tempo morto ligada, o
  CPM gravado já é a contagem verdadeira, então o app converte em µSv/h pela
  reta do 1º ponto. O Geiger saturado continua olhando o que o contador
  mandou, com a curva dele. No fundo da casa não muda nada; perto de
  100 µSv/h a diferença é de uns 3%.
- A tela usa o fator em uso no app (inclusive o último lido, com o contador
  desconectado) em vez de 0,0065 fixo.

## 1.14.0
- **Relatório técnico em PDF**:
  - **gráfico com todas as leituras por minuto**, em vez da média por
    trecho. Em períodos longos cada coluna mostra a faixa do menor ao maior
    minuto (nenhum pico some) e a média; trechos sem dado ficam cinza,
    episódios de alerta com o fundo da cor do nível, e um pico isolado muito
    alto aparece cortado com o valor escrito, para não achatar o resto;
  - **estatística das leituras por minuto**: n, média, mediana,
    desvio-padrão real, σ esperado só pelo acaso (√média), dispersão
    (real ÷ acaso) e percentis P5, P25, P50, P75 e P95;
  - **histograma** dos minutos por faixa de CPM com a curva de Poisson da
    mesma média;
  - **episódios de alerta** com duração, mínimo/média/máximo dos minutos,
    pico, limite e quanto passou dele;
  - **tempo acima de cada limite operacional**;
  - **lacunas**: tempo total sem leitura e os trechos de 10 min ou mais;
  - cobertura escrita como "X de Y min" e dias com dado;
  - **configurações e método** (calibração e de onde veio, tempo morto,
    fundo de referência, limites, como a dose é calculada) e **limitações**.
- **Calibração guardada**: a última calibração lida do contador fica em
  `calibracao.json` e vale enquanto ele estiver desconectado. Antes, sem o
  contador, o app voltava para a de fábrica (0,0065 µSv/h por CPM) e a dose e
  os relatórios saíam com outro fator.

## 1.13.4
- **Código reorganizado em pastas** (para o GitHub), sem mudar nada no
  funcionamento: o app foi para `internal/app`, o gerador de PDF virou o
  pacote `internal/pdf`, ícones e arquivos de empacotamento em `build/`,
  scripts em `scripts/` e o roadmap em `docs/`. Na raiz ficam só o `main.go`,
  o `go.mod` e os documentos.

## 1.13.3
- Padrão por hora: o contorno do quadrado com o mouse em cima (ou
  selecionado) não é mais cortado na última linha e na última coluna.

## 1.13.2
- Correção da 1.13.1: o GMC-800 manda mesmo 7 bytes no GETSERIAL
  (ex.: `0A 1B 2C 3D 4E 5F 60`); o código de 12 bytes da tela "Sobre" é outro e
  não sai pela USB. O campo virou "Série (USB)", com uma dica dizendo que
  para a GQ vale o código da tela.
- O aviso "baixe o histórico uma vez" (memória do contador) não aparece mais
  por causa da mudança no jeito de escrever o número de série.

## 1.13.1
- O número de série é lido inteiro (o que vier até a porta ficar quieta, e
  não um tamanho fixo) e mostrado em bytes separados, como na tela do
  contador.

## 1.13.0
- **Relatório de um trecho**: com um trecho selecionado no gráfico do CPM
  (arrastando) ou no padrão por hora (clicando nos quadrados), a barra do
  zoom ganhou "Relatório (PDF)" só daquele trecho.
- **Pasta dos relatórios**: os PDFs vão direto para uma pasta, sem perguntar
  onde salvar. Padrão: Documentos\GMC Monitor\Relatórios. Dá para trocar em
  Configurações → Relatórios (Escolher pasta, Abrir pasta, Voltar para a
  padrão). Não sobrescreve: um nome repetido vira "-2", "-3"...
- O nome do arquivo diz o período (ex.: `gmc-relatorio-semana-passada-2026-10-26_a_2026-11-01.pdf`,
  `gmc-relatorio-trecho-selecionado-2026-11-03_14h00-15h00.pdf`).

## 1.12.0
- **Relatório em PDF** (antes era uma página HTML): Resumo → "Relatório
  (PDF)" salva o arquivo e abre no leitor padrão. Mesmo conteúdo (cartões,
  gráfico do CPM, tabela por dia, alertas por episódio, medições de objeto),
  em A4, com cabeçalho e número de página. Feito pelo próprio app, sem
  biblioteca de fora, nas fontes padrão do PDF (abre em qualquer leitor).

## 1.11.1
- **Log da leitura ao vivo mais técnico**:
  - ao iniciar: porta, comandos (GETCPM + GETCPS a cada 1 s), prazo por
    comando e se o tempo morto está ligado (com o τ);
  - ao encerrar: quanto tempo durou, quantas leituras, quantos erros e o
    motivo (app fechando, Desconectar, porta travada, porta sumiu, 3 erros);
  - no primeiro erro, o que a mensagem do Windows quer dizer na prática
    ("Access is denied" = o adaptador USB sumiu; "not functioning" = o driver
    USB parou de responder);
  - depois de 3 erros, uma linha diz se a porta sumiu da lista do Windows
    (cabo desconectado ou USB caiu).
- "Access is denied" deixou de ser ERROR: é o que o Windows diz quando o
  cabo sai, então fica WARN. ERROR fica para porta travada e driver parado.

## 1.11.0
- **Log técnico dentro do app** (botão Eventos → aba "Log técnico"), no
  estilo Kibana/Grafana:
  - cada linha com nível **DEBUG, INFO, WARN ou ERROR** e o componente
    (conexão, reconexão, leitura ao vivo, histórico, portas...);
  - histograma empilhado por nível; clicar numa barra dá zoom naquele trecho;
  - filtros por período (15 min a tudo), nível (com a contagem de cada),
    componente (ou clicando nele na tabela) e busca no texto, com destaque;
  - "ao vivo": atualiza a cada 3 s com a janela aberta.
  - DEBUG começa escondido: são as tentativas de reconexão a cada 10 s com o
    cabo fora, que eram a maior parte do log.
- O `log.txt` passa a gravar o nível em cada linha. As linhas antigas, sem
  nível, são classificadas pelo texto com as mesmas regras.
- A lista de eventos continua na aba "Eventos" da mesma janela.

## 1.10.1
- O brilho no topo do cartão da leitura muda com o nível: verde no normal,
  amarelo na atenção, laranja no alarme e vermelho no perigo (com transição
  suave).

## 1.10.0
- **Visual repaginado**, mais calmo: o verde forte fica para o status, o
  nível e os botões de salvar.
  - Seletores (unidade, período, dias): o escolhido fica com contorno e texto
    verdes, em vez do bloco verde cheio.
  - Abas de cima (Gráficos, Resumo...) com sublinhado na aba ativa.
  - Botões do topo discretos (a caixa aparece ao passar o mouse).
  - Textos de apoio dos cartões em fonte normal; mono só nos números.
  - Leitura atual com número mais forte, faixa de nível mais fina e um brilho
    leve no topo do cartão; cores de fundo um tom mais escuras.
  - Teto da dose num seletor pequeno, na mesma linha.

## 1.9.0
- **Relatório do período**: na aba Resumo, o botão "Relatório" salva uma
  página com o período escolhido (média, dose, fundo, mínimo e máximo,
  alertas, chuva, gráfico do CPM, tabela por dia e medições de objeto) e abre
  no navegador, pronta para guardar, imprimir ou salvar em PDF. O Resumo
  ganhou "Este mês" e "Mês passado".
- **Alertas por episódio**: atenção que vira alarme (ou perigo) e volta
  aparece numa linha só, com o nível mais alto, o começo da atenção, o fim e
  o maior pico ("atenção às 09:02 → alarme às 09:06"). O Resumo e o relatório
  contam assim também. Os alertas continuam gravados separados.
- A comparação da dose não fala mais em bananas; fica só o raio-X de tórax
  (agora também em frações pequenas, como 1/200).

## 1.8.0
Limpeza: saem três recursos que não estavam em uso.
- **GMCmap.com**: o envio das leituras ao site saiu (seção, campos e código).
- **Sincronização por pasta** (Google Drive, OneDrive...): saiu. O backup
  manual continua igual (Configurações → Backup: fazer e importar .zip, que
  junta só o que falta). A sincronização entre PCs vai voltar direto de app
  para app, com pareamento (e Tailscale para fora de casa).
- **Alarme remoto (Cardputer)**: saiu o servidor na rede de casa, a pílula do
  topo, a seção nas Configurações e o firmware. Pode voltar mais para frente
  com outra função.
- As opções antigas no `config.json` são ignoradas; o `sync.json` antigo
  fica na pasta de dados sem uso (pode apagar).

## 1.7.4
- **Configurações → Contador**: os ajustes do aparelho (memória, FET, alarme,
  som, tela, calibração) ficam na própria seção, sem abrir outra janela por
  cima. São lidos do contador quando a seção abre; continuam indo para o
  aparelho só pelo botão "Gravar no contador". Se mudar algum e clicar em
  "Salvar", o app avisa que não foi para o contador.
- Menu das Configurações: item em duas linhas ("Alarme remoto (Cardputer)")
  não encosta mais no de baixo.

## 1.7.3
- **Leitura atual**: a dose de hoje saiu da fileira de baixo e foi para o
  canto de cima, ao lado do número. A fileira ficou com média, mínimo e
  máximo.
- O link da régua foi para baixo da barra de faixas ("ver na régua"), no canto
  direito.

## 1.7.2
- **Pasta do WebView fixa**: a tela agora guarda o cache em
  `%APPDATA%\GMC Monitor\webview`. Antes o WebView2 criava uma pasta nova em
  `%APPDATA%` para cada nome de .exe (uma por versão baixada). As pastas
  antigas (`GMC-Monitor-vX.exe`) são apagadas sozinhas 30 s depois de abrir;
  a que estiver em uso fica para a próxima vez. As escolhas da tela (unidade,
  período do gráfico, teto da dose) agora passam de uma versão para outra.
- **Reconexão depois de a porta travar**: espera 3 s antes da primeira
  tentativa. Antes dava "em uso por outro programa (feche o Data Viewer)" na
  hora, só porque o Windows ainda segurava a porta.
- Quando o USB do contador cai e a porta existe mas não responde, a mensagem
  diz isso ("o USB do contador travou; tire e recoloque o cabo") em vez de
  "Invalid serial port".

## 1.7.1
- **Régua**: entram os três maiores acidentes, em vermelho.
  - Taxa: Goiânia 1987 (a cápsula de césio-137 a 1 m, 4,56 Gy/h), Chernobyl
    1986 (o "pé de elefante", ~10.000 R/h ≈ 100 Sv/h em dez/1986) e
    Fukushima (contenção do reator 2, ~530 Sv/h estimados em 2017).
  - Dose: Fukushima (maior dose de um trabalhador, 678,8 mSv), Goiânia (as
    4 vítimas fatais, 4,5 a 6 Gy) e Chernobyl (134 com síndrome aguda, até
    16 Gy).

## 1.7.0
- **Régua de radiação**, para ter noção de grandeza: duas escadas em escala
  logarítmica, uma de taxa (µSv/h) e uma de dose (µSv), com referências
  conhecidas (banana, raio-X de tórax, avião, tomografia, limites do público
  e do trabalhador, efeitos agudos) e as suas próprias (fundo da casa, um dia
  e um ano de fundo, objetos medidos e salvos). A sua leitura de agora e a
  dose do período entram no meio, com quantas vezes cada item é maior ou
  menor. Abre pelo link "régua" embaixo da leitura atual e por "ver na
  régua" no cartão de dose.
- O cartão de dose mostra uma comparação curta (ex.: "≈ 64 bananas · ≈ 1/3
  de um raio-X de tórax"), e o quadradinho "Dose hoje" mostra a mesma ao
  passar o mouse.

## 1.6.2
- **Configurações repaginadas**: janela maior, com as seções numa lista à
  esquerda (Geral e Níveis do app) e uma seção por vez à direita. O título
  com o X e o rodapé com "Salvar" ficam fixos, sem rolar junto. Em janela
  estreita, a lista vira uma faixa no topo. Os atalhos (chuva, alarme remoto,
  GMCmap) abrem direto na seção certa, e o app lembra a última seção aberta.

## 1.6.1
- **Abrir uma versão nova com a antiga na bandeja**: antes, o Windows
  mandava o pedido para a cópia aberta, que só mostrava a janela dela (a
  versão velha). Agora cada cópia diz a versão e o .exe dela; se for outra,
  a antiga abre a nova e fecha sozinha (a nova espera ela soltar a porta).
  Vale a partir da próxima atualização: a 1.6.0 e anteriores ainda não sabem
  fazer isso, então desta vez feche a antiga pelo ícone da bandeja → Sair.
- **Iniciar com o Windows**: as Configurações mostram o que está registrado e
  avisam se o registro aponta para um .exe que não existe mais, para outra
  cópia, ou se o Windows desativou o app em Gerenciador de Tarefas →
  Aplicativos de inicialização. Um botão corrige (registra este .exe e tira a
  trava do Gerenciador). Ligar a opção também tira essa trava. O log técnico
  anota o estado a cada abertura e se a cópia abriu com o Windows.

## 1.6.0
- **Compensação de tempo morto** (Configurações → Níveis do app → Tempo
  morto do tubo), desligada por padrão. Usa a mesma conta do Rad Pro:
  real = medida ÷ (1 − medida × τ), com o fator limitado a 10×. τ padrão de
  200 µs (típico do M4011, o tubo do GMC-800), ajustável de 5 a 500 µs. Uma
  tabela mostra na hora quanto a correção muda cada taxa (no fundo, +0,009%;
  em 30 mil CPM, +11%).
- Vale para a leitura ao vivo (CPM e CPS), alertas, gravação e o histórico
  baixado do contador daqui em diante. O que já está gravado não muda. A
  detecção de "Geiger saturado" continua olhando o valor do próprio contador.
- Com a correção agindo, a leitura mostra quanto ela somou e o CPM original
  ("tempo morto +11% (contador: 30.000 CPM)").

## 1.5.1
- **σ de volta**, para aprender a ler os números:
  - embaixo da leitura atual: quantos σ ela está do fundo, o fundo e o
    tamanho de 1σ (ex.: "+0,6σ · fundo 28 CPM · 1σ = 5,3 CPM");
  - **"o que é σ?"** abre uma explicação com os seus números: como se
    calcula, a leitura de agora em σ e uma tabela de quão raro é passar de
    +1σ a +5σ só por acaso (Poisson do seu fundo, em chance por minuto e
    "quantas vezes por dia");
  - pico dos alertas em σ (lista, alertas ativos e detalhe do alerta);
  - padrão por hora: a diferença para a média da hora também em σ (com o σ
    de uma média, que é menor);
  - a medição de objeto nos eventos mostra quantos σ deu a diferença.

## 1.5.0
- Aba **RadiaCode** com **exemplos simulados** funcionando, no mesmo formato
  de dados do RadiaCode 102 (1.024 canais com calibração de energia), para
  deixar as telas prontas antes do aparelho chegar:
  - **Espectro** acumulando ao vivo para fundo da casa, eletrodo de tório,
    detector de fumaça, sal light e chuva; escala linear/log, velocidade,
    fundo esperado por cima e cursor que diz qual isótopo fica em cada pico.
  - **Espectrograma** de 3 horas simuladas (chuva e eletrodo perto).
  - **Dureza** (fótons acima de 1.500 keV sobre 150 a 1.500 keV, relativa ao
    fundo) junto da contagem, com palpite da fonte pelo formato do espectro.
  - **Log** de exemplo, **modo busca** com ponteiro, escala automática,
    distância, tendência e cliques de som, e **atividade com KCl** com
    atividade esperada, incerteza pelo tempo de medição e o pico do K-40.

## 1.4.1
- **Medir objeto mostra as contas**: tabela com contagens, tempo, CPM médio
  e incerteza de Poisson (1σ) do fundo e do objeto, mais a diferença com a
  incerteza combinada e quantos σ ela vale. Nas medições salvas, botão
  "Detalhes" abre a mesma tabela.
- Durante a medição, o andamento mostra também a taxa e a incerteza até o
  momento (ex.: "122/300 s · 210 contagens · 103,3 ± 7,1 CPM").

## 1.4.0
- Nova aba **RadiaCode** (em construção): mostra o que o app vai fazer com o
  RadiaCode 102 conectado, na ordem em que vai ficar pronto: espectro,
  espectrograma, dureza nos gráficos, importar o log, modo busca e atividade
  com KCl. Ainda não conecta no aparelho.

## 1.3.2
- **Padrão por hora do dia**: na caixinha que aparece ao passar o mouse, os
  alertas da hora vêm do mais perigoso para o menos (perigo, alarme,
  atenção); no mesmo nível, o de maior pico primeiro.

## 1.3.1
- **Alertas ativos**: durante um alarme (ou perigo), a atenção não aparece
  mais como um segundo alerta ativo. Fica só o nível mais alto, com
  "atenção desde HH:MM" na linha dele. O histórico de alertas não mudou.

## 1.3.0
- **Sincronização mais rápida**:
  - quando o contador desconecta (sozinho ou pelo botão), o app grava o
    backup na pasta de sincronização na hora;
  - ao abrir, o app junta o que os outros PCs mandaram em 10 s (antes, 1 min);
  - a cada 2 min ele olha se chegou arquivo novo de outro PC e junta (antes,
    só a cada 30 min). O backup deste PC continua a cada 30 min e ao fechar.

## 1.2.3
- Medições salvas: ao clicar em "Editar", os campos de nome e anotação
  passavam da largura do cartão e criavam uma barra de rolagem lateral.
  Agora eles encolhem para caber, com o botão "Salvar" sempre visível.

## 1.2.2
- O **alarme remoto (Cardputer)** fica só na versão Windows: no Linux a opção
  some das Configurações, o indicador do cabeçalho não aparece e a
  transmissão na rede não liga.
- **GMC Remoto 1.4.0** (firmware do Cardputer), focado em bateria:
  - CPU a 80 MHz (era 240) e Wi-Fi em modo de economia (o rádio dorme entre
    os beacons do roteador; antes ficava ligado o tempo todo). Se o roteador
    não entregar o broadcast, o pedido direto ao PC continua cobrindo.
  - A tela só é redesenhada quando algo visível muda (antes, 5 vezes por
    segundo); bateria lida a cada 30 s e sinal do Wi-Fi a cada 5 s.
  - Tela fraca depois de 20 s sem tecla e **desligada depois de 1 min** com
    tudo normal. Alerta, perda de contato, contador desconectado ou mudança
    de nível acendem de novo. Com a tela desligada, a primeira tecla só
    acende.
  - Aviso "estou escutando" para o PC a cada 20 s (era 10 s).
  - Bateria em vermelho no canto quando está em 15% ou menos.

## 1.2.1
- **Versão para Linux** (x86_64): binário pronto, instalador para o menu de
  aplicativos (`install.sh`) e script que gera o **AppImage**
  (`build-appimage.sh`). Precisa da WebKitGTK 4.1 no sistema.
- No Linux, as opções que só existem no Windows (bandeja, iniciar com o
  sistema) ficam escondidas nas Configurações.

## 1.2.0
- Anotação de **trecho**: selecione um trecho no gráfico (arrastando) e
  clique em **"Anotar este trecho"** na barra do zoom. Aparece como faixa
  roxa clara entre o começo e o fim. Na janela da anotação, o campo "Até"
  vazio continua sendo um momento só.

## 1.1.0
- **Anotações no gráfico**: "+ anotar" embaixo do gráfico de CPM, ou dois
  cliques no gráfico para anotar naquele momento. Aparecem como linha
  tracejada roxa com uma marca no topo (clique para editar ou apagar) e no
  texto ao passar o mouse. A aba Resumo lista as anotações do período. Ficam
  em `anotacoes.json` e vão no backup/sincronização.

## 1.0.0
Primeira versão "major": o app cobre o uso do dia a dia.

- **Aba Resumo**: hoje, ontem, esta semana, semana passada ou qualquer dia.
  Média (comparada ao seu normal), máximo e mínimo (com a hora), dose, tempo
  medido e fora de casa, alertas, chuva e medições, com o gráfico do período.
- **Comparar** dois períodos: hoje × ontem (até a mesma hora), 7 dias × 7
  dias antes, ou dois dias escolhidos. Curvas sobrepostas e tabela com a
  diferença.
- **Dados novos com resumo**: ao baixar o histórico, cada trecho novo (ex.:
  a noite dormindo) mostra máximo, mínimo, dose, alertas e chuva, além da
  pergunta "em casa / fora de casa".
- **Backup e sincronização** (Configurações → Geral): backup .zip, importar
  backup (junta só o que falta) e sincronização por uma pasta compartilhada
  (OneDrive, Google Drive, Syncthing, rede), para usar o app no Windows e no
  Linux com os mesmos dados. Cada PC grava o seu arquivo a cada 30 min e ao
  fechar, e junta os dos outros.
- **Linux**: `build-linux.sh` e instruções no README (compila no próprio
  Linux; sem bandeja).
- Ícone da bandeja também muda de cor no **modo demo** (dica com "DEMO").
- Topo: contador → Cardputer → relógio.
- Versão do .exe (propriedades do arquivo) atualizada.

## 0.25.6
- No topo, ao lado do relógio: **"cardputer · conectado"** (bolinha verde)
  ou **"sem contato"**, quando o alarme remoto está ligado. Clique abre as
  Configurações.
- Corrigido: a linha de status do alarme remoto nas Configurações dava erro
  e não aparecia (desde a 0.25.3).

## 0.25.5
- Alarme remoto: o pacote leva a hora do PC (para o relógio do Cardputer).
- Firmware GMC Remoto 1.3.0: relógio no canto superior esquerdo (hora do
  PC); etiqueta DEMO ao lado do relógio, sem repetir "DEMO" na faixa; o
  tempo de silêncio saiu da faixa (o "PERIGO - AFASTE-SE" agora cabe) e
  aparece embaixo, em amarelo, no lugar da ajuda.

## 0.25.4
- Alarme remoto: com o **modo demo** ligado no PC, a rede recebe os valores
  simulados marcados como demo (o Cardputer mostra "DEMO" e toca os alarmes
  do demo; a página do celular mostra "modo demo do PC").
- Firmware GMC Remoto 1.2.0: etiqueta DEMO.

## 0.25.3
- Alarme remoto: o IP do PC aparece em destaque nas Configurações (rede de
  casa primeiro, adaptadores virtuais por último), para digitar no
  Cardputer.
- Firmware GMC Remoto 1.1.0: tecla **I** para digitar o IP do PC, para
  quando o roteador não deixa o broadcast passar. Enquanto procura o PC, a
  tela mostra o IP do próprio Cardputer.

## 0.25.2
- **Log técnico** (`log.txt` na pasta de dados, botão "Abrir log técnico"
  em Eventos): conexões, cada erro da porta com a mensagem exata, leituras
  lentas, porta travada (com a lista de portas que o Windows via na hora),
  tentativas de reconexão, PC parado/suspenso, downloads do histórico.
  Passando de 2 MB vira `log-anterior.txt`.
- Corrigido: depois de a porta travar, a **reconexão podia ficar presa para
  sempre** (abrir a porta e o primeiro comando não tinham prazo). Agora tem
  prazo, e se a porta continuar presa o app pede para tirar e colocar o
  cabo USB.
- Aba Chuva: "Chovendo desde" só aparece enquanto o radar ainda mostra
  chuva; parou, vira "Choveu das ... às ...".

## 0.25.1
- Aba Chuva: mostra a **chuva em andamento** (ou recente) num quadro no
  topo: desde quando, intensidade, a base de antes e o CPM dos últimos
  15 min. Diz quando ela entra na conta (2 h depois do começo), quando o
  resultado fecha (4 h) ou por que não vai entrar (ex.: choveu menos de 3 h
  antes). Na lista, chuva com as 4 h ainda correndo aparece como "ainda
  medindo".

## 0.25.0
- **Alarme remoto**: Configurações → Geral → Alarme remoto. Ligado, o app
  manda a leitura atual para a rede de casa a cada segundo (broadcast UDP
  na porta 47800) e responde em `http://IP-do-PC:47800` (página que dá para
  abrir no celular) e `/agora` (JSON). É só leitura. Mostra os IPs do PC,
  quem está escutando e tem um botão para testar o alarme no Cardputer.
- Novo: firmware **GMC Remoto** para o Cardputer, na pasta `cardputer/`
  (Arduino). Mostra o CPM, toca alarme e perigo, avisa "sem contato" se
  parar de receber, silencia 5 min com M ou Enter.

## 0.24.1
- "Chuva e radiação" saiu dos gráficos e virou a aba **Chuva**, ao lado de
  "Fora de casa". A conta só roda com a aba aberta.

## 0.24.0
- **Ícone na bandeja** (perto do relógio): fechar a janela deixa o app
  rodando escondido, lendo, gravando e alarmando. A dica do ícone mostra o
  CPM atual, e o ícone ganha uma bolinha amarela, laranja ou vermelha em
  atenção, alarme ou perigo. Alarme e perigo trazem a janela de volta.
  Para fechar de vez: clique no ícone → "Sair". Dá para desligar em
  Configurações → Geral → Janela.
- **Iniciar com o Windows** (opcional, mesma seção): abre direto na
  bandeja. Abrindo uma versão nova, ela passa a ser a que inicia.
- **Chuva e radiação**: novo card nos gráficos. O app acha as chuvas dos
  últimos 60 dias (radar quando há, senão o modelo), compara o CPM das 2 h
  antes com as 4 h depois e faz a mesma conta em horários sem chuva, para
  separar o efeito da chuva da variação normal. Mostra o efeito médio, em
  quanto tempo vem o pico e em quanto tempo volta, a curva média e a lista
  das chuvas (clique para abrir o trecho nos gráficos).
- A primeira busca de chuva pega 31 dias para trás (antes 7).
- Gráfico do alerta: legendas "leitura a cada segundo" e "valor do minuto
  (gráfico principal)".

## 0.23.2
- No zoom do gráfico de CPM, trechos com alerta passam a usar as leituras
  por segundo gravadas do alerta: o pico aparece na própria linha, não só
  como ponto. O resto do trecho continua com o valor de cada minuto (em
  degraus), e a barra do zoom avisa "por segundo só perto dos alertas".

## 0.23.1
- Corrigido: o app podia **parar de atualizar** e, depois de fechado,
  **não abria mais** (o processo ficava preso no fundo). No Windows, a
  leitura ou escrita na porta serial pode travar no driver (USB suspenso,
  PC voltando da suspensão); o app esperava por ela para sempre, inclusive
  ao fechar. Agora cada leitura tem prazo de 6 s (travou: reconecta
  sozinho), fechar a porta não espera a leitura presa e o app sempre termina
  em até 5 s depois de fechar a janela. Se uma cópia travada ainda estiver
  aberta, abrir de novo a encerra.
- Quando algo trava, o app grava `diagnostico.txt` na pasta de dados.
- Corrigido: o detalhe por segundo dos alertas sumia depois de reabrir o
  app (o nome do arquivo usava milissegundos, que o eventos.csv não guarda).
  Os arquivos já gravados voltam a aparecer.

## 0.23.0
- **Aviso de "sem contagens"**: se o CPM ficar em 0 por 2 min seguidos (uns
  3 min sem nenhum pulso, praticamente impossível com o fundo normal), o
  card da leitura mostra "Sem contagens" em vermelho, aparece um aviso e
  fica registrado em Eventos. Quando voltar a contar, também registra.
  Com o Geiger saturado, quem avisa é o aviso de saturação.

## 0.22.3
- Aviso de perigo: "Saia agora" virou "Afaste-se agora".

## 0.22.2
- Os números do pico agora batem em todo lugar. O CPM do contador é a soma
  dos últimos 60 s, recalculada a cada segundo; o gráfico guarda um valor por
  minuto do relógio. O pico de um alerta é a maior leitura de segundo e pode
  passar um pouco do valor do minuto (ex.: 52 no alerta, 51 no minuto).
  Agora esse pico entra no "Máximo · 24 h" e no "máx" do gráfico, aparece
  como um ponto colorido no gráfico de CPM e no texto ao passar o mouse.
- Hora do pico dos alertas ao vivo com segundos.
- Alertas antigos diziam "(acima de 50)" mesmo com 50 CPM; agora mostram
  "(a partir de 50)", como os novos.

## 0.22.1
- A pergunta "foi fora de casa?" ao baixar o histórico agora vale para
  trechos novos a partir de **15 min** (antes 30). O tempo pode ser mudado em
  Configurações → Geral → Fora de casa (de 5 min a 4 h).

## 0.22.0
- **Gráfico de cada alerta** na tela Alertas: clicando num alerta (atenção,
  alarme ou perigo), aparece ao lado o gráfico dele, com a faixa do alerta,
  a linha do limite, o ponto do pico e alguns minutos antes e depois.
  "Ver nos gráficos" abre o mesmo trecho nos gráficos principais.
- Os alertas ao vivo guardam as leituras **por segundo** (2 min antes até
  2 min depois) em `alertas-detalhe/`, então o gráfico continua detalhado
  depois de fechar o app. Alertas antigos e os achados no histórico do
  contador mostram o valor de cada minuto.
- Clicar num alerta em "Últimos alertas" abre a tela Alertas já nele.

## 0.21.3
- Corrigido: nos gráficos de 15 e 30 min, o começo (até 1 minuto) ficava
  vazio quando o app tinha sido aberto há pouco. O minuto gravado em que o
  gráfico começa não entrava.

## 0.21.2
- Saiu o link "marcar fora de casa" embaixo do gráfico (a aba Fora de casa
  já faz isso).

## 0.21.1
- Configurações: o texto da chuva agora explica o radar (com o app aberto) e
  o modelo do Open-Meteo (horários sem radar).

## 0.21.0
- **Chuva pelo radar**, automática: a cada 10 minutos o app lê o radar
  meteorológico (RainViewer) no ponto da sua cidade e guarda a intensidade
  (fraca, moderada, forte). No gráfico, a chuva do radar aparece em barras
  azuis de 10 min; onde não há leitura do radar, ficam as barras mais claras
  do modelo (Open-Meteo).
- Ao abrir o app, ele pega as últimas 2 horas de radar. Com o app fechado,
  o radar daquele período não fica guardado (o serviço só guarda 2 h).
- O aviso embaixo do gráfico mostra "chovendo (forte)" ou "sem chuva" e a
  hora do radar. Se o radar não cobrir o seu local, avisa e usa só o modelo.
- Novo arquivo: `chuva-radar.json`.

## 0.20.4
- Corrigido: a chuva aparecia **uma hora atrasada** no gráfico. O Open-Meteo
  informa às 16:00 a chuva que caiu das 15:00 às 16:00; o app desenhava das
  16:00 às 17:00. O cache antigo (`chuva.json`) é descartado e baixado de
  novo.
- Clicar no aviso de chuva embaixo do gráfico busca a chuva **na hora** (sem
  esperar os 30 min).
- O aviso e as Configurações explicam que são dados de modelo do tempo, não
  de radar: chuva bem localizada pode não aparecer.

## 0.20.3
- Medições salvas agora guardam **cada segundo** (CPM ao vivo e contagens de
  cada fase). Clicando numa medição da lista, o gráfico "Durante a medição"
  mostra aquela medição, igual a quando estava medindo; "Voltar à medição
  atual" volta ao normal. Medições feitas antes desta versão não têm esses
  dados (o gráfico avisa).

## 0.20.2
- Saíram as dicas "arraste no gráfico para dar zoom" (CPM e dose) e "clique
  ou arraste numa linha…" (padrão por hora).

## 0.20.1
- **Gráfico durante a medição** de objeto: CPM ao vivo (linha fina) e a média
  acumulada do fundo (cinza) e com o objeto (verde-água), cada uma com a faixa
  da margem de erro, segundo a segundo. O eixo vai até o fim previsto da
  medição e o título mostra os valores atuais.
- Eventos voltou a ser janela, com o botão "Eventos" no topo.
- Saiu o link "histórico de alertas" do card de alertas (a tela Alertas faz o
  mesmo).

## 0.20.0
- **Sem modais apertados**: Medir objeto, Alertas (histórico), Fora de casa e
  Eventos viraram telas que ocupam o lugar dos gráficos, na coluna da
  direita. Uma barra no topo da coluna troca entre **Gráficos**, Medir objeto,
  Alertas, Fora de casa e Eventos; cada tela tem "Voltar aos gráficos". A
  leitura ao vivo e os alertas continuam à esquerda (dá para acompanhar a
  leitura enquanto mede um objeto).
  - Medir objeto: medição de um lado, medições salvas do outro.
  - Fora de casa: lista de um lado, formulário do outro.
  - Os botões "Medir objeto" e "Eventos" saíram do topo (estão na barra).
- As janelas que continuam como modal (Configurações, Dados novos) ficaram
  mais largas.
- Corrigido: ao abrir um trecho (Ver no gráfico, mapa de calor) o gráfico às
  vezes voltava para a última hora por causa de uma resposta atrasada.

## 0.19.0
- **Medições salvas**: toda medição de objeto terminada (fora do modo demo)
  fica guardada em `medicoes.json`, com nome, horário, as duas fases e o
  resultado.
  - No gráfico de CPM, a medição aparece marcada: faixa cinza no fundo,
    verde-água com o objeto; passar o mouse mostra "medição: nome (fase)".
  - Em "Medir objeto", a lista **Medições salvas** mostra todas, com "Ver no
    gráfico" (abre o trecho com zoom), "Editar" (nome e anotação) e "Apagar".
  - Os minutos com o objeto encostado não entram no fundo da casa (normal da
    leitura, fundo do dosímetro, fundo diário e padrão por hora).

## 0.18.0
- Depois de **baixar o histórico**, se vierem trechos que o app não tinha visto
  (30 min ou mais seguidos), aparece uma janela perguntando se cada um foi
  **fora de casa**. Cada trecho mostra a média de CPM e o seu normal em casa,
  para ajudar a lembrar. "Em casa" (dormindo, PC desligado...) só fecha a
  pergunta; "Fora de casa" pede um nome e já marca o período. Trechos já
  marcados não são perguntados de novo.

## 0.17.0
- **Fora de casa**: marque os períodos em que o contador estava fora de casa
  (passeio, viagem...). Dá para marcar direto do zoom ("Marcar como fora de
  casa", já com o trecho preenchido) ou pelo link "fora de casa" embaixo do
  gráfico de CPM, que abre a lista para ver, editar e apagar.
  - As leituras continuam guardadas e entram na **dose**.
  - Ficam fora de tudo que descreve o **fundo da casa**: o "dentro do normal"
    da leitura, o fundo e o teto do dosímetro, o fundo diário e o padrão por
    hora.
  - Nos gráficos de CPM e de dose, esses trechos aparecem com uma faixa lilás;
    no padrão por hora, as horas só fora de casa ficam lilás e as parciais
    ganham um traço lilás embaixo. Os alertas que aconteceram nesses períodos
    ganham uma etiqueta com o nome.
  - Os períodos ficam em `fora-de-casa.json`, na pasta de dados.

## 0.16.3
- Zoom: agora há **um só botão "Sair do zoom"**, que tira o zoom dos dois
  gráficos (CPM e dose) e a seleção do mapa de calor de uma vez. Ele fica no
  primeiro gráfico com zoom; o outro mostra só o "Voltar", quando houver.
  Vindo do mapa de calor, a faixa da dose diz "mesmo trecho do gráfico de CPM".

## 0.16.2
- Padrão por hora do dia: **clicar num quadrado abre aquela hora nos gráficos
  de CPM e de dose** (com zoom). Arrastando pelos quadrados de uma mesma linha
  (o mesmo dia), abre o trecho inteiro, por exemplo das 8h às 12h. Os
  quadrados escolhidos ficam contornados enquanto o zoom estiver aberto, e a
  página sobe até os gráficos se eles estiverem fora da tela.

## 0.16.1
- **Zoom também no gráfico de dose acumulada**: arraste num trecho e o gráfico
  mostra a dose só daquele período, começando do zero, com fundo e teto do
  mesmo trecho (um ponto por minuto quando cabe). "Voltar" e "Sair do zoom"
  como no gráfico de CPM.
- No zoom, o eixo do tempo mostra a data quando o trecho passa da meia-noite.

## 0.16.0
- **Zoom no gráfico de CPM**: clique e arraste de um ponto a outro para ver só
  aquele trecho, com a resolução mais fina que existe (leituras a cada 5 s na
  última hora, por minuto antes disso). A faixa do zoom mostra o período, a
  dose no trecho e de quanto em quanto tempo é cada ponto; dá para dar zoom
  dentro do zoom, "Voltar" e "Sair do zoom". Escolher 15 min/30 min/1 h/24 h/
  7 dias também sai do zoom.
- Chuva: opção **"Ocultar a cidade na tela"**. Com ela, o gráfico mostra só
  "chuva configurada · atualizada às…" e as Configurações mostram "Local:
  oculto".

## 0.15.1
- A unidade "mR/h" virou **"mrem/h"** (milirem por hora), como aparece no
  GMC-800. A conta continua a mesma e é exata: 1 mrem/h = 10 µSv/h.

## 0.15.0
- **Textos dos avisos bem mais curtos**, para ler em segundos. Os telefones de
  emergência (CNEN 24 h e Bombeiros) ficam à vista na faixa de perigo.
- Saíram as listras do modo demo: a faixa tem a mesma cor do alarme de verdade
  e o título diz "(demo)".
- Nova aba **"Níveis do app"** nas Configurações: dá para ajustar o aviso de
  atenção (padrão 50 CPM), o alarme de perigo (padrão 1.000 CPM), quanto tempo
  o "Silenciar" do perigo dura (5 min) e os limites do Geiger saturado (fim da
  escala 2.000 µSv/h, queda brusca a partir de 1.000 µSv/h, aviso por 60 s).
  Mostra quanto cada nível dá em µSv/h com a calibração atual e tem um botão
  "Restaurar padrões".

## 0.14.2
- A faixa do alarme não pisca mais (oscilava entre claro e escuro e
  atrapalhava a leitura). Ela fica com a cor fixa: laranja no alarme,
  vermelha no perigo e mais escura quando o som está silenciado.

## 0.14.1
- Enquanto um alarme toca, o gráfico de CPM mostra a linha do **seu limite**
  (laranja) e a do **perigo do app** (vermelha, 1.000 CPM), também em µSv/h.
  A linha do perigo só entra na escala quando a leitura chega perto dela (para
  não achatar o gráfico); fora isso, aparece na legenda como "acima da escala".
  As linhas somem quando o alarme acaba.

## 0.14.0
- **Geiger saturado**: em radiação extrema o tubo Geiger para de contar
  direito. O app avisa com "GEIGER SATURADO — acima do que o aparelho mede"
  quando a leitura chega ao fim da escala do GMC-800 (2.000 µSv/h, pelo
  manual), e também quando a leitura despenca de repente depois de um valor
  extremo (o tubo pode travar e mostrar menos que o real). Nesse caso o aviso
  e o alarme de perigo ficam ligados por pelo menos 60 s. A saturação fica
  anotada nos eventos.
- O alarme de perigo mostra como a leitura subiu ("subiu de 0,28 µSv/h para o
  perigo em 3 s: subida brusca…") para ajudar a saber se foi você que chegou
  perto de algo ou se algo chegou perto de você.
- Orientações do alarme com os dois casos: se você estava andando, volte por
  onde veio; se algo ou alguém chegou perto, afaste-se disso e ponha paredes
  entre vocês (ou saia do caminho, se a fonte estiver passando).

## 0.13.1
- Saiu a moldura colorida em volta da tela durante o alarme: atrapalhava a
  leitura. O alarme continua com a faixa no topo (laranja ou vermelha), o som,
  o título da janela e o número da leitura colorido.

## 0.13.0
- **Dois níveis de alarme:**
  - **Alarme** (o limite que você escolhe, ex.: 100 CPM): faixa e moldura
    **laranja**, com o som das Configurações; "Silenciar" cala até o alarme
    acabar.
  - **Perigo** (fixo em **1.000 CPM**, cerca de 10 µSv/h): faixa e moldura
    **vermelhas** piscando mais rápido e um som próprio mais agressivo. Não dá
    para desligar nem trocar o som, vale mesmo com o alarme desligado e sempre
    traz a janela para a frente. "Silenciar 5 min" cala por 5 minutos; se
    continuar alto, volta a tocar.
  - O perigo tem prioridade: se o seu limite for 1.000 CPM ou mais, as
    Configurações avisam que quem toca é o alarme de perigo do app, e a faixa
    vermelha também diz isso.
- Cores dos alertas nas listas e no mapa de calor: atenção amarelo, alarme
  laranja, perigo vermelho. Os alertas de perigo também são achados no
  histórico do contador.
- Botão "Testar alarme de perigo (5 s)" nas Configurações. No modo demo, os
  alarmes ficam listrados (a cor continua sendo a do nível).
- Texto dos alertas: "a partir de 50" no lugar de "acima de 50".

## 0.12.4
- Padrão por hora do dia: as horas em que houve alerta ficam laranja
  (atenção) ou vermelhas (alarme), inclusive os alertas achados no histórico
  do contador. A caixinha do quadrado mostra a hora e o pico de cada alerta.

## 0.12.3
- Saiu o símbolo σ. Embaixo da leitura agora aparece em palavras se ela está
  normal, comparada com a faixa em que o seu fundo costuma ficar ("Dentro do
  normal — seu fundo costuma ficar entre 11 e 29 CPM", "Um pouco acima do
  normal… pode ser só variação", "Acima do normal… dificilmente é só
  variação"). Na medição de objeto e no registro também saiu o σ.
- Busca de chuva: embaixo do gráfico de CPM aparece sempre se ela está
  desligada, buscando agora, quando atualizou ou se o serviço de clima não
  respondeu (aí espera 10 min antes de tentar de novo). Clicar em "chuva:
  desligada" abre as Configurações. As Configurações também mostram a última
  e a próxima busca.

## 0.12.2
- Mapa de calor: passar o mouse num quadrado mostra uma caixinha com o dia e a
  hora, o CPM médio, o valor em µSv/h, a diferença para a média daquela hora e
  quantos minutos foram medidos.

## 0.12.1
- Mensagem de conexão mais clara: com o contador desligado ou o cabo fora,
  o topo mostra "desconectado · COM13 não encontrada" (bolinha cinza, não
  vermelha) enquanto o app tenta de novo a cada 10 s. Porta em uso por outro
  programa e falta de permissão também ganharam frases em português.

## 0.12.0
- **Alertas retroativos**: ao baixar o histórico, o app procura nos minutos
  que ele não tinha visto (app fechado ou desconectado) os trechos acima de
  50 CPM ou do limite do alarme e anota um alerta com a data e a hora em que
  aconteceram, o pico e quanto tempo durou. Na primeira vez que esta versão
  abre, ela também confere o histórico que já tinha sido baixado antes.
- O card "Alertas" virou **"Alertas ativos"**: mostra o que está acontecendo
  agora (desde quando, valor inicial e pico) e, embaixo, os 3 últimos alertas.
- Botão **"histórico de alertas"** com todos os alertas separados por dia e
  filtros (ao vivo, do contador, só alarmes).
- Os alertas agora guardam quando acabaram e o pico. O `eventos.csv` ganhou
  colunas novas (os arquivos antigos continuam abrindo).

## 0.11.0
- Visual de volta ao da 0.10.1 (a compactação da 0.10.2 ficou mais difícil de
  ler).
- **Quão incomum é a leitura**: embaixo do número aparece quantas vezes a
  variação natural ela está acima do seu fundo (mediana dos últimos 30 dias),
  por exemplo "dentro do normal" ou "3,4 σ acima do fundo — incomum".
- **Medir um objeto** (botão no topo): mede o fundo por um tempo, depois o
  mesmo tempo com o objeto encostado, e diz se a diferença é real ou ruído, com
  a margem de erro. O resultado fica anotado nos eventos.
- **Projeção anual** no cartão de dose: quanto daria em um ano no ritmo atual.
- **Padrão por hora**: mapa de calor da média de CPM por hora × dia (7, 14 ou
  30 dias), com a hora mais alta e a mais baixa.
- **Chuva no gráfico** (opcional, nas Configurações): escolha a cidade e o app
  marca a chuva de cada hora no gráfico de CPM (dados grátis do Open-Meteo,
  guardados em `chuva.json`). Chuva costuma dar picos curtos de alguns CPM.

## 0.10.2
- Corrigido: o número da dose acumulada tinha ficado do tamanho da leitura
  principal (uma regra de estilo da 0.10.1 passou por cima), e isso criava
  rolagem em 1920×1080.
- Legendas dos cartões mais curtas, linhas do fundo diário e espaços entre os
  cartões um pouco menores: sobra folga sem rolar em 1920×1080.

## 0.10.1
- Corrigido: o ícone não aparecia na barra de título da janela (o Wails procura
  o ícone pelo ID 3 dentro do .exe; ele estava com outro nome).
- Corrigido: os botões CPM / µSv/h / mR/h / CPS mudavam de lugar quando o
  número ficava mais largo. Agora ficam sempre logo abaixo da leitura.
- Cartão da leitura e cartões de média/dose mais compactos (legendas numa linha
  só), para continuar cabendo tudo sem rolar em 1920×1080.

## 0.10.0
- Saiu o card "Contador". O modelo continua no indicador de conexão e o
  relógio ganhou um indicador próprio no topo ("relógio certo" / "relógio 5 s
  adiantado", em amarelo se estiver errado); clicar nele sincroniza com o PC
  (o botão "Sincronizar relógio" saiu do topo).
- Série, porta, calibração, o botão "Ajustes do contador" e as chaves do alarme
  e do GMCmap foram para as Configurações.
- Layout em duas colunas a partir de 1100 px: leitura, cartões (em linha única
  nas telas largas), alertas e fundo diário à esquerda; gráficos à direita.

## 0.9.0
- Corrigido: quando o alarme disparava com a janela maximizada, o app tirava a
  janela do maximizado (o "restaurar" do Windows). Agora só restaura se ela
  estiver minimizada.
- Avisos com orientação: uma frase curta sempre visível e um botão "O que
  fazer" com os passos. No alarme, as orientações mudam com o nível (abaixo de
  10 µSv/h, de 10 a 100 µSv/h e acima de 100 µSv/h) e trazem os contatos de
  emergência radiológica da CNEN.
- As faixas de aviso agora ficam no fluxo da página (fixas no topo ao rolar),
  em vez de sobrepor o conteúdo.
- Layout mais denso: em telas largas (1500 px ou mais) três colunas — leitura,
  cartões e alertas; gráficos; contador e fundo diário. Em 1920×1080 cabe tudo
  sem rolar. Margens e alturas menores; a explicação do teto da dose virou um
  "Como o teto é calculado" que abre ao clicar.
- O card Alertas foi para perto da leitura.

## 0.8.0
- Aviso de atenção a partir de 50 CPM (faixa do manual): faixa amarela no topo
  com botão OK, leitura em amarelo, 3 bipes uma vez por episódio e um alerta
  anotado. Sai quando o valor volta abaixo de 45 CPM. Pode ser desligado nas
  configurações. Se o valor já pular direto para o alarme, só o alarme toca.
- Alertas (atenção e alarme) separados dos eventos: os alertas ficam no card
  "Alertas"; os eventos (conexão, histórico, relógio, ajustes) ficam no botão
  "Eventos", que abre uma janela com a lista completa.
- `eventos.csv` ganhou a coluna do tipo (evento/alerta); arquivos antigos
  continuam sendo lidos.

## 0.7.0
- Alarme bem mais agressivo:
  - som em loop até o alarme acabar ou ser silenciado: sirene contínua
    (padrão) ou bipes rápidos; acelera quando passa do dobro do limite;
  - faixa vermelha fixa no topo com o valor, o limite e o botão Silenciar
    (Esc também silencia) e moldura pulsando na tela inteira (sem animação
    se o Windows pedir menos movimento);
  - título da janela muda para "ALARME" e a janela vem para a frente, mesmo
    minimizada (opção nas configurações);
  - botão "Testar alarme (5 s)" nas configurações.
- No modo demo o alarme simulado fica âmbar e o real continua vermelho, com
  prioridade.
- Gráfico de CPM com opção de mostrar em µSv/h (usa a calibração do contador).

## 0.6.0
- Gráfico de CPM com períodos de 15 e 30 min, usando as leituras por segundo
  (atualiza sozinho a cada 5 s).
- Dose acumulada com períodos de 3 h, 6 h e 12 h.
- Modo demo: um valor de CPM simulado (digitado, no controle deslizante ou nos
  atalhos) alimenta a tela, o alarme e os gráficos. O contador continua sendo
  lido e as leituras reais continuam indo para os arquivos; nada simulado é
  gravado, e ao sair do demo tudo some. O alarme real continua valendo durante
  o demo. Opção de variar como contagem de verdade (Poisson, janela de 60 s).

## 0.5.0
- Gráfico de dose acumulada (dosímetro) para 24 h, 7 dias, 30 dias e 1 ano,
  com três linhas: dose medida, só o fundo natural e o teto.
- Teto = fundo medido (mediana dos últimos 30 dias) + limite anual
  proporcional ao tempo medido: público 1 mSv/ano ou trabalhador exposto
  20 mSv/ano (CNEN NN 3.01 / ICRP, que não contam o fundo natural).
- Mostra quanto da margem acima do fundo já foi usada no período.

## 0.4.0
- Recomendação de intervalo de gravação na memória (a cada minuto), com prós e
  contras de cada modo e botão "Usar o recomendado".
- Mostra quanto da memória do contador está ocupado (desde o último histórico
  baixado) e quanto tempo o espaço livre dura no modo escolhido.
- Versão do app no rodapé e nas propriedades do .exe.

## 0.3.0
- Calibração editável (6 pontos, ou uma sensibilidade única para todos).
- Novos ajustes: estimativa rápida (FET), volume da voz, modo escuro,
  economia de energia e LED. Tensão do tubo só para leitura.
- Campos com valor não reconhecido ficam travados individualmente.
- Ferramenta "Descobrir onde fica um ajuste" (compara a configuração antes e
  depois de mudar algo no menu do contador).

## 0.2.0
- Janela "Ajustes do contador": gravação na memória, alarme (limite, volume,
  vibração), som de clique e brilho, com cópia de segurança, conferência e
  restauração.

## 0.1.0
- Primeira versão: leitura ao vivo, gráficos, histórico, relógio, alarme,
  GMCmap e exportação CSV.
