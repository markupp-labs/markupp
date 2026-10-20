import {
  loadHttpConfig,
  type HttpConfig,
  type McpEnvironment,
} from "./config.js";

// CliDeps é o que o CLI faz fora de si: escrever no stderr e servir o transporte HTTP.
export type CliDeps = {
  writeError: (line: string) => void;
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
    config = loadHttpConfig(env);
  } catch (error) {
    deps.writeError(error instanceof Error ? error.message : String(error));
    return 1;
  }
  await deps.serveHttp(config);
  return 0;
}
