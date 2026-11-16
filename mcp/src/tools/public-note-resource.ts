import type { FastMCP } from "fastmcp";
import type { PublicSession } from "../session.js";
import {
  callPublicApi,
  checkPublicId,
  type PublicApiFactory,
} from "./public-tool-support.js";

// PUBLIC_NOTE_URI_TEMPLATE aponta uma nota pública pelo id público.
export const PUBLIC_NOTE_URI_TEMPLATE = "markupp-public://notes/{public_id}";

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
      "Reads a published note's markdown content by its public_id, the 12 letters and digits at the end of its public URL.",
    arguments: [
      {
        name: "public_id",
        required: true,
        description:
          "Public id of the note, 12 letters or digits, from its public URL /p/{title}-{public_id}.",
      },
    ],
    load: async ({ public_id }, session) => {
      checkPublicId(public_id);
      const api = factory(session);
      const note = await callPublicApi(() => api.getNote(public_id));
      return { text: note.content, mimeType: "text/markdown" };
    },
  });
}
