import { describe, expect, it } from "vitest";
import { FakeClock } from "../testing/fake-clock.js";
import { exchangeConfig } from "../testing/exchange-config.js";
import { FakeFetch } from "../testing/fake-fetch.js";
import {
  ApiTokenCache,
  EXPIRY_MARGIN_MS,
  agentTokenKey,
} from "./api-token-cache.js";
import { exchangeAgentToken } from "./token-exchange.js";

function cacheOver(
  fetcher: FakeFetch,
  clock: FakeClock,
  maxEntries = 10,
): ApiTokenCache {
  const config = exchangeConfig(fetcher, clock);
  return new ApiTokenCache(
    (t) => exchangeAgentToken(config, t),
    clock.now,
    maxEntries,
  );
}

function subjectTokens(fetcher: FakeFetch): Promise<(string | null)[]> {
  return Promise.all(
    fetcher.requests.map(async (r) =>
      new URLSearchParams(await r.clone().text()).get("subject_token"),
    ),
  );
}

describe("ApiTokenCache", () => {
  it("acerta o cache enquanto o JWT vale e não troca de novo", async () => {
    const fetcher = new FakeFetch().respondWith(200, {
      access_token: "jwt-1",
      expires_in: 300,
    });
    const clock = new FakeClock();
    const cache = cacheOver(fetcher, clock);
    expect(await cache.apiTokenFor("agente")).toBe("jwt-1");
    clock.advance(300_000 - EXPIRY_MARGIN_MS - 1);
    expect(await cache.apiTokenFor("agente")).toBe("jwt-1");
    expect(fetcher.requests).toHaveLength(1);
  });

  it("troca de novo quando o JWT entra na margem de expiração", async () => {
    const fetcher = new FakeFetch()
      .respondWith(200, { access_token: "jwt-1", expires_in: 300 })
      .respondWith(200, { access_token: "jwt-2", expires_in: 300 });
    const clock = new FakeClock();
    const cache = cacheOver(fetcher, clock);
    await cache.apiTokenFor("agente");
    clock.advance(300_000 - EXPIRY_MARGIN_MS);
    expect(await cache.apiTokenFor("agente")).toBe("jwt-2");
    expect(await subjectTokens(fetcher)).toEqual(["agente", "agente"]);
  });

  it("guarda pelo SHA-256 do token do agente, nunca em claro", async () => {
    const fetcher = new FakeFetch().respondWith(200, {
      access_token: "jwt",
      expires_in: 300,
    });
    const cache = cacheOver(fetcher, new FakeClock());
    await cache.apiTokenFor("token-do-agente");
    const expected =
      "5a2f47c3578c40721fec2bf1acdc289ef5ab6446cbee3dfb79bbf4886532733d";
    expect(cache.storedKeys()).toEqual([expected]);
    expect(cache.storedKeys().join()).not.toContain("token-do-agente");
  });

  it("despeja a entrada mais antiga ao passar do teto", async () => {
    const fetcher = new FakeFetch()
      .respondWith(200, { access_token: "a", expires_in: 300 })
      .respondWith(200, { access_token: "b", expires_in: 300 })
      .respondWith(200, { access_token: "c", expires_in: 300 });
    const cache = cacheOver(fetcher, new FakeClock(), 2);
    for (const token of ["um", "dois", "tres"]) await cache.apiTokenFor(token);
    expect(cache.storedKeys()).toEqual([
      await agentTokenKey("dois"),
      await agentTokenKey("tres"),
    ]);
  });

  it("recusa teto que não é inteiro positivo, citando o valor", () => {
    expect(() => cacheOver(new FakeFetch(), new FakeClock(), 0)).toThrow(
      "teto do cache de tokens invalido: 0",
    );
  });
});

describe("agentTokenKey", () => {
  it("é o SHA-256 em hex do token", async () => {
    expect(await agentTokenKey("abc")).toBe(
      "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
    );
  });
});
