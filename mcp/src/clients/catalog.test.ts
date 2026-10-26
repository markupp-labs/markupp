import { describe, expect, it } from "vitest";
import { agentClientsFor, type AgentClient } from "./catalog.js";

const INSTANCE = "https://notas.exemplo.com.br/";
const MCP_URL = "https://notas.exemplo.com.br/mcp";

function clientNamed(id: string): AgentClient {
  const client = agentClientsFor(INSTANCE).find((each) => each.id === id);
  if (client === undefined) throw new Error(`cliente ${id} ausente`);
  return client;
}

function snippetText(clientId: string, snippetId: string): string {
  const snippet = clientNamed(clientId).snippets.find(
    (each) => each.id === snippetId,
  );
  if (snippet === undefined)
    throw new Error(`snippet ${clientId}/${snippetId} ausente`);
  return snippet.text;
}

function deeplinkHref(clientId: string): string {
  const [deeplink] = clientNamed(clientId).deeplinks;
  if (deeplink === undefined) throw new Error(`deeplink de ${clientId} ausente`);
  return deeplink.href;
}

describe("agentClientsFor", () => {
  it("lista os clientes na ordem das abas", () => {
    expect(agentClientsFor(INSTANCE).map((client) => client.id)).toEqual([
      "claude-code",
      "codex",
      "cursor",
      "vscode",
      "claude-desktop",
      "gemini-cli",
      "windsurf",
      "stdio",
    ]);
  });

  it("registra no Claude Code pelo transporte http", () => {
    expect(snippetText("claude-code", "cli")).toBe(
      `claude mcp add --transport http markupp ${MCP_URL}`,
    );
  });

  it("registra no Codex pela url e entra pelo mcp login", () => {
    expect(snippetText("codex", "cli")).toBe(
      `codex mcp add markupp --url ${MCP_URL}`,
    );
    expect(snippetText("codex", "login")).toBe("codex mcp login markupp");
    expect(snippetText("codex", "config")).toBe(
      `[mcp_servers.markupp]\nurl = "${MCP_URL}"`,
    );
  });

  it("monta o deeplink do Cursor com a config em base64", () => {
    const href = deeplinkHref("cursor");
    const prefix =
      "cursor://anysphere.cursor-deeplink/mcp/install?name=markupp&config=";
    expect(href.startsWith(prefix)).toBe(true);
    const config = decodeURIComponent(href.slice(prefix.length));
    expect(JSON.parse(Buffer.from(config, "base64").toString("utf8"))).toEqual({
      url: MCP_URL,
    });
    expect(JSON.parse(snippetText("cursor", "config"))).toEqual({
      mcpServers: { markupp: { url: MCP_URL } },
    });
  });

  it("monta o deeplink do VS Code com o json na url", () => {
    const href = deeplinkHref("vscode");
    expect(href.startsWith("vscode:mcp/install?")).toBe(true);
    const payload = decodeURIComponent(
      href.slice("vscode:mcp/install?".length),
    );
    expect(JSON.parse(payload)).toEqual({
      name: "markupp",
      type: "http",
      url: MCP_URL,
    });
    expect(JSON.parse(snippetText("vscode", "config"))).toEqual({
      servers: { markupp: { type: "http", url: MCP_URL } },
    });
  });

  it("dá ao Claude Desktop só a url do conector personalizado", () => {
    expect(snippetText("claude-desktop", "url")).toBe(MCP_URL);
  });

  it("usa httpUrl no Gemini CLI e serverUrl no Windsurf", () => {
    expect(snippetText("gemini-cli", "cli")).toBe(
      `gemini mcp add --transport http markupp ${MCP_URL}`,
    );
    expect(JSON.parse(snippetText("gemini-cli", "config"))).toEqual({
      mcpServers: { markupp: { httpUrl: MCP_URL } },
    });
    expect(JSON.parse(snippetText("windsurf", "config"))).toEqual({
      mcpServers: { markupp: { serverUrl: MCP_URL } },
    });
  });

  it("entra no stdio pela url da instância, sem o /mcp", () => {
    expect(snippetText("stdio", "login")).toBe(
      "npx -y markupp-mcp login https://notas.exemplo.com.br",
    );
    expect(JSON.parse(snippetText("stdio", "config"))).toEqual({
      mcpServers: { markupp: { command: "npx", args: ["-y", "markupp-mcp"] } },
    });
  });

  it("mantém o caminho de uma instância fora da raiz", () => {
    const [claudeCode] = agentClientsFor("http://localhost:8080/markupp//");
    expect(claudeCode?.snippets[0]?.text).toBe(
      "claude mcp add --transport http markupp http://localhost:8080/markupp/mcp",
    );
  });

  it("recusa url que não é http citando o valor", () => {
    expect(() => agentClientsFor("ftp://notas.exemplo.com.br")).toThrow(
      'url da instancia "ftp://notas.exemplo.com.br"',
    );
    expect(() => agentClientsFor("notas")).toThrow("esperado http(s)://host");
  });
});
