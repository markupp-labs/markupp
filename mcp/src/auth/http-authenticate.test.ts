import { describe, expect, it } from "vitest";
import { FakeClock } from "../testing/fake-clock.js";
import { exchangeConfig } from "../testing/exchange-config.js";
import { FakeFetch } from "../testing/fake-fetch.js";
import { RecordingAuthLog } from "../testing/recording-auth-log.js";
import { ApiTokenCache } from "./api-token-cache.js";
import {
  bearerToken,
  createHttpAuthenticate,
  exchangeFailureLog,
  type HttpAuthenticate,
} from "./http-authenticate.js";
import { TokenExchangeError, exchangeAgentToken } from "./token-exchange.js";

const RESOURCE = "https://mcp.exemplo";
const METADATA = `resource_metadata="${RESOURCE}/.well-known/oauth-protected-resource"`;

class AuthenticateHarness {
  readonly fetcher = new FakeFetch();
  readonly clock = new FakeClock();
  readonly log = new RecordingAuthLog();
  readonly authenticate: HttpAuthenticate;

  constructor(resourceUrl: string = RESOURCE) {
    const config = exchangeConfig(this.fetcher, this.clock, {
      resourceUrl,
    });
    const tokens = new ApiTokenCache(
      (t) => exchangeAgentToken(config, t),
      this.clock.now,
      10,
    );
    this.authenticate = createHttpAuthenticate({
      resourceUrl,
      tokens,
      log: this.log.write,
    });
  }

  rejection(authorization?: string): Promise<Response> {
    const headers = authorization === undefined ? {} : { authorization };
    return this.authenticate({ headers }).then(
      () => {
        throw new Error("authenticate aceitou; esperado Response lançada");
      },
      (thrown: unknown) => thrown as Response,
    );
  }
}

describe("createHttpAuthenticate", () => {
  it("responde 401 sem error quando não há token", async () => {
    const response = await new AuthenticateHarness().rejection();
    expect(response.status).toBe(401);
    expect(response.headers.get("www-authenticate")).toBe(`Bearer ${METADATA}`);
  });

  it("aponta a metadata com o caminho do resource depois do well-known", async () => {
    const harness = new AuthenticateHarness("https://mcp.exemplo/mcp");
    const response = await harness.rejection();
    expect(response.headers.get("www-authenticate")).toBe(
      'Bearer resource_metadata="https://mcp.exemplo/.well-known/oauth-protected-resource/mcp"',
    );
  });

  it("trata outro esquema como requisição sem token", async () => {
    const response = await new AuthenticateHarness().rejection("Basic abc");
    expect(response.headers.get("www-authenticate")).toBe(`Bearer ${METADATA}`);
  });

  it("responde 401 com invalid_token quando a troca dá invalid_grant", async () => {
    const harness = new AuthenticateHarness();
    harness.fetcher.respondWith(400, {
      error: "invalid_grant",
      error_description: "x",
    });
    const response = await harness.rejection("Bearer revogado");
    expect(response.status).toBe(401);
    expect(response.headers.get("www-authenticate")).toBe(
      `Bearer ${METADATA}, error="invalid_token"`,
    );
    expect(harness.log.entries).toEqual([]);
  });

  it("responde 500 e loga erro de configuração quando a troca dá invalid_client", async () => {
    const harness = new AuthenticateHarness();
    harness.fetcher.respondWith(401, {
      error: "invalid_client",
      error_description: "x",
    });
    const response = await harness.rejection("Bearer token-do-agente");
    expect(response.status).toBe(500);
    expect(response.headers.get("www-authenticate")).toBeNull();
    expect(harness.log.entries).toMatchObject([
      { level: "error", oauth_error: "invalid_client", http_status: 401 },
    ]);
    expect(JSON.stringify(harness.log.entries)).not.toContain(
      "token-do-agente",
    );
  });

  it("devolve sessão cujo apiToken troca de novo depois que o JWT expira", async () => {
    const harness = new AuthenticateHarness();
    harness.fetcher
      .respondWith(200, { access_token: "jwt-1", expires_in: 300 })
      .respondWith(200, { access_token: "jwt-2", expires_in: 300 });
    const session = await harness.authenticate({
      headers: { authorization: "Bearer ag" },
    });
    expect(await session.apiToken()).toBe("jwt-1");
    harness.clock.advance(300_000);
    expect(await session.apiToken()).toBe("jwt-2");
    expect(harness.fetcher.requests).toHaveLength(2);
  });
});

describe("apiToken da sessão", () => {
  async function sessionRenewingWith(
    harness: AuthenticateHarness,
    refusal: { status: number; error: string },
  ): Promise<() => Promise<string>> {
    harness.fetcher
      .respondWith(200, { access_token: "jwt-1", expires_in: 300 })
      .respondWith(refusal.status, {
        error: refusal.error,
        error_description: "x",
      });
    const session = await harness.authenticate({
      headers: { authorization: "Bearer token-do-agente" },
    });
    harness.clock.advance(300_000);
    return session.apiToken;
  }

  it("repassa a recusa invalid_grant na renovação, que pede novo login", async () => {
    const harness = new AuthenticateHarness();
    const apiToken = await sessionRenewingWith(harness, {
      status: 400,
      error: "invalid_grant",
    });
    await expect(apiToken()).rejects.toBeInstanceOf(TokenExchangeError);
    expect(harness.log.entries).toEqual([]);
  });

  it("loga e esconde do agente a falha de renovação que não é do token dele", async () => {
    const harness = new AuthenticateHarness();
    const apiToken = await sessionRenewingWith(harness, {
      status: 401,
      error: "invalid_client",
    });
    const failure = await apiToken().catch((error: unknown) => error);
    expect(failure).not.toBeInstanceOf(TokenExchangeError);
    expect(String(failure)).toMatch(/^Error: internal error: /);
    expect(harness.log.entries).toMatchObject([
      { oauth_error: "invalid_client" },
    ]);
    expect(JSON.stringify(harness.log.entries)).not.toContain(
      "token-do-agente",
    );
  });
});

describe("bearerToken", () => {
  it("lê o token sem diferenciar maiúsculas no esquema", () => {
    expect(bearerToken("bearer abc")).toBe("abc");
    expect(bearerToken("Bearer ")).toBeUndefined();
    expect(bearerToken(undefined)).toBeUndefined();
  });

  it("recusa valor com espaço, que não é um token68", () => {
    expect(bearerToken("Bearer a b")).toBeUndefined();
  });
});

describe("exchangeFailureLog", () => {
  it("registra falha que não veio do issuer só com a mensagem", () => {
    expect(exchangeFailureLog(new Error("rede fora"))).toEqual({
      level: "error",
      msg: "troca de token falhou",
      error: "rede fora",
    });
  });

  it("nomeia a recusa genérica do issuer pelo código", () => {
    const entry = exchangeFailureLog(
      new TokenExchangeError(400, "invalid_target", "x"),
    );
    expect(entry).toMatchObject({
      msg: "issuer recusou a troca de token",
      oauth_error: "invalid_target",
    });
  });
});
