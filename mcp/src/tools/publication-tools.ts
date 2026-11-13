import type { FastMCP } from "fastmcp";
import { z } from "zod";
import type { MarkuppSession } from "../session.js";
import {
  apiFor,
  callApi,
  readOnlyAnnotations,
  tenantParameter,
  vaultIdParameter,
  writeAnnotations,
  type MarkuppApiFactory,
} from "./tool-support.js";

type Server = FastMCP<MarkuppSession>;

// addPublicationTools registra publish, unpublish e list_publications (ADR-0048).
export function addPublicationTools(
  server: Server,
  factory: MarkuppApiFactory,
): void {
  addPublish(server, factory);
  addUnpublish(server, factory);
  addListPublications(server, factory);
}

// publicationTarget é o alvo como a API recebe: nota pelo id, pasta pelo caminho ou o
// cofre inteiro.
const publicationTarget = z
  .discriminatedUnion("kind", [
    z.object({
      kind: z.literal("note"),
      note_id: z.string().min(1).describe("Note id from list_notes."),
    }),
    z.object({
      kind: z.literal("folder"),
      path: z
        .string()
        .min(1)
        .describe(
          "Folder path without leading or trailing slash, e.g. docs/deploy.",
        ),
    }),
    z.object({ kind: z.literal("vault") }),
  ])
  .describe(
    "What to publish: one note, a folder with every note in it now and later, or the whole vault.",
  );

const publishParameters = z.object({
  tenant: tenantParameter,
  vault_id: vaultIdParameter,
  target: publicationTarget,
  confirm: z
    .string()
    .min(1)
    .describe(
      "The target's name as the user typed or explicitly confirmed it: the note title, the folder path or the vault name. Never fill it on your own.",
    ),
  indexable: z
    .boolean()
    .default(false)
    .describe("Let search engines index the public pages. Default false."),
});

function addPublish(server: Server, factory: MarkuppApiFactory): void {
  server.addTool({
    name: "publish",
    description:
      "Publishes a note, a folder or a whole vault for anyone to read without an account, people and agents alike. Before calling, tell the user what becomes public and ask them to confirm by typing the target's name. Returns the publication; a note's public URL is /p/{title_slug}-{public_id}.",
    parameters: publishParameters,
    annotations: { ...writeAnnotations, openWorldHint: true },
    execute: async ({ tenant, vault_id, ...request }, context) => {
      const api = apiFor(factory, context);
      const published = await callApi(() =>
        api.publish(tenant, vault_id, request),
      );
      return JSON.stringify(published);
    },
  });
}

function addUnpublish(server: Server, factory: MarkuppApiFactory): void {
  server.addTool({
    name: "unpublish",
    description:
      "Unpublishes a publication: its notes become private and their public ids stop working, except notes another publication still covers. Publishing again creates new public ids.",
    parameters: z.object({
      tenant: tenantParameter,
      publication_id: z
        .string()
        .min(1)
        .describe("Publication id from list_publications."),
    }),
    annotations: {
      ...writeAnnotations,
      destructiveHint: true,
      idempotentHint: true,
    },
    execute: async ({ tenant, publication_id }, context) => {
      const api = apiFor(factory, context);
      await callApi(() => api.unpublish(tenant, publication_id));
      return JSON.stringify({ unpublished: publication_id });
    },
  });
}

function addListPublications(server: Server, factory: MarkuppApiFactory): void {
  server.addTool({
    name: "list_publications",
    description:
      "Lists what is public in the tenant (owners and admins see every publication, others the ones they can manage): target, vault, note count, indexing, who published and when.",
    parameters: z.object({
      tenant: tenantParameter,
      vault_id: vaultIdParameter
        .optional()
        .describe(
          "Vault id from list_vaults to list only that vault; omit for the whole tenant.",
        ),
    }),
    annotations: readOnlyAnnotations,
    execute: async ({ tenant, vault_id }, context) => {
      const api = apiFor(factory, context);
      return JSON.stringify(
        await callApi(() => api.listPublications(tenant, vault_id)),
      );
    },
  });
}
