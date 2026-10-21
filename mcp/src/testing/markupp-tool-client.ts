import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { InMemoryTransport } from "@modelcontextprotocol/sdk/inMemory.js";
import { FastMCP } from "fastmcp";
import type { MarkuppApi } from "../markupp-api.js";
import type { MarkuppSession } from "../session.js";
import { registerMarkuppTools } from "../tools/register-markupp-tools.js";
import { FakeApiTokens } from "./fake-markupp-api.js";

type ToolText = { text: string; isError: boolean; parts: string[] };
type ResourceText = { text: string; mimeType?: string; uri: string };
type ListedTool = Awaited<ReturnType<Client["listTools"]>>["tools"][number];

// MarkuppToolClient liga um FastMCP com as ferramentas do markupp a um cliente MCP em
// memória, para o teste passar pelo mesmo caminho do servidor de verdade.
export class MarkuppToolClient {
  private readonly client = new Client({ name: "teste", version: "0.0.0" });

  private constructor(private readonly server: FastMCP<MarkuppSession>) {}

  // connect sem authenticated liga a sessão sem login, como o FastMCP sem authenticate.
  static async connect(
    api: MarkuppApi,
    authenticated = true,
  ): Promise<MarkuppToolClient> {
    const server = new FastMCP<MarkuppSession>({
      name: "markupp-teste",
      version: "0.0.0",
    });
    registerMarkuppTools(server, () => api);
    const harness = new MarkuppToolClient(server);
    const [clientSide, serverSide] = InMemoryTransport.createLinkedPair();
    const session: MarkuppSession | undefined = authenticated
      ? { apiToken: new FakeApiTokens("jwt").source }
      : undefined;
    await Promise.all([
      server.connect(serverSide, session),
      harness.client.connect(clientSide),
    ]);
    return harness;
  }

  async call(name: string, args: Record<string, unknown>): Promise<ToolText> {
    const result = await this.client.callTool({ name, arguments: args });
    const content = result.content as { type: string; text?: string }[];
    const parts = content.map((part) => part.text ?? "");
    return { text: parts.join("\n"), isError: result.isError === true, parts };
  }

  // read lê o resource uri; a falha chega como a rejeição do cliente MCP.
  async read(uri: string): Promise<ResourceText> {
    const { contents } = await this.client.readResource({ uri });
    const [first] = contents;
    if (!first || !("text" in first))
      throw new Error(
        `resource ${uri} sem conteúdo de texto: ${JSON.stringify(contents)}`,
      );
    return { text: first.text, mimeType: first.mimeType, uri: first.uri };
  }

  async tool(name: string): Promise<ListedTool> {
    const { tools } = await this.client.listTools();
    const found = tools.find((tool) => tool.name === name);
    if (!found)
      throw new Error(
        `ferramenta ${name} não registrada; há ${tools.map((t) => t.name).join(", ")}`,
      );
    return found;
  }

  async close(): Promise<void> {
    await this.client.close();
    await this.server.stop();
  }
}
