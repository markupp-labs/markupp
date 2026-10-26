import { loadHttpEnv, type HttpConfig, type McpEnvironment } from "./config.js";
import { errorMessage, resolveHttpConfig } from "./resolve-http-config.js";
import {
  login,
  logout,
  serveStdio,
  type StdioCommandDeps,
} from "./stdio/commands.js";

// CliDeps é o que o CLI faz fora de si: escrever no stderr e, só na ajuda e na versão,
// no stdout; o I/O da subida e da máquina da pessoa, e servir cada transporte.
export type CliDeps = StdioCommandDeps & {
  writeOutput: (line: string) => void;
  version: string;
  serveHttp: (config: HttpConfig) => Promise<void>;
};

// Subcommand é um subcomando com o máximo de argumentos que ele aceita.
type Subcommand = {
  maxArgs: number;
  run: (args: string[], env: McpEnvironment, deps: CliDeps) => Promise<number>;
};

export const USAGE = `uso: markupp-mcp [login [url] | logout | http | help | --version]
  (sem subcomando)  serve o MCP em stdio com o login guardado
  login [url]       entra pelo navegador (url padrao http://localhost:8080)
  logout            revoga e apaga o login guardado
  http              serve o MCP em Streamable HTTP
  help, -h, --help  mostra este uso
  --version, -v     mostra a versao`;

// INFO_FLAGS são as opções que só imprimem algo no stdout e saem com 0, sem servir nada.
const INFO_FLAGS: Record<string, (deps: CliDeps) => string> = {
  help: () => USAGE,
  "-h": () => USAGE,
  "--help": () => USAGE,
  "-v": (deps) => deps.version,
  "--version": (deps) => deps.version,
};

const SUBCOMMANDS: Record<string, Subcommand> = {
  login: { maxArgs: 1, run: ([url], env, deps) => login(url, env, deps) },
  logout: { maxArgs: 0, run: (_args, env, deps) => logout(env, deps) },
  http: { maxArgs: 0, run: (_args, env, deps) => serveHttpFromEnv(env, deps) },
};

// runCli escolhe o subcomando e devolve o código de saída: 1 na falha, 2 no uso errado.
// Só a ajuda e a versão vão para o stdout, que no stdio é do protocolo MCP.
export async function runCli(
  argv: string[],
  env: McpEnvironment,
  deps: CliDeps,
): Promise<number> {
  const [command, ...args] = argv;
  if (command === undefined)
    return exitCodeOf(deps, () => serveStdio(env, deps));
  const info = ownEntry(INFO_FLAGS, command);
  if (info !== undefined) return printInfo(info(deps), deps);
  const subcommand = ownEntry(SUBCOMMANDS, command);
  if (subcommand === undefined) {
    deps.writeError(`subcomando "${command}" desconhecido\n${USAGE}`);
    return 2;
  }
  if (args.length > subcommand.maxArgs) {
    deps.writeError(
      `argumentos demais para ${command}: ${args.join(" ")}\n${USAGE}`,
    );
    return 2;
  }
  return exitCodeOf(deps, () => subcommand.run(args, env, deps));
}

// ownEntry lê só chaves próprias, para "toString" não virar subcomando.
function ownEntry<T>(entries: Record<string, T>, key: string): T | undefined {
  return Object.hasOwn(entries, key) ? entries[key] : undefined;
}

function printInfo(line: string, deps: CliDeps): number {
  deps.writeOutput(line);
  return 0;
}

async function exitCodeOf(
  deps: CliDeps,
  run: () => Promise<number>,
): Promise<number> {
  try {
    return await run();
  } catch (error) {
    deps.writeError(errorMessage(error));
    return 1;
  }
}

async function serveHttpFromEnv(
  env: McpEnvironment,
  deps: CliDeps,
): Promise<number> {
  const config = await resolveHttpConfig(loadHttpEnv(env), deps.startup);
  await deps.serveHttp(config);
  return 0;
}
