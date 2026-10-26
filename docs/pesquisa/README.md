# Medições da Pesquisa de Cobrança

Scripts e resultados que sustentam os números de [pesquisa-de-cobranca.md](../pesquisa-de-cobranca.md).
Os resultados em `resultados/` são os da medição de 25/10/2026.

## O que cada script mede

| Script | Mede | Resultado |
|---|---|---|
| `tokens.py` | tokens e trechos por nota em markdown real | `tokens.json` |
| `vectors.py` | tamanho, tempo de construção, latência e acerto do índice de vetores | `vec_10k.json`, `vec_100k.json` |
| `ingest.py` | espaço no banco e tempo de indexação no markupp real | `ingest.json` |
| `search_latency.py` | latência da busca na API real | `search.json` |
| `cost_model.py` | custo mensal no pior caso e margem de cada plano | `cost_model.json` |

## Como reproduzir

Requisitos: Docker e [uv](https://docs.astral.sh/uv/). Os comandos rodam na raiz do
repositório.

Ambiente Python e notas de teste:

```bash
uv venv /tmp/bench-venv
uv pip install -p /tmp/bench-venv/bin/python tiktoken numpy "psycopg[binary]" pgvector
git clone --depth 1 https://github.com/obsidianmd/obsidian-help.git /tmp/corpus/obsidian-help
```

Tokens por nota. O primeiro argumento é a pasta que contém `corpus/`:

```bash
/tmp/bench-venv/bin/python docs/pesquisa/scripts/tokens.py /tmp docs
```

Índice de vetores, com PostgreSQL 17 e pgvector 0.8.1:

```bash
docker run -d --name pgvbench -e POSTGRES_PASSWORD=bench -p 55432:5432 --shm-size=1g pgvector/pgvector:0.8.1-pg17
docker exec pgvbench psql -U postgres -c "alter system set maintenance_work_mem='1536MB'"
docker exec pgvbench psql -U postgres -c "alter system set shared_buffers='1GB'"
docker restart pgvbench
docker exec pgvbench psql -U postgres -c "create extension vector"
/tmp/bench-venv/bin/python docs/pesquisa/scripts/vectors.py 100000 float32,halfvec
```

Markupp real, num projeto Docker separado. O primeiro login cria a conta usada pelos scripts:

```bash
MARKUPP_PORT=18080 docker compose -p markuppbench up -d --build markupp indexer
/tmp/bench-venv/bin/python docs/pesquisa/scripts/ingest.py /tmp/corpus/obsidian-help
/tmp/bench-venv/bin/python docs/pesquisa/scripts/search_latency.py
```

Custo por plano, a partir dos resultados:

```bash
/tmp/bench-venv/bin/python docs/pesquisa/scripts/cost_model.py docs/pesquisa/resultados
```

Para remover o ambiente de medição:

```bash
docker compose -p markuppbench down -v
docker rm -f pgvbench
```

## Observações

- A contagem de tokens usa o divisor `cl100k_base`, porque o do Amazon Titan não é público.
  A diferença entre divisores é pequena para estimar custo.
- Os vetores são artificiais, agrupados por tema para imitar textos reais, já que o markupp
  ainda não gera vetores.
- Os números variam com a máquina. A medição original rodou em WSL2 com 12 núcleos e 5 GB de
  memória.
