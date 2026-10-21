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

// addTenantTools registra list_tenants, por onde o agente descobre o id de tenant que
// as outras ferramentas pedem, e create_tenant.
export function addTenantTools(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  addListTenants(server, factory);
  addCreateTenant(server, factory);
}

function addListTenants(
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

// O servidor apara o nome e limita a 100 caracteres; o schema já recusa o que passa disso.
const createTenantParameters = z.object({
  name: z
    .string()
    .trim()
    .min(1)
    .max(100)
    .describe("Tenant name, up to 100 characters."),
});

function addCreateTenant(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "create_tenant",
    description:
      "Creates a new tenant with you as owner and returns its id, ready for every tenant tool.",
    parameters: createTenantParameters,
    annotations: writeAnnotations,
    execute: async ({ name }, context) => {
      const api = apiFor(factory, context);
      return JSON.stringify(await callApi(() => api.createTenant(name)));
    },
  });
}
