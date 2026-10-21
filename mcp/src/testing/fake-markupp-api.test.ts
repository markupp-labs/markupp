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
