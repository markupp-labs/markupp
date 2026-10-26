import { afterEach, describe, expect, it } from "vitest";
import type { PublicSession } from "../session.js";
import { FakePublicMarkuppApi } from "../testing/fake-public-markupp-api.js";
import { MarkuppToolClient } from "../testing/markupp-tool-client.js";

let client: MarkuppToolClient<PublicSession>;

afterEach(async () => {
  await client.close();
});

describe("resource markupp-public://{vault}/notes/{id}", () => {
  it("devolve o markdown da nota publicada como text/markdown", async () => {
    const api = new FakePublicMarkuppApi().withVault("docs");
    const note = api.withNote("docs", "Start.md", "# Start\n\n- item");
    client = await MarkuppToolClient.connectPublic(api);
    const uri = `markupp-public://docs/notes/${note.id}`;
    expect(await client.read(uri)).toEqual({
      text: "# Start\n\n- item",
      mimeType: "text/markdown",
      uri,
    });
  });

  it("falha com o código e o próximo passo quando o cofre não está publicado", async () => {
    client = await MarkuppToolClient.connectPublic(new FakePublicMarkuppApi());
    await expect(
      client.read("markupp-public://sumiu/notes/n1"),
    ).rejects.toThrow(/not_found: .*slug/);
  });

  it("recusa o id '..' antes de chamar a API", async () => {
    const api = new FakePublicMarkuppApi().withVault("docs");
    client = await MarkuppToolClient.connectPublic(api);
    await expect(client.read("markupp-public://docs/notes/..")).rejects.toThrow(
      /id "\.\." .*a-z/,
    );
    expect(api.calls).toEqual([]);
  });

  it("recusa o vault fora do formato do slug antes de chamar a API", async () => {
    const api = new FakePublicMarkuppApi().withVault("docs");
    client = await MarkuppToolClient.connectPublic(api);
    await expect(client.read("markupp-public://../notes/n1")).rejects.toThrow(
      /vault "\.\." .*a-z/,
    );
    expect(api.calls).toEqual([]);
  });
});
