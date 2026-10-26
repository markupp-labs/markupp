import { describe, expect, it } from "vitest";
import { MarkuppApiError } from "./markupp-api.js";
import { HttpPublicMarkuppApi } from "./public-api.js";
import { FakeFetch } from "./testing/fake-fetch.js";

const BASE = "http://markupp.interno:8080";

function apiWith(
  upstream: FakeFetch,
  forwardedFor?: string,
): HttpPublicMarkuppApi {
  return new HttpPublicMarkuppApi({
    baseUrl: `${BASE}/`,
    fetch: upstream.fetch,
    forwardedFor,
  });
}

const listed = {
  id: "n1",
  title: "Start here",
  path: "guide/Start here.md",
  updated_at: "2026-10-01T00:00:00Z",
  content_sha256: "abc",
  pagerank: 0.4,
};

describe("HttpPublicMarkuppApi", () => {
  it("lista as notas do cofre pelo slug, sem token, só com id, title, path e updated_at", async () => {
    const upstream = new FakeFetch().respondWith(200, [listed]);
    const notes = await apiWith(upstream).listNotes("docs", {
      sort: "pagerank",
      limit: 5,
      offset: 10,
    });
    const [request] = upstream.requests;
    expect(request?.url).toBe(
      `${BASE}/v1/public/vaults/docs/notes?sort=pagerank&limit=5&offset=10`,
    );
    expect(request?.headers.get("authorization")).toBeNull();
    expect(notes).toEqual([
      {
        id: "n1",
        title: "Start here",
        path: "guide/Start here.md",
        updated_at: "2026-10-01T00:00:00Z",
      },
    ]);
  });

  it("deixa a ordem por caminho para o padrão da API", async () => {
    const upstream = new FakeFetch().respondWith(200, []);
    await apiWith(upstream).listNotes("docs", { sort: "path" });
    expect(upstream.requests[0]?.url).toBe(
      `${BASE}/v1/public/vaults/docs/notes`,
    );
  });

  it("lê a nota com o slug e o id escapados", async () => {
    const note = { ...listed, content: "# Oi", created_at: "2026-09-01" };
    const upstream = new FakeFetch().respondWith(200, note);
    const read = await apiWith(upstream).getNote("docs", "a/b");
    expect(upstream.requests[0]?.url).toBe(
      `${BASE}/v1/public/vaults/docs/notes/a%2Fb`,
    );
    expect(read.content).toBe("# Oi");
  });

  it("busca com q e a página", async () => {
    const hit = {
      note: { id: "n1", title: "Start", path: "Start.md" },
      rank: 1,
      snippet: "o <b>termo</b>",
    };
    const upstream = new FakeFetch().respondWith(200, [hit]);
    const found = await apiWith(upstream).searchNotes("docs", "termo & cia", {
      limit: 3,
    });
    expect(upstream.requests[0]?.url).toBe(
      `${BASE}/v1/public/vaults/docs/search?q=termo+%26+cia&limit=3`,
    );
    expect(found).toEqual([hit]);
  });

  it("repassa o X-Forwarded-For do agente, para o limite por IP ser o dele", async () => {
    const upstream = new FakeFetch().respondWith(200, []);
    await apiWith(upstream, "203.0.113.7, 10.0.0.2").listNotes("docs");
    expect(upstream.requests[0]?.headers.get("x-forwarded-for")).toBe(
      "203.0.113.7, 10.0.0.2",
    );
  });

  it("manda o HTTP Basic do MCP, para o Go confiar no X-Forwarded-For repassado", async () => {
    const upstream = new FakeFetch().respondWith(200, []);
    await new HttpPublicMarkuppApi({
      baseUrl: BASE,
      fetch: upstream.fetch,
      relayAuthorization: "Basic cmVsYXk=",
    }).listNotes("docs");
    expect(upstream.requests[0]?.headers.get("authorization")).toBe(
      "Basic cmVsYXk=",
    );
  });

  it("traduz o erro da API em MarkuppApiError com o código", async () => {
    const upstream = new FakeFetch().respondWith(404, {
      error: "not_found",
      message: "cofre publico nao encontrado",
    });
    const failure = apiWith(upstream).getNote("x", "n1");
    await expect(failure).rejects.toBeInstanceOf(MarkuppApiError);
    await expect(failure).rejects.toMatchObject({
      status: 404,
      code: "not_found",
    });
  });
});
