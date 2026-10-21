import { FastMCP, type ServerOptions } from "fastmcp";
import { HttpMarkuppApi } from "../markupp-api.js";
import { SERVER_INFO } from "../server-info.js";
import type { ApiTokenSource, MarkuppSession } from "../session.js";
import { registerMarkuppTools } from "../tools/register-markupp-tools.js";

// StdioConfig é o login guardado: o issuer, que é também a base da API, e o JWT
// renovado pelo refresh do disco.
export type StdioConfig = { issuer: string; apiToken: ApiTokenSource };

export type StdioServerDeps = {
  fetch: typeof fetch;
  writeLog: (line: string) => void;
};

type FastMcpLogger = NonNullable<ServerOptions<MarkuppSession>["logger"]>;
type LogLevel = keyof FastMcpLogger;

// createStdioServer monta o FastMCP do stdio. No stdio o FastMCP chama o authenticate
// uma vez, sem requisição, e entrega o retorno como a sessão de todas as ferramentas.
// O log vai para o stderr porque o stdout é do protocolo MCP.
export function createStdioServer(
  config: StdioConfig,
  deps: StdioServerDeps,
): FastMCP<MarkuppSession> {
  const session: MarkuppSession = { apiToken: config.apiToken };
  const server = new FastMCP<MarkuppSession>({
    ...SERVER_INFO,
    authenticate: async () => session,
    logger: jsonLogger(deps.writeLog),
  });
  registerMarkuppTools(
    server,
    (caller) =>
      new HttpMarkuppApi({
        baseUrl: config.issuer,
        fetch: deps.fetch,
        apiToken: caller.apiToken,
      }),
  );
  return server;
}

export function startStdioServer(
  server: FastMCP<MarkuppSession>,
): Promise<void> {
  return server.start({ transportType: "stdio" });
}

// jsonLogger troca o console padrão do FastMCP, que escreveria info e debug no stdout,
// por uma linha JSON por entrada.
export function jsonLogger(writeLog: (line: string) => void): FastMcpLogger {
  const at =
    (level: LogLevel) =>
    (...args: unknown[]): void =>
      writeLog(JSON.stringify({ level, msg: args.map(String).join(" ") }));
  return {
    debug: at("debug"),
    error: at("error"),
    info: at("info"),
    log: at("log"),
    warn: at("warn"),
  };
}
