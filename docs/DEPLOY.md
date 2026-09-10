# Deploy

O Markupp tem dois artefatos: o **servidor** em Go e o **plugin Obsidian**. O servidor expõe a API REST e o plugin é o cliente que envia notas.

## Pré-requisitos

- Docker 24+ para rodar o servidor.
- Obsidian 1.5+ para o plugin.
- (Opcional, só pra buildar do código) Go 1.26 e Node 20+.

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

## Instalar o plugin no Obsidian

1. Baixe `main.js`, `manifest.json` e `styles.css` da [release mais recente](https://github.com/IFSC-ES2/projeto-markupp/releases).
2. Copie os três arquivos para `<seu-vault>/.obsidian/plugins/obsidian-markupp-plugin/`.
3. Em **Settings → Community Plugins**, habilite "Markupp Plugin".
4. Em **Settings → Markupp Plugin**, configure a URL do servidor (default `http://localhost:8080`).

## Validar

Smoke test pela API (sem precisar do Obsidian):

```sh
ID=$(curl -s -X POST http://localhost:8080/notes \
  -H 'Content-Type: application/json' \
  -d '{"path":"validacao.md","content":"oi"}' \
  | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
curl -s http://localhost:8080/notes/$ID
curl -s -X DELETE http://localhost:8080/notes/$ID -w '%{http_code}\n'
```

Esperado: `POST` retorna JSON com `id` UUID, `GET` traz a nota, `DELETE` responde `204`. Rotas completas em `markupp/openapi.yaml`.

Para validar pelo [plugin](https://github.com/markupp-labs/obsidian-markupp-plugin): abra a Source Control View (ícone na barra lateral ou o comando "Abrir source control"), edite/crie uma nota no vault e rode Push (ou Sync). A nota deve aparecer no servidor (confirme com o GET /notes).

## Build a partir do código fonte

Útil pra contribuir ou reproduzir a imagem localmente.

```sh
# servidor
cd markupp
go build -o markupp ./cmd/markupp
./markupp
```

A imagem Docker é gerada por `markupp/Dockerfile` (multi-stage, alpine, ~20MB). O workflow `.github/workflows/release.yml` publica `riedelgab/ifsces2` em cada tag `v*`.

## Aviso

O servidor não tem autenticação ([ADR-0005](adrs/ADR-0005-seguranca-fora-do-mvp.md)). **Não exponha fora de `localhost`** — qualquer um com acesso à porta consegue ler, criar e apagar notas.
