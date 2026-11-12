import { afterEach, describe, expect, it } from "vitest";
import type { PublicSession } from "../session.js";
import { FakePublicMarkuppApi } from "../testing/fake-public-markupp-api.js";
import { MarkuppToolClient } from "../testing/markupp-tool-client.js";

let client: MarkuppToolClient<PublicSession>;

afterEach(async () => {
  await client.close();
});

const UNKNOWN_ID = "naoexiste123";

describe("registro público", () => {
  it("expõe só as ferramentas de leitura das notas públicas", async () => {
    client = await MarkuppToolClient.connectPublic(new FakePublicMarkuppApi());
    expect(await client.toolNames()).toEqual([
      "list_public_notes",
      "read_public_note",
      "search_public_notes",
    ]);
  });

  it("anuncia que todas só leem", async () => {
    client = await MarkuppToolClient.connectPublic(new FakePublicMarkuppApi());
    for (const name of await client.toolNames()) {
      expect((await client.tool(name)).annotations).toMatchObject({
        readOnlyHint: true,
      });
    }
  });

  it("entrega ao factory a sessão anônima com o X-Forwarded-For", async () => {
    const api = new FakePublicMarkuppApi();
    const note = api.withNote("guia", "a.md", "x");
    const sessions: (PublicSession | undefined)[] = [];
    const session = { forwardedFor: "203.0.113.7" };
    client = await MarkuppToolClient.connectPublic(api, session, sessions);
    await client.call("list_public_notes", { public_id: note.public_id });
    expect(sessions).toEqual([session]);
  });
});

describe("list_public_notes", () => {
  it("devolve as notas da mesma publicação e o total", async () => {
    const api = new FakePublicMarkuppApi();
    const note = api.withNote("guia", "x/Start here.md", "x");
    api.withNote("outra", "b.md", "y");
    client = await MarkuppToolClient.connectPublic(api);
    const result = await client.call("list_public_notes", {
      public_id: note.public_id,
    });
    expect(JSON.parse(result.text)).toEqual({
      items: [
        {
          public_id: note.public_id,
          title: "Start here",
          title_slug: "start-here",
          path: "x/Start here.md",
        },
      ],
      total: 1,
    });
  });

  it("repassa a página à API", async () => {
    const api = new FakePublicMarkuppApi();
    const note = api.withNote("guia", "a.md", "x");
    client = await MarkuppToolClient.connectPublic(api);
    await client.call("list_public_notes", {
      public_id: note.public_id,
      limit: 5,
      offset: 10,
    });
    expect(api.calls).toEqual([
      `tree ${note.public_id} {"limit":5,"offset":10}`,
    ]);
  });

  it("orienta a conferir o id quando a nota não é pública", async () => {
    client = await MarkuppToolClient.connectPublic(new FakePublicMarkuppApi());
    const result = await client.call("list_public_notes", {
      public_id: UNKNOWN_ID,
    });
    expect(result.isError).toBe(true);
    expect(result.text).toMatch(/^not_found: .*public URL/);
  });

  it("recusa id fora do formato antes de chamar a API, citando o valor e o formato", async () => {
    const api = new FakePublicMarkuppApi();
    client = await MarkuppToolClient.connectPublic(api);
    await expect(
      client.call("list_public_notes", { public_id: "../x" }),
    ).rejects.toThrow(/public_id "\.\.\/x" .*\^\[A-Za-z0-9\]\{12\}\$/);
    expect(api.calls).toEqual([]);
  });
});

describe("search_public_notes", () => {
  it("devolve as notas achadas na publicação e o total", async () => {
    const api = new FakePublicMarkuppApi();
    const match = api.withNote("guia", "Agentes.md", "conectar o agente");
    api.withNote("guia", "Outra.md", "nada a ver");
    api.withNote("fora", "Fora.md", "outro agente");
    client = await MarkuppToolClient.connectPublic(api);
    const result = await client.call("search_public_notes", {
      public_id: match.public_id,
      query: "agente",
    });
    expect(JSON.parse(result.text)).toEqual({
      items: [
        {
          public_id: match.public_id,
          title: "Agentes",
          title_slug: "agentes",
          snippet: "conectar o agente",
          rank: 1,
        },
      ],
      total: 1,
    });
  });

  it("troca os marcadores de realce do trecho por negrito markdown", async () => {
    const api = new FakePublicMarkuppApi();
    const note = api.withNote("guia", "A.md", "conectar o \u0002agente\u0003");
    client = await MarkuppToolClient.connectPublic(api);
    const result = await client.call("search_public_notes", {
      public_id: note.public_id,
      query: "agente",
    });
    expect(JSON.parse(result.text).items[0].snippet).toBe(
      "conectar o **agente**",
    );
  });
});

describe("read_public_note", () => {
  it("devolve os metadados e depois o markdown cru", async () => {
    const api = new FakePublicMarkuppApi();
    const note = api.withNote("guia", "Start.md", '# Início\n\n"aspas"');
    client = await MarkuppToolClient.connectPublic(api);
    const result = await client.call("read_public_note", {
      public_id: note.public_id,
    });
    const [metadata, content] = result.parts;
    const { content: _markdown, ...expected } = note;
    expect(JSON.parse(metadata ?? "")).toEqual(expected);
    expect(content).toBe(note.content);
  });

  it("devolve not_found com o próximo passo quando o id não é público", async () => {
    client = await MarkuppToolClient.connectPublic(new FakePublicMarkuppApi());
    const result = await client.call("read_public_note", {
      public_id: UNKNOWN_ID,
    });
    expect(result.isError).toBe(true);
    expect(result.text).toMatch(/^not_found: .*search_public_notes/);
  });

  it("recusa o id '..', que o fetch normalizaria para outra rota", async () => {
    const api = new FakePublicMarkuppApi();
    client = await MarkuppToolClient.connectPublic(api);
    await expect(
      client.call("read_public_note", { public_id: ".." }),
    ).rejects.toThrow(/public_id "\.\." .*A-Za-z0-9/);
    expect(api.calls).toEqual([]);
  });
});
