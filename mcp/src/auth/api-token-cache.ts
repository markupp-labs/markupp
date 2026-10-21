import type { Clock, ExchangedApiToken } from "./token-exchange.js";

// AgentTokenExchange troca o token do agente por um JWT da API, sem cache.
export type AgentTokenExchange = (
  agentToken: string,
) => Promise<ExchangedApiToken>;

// EXPIRY_MARGIN_MS é quanto antes do exp o JWT deixa de ser entregue, para não
// expirar no caminho até a API.
export const EXPIRY_MARGIN_MS = 30_000;

// ApiTokenCache guarda o JWT da API por token do agente até perto de expirar. A chave
// é o SHA-256 do token do agente, que nunca fica em claro na memória do cache.
export class ApiTokenCache {
  private readonly entries = new Map<string, ExchangedApiToken>();
  private readonly exchange: AgentTokenExchange;
  private readonly clock: Clock;
  private readonly maxEntries: number;

  constructor(exchange: AgentTokenExchange, clock: Clock, maxEntries: number) {
    if (!Number.isInteger(maxEntries) || maxEntries < 1) {
      throw new Error(
        `teto do cache de tokens invalido: ${maxEntries}; esperado inteiro maior que zero`,
      );
    }
    this.exchange = exchange;
    this.clock = clock;
    this.maxEntries = maxEntries;
  }

  // apiTokenFor devolve o JWT guardado enquanto vale e troca de novo quando expira.
  async apiTokenFor(agentToken: string): Promise<string> {
    const key = await agentTokenKey(agentToken);
    const cached = this.entries.get(key);
    if (cached && this.clock() < cached.expiresAtMs - EXPIRY_MARGIN_MS) {
      return cached.accessToken;
    }
    const fresh = await this.exchange(agentToken);
    this.store(key, fresh);
    return fresh.accessToken;
  }

  // storedKeys existe para os testes conferirem o hash e o despejo; expõe só as
  // chaves SHA-256, nunca o token do agente nem o JWT.
  storedKeys(): string[] {
    return [...this.entries.keys()];
  }

  // ponytail: despejo pela ordem de inserção do Map (FIFO), sem LRU nem varredura de
  // expirados; com mais agentes ativos que o teto, um token em uso pode ser despejado
  // e trocado de novo. Troque por LRU se as trocas repetidas aparecerem.
  private store(key: string, token: ExchangedApiToken): void {
    this.entries.delete(key);
    const oldest = this.entries.keys().next().value;
    if (this.entries.size >= this.maxEntries && oldest !== undefined) {
      this.entries.delete(oldest);
    }
    this.entries.set(key, token);
  }
}

// agentTokenKey é o SHA-256 em hex do token do agente, pela Web Crypto.
export async function agentTokenKey(agentToken: string): Promise<string> {
  const bytes = new TextEncoder().encode(agentToken);
  const digest = await crypto.subtle.digest("SHA-256", bytes);
  return Array.from(new Uint8Array(digest), (b) =>
    b.toString(16).padStart(2, "0"),
  ).join("");
}
