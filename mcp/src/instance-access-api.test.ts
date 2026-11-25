import { describe, expect, it } from "vitest";
import { HttpMarkuppApi } from "./markupp-api.js";
import { FakeApiTokens } from "./testing/fake-api-tokens.js";
import { FakeFetch } from "./testing/fake-fetch.js";

type Call = (api: HttpMarkuppApi) => Promise<unknown>;

describe("HttpMarkuppApi sessões, contas e acessos da instância", () => {
  it.each<[string, Call, string, unknown]>([
    [
      "minhas sessões",
      (api) => api.listMySessions(),
      "GET /v1/me/sessions",
      null,
    ],
    [
      "encerra minha sessão",
      (api) => api.endMySession("s/1"),
      "DELETE /v1/me/sessions/s%2F1",
      null,
    ],
    [
      "lista contas com filtro",
      (api) =>
        api.listAccounts({
          q: "ana",
          status: "blocked",
          limit: 10,
          offset: 20,
        }),
      "GET /v1/instance/accounts?q=ana&status=blocked&limit=10&offset=20",
      null,
    ],
    [
      "lista contas sem filtro",
      (api) => api.listAccounts({}),
      "GET /v1/instance/accounts",
      null,
    ],
    [
      "descreve conta",
      (api) => api.getInstanceAccount("u2"),
      "GET /v1/instance/accounts/u2",
      null,
    ],
    [
      "bloqueia conta",
      (api) => api.setAccountBlocked("u2", true),
      "PATCH /v1/instance/accounts/u2",
      { blocked: true },
    ],
    [
      "apaga conta",
      (api) => api.deleteInstanceAccount("u2"),
      "DELETE /v1/instance/accounts/u2",
      null,
    ],
    [
      "sessões da conta",
      (api) => api.listAccountSessions("u2"),
      "GET /v1/instance/accounts/u2/sessions",
      null,
    ],
    [
      "encerra sessão da conta",
      (api) => api.endAccountSession("u2", "s1"),
      "DELETE /v1/instance/accounts/u2/sessions/s1",
      null,
    ],
    [
      "apps da conta",
      (api) => api.listAccountAuthorizations("u2"),
      "GET /v1/instance/accounts/u2/authorizations",
      null,
    ],
    [
      "revoga app da conta",
      (api) => api.revokeAccountAuthorization("u2", "a1"),
      "DELETE /v1/instance/accounts/u2/authorizations/a1",
      null,
    ],
    [
      "lista bloqueios de IP",
      (api) => api.listIpBlocks(),
      "GET /v1/instance/ip-blocks",
      null,
    ],
    [
      "bloqueia IP",
      (api) => api.blockIp({ cidr: "198.51.100.0/24", note: "x" }),
      "POST /v1/instance/ip-blocks",
      { cidr: "198.51.100.0/24", note: "x" },
    ],
    [
      "desbloqueia IP",
      (api) => api.unblockIp("b1"),
      "DELETE /v1/instance/ip-blocks/b1",
      null,
    ],
    [
      "lê o histórico",
      (api) =>
        api.listAuditEvents({
          account: "u2",
          result: "failure",
          since: "2026-11-01T00:00:00Z",
          limit: 5,
        }),
      "GET /v1/instance/audit-events?account=u2&result=failure&since=2026-11-01T00%3A00%3A00Z&limit=5",
      null,
    ],
    [
      "lê o prazo",
      (api) => api.getRetention(),
      "GET /v1/instance/retention",
      null,
    ],
    [
      "troca o prazo",
      (api) => api.setRetention(30),
      "PUT /v1/instance/retention",
      { days: 30 },
    ],
  ])("%s sem tenant", async (_name, call, line, body) => {
    const fake = new FakeFetch().respondWith(200, {});
    await call(
      new HttpMarkuppApi({
        baseUrl: "http://api.test/",
        fetch: fake.fetch,
        apiToken: new FakeApiTokens("jwt-1").source,
      }),
    );
    const [request] = fake.requests;
    expect(`${request?.method} ${request?.url}`).toBe(
      line.replace(" ", " http://api.test"),
    );
    expect(request?.headers.has("x-markupp-tenant")).toBe(false);
    expect(body === null ? null : await request?.json()).toEqual(body);
  });
});
