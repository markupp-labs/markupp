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

const myInviteParameters = z.object({
  invite_id: z.string().min(1).describe("Invite id from list_my_invites."),
});

// addMyInviteTools registra list_my_invites, accept_invite e decline_invite, os
// convites endereçados à conta, sem tenant: a conta ainda não está nele.
export function addMyInviteTools(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  addListMyInvites(server, factory);
  addAcceptInvite(server, factory);
  addDeclineInvite(server, factory);
}

function addListMyInvites(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "list_my_invites",
    description:
      "Lists the pending invites to your own account, with tenant name and role. Call it for the invite id that accept_invite and decline_invite take.",
    parameters: z.object({}),
    annotations: readOnlyAnnotations,
    execute: async (_args, context) => {
      const api = apiFor(factory, context);
      return JSON.stringify(await callApi(() => api.listMyInvites()));
    },
  });
}

function addAcceptInvite(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "accept_invite",
    description:
      "Accepts an invite to your account and joins its tenant, after list_my_invites. Returns the tenant id and your role there.",
    parameters: myInviteParameters,
    annotations: { ...writeAnnotations, idempotentHint: true },
    execute: async ({ invite_id }, context) => {
      const api = apiFor(factory, context);
      return JSON.stringify(await callApi(() => api.acceptInvite(invite_id)));
    },
  });
}

function addDeclineInvite(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "decline_invite",
    description:
      "Declines an invite to your account, after list_my_invites, when the user does not want to join that tenant.",
    parameters: myInviteParameters,
    annotations: {
      ...writeAnnotations,
      destructiveHint: true,
      idempotentHint: true,
    },
    execute: async ({ invite_id }, context) => {
      await callApi(() => apiFor(factory, context).declineInvite(invite_id));
      return JSON.stringify({ declined: invite_id });
    },
  });
}
