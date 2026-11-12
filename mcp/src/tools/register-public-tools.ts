import type { FastMCP } from "fastmcp";
import type { PublicSession } from "../session.js";
import { addPublicNoteResource } from "./public-note-resource.js";
import { addPublicNoteTools } from "./public-note-tools.js";
import type { PublicApiFactory } from "./public-tool-support.js";

export type { PublicApiFactory } from "./public-tool-support.js";

// registerPublicTools registra só a leitura das notas públicas, num servidor à parte
// do autenticado (ADR-0050): as ferramentas dos dois nunca se misturam.
export function registerPublicTools(
  server: FastMCP<PublicSession>,
  factory: PublicApiFactory,
): void {
  addPublicNoteTools(server, factory);
  addPublicNoteResource(server, factory);
}
