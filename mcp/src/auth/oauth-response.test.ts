import { describe, expect, it } from "vitest";
import { oauthErrorIn, readBody } from "./oauth-response.js";

describe("readBody", () => {
  it("lê o JSON e guarda o trecho do texto", async () => {
    const body = await readBody(new Response('{"a":1}'));
    expect(body).toEqual({ excerpt: '{"a":1}', json: { a: 1 } });
  });

  it("devolve json undefined quando o corpo não é JSON", async () => {
    const body = await readBody(new Response("<html>"));
    expect(body).toEqual({ excerpt: "<html>", json: undefined });
  });
});

describe("oauthErrorIn", () => {
  it("lê o código e a descrição do erro do protocolo", () => {
    const json = { error: "invalid_grant", error_description: "usado" };
    expect(oauthErrorIn({ excerpt: "", json })).toEqual({
      code: "invalid_grant",
      description: "usado",
    });
  });

  it("cita o trecho do corpo fora do formato", () => {
    const fields = oauthErrorIn({ excerpt: "<html>", json: undefined });
    expect(fields.code).toBe("sem_codigo_oauth");
    expect(fields.description).toContain('"<html>"');
  });
});
