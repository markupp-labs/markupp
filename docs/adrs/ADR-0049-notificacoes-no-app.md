# ADR-0049: Notificações no painel por polling

## Status

Proposta

## Contexto

O convite só chega por email, pela caixa de saída do ADR-0021. Quem já está no painel não vê o
convite pendente sem abrir a lista de tenants. O pedido é um sino na parte de baixo da barra de
atividades que reúna o que pede atenção da conta, começando pelos convites. O painel não tem
canal em tempo real: o ActionCable está desligado e não há Redis

## Decisão

A notificação é da conta, não do tenant, e a API a expõe em `GET /v1/me/notifications`, no
mesmo caminho de conta de `/v1/me/invites`. Nesta primeira versão a única fonte é o convite
pendente, e a notificação é derivada na leitura a partir dos convites, sem tabela nova. O
contrato já carrega o tipo de cada item para receber outras fontes depois.

O painel mostra o contador no sino e atualiza por polling: um turbo-frame recarrega a cada 60
segundos, pausa com a aba oculta e recarrega quando ela volta. A lista abre num popover, e
aceitar ou recusar reaproveita as ações de convite que o painel já tem

## Alternativas consideradas

- Server-Sent Events com `pg_notify`: entrega na hora, mas exige conexão longa no ingress, um
  ouvinte no Go e mais estado no painel, para um evento que raramente é urgente
- ActionCable no painel: precisa de Redis ou solid_cable e o painel não fala com o banco
- Tabela de notificações já nesta versão: duplicaria o estado do convite e exigiria manter as
  duas coisas coerentes ao aceitar, recusar, revogar ou expirar
- Reaproveitar a tabela `notifications`: ela é a caixa de saída de emails, com outro dono, outro
  papel e outro ciclo de vida

## Consequências

- O convite aparece no painel com até 60 segundos de atraso
- Não há estado de lido: o contador é o número de convites pendentes e cai quando a conta
  responde
- Uma fonte que não seja derivável, como a marcação de pessoas numa nota, vai exigir tabela
  própria e um novo ADR
