import type { Clock } from "../auth/token-exchange.js";
import type { CredentialStore, StoredCredential } from "./credential-store.js";
import type {
  ListenForCallback,
  LoopbackCallback,
} from "./loopback-callback.js";
import { createPkcePair, randomUrlToken, type PkcePair } from "./pkce.js";
import {
  codeGrantForm,
  discoverAuthServer,
  PUBLIC_CLIENT_ID,
  requestTokens,
  STDIO_SCOPE,
  type AuthServerMetadata,
} from "./public-client.js";

export const DEFAULT_LOGIN_URL = "http://localhost:8080";
export const CALLBACK_TIMEOUT_MS = 5 * 60_000;

// LoginDeps é o I/O do login: o Go, o disco, o servidor do callback, o navegador, o
// prazo de espera e o stderr para a pessoa.
export type LoginDeps = {
  fetch: typeof fetch;
  clock: Clock;
  store: CredentialStore;
  listenForCallback: ListenForCallback;
  openBrowser: (url: string) => void;
  timeout: (ms: number) => Promise<void>;
  writeError: (line: string) => void;
};

// LoginAttempt é o que só este login conhece: o PKCE, o state e o callback dele.
type LoginAttempt = { pkce: PkcePair; callback: LoopbackCallback };

// runLogin faz o authorization code com PKCE pelo navegador (RFC 8252) e grava a
// credencial com o issuer da metadata.
export async function runLogin(
  url: string,
  deps: LoginDeps,
): Promise<StoredCredential> {
  const metadata = await discoverAuthServer(url, deps.fetch);
  const state = randomUrlToken();
  const attempt = {
    pkce: createPkcePair(),
    callback: await deps.listenForCallback(state),
  };
  let query: URLSearchParams;
  try {
    query = await waitForBrowser(
      authorizeUrl(metadata, attempt, state),
      attempt,
      deps,
    );
  } finally {
    await attempt.callback.close();
  }
  return storeTokens(metadata, attempt, authorizationCode(query), deps);
}

export function authorizeUrl(
  metadata: AuthServerMetadata,
  attempt: LoginAttempt,
  state: string,
): string {
  const url = new URL(metadata.authorizationEndpoint);
  url.search = new URLSearchParams({
    response_type: "code",
    client_id: PUBLIC_CLIENT_ID,
    redirect_uri: attempt.callback.redirectUri,
    code_challenge: attempt.pkce.challenge,
    code_challenge_method: "S256",
    state,
    scope: STDIO_SCOPE,
    resource: metadata.issuer,
  }).toString();
  return url.href;
}

async function waitForBrowser(
  url: string,
  attempt: LoginAttempt,
  deps: LoginDeps,
): Promise<URLSearchParams> {
  deps.writeError(
    `Opening the browser to sign in to markupp. If it does not open, visit:\n${url}`,
  );
  deps.openBrowser(url);
  const expired = deps.timeout(CALLBACK_TIMEOUT_MS).then((): never => {
    throw new Error(
      `the browser did not return to ${attempt.callback.redirectUri} within ${CALLBACK_TIMEOUT_MS / 60_000} minutes, expected the redirect after consent; run markupp-mcp login again`,
    );
  });
  return Promise.race([attempt.callback.received, expired]);
}

// authorizationCode lê o code do retorno ou falha com o erro OAuth que o Go mandou.
function authorizationCode(query: URLSearchParams): string {
  const error = query.get("error");
  if (error !== null) {
    const description = query.get("error_description") ?? "";
    throw new Error(
      `login refused by markupp: error ${error} (${description})`,
    );
  }
  const code = query.get("code");
  if (!code) {
    throw new Error(
      "browser redirect without code or error, expected ?code=...&state=...",
    );
  }
  return code;
}

async function storeTokens(
  metadata: AuthServerMetadata,
  attempt: LoginAttempt,
  code: string,
  deps: LoginDeps,
): Promise<StoredCredential> {
  const { issuer, tokenEndpoint } = metadata;
  const form = codeGrantForm(
    issuer,
    code,
    attempt.callback.redirectUri,
    attempt.pkce.verifier,
  );
  const credential = {
    issuer,
    ...(await requestTokens(tokenEndpoint, form, deps)),
  };
  await deps.store.save(credential);
  return credential;
}
