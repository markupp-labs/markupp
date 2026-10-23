import { FastMCP } from "fastmcp";
import { ApiTokenCache } from "./auth/api-token-cache.js";
import {
  createHttpAuthenticate,
  type AuthLogSink,
} from "./auth/http-authenticate.js";
import {
  MARKUPP_SCOPES,
  protectedResourceOptions,
} from "./auth/protected-resource.js";
import { exchangeAgentToken, type Clock } from "./auth/token-exchange.js";
import type { HttpConfig } from "./config.js";
import { HttpMarkuppApi } from "./markupp-api.js";
import { forwardDualStack, freeLoopbackPort } from "./dual-stack-forwarder.js";
import { SERVER_INFO } from "./server-info.js";
import type { MarkuppSession } from "./session.js";
import { registerMarkuppTools } from "./tools/register-markupp-tools.js";

// ponytail: teto fixo de agentes com JWT em cache; vira variável se a troca repetida
// aparecer com mais agentes ativos que isso.
const API_TOKEN_CACHE_ENTRIES = 1000;

// HttpServerDeps é o que o servidor HTTP recebe de fora: o fetch para o Go, o relógio
// e o destino do log de falhas de autenticação.
export type HttpServerDeps = {
  fetch: typeof fetch;
  clock: Clock;
  log: AuthLogSink;
};

// createHttpServer monta o FastMCP do transporte HTTP: o authenticate troca o token do
// agente no Go, a metadata RFC 9728 aponta o issuer e cada ferramenta chama a API com o
// JWT da sessão.
export function createHttpServer(
  config: HttpConfig,
  deps: HttpServerDeps,
): FastMCP<MarkuppSession> {
  const server = new FastMCP<MarkuppSession>({
    ...SERVER_INFO,
    authenticate: createHttpAuthenticate({
      resourceUrl: config.resourceUrl,
      tokens: apiTokenCache(config, deps),
      log: deps.log,
    }),
    oauth: protectedResourceOptions(
      config.resourceUrl,
      config.issuerUrl,
      MARKUPP_SCOPES,
    ),
  });
  registerMarkuppTools(server, (session) =>
    apiForSession(config, deps, session),
  );
  return server;
}

function apiTokenCache(
  config: HttpConfig,
  deps: HttpServerDeps,
): ApiTokenCache {
  const exchangeConfig = { ...config, fetch: deps.fetch, clock: deps.clock };
  return new ApiTokenCache(
    (agentToken) => exchangeAgentToken(exchangeConfig, agentToken),
    deps.clock,
    API_TOKEN_CACHE_ENTRIES,
  );
}

function apiForSession(
  config: HttpConfig,
  deps: HttpServerDeps,
  session: MarkuppSession,
): HttpMarkuppApi {
  return new HttpMarkuppApi({
    baseUrl: config.apiUrl,
    fetch: deps.fetch,
    apiToken: session.apiToken,
  });
}

// HttpServerStop encerra o encaminhador e o servidor MCP.
export type HttpServerStop = () => Promise<void>;

// startHttpServer sobe o Streamable HTTP na porta e na rota tiradas do resource, em
// todas as interfaces como o servidor Go, para ser alcançável dentro de um container:
// o fastmcp fica no loopback, atrás do encaminhador que atende IPv6 e IPv4.
export async function startHttpServer(
  server: FastMCP<MarkuppSession>,
  config: HttpConfig,
): Promise<HttpServerStop> {
  const internalPort = await freeLoopbackPort();
  await server.start({
    transportType: "httpStream",
    httpStream: {
      host: "127.0.0.1",
      port: internalPort,
      endpoint: config.endpoint,
    },
  });
  const forwarder = await forwardDualStack(config.port, internalPort);
  return async () => {
    await forwarder.close();
    await server.stop();
  };
}
