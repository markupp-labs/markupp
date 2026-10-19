# Deploy

Este documento cobre o deploy do **servidor** Markupp, escrito em Go. O servidor expõe a API
REST que os clientes consomem. A imagem é publicada em `ghcr.io/markupp-labs/markupp` a cada
tag `v*`, e a configuração vem de variáveis de ambiente, descritas no
[README do servidor](../markupp/README.md#configuração).

## Kubernetes

A instalação em Kubernetes é o componente Pulumi em `deploy/pulumi` (ADR-0025). Ele cria, no
namespace informado:

- o banco: um PostgreSQL de uma instância no namespace, com senha gerada, ou um Secret com a
  URL de um PostgreSQL externo;
- um Job de migração que roda antes da API a cada versão;
- o Deployment e o Service da API;
- o ScaledJob do indexador, que o KEDA usa para criar um Job por cofre com nota pendente;
- com email configurado, o Deployment do notificador e o ScaledObject que o leva a zero réplicas
  enquanto não há email na fila;
- o TriggerAuthentication que entrega ao KEDA a URL de um papel do banco que só conta os cofres
  pendentes e as notificações vencidas;
- um Gateway com HTTPRoute, e um Issuer do cert-manager para o certificado.

### Pré-requisitos do cluster

- Gateway API, com um controlador e uma GatewayClass (padrão `cilium`)
- cert-manager
- KEDA, para o indexador e o notificador. Sem ele, informe `indexer: {enabled: false}` e deixe
  `email` de fora, e banco e API sobem sem indexação e sem email. Com banco externo, o host da URL
  precisa resolver também do namespace do KEDA, e o usuário dela precisa de CREATEROLE para a
  migração dar login ao papel do scaler
- o namespace do markupp, já criado
- uma credencial com permissão no namespace para Secret, Job, Deployment, StatefulSet, Service,
  Gateway, HTTPRoute, Issuer, ScaledJob, ScaledObject e TriggerAuthentication
- uma StorageClass para o volume do PostgreSQL. Sem informar, vale a padrão do cluster
- um backend de estado do Pulumi

### Usar o componente

Em qualquer linguagem do Pulumi, inclusive YAML:

```sh
pulumi package add github.com/markupp-labs/markupp/deploy/pulumi@v1.1.0
```

```yaml
resources:
  markupp:
    type: markupp:index:Markupp
    properties:
      namespace: markupp
      image: ghcr.io/markupp-labs/markupp:1.1.0
      host: notas.exemplo.com
      acmeEmail: ops@exemplo.com
      database:
        storageClass: longhorn-fast
```

Para enviar o convite por email, informe `email` com `smtpUrl`, no formato
`smtp://usuario:senha@host:porta` ou `smtps://`, e `from`. A URL vai para o Secret do markupp,
e só o notificador a recebe: a API recebe apenas `MARKUPP_INVITE_BY_EMAIL=true`.
O email leva o token do convite, então o notificador exige TLS: com smtp:// o servidor precisa
oferecer STARTTLS. Texto puro só com `?tls=none` no fim da URL, para servidor local.
Com email, a API deixa de devolver o token do convite, e o notificador gera o token na entrega e
o envia só por email.

Com o painel web publicado, a variável `MARKUPP_PANEL_URL` no notificador, uma URL absoluta
http:// ou https:// como https://painel.exemplo.com, faz os emails de convite, verificação de
email e troca de senha levarem primeiro o link da página do painel que recebe o token
(/invite_acceptance/new, /email_confirmation/new e /password_reset/new, com ?token=), e depois o
token em texto para quem não conseguir abrir o link. Sem ela, o email leva só o token. No
componente, ela vem de `email.panelUrl`.

A API limita login, refresh, aceite de convite, pedido de verificação de email e pedido e troca
de senha a 5 tentativas por minuto por IP e, quando o corpo traz email, por email (ADR-0037).
Acima disso responde 429 com Retry-After. A variável `MARKUPP_RATE_LIMIT_PER_MINUTE`, de 1 a
10000, troca o 5. A contagem fica na memória de cada réplica, então com N réplicas o limite
efetivo chega a N vezes o configurado. O IP é o da conexão, e endereços IPv6 do mesmo prefixo
/64 contam como um só. Atrás de proxy, todo cliente aparece com o IP do proxy e divide o mesmo
limite, a menos que a variável `MARKUPP_TRUSTED_PROXIES` liste a rede do proxy: CIDRs IPv4 ou
IPv6 separados por vírgula, como 10.0.0.0/8,fd00::/8. Com ela, a conexão que vem de uma dessas
redes tem o X-Forwarded-For lido da direita para a esquerda, e o IP que conta é o primeiro que
não é proxy confiável. Header vazio ou malformado fica com o IP da conexão, e conexão de fora da
lista nunca tem o header lido. Liste só as redes dos proxies: um CIDR que cubra clientes deixa
qualquer um deles escolher o IP que conta. No componente, `trustedProxies` recebe a lista de
CIDRs e `rateLimitPerMinute` o limite, e os dois vão só para a API.

Sem outra opção, o componente cria um Gateway próprio nas portas 80 e 443. Em cluster onde
outro Gateway já ocupa essas portas, como o Cilium em hostNetwork, que exige porta única por
Gateway, informe `existingGateway` com nome, namespace e listener. O componente cria só o
HTTPRoute, e o Gateway precisa aceitar rotas do namespace do markupp em `allowedRoutes`. Nesse
caso o certificado fica com quem administra o Gateway.

### Stacks da equipe

O projeto em `deploy/stacks` usa o componente pelo caminho local e tem duas stacks:

- `prod`: o Enterprise no cluster de produção, com Let's Encrypt
- `dev`: um cluster kind que o CI sobe em cada PR e a cada push em dev para testar a implantação, com
  certificado autoassinado

Para rodar o mesmo teste da stack dev na própria máquina, com Docker, kind, helm, kubectl e
Pulumi instalados:

```sh
make smoke-kind
```

O smoke usa kubeconfig e estado do Pulumi próprios, num diretório temporário, e apaga o
cluster no fim, passe ou falhe.

Para acompanhar o mesmo ambiente de pé, com o painel Headlamp:

```sh
make kind-up      # sobe o cluster, o painel e a stack dev
make kind-painel  # mostra o token e abre o painel em http://localhost:4466
make kind-down    # apaga tudo
```

O ambiente de pé guarda kubeconfig e estado do Pulumi em `~/.cache/markupp-kind`, sem mexer
no `~/.kube/config`.

## Desenvolvimento local

O compose sobe PostgreSQL, aplica as migrações e roda o servidor com recarga automática, junto
com o indexador e o notificador em laço e o Mailpit, que recebe todo email de desenvolvimento:

```sh
make run
```

Servidor em `http://localhost:8080` e caixa de email em `http://localhost:8025`. Os testes rodam no host e precisam de Go e de Docker:

```sh
make test
```

## Validar

Smoke test pela API. Com o banco vazio, o primeiro login cria a conta como owner do tenant
inicial:

```sh
TOKEN=$(curl -s -X POST http://localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@exemplo.dev","password":"troque-esta-senha"}' \
  | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
ID=$(curl -s -X POST http://localhost:8080/notes \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"path":"validacao.md","content":"oi"}' \
  | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/notes/$ID
curl -s -X DELETE -H "Authorization: Bearer $TOKEN" http://localhost:8080/notes/$ID -w '%{http_code}\n'
```

Esperado: o login devolve `access_token`, `POST` retorna JSON com `id` UUID, `GET` traz a nota,
`DELETE` responde `204`, e qualquer rota de notas sem o token responde `401`. Rotas completas em
`markupp/openapi.yaml`.

## Tenants

A conta é da instância e participa de vários tenants, cada um com um papel
([ADR-0035](adrs/ADR-0035-conta-global-com-vinculo-por-tenant.md)). Quem tem um tenant só não
precisa dizer nada. Quem tem mais de um escolhe por requisição com o cabeçalho
`X-Markupp-Tenant`, usando um id de `GET /me/tenants`, que mostra também quantas notas a conta
lê em cada tenant e quando a mais recente mudou. Qualquer conta cria tenant em
`POST /tenants` e vira owner dele.

A conta de auto-registro (`MARKUPP_AUTH_REGISTRATION_ENABLED=true`) nasce sem email provado e
não recebe convite pela lista até verificar. Com SMTP configurado, ela recebe o email de
verificação no cadastro, confirma com o token logada nela em `POST /auth/email/verify` e pode
pedir outro em `POST /auth/email/verification`. Sem SMTP, nada
prova o email dela, e qualquer pessoa pode ocupar um email alheio antes do dono: só ligue o
auto-registro com SMTP.

Com SMTP, quem esqueceu a senha pede a troca em `POST /auth/password/forgot` e troca com o token
do email em `POST /auth/password/reset`.

## Aviso

Com o banco vazio, quem fizer o primeiro login vira owner do tenant inicial
([ADR-0035](adrs/ADR-0035-conta-global-com-vinculo-por-tenant.md)).
Faça esse login antes de expor o servidor publicamente.
