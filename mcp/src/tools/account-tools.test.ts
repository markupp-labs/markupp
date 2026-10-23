import { afterEach, describe, expect, it } from "vitest";
import { FakeMarkuppApi } from "../testing/fake-markupp-api.js";
import { FakeAccountEmail, FakeAccountId } from "../testing/fake-vaults.js";
import { MarkuppToolClient } from "../testing/markupp-tool-client.js";

let client: MarkuppToolClient;

afterEach(async () => {
  await client.close();
});

describe("whoami", () => {
  it("devolve o id e o email da conta do token", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    const result = await client.call("whoami", {});
    expect(result.isError).toBe(false);
    expect(JSON.parse(result.text)).toEqual({
      id: FakeAccountId,
      email: FakeAccountEmail,
    });
  });

  it("anuncia que só lê e cita remove_vault_member", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi());
    const tool = await client.tool("whoami");
    expect(tool.annotations).toMatchObject({ readOnlyHint: true });
    expect(tool.description).toMatch(/remove_vault_member/);
  });

  it("pede para reconectar quando a sessão MCP não tem login", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi(), false);
    const result = await client.call("whoami", {});
    expect(result.text).toMatch(/^unauthenticated: .*sign in/);
  });
});
