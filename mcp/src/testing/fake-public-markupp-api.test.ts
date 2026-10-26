import { describe, expect, it } from "vitest";
import { FakePublicMarkuppApi } from "./fake-public-markupp-api.js";

describe("FakePublicMarkuppApi", () => {
  it("recusa slug sem cofre publicado com not_published", async () => {
    await expect(
      new FakePublicMarkuppApi().listNotes("sumiu"),
    ).rejects.toMatchObject({ status: 404, code: "not_published" });
  });

  it("recusa nota fora do cofre com not_found", async () => {
    const api = new FakePublicMarkuppApi().withVault("docs");
    await expect(api.getNote("docs", "n9")).rejects.toMatchObject({
      code: "not_found",
    });
  });

  it("tira o título do nome do arquivo", () => {
    const note = new FakePublicMarkuppApi()
      .withVault("docs")
      .withNote("docs", "guia/Start here.md", "x");
    expect(note.title).toBe("Start here");
  });
});
