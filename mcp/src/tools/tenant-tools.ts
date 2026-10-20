import type { FastMCP } from "fastmcp";
import { z } from "zod";
import type { MarkuppSession } from "../session.js";
import {
  apiFor,
  callApi,
  readOnlyAnnotations,
  type MarkuppApiFactory,
} from "./tool-support.js";

// addTenantTools registra list_tenants, por onde o agente descobre o id de tenant que
// as ferramentas de notas pedem.
export function addTenantTools(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "list_tenants",
    description:
      "Lists the tenants you can access, with your role in each. Call it first: every note tool takes a tenant id from here. The viewer role reads notes only.",
    parameters: z.object({}),
    annotations: readOnlyAnnotations,
    execute: async (_args, context) => {
      const tenants = await callApi(() =>
        apiFor(factory, context).listTenants(),
      );
      return JSON.stringify(tenants);
    },
  });
}
