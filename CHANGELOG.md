# Versões

## 2.62.1
- Isótopos no "Acumulado do aparelho": além de passar do contínuo, o pico
  tem que subir acima das duas janelas vizinhas. Assim, o platô Compton de
  uma fonte (Cs-137, ou o K-40 do sal light) não vira rádio, tório ou urânio
  que não estão lá.

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

## 1.46.1
- Mapa de calor (Padrão por hora do dia): os alertas do RadiaCode não aparecem
  mais marcados no mapa do GMC.

## 1.46.0
- **Atividade (Bq) de verdade** na aba RadiaCode (cartão 6, no lugar do
  exemplo do KCl):
  - **calibrar com KCl** (KCl puro ou sal light, pela % de KCl): mede a
    eficiência do aparelho em 1.460 keV naquela posição;
  - **calibrar com o eletrodo de tório** (WT10 a WT40, com a idade se
    souber): as linhas de 238,6, 583,2 e 2.614,5 keV dão a forma da curva
    de eficiência (como ela muda com a energia);
  - **medir amostra**: K-40, Cs-137, Ra-226, Ra-228, série do Th-232 e
    U-238, em Bq e Bq/kg com a incerteza, ou "abaixo de" (atividade mínima
    detectável) quando não aparece; prontos para castanha-do-pará, areia
    monazítica, granito/porcelanato, vidro de urânio, alimento e adubo, com
    valores típicos para comparar;
  - fundo e amostra em fases, com tempo escolhido (ou até parar), resultado
    parcial e quanto tempo falta para ±10%; o fundo fica salvo para as
    próximas;
  - os picos são ajustados juntos (gaussianas de posição e largura conhecidas
    mais uma reta), para linhas vizinhas como 583/609 e 338/352 não se
    misturarem;
  - continua se o app fechar no meio; calibração, fundo e medidas entram no
    backup.

## 1.45.0
- **Modo busca de verdade** na aba RadiaCode (cartão 5), com o aparelho
  conectado:
  - usa as amostras de ~0,5 s que o RadiaCode manda e atualiza 2 vezes por
    segundo, com escala automática (×1 a ×1000);
  - média que se adapta: até 20 s enquanto a contagem fica parada, só o
    último segundo quando ela pula;
  - "× o normal" colorido (o normal é a mediana das últimas 24 h, ou o lugar
    onde você estiver, com "Usar o agora como normal"), ≈ µSv/h e o máximo
    com a hora (com "Zerar máximo");
  - som: cliques (um por contagem) ou tom que fica mais agudo quanto mais
    acima do normal, para procurar sem olhar a tela;
  - tendência dos últimos 60 s com a linha do normal.
  Sem o RadiaCode conectado, continua o exemplo simulado.

## 1.44.0
- **Log do RadiaCode** na aba RadiaCode (cartão 4), no estilo da aba Log do
  app do celular:
  - a taxa de dose que o aparelho manda a cada ~1,5 min (µSv/h e cps, com o
    ±), a temperatura, a bateria e a dose acumulada (Σ), e os eventos
    (ligou, carregando, alarmes…), do mais novo para o mais velho;
  - filtros por tipo (os três ícones), contador "mostrando/total" e período
    (hoje, 7 dias, 30 dias, tudo);
  - **importar do celular**: o .csv que o app do RadiaCode exporta entra no
    mesmo log, marcado como "celular", sem repetir o que o PC já tinha;
  - na primeira vez, o log é montado com os dados brutos já gravados;
  - fica em `radiacode/log-AAAA-MM.csv` e entra no backup (e na junção de
    backups, linha a linha).
- O cartão "Importar o log" simulado saiu: o RadiaCode não guarda histórico
  para baixar (cada app só tem o que recebeu conectado).

