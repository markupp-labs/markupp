import { afterEach, describe, expect, it } from "vitest";
import { FakeMarkuppApi } from "../testing/fake-markupp-api.js";
import { FakeAccountId } from "../testing/fake-vaults.js";
import { MarkuppToolClient } from "../testing/markupp-tool-client.js";

let client: MarkuppToolClient;

afterEach(async () => {
  await client.close();
});

describe("list_notes", () => {
  it("devolve id, path e updated_at de cada nota, em ordem de path", async () => {
    const api = new FakeMarkuppApi().withTenant("t1");
    const second = api.withNote("t1", "b/plano.md", "x");
    const first = api.withNote("t1", "a.md", "y");
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("list_notes", { tenant: "t1" });
    expect(JSON.parse(result.text)).toEqual([
      { id: first.id, path: "a.md", updated_at: first.updated_at },
      { id: second.id, path: "b/plano.md", updated_at: second.updated_at },
    ]);
  });

  it("com vault_id lista só as notas do cofre", async () => {
    const api = new FakeMarkuppApi().withTenant("t1");
    const vault = api.withVault("t1", "Casa");
    api.withNote("t1", "fora.md", "x");
    const inside = api.withNote("t1", "dentro.md", "y", vault.id);
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("list_notes", {
      tenant: "t1",
      vault_id: vault.id,
    });
    expect(JSON.parse(result.text)).toEqual([
      { id: inside.id, path: "dentro.md", updated_at: inside.updated_at },
    ]);
  });

  it("aponta list_vaults quando o cofre não está ao alcance", async () => {
    client = await MarkuppToolClient.connect(
      new FakeMarkuppApi().withTenant("t1"),
    );
    const result = await client.call("list_notes", {
      tenant: "t1",
      vault_id: "v9",
    });
    expect(result.isError).toBe(true);
    expect(result.text).toMatch(/^vault_forbidden: .*list_vaults/);
  });
});

describe("search_notes", () => {
  it("devolve só id, path e updated_at, da mais recente para a mais antiga", async () => {
    const api = new FakeMarkuppApi().withTenant("t1");
    const older = api.withNote("t1", "a.md", "plano de Viagem");
    const newer = api.withNote("t1", "b.md", "outra viagem");
    api.withNote("t1", "c.md", "nada a ver");
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("search_notes", {
      tenant: "t1",
      query: "viagem",
    });
    expect(JSON.parse(result.text)).toEqual([
      { id: newer.id, path: "b.md", updated_at: newer.updated_at },
      { id: older.id, path: "a.md", updated_at: older.updated_at },
    ]);
  });

  it("pagina com limit e offset", async () => {
    const api = new FakeMarkuppApi().withTenant("t1");
    api.withNote("t1", "a.md", "x");
    const middle = api.withNote("t1", "b.md", "x");
    api.withNote("t1", "c.md", "x");
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("search_notes", {
      tenant: "t1",
      query: "x",
      limit: 1,
      offset: 1,
    });
    expect(JSON.parse(result.text)).toEqual([
      { id: middle.id, path: "b.md", updated_at: middle.updated_at },
    ]);
  });

  it("com vault_id busca só no cofre e devolve id e path", async () => {
    const api = new FakeMarkuppApi().withTenant("t1");
    const vault = api.withVault("t1", "Casa");
    api.withNote("t1", "fora.md", "viagem");
    const inside = api.withNote("t1", "dentro.md", "viagem", vault.id);
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("search_notes", {
      tenant: "t1",
      query: "viagem",
      vault_id: vault.id,
    });
    expect(JSON.parse(result.text)).toEqual([
      { id: inside.id, path: "dentro.md" },
    ]);
  });

  it("com vault_id de cofre fora do alcance devolve lista vazia, como a API", async () => {
    const api = new FakeMarkuppApi().withTenant("t1");
    const vault = api.withVault("t1", "Casa");
    api.withNote("t1", "dentro.md", "viagem", vault.id);
    await api.removeVaultMember("t1", vault.id, FakeAccountId);
    client = await MarkuppToolClient.connect(api);
    for (const vault_id of [vault.id, "v9"]) {
      const result = await client.call("search_notes", {
        tenant: "t1",
        query: "viagem",
        vault_id,
      });
      expect(result.isError).toBe(false);
      expect(JSON.parse(result.text)).toEqual([]);
    }
  });

  it("descreve a busca de texto completo por relevância", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    const tool = await client.tool("search_notes");
    expect(tool.description).toMatch(/full-text/);
    expect(tool.description).toMatch(/relevan/);
    expect(tool.description).not.toMatch(/newest first|case-insensitive/);
    expect(tool.inputSchema.properties?.query).toMatchObject({
      description: expect.stringMatching(/words/),
    });
    expect(tool.inputSchema.properties?.query).not.toMatchObject({
      description: expect.stringMatching(/contains/),
    });
  });

  it("aponta list_tenants quando o tenant não é da conta", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    const result = await client.call("search_notes", {
      tenant: "outro",
      query: "x",
    });
    expect(result.isError).toBe(true);
    expect(result.text).toMatch(/^tenant_forbidden: .*Call list_tenants/);
  });
});

describe("read_note", () => {
  it("devolve os metadados e depois o markdown cru", async () => {
    const api = new FakeMarkuppApi().withTenant("t1");
    const note = api.withNote("t1", "a.md", '# Título\n\n"aspas" e \\barra');
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("read_note", {
      tenant: "t1",
      id: note.id,
    });
    const [metadata, content] = result.parts;
    expect(JSON.parse(metadata ?? "")).toEqual({
      id: note.id,
      path: "a.md",
      created_at: note.created_at,
      updated_at: note.updated_at,
    });
    expect(content).toBe(note.content);
  });

  it("aponta search_notes quando a nota não existe", async () => {
    client = await MarkuppToolClient.connect(
      new FakeMarkuppApi().withTenant("t1"),
    );
    const result = await client.call("read_note", {
      tenant: "t1",
      id: "sumiu",
    });
    expect(result.isError).toBe(true);
    expect(result.text).toMatch(/^not_found: .*search_notes/);
  });

  it("anuncia que só lê", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    expect((await client.tool("read_note")).annotations).toMatchObject({
      readOnlyHint: true,
    });
    expect((await client.tool("search_notes")).annotations).toMatchObject({
      readOnlyHint: true,
    });
  });
});
