import { describe, expect, it } from "vitest";
import { FakePublicMarkuppApi } from "./fake-public-markupp-api.js";

describe("FakePublicMarkuppApi", () => {
  it("recusa id público desconhecido com not_found, como a API", async () => {
    await expect(
      new FakePublicMarkuppApi().getNote("naoexiste123"),
    ).rejects.toMatchObject({ status: 404, code: "not_found" });
  });

  it("gera id público de 12 letras e dígitos e tira o título do arquivo", () => {
    const note = new FakePublicMarkuppApi().withNote(
      "guia",
      "guia/Start here.md",
      "x",
    );
    expect(note.public_id).toMatch(/^[A-Za-z0-9]{12}$/);
    expect(note.title).toBe("Start here");
  });

  it("recorta a árvore pela publicação da nota lida", async () => {
    const api = new FakePublicMarkuppApi();
    const guia = api.withNote("guia", "a.md", "x");
    api.withNote("outra", "b.md", "y");
    const tree = await api.listTree(guia.public_id);
    expect(tree.items.map((item) => item.public_id)).toEqual([guia.public_id]);
    expect(tree.total).toBe(1);
  });
});
