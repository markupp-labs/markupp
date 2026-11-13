import { describe, expect, it } from "vitest";
import { FakeMyInvites } from "./fake-my-invites.js";

describe("FakeMyInvites", () => {
  it("tira o convite respondido e recusa o que não está na lista", () => {
    const invites = new FakeMyInvites();
    const invite = invites.add({
      id: "i1",
      tenant_id: "t1",
      tenant_name: "T",
      role: "member",
      expires_at: "2026-01-01T00:00:00Z",
    });
    expect(invites.take("i1")).toEqual(invite);
    expect(invites.list()).toEqual([]);
    expect(() => invites.take("i1")).toThrow(/invite not found/);
  });
});
