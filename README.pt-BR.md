<p align="center">
  <img src="docs/logo-light.svg#gh-light-mode-only" alt="RadLog" width="300">
  <img src="docs/logo-dark.svg#gh-dark-mode-only" alt="RadLog" width="300">
</p>

<p align="center"><a href="README.md">English</a> · <b>Português (Brasil)</b></p>

O RadLog é um app de computador que registra radiação ionizante de dois
aparelhos: o contador Geiger **GQ GMC-800** e o dosímetro e espectrômetro
gama **Radiacode 102**. Roda no Windows e no Linux, funciona com um aparelho
ou com os dois e está em português e em inglês.

Ele lê os aparelhos pela USB (o Radiacode 102 também pelo Bluetooth), guarda
uma leitura por minuto e mostra tudo em µSv/h: leitura ao vivo, gráficos,
dose, alertas, espectro, clima × radiação, medição de objetos e relatórios em
PDF. Os dados ficam só no seu computador.

Versão atual: veja as [Releases](../../releases) e o [CHANGELOG](CHANGELOG.md).
Um app de Android (3.0) está a caminho.

> Até a 1.x se chamava **GMC Monitor**. A 2.0 leva os dados sozinha para a
> pasta nova na primeira vez que abre.

![Tela Agora, com o GQ GMC-800 e o Radiacode 102 lado a lado](docs/prints/pt-agora.png)

## Propósito do app

O RadLog é um projeto de estudo. Foi feito para aprender radioproteção e como
esses dois aparelhos funcionam, e está aberto para quem estiver aprendendo as
mesmas coisas.

Não é um app profissional. As leituras e as doses são estimativas de
aparelhos de uso pessoal, o app não foi certificado nem validado por ninguém
e não substitui um instrumento calibrado, um dosímetro oficial nem um
profissional de radioproteção. Numa emergência de verdade, siga as
autoridades locais.

## Baixar e atualizar

Os arquivos prontos ficam em **[Releases](../../releases)**:

- **Windows**: `RadLog-vX.Y.Z.exe`. Não precisa instalar; o WebView2 já vem
  no Windows 10/11.
- **Linux**: `RadLog-vX.Y.Z-x86_64.AppImage`, ou o
  `radlog-vX.Y.Z-linux-x86_64.tar.gz` com o binário e o `install.sh`. Precisa
  da WebKitGTK 4.1 (`sudo apt install libwebkit2gtk-4.1-0`).

O app procura versão nova ao abrir e uma vez por dia. Antes de instalar, ele
confere o SHA-256 do arquivo baixado e a **assinatura Ed25519** da lista de
somas (`SHA256SUMS.txt.sig`): sem assinatura válida, a versão é recusada.
Também dá para trocar à mão: abra a versão nova com a antiga aberta, e a
antiga fecha e passa o lugar.

## Como funciona

Cada aparelho tem um papel. No padrão, o GMC-800 é o **monitor da casa**:
fica parado num lugar e mede o fundo de onde você mora. O Radiacode 102 é o
seu **dosímetro pessoal**: vai com você, e as leituras dele formam a sua
dose. Qualquer trecho pode ser marcado de outro jeito: você levou o Geiger,
deixou o Radiacode em casa ou um aparelho ficou fora do cálculo. Esses
trechos continuam contando na dose, mas não entram no fundo da casa.

## As telas

- **Agora**: os dois aparelhos lado a lado, cada um com mostrador, nível
  (seguro, atenção, alarme, perigo, sempre com cor **e** forma), frase
  dizendo se está dentro do normal, a última hora e a fita do dia. O valor
  em µSv/h também aparece em mrem/h. O anel do mostrador pode piscar a cada
  contagem.
- **Gráficos**: ao longo do tempo (com zoom arrastando), por hora do dia,
  padrões, **tendência** (fundo por dia em µSv/h e subida lenta do fundo) e
  o **calendário do ano** (fundo da casa ou sua dose, dia a dia).
- **Resumo**: Dia, Semana ou Mês com setas para andar no tempo, uma frase
  com o que importa, a dose comparada com um raio-X de tórax, o tempo em
  casa e fora, o gráfico com a fita dos dias e as saídas do período. Também
  compara dois dias e gera o **relatório em PDF**.
- **ALARA**: de onde vem a sua dose e testes de tempo, distância e
  blindagem.
- **Alertas**: uma lista só, dos dois aparelhos, com o detalhe ao lado
  (gráfico, duração, pico). Cada alerta diz se o outro aparelho
  **confirmou** a subida (confirmação cruzada), e o painel **"Por que
  subiu?"** confere as causas comuns (acaso, o outro aparelho, chuva,
  radônio, pressão, objeto, saída de casa) com os seus dados.
