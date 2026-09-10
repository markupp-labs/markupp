# ADR-0021: Envio de email com provedor configurável

## Status

Aceita

## Contexto

Convite de usuário, recuperação de acesso por credencial local, recibo e aviso de cobrança e
alerta de operação dependem de envio de mensagem. O self-host não pode depender de serviço de
terceiro para funcionar, e o Enterprise precisa de entrega confiável e mensurável

## Decisão

Envio por provedor configurável: SMTP genérico e Amazon SES. O Enterprise usa SES, o self-host
aponta para o próprio SMTP. Sem provedor configurado, o sistema não envia e os fluxos que
dependem de envio ficam explicitamente indisponíveis

## Alternativas consideradas

- Só SES: obriga quem hospeda por conta própria a manter conta na AWS
- Só SMTP, inclusive no Enterprise via SES por SMTP: funciona, mas abre mão do controle de
  reputação, da métrica de entrega e do tratamento de rejeição que a API do SES entrega
- Fila dedicada de notificação desde o início: complexidade antes de existir volume. Envio
  assíncrono com repetição resolve, e fila entra quando o volume for medido

## Consequências

- É o terceiro provedor configurável do projeto, junto de embedding (ADR-0019) e autenticação
  (ADR-0020), o que justifica extrair o padrão uma vez em vez de repetir a estrutura três vezes
- Recuperação de acesso por credencial local só existe onde há provedor de email configurado
- Envio sai do caminho da requisição e vai para o mesmo trabalho assíncrono da indexação
  (ADR-0018), senão a latência da resposta fica presa à do servidor de email
- Notificação dentro da interface, sem email, é decisão separada e não está coberta aqui
