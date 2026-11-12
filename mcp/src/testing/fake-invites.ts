import type { TenantInvite } from "../markupp-api.js";
import { inviteNotFound } from "./fake-my-invites.js";

// FakeInvites guarda os convites de cada tenant; os endereçados à conta da sessão ficam
// com a FakeMyInvites.
// Tenant, papel e membros ficam com a FakeMarkuppApi, que confere tudo antes de chamar.
export class FakeInvites {
  private readonly byTenant = new Map<string, TenantInvite[]>();
  private count = 0;

  constructor(private readonly clock: () => string) {}

  issue(tenant: string, email: string, role: string): TenantInvite {
    const at = this.clock();
    const id = `invite-${this.next()}`;
    const invite = { id, email, role, expires_at: at, created_at: at };
    this.of(tenant).push(invite);
    return invite;
  }

  listed(tenant: string): TenantInvite[] {
    return [...this.of(tenant)];
  }

  revoke(tenant: string, inviteId: string): void {
    const list = this.of(tenant);
    const index = list.findIndex((invite) => invite.id === inviteId);
    if (index < 0) throw inviteNotFound();
    list.splice(index, 1);
  }

  private of(tenant: string): TenantInvite[] {
    const list = this.byTenant.get(tenant) ?? [];
    this.byTenant.set(tenant, list);
    return list;
  }

  private next(): number {
    this.count += 1;
    return this.count;
  }
}
