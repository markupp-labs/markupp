import type { FastMCP } from "fastmcp";
import { z } from "zod";
import type { MarkuppSession } from "../session.js";
import {
  apiFor,
  callApi,
  nameParameter,
  readOnlyAnnotations,
  tenantParameter,
  vaultLocator,
  writeAnnotations,
  type MarkuppApiFactory,
} from "./tool-support.js";

const vaultName = nameParameter.describe("Vault name, up to 100 characters.");

// addVaultTools registra list_vaults, create_vault, rename_vault e delete_vault. A API
// só alcança os cofres de que a conta é membro, e o viewer não escreve neles.
export function addVaultTools(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  addListVaults(server, factory);
  addCreateVault(server, factory);
  addRenameVault(server, factory);
  addDeleteVault(server, factory);
}

function addListVaults(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "list_vaults",
    description:
      "Lists the vaults you belong to in the tenant, with id, name and created_at. Call it for the vault_id that the vault tools and the vault filter of the note tools take.",
    parameters: z.object({ tenant: tenantParameter }),
    annotations: readOnlyAnnotations,
    execute: async ({ tenant }, context) => {
      const api = apiFor(factory, context);
      return JSON.stringify(await callApi(() => api.listVaults(tenant)));
    },
  });
}

function addCreateVault(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "create_vault",
    description:
      "Creates a vault in the tenant with you as its only member and returns its id. Share it with add_vault_member.",
    parameters: z.object({ tenant: tenantParameter, name: vaultName }),
    annotations: writeAnnotations,
    execute: async ({ tenant, name }, context) => {
      const api = apiFor(factory, context);
      return JSON.stringify(await callApi(() => api.createVault(tenant, name)));
    },
  });
}

function addRenameVault(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "rename_vault",
    description:
      "Renames a vault you belong to and returns it. Names need not be unique.",
    parameters: vaultLocator.extend({ name: vaultName }),
    annotations: { ...writeAnnotations, idempotentHint: true },
    execute: async ({ tenant, vault_id, name }, context) => {
      const api = apiFor(factory, context);
      const renamed = await callApi(() =>
        api.renameVault(tenant, vault_id, name),
      );
      return JSON.stringify(renamed);
    },
  });
}

function addDeleteVault(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "delete_vault",
    description:
      "Deletes a vault permanently together with all of its notes, for every member. Use it only when the user asked for that vault to be deleted; to just leave it, use remove_vault_member with your own user id.",
    parameters: vaultLocator,
    annotations: {
      ...writeAnnotations,
      destructiveHint: true,
      idempotentHint: true,
    },
    execute: async ({ tenant, vault_id }, context) => {
      await callApi(() =>
        apiFor(factory, context).deleteVault(tenant, vault_id),
      );
      return JSON.stringify({ deleted: vault_id });
    },
  });
}
