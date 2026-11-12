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
