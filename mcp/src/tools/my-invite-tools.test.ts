import { afterEach, describe, expect, it } from "vitest";
import { FakeMarkuppApi } from "../testing/fake-markupp-api.js";
import { MarkuppToolClient } from "../testing/markupp-tool-client.js";

let client: MarkuppToolClient;

afterEach(async () => {
  await client.close();
});

describe("list_my_invites", () => {
  it("devolve os convites pendentes da conta", async () => {
    const api = new FakeMarkuppApi();
    const invite = api.withMyInvite("t2", "Time", "member");
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("list_my_invites", {});
    expect(JSON.parse(result.text)).toEqual([invite]);
  });

  it("anuncia que só lê", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    expect((await client.tool("list_my_invites")).annotations).toMatchObject({
      readOnlyHint: true,
    });
  });
});

describe("accept_invite", () => {
  it("entra no tenant do convite e devolve o vínculo", async () => {
    const api = new FakeMarkuppApi();
    const invite = api.withMyInvite("t2", "Time", "member");
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("accept_invite", { invite_id: invite.id });
    const joined = { id: "t2", name: "Time", role: "member" };
    expect(JSON.parse(result.text)).toEqual(joined);
    expect(await api.listTenants()).toEqual([joined]);
  });

  it("manda achar o id com list_my_invites quando o convite não existe", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    const result = await client.call("accept_invite", { invite_id: "i9" });
    expect(result.text).toMatch(/^invite_not_found: .*list_my_invites/);
  });
});

describe("decline_invite", () => {
  it("recusa o convite e ele some da lista", async () => {
    const api = new FakeMarkuppApi();
    const invite = api.withMyInvite("t2", "Time", "member");
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("decline_invite", {
      invite_id: invite.id,
    });
    expect(JSON.parse(result.text)).toEqual({ declined: invite.id });
    expect(await api.listMyInvites()).toEqual([]);
  });
});

describe("decline_invite", () => {
  it("anuncia que não tem volta, para o cliente pedir confirmação", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    expect((await client.tool("decline_invite")).annotations).toMatchObject({
      destructiveHint: true,
    });
  });
});
