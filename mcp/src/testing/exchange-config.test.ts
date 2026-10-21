import { describe, expect, it } from "vitest";
import { exchangeConfig } from "./exchange-config.js";
import { FakeClock } from "./fake-clock.js";
import { FakeFetch } from "./fake-fetch.js";

describe("exchangeConfig", () => {
  it("liga fetch e relógio falsos e aplica só os overrides pedidos", () => {
    const fetcher = new FakeFetch();
    const clock = new FakeClock(5);
    const config = exchangeConfig(fetcher, clock, { resourceSecret: "outro" });
    expect(config.fetch).toBe(fetcher.fetch);
    expect(config.clock()).toBe(5);
    expect(config.resourceSecret).toBe("outro");
    expect(config.apiUrl).toBe("https://markupp.exemplo");
  });
});
