import type { FastMCP } from "fastmcp";
import type { MarkuppSession } from "../session.js";
import { addNoteReadTools } from "./note-read-tools.js";
import { addNoteWriteTools } from "./note-write-tools.js";
import { addTenantTools } from "./tenant-tools.js";
import type { MarkuppApiFactory } from "./tool-support.js";

export type { MarkuppApiFactory } from "./tool-support.js";

// registerMarkuppTools registra as ferramentas de tenants e notas. factory monta o
// cliente da API a cada chamada, com o JWT da sessão MCP que chamou.
export function registerMarkuppTools(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  addTenantTools(server, factory);
  addNoteReadTools(server, factory);
  addNoteWriteTools(server, factory);
}
