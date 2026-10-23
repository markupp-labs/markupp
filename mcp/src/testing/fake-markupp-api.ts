import type { ApiTokenSource } from "../session.js";
import {
  MarkuppApiError,
  noteSummary,
  type FoundNote,
  type IssuedInvite,
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
import { FakeAccountId, FakeVaults } from "./fake-vaults.js";

// FakeApiTokens entrega os JWTs na ordem dada e repete o último quando acabam.
export class FakeApiTokens {
  private readonly tokens: string[];

  constructor(...tokens: [string, ...string[]]) {
    this.tokens = tokens;
  }

  readonly source: ApiTokenSource = async () =>
    this.tokens.length > 1
      ? (this.tokens.shift() ?? "")
      : (this.tokens[0] ?? "");
}

type PendingWrite = { id: string; content: string };

// DefaultVaultId é o cofre das notas criadas sem cofre, fora da lista de cofres.
const DefaultVaultId = "vault-default";

// FakeMarkuppApi guarda tenants, notas, membros, convites e cofres em memória e responde
// com os mesmos códigos de erro da API: tenant_forbidden, read_only, not_found,
// duplicate_path, conflict, forbidden, member_not_found, invite_not_found,
// already_member e os dos cofres, que FakeVaults descreve.
export class FakeMarkuppApi implements MarkuppApi {
  readonly revisions: NoteRevision[] = [];
  private readonly tenants: TenantMembership[] = [];
  private readonly notes = new Map<string, Note[]>();
  private readonly members = new Map<string, Member[]>();
  private readonly invites = new Map<string, TenantInvite[]>();
  private readonly myInvites: PendingInvite[] = [];
  private clockTick = 0;
  private pendingWrite: PendingWrite | null = null;
  private readonly vaults = new FakeVaults(() => this.tick());

  withTenant(id: string, role = "member", name = id): this {
    this.tenants.push({ id, name, role });
    this.notes.set(id, []);
    this.members.set(id, []);
    this.invites.set(id, []);
    return this;
  }

  // withMember grava o membro direto, sem conferir papel, para montar o cenário.
  withMember(tenant: string, id: string, email: string, role: string): Member {
    const member = { id, email, role, joined_at: this.tick() };
    this.membersOf(tenant).push(member);
    return member;
  }

  withInvite(tenant: string, email: string, role: string): TenantInvite {
    const at = this.tick();
    const invite = {
      id: `invite-${this.clockTick}`,
      email,
      role,
      expires_at: at,
      created_at: at,
    };
    this.invitesOf(tenant).push(invite);
    return invite;
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
    this.myInvites.push(invite);
    return invite;
  }

  membersIn(tenant: string): Member[] {
    return [...this.membersOf(tenant)];
  }

  invitesIn(tenant: string): TenantInvite[] {
    return [...this.invitesOf(tenant)];
  }

  // withVault grava o cofre direto, sem conferir papel, com a conta como membro.
  withVault(tenant: string, name: string): Vault {
    this.notesIn(tenant);
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
    this.notesIn(tenant).push(note);
    return note;
  }

  // concurrentWriteOnNextRead simula outro cliente gravando logo depois da próxima
  // leitura da nota id, o que deixa velho o updated_at que essa leitura devolveu.
  concurrentWriteOnNextRead(id: string, content: string): void {
    this.pendingWrite = { id, content };
  }

  stored(tenant: string, id: string): Note {
    return this.find(tenant, id);
  }

  async listTenants(): Promise<TenantMembership[]> {
    return [...this.tenants];
  }

  async searchNotes(
    tenant: string,
    query: string,
    page: SearchPage = {},
    vault?: string,
  ): Promise<FoundNote[]> {
    const offset = page.offset ?? 0;
    const needle = query.toLowerCase();
    return this.notesOf(tenant, vault)
      .filter((note) => note.content.toLowerCase().includes(needle))
      .sort((a, b) => b.updated_at.localeCompare(a.updated_at))
      .slice(offset, offset + (page.limit ?? 10))
      .map((note) => (vault === undefined ? noteSummary(note) : found(note)));
  }

  async listNotes(tenant: string, vault?: string): Promise<NoteSummary[]> {
    return this.notesOf(tenant, vault).map(noteSummary);
  }

  async getNote(tenant: string, id: string): Promise<Note> {
    const note = { ...this.find(tenant, id) };
    this.applyPendingWrite(tenant, id);
    return note;
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
    const note = this.find(tenant, id);
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
    const list = this.notesIn(tenant);
    list.splice(list.indexOf(this.find(tenant, id)), 1);
  }

  async listMembers(tenant: string): Promise<Member[]> {
    this.requireAdmin(tenant);
    return this.membersIn(tenant);
  }

  async changeMemberRole(
    tenant: string,
    userId: string,
    role: Role,
  ): Promise<void> {
    this.requireAdmin(tenant);
    this.findMember(tenant, userId).role = role;
  }

  async removeMember(tenant: string, userId: string): Promise<void> {
    this.requireAdmin(tenant);
    const list = this.membersOf(tenant);
    list.splice(list.indexOf(this.findMember(tenant, userId)), 1);
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
    if (this.membersOf(tenant).some((m) => m.email === email)) {
      throw new MarkuppApiError(
        409,
        "already_member",
        "email ja pertence a um usuario do tenant",
      );
    }
    const invite = this.withInvite(tenant, email, role);
    return { ...invite, token: `token-${invite.id}` };
  }

  async revokeInvite(tenant: string, inviteId: string): Promise<void> {
    this.requireAdmin(tenant);
    const list = this.invitesOf(tenant);
    const index = list.findIndex((invite) => invite.id === inviteId);
    if (index < 0) throw inviteNotFound();
    list.splice(index, 1);
  }

  async createTenant(name: string): Promise<TenantMembership> {
    const membership = { id: `tenant-${this.tick()}`, name, role: "owner" };
    this.withTenant(membership.id, membership.role, membership.name);
    return membership;
  }

  async listMyInvites(): Promise<PendingInvite[]> {
    return [...this.myInvites];
  }

  async acceptInvite(inviteId: string): Promise<TenantMembership> {
    const invite = this.takeMyInvite(inviteId);
    if (this.tenants.some((t) => t.id === invite.tenant_id)) {
      throw new MarkuppApiError(
        409,
        "already_member",
        "a conta ja e membro desse tenant",
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
    this.takeMyInvite(inviteId);
  }

  async listVaults(tenant: string): Promise<Vault[]> {
    this.notesIn(tenant);
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
    const kept = this.notesIn(tenant).filter((n) => n.vault_id !== vault);
    this.notes.set(tenant, kept);
  }

  async listVaultMembers(tenant: string, vault: string): Promise<Member[]> {
    this.notesIn(tenant);
    return this.vaults.members(tenant, vault);
  }

  async addVaultMember(
    tenant: string,
    vault: string,
    userId: string,
  ): Promise<void> {
    this.requireWriter(tenant);
    const member = this.membersOf(tenant).find((m) => m.id === userId);
    this.vaults.add(tenant, vault, userId, member);
  }

  // removeVaultMember deixa o viewer tirar só a si mesmo, que é sair do cofre.
  async removeVaultMember(
    tenant: string,
    vault: string,
    userId: string,
  ): Promise<void> {
    if (userId === FakeAccountId) this.notesIn(tenant);
    else this.requireWriter(tenant);
    this.vaults.remove(tenant, vault, userId);
  }

  private takeMyInvite(inviteId: string): PendingInvite {
    const index = this.myInvites.findIndex((invite) => invite.id === inviteId);
    const [invite] = index < 0 ? [] : this.myInvites.splice(index, 1);
    if (!invite) throw inviteNotFound();
    return invite;
  }

  private requireAdmin(tenant: string): void {
    this.notesIn(tenant);
    const role = this.tenants.find((t) => t.id === tenant)?.role;
    if (role !== "owner" && role !== "admin") {
      throw new MarkuppApiError(
        403,
        "forbidden",
        "acesso restrito a administradores",
      );
    }
  }

  private findMember(tenant: string, userId: string): Member {
    const member = this.membersOf(tenant).find((m) => m.id === userId);
    if (!member)
      throw new MarkuppApiError(
        404,
        "member_not_found",
        "usuario nao e membro deste tenant",
      );
    return member;
  }

  private membersOf(tenant: string): Member[] {
    this.notesIn(tenant);
    return this.members.get(tenant) ?? [];
  }

  private invitesOf(tenant: string): TenantInvite[] {
    this.notesIn(tenant);
    return this.invites.get(tenant) ?? [];
  }

  private applyPendingWrite(tenant: string, id: string): void {
    if (this.pendingWrite?.id !== id) return;
    Object.assign(this.find(tenant, id), {
      content: this.pendingWrite.content,
      updated_at: this.tick(),
    });
    this.pendingWrite = null;
  }

  private requireWriter(tenant: string): void {
    this.notesIn(tenant);
    if (this.roleIn(tenant) === "viewer") {
      throw new MarkuppApiError(
        403,
        "read_only",
        "o papel viewer so le as notas",
      );
    }
  }

  private roleIn(tenant: string): string | undefined {
    return this.tenants.find((t) => t.id === tenant)?.role;
  }

  // notesOf devolve as notas do tenant ou, com vault, só as do cofre que a conta alcança.
  private notesOf(tenant: string, vault?: string): Note[] {
    const all = this.notesIn(tenant);
    if (vault === undefined) return all;
    if (vault !== DefaultVaultId) this.vaults.find(tenant, vault);
    return all.filter((note) => note.vault_id === vault);
  }

  private notesIn(tenant: string): Note[] {
    const list = this.notes.get(tenant);
    if (!list)
      throw new MarkuppApiError(
        403,
        "tenant_forbidden",
        "sem acesso a esse tenant",
      );
    return list;
  }

  private find(tenant: string, id: string): Note {
    const note = this.notesIn(tenant).find((n) => n.id === id);
    if (!note)
      throw new MarkuppApiError(404, "not_found", "nota não encontrada");
    return note;
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

function inviteNotFound(): MarkuppApiError {
  return new MarkuppApiError(
    404,
    "invite_not_found",
    "convite inexistente, vencido ou ja respondido",
  );
}
