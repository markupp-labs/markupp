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

type Server = FastMCP<MarkuppSession>;

// addAccessTools registra as ferramentas do bloqueio de IP, do histórico e do prazo de
// guarda, do administrador da instância (ADR-0061). A API recusa as outras contas.
export function addAccessTools(
  server: Server,
  factory: MarkuppApiFactory,
): void {
  addIpBlockTools(server, factory);
  addHistoryTools(server, factory);
}

function addIpBlockTools(server: Server, factory: MarkuppApiFactory): void {
  server.addTool({
    name: "list_ip_blocks",
    description:
      "Instance admins only. Lists the IPs and ranges the instance refuses on every route.",
    parameters: z.object({}),
    annotations: readOnlyAnnotations,
    execute: async (_args, context) => {
      const api = apiFor(factory, context);
      return JSON.stringify(await callApi(() => api.listIpBlocks()));
    },
  });
  server.addTool({
    name: "block_ip",
    description:
      "Instance admins only, when the user asked for it. Blocks an IP or a CIDR range on every route within seconds. A range that includes your own IP is refused.",
    parameters: z.object({
      cidr: z
        .string()
        .min(1)
        .describe("IP or CIDR range, such as 198.51.100.7 or 198.51.100.0/24."),
      note: z
        .string()
        .max(500)
        .optional()
        .describe("Why it is blocked, for other admins."),
    }),
    annotations: writeAnnotations,
    execute: async (creation, context) => {
      const api = apiFor(factory, context);
      return JSON.stringify(await callApi(() => api.blockIp(creation)));
    },
  });
  server.addTool({
    name: "unblock_ip",
    description:
      "Instance admins only, when the user asked for it. Removes an IP block.",
    parameters: z.object({
      id: z.string().min(1).describe("Block id from list_ip_blocks."),
    }),
    annotations: { ...writeAnnotations, idempotentHint: true },
    execute: async ({ id }, context) => {
      const api = apiFor(factory, context);
      await callApi(() => api.unblockIp(id));
      return `IP block ${id} removed.`;
    },
  });
}

function addHistoryTools(server: Server, factory: MarkuppApiFactory): void {
  server.addTool({
    name: "list_audit_events",
    description:
      "Instance admins only. Reads the history of sign-ins, failed sign-ins and sensitive changes, newest first, with IP and user agent. It only goes back as far as the retention period.",
    parameters: z.object({
      account: z
        .string()
        .optional()
        .describe("Account id from list_accounts that did the action."),
      action: z
        .string()
        .optional()
        .describe("Action, such as auth.login or auth.password_reset."),
      result: z.enum(["success", "failure", "blocked"]).optional(),
      ip: z.string().optional(),
      since: z
        .string()
        .datetime({ offset: true })
        .optional()
        .describe("RFC 3339 start, inclusive."),
      until: z
        .string()
        .datetime({ offset: true })
        .optional()
        .describe("RFC 3339 end, exclusive."),
      limit: z.number().int().min(1).max(200).optional(),
      offset: z.number().int().min(0).max(10000).optional(),
    }),
    annotations: readOnlyAnnotations,
    execute: async (query, context) => {
      const api = apiFor(factory, context);
      return JSON.stringify(await callApi(() => api.listAuditEvents(query)));
    },
  });
  server.addTool({
    name: "get_access_history_retention",
    description:
      "Instance admins only. Says for how many days the history and IPs are kept, and whether the hosting configuration locks it.",
    parameters: z.object({}),
    annotations: readOnlyAnnotations,
    execute: async (_args, context) => {
      const api = apiFor(factory, context);
      return JSON.stringify(await callApi(() => api.getRetention()));
    },
  });
  server.addTool({
    name: "set_access_history_retention",
    description:
      "Instance admins only, when the user asked for it. Sets for how many days the history and IPs are kept, from 1 to 3650; older entries are deleted in the next daily cleanup. Refused while the hosting configuration locks it.",
    parameters: z.object({ days: z.number().int().min(1).max(3650) }),
    annotations: { ...writeAnnotations, idempotentHint: true },
    execute: async ({ days }, context) => {
      const api = apiFor(factory, context);
      return JSON.stringify(await callApi(() => api.setRetention(days)));
    },
  });
}
