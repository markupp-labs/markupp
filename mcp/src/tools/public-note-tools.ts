import type { ContentResult, FastMCP } from "fastmcp";
import { z } from "zod";
import type { PublicNote, PublicSearchHit } from "../public-api.js";
import type { PublicSession } from "../session.js";
import {
  callPublicApi,
  publicNoteLocator,
  vaultSlugParameter,
  type PublicApiFactory,
} from "./public-tool-support.js";
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

// addPublicNoteTools registra list_public_notes, search_public_notes e
// read_public_note, que leem os cofres publicados sem login.
export function addPublicNoteTools(
  server: PublicServer,
  factory: PublicApiFactory,
): void {
  addListPublicNotes(server, factory);
  addSearchPublicNotes(server, factory);
  addReadPublicNote(server, factory);
}

const listPublicNotesParameters = z.object({
  vault: vaultSlugParameter,
  sort: z
    .enum(["path", "pagerank", "updated"])
    .optional()
    .describe(
      "path (default, ascending), pagerank (most central notes in the link graph first) or updated (newest first).",
    ),
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
      "Lists a page of the notes in a published markupp vault as id, title, path and updated_at. Use sort pagerank to start from the most central notes; to find notes by what they say, use search_public_notes.",
    parameters: listPublicNotesParameters,
    annotations: publicReadAnnotations,
    execute: async ({ vault, sort, limit, offset }, context) => {
      const api = factory(context.session);
      const page = { sort, limit, offset };
      const notes = await callPublicApi(() => api.listNotes(vault, page));
      return JSON.stringify(notes);
    },
  });
}

const searchPublicNotesParameters = z.object({
  vault: vaultSlugParameter,
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
      "Searches the notes of a published markupp vault, best match first. Returns id, title, path and a snippet of each match; open the relevant ones with read_public_note.",
    parameters: searchPublicNotesParameters,
    annotations: publicReadAnnotations,
    execute: async ({ vault, query, limit, offset }, context) => {
      const api = factory(context.session);
      const hits = await callPublicApi(() =>
        api.searchNotes(vault, query, { limit, offset }),
      );
      return JSON.stringify(hits.map(searchHitView));
    },
  });
}

function searchHitView(hit: PublicSearchHit): Record<string, string> {
  return { ...hit.note, snippet: hit.snippet };
}

function addReadPublicNote(
  server: PublicServer,
  factory: PublicApiFactory,
): void {
  server.addTool({
    name: "read_public_note",
    description:
      "Reads a published note's full markdown content and metadata by vault slug and note id. Wikilinks like [[Title]] point to other notes of the same vault: find them with search_public_notes.",
    parameters: publicNoteLocator,
    annotations: publicReadAnnotations,
    execute: async ({ vault, id }, context) => {
      const api = factory(context.session);
      const note = await callPublicApi(() => api.getNote(vault, id));
      return publicNoteContent(note);
    },
  });
}

// publicNoteContent separa os metadados do markdown cru, como read_note.
function publicNoteContent(note: PublicNote): ContentResult {
  const { id, title, path, created_at, updated_at } = note;
  const metadata = { id, title, path, created_at, updated_at };
  return {
    content: [
      { type: "text", text: JSON.stringify(metadata) },
      { type: "text", text: note.content },
    ],
  };
}
