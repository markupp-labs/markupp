import type { MarkuppSession } from "../session.js";
import type { ApiTokenCache } from "./api-token-cache.js";
import { protectedResourceMetadataUrl } from "./protected-resource.js";
import { isAgentTokenRefused, TokenExchangeError } from "./token-exchange.js";

// AuthFailureLog é a entrada JSON de uma troca de token que falhou sem ser culpa do
// agente; nunca leva o token do agente.
export type AuthFailureLog = {
  level: "error";
  msg: string;
  error: string;
  oauth_error?: string;
  http_status?: number;
};

export type AuthLogSink = (entry: AuthFailureLog) => void;

export type HttpAuthenticateConfig = {
  resourceUrl: string;
  tokens: ApiTokenCache;
  log: AuthLogSink;
};

// AuthenticatedRequest é a parte do http.IncomingMessage que o authenticate lê; o
// tsconfig não carrega os tipos do Node.
export type AuthenticatedRequest = { headers: { authorization?: string } };

export type HttpAuthenticate = (
  request: AuthenticatedRequest,
) => Promise<MarkuppSession>;

// createHttpAuthenticate monta o authenticate do FastMCP no HTTP. Ele roda a cada
// requisição, então a troca acontece já aqui (vinda do cache quase sempre) para
// responder 401 ao token recusado; o apiToken da sessão troca de novo quando o JWT
// expira. A recusa sai como Response lançada, que o mcp-proxy repassa com os cabeçalhos.
export function createHttpAuthenticate(
  config: HttpAuthenticateConfig,
): HttpAuthenticate {
  return async (request) => {
    const agentToken = bearerToken(request.headers.authorization);
    if (agentToken === undefined) {
      throw unauthorized(config.resourceUrl, false);
    }
    await checkAgentToken(config, agentToken);
    return { apiToken: () => renewApiToken(config, agentToken) };
  };
}

// renewApiToken entrega o JWT no meio da sessão. A recusa do token do agente segue para
// a ferramenta pedir novo login; o resto vai para o log e chega ao agente sem detalhe.
async function renewApiToken(
  config: HttpAuthenticateConfig,
  agentToken: string,
): Promise<string> {
  try {
    return await config.tokens.apiTokenFor(agentToken);
  } catch (error) {
    if (isAgentTokenRefused(error)) throw error;
    config.log(exchangeFailureLog(error));
    throw new Error("internal error: could not renew the markupp API token");
  }
}

// bearerToken lê o token de "Authorization: Bearer <token>"; outro esquema conta como
// requisição sem token (RFC 6750, seção 3.1).
export function bearerToken(header: string | undefined): string | undefined {
  const match = /^Bearer +(\S+) *$/i.exec(header ?? "");
  return match?.[1];
}

async function checkAgentToken(
  config: HttpAuthenticateConfig,
  agentToken: string,
): Promise<void> {
  try {
    await config.tokens.apiTokenFor(agentToken);
  } catch (error) {
    throw refusalResponse(config, error);
  }
}

// refusalResponse responde 401 ao token do agente recusado e 500 ao resto, que é erro
// de configuração ou do issuer e vai para o log.
function refusalResponse(
  config: HttpAuthenticateConfig,
  error: unknown,
): Response {
  if (isAgentTokenRefused(error)) {
    return unauthorized(config.resourceUrl, true);
  }
  config.log(exchangeFailureLog(error));
  return jsonRpcError(500, -32603, "Internal Server Error", {});
}

export function exchangeFailureLog(error: unknown): AuthFailureLog {
  const message = error instanceof Error ? error.message : String(error);
  if (!(error instanceof TokenExchangeError)) {
    return { level: "error", msg: "troca de token falhou", error: message };
  }
  const msg =
    error.oauthError === "invalid_client"
      ? "issuer recusou a credencial do servidor MCP; confira a URL do resource e o segredo configurados nos dois lados"
      : "issuer recusou a troca de token";
  return {
    level: "error",
    msg,
    error: message,
    oauth_error: error.oauthError,
    http_status: error.httpStatus,
  };
}

// unauthorized aponta a metadata do recurso protegido (RFC 9728, seção 5.1) e marca
// invalid_token quando havia token (RFC 6750, seção 3.1).
export function unauthorized(resourceUrl: string, hadToken: boolean): Response {
  const metadata = `resource_metadata="${protectedResourceMetadataUrl(resourceUrl)}"`;
  const challenge = hadToken ? `${metadata}, error="invalid_token"` : metadata;
  const message = hadToken ? "Unauthorized: invalid token" : "Unauthorized";
  return jsonRpcError(401, -32000, message, {
    "www-authenticate": `Bearer ${challenge}`,
  });
}

// jsonRpcError segue o envelope que o mcp-proxy usa nas próprias recusas.
function jsonRpcError(
  status: number,
  code: number,
  message: string,
  headers: Record<string, string>,
): Response {
  const body = JSON.stringify({
    error: { code, message },
    id: null,
    jsonrpc: "2.0",
  });
  return new Response(body, {
    status,
    headers: { "content-type": "application/json", ...headers },
  });
}
