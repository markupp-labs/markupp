import { MarkuppApiError, type Member, type Vault } from "../markupp-api.js";

// FakeAccountId e FakeAccountEmail são a conta da sessão na API falsa.
export const FakeAccountId = "me";
export const FakeAccountEmail = "me@markupp.test";

type StoredVault = Vault & { tenant: string; members: Member[] };

// FakeVaults guarda os cofres de cada tenant e os membros de cada um, e só alcança os
// cofres de que a conta da sessão é membro, como o RLS da API. Papel e tenant ficam com
// a FakeMarkuppApi, que confere os dois antes de chamar.
export class FakeVaults {
  private readonly stored: StoredVault[] = [];
  private count = 0;

  constructor(private readonly clock: () => string) {}

  // create grava o cofre com a conta da sessão como primeiro membro, com o papel role.
  create(tenant: string, name: string, role: string): Vault {
    this.count += 1;
    const at = this.clock();
    const vault = { id: `vault-${this.count}`, name, created_at: at };
    const account = {
      id: FakeAccountId,
      email: FakeAccountEmail,
      role,
      joined_at: at,
    };
    this.stored.push({ ...vault, tenant, members: [account] });
    return vault;
  }

  list(tenant: string): Vault[] {
    return this.stored
      .filter((vault) => vault.tenant === tenant && hasAccount(vault))
      .map(publicVault);
  }

  rename(tenant: string, id: string, name: string): Vault {
    const vault = this.find(tenant, id);
    vault.name = name;
    return publicVault(vault);
  }

  delete(tenant: string, id: string): void {
    this.stored.splice(this.stored.indexOf(this.find(tenant, id)), 1);
  }

  members(tenant: string, id: string): Member[] {
    return [...this.find(tenant, id).members];
  }

  // add põe no cofre a conta do tenant; member vem undefined quando ela não é do tenant.
  add(tenant: string, id: string, userId: string, member?: Member): void {
    const vault = this.find(tenant, id);
    if (!member) throw notTenantMember(userId);
    if (vault.members.some((m) => m.id === userId))
      throw new MarkuppApiError(
        409,
        "already_vault_member",
        `conta ${userId} ja e membro do cofre`,
      );
    vault.members.push({ ...member, joined_at: this.clock() });
  }

  remove(tenant: string, id: string, userId: string): void {
    const list = this.find(tenant, id).members;
    const index = list.findIndex((m) => m.id === userId);
    if (index < 0)
      throw new MarkuppApiError(
        404,
        "vault_member_not_found",
        `conta ${userId} nao e membro do cofre`,
      );
    list.splice(index, 1);
  }

  // find acha o cofre do tenant de que a conta é membro, ou responde vault_forbidden.
  find(tenant: string, id: string): StoredVault {
    const vault = this.stored.find(
      (v) => v.tenant === tenant && v.id === id && hasAccount(v),
    );
    if (!vault)
      throw new MarkuppApiError(
        403,
        "vault_forbidden",
        `cofre ${id} inexistente ou fora do alcance da conta`,
      );
    return vault;
  }
}

function hasAccount(vault: StoredVault): boolean {
  return vault.members.some((member) => member.id === FakeAccountId);
}

function publicVault(vault: StoredVault): Vault {
  const { id, name, created_at } = vault;
  return { id, name, created_at };
}

function notTenantMember(userId: string): MarkuppApiError {
  return new MarkuppApiError(
    422,
    "not_tenant_member",
    `conta ${userId} nao pertence ao tenant`,
  );
}
