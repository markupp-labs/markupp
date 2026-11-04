import { afterEach, describe, expect, it } from "vitest";
import { FakeMarkuppApi } from "../testing/fake-markupp-api.js";
import { MarkuppToolClient } from "../testing/markupp-tool-client.js";

let client: MarkuppToolClient;

afterEach(async () => {
  await client.close();
});

describe("list_vaults", () => {
  it("devolve os cofres de que a conta é membro", async () => {
    const api = new FakeMarkuppApi().withTenant("t1");
    const vault = api.withVault("t1", "Casa");
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("list_vaults", { tenant: "t1" });
    expect(JSON.parse(result.text)).toEqual([vault]);
  });

  it("anuncia que só lê", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    expect((await client.tool("list_vaults")).annotations).toMatchObject({
      readOnlyHint: true,
    });
  });
});

describe("create_vault", () => {
  it("cria o cofre com o nome aparado e devolve id e nome", async () => {
    const api = new FakeMarkuppApi().withTenant("t1");
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("create_vault", {
      tenant: "t1",
      name: "  Projeto  ",
    });
    const created = JSON.parse(result.text) as { id: string; name: string };
    expect(created.name).toBe("Projeto");
    expect(await api.listVaults("t1")).toEqual([created]);
  });

  it("recusa nome acima de 100 caracteres antes de chamar a API", async () => {
    client = await MarkuppToolClient.connect(
      new FakeMarkuppApi().withTenant("t1"),
    );
    const call = client.call("create_vault", {
      tenant: "t1",
      name: "x".repeat(101),
    });
    await expect(call).rejects.toThrow(/name/);
  });

  it("aponta os papéis que escrevem quando o viewer tenta criar", async () => {
    client = await MarkuppToolClient.connect(
      new FakeMarkuppApi().withTenant("t1", "viewer"),
    );
    const result = await client.call("create_vault", {
      tenant: "t1",
      name: "Projeto",
    });
    expect(result.text).toMatch(/^read_only: .*role owner, admin or member/);
  });
});

describe("rename_vault", () => {
  it("troca o nome e devolve o cofre", async () => {
    const api = new FakeMarkuppApi().withTenant("t1");
    const vault = api.withVault("t1", "Casa");
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("rename_vault", {
      tenant: "t1",
      vault_id: vault.id,
      name: "Lar",
    });
    expect(JSON.parse(result.text)).toEqual({ ...vault, name: "Lar" });
  });

  it("aponta list_vaults quando o cofre não está ao alcance", async () => {
    client = await MarkuppToolClient.connect(
      new FakeMarkuppApi().withTenant("t1"),
    );
    const result = await client.call("rename_vault", {
      tenant: "t1",
      vault_id: "v9",
      name: "Lar",
    });
    expect(result.isError).toBe(true);
    expect(result.text).toMatch(/^vault_forbidden: .*list_vaults/);
  });

  it("anuncia escrita idempotente e não destrutiva", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    expect((await client.tool("rename_vault")).annotations).toMatchObject({
      readOnlyHint: false,
      destructiveHint: false,
      idempotentHint: true,
    });
  });
});

describe("delete_vault", () => {
  it("apaga o cofre com as notas dele", async () => {
    const api = new FakeMarkuppApi().withTenant("t1");
    const vault = api.withVault("t1", "Casa");
    api.withNote("t1", "a.md", "x", vault.id);
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("delete_vault", {
      tenant: "t1",
      vault_id: vault.id,
    });
    expect(JSON.parse(result.text)).toEqual({ deleted: vault.id });
    expect(await api.listVaults("t1")).toEqual([]);
    expect(await api.listNotes("t1")).toEqual([]);
  });

  it("avisa na descrição que as notas vão junto", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    const tool = await client.tool("delete_vault");
    expect(tool.description).toMatch(/notes/);
    expect(tool.annotations).toMatchObject({
      destructiveHint: true,
      idempotentHint: true,
    });
  });
});
