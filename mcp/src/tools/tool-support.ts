import { UserError, type Context } from "fastmcp";
import { z } from "zod";
import { isAgentTokenRefused } from "../auth/token-exchange.js";
import {
  MarkuppApiError,
  type MarkuppApi,
  type NoteSummary,
} from "../markupp-api.js";
import type { MarkuppSession } from "../session.js";

// MarkuppApiFactory monta o cliente da API com o JWT da sessão MCP que chamou.
export type MarkuppApiFactory = (session: MarkuppSession) => MarkuppApi;

export const tenantParameter = z
  .string()
  .min(1)
  .describe("Tenant id from list_tenants.");

// readOnlyAnnotations marca as ferramentas que só leem; writeAnnotations é a base das
// escritas, que cada ferramenta ajusta.
export const readOnlyAnnotations = { readOnlyHint: true, openWorldHint: false };
export const writeAnnotations = {
  readOnlyHint: false,
  destructiveHint: false,
  idempotentHint: false,
  openWorldHint: false,
};

export const noteIdParameter = z
  .string()
  .min(1)
  .describe("Note id from list_notes, search_notes or create_note.");

// noteLocator aponta uma nota: o tenant e o id dela.
export const noteLocator = z.object({
  tenant: tenantParameter,
  id: noteIdParameter,
});

// noteHeader é o que as ferramentas devolvem de cada nota quando o conteúdo não importa:
// na busca o agente abre a nota com read_note, na escrita ele já sabe o que mandou.
export function noteHeader(note: NoteSummary): string {
  const { id, path, updated_at } = note;
  return JSON.stringify({ id, path, updated_at });
}

const reconnect = "Reconnect the server and sign in.";

// nextStepByCode diz ao agente o que fazer diante de cada erro da API que ele resolve
// sozinho ou com o usuário; os outros códigos seguem só com a mensagem da API.
const nextStepByCode = {
  tenant_forbidden: "Call list_tenants and use one of the ids it returns.",
  read_only:
    "Write in a tenant where list_tenants shows role owner, admin or member, or ask the user for that role.",
  not_found: "Find the current note id with search_notes.",
  duplicate_path:
    "A note already uses this path: find its id with list_notes and change it with edit_note.",
  conflict:
    "Another write changed the note after it was read. Call read_note again and retry edit_note against the new content.",
  invalid_path: "Use a non-empty path up to 1024 characters without '..'.",
  invalid_content: "Split the content into smaller notes.",
  insufficient_scope:
    "Ask the user to reconnect the server granting the scope this message names.",
  unauthorized: reconnect,
  forbidden:
    "Use a tenant where list_tenants shows role owner or admin, or ask an owner of this tenant.",
  member_not_found: "Find the current user id with list_members.",
  invite_not_found:
    "Find the current invite id with list_invites for tenant invites or list_my_invites for your own.",
  last_owner:
    "Give another member the owner role with change_member_role first.",
  role_above_own:
    "Act only on members and roles up to your own role in list_tenants, or ask an owner.",
  already_member:
    "They already belong to the tenant: see list_members, or list_tenants for your own account.",
} as const satisfies Record<string, string>;

// apiFor devolve o cliente da API da sessão que chamou a ferramenta ou leu o resource.
export function apiFor(
  factory: MarkuppApiFactory,
  context: Pick<Context<MarkuppSession>, "session">,
): MarkuppApi {
  if (!context.session) {
    throw unauthenticated("this MCP session carries no markupp login");
  }
  return factory(context.session);
}

// callApi traduz as falhas da API e da renovação do JWT em UserError com o código, a
// mensagem e o próximo passo, que é o que o agente lê.
export async function callApi<T>(call: () => Promise<T>): Promise<T> {
  try {
    return await call();
  } catch (error) {
    throw toolFailure(error);
  }
}

// toolFailure deixa passar o que não é da API nem a recusa do token do agente, como
// falha de rede ou erro interno da renovação, que já foi para o log.
function toolFailure(error: unknown): unknown {
  if (error instanceof MarkuppApiError) return toUserError(error);
  if (isAgentTokenRefused(error)) return unauthenticated(error.message);
  return error;
}

function unauthenticated(reason: string): UserError {
  return new UserError(`unauthenticated: ${reason}. ${reconnect}`);
}

export function toUserError(error: MarkuppApiError): UserError {
  const nextStep = nextStepFor(error.code);
  const text = `${error.code}: ${error.message}`;
  return new UserError(nextStep ? `${text}. ${nextStep}` : text, {
    code: error.code,
    status: error.status,
  });
}

// nextStepFor lê só as chaves próprias do mapa, para "toString" não achar um método.
export function nextStepFor(code: string): string | undefined {
  if (!Object.hasOwn(nextStepByCode, code)) return undefined;
  return nextStepByCode[code as keyof typeof nextStepByCode];
}
