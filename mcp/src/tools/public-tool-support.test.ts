import { UserError } from "fastmcp";
import { describe, expect, it } from "vitest";
import { MarkuppApiError } from "../markupp-api.js";
import { callPublicApi } from "./public-tool-support.js";

function failWith(status: number, code: string): () => Promise<never> {
  return () => Promise.reject(new MarkuppApiError(status, code, "msg"));
}

describe("callPublicApi", () => {
  it("pede para esperar quando a API limita o IP", async () => {
    const failure = callPublicApi(failWith(429, "rate_limited"));
    await expect(failure).rejects.toBeInstanceOf(UserError);
    await expect(failure).rejects.toThrow(/^rate_limited: msg\. Wait/);
  });

  it("não sugere as ferramentas autenticadas", async () => {
    await expect(
      callPublicApi(failWith(403, "tenant_forbidden")),
    ).rejects.toThrow(/^tenant_forbidden: msg$/);
  });

  it("deixa passar a falha que não é da API", async () => {
    const network = new TypeError("fetch failed");
    await expect(callPublicApi(() => Promise.reject(network))).rejects.toBe(
      network,
    );
  });
});
