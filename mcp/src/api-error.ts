// MarkuppApiError é a resposta de erro {error, message} da API, com o status HTTP.
export class MarkuppApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
  ) {
    super(message);
    this.name = "MarkuppApiError";
  }
}

// toApiError lê o corpo {error, message}; um corpo fora desse formato (um proxy no
// meio, por exemplo) vira o código unexpected_response com o status e o texto recebido.
export async function toApiError(
  call: { method: string; path: string },
  response: Response,
): Promise<MarkuppApiError> {
  const text = await response.text();
  const parsed = parseErrorBody(text);
  if (parsed)
    return new MarkuppApiError(response.status, parsed.error, parsed.message);
  const message = `${call.method} ${call.path} answered ${response.status} with a body that does not match {error, message}: ${text.slice(0, 200)}`;
  return new MarkuppApiError(response.status, "unexpected_response", message);
}

function parseErrorBody(
  text: string,
): { error: string; message: string } | null {
  try {
    const body: unknown = JSON.parse(text);
    if (!isErrorBody(body)) return null;
    return body;
  } catch {
    return null;
  }
}

function isErrorBody(
  body: unknown,
): body is { error: string; message: string } {
  if (typeof body !== "object" || body === null) return false;
  const fields = body as Record<string, unknown>;
  return typeof fields.error === "string" && typeof fields.message === "string";
}
