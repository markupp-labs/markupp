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

// PUBLIC_ID_FORMAT é o id público da nota (ADR-0048): 12 letras e dígitos, o fim do
// endereço /p/{titulo}-{public_id}. Recusa também '.', '..' e '/', que o fetch
// normalizaria para outra rota de /v1/public.
const PUBLIC_ID_FORMAT = /^[A-Za-z0-9]{12}$/;

// formatMismatch descreve o public_id fora do formato com o valor e o padrão esperado.
function formatMismatch(value: unknown): string {
  return `public_id ${JSON.stringify(value)} does not match the pattern ${PUBLIC_ID_FORMAT.source} (12 letters or digits)`;
}

export const publicIdParameter = z
  .string()
  .regex(PUBLIC_ID_FORMAT, { error: (issue) => formatMismatch(issue.input) })
  .describe(
    "Public id of a published note: the 12 letters and digits at the end of its public URL /p/{title}-{public_id}, or a public_id returned by another public tool. Ask the user for it when unknown.",
  );

// checkPublicId valida o argumento do resource, que não passa pelo schema das
// ferramentas, e recusa com UserError o que estiver fora do formato.
export function checkPublicId(publicId: string): void {
  if (!PUBLIC_ID_FORMAT.test(publicId))
    throw new UserError(formatMismatch(publicId));
}

// nextStepByPublicCode só aponta ferramentas públicas: o agente de /mcp/public não tem
// as autenticadas. A API responde not_found igual para id inexistente e para nota
// despublicada (ADR-0048), então o passo cobre os dois.
const nextStepByPublicCode = {
  not_found:
    "No note is published under this public_id: it never existed, was unpublished, or left its published folder. Check the id with the user (the last 12 characters of the public URL), or find the note again with search_public_notes from another note of the same publication.",
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
