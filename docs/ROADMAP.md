# Roadmap

O que está planejado para o app. O que já foi feito está no [CHANGELOG](../CHANGELOG.md).

## 2.0 — RadLog + RadiaCode

O app passa a se chamar **RadLog** e deixa de ser só do GQ GMC-800.

- [ ] Novo nome: a pasta de dados migra sozinha na primeira abertura; a troca
      automática de versão continua funcionando com as versões 1.x abertas;
      o início com o Windows é refeito com o nome novo.
- [ ] Conexão com o **RadiaCode 102** (USB primeiro, Bluetooth depois), na
      ordem da aba RadiaCode (hoje com dados simulados):
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

## Comunicação entre PCs (1.x)

- [ ] Sincronização direta app ↔ app com pareamento (código de 6 dígitos):
      cada PC se anuncia na rede e troca só o que falta.
- [ ] Funcionar fora de casa por Tailscale, sem mudar nada no app.

## Ideias em aberto

- [ ] Aviso de "pico suspeito de interferência" quando um segundo vier muito
      fora do padrão de Poisson (ruído elétrico, celular encostado).
- [ ] Tempo morto: medir o τ real do tubo (precisa do tempo de cada pulso).
      Hoje usa 200 µs, típico do M4011.
- [ ] Rad Pro no GMC-800: se um dia, o app aprende o protocolo do Rad Pro.

## Decidido não fazer (por enquanto)

- Gravar por segundo (fica por minuto; o detalhe por segundo é só nos alertas).
- µGy/h e µR/h como unidades (no lugar, a régua de radiação).
- Tirar as medições de objeto da dose acumulada.
- Trilha no mapa e câmera do RadiaCode.
- Fundo de referência com menos de 30 dias.
