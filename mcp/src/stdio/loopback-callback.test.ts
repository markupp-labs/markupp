import { describe, expect, it } from "vitest";
import { callbackReply, listenOnLoopback } from "./loopback-callback.js";

function callbackUrl(query: string): URL {
  return new URL(`http://127.0.0.1:5/callback?${query}`);
}

describe("callbackReply", () => {
  it("aceita o retorno com o state esperado", () => {
    const answer = callbackReply(callbackUrl("code=c&state=s"), "s");
    expect(answer).toMatchObject({ status: 200, accepted: true });
    expect(answer.html).toContain("Pode fechar esta aba");
  });

  it("recusa o state diferente e segue esperando", () => {
    const answer = callbackReply(callbackUrl("code=c&state=x"), "s");
    expect(answer).toMatchObject({ status: 400, accepted: false });
  });

  it("aceita o erro OAuth com o state esperado", () => {
    const answer = callbackReply(
      callbackUrl("error=access_denied&state=s"),
      "s",
    );
    expect(answer).toMatchObject({ status: 200, accepted: true });
    expect(answer.html).toContain("Login recusado");
  });

  it("responde 404 fora da rota do callback", () => {
    const answer = callbackReply(new URL("http://127.0.0.1:5/x?state=s"), "s");
    expect(answer).toMatchObject({ status: 404, accepted: false });
  });
});

// Este teste sobe o servidor de verdade em 127.0.0.1, que é a camada fina do callback.
describe("listenOnLoopback", () => {
  it("entrega a query do primeiro retorno com o state certo", async () => {
    const callback = await listenOnLoopback("s");
    try {
      expect(callback.redirectUri).toMatch(
        /^http:\/\/127\.0\.0\.1:\d+\/callback$/,
      );
      const stray = await fetch(`${callback.redirectUri}?code=x&state=outro`);
      expect(stray.status).toBe(400);
      const back = await fetch(`${callback.redirectUri}?code=c&state=s`);
      expect(back.status).toBe(200);
      expect((await callback.received).get("code")).toBe("c");
    } finally {
      await callback.close();
    }
  });
});
