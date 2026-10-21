import { describe, expect, it } from "vitest";
import type { HttpEnvConfig } from "./config.js";
import { errorMessage, resolveHttpConfig } from "./resolve-http-config.js";
import { FakeClock } from "./testing/fake-clock.js";
import { FakeFetch } from "./testing/fake-fetch.js";
import { FakeFiles } from "./testing/fake-files.js";

const SECRET = "s".repeat(32);
const METADATA_URL =
  "http://markupp:8080/.well-known/oauth-authorization-server";

// StartingGo é o Go visto pelo servidor MCP na subida: a metadata pelo FakeFetch, o
// volume do segredo pelo FakeFiles e o tempo pelo FakeClock.
class StartingGo {
  readonly upstream = new FakeFetch();
  readonly files = new FakeFiles();
  readonly clock = new FakeClock();
  readonly deps = {
    fetch: this.upstream.fetch,
    readFile: this.files.readFile,
    clock: this.clock.now,
    sleep: this.clock.sleep,
  };

  publishIssuer(issuer: unknown): this {
    this.upstream.respondWith(200, { issuer });
    return this;
  }
}

function envConfig(overrides: Partial<HttpEnvConfig> = {}): HttpEnvConfig {
  return {
    apiUrl: "http://markupp:8080",
    resourceUrl: undefined,
    secret: { kind: "value", value: SECRET },
    port: 3000,
    ...overrides,
  };
}

describe("resolveHttpConfig", () => {
  it("descobre o issuer e usa issuer/mcp como resource", async () => {
    const go = new StartingGo().publishIssuer("https://markupp.exemplo");
    expect(await resolveHttpConfig(envConfig(), go.deps)).toEqual({
      apiUrl: "http://markupp:8080",
      issuerUrl: "https://markupp.exemplo",
      resourceUrl: "https://markupp.exemplo/mcp",
      resourceSecret: SECRET,
      endpoint: "/mcp",
      port: 3000,
    });
    expect(go.upstream.requests[0]?.url).toBe(METADATA_URL);
  });

  it("usa o resource dado e tira dele o endpoint", async () => {
    const go = new StartingGo().publishIssuer("https://markupp.exemplo");
    const resourceUrl = "https://mcp.exemplo/agentes";
    const config = await resolveHttpConfig(envConfig({ resourceUrl }), go.deps);
    expect(config.resourceUrl).toBe(resourceUrl);
    expect(config.endpoint).toBe("/agentes");
  });

  it("tenta de novo enquanto o Go sobe", async () => {
    const go = new StartingGo();
    go.upstream.respondWith(503).respondWith(502);
    go.publishIssuer("http://localhost:8080");
    const config = await resolveHttpConfig(envConfig(), go.deps);
    expect(config.issuerUrl).toBe("http://localhost:8080");
    expect(go.upstream.requests).toHaveLength(3);
  });

  it("vencido o prazo, falha com a URL consultada e o último erro", async () => {
    const go = new StartingGo();
    const attempt = resolveHttpConfig(envConfig(), go.deps);
    await expect(attempt).rejects.toThrow(
      `metadata do Go em ${METADATA_URL} indisponivel apos 60s, ultimo erro: FakeFetch sem resposta`,
    );
    expect(go.upstream.requests).toHaveLength(61);
  });

  it("recusa o issuer que o agente não poderia usar", async () => {
    const go = new StartingGo().publishIssuer("http://markupp:8080");
    await expect(resolveHttpConfig(envConfig(), go.deps)).rejects.toThrow(
      `issuer em ${METADATA_URL}="http://markupp:8080", esperado https://`,
    );
  });

  it("recusa o issuer com caminho, como o Go", async () => {
    const go = new StartingGo().publishIssuer("https://markupp.exemplo/x");
    await expect(resolveHttpConfig(envConfig(), go.deps)).rejects.toThrow(
      "sem caminho",
    );
  });

  it("lê o segredo do arquivo sem os espaços das pontas", async () => {
    const go = new StartingGo().publishIssuer("https://markupp.exemplo");
    go.files.write("/run/s", `  ${SECRET}\n`);
    const secret = { kind: "file", path: "/run/s" } as const;
    const config = await resolveHttpConfig(envConfig({ secret }), go.deps);
    expect(config.resourceSecret).toBe(SECRET);
  });

  it("espera o Go escrever o arquivo do segredo", async () => {
    const go = new StartingGo().publishIssuer("https://markupp.exemplo");
    go.files.write("/run/s", SECRET, 2);
    const secret = { kind: "file", path: "/run/s" } as const;
    const config = await resolveHttpConfig(envConfig({ secret }), go.deps);
    expect(config.resourceSecret).toBe(SECRET);
    expect(go.files.reads).toHaveLength(3);
  });

  it("trata o arquivo vazio como ainda não escrito até o prazo", async () => {
    const go = new StartingGo().publishIssuer("https://markupp.exemplo");
    go.files.write("/run/s", "\n");
    const secret = { kind: "file", path: "/run/s" } as const;
    await expect(
      resolveHttpConfig(envConfig({ secret }), go.deps),
    ).rejects.toThrow(
      "segredo do resource em /run/s indisponivel apos 60s, ultimo erro: arquivo vazio",
    );
  });

  it("recusa segredo curto no arquivo citando o caminho e nunca o valor", async () => {
    const go = new StartingGo().publishIssuer("https://markupp.exemplo");
    go.files.write("/run/s", "segredo-curto");
    const secret = { kind: "file", path: "/run/s" } as const;
    const attempt = resolveHttpConfig(envConfig({ secret }), go.deps);
    await expect(attempt).rejects.toThrow(
      "segredo em /run/s com 13 bytes, esperado ao menos 32 bytes aleatorios",
    );
    await expect(attempt).rejects.not.toThrow("segredo-curto");
  });
});

describe("errorMessage", () => {
  it("junta a causa, onde o fetch do Node põe o ECONNREFUSED", () => {
    const error = new TypeError("fetch failed", {
      cause: new Error("connect ECONNREFUSED 127.0.0.1:8080"),
    });
    expect(errorMessage(error)).toBe(
      "fetch failed: connect ECONNREFUSED 127.0.0.1:8080",
    );
  });

  it("converte em texto o que não é Error", () => {
    expect(errorMessage("falhou")).toBe("falhou");
  });
});