## 1.43.0
- **Dureza de verdade** na aba RadiaCode (cartão 3), com os espectros que o
  app grava de minuto em minuto:
  - contagem (cps) e dureza (fótons acima de 1.500 keV sobre os de 150 a
    1.500 keV) no mesmo período do espectrograma (3 h a 7 dias, os dois
    cartões andam juntos);
  - 1,00 é o fundo da casa: a mediana das horas dos últimos 7 dias; cada ponto
    usa uns 20 minutos em volta, com a faixa de incerteza;
  - chuva marcada em azul e os intervalos sem leitura em cinza;
  - passe o mouse: cps (× o fundo), dureza ± incerteza, chuva e, quando a
    subida é grande o bastante, **com o que parece** (o app soma os espectros
    daquele trecho e procura as linhas gama acima do fundo, como na medição
    de objeto);
  - passar o mouse num cartão marca o mesmo momento no outro.

## 1.42.0
- **Espectrograma de verdade** na aba RadiaCode (o cartão 2 deixou de ser
  exemplo quando há dados do aparelho):
  - com o RadiaCode conectado, o app lê o espectro uma vez por minuto e guarda
    o que chegou naquele minuto (`radiacode/espectros`, ~300 KB por dia);
  - períodos de 3 h, 12 h, 24 h e 7 dias (linhas de 1, 2, 5 e 30 min) e
    energia até 1.500 ou 3.000 keV; o mais recente fica embaixo;
  - se o app fechar ou o Bluetooth cair, o aparelho segue contando: a linha
    seguinte cobre o intervalo todo e aparece mais apagada (é a média dele);
  - passe o mouse: horário, cps, energia, contagens por minuto e o isótopo que
    costuma aparecer ali;
  - entra no backup e na junção de backups.

