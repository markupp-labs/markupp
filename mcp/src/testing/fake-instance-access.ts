import {
  MarkuppApiError,
  type AuditQuery,
  type InstanceAccount,
  type InstanceAccountDetail,
  type IpBlock,
  type Retention,
  type Session,
} from "../markupp-api.js";
import type { InstanceAccessApi } from "../instance-access-api.js";

const FakeAccessTime = "2026-11-25T12:00:00Z";

// FakeInstanceAccess guarda as sessões da conta e as contas, os bloqueios de IP, o
// histórico e o prazo da instância, e responde como a API: instance_admin_required para
// quem não administra, session_not_found e account_not_found.
export class FakeInstanceAccess {
  admin = false;
  private readonly sessions: Session[] = [];
  private readonly accounts = new Map<string, InstanceAccount>();
  private readonly ipBlocks: IpBlock[] = [];
  private retention: Retention = { days: 365, host_locked: false };
  lastAuditQuery: AuditQuery | undefined;

  seedSession(id: string, current: boolean): void {
    const at = FakeAccessTime;
    this.sessions.push({
      id,
      ip: "203.0.113.7",
      user_agent: "Firefox",
      created_at: at,
      last_used_at: at,
      current,
    });
  }

  seedAccount(id: string, email: string): void {
    this.accounts.set(id, {
      id,
      email,
      email_verified: true,
      login_methods: ["local"],
      created_at: FakeAccessTime,
      last_access_at: null,
      blocked: false,
    });
  }

  // api devolve as chamadas da API, como o HttpMarkuppApi as liga.
  api(): InstanceAccessApi {
    return {
      listMySessions: async () => [...this.sessions],
      endMySession: async (id) => this.endSession(id),
      listAccounts: async (q) => {
        this.requireAdmin();
        return [...this.accounts.values()].filter(
          (a) =>
            (!q.q || a.email.includes(q.q)) &&
            (!q.status || a.blocked === (q.status === "blocked")),
        );
      },
      getInstanceAccount: async (id) => this.detail(id),
      setAccountBlocked: async (id, blocked) => {
        this.detail(id);
        this.accounts.set(id, { ...this.accounts.get(id)!, blocked });
        return this.detail(id);
      },
      deleteInstanceAccount: async (id) => {
        this.detail(id);
        this.accounts.delete(id);
      },
      listAccountSessions: async (id) => {
        this.detail(id);
        return [];
      },
      endAccountSession: async () => {
        this.requireAdmin();
        throw new MarkuppApiError(
          404,
          "session_not_found",
          "session not found or already ended",
        );
      },
      listAccountAuthorizations: async (id) => {
        this.detail(id);
        return [];
      },
      revokeAccountAuthorization: async () => {
        this.requireAdmin();
        throw new MarkuppApiError(
          404,
          "authorization_not_found",
          "authorization not found or already revoked",
        );
      },
      listIpBlocks: async () => {
        this.requireAdmin();
        return [...this.ipBlocks];
      },
      blockIp: async (creation) => {
        this.requireAdmin();
        const block = {
          id: `b${this.ipBlocks.length + 1}`,
          cidr: creation.cidr,
          note: creation.note ?? "",
          created_at: FakeAccessTime,
        };
        this.ipBlocks.push(block);
        return block;
      },
      unblockIp: async (id) => {
        this.requireAdmin();
        this.ipBlocks.splice(
          this.ipBlocks.findIndex((b) => b.id === id),
          1,
        );
      },
      listAuditEvents: async (q) => {
        this.requireAdmin();
        this.lastAuditQuery = q;
        return [];
      },
      getRetention: async () => {
        this.requireAdmin();
        return this.retention;
      },
      setRetention: async (days) => {
        this.requireAdmin();
        this.retention = { days, host_locked: false };
        return this.retention;
      },
    };
  }

  private endSession(id: string): void {
    const index = this.sessions.findIndex((s) => s.id === id);
    if (index < 0) {
      throw new MarkuppApiError(
        404,
        "session_not_found",
        "session not found or already ended",
      );
    }
    this.sessions.splice(index, 1);
  }

  private detail(id: string): InstanceAccountDetail {
    this.requireAdmin();
    const account = this.accounts.get(id);
    if (!account)
      throw new MarkuppApiError(404, "account_not_found", "account not found");
    return { ...account, sole_owner_tenants: [] };
  }

  private requireAdmin(): void {
    if (!this.admin) {
      throw new MarkuppApiError(
        403,
        "instance_admin_required",
        "account is not an instance admin",
      );
    }
  }
}
