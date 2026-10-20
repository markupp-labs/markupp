import { loadHttpEnv, type HttpConfig, type McpEnvironment } from "./config.js";
import {
  errorMessage,
  resolveHttpConfig,
  type StartupDeps,
} from "./resolve-http-config.js";

// CliDeps é o que o CLI faz fora de si: escrever no stderr, o I/O da subida e servir o
// transporte HTTP.
export type CliDeps = {
  writeError: (line: string) => void;
  startup: StartupDeps;
  serveHttp: (config: HttpConfig) => Promise<void>;
};

const USAGE = "uso: markupp-mcp http";

// runCli escolhe o transporte pelo subcomando e devolve o código de saída. Sem
// subcomando seria o stdio, que só ganha login na próxima fatia.
export async function runCli(
  argv: string[],
  env: McpEnvironment,
  deps: CliDeps,
): Promise<number> {
  const [command] = argv;
  if (command === undefined) {
    deps.writeError(
      `o transporte stdio ainda nao tem login; por enquanto use markupp-mcp http\n${USAGE}`,
    );
    return 1;
  }
  if (command !== "http") {
    deps.writeError(`subcomando "${command}" desconhecido\n${USAGE}`);
    return 2;
  }
  return serveHttpFromEnv(env, deps);
}

async function serveHttpFromEnv(
  env: McpEnvironment,
  deps: CliDeps,
): Promise<number> {
  let config: HttpConfig;
  try {
    config = await resolveHttpConfig(loadHttpEnv(env), deps.startup);
  } catch (error) {
    deps.writeError(errorMessage(error));
    return 1;
  }
  await deps.serveHttp(config);
  return 0;
}
