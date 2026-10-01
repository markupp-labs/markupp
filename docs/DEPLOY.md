# Deploy

Este documento cobre o deploy do **servidor** Markupp, escrito em Go. O servidor expõe a API
REST que os clientes consomem. A imagem é publicada em `ghcr.io/markupp-labs/markupp` a cada
tag `v*`, e a configuração vem de variáveis de ambiente, descritas no
[README do servidor](../markupp/README.md#configuração).

## Kubernetes

A instalação em Kubernetes é o componente Pulumi em `deploy/pulumi` (ADR-0025). Ele cria, no
namespace informado:

- o banco: um cluster CloudNativePG, ou um Secret com a URL de um PostgreSQL externo;
- um Job de migração que roda antes da API a cada versão;
- o Deployment e o Service da API;
- um Gateway com HTTPRoute, e um Issuer do cert-manager para o certificado.

### Pré-requisitos do cluster

- Gateway API, com um controlador e uma GatewayClass (padrão `cilium`)
- cert-manager
- o namespace do markupp, já criado
- uma credencial com permissão no namespace para Secret, Job, Deployment, Service, Gateway,
  HTTPRoute, Issuer e, com CloudNativePG, Cluster
- opcional: o operador CloudNativePG e uma StorageClass para os volumes dele. Sem ele, informe
  `database.externalUrl`
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
        storageClass: longhorn-single
```

### Stacks da equipe

O projeto em `deploy/stacks` usa o componente pelo caminho local e tem duas stacks:

- `prod`: o Enterprise no cluster do IFSC, com Let's Encrypt
- `dev`: um cluster kind que o CI sobe em cada PR e a cada push em dev para testar a implantação, com
  certificado autoassinado

Para rodar a stack dev na própria máquina, com kind, helm e Pulumi instalados:

```sh
./deploy/kind/prepara-cluster.sh
docker build -t markupp:dev markupp
kind load docker-image markupp:dev --name markupp-dev
cd deploy/stacks
pulumi login --local
pulumi stack select dev --create
pulumi up --config image=markupp:dev
curl -k --resolve markupp.local:8443:127.0.0.1 https://markupp.local:8443/healthz
```

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

Smoke test pela API:

```sh
ID=$(curl -s -X POST http://localhost:8080/notes \
  -H 'Content-Type: application/json' \
  -d '{"path":"validacao.md","content":"oi"}' \
  | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
curl -s http://localhost:8080/notes/$ID
curl -s -X DELETE http://localhost:8080/notes/$ID -w '%{http_code}\n'
```

Esperado: `POST` retorna JSON com `id` UUID, `GET` traz a nota, `DELETE` responde `204`. Rotas
completas em `markupp/openapi.yaml`.

## Aviso

O servidor não tem autenticação ([ADR-0005](adrs/ADR-0005-seguranca-fora-do-mvp.md)). Não
exponha publicamente antes de ela existir: qualquer um com acesso à porta consegue ler, criar e
apagar notas.
