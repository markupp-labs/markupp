import { afterEach, describe, expect, it } from "vitest";
import { FakeMarkuppApi } from "../testing/fake-markupp-api.js";
import { MarkuppToolClient } from "../testing/markupp-tool-client.js";

let client: MarkuppToolClient;

afterEach(async () => {
  await client.close();
});

const remote = {
  provider: "openai-compat",
  url: "https://api.example/v1",
  model: "text-embedding-3-small",
};

describe("get_embedding_provider", () => {
  it("devolve a escolha da instância sem a chave", async () => {
    client = await MarkuppToolClient.connect(
      new FakeMarkuppApi().withInstanceAdmin(),
    );
    const result = await client.call("get_embedding_provider", {});
    expect(result.isError).toBe(false);
    expect(JSON.parse(result.text)).toEqual({
      provider: "none",
      url: "",
      model: "",
      api_key_set: false,
      locked: false,
      host_provider: "",
      model_status: null,
    });
  });

  it("recusa quem não administra a instância", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    const result = await client.call("get_embedding_provider", {});
    expect(result.isError).toBe(true);
    expect(result.text).toMatch(/^instance_admin_required: /);
  });

  it("anuncia que só lê", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    expect(
      (await client.tool("get_embedding_provider")).annotations,
    ).toMatchObject({ readOnlyHint: true });
  });
});

describe("set_embedding_provider", () => {
  it("grava o provedor remoto com a chave e devolve só que há chave", async () => {
    client = await MarkuppToolClient.connect(
      new FakeMarkuppApi().withInstanceAdmin(),
    );
    const result = await client.call("set_embedding_provider", {
      ...remote,
      api_key: "sk-secret",
    });
    expect(result.isError).toBe(false);
    expect(result.text).not.toContain("sk-secret");
    expect(JSON.parse(result.text)).toMatchObject({
      ...remote,
      api_key_set: true,
    });
  });

  it("sem api_key mantém a chave, e com api_key vazia a apaga", async () => {
    client = await MarkuppToolClient.connect(
      new FakeMarkuppApi().withInstanceAdmin(),
    );
    await client.call("set_embedding_provider", { ...remote, api_key: "sk" });
    const kept = await client.call("set_embedding_provider", remote);
    expect(JSON.parse(kept.text)).toMatchObject({ api_key_set: true });
    const cleared = await client.call("set_embedding_provider", {
      ...remote,
      api_key: "",
    });
    expect(JSON.parse(cleared.text)).toMatchObject({ api_key_set: false });
  });

  it("recusa quando quem hospeda trava o provedor", async () => {
    client = await MarkuppToolClient.connect(
      new FakeMarkuppApi().withInstanceAdmin().withEmbeddingLockedByHost(),
    );
    const result = await client.call("set_embedding_provider", {
      provider: "local",
    });
    expect(result.isError).toBe(true);
    expect(result.text).toMatch(/^host_locked: /);
  });

  it("recusa provedor fora da lista antes de chamar a API", async () => {
    client = await MarkuppToolClient.connect(
      new FakeMarkuppApi().withInstanceAdmin(),
    );
    const result = await client.call("set_embedding_provider", {
      provider: "ollama",
    });
    expect(result.isError).toBe(true);
  });

  it("anuncia escrita idempotente", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    expect(
      (await client.tool("set_embedding_provider")).annotations,
    ).toMatchObject({ readOnlyHint: false, idempotentHint: true });
  });
});
