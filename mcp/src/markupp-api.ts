import { toApiError } from "./api-error.js";
import type {
  Publication,
  PublicationList,
  PublishRequest,
} from "./publication.js";
import type { ApiTokenSource } from "./session.js";
import type {
  Account,
  FoundNote,
  IssuedInvite,
  Member,
  Note,
  NoteRevision,
  NoteSummary,
  PendingInvite,
  Role,
  SearchPage,
  TenantInvite,
  TenantMembership,
  Vault,
} from "./markupp-api-types.js";

export { MarkuppApiError, toApiError } from "./api-error.js";
export type * from "./markupp-api-types.js";

// MarkuppApi é a fatia da API REST do markupp que as ferramentas MCP usam. tenant é o
// id do tenant em que a chamada age; vault, quando vem, restringe a nota a esse cofre.
export interface MarkuppApi {
  getAccount(): Promise<Account>;
  listTenants(): Promise<TenantMembership[]>;
  listNotes(tenant: string, vault?: string): Promise<NoteSummary[]>;
  searchNotes(
    tenant: string,
    query: string,
    page?: SearchPage,
    vault?: string,
  ): Promise<FoundNote[]>;
  getNote(tenant: string, id: string): Promise<Note>;
  createNote(
    tenant: string,
    path: string,
    content: string,
    vault?: string,
  ): Promise<Note>;
  updateNote(tenant: string, id: string, revision: NoteRevision): Promise<Note>;
  deleteNote(tenant: string, id: string): Promise<void>;
  listMembers(tenant: string): Promise<Member[]>;
  changeMemberRole(tenant: string, userId: string, role: Role): Promise<void>;
  removeMember(tenant: string, userId: string): Promise<void>;
  listInvites(tenant: string): Promise<TenantInvite[]>;
  inviteMember(
    tenant: string,
    email: string,
    role: Role,
  ): Promise<IssuedInvite>;
  revokeInvite(tenant: string, inviteId: string): Promise<void>;
  createTenant(name: string): Promise<TenantMembership>;
  listMyInvites(): Promise<PendingInvite[]>;
  acceptInvite(inviteId: string): Promise<TenantMembership>;
  declineInvite(inviteId: string): Promise<void>;
  listVaults(tenant: string): Promise<Vault[]>;
  createVault(tenant: string, name: string): Promise<Vault>;
  renameVault(tenant: string, vault: string, name: string): Promise<Vault>;
  deleteVault(tenant: string, vault: string): Promise<void>;
  listVaultMembers(tenant: string, vault: string): Promise<Member[]>;
  addVaultMember(tenant: string, vault: string, userId: string): Promise<void>;
  removeVaultMember(
    tenant: string,
    vault: string,
    userId: string,
  ): Promise<void>;
  publish(
    tenant: string,
    vault: string,
    request: PublishRequest,
  ): Promise<Publication>;
  unpublish(tenant: string, publication: string): Promise<void>;
  listPublications(tenant: string, vault?: string): Promise<PublicationList>;
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
  method: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  path: string;
  tenant?: string;
  body?: unknown;
};

// SearchHit é o resultado de /v1/search/notes, com a nota resumida e o cofre dela.
type SearchHit = { note: { id: string; path: string }; vault_id: string };

// TenantHeader é onde a API lê em qual tenant da conta a requisição age.
const TenantHeader = "X-Markupp-Tenant";

// MaxLimit e MaxOffset são o maior limit e o maior offset que a API aceita numa página.
const MaxLimit = 100;
const MaxOffset = 10_000;

// HttpMarkuppApi fala com a API do markupp só pelas rotas /v1.
export class HttpMarkuppApi implements MarkuppApi {
  private readonly baseUrl: string;

  constructor(private readonly options: HttpMarkuppApiOptions) {
    this.baseUrl = options.baseUrl.replace(/\/+$/, "");
  }

