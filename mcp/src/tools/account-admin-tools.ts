import type { FastMCP } from "fastmcp";
import { z } from "zod";
import type { MarkuppSession } from "../session.js";
import {
  apiFor,
  callApi,
  readOnlyAnnotations,
  writeAnnotations,
  type MarkuppApiFactory,
} from "./tool-support.js";

const accountId = z.string().min(1).describe("Account id from list_accounts.");

const destructive = {
  ...writeAnnotations,
  destructiveHint: true,
  idempotentHint: true,
};

type Server = FastMCP<MarkuppSession>;

// addAccountAdminTools registra as ferramentas das contas da instância, do administrador
// da instância (ADR-0061). A API recusa as outras contas.
export function addAccountAdminTools(
  server: Server,
  factory: MarkuppApiFactory,
): void {
  addAccountReads(server, factory);
  addAccountBlocking(server, factory);
  addAccountAccessCuts(server, factory);
}

function addAccountReads(server: Server, factory: MarkuppApiFactory): void {
  server.addTool({
    name: "list_accounts",
    description:
      "Instance admins only. Lists the accounts of the instance, newest first, with email, whether it is verified, how they sign in, creation, last access and whether they are blocked.",
    parameters: z.object({
      q: z.string().optional().describe("Part of the email, case insensitive."),
      status: z.enum(["active", "blocked"]).optional(),
      limit: z.number().int().min(1).max(200).optional(),
      offset: z.number().int().min(0).max(10000).optional(),
    }),
    annotations: readOnlyAnnotations,
    execute: async (query, context) => {
      const api = apiFor(factory, context);
      return JSON.stringify(await callApi(() => api.listAccounts(query)));
    },
  });
  server.addTool({
    name: "get_account",
    description:
      "Instance admins only. Describes an account, including the tenants that would be deleted with it because it is their only owner.",
    parameters: z.object({ id: accountId }),
    annotations: readOnlyAnnotations,
    execute: async ({ id }, context) => {
      const api = apiFor(factory, context);
      return JSON.stringify(await callApi(() => api.getInstanceAccount(id)));
    },
  });
}

function addAccountBlocking(server: Server, factory: MarkuppApiFactory): void {
  server.addTool({
    name: "block_account",
    description:
      "Instance admins only, when the user asked for it. Blocks an account: it cannot sign in by any method, its sessions and authorized apps are cut, and access ends within minutes. Its notes stay. You cannot block your own account.",
    parameters: z.object({ id: accountId }),
    annotations: { ...writeAnnotations, idempotentHint: true },
    execute: async ({ id }, context) => {
      const api = apiFor(factory, context);
      return JSON.stringify(
        await callApi(() => api.setAccountBlocked(id, true)),
      );
    },
  });
  server.addTool({
    name: "unblock_account",
    description:
      "Instance admins only, when the user asked for it. Lets a blocked account sign in again.",
    parameters: z.object({ id: accountId }),
    annotations: { ...writeAnnotations, idempotentHint: true },
    execute: async ({ id }, context) => {
      const api = apiFor(factory, context);
      return JSON.stringify(
        await callApi(() => api.setAccountBlocked(id, false)),
      );
    },
  });
  server.addTool({
    name: "delete_account",
    description:
      "Instance admins only, when the user asked for that account to be deleted. Deletes the account and every tenant where it is the only owner, with vaults and notes, which cannot be undone; check get_account first and tell the user what goes with it. You cannot delete your own account.",
    parameters: z.object({ id: accountId }),
    annotations: destructive,
    execute: async ({ id }, context) => {
      const api = apiFor(factory, context);
      await callApi(() => api.deleteInstanceAccount(id));
      return `Account ${id} deleted.`;
    },
  });
}

function addAccountAccessCuts(
  server: Server,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "list_account_sessions",
    description:
      "Instance admins only. Lists the active sign-in sessions of an account with IP, user agent and last use.",
    parameters: z.object({ id: accountId }),
    annotations: readOnlyAnnotations,
    execute: async ({ id }, context) => {
      const api = apiFor(factory, context);
      return JSON.stringify(await callApi(() => api.listAccountSessions(id)));
    },
  });
  server.addTool({
    name: "end_account_session",
    description:
      "Instance admins only, when the user asked for it. Ends a sign-in session of an account.",
    parameters: z.object({
      id: accountId,
      session_id: z
        .string()
        .min(1)
        .describe("Session id from list_account_sessions."),
    }),
    annotations: destructive,
    execute: async ({ id, session_id }, context) => {
      const api = apiFor(factory, context);
      await callApi(() => api.endAccountSession(id, session_id));
      return `Session ${session_id} of account ${id} ended.`;
    },
  });
  server.addTool({
    name: "list_account_authorizations",
    description:
      "Instance admins only. Lists the apps an account authorized, such as MCP clients, with IP, user agent and last use.",
    parameters: z.object({ id: accountId }),
    annotations: readOnlyAnnotations,
    execute: async ({ id }, context) => {
      const api = apiFor(factory, context);
      return JSON.stringify(
        await callApi(() => api.listAccountAuthorizations(id)),
      );
    },
  });
  server.addTool({
    name: "revoke_account_authorization",
    description:
      "Instance admins only, when the user asked for it. Revokes an app authorized by an account and every token it holds.",
    parameters: z.object({
      id: accountId,
      authorization_id: z
        .string()
        .min(1)
        .describe("Authorization id from list_account_authorizations."),
    }),
    annotations: destructive,
    execute: async ({ id, authorization_id }, context) => {
      const api = apiFor(factory, context);
      await callApi(() => api.revokeAccountAuthorization(id, authorization_id));
      return `Authorization ${authorization_id} of account ${id} revoked.`;
    },
  });
}
