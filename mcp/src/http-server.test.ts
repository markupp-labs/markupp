import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StreamableHTTPClientTransport } from "@modelcontextprotocol/sdk/client/streamableHttp.js";
import { createServer } from "node:net";
import { afterEach, describe, expect, it } from "vitest";
import type { HttpConfig } from "./config.js";
import {
  createHttpServer,
  startHttpServer,
  type HttpServerStop,
} from "./http-server.js";
import { FakeClock } from "./testing/fake-clock.js";
import { FakeFetch } from "./testing/fake-fetch.js";
import { RecordingAuthLog } from "./testing/recording-auth-log.js";

// freePort pede ao sistema uma porta livre para o servidor do teste.
function freePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const probe = createServer().listen(0, "127.0.0.1", () => {
      const address = probe.address();
      probe.close(() =>
        typeof address === "object" && address
          ? resolve(address.port)
          : reject(new Error(`endereço inesperado: ${String(address)}`)),
      );
    });
  });
}

// RunningHttpServer é o servidor HTTP montado como em produção, com o Go trocado pelo
// FakeFetch, ouvindo numa porta livre.
class RunningHttpServer {
  readonly upstream = new FakeFetch();
  readonly log = new RecordingAuthLog();
  private stopServer: HttpServerStop | undefined;
  config!: HttpConfig;

  async start(): Promise<this> {
    const port = await freePort();
    this.config = {
      apiUrl: "http://markupp.interno:8080",
      issuerUrl: "https://markupp.exemplo",
      resourceUrl: `http://127.0.0.1:${port}/mcp`,
      resourceSecret: "s".repeat(32),
      endpoint: "/mcp",
      port,
    };
    const deps = {
      fetch: this.upstream.fetch,
      clock: new FakeClock().now,
      log: this.log.write,
    };
    this.stopServer = await startHttpServer(
      createHttpServer(this.config, deps),
      this.config,
    );
    return this;
  }

  url(path: string): string {
    return `http://127.0.0.1:${this.config.port}${path}`;
  }

  async stop(): Promise<void> {
    await this.stopServer?.();
  }
}

let running: RunningHttpServer;

afterEach(async () => {
  await running.stop();
});

describe("servidor HTTP", () => {
  it("publica a metadata do recurso protegido apontando o issuer", async () => {
    running = await new RunningHttpServer().start();
    const response = await fetch(
      running.url("/.well-known/oauth-protected-resource/mcp"),
    );
    expect(response.status).toBe(200);
    expect(await response.json()).toMatchObject({
      resource: running.config.resourceUrl,
      authorization_servers: ["https://markupp.exemplo"],
    });
  });

  it("atende por IPv6, como nos pods de um cluster de pilha dupla", async () => {
    running = await new RunningHttpServer().start();
    const response = await fetch(
      `http://[::1]:${running.config.port}/.well-known/oauth-protected-resource/mcp`,
    );
    expect(response.status).toBe(200);
  });

  it("responde 401 com o desafio Bearer quando falta o token", async () => {
    running = await new RunningHttpServer().start();
    const response = await fetch(running.url("/mcp"), {
      method: "POST",
      headers: {
        "content-type": "application/json",
        accept: "application/json, text/event-stream",
      },
      body: JSON.stringify({ jsonrpc: "2.0", id: 1, method: "ping" }),
    });
    expect(response.status).toBe(401);
    expect(response.headers.get("www-authenticate")).toContain(
      "/.well-known/oauth-protected-resource/mcp",
    );
  });

  it("troca o token do agente no Go interno e chama a API com o JWT", async () => {
    running = await new RunningHttpServer().start();
    running.upstream
      .respondWith(200, { access_token: "jwt-api", expires_in: 300 })
      .respondWith(200, [{ id: "t1", name: "Casa", role: "owner" }]);
    const client = new Client({ name: "teste", version: "0.0.0" });
    const transport = new StreamableHTTPClientTransport(
      new URL(running.url("/mcp")),
      { requestInit: { headers: { authorization: "Bearer token-do-agente" } } },
    );
    await client.connect(transport);
    const result = await client.callTool({
      name: "list_tenants",
      arguments: {},
    });
    await client.close();
    const [exchange, tenants] = running.upstream.requests;
    expect(exchange?.url).toBe("http://markupp.interno:8080/oauth/token");
    expect(tenants?.url).toBe("http://markupp.interno:8080/v1/me/tenants");
    expect(tenants?.headers.get("authorization")).toBe("Bearer jwt-api");
    expect(JSON.stringify(result.content)).toContain("Casa");
  });
});

describe("servidor HTTP em /mcp/public", () => {
  async function publicClient(headers: Record<string, string> = {}) {
    const client = new Client({ name: "teste", version: "0.0.0" });
    const transport = new StreamableHTTPClientTransport(
      new URL(running.url("/mcp/public")),
      { requestInit: { headers } },
    );
    await client.connect(transport);
    return client;
  }

  it("atende sem token e lista só as ferramentas públicas", async () => {
    running = await new RunningHttpServer().start();
    const client = await publicClient();
    const { tools } = await client.listTools();
    await client.close();
    expect(tools.map((tool) => tool.name).sort()).toEqual([
      "list_public_notes",
      "read_public_note",
      "search_public_notes",
    ]);
  });

  it("chama a leitura pública do Go sem Authorization, com o X-Forwarded-For do agente", async () => {
    running = await new RunningHttpServer().start();
    running.upstream.respondWith(200, []);
    const client = await publicClient({ "x-forwarded-for": "203.0.113.7" });
    const result = await client.callTool({
      name: "list_public_notes",
      arguments: { vault: "docs" },
    });
    await client.close();
    const [list] = running.upstream.requests;
    expect(result.isError).toBeFalsy();
    expect(list?.url).toBe(
      "http://markupp.interno:8080/v1/public/vaults/docs/notes",
    );
    expect(list?.headers.get("authorization")).toBeNull();
    expect(list?.headers.get("x-forwarded-for")).toBe("203.0.113.7");
  });

  it("mantém o desafio OAuth no endpoint autenticado", async () => {
    running = await new RunningHttpServer().start();
    const response = await fetch(running.url("/mcp"), {
      method: "POST",
      headers: {
        "content-type": "application/json",
        accept: "application/json, text/event-stream",
      },
      body: JSON.stringify({ jsonrpc: "2.0", id: 1, method: "ping" }),
    });
    expect(response.status).toBe(401);
  });
});
