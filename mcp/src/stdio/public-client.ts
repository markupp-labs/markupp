import { z } from "zod";
import {
  oauthErrorIn,
  readBody,
  type ReadBody,
} from "../auth/oauth-response.js";
import { MARKUPP_SCOPES } from "../auth/protected-resource.js";
import type { Clock } from "../auth/token-exchange.js";
import { checkPublicUrl, ISSUER_RULE } from "../config.js";
import { METADATA_PATH, REQUEST_TIMEOUT_MS } from "../resolve-http-config.js";

// PUBLIC_CLIENT_ID é o cliente público que o Go conhece de fábrica para o stdio
// (ADR-0042): sem segredo, redirect só em loopback e PKCE.
export const PUBLIC_CLIENT_ID = "markupp-mcp";
export const STDIO_SCOPE = MARKUPP_SCOPES.join(" ");

// AuthServerMetadata é a parte da metadata RFC 8414 que o login, a renovação e o
// logout usam.
export type AuthServerMetadata = {
  issuer: string;
  authorizationEndpoint: string;
  tokenEndpoint: string;
  revocationEndpoint: string;
};

// IssuedTokens é o par que o endpoint de token entrega ao cliente público: o JWT curto
// da API, quando ele expira, e o refresh opaco e rotativo.
export type IssuedTokens = {
  accessToken: string;
  expiresAtMs: number;
  refreshToken: string;
};

export type TokenRequestDeps = { fetch: typeof fetch; clock: Clock };

// OAuthGrantError é a recusa do endpoint de token ou de revogação, com o código OAuth.
export class OAuthGrantError extends Error {
  readonly oauthError: string;

  constructor(endpoint: string, status: number, code: string, detail: string) {
    super(
      `${endpoint} recusou o pedido: status ${status}, error ${code} (${detail}); esperado 200`,
    );
    this.name = "OAuthGrantError";
    this.oauthError = code;
  }
}

// isInvalidGrant separa o refresh ou code recusado, que só um novo login resolve.
export function isInvalidGrant(error: unknown): error is OAuthGrantError {
  return (
    error instanceof OAuthGrantError && error.oauthError === "invalid_grant"
  );
}

const METADATA_SHAPE =
  "esperado {issuer, authorization_endpoint, token_endpoint, revocation_endpoint}";

const metadataBody = z.object({
  issuer: z.string(),
  authorization_endpoint: z.string(),
  token_endpoint: z.string(),
  revocation_endpoint: z.string(),
});

const TOKENS_SHAPE =
  "esperado {access_token: string, expires_in: inteiro positivo, refresh_token: string}";

const tokensBody = z.object({
  access_token: z.string().min(1),
  expires_in: z.number().int().positive(),
  refresh_token: z.string().min(1),
});

// discoverAuthServer lê a metadata do markupp em url e exige o issuer igual à origem
// pedida (RFC 8414, seção 3.3) e os endpoints nessa mesma origem, para o refresh nunca
// sair para outro host.
export async function discoverAuthServer(
  url: string,
  fetcher: typeof fetch,
): Promise<AuthServerMetadata> {
  const origin = checkPublicUrl("url do markupp", url, ISSUER_RULE).origin;
  const metadataUrl = `${origin}${METADATA_PATH}`;
  const signal = AbortSignal.timeout(REQUEST_TIMEOUT_MS);
  const response = await fetcher(metadataUrl, { signal });
  if (!response.ok) {
    throw new Error(
      `metadata em ${metadataUrl} respondeu status ${response.status}, esperado 200`,
    );
  }
  return metadataFrom(metadataUrl, origin, await readBody(response));
}

function metadataFrom(
  metadataUrl: string,
  origin: string,
  body: ReadBody,
): AuthServerMetadata {
  const parsed = metadataBody.safeParse(body.json);
  if (!parsed.success) {
    throw new Error(
      `metadata em ${metadataUrl} fora do formato: ${JSON.stringify(body.excerpt)}; ${METADATA_SHAPE}`,
    );
  }
  const { issuer } = parsed.data;
  if (URL.parse(issuer)?.origin !== origin) {
    throw new Error(
      `issuer em ${metadataUrl}="${issuer}", esperado ${origin}, a origem da url pedida`,
    );
  }
  return endpointsOf(parsed.data, origin);
}

