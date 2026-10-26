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

// vaultSlugParameter segue o formato do slug que a API aceita na publicação.
export const vaultSlugParameter = z
  .string()
  .regex(/^[a-z0-9-]{3,63}$/)
  .describe(
    "Slug of the published vault: the {slug} in its public URL /p/{slug}, 3 to 63 characters of a-z, 0-9 and '-'. Ask the user for it when unknown.",
  );

export const publicNoteIdParameter = z
  .string()
  .min(1)
  .describe("Note id from list_public_notes or search_public_notes.");

// publicNoteLocator aponta uma nota publicada: o slug do cofre e o id dela.
export const publicNoteLocator = z.object({
  vault: vaultSlugParameter,
  id: publicNoteIdParameter,
});

// nextStepByPublicCode só aponta ferramentas públicas: o agente de /mcp/public não tem
// as autenticadas.
const nextStepByPublicCode = {
  not_published:
    "No published vault has this slug: check it with the user, from the public URL /p/{slug}.",
  not_found:
    "The note is not in this vault: find its current id with list_public_notes or search_public_notes.",
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
