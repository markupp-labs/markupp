import { afterEach, describe, expect, it } from "vitest";
import { FakeMarkuppApi } from "../testing/fake-markupp-api.js";
import { MarkuppToolClient } from "../testing/markupp-tool-client.js";

let client: MarkuppToolClient;

afterEach(async () => {
  await client.close();
});

const gitlab = {
  name: "GitLab",
  issuer: "https://gitlab.example",
  client_id: "markupp",
  enabled: true,
};

async function adminClient(api = new FakeMarkuppApi()): Promise<void> {
  client = await MarkuppToolClient.connect(api.withInstanceAdmin());
}

describe("list_login_providers", () => {
  it("lista os provedores sem o segredo", async () => {
    await adminClient(new FakeMarkuppApi().withLoginProvider("google", 3));
    const result = await client.call("list_login_providers", {});
    expect(result.isError).toBe(false);
    expect(result.text).not.toContain("seeded-secret");
    expect(JSON.parse(result.text)).toEqual({
      locked: false,
      providers: [
        {
          id: "google",
          name: "google",
          issuer: "https://google.test",
          client_id: "google-client",
          client_secret_set: true,
          enabled: true,
          locked: false,
          callback_url: "https://markupp.test/v1/auth/oidc/google/callback",
          linked_accounts: 3,
        },
      ],
    });
  });

  it("recusa quem não administra a instância", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    const result = await client.call("list_login_providers", {});
    expect(result.isError).toBe(true);
    expect(result.text).toMatch(/^instance_admin_required: /);
  });

  it("anuncia que só lê", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    expect(
      (await client.tool("list_login_providers")).annotations,
    ).toMatchObject({ readOnlyHint: true });
  });
});

describe("create_login_provider", () => {
  // O GitHub não fala OIDC e vai sem issuer.
  it("aceita o github sem issuer", async () => {
    await adminClient();
    const result = await client.call("create_login_provider", {
      id: "github",
      name: "GitHub",
      client_id: "gh",
      client_secret: "gh-secret",
      enabled: true,
    });
    expect(result.isError).toBe(false);
    expect(JSON.parse(result.text)).toMatchObject({ id: "github", issuer: "" });
  });

  it("cria o provedor e devolve só que há segredo", async () => {
    await adminClient();
    const result = await client.call("create_login_provider", {
      id: "gitlab",
      ...gitlab,
      client_secret: "oidc-secret",
    });
    expect(result.isError).toBe(false);
    expect(result.text).not.toContain("oidc-secret");
    expect(JSON.parse(result.text)).toMatchObject({
      id: "gitlab",
      ...gitlab,
      client_secret_set: true,
      linked_accounts: 0,
    });
  });

  it("recusa id que já existe", async () => {
    await adminClient(new FakeMarkuppApi().withLoginProvider("gitlab"));
    const result = await client.call("create_login_provider", {
      id: "gitlab",
      ...gitlab,
      client_secret: "s",
    });
    expect(result.isError).toBe(true);
    expect(result.text).toMatch(/^login_provider_exists: /);
  });

  it("repassa a recusa de provedor inválido", async () => {
    await adminClient();
    const result = await client.call("create_login_provider", {
      id: "gitlab",
      ...gitlab,
      issuer: "http://gitlab.example",
      client_secret: "s",
    });
    expect(result.isError).toBe(true);
    expect(result.text).toMatch(/^invalid_login_provider: /);
  });

  it("recusa quando quem hospeda trava os provedores", async () => {
    await adminClient(new FakeMarkuppApi().withLoginProvidersLockedByHost());
    const result = await client.call("create_login_provider", {
      id: "gitlab",
      ...gitlab,
      client_secret: "s",
    });
    expect(result.isError).toBe(true);
    expect(result.text).toMatch(/^host_locked: /);
  });

  it("exige o segredo antes de chamar a API", async () => {
    await adminClient();
    const call = client.call("create_login_provider", {
      id: "gitlab",
      ...gitlab,
    });
    await expect(call).rejects.toThrow(/client_secret/);
  });
});

describe("update_login_provider", () => {
  it("sem client_secret troca os campos e mantém o segredo", async () => {
    await adminClient(new FakeMarkuppApi().withLoginProvider("gitlab"));
    const result = await client.call("update_login_provider", {
      id: "gitlab",
      ...gitlab,
      enabled: false,
    });
    expect(result.isError).toBe(false);
    expect(JSON.parse(result.text)).toMatchObject({
      id: "gitlab",
      ...gitlab,
      enabled: false,
      client_secret_set: true,
    });
  });

  it("não devolve o segredo novo", async () => {
    await adminClient(new FakeMarkuppApi().withLoginProvider("gitlab"));
    const result = await client.call("update_login_provider", {
      id: "gitlab",
      ...gitlab,
      client_secret: "rotated-secret",
    });
    expect(result.isError).toBe(false);
    expect(result.text).not.toContain("rotated-secret");
  });

  it("exige os campos que a API substitui", async () => {
    await adminClient(new FakeMarkuppApi().withLoginProvider("gitlab"));
    const call = client.call("update_login_provider", {
      id: "gitlab",
      enabled: false,
    });
    await expect(call).rejects.toThrow(/name/);
  });

  it("recusa provedor que não existe", async () => {
    await adminClient();
    const result = await client.call("update_login_provider", {
      id: "nope",
      ...gitlab,
    });
    expect(result.isError).toBe(true);
    expect(result.text).toMatch(/^login_provider_not_found: /);
  });

  it("recusa trocar o issuer com conta ligada", async () => {
    await adminClient(new FakeMarkuppApi().withLoginProvider("gitlab", 2));
    const result = await client.call("update_login_provider", {
      id: "gitlab",
      ...gitlab,
    });
    expect(result.isError).toBe(true);
    expect(result.text).toMatch(/^login_provider_issuer_locked: /);
  });

  it("anuncia escrita idempotente", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    expect(
      (await client.tool("update_login_provider")).annotations,
    ).toMatchObject({ readOnlyHint: false, idempotentHint: true });
  });
});

describe("delete_login_provider", () => {
  it("apaga o provedor e conta as ligações removidas", async () => {
    await adminClient(new FakeMarkuppApi().withLoginProvider("gitlab", 2));
    const result = await client.call("delete_login_provider", { id: "gitlab" });
    expect(result.isError).toBe(false);
    expect(JSON.parse(result.text)).toEqual({ removed_links: 2 });
    const list = await client.call("list_login_providers", {});
    expect(JSON.parse(list.text)).toMatchObject({ providers: [] });
  });

  it("recusa provedor que não existe", async () => {
    await adminClient();
    const result = await client.call("delete_login_provider", { id: "nope" });
    expect(result.isError).toBe(true);
    expect(result.text).toMatch(/^login_provider_not_found: /);
  });

  it("anuncia que destrói e avisa que não tem volta", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    const tool = await client.tool("delete_login_provider");
    expect(tool.annotations).toMatchObject({ destructiveHint: true });
    expect(tool.description).toMatch(/every account/);
    expect(tool.description).toMatch(/cannot be undone/);
  });
});
