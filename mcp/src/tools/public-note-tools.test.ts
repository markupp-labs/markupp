import { afterEach, describe, expect, it } from "vitest";
import type { PublicSession } from "../session.js";
import { FakePublicMarkuppApi } from "../testing/fake-public-markupp-api.js";
import { MarkuppToolClient } from "../testing/markupp-tool-client.js";

let client: MarkuppToolClient<PublicSession>;

afterEach(async () => {
  await client.close();
});

function docsVault(): FakePublicMarkuppApi {
  return new FakePublicMarkuppApi().withVault("docs");
}

describe("registro público", () => {
  it("expõe só as ferramentas de leitura dos cofres publicados", async () => {
    client = await MarkuppToolClient.connectPublic(docsVault());
    expect(await client.toolNames()).toEqual([
      "list_public_notes",
      "read_public_note",
      "search_public_notes",
    ]);
  });

  it("anuncia que todas só leem", async () => {
    client = await MarkuppToolClient.connectPublic(docsVault());
    for (const name of await client.toolNames()) {
      expect((await client.tool(name)).annotations).toMatchObject({
        readOnlyHint: true,
      });
    }
  });

  it("entrega ao factory a sessão anônima com o X-Forwarded-For", async () => {
    const sessions: (PublicSession | undefined)[] = [];
    const session = { forwardedFor: "203.0.113.7" };
    client = await MarkuppToolClient.connectPublic(
      docsVault(),
      session,
      sessions,
    );
    await client.call("list_public_notes", { vault: "docs" });
    expect(sessions).toEqual([session]);
  });
});

describe("list_public_notes", () => {
  it("devolve id, title, path e updated_at de cada nota", async () => {
    const api = docsVault();
    const note = api.withNote("docs", "guia/Start here.md", "x");
    client = await MarkuppToolClient.connectPublic(api);
    const result = await client.call("list_public_notes", { vault: "docs" });
    expect(JSON.parse(result.text)).toEqual([
      {
        id: note.id,
        title: "Start here",
        path: "guia/Start here.md",
        updated_at: note.updated_at,
      },
    ]);
  });

  it("repassa a ordem e a página à API", async () => {
    const api = docsVault();
    client = await MarkuppToolClient.connectPublic(api);
    await client.call("list_public_notes", {
      vault: "docs",
      sort: "pagerank",
      limit: 5,
      offset: 10,
    });
    expect(api.calls).toEqual([
      'list docs {"sort":"pagerank","limit":5,"offset":10}',
    ]);
  });

  it("diz para conferir o slug quando o cofre não está publicado", async () => {
    client = await MarkuppToolClient.connectPublic(new FakePublicMarkuppApi());
    const result = await client.call("list_public_notes", { vault: "sumiu" });
    expect(result.isError).toBe(true);
    expect(result.text).toMatch(/^not_found: .*slug/);
  });

  it("recusa slug fora do formato antes de chamar a API", async () => {
    const api = docsVault();
    client = await MarkuppToolClient.connectPublic(api);
    await expect(
      client.call("list_public_notes", { vault: "../x" }),
    ).rejects.toThrow(/vault: .*pattern/);
    expect(api.calls).toEqual([]);
  });
});

describe("search_public_notes", () => {
  it("devolve id, title, path e o trecho de cada nota achada", async () => {
    const api = docsVault();
    const match = api.withNote("docs", "Agentes.md", "conectar o agente");
    api.withNote("docs", "Outra.md", "nada a ver");
    client = await MarkuppToolClient.connectPublic(api);
    const result = await client.call("search_public_notes", {
      vault: "docs",
      query: "agente",
    });
    expect(JSON.parse(result.text)).toEqual([
      {
        id: match.id,
        title: "Agentes",
        path: "Agentes.md",
        snippet: "conectar o agente",
      },
    ]);
  });
});

describe("read_public_note", () => {
  it("devolve os metadados e depois o markdown cru", async () => {
    const api = docsVault();
    const note = api.withNote("docs", "Start.md", '# Início\n\n"aspas"');
    client = await MarkuppToolClient.connectPublic(api);
    const result = await client.call("read_public_note", {
      vault: "docs",
      id: note.id,
    });
    const [metadata, content] = result.parts;
    expect(JSON.parse(metadata ?? "")).toEqual({
      id: note.id,
      title: "Start",
      path: "Start.md",
      created_at: note.created_at,
      updated_at: note.updated_at,
    });
    expect(content).toBe(note.content);
  });

  it("aponta list_public_notes quando a nota não existe", async () => {
    client = await MarkuppToolClient.connectPublic(docsVault());
    const result = await client.call("read_public_note", {
      vault: "docs",
      id: "n9",
    });
    expect(result.isError).toBe(true);
    expect(result.text).toMatch(/^not_found: .*list_public_notes/);
  });

  it("recusa o id '..', que o fetch normalizaria para a rota do cofre", async () => {
    const api = docsVault();
    client = await MarkuppToolClient.connectPublic(api);
    await expect(
      client.call("read_public_note", { vault: "docs", id: ".." }),
    ).rejects.toThrow(/id: .*"\.\.".*a-z/);
    expect(api.calls).toEqual([]);
  });
});
