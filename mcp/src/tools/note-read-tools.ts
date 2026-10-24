import type { ContentResult, FastMCP } from "fastmcp";
import { z } from "zod";
import type { Note } from "../markupp-api.js";
import type { MarkuppSession } from "../session.js";
import {
  apiFor,
  callApi,
  noteHeader,
  noteLocator,
  readOnlyAnnotations,
  tenantParameter,
  vaultIdParameter,
  type MarkuppApiFactory,
} from "./tool-support.js";

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
      "Lists every note in the tenant, or only in one vault, as id, path and updated_at, sorted by path. Use it to find a note by its path; to find notes by what they say, use search_notes.",
    parameters: z.object({
      tenant: tenantParameter,
      vault_id: vaultIdParameter
        .optional()
        .describe(
          "Vault id from list_vaults to list only that vault; omit for every vault you belong to.",
        ),
    }),
    annotations: readOnlyAnnotations,
    execute: async ({ tenant, vault_id }, context) => {
      const api = apiFor(factory, context);
      const notes = await callApi(() => api.listNotes(tenant, vault_id));
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
      'Words to find in the note content; every word must appear. Takes a "quoted phrase", OR between words and -word to exclude, as in a web search.',
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
  vault_id: vaultIdParameter
    .optional()
    .describe(
      "Vault id from list_vaults to search only that vault; its results come without updated_at, and a vault you do not belong to finds nothing.",
    ),
});

// A busca da API é de texto completo, ordenada por relevância; a ferramenta devolve só
// id, path e updated_at e o agente abre com read_note as que interessarem.
function addSearchNotes(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "search_notes",
    description:
      "Searches note content with full-text search, most relevant first. It matches words, not substrings. Returns id, path and updated_at of each match; open the relevant ones with read_note.",
    parameters: searchNotesParameters,
    annotations: readOnlyAnnotations,
    execute: async ({ tenant, query, limit, offset, vault_id }, context) => {
      const api = apiFor(factory, context);
      const found = await callApi(() =>
        api.searchNotes(tenant, query, { limit, offset }, vault_id),
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
    annotations: readOnlyAnnotations,
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