## 1.41.0
- **Espectro ao vivo** na aba RadiaCode (o primeiro dos exemplos simulados
  agora é de verdade, com o RadiaCode conectado):
  - mostra o que o aparelho juntou desde o **Recomeçar** (encoste a amostra e
    clique), ou o **acumulado do aparelho**;
  - a linha tracejada é o fundo da casa no mesmo tempo (o acumulado do
    aparelho, horas em casa), e os riscos vermelhos marcam as linhas gama que
    subiram acima dele, com o palpite do que é ("parece: ~Th-232");
  - números ao vivo: tempo, contagens, cps com a incerteza, o µSv/h do
    aparelho e a dureza (acima de 1.500 keV sobre 150–1.500 keV, relativa ao
    fundo); passe o mouse para ver os isótopos de cada energia;
  - lê o espectro a cada 2 s pelo USB e a cada 6 s pelo Bluetooth, só com a
    aba aberta. Sem o RadiaCode, continua o exemplo simulado.
  - O cartão antigo "Espectro do aparelho" saiu (virou o modo "Acumulado do
    aparelho").
- Os botões no rodapé das janelas (ex.: a da atualização) não ficam mais
  colados.

## 1.40.1
- Configurações → RadiaCode: uma linha mostra o estado da conexão o tempo
  todo (conectado pelo USB/Bluetooth, conectando, procurando ou o erro). Ao
  escolher um RadiaCode na busca do Bluetooth, aparece "Conectando a…", o app
  tenta na hora (antes esperava a vez, até 30 s) e avisa quando conectou; se
  ele estiver no cabo USB ou o Bluetooth falhar, a tela explica.
- As mensagens da busca do Bluetooth não somem mais sozinhas a cada leitura
  do RadiaCode.

## 1.40.0
- **Gráfico da dose do RadiaCode com o número do aparelho**: o app agora
  guarda cada total que o RadiaCode manda (`radiacode/dose-AAAA-MM.csv`) e o
  gráfico "Sua dose (RadiaCode)" usa esses totais, não mais a soma dos
  minutos gravados pelo PC. Como o aparelho continua contando desconectado,
  o que ele somou longe do PC entra quando ele volta (no gráfico, espalhado
  pelo tempo em que esteve fora). O fundo e o teto crescem com o tempo em que
  o aparelho contou. Se ele for zerado no meio, a conta continua do zero dele.
- Na primeira vez, o app recupera esses totais dos dados brutos já gravados
  (desde a chegada do RadiaCode), então o histórico não começa vazio.
- A linha "Calculado pelo app" continua embaixo, com a soma dos minutos do
  PC, para comparar.

## 1.39.2
- RadiaCode, dados brutos: agora é um arquivo por conexão
  (`bruto/AAAA-MM-DD-HHMMSS.jsonl.gz`). Antes cada conexão era emendada no
  arquivo do dia, e quando o app era fechado sem terminar o arquivo (ex.: na
  atualização), o resto do dia ficava ilegível para os leitores comuns.
- O backup leva a dose acumulada do aparelho (a última recebida); ao juntar
  backups, fica a mais recente.
- O nome do modelo aparece no log de conexão ("RadiaCode-102"), tirado do
  número de série quando o aparelho não manda.
- "RadiaCode: configuração alterada" entra no log no máximo uma vez por
  minuto (cada toque no menu do aparelho gerava uma linha).
- Espectro da medição: os picos ficaram mais rigorosos. O contínuo embaixo do
  pico agora é estimado em escala log (ele cai quase exponencial), o pico tem
  que se destacar pelo menos 5% do contínuo (50% abaixo de 100 keV) e o Co-60
  só conta com as duas linhas (as do Bi-214 em volta de 1173 keV não viram
  mais Co-60).

## 1.39.1
- Medir objeto com os dois aparelhos: o gráfico agora mostra o segundo
  aparelho também, em µSv/h (rosa para o RadiaCode, verde para o GMC;
  tracejado quando ele está com você), ao vivo e nas medições salvas. A
  legenda diz de qual aparelho é cada linha.

## 1.39.0
- **Medir objeto em µSv/h**: tudo na tela de medir passa a ser em µSv/h
  (fundo, objeto, líquido, incerteza, limite de detecção, gráfico, medições
  salvas e o relatório em PDF). As contas continuam nas contagens (σ, Poisson);
  só a unidade mostrada mudou, e as contagens de cada fase continuam
  aparecendo. No RadiaCode vale o µSv/h que ele calcula; no GMC, a
  calibração dele.
- **Com os dois aparelhos, os dois aparecem ao vivo**: em cima das fases, um
  quadro para cada aparelho, desde o fundo: leitura de agora, contagens da
  fase e a média; com o objeto, quanto subiu (ou, no que está com você, a sua
  dose até agora).

## 1.38.0
- **Medir objeto com os dois aparelhos**: o campo "Aparelho" ganhou duas
  opções.
  - **Os dois perto do objeto**: o RadiaCode e o GMC medem ao mesmo tempo,
    cada um contra o seu fundo. No automático, só para quando os dois têm
    resposta. O resultado mostra os dois e o que a comparação sugere: o GMC
    vê beta e gama, o RadiaCode quase só gama, então se o GMC sobe bem mais,
    o objeto deve emitir beta.
  - **Um no objeto, outro com você**: você escolhe qual fica no objeto; o
    outro fica com você (bolso ou mão) e, no fim, o app mostra quanto você
    recebeu enquanto o objeto era medido (dose, média, máximo e quanto ficou
    acima do seu fundo).
  - Os dois jeitos ficam nas medições salvas e no relatório em PDF.

## 1.37.0
- **Cartão "Sua dose (RadiaCode)"**: o número grande agora é a dose do
  próprio aparelho (desde que foi zerado, a mesma da tela dele), andando ao
  vivo. A dose que o app calcula com os minutos gravados no período escolhido
  fica embaixo, menor ("Calculado pelo app"), com quanto do período foi
  gravado. O teto e o gráfico continuam sendo do período. No zoom (um trecho
  escolhido no gráfico) e enquanto o aparelho ainda não mandou o total, o
  número grande continua sendo o do app.

## 1.36.1
- Dose do aparelho no cartão de dose: logo depois de conectar, enquanto o
  RadiaCode ainda não mandou o total (ele manda a cada 3 a 4 min), a linha
  "No aparelho" já aparece avisando que está esperando, em vez de ficar
  escondida.
- Quando o Bluetooth cai e volta, a bateria e a temperatura do RadiaCode
  continuam aparecendo (as últimas recebidas) até ele mandar de novo.

## 1.36.0
- **Dose acumulada do RadiaCode sempre atualizada**: o cartão "Sua dose
  (RadiaCode)" mostra o total do próprio aparelho (desde que foi zerado, o
  mesmo número da tela dele). O aparelho manda esse total a cada ~4 min; entre
  um envio e outro o app soma a leitura ao vivo, então o número anda a cada
  segundo e se acerta no envio seguinte. Desconectado, aparece o último total
  recebido, com a hora. A aba RadiaCode usa o mesmo número.

