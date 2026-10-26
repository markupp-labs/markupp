import { createServer, type Server } from "node:http";
import { afterEach, describe, expect, it } from "vitest";
import { relayToLoopback } from "./loopback-relay.js";

// EchoServer faz o papel do servidor público no loopback: responde com o método, o
// caminho, o corpo e os headers que recebeu.
class EchoServer {
  private server: Server | undefined;
  port = 0;

  async start(): Promise<this> {
    this.server = createServer((req, res) => {
      const chunks: Buffer[] = [];
      req.on("data", (chunk: Buffer) => chunks.push(chunk));
      req.on("end", () => {
        res.setHeader("x-eco", `${req.method} ${req.url}`);
        res.setHeader("x-host", req.headers.host ?? "");
        res.setHeader("x-agente", req.headers["x-forwarded-for"] ?? "");
        res.writeHead(202).end(Buffer.concat(chunks));
      });
    });
    await new Promise<void>((resolve) =>
      this.server?.listen(0, "127.0.0.1", resolve),
    );
    const address = this.server.address();
    this.port = typeof address === "object" && address ? address.port : 0;
    return this;
  }

  async stop(): Promise<void> {
    await new Promise((resolve) => this.server?.close(resolve));
  }
}

let echo: EchoServer;

afterEach(async () => {
  await echo.stop();
});

describe("relayToLoopback", () => {
  it("repassa método, caminho, query, corpo e headers ao loopback", async () => {
    echo = await new EchoServer().start();
    const request = new Request("http://markupp.exemplo/mcp/public?x=1", {
      method: "POST",
      headers: { "x-forwarded-for": "203.0.113.7" },
      body: '{"jsonrpc":"2.0"}',
    });
    const response = await relayToLoopback(request, echo.port);
    expect(response.status).toBe(202);
    expect(response.headers.get("x-eco")).toBe("POST /mcp/public?x=1");
    expect(response.headers.get("x-host")).toBe(`127.0.0.1:${echo.port}`);
    expect(response.headers.get("x-agente")).toBe("203.0.113.7");
    expect(await response.text()).toBe('{"jsonrpc":"2.0"}');
  });

  it("não manda corpo no GET", async () => {
    echo = await new EchoServer().start();
    const request = new Request("http://markupp.exemplo/mcp/public");
    const response = await relayToLoopback(request, echo.port);
    expect(response.headers.get("x-eco")).toBe("GET /mcp/public");
    expect(await response.text()).toBe("");
  });
});
