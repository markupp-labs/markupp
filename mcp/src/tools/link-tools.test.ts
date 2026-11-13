import { afterEach, describe, expect, it } from "vitest";
import { FakeMarkuppApi } from "../testing/fake-markupp-api.js";
import { MarkuppToolClient } from "../testing/markupp-tool-client.js";

let client: MarkuppToolClient;

afterEach(async () => {
  await client.close();
});

type MentionVault = {
  api: FakeMarkuppApi;
  vault: string;
  a: string;
  b: string;
};

// vaultWithMention monta um cofre em que a.md cita o título de b.md, o que dá a ligação
// mentioned de a para b, e b.md escreve [[a]], a ligação explicit de b para a.
function vaultWithMention(role = "member"): MentionVault {
  const api = new FakeMarkuppApi().withTenant("t1", role);
  const vault = api.withVault("t1", "Casa").id;
  const a = api.withNote("t1", "a.md", "fala de b", vault);
  const b = api.withNote("t1", "b.md", "[[a]]", vault);
  api.withLink(a, b, "mentioned").withLink(b, a, "explicit");
  return { api, vault, a: a.id, b: b.id };
}

function linkTypes(text: string): string[] {
  return (JSON.parse(text) as { type: string }[]).map((link) => link.type);
}

describe("list_links", () => {
  it("devolve as ligações que a nota faz quando não há direção", async () => {
    const { api, a, b } = vaultWithMention();
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("list_links", { tenant: "t1", id: a });
    const [link] = JSON.parse(result.text) as {
      source: { id: string };
      target: { id: string };
      type: string;
    }[];
    expect(link).toMatchObject({
      source: { id: a },
      target: { id: b },
      type: "mentioned",
    });
  });

  it("devolve as ligações que chegam na nota com direction incoming", async () => {
    const { api, a } = vaultWithMention();
    client = await MarkuppToolClient.connect(api);
    const args = { tenant: "t1", id: a, direction: "incoming" };
    const result = await client.call("list_links", args);
    expect(linkTypes(result.text)).toEqual(["explicit"]);
  });

  it("filtra pelo tipo", async () => {
    const { api, a } = vaultWithMention();
    client = await MarkuppToolClient.connect(api);
    const args = { tenant: "t1", id: a, type: "explicit" };
    const result = await client.call("list_links", args);
    expect(JSON.parse(result.text)).toEqual([]);
  });

  it("filtra pela ligação related, a por texto parecido", async () => {
    const api = new FakeMarkuppApi().withTenant("t1", "member");
    const vault = api.withVault("t1", "Casa").id;
    const a = api.withNote("t1", "a.md", "kubernetes no cluster", vault);
    const b = api.withNote("t1", "b.md", "cluster de kubernetes", vault);
    api.withLink(a, b, "related").withLink(a, b, "explicit");
    client = await MarkuppToolClient.connect(api);
    const args = { tenant: "t1", id: a.id, type: "related" };
    const result = await client.call("list_links", args);
    expect(linkTypes(result.text)).toEqual(["related"]);
  });

  it("descreve a ligação related como automática", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    const tool = await client.tool("list_links");
    expect(tool.description).toMatch(/related is an automatic link/);
  });

  it("aponta search_notes quando a nota não existe", async () => {
    const { api } = vaultWithMention();
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("list_links", { tenant: "t1", id: "n9" });
    expect(result.text).toMatch(/^not_found: .*search_notes/);
  });

  it("descreve a ligação mentioned como automática e cita refuse_link", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    const tool = await client.tool("list_links");
    expect(tool.description).toMatch(/mentioned/);
    expect(tool.description).toMatch(/automatic/);
    expect(tool.description).toMatch(/refuse_link/);
    expect(tool.annotations).toMatchObject({ readOnlyHint: true });
  });
});

