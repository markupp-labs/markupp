import type { FastMCP } from "fastmcp";
import { z } from "zod";
import type { MarkuppSession } from "../session.js";
import {
  adminTenantParameter,
  apiFor,
  callApi,
  readOnlyAnnotations,
  roleParameter,
  writeAnnotations,
  type MarkuppApiFactory,
} from "./tool-support.js";

const userIdParameter = z
  .string()
  .min(1)
  .describe("Member user id from list_members.");

const memberLocator = z.object({
  tenant: adminTenantParameter,
  user_id: userIdParameter,
});

// addMemberTools registra list_members, change_member_role e remove_member, as rotas
// /v1/users que pedem papel owner ou admin no tenant.
export function addMemberTools(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  addListMembers(server, factory);
  addChangeMemberRole(server, factory);
  addRemoveMember(server, factory);
}

function addListMembers(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "list_members",
    description:
      "Lists the tenant's members with user id, email, role and joined_at. Call it for the user id that change_member_role and remove_member take.",
    parameters: z.object({ tenant: adminTenantParameter }),
    annotations: readOnlyAnnotations,
    execute: async ({ tenant }, context) => {
      const api = apiFor(factory, context);
      return JSON.stringify(await callApi(() => api.listMembers(tenant)));
    },
  });
}

function addChangeMemberRole(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "change_member_role",
    description:
      "Changes a member's role in the tenant, after list_members. The tenant always keeps at least one owner.",
    parameters: memberLocator.extend({ role: roleParameter }),
    annotations: { ...writeAnnotations, idempotentHint: true },
    execute: async ({ tenant, user_id, role }, context) => {
      const api = apiFor(factory, context);
      await callApi(() => api.changeMemberRole(tenant, user_id, role));
      return JSON.stringify({ user_id, role });
    },
  });
}

function addRemoveMember(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "remove_member",
    description:
      "Removes a member from the tenant, after list_members, when the user asked for that member to leave.",
    parameters: memberLocator,
    annotations: {
      ...writeAnnotations,
      destructiveHint: true,
      idempotentHint: true,
    },
    execute: async ({ tenant, user_id }, context) => {
      await callApi(() =>
        apiFor(factory, context).removeMember(tenant, user_id),
      );
      return JSON.stringify({ removed: user_id });
    },
  });
}
