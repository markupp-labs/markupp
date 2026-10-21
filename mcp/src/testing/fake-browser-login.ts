import type { LoopbackCallback } from "../stdio/loopback-callback.js";

const FAKE_REDIRECT_URI = "http://127.0.0.1:43123/callback";

// FakeLoopbackCallback troca o servidor do callback: guarda o state esperado e entrega
// a query quando o FakeBrowser volta.
export class FakeLoopbackCallback {
  expectedState = "";
  closed = false;
  private deliver: (query: URLSearchParams) => void = () => undefined;
  private readonly received = new Promise<URLSearchParams>((resolve) => {
    this.deliver = resolve;
  });

  readonly listen = async (
    expectedState: string,
  ): Promise<LoopbackCallback> => {
    this.expectedState = expectedState;
    return {
      redirectUri: FAKE_REDIRECT_URI,
      received: this.received,
      close: async () => {
        this.closed = true;
      },
    };
  };

  back(query: Record<string, string>): void {
    this.deliver(new URLSearchParams(query));
  }
}

// BrowserOutcome é o que a pessoa faz no painel: consentir, recusar ou abandonar a aba.
export type BrowserOutcome =
  | { kind: "consent"; code: string }
  | { kind: "deny"; error: string }
  | { kind: "abandon" };

// FakeBrowser faz o papel do navegador com o Go: abre a URL do authorize e volta ao
// callback com o state dela e o desfecho dado.
export class FakeBrowser {
  readonly opened: URL[] = [];

  constructor(
    private readonly callback: FakeLoopbackCallback,
    private readonly outcome: BrowserOutcome,
  ) {}

  readonly open = (url: string): void => {
    const authorize = new URL(url);
    this.opened.push(authorize);
    const state = authorize.searchParams.get("state") ?? "";
    if (this.outcome.kind === "consent")
      this.callback.back({ code: this.outcome.code, state });
    if (this.outcome.kind === "deny")
      this.callback.back({ error: this.outcome.error, state });
  };
}
