# Formato da nuvem do RadLog (versão 2)

Este documento descreve os arquivos que o RadLog grava na pasta `RadLog` do
Google Drive da pessoa. Ele serve para outro leitor (o app de Android, por
exemplo) ler e escrever a mesma nuvem. O código de referência está em
`internal/sync` (cifra, nomes e envelope) e em `internal/app/cloudsync.go`
(o que vai em cada arquivo).

A versão 2 entrou na 2.80.0. Uma nuvem da versão 1 (2.79.x) não é convertida:
o app pede para começar uma nova, e a pasta antiga vai para a lixeira do Drive.

## Pasta e metadados

A pasta fica na raiz do Drive, com o nome `RadLog`. O app só enxerga o que ele
mesmo criou (escopo `drive.file`).

O único arquivo em claro é o `radlog-nuvem.json`:

```json
{"v": 2, "salt": "<32 bytes em base64>", "check": "<base64>"}
```

- `v` é a versão dos metadados (`MetaVersion`). Com `v` menor, o app mostra
  "nuvem do formato antigo". Com `v` maior, ele recusa e pede para atualizar.
- `salt` é sorteado quando a nuvem é criada.
- `check` é `Seal("radlog-nuvem.json", "radlog-nuvem-v1")`. Se ele abre, o
  código de recuperação está certo.

## Chaves

1. O código de recuperação tem 24 símbolos Crockford base32, mostrados em 6
   grupos de 4 (`K7QF-3XWM-…`). Antes de usar, ele é normalizado: sem traços
   nem espaços, em maiúsculas, com O→0 e I/L→1.
2. Aplique HKDF-SHA256 com IKM = o código normalizado (ASCII), salt = `salt`
   e info = `radlog nuvem v1`, gerando 64 bytes.
3. Os bytes 0 a 31 são a chave `enc` (AES-256-GCM). Os bytes 32 a 63 são a
   chave `name` (HMAC-SHA256).

## Nomes dos arquivos

- `FileName(x)` = os 32 primeiros caracteres hexadecimais de
  `HMAC-SHA256(name, x)`.
- `Prefix(aparelho)` = `FileName("aparelho|" + aparelho)[:8]`.
- `Name(aparelho, chave)` = `Prefix(aparelho) + FileName(aparelho + "|" + chave)[:24]`.

Todos os arquivos de um aparelho começam com o mesmo prefixo de 8 letras. Assim
dá para remover um aparelho sem saber os nomes. O nome não mostra data nem
tipo.

Cada aparelho só escreve arquivos com o próprio prefixo e lê os dos outros.

## Cifra e envelope

O conteúdo de cada arquivo (todos menos o `radlog-nuvem.json`) é:

```
nonce (12 bytes) | AES-256-GCM(enc, nonce, envelope, dado associado = nome do arquivo)
```

O nome entra como dado associado. Um arquivo renomeado ou trocado de lugar não
abre.

O envelope é:

```
"RL" | versão (1 byte, hoje 2) | tipo (1 byte) | conteúdo
```

| Tipo | Chave do arquivo | Conteúdo |
|---|---|---|
| `D` | `aparelho` | JSON do aparelho (abaixo) |
| `C` | `comum` | zip com os pequenos |
| `P` | `dia\|AAAA-MM-DD` | zip com um dia (UTC) |
| `S` | `espectros\|AAAA-MM` (ou `espectros\|sem-data`) | zip com os espectros fechados do mês |
| `X` | `anexo\|<UID da medição>/<arquivo>` | um anexo de medição |

Os zips têm a mesma estrutura do "Exportar backup", com um `backup.json` de
`{"app":"radlog","version":"nuvem"}`, sem nome do PC nem data:

- **comum**:
  - `eventos.csv` (alertas encerrados)
  - `fora-de-casa.json`
  - `medicoes.json`
  - `anotacoes.json`
  - `apagados.json`
  - os pequenos do RC102 e os históricos de configuração e calibração
