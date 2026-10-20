import { describe, expect, it } from "vitest";
import { HttpMarkuppApi, MarkuppApiError, type Note } from "./markupp-api.js";
import { FakeFetch } from "./testing/fake-fetch.js";
import { FakeApiTokens } from "./testing/fake-markupp-api.js";

const note: Note = {
  id: "n1",
  vault_id: "v1",
  path: "ideias.md",
  content: "# Ideias",
  created_at: "2026-10-01T10:00:00Z",
  updated_at: "2026-10-02T10:00:00Z",
};

function apiWith(fake: FakeFetch): HttpMarkuppApi {
  return new HttpMarkuppApi({
    baseUrl: "http://api.test/",
    fetch: fake.fetch,
    apiToken: new FakeApiTokens("jwt-1").source,
  });
}

function onlyRequest(fake: FakeFetch): Request {
  const [request] = fake.requests;
  if (!request || fake.requests.length !== 1) {
    throw new Error(`esperava 1 requisição, chegaram ${fake.requests.length}`);
  }
  return request;
}

describe("HttpMarkuppApi", () => {
  it("lista os tenants da conta com o JWT e sem cabeçalho de tenant", async () => {
    const tenants = [{ id: "t1", name: "Casa", role: "owner" }];
    const fake = new FakeFetch().respondWith(200, tenants);
    expect(await apiWith(fake).listTenants()).toEqual(tenants);
    const request = onlyRequest(fake);
    expect(`${request.method} ${request.url}`).toBe(
      "GET http://api.test/v1/me/tenants",
    );
    expect(request.headers.get("authorization")).toBe("Bearer jwt-1");
    expect(request.headers.has("x-markupp-tenant")).toBe(false);
  });

  it("busca notas com query, limit e offset na query string e o tenant no cabeçalho", async () => {
    const fake = new FakeFetch().respondWith(200, []);
    await apiWith(fake).searchNotes("t1", "a b&c", { limit: 5, offset: 10 });
    const request = onlyRequest(fake);
    expect(request.url).toBe(
      "http://api.test/v1/notes/search?query=a+b%26c&limit=5&offset=10",
    );
    expect(request.headers.get("x-markupp-tenant")).toBe("t1");
  });

  it("lista as notas do tenant sem devolver o conteúdo", async () => {
    const note = {
      id: "n1",
      vault_id: "v1",
      path: "a.md",
      content: "corpo",
      created_at: "2026-10-20T10:00:00Z",
      updated_at: "2026-10-20T11:00:00Z",
    };
    const fake = new FakeFetch().respondWith(200, [note]);
    const listed = await apiWith(fake).listNotes("t1");
    const request = onlyRequest(fake);
    expect(`${request.method} ${request.url}`).toBe(
      "GET http://api.test/v1/notes",
    );
    expect(request.headers.get("x-markupp-tenant")).toBe("t1");
    expect(listed).toEqual([
      {
        id: "n1",
        vault_id: "v1",
        path: "a.md",
        updated_at: "2026-10-20T11:00:00Z",
      },
    ]);
  });

  it("busca sem limit nem offset deixa o padrão do servidor", async () => {
    const fake = new FakeFetch().respondWith(200, []);
    await apiWith(fake).searchNotes("t1", "x");
    expect(onlyRequest(fake).url).toBe(
      "http://api.test/v1/notes/search?query=x",
    );
  });

  it("lê a nota pelo id escapado", async () => {
    const fake = new FakeFetch().respondWith(200, note);
    expect(await apiWith(fake).getNote("t1", "a/b")).toEqual(note);
    const request = onlyRequest(fake);
    expect(`${request.method} ${request.url}`).toBe(
      "GET http://api.test/v1/notes/a%2Fb",
    );
    expect(request.headers.get("x-markupp-tenant")).toBe("t1");
  });

  it("cria a nota no cofre padrão, com vault_id vazio", async () => {
    const fake = new FakeFetch().respondWith(201, note);
    await apiWith(fake).createNote("t1", "ideias.md", "# Ideias");
    const request = onlyRequest(fake);
    expect(`${request.method} ${request.url}`).toBe(
      "POST http://api.test/v1/notes",
    );
    expect(request.headers.get("content-type")).toBe("application/json");
    expect(await request.json()).toEqual({
      vault_id: "",
      path: "ideias.md",
      content: "# Ideias",
    });
  });

  it("atualiza a nota com o lastModifiedAt lido e force false", async () => {
    const fake = new FakeFetch().respondWith(200, note);
    const revision = {
      path: "ideias.md",
      content: "novo",
      lastModifiedAt: "2026-10-02T10:00:00Z",
    };
    await apiWith(fake).updateNote("t1", "n1", revision);
    const request = onlyRequest(fake);
    expect(`${request.method} ${request.url}`).toBe(
      "PUT http://api.test/v1/notes/n1",
    );
    expect(await request.json()).toEqual({ ...revision, force: false });
  });

  it("apaga a nota e aceita o 204 sem corpo", async () => {
    const fake = new FakeFetch().respondWith(204);
    await apiWith(fake).deleteNote("t1", "n1");
    const request = onlyRequest(fake);
    expect(`${request.method} ${request.url}`).toBe(
      "DELETE http://api.test/v1/notes/n1",
    );
    expect(request.headers.get("x-markupp-tenant")).toBe("t1");
  });

  it("pede o JWT à fonte a cada chamada", async () => {
    const fake = new FakeFetch().respondWith(200, []).respondWith(200, []);
    const api = new HttpMarkuppApi({
      baseUrl: "http://api.test",
      fetch: fake.fetch,
      apiToken: new FakeApiTokens("jwt-a", "jwt-b").source,
    });
    await api.listTenants();
    await api.listTenants();
    expect(fake.requests.map((r) => r.headers.get("authorization"))).toEqual([
      "Bearer jwt-a",
      "Bearer jwt-b",
    ]);
  });

  it("vira MarkuppApiError com status, code e message do corpo de erro", async () => {
    const body = { error: "conflict", message: "nota foi atualizada" };
    const fake = new FakeFetch().respondWith(409, body);
    const failure = apiWith(fake).getNote("t1", "n1");
    await expect(failure).rejects.toBeInstanceOf(MarkuppApiError);
    await expect(failure).rejects.toMatchObject({
      status: 409,
      code: "conflict",
      message: "nota foi atualizada",
    });
  });

  it("vira unexpected_response com o status quando o corpo de erro foge do formato", async () => {
    const fake = new FakeFetch().respondWith(502, "bad gateway");
    await expect(apiWith(fake).listTenants()).rejects.toMatchObject({
      status: 502,
      code: "unexpected_response",
      message: expect.stringContaining("GET /v1/me/tenants respondeu 502"),
    });
  });
});
