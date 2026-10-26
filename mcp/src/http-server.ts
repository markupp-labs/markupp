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
import { relayToLoopback } from "./loopback-relay.js";
import {
  createPublicServer,
  publicEndpointOf,
  startPublicServer,
} from "./public-http-server.js";
import { SERVER_INFO } from "./server-info.js";
import type { MarkuppSession, PublicSession } from "./session.js";
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

// HttpServers são os dois servidores do transporte HTTP: o autenticado, em
// config.endpoint, e o público sem login, embaixo dele (ADR-0051).
export type HttpServers = {
  authenticated: FastMCP<MarkuppSession>;
  anonymous: FastMCP<PublicSession>;
};

// createHttpServer monta os dois servidores, cada um com o próprio registro de
// ferramentas, que nunca se misturam.
export function createHttpServer(
  config: HttpConfig,
  deps: HttpServerDeps,
): HttpServers {
  return {
    authenticated: createAuthenticatedServer(config, deps),
    anonymous: createPublicServer(config.apiUrl, deps.fetch),
  };
}

// createAuthenticatedServer monta o FastMCP com OAuth: o authenticate troca o token do
// agente no Go, a metadata RFC 9728 aponta o issuer e cada ferramenta chama a API com
// o JWT da sessão.
function createAuthenticatedServer(
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

// HttpServerStop encerra o encaminhador e os dois servidores MCP.
export type HttpServerStop = () => Promise<void>;

// startHttpServer sobe o Streamable HTTP na porta e na rota tiradas do resource, em
// todas as interfaces como o servidor Go, para ser alcançável dentro de um container:
// o fastmcp fica no loopback, atrás do encaminhador que atende IPv6 e IPv4. O público
// fica noutra porta do loopback, e o autenticado repassa a rota dele para lá.
export async function startHttpServer(
  servers: HttpServers,
  config: HttpConfig,
): Promise<HttpServerStop> {
  await mountPublicEndpoint(servers, publicEndpointOf(config.endpoint));
  const internalPort = await freeLoopbackPort();
  await servers.authenticated.start({
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
    await servers.authenticated.stop();
    await servers.anonymous.stop();
  };
}

// mountPublicEndpoint sobe o servidor público e liga a rota dele no autenticado, que
// atende as rotas próprias antes de pedir o token.
async function mountPublicEndpoint(
  servers: HttpServers,
  endpoint: `/${string}`,
): Promise<void> {
  const port = await startPublicServer(servers.anonymous, endpoint);
  servers.authenticated
    .getApp()
    .all(endpoint, (context) => relayToLoopback(context.req.raw, port));
}
