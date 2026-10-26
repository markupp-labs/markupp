# ADR-0021: Envio de email com provedor configurável

## Status

Aceita

## Contexto

Convite de usuário, recuperação de acesso por credencial local e alerta de operação dependem
de envio de mensagem. O self-host não pode depender de serviço de terceiro para funcionar,
e o Multi-tenant precisa de entrega confiável e mensurável

## Decisão

Envio por SMTP genérico nas duas edições, configurado por URL. Cada instalação aponta para o
servidor de email que tiver. Sem SMTP configurado, o sistema não envia e os fluxos que dependem
de envio ficam explicitamente indisponíveis

## Alternativas consideradas

- API de um serviço de envio: traz métrica de entrega e tratamento de rejeição, e prende a
  instalação a um fornecedor, enquanto qualquer serviço de envio também aceita SMTP
- Fila dedicada de notificação desde o início: complexidade antes de existir volume. Envio
  assíncrono com repetição resolve, e fila entra quando o volume for medido

## Consequências

- Métrica de entrega e tratamento de rejeição ficam com o servidor de email escolhido
- Recuperação de acesso por credencial local só existe onde há provedor de email configurado
- Envio sai do caminho da requisição e vai para o mesmo trabalho assíncrono da indexação
  (ADR-0018), senão a latência da resposta fica presa à do servidor de email
- Notificação dentro da interface, sem email, é decisão separada e não está coberta aqui
