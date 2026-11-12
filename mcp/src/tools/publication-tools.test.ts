import { afterEach, describe, expect, it } from "vitest";
import { FakeMarkuppApi } from "../testing/fake-markupp-api.js";
import { MarkuppToolClient } from "../testing/markupp-tool-client.js";

let client: MarkuppToolClient;

afterEach(async () => {
  await client.close();
});

function vaultWithNote(role = "owner") {
  const api = new FakeMarkuppApi().withTenant("t1", role);
  const vault = api.withVault("t1", "Docs");
  const note = api.withNote("t1", "guia/Deploy.md", "x", vault.id);
  return { api, vault, note };
}

describe("publish", () => {
  it("publica a nota confirmada pelo título e devolve a publicação", async () => {
    const { api, vault, note } = vaultWithNote("member");
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("publish", {
      tenant: "t1",
      vault_id: vault.id,
      target: { kind: "note", note_id: note.id },
      confirm: "Deploy",
    });
    const published = JSON.parse(result.text);
    expect(published).toMatchObject({ kind: "note", indexable: false });
    expect(await api.listPublications("t1")).toEqual({
      items: [published],
      total: 1,
    });
  });

  it("repassa indexable e o caminho da pasta", async () => {
    const { api, vault } = vaultWithNote();
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("publish", {
      tenant: "t1",
      vault_id: vault.id,
      target: { kind: "folder", path: "guia" },
      confirm: "guia",
      indexable: true,
    });
    expect(JSON.parse(result.text)).toMatchObject({
      kind: "folder",
      folder_path: "guia",
      indexable: true,
    });
  });

  it("pede o nome exato ao usuário quando a confirmação não bate", async () => {
    const { api, vault } = vaultWithNote();
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("publish", {
      tenant: "t1",
      vault_id: vault.id,
      target: { kind: "vault" },
      confirm: "docs",
    });
    expect(result.isError).toBe(true);
    expect(result.text).toMatch(/^confirmation_mismatch: .*"Docs".*user/);
  });

  it("aponta owner ou admin quando o member publica a pasta", async () => {
    const { api, vault } = vaultWithNote("member");
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("publish", {
      tenant: "t1",
      vault_id: vault.id,
      target: { kind: "folder", path: "guia" },
      confirm: "guia",
    });
    expect(result.text).toMatch(/^publish_forbidden: .*owner or admin/);
  });

  it("recusa a pasta sem path antes de chamar a API", async () => {
    const { api, vault } = vaultWithNote();
    client = await MarkuppToolClient.connect(api);
    const call = client.call("publish", {
      tenant: "t1",
      vault_id: vault.id,
      target: { kind: "folder" },
      confirm: "guia",
    });
    await expect(call).rejects.toThrow(/path/);
    expect(await api.listPublications("t1")).toEqual({ items: [], total: 0 });
  });

  it("diz no confirm que o nome vem do usuário", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    const tool = await client.tool("publish");
    const schema = tool.inputSchema as {
      properties: { confirm: { description: string } };
    };
    expect(schema.properties.confirm.description).toMatch(/user/);
    expect(tool.annotations).toMatchObject({ readOnlyHint: false });
  });
});

describe("list_publications", () => {
  it("lista as publicações do tenant ou só as do cofre", async () => {
    const { api, vault } = vaultWithNote();
    const outro = api.withVault("t1", "Outro");
    api.withNote("t1", "a.md", "y", outro.id);
    const request = { target: { kind: "vault" as const }, indexable: false };
    await api.publish("t1", vault.id, { ...request, confirm: "Docs" });
    await api.publish("t1", outro.id, { ...request, confirm: "Outro" });
    client = await MarkuppToolClient.connect(api);
    const all = await client.call("list_publications", { tenant: "t1" });
    const only = await client.call("list_publications", {
      tenant: "t1",
      vault_id: outro.id,
    });
    expect(JSON.parse(all.text).total).toBe(2);
    expect(JSON.parse(only.text).items).toMatchObject([
      { vault_name: "Outro" },
    ]);
  });

  it("anuncia que só lê", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    expect((await client.tool("list_publications")).annotations).toMatchObject({
      readOnlyHint: true,
    });
  });
});

describe("unpublish", () => {
  it("despublica e devolve o id despublicado", async () => {
    const { api, vault } = vaultWithNote();
    const published = await api.publish("t1", vault.id, {
      target: { kind: "vault" },
      confirm: "Docs",
      indexable: false,
    });
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("unpublish", {
      tenant: "t1",
      publication_id: published.id,
    });
    expect(JSON.parse(result.text)).toEqual({ unpublished: published.id });
    expect((await api.listPublications("t1")).total).toBe(0);
  });

  it("aponta list_publications quando a publicação não existe", async () => {
    client = await MarkuppToolClient.connect(vaultWithNote().api);
    const result = await client.call("unpublish", {
      tenant: "t1",
      publication_id: "sumiu",
    });
    expect(result.text).toMatch(/^publication_not_found: .*list_publications/);
  });
});