## 1.35.1
- RadiaCode: quando a conexão caía e voltava no meio de um minuto, o minuto
  era gravado duas vezes e contava duas vezes na dose. Agora as duas partes
  viram um minuto só (com a média das duas), inclusive nos arquivos já
  gravados.

## 1.35.0
- **Chuva com o RadiaCode**: a aba Chuva ganhou a escolha do aparelho
  (RadiaCode ou GMC-800). Da primeira vez, vem o RadiaCode se ele já tiver
  dado gravado: ele conta umas 30 vezes mais que o GMC, então a subida depois
  da chuva aparece com bem menos ruído.
  - A conta usa os minutos do RadiaCode em casa (fora de alertas e dos
    períodos fora de casa), no CPM dele (cps × 60).
  - Os resultados aparecem em % da base e em µSv/h: efeito da chuva, curva
    média, cada chuva (antes → pico) e a chuva de agora.

## 1.34.0
- **Fora de casa com o RadiaCode**: cada período mostra o que o RadiaCode
  mediu nele (dose, média comparada com o seu normal em casa, máximo, quanto
  foi medido e alertas) e o GMC-800 no mesmo tempo. Em cima da lista, a dose
  do RadiaCode somando todos os períodos. Quando o RadiaCode estava no app do
  celular, o período avisa que os dados ficaram lá.
- Os campos de início e fim do período não passam mais da borda em janelas
  estreitas.

## 1.33.0
- **Alertas do RadiaCode**: o RadiaCode agora gera alertas como o GMC, em
  µSv/h, com os limites do próprio aparelho:
  - **atenção** = alarme 1 do RadiaCode, **alarme** = alarme 2, **perigo** =
    o perigo do app convertido. Se você mudar os alarmes no aparelho, o app
    relê sozinho;
  - o PC avisa igual ao GMC: atenção bipa e mostra a faixa; alarme e perigo
    tocam o som e trazem a janela para a frente (seguem as mesmas opções de
    ligar/desligar);
  - na aba **Alertas**, um filtro novo (Os dois · RadiaCode · GMC-800), a
    etiqueta "RadiaCode" e o gráfico do alerta em µSv/h com a leitura ao vivo
    dele;
  - o **Resumo** e o **relatório em PDF** contam os alertas do RadiaCode no
    bloco dele. Os números do GMC continuam só com os alertas do GMC.

## 1.32.0
- **Medir objeto com o RadiaCode**: novo campo "Aparelho" (RadiaCode ou
  GMC-800). Da primeira vez, vem o RadiaCode se ele estiver conectado.
  - As contagens vêm das amostras cruas do aparelho (a cada 0,5 s) e o
    µSv/h é a média do que o próprio RadiaCode calcula em cada fase.
  - **Fundo da casa** pelo RadiaCode: os minutos dele em casa da última
    hora. Só vale se ele ficou parado no lugar da medição (no bolso o fundo
    é outro), e a tela avisa.
  - **Espectro**: o app guarda o espectro de cada fase (com o fundo da casa,
    usa o espectro acumulado do aparelho na escala do tempo do objeto) e
    mostra o gráfico no resultado e nas medições salvas, com as linhas gama
    que subiram acima do fundo (Am-241, U-235, séries do tório e do rádio,
    Cs-137, Co-60, K-40). É uma pista do radionuclídeo, não uma
    identificação garantida.
  - O **relatório em PDF** da medição ganhou o espectro e os textos do
    RadiaCode (detector, calibração do aparelho, limitações).

## 1.31.0
- **Resumo com o RadiaCode**: quando ele tem dado no período, o resumo
  começa por ele:
  - dose, média (comparada com o seu fundo em casa), máximo e mínimo em
    µSv/h;
  - **em casa × fora de casa**: dose e tempo de cada um, com a média e o
    máximo fora;
  - quanto do período foi medido.
  Depois vem o bloco do **GMC-800 (casa)** como antes. O gráfico do resumo
  fica em µSv/h com os dois aparelhos (RadiaCode rosa, GMC verde). No modo
  Comparar entram as linhas de dose, média e fora de casa do RadiaCode.
