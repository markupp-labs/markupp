import {
  MarkuppApiError,
  type Account,
  noteSummary,
  type FoundNote,
  type InstanceEmbedding,
  type InstanceEmbeddingUpdate,
  type IssuedInvite,
  type Link,
  type LinkQuery,
  type LinkRefusal,
  type LinkType,
  type LoginProvider,
  type LoginProviderChange,
  type LoginProviderCreation,
  type LoginProviderList,
  type LoginProviderRemoval,
  type MarkuppApi,
  type Member,
  type Note,
  type NoteRevision,
  type NoteSummary,
  type PendingInvite,
  type Role,
  type SearchPage,
  type TenantInvite,
  type TenantMembership,
  type Vault,
} from "../markupp-api.js";
import type {
  Publication,
  PublicationList,
  PublishRequest,
} from "../publication.js";
import type { InstanceAccessApi } from "../instance-access-api.js";
import { FakeInstanceAccess } from "./fake-instance-access.js";
import { FakeInstanceEmbedding } from "./fake-instance-embedding.js";
import { FakeInstanceLoginProviders } from "./fake-instance-login-providers.js";
import { FakeInvites } from "./fake-invites.js";
import { FakeLinks } from "./fake-links.js";
import { FakeMembers } from "./fake-members.js";
import { FakeNotes } from "./fake-notes.js";
import { FakeMyInvites } from "./fake-my-invites.js";
import { FakePublications } from "./fake-publications.js";
import { FakeAccountEmail, FakeAccountId, FakeVaults } from "./fake-vaults.js";

// DefaultVaultId é o cofre das notas criadas sem cofre, fora da lista de cofres.
const DefaultVaultId = "vault-default";

// FakeMarkuppApi guarda tenants, notas, membros, convites e cofres em memória e responde
// com os mesmos códigos de erro da API: tenant_forbidden, read_only, not_found,
// duplicate_path, conflict, forbidden, member_not_found, invite_not_found,
// already_member e os dos cofres, das publicações e das ligações, que FakeVaults,
// FakePublications e FakeLinks descrevem.
// As chamadas da instância vêm do FakeInstanceAccess, ligadas no construtor.
export interface FakeMarkuppApi extends InstanceAccessApi {}

export class FakeMarkuppApi implements MarkuppApi {
  readonly revisions: NoteRevision[] = [];
  private readonly tenants: TenantMembership[] = [];
  private readonly members = new FakeMembers();
  private readonly myInvites = new FakeMyInvites();
  private clockTick = 0;
  private readonly notes = new FakeNotes(() => this.tick());
  private readonly vaults = new FakeVaults(() => this.tick());
  private readonly publications = new FakePublications(() => this.tick());
  private readonly invites = new FakeInvites(() => this.tick());
  private readonly links = new FakeLinks(() => this.tick());
  private readonly instanceEmbedding = new FakeInstanceEmbedding();
  private readonly loginProviders = new FakeInstanceLoginProviders();
  private readonly instanceAccess = new FakeInstanceAccess();

  constructor() {
    Object.assign(this, this.instanceAccess.api());
  }

  // withSession grava uma sessão de login da conta; current marca a de quem pede.
  withSession(id: string, current: boolean): this {
    this.instanceAccess.seedSession(id, current);
    return this;
  }

  // withInstanceAccount grava uma conta da instância, ativa.
  withInstanceAccount(id: string, email: string): this {
    this.instanceAccess.seedAccount(id, email);
    return this;
  }

  // lastAuditQuery é o filtro da última leitura do histórico.
  lastAuditQuery(): unknown {
    return this.instanceAccess.lastAuditQuery;
  }

  withTenant(id: string, role = "member", name = id): this {
    this.tenants.push({ id, name, role });
    this.notes.open(id);
    return this;
  }

  // withMember grava o membro direto, sem conferir papel, para montar o cenário.
  withMember(tenant: string, id: string, email: string, role: string): Member {
    this.notes.in(tenant);
    return this.members.add(tenant, {
      id,
      email,
      role,
      joined_at: this.tick(),
    });
  }

  // withInstanceAdmin faz da conta administradora da instância.
  withInstanceAdmin(): this {
    this.instanceEmbedding.admin = true;
    this.loginProviders.admin = true;
    this.instanceAccess.admin = true;
    return this;
  }

  withLoginProvidersLockedByHost(): this {
    this.loginProviders.locked = true;
    return this;
  }