describe("refuse_link", () => {
  it("tira a ligação mentioned do par e deixa a escrita", async () => {
    const { api, vault, a, b } = vaultWithMention();
    client = await MarkuppToolClient.connect(api);
    const args = { tenant: "t1", vault_id: vault, note_ids: [b, a] };
    const result = await client.call("refuse_link", args);
    const refusal = JSON.parse(result.text) as { notes: { id: string }[] };
    expect(refusal.notes.map((note) => note.id)).toEqual([a, b]);
    const outgoing = await client.call("list_links", { tenant: "t1", id: a });
    expect(JSON.parse(outgoing.text)).toEqual([]);
    const incoming = { tenant: "t1", id: a, direction: "incoming" };
    expect(linkTypes((await client.call("list_links", incoming)).text)).toEqual(
      ["explicit"],
    );
  });

  it("devolve a mesma recusa ao recusar o par de novo", async () => {
    const { api, vault, a, b } = vaultWithMention();
    client = await MarkuppToolClient.connect(api);
    const args = { tenant: "t1", vault_id: vault, note_ids: [a, b] };
    const first = await client.call("refuse_link", args);
    const again = await client.call("refuse_link", args);
    expect(JSON.parse(again.text)).toEqual(JSON.parse(first.text));
  });

  it.each([
    ["viewer", "b", /^read_only: /],
    ["member", "a", /^same_note: .*two different notes/],
    ["member", "fora", /^note_outside_vault: .*list_notes/],
  ])("recusa como %s com a outra nota %s", async (role, other, error) => {
    const scene = vaultWithMention(role);
    const fora = scene.api.withNote("t1", "fora.md", "x").id;
    const ids: Record<string, string> = { a: scene.a, b: scene.b, fora };
    client = await MarkuppToolClient.connect(scene.api);
    const note_ids = [scene.a, ids[other]];
    const args = { tenant: "t1", vault_id: scene.vault, note_ids };
    const result = await client.call("refuse_link", args);
    expect(result.isError).toBe(true);
    expect(result.text).toMatch(error);
  });

  it("descreve que só a ligação automática sai e anuncia que é idempotente", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    const tool = await client.tool("refuse_link");
    expect(tool.description).toMatch(/mentioned/);
    expect(tool.description).toMatch(/written links/);
    expect(tool.annotations).toMatchObject({
      readOnlyHint: false,
      destructiveHint: false,
      idempotentHint: true,
    });
  });
});

describe("list_link_refusals", () => {
  it("lista as recusas do cofre", async () => {
    const { api, vault, a, b } = vaultWithMention();
    const refusal = await api.refuseLink("t1", vault, [a, b]);
    client = await MarkuppToolClient.connect(api);
    const args = { tenant: "t1", vault_id: vault, limit: 5 };
    const result = await client.call("list_link_refusals", args);
    expect(JSON.parse(result.text)).toEqual([refusal]);
  });

  it("anuncia que só lê", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    expect((await client.tool("list_link_refusals")).annotations).toMatchObject(
      { readOnlyHint: true },
    );
  });
});

describe("undo_link_refusal", () => {
  it("desfaz a recusa e a ligação mentioned volta", async () => {
    const { api, vault, a, b } = vaultWithMention();
    const refusal = await api.refuseLink("t1", vault, [a, b]);
    client = await MarkuppToolClient.connect(api);
    const args = { tenant: "t1", vault_id: vault, refusal_id: refusal.id };
    const result = await client.call("undo_link_refusal", args);
    expect(JSON.parse(result.text)).toEqual({ undone: refusal.id });
    expect(await api.listLinkRefusals("t1", vault)).toEqual([]);
    const outgoing = await client.call("list_links", { tenant: "t1", id: a });
    expect(linkTypes(outgoing.text)).toEqual(["mentioned"]);
  });

  it("aponta list_link_refusals quando a recusa não existe", async () => {
    const { api, vault } = vaultWithMention();
    client = await MarkuppToolClient.connect(api);
    const args = { tenant: "t1", vault_id: vault, refusal_id: "r9" };
    const result = await client.call("undo_link_refusal", args);
    expect(result.text).toMatch(
      /^link_refusal_not_found: .*list_link_refusals/,
    );
  });

  it("anuncia que não é destrutiva e é idempotente", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    expect((await client.tool("undo_link_refusal")).annotations).toMatchObject({
      destructiveHint: false,
      idempotentHint: true,
    });
  });
});
