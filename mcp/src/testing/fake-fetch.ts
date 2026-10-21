// FakeFetch troca o fetch global nos testes: guarda cada requisição recebida e
// devolve as respostas enfileiradas na ordem em que chegam.
export class FakeFetch {
  readonly requests: Request[] = [];
  private readonly queued: Response[] = [];

  respondWith(
    status: number,
    body?: unknown,
    headers?: Record<string, string>,
  ): this {
    const text = body === undefined ? null : JSON.stringify(body);
    const contentType: Record<string, string> =
      text === null ? {} : { "content-type": "application/json" };
    this.queued.push(
      new Response(text, { status, headers: { ...contentType, ...headers } }),
    );
    return this;
  }

  readonly fetch = async (
    input: RequestInfo | URL,
    init?: RequestInit,
  ): Promise<Response> => {
    this.requests.push(new Request(input, init));
    const next = this.queued.shift();
    if (!next) {
      throw new Error(
        `FakeFetch sem resposta enfileirada para ${init?.method ?? "GET"} ${String(input)}`,
      );
    }
    return next;
  };
}