- **dia**:
  - `leituras/sincronizacao.csv` (minutos do GMC-800)
  - as linhas do RC102 daquele dia
  - `alertas-detalhe/<início>-….json`, para os alertas que começam no dia
- **espectros**: os arquivos `radiacode/espectros/…` fechados.

O anexo é:

```
tamanho do cabeçalho (4 bytes, big-endian) | cabeçalho JSON | bytes do arquivo
```

O cabeçalho é `{"uid": "<UID da medição>", "start": <início em s>, "file": "<nome>"}`.
Fotos sobem sem EXIF e sem lugar.

O aparelho (`D`) é:

```json
{"id": "<16 hex>", "name": "nome do PC", "lastUp": <ms>, "writer": "<16 hex>", "seq": 3}
```

- `id` fica guardado em `nuvem-aparelho.json`. Depois de desconectar e entrar
  de novo, o mesmo PC continua o mesmo aparelho.
- `writer` muda a cada instalação: quando a nuvem é ligada de novo, quando a
  chave fica ilegível ou quando o PC tem outro nome.
- Se o arquivo do próprio aparelho mudou na nuvem sem ter sido escrito daqui,
  e o `writer` é outro, a pasta foi copiada para outro PC. Nesse caso a nuvem
  pausa e pergunta se este é um aparelho novo ou o mesmo.

## Quem lê

- Um arquivo que não abre (`ErrOpen`) ou que não é um envelope (`ErrFormat`)
  é marcado como visto e não é baixado de novo.
- Um arquivo de versão maior, ou de tipo desconhecido, é marcado como
  `v<versão daqui>:<md5>`. Ele é baixado de novo quando o app atualizar.
- A ordem de leitura é: aparelhos, depois o comum, depois dias e espectros, e
  por último os anexos. O anexo procura a medição pelo UID e, sem UID, pelo
  início. Se a medição ainda não chegou, o anexo fica para a próxima vez.
- Um pacote que não junta é tentado de novo, sem travar o resto.

## Editar e apagar

Anotações, medições e saídas de casa têm um `uid` (o mesmo em todos os
aparelhos) e `edited` (ms da última edição).

- Os itens de antes da 2.80 ganham um UID tirado do próprio item:
  - anotação: `n-` + 16 hex do SHA-256 de `texto|time|end`
  - medição: `m-<início em s>`
  - saída: `p-<from>`
- Na mescla, a edição mais nova vale.
- `apagados.json` é uma lista de `{"k": "n"|"m"|"p", "id": "<uid>", "t": <ms>}`.
  Um item com marca de apagado é tirado e não volta.

## Vetores de teste

Use o código `K7QF-3XWM-9ABC-DEFG-HJKM-NPQR`, o sal `00 01 02 … 1f` (32 bytes)
e o aparelho `0011223344556677`:

| | |
|---|---|
| `enc` | `e77684f9861da824d669fb7672691fc1e011378a7ffe9b031ebca178c5716184` |
| `name` | `5ac18db1b9a79dfaa2b9c757562d8b6e9e49b3f19a78c773649bd16f6d7c7eb0` |
| `Prefix` | `0ce3f80b` |
| `Name(…, "comum")` | `0ce3f80b47db492ec604b7cc7ede40b2` |
| `Name(…, "aparelho")` | `0ce3f80b16eaf95f0227232136a5fecf` |

O arquivo do aparelho, com nonce zerado, é:

```
000000000000000000000000cf794b1dac5031aa2d0cee0c826097dd0abf758a1286871ebd4cb84324a828d7c9f837fef8f1673848b5ecfe75b5a93eb34c246f0407a85a87ed6fb1c58ccd7143ce0738195b929d995d52f42289d6664404e087a3e66c9d92
```

Aberto com o nome `0ce3f80b16eaf95f0227232136a5fecf`, ele dá o envelope `RL`,
2, `D`, seguido de:

```
{"id":"0011223344556677","name":"pc","lastUp":0,"writer":"w","seq":1}
```

O teste `TestFormatVectors` (`internal/sync/format_test.go`) confere esses
valores.
