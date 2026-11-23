import { afterEach, describe, expect, it } from "vitest";
import type { PublicSession } from "../session.js";
import { FakePublicMarkuppApi } from "../testing/fake-public-markupp-api.js";
import { MarkuppToolClient } from "../testing/markupp-tool-client.js";

let client: MarkuppToolClient<PublicSession>;

afterEach(async () => {
  await client.close();
});

describe("resource markupp-public://notes/{public_id}", () => {
  it("devolve o markdown da nota pública como text/markdown", async () => {
    const api = new FakePublicMarkuppApi();
    const note = api.withNote("guia", "Start.md", "# Start\n\n- item");
    client = await MarkuppToolClient.connectPublic(api);
    const uri = `markupp-public://notes/${note.public_id}`;
    expect(await client.read(uri)).toEqual({
      text: "# Start\n\n- item",
      mimeType: "text/markdown",
      uri,
    });
  });

  it("falha com o código e o próximo passo quando o id não é público", async () => {
    client = await MarkuppToolClient.connectPublic(new FakePublicMarkuppApi());
    await expect(
      client.read("markupp-public://notes/naoexiste123"),
    ).rejects.toThrow(/not_found: .*public URL/);
  });

  it("recusa o id fora do formato antes de chamar a API", async () => {
    const api = new FakePublicMarkuppApi();
    client = await MarkuppToolClient.connectPublic(api);
    await expect(client.read("markupp-public://notes/curto")).rejects.toThrow(
      /public_id "curto" .*A-Za-z0-9/,
    );
    expect(api.calls).toEqual([]);
  });
});