- O **relatório em PDF** ganhou a seção "RadiaCode (dosímetro pessoal)",
  com os mesmos números.

## 1.30.0
- **RadiaCode como aparelho principal** (seu dosímetro), com o GMC como
  contador da casa:
  - o quadro e a pílula do RadiaCode vêm primeiro, e o GMC fica à direita;
  - **gráfico**: em µSv/h, o RadiaCode é a curva cheia (rosa) e o GMC
    (verde) vira uma linha de referência. Na primeira vez com o RadiaCode,
    o gráfico passa para µSv/h;
  - **cartão de dose**: mostra a **sua dose pelo RadiaCode**, com o fundo
    em casa dele, e o GMC-800 (casa) no mesmo período logo abaixo. Um seletor
    troca para a dose da casa (GMC-800). Os minutos sem leitura continuam
    fora da conta;
  - a régua de dose mostra os dois no mesmo período.

## 1.29.0
- **Régua de radiação com os dois aparelhos**:
  - **Taxa agora:** duas linhas, "Agora no GMC-800" (âmbar) e "Agora no
    RadiaCode" (rosa). O fundo da casa também aparece pelos dois.
  - **Dose recebida:** a dose do GMC no período escolhido e a dose de hoje
    no RadiaCode.
  - As comparações ("3× mais") continuam em relação ao GMC.
- Textos da régua que faltavam em inglês ("≈ igual", "mil×", "1 ano",
  Chernobyl).

## 1.28.5
- A pílula do RadiaCode no topo agora mostra só o nome e a bateria
  ("RadiaCode · 100%"). O µSv/h já está no quadro.

## 1.28.4
- A régua do RadiaCode agora usa os **alarmes do próprio aparelho** (no seu,
  0,40 µSv/h para o alarme 1 e 1,20 µSv/h para o alarme 2), como a do GMC
  usa as faixas do manual dele. O nível aparece como Seguro, Alarme 1 ou
  Alarme 2. Se o app não conseguir ler os alarmes, volta às faixas do app.

## 1.28.3
- Os quadros **GMC-800 | RadiaCode** agora têm a mesma estrutura, linha por
  linha:
  - número, linha de baixo, "normal", σ, unidades, régua e medidas ficam na
    mesma altura nos dois, mesmo quando um texto quebra em duas linhas ou
    some (por exemplo, com o RadiaCode desconectado);
  - as unidades vêm na mesma ordem (CPM, µSv/h, mrem/h, CPS), e cada uma
    mostra as mesmas informações embaixo nos dois aparelhos;
  - o mesmo tipo de número;
  - o "o que é σ?" também aparece no RadiaCode;
  - a porta do GMC aparece no canto, como a conexão do RadiaCode;
  - as medidas do GMC mostram só a unidade, como as do RadiaCode.

## 1.28.2
- Os quadros **GMC-800 | RadiaCode** agora têm a mesma largura e o mesmo
  formato.
  - A média de 1 h e o mínimo e o máximo de 24 h do GMC entraram no quadro
    dele, como no do RadiaCode. Os cartões separados embaixo somem enquanto
    os dois aparecem lado a lado.
  - O quadro do RadiaCode ganhou a mesma cor de fundo do GMC, que muda com o
    nível dele (verde, âmbar em atenção, laranja/vermelho em alerta). O
    número também muda de cor.
  - Fica sem cor quando está desconectado.

## 1.28.1
- **RadiaCode com firmware 4.15**: esse firmware manda um byte a mais
  depois de cada leitura ao vivo, coisa que a biblioteca aberta não
  conhece. Com isso o app descartava todo o resto da resposta, inclusive o
  pacote com a **bateria**, a temperatura e a dose acumulada. Agora ele pula
  esse byte e lê tudo. A bateria aparece em até 4 min depois de conectar.
- As **mensagens internas do aparelho** ("BLE: client connected", erros de
  quadro…) vão para o log técnico (log.txt), e não mais para os Eventos.
