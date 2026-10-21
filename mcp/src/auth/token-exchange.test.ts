import { describe, expect, it } from "vitest";
import { FakeClock } from "../testing/fake-clock.js";
import { exchangeConfig } from "../testing/exchange-config.js";
import { FakeFetch } from "../testing/fake-fetch.js";
import {
  ACCESS_TOKEN_TYPE,
  TOKEN_EXCHANGE_GRANT,
  TokenExchangeError,
  exchangeAgentToken,
  resourceBasicAuth,
  type TokenExchangeConfig,
} from "./token-exchange.js";

const RESOURCE = "https://mcp.exemplo/mcp";
const SECRET = "s+creto x:%ç";

function configFor(fetcher: FakeFetch, clock: FakeClock): TokenExchangeConfig {
  return exchangeConfig(fetcher, clock, {
    resourceUrl: RESOURCE,
    resourceSecret: SECRET,
  });
}

// decodeBasic desfaz o Basic como o Go: base64, separa no primeiro ":" e decodifica.
function decodeBasic(header: string | null): [string, string] {
  const raw = atob((header ?? "").replace(/^Basic /, ""));
  const colon = raw.indexOf(":");
  return [
    decodeURIComponent(raw.slice(0, colon)),
    decodeURIComponent(raw.slice(colon + 1)),
  ];
}

async function caughtError(promise: Promise<unknown>): Promise<unknown> {
  return promise.then(
    () => undefined,
    (error: unknown) => error,
  );
}

describe("exchangeAgentToken", () => {
  it("manda o formulário RFC 8693 ao endpoint de token do issuer", async () => {
    const fetcher = new FakeFetch().respondWith(200, {
      access_token: "jwt",
      expires_in: 300,
    });
    await exchangeAgentToken(configFor(fetcher, new FakeClock()), "agente");
    const request = fetcher.requests[0]!;
    expect(`${request.method} ${request.url}`).toBe(
      "POST https://markupp.exemplo/oauth/token",
    );
    expect(request.headers.get("content-type")).toBe(
      "application/x-www-form-urlencoded",
    );
    expect(
      Object.fromEntries(new URLSearchParams(await request.text())),
    ).toEqual({
      grant_type: TOKEN_EXCHANGE_GRANT,
      subject_token: "agente",
      subject_token_type: ACCESS_TOKEN_TYPE,
    });
  });

  it("autentica com Basic de URL e segredo percent-encoded", async () => {
    const fetcher = new FakeFetch().respondWith(200, {
      access_token: "jwt",
      expires_in: 300,
    });
    await exchangeAgentToken(configFor(fetcher, new FakeClock()), "agente");
    const header = fetcher.requests[0]!.headers.get("authorization");
    expect(atob(header!.replace(/^Basic /, ""))).toBe(
      "https%3A%2F%2Fmcp.exemplo%2Fmcp:s%2Bcreto%20x%3A%25%C3%A7",
    );
    expect(decodeBasic(header)).toEqual([RESOURCE, SECRET]);
  });

  it("devolve o JWT e o instante de expiração pelo relógio", async () => {
    const clock = new FakeClock(10_000);
    const fetcher = new FakeFetch().respondWith(200, {
      access_token: "jwt",
      issued_token_type: ACCESS_TOKEN_TYPE,
      token_type: "Bearer",
      expires_in: 300,
      scope: "notas:leitura",
    });
    const exchanged = await exchangeAgentToken(configFor(fetcher, clock), "a");
    expect(exchanged).toEqual({ accessToken: "jwt", expiresAtMs: 310_000 });
  });

  it("recusa com o código OAuth da resposta", async () => {
    const fetcher = new FakeFetch().respondWith(400, {
      error: "invalid_grant",
      error_description: "subject_token invalido",
    });
    const error = await caughtError(
      exchangeAgentToken(configFor(fetcher, new FakeClock()), "agente"),
    );
    expect(error).toBeInstanceOf(TokenExchangeError);
    expect(error).toMatchObject({
      httpStatus: 400,
      oauthError: "invalid_grant",
    });
  });

  it("recusa sem código citando o corpo JSON que não é OAuth", async () => {
    const fetcher = new FakeFetch().respondWith(502, "gateway");
    const error = await caughtError(
      exchangeAgentToken(configFor(fetcher, new FakeClock()), "agente"),
    );
    expect(error).toMatchObject({
      httpStatus: 502,
      oauthError: "sem_codigo_oauth",
    });
    expect((error as Error).message).toContain('"\\"gateway\\""');
  });

  it("recusa sem código quando o corpo do erro nem é JSON", async () => {
    const fetcher = new FakeFetch().respondWith(502);
    const error = await caughtError(
      exchangeAgentToken(configFor(fetcher, new FakeClock()), "agente"),
    );
    expect(error).toMatchObject({
      httpStatus: 502,
      oauthError: "sem_codigo_oauth",
    });
  });

  it("falha citando status e formato quando o 200 não é JSON", async () => {
    const fetcher = new FakeFetch().respondWith(200);
    await expect(
      exchangeAgentToken(configFor(fetcher, new FakeClock()), "agente"),
    ).rejects.toThrow(
      'resposta da troca de token nao e JSON: status 200, corpo ""; esperado {access_token: string',
    );
  });

  it("falha nomeando o formato quando o 200 vem sem access_token", async () => {
    const fetcher = new FakeFetch().respondWith(200, { expires_in: 300 });
    await expect(
      exchangeAgentToken(configFor(fetcher, new FakeClock()), "agente"),
    ).rejects.toThrow("esperado {access_token: string");
  });
});

describe("resourceBasicAuth", () => {
  it("codifica antes do base64 para o ':' da URL não partir a credencial", () => {
    expect(resourceBasicAuth("https://a", "b")).toBe(
      `Basic ${btoa("https%3A%2F%2Fa:b")}`,
    );
  });
});
