import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { InMemoryTransport } from "@modelcontextprotocol/sdk/inMemory.js";
import { describe, expect, it } from "vitest";
import { TEST_ISSUER } from "../testing/auth-server-metadata.js";
import { FakeFetch } from "../testing/fake-fetch.js";
import { FakeApiTokens } from "../testing/fake-markupp-api.js";
import { createStdioServer, jsonLogger } from "./stdio-server.js";

describe("createStdioServer", () => {
  it("chama a API em {issuer}/v1 com o JWT da sessão", async () => {
    const upstream = new FakeFetch().respondWith(200, []);
    const apiToken = new FakeApiTokens("jwt-stdio").source;
    const server = createStdioServer(
      { issuer: TEST_ISSUER, apiToken },
      { fetch: upstream.fetch, writeLog: () => undefined },
    );
    const client = new Client({ name: "teste", version: "0.0.0" });
    const [clientSide, serverSide] = InMemoryTransport.createLinkedPair();
    // connect recebe a sessão que o authenticate do stdio devolveria.
    await Promise.all([
      server.connect(serverSide, { apiToken }),
      client.connect(clientSide),
    ]);
    await client.callTool({ name: "list_tenants", arguments: {} });
    const request = upstream.requests[0];
    expect(request?.url).toBe(`${TEST_ISSUER}/v1/me/tenants`);
    expect(request?.headers.get("authorization")).toBe("Bearer jwt-stdio");
    await client.close();
    await server.stop();
  });
});

describe("jsonLogger", () => {
  it("escreve cada entrada como uma linha JSON", () => {
    const lines: string[] = [];
    jsonLogger((line) => void lines.push(line)).info("[FastMCP info]", "x");
    expect(lines).toEqual(['{"level":"info","msg":"[FastMCP info] x"}']);
  });
});
