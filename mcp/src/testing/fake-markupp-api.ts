import type { ApiTokenSource } from "../session.js";
import {
  MarkuppApiError,
  noteSummary,
  type MarkuppApi,
  type Note,
  type NoteRevision,
  type NoteSummary,
  type SearchPage,
  type TenantMembership,
} from "../markupp-api.js";

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

// FakeMarkuppApi guarda tenants e notas em memória e responde com os mesmos códigos
// de erro da API: tenant_forbidden, read_only, not_found, duplicate_path e conflict.
export class FakeMarkuppApi implements MarkuppApi {
  readonly revisions: NoteRevision[] = [];
  private readonly tenants: TenantMembership[] = [];
  private readonly notes = new Map<string, Note[]>();
  private clockTick = 0;
  private pendingWrite: PendingWrite | null = null;

  withTenant(id: string, role = "member", name = id): this {
    this.tenants.push({ id, name, role });
    this.notes.set(id, []);
    return this;
  }

  // withNote grava a nota direto, sem conferir papel, para montar o cenário.
  withNote(tenant: string, path: string, content: string): Note {
    const note = this.newNote(path, content);
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
  ): Promise<NoteSummary[]> {
    const offset = page.offset ?? 0;
    const needle = query.toLowerCase();
    return this.notesIn(tenant)
      .filter((note) => note.content.toLowerCase().includes(needle))
      .sort((a, b) => b.updated_at.localeCompare(a.updated_at))
      .slice(offset, offset + (page.limit ?? 10))
      .map(noteSummary);
  }

  async listNotes(tenant: string): Promise<NoteSummary[]> {
    return this.notesIn(tenant).map(noteSummary);
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
  ): Promise<Note> {
    this.requireWriter(tenant);
    if (this.notesIn(tenant).some((note) => note.path === path)) {
      throw new MarkuppApiError(409, "duplicate_path", "path já existe");
    }
    return { ...this.withNote(tenant, path, content) };
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
    const role = this.tenants.find((t) => t.id === tenant)?.role;
    if (role === "viewer") {
      throw new MarkuppApiError(
        403,
        "read_only",
        "o papel viewer so le as notas",
      );
    }
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

  private newNote(path: string, content: string): Note {
    const at = this.tick();
    const id = `note-${this.clockTick}`;
    return {
      id,
      vault_id: "vault-default",
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