  // withLoginProvider grava um provedor de login externo com linkedAccounts contas
  // ligadas; com conta ligada, o issuer não muda.
  withLoginProvider(id: string, linkedAccounts = 0): this {
    this.loginProviders.seed(id, linkedAccounts);
    return this;
  }

  withEmbeddingLockedByHost(): this {
    this.instanceEmbedding.locked = true;
    return this;
  }

  withInvite(tenant: string, email: string, role: string): TenantInvite {
    this.notes.in(tenant);
    return this.invites.issue(tenant, email, role);
  }

  // withMyInvite grava um convite endereçado à conta, para o tenant tenantId ainda fora dela.
  withMyInvite(
    tenantId: string,
    tenantName: string,
    role: string,
  ): PendingInvite {
    const at = this.tick();
    const invite = {
      id: `my-invite-${this.clockTick}`,
      tenant_id: tenantId,
      tenant_name: tenantName,
      role,
      expires_at: at,
    };
    return this.myInvites.add(invite);
  }

  membersIn(tenant: string): Member[] {
    this.notes.in(tenant);
    return this.members.listed(tenant);
  }

  invitesIn(tenant: string): TenantInvite[] {
    this.notes.in(tenant);
    return this.invites.listed(tenant);
  }

  // withVault grava o cofre direto, sem conferir papel, com a conta como membro.
  withVault(tenant: string, name: string): Vault {
    this.notes.in(tenant);
    return this.vaults.create(tenant, name, this.roleIn(tenant) ?? "");
  }

  // withNote grava a nota direto, sem conferir papel, para montar o cenário.
  withNote(
    tenant: string,
    path: string,
    content: string,
    vault = DefaultVaultId,
  ): Note {
    const note = this.newNote(path, content, vault);
    this.notes.in(tenant).push(note);
    return note;
  }

  // withLink grava a ligação de source para target, como o indexador faria.
  withLink(source: Note, target: Note, type: LinkType): this {
    this.links.add(source, target, type);
    return this;
  }

  concurrentWriteOnNextRead(id: string, content: string): void {
    this.notes.concurrentWriteOnNextRead(id, content);
  }

  stored(tenant: string, id: string): Note {
    return this.notes.find(tenant, id);
  }

  async getInstanceEmbedding(): Promise<InstanceEmbedding> {
    return this.instanceEmbedding.view();
  }

  async setInstanceEmbedding(
    update: InstanceEmbeddingUpdate,
  ): Promise<InstanceEmbedding> {
    return this.instanceEmbedding.update(update);
  }

  async listLoginProviders(): Promise<LoginProviderList> {
    return this.loginProviders.list();
  }

  async createLoginProvider(
    creation: LoginProviderCreation,
  ): Promise<LoginProvider> {
    return this.loginProviders.create(creation);
  }

  async updateLoginProvider(
    id: string,
    change: LoginProviderChange,
  ): Promise<LoginProvider> {
    return this.loginProviders.update(id, change);
  }

  async deleteLoginProvider(id: string): Promise<LoginProviderRemoval> {
    return this.loginProviders.remove(id);
  }

  async getAccount(): Promise<Account> {
    return { id: FakeAccountId, email: FakeAccountEmail };
  }

  async listTenants(): Promise<TenantMembership[]> {
    return [...this.tenants];
  }

  // ponytail: casa substring e ordena por data, um substituto da busca de texto
  // completo por relevância da API; os testes não dependem do ranking dela.
  async searchNotes(
    tenant: string,
    query: string,
    page: SearchPage = {},
    vault?: string,
  ): Promise<FoundNote[]> {
    const offset = page.offset ?? 0;
    const needle = query.toLowerCase();
    return this.searchable(tenant, vault)
      .filter((note) => note.content.toLowerCase().includes(needle))
      .sort((a, b) => b.updated_at.localeCompare(a.updated_at))
      .slice(offset, offset + (page.limit ?? 10))
      .map((note) => (vault === undefined ? noteSummary(note) : found(note)));
  }

  async listNotes(tenant: string, vault?: string): Promise<NoteSummary[]> {
    return this.notesOf(tenant, vault).map(noteSummary);
  }

  async getNote(tenant: string, id: string): Promise<Note> {
    return this.notes.read(tenant, id);
  }

