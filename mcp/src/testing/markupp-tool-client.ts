import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { InMemoryTransport } from "@modelcontextprotocol/sdk/inMemory.js";
import { FastMCP, type FastMCPSessionAuth } from "fastmcp";
import type { MarkuppApi } from "../markupp-api.js";
import type { PublicMarkuppApi } from "../public-api.js";
import type { MarkuppSession, PublicSession } from "../session.js";
import { registerMarkuppTools } from "../tools/register-markupp-tools.js";
import { registerPublicTools } from "../tools/register-public-tools.js";
import { FakeApiTokens } from "./fake-markupp-api.js";

type ToolText = { text: string; isError: boolean; parts: string[] };
type ResourceText = { text: string; mimeType?: string; uri: string };
type ListedTool = Awaited<ReturnType<Client["listTools"]>>["tools"][number];

// MarkuppToolClient liga um FastMCP com as ferramentas do markupp a um cliente MCP em
// memória, para o teste passar pelo mesmo caminho do servidor de verdade.
export class MarkuppToolClient<S extends FastMCPSessionAuth = MarkuppSession> {
  private readonly client = new Client({ name: "teste", version: "0.0.0" });

  private constructor(private readonly server: FastMCP<S>) {}

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
    const session: MarkuppSession | undefined = authenticated
      ? { apiToken: new FakeApiTokens("jwt").source }
      : undefined;
    return MarkuppToolClient.linkTo(server, session);
  }

  // connectPublic liga só as ferramentas públicas com a sessão anônima dada; sessions
  // recebe a sessão que cada chamada entregou ao factory.
  static async connectPublic(
    api: PublicMarkuppApi,
    session: PublicSession = {},
    sessions: (PublicSession | undefined)[] = [],
  ): Promise<MarkuppToolClient<PublicSession>> {
    const server = new FastMCP<PublicSession>({
      name: "markupp-publico-teste",
      version: "0.0.0",
    });
    registerPublicTools(server, (received) => {
      sessions.push(received);
      return api;
    });
    return MarkuppToolClient.linkTo(server, session);
  }

  private static async linkTo<T extends FastMCPSessionAuth>(
    server: FastMCP<T>,
    session: T | undefined,
  ): Promise<MarkuppToolClient<T>> {
    const harness = new MarkuppToolClient(server);
    const [clientSide, serverSide] = InMemoryTransport.createLinkedPair();
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

  // toolNames lista os nomes das ferramentas registradas, em ordem alfabética.
  async toolNames(): Promise<string[]> {
    const { tools } = await this.client.listTools();
    return tools.map((tool) => tool.name).sort();
  }

  async close(): Promise<void> {
    await this.client.close();
    await this.server.stop();
  }
}
