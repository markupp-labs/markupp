import type { ContentResult, FastMCP } from "fastmcp";
import { z } from "zod";
import type { PublicNote, PublicSearchHit } from "../public-api.js";
import type { PublicSession } from "../session.js";
import {
  callPublicApi,
  publicIdParameter,
  type PublicApiFactory,
} from "./public-tool-support.js";
import { markdownHighlight } from "./snippet-highlight.js";
import { readOnlyAnnotations } from "./tool-support.js";

type PublicServer = FastMCP<PublicSession>;

// A leitura pública lê o que qualquer pessoa lê na web, por isso o openWorldHint.
const publicReadAnnotations = { ...readOnlyAnnotations, openWorldHint: true };

const pageLimit = z
  .number()
  .int()
  .min(1)
  .max(100)
  .optional()
  .describe("Max results. Default 10.");

const pageOffset = z
  .number()
  .int()
  .min(0)
  .max(10000)
  .optional()
  .describe("Results to skip, to page past the first ones.");

// addPublicNoteTools registra read_public_note, list_public_notes e
// search_public_notes, que leem as notas públicas sem login pelo id público.
export function addPublicNoteTools(
  server: PublicServer,
  factory: PublicApiFactory,
): void {
  addListPublicNotes(server, factory);
  addSearchPublicNotes(server, factory);
  addReadPublicNote(server, factory);
}

const listPublicNotesParameters = z.object({
  public_id: publicIdParameter,
  limit: pageLimit,
  offset: pageOffset,
});

function addListPublicNotes(
  server: PublicServer,
  factory: PublicApiFactory,
): void {
  server.addTool({
    name: "list_public_notes",
    description:
      "Lists a page of the notes published together with the given note (the note alone, its published folder or its vault), as public_id, title, title_slug and path relative to the published folder, plus the total. To find notes by what they say, use search_public_notes.",
    parameters: listPublicNotesParameters,
    annotations: publicReadAnnotations,
    execute: async ({ public_id, limit, offset }, context) => {
      const api = factory(context.session);
      const page = { limit, offset };
      const tree = await callPublicApi(() => api.listTree(public_id, page));
      return JSON.stringify(tree);
    },
  });
}

const searchPublicNotesParameters = z.object({
  public_id: publicIdParameter,
  query: z
    .string()
    .min(1)
    .max(256)
    .describe("Words to search in the notes, full-text, best match first."),
  limit: pageLimit,
  offset: pageOffset,
});

function addSearchPublicNotes(
  server: PublicServer,
  factory: PublicApiFactory,
): void {
  server.addTool({
    name: "search_public_notes",
    description:
      "Searches the notes published together with the given note, best match first. Returns public_id, title, title_slug, rank and a snippet with the matches in **bold**, plus the total; open the relevant ones with read_public_note.",
    parameters: searchPublicNotesParameters,
    annotations: publicReadAnnotations,
    execute: async ({ public_id, query, limit, offset }, context) => {
      const api = factory(context.session);
      const found = await callPublicApi(() =>
        api.searchNotes(public_id, query, { limit, offset }),
      );
      const items = found.items.map(searchHitView);
      return JSON.stringify({ items, total: found.total });
    },
  });
}

function searchHitView(hit: PublicSearchHit): PublicSearchHit {
  return { ...hit, snippet: markdownHighlight(hit.snippet) };
}

function addReadPublicNote(
  server: PublicServer,
  factory: PublicApiFactory,
): void {
  server.addTool({
    name: "read_public_note",
    description:
      "Reads a published note by public_id: first its metadata (title, updated_at, the publication it belongs to and its links, where public_id is null for targets that are not public), then the raw markdown. Follow a link with read_public_note on its public_id.",
    parameters: z.object({ public_id: publicIdParameter }),
    annotations: publicReadAnnotations,
    execute: async ({ public_id }, context) => {
      const api = factory(context.session);
      const note = await callPublicApi(() => api.getNote(public_id));
      return publicNoteContent(note);
    },
  });
}

// publicNoteContent separa os metadados do markdown cru, como read_note.
function publicNoteContent(note: PublicNote): ContentResult {
  const { content, ...metadata } = note;
  return {
    content: [
      { type: "text", text: JSON.stringify(metadata) },
      { type: "text", text: content },
    ],
  };
}
