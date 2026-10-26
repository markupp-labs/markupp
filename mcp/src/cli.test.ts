import { describe, expect, it } from "vitest";
import type { HttpConfig } from "./config.js";
import { runCli, USAGE, type CliDeps } from "./cli.js";
import type { LocalDeps } from "./stdio/commands.js";
import { CredentialStore } from "./stdio/credential-store.js";
import type { StdioConfig } from "./stdio/stdio-server.js";
import {
  authServerMetadata,
  TEST_ISSUER,
} from "./testing/auth-server-metadata.js";
import {
  FakeBrowser,
  FakeLoopbackCallback,
  type BrowserOutcome,
} from "./testing/fake-browser-login.js";
import { FakeClock } from "./testing/fake-clock.js";
import { FakeFetch } from "./testing/fake-fetch.js";
import { FakeFiles } from "./testing/fake-files.js";
import { FakeLocalFiles } from "./testing/fake-local-files.js";

const SECRET_PATH = "/run/markupp/mcp-secret";
const CREDENTIALS = "/home/ana/.config/markupp/credentials.json";

// RecordingCli guarda o que o CLI escreveria no stderr e o que mandaria servir, com o
// Go, o volume do segredo, o disco da pessoa e o navegador trocados por fakes.
class RecordingCli implements CliDeps {
  readonly stderr: string[] = [];
  readonly stdout: string[] = [];
  readonly version = "1.2.3";
  readonly upstream = new FakeFetch();
  readonly files = new FakeFiles().write(SECRET_PATH, "s".repeat(32));
  private readonly clock = new FakeClock();
  readonly localFiles = new FakeLocalFiles(this.clock.now);
  readonly callback = new FakeLoopbackCallback();
  readonly browser: FakeBrowser;
  readonly startup = {
    fetch: this.upstream.fetch,
    readFile: this.files.readFile,
    clock: this.clock.now,
    sleep: this.clock.sleep,
  };
  readonly local: LocalDeps;
  served: HttpConfig | undefined;
  servedStdio: StdioConfig | undefined;

  constructor(outcome: BrowserOutcome = { kind: "abandon" }) {
    this.browser = new FakeBrowser(this.callback, outcome);
    this.local = {
      files: this.localFiles,
      homeDir: "/home/ana",
      listenForCallback: this.callback.listen,
      openBrowser: this.browser.open,
      timeout: new FakeClock().sleep,
    };
  }

  readonly writeError = (line: string): void => {
    this.stderr.push(line);
  };

  readonly writeOutput = (line: string): void => {
    this.stdout.push(line);
  };

  readonly serveHttp = async (config: HttpConfig): Promise<void> => {
    this.served = config;
  };

  readonly serveStdio = async (config: StdioConfig): Promise<void> => {
    this.servedStdio = config;
  };

  store(path: string = CREDENTIALS): CredentialStore {
    return new CredentialStore(path, this.localFiles);
  }

  async seedCredential(): Promise<this> {
    await this.store().save({
      issuer: TEST_ISSUER,
      refreshToken: "r1",
      accessToken: "jwt-guardado",
      expiresAtMs: this.clock.now() + 300_000,
    });
    return this;
  }
}

describe("runCli http", () => {
  it("sobe o transporte http só com o caminho do segredo", async () => {
    const cli = new RecordingCli();
    cli.upstream.respondWith(200, { issuer: "http://localhost:8080" });
    const env = { MARKUPP_MCP_SECRET_PATH: SECRET_PATH };
    expect(await runCli(["http"], env, cli)).toBe(0);
    expect(cli.served?.resourceUrl).toBe("http://localhost:8080/mcp");
    expect(cli.stderr).toEqual([]);
  });

  it("sai com 1 e a mensagem da variável inválida, sem subir nada", async () => {
    const cli = new RecordingCli();
    const env = { MARKUPP_MCP_SECRET_PATH: SECRET_PATH, MARKUPP_MCP_PORT: "x" };
    expect(await runCli(["http"], env, cli)).toBe(1);
    expect(cli.stderr.join("\n")).toContain('MARKUPP_MCP_PORT="x"');
    expect(cli.served).toBeUndefined();
  });

  it("sai com 1 quando o Go não responde a metadata no prazo", async () => {
    const cli = new RecordingCli();
    const env = { MARKUPP_MCP_SECRET_PATH: SECRET_PATH };
    expect(await runCli(["http"], env, cli)).toBe(1);
    expect(cli.stderr.join("\n")).toContain(
      "http://localhost:8080/.well-known/oauth-authorization-server",
    );
    expect(cli.served).toBeUndefined();
  });
});

