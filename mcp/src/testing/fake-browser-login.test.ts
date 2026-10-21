import { describe, expect, it } from "vitest";
import { FakeBrowser, FakeLoopbackCallback } from "./fake-browser-login.js";

describe("FakeBrowser", () => {
  it("volta ao callback com o state da URL aberta", async () => {
    const callback = new FakeLoopbackCallback();
    const listening = await callback.listen("s");
    new FakeBrowser(callback, { kind: "consent", code: "c" }).open(
      "http://localhost:8080/oauth/authorize?state=s",
    );
    const query = await listening.received;
    expect(Object.fromEntries(query)).toEqual({ code: "c", state: "s" });
    await listening.close();
    expect(callback.closed).toBe(true);
  });
});
