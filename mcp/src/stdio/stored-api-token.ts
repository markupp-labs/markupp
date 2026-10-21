import { EXPIRY_MARGIN_MS } from "../auth/api-token-cache.js";
import type { Clock } from "../auth/token-exchange.js";
import type { ApiTokenSource } from "../session.js";
import type { CredentialStore, StoredCredential } from "./credential-store.js";
import { withFileLock, type LockDeps } from "./file-lock.js";
import {
  discoverAuthServer,
  isInvalidGrant,
  refreshGrantForm,
  requestTokens,
} from "./public-client.js";

// StoredApiTokenDeps é o que a renovação no stdio usa: a credencial no disco, a trava
// entre processos e o fetch para o Go.
export type StoredApiTokenDeps = LockDeps & {
  store: CredentialStore;
  fetch: typeof fetch;
};

// LoginRequiredError é a credencial ausente ou recusada pelo Go; o texto vai ao agente,
// que só resolve pedindo à pessoa um novo login no terminal.
export class LoginRequiredError extends Error {
  constructor(reason: string, issuer: string = "") {
    const command =
      issuer === "" ? "markupp-mcp login" : `markupp-mcp login ${issuer}`;
    super(
      `unauthenticated: ${reason}. Ask the user to run "${command}" in a terminal and retry.`,
    );
    this.name = "LoginRequiredError";
  }
}

// storedApiToken é o ApiTokenSource do stdio: entrega o JWT do disco enquanto faltar
// mais que a margem para expirar e, senão, renova com o lock entre processos.
export function storedApiToken(deps: StoredApiTokenDeps): ApiTokenSource {
  return async () => {
    const current = await loadRequired(deps.store);
    if (stillValid(current, deps.clock)) return current.accessToken;
    return withFileLock(deps.store.lockPath, deps, () => renewUnderLock(deps));
  };
}

// renewUnderLock relê o arquivo já com o lock: se outro processo renovou enquanto este
// esperava, usa o JWT dele em vez de girar de novo o refresh que ele já girou.
async function renewUnderLock(deps: StoredApiTokenDeps): Promise<string> {
  const current = await loadRequired(deps.store);
  if (stillValid(current, deps.clock)) return current.accessToken;
  const renewed = await refreshCredential(current, deps);
  await deps.store.save(renewed);
  return renewed.accessToken;
}

async function refreshCredential(
  current: StoredCredential,
  deps: StoredApiTokenDeps,
): Promise<StoredCredential> {
  const metadata = await discoverAuthServer(current.issuer, deps.fetch);
  const form = refreshGrantForm(current.issuer, current.refreshToken);
  try {
    const tokens = await requestTokens(metadata.tokenEndpoint, form, deps);
    return { issuer: current.issuer, ...tokens };
  } catch (error) {
    if (!isInvalidGrant(error)) throw error;
    await deps.store.remove();
    throw new LoginRequiredError(
      "the markupp server refused the stored login",
      current.issuer,
    );
  }
}

async function loadRequired(store: CredentialStore): Promise<StoredCredential> {
  const credential = await store.load();
  if (credential === undefined) {
    throw new LoginRequiredError(`no markupp login stored in ${store.path}`);
  }
  return credential;
}

function stillValid(credential: StoredCredential, clock: Clock): boolean {
  return clock() < credential.expiresAtMs - EXPIRY_MARGIN_MS;
}
