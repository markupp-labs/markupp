"""Converte as medições em custo mensal de pior caso por plano, com o teto de notas cheio."""

import json
import pathlib
import sys

TITAN_USD_PER_M_TOKENS = 0.02  # aws.amazon.com, Titan Text Embeddings v2
RAM_USD_PER_GB_MONTH = 6.0  # DigitalOcean Basic 16 GiB a US$ 96 (CPU e disco embutidos)
DISK_USD_PER_GB_MONTH = 0.08  # AWS EBS gp3
HOT_FRACTION_NON_VECTOR = 0.20  # parte do banco léxico que fica em cache
REINDEX_PER_MONTH = 0.5  # 10% das notas editadas x 5 reindexações
PADDLE_PCT, PADDLE_FIXED = 0.05, 0.50

PLANS = {
    "Free": {"notas": 1_000, "mensal": 0.0, "anual": 0.0},
    "Pro": {"notas": 20_000, "mensal": 10.0, "anual": 96.0},
    "Max": {"notas": 100_000, "mensal": 49.0, "anual": 468.0},
}


def per_note(results: pathlib.Path) -> dict:
    ingest = json.loads((results / "ingest.json").read_text())
    bench = json.loads((results / "vec_100k.json").read_text())
    vec = bench["halfvec"]
    tokens = json.loads((results / "tokens.json").read_text())
    chunks = max(t["chunks_por_nota"] for t in tokens.values())
    emb_tokens = max(t["tokens_embedados_por_nota"] for t in tokens.values())
    vec_index_b = vec["indice_mb"] * 2**20 / bench["vetores"]
    vec_total_b = vec["bytes_por_vetor_total"]
    lexical_b = ingest["bytes_banco_por_nota"]
    return {
        "disco_b": lexical_b + chunks * vec_total_b,
        "ram_b": chunks * vec_index_b
        + HOT_FRACTION_NON_VECTOR * (lexical_b + chunks * (vec_total_b - vec_index_b)),
        "tokens": emb_tokens,
    }


def monthly_cost(notes: int, unit: dict) -> dict:
    gb = 2**30
    ram = notes * unit["ram_b"] / gb * RAM_USD_PER_GB_MONTH
    disk = notes * unit["disco_b"] / gb * DISK_USD_PER_GB_MONTH
    emb = notes * REINDEX_PER_MONTH * unit["tokens"] / 1e6 * TITAN_USD_PER_M_TOKENS
    first_import = notes * unit["tokens"] / 1e6 * TITAN_USD_PER_M_TOKENS
    return {
        "ram": round(ram, 2),
        "disco": round(disk, 2),
        "embedding": round(emb, 2),
        "total": round(ram + disk + emb, 2),
        "importacao_unica": round(first_import, 2),
    }


def net_revenue(plan: dict) -> dict:
    monthly = plan["mensal"] - (plan["mensal"] * PADDLE_PCT + PADDLE_FIXED) if plan["mensal"] else 0.0
    annual = (plan["anual"] - (plan["anual"] * PADDLE_PCT + PADDLE_FIXED)) / 12 if plan["anual"] else 0.0
    return {"liquido_mensal": round(monthly, 2), "liquido_anual_por_mes": round(annual, 2)}


def main() -> None:
    unit = per_note(pathlib.Path(sys.argv[1]))
    out = {
        "por_nota": {
            "disco_kb": round(unit["disco_b"] / 1024, 1),
            "ram_kb": round(unit["ram_b"] / 1024, 1),
            "tokens_embedados": unit["tokens"],
        }
    }
    for name, plan in PLANS.items():
        cost = monthly_cost(plan["notas"], unit)
        rev = net_revenue(plan)
        margin = (
            (rev["liquido_anual_por_mes"] - cost["total"]) / rev["liquido_anual_por_mes"]
            if plan["anual"]
            else None
        )
        out[name] = {
            "teto_notas": plan["notas"],
            "custo_pior_caso": cost,
            **rev,
            "margem_pior_caso_anual": None if margin is None else f"{margin:.0%}",
        }
    print(json.dumps(out, indent=1, ensure_ascii=False))


if __name__ == "__main__":
    main()
