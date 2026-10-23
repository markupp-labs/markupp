import { describe, expect, it } from "vitest";
import { FakeApiTokens, FakeMarkuppApi } from "./fake-markupp-api.js";
import { FakeAccountId } from "./fake-vaults.js";

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

describe("FakeMarkuppApi na administração", () => {
  it("recusa com forbidden quem não é owner nem admin", async () => {
    const api = new FakeMarkuppApi().withTenant("t1", "viewer");
    await expect(api.listInvites("t1")).rejects.toMatchObject({
      status: 403,
      code: "forbidden",
    });
  });

  it("recusa com already_member o convite aceito de tenant da conta", async () => {
    const api = new FakeMarkuppApi().withTenant("t1");
    const invite = api.withMyInvite("t1", "Casa", "member");
    await expect(api.acceptInvite(invite.id)).rejects.toMatchObject({
      status: 409,
      code: "already_member",
    });
  });

  it("recusa com invite_not_found o convite já recusado", async () => {
    const api = new FakeMarkuppApi();
    const invite = api.withMyInvite("t2", "Time", "member");
    await api.declineInvite(invite.id);
    await expect(api.declineInvite(invite.id)).rejects.toMatchObject({
      status: 404,
      code: "invite_not_found",
    });
  });
});

describe("FakeMarkuppApi nos cofres", () => {
  it("deixa o viewer sair do cofre e recusa as outras escritas com read_only", async () => {
    const api = new FakeMarkuppApi().withTenant("t1", "viewer");
    const vault = api.withVault("t1", "Casa");
    await expect(api.createVault("t1", "Outro")).rejects.toMatchObject({
      status: 403,
      code: "read_only",
    });
    await api.removeVaultMember("t1", vault.id, FakeAccountId);
    expect(await api.listVaults("t1")).toEqual([]);
  });

  it("responde vault_forbidden ao cofre de que a conta saiu", async () => {
    const api = new FakeMarkuppApi().withTenant("t1");
    const vault = api.withVault("t1", "Casa");
    await api.removeVaultMember("t1", vault.id, FakeAccountId);
    await expect(api.listVaultMembers("t1", vault.id)).rejects.toMatchObject({
      status: 403,
      code: "vault_forbidden",
    });
  });

  it("apaga as notas junto com o cofre", async () => {
    const api = new FakeMarkuppApi().withTenant("t1");
    const vault = api.withVault("t1", "Casa");
    const kept = api.withNote("t1", "fica.md", "x");
    await api.createNote("t1", "vai.md", "y", vault.id);
    await api.deleteVault("t1", vault.id);
    expect((await api.listNotes("t1")).map((n) => n.id)).toEqual([kept.id]);
  });

  it.each([
    ["u9", 422, "not_tenant_member"],
    ["u1", 409, "already_vault_member"],
  ])("recusa adicionar %s com %s %s", async (userId, status, code) => {
    const api = new FakeMarkuppApi().withTenant("t1");
    api.withMember("t1", "u1", "ana@x.dev", "member");
    const vault = api.withVault("t1", "Casa");
    await api.addVaultMember("t1", vault.id, "u1");
    const adding = api.addVaultMember("t1", vault.id, userId);
    await expect(adding).rejects.toMatchObject({ status, code });
  });

  it("recusa tirar quem não é do cofre com vault_member_not_found", async () => {
    const api = new FakeMarkuppApi().withTenant("t1");
    const vault = api.withVault("t1", "Casa");
    const removing = api.removeVaultMember("t1", vault.id, "u9");
    await expect(removing).rejects.toMatchObject({
      status: 404,
      code: "vault_member_not_found",
    });
  });
});
