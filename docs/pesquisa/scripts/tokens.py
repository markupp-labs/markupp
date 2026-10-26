"""Mede palavras, tokens e chunks por nota markdown real."""

import json
import pathlib
import statistics
import sys

import tiktoken

ENC = tiktoken.get_encoding("cl100k_base")
CHUNK, OVERLAP = 512, 77  # 15% de sobreposição


def chunks_for(tokens: int) -> int:
    if tokens <= CHUNK:
        return 1
    step = CHUNK - OVERLAP
    return 1 + -(-(tokens - CHUNK) // step)


def measure(paths: list[pathlib.Path]) -> dict:
    words, toks, chs, embedded = [], [], [], []
    for p in paths:
        text = p.read_text(encoding="utf-8", errors="ignore")
        t = len(ENC.encode(text))
        c = chunks_for(t)
        words.append(len(text.split()))
        toks.append(t)
        chs.append(c)
        embedded.append(t + (c - 1) * OVERLAP)
    q = statistics.quantiles(toks, n=20)
    return {
        "notas": len(paths),
        "palavras_media": round(statistics.mean(words)),
        "tokens_media": round(statistics.mean(toks)),
        "tokens_mediana": statistics.median(toks),
        "tokens_p95": round(q[18]),
        "chunks_por_nota": round(statistics.mean(chs), 2),
        "tokens_embedados_por_nota": round(statistics.mean(embedded)),
        "tokens_por_palavra": round(sum(toks) / max(sum(words), 1), 2),
    }


def main() -> None:
    root = pathlib.Path(sys.argv[1])
    sets = {
        "obsidian_en": list((root / "corpus/obsidian-help/en").rglob("*.md")),
        "obsidian_pt-br": list((root / "corpus/obsidian-help/pt-br").rglob("*.md")),
        "markupp_docs_pt": list(pathlib.Path(sys.argv[2]).rglob("*.md")),
    }
    result = {k: measure(v) for k, v in sets.items() if v}
    print(json.dumps(result, indent=1, ensure_ascii=False))


if __name__ == "__main__":
    main()
