import { describe, expect, it } from "vitest";
import { FakeClock } from "./fake-clock.js";

describe("FakeClock", () => {
  it("só anda quando o teste manda", () => {
    const clock = new FakeClock(1_000);
    expect(clock.now()).toBe(1_000);
    clock.advance(250);
    expect(clock.now()).toBe(1_250);
  });
});
