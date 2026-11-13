# Security

[Português abaixo](#segurança)

## Reporting a problem

If you find a security problem in RadLog (for example in the updater, the
sync between PCs or the backup import), please open a
[private security advisory](../../security/advisories/new) on GitHub instead
of a public issue. Say what you found and how to reproduce it. You'll get an
answer as soon as possible; RadLog is a study project kept by one person, so
there is no fixed deadline.

Only the latest release gets fixes.

## How releases are signed

Every release has a `SHA256SUMS.txt` with the SHA-256 of each file and a
`SHA256SUMS.txt.sig`, an Ed25519 signature of that file. The app (2.16 and
later) only installs an update when the signature matches one of the public
keys built into it ([internal/updsig/updsig.go](internal/updsig/updsig.go)),
the file's SHA-256 matches, and the file name carries the announced version.

Release public key (base64):

```
47Gd5qTSvqncJzSIKx3nXlBMOhHwdJkp/Wes9hC7EQk=
```

Reserve public key (base64, accepted from 2.72.0 on; see below):

```
WikNSgdGuqYSX8kLHJQ+lHqxeACcwDFeCDiQj1e/ujQ=
```

To check a download by hand: `go run ./scripts/sign verify SHA256SUMS.txt`
next to `SHA256SUMS.txt.sig`, then `sha256sum -c SHA256SUMS.txt`.

## If the signing key is lost

The private key lives in the `RADLOG_SIGN_KEY` secret of the repository and
in an offline copy kept by the maintainer. If both were lost, no installed
app would accept another update. To avoid that, the app accepts a second,
reserve public key (`ReserveKey` in `internal/updsig/updsig.go`), whose
private half is kept offline and outside GitHub and is only used if the main
key is gone. Steps then:

1. Put the reserve private key in the `RADLOG_SIGN_KEY` secret.
2. In `internal/updsig/updsig.go`, move the reserve public key into
   `PubKey` (it is the main key now). Generate a new reserve pair
   (`go run ./scripts/sign gen`), keep its private half offline, and put the
   new public half in `ReserveKey`. Update the two keys at the top of this
   file.
3. Publish a release. Installed apps accept it because it is signed with
   their reserve key; the new version trusts the new pair.

`TestKeyRotation` in `internal/updsig/updsig_test.go` rehearses these steps
with throwaway keys.

## If a signing key leaks

If someone else may have a private key (secret exposed in a log, PC
compromised), act fast, because installed apps keep accepting that key
until they update:

- **Main key leaked:** do the three steps above right away. Also revoke
  the GitHub tokens and sessions that could publish a release, and check
  the latest releases for files you didn't publish.
- **Reserve key leaked:** keep `PubKey`, generate a new reserve pair, put
  its public half in `ReserveKey` and publish a release.
- **Both leaked:** no update can be trusted through the app. Say so in the
  README and in a release, and ask people to download the next version by
  hand and check it as shown above, against keys published somewhere other
  than GitHub.

---

# Segurança

## Avisar de um problema

Se achar um problema de segurança no RadLog (no atualizador, na
sincronização entre PCs ou na importação de backup, por exemplo), abra um
[aviso de segurança privado](../../security/advisories/new) no GitHub em vez
de uma issue pública, dizendo o que achou e como repetir. A resposta vem
assim que der: o RadLog é um projeto de estudo mantido por uma pessoa, sem
prazo fixo. Só a versão mais recente recebe correção.

## Assinatura das releases

Cada release traz o `SHA256SUMS.txt` (o SHA-256 de cada arquivo) e o
`SHA256SUMS.txt.sig`, a assinatura Ed25519 dele. O app (2.16 em diante) só
instala uma versão nova se a assinatura bater com uma das chaves públicas
dele, a soma bater e o nome do arquivo trouxer a versão anunciada. As duas
chaves públicas (principal e reserva, esta desde a 2.72.0) estão acima.

## Se a chave de assinatura se perder

A chave privada fica no segredo `RADLOG_SIGN_KEY` do repositório e numa
cópia fora da internet guardada por quem mantém o app. Se as duas se
perdessem, nenhum app instalado aceitaria mais atualização. Por isso o app
aceita uma segunda chave pública, de reserva (`ReserveKey`), cuja privada
fica guardada fora do GitHub e só é usada se a principal sumir. Os passos,
nesse caso:

1. Pôr a privada de reserva no segredo `RADLOG_SIGN_KEY`.
2. Em `internal/updsig/updsig.go`, passar a pública de reserva para
   `PubKey` (ela vira a principal). Gerar um par de reserva novo
   (`go run ./scripts/sign gen`), guardar a privada fora da internet e pôr a
   pública nova em `ReserveKey`. Atualizar as duas chaves no começo deste
   arquivo.
3. Publicar uma versão. Os apps instalados aceitam porque ela vem assinada
   com a reserva deles; a versão nova já confia no par novo.

O `TestKeyRotation` em `internal/updsig/updsig_test.go` ensaia esses passos
com chaves de teste.

## Se uma chave vazar

Se outra pessoa pode ter uma chave privada (segredo num log, PC invadido),
é preciso agir rápido, porque os apps instalados aceitam essa chave até
atualizarem:

- **Vazou a principal:** os três passos acima, na hora. E revogar os tokens
  e sessões do GitHub que podem publicar release, e conferir se as últimas
  releases têm arquivo que você não publicou.
- **Vazou a de reserva:** manter a `PubKey`, gerar um par de reserva novo,
  pôr a pública em `ReserveKey` e publicar uma versão.
- **Vazaram as duas:** nenhuma atualização pelo app é confiável. Avisar no
  README e numa release, e pedir para baixar a próxima versão à mão e
  conferir como descrito acima, com as chaves publicadas em outro lugar além
  do GitHub.
