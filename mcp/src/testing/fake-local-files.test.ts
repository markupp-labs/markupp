import { describe, expect, it } from "vitest";
import { FakeClock } from "./fake-clock.js";
import { FakeLocalFiles } from "./fake-local-files.js";

describe("FakeLocalFiles", () => {
  it("guarda o conteúdo e o instante da escrita", async () => {
    const clock = new FakeClock();
    const files = new FakeLocalFiles(clock.now);
    await files.writePrivate("/c", "x");
    expect(await files.readText("/c")).toBe("x");
    expect(await files.modifiedAtMs("/c")).toBe(clock.now());
    expect(files.privateWrites).toEqual(["/c"]);
  });

  it("cria o arquivo exclusivo uma vez só", async () => {
    const files = new FakeLocalFiles(new FakeClock().now);
    expect(await files.createExclusive("/l")).toBe(true);
    expect(await files.createExclusive("/l")).toBe(false);
    await files.remove("/l");
    expect(files.has("/l")).toBe(false);
  });
});
