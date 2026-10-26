import { describe, expect, it } from "vitest";
import { encodeDeeplinkPayload, strictUriEncode } from "./link-encoding.js";

describe("strictUriEncode", () => {
  it("codifica tudo fora de letras, dígitos e -_.~, como o url_encode do Ruby", () => {
    expect(strictUriEncode(`a!'()*b ~-_.:/"{}`)).toBe(
      "a%21%27%28%29%2Ab%20~-_.%3A%2F%22%7B%7D",
    );
  });
});

describe("encodeDeeplinkPayload", () => {
  it("passa o json para base64 e codifica o + / = da base64", () => {
    const encoded = encodeDeeplinkPayload(
      { url: "https://a.b/?>>" },
      "base64-json",
    );
    expect(encoded).not.toMatch(/[+/=]/);
    expect(
      Buffer.from(decodeURIComponent(encoded), "base64").toString("utf8"),
    ).toBe('{"url":"https://a.b/?>>"}');
  });

  it("codifica o json direto na url", () => {
    expect(encodeDeeplinkPayload({ name: "markupp" }, "uri-json")).toBe(
      "%7B%22name%22%3A%22markupp%22%7D",
    );
  });
});
