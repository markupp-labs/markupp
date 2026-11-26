import { withQuery } from "./api-paths.js";
import type {
  AccountQuery,
  AppAuthorization,
  AuditEvent,
  AuditQuery,
  InstanceAccount,
  InstanceAccountDetail,
  IpBlock,
  IpBlockCreation,
  Retention,
  Session,
} from "./markupp-api-types.js";

// InstanceAccessApi é a fatia da API das sessões da conta e da gestão de contas e acessos
// pelo administrador da instância (ADR-0061). Nenhuma chamada leva tenant.
export interface InstanceAccessApi {
  listMySessions(): Promise<Session[]>;
  endMySession(id: string): Promise<void>;
  listAccounts(query: AccountQuery): Promise<InstanceAccount[]>;
  getInstanceAccount(id: string): Promise<InstanceAccountDetail>;
  setAccountBlocked(
    id: string,
    blocked: boolean,
  ): Promise<InstanceAccountDetail>;
  deleteInstanceAccount(id: string): Promise<void>;
  listAccountSessions(id: string): Promise<Session[]>;
  endAccountSession(id: string, sessionId: string): Promise<void>;
  listAccountAuthorizations(id: string): Promise<AppAuthorization[]>;
  revokeAccountAuthorization(
    id: string,
    authorizationId: string,
  ): Promise<void>;
  listIpBlocks(): Promise<IpBlock[]>;
  blockIp(creation: IpBlockCreation): Promise<IpBlock>;
  unblockIp(id: string): Promise<void>;
  listAuditEvents(query: AuditQuery): Promise<AuditEvent[]>;
  getRetention(): Promise<Retention>;
  setRetention(days: number): Promise<Retention>;
}

// InstanceAccessCall é uma chamada sem tenant: json lê a resposta, send só confere o status.
export type InstanceAccessCall = {
  method: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  path: string;
  body?: unknown;
};

export type InstanceAccessTransport = {
  json: <T>(call: InstanceAccessCall) => Promise<T>;
  send: (call: InstanceAccessCall) => Promise<void>;
};

const Accounts = "/v1/instance/accounts";
const IpBlocks = "/v1/instance/ip-blocks";
const Retention = "/v1/instance/retention";

function segment(value: string): string {
  return encodeURIComponent(value);
}

function accountPath(id: string, ...rest: string[]): string {
  return [Accounts, id, ...rest]
    .map((part, i) => (i === 0 ? part : segment(part)))
    .join("/");
}

// queryOf põe na query string só os filtros que vieram, na ordem dada.
function queryOf(
  filters: Record<string, string | number | undefined>,
): URLSearchParams {
  const params = new URLSearchParams();
  for (const [name, value] of Object.entries(filters)) {
    if (value !== undefined && value !== "") params.set(name, String(value));
  }
  return params;
}

// instanceAccessCalls monta as chamadas sobre o transporte do HttpMarkuppApi.
export function instanceAccessCalls(
  t: InstanceAccessTransport,
): InstanceAccessApi {
  return {
    listMySessions: () => t.json({ method: "GET", path: "/v1/me/sessions" }),
    endMySession: (id) =>
      t.send({ method: "DELETE", path: `/v1/me/sessions/${segment(id)}` }),
    listAccounts: (q) =>
      t.json({
        method: "GET",
        path: withQuery(
          Accounts,
          queryOf({
            q: q.q,
            status: q.status,
            limit: q.limit,
            offset: q.offset,
          }),
        ),
      }),
    getInstanceAccount: (id) =>
      t.json({ method: "GET", path: accountPath(id) }),
    setAccountBlocked: (id, blocked) =>
      t.json({ method: "PATCH", path: accountPath(id), body: { blocked } }),
    deleteInstanceAccount: (id) =>
      t.send({ method: "DELETE", path: accountPath(id) }),
    listAccountSessions: (id) =>
      t.json({ method: "GET", path: accountPath(id, "sessions") }),
    endAccountSession: (id, sessionId) =>
      t.send({
        method: "DELETE",
        path: accountPath(id, "sessions", sessionId),
      }),
    listAccountAuthorizations: (id) =>
      t.json({ method: "GET", path: accountPath(id, "authorizations") }),
    revokeAccountAuthorization: (id, authorizationId) =>
      t.send({
        method: "DELETE",
        path: accountPath(id, "authorizations", authorizationId),
      }),
    listIpBlocks: () => t.json({ method: "GET", path: IpBlocks }),
    blockIp: (creation) =>
      t.json({ method: "POST", path: IpBlocks, body: creation }),
    unblockIp: (id) =>
      t.send({ method: "DELETE", path: `${IpBlocks}/${segment(id)}` }),
    listAuditEvents: (q) =>
      t.json({
        method: "GET",
        path: withQuery("/v1/instance/audit-events", queryOf(q)),
      }),
    getRetention: () => t.json({ method: "GET", path: Retention }),
    setRetention: (days) =>
      t.json({ method: "PUT", path: Retention, body: { days } }),
  };
}
