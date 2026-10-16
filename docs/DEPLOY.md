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
- o ScaledJob do indexador, que o KEDA usa para criar um Job por cofre com nota pendente, e o
  TriggerAuthentication que entrega ao KEDA a URL de um papel do banco que só conta os cofres
  pendentes;
- um Gateway com HTTPRoute, e um Issuer do cert-manager para o certificado.

### Pré-requisitos do cluster

- Gateway API, com um controlador e uma GatewayClass (padrão `cilium`)
- cert-manager
- KEDA, para o indexador. Sem ele, informe `indexer: {enabled: false}`, e banco e API sobem sem
  indexação. Com banco externo, o host da URL precisa resolver também do namespace do KEDA, e o
  usuário dela precisa de CREATEROLE para a migração dar login ao papel do scaler
- o namespace do markupp, já criado
- uma credencial com permissão no namespace para Secret, Job, Deployment, StatefulSet, Service,
  Gateway, HTTPRoute, Issuer, ScaledJob e TriggerAuthentication
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

O compose sobe PostgreSQL, aplica as migrações e roda o servidor com recarga automática:

```sh
make run
```

Servidor em `http://localhost:8080`. Os testes rodam no host e precisam de Go e de Docker:

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
`X-Markupp-Tenant`, usando um id de `GET /me/tenants`. Qualquer conta cria tenant em
`POST /tenants` e vira owner dele.

Sem provedor de email, nada prova o email de quem se cadastra sozinho. Por isso a conta de
auto-registro (`MARKUPP_AUTH_REGISTRATION_ENABLED=true`) não recebe convite pela lista, e
qualquer pessoa pode ocupar um email alheio antes do dono. Deixe o auto-registro desligado
enquanto a verificação de email não existir.

## Aviso

Com o banco vazio, quem fizer o primeiro login vira owner do tenant inicial
([ADR-0035](adrs/ADR-0035-conta-global-com-vinculo-por-tenant.md)).
Faça esse login antes de expor o servidor publicamente.
