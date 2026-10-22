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
pulumi package add github.com/markupp-labs/markupp/deploy/pulumi@v1.0.0-rc.2
```

```yaml
resources:
  markupp:
    type: markupp:index:Markupp
    properties:
      namespace: markupp
      image: ghcr.io/markupp-labs/markupp:1.0.0-rc.2
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
o envia só por email. O bloco `email` vazio vale como sem email, porque o Pulumi YAML não omite
propriedade, e o bloco com `from` sem `smtpUrl` é recusado.

Os emails de convite, verificação de email e troca de senha levam primeiro o link da página do
painel que recebe o token (/invite_acceptance/new, /email_confirmation/new e
/password_reset/new, com ?token=), e depois o token em texto para quem não conseguir abrir o
link. O painel é o de `MARKUPP_PANEL_URL` no notificador, a mesma variável do OAuth descrita
abaixo, com `http://localhost:3999` como padrão. No componente, ela vem de `email.panelUrl`.

A API limita login, refresh, aceite de convite, pedido de verificação de email e pedido e troca
de senha a 5 tentativas por minuto por IP e, quando o corpo traz email, por email (ADR-0044).
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

As duas sobem sem email. Para ligar numa stack, com a URL do SMTP cifrada no arquivo da stack:

```sh
pulumi config set --path --secret email.smtpUrl 'smtps://usuario:senha@host:465'
pulumi config set --path email.from 'markupp <nao-responda@exemplo.com>'
pulumi config set --path email.panelUrl https://painel.exemplo.com
```

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

Nada precisa ser configurado antes: `docker compose up` também funciona. Servidor em
`http://localhost:8080`, servidor MCP em `http://localhost:8080/mcp` e caixa de email em
`http://localhost:8025`. Na primeira subida o servidor gera os próprios segredos e os guarda no
banco (ADR-0041). O consentimento do OAuth abre no painel em `http://localhost:3999`, que roda
fora do compose. Os testes rodam no host e precisam de Go e de Docker:

```sh
make test
```

O teste de ponta a ponta do servidor MCP sobe a stack num projeto do compose à parte, faz o
OAuth e o login do stdio contra o Go real e apaga tudo no fim. Precisa de Node 26 com `npm ci`
feito em `mcp/`; com a stack de dev de pé, troque as portas por `MARKUPP_PORT` e `MAILPIT_PORT`:

```sh
make e2e-mcp
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
`POST /tenants` e vira owner dele. Só o owner apaga o tenant, em `DELETE /tenants/{id}`, e
apaga de vez: cofres, notas, vínculos e convites vão junto, e as contas continuam.

A conta de auto-registro (`MARKUPP_AUTH_REGISTRATION_ENABLED=true`) nasce sem email provado e
não recebe convite pela lista até verificar. Com SMTP configurado, ela recebe o email de
verificação no cadastro, confirma com o token logada nela em `POST /auth/email/verify` e pode
pedir outro em `POST /auth/email/verification`. Sem SMTP, nada
prova o email dela, e qualquer pessoa pode ocupar um email alheio antes do dono: só ligue o
auto-registro com SMTP.

Com SMTP, quem esqueceu a senha pede a troca em `POST /auth/password/forgot` e troca com o token
do email em `POST /auth/password/reset`.

## OAuth

O authorization server OAuth 2.1 vem ligado, e nenhuma variável é obrigatória. A única que um
self-host normalmente define é a URL pública:

- `MARKUPP_PUBLIC_URL`: o issuer, como `https://notas.exemplo.com`, sem caminho, query nem
  fragmento. Padrão: `http://localhost` na porta do servidor
- `MARKUPP_PANEL_URL`: o painel que faz o login e o consentimento, como
  `https://notas.exemplo.com/painel`, sem query nem fragmento. Padrão: `http://localhost:3999`
- `MARKUPP_OAUTH_RESOURCES`: os servidores de recursos que aceitam os tokens, em pares
  `url=segredo` separados por vírgula, cada segredo com ao menos 32 bytes (ex.:
  `openssl rand -hex 32`) e cada URL uma vez só, sem query nem fragmento. Sem ela, o único
  resource é o servidor MCP em `MARKUPP_PUBLIC_URL/mcp`, com segredo gerado e guardado no banco
- `MARKUPP_AUTH_SECRET`: o segredo do JWT. Sem ela, o servidor gera um na primeira subida e o
  guarda no banco, que todas as réplicas leem

Definidas, as variáveis valem no lugar do que está no banco, que é como o cluster opera.

As URLs exigem https e um host, sem usuário nem senha; http só vale para localhost, 127.0.0.1
e [::1]. O servidor guarda cada URL com esquema e host em minúsculas, sem a porta padrão do
esquema (443 em https, 80 em http) e sem barra final, então `https://MCP.exemplo.com:443/mcp/` e
`https://mcp.exemplo.com/mcp` contam como o mesmo recurso.

As validades do fluxo são opcionais e só são lidas com o OAuth ligado. Cada uma aceita um
inteiro dentro da faixa, e um valor fora dela impede o servidor de subir:

| Variável | Padrão | Faixa | O que controla |
|---|---|---|---|
| `MARKUPP_OAUTH_ACCESS_MINUTES` | 60 | 1 a 1440 | validade do access token |
| `MARKUPP_OAUTH_REFRESH_DAYS` | 30 | 1 a 365 | validade do refresh token |
| `MARKUPP_OAUTH_REQUEST_MINUTES` | 10 | 1 a 60 | espera pelo consentimento no painel |
| `MARKUPP_OAUTH_CODE_SECONDS` | 60 | 10 a 600 | validade do authorization code |
| `MARKUPP_OAUTH_EXCHANGE_MINUTES` | 5 | 1 a 15 | validade do token da API entregue ao resource pela troca de token |

