import type { FastMCP } from "fastmcp";
import type { MarkuppSession } from "../session.js";
import { addInviteTools } from "./invite-tools.js";
import { addMemberTools } from "./member-tools.js";
import { addNoteReadTools } from "./note-read-tools.js";
import { addNoteResource } from "./note-resource.js";
import { addNoteWriteTools } from "./note-write-tools.js";
import { addTenantTools } from "./tenant-tools.js";
import type { MarkuppApiFactory } from "./tool-support.js";

export type { MarkuppApiFactory } from "./tool-support.js";

// registerMarkuppTools registra as ferramentas de tenants e notas e o template de
// resource das notas. factory monta o cliente da API a cada chamada, com o JWT da sessão
// MCP que chamou.
export function registerMarkuppTools(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  addTenantTools(server, factory);
  addNoteReadTools(server, factory);
  addNoteWriteTools(server, factory);
  addNoteResource(server, factory);
  addMemberTools(server, factory);
  addInviteTools(server, factory);
}
