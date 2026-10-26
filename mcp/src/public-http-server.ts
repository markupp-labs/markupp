import type { IncomingMessage } from "node:http";
import { FastMCP } from "fastmcp";
import { resourceBasicAuth } from "./auth/token-exchange.js";
import type { HttpConfig } from "./config.js";
import { freeLoopbackPort } from "./dual-stack-forwarder.js";
import type { FetchFunction } from "./markupp-api.js";
import { HttpPublicMarkuppApi } from "./public-api.js";
import { SERVER_INFO } from "./server-info.js";
import type { PublicSession } from "./session.js";
import { registerPublicTools } from "./tools/register-public-tools.js";

const PUBLIC_INSTRUCTIONS =
  "Reads markupp vaults that their owners published for anyone, without an account. Every tool takes the vault slug, the {slug} of its public URL /p/{slug}: ask the user for it when unknown. Start with list_public_notes sorted by pagerank or with search_public_notes, then read_public_note.";

// PublicServerConfig é a API do Go e a credencial de resource do MCP, que ele manda à
// leitura pública para o Go contar o limite pelo IP do agente.
export type PublicServerConfig = Pick<
  HttpConfig,
  "apiUrl" | "resourceUrl" | "resourceSecret"
>;

// createPublicServer monta o FastMCP de /mcp/public (ADR-0051): sem OAuth, com só as
// ferramentas de leitura dos cofres publicados, que chamam /v1/public no Go.
export function createPublicServer(
  config: PublicServerConfig,
  fetcher: FetchFunction,
): FastMCP<PublicSession> {
  const relayAuthorization = resourceBasicAuth(
    config.resourceUrl,
    config.resourceSecret,
  );
  const server = new FastMCP<PublicSession>({
    ...SERVER_INFO,
    name: `${SERVER_INFO.name}-public`,
    instructions: PUBLIC_INSTRUCTIONS,
    authenticate: async (request) => publicSessionOf(request),
  });
  registerPublicTools(
    server,
    (session) =>
      new HttpPublicMarkuppApi({
        baseUrl: config.apiUrl,
        fetch: fetcher,
        forwardedFor: session?.forwardedFor,
        relayAuthorization,
      }),
  );
  return server;
}

// publicSessionOf nunca recusa: só guarda o X-Forwarded-For que chegou, para o Go
// contar o limite pelo IP do agente.
export function publicSessionOf(request: IncomingMessage): PublicSession {
  const header = request.headers["x-forwarded-for"];
  const forwardedFor = Array.isArray(header) ? header.join(", ") : header;
  return forwardedFor ? { forwardedFor } : {};
}

// publicEndpointOf põe o endpoint público embaixo do autenticado: /mcp vira
// /mcp/public.
export function publicEndpointOf(endpoint: `/${string}`): `/${string}` {
  return `/${endpoint.slice(1).replace(/\/+$/, "")}/public`;
}

// startPublicServer sobe o servidor público sem estado no loopback, alcançável só pelo
// repasse do servidor autenticado, e devolve a porta.
export async function startPublicServer(
  server: FastMCP<PublicSession>,
  endpoint: `/${string}`,
): Promise<number> {
  const port = await freeLoopbackPort();
  await server.start({
    transportType: "httpStream",
    httpStream: { host: "127.0.0.1", port, endpoint, stateless: true },
  });
  return port;
}
