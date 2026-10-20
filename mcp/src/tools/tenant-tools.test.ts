import { afterEach, describe, expect, it } from "vitest";
import { FakeMarkuppApi } from "../testing/fake-markupp-api.js";
import { MarkuppToolClient } from "../testing/markupp-tool-client.js";

let client: MarkuppToolClient;

afterEach(async () => {
  await client.close();
});

describe("list_tenants", () => {
  it("devolve id, nome e papel de cada tenant da conta", async () => {
    const api = new FakeMarkuppApi()
      .withTenant("t1", "owner", "Casa")
      .withTenant("t2", "viewer", "Time");
    client = await MarkuppToolClient.connect(api);
    const result = await client.call("list_tenants", {});
    expect(result.isError).toBe(false);
    expect(JSON.parse(result.text)).toEqual([
      { id: "t1", name: "Casa", role: "owner" },
      { id: "t2", name: "Time", role: "viewer" },
    ]);
  });

  it("pede para reconectar quando a sessão MCP não tem login", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi(), false);
    const result = await client.call("list_tenants", {});
    expect(result.isError).toBe(true);
    expect(result.text).toMatch(/^unauthenticated: .*sign in/);
  });

  it("anuncia que só lê", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    expect((await client.tool("list_tenants")).annotations).toMatchObject({
      readOnlyHint: true,
    });
  });
});
