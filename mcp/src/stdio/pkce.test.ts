import { describe, expect, it } from "vitest";
import { createPkcePair, randomUrlToken, s256Challenge } from "./pkce.js";

describe("pkce", () => {
  it("calcula o challenge S256 do exemplo da RFC 7636", () => {
    const verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk";
    expect(s256Challenge(verifier)).toBe(
      "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM",
    );
  });

  it("gera o par com o challenge do próprio verifier", () => {
    const pair = createPkcePair();
    expect(pair.verifier).toMatch(/^[A-Za-z0-9_-]{43}$/);
    expect(pair.challenge).toBe(s256Challenge(pair.verifier));
  });

  it("não repete o token aleatório", () => {
    expect(randomUrlToken()).not.toBe(randomUrlToken());
  });
});
