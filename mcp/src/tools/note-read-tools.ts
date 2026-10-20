import type { ContentResult, FastMCP } from "fastmcp";
import { z } from "zod";
import type { Note } from "../markupp-api.js";
import type { MarkuppSession } from "../session.js";
import {
  apiFor,
  callApi,
  noteHeader,
  noteLocator,
  tenantParameter,
  type MarkuppApiFactory,
} from "./tool-support.js";

const readOnly = { readOnlyHint: true, openWorldHint: false };

// addNoteReadTools registra list_notes, search_notes e read_note, os passos antes de
// editar.
export function addNoteReadTools(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  addListNotes(server, factory);
  addSearchNotes(server, factory);
  addReadNote(server, factory);
}

function addListNotes(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "list_notes",
    description:
      "Lists every note in the tenant as id, path and updated_at, sorted by path. Use it to find a note by its path; to find notes by what they say, use search_notes.",
    parameters: z.object({ tenant: tenantParameter }),
    annotations: readOnly,
    execute: async ({ tenant }, context) => {
      const api = apiFor(factory, context);
      const notes = await callApi(() => api.listNotes(tenant));
      notes.sort((a, b) => a.path.localeCompare(b.path));
      return `[${notes.map(noteHeader).join(",")}]`;
    },
  });
}

const searchNotesParameters = z.object({
  tenant: tenantParameter,
  query: z
    .string()
    .min(1)
    .describe(
      "Text the note content contains, case-insensitive. Search one word or short phrase per call.",
    ),
  limit: z
    .number()
    .int()
    .min(1)
    .max(100)
    .optional()
    .describe("Max results. Default 10."),
  offset: z
    .number()
    .int()
    .min(0)
    .max(10000)
    .optional()
    .describe("Results to skip, to page past the first ones."),
});

// A busca da API casa substring do conteúdo, sem trecho; por isso a ferramenta devolve
// só id, path e updated_at e o agente abre com read_note as que interessarem.
function addSearchNotes(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "search_notes",
    description:
      "Searches note content for a text, case-insensitive, newest first. Returns id, path and updated_at of each match; open the relevant ones with read_note.",
    parameters: searchNotesParameters,
    annotations: readOnly,
    execute: async ({ tenant, query, limit, offset }, context) => {
      const api = apiFor(factory, context);
      const found = await callApi(() =>
        api.searchNotes(tenant, query, { limit, offset }),
      );
      return `[${found.map(noteHeader).join(",")}]`;
    },
  });
}

function addReadNote(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "read_note",
    description:
      "Reads a note's full markdown content and metadata by id. Read the note before edit_note, so old_text is copied from its current content.",
    parameters: noteLocator,
    annotations: readOnly,
    execute: async ({ tenant, id }, context) => {
      const note = await callApi(() =>
        apiFor(factory, context).getNote(tenant, id),
      );
      return noteContent(note);
    },
  });
}

// noteContent separa os metadados do conteúdo cru, para o agente copiar old_text do
// markdown sem desfazer o escape de uma string JSON.
export function noteContent(note: Note): ContentResult {
  const { id, path, created_at, updated_at } = note;
  return {
    content: [
      {
        type: "text",
        text: JSON.stringify({ id, path, created_at, updated_at }),
      },
      { type: "text", text: note.content },
    ],
  };
}
