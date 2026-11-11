#!/usr/bin/env bash
# Confere no release.yml que o token OIDC do npm só existe num job que publica um
# tarball pronto, sem instalar dependências nem rodar código de terceiros, e que o
# npm pack cria a pasta de destino antes de gravar o tarball.
set -uo pipefail

WORKFLOW="$(cd "$(dirname "$0")/.." && pwd)/.github/workflows/release.yml"

python3 - "$WORKFLOW" <<'PYTHON'
import re
import sys

import yaml

# Comandos que rodam código de dependências ou scripts de ciclo de vida do pacote.
THIRD_PARTY_CODE = re.compile(r"\bnpm (ci|install|i|pack|version|test|run)\b|\bnpx\b|\bvitest\b")


def jobs_with_id_token(workflow):
    return {
        name: job
        for name, job in workflow["jobs"].items()
        if (job.get("permissions") or {}).get("id-token") == "write"
    }


def commands(job):
    return [step["run"] for step in job.get("steps", []) if "run" in step]


def job_failures(name, job):
    failures = []
    for command in commands(job):
        if THIRD_PARTY_CODE.search(command):
            failures.append(f"job {name} has id-token and runs {command.strip()!r}, expected it to only download and publish the tarball")
    if not any(re.search(r"npm publish\b.*\.tgz", c, re.S) for c in commands(job)):
        failures.append(f"job {name} has id-token and does not publish a .tgz, expected npm publish <file>.tgz")
    return failures


def pack_destination_failures(workflow):
    """npm pack --pack-destination não cria a pasta: ela vem de um mkdir -p no mesmo passo."""
    failures = []
    for name, job in workflow["jobs"].items():
        for command in commands(job):
            match = re.search(r"--pack-destination\s+(\S+)", command)
            if match and f"mkdir -p {match.group(1)}" not in command:
                failures.append(f"job {name} packs into {match.group(1)} without mkdir -p {match.group(1)} first, expected the folder created in the same step")
    return failures


workflow = yaml.safe_load(open(sys.argv[1], encoding="utf-8"))
jobs = jobs_with_id_token(workflow)
failures = [f for name, job in jobs.items() for f in job_failures(name, job)]
failures += pack_destination_failures(workflow)
if not jobs:
    failures.append(f"no job with id-token in {sys.argv[1]}, expected the job that publishes to npm")
for failure in failures:
    print(f"FAIL  {failure}")
if failures:
    sys.exit(1)
print("ok    the npm id-token stays in a job that only publishes the tarball")
print("ok    npm pack creates its destination folder")
PYTHON
