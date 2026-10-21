import { afterEach, describe, expect, it } from "vitest";
import { FakeMarkuppApi } from "../testing/fake-markupp-api.js";
import { MarkuppToolClient } from "../testing/markupp-tool-client.js";

let client: MarkuppToolClient;

afterEach(async () => {
  await client.close();
});

function adminTenant(): FakeMarkuppApi {
  return new FakeMarkuppApi().withTenant("t1", "admin");
}

describe("list_members", () => {
  it("devolve id, email, papel e entrada de cada membro", async () => {
    const api = adminTenant();
    const ana = api.withMember("t1", "u1", "ana@x.dev", "member");
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("list_members", { tenant: "t1" });
    expect(JSON.parse(result.text)).toEqual([ana]);
  });

  it("manda usar um tenant de owner ou admin quando o papel não administra", async () => {
    const api = new FakeMarkuppApi().withTenant("t1", "member");
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("list_members", { tenant: "t1" });
    expect(result.isError).toBe(true);
    expect(result.text).toMatch(/^forbidden: .*role owner or admin/);
  });

  it("anuncia que só lê", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    expect((await client.tool("list_members")).annotations).toMatchObject({
      readOnlyHint: true,
    });
  });
});

describe("change_member_role", () => {
  it("troca o papel do membro e devolve o novo papel", async () => {
    const api = adminTenant();
    api.withMember("t1", "u1", "ana@x.dev", "viewer");
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("change_member_role", {
      tenant: "t1",
      user_id: "u1",
      role: "member",
    });
    expect(JSON.parse(result.text)).toEqual({ user_id: "u1", role: "member" });
    expect(api.membersIn("t1")[0]?.role).toBe("member");
  });

  it("recusa papel fora da lista antes de chamar a API", async () => {
    client = await MarkuppToolClient.connect(adminTenant());
    const call = client.call("change_member_role", {
      tenant: "t1",
      user_id: "u1",
      role: "dono",
    });
    await expect(call).rejects.toThrow(/role: Invalid option/);
  });

  it("manda achar o id com list_members quando o membro não existe", async () => {
    client = await MarkuppToolClient.connect(adminTenant());
    const result = await client.call("change_member_role", {
      tenant: "t1",
      user_id: "u9",
      role: "member",
    });
    expect(result.text).toMatch(/^member_not_found: .*list_members/);
  });

  it("anuncia escrita idempotente e não destrutiva", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    expect((await client.tool("change_member_role")).annotations).toMatchObject(
      { readOnlyHint: false, destructiveHint: false, idempotentHint: true },
    );
  });
});

describe("remove_member", () => {
  it("tira o membro do tenant", async () => {
    const api = adminTenant();
    api.withMember("t1", "u1", "ana@x.dev", "member");
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("remove_member", {
      tenant: "t1",
      user_id: "u1",
    });
    expect(JSON.parse(result.text)).toEqual({ removed: "u1" });
    expect(api.membersIn("t1")).toEqual([]);
  });

  it("anuncia que é destrutiva", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    expect((await client.tool("remove_member")).annotations).toMatchObject({
      destructiveHint: true,
      idempotentHint: true,
    });
  });
});