  // getAccount lê a conta em /v1/auth/me, que aceita token OAuth de qualquer escopo.
  async getAccount(): Promise<Account> {
    const { id, email } = await this.json<Account>({
      method: "GET",
      path: "/v1/auth/me",
    });
    return { id, email };
  }

  listTenants(): Promise<TenantMembership[]> {
    return this.json({ method: "GET", path: "/v1/me/tenants" });
  }

  // listNotes recebe as notas inteiras da API e devolve só o resumo de cada uma.
  async listNotes(tenant: string, vault?: string): Promise<NoteSummary[]> {
    if (vault !== undefined) return this.listVaultNotes(tenant, vault);
    const notes = await this.json<Note[]>({
      method: "GET",
      path: "/v1/notes",
      tenant,
    });
    return notes.map(noteSummary);
  }

  // listVaultNotes pede páginas do maior tamanho que a rota do cofre aceita até vir uma
  // incompleta.
  // ponytail: para no offset máximo da rota, cerca de 10 mil notas por cofre; passar
  // disso pede paginar a ferramenta list_notes.
  private async listVaultNotes(
    tenant: string,
    vault: string,
  ): Promise<NoteSummary[]> {
    const notes: NoteSummary[] = [];
    for (let offset = 0; offset <= MaxOffset; offset += MaxLimit) {
      const params = `limit=${MaxLimit}&offset=${offset}`;
      const path = `${vaultPath(vault)}/notes?${params}`;
      const page = await this.json<NoteSummary[]>({
        method: "GET",
        path,
        tenant,
      });
      notes.push(
        ...page.map((note) => noteSummary({ ...note, vault_id: vault })),
      );
      if (page.length < MaxLimit) break;
    }
    return notes;
  }

  // searchNotes busca pela rota antiga, que traz updated_at; filtrar por cofre só a
  // busca de /v1/search/notes faz.
  async searchNotes(
    tenant: string,
    query: string,
    page: SearchPage = {},
    vault?: string,
  ): Promise<FoundNote[]> {
    if (vault !== undefined)
      return this.searchVault(tenant, query, page, vault);
    const params = withPage(new URLSearchParams({ query }), page);
    const path = `/v1/notes/search?${params}`;
    return this.json({ method: "GET", path, tenant });
  }

  private async searchVault(
    tenant: string,
    query: string,
    page: SearchPage,
    vault: string,
  ): Promise<FoundNote[]> {
    const params = withPage(new URLSearchParams({ q: query, vault }), page);
    const path = `/v1/search/notes?${params}`;
    const hits = await this.json<SearchHit[]>({ method: "GET", path, tenant });
    return hits.map(({ note: { id, path }, vault_id }) => ({
      id,
      vault_id,
      path,
    }));
  }

  getNote(tenant: string, id: string): Promise<Note> {
    return this.json({ method: "GET", path: notePath(id), tenant });
  }

