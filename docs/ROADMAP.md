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
- [ ] Intercalibração: os dois aparelhos lado a lado por 1–2 dias para achar o
      fator entre eles.

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
