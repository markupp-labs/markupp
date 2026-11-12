import type { FastMCP } from "fastmcp";
import { z } from "zod";
import type { MarkuppSession } from "../session.js";
import {
  apiFor,
  callApi,
  limitParameter,
  noteIdParameter,
  noteLocator,
  offsetParameter,
  readOnlyAnnotations,
  vaultLocator,
  writeAnnotations,
  type MarkuppApiFactory,
} from "./tool-support.js";

const listLinksParameters = noteLocator.extend({
  direction: z
    .enum(["outgoing", "incoming"])
    .default("outgoing")
    .describe(
      "outgoing for the links this note makes, incoming for the links other notes make to it.",
    ),
  type: z
    .enum(["explicit", "embed", "typed", "inferred", "mentioned"])
    .optional()
    .describe("Only links of this type; omit for every type."),
  limit: limitParameter,
  offset: offsetParameter,
});

const refuseLinkParameters = vaultLocator.extend({
  note_ids: z
    .array(noteIdParameter)
    .length(2)
    .describe("The two notes of the vault, in any order."),
});

const refusalLocator = vaultLocator.extend({
  refusal_id: z
    .string()
    .min(1)
    .describe("Refusal id from list_link_refusals or refuse_link."),
});

// addLinkTools registra list_links, refuse_link, list_link_refusals e
// undo_link_refusal. A recusa só afeta a ligação automática, que o servidor cria.
export function addLinkTools(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  addListLinks(server, factory);
  addRefuseLink(server, factory);
  addListLinkRefusals(server, factory);
  addUndoLinkRefusal(server, factory);
}

function addListLinks(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "list_links",
    description:
      "Lists a note's links: the ones it makes (outgoing) or the ones other notes make to it (incoming), each with source, target (null when unresolved), cited_title and type. Types: explicit is a wikilink or markdown link, embed is ![[...]], typed is a frontmatter key whose value is a wikilink (the key is the predicate), inferred is the server's guess for a link that does not resolve, and mentioned is an automatic link the server created because the note cites the other note's title in its text. When the user says a mentioned link is wrong, remove it with refuse_link.",
    parameters: listLinksParameters,
    annotations: readOnlyAnnotations,
    execute: async ({ tenant, id, ...query }, context) => {
      const api = apiFor(factory, context);
      return JSON.stringify(
        await callApi(() => api.listLinks(tenant, id, query)),
      );
    },
  });
}

function addRefuseLink(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "refuse_link",
    description:
      "Use it when the user says two notes should not be linked automatically. The server then never creates an automatic link (type mentioned) between them, in either direction, until undo_link_refusal; written links are not affected. The link leaves list_links at once and the graph measures on the next indexing pass. Refusing the same pair again returns the existing refusal.",
    parameters: refuseLinkParameters,
    annotations: { ...writeAnnotations, idempotentHint: true },
    execute: async ({ tenant, vault_id, note_ids }, context) => {
      const refusal = await callApi(() =>
        apiFor(factory, context).refuseLink(tenant, vault_id, note_ids),
      );
      return JSON.stringify(refusal);
    },
  });
}

function addListLinkRefusals(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "list_link_refusals",
    description:
      "Lists the vault's link refusals, newest first, each with id, the two notes and created_at. Call it for the refusal id that undo_link_refusal takes.",
    parameters: vaultLocator.extend({
      limit: limitParameter,
      offset: offsetParameter,
    }),
    annotations: readOnlyAnnotations,
    execute: async ({ tenant, vault_id, limit, offset }, context) => {
      const api = apiFor(factory, context);
      const refusals = await callApi(() =>
        api.listLinkRefusals(tenant, vault_id, { limit, offset }),
      );
      return JSON.stringify(refusals);
    },
  });
}

function addUndoLinkRefusal(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "undo_link_refusal",
    description:
      "Undoes a link refusal from list_link_refusals, when the user asks for it. The server may link the two notes automatically again on the next indexing pass.",
    parameters: refusalLocator,
    annotations: { ...writeAnnotations, idempotentHint: true },
    execute: async ({ tenant, vault_id, refusal_id }, context) => {
      await callApi(() =>
        apiFor(factory, context).undoLinkRefusal(tenant, vault_id, refusal_id),
      );
      return JSON.stringify({ undone: refusal_id });
    },
  });
}
