import { describe, expect, it } from "vitest";
import { loadHttpEnv, type McpEnvironment } from "./config.js";

const SECRET = "s".repeat(32);

function environment(overrides: McpEnvironment = {}): McpEnvironment {
  return { MARKUPP_MCP_RESOURCE_SECRET: SECRET, ...overrides };
}

describe("loadHttpEnv", () => {
  it("usa a API em localhost:8080 e a porta 3000 quando elas faltam", () => {
    expect(loadHttpEnv(environment())).toEqual({
      apiUrl: "http://localhost:8080",
      resourceUrl: undefined,
      secret: { kind: "value", value: SECRET },
      port: 3000,
    });
  });

  it("lê a porta dada", () => {
    const config = loadHttpEnv(environment({ MARKUPP_MCP_PORT: "8443" }));
    expect(config.port).toBe(8443);
  });

  it.each(["0", "65536", "80a", "-1"])("recusa a porta %s", (port) => {
    expect(() => loadHttpEnv(environment({ MARKUPP_MCP_PORT: port }))).toThrow(
      `MARKUPP_MCP_PORT="${port}", esperado inteiro de 1 a 65535`,
    );
  });

  it("aceita http em qualquer host na API, que é endereço interno", () => {
    const config = loadHttpEnv(
      environment({ MARKUPP_MCP_API_URL: "http://markupp.svc.cluster.local/" }),
    );
    expect(config.apiUrl).toBe("http://markupp.svc.cluster.local");
  });

  it("recusa a API fora de http ou https", () => {
    expect(() =>
      loadHttpEnv(environment({ MARKUPP_MCP_API_URL: "ftp://markupp" })),
    ).toThrow('MARKUPP_MCP_API_URL="ftp://markupp", esperado http(s)://host');
  });

  it("lê o resource dado", () => {
    const config = loadHttpEnv(
      environment({ MARKUPP_MCP_RESOURCE_URL: "https://mcp.exemplo/mcp" }),
    );
    expect(config.resourceUrl).toBe("https://mcp.exemplo/mcp");
  });

  it("recusa http fora de loopback no resource", () => {
    expect(() =>
      loadHttpEnv(
        environment({ MARKUPP_MCP_RESOURCE_URL: "http://exemplo/mcp" }),
      ),
    ).toThrow(
      'MARKUPP_MCP_RESOURCE_URL="http://exemplo/mcp", esperado https://',
    );
  });

  it("aceita http em loopback no resource", () => {
    const config = loadHttpEnv(
      environment({ MARKUPP_MCP_RESOURCE_URL: "http://127.0.0.1:3000/mcp" }),
    );
    expect(config.resourceUrl).toBe("http://127.0.0.1:3000/mcp");
  });

  it("exige caminho no resource, que vira a rota do endpoint", () => {
    expect(() =>
      loadHttpEnv(
        environment({ MARKUPP_MCP_RESOURCE_URL: "https://mcp.exemplo" }),
      ),
    ).toThrow(
      'MARKUPP_MCP_RESOURCE_URL="https://mcp.exemplo", esperado https://host[:porta]/caminho',
    );
  });

  it("recusa query no resource", () => {
    expect(() =>
      loadHttpEnv(
        environment({
          MARKUPP_MCP_RESOURCE_URL: "https://mcp.exemplo/mcp?a=1",
        }),
      ),
    ).toThrow("MARKUPP_MCP_RESOURCE_URL");
  });

  it("recusa segredo curto citando o tamanho e nunca o valor", () => {
    const attempt = () =>
      loadHttpEnv(
        environment({ MARKUPP_MCP_RESOURCE_SECRET: "segredo-curto" }),
      );
    expect(attempt).toThrow(
      "MARKUPP_MCP_RESOURCE_SECRET com 13 bytes, esperado ao menos 32 bytes aleatorios",
    );
    expect(attempt).not.toThrow("segredo-curto");
  });

  it("lê o caminho do segredo quando falta o segredo na variável", () => {
    const config = loadHttpEnv({ MARKUPP_MCP_SECRET_PATH: "/run/s" });
    expect(config.secret).toEqual({ kind: "file", path: "/run/s" });
  });

  it("prefere o segredo na variável ao caminho do arquivo", () => {
    const config = loadHttpEnv(
      environment({ MARKUPP_MCP_SECRET_PATH: "/run/s" }),
    );
    expect(config.secret).toEqual({ kind: "value", value: SECRET });
  });

  it("sem segredo nem caminho, cita as duas formas de fornecer", () => {
    expect(() => loadHttpEnv({})).toThrow(
      /MARKUPP_MCP_RESOURCE_SECRET e MARKUPP_MCP_SECRET_PATH vazias/,
    );
  });
});