- "Configuração alterada" logo ao conectar não entra mais em Eventos: era o
  próprio app acertando o relógio do RadiaCode.
- O retrato da conexão guarda a resposta crua quando o formato de uma área
  de dados não é entendido (ex.: a cópia da memória).

## 1.28.0
- **Dados brutos do RadiaCode**: para descobrir o que mais dá para
  aproveitar do aparelho, o app guarda tudo o que ele manda, na pasta
  `radiacode/bruto`.
  - **Todo registro como veio** (hex), com os campos que já se sabe ler.
    Isso inclui os que o app ainda não usa: histórico interno (DoseRateDB),
    contagens cruas, acelerômetro e blocos de amostras.
  - O **espectro** atual e o acumulado, **a cada hora**.
  - **A cada conexão**: todos os registradores conhecidos e todas as
    "strings virtuais", inclusive as de formato ainda desconhecido, como a
    **cópia da memória** (MEM_SNAPSHOT) e o arquivo de registradores.
  - Fica em arquivos .jsonl.gz por dia, de alguns MB. Dá para desligar em
    Configurações → RadiaCode.
- O **backup** agora leva os minutos do RadiaCode e os dados brutos. Ao
  restaurar, os minutos se juntam pela hora, sem repetir.

## 1.27.2
- O quadro do RadiaCode agora mostra as mesmas informações do GMC:
  - dose de hoje com os minutos medidos;
  - CPS, CPM e incerteza;
  - **normal/σ** pelo fundo em casa dos últimos 7 dias (a faixa típica);
  - seletor de unidade (µSv/h, mrem/h, CPS, CPM);
  - **régua** com as faixas do app em µSv/h;
  - **medidas**: média de 1 h e mínimo e máximo de 24 h.
- Lado a lado, a dose de hoje vai para uma linha embaixo do número nos dois
  quadros, e a coluna da esquerda fica mais larga.
- As abas da direita não quebram mais em duas linhas.

## 1.27.1
- Quadros lado a lado na tela principal: **GMC-800 | RadiaCode**, com a
  mesma altura. A coluna da esquerda fica um pouco mais larga quando o
  RadiaCode aparece.
- O quadro do RadiaCode mostra o µSv/h grande, a incerteza, os cps e a dose
  de hoje; a conexão e a bateria ficam no canto.
- Desconectado: mostra o último valor apagado, a hora da leitura e a dose de
  hoje.

## 1.27.0
- **Tela principal com os dois aparelhos**: logo abaixo do quadro do GMC
  (agora chamado "GMC-800") aparece o quadro do **RadiaCode**.
  - Conectado: µSv/h grande, incerteza, cps, dose de hoje e bateria.
  - Desconectado: uma linha com a última leitura e a dose de hoje.
  - Quem nunca conectou um RadiaCode não vê nada diferente.
- **Gráfico**: em µSv/h, a curva do RadiaCode (rosa) aparece junto com a do
  GMC, com a média nas estatísticas e o valor na dica ao passar o mouse. Em
  CPM, a legenda avisa que ela só aparece em µSv/h; clicando, o gráfico
  troca de unidade.
- Na aba RadiaCode, a comparação usa as mesmas cores (GMC verde, RadiaCode
  rosa).

## 1.26.0
- **RadiaCode pelo Bluetooth no Windows** (Windows 10 ou mais novo), pelas
  APIs do próprio Windows, sem instalar nada. Em Configurações → RadiaCode,
  "Procurar no Bluetooth" acha o aparelho e "Usar este" grava o endereço.
  Daí em diante o app conecta sozinho. Se o celular estiver conectado no
  RadiaCode, o PC tenta de novo a cada 30 s e assume quando o celular solta.

