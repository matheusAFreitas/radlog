# RadLog

> Hoje ainda se chama **GMC Monitor** (versões 1.x). A partir da 2.0, com o
> RadiaCode, passa a se chamar RadLog. Veja o [ROADMAP](ROADMAP.md).

Monitor para o contador Geiger **GQ GMC-800**, feito em Go + Wails, para
Windows e Linux. Lê o contador pela USB a cada segundo, guarda uma leitura por
minuto e mostra tudo numa tela só: leitura ao vivo, gráficos, dose acumulada,
alertas, chuva × radiação, medição de objetos, relatórios em PDF e um log
técnico com níveis.

Versão atual: veja `version.go` e o [CHANGELOG](CHANGELOG.md).

## Baixar

Os arquivos prontos ficam em **[Releases](../../releases)**:

- Windows: `GMC-Monitor-vX.exe` (não precisa instalar; o WebView2 já vem no
  Windows 10/11).
- Linux: `GMC-Monitor-vX-x86_64.AppImage`, ou o `.tar.gz` com o binário e o
  `install.sh`. Precisa da WebKitGTK 4.1 (`sudo apt install libwebkit2gtk-4.1-0`).

Para atualizar, abra a versão nova com a antiga aberta: a antiga fecha e passa
o lugar para a nova sozinha.

## O que ele faz

- Leitura ao vivo em CPM, µSv/h, mrem/h ou CPS, com as faixas do manual
  (abaixo de 50 CPM seguro, 50–99 atenção, 100+ alerta).
- Converte para µSv/h usando a **calibração gravada no próprio contador**
  (se não conseguir ler, usa a de fábrica: 0,0065 µSv/h por CPM).
- Gráfico em CPM ou µSv/h, de 15 min, 30 min, 1 h, 24 h e 7 dias, média/mínimo/máximo e dose acumulada no dia.
  Arrastando no gráfico dá zoom num trecho, com a resolução mais fina disponível.
- **Dosímetro**: dose acumulada em 3 h, 6 h, 12 h, 24 h, 7 dias, 30 dias ou 1 ano, comparada
  com o fundo natural e com um teto (fundo + limite anual da CNEN/ICRP para o
  público, 1 mSv/ano, ou para trabalhador exposto, 20 mSv/ano). Os limites não
  contam a radiação natural, por isso o teto parte do seu fundo medido.
- Diz em palavras se a leitura atual está **dentro do normal** para o seu
  fundo (faixa onde caem 95% das leituras pela estatística de Poisson; acima
  de 3 desvios dificilmente é acaso).
- **Medir um objeto**: fundo e objeto pelo mesmo tempo, diferença com margem
  de erro e veredito. As medições ficam salvas e marcadas no gráfico.
- Projeção da dose em um ano no ritmo atual.
- **Mapa de calor** hora × dia da média de CPM.
- Períodos **fora de casa**: as leituras entram na dose, mas não no fundo da casa.
- **Chuva** no gráfico de CPM (radar RainViewer a cada 10 min + modelo Open-Meteo, opcional), com aviso de
  quando está buscando, desligada ou sem acesso.
- Baixa o histórico da memória do contador, salva uma cópia bruta (`.bin`)
  e junta as leituras às do app.
