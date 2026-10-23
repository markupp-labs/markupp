import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StreamableHTTPClientTransport } from "@modelcontextprotocol/sdk/client/streamableHttp.js";
import type { FastMCP } from "fastmcp";
import { createServer } from "node:net";
import { afterEach, describe, expect, it } from "vitest";
import type { HttpConfig } from "./config.js";
import { createHttpServer, startHttpServer } from "./http-server.js";
import type { MarkuppSession } from "./session.js";
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
  private server: FastMCP<MarkuppSession> | undefined;
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
    this.server = createHttpServer(this.config, deps);
    await startHttpServer(this.server, this.config);
    return this;
  }

  url(path: string): string {
    return `http://127.0.0.1:${this.config.port}${path}`;
  }

  async stop(): Promise<void> {
    await this.server?.stop();
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
