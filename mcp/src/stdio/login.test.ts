import { describe, expect, it } from "vitest";
import {
  authServerMetadata,
  TEST_ISSUER,
} from "../testing/auth-server-metadata.js";
import {
  FakeBrowser,
  FakeLoopbackCallback,
  type BrowserOutcome,
} from "../testing/fake-browser-login.js";
import { FakeClock } from "../testing/fake-clock.js";
import { FakeFetch } from "../testing/fake-fetch.js";
import { FakeLocalFiles } from "../testing/fake-local-files.js";
import { CredentialStore } from "./credential-store.js";
import { runLogin, type LoginDeps } from "./login.js";
import { s256Challenge } from "./pkce.js";

const PATH = "/home/ana/.config/markupp/credentials.json";

// LoginRig liga o login ao Go, ao disco, ao navegador e ao callback falsos; o prazo do
// callback anda num relógio próprio, para o vencimento não mexer no do token.
class LoginRig {
  readonly clock = new FakeClock();
  readonly deadline = new FakeClock();
  readonly upstream = new FakeFetch().respondWith(200, authServerMetadata());
  readonly store = new CredentialStore(
    PATH,
    new FakeLocalFiles(this.clock.now),
  );
  readonly callback = new FakeLoopbackCallback();
  readonly stderr: string[] = [];
  readonly browser: FakeBrowser;

  constructor(outcome: BrowserOutcome) {
    this.browser = new FakeBrowser(this.callback, outcome);
  }

  deps(): LoginDeps {
    return {
      fetch: this.upstream.fetch,
      clock: this.clock.now,
      store: this.store,
      listenForCallback: this.callback.listen,
      openBrowser: this.browser.open,
      timeout: this.deadline.sleep,
      writeError: (line) => void this.stderr.push(line),
    };
  }
}

describe("runLogin", () => {
  it("abre o authorize com PKCE e grava o par trocado pelo code", async () => {
    const rig = new LoginRig({ kind: "consent", code: "c1" });
    rig.upstream.respondWith(200, {
      access_token: "jwt",
      expires_in: 300,
      refresh_token: "r1",
    });
    await runLogin(TEST_ISSUER, rig.deps());
    const authorize = rig.browser.opened[0];
    const params = Object.fromEntries(authorize?.searchParams ?? []);
    expect(authorize?.origin + (authorize?.pathname ?? "")).toBe(
      `${TEST_ISSUER}/oauth/authorize`,
    );
    expect(params).toMatchObject({
      response_type: "code",
      client_id: "markupp-mcp",
      redirect_uri: "http://127.0.0.1:43123/callback",
      code_challenge_method: "S256",
      state: rig.callback.expectedState,
      scope: "notas:leitura notas:escrita administracao",
      resource: TEST_ISSUER,
    });
    const form = Object.fromEntries(
      new URLSearchParams(await rig.upstream.requests[1]?.text()),
    );
    expect(form).toMatchObject({
      grant_type: "authorization_code",
      code: "c1",
      redirect_uri: "http://127.0.0.1:43123/callback",
      resource: TEST_ISSUER,
    });
    expect(s256Challenge(form.code_verifier ?? "")).toBe(params.code_challenge);
    expect(await rig.store.load()).toEqual({
      issuer: TEST_ISSUER,
      refreshToken: "r1",
      accessToken: "jwt",
      expiresAtMs: rig.clock.now() + 300_000,
    });
    expect(rig.callback.closed).toBe(true);
    expect(rig.stderr.join("\n")).toContain(authorize?.href);
  });

  it("falha com o erro OAuth do retorno, sem gravar nada", async () => {
    const rig = new LoginRig({ kind: "deny", error: "access_denied" });
    await expect(runLogin(TEST_ISSUER, rig.deps())).rejects.toThrow(
      "login recusado pelo markupp: error access_denied",
    );
    expect(await rig.store.load()).toBeUndefined();
    expect(rig.upstream.requests).toHaveLength(1);
  });

  it("vence o prazo quando o navegador não volta, e fecha o callback", async () => {
    const rig = new LoginRig({ kind: "abandon" });
    await expect(runLogin(TEST_ISSUER, rig.deps())).rejects.toThrow(
      "o navegador nao voltou a http://127.0.0.1:43123/callback em 5 minutos",
    );
    expect(rig.callback.closed).toBe(true);
  });
});
