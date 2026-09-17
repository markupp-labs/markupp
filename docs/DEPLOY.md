# Deploy

Este documento cobre o deploy do **servidor** Markupp, escrito em Go. O servidor expõe a API REST que os clientes consomem.

## Pré-requisitos

- Docker 24+ para rodar o servidor.
- (Opcional, só pra buildar do código) Go 1.26.

## Subir o servidor

Com Docker instalado:

```sh
docker run -d --name markupp -p 8080:8080 -v markupp_data:/data riedelgab/ifsces2:latest
```

- Servidor disponível em `http://localhost:8080`.
- Banco SQLite persistido no volume `markupp_data`.
- Para parar e remover: `docker stop markupp && docker rm markupp`.

### Configuração (opcional)

Os defaults atendem a maioria dos casos. Para mudar porta ou tamanho máximo de nota, monte um `config.json` no container:

```sh
docker run -d --name markupp \
  -p 9090:9090 \
  -v markupp_data:/data \
  -v "$(pwd)/config.json:/data/config.json" \
  riedelgab/ifsces2:latest
```

| Chave | Default | O que é |
| --- | --- | --- |
| `port` | `8080` | Porta HTTP |
| `db_path` | `./markupp.db` | Arquivo SQLite |
| `max_note_size` | `52428800` | Tamanho máximo de uma nota (bytes) |

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

Esperado: `POST` retorna JSON com `id` UUID, `GET` traz a nota, `DELETE` responde `204`. Rotas completas em `markupp/openapi.yaml`.

## Build a partir do código fonte

Útil pra contribuir ou reproduzir a imagem localmente.

```sh
cd markupp
go build -o markupp ./cmd/markupp
./markupp
```

A imagem Docker é gerada por `markupp/Dockerfile` (multi-stage, alpine, ~20MB). O workflow `.github/workflows/release.yml` publica `riedelgab/ifsces2` em cada tag `v*`.

## Aviso

O servidor não tem autenticação ([ADR-0005](adrs/ADR-0005-seguranca-fora-do-mvp.md)). **Não exponha fora de `localhost`** — qualquer um com acesso à porta consegue ler, criar e apagar notas.