Na troca de token, o servidor MCP se autentica em `POST /oauth/token` por HTTP Basic, com a URL
do resource como usuário e o segredo dele como senha, os dois percent-encoded em
application/x-www-form-urlencoded antes do Base64 (RFC 6749, seção 2.3.1). Sem essa codificação,
os dois-pontos da URL quebram o par usuário e senha e a troca responde 401.

O token da API entregue pela troca não passa pelo banco: revogar a Autorização ou os tokens
OAuth não o derruba, e ele segue valendo até expirar (ADR-0046). Por isso a validade dele é
curta.

Os clientes também podem se identificar por um Client ID Metadata Document: o client_id é
uma URL https que o servidor busca para ler o nome e os redirect_uris do cliente. A busca
tem limites próprios, também opcionais e lidos só com o OAuth ligado:

| Variável | Padrão | Faixa | O que controla |
|---|---|---|---|
| `MARKUPP_OAUTH_CIMD_TIMEOUT_SECONDS` | 5 | 1 a 30 | tempo máximo da busca do documento |
| `MARKUPP_OAUTH_CIMD_MAX_KB` | 64 | 1 a 1024 | tamanho máximo do documento |
| `MARKUPP_OAUTH_CIMD_CACHE_MIN_MINUTES` | 5 | 1 a 60 | menor tempo de cache do documento e tempo em que uma busca que falhou fica lembrada, recusando o client_id sem buscar de novo |
| `MARKUPP_OAUTH_CIMD_CACHE_MAX_HOURS` | 24 | 1 a 168 | maior tempo de cache do documento |

O tempo de cache vem do max-age do Cache-Control da resposta, limitado ao intervalo acima;
sem o cabeçalho, o documento fica uma hora em cache. O mínimo não pode passar do máximo.

A busca do documento ignora HTTP_PROXY e HTTPS_PROXY e recusa destinos fora da internet
pública, então o servidor precisa de saída direta para a internet.

Clientes sem documento publicado podem se registrar sozinhos em `POST /oauth/register`
(registro dinâmico, RFC 7591), sem login. O registro só aceita clientes públicos, sem
client_secret, e as mesmas redirect_uris do documento. Um cliente registrado sem nenhuma
autorização ativa conta como pendente, com o prazo contado desde o registro ou desde a
última revogação de autorização dele, o que vier depois: o servidor apaga os pendentes com
o prazo cumprido, junto com as autorizações revogadas deles (a auditoria fica), e, com o
teto atingido, recusa novos registros com 429 até abrir vaga. As
variáveis são opcionais e lidas só com o OAuth ligado:

| Variável | Padrão | Faixa | O que controla |
|---|---|---|---|
| `MARKUPP_OAUTH_DCR_ENABLED` | false | true ou false | liga o registro dinâmico; desligado, `/oauth/register` responde 404 e a metadata não o anuncia |
| `MARKUPP_OAUTH_DCR_MAX_PENDING` | 1000 | 1 a 100000 | teto de clientes pendentes |
| `MARKUPP_OAUTH_DCR_PENDING_HOURS` | 24 | 1 a 168 | prazo, desde o registro ou a última revogação, para o cliente pendente receber uma autorização antes de ser apagado |

## Servidor MCP

No self-host o servidor MCP fica na mesma origem, em `/mcp`: o servidor Go encaminha a rota e a
metadata do recurso protegido ao container do MCP. O Go escreve o segredo do MCP num arquivo de
volume compartilhado, e o MCP descobre o issuer pela metadata do Go. O compose já liga as duas
pontas:

| Variável | Onde | Padrão | O que é |
|---|---|---|---|
| `MARKUPP_MCP_UPSTREAM_URL` | Go | sem `/mcp` | endereço interno do servidor MCP, para onde `/mcp` é encaminhado |
| `MARKUPP_MCP_SECRET_PATH` | Go e MCP | sem arquivo | arquivo do segredo do MCP: o Go escreve e o MCP lê |
| `MARKUPP_MCP_API_URL` | MCP | `http://localhost:8080` | endereço do servidor Go visto pelo MCP; aceita http em qualquer host |
| `MARKUPP_MCP_RESOURCE_URL` | MCP | issuer + `/mcp` | URL pública do MCP com o caminho do endpoint; o caminho vira a rota |
| `MARKUPP_MCP_RESOURCE_SECRET` | MCP | o do arquivo | segredo do par em `MARKUPP_OAUTH_RESOURCES`, com ao menos 32 bytes |
| `MARKUPP_MCP_PORT` | MCP | 3000 | porta em que o MCP escuta, em todas as interfaces |

O servidor MCP sobe com `markupp-mcp http` (a imagem já começa assim) e espera até 60 segundos
pelo Go e pelo arquivo do segredo. No cluster, com o MCP em subdomínio próprio (ADR-0045), o
`MARKUPP_OAUTH_RESOURCES` do Go e o `MARKUPP_MCP_RESOURCE_URL` e o `MARKUPP_MCP_RESOURCE_SECRET`
do MCP vêm do Secret.

## Aviso

Com o banco vazio, quem fizer o primeiro login vira owner do tenant inicial
([ADR-0035](adrs/ADR-0035-conta-global-com-vinculo-por-tenant.md)).
Faça esse login antes de expor o servidor publicamente.
