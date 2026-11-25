import { afterEach, describe, expect, it } from "vitest";
import { FakeMarkuppApi } from "../testing/fake-markupp-api.js";
import { MarkuppToolClient } from "../testing/markupp-tool-client.js";

let client: MarkuppToolClient;

afterEach(async () => {
  await client.close();
});

async function adminClient(
  api = new FakeMarkuppApi(),
): Promise<FakeMarkuppApi> {
  client = await MarkuppToolClient.connect(api.withInstanceAdmin());
  return api;
}

describe("sessões da conta", () => {
  it("list_my_sessions lista as sessões com a atual marcada", async () => {
    const api = new FakeMarkuppApi()
      .withSession("s1", true)
      .withSession("s2", false);
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("list_my_sessions", {});
    expect(result.isError).toBe(false);
    expect(JSON.parse(result.text)).toMatchObject([
      { id: "s1", current: true },
      { id: "s2", current: false },
    ]);
  });

  it("end_my_session encerra e avisa quando não acha", async () => {
    const api = new FakeMarkuppApi().withSession("s2", false);
    client = await MarkuppToolClient.connect(api);
    expect((await client.call("end_my_session", { id: "s2" })).isError).toBe(
      false,
    );
    const again = await client.call("end_my_session", { id: "s2" });
    expect(again.isError).toBe(true);
    expect(again.text).toMatch(/^session_not_found: /);
  });
});

describe("contas da instância", () => {
  it("list_accounts repassa o filtro", async () => {
    await adminClient(
      new FakeMarkuppApi().withInstanceAccount("u2", "bia@exemplo.org"),
    );
    const result = await client.call("list_accounts", {
      q: "bia",
      status: "active",
    });
    expect(result.isError).toBe(false);
    expect(JSON.parse(result.text)).toMatchObject([
      { id: "u2", email: "bia@exemplo.org", blocked: false },
    ]);
  });

  it("block_account e unblock_account mudam o bloqueio", async () => {
    await adminClient(
      new FakeMarkuppApi().withInstanceAccount("u2", "bia@exemplo.org"),
    );
    const blocked = await client.call("block_account", { id: "u2" });
    expect(JSON.parse(blocked.text)).toMatchObject({ id: "u2", blocked: true });
    const unblocked = await client.call("unblock_account", { id: "u2" });
    expect(JSON.parse(unblocked.text)).toMatchObject({
      id: "u2",
      blocked: false,
    });
  });

  it("delete_account é destrutiva", async () => {
    await adminClient();
    expect((await client.tool("delete_account")).annotations).toMatchObject({
      destructiveHint: true,
    });
  });

  it("recusa quem não administra a instância", async () => {
    client = await MarkuppToolClient.connect(
      new FakeMarkuppApi().withInstanceAccount("u2", "bia@exemplo.org"),
    );
    const result = await client.call("list_accounts", {});
    expect(result.isError).toBe(true);
    expect(result.text).toMatch(/^instance_admin_required: /);
  });
});

describe("acessos da instância", () => {
  it("block_ip devolve o bloqueio e list_ip_blocks o lista", async () => {
    await adminClient();
    const created = await client.call("block_ip", {
      cidr: "198.51.100.0/24",
      note: "varredura",
    });
    expect(created.isError).toBe(false);
    const list = await client.call("list_ip_blocks", {});
    expect(JSON.parse(list.text)).toMatchObject([
      { cidr: "198.51.100.0/24", note: "varredura" },
    ]);
  });

  it("list_audit_events repassa os filtros", async () => {
    const api = await adminClient();
    await client.call("list_audit_events", {
      account: "u2",
      action: "auth.login",
      result: "failure",
    });
    expect(api.lastAuditQuery()).toEqual({
      account: "u2",
      action: "auth.login",
      result: "failure",
    });
  });

  it("set_access_history_retention troca o prazo", async () => {
    await adminClient();
    const result = await client.call("set_access_history_retention", {
      days: 30,
    });
    expect(JSON.parse(result.text)).toEqual({ days: 30, host_locked: false });
    const read = await client.call("get_access_history_retention", {});
    expect(JSON.parse(read.text)).toEqual({ days: 30, host_locked: false });
  });
});
