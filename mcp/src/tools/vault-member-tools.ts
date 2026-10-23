import type { FastMCP } from "fastmcp";
import { z } from "zod";
import type { MarkuppSession } from "../session.js";
import {
  apiFor,
  callApi,
  readOnlyAnnotations,
  vaultLocator,
  writeAnnotations,
  type MarkuppApiFactory,
} from "./tool-support.js";

const vaultMemberLocator = vaultLocator.extend({
  user_id: z
    .string()
    .min(1)
    .describe("Account user id from list_members or list_vault_members."),
});

// addVaultMemberTools registra list_vault_members, add_vault_member e
// remove_vault_member. Entra no cofre só conta que já é do tenant.
export function addVaultMemberTools(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  addListVaultMembers(server, factory);
  addAddVaultMember(server, factory);
  addRemoveVaultMember(server, factory);
}

function addListVaultMembers(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "list_vault_members",
    description:
      "Lists the members of a vault you belong to, with user id, email, tenant role and joined_at.",
    parameters: vaultLocator,
    annotations: readOnlyAnnotations,
    execute: async ({ tenant, vault_id }, context) => {
      const api = apiFor(factory, context);
      const members = await callApi(() =>
        api.listVaultMembers(tenant, vault_id),
      );
      return JSON.stringify(members);
    },
  });
}

function addAddVaultMember(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "add_vault_member",
    description:
      "Gives an account of the tenant access to a vault you belong to. It only takes accounts already in the tenant: find their user id by email with list_members, and bring someone from outside in with invite_member first.",
    parameters: vaultMemberLocator,
    annotations: { ...writeAnnotations, idempotentHint: true },
    execute: async ({ tenant, vault_id, user_id }, context) => {
      await callApi(() =>
        apiFor(factory, context).addVaultMember(tenant, vault_id, user_id),
      );
      return JSON.stringify({ vault_id, user_id });
    },
  });
}

function addRemoveVaultMember(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "remove_vault_member",
    description:
      "Removes an account from a vault, after list_vault_members, when the user asked for it. Your own user id leaves the vault, which even the viewer role may do. The vault and its notes stay for the other members.",
    parameters: vaultMemberLocator,
    annotations: {
      ...writeAnnotations,
      destructiveHint: true,
      idempotentHint: true,
    },
    execute: async ({ tenant, vault_id, user_id }, context) => {
      await callApi(() =>
        apiFor(factory, context).removeVaultMember(tenant, vault_id, user_id),
      );
      return JSON.stringify({ removed: user_id });
    },
  });
}