describe("runCli stdio", () => {
  it("sai com 1 sem login guardado, pedindo markupp-mcp login", async () => {
    const cli = new RecordingCli();
    expect(await runCli([], {}, cli)).toBe(1);
    expect(cli.stderr.join("\n")).toContain("rode markupp-mcp login [url]");
    expect(cli.servedStdio).toBeUndefined();
  });

  it("serve o stdio com o issuer e o JWT do login guardado", async () => {
    const cli = await new RecordingCli().seedCredential();
    expect(await runCli([], {}, cli)).toBe(0);
    expect(cli.servedStdio?.issuer).toBe(TEST_ISSUER);
    expect(await cli.servedStdio?.apiToken()).toBe("jwt-guardado");
    expect(cli.stderr).toEqual([]);
  });

  it("lê a credencial do XDG_CONFIG_HOME", async () => {
    const cli = new RecordingCli();
    expect(await runCli([], { XDG_CONFIG_HOME: "/xdg" }, cli)).toBe(1);
    expect(cli.stderr.join("\n")).toContain("/xdg/markupp/credentials.json");
  });
});

describe("runCli login e logout", () => {
  it("faz o login no localhost:8080 por padrão e grava a credencial", async () => {
    const cli = new RecordingCli({ kind: "consent", code: "c1" });
    cli.upstream.respondWith(200, authServerMetadata()).respondWith(200, {
      access_token: "jwt",
      expires_in: 300,
      refresh_token: "r1",
    });
    expect(await runCli(["login"], {}, cli)).toBe(0);
    expect((await cli.store().load())?.refreshToken).toBe("r1");
    expect(cli.stderr.at(-1)).toBe(`login feito em ${TEST_ISSUER}`);
  });

  it("sai com 1 quando a pessoa recusa no painel", async () => {
    const cli = new RecordingCli({ kind: "deny", error: "access_denied" });
    cli.upstream.respondWith(200, authServerMetadata());
    expect(await runCli(["login", TEST_ISSUER], {}, cli)).toBe(1);
    expect(cli.stderr.at(-1)).toContain("error access_denied");
  });

  it("revoga e apaga no logout", async () => {
    const cli = await new RecordingCli().seedCredential();
    cli.upstream.respondWith(200, authServerMetadata()).respondWith(200);
    expect(await runCli(["logout"], {}, cli)).toBe(0);
    expect(await cli.store().load()).toBeUndefined();
    expect(cli.stderr).toEqual([`login revogado e apagado de ${CREDENTIALS}`]);
  });

  it("avisa e sai com 0 no logout sem login guardado", async () => {
    const cli = new RecordingCli();
    expect(await runCli(["logout"], {}, cli)).toBe(0);
    expect(cli.stderr.join("\n")).toContain("nada a revogar");
  });
});

describe("runCli uso", () => {
  it("sai com 2 e o uso diante de subcomando desconhecido", async () => {
    const cli = new RecordingCli();
    expect(await runCli(["htp"], {}, cli)).toBe(2);
    expect(cli.stderr.join("\n")).toMatch(/subcomando "htp" desconhecido/);
    expect(cli.stderr.join("\n")).toContain(USAGE);
  });

  it("sai com 2 diante de argumentos demais", async () => {
    const cli = new RecordingCli();
    expect(await runCli(["logout", "x"], {}, cli)).toBe(2);
    expect(cli.stderr.join("\n")).toContain("argumentos demais para logout: x");
  });

  it("não confunde método do objeto com subcomando", async () => {
    const cli = new RecordingCli();
    expect(await runCli(["toString"], {}, cli)).toBe(2);
  });
});

describe("runCli ajuda e versão", () => {
  it.each(["--help", "-h", "help"])(
    "%s imprime o uso no stdout e sai com 0",
    async (flag) => {
      const cli = new RecordingCli();
      expect(await runCli([flag], {}, cli)).toBe(0);
      expect(cli.stdout).toEqual([USAGE]);
      expect(cli.stderr).toEqual([]);
      expect(cli.servedStdio).toBeUndefined();
    },
  );

  it.each(["--version", "-v"])(
    "%s imprime a versão no stdout e sai com 0",
    async (flag) => {
      const cli = new RecordingCli();
      expect(await runCli([flag], {}, cli)).toBe(0);
      expect(cli.stdout).toEqual(["1.2.3"]);
      expect(cli.stderr).toEqual([]);
    },
  );

  it("o uso lista as opções de ajuda e versão", () => {
    expect(USAGE).toContain("--help");
    expect(USAGE).toContain("--version");
  });
});
