import { UserError } from "fastmcp";
import { z } from "zod";
import { MarkuppApiError } from "../markupp-api.js";
import type { PublicMarkuppApi } from "../public-api.js";
import type { PublicSession } from "../session.js";
import { stepFrom, userErrorWithStep } from "./tool-support.js";

// PublicApiFactory monta o cliente da leitura pública com a sessão anônima que chamou,
// que pode faltar quando o servidor roda sem authenticate.
export type PublicApiFactory = (
  session: PublicSession | undefined,
) => PublicMarkuppApi;

const VAULT_SLUG_FORMAT = /^[a-z0-9-]{3,63}$/;
// PUBLIC_NOTE_ID_FORMAT cobre o UUID da nota e recusa '.', '..' e '/', que o fetch
// normalizaria para outra rota de /v1/public.
const PUBLIC_NOTE_ID_FORMAT = /^[A-Za-z0-9-]{1,64}$/;

// formatMismatch descreve o argumento fora do formato com o valor e o padrão esperado.
function formatMismatch(name: string, value: unknown, format: RegExp): string {
  return `${name} ${JSON.stringify(value)} does not match the pattern ${format.source}`;
}

function formattedString(name: string, format: RegExp): z.ZodString {
  return z.string().regex(format, {
    error: (issue) => formatMismatch(name, issue.input, format),
  });
}

// vaultSlugParameter segue o formato do slug que a API aceita na publicação.
export const vaultSlugParameter = formattedString(
  "vault",
  VAULT_SLUG_FORMAT,
).describe(
  "Slug of the published vault: the {slug} in its public URL /p/{slug}, 3 to 63 characters of a-z, 0-9 and '-'. Ask the user for it when unknown.",
);

export const publicNoteIdParameter = formattedString(
  "id",
  PUBLIC_NOTE_ID_FORMAT,
).describe("Note id from list_public_notes or search_public_notes.");

// checkPublicNoteArguments valida os argumentos do resource, que não passam pelo
// schema das ferramentas, e recusa com UserError o que estiver fora do formato.
export function checkPublicNoteArguments(vault: string, id: string): void {
  if (!VAULT_SLUG_FORMAT.test(vault))
    throw new UserError(formatMismatch("vault", vault, VAULT_SLUG_FORMAT));
  if (!PUBLIC_NOTE_ID_FORMAT.test(id))
    throw new UserError(formatMismatch("id", id, PUBLIC_NOTE_ID_FORMAT));
}

// publicNoteLocator aponta uma nota publicada: o slug do cofre e o id dela.
export const publicNoteLocator = z.object({
  vault: vaultSlugParameter,
  id: publicNoteIdParameter,
});

// nextStepByPublicCode só aponta ferramentas públicas: o agente de /mcp/public não tem
// as autenticadas. A API responde not_found igual para o slug sem cofre publicado e
// para a nota fora dele (ADR-0048), então o passo cobre os dois.
const nextStepByPublicCode = {
  not_found:
    "Either no vault is published with this slug, so check the slug with the user from the public URL /p/{slug}, or the note is not in it, so find its current id with list_public_notes or search_public_notes.",
  rate_limited:
    "Wait the seconds this message names before the next call, and read fewer notes per minute.",
} as const satisfies Record<string, string>;

// callPublicApi traduz as falhas da API em UserError com o código, a mensagem e o
// próximo passo; o resto, como falha de rede, segue como veio.
export async function callPublicApi<T>(call: () => Promise<T>): Promise<T> {
  try {
    return await call();
  } catch (error) {
    if (!(error instanceof MarkuppApiError)) throw error;
    throw userErrorWithStep(error, stepFrom(nextStepByPublicCode, error.code));
  }
}
