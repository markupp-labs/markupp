import { describe, expect, it } from "vitest";
import { FakeFetch } from "./fake-fetch.js";

describe("FakeFetch", () => {
  it("devolve as respostas na ordem e guarda as requisições", async () => {
    const fake = new FakeFetch()
      .respondWith(200, { ok: true })
      .respondWith(204);
    const first = await fake.fetch("http://api/v1/a", { method: "POST" });
    const second = await fake.fetch("http://api/v1/b");
    expect(await first.json()).toEqual({ ok: true });
    expect(second.status).toBe(204);
    expect(fake.requests.map((r) => `${r.method} ${r.url}`)).toEqual([
      "POST http://api/v1/a",
      "GET http://api/v1/b",
    ]);
  });

  it("falha com o método e a url quando a fila acaba", async () => {
    await expect(new FakeFetch().fetch("http://api/v1/x")).rejects.toThrow(
      "GET http://api/v1/x",
    );
  });
});