## 1.25.0
- **RadiaCode (primeira versão)**: o app procura o RadiaCode sozinho e
  conecta pelo **USB** (Windows e Linux) ou pelo **Bluetooth** (por enquanto
  só no Linux). O protocolo segue a biblioteca aberta cdump/radiacode.
  - Na aba RadiaCode: taxa de dose, contagem, dose acumulada, bateria,
    temperatura, modelo, número de série e firmware ao vivo.
  - A pílula no topo mostra o µSv/h e a bateria.
  - Os eventos do aparelho (alarmes, carga) entram nos Eventos.
  - O app grava um ponto por minuto, separado das leituras do GMC
    (pasta `radiacode`).
  - **Lado a lado com o GMC**: os dois aparelhos em µSv/h no mesmo gráfico,
    com a razão RadiaCode ÷ GMC e a correlação minuto a minuto. É o começo da
    intercalibração.
  - **Espectro do aparelho**: o atual ou o acumulado, em escala log ou linear.
  - Em Configurações → RadiaCode: ligar e desligar a procura, além do
    endereço Bluetooth e da busca pelo aparelho.
- No Windows, o USB precisa do driver WinUSB. Se o aparelho estiver com
  outro driver, o erro mostra qual é.

## 1.24.0
- **Medir objeto no automático**: a fase com o objeto para sozinha quando já
  dá para responder. Para quando o objeto está claramente acima do fundo
  (4σ) ou quando dá para garantir que ele sobe menos que 50%, 20% ou 10% do
  fundo, à escolha. Vai até 10 min, 30 min ou 1 h. Durante a medição aparece
  o resultado parcial e quanto falta para decidir. Os tempos fixos continuam
  disponíveis.
- **Fundo da casa**: dá para pular a fase do fundo e usar a média da última
  hora gravada (em casa, fora de alertas e medições). Vale quando o objeto é
  medido no lugar de sempre do contador. É bem mais preciso que 1–5 min
  medidos na hora e corta o tempo pela metade.
- Campo **distância** (encostado, 5 cm…), que aparece no histórico e no
  relatório.
