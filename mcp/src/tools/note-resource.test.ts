import { afterEach, describe, expect, it } from "vitest";
import { FakeMarkuppApi } from "../testing/fake-markupp-api.js";
import { MarkuppToolClient } from "../testing/markupp-tool-client.js";

let client: MarkuppToolClient;

afterEach(async () => {
  await client.close();
});

describe("resource markupp://{tenant}/notes/{id}", () => {
  it("devolve o markdown da nota como text/markdown", async () => {
    const api = new FakeMarkuppApi().withTenant("t1");
    const note = api.withNote("t1", "a.md", "# Plano\n\n- item");
    client = await MarkuppToolClient.connect(api);
    const uri = `markupp://t1/notes/${note.id}`;
    expect(await client.read(uri)).toEqual({
      text: "# Plano\n\n- item",
      mimeType: "text/markdown",
      uri,
    });
  });

  it("falha com o código e o próximo passo quando a nota não existe", async () => {
    client = await MarkuppToolClient.connect(
      new FakeMarkuppApi().withTenant("t1"),
    );
    await expect(client.read("markupp://t1/notes/n9")).rejects.toThrow(
      /not_found: .*search_notes/,
    );
  });

  it("pede para reconectar quando a sessão MCP não tem login", async () => {
    client = await MarkuppToolClient.connect(new FakeMarkuppApi(), false);
    await expect(client.read("markupp://t1/notes/n1")).rejects.toThrow(
      /unauthenticated: .*sign in/,
    );
  });
});
