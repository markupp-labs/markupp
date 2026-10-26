import type { FastMCP } from "fastmcp";
import type { PublicSession } from "../session.js";
import { callPublicApi, type PublicApiFactory } from "./public-tool-support.js";

// PUBLIC_NOTE_URI_TEMPLATE aponta uma nota publicada pelo slug do cofre e pelo id.
export const PUBLIC_NOTE_URI_TEMPLATE = "markupp-public://{vault}/notes/{id}";

// addPublicNoteResource registra o template das notas publicadas, só para leitura
// direta pela URI, como o das notas da conta.
export function addPublicNoteResource(
  server: FastMCP<PublicSession>,
  factory: PublicApiFactory,
): void {
  server.addResourceTemplate({
    name: "public_note",
    uriTemplate: PUBLIC_NOTE_URI_TEMPLATE,
    mimeType: "text/markdown",
    description:
      "Reads a published note's markdown content by vault slug and note id, the ids that list_public_notes returns.",
    arguments: [
      {
        name: "vault",
        required: true,
        description: "Slug of the published vault, from its URL /p/{slug}.",
      },
      {
        name: "id",
        required: true,
        description: "Note id from list_public_notes or search_public_notes.",
      },
    ],
    load: async ({ vault, id }, session) => {
      const api = factory(session);
      const note = await callPublicApi(() => api.getNote(vault, id));
      return { text: note.content, mimeType: "text/markdown" };
    },
  });
}
