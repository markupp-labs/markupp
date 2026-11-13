# ADR-0057: Provedor de embedding no banco, com a configuração de quem hospeda vencendo

## Status

Proposta

## Contexto

O provedor de embedding é um só por instalação (ADR-0029) e só se escolhia nas variáveis do
indexador de embedding, que quem hospeda edita e reinicia. A chave do endpoint remoto também
ficava só ali

## Decisão

O administrador da instância, conta de email verificado que quem hospeda lista em
`MARKUPP_INSTANCE_ADMINS`, escolhe o provedor pela API, e a escolha fica numa linha da
instância no banco. Com `MARKUPP_EMBEDDING_PROVIDER` definida, mesmo como none, vale a
configuração de quem hospeda, e a escolha do banco fica travada. O indexador de embedding é o
único que lê as variáveis: ele relê a escolha a cada intervalo, troca o provedor sem reiniciar
e grava no banco a trava e o estado do modelo, que a API só lê

A chave do endpoint remoto fica no banco cifrada com AES-256-GCM. A chave de cifra é a
`MARKUPP_SECRET_KEY`; sem ela, é um segredo da instância gerado no banco, como os do ADR-0041,
que passa a valer também para essa chave

## Alternativas consideradas

- Só variáveis, como antes: trocar o provedor pede acesso ao servidor e reinício
- O banco vencendo as variáveis: quem hospeda perde o controle do que sai da máquina
- O administrador do tenant escolhe: o modelo vale para todos os tenants
- A API informando o estado por uma porta do indexador: mais uma porta para abrir e proteger
- A chave sem cifra, como o segredo do JWT: uma cópia do banco revela a chave de um serviço pago

## Consequências

- Quem faz self-host liga a ligação por sentido pelo painel, sem editar arquivo
- Sem `MARKUPP_SECRET_KEY`, quem tem uma cópia do banco também decifra a chave
- Perder ou trocar a `MARKUPP_SECRET_KEY` torna a chave guardada ilegível, e ela precisa ser
  gravada de novo
- O administrador vem da lista no login; tirar alguém dela vale no painel no próximo login, e
  na API na hora
