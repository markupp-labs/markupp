import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StreamableHTTPClientTransport } from "@modelcontextprotocol/sdk/client/streamableHttp.js";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { markuppUrl } from "./markupp-stack.js";
import { loginByPassword } from "./panel-consent.js";
import { callToolJson, callToolTexts } from "./tool-text.js";

// O e2e publica uma nota pela API, como o painel faria, e lê pelo MCP público sem
// login nenhum: o cliente não tem authProvider.
const marker = `publico-${Date.now()}`;
const publicMcpUrl = new URL(`${markuppUrl()}/mcp/public`);
const client = new Client({ name: "markupp-e2e-public", version: "0.0.0" });

type CreatedNote = { id: string; vault_id: string };
type Publication = { id: string; public_id: string };
type PublicPage = { items: { public_id: string }[]; total: number };

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

  createNote(path: string, content: string): Promise<CreatedNote> {
    return this.send("POST", "/v1/notes", { vault_id: "", path, content });
  }

  // publishNote confirma pelo nome que a prévia devolve, como o painel pede
  // (ADR-0048): a nota em /v1/notes não traz o título.
  async publishNote(note: CreatedNote): Promise<Publication> {
    const path = `/v1/vaults/${encodeURIComponent(note.vault_id)}/publications`;
    const query = new URLSearchParams({ kind: "note", note_id: note.id });
    const preview = await this.send<{ target_name: string }>(
      "GET",
      `${path}/preview?${query}`,
    );
    const target = { kind: "note", note_id: note.id };
    return this.send("POST", path, { target, confirm: preview.target_name });
  }

  unpublish(publication: Publication): Promise<unknown> {
    const path = `/v1/publications/${encodeURIComponent(publication.id)}`;
    return this.send("DELETE", path);
  }

  private send<T>(method: string, path: string, body?: unknown): Promise<T> {
    const headers = {
      authorization: `Bearer ${this.token}`,
      "x-markupp-tenant": this.tenant,
    };
    return call<T>(method, path, headers, body);
  }
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
let publication: Publication;

beforeAll(async () => {
  owner = await OwnerApi.login();
  const note = await owner.createNote(
    `publico/${marker}.md`,
    `# Nota publica\n\nconteudo com ${marker}\n`,
  );
  publication = await owner.publishNote(note);
  await client.connect(new StreamableHTTPClientTransport(publicMcpUrl));
});

afterAll(() => client.close());

// SEARCH_INDEX_WAIT cobre o intervalo de 30s do indexador do compose.
const SEARCH_INDEX_WAIT = { timeout: 60_000, interval: 1_000 };

async function publicIds(name: string, query?: string): Promise<string[]> {
  const args = { public_id: publication.public_id, ...(query && { query }) };
  const page = await callToolJson<PublicPage>(client, name, args);
  return page.items.map((item) => item.public_id);
}

describe("transporte HTTP público, sem login", () => {
  it("lista só as ferramentas de leitura das notas públicas", async () => {
    const { tools } = await client.listTools();
    expect(tools.map((tool) => tool.name).sort()).toEqual([
      "list_public_notes",
      "read_public_note",
      "search_public_notes",
    ]);
  });

  it("lê, lista e busca a nota publicada pelo id público", async () => {
    const [, content] = await callToolTexts(client, "read_public_note", {
      public_id: publication.public_id,
    });
    expect(content).toBe(`# Nota publica\n\nconteudo com ${marker}\n`);
    expect(await publicIds("list_public_notes")).toEqual([
      publication.public_id,
    ]);
    await expect
      .poll(() => publicIds("search_public_notes", marker), SEARCH_INDEX_WAIT)
      .toEqual([publication.public_id]);
  }, 120_000);

  it("responde not_found depois que a nota é despublicada", async () => {
    await owner.unpublish(publication);
    await expect(
      callToolTexts(client, "read_public_note", {
        public_id: publication.public_id,
      }),
    ).rejects.toThrow(/not_found/);
  });
});
