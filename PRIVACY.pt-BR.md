# Política de privacidade do RadLog

*Atualizada em 13/11/2026* · [English](PRIVACY.md)

O RadLog é um app de computador gratuito e de código aberto que grava as leituras de medidores pessoais de radiação (GQ GMC-800 e Radiacode 102). Esta página explica o que acontece com os seus dados.

## Resumo

- O RadLog **não tem servidor** próprio. Não existe conta com a gente, nem estatísticas de uso, anúncios ou rastreamento.
- As leituras ficam no seu computador, na pasta de dados do app.
- Se você ligar o backup na nuvem (opcional), o RadLog guarda uma cópia **cifrada** dos seus dados no **seu próprio Google Drive**. Ninguém mais consegue ler, nem quem faz o RadLog, nem o Google.

## Backup na nuvem (Google Drive), opcional

O backup na nuvem vem desligado. Quando você liga e entra com o Google:

- **Permissões pedidas:** `openid`, `email` e `https://www.googleapis.com/auth/drive.file`.
  - `drive.file` só deixa o RadLog ver e mudar os arquivos e a pasta que o próprio RadLog criou no seu Drive. Ele não vê, não lê e não muda nenhum outro arquivo do seu Drive.
  - `email` serve só para mostrar qual conta Google está ligada, na tela de configurações do app. Ele não vai para o registro, o backup, os PDFs nem a nuvem.
- **O que sobe:** leituras, alertas, anotações, medições e os anexos delas, cifrados no seu computador com AES-256-GCM antes de subir. A chave sai de um código de recuperação que só você tem. Os nomes dos arquivos no Drive parecem aleatórios e não mostram datas nem aparelhos.
- **O que nunca sobe:** lugar, coordenadas e metadados de foto (a localização das fotos é tirada antes de subir).
- **Por onde passa:** direto do seu computador para o Google Drive, sem passar por servidor nosso.
- **Credenciais:** o acesso do Google e o código de recuperação ficam só no seu computador (protegidos pelo DPAPI no Windows).
- **Para desligar:** "Desconectar do Google" no app apaga o acesso e o código do seu computador. Você também pode revogar o acesso em https://myaccount.google.com/permissions e apagar a pasta RadLog do seu Drive quando quiser.

O uso e a transferência de informações recebidas das APIs do Google pelo RadLog seguem a [Política de Dados do Usuário dos Serviços de API do Google](https://developers.google.com/terms/api-services-user-data-policy), incluindo os requisitos de Uso Limitado. Os dados das APIs do Google servem só para fazer o backup para você. Eles não são vendidos, não são compartilhados com terceiros, não são usados para publicidade e não são usados para treinar modelos de IA.

## Outras conexões com a internet

O RadLog só usa a internet nestas funções opcionais:

- **Ver se há versão nova:** GitHub (`api.github.com`).
- **Clima e radar de chuva:** Open-Meteo e RainViewer, com o lugar que você escolhe nas configurações. Esses serviços recebem as coordenadas necessárias para responder.
- **Sincronização entre os seus PCs na rede local:** conexão direta e cifrada entre os seus próprios computadores.

## Compartilhamento de dados

Não coletamos, vendemos nem compartilhamos dados pessoais. Não temos acesso aos seus dados.

## Contato

Dúvidas ou pedidos: abra uma issue em https://github.com/matheusAFreitas/radlog/issues.
