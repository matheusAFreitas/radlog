# Política de privacidade do RadLog

*Atualizada em 14/11/2026* · [English](PRIVACY.md)

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
- **O que sobe:** tudo vai cifrado no seu computador com AES-256-GCM antes de subir. A chave sai de um código de recuperação que só você tem. Os nomes dos arquivos no Drive parecem aleatórios e não mostram datas nem aparelhos. Sobem:
  - leituras (minutos do GMC-800 e linhas do Radiacode 102) e os espectros;
  - alertas, anotações e medições, com os anexos delas;
  - saídas de casa, com o nome que você deu a cada uma (se esse nome for o de um lugar, ele sobe, cifrado);
  - os históricos de configuração e de calibração dos medidores;
  - o nome deste aparelho na nuvem, que por padrão é o nome do PC (dá para trocar no app).
- **O que nunca sobe:** o lugar e as coordenadas das configurações, e os metadados de foto (a localização das fotos jpg, png, webp, heic e avif é tirada antes de subir). Anexos de tipos que podem ter a localização dentro e que o app não sabe limpar (vídeos, fotos TIFF e RAW) ficam só no seu computador: não vão para a nuvem, o backup nem a sincronização pela rede.
- **Por onde passa:** direto do seu computador para o Google Drive, sem passar por servidor nosso.
- **Credenciais:** o acesso do Google e o código de recuperação ficam só no seu computador. No Windows são protegidos pelo DPAPI; no Linux são cifrados com uma chave guardada no cofre do sistema (GNOME Keyring ou KWallet) e, sem cofre, ficam num arquivo que só o seu usuário lê. No Windows, o código copiado no app não entra no histórico da área de transferência e sai dela depois de 1 minuto.
- **Para desligar:** "Desconectar do Google" no app apaga o acesso e o código do seu computador. Se ele for o único aparelho da nuvem, o RadLog também sai da lista de apps da sua conta Google. Você também pode revogar o acesso em https://myaccount.google.com/permissions e apagar a pasta RadLog do seu Drive quando quiser.

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
