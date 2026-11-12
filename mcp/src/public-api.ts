import {
  toApiError,
  type FetchFunction,
  type SearchPage,
} from "./markupp-api.js";

// PublicationKind é o alvo da publicação que cobre a nota (ADR-0048).
export type PublicationKind = "note" | "folder" | "vault";

// PublicNoteLink é uma ligação da nota pública; public_id e title_slug vêm null quando
// o alvo não é público.
export type PublicNoteLink = {
  cited_title: string;
  kind: string;
  public_id: string | null;
  title_slug: string | null;
};

// PublicNote é a nota pública inteira, como /v1/public/notes/{public_id} devolve.
export type PublicNote = {
  public_id: string;
  title: string;
  title_slug: string;
  content: string;
  updated_at: string;
  indexable: boolean;
  publication: { kind: PublicationKind; title: string; root_public_id: string };
  links: PublicNoteLink[];
};

// PublicTreeItem é uma nota da mesma publicação, com path relativo à pasta publicada.
export type PublicTreeItem = {
  public_id: string;
  title: string;
  title_slug: string;
  path: string;
};

// PublicSearchHit é um resultado da busca na publicação, com o trecho do ts_headline.
export type PublicSearchHit = {
  public_id: string;
  title: string;
  title_slug: string;
  snippet: string;
  rank: number;
};

// PublicPage é uma página da árvore ou da busca e o total que casou.
export type PublicPage<T> = { items: T[]; total: number };

// PublicMarkuppApi é a leitura sem conta das notas públicas (ADR-0048), pelo id
// público; árvore e busca ficam na publicação da nota lida.
export interface PublicMarkuppApi {
  getNote(publicId: string): Promise<PublicNote>;
  listTree(
    publicId: string,
    page?: SearchPage,
  ): Promise<PublicPage<PublicTreeItem>>;
  searchNotes(
    publicId: string,
    query: string,
    page?: SearchPage,
  ): Promise<PublicPage<PublicSearchHit>>;
}

export type HttpPublicMarkuppApiOptions = {
  baseUrl: string;
  fetch: FetchFunction;
  // forwardedFor é o X-Forwarded-For que chegou em /mcp/public, com o salto que o Go
  // escreveu ao encaminhar, repassado para o Go contar o limite por IP do agente.
  forwardedFor?: string;
  // relayAuthorization é o HTTP Basic do MCP como resource: só com ele o Go conta o
  // limite pelo X-Forwarded-For repassado (ADR-0050).
  relayAuthorization?: string;
};

// HttpPublicMarkuppApi fala só com as rotas /v1/public/notes/{public_id}, sem o token
// do agente.
export class HttpPublicMarkuppApi implements PublicMarkuppApi {
  private readonly baseUrl: string;

  constructor(private readonly options: HttpPublicMarkuppApiOptions) {
    this.baseUrl = options.baseUrl.replace(/\/+$/, "");
  }

  getNote(publicId: string): Promise<PublicNote> {
    return this.get(notePath(publicId));
  }

  listTree(
    publicId: string,
    page: SearchPage = {},
  ): Promise<PublicPage<PublicTreeItem>> {
    const params = new URLSearchParams();
    appendPage(params, page);
    return this.get(withQuery(`${notePath(publicId)}/tree`, params));
  }

  searchNotes(
    publicId: string,
    query: string,
    page: SearchPage = {},
  ): Promise<PublicPage<PublicSearchHit>> {
    const params = new URLSearchParams({ q: query });
    appendPage(params, page);
    return this.get(withQuery(`${notePath(publicId)}/search`, params));
  }

  private async get<T>(path: string): Promise<T> {
    const headers: Record<string, string> = { accept: "application/json" };
    if (this.options.forwardedFor)
      headers["x-forwarded-for"] = this.options.forwardedFor;
    if (this.options.relayAuthorization)
      headers.authorization = this.options.relayAuthorization;
    const response = await this.options.fetch(this.baseUrl + path, { headers });
    if (!response.ok) throw await toApiError({ method: "GET", path }, response);
    return (await response.json()) as T;
  }
}

function notePath(publicId: string): string {
  return `/v1/public/notes/${encodeURIComponent(publicId)}`;
}

// appendPage acrescenta limit e offset, quando vêm, depois do que params já tem.
function appendPage(params: URLSearchParams, page: SearchPage): void {
  if (page.limit !== undefined) params.set("limit", String(page.limit));
  if (page.offset !== undefined) params.set("offset", String(page.offset));
}

function withQuery(path: string, params: URLSearchParams): string {
  const query = params.toString();
  return query === "" ? path : `${path}?${query}`;
}