- **Medir objeto**: fundo e objeto pelo mesmo tempo, a diferença com margem
  de erro, um veredito e a dose comparada com coisas conhecidas.
- **Fora de casa**: os trechos em que um aparelho saiu do lugar de sempre.
  Dá para programar uma saída antes; quando o aparelho volta, o app acha o
  horário pelo que ele mediu e pergunta antes de marcar.
  Cada saída mostra o aparelho que foi junto, e o app pergunta "Voltou para
  casa?" quando as leituras voltam a bater com as de casa. O **Resumo da
  saída** traz horário, dose, pico, gráfico, o espectro da saída comparado
  com o de casa e o que o aparelho registrou (bateria, temperatura, avisos).
- **Clima**: chuva (radar RainViewer e modelo Open-Meteo) e pressão
  × radônio. Opcional; a cidade fica só nos ajustes deste PC.
- **GQ GMC-800**: abas **Agora** (leitura, histórico da memória do
  contador, relógio, ajustes do aparelho e calibração, inclusive gravar no
  próprio contador o fator da intercalibração com o Radiacode 102) e
  **Saúde do tubo** (se o tubo Geiger está contando como deveria).
- **Radiacode 102**: leitura, **Tela do aparelho** (o espectro que o
  aparelho mostra, que o Recomeçar zera) e **Acumulado do aparelho** (o
  espectro de longo prazo, só leitura), os dois com isótopos e zoom no
  pico; **Espectrograma** (espectro ao longo do tempo), busca, atividade
  (Bq), log, ajustes e alarmes do aparelho, e o histórico que ele gravou
  longe do PC. Cada cartão se explica em "Como funciona".
- **Configurações**: alarmes e níveis do app, janela e tela (tamanho,
  densidade, número no ícone da bandeja, pulso), dados, backup e
  sincronização entre PCs.

| Resumo | Alertas |
|---|---|
| ![Resumo do dia](docs/prints/pt-resumo.png) | ![Alertas com o detalhe ao lado](docs/prints/pt-alertas.png) |
| **Radiacode 102** | **Fora de casa** |
| ![Aba do Radiacode 102](docs/prints/pt-radiacode.png) | ![Saídas de casa](docs/prints/pt-fora.png) |

Os prints são do modo demonstração, com dados de mentira.

Busca rápida com **Ctrl+K**. O botão do logo serve para anotar, marcar que
você saiu de casa e medir um objeto.

## Dose

A sua dose vem do aparelho que anda com você, normalmente o Radiacode 102.
O valor principal é o **contador de dose do próprio aparelho**; ao lado, o
app mostra a soma que ele mesmo fez com as leituras de cada minuto, como
conferência. O monitor da casa mede o lugar, não você.

## Relatório em PDF

O botão **Relatório (PDF)** abre um diálogo para escolher entre
**Completo** e **Só o resumo**, e se entram as anotações e as configurações
(o app lembra da escolha):

1. **Resumo**: uma frase, quatro números, o gráfico dos dois aparelhos com
   as saídas marcadas e a fita dos dias. A dose vem comparada com a que você
   teria ficando em casa: normal, acima do normal ou alta.
2. **Por dia**: fundo da casa, sua dose, pico e alertas de cada dia; a lista
   de alertas (confirmados ou não pelo outro aparelho), as saídas, as
   medições, a chuva e as anotações.
3. **Espectro**: o espectro do Radiacode 102 no período e o de cada saída,
   comparados com o de casa, com os isótopos achados.
4. **Anexo técnico** (A1 a A8): estatística, dias, fundo, episódios, tempo
   acima dos limites, lacunas, configurações e limitações.

O PDF nunca leva GPS nem nome de lugar.

<p align="center"><img src="docs/prints/pt-pdf.png" alt="Página 1 do relatório em PDF" width="420"></p>

## Alarmes

- Faixas do app em µSv/h (atenção, alarme e perigo), por aparelho. O nível
  **perigo** não pode ser desligado, só silenciado por um tempo.
- O Radiacode 102 também pode usar os alarmes do próprio aparelho.
- Aviso de Geiger saturado, de leitura fora da escala e de **subida lenta
  do fundo** (quando o fundo da casa sobe aos poucos ao longo de semanas).
- Alertas retroativos: o que passou do limite com o app fechado vira alerta
  com a hora certa quando o histórico é baixado.
- Ícone na bandeja com a leitura na cor do nível; alarme e perigo trazem a
  janela para a frente.

## Onde ficam os dados

