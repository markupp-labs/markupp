import type { FastMCP } from "fastmcp";
import { z } from "zod";
import type { MarkuppApi } from "../markupp-api.js";
import type { MarkuppSession } from "../session.js";
import { replaceExactlyOnce } from "./text-replacement.js";
import {
  apiFor,
  callApi,
  noteHeader,
  noteIdParameter,
  noteLocator,
  tenantParameter,
  type MarkuppApiFactory,
} from "./tool-support.js";

// writeAnnotations é a base das escritas; create_note só acrescenta, as outras mudam ou
// apagam o que existe.
const writeAnnotations = {
  readOnlyHint: false,
  destructiveHint: false,
  idempotentHint: false,
  openWorldHint: false,
};
const destructiveWrite = { ...writeAnnotations, destructiveHint: true };

const createNoteParameters = z.object({
  tenant: tenantParameter,
  path: z
    .string()
    .min(1)
    .describe("Path unique within the vault, such as projects/markupp.md."),
  content: z.string().describe("Full markdown content."),
});

const editNoteParameters = z.object({
  tenant: tenantParameter,
  id: noteIdParameter,
  old_text: z
    .string()
    .min(1)
    .describe(
      "Passage copied verbatim from the current note, long enough to occur once.",
    ),
  new_text: z
    .string()
    .describe("Replacement for old_text; an empty string deletes the passage."),
});

// addNoteWriteTools registra create_note, edit_note e delete_note.
export function addNoteWriteTools(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  addCreateNote(server, factory);
  addEditNote(server, factory);
  addDeleteNote(server, factory);
}

function addCreateNote(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "create_note",
    description:
      "Creates a new note in the tenant's default vault and returns its id. To change a note that already exists, use edit_note.",
    parameters: createNoteParameters,
    annotations: writeAnnotations,
    execute: async ({ tenant, path, content }, context) => {
      const api = apiFor(factory, context);
      return noteHeader(
        await callApi(() => api.createNote(tenant, path, content)),
      );
    },
  });
}

function addEditNote(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "edit_note",
    description:
      "Edits a note by replacing one exact passage: old_text must occur exactly once and becomes new_text. Use it for every change to an existing note, after read_note.",
    parameters: editNoteParameters,
    annotations: destructiveWrite,
    execute: async ({ tenant, id, old_text, new_text }, context) =>
      editNoteContent(apiFor(factory, context), tenant, id, old_text, new_text),
  });
}

// editNoteContent relê a nota, troca o trecho único e manda o PUT com o updated_at
// lido, para a API recusar com conflict se outra escrita entrou no meio.
export async function editNoteContent(
  api: MarkuppApi,
  tenant: string,
  id: string,
  oldText: string,
  newText: string,
): Promise<string> {
  const note = await callApi(() => api.getNote(tenant, id));
  const content = replaceExactlyOnce(note.content, oldText, newText);
  const revision = {
    path: note.path,
    content,
    lastModifiedAt: note.updated_at,
  };
  return noteHeader(await callApi(() => api.updateNote(tenant, id, revision)));
}

function addDeleteNote(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addTool({
    name: "delete_note",
    description:
      "Deletes a note permanently by id, when the user asked for that note to be removed.",
    parameters: noteLocator,
    annotations: { ...destructiveWrite, idempotentHint: true },
    execute: async ({ tenant, id }, context) => {
      await callApi(() => apiFor(factory, context).deleteNote(tenant, id));
      return JSON.stringify({ deleted: id });
    },
  });
}
