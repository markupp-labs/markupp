import type { FastMCP } from "fastmcp";
import { z } from "zod";
import type { MarkuppSession } from "../session.js";
import {
  apiFor,
  callApi,
  readOnlyAnnotations,
  type MarkuppApiFactory,
} from "./tool-support.js";

// addAccountTools registra whoami, por onde o agente sabe o id da própria conta, que
// remove_vault_member pede para sair de um cofre.
export function addAccountTools(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "whoami",
    description:
      "Returns the user id and email of the account you act as. Use the id to find yourself in list_members or list_vault_members, or to leave a vault with remove_vault_member.",
    parameters: z.object({}),
    annotations: readOnlyAnnotations,
    execute: async (_args, context) => {
      const account = await callApi(() =>
        apiFor(factory, context).getAccount(),
      );
      return JSON.stringify(account);
    },
  });
}
