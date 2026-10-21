import { describe, expect, it } from "vitest";
import {
  authServerMetadata,
  TEST_ISSUER,
} from "../testing/auth-server-metadata.js";
import { FakeClock } from "../testing/fake-clock.js";
import { FakeFetch } from "../testing/fake-fetch.js";
import {
  codeGrantForm,
  discoverAuthServer,
  isInvalidGrant,
  refreshGrantForm,
  requestTokens,
  revokeToken,
} from "./public-client.js";

const TOKEN_ENDPOINT = `${TEST_ISSUER}/oauth/token`;

async function formOf(request: Request): Promise<Record<string, string>> {
  return Object.fromEntries(new URLSearchParams(await request.text()));
}

describe("discoverAuthServer", () => {
  it("lê os endpoints da metadata do issuer", async () => {
    const upstream = new FakeFetch().respondWith(200, authServerMetadata());
    const metadata = await discoverAuthServer(TEST_ISSUER, upstream.fetch);
    expect(metadata.tokenEndpoint).toBe(TOKEN_ENDPOINT);
    expect(metadata.revocationEndpoint).toBe(`${TEST_ISSUER}/oauth/revoke`);
    expect(upstream.requests[0]?.url).toBe(
      `${TEST_ISSUER}/.well-known/oauth-authorization-server`,
    );
  });

  it("recusa o issuer de outra origem", async () => {
    const body = authServerMetadata("https://outro.exemplo");
    const upstream = new FakeFetch().respondWith(200, body);
    await expect(
      discoverAuthServer(TEST_ISSUER, upstream.fetch),
    ).rejects.toThrow(
      'issuer em http://localhost:8080/.well-known/oauth-authorization-server="https://outro.exemplo"',
    );
  });

  it("recusa o endpoint de token fora da origem do issuer", async () => {
    const body = {
      ...authServerMetadata(),
      token_endpoint: "https://ladrao.exemplo/token",
    };
    const upstream = new FakeFetch().respondWith(200, body);
    await expect(
      discoverAuthServer(TEST_ISSUER, upstream.fetch),
    ).rejects.toThrow('token_endpoint="https://ladrao.exemplo/token"');
  });

  it("recusa url http fora de loopback antes de qualquer requisição", async () => {
    const upstream = new FakeFetch();
    await expect(
      discoverAuthServer("http://markupp.exemplo", upstream.fetch),
    ).rejects.toThrow('url do markupp="http://markupp.exemplo"');
    expect(upstream.requests).toHaveLength(0);
  });
});

describe("requestTokens", () => {
  it("posta o code sem Basic e devolve o par com o vencimento", async () => {
    const upstream = new FakeFetch().respondWith(200, {
      access_token: "jwt",
      expires_in: 300,
      refresh_token: "r1",
    });
    const clock = new FakeClock();
    const form = codeGrantForm(
      TEST_ISSUER,
      "c",
      "http://127.0.0.1:5/callback",
      "v",
    );
    const tokens = await requestTokens(TOKEN_ENDPOINT, form, {
      fetch: upstream.fetch,
      clock: clock.now,
    });
    expect(tokens).toEqual({
      accessToken: "jwt",
      expiresAtMs: clock.now() + 300_000,
      refreshToken: "r1",
    });
    const request = upstream.requests[0];
    expect(request?.headers.get("authorization")).toBeNull();
    expect(await formOf(request as Request)).toEqual({
      grant_type: "authorization_code",
      code: "c",
      redirect_uri: "http://127.0.0.1:5/callback",
      client_id: "markupp-mcp",
      code_verifier: "v",
      resource: TEST_ISSUER,
    });
  });

  it("marca o refresh recusado como invalid_grant", async () => {
    const upstream = new FakeFetch().respondWith(400, {
      error: "invalid_grant",
      error_description: "refresh ja usado",
    });
    const deps = { fetch: upstream.fetch, clock: new FakeClock().now };
    const form = refreshGrantForm(TEST_ISSUER, "r1");
    const failure = await requestTokens(TOKEN_ENDPOINT, form, deps).catch(
      (error: unknown) => error,
    );
    expect(isInvalidGrant(failure)).toBe(true);
    expect(String(failure)).toContain("refresh ja usado");
  });

  it("cita só os campos faltando, nunca o token da resposta", async () => {
    const upstream = new FakeFetch().respondWith(200, {
      access_token: "jwt-secreto",
      expires_in: 300,
    });
    const deps = { fetch: upstream.fetch, clock: new FakeClock().now };
    const failure = await requestTokens(TOKEN_ENDPOINT, {}, deps).catch(
      (error: unknown) => String(error),
    );
    expect(failure).toContain("[refresh_token]");
    expect(failure).not.toContain("jwt-secreto");
  });
});

describe("revokeToken", () => {
  it("revoga com o token e o client_id do cliente público", async () => {
    const upstream = new FakeFetch().respondWith(200);
    await revokeToken(`${TEST_ISSUER}/oauth/revoke`, "r1", upstream.fetch);
    expect(await formOf(upstream.requests[0] as Request)).toEqual({
      token: "r1",
      client_id: "markupp-mcp",
    });
  });

  it("falha com o código da recusa", async () => {
    const upstream = new FakeFetch().respondWith(400, {
      error: "invalid_request",
    });
    await expect(
      revokeToken(`${TEST_ISSUER}/oauth/revoke`, "r1", upstream.fetch),
    ).rejects.toThrow("error invalid_request");
  });
});
