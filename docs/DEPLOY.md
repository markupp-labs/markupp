# Deploy

Este documento cobre o deploy do **servidor** Markupp, escrito em Go. O servidor expõe a API
REST que os clientes consomem e guarda as notas num PostgreSQL. A configuração vem de variáveis
de ambiente, descritas no [README do servidor](../markupp/README.md#configuração).

## Pré-requisitos

- Docker 24+ com o plugin compose
- Go 1.26, para rodar os testes

## Subir o servidor

O compose sobe o PostgreSQL, aplica as migrações com `markupp migrate` e só então sobe o
servidor:

```sh
make run
```

Servidor disponível em `http://localhost:8080`. Para parar: `docker compose down`, e para
apagar também o banco: `docker compose down -v`.

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

Os testes rodam no host e sobem um PostgreSQL descartável com testcontainers:

```sh
make test
```

## Build a partir do código fonte

Com `MARKUPP_DATABASE_URL` apontando para um PostgreSQL:

```sh
cd markupp
go build -o markupp ./cmd/markupp
./markupp migrate
./markupp
```

A imagem Docker é gerada por `markupp/Dockerfile` (multi-stage, alpine), e o workflow
`.github/workflows/release.yml` publica a imagem a cada tag `v*`.

## Aviso

O servidor não tem autenticação ([ADR-0005](adrs/ADR-0005-seguranca-fora-do-mvp.md)). **Não
exponha fora de `localhost`**: qualquer um com acesso à porta consegue ler, criar e apagar
notas.
