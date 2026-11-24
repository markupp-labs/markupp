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

const providerIdParameter = z
  .string()
  .min(1)
  .describe("Login provider id from list_login_providers.");

// providerFields são os campos não secretos, que o PUT substitui por inteiro.
const providerFields = z.object({
  name: z.string().trim().min(1).describe("Name shown on the sign-in button."),
  issuer: z
    .string()
    .max(2048)
    .default("")
    .describe(
      "OpenID Connect issuer URL, such as https://accounts.google.com. Empty only for the id github. It cannot change while accounts are linked.",
    ),
  client_id: z
    .string()
    .min(1)
    .max(2048)
    .describe("OAuth client id registered at the provider."),
  enabled: z
    .boolean()
    .describe("Whether the sign-in button is offered to users."),
});

const clientSecretParameter = z
  .string()
  .min(1)
  .max(2048)
  .describe(
    "OAuth client secret, stored encrypted and never returned; outputs only say whether one is set.",
  );

const providerCreation = providerFields.extend({
  id: z
    .string()
    .min(1)
    .describe(
      "Stable id of the provider, part of its callback URL; it never changes later.",
    ),
  client_secret: clientSecretParameter,
});

const providerChange = providerFields.extend({
  id: providerIdParameter,
  client_secret: clientSecretParameter
    .optional()
    .describe(
      "New OAuth client secret, stored encrypted and never returned. Omit to keep the stored secret.",
    ),
});

// addLoginProviderTools registra as ferramentas dos provedores de login externo, do
// administrador da instância. A API recusa as outras contas.
export function addLoginProviderTools(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  addListLoginProviders(server, factory);
  addCreateLoginProvider(server, factory);
  addUpdateLoginProvider(server, factory);
  addDeleteLoginProvider(server, factory);
}

function addListLoginProviders(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "list_login_providers",
    description:
      "Instance admins only. Lists the external login providers (OpenID Connect) with their callback URL, whether a client secret is set (never the secret), how many accounts are linked, and whether the hosting configuration locks the list or a provider's issuer.",
    parameters: z.object({}),
    annotations: readOnlyAnnotations,
    execute: async (_args, context) => {
      const api = apiFor(factory, context);
      return JSON.stringify(await callApi(() => api.listLoginProviders()));
    },
  });
}

function addCreateLoginProvider(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "create_login_provider",
    description:
      "Instance admins only, when the user asked for it. Adds an external login provider (OpenID Connect); register the returned callback_url at the provider. Refused while the hosting configuration locks the providers.",
    parameters: providerCreation,
    annotations: writeAnnotations,
    execute: async (creation, context) => {
      const api = apiFor(factory, context);
      return JSON.stringify(
        await callApi(() => api.createLoginProvider(creation)),
      );
    },
  });
}

function addUpdateLoginProvider(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "update_login_provider",
    description:
      "Instance admins only, when the user asked for it. Replaces name, issuer, client_id and enabled of a login provider, so send all of them: read the current values with list_login_providers and change only what the user asked. The id never changes. Refused while the hosting configuration locks the providers or the issuer.",
    parameters: providerChange,
    annotations: { ...writeAnnotations, idempotentHint: true },
    execute: async ({ id, ...change }, context) => {
      const api = apiFor(factory, context);
      return JSON.stringify(
        await callApi(() => api.updateLoginProvider(id, change)),
      );
    },
  });
}

function addDeleteLoginProvider(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "delete_login_provider",
    description:
      "Instance admins only, when the user asked for that provider to be deleted. Deletes the login provider and removes the link of every account with it, which cannot be undone; returns how many links were removed. To just hide the sign-in button, use update_login_provider with enabled false.",
    parameters: z.object({ id: providerIdParameter }),
    annotations: {
      ...writeAnnotations,
      destructiveHint: true,
      idempotentHint: true,
    },
    execute: async ({ id }, context) => {
      const api = apiFor(factory, context);
      return JSON.stringify(await callApi(() => api.deleteLoginProvider(id)));
    },
  });
}
