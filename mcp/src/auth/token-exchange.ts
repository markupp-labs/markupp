import { z } from "zod";
import { oauthErrorIn, readBody } from "./oauth-response.js";

// Clock devolve o instante atual em milissegundos desde a época.
export type Clock = () => number;

export const TOKEN_EXCHANGE_GRANT =
  "urn:ietf:params:oauth:grant-type:token-exchange";
export const ACCESS_TOKEN_TYPE =
  "urn:ietf:params:oauth:token-type:access_token";

// TokenExchangeConfig é o servidor MCP como resource do Go: a URL dele, o segredo
// configurado no Go para ela e o endereço do Go por onde a troca passa, que pode ser o
// interno do cluster ou do compose em vez da URL pública do issuer.
export type TokenExchangeConfig = {
  apiUrl: string;
  resourceUrl: string;
  resourceSecret: string;
  fetch: typeof fetch;
  clock: Clock;
};

// ExchangedApiToken é o JWT curto da API e o instante em que ele expira.
export type ExchangedApiToken = { accessToken: string; expiresAtMs: number };

// TokenExchangeError é a recusa do endpoint de token, com o código OAuth da resposta
// (invalid_grant: token do agente; invalid_client: credencial do servidor MCP).
export class TokenExchangeError extends Error {
  readonly httpStatus: number;
  readonly oauthError: string;

  constructor(httpStatus: number, oauthError: string, description: string) {
    super(
      `troca de token recusada pelo issuer: status ${httpStatus}, error ${oauthError} (${description}); esperado 200 com access_token e expires_in`,
    );
    this.name = "TokenExchangeError";
    this.httpStatus = httpStatus;
    this.oauthError = oauthError;
  }
}

// isAgentTokenRefused separa a recusa do token do agente (revogado, expirado ou de outro
// resource), que um novo login resolve, das falhas de configuração e do issuer.
export function isAgentTokenRefused(
  error: unknown,
): error is TokenExchangeError {
  return (
    error instanceof TokenExchangeError && error.oauthError === "invalid_grant"
  );
}

const exchangeSuccessBody = z.object({
  access_token: z.string().min(1),
  expires_in: z.number().int().positive(),
});

const SUCCESS_SHAPE =
  "esperado {access_token: string, expires_in: inteiro positivo}";

// exchangeAgentToken troca o token opaco do agente pelo JWT da API (RFC 8693), com o
// servidor MCP autenticado por HTTP Basic como resource.
export async function exchangeAgentToken(
  config: TokenExchangeConfig,
  agentToken: string,
): Promise<ExchangedApiToken> {
  const response = await config.fetch(
    new URL("/oauth/token", config.apiUrl),
    exchangeRequestInit(config, agentToken),
  );
  if (!response.ok) {
    throw await exchangeRefusal(response);
  }
  return readExchangedToken(response, config.clock());
}

function exchangeRequestInit(
  config: TokenExchangeConfig,
  agentToken: string,
): RequestInit {
  return {
    method: "POST",
    headers: {
      authorization: resourceBasicAuth(
        config.resourceUrl,
        config.resourceSecret,
      ),
      "content-type": "application/x-www-form-urlencoded",
      accept: "application/json",
    },
    body: exchangeForm(agentToken),
  };
}

function exchangeForm(agentToken: string): URLSearchParams {
  return new URLSearchParams({
    grant_type: TOKEN_EXCHANGE_GRANT,
    subject_token: agentToken,
    subject_token_type: ACCESS_TOKEN_TYPE,
  });
}

// resourceBasicAuth codifica usuário e senha antes do base64 (RFC 6749, seção 2.3.1):
// sem isso o ":" da URL do resource partiria a credencial no lugar errado.
export function resourceBasicAuth(resourceUrl: string, secret: string): string {
  const credentials = `${encodeURIComponent(resourceUrl)}:${encodeURIComponent(secret)}`;
  return `Basic ${btoa(credentials)}`;
}

async function exchangeRefusal(
  response: Response,
): Promise<TokenExchangeError> {
  const { code, description } = oauthErrorIn(await readBody(response));
  return new TokenExchangeError(response.status, code, description);
}

async function readExchangedToken(
  response: Response,
  nowMs: number,
): Promise<ExchangedApiToken> {
  const body = await readBody(response);
  if (body.json === undefined) {
    throw new Error(
      `resposta da troca de token nao e JSON: status ${response.status}, corpo ${JSON.stringify(body.excerpt)}; ${SUCCESS_SHAPE}`,
    );
  }
  const parsed = exchangeSuccessBody.safeParse(body.json);
  if (!parsed.success) {
    throw new Error(
      `resposta da troca de token fora do formato: ${parsed.error.message}; ${SUCCESS_SHAPE}`,
    );
  }
  const { access_token, expires_in } = parsed.data;
  return { accessToken: access_token, expiresAtMs: nowMs + expires_in * 1000 };
}