  // createNote sem cofre manda vault_id vazio: o servidor grava no cofre padrão do
  // tenant; com cofre usa a rota do cofre.
  createNote(
    tenant: string,
    path: string,
    content: string,
    vault?: string,
  ): Promise<Note> {
    if (vault !== undefined) {
      const body = { path, content };
      const route = `${vaultPath(vault)}/notes`;
      return this.json({ method: "POST", path: route, tenant, body });
    }
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

  listMembers(tenant: string): Promise<Member[]> {
    return this.json({ method: "GET", path: "/v1/users", tenant });
  }

  async changeMemberRole(
    tenant: string,
    userId: string,
    role: Role,
  ): Promise<void> {
    const path = memberPath(userId);
    await this.send({ method: "PATCH", path, tenant, body: { role } });
  }

  async removeMember(tenant: string, userId: string): Promise<void> {
    await this.send({ method: "DELETE", path: memberPath(userId), tenant });
  }

  listInvites(tenant: string): Promise<TenantInvite[]> {
    return this.json({ method: "GET", path: "/v1/users/invites", tenant });
  }

  inviteMember(
    tenant: string,
    email: string,
    role: Role,
  ): Promise<IssuedInvite> {
    const body = { email, role };
    return this.json({
      method: "POST",
      path: "/v1/users/invites",
      tenant,
      body,
    });
  }

  async revokeInvite(tenant: string, inviteId: string): Promise<void> {
    const path = `/v1/users/invites/${encodeURIComponent(inviteId)}`;
    await this.send({ method: "DELETE", path, tenant });
  }

  createTenant(name: string): Promise<TenantMembership> {
    return this.json({ method: "POST", path: "/v1/tenants", body: { name } });
  }

  listMyInvites(): Promise<PendingInvite[]> {
    return this.json({ method: "GET", path: "/v1/me/invites" });
  }

  acceptInvite(inviteId: string): Promise<TenantMembership> {
    const path = `${myInvitePath(inviteId)}/accept`;
    return this.json({ method: "POST", path });
  }

  async declineInvite(inviteId: string): Promise<void> {
    await this.send({
      method: "POST",
      path: `${myInvitePath(inviteId)}/decline`,
    });
  }

  listVaults(tenant: string): Promise<Vault[]> {
    return this.json({ method: "GET", path: "/v1/vaults", tenant });
  }

  createVault(tenant: string, name: string): Promise<Vault> {
    const body = { name };
    return this.json({ method: "POST", path: "/v1/vaults", tenant, body });
  }

  renameVault(tenant: string, vault: string, name: string): Promise<Vault> {
    const path = vaultPath(vault);
    return this.json({ method: "PATCH", path, tenant, body: { name } });
  }

  async deleteVault(tenant: string, vault: string): Promise<void> {
    await this.send({ method: "DELETE", path: vaultPath(vault), tenant });
  }

  listVaultMembers(tenant: string, vault: string): Promise<Member[]> {
    const path = `${vaultPath(vault)}/members`;
    return this.json({ method: "GET", path, tenant });
  }

  async addVaultMember(
    tenant: string,
    vault: string,
    userId: string,
  ): Promise<void> {
    const path = `${vaultPath(vault)}/members`;
    await this.send({
      method: "POST",
      path,
      tenant,
      body: { user_id: userId },
    });
  }

  async removeVaultMember(
    tenant: string,
    vault: string,
    userId: string,
  ): Promise<void> {
    const path = `${vaultPath(vault)}/members/${encodeURIComponent(userId)}`;
    await this.send({ method: "DELETE", path, tenant });
  }

  publish(
    tenant: string,
    vault: string,
    request: PublishRequest,
  ): Promise<Publication> {
    const path = `${vaultPath(vault)}/publications`;
    return this.json({ method: "POST", path, tenant, body: request });
  }

  async unpublish(tenant: string, publication: string): Promise<void> {
    const path = `/v1/publications/${encodeURIComponent(publication)}`;
    await this.send({ method: "DELETE", path, tenant });
  }

  // listPublications sem cofre lista as publicações visíveis em todo o tenant.
  listPublications(tenant: string, vault?: string): Promise<PublicationList> {
    const query =
      vault === undefined ? "" : `?${new URLSearchParams({ vault_id: vault })}`;
    return this.json({
      method: "GET",
      path: `/v1/publications${query}`,
      tenant,
    });
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

function memberPath(userId: string): string {
  return `/v1/users/${encodeURIComponent(userId)}`;
}

function vaultPath(vault: string): string {
  return `/v1/vaults/${encodeURIComponent(vault)}`;
}

// withPage põe na query string o limit e o offset que vieram, para o servidor usar o
// padrão dele nos que faltam.
function withPage(params: URLSearchParams, page: SearchPage): URLSearchParams {
  if (page.limit !== undefined) params.set("limit", String(page.limit));
  if (page.offset !== undefined) params.set("offset", String(page.offset));
  return params;
}

function myInvitePath(inviteId: string): string {
  return `/v1/me/invites/${encodeURIComponent(inviteId)}`;
}

export function noteSummary(note: NoteSummary): NoteSummary {
  const { id, vault_id, path, updated_at } = note;
  return { id, vault_id, path, updated_at };
}
