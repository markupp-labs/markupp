import { describe, expect, it } from "vitest";
import type { HttpConfig } from "./config.js";
import { runCli, type CliDeps } from "./cli.js";
import { FakeClock } from "./testing/fake-clock.js";
import { FakeFetch } from "./testing/fake-fetch.js";
import { FakeFiles } from "./testing/fake-files.js";

const SECRET_PATH = "/run/markupp/mcp-secret";

// RecordingCli guarda o que o CLI escreveria no stderr e a config que mandaria servir,
// com o Go e o volume do segredo trocados por fakes.
class RecordingCli implements CliDeps {
  readonly stderr: string[] = [];
  readonly upstream = new FakeFetch();
  readonly files = new FakeFiles().write(SECRET_PATH, "s".repeat(32));
  private readonly clock = new FakeClock();
  readonly startup = {
    fetch: this.upstream.fetch,
    readFile: this.files.readFile,
    clock: this.clock.now,
    sleep: this.clock.sleep,
  };
  served: HttpConfig | undefined;

  readonly writeError = (line: string): void => {
    this.stderr.push(line);
  };

  readonly serveHttp = async (config: HttpConfig): Promise<void> => {
    this.served = config;
  };
}

describe("runCli", () => {
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

  it("sai com 1 sem subcomando, porque o stdio ainda não tem login", async () => {
    const cli = new RecordingCli();
    expect(await runCli([], {}, cli)).toBe(1);
    expect(cli.stderr.join("\n")).toContain("markupp-mcp http");
  });

  it("sai com 2 e o uso diante de subcomando desconhecido", async () => {
    const cli = new RecordingCli();
    expect(await runCli(["htp"], {}, cli)).toBe(2);
    expect(cli.stderr.join("\n")).toMatch(/subcomando "htp" desconhecido/);
  });
});
