import type { ApiTokenSource } from "./session.js";

// TenantMembership é um vínculo da conta como GET /v1/me/tenants devolve.
export type TenantMembership = { id: string; name: string; role: string };

// NoteSummary é o que a busca devolve de cada nota, sem o conteúdo.
export type NoteSummary = {
  id: string;
  vault_id: string;
  path: string;
  updated_at: string;
};

// Note é a nota inteira, como as rotas /v1/notes/{id} e POST /v1/notes devolvem.
export type Note = NoteSummary & { content: string; created_at: string };

// SearchPage pagina a busca; o servidor limita limit a 1..100 e usa 10 se faltar.
export type SearchPage = { limit?: number; offset?: number };

// NoteRevision é a nova versão da nota com o updated_at lido, que o servidor compara
// para recusar com 409 a escrita feita sobre uma versão velha.
export type NoteRevision = {
  path: string;
  content: string;
  lastModifiedAt: string;
};

// MarkuppApi é a fatia da API REST do markupp que as ferramentas MCP usam. tenant é o
// id do tenant em que a chamada age.
export interface MarkuppApi {
  listTenants(): Promise<TenantMembership[]>;
  listNotes(tenant: string): Promise<NoteSummary[]>;
  searchNotes(
    tenant: string,
    query: string,
    page?: SearchPage,
  ): Promise<NoteSummary[]>;
  getNote(tenant: string, id: string): Promise<Note>;
  createNote(tenant: string, path: string, content: string): Promise<Note>;
  updateNote(tenant: string, id: string, revision: NoteRevision): Promise<Note>;
  deleteNote(tenant: string, id: string): Promise<void>;
}

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

export type FetchFunction = (
  input: RequestInfo | URL,
  init?: RequestInit,
) => Promise<Response>;

export type HttpMarkuppApiOptions = {
  baseUrl: string;
  fetch: FetchFunction;
  apiToken: ApiTokenSource;
};

type ApiCall = {
  method: "GET" | "POST" | "PUT" | "DELETE";
  path: string;
  tenant?: string;
  body?: unknown;
};

// TenantHeader é onde a API lê em qual tenant da conta a requisição age.
const TenantHeader = "X-Markupp-Tenant";

// HttpMarkuppApi fala com a API do markupp só pelas rotas /v1.
export class HttpMarkuppApi implements MarkuppApi {
  private readonly baseUrl: string;

  constructor(private readonly options: HttpMarkuppApiOptions) {
    this.baseUrl = options.baseUrl.replace(/\/+$/, "");
  }

  listTenants(): Promise<TenantMembership[]> {
    return this.json({ method: "GET", path: "/v1/me/tenants" });
  }

  // listNotes recebe as notas inteiras da API e devolve só o resumo de cada uma.
  async listNotes(tenant: string): Promise<NoteSummary[]> {
    const notes = await this.json<Note[]>({
      method: "GET",
      path: "/v1/notes",
      tenant,
    });
    return notes.map(noteSummary);
  }

  searchNotes(
    tenant: string,
    query: string,
    page: SearchPage = {},
  ): Promise<NoteSummary[]> {
    const params = new URLSearchParams({ query });
    if (page.limit !== undefined) params.set("limit", String(page.limit));
    if (page.offset !== undefined) params.set("offset", String(page.offset));
    return this.json({
      method: "GET",
      path: `/v1/notes/search?${params}`,
      tenant,
    });
  }

  getNote(tenant: string, id: string): Promise<Note> {
    return this.json({ method: "GET", path: notePath(id), tenant });
  }

  // createNote manda vault_id vazio: o servidor grava no cofre padrão do tenant.
  createNote(tenant: string, path: string, content: string): Promise<Note> {
    const body = { vault_id: "", path, content };
    return this.json({ method: "POST", path: "/v1/notes", tenant, body });
  }

  // updateNote manda force false para o servidor recusar com 409 a versão velha.
  updateNote(
    tenant: string,
    id: string,
    revision: NoteRevision,
  ): Promise<Note> {
    const body = { ...revision, force: false };
    return this.json({ method: "PUT", path: notePath(id), tenant, body });
  }

  async deleteNote(tenant: string, id: string): Promise<void> {
    await this.send({ method: "DELETE", path: notePath(id), tenant });
  }

  private async json<T>(call: ApiCall): Promise<T> {
    const response = await this.send(call);
    return (await response.json()) as T;
  }

  private async send(call: ApiCall): Promise<Response> {
    const response = await this.options.fetch(this.baseUrl + call.path, {
      method: call.method,
      headers: await this.headersFor(call),
      body: call.body === undefined ? undefined : JSON.stringify(call.body),
    });
    if (!response.ok) throw await toApiError(call, response);
    return response;
  }

  private async headersFor(call: ApiCall): Promise<Record<string, string>> {
    const headers: Record<string, string> = {
      authorization: `Bearer ${await this.options.apiToken()}`,
    };
    if (call.tenant !== undefined) headers[TenantHeader] = call.tenant;
    if (call.body !== undefined) headers["content-type"] = "application/json";
    return headers;
  }
}

function notePath(id: string): string {
  return `/v1/notes/${encodeURIComponent(id)}`;
}

// toApiError lê o corpo {error, message}; um corpo fora desse formato (um proxy no
// meio, por exemplo) vira o código unexpected_response com o status e o texto recebido.
async function toApiError(
  call: ApiCall,
  response: Response,
): Promise<MarkuppApiError> {
  const text = await response.text();
  const parsed = parseErrorBody(text);
  if (parsed)
    return new MarkuppApiError(response.status, parsed.error, parsed.message);
  const message = `${call.method} ${call.path} respondeu ${response.status} com corpo fora do formato {error, message}: ${text.slice(0, 200)}`;
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

export function noteSummary(note: NoteSummary): NoteSummary {
  const { id, vault_id, path, updated_at } = note;
  return { id, vault_id, path, updated_at };
}
