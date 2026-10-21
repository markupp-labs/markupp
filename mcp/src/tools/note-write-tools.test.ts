import { afterEach, describe, expect, it } from "vitest";
import { FakeMarkuppApi } from "../testing/fake-markupp-api.js";
import { MarkuppToolClient } from "../testing/markupp-tool-client.js";

let client: MarkuppToolClient;

afterEach(async () => {
  await client.close();
});

describe("create_note", () => {
  it("cria a nota e devolve id, path e updated_at", async () => {
    const api = new FakeMarkuppApi().withTenant("t1");
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("create_note", {
      tenant: "t1",
      path: "a.md",
      content: "# A",
    });
    const created = JSON.parse(result.text) as { id: string };
    expect(api.stored("t1", created.id)).toMatchObject({
      path: "a.md",
      content: "# A",
    });
    expect(Object.keys(created)).toEqual(["id", "path", "updated_at"]);
  });

  it("manda achar e editar a nota quando o path já existe", async () => {
    const api = new FakeMarkuppApi().withTenant("t1");
    api.withNote("t1", "a.md", "x");
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("create_note", {
      tenant: "t1",
      path: "a.md",
      content: "y",
    });
    expect(result.isError).toBe(true);
    expect(result.text).toMatch(/^duplicate_path: .*list_notes.*edit_note/);
  });

  it("aponta os papéis que escrevem quando o viewer tenta escrever", async () => {
    client = await MarkuppToolClient.connect(
      new FakeMarkuppApi().withTenant("t1", "viewer"),
    );
    const result = await client.call("create_note", {
      tenant: "t1",
      path: "a.md",
      content: "y",
    });
    expect(result.isError).toBe(true);
    expect(result.text).toMatch(/^read_only: .*role owner, admin or member/);
  });

  it("anuncia escrita que só acrescenta e não é idempotente", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    expect((await client.tool("create_note")).annotations).toMatchObject({
      readOnlyHint: false,
      destructiveHint: false,
      idempotentHint: false,
    });
  });
});

describe("edit_note", () => {
  async function editing(
    content: string,
  ): Promise<{ api: FakeMarkuppApi; id: string }> {
    const api = new FakeMarkuppApi().withTenant("t1");
    const note = api.withNote("t1", "a.md", content);
    client = await MarkuppToolClient.connect(api);
    return { api, id: note.id };
  }

  it("troca o único trecho e manda o PUT com o updated_at lido", async () => {
    const { api, id } = await editing("um dois três");
    const before = api.stored("t1", id).updated_at;
    const result = await client.call("edit_note", {
      tenant: "t1",
      id,
      old_text: "dois",
      new_text: "2",
    });
    expect(result.isError).toBe(false);
    expect(api.revisions).toEqual([
      { path: "a.md", content: "um 2 três", lastModifiedAt: before },
    ]);
  });

  it("recusa trecho ausente pedindo para reler e copiar", async () => {
    const { api, id } = await editing("um dois três");
    const result = await client.call("edit_note", {
      tenant: "t1",
      id,
      old_text: "quatro",
      new_text: "4",
    });
    expect(result.isError).toBe(true);
    expect(result.text).toMatch(
      /^old_text_not_found: old_text occurs 0 times.*read_note/,
    );
    expect(api.revisions).toEqual([]);
  });

  it("recusa trecho repetido dizendo quantas vezes ocorre e pedindo trecho maior", async () => {
    const { api, id } = await editing("a b a c a");
    const result = await client.call("edit_note", {
      tenant: "t1",
      id,
      old_text: "a",
      new_text: "z",
    });
    expect(result.isError).toBe(true);
    expect(result.text).toMatch(
      /^old_text_ambiguous: old_text occurs 3 times.*longer old_text/,
    );
    expect(api.revisions).toEqual([]);
  });

  it("pede para reler e tentar de novo quando outra escrita entra no meio", async () => {
    const { api, id } = await editing("um dois");
    api.concurrentWriteOnNextRead(id, "um dois, de outro cliente");
    const result = await client.call("edit_note", {
      tenant: "t1",
      id,
      old_text: "dois",
      new_text: "2",
    });
    expect(result.isError).toBe(true);
    expect(result.text).toMatch(
      /^conflict: .*Call read_note again and retry edit_note/,
    );
    expect(api.stored("t1", id).content).toBe("um dois, de outro cliente");
  });

  it("anuncia escrita destrutiva e não idempotente", async () => {
    await editing("x");
    expect((await client.tool("edit_note")).annotations).toMatchObject({
      readOnlyHint: false,
      destructiveHint: true,
      idempotentHint: false,
    });
  });
});

describe("delete_note", () => {
  it("apaga a nota", async () => {
    const api = new FakeMarkuppApi().withTenant("t1");
    const note = api.withNote("t1", "a.md", "x");
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("delete_note", {
      tenant: "t1",
      id: note.id,
    });
    expect(JSON.parse(result.text)).toEqual({ deleted: note.id });
    expect(() => api.stored("t1", note.id)).toThrow("nota não encontrada");
  });

  it("anuncia escrita destrutiva e idempotente", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    expect((await client.tool("delete_note")).annotations).toMatchObject({
      destructiveHint: true,
      idempotentHint: true,
    });
  });
});
