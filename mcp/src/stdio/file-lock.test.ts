import { describe, expect, it } from "vitest";
import { FakeClock } from "../testing/fake-clock.js";
import { FakeLocalFiles } from "../testing/fake-local-files.js";
import { STALE_LOCK_MS, withFileLock } from "./file-lock.js";

const LOCK = "/c.lock";

function lockDeps(): { files: FakeLocalFiles; clock: FakeClock } {
  const clock = new FakeClock();
  return { files: new FakeLocalFiles(clock.now), clock };
}

describe("withFileLock", () => {
  it("segura o lock durante a seção crítica e solta no fim", async () => {
    const { files, clock } = lockDeps();
    const deps = { files, clock: clock.now, sleep: clock.sleep };
    const held = await withFileLock(LOCK, deps, async () => files.has(LOCK));
    expect(held).toBe(true);
    expect(files.has(LOCK)).toBe(false);
  });

  it("solta o lock quando a seção crítica falha", async () => {
    const { files, clock } = lockDeps();
    const deps = { files, clock: clock.now, sleep: clock.sleep };
    const failing = withFileLock(LOCK, deps, async () => {
      throw new Error("falhou");
    });
    await expect(failing).rejects.toThrow("falhou");
    expect(files.has(LOCK)).toBe(false);
  });

  it("espera o lock vivo e toma o abandonado depois do prazo", async () => {
    const { files, clock } = lockDeps();
    files.put(LOCK, "");
    const start = clock.now();
    const deps = { files, clock: clock.now, sleep: clock.sleep };
    await withFileLock(LOCK, deps, async () => undefined);
    expect(clock.now() - start).toBeGreaterThan(STALE_LOCK_MS);
    expect(clock.now() - start).toBeLessThan(STALE_LOCK_MS + 1_000);
  });
});
