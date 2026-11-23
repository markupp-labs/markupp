import { MarkuppApiError, type Note } from "../markupp-api.js";

type PendingWrite = { id: string; content: string };

// FakeNotes guarda as notas de cada tenant e responde tenant_forbidden ao tenant que a
// conta não alcança e not_found à nota que não existe.
// Papel e cofres ficam com a FakeMarkuppApi, que confere tudo antes de chamar.
export class FakeNotes {
  private readonly byTenant = new Map<string, Note[]>();
  private pendingWrite: PendingWrite | null = null;

  constructor(private readonly clock: () => string) {}

  open(tenant: string): void {
    this.byTenant.set(tenant, []);
  }

  // in devolve a lista viva do tenant e é também a conferência de acesso a ele.
  in(tenant: string): Note[] {
    const list = this.byTenant.get(tenant);
    if (!list)
      throw new MarkuppApiError(
        403,
        "tenant_forbidden",
        "this account has no access to this tenant",
      );
    return list;
  }

  find(tenant: string, id: string): Note {
    const note = this.in(tenant).find((n) => n.id === id);
    if (!note)
      throw new MarkuppApiError(404, "not_found", "nota não encontrada");
    return note;
  }

  remove(tenant: string, id: string): void {
    const list = this.in(tenant);
    list.splice(list.indexOf(this.find(tenant, id)), 1);
  }

  removeVault(tenant: string, vault: string): void {
    const kept = this.in(tenant).filter((n) => n.vault_id !== vault);
    this.byTenant.set(tenant, kept);
  }

  // concurrentWriteOnNextRead simula outro cliente gravando logo depois da próxima
  // leitura da nota id, o que deixa velho o updated_at que essa leitura devolveu.
  concurrentWriteOnNextRead(id: string, content: string): void {
    this.pendingWrite = { id, content };
  }

  // read devolve uma cópia da nota e só depois aplica a escrita concorrente pendente.
  read(tenant: string, id: string): Note {
    const note = { ...this.find(tenant, id) };
    this.applyPendingWrite(tenant, id);
    return note;
  }

  private applyPendingWrite(tenant: string, id: string): void {
    if (this.pendingWrite?.id !== id) return;
    Object.assign(this.find(tenant, id), {
      content: this.pendingWrite.content,
      updated_at: this.clock(),
    });
    this.pendingWrite = null;
  }
}
