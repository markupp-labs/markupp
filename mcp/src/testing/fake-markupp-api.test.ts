import { describe, expect, it } from "vitest";
import { FakeApiTokens, FakeMarkuppApi } from "./fake-markupp-api.js";

describe("FakeApiTokens", () => {
  it("entrega os tokens na ordem e repete o último", async () => {
    const tokens = new FakeApiTokens("a", "b");
    expect([
      await tokens.source(),
      await tokens.source(),
      await tokens.source(),
    ]).toEqual(["a", "b", "b"]);
  });
});

describe("FakeMarkuppApi", () => {
  it("recusa a escrita sobre um updated_at velho com conflict", async () => {
    const api = new FakeMarkuppApi().withTenant("t1");
    const note = api.withNote("t1", "a.md", "x");
    api.concurrentWriteOnNextRead(note.id, "y");
    const read = await api.getNote("t1", note.id);
    const write = api.updateNote("t1", note.id, {
      path: "a.md",
      content: "z",
      lastModifiedAt: read.updated_at,
    });
    await expect(write).rejects.toMatchObject({
      status: 409,
      code: "conflict",
    });
  });

  it("recusa tenant fora da conta com tenant_forbidden", async () => {
    await expect(new FakeMarkuppApi().getNote("t9", "n")).rejects.toMatchObject(
      { code: "tenant_forbidden" },
    );
  });
});