function endpointsOf(
  body: z.infer<typeof metadataBody>,
  origin: string,
): AuthServerMetadata {
  return {
    issuer: body.issuer,
    authorizationEndpoint: sameOrigin(
      "authorization_endpoint",
      body.authorization_endpoint,
      origin,
    ),
    tokenEndpoint: sameOrigin("token_endpoint", body.token_endpoint, origin),
    revocationEndpoint: sameOrigin(
      "revocation_endpoint",
      body.revocation_endpoint,
      origin,
    ),
  };
}

function sameOrigin(name: string, raw: string, origin: string): string {
  const url = URL.parse(raw);
  if (!url || url.origin !== origin) {
    throw new Error(`${name}="${raw}", esperado uma url na origem ${origin}`);
  }
  return url.href;
}

// codeGrantForm troca o code do callback pelo par de tokens, com o verifier do PKCE.
export function codeGrantForm(
  issuer: string,
  code: string,
  redirectUri: string,
  codeVerifier: string,
): Record<string, string> {
  return {
    grant_type: "authorization_code",
    code,
    redirect_uri: redirectUri,
    client_id: PUBLIC_CLIENT_ID,
    code_verifier: codeVerifier,
    resource: issuer,
  };
}

// refreshGrantForm gira o refresh: o Go devolve um par novo e invalida o anterior.
export function refreshGrantForm(
  issuer: string,
  refreshToken: string,
): Record<string, string> {
  return {
    grant_type: "refresh_token",
    refresh_token: refreshToken,
    client_id: PUBLIC_CLIENT_ID,
    resource: issuer,
  };
}

// requestTokens posta o formulário no endpoint de token, sem Basic: o cliente é
// público.
export async function requestTokens(
  tokenEndpoint: string,
  form: Record<string, string>,
  deps: TokenRequestDeps,
): Promise<IssuedTokens> {
  const response = await deps.fetch(tokenEndpoint, postForm(form));
  const body = await readBody(response);
  if (!response.ok) throw grantRefusal(tokenEndpoint, response.status, body);
  return issuedTokens(tokenEndpoint, body, deps.clock());
}

// issuedTokens nunca cita o corpo no erro: a resposta de sucesso carrega os tokens.
function issuedTokens(
  tokenEndpoint: string,
  body: ReadBody,
  nowMs: number,
): IssuedTokens {
  const parsed = tokensBody.safeParse(body.json);
  if (!parsed.success) {
    const fields = parsed.error.issues.map((i) => i.path.join(".")).join(", ");
    throw new Error(
      `resposta de ${tokenEndpoint} fora do formato nos campos [${fields}]; ${TOKENS_SHAPE}`,
    );
  }
  const { access_token, expires_in, refresh_token } = parsed.data;
  return {
    accessToken: access_token,
    expiresAtMs: nowMs + expires_in * 1000,
    refreshToken: refresh_token,
  };
}

// revokeToken revoga o token no endpoint de revogação (RFC 7009).
export async function revokeToken(
  revocationEndpoint: string,
  token: string,
  fetcher: typeof fetch,
): Promise<void> {
  const form = { token, client_id: PUBLIC_CLIENT_ID };
  const response = await fetcher(revocationEndpoint, postForm(form));
  if (response.ok) return;
  throw grantRefusal(
    revocationEndpoint,
    response.status,
    await readBody(response),
  );
}

function postForm(form: Record<string, string>): RequestInit {
  return {
    method: "POST",
    headers: {
      "content-type": "application/x-www-form-urlencoded",
      accept: "application/json",
    },
    body: new URLSearchParams(form),
    signal: AbortSignal.timeout(REQUEST_TIMEOUT_MS),
  };
}

function grantRefusal(
  endpoint: string,
  status: number,
  body: ReadBody,
): OAuthGrantError {
  const { code, description } = oauthErrorIn(body);
  return new OAuthGrantError(endpoint, status, code, description);
}
