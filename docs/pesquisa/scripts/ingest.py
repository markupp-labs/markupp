"""Importa notas markdown reais no markupp pela API e mede o custo por nota no banco."""

import json
import pathlib
import subprocess
import sys
import time
import urllib.error
import urllib.request

API = "http://localhost:18080/v1"
PSQL = ["docker", "exec", "markuppbench-postgres-1", "psql", "-U", "markupp", "-d", "markupp", "-tAc"]


def call(method: str, path: str, body: dict | None, token: str | None = None) -> dict:
    req = urllib.request.Request(API + path, method=method, data=json.dumps(body).encode() if body else None)
    req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", f"Bearer {token}")
    with urllib.request.urlopen(req, timeout=30) as resp:
        return json.loads(resp.read() or b"{}")


def sql(query: str) -> str:
    return subprocess.run(PSQL + [query], capture_output=True, text=True, check=True).stdout.strip()


def import_notes(token: str, files: list[pathlib.Path], root: pathlib.Path) -> int:
    created = 0
    for f in files:
        path = str(f.relative_to(root)).replace(" ", "-")
        content = f.read_text(encoding="utf-8", errors="ignore")
        try:
            call("POST", "/notes", {"path": path, "content": content}, token)
            created += 1
        except urllib.error.HTTPError as err:
            print(json.dumps({"falha": path, "status": err.code}), file=sys.stderr)
    return created


def wait_index(timeout_s: int) -> float:
    start = time.perf_counter()
    while time.perf_counter() - start < timeout_s:
        pending = (
            "select (select count(*) from note_index_queue) + (select count(*) from vault_second_stage_queue)"
        )
        if sql(pending) == "0":
            return time.perf_counter() - start
        time.sleep(2)
    return -1.0


def table_sizes() -> dict:
    rows = sql(
        "select relname, pg_total_relation_size(c.oid) from pg_class c join pg_namespace n "
        "on n.oid = c.relnamespace where n.nspname = 'public' and c.relkind = 'r' "
        "order by 2 desc limit 15"
    )
    return {r.split("|")[0]: int(r.split("|")[1]) for r in rows.splitlines()}


def main() -> None:
    root = pathlib.Path(sys.argv[1])
    files = sorted(root.rglob("*.md"))
    token = call("POST", "/auth/login", {"email": "bench@markupp.local", "password": "bench-senha-123"})[
        "access_token"
    ]
    before = table_sizes()
    t = time.perf_counter()
    created = import_notes(token, files, root)
    write_s = time.perf_counter() - t
    index_s = wait_index(900)
    after = table_sizes()
    delta = {k: after[k] - before.get(k, 0) for k in after}
    total = sum(delta.values())
    raw = sum(len(f.read_bytes()) for f in files)
    print(
        json.dumps(
            {
                "notas": created,
                "markdown_bruto_kb": raw // 1024,
                "escrita_s": round(write_s, 1),
                "notas_por_s": round(created / write_s, 1),
                "indexacao_s": round(index_s, 1),
                "banco_total_kb": total // 1024,
                "bytes_banco_por_nota": total // max(created, 1),
                "por_tabela_kb": {k: v // 1024 for k, v in delta.items() if v > 0},
            },
            indent=1,
        )
    )


if __name__ == "__main__":
    main()
