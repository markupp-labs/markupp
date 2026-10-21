import { afterEach, describe, expect, it } from "vitest";
import { FakeMarkuppApi } from "../testing/fake-markupp-api.js";
import { MarkuppToolClient } from "../testing/markupp-tool-client.js";

let client: MarkuppToolClient;

afterEach(async () => {
  await client.close();
});

function ownerTenant(): FakeMarkuppApi {
  return new FakeMarkuppApi().withTenant("t1", "owner");
}

describe("list_invites", () => {
  it("devolve os convites do tenant", async () => {
    const api = ownerTenant();
    const invite = api.withInvite("t1", "bia@x.dev", "viewer");
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("list_invites", { tenant: "t1" });
    expect(JSON.parse(result.text)).toEqual([invite]);
  });

  it("anuncia que só lê", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    expect((await client.tool("list_invites")).annotations).toMatchObject({
      readOnlyHint: true,
    });
  });
});

describe("invite_member", () => {
  it("emite o convite e devolve o token para repasse", async () => {
    const api = ownerTenant();
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("invite_member", {
      tenant: "t1",
      email: "bia@x.dev",
      role: "member",
    });
    const issued = JSON.parse(result.text);
    expect(issued).toMatchObject({ email: "bia@x.dev", role: "member" });
    expect(issued.token).toMatch(/^token-/);
    expect(api.invitesIn("t1")).toHaveLength(1);
  });

  it("aponta list_members quando o email já é membro", async () => {
    const api = ownerTenant();
    api.withMember("t1", "u1", "bia@x.dev", "member");
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("invite_member", {
      tenant: "t1",
      email: "bia@x.dev",
      role: "member",
    });
    expect(result.text).toMatch(/^already_member: .*list_members/);
  });
});

describe("revoke_invite", () => {
  it("apaga o convite pendente", async () => {
    const api = ownerTenant();
    const invite = api.withInvite("t1", "bia@x.dev", "viewer");
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("revoke_invite", {
      tenant: "t1",
      invite_id: invite.id,
    });
    expect(JSON.parse(result.text)).toEqual({ revoked: invite.id });
    expect(api.invitesIn("t1")).toEqual([]);
  });

  it("manda achar o id com list_invites quando o convite não existe", async () => {
    client = await MarkuppToolClient.connect(ownerTenant());
    const result = await client.call("revoke_invite", {
      tenant: "t1",
      invite_id: "i9",
    });
    expect(result.text).toMatch(/^invite_not_found: .*list_invites/);
  });

  it("anuncia que é destrutiva", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    expect((await client.tool("revoke_invite")).annotations).toMatchObject({
      destructiveHint: true,
      idempotentHint: true,
    });
  });
});
