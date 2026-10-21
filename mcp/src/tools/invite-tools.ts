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

const inviteMemberParameters = z.object({
  tenant: adminTenantParameter,
  email: z.string().min(1).describe("Email of the person to invite."),
  role: roleParameter,
});

const revokeInviteParameters = z.object({
  tenant: adminTenantParameter,
  invite_id: z.string().min(1).describe("Invite id from list_invites."),
});

// addInviteTools registra list_invites, invite_member e revoke_invite, os convites do
// tenant, que pedem papel owner ou admin nele.
export function addInviteTools(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  addListInvites(server, factory);
  addInviteMember(server, factory);
  addRevokeInvite(server, factory);
}

function addListInvites(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "list_invites",
    description:
      "Lists the tenant's invites with id, email, role and expires_at; accepted ones carry accepted_at. Call it for the invite id that revoke_invite takes.",
    parameters: z.object({ tenant: adminTenantParameter }),
    annotations: readOnlyAnnotations,
    execute: async ({ tenant }, context) => {
      const api = apiFor(factory, context);
      return JSON.stringify(await callApi(() => api.listInvites(tenant)));
    },
  });
}

// O token só volta quando o servidor não manda o convite por email; aí quem convida
// repassa o token à mão.
function addInviteMember(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "invite_member",
    description:
      "Invites an email into the tenant with a role. When the result carries a token, give it to the user to hand to the invitee; without one, the invite went by email.",
    parameters: inviteMemberParameters,
    annotations: writeAnnotations,
    execute: async ({ tenant, email, role }, context) => {
      const api = apiFor(factory, context);
      const issued = await callApi(() => api.inviteMember(tenant, email, role));
      return JSON.stringify(issued);
    },
  });
}

function addRevokeInvite(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "revoke_invite",
    description:
      "Revokes a pending tenant invite so it can no longer be accepted, after list_invites.",
    parameters: revokeInviteParameters,
    annotations: {
      ...writeAnnotations,
      destructiveHint: true,
      idempotentHint: true,
    },
    execute: async ({ tenant, invite_id }, context) => {
      const api = apiFor(factory, context);
      await callApi(() => api.revokeInvite(tenant, invite_id));
      return JSON.stringify({ revoked: invite_id });
    },
  });
}
