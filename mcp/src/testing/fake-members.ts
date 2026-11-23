import { MarkuppApiError, type Member } from "../markupp-api.js";

// FakeMembers guarda os membros de cada tenant e responde member_not_found e
// already_member como a API.
// Tenant e papel ficam com a FakeMarkuppApi, que confere tudo antes de chamar.
export class FakeMembers {
  private readonly byTenant = new Map<string, Member[]>();

  add(tenant: string, member: Member): Member {
    this.of(tenant).push(member);
    return member;
  }

  listed(tenant: string): Member[] {
    return [...this.of(tenant)];
  }

  // lookup devolve o membro ou undefined, sem erro, para quem só quer conferir.
  lookup(tenant: string, userId: string): Member | undefined {
    return this.of(tenant).find((m) => m.id === userId);
  }

  changeRole(tenant: string, userId: string, role: string): void {
    this.find(tenant, userId).role = role;
  }

  remove(tenant: string, userId: string): void {
    const list = this.of(tenant);
    list.splice(list.indexOf(this.find(tenant, userId)), 1);
  }

  requireNotMember(tenant: string, email: string): void {
    if (!this.of(tenant).some((m) => m.email === email)) return;
    throw new MarkuppApiError(
      409,
      "already_member",
      "email already belongs to a user of the tenant",
    );
  }

  private find(tenant: string, userId: string): Member {
    const member = this.lookup(tenant, userId);
    if (!member)
      throw new MarkuppApiError(
        404,
        "member_not_found",
        "account is not a member of this tenant",
      );
    return member;
  }

  private of(tenant: string): Member[] {
    const list = this.byTenant.get(tenant) ?? [];
    this.byTenant.set(tenant, list);
    return list;
  }
}