  async createNote(
    tenant: string,
    path: string,
    content: string,
    vault?: string,
  ): Promise<Note> {
    this.requireWriter(tenant);
    const target = vault ?? DefaultVaultId;
    const inVault = this.notesOf(tenant, target);
    if (inVault.some((note) => note.path === path)) {
      throw new MarkuppApiError(409, "duplicate_path", "path já existe");
    }
    return { ...this.withNote(tenant, path, content, target) };
  }

  async updateNote(
    tenant: string,
    id: string,
    revision: NoteRevision,
  ): Promise<Note> {
    this.requireWriter(tenant);
    const note = this.notes.find(tenant, id);
    if (note.updated_at !== revision.lastModifiedAt) {
      throw new MarkuppApiError(
        409,
        "conflict",
        "nota foi atualizada por outro cliente",
      );
    }
    this.revisions.push(revision);
    Object.assign(note, {
      path: revision.path,
      content: revision.content,
      updated_at: this.tick(),
    });
    return { ...note };
  }

  async deleteNote(tenant: string, id: string): Promise<void> {
    this.requireWriter(tenant);
    this.notes.remove(tenant, id);
  }

  // listMembers serve a qualquer membro do tenant, que escolhe ali quem pôr no cofre.
  async listMembers(tenant: string): Promise<Member[]> {
    return this.membersIn(tenant);
  }

  async changeMemberRole(
    tenant: string,
    userId: string,
    role: Role,
  ): Promise<void> {
    this.requireAdmin(tenant);
    this.members.changeRole(tenant, userId, role);
  }

  async removeMember(tenant: string, userId: string): Promise<void> {
    this.requireAdmin(tenant);
    this.members.remove(tenant, userId);
  }

  async listInvites(tenant: string): Promise<TenantInvite[]> {
    this.requireAdmin(tenant);
    return this.invitesIn(tenant);
  }

  // inviteMember age como o servidor sem email: devolve o token para repasse à mão.
  async inviteMember(
    tenant: string,
    email: string,
    role: Role,
  ): Promise<IssuedInvite> {
    this.requireAdmin(tenant);
    this.members.requireNotMember(tenant, email);
    const invite = this.withInvite(tenant, email, role);
    return { ...invite, token: `token-${invite.id}` };
  }

  async revokeInvite(tenant: string, inviteId: string): Promise<void> {
    this.requireAdmin(tenant);
    this.invites.revoke(tenant, inviteId);
  }

  async createTenant(name: string): Promise<TenantMembership> {
    const membership = { id: `tenant-${this.tick()}`, name, role: "owner" };
    this.withTenant(membership.id, membership.role, membership.name);
    return membership;
  }

  async listMyInvites(): Promise<PendingInvite[]> {
    return this.myInvites.list();
  }

  async acceptInvite(inviteId: string): Promise<TenantMembership> {
    const invite = this.myInvites.take(inviteId);
    if (this.tenants.some((t) => t.id === invite.tenant_id)) {
      throw new MarkuppApiError(
        409,
        "already_member",
        "this account is already a member of the tenant",
      );
    }
    this.withTenant(invite.tenant_id, invite.role, invite.tenant_name);
    return {
      id: invite.tenant_id,
      name: invite.tenant_name,
      role: invite.role,
    };
  }

  async declineInvite(inviteId: string): Promise<void> {
    this.myInvites.take(inviteId);
  }

  async listVaults(tenant: string): Promise<Vault[]> {
    this.notes.in(tenant);
    return this.vaults.list(tenant);
  }

  async createVault(tenant: string, name: string): Promise<Vault> {
    this.requireWriter(tenant);
    return this.withVault(tenant, name);
  }

  async renameVault(
    tenant: string,
    vault: string,
    name: string,
  ): Promise<Vault> {
    this.requireWriter(tenant);
    return this.vaults.rename(tenant, vault, name);
  }

  // deleteVault leva as notas do cofre junto, como o ON DELETE CASCADE do banco.
  async deleteVault(tenant: string, vault: string): Promise<void> {
    this.requireWriter(tenant);
    this.vaults.delete(tenant, vault);
    this.notes.removeVault(tenant, vault);
  }

  async listVaultMembers(tenant: string, vault: string): Promise<Member[]> {
    this.notes.in(tenant);
    return this.vaults.members(tenant, vault);
  }

  async addVaultMember(
    tenant: string,
    vault: string,
    userId: string,
  ): Promise<void> {
    this.requireWriter(tenant);
    const member = this.members.lookup(tenant, userId);
    this.vaults.add(tenant, vault, userId, member);
  }

