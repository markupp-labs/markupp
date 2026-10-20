import type { TokenExchangeConfig } from "../auth/token-exchange.js";
import type { FakeClock } from "./fake-clock.js";
import type { FakeFetch } from "./fake-fetch.js";

// exchangeConfig monta nos testes o servidor MCP como resource do Go, com fetch e
// relógio falsos; overrides troca só os campos que o teste exercita.
export function exchangeConfig(
  fetcher: FakeFetch,
  clock: FakeClock,
  overrides: Partial<TokenExchangeConfig> = {},
): TokenExchangeConfig {
  return {
    apiUrl: "https://markupp.exemplo",
    resourceUrl: "https://mcp.exemplo",
    resourceSecret: "segredo",
    fetch: fetcher.fetch,
    clock: clock.now,
    ...overrides,
  };
}
