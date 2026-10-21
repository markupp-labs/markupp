import { createHash, randomBytes } from "node:crypto";

// PkcePair é o verifier que fica no processo e o challenge S256 que vai no authorize
// (RFC 7636).
export type PkcePair = { verifier: string; challenge: string };

// randomUrlToken são 32 bytes aleatórios em base64url, 43 caracteres: serve de
// verifier e de state.
export function randomUrlToken(): string {
  return randomBytes(32).toString("base64url");
}

export function s256Challenge(verifier: string): string {
  return createHash("sha256").update(verifier).digest("base64url");
}

export function createPkcePair(): PkcePair {
  const verifier = randomUrlToken();
  return { verifier, challenge: s256Challenge(verifier) };
}