  // removeVaultMember deixa o viewer tirar só a si mesmo, que é sair do cofre.
  async removeVaultMember(
    tenant: string,
    vault: string,
    userId: string,
  ): Promise<void> {
    if (userId === FakeAccountId) this.notes.in(tenant);
    else this.requireWriter(tenant);
    this.vaults.remove(tenant, vault, userId);
  }

  async listLinks(
    tenant: string,
    note: string,
    query: LinkQuery,
  ): Promise<Link[]> {
    this.notes.find(tenant, note);
    return this.links.list(note, query);
  }

  async listLinkRefusals(
    tenant: string,
    vault: string,
    page: SearchPage = {},
  ): Promise<LinkRefusal[]> {
    this.notes.in(tenant);
    this.vaults.find(tenant, vault);
    return this.links.refusalsIn(vault, page);
  }

  async refuseLink(
    tenant: string,
    vault: string,
    noteIds: string[],
  ): Promise<LinkRefusal> {
    this.requireWriter(tenant);
    this.vaults.find(tenant, vault);
    return this.links.refuse(vault, noteIds, this.notes.in(tenant));
  }

  async undoLinkRefusal(
    tenant: string,
    vault: string,
    refusalId: string,
  ): Promise<void> {
    this.requireWriter(tenant);
    this.vaults.find(tenant, vault);
    this.links.undo(vault, refusalId);
  }

  private requireAdmin(tenant: string): void {
    this.notes.in(tenant);
    const role = this.tenants.find((t) => t.id === tenant)?.role;
    if (role !== "owner" && role !== "admin") {
      throw new MarkuppApiError(
        403,
        "forbidden",
        "only tenant owners and admins can do this",
      );
    }
  }

  async publish(
    tenant: string,
    vault: string,
    request: PublishRequest,
  ): Promise<Publication> {
    const notes = this.notesOf(tenant, vault);
    const role = this.roleIn(tenant) ?? "";
    const found = this.vaults.find(tenant, vault);
    const scope = { tenant, role, notes, vault: found };
    return this.publications.publish(scope, request);
  }

  async unpublish(tenant: string, publication: string): Promise<void> {
    this.notes.in(tenant);
    this.publications.remove(tenant, publication);
  }

  async listPublications(
    tenant: string,
    vault?: string,
  ): Promise<PublicationList> {
    this.notes.in(tenant);
    return this.publications.list(tenant, vault);
  }

  private requireWriter(tenant: string): void {
    this.notes.in(tenant);
    if (this.roleIn(tenant) === "viewer") {
      throw new MarkuppApiError(
        403,
        "read_only",
        "the viewer role can only read",
      );
    }
  }

  private roleIn(tenant: string): string | undefined {
    return this.tenants.find((t) => t.id === tenant)?.role;
  }

  // notesOf devolve as notas do tenant ou, com vault, só as do cofre que a conta alcança.
  private notesOf(tenant: string, vault?: string): Note[] {
    const all = this.notes.in(tenant);
    if (vault === undefined) return all;
    if (vault !== DefaultVaultId) this.vaults.find(tenant, vault);
    return all.filter((note) => note.vault_id === vault);
  }

  // searchable são as notas em que a busca procura: cofre fora do alcance não casa nada,
  // como o RLS da busca da API, em vez de responder vault_forbidden.
  private searchable(tenant: string, vault?: string): Note[] {
    try {
      return this.notesOf(tenant, vault);
    } catch (error) {
      if (error instanceof MarkuppApiError && error.code === "vault_forbidden")
        return [];
      throw error;
    }
  }

  private newNote(path: string, content: string, vault: string): Note {
    const at = this.tick();
    const id = `note-${this.clockTick}`;
    return {
      id,
      vault_id: vault,
      path,
      content,
      created_at: at,
      updated_at: at,
    };
  }

  // tick avança um relógio lógico, um segundo por escrita, para o updated_at mudar sempre.
  private tick(): string {
    this.clockTick += 1;
    return new Date(Date.UTC(2026, 0, 1, 0, 0, this.clockTick)).toISOString();
  }
}

// found é a nota como a busca num cofre devolve, sem updated_at.
function found(note: Note): FoundNote {
  const { id, vault_id, path } = note;
  return { id, vault_id, path };
}
