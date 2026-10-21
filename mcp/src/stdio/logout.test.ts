import { describe, expect, it } from "vitest";
import {
  authServerMetadata,
  TEST_ISSUER,
} from "../testing/auth-server-metadata.js";
import { FakeClock } from "../testing/fake-clock.js";
import { FakeFetch } from "../testing/fake-fetch.js";
import { FakeLocalFiles } from "../testing/fake-local-files.js";
import { CredentialStore } from "./credential-store.js";
import { runLogout, type LogoutDeps } from "./logout.js";

const PATH = "/home/ana/.config/markupp/credentials.json";

// LogoutRig liga o logout a um disco com credencial e a um Go falsos.
class LogoutRig {
  readonly clock = new FakeClock();
  readonly files = new FakeLocalFiles(this.clock.now);
  readonly store = new CredentialStore(PATH, this.files);
  readonly upstream = new FakeFetch();

  async seed(): Promise<this> {
    await this.store.save({
      issuer: TEST_ISSUER,
      refreshToken: "r1",
      accessToken: "jwt",
      expiresAtMs: this.clock.now() + 300_000,
    });
    return this;
  }

  deps(): LogoutDeps {
    return {
      store: this.store,
      files: this.files,
      fetch: this.upstream.fetch,
      clock: this.clock.now,
      sleep: this.clock.sleep,
    };
  }
}

describe("runLogout", () => {
  it("revoga o refresh e apaga a credencial", async () => {
    const rig = await new LogoutRig().seed();
    rig.upstream.respondWith(200, authServerMetadata()).respondWith(200);
    expect(await runLogout(rig.deps())).toBe("revoked");
    const revoke = rig.upstream.requests[1];
    expect(revoke?.url).toBe(`${TEST_ISSUER}/oauth/revoke`);
    expect(new URLSearchParams(await revoke?.text()).get("token")).toBe("r1");
    expect(await rig.store.load()).toBeUndefined();
    expect(rig.files.has(rig.store.lockPath)).toBe(false);
  });

  it("não faz nada sem credencial", async () => {
    const rig = new LogoutRig();
    expect(await runLogout(rig.deps())).toBe("absent");
    expect(rig.upstream.requests).toHaveLength(0);
  });

  it("apaga a credencial e manda revogar pelo painel quando o Go falha", async () => {
    const rig = await new LogoutRig().seed();
    rig.upstream.respondWith(503);
    await expect(runLogout(rig.deps())).rejects.toThrow(
      "revogue a Autorizacao pelo painel",
    );
    expect(await rig.store.load()).toBeUndefined();
  });
});
