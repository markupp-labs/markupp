import type { FastMCP } from "fastmcp";
import type { MarkuppSession } from "../session.js";
import { apiFor, callApi, type MarkuppApiFactory } from "./tool-support.js";

// NOTE_URI_TEMPLATE aponta uma nota pelo id do tenant e pelo id da nota.
export const NOTE_URI_TEMPLATE = "markupp://{tenant}/notes/{id}";

// addNoteResource registra o template das notas, só para leitura direta pela URI: não
// há resources/list, e quem descobre as notas são list_notes e search_notes. O FastMCP
// entrega a sessão do authenticate como segundo argumento do load.
export function addNoteResource(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  server.addResourceTemplate({
    name: "note",
    uriTemplate: NOTE_URI_TEMPLATE,
    mimeType: "text/markdown",
    description:
      "Reads a note's markdown content by tenant id and note id, the ids that list_tenants and list_notes return.",
    arguments: [
      {
        name: "tenant",
        required: true,
        description: "Tenant id from list_tenants.",
      },
      {
        name: "id",
        required: true,
        description: "Note id from list_notes, search_notes or create_note.",
      },
    ],
    load: async ({ tenant, id }, session) => {
      const api = apiFor(factory, { session });
      const note = await callApi(() => api.getNote(tenant, id));
      return { text: note.content, mimeType: "text/markdown" };
    },
  });
}
