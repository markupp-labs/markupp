import { describe, expect, it } from "vitest";
import { HttpMarkuppApi } from "./markupp-api.js";
import { FakeApiTokens } from "./testing/fake-api-tokens.js";
import { FakeFetch } from "./testing/fake-fetch.js";

const loginFields = {
  name: "GitLab",
  issuer: "https://gitlab.example",
  client_id: "markupp",
  enabled: true,
};

const creation = { ...loginFields, id: "gl", client_secret: "s" };

describe("HttpMarkuppApi login providers", () => {
  it.each([
    [
      "lista",
      (api: HttpMarkuppApi) => api.listLoginProviders(),
      "GET /v1/instance/login-providers",
      null,
    ],
    [
      "cria",
      (api: HttpMarkuppApi) => api.createLoginProvider(creation),
      "POST /v1/instance/login-providers",
      creation,
    ],
    [
      "troca",
      (api: HttpMarkuppApi) => api.updateLoginProvider("g/l", loginFields),
      "PUT /v1/instance/login-providers/g%2Fl",
      loginFields,
    ],
    [
      "apaga",
      (api: HttpMarkuppApi) => api.deleteLoginProvider("gl"),
      "DELETE /v1/instance/login-providers/gl",
      null,
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
    expect(fake.requests).toHaveLength(1);
    expect(`${request?.method} ${request?.url}`).toBe(
      line.replace(" ", " http://api.test"),
    );
    expect(request?.headers.has("x-markupp-tenant")).toBe(false);
    expect(body === null ? null : await request?.json()).toEqual(body);
  });
});
