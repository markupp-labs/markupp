import {
  toApiError,
  type FetchFunction,
  type SearchPage,
} from "./markupp-api.js";

// PublicNoteItem é o que a lista pública devolve de cada nota, sem o conteúdo.
export type PublicNoteItem = {
  id: string;
  title: string;
  path: string;
  updated_at: string;
};

// PublicNote é a nota publicada inteira, como /v1/public/vaults/{slug}/notes/{id}
// devolve.
export type PublicNote = PublicNoteItem & {
  content: string;
  created_at: string;
};

// PublicSearchHit é um resultado da busca pública: a nota resumida e o trecho achado.
export type PublicSearchHit = {
  note: { id: string; title: string; path: string };
  rank: number;
  snippet: string;
};

// PublicNoteOrder é a ordem da lista: por caminho, crescente, ou pela centralidade no
// grafo e pela edição mais recente, decrescentes.
export type PublicNoteOrder = "path" | "pagerank" | "updated";

// PublicListPage é a ordem e a página da lista; a API limita limit a 1..100 e usa 10.
export type PublicListPage = SearchPage & { sort?: PublicNoteOrder };

// PublicMarkuppApi é a leitura sem conta dos cofres publicados (ADR-0048), pelo slug.
export interface PublicMarkuppApi {
  listNotes(slug: string, page?: PublicListPage): Promise<PublicNoteItem[]>;
  getNote(slug: string, id: string): Promise<PublicNote>;
  searchNotes(
    slug: string,
    query: string,
    page?: SearchPage,
  ): Promise<PublicSearchHit[]>;
}

export type HttpPublicMarkuppApiOptions = {
  baseUrl: string;
  fetch: FetchFunction;
  // forwardedFor é o X-Forwarded-For que chegou em /mcp/public, com o salto que o Go
  // escreveu ao encaminhar, repassado para o Go contar o limite por IP do agente.
  forwardedFor?: string;
  // relayAuthorization é o HTTP Basic do MCP como resource: só com ele o Go conta o
  // limite pelo X-Forwarded-For repassado (ADR-0051).
  relayAuthorization?: string;
};

// HttpPublicMarkuppApi fala só com as rotas /v1/public/vaults/{slug}, sem o token do
// agente.
export class HttpPublicMarkuppApi implements PublicMarkuppApi {
  private readonly baseUrl: string;

  constructor(private readonly options: HttpPublicMarkuppApiOptions) {
    this.baseUrl = options.baseUrl.replace(/\/+$/, "");
  }

  // listNotes recebe da API também o hash e o rank e devolve só o resumo.
  async listNotes(
    slug: string,
    page: PublicListPage = {},
  ): Promise<PublicNoteItem[]> {
    const params = new URLSearchParams();
    if (page.sort && page.sort !== "path") params.set("sort", page.sort);
    appendPage(params, page);
    const items = await this.get<PublicNoteItem[]>(
      withQuery(`${vaultPath(slug)}/notes`, params),
    );
    return items.map(({ id, title, path, updated_at }) => ({
      id,
      title,
      path,
      updated_at,
    }));
  }

  getNote(slug: string, id: string): Promise<PublicNote> {
    return this.get(`${vaultPath(slug)}/notes/${encodeURIComponent(id)}`);
  }

  searchNotes(
    slug: string,
    query: string,
    page: SearchPage = {},
  ): Promise<PublicSearchHit[]> {
    const params = new URLSearchParams({ q: query });
    appendPage(params, page);
    return this.get(withQuery(`${vaultPath(slug)}/search`, params));
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

function vaultPath(slug: string): string {
  return `/v1/public/vaults/${encodeURIComponent(slug)}`;
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
