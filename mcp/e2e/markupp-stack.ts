import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { setTimeout as sleep } from "node:timers/promises";

// A stack do e2e roda num projeto do compose só dela, para o down -v do fim não apagar o
// banco da stack de desenvolvimento, e sem .env, como numa primeira subida (ADR-0041).
const COMPOSE_PROJECT = "markupp-e2e";
const COMPOSE_FILE = fileURLToPath(
  new URL("../../docker-compose.yaml", import.meta.url),
);
const STARTUP_TIMEOUT_MS = 10 * 60_000;
const POLL_INTERVAL_MS = 2_000;

// markuppUrl é a URL pública do Go, que o compose deriva de MARKUPP_PORT.
export function markuppUrl(): string {
  return `http://localhost:${process.env.MARKUPP_PORT ?? "8080"}`;
}

function compose(args: string[]): void {
  const fullArgs = ["compose", "-p", COMPOSE_PROJECT, "-f", COMPOSE_FILE];
  fullArgs.push("--env-file", "/dev/null", ...args);
  const result = spawnSync("docker", fullArgs, { stdio: "inherit" });
  if (result.status !== 0) {
    throw new Error(
      `docker ${fullArgs.join(" ")} saiu com ${result.status ?? result.signal}, esperado 0`,
    );
  }
}

// waitUntilOk consulta url até ela responder 200; o Go compila na subida e demora.
async function waitUntilOk(url: string): Promise<void> {
  const deadline = Date.now() + STARTUP_TIMEOUT_MS;
  let last = "sem resposta";
  while (Date.now() < deadline) {
    last = await probe(url);
    if (last === "200") return;
    await sleep(POLL_INTERVAL_MS);
  }
  throw new Error(
    `${url} respondeu ${last} por ${STARTUP_TIMEOUT_MS / 60_000} minutos, esperado 200`,
  );
}

async function probe(url: string): Promise<string> {
  try {
    return String((await fetch(url)).status);
  } catch (error) {
    return error instanceof Error ? error.message : String(error);
  }
}

// setup é o globalSetup do Vitest: sobe a stack e espera o Go e o MCP atrás do /mcp.
export async function setup(): Promise<void> {
  compose(["up", "-d", "--build"]);
  try {
    await waitUntilOk(`${markuppUrl()}/v1/healthz`);
    await waitUntilOk(
      `${markuppUrl()}/.well-known/oauth-protected-resource/mcp`,
    );
  } catch (error) {
    compose(["logs", "--tail", "100"]);
    teardown();
    throw error;
  }
}

// teardown derruba a stack com os volumes, inclusive quando algum teste falhou.
export function teardown(): void {
  compose(["down", "-v", "--remove-orphans"]);
}
