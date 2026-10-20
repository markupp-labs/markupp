import { describe, expect, it } from "vitest";
import type { HttpConfig } from "./config.js";
import { runCli, type CliDeps } from "./cli.js";

const VALID_ENV = {
  MARKUPP_MCP_API_URL: "http://markupp:8080",
  MARKUPP_MCP_ISSUER_URL: "http://localhost:8080",
  MARKUPP_MCP_RESOURCE_URL: "http://localhost:3000/mcp",
  MARKUPP_MCP_RESOURCE_SECRET: "s".repeat(32),
};

// RecordingCli guarda o que o CLI escreveria no stderr e a config que mandaria servir.
class RecordingCli implements CliDeps {
  readonly stderr: string[] = [];
  served: HttpConfig | undefined;

  readonly writeError = (line: string): void => {
    this.stderr.push(line);
  };

  readonly serveHttp = async (config: HttpConfig): Promise<void> => {
    this.served = config;
  };
}

describe("runCli", () => {
  it("sobe o transporte http com a config lida do ambiente", async () => {
    const cli = new RecordingCli();
    expect(await runCli(["http"], VALID_ENV, cli)).toBe(0);
    expect(cli.served?.endpoint).toBe("/mcp");
    expect(cli.stderr).toEqual([]);
  });

  it("sai com 1 e a mensagem da variável inválida, sem subir nada", async () => {
    const cli = new RecordingCli();
    const env = { ...VALID_ENV, MARKUPP_MCP_PORT: "x" };
    expect(await runCli(["http"], env, cli)).toBe(1);
    expect(cli.stderr.join("\n")).toContain('MARKUPP_MCP_PORT="x"');
    expect(cli.served).toBeUndefined();
  });

  it("sai com 1 sem subcomando, porque o stdio ainda não tem login", async () => {
    const cli = new RecordingCli();
    expect(await runCli([], VALID_ENV, cli)).toBe(1);
    expect(cli.stderr.join("\n")).toContain("markupp-mcp http");
  });

  it("sai com 2 e o uso diante de subcomando desconhecido", async () => {
    const cli = new RecordingCli();
    expect(await runCli(["htp"], VALID_ENV, cli)).toBe(2);
    expect(cli.stderr.join("\n")).toMatch(/subcomando "htp" desconhecido/);
  });
});
