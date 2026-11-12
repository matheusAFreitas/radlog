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
2. Generate a new reserve pair (`go run ./scripts/sign gen`), keep its
   private half offline, and put the new public half in `ReserveKey`.
3. Publish a release; from then on the old reserve key is the main one.

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
nesse caso, são os três da seção em inglês: pôr a privada de reserva no
segredo, gerar uma reserva nova e publicar uma versão.
