import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StreamableHTTPClientTransport } from "@modelcontextprotocol/sdk/client/streamableHttp.js";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { markuppUrl } from "./markupp-stack.js";
import { loginByPassword } from "./panel-consent.js";
import { callToolJson, callToolTexts } from "./tool-text.js";

// O e2e publica um cofre pela API, como o owner faria no painel, e lê pelo MCP público
// sem login nenhum: o cliente não tem authProvider.
const marker = `publico-${Date.now()}`;
const slug = `e2e-${Date.now()}`;
const publicMcpUrl = new URL(`${markuppUrl()}/mcp/public`);
const client = new Client({ name: "markupp-e2e-public", version: "0.0.0" });

type PublishedNote = { id: string; vaultId: string };
type PublicNoteItem = { id: string; title: string; path: string };

// OwnerApi chama a API como a conta do e2e, no tenant dela.
class OwnerApi {
  private constructor(
    private readonly token: string,
    private readonly tenant: string,
  ) {}

  static async login(): Promise<OwnerApi> {
    const token = await loginByPassword(markuppUrl());
    const tenants = await call<{ id: string }[]>("GET", "/v1/me/tenants", {
      authorization: `Bearer ${token}`,
    });
    const tenant = tenants[0]?.id;
    if (!tenant) throw new Error("conta sem tenant, esperado ao menos um");
    return new OwnerApi(token, tenant);
  }

  async createNote(path: string, content: string): Promise<PublishedNote> {
    const body = { vault_id: "", path, content };
    const note = await this.send<{ id: string; vault_id: string }>(
      "POST",
      "/v1/notes",
      body,
    );
    return { id: note.id, vaultId: note.vault_id };
  }

  publish(vaultId: string): Promise<unknown> {
    return this.send("PUT", publicationPath(vaultId), { slug });
  }

  unpublish(vaultId: string): Promise<unknown> {
    return this.send("DELETE", publicationPath(vaultId));
  }

  private send<T>(method: string, path: string, body?: unknown): Promise<T> {
    const headers = {
      authorization: `Bearer ${this.token}`,
      "x-markupp-tenant": this.tenant,
    };
    return call<T>(method, path, headers, body);
  }
}

function publicationPath(vaultId: string): string {
  return `/v1/vaults/${encodeURIComponent(vaultId)}/publication`;
}

async function call<T>(
  method: string,
  path: string,
  headers: Record<string, string>,
  body?: unknown,
): Promise<T> {
  const init: RequestInit = { method, headers: { ...headers } };
  if (body !== undefined) {
    init.body = JSON.stringify(body);
    init.headers = { ...headers, "content-type": "application/json" };
  }
  const response = await fetch(`${markuppUrl()}${path}`, init);
  const text = await response.text();
  if (!response.ok)
    throw new Error(
      `${method} ${path} respondeu ${response.status} ${text}, esperado 2xx`,
    );
  return (text === "" ? undefined : JSON.parse(text)) as T;
}

let owner: OwnerApi;
let note: PublishedNote;

beforeAll(async () => {
  owner = await OwnerApi.login();
  note = await owner.createNote(
    `publico/${marker}.md`,
    `# Nota publica\n\nconteudo com ${marker}\n`,
  );
  await owner.publish(note.vaultId);
  await client.connect(new StreamableHTTPClientTransport(publicMcpUrl));
});

afterAll(() => client.close());

// SEARCH_INDEX_WAIT cobre o intervalo de 30s do indexador do compose.
const SEARCH_INDEX_WAIT = { timeout: 60_000, interval: 1_000 };

async function searchIds(query: string): Promise<string[]> {
  const found = await callToolJson<PublicNoteItem[]>(
    client,
    "search_public_notes",
    { vault: slug, query },
  );
  return found.map((item) => item.id);
}

describe("transporte HTTP público, sem login", () => {
  it("lista só as ferramentas de leitura dos cofres publicados", async () => {
    const { tools } = await client.listTools();
    expect(tools.map((tool) => tool.name).sort()).toEqual([
      "list_public_notes",
      "read_public_note",
      "search_public_notes",
    ]);
  });

  it("lista, lê e busca a nota do cofre publicado", async () => {
    const listed = await callToolJson<PublicNoteItem[]>(
      client,
      "list_public_notes",
      { vault: slug },
    );
    expect(listed.map((item) => item.id)).toContain(note.id);
    const [, content] = await callToolTexts(client, "read_public_note", {
      vault: slug,
      id: note.id,
    });
    expect(content).toBe(`# Nota publica\n\nconteudo com ${marker}\n`);
    await expect
      .poll(() => searchIds(marker), SEARCH_INDEX_WAIT)
      .toEqual([note.id]);
  }, 120_000);

  it("responde not_published depois que o cofre é despublicado", async () => {
    await owner.unpublish(note.vaultId);
    await expect(
      callToolTexts(client, "read_public_note", { vault: slug, id: note.id }),
    ).rejects.toThrow(/not_published/);
  });
});
