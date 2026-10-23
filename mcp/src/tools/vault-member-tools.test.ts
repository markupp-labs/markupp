import { afterEach, describe, expect, it } from "vitest";
import { FakeMarkuppApi } from "../testing/fake-markupp-api.js";
import { MarkuppToolClient } from "../testing/markupp-tool-client.js";
import { FakeAccountId } from "../testing/fake-vaults.js";

let client: MarkuppToolClient;

afterEach(async () => {
  await client.close();
});

// tenantWithVault monta o tenant t1 com a colega u1 e um cofre de que só a conta é membro.
function tenantWithVault(role = "member"): {
  api: FakeMarkuppApi;
  vault: string;
} {
  const api = new FakeMarkuppApi().withTenant("t1", role);
  api.withMember("t1", "u1", "ana@x.dev", "member");
  return { api, vault: api.withVault("t1", "Casa").id };
}

describe("list_vault_members", () => {
  it("devolve id, email, papel no tenant e entrada de cada membro", async () => {
    const { api, vault } = tenantWithVault();
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("list_vault_members", {
      tenant: "t1",
      vault_id: vault,
    });
    expect(JSON.parse(result.text)).toEqual(
      await api.listVaultMembers("t1", vault),
    );
  });

  it("anuncia que só lê", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    expect((await client.tool("list_vault_members")).annotations).toMatchObject(
      { readOnlyHint: true },
    );
  });
});

describe("add_vault_member", () => {
  it("põe a conta do tenant no cofre", async () => {
    const { api, vault } = tenantWithVault();
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("add_vault_member", {
      tenant: "t1",
      vault_id: vault,
      user_id: "u1",
    });
    expect(JSON.parse(result.text)).toEqual({ vault_id: vault, user_id: "u1" });
    const members = await api.listVaultMembers("t1", vault);
    expect(members.map((m) => m.id)).toEqual([FakeAccountId, "u1"]);
  });

  it("manda achar a conta em list_members ou convidar quando ela não é do tenant", async () => {
    const { api, vault } = tenantWithVault();
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("add_vault_member", {
      tenant: "t1",
      vault_id: vault,
      user_id: "u9",
    });
    expect(result.isError).toBe(true);
    expect(result.text).toMatch(
      /^not_tenant_member: .*list_members.*invite_member/,
    );
  });

  it("aponta list_vault_members quando a conta já é do cofre", async () => {
    const { api, vault } = tenantWithVault();
    await api.addVaultMember("t1", vault, "u1");
    client = await MarkuppToolClient.connect(api);
    const args = { tenant: "t1", vault_id: vault, user_id: "u1" };
    const result = await client.call("add_vault_member", args);
    expect(result.text).toMatch(/^already_vault_member: .*list_vault_members/);
  });

  it("descreve que só aceita conta do tenant e onde achar o id", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    const tool = await client.tool("add_vault_member");
    expect(tool.description).toMatch(/tenant/);
    expect(tool.description).toMatch(/list_members/);
  });
});

describe("remove_vault_member", () => {
  it("tira a conta do cofre", async () => {
    const { api, vault } = tenantWithVault();
    await api.addVaultMember("t1", vault, "u1");
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("remove_vault_member", {
      tenant: "t1",
      vault_id: vault,
      user_id: "u1",
    });
    expect(JSON.parse(result.text)).toEqual({ removed: "u1" });
    const members = await api.listVaultMembers("t1", vault);
    expect(members.map((m) => m.id)).toEqual([FakeAccountId]);
  });

  it("deixa o viewer sair do cofre com o próprio id", async () => {
    const { api, vault } = tenantWithVault("viewer");
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("remove_vault_member", {
      tenant: "t1",
      vault_id: vault,
      user_id: FakeAccountId,
    });
    expect(result.isError).toBe(false);
    expect(await api.listVaults("t1")).toEqual([]);
  });

  it("aponta list_vault_members quando a conta não é do cofre", async () => {
    const { api, vault } = tenantWithVault();
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("remove_vault_member", {
      tenant: "t1",
      vault_id: vault,
      user_id: "u1",
    });
    expect(result.text).toMatch(
      /^vault_member_not_found: .*list_vault_members/,
    );
  });

  it("anuncia que é destrutiva", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    expect(
      (await client.tool("remove_vault_member")).annotations,
    ).toMatchObject({ destructiveHint: true, idempotentHint: true });
  });

  it("aponta whoami para achar o próprio id antes de sair do cofre", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    const tool = await client.tool("remove_vault_member");
    expect(tool.description).toMatch(/whoami/);
  });
});
