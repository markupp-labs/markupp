# Servidor Markupp

Servidor REST do Markupp em Go.

## Build

```sh
go build ./cmd/markupp
```

## Execução

```sh
./markupp
```

## Configuração

O servidor lê a configuração de variáveis de ambiente.

| Variável | Default | O que é |
| --- | --- | --- |
| `MARKUPP_DATABASE_URL` | nenhum, obrigatória | Conexão PostgreSQL, no formato `postgres://usuario:senha@host:5432/banco` |
| `MARKUPP_PORT` | `8080` | Porta HTTP |
| `MARKUPP_MAX_NOTE_SIZE` | `52428800` | Tamanho máximo de uma nota, em bytes |
| `MARKUPP_ALLOWED_ORIGINS` | vazio | Origens liberadas no CORS, separadas por vírgula. Vazio desliga o CORS |

Os testes sobem um PostgreSQL descartável com testcontainers, então `go test ./...` precisa de
Docker disponível.