- O relatório da medição mostra como ela parou, o limite superior ("se tiver
  algo, é menos que +X CPM") e o limite de detecção com tempos diferentes
  para fundo e objeto.

## 1.23.2
- Em inglês, o erro ao abrir a porta do contador ("não consegui abrir …") e
  outras mensagens de erro ainda saíam em português. Também: a chuva na dica
  do gráfico, "Chovendo desde…", a validade do código de pareamento e
  alguns eventos (modo demo, tempo morto, limpeza de versões antigas).
- Resumo da sincronização e do backup com singular certo: "1 anotação",
  "1 período fora de casa" (antes saía "1 anotações").

## 1.23.1
- Em inglês, o aviso de chuva do gráfico ("buscando chuva…", "chuva em …",
  "radar às …") e mais alguns textos da chuva e do pareamento ainda saíam em
  português.

## 1.23.0
- **Sincronização entre PCs** (Configurações → Sincronização): dois apps
  trocam o que falta um no outro (leituras por minuto, alertas encerrados,
  períodos fora de casa, medições, anotações e os segundos dos alertas). É a
  mesma mescla do "Importar backup": nada é apagado nem substituído.
- **Pareamento com código de 6 dígitos**: um PC "aceita conexões" e mostra o
  código (vale 5 min, um pareamento, até 5 tentativas erradas); o outro acha
  ele na rede (ou pelo endereço) e digita. O código não vira a chave: o
  pareamento usa CPace (troca de chaves autenticada por senha), então quem
  escuta a rede não consegue descobrir nada, e quem tentar se meter no meio só
  testa um código por vez.
- **Troca só o que falta**: cada lado resume os minutos de cada dia; só vão
  os dias diferentes e os segundos de alerta que o outro não tem. Tudo
  cifrado (AES-256-GCM) com a chave do par.
- **Sozinho a cada 10 min** (dá para desligar) e "Sincronizar agora".
- **Fora de casa pelo Tailscale**: o PC que aceita conexões informa todos os
  endereços dele (rede de casa e 100.x.y.z do Tailscale) e o outro tenta um
  por um. Dá para acrescentar endereços à mão em "Endereços".
- Só quem "aceita conexões" escuta na rede (porta 47862). No Windows isso
  pede liberação no firewall; por isso a sugestão é deixar ligado no
  notebook (Linux) e o PC só conectar.

## 1.22.1
- **As versões baixadas pelo app ficam numa pasta só dele**:
  Documentos\GMC Monitor\Versões, ao lado de Relatórios, em vez da pasta do
  .exe aberto (que costumava ser Downloads).
- **As versões antigas são apagadas sozinhas**: 2 minutos depois que a versão
  nova abre e fica rodando, o app apaga os GMC-Monitor-vX.Y.Z.exe (ou
  AppImage) de versões mais antigas, na pasta das versões e na pasta de onde
  veio a versão anterior. Nunca apaga a que está aberta, uma mais nova nem
  arquivos com outro nome. Dá para desligar em Configurações → Atualizações,
  onde também tem "Abrir pasta".

## 1.22.0
Segunda parte do inglês:
- **Relatórios em PDF em inglês**: o relatório do período e o da medição de
  objeto saem no idioma do app, com números com ponto decimal. O texto é
  traduzido antes de medir, então quebra de linha e cortes continuam certos.
- **Menu da bandeja e título da janela** (ALARM: 150 CPM) no idioma do app;
  o menu troca na hora quando você muda o idioma.
- **Registro técnico** (Eventos e log) com as mensagens em inglês. O arquivo
  log.txt continua em português; os nomes dos componentes também (são os
  filtros), mas aparecem traduzidos na tela.
- A tradução fica num lugar só, internal/i18n/en.tsv, usada pela tela e pelo
  Go.

## 1.21.0
- **Interface em inglês**: Configurações → Idioma (Automático, Português ou
  English). No automático, segue o idioma do sistema. A troca é na hora (a
  janela recarrega).
- Traduz a tela inteira, inclusive os textos que vêm do contador e do app
  (alertas, eventos, conclusões das medições). Os eventos antigos, gravados em
  português, também aparecem em inglês.
- Números e datas seguem o idioma (0.27 em inglês, 0,27 em português).
- Ainda em português nesta versão: relatórios em PDF, menu da bandeja e o
  texto das linhas do registro técnico.
- Para quem for traduzir: as frases ficam em internal/i18n/en.tsv;
  scripts/i18n_extract.py lista os textos do app e scripts/i18n_build.py gera
  a tradução usada pela tela.

## 1.20.0
- **Medir objeto em duas partes**: "Nova medição" e "Medições salvas".
- **Medições salvas** com a lista à esquerda (nome, data, duração e o líquido
  em CPM, com busca pelo nome ou pela anotação) e a medição escolhida à
  direita:
  - conclusão em destaque;
  - fundo, com o objeto, líquido e objeto ÷ fundo;
  - o gráfico segundo a segundo;
  - a tabela das duas fases;
  - a geometria/anotação;
  - o limite de detecção e o tempo para ±10% e ±5%;
  - os botões Relatório (PDF), Ver no gráfico, Editar e Apagar.
- No fim de uma medição, "Ver em Medições salvas" abre ela direto.

## 1.19.1
- **Nome comprido não é mais cortado no relatório**: o título da medição
  diminui um pouco e, se ainda não couber, quebra em até 3 linhas. O mesmo
  vale para o período dos outros relatórios (ex.: "Fora de casa: …").

## 1.19.0
- **Relatório de uma medição de objeto (PDF)**: botão "Relatório (PDF)" em
  cada medição salva. Traz:
  - a conclusão e os números das duas fases (contagens, tempo, CPM ± σ);
  - o líquido em CPM e µSv/h no tubo, objeto ÷ fundo e a significância (σ);
  - o gráfico segundo a segundo, com a média e a margem de erro de cada fase;
  - a distribuição das contagens por segundo comparada com o acaso (Poisson),
    para ver se algo mudou durante a medição;
  - o limite de detecção (Currie) para o tempo usado e quanto tempo daria
    ±10% e ±5% de precisão;
  - geometria (a anotação da medição), contador, calibração, método e
    limitações.
- **O relatório sai sozinho no fim da medição**: salvo na pasta de relatórios,
  com "Abrir relatório" no resultado. Dá para desligar em Configurações →
  Relatórios.
- **Relatório de um período fora de casa**: botão "Relatório (PDF)" em cada
  período da lista Fora de casa.

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
