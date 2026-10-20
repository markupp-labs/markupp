import type { Clock } from "./auth/token-exchange.js";
import {
  checkPublicUrl,
  checkSecretSize,
  ISSUER_RULE,
  type HttpConfig,
  type HttpEnvConfig,
  type ResourceSecretSource,
} from "./config.js";

// StartupDeps é o I/O da subida: o fetch para a metadata do Go, a leitura do arquivo
// do segredo e o relógio com a espera entre tentativas.
export type StartupDeps = {
  fetch: typeof fetch;
  readFile: (path: string) => Promise<string>;
  clock: Clock;
  sleep: (ms: number) => Promise<void>;
};

type Settled<T> = { ok: true; value: T } | { ok: false; error: unknown };

const RETRY_INTERVAL_MS = 1_000;
const STARTUP_TIMEOUT_MS = 60_000;
export const REQUEST_TIMEOUT_MS = 5_000;
export const METADATA_PATH = "/.well-known/oauth-authorization-server";

// resolveHttpConfig completa a config do ambiente com o que depende do Go: o issuer
// da metadata RFC 8414 e, sem a variável, o segredo do arquivo que o Go escreve.
export async function resolveHttpConfig(
  env: HttpEnvConfig,
  deps: StartupDeps,
): Promise<HttpConfig> {
  const issuerUrl = await discoverIssuer(env.apiUrl, deps);
  const resourceUrl = env.resourceUrl ?? `${issuerUrl}/mcp`;
  return {
    apiUrl: env.apiUrl,
    issuerUrl,
    resourceUrl,
    resourceSecret: await resourceSecret(env.secret, deps),
    endpoint: new URL(resourceUrl).pathname as `/${string}`,
    port: env.port,
  };
}

async function discoverIssuer(
  apiUrl: string,
  deps: StartupDeps,
): Promise<string> {
  const metadataUrl = `${apiUrl}${METADATA_PATH}`;
  const issuer = await retryUntilDeadline(
    `metadata do Go em ${metadataUrl}`,
    () => fetchIssuer(metadataUrl, deps.fetch),
    deps,
  );
  return checkPublicUrl(`issuer em ${metadataUrl}`, issuer, ISSUER_RULE).origin;
}

async function fetchIssuer(
  metadataUrl: string,
  fetcher: typeof fetch,
): Promise<string> {
  const signal = AbortSignal.timeout(REQUEST_TIMEOUT_MS);
  const response = await fetcher(metadataUrl, { signal });
  if (!response.ok) {
    throw new Error(`status ${response.status}, esperado 200`);
  }
  const metadata = (await response.json()) as { issuer?: unknown };
  if (typeof metadata.issuer !== "string") {
    throw new Error(
      `issuer=${JSON.stringify(metadata.issuer)}, esperado a URL do issuer em texto`,
    );
  }
  return metadata.issuer;
}

async function resourceSecret(
  source: ResourceSecretSource,
  deps: StartupDeps,
): Promise<string> {
  if (source.kind === "value") return source.value;
  const secret = await retryUntilDeadline(
    `segredo do resource em ${source.path}`,
    () => readSecretFile(source.path, deps.readFile),
    deps,
  );
  return checkSecretSize(`segredo em ${source.path}`, secret);
}

// readSecretFile trata o arquivo vazio como ainda não escrito, e o erro nunca cita o
// conteúdo.
async function readSecretFile(
  path: string,
  readFile: StartupDeps["readFile"],
): Promise<string> {
  const secret = (await readFile(path)).trim();
  if (secret === "") {
    throw new Error("arquivo vazio, esperado o segredo escrito pelo Go");
  }
  return secret;
}

// retryUntilDeadline repete a tentativa enquanto o Go sobe e, vencido o prazo, falha
// com o que se esperava e o último erro.
async function retryUntilDeadline<T>(
  waitingFor: string,
  attempt: () => Promise<T>,
  deps: StartupDeps,
): Promise<T> {
  const deadline = deps.clock() + STARTUP_TIMEOUT_MS;
  for (;;) {
    const outcome = await settle(attempt);
    if (outcome.ok) return outcome.value;
    if (deps.clock() >= deadline) {
      throw new Error(
        `${waitingFor} indisponivel apos ${STARTUP_TIMEOUT_MS / 1000}s, ultimo erro: ${errorMessage(outcome.error)}`,
      );
    }
    await deps.sleep(RETRY_INTERVAL_MS);
  }
}

function settle<T>(attempt: () => Promise<T>): Promise<Settled<T>> {
  return attempt().then(
    (value): Settled<T> => ({ ok: true, value }),
    (error: unknown): Settled<T> => ({ ok: false, error }),
  );
}

// errorMessage inclui a causa, que no fetch do Node é onde fica o ECONNREFUSED.
export function errorMessage(error: unknown): string {
  if (!(error instanceof Error)) return String(error);
  const cause = error.cause instanceof Error ? `: ${error.cause.message}` : "";
  return `${error.message}${cause}`;
}
