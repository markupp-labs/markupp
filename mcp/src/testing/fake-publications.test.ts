import { describe, expect, it } from "vitest";
import { FakeMarkuppApi } from "./fake-markupp-api.js";

function vaultWithNote(role: string) {
  const api = new FakeMarkuppApi().withTenant("t1", role);
  const vault = api.withVault("t1", "Docs");
  const note = api.withNote("t1", "guia/Deploy.md", "x", vault.id);
  return { api, vault, note };
}

describe("FakeMarkuppApi nas publicações", () => {
  it("publica a nota confirmada pelo título, com id público", async () => {
    const { api, vault, note } = vaultWithNote("member");
    const published = await api.publish("t1", vault.id, {
      target: { kind: "note", note_id: note.id },
      confirm: "Deploy",
      indexable: false,
    });
    expect(published).toMatchObject({
      kind: "note",
      note_id: note.id,
      target_name: "Deploy",
      note_count: 1,
    });
    expect(published.public_id).toMatch(/^[A-Za-z0-9]{12}$/);
  });

  it("recusa a pasta sem nota com folder_not_found", async () => {
    const { api, vault } = vaultWithNote("owner");
    const publish = api.publish("t1", vault.id, {
      target: { kind: "folder", path: "vazia" },
      confirm: "vazia",
      indexable: false,
    });
    await expect(publish).rejects.toMatchObject({
      status: 404,
      code: "folder_not_found",
    });
  });

  it("recusa publicar duas vezes o mesmo alvo com already_published", async () => {
    const { api, vault } = vaultWithNote("owner");
    const request = {
      target: { kind: "vault" as const },
      confirm: "Docs",
      indexable: false,
    };
    await api.publish("t1", vault.id, request);
    await expect(api.publish("t1", vault.id, request)).rejects.toMatchObject({
      status: 409,
      code: "already_published",
    });
  });
});
