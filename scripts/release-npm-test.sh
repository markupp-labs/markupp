#!/usr/bin/env bash
# Confere no release.yml que o token OIDC do npm só existe num job que publica um
# tarball pronto, sem instalar dependências nem rodar código de terceiros.
set -uo pipefail

WORKFLOW="$(cd "$(dirname "$0")/.." && pwd)/.github/workflows/release.yml"

python3 - "$WORKFLOW" <<'PYTHON'
import re
import sys

import yaml

# Comandos que rodam código de dependências ou scripts de ciclo de vida do pacote.
CODIGO_DE_TERCEIROS = re.compile(r"\bnpm (ci|install|i|pack|version|test|run)\b|\bnpx\b|\bvitest\b")


def jobs_com_id_token(workflow):
    return {
        nome: job
        for nome, job in workflow["jobs"].items()
        if (job.get("permissions") or {}).get("id-token") == "write"
    }


def comandos(job):
    return [passo["run"] for passo in job.get("steps", []) if "run" in passo]


def falhas_do_job(nome, job):
    falhas = []
    for comando in comandos(job):
        if CODIGO_DE_TERCEIROS.search(comando):
            falhas.append(f"job {nome} tem id-token e roda {comando.strip()!r}, esperado só baixar e publicar o tarball")
    if not any(re.search(r"npm publish\b.*\.tgz", c, re.S) for c in comandos(job)):
        falhas.append(f"job {nome} tem id-token e não publica um .tgz, esperado npm publish <arquivo>.tgz")
    return falhas


workflow = yaml.safe_load(open(sys.argv[1], encoding="utf-8"))
jobs = jobs_com_id_token(workflow)
falhas = [f for nome, job in jobs.items() for f in falhas_do_job(nome, job)]
if not jobs:
    falhas.append(f"nenhum job com id-token em {sys.argv[1]}, esperado o job que publica no npm")
for falha in falhas:
    print(f"FALHA {falha}")
if falhas:
    sys.exit(1)
print("ok    o id-token do npm fica num job que só publica o tarball")
PYTHON
