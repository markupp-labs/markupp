import type { SpawnOptions } from "node:child_process";

// BrowserCommand é o comando que abre a URL no navegador padrão do sistema.
export type BrowserCommand = {
  command: string;
  args: string[];
  verbatim: boolean;
};

// SpawnedProcess é a parte do ChildProcess que o abridor usa.
export type SpawnedProcess = {
  once(event: "error", listener: (error: Error) => void): unknown;
  unref(): void;
};

export type SpawnProcess = (
  command: string,
  args: string[],
  options: SpawnOptions,
) => SpawnedProcess;

// browserCommand escolhe o abridor pela plataforma. No Windows a URL vai entre aspas e
// sem reescrita de argumentos, senão o cmd partiria a query no "&"; o "" antes dela é
// o título da janela que o start espera.
export function browserCommand(
  platform: NodeJS.Platform,
  url: string,
): BrowserCommand {
  if (platform === "darwin")
    return { command: "open", args: [url], verbatim: false };
  if (platform === "win32") {
    return {
      command: "cmd",
      args: ["/c", "start", '""', `"${url}"`],
      verbatim: true,
    };
  }
  return { command: "xdg-open", args: [url], verbatim: false };
}

// createBrowserOpener abre a URL sem esperar o navegador e ignora a falha: o login
// também escreve a URL no stderr para quem não tem navegador.
export function createBrowserOpener(
  platform: NodeJS.Platform,
  spawnProcess: SpawnProcess,
): (url: string) => void {
  return (url) => {
    const { command, args, verbatim } = browserCommand(platform, url);
    const child = spawnProcess(command, args, {
      detached: true,
      stdio: "ignore",
      windowsVerbatimArguments: verbatim,
    });
    child.once("error", () => undefined);
    child.unref();
  };
}
