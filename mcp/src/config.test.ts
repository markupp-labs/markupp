import { describe, expect, it } from "vitest";
import { loadHttpConfig, type McpEnvironment } from "./config.js";

const SECRET = "s".repeat(32);

function environment(overrides: McpEnvironment = {}): McpEnvironment {
  return {
    MARKUPP_MCP_API_URL: "http://markupp:8080",
    MARKUPP_MCP_ISSUER_URL: "https://markupp.exemplo",
    MARKUPP_MCP_RESOURCE_URL: "https://mcp.exemplo/mcp",
    MARKUPP_MCP_RESOURCE_SECRET: SECRET,
    ...overrides,
  };
}

describe("loadHttpConfig", () => {
  it("lê as variáveis e usa a porta 3000 quando ela falta", () => {
    expect(loadHttpConfig(environment())).toEqual({
      apiUrl: "http://markupp:8080",
      issuerUrl: "https://markupp.exemplo",
      resourceUrl: "https://mcp.exemplo/mcp",
      resourceSecret: SECRET,
      endpoint: "/mcp",
      port: 3000,
    });
  });

  it("lê a porta dada", () => {
    const config = loadHttpConfig(environment({ MARKUPP_MCP_PORT: "8443" }));
    expect(config.port).toBe(8443);
  });

  it.each(["0", "65536", "80a", "-1"])("recusa a porta %s", (port) => {
    expect(() =>
      loadHttpConfig(environment({ MARKUPP_MCP_PORT: port })),
    ).toThrow(`MARKUPP_MCP_PORT="${port}", esperado inteiro de 1 a 65535`);
  });

  it.each([
    "MARKUPP_MCP_API_URL",
    "MARKUPP_MCP_ISSUER_URL",
    "MARKUPP_MCP_RESOURCE_URL",
    "MARKUPP_MCP_RESOURCE_SECRET",
  ])("exige %s", (key) => {
    expect(() => loadHttpConfig(environment({ [key]: "" }))).toThrow(
      `${key} vazia`,
    );
  });

  it("aceita http em qualquer host na API, que é endereço interno", () => {
    const config = loadHttpConfig(
      environment({ MARKUPP_MCP_API_URL: "http://markupp.svc.cluster.local" }),
    );
    expect(config.apiUrl).toBe("http://markupp.svc.cluster.local");
  });

  it("recusa a API fora de http ou https", () => {
    expect(() =>
      loadHttpConfig(environment({ MARKUPP_MCP_API_URL: "ftp://markupp" })),
    ).toThrow('MARKUPP_MCP_API_URL="ftp://markupp", esperado http(s)://host');
  });

  it.each(["MARKUPP_MCP_ISSUER_URL", "MARKUPP_MCP_RESOURCE_URL"])(
    "recusa http fora de loopback em %s",
    (key) => {
      expect(() =>
        loadHttpConfig(environment({ [key]: "http://exemplo/mcp" })),
      ).toThrow(`${key}="http://exemplo/mcp", esperado https://`);
    },
  );

  it("aceita http em loopback no issuer e no resource", () => {
    const config = loadHttpConfig(
      environment({
        MARKUPP_MCP_ISSUER_URL: "http://localhost:8080",
        MARKUPP_MCP_RESOURCE_URL: "http://127.0.0.1:3000/mcp",
      }),
    );
    expect(config.resourceUrl).toBe("http://127.0.0.1:3000/mcp");
  });

  it("recusa o issuer com caminho, como o Go", () => {
    expect(() =>
      loadHttpConfig(
        environment({ MARKUPP_MCP_ISSUER_URL: "https://markupp.exemplo/x" }),
      ),
    ).toThrow("sem caminho");
  });

  it("exige caminho no resource, que vira a rota do endpoint", () => {
    expect(() =>
      loadHttpConfig(
        environment({ MARKUPP_MCP_RESOURCE_URL: "https://mcp.exemplo" }),
      ),
    ).toThrow(
      'MARKUPP_MCP_RESOURCE_URL="https://mcp.exemplo", esperado https://host[:porta]/caminho',
    );
  });

  it("recusa query no resource", () => {
    expect(() =>
      loadHttpConfig(
        environment({
          MARKUPP_MCP_RESOURCE_URL: "https://mcp.exemplo/mcp?a=1",
        }),
      ),
    ).toThrow("MARKUPP_MCP_RESOURCE_URL");
  });

  it("recusa segredo curto citando o tamanho e nunca o valor", () => {
    const attempt = () =>
      loadHttpConfig(
        environment({ MARKUPP_MCP_RESOURCE_SECRET: "segredo-curto" }),
      );
    expect(attempt).toThrow(
      "MARKUPP_MCP_RESOURCE_SECRET com 13 bytes, esperado ao menos 32 bytes aleatorios",
    );
    expect(attempt).not.toThrow("segredo-curto");
  });
});