Windows: `%AppData%\RadLog\`. Linux: `~/.config/RadLog/`. O botão "pasta de
dados" abre a pasta. O que tem lá:

- `leituras/AAAA-MM.csv`: uma linha por minuto de cada aparelho, abre no
  Excel;
- `historico/` e `configuracao/`: cópias brutas da memória e da
  configuração do contador;
- `eventos.csv`, `medicoes.json`, `fora-de-casa.json`, `anotacoes.json`:
  alertas, medições, saídas de casa e anotações;
- `radiacode/`: os minutos, a dose, os espectros por minuto e as medições
  de atividade do Radiacode 102;
- `alertas-detalhe/`: as leituras segundo a segundo em volta de cada
  alerta;
- `medicoes-anexos/`: fotos e arquivos anexados às medições;
- `chuva*.json`: chuva e pressão (se o Clima estiver ligado);
- `config.json`: configurações; `log.txt` e `diagnostico.txt`: log técnico;
- `sincronizacao.json`: as chaves dos PCs pareados. Não compartilhe este
  arquivo: é ele que deixa outro PC sincronizar com o seu. Nunca entra no
  backup.

O **backup** (.zip) junta só o que falta ao importar e serve para levar os
dados de um PC para outro, Windows ou Linux. Ele também leva as
configurações que valem em qualquer PC (alarme, níveis, som, bandeja,
idioma); ao importar, o app pergunta antes de usar. Localização e cidade
nunca vão para PDF, relatório ou backup.

## Compilar

Precisa do [Go](https://go.dev/dl/) 1.26 ou mais novo (o CI usa o 1.27). Não precisa do Wails
CLI nem de Node: a interface é HTML, CSS e JS puros em `frontend/dist/`.

Windows (precisa do WebView2, que já vem no Windows 10/11):

```
scripts\build.bat
```

Linux:

```
sudo apt install golang libgtk-3-dev libwebkit2gtk-4.1-dev   # Debian/Ubuntu
./scripts/build-linux.sh
```

Para ler os aparelhos sem root no Linux, o usuário precisa estar no grupo
`dialout` (`sudo usermod -aG dialout $USER` e entrar de novo).

A interface também abre direto no navegador (`frontend/dist/index.html`) em
**modo demonstração**, com dados falsos dos dois aparelhos.

### Releases

O GitHub Actions (`.github/workflows/release.yml`) compila o `.exe`, o
AppImage e o `.tar.gz` quando a versão de `internal/app/version.go` muda no
`main`, assina o `SHA256SUMS.txt` e publica com o trecho da versão no
CHANGELOG. Quando o segredo `NOTION_TOKEN` existe, um passo a mais registra
a versão no quadro do projeto no Notion.

## Testes

```
go test ./internal/...
```

Os testes do GMC-800 usam respostas gravadas de um aparelho real.

## Estrutura

```
main.go                 embute a interface e chama o app
internal/app/           o app (os métodos exportados de App são o que a interface chama)
internal/gmc/           protocolo do GQ GMC-800 (serial, calibração, histórico)
internal/radiacode/     protocolo do Radiacode 102 (USB e Bluetooth)
internal/core/          contas puras: alertas, dose, intercalibração, níveis, estatística
internal/store/         leituras em CSV mensal
internal/pdf/           gerador de PDF mínimo, sem dependências
internal/i18n/          tradução (en.tsv)
internal/updsig/        chave pública que confere as atualizações
frontend/dist/          interface (um arquivo .js por tela)
build/                  ícone, manifesto do .exe e arquivos do Linux
scripts/                build e ferramentas de tradução
```

## Observações sobre o GMC-800

O GMC-800 diz seguir o GQ-RFC1201, mas na prática difere: CPM e CPS vêm em
4 bytes, a configuração tem 512 bytes com layout próprio e o `SPIR` às vezes
manda um byte a mais. A configuração é gravada com `ECFG`,
`WCFG[endereço alto][endereço baixo][valor]` e `CFGUPDATE`. Os detalhes
foram conferidos com o projeto [PyGMC](https://pypi.org/project/pygmc).

## Licença

Código sob a licença MIT (veja [LICENSE](LICENSE)). Fontes IBM Plex sob a SIL
Open Font License (`frontend/dist/fonts/OFL-IBM-Plex.txt`). Os avisos de
licença das bibliotecas que vão dentro do app estão em
[frontend/dist/licencas.txt](frontend/dist/licencas.txt) e em Configurações ›
Atualizações e sobre. Para avisar de um problema de segurança, veja o
[SECURITY.md](SECURITY.md).

Não é um produto da GQ Electronics nem da Radiacode. "GMC" e "GQ" são marcas
da GQ Electronics; "Radiacode" é marca da Radiacode. O app só fala com os
aparelhos pelos protocolos deles.
