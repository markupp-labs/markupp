import { describe, expect, it } from "vitest";
import { EXPIRY_MARGIN_MS } from "../auth/api-token-cache.js";
import {
  authServerMetadata,
  TEST_ISSUER,
} from "../testing/auth-server-metadata.js";
import { FakeClock } from "../testing/fake-clock.js";
import { FakeFetch } from "../testing/fake-fetch.js";
import { FakeLocalFiles } from "../testing/fake-local-files.js";
import { CredentialStore, type StoredCredential } from "./credential-store.js";
import { STALE_LOCK_MS } from "./file-lock.js";
import { LoginRequiredError, storedApiToken } from "./stored-api-token.js";

const PATH = "/home/ana/.config/markupp/credentials.json";

// StdioTokenRig liga a renovação a um disco, um relógio e um Go falsos, com uma
// credencial cujo JWT vence em 5 minutos.
class StdioTokenRig {
  readonly clock = new FakeClock();
  readonly files = new FakeLocalFiles(this.clock.now);
  readonly upstream = new FakeFetch();
  readonly store = new CredentialStore(PATH, this.files);
  sleep: (ms: number) => Promise<void> = this.clock.sleep;

  async seed(overrides: Partial<StoredCredential> = {}): Promise<this> {
    await this.store.save(this.credential(overrides));
    return this;
  }

  credential(overrides: Partial<StoredCredential> = {}): StoredCredential {
    return {
      issuer: TEST_ISSUER,
      refreshToken: "r1",
      accessToken: "jwt-1",
      expiresAtMs: this.clock.now() + 300_000,
      ...overrides,
    };
  }

  apiToken(): Promise<string> {
    return storedApiToken({
      store: this.store,
      files: this.files,
      fetch: this.upstream.fetch,
      clock: this.clock.now,
      sleep: (ms) => this.sleep(ms),
    })();
  }

  respondWithRotation(): this {
    this.upstream.respondWith(200, authServerMetadata()).respondWith(200, {
      access_token: "jwt-2",
      expires_in: 300,
      refresh_token: "r2",
    });
    return this;
  }
}

// OtherProcessRenewal faz o papel de outro processo stdio: segura o lock e, na primeira
// espera deste processo, grava o par que renovou e solta o lock.
class OtherProcessRenewal {
  constructor(private readonly rig: StdioTokenRig) {
    rig.files.put(rig.store.lockPath, "");
  }

  readonly sleep = async (ms: number): Promise<void> => {
    this.rig.clock.advance(ms);
    await this.rig.store.save(
      this.rig.credential({
        accessToken: "jwt-outro",
        refreshToken: "r-outro",
      }),
    );
    await this.rig.files.remove(this.rig.store.lockPath);
  };
}

async function formOf(
  request: Request | undefined,
): Promise<Record<string, string>> {
  return Object.fromEntries(new URLSearchParams(await request?.text()));
}

describe("storedApiToken", () => {
  it("reaproveita o JWT do disco enquanto vale, sem falar com o Go", async () => {
    const rig = await new StdioTokenRig().seed();
    rig.clock.advance(300_000 - EXPIRY_MARGIN_MS - 1);
    expect(await rig.apiToken()).toBe("jwt-1");
    expect(rig.upstream.requests).toHaveLength(0);
  });

  it("renova na margem de expiração e grava o refresh girado", async () => {
    const rig = (await new StdioTokenRig().seed()).respondWithRotation();
    rig.clock.advance(300_000 - EXPIRY_MARGIN_MS);
    expect(await rig.apiToken()).toBe("jwt-2");
    expect(await formOf(rig.upstream.requests[1])).toEqual({
      grant_type: "refresh_token",
      refresh_token: "r1",
      client_id: "markupp-mcp",
      resource: TEST_ISSUER,
    });
    const stored = await rig.store.load();
    expect(stored?.refreshToken).toBe("r2");
    expect(stored?.expiresAtMs).toBe(rig.clock.now() + 300_000);
    expect(rig.files.has(rig.store.lockPath)).toBe(false);
  });

  it("usa o JWT que outro processo renovou enquanto esperava o lock", async () => {
    const rig = await new StdioTokenRig().seed();
    rig.clock.advance(300_000);
    rig.sleep = new OtherProcessRenewal(rig).sleep;
    expect(await rig.apiToken()).toBe("jwt-outro");
    expect(rig.upstream.requests).toHaveLength(0);
    expect((await rig.store.load())?.refreshToken).toBe("r-outro");
  });

  it("toma o lock abandonado e renova", async () => {
    const rig = (await new StdioTokenRig().seed()).respondWithRotation();
    rig.clock.advance(300_000);
    rig.files.put(rig.store.lockPath, "");
    const lockedAt = rig.clock.now();
    expect(await rig.apiToken()).toBe("jwt-2");
    expect(rig.clock.now() - lockedAt).toBeGreaterThan(STALE_LOCK_MS);
  });

  it("apaga a credencial e pede login quando o Go recusa o refresh", async () => {
    const rig = await new StdioTokenRig().seed();
    rig.upstream
      .respondWith(200, authServerMetadata())
      .respondWith(400, { error: "invalid_grant" });
    rig.clock.advance(300_000);
    const failure = await rig.apiToken().catch((error: unknown) => error);
    expect(failure).toBeInstanceOf(LoginRequiredError);
    expect(String(failure)).toContain(`markupp-mcp login ${TEST_ISSUER}`);
    expect(String(failure)).not.toContain("r1");
    expect(await rig.store.load()).toBeUndefined();
  });

  it("mantém a credencial quando a renovação falha por outro motivo", async () => {
    const rig = await new StdioTokenRig().seed();
    rig.upstream.respondWith(200, authServerMetadata()).respondWith(500, {});
    rig.clock.advance(300_000);
    await expect(rig.apiToken()).rejects.toThrow("status 500");
    expect((await rig.store.load())?.refreshToken).toBe("r1");
  });

  it("pede login sem credencial no disco", async () => {
    const rig = new StdioTokenRig();
    await expect(rig.apiToken()).rejects.toThrow(
      `no markupp login stored in ${PATH}`,
    );
  });
});
