# Deploy

Este documento cobre o deploy do **servidor** Markupp, escrito em Go. O servidor expõe a API
REST que os clientes consomem. A imagem é publicada em `ghcr.io/markupp-labs/markupp` a cada
tag `v*`, e a configuração vem de variáveis de ambiente, descritas no
[README do servidor](../markupp/README.md#configuração).

## Docker

Para rodar o markupp numa máquina, sem cluster, o `docker-compose.yaml` da raiz sobe o
PostgreSQL, as migrações, o servidor, o indexador, o indexador de embedding, o notificador, o
painel e o servidor MCP a partir das imagens publicadas, sem buildar nada. Baixe o `docker-compose.yaml` e o
`example.env` da raiz do repositório para uma pasta, copie o `example.env` para `.env` e
preencha:

- `POSTGRES_PASSWORD`, só com letras e números;
- `SECRET_KEY_BASE`, gerado com `openssl rand -hex 64`;
- `MARKUPP_VERSION`, a versão das imagens, sem o `v` da tag.

```sh
docker compose up -d
```

Tudo em `http://localhost:6275`: o painel na raiz, a API em `/v1` e o servidor MCP em `/mcp`.
Só essa porta é publicada; `MARKUPP_PORT` troca o lado do host. Na primeira subida o servidor gera os próprios segredos e os guarda
no banco (ADR-0041).

Com um proxy HTTPS na frente (Caddy, Traefik, Nginx), aponte `MARKUPP_PUBLIC_URL` para o
domínio público e troque `MARKUPP_PANEL_FORCE_SSL` para `true`. O painel segue na raiz do mesmo
domínio (ADR-0058); `MARKUPP_PANEL_URL` só é preciso com o painel em subdomínio próprio.

Sem `MARKUPP_SMTP_URL`, a instância roda sem email: o notificador fica ocioso e o convite
devolve o token para ser repassado à mão. Com SMTP, preencha também `MARKUPP_SMTP_FROM`, e o
convite passa a sair por email.

Para atualizar, troque `MARKUPP_VERSION` no `.env` e rode `docker compose up -d` de novo. As
migrações rodam antes do servidor.

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
abaixo, com `MARKUPP_PUBLIC_URL` como padrão. No componente, ela vem de `email.panelUrl`.

Em 24 horas, cada conta emite até 20 convites, somando todos os tenants dela, e cada tenant
recebe até 50; acima disso a API responde 429 `invite_limit_reached`. Convite revogado continua
contando, e as variáveis `MARKUPP_INVITE_LIMIT_PER_ACCOUNT` e `MARKUPP_INVITE_LIMIT_PER_TENANT`,
de 1 a 10000, trocam os dois números. A contagem fica no banco e vale entre réplicas. Com SMTP,
só convida a conta que já confirmou o email, e o assunto do convite não leva o nome do tenant:
os dois impedem que o remetente da instância vire relay de spam e phishing.

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
CIDRs e `rateLimitPerMinute` o limite, e os dois vão só para a API. A mesma lista decide o que
o Go repassa ao painel e ao servidor MCP que encaminha: de proxy confiável, o X-Forwarded-For
de entrada segue com o salto da conexão no fim; de outra conexão, segue só esse salto.

A leitura sem conta das notas publicadas (ADR-0048), nas rotas `/v1/public/notes/{public_id}`,
tem limite próprio: 300 pedidos por minuto por IP, somadas todas essas rotas, trocado pela
variável `MARKUPP_PUBLIC_RATE_LIMIT_PER_MINUTE`, de 1 a 10000. O IP segue as mesmas regras de
`MARKUPP_TRUSTED_PROXIES`. Um site que renderiza as páginas no servidor lendo essas rotas faz
todos os pedidos com o IP dele, então o limite precisa caber no tráfego do site, já descontado o
que o cache por ETag dele economiza. A migração 00040 cria o papel `markupp_reader`, sem login,
para essa leitura, com o mesmo CREATEROLE que a migração já exige para os outros papéis.

O painel serve as notas públicas em /p/ (ADR-0048) e lê a API pelo Service, passando o IP de
quem visita no X-Forwarded-For. O Go só lê esse IP porque o painel se apresenta como o relay da
leitura pública (ADR-0050), com um segredo só dele, que vale mesmo com o MCP e o OAuth
desligados e só para a leitura pública. O Go gera esse segredo no banco na primeira subida,
como o do MCP, e o escreve no arquivo de `MARKUPP_PANEL_RELAY_SECRET_PATH`: o compose monta
esse volume no Go e no painel, e o painel não monta o segredo do MCP. No cluster, o componente
gera o segredo num Secret e o passa à API e ao painel em `MARKUPP_PANEL_RELAY_SECRET`, que no
Go troca o segredo do banco e precisa de ao menos 32 bytes.

Sem outra opção, o componente cria um Gateway próprio nas portas 80 e 443. Em cluster onde
outro Gateway já ocupa essas portas, como o Cilium em hostNetwork, que exige porta única por
Gateway, informe `existingGateway` com nome, namespace e listener. O componente cria só o
HTTPRoute, e o Gateway precisa aceitar rotas do namespace do markupp em `allowedRoutes`. Nesse
caso o certificado fica com quem administra o Gateway.

Para subir o painel, informe `panel` com `image` e `host`. Ele fica num host próprio, sem o
encaminhamento pelo Go do compose, com uma réplica só: a renovação do token mora na memória
do processo. O componente gera a `secret_key_base` e o segredo de relay do painel num Secret,
aponta o painel para a API pelo Service e passa à API o `MARKUPP_PANEL_URL`, que leva o consentimento do OAuth e os links dos
emails para o painel. Com `existingGateway`, `panel.sectionName` escolhe o listener do host do
painel; com Gateway próprio, o componente abre um listener para ele.

Para subir o servidor MCP, informe `mcp` com `image`. Ele atende na mesma origem da API, em
`/mcp`, encaminhado pelo Go ao Service dele, e o componente guarda num Secret o segredo do
resource, que vai para a API em `MARKUPP_OAUTH_RESOURCES` e para o MCP em
`MARKUPP_MCP_RESOURCE_SECRET`.

Para administrar a instância pelo painel, informe `instanceAdmins` com os emails das contas,
que vão para a API em `MARKUPP_INSTANCE_ADMINS`. O componente gera a `MARKUPP_SECRET_KEY` no
Secret do banco e a passa à API e ao embedder.

O componente sobe o embedder num Deployment de uma réplica, com a imagem `markupp-embedder` da
mesma tag de `image` (ou `embedderImage`, obrigatória com `image` fixada por digest), rodando
`embed-loop`. Os pesos do modelo local ficam num PersistentVolumeClaim de 2Gi em
`/var/lib/markupp/models`, com a `storageClass` do banco. Com `embedderModelVolume: false`, o
componente não cria o PersistentVolumeClaim e os pesos ficam num emptyDir: serve a quem não pode
criar volume no namespace ou usa só o provedor `openai-compat`, que não guarda pesos, e o provedor
local baixa o modelo a cada subida do pod. Sem `MARKUPP_EMBEDDING_PROVIDER`, vale a
escolha do administrador da instância no painel.

### Stack dev

O projeto em `deploy/stacks` usa o componente pelo caminho local e tem uma stack, `dev`: um
cluster kind que o CI sobe em cada PR e a cada push em dev para testar a implantação, com
certificado autoassinado.

A cada tag `v*`, o workflow de release publica as imagens do servidor, do painel e do MCP. Com
aprovação no environment `production` do GitHub, que guarda o secret `CLOUD_DISPATCH_TOKEN`, o
release também avisa a tag nova por `repository_dispatch`; sem o secret (forks e instalações
self-host), o passo é pulado.

A stack sobe sem email. Para ligar numa stack, com a URL do SMTP cifrada no arquivo da stack:

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

O ambiente de desenvolvimento vive em `compose.dev.yaml`. Nada precisa ser configurado antes:
`docker compose -f compose.dev.yaml up` também funciona. Como no self-host, tudo fica em
`http://localhost:6275` (painel na raiz, API em `/v1`, MCP em `/mcp`), e a caixa de email do
Mailpit em `http://127.0.0.1:8025`. Na primeira subida o servidor gera os próprios segredos e os guarda
no banco (ADR-0041). Os testes rodam no host e precisam de Go e de Docker:

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

Smoke test pela API. Com o banco vazio, o primeiro cadastro cria a conta como owner do tenant
inicial, e já devolve a sessão:

```sh
TOKEN=$(curl -s -X POST http://localhost:6275/v1/accounts \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@exemplo.dev","password":"troque-esta-senha"}' \
  | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
ID=$(curl -s -X POST http://localhost:6275/v1/notes \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"path":"validacao.md","content":"oi"}' \
  | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:6275/v1/notes/$ID
curl -s -X DELETE -H "Authorization: Bearer $TOKEN" http://localhost:6275/v1/notes/$ID -w '%{http_code}\n'
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

O login só entra em conta que já existe; a conta nasce no cadastro, em `POST /v1/accounts` ou
na tela "Criar conta" do painel. Depois da primeira, o cadastro só aceita conta nova com
`MARKUPP_AUTH_REGISTRATION_ENABLED=true`; desligado, ele responde 403 `registration_disabled` e
a conta nasce só pelo convite.

Com o auto-registro ligado, o administrador da instância abre e fecha o cadastro de cada método,
email e senha e cada provedor de login externo, em Configurações > Instância > Cadastro, sem reiniciar
([ADR-0059](adrs/ADR-0059-cadastro-por-metodo.md)). Método sem escolha está aberto. Fechar o
cadastro de um método não fecha o login de quem já tem conta por ele. Para fixar o valor e travar
o painel, defina `MARKUPP_AUTH_LOCAL_REGISTRATION` (email e senha) ou
`MARKUPP_LOGIN_<ID>_REGISTRATION` (um provedor) como `true` ou `false`. Com o auto-registro
desligado, todos os métodos ficam fechados e travados.

A conta de auto-registro (`MARKUPP_AUTH_REGISTRATION_ENABLED=true`) nasce sem email provado e
não recebe convite pela lista até verificar. Com SMTP configurado, ela recebe o email de
verificação no cadastro, confirma com o token logada nela em `POST /auth/email/verify` e pode
pedir outro em `POST /auth/email/verification`. Sem SMTP, nada
prova o email dela, e qualquer pessoa pode ocupar um email alheio antes do dono: só ligue o
auto-registro com SMTP. No componente, `registrationEnabled: true` liga o auto-registro, e ele é
recusado sem o bloco `email`.

Com SMTP, quem esqueceu a senha pede a troca em `POST /auth/password/forgot` e troca com o token
do email em `POST /auth/password/reset`.

## Login externo

Opcional e desligado por padrão ([ADR-0054](adrs/ADR-0054-login-por-provedor-externo.md)): a
instalação pode aceitar login por qualquer provedor OpenID Connect (Google, Keycloak, Authentik)
e pelo GitHub. Sem configuração, o painel não mostra botão nenhum.

O jeito mais simples é pelo painel, em Configurações > Instância > Login externo, que só aparece
para o administrador da instância ([ADR-0060](adrs/ADR-0060-login-externo-no-banco.md)). Ele
cadastra cada provedor com id, nome, issuer, client id e client secret, e a tela mostra o
callback a cadastrar no provedor. O segredo fica no banco cifrado como a chave do embedding, e
a API passa a usar a mudança em alguns segundos, sem reiniciar. A API expõe o mesmo em
`/v1/instance/login-providers`, e o servidor MCP em ferramentas do administrador. Apagar um
provedor apaga os vínculos das contas com ele.

Para fixar a lista no deploy, use as variáveis: com `MARKUPP_LOGIN_PROVIDERS` definida, vale só
ela, e a lista do painel aparece travada. `MARKUPP_LOGIN_PROVIDERS` lista os ids, separados por vírgula, com letras minúsculas, dígitos e
hífen. Cada id lê as próprias variáveis, com o id em maiúsculas e o hífen virando sublinhado:

- `MARKUPP_LOGIN_<ID>_CLIENT_ID` e `MARKUPP_LOGIN_<ID>_CLIENT_SECRET`: obrigatórias, do cadastro
  do app no provedor
- `MARKUPP_LOGIN_<ID>_ISSUER`: o issuer OIDC, exatamente como o provedor publica (Google:
  `https://accounts.google.com`). Sem ela, só o id `github` é aceito, e usa o GitHub
- `MARKUPP_LOGIN_<ID>_NAME`: o nome do botão. Padrão: o id

No provedor, cadastre como redirect URI o callback do painel, `MARKUPP_PANEL_URL/auth/<id>/callback`, ou
`MARKUPP_PUBLIC_URL/auth/<id>/callback` com o painel na raiz (ex.: `https://notas.exemplo.com/auth/google/callback`). No GitHub, crie um OAuth App; o login pede
os escopos read:user e user:email.

O login só entra em conta que já tem a identidade vinculada. A primeira vez com um provedor é
cadastro, pelo botão da tela "Criar conta", e segue o auto-registro como o cadastro por senha; a
conta nasce com o email que o provedor informa, já verificada quando o provedor o afirma
verificado. Quem já tem conta vincula o provedor logado, em Configurações > Conta, e o servidor
nunca vincula pelo email. Desvincular recusa a última forma de entrar que vale na instalação.

O docker-compose.yaml não repassa essas variáveis. Ponha-as num `compose.override.yaml` ao lado
dele, que o docker compose lê sozinho:

```yaml
services:
  markupp:
    environment:
      MARKUPP_LOGIN_PROVIDERS: google,github
      MARKUPP_LOGIN_GOOGLE_ISSUER: https://accounts.google.com
      MARKUPP_LOGIN_GOOGLE_NAME: Google
      MARKUPP_LOGIN_GOOGLE_CLIENT_ID: ${MARKUPP_LOGIN_GOOGLE_CLIENT_ID}
      MARKUPP_LOGIN_GOOGLE_CLIENT_SECRET: ${MARKUPP_LOGIN_GOOGLE_CLIENT_SECRET}
      MARKUPP_LOGIN_GITHUB_NAME: GitHub
      MARKUPP_LOGIN_GITHUB_CLIENT_ID: ${MARKUPP_LOGIN_GITHUB_CLIENT_ID}
      MARKUPP_LOGIN_GITHUB_CLIENT_SECRET: ${MARKUPP_LOGIN_GITHUB_CLIENT_SECRET}
```

Os segredos ficam no `.env`. Um provedor OIDC fora do ar não impede o servidor de subir: a
descoberta acontece no primeiro login, e o botão responde que o provedor não respondeu.

## Ligação por sentido

Opcional e desligada por padrão ([ADR-0053](adrs/ADR-0053-ligacoes-automaticas.md)): o
indexador de embedding transforma cada nota num vetor, e o servidor liga as notas que tratam do
mesmo assunto. Ele roda com `embed-loop` na imagem `ghcr.io/markupp-labs/markupp-embedder`, que
é o serviço `embedder` do `docker-compose.yaml`.

O provedor se escolhe de dois jeitos ([ADR-0057](adrs/ADR-0057-provedor-de-embedding-no-banco.md)):

- pelo painel, em Settings, grupo Instance, seção Embedding, que só aparece para o
  administrador da instância: a conta cujo email, já verificado, está em
  `MARKUPP_INSTANCE_ADMINS` (emails separados por vírgula, lida pela API). Sem a variável,
  ninguém administra pelo painel. A API expõe o mesmo em `GET` e `PUT /v1/instance/embedding`,
  e o servidor MCP nas ferramentas `get_embedding_provider` e `set_embedding_provider`. O
  embedder relê a escolha a cada `MARKUPP_INDEX_INTERVAL` e troca de provedor sem reiniciar;
- pela variável `MARKUPP_EMBEDDING_PROVIDER` do embedder. Definida, mesmo como `none`, ela vence
  a escolha do painel, que aparece travada. Vazia ou ausente, vale a escolha do painel, com
  `none` como padrão.

Os provedores são:

- `none`: sem ligação por sentido, e nada é baixado. O embedder fica ocioso e apaga os
  vetores de um provedor anterior;
- `local`: o modelo roda na própria máquina, no ONNX Runtime, em amd64 ou arm64. Ao ligar, ele
  baixa os arquivos do modelo para `MARKUPP_EMBEDDING_MODEL_DIR` (padrão
  `/var/lib/markupp/models`, um volume no compose), uns 300 MB em x86 com AVX2 e 1,2 GB nas
  outras CPUs, e recusa arquivo com hash diferente
  ([ADR-0056](adrs/ADR-0056-pedacos-e-media.md)). O painel mostra o estado do modelo: baixando,
  pronto ou erro, com a mensagem. Gravar a mesma escolha de novo tenta outra vez;
- `openai-compat`: pede os vetores a um endpoint `/embeddings` no formato da OpenAI, com a URL
  (a base, sem `/embeddings`), o modelo e, se o servidor pedir, a chave. Pelas variáveis, são
  `MARKUPP_EMBEDDING_URL`, `MARKUPP_EMBEDDING_MODEL` e `MARKUPP_EMBEDDING_API_KEY`. O texto das
  notas sai da máquina.

A chave escolhida pelo painel fica no banco, cifrada com AES-256-GCM, e a API nunca a devolve.
A chave de cifra é a `MARKUPP_SECRET_KEY` (ao menos 32 bytes, gere com
`openssl rand -hex 32`), que a API e o embedder precisam receber iguais. Sem ela, o servidor gera
uma no banco na primeira subida, e aí quem tem uma cópia do banco também decifra a chave.
Definir a `MARKUPP_SECRET_KEY` depois é tratado sozinho: a API sela de novo a chave guardada
com ela. Trocar o valor ou perder a `MARKUPP_SECRET_KEY` deixa a chave guardada ilegível, e ela precisa ser
gravada de novo pelo painel.

`MARKUPP_EMBEDDING_CHUNK_TOKENS` (padrão 512) é o tamanho do pedaço de nota que vira um vetor.
Trocar de provedor ou de modelo apaga os vetores e refaz os de todas as notas.

Os vetores ficam no Postgres, com o pgvector ([ADR-0055](adrs/ADR-0055-vetores-no-postgres.md)).
O compose e o componente Pulumi usam a imagem oficial do pgvector, `pgvector/pgvector`, com o
Postgres 17 em Debian. Num PostgreSQL externo, instale o pgvector e crie a extensão com um
superusuário (`CREATE EXTENSION vector;`) antes de migrar.

O banco criado antes, na imagem `postgres` em Alpine, ordena o texto de outro jeito que o
Debian; a migração da ligação por sentido refaz os índices das tabelas do markupp uma vez.

## OAuth

O authorization server OAuth 2.1 vem ligado, e nenhuma variável é obrigatória. A única que um
self-host normalmente define é a URL pública:

- `MARKUPP_PUBLIC_URL`: o issuer, como `https://notas.exemplo.com`, sem caminho, query nem
  fragmento. Padrão: `http://localhost` na porta do servidor
- `MARKUPP_PANEL_URL`: o painel que faz o login e o consentimento, como
  `https://painel.exemplo.com`, sem query nem fragmento. Padrão: `MARKUPP_PUBLIC_URL`, com o
  painel na raiz, encaminhado pelo Go
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
| `MARKUPP_PANEL_UPSTREAM_URL` | Go | sem painel na raiz | endereço interno do painel, para onde vai todo caminho que o Go não atende |
| `MARKUPP_MCP_SECRET_PATH` | Go e MCP | sem arquivo | arquivo do segredo do MCP: o Go escreve e o MCP lê |
| `MARKUPP_PANEL_RELAY_SECRET_PATH` | Go e painel | sem arquivo | arquivo do segredo de relay do painel: o Go escreve e o painel lê |
| `MARKUPP_PANEL_RELAY_SECRET` | Go e painel | o do banco no Go, o do arquivo no painel | segredo de relay do painel, com ao menos 32 bytes |
| `MARKUPP_MCP_API_URL` | MCP | `http://localhost:6275` | endereço do servidor Go visto pelo MCP; aceita http em qualquer host |
| `MARKUPP_MCP_RESOURCE_URL` | MCP | issuer + `/mcp` | URL pública do MCP com o caminho do endpoint; o caminho vira a rota |
| `MARKUPP_MCP_RESOURCE_SECRET` | MCP | o do arquivo | segredo do par em `MARKUPP_OAUTH_RESOURCES`, com ao menos 32 bytes |
| `MARKUPP_MCP_PORT` | MCP | 3000 | porta em que o MCP escuta, em todas as interfaces |

O servidor MCP sobe com `markupp-mcp http` (a imagem já começa assim) e espera até 60 segundos
pelo Go e pelo arquivo do segredo. No cluster, com o MCP em subdomínio próprio (ADR-0045), o
`MARKUPP_OAUTH_RESOURCES` do Go e o `MARKUPP_MCP_RESOURCE_URL` e o `MARKUPP_MCP_RESOURCE_SECRET`
do MCP vêm do Secret.

O mesmo servidor atende, sem OAuth, a rota do endpoint seguida de `/public` (`/mcp/public` por
padrão), que o Go também encaminha (ADR-0050). Ali o agente só lê as notas publicadas, pelo id
público, com as ferramentas `read_public_note`, `list_public_notes` e `search_public_notes`, que
chamam as rotas `/v1/public/notes/{public_id}` da API sem token. O limite de
`MARKUPP_PUBLIC_RATE_LIMIT_PER_MINUTE` vale também para elas, contado pelo IP de cada agente, sem
configuração: o MCP repassa ao Go o X-Forwarded-For que recebeu em `/mcp/public` junto com o HTTP
Basic do resource dele (a URL e o segredo que já usa na troca de token). Só com essa credencial o
Go trata o último salto do header, o que ele mesmo escreveu ao encaminhar, como a conexão do
agente, e segue dali a regra de `MARKUPP_TRUSTED_PROXIES`, como num pedido direto à API. Sem a
credencial, o header é ignorado. Não exponha a porta do MCP fora da rede interna: quem a alcança
sem passar pelo Go escolhe o IP que conta.

## Aviso

Com o banco vazio, quem criar a primeira conta, pelo painel em "Criar conta" ou em
`POST /v1/accounts`, vira owner do tenant inicial
([ADR-0035](adrs/ADR-0035-conta-global-com-vinculo-por-tenant.md)). Isso vale mesmo com o
auto-registro desligado. Crie essa conta antes de expor o servidor publicamente.
