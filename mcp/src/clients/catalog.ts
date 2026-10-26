import type { AgentClient, AgentClientTemplate } from "./catalog-types.js";
import type { JsonValue } from "./link-encoding.js";
import {
  PLACEHOLDERS,
  instanceUrlsFor,
  renderAgentClient,
} from "./template-rendering.js";

export type { AgentClient, AgentClientTemplate } from "./catalog-types.js";

// Catálogo único dos clientes MCP: o painel (settings/agents) e o site das docs mostram
// estes trechos. A sintaxe de cada um segue a documentação oficial do cliente em docsUrl.
const MCP = PLACEHOLDERS.mcpUrl;
const SERVER = "markupp";

function jsonSnippet(value: JsonValue): string {
  return JSON.stringify(value, null, 2);
}

const STDIO_SERVER = { command: "npx", args: ["-y", "markupp-mcp"] };

export const AGENT_CLIENT_TEMPLATES: AgentClientTemplate[] = [
  {
    id: "claude-code",
    name: "Claude Code",
    docsUrl: "https://code.claude.com/docs/en/mcp",
    snippets: [
      {
        id: "cli",
        language: "shell",
        template: `claude mcp add --transport http ${SERVER} ${MCP}`,
      },
    ],
    deeplinks: [],
  },
  {
    id: "codex",
    name: "Codex",
    docsUrl: "https://developers.openai.com/codex/mcp",
    snippets: [
      {
        id: "cli",
        language: "shell",
        template: `codex mcp add ${SERVER} --url ${MCP}`,
      },
      { id: "login", language: "shell", template: `codex mcp login ${SERVER}` },
      {
        id: "config",
        language: "toml",
        path: "~/.codex/config.toml",
        template: `[mcp_servers.${SERVER}]\nurl = "${MCP}"`,
      },
    ],
    deeplinks: [],
  },
  {
    id: "cursor",
    name: "Cursor",
    docsUrl: "https://cursor.com/docs/context/mcp",
    snippets: [
      {
        id: "config",
        language: "json",
        path: "~/.cursor/mcp.json",
        template: jsonSnippet({ mcpServers: { [SERVER]: { url: MCP } } }),
      },
    ],
    deeplinks: [
      {
        id: "install",
        base: `cursor://anysphere.cursor-deeplink/mcp/install?name=${SERVER}&config=`,
        payload: { url: MCP },
        encoding: "base64-json",
      },
    ],
  },
  {
    id: "vscode",
    name: "VS Code",
    docsUrl:
      "https://code.visualstudio.com/docs/copilot/customization/mcp-servers",
    snippets: [
      {
        id: "cli",
        language: "shell",
        template: `code --add-mcp '${JSON.stringify({ name: SERVER, type: "http", url: MCP })}'`,
      },
      {
        id: "config",
        language: "json",
        path: ".vscode/mcp.json",
        template: jsonSnippet({
          servers: { [SERVER]: { type: "http", url: MCP } },
        }),
      },
    ],
    deeplinks: [
      {
        id: "install",
        base: "vscode:mcp/install?",
        payload: { name: SERVER, type: "http", url: MCP },
        encoding: "uri-json",
      },
    ],
  },
  {
    id: "claude-desktop",
    name: "Claude Desktop",
    docsUrl:
      "https://support.claude.com/en/articles/11175166-getting-started-with-custom-connectors-using-remote-mcp",
    snippets: [{ id: "url", language: "text", template: MCP }],
    deeplinks: [],
  },
  {
    id: "gemini-cli",
    name: "Gemini CLI",
    docsUrl: "https://geminicli.com/docs/tools/mcp-server/",
    snippets: [
      {
        id: "cli",
        language: "shell",
        template: `gemini mcp add --transport http ${SERVER} ${MCP}`,
      },
      {
        id: "config",
        language: "json",
        path: "~/.gemini/settings.json",
        template: jsonSnippet({ mcpServers: { [SERVER]: { httpUrl: MCP } } }),
      },
    ],
    deeplinks: [],
  },
  {
    id: "windsurf",
    name: "Windsurf",
    docsUrl: "https://docs.windsurf.com/windsurf/cascade/mcp",
    snippets: [
      {
        id: "config",
        language: "json",
        path: "~/.codeium/windsurf/mcp_config.json",
        template: jsonSnippet({ mcpServers: { [SERVER]: { serverUrl: MCP } } }),
      },
    ],
    deeplinks: [],
  },
  {
    id: "stdio",
    name: "stdio (npx)",
    docsUrl: "https://www.npmjs.com/package/markupp-mcp",
    snippets: [
      {
        id: "login",
        language: "shell",
        template: `npx -y markupp-mcp login ${PLACEHOLDERS.instanceUrl}`,
      },
      {
        id: "config",
        language: "json",
        template: jsonSnippet({ mcpServers: { [SERVER]: STDIO_SERVER } }),
      },
    ],
    deeplinks: [],
  },
];

// agentClientsFor monta o catálogo para a url pública da instância (a do Go, sem /mcp).
export function agentClientsFor(instanceUrl: string): AgentClient[] {
  const urls = instanceUrlsFor(instanceUrl);
  return AGENT_CLIENT_TEMPLATES.map((client) =>
    renderAgentClient(client, urls),
  );
}
