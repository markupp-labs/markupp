import type { FastMCP } from "fastmcp";
import type { MarkuppSession } from "../session.js";
import { addAccessTools } from "./access-tools.js";
import { addAccountAdminTools } from "./account-admin-tools.js";
import { addAccountTools } from "./account-tools.js";
import { addInstanceTools } from "./instance-tools.js";
import { addInviteTools } from "./invite-tools.js";
import { addLinkTools } from "./link-tools.js";
import { addLoginProviderTools } from "./login-provider-tools.js";
import { addMemberTools } from "./member-tools.js";
import { addMyInviteTools } from "./my-invite-tools.js";
import { addNoteReadTools } from "./note-read-tools.js";
import { addNoteResource } from "./note-resource.js";
import { addNoteWriteTools } from "./note-write-tools.js";
import { addPublicationTools } from "./publication-tools.js";
import { addSessionTools } from "./session-tools.js";
import { addTenantTools } from "./tenant-tools.js";
import type { MarkuppApiFactory } from "./tool-support.js";
import { addVaultMemberTools } from "./vault-member-tools.js";
import { addVaultTools } from "./vault-tools.js";

export type { MarkuppApiFactory } from "./tool-support.js";

// registerMarkuppTools registra as ferramentas da conta, de tenants, cofres, notas,
// ligações, membros, convites, publicações, sessões, da instância, das contas e acessos da
// instância e dos provedores de login externo e o template de resource das notas.
// factory monta o cliente da API a cada chamada, com o JWT da sessão MCP que chamou.
export function registerMarkuppTools(
  server: FastMCP<MarkuppSession>,
  factory: MarkuppApiFactory,
): void {
  addAccountTools(server, factory);
  addTenantTools(server, factory);
  addVaultTools(server, factory);
  addVaultMemberTools(server, factory);
  addNoteReadTools(server, factory);
  addNoteWriteTools(server, factory);
  addNoteResource(server, factory);
  addPublicationTools(server, factory);
  addLinkTools(server, factory);
  addMemberTools(server, factory);
  addInviteTools(server, factory);
  addMyInviteTools(server, factory);
  addInstanceTools(server, factory);
  addLoginProviderTools(server, factory);
  addSessionTools(server, factory);
  addAccountAdminTools(server, factory);
  addAccessTools(server, factory);
}
