import { z } from "zod";

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

const oauthErrorBody = z.object({
  error: z.string().min(1),
  error_description: z.string().optional(),
});

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

// BODY_EXCERPT_CHARS limita o trecho do corpo citado nas mensagens de erro; as
// respostas do issuer nunca repetem a credencial nem o subject_token.
const BODY_EXCERPT_CHARS = 200;

// ReadBody é o corpo lido uma vez: o trecho do texto cru, para citar no erro, e o JSON
// (undefined quando o texto não é JSON válido).
type ReadBody = { excerpt: string; json: unknown };

// readBody lê o corpo como texto e tenta o JSON sem lançar: proxy na frente do issuer
// pode devolver HTML.
async function readBody(response: Response): Promise<ReadBody> {
  const text = await response.text();
  const excerpt = text.slice(0, BODY_EXCERPT_CHARS);
  try {
    return { excerpt, json: JSON.parse(text) as unknown };
  } catch {
    return { excerpt, json: undefined };
  }
}

async function exchangeRefusal(
  response: Response,
): Promise<TokenExchangeError> {
  const body = await readBody(response);
  const parsed = oauthErrorBody.safeParse(body.json);
  if (!parsed.success) {
    return new TokenExchangeError(
      response.status,
      "sem_codigo_oauth",
      `corpo fora do formato {error, error_description}: ${JSON.stringify(body.excerpt)}`,
    );
  }
  const { error, error_description } = parsed.data;
  return new TokenExchangeError(
    response.status,
    error,
    error_description ?? "",
  );
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
