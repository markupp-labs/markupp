import { describe, expect, it } from "vitest";
import { MarkuppApiError } from "./markupp-api.js";
import { HttpPublicMarkuppApi } from "./public-api.js";
import { FakeFetch } from "./testing/fake-fetch.js";

const BASE = "http://markupp.interno:8080";
const PUBLIC_ID = "Ab3dEf6hIj9k";

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

const publicNote = {
  public_id: PUBLIC_ID,
  title: "Start here",
  title_slug: "start-here",
  content: "# Oi",
  updated_at: "2026-10-01T00:00:00Z",
  indexable: false,
  publication: { kind: "folder", title: "guia", root_public_id: null },
  links: [
    {
      cited_title: "Privada",
      kind: "explicit",
      public_id: null,
      title_slug: null,
    },
  ],
};

describe("HttpPublicMarkuppApi", () => {
  it("lê a nota pelo id público, sem token", async () => {
    const upstream = new FakeFetch().respondWith(200, publicNote);
    const read = await apiWith(upstream).getNote(PUBLIC_ID);
    const [request] = upstream.requests;
    expect(request?.url).toBe(`${BASE}/v1/public/notes/${PUBLIC_ID}`);
    expect(request?.headers.get("authorization")).toBeNull();
    expect(read).toEqual(publicNote);
  });

  it("lista a árvore da publicação com a página", async () => {
    const tree = { items: [], total: 0 };
    const upstream = new FakeFetch().respondWith(200, tree);
    const listed = await apiWith(upstream).listTree(PUBLIC_ID, {
      limit: 5,
      offset: 10,
    });
    expect(upstream.requests[0]?.url).toBe(
      `${BASE}/v1/public/notes/${PUBLIC_ID}/tree?limit=5&offset=10`,
    );
    expect(listed).toEqual(tree);
  });

  it("busca na publicação com q e a página", async () => {
    const found = { items: [], total: 0 };
    const upstream = new FakeFetch().respondWith(200, found);
    await apiWith(upstream).searchNotes(PUBLIC_ID, "termo & cia", {
      limit: 3,
    });
    expect(upstream.requests[0]?.url).toBe(
      `${BASE}/v1/public/notes/${PUBLIC_ID}/search?q=termo+%26+cia&limit=3`,
    );
  });

  it("repassa o X-Forwarded-For do agente, para o limite por IP ser o dele", async () => {
    const upstream = new FakeFetch().respondWith(200, publicNote);
    await apiWith(upstream, "203.0.113.7, 10.0.0.2").getNote(PUBLIC_ID);
    expect(upstream.requests[0]?.headers.get("x-forwarded-for")).toBe(
      "203.0.113.7, 10.0.0.2",
    );
  });

  it("manda o HTTP Basic do MCP, para o Go confiar no X-Forwarded-For repassado", async () => {
    const upstream = new FakeFetch().respondWith(200, publicNote);
    await new HttpPublicMarkuppApi({
      baseUrl: BASE,
      fetch: upstream.fetch,
      relayAuthorization: "Basic cmVsYXk=",
    }).getNote(PUBLIC_ID);
    expect(upstream.requests[0]?.headers.get("authorization")).toBe(
      "Basic cmVsYXk=",
    );
  });

  it("traduz o erro da API em MarkuppApiError com o código", async () => {
    const upstream = new FakeFetch().respondWith(404, {
      error: "not_found",
      message: "nota publica nao encontrada",
    });
    const failure = apiWith(upstream).getNote(PUBLIC_ID);
    await expect(failure).rejects.toBeInstanceOf(MarkuppApiError);
    await expect(failure).rejects.toMatchObject({
      status: 404,
      code: "not_found",
    });
  });
});
