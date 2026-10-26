"""Mede a latência da busca na API real do markupp."""

import json
import statistics
import time
import urllib.parse
import urllib.request

API = "http://localhost:18080/v1"
TERMS = [
    "sync",
    "plugin",
    "graph view",
    "canvas",
    "links",
    "backlinks",
    "templates",
    "daily notes",
    "publish",
    "vault",
    "markdown",
    "embed",
    "search",
    "tags",
    "properties",
    "hotkeys",
    "themes",
    "community plugins",
    "callouts",
    "aliases",
]


def login() -> str:
    body = json.dumps({"email": "bench@markupp.local", "password": "bench-senha-123"}).encode()
    req = urllib.request.Request(API + "/auth/login", data=body, headers={"Content-Type": "application/json"})
    return json.loads(urllib.request.urlopen(req).read())["access_token"]


def timed_search(token: str, term: str) -> float:
    url = f"{API}/search/notes?q={urllib.parse.quote(term)}"
    req = urllib.request.Request(url, headers={"Authorization": f"Bearer {token}"})
    t = time.perf_counter()
    urllib.request.urlopen(req).read()
    return (time.perf_counter() - t) * 1000


def main() -> None:
    token = login()
    times = [timed_search(token, term) for _ in range(10) for term in TERMS]
    q = statistics.quantiles(times, n=20)
    print(
        json.dumps(
            {"buscas": len(times), "p50_ms": round(statistics.median(times), 1), "p95_ms": round(q[18], 1)},
            indent=1,
        )
    )


if __name__ == "__main__":
    main()