- Mostra se o relógio do contador está certo e acerta com o do PC num clique.
- **Ajustes do contador** pelo app: gravação na memória, estimativa rápida (FET),
  limite/volume do alarme, vibração, som de clique, volume da voz, modo escuro,
  brilho, economia de energia, LED e os **6 pontos de calibração**. A tensão do
  tubo aparece só para leitura.
  Antes de gravar, salva uma cópia da configuração em `configuracao\` e confere
  lendo de volta; se der errado, tenta voltar a anterior. Campos com valor
  estranho ficam travados, e se a calibração não parecer válida (outro layout de
  firmware) nada é gravado.
- Recomenda o intervalo de gravação (a cada minuto) e mostra quanto da memória
  do contador está ocupado e quanto tempo o espaço livre dura em cada modo.
- **Descobrir onde fica um ajuste**: tira uma "foto" da configuração, você muda
  algo no menu do contador e o app mostra quais bytes mudaram.
- Aviso de atenção a partir de 50 CPM (faixa amarela e 3 bipes).
- **Alertas ativos** na tela principal (desde quando, pico) e os 3 últimos;
  histórico completo de alertas com filtros.
- **Alertas retroativos**: ao baixar o histórico, os trechos acima de 50 CPM
  (ou do limite do alarme) que aconteceram com o app fechado viram alertas com
  a data e a hora certas.
- Alarme a partir de um limite de CPM: sirene ou bipes em loop, faixa
  laranja, título da janela e janela trazida para a frente.
- **Alarme de perigo** (padrão 1.000 CPM): vermelho, som próprio mais forte,
  não pode ser desligado (só silenciado por um tempo) e tem prioridade sobre o
  alarme do usuário.
- Aba **Níveis do app** nas Configurações: atenção, perigo, tempo do
  "Silenciar" e limites do Geiger saturado.
- Aviso de **Geiger saturado** (fim da escala ou queda brusca depois de um
  valor extremo) e de como a leitura subiu até o perigo.
- **Modo demo** para testar alarme e gráficos com um valor simulado, sem mexer
  nos arquivos (as leituras reais continuam sendo gravadas).
- **Resumo** de hoje, ontem, da semana ou de qualquer dia (média comparada
  ao seu normal, máximo, mínimo, dose, tempo medido, alertas, chuva) e
  **comparação** de dois períodos (hoje × ontem até a mesma hora, 7 dias × 7
  dias antes, dois dias quaisquer), com as curvas sobrepostas.
- Ao baixar o histórico, cada trecho novo (ex.: a noite) vem com um resumo:
  máximo, dose, alertas e chuva.
- **Chuva × radiação**: efeito médio da chuva no CPM, comparado com horários
  sem chuva, e a chuva em andamento.
- **Gráfico de cada alerta**, com o detalhe por segundo dos alertas ao vivo.
- **Ícone na bandeja** (muda de cor em atenção, alarme e perigo, inclusive no
  demo) e iniciar com o Windows.
- **Backup** (.zip); importar só junta o que falta (serve para levar os dados
  de um PC para outro, Windows ou Linux).
- Log técnico (`log.txt`) e `diagnostico.txt` para investigar travamentos da
  porta; leitura e reconexão com prazo.
- Exporta tudo para CSV.
- Reconecta sozinho se o cabo cair.

## Onde ficam os dados

`%AppData%\GMC Monitor\` (o botão "pasta de dados" no rodapé abre):

- `leituras\AAAA-MM.csv` — uma linha por minuto, abre direto no Excel
- `historico\*.bin` — cópias brutas da memória do contador
- `configuracao\*.bin` — cópias da configuração do contador
- `eventos.csv` — eventos e alertas (colunas: data, texto, tipo, nível, CPM, pico, hora do pico, fim, retroativo)
- `medicoes.json` — medições de objeto salvas
- `fora-de-casa.json` — períodos marcados como fora de casa
- `alertas-retro.ok` — marca que o histórico antigo já foi conferido
- `chuva.json` — chuva por hora baixada do Open-Meteo (se ligado)
- `alertas-detalhe/` — leituras por segundo de cada alerta ao vivo (para o gráfico do alerta)
- `chuva-radar.json` — chuva lida no radar (RainViewer) a cada 10 min, 60 dias
- `memoria.json` — uso da memória do contador no último histórico baixado
- `config.json` — configurações
- `log.txt` (e `log-anterior.txt`) — log técnico; `diagnostico.txt` — estado do app quando algo travou
- `anotacoes.json` — anotações do gráfico

No Linux, a pasta é `~/.config/GMC Monitor/`.

## Compilar

Precisa do [Go](https://go.dev/dl/) (1.22 ou mais novo) e do WebView2, que já
vem no Windows 10/11. Não precisa do Wails CLI nem de Node.

Na pasta do projeto:

```
build.bat
```

Isso baixa as dependências (`go mod tidy`), roda os testes e gera
`GMC-Monitor.exe`.

As releases são geradas pelo GitHub Actions (`.github/workflows/release.yml`):
uma tag `vX.Y.Z` igual ao `AppVersion` de `version.go` compila o `.exe`, o
AppImage e o `.tar.gz` e publica com o trecho da versão no CHANGELOG.

### Linux

O pacote `gmc-monitor-vX-linux-x86_64.tar.gz` já traz o binário pronto, o
`install.sh` (atalho no menu) e o `build-appimage.sh` (gera o AppImage; os
arquivos ficam em `packaging/linux/`). Precisa da WebKitGTK 4.1 instalada
(`sudo apt install libwebkit2gtk-4.1-0`).

Para compilar no próprio Linux:

```
sudo apt install golang libgtk-3-dev libwebkit2gtk-4.1-dev   # Debian/Ubuntu
./build-linux.sh
```

Para ler o contador sem root, o usuário precisa estar no grupo `dialout`
(`sudo usermod -aG dialout $USER` e entrar de novo). A porta aparece como
`/dev/ttyUSB0`. No Linux não há ícone na bandeja, "iniciar com o sistema"
(o resto é igual, e o backup de um abre no outro).

## Estrutura

```
main.go                 abre a janela (Wails)
app.go                  tudo que a tela chama: conexão, leitura, histórico, gráficos
devconfig.go            ler e gravar os ajustes do contador
dose.go                 cálculo do dosímetro (dose, fundo e teto)
demo.go                 modo demo e buffer das leituras por segundo
stats.go                fundo, desvios (σ) e mapa de calor
measure.go              medir um objeto
weather.go              chuva (Open-Meteo)
radar.go                chuva pelo radar (RainViewer)
rainimpact.go           chuva × radiação (efeito da chuva no CPM)
tray*.go                ícone na bandeja e iniciar com o Windows
watchdog.go             leitura travada, fechamento e diagnóstico
alertdetail.go          gráfico detalhado de cada alerta
summary.go              resumo de período (aba Resumo, dados novos)
notes.go                anotações no gráfico
backup.go               backup .zip (gerar e importar mesclando)
applog.go               log técnico (log.txt)
logs.go                 visor do log técnico (níveis, histograma, filtros)
report.go               relatório do período (dados, pasta dos relatórios)
reportpdf.go, pdf.go    o PDF do relatório (gerador próprio, sem dependências)
handoff*.go             troca automática de versão
webview.go              pasta do WebView2 e limpeza das antigas
periods.go              períodos fora de casa
levels.go               níveis do app (atenção, perigo, saturado)
saturation.go           Geiger saturado e subida até o perigo
alerts.go               alertas ativos, retroativos e o eventos.csv
alarm.go                título e foco da janela durante o alarme
internal/gmc/           protocolo do contador (serial, comandos, calibração, histórico)
internal/store/         gravação das leituras em CSV mensal
frontend/dist/          interface (HTML, CSS e JS puros, sem build)
winres/                 ícone e manifesto do .exe
```

A interface também abre direto no navegador (`frontend/dist/index.html`) em
**modo demonstração**, com dados falsos — útil para mexer no visual sem o
contador.

## Testes

```
go test ./internal/...
```

Os testes usam um GMC-800 simulado com respostas gravadas de um aparelho real.
`app_e2e_test.go` testa o app inteiro contra uma porta serial simulada (só roda
com a variável `GMC_TEST_PORT` definida).

## Observações sobre o protocolo

O GMC-800 diz seguir o GQ-RFC1201, mas na prática difere: CPM e CPS vêm em
4 bytes, a configuração tem 512 bytes com layout próprio e o `SPIR` às vezes
manda um byte a mais. A configuração é gravada com `ECFG`, `WCFG[endereço alto][endereço baixo][valor]`
e `CFGUPDATE`, como a GQ explicou no fórum para os modelos de 512 bytes. Os detalhes foram conferidos com o projeto
[PyGMC](https://pypi.org/project/pygmc), que testou num GMC-800 real.

## Licença

Código sob a licença MIT (veja [LICENSE](LICENSE)). Fontes IBM Plex sob a SIL
Open Font License (`frontend/dist/fonts/OFL-IBM-Plex.txt`).

Não é um produto da GQ Electronics. "GMC" e "GQ" são marcas deles; o app só
fala com o contador pelo protocolo público GQ-RFC1201.
