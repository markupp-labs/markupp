"""Mede tamanho, tempo de construção, latência e recall do HNSW no pgvector."""

import json
import sys
import time

import numpy as np
import psycopg
from pgvector.psycopg import register_vector

DIM = 1024
URI = "postgresql://postgres:bench@localhost:55432/postgres"
RNG = np.random.default_rng(42)
CENTROIDS = RNG.normal(size=(500, DIM)).astype(np.float32)


def clustered(n: int) -> np.ndarray:
    """Vetores agrupados por tema, como embeddings reais de um acervo."""
    picks = RNG.integers(0, len(CENTROIDS), n)
    noise = RNG.normal(scale=0.6, size=(n, DIM)).astype(np.float32)
    v = CENTROIDS[picks] + noise
    return v / np.linalg.norm(v, axis=1, keepdims=True)


def load(conn: psycopg.Connection, table: str, vecs: np.ndarray) -> None:
    conn.execute(f"drop table if exists {table}")
    conn.execute(f"create table {table} (id bigint primary key, v vector({DIM}))")
    with conn.cursor().copy(f"copy {table} (id, v) from stdin with (format binary)") as cp:
        cp.set_types(["int8", "vector"])
        for i, v in enumerate(vecs):
            cp.write_row((i, v))
    conn.execute(f"vacuum analyze {table}")


def build(conn: psycopg.Connection, table: str, kind: str) -> float:
    expr = "v vector_cosine_ops" if kind == "float32" else f"(v::halfvec({DIM})) halfvec_cosine_ops"
    start = time.perf_counter()
    conn.execute(f"create index {table}_{kind} on {table} using hnsw ({expr})")
    return time.perf_counter() - start


def query_sql(table: str, kind: str) -> str:
    if kind == "float32":
        return f"select id from {table} order by v <=> %s limit 10"
    return f"select id from {table} order by v::halfvec({DIM}) <=> %s::halfvec({DIM}) limit 10"


def latency_and_recall(conn: psycopg.Connection, table: str, kind: str, queries: np.ndarray) -> dict:
    sql = query_sql(table, kind)
    conn.execute("set hnsw.ef_search = 40")
    times, hits = [], 0
    for q in queries:
        t = time.perf_counter()
        got = {r[0] for r in conn.execute(sql, (q,)).fetchall()}
        times.append((time.perf_counter() - t) * 1000)
        if len(times) <= 50:
            hits += len(got & exact_top10(conn, table, q))
    return {
        "p50_ms": round(float(np.percentile(times, 50)), 2),
        "p95_ms": round(float(np.percentile(times, 95)), 2),
        "recall_at_10": round(hits / 500, 3),
    }


def exact_top10(conn: psycopg.Connection, table: str, q: np.ndarray) -> set:
    with conn.transaction():
        conn.execute("set local enable_indexscan = off")
        rows = conn.execute(f"select id from {table} order by v <=> %s limit 10", (q,)).fetchall()
    return {r[0] for r in rows}


def sizes(conn: psycopg.Connection, table: str, index: str) -> dict:
    row = conn.execute("select pg_table_size(%s), pg_relation_size(%s)", (table, index)).fetchone()
    return {"tabela_mb": round(row[0] / 2**20, 1), "indice_mb": round(row[1] / 2**20, 1)}


def run(n: int, kinds: list[str]) -> dict:
    out = {}
    with psycopg.connect(URI, autocommit=True) as conn:
        register_vector(conn)
        table = f"items_{n}"
        load(conn, table, clustered(n))
        queries = clustered(500)
        for kind in kinds:
            secs = build(conn, table, kind)
            res = {"build_s": round(secs, 1), **sizes(conn, table, f"{table}_{kind}")}
            res.update(latency_and_recall(conn, table, kind, queries))
            res["bytes_por_vetor_total"] = round((res["tabela_mb"] + res["indice_mb"]) * 2**20 / n)
            out[kind] = res
            conn.execute(f"drop index {table}_{kind}")
        conn.execute(f"drop table {table}")
    return out


def main() -> None:
    n = int(sys.argv[1])
    kinds = sys.argv[2].split(",")
    print(json.dumps({"vetores": n, **run(n, kinds)}, indent=1))


if __name__ == "__main__":
    main()
