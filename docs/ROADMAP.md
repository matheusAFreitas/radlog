# Roadmap

O que está planejado para o app. O que já foi feito está no [CHANGELOG](../CHANGELOG.md).

## 2.0 — RadLog + RadiaCode

O app passa a se chamar **RadLog** e deixa de ser só do GQ GMC-800.

- [ ] Novo nome: a pasta de dados migra sozinha na primeira abertura; a troca
      automática de versão continua funcionando com as versões 1.x abertas;
      o início com o Windows é refeito com o nome novo.
- [ ] Papéis: o **GMC-800 é a base da casa** (fica parado, monitora o
      ambiente) e o **RadiaCode 102 é o dosímetro pessoal** (anda pela casa e
      fora dela).
- [ ] Conexão com o RadiaCode por **USB e Bluetooth**. O app do PC tenta
      conectar sozinho de tempos em tempos: quando o celular solta o aparelho
      (app oficial fechado ou fora do alcance), o PC assume e baixa o que
      ficou na memória do RadiaCode.
- [ ] **Alarmes em dois blocos**: "Base (GMC-800)" continua como hoje;
      "Dosímetro (RadiaCode)" em µSv/h e dose acumulada, configurando os
      alarmes do próprio aparelho (na rua quem apita é ele) e registrando os
      que tocaram. Fundo de referência e saturação por aparelho.
- [ ] **Tela principal com os dois**: pílula de conexão de cada aparelho no
      topo; quadros "GMC-800" (CPM, µSv/h, estado × fundo de 30 d) e
      "RadiaCode 102" (µSv/h, cps, dose de hoje, bateria, lugar) lado a lado;
      um gráfico com as duas curvas, marcando quando estavam lado a lado; dose
      do dia e do mês e alertas por aparelho embaixo. A decidir com o
      aparelho em mãos: o quadro do RadiaCode desconectado (última leitura ou
      some) e o gráfico todo em µSv/h ou o GMC em CPM com outro eixo.
- [ ] **Minha dose**: dose pessoal do dia, semana, mês e ano, comparada com o
      esperado só pelo fundo natural e com o limite de 1 mSv/ano acima do
      natural.
- [ ] Importar os rastros exportados pelo app oficial do RadiaCode (uso na
      rua), para a dose pessoal juntar casa e rua.
- [ ] Confirmação cruzada só quando o RadiaCode estiver na mesa, ao lado do
      GMC (ex.: no USB): os dois subindo juntos = radiação; só um = suspeito.
- [ ] Recursos da aba RadiaCode (hoje com dados simulados), nesta ordem:
  1. Espectro
  2. Espectrograma
  3. Dureza do espectro nos gráficos
  4. Importar o log do aparelho (curva separada, "pessoal")
  5. Modo busca
  6. Atividade com KCl
- [ ] **Casa × pessoal** no Resumo: dose do monitor de área (GMC) e do
      dosímetro pessoal (RadiaCode), e a razão entre os dois.
- [ ] **Intercalibração** (primeiro teste com o RadiaCode): os dois lado a
      lado por pelo menos 24 h, na mesma altura, sem um blindar o outro. Os
      dois em µSv/h no mesmo gráfico, razão e correlação minuto a minuto, e o
      fator entre eles. O fator vale para o fundo natural; com fontes de outra
      energia ele muda (o tubo Geiger exagera em energias baixas). Serve
      também para testar a confirmação cruzada.
- [ ] **ALARA** (segundo teste, com o RadiaCode no bolso): tempo, distância e
      blindagem medidos.
  - De onde vem a dose: marcar trechos com lugar ou atividade (quarto,
    trabalho, metrô, rua) e ver quanto da dose do dia veio de cada um.
  - Teste de distância guiado: o mesmo objeto a 5, 10, 20 e 40 cm; o app
    mostra se segue o inverso do quadrado e a partir de onde vira fundo.
  - Teste de blindagem: medição de objeto com a etapa a mais "com o material
    na frente", dando quanto o material cortou.
  - Antes e depois: quanto de dose por mês uma mudança economizou (ex.: a
    caixa do eletrodo longe da mesa).

## 3.0 — App Android (depois da 2.0)

- [ ] Dosímetro no celular: conexão Bluetooth com o RadiaCode o dia todo
      (serviço em primeiro plano, sem otimização de bateria), GPS, alarme e
      notificação; reaproveita o núcleo em Go (protocolo, estatística, dose).
- [ ] Sincroniza com o app do PC (mesmo pareamento da comunicação entre PCs).

## Comunicação entre PCs (1.x)

- [ ] Sincronização direta app ↔ app com pareamento (código de 6 dígitos):
      cada PC se anuncia na rede e troca só o que falta.
- [ ] Funcionar fora de casa por Tailscale, sem mudar nada no app.

## Ideias em aberto

- [ ] Tendência do fundo: média móvel de 24 h no gráfico, para ver o fundo
      subir ou descer devagar ao longo das semanas.
- [ ] Tempo morto: medir o τ real do tubo (precisa do tempo de cada pulso).
      Hoje usa 200 µs, típico do M4011.
- [ ] Rad Pro no GMC-800: se um dia, o app aprende o protocolo do Rad Pro.

## Decidido não fazer (por enquanto)

- Gravar por segundo (fica por minuto; o detalhe por segundo é só nos alertas).
- µGy/h e µR/h como unidades (no lugar, a régua de radiação).
- Tirar as medições de objeto da dose acumulada.
- Trilha no mapa e câmera do RadiaCode.
- Fundo de referência com menos de 30 dias.
