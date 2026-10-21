import type { McpEnvironment } from "../config.js";
import type { StartupDeps } from "../resolve-http-config.js";
import { CredentialStore, credentialsPath } from "./credential-store.js";
import type { LockDeps } from "./file-lock.js";
import type { LocalFiles } from "./local-files.js";
import { DEFAULT_LOGIN_URL, runLogin } from "./login.js";
import { runLogout } from "./logout.js";
import type { ListenForCallback } from "./loopback-callback.js";
import type { StdioConfig } from "./stdio-server.js";
import { storedApiToken } from "./stored-api-token.js";

// LocalDeps é o I/O da máquina da pessoa: o disco da credencial, a pasta pessoal, o
// callback do navegador, o abridor e o prazo do login.
export type LocalDeps = {
  files: LocalFiles;
  homeDir: string;
  listenForCallback: ListenForCallback;
  openBrowser: (url: string) => void;
  timeout: (ms: number) => Promise<void>;
};

// StdioCommandDeps é o que os subcomandos do stdio usam; o fetch, o relógio e a espera
// são os mesmos da subida do HTTP.
export type StdioCommandDeps = {
  writeError: (line: string) => void;
  startup: StartupDeps;
  local: LocalDeps;
  serveStdio: (config: StdioConfig) => Promise<void>;
};

// serveStdio serve o MCP em stdio com o login guardado; sem ele, só o stderr explica.
export async function serveStdio(
  env: McpEnvironment,
  deps: StdioCommandDeps,
): Promise<number> {
  const store = credentialStore(env, deps);
  const credential = await store.load();
  if (credential === undefined) {
    deps.writeError(
      `sem login do markupp em ${store.path}; rode markupp-mcp login [url] no terminal antes de usar o transporte stdio`,
    );
    return 1;
  }
  const apiToken = storedApiToken({
    ...lockDeps(deps),
    store,
    fetch: deps.startup.fetch,
  });
  await deps.serveStdio({ issuer: credential.issuer, apiToken });
  return 0;
}

export async function login(
  url: string | undefined,
  env: McpEnvironment,
  deps: StdioCommandDeps,
): Promise<number> {
  const credential = await runLogin(url ?? DEFAULT_LOGIN_URL, {
    ...deps.local,
    fetch: deps.startup.fetch,
    clock: deps.startup.clock,
    store: credentialStore(env, deps),
    writeError: deps.writeError,
  });
  deps.writeError(`login feito em ${credential.issuer}`);
  return 0;
}

export async function logout(
  env: McpEnvironment,
  deps: StdioCommandDeps,
): Promise<number> {
  const store = credentialStore(env, deps);
  const result = await runLogout({
    ...lockDeps(deps),
    store,
    fetch: deps.startup.fetch,
  });
  const message =
    result === "absent"
      ? `nenhum login do markupp em ${store.path}; nada a revogar`
      : `login revogado e apagado de ${store.path}`;
  deps.writeError(message);
  return 0;
}

function credentialStore(
  env: McpEnvironment,
  deps: StdioCommandDeps,
): CredentialStore {
  return new CredentialStore(
    credentialsPath(env, deps.local.homeDir),
    deps.local.files,
  );
}

function lockDeps(deps: StdioCommandDeps): LockDeps {
  return {
    files: deps.local.files,
    clock: deps.startup.clock,
    sleep: deps.startup.sleep,
  };
}
