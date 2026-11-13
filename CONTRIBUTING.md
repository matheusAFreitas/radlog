# Como contribuir

## Branches

Uma branch por mudança, sempre saindo do `main`, com um prefixo e um nome
curto em minúsculas, separado por hífen:

| Prefixo    | Para quê                                                   | Versão |
|------------|------------------------------------------------------------|--------|
| `feat/`    | coisa nova ou mudança que o usuário percebe                | minor ou major |
| `fix/`     | correção, texto, ajuste visual pequeno                     | patch  |
| `chore/`   | arrumação sem versão nova (CI, docs, scripts, workflows)   | nenhuma |
| `hotfix/`  | correção urgente de uma versão publicada; fura a fila      | patch  |

Exemplos: `feat/resumo-da-saida`, `fix/alerta-com-filtro-vazio`,
`chore/apagar-branches`, `hotfix/atualizador-sem-assinatura`.

O prefixo diz o tipo da mudança; quem decide o número é o `version.go` do PR,
pelo SemVer: patch para correção, minor para feature, major para um marco
grande (ex.: o app de Android, 3.0.0). Major também usa `feat/`
(ex.: `feat/app-android`).

## Fluxo

1. Criar a branch a partir do `main` atualizado.
2. Abrir o PR para o `main`. Toda mudança que gera versão leva junto
   `internal/app/version.go`, `build/windows/winres.json` e `CHANGELOG.md`.
3. Merge com o CI verde. A release sai do `main` sozinha quando o
   `version.go` muda, uma por vez: o atualizador do app lê o
   `/releases/latest`, então só se faz merge de outra versão depois que a
   anterior estiver publicada como latest.
4. Apagar a branch depois do merge (o GitHub faz isso sozinho com
   "Automatically delete head branches" ligado; se não, use o workflow
   manual **apagar-branches** em Actions).

Não há branch `develop`: cada merge no `main` já é uma versão publicada.

## Antes do PR

O CI confere tudo isto; vale rodar antes para não esperar a volta:

1. **Formato e testes:** `gofmt -l internal` sem saída, `go vet ./internal/...`
   e `go test ./internal/...` (o CI também roda com `-race` e no Windows).
   Use Go 1.27 (o mesmo do `go.mod` e do CI). Sem internet, dá para rodar
   com um `local.mod` próprio (`GOFLAGS=-modfile=local.mod GOPROXY=off`);
   ele fica fora do repositório (`.gitignore`) e não é necessário para
   quem tem internet.
2. **staticcheck:** `staticcheck -tags webkit2_41 ./...` e, para o código do
   Windows, `GOOS=windows staticcheck -tags desktop,production ./...`.
3. **Tradução:** toda frase nova da tela entra em `internal/i18n/en.tsv`
   (`português<TAB>inglês`, com `{0}`, `{1}`… nas partes variáveis) e depois
   `python3 scripts/i18n_build.py`. `python3 scripts/i18n_check.py` reprova
   frase nova sem tradução; os pendentes antigos estão em
   `scripts/i18n_pendentes.txt` (ao traduzir um, tire a linha).
4. **Telas:** `node scripts/telas_test.js` abre as 11 páginas no modo
   demonstração, em duas larguras e nos dois idiomas, e falha com erro de
   script ou rolagem horizontal.
5. **Licenças:** mudou dependência no `go.mod`? Rode
   `go run ./scripts/licenses` para refazer `frontend/dist/licencas.txt`.

## Versão nova

Os três arquivos dizem o mesmo número (o CI confere com
`scripts/check-version.sh`):

- `internal/app/version.go`;
- `build/windows/winres.json` (todos os campos, inclusive o fixo `X.Y.Z.0`);
- o topo do `CHANGELOG.md`: `## X.Y.Z`, os itens em português, uma linha
  em branco, `**English**`, outra linha em branco e os mesmos itens em
  inglês, na mesma ordem. As notas da release saem desse trecho.

## Dados pessoais

Nada de dado pessoal em print, relatório, teste ou exemplo: nome de
computador, caminho de pasta, localização, cidade ou leitura real de alguém.
Os prints do README saem do modo demonstração.

## Compilar

Ver a seção "Compilar" do [README.pt-BR.md](README.pt-BR.md). No Windows,
`scripts\build.bat` testa, gera o ícone e a versão do `.exe` e compila.
