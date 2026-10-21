import { z } from "zod";

// BODY_EXCERPT_CHARS limita o trecho do corpo citado nas mensagens de erro; as
// respostas de erro do issuer nunca repetem credencial nem token.
const BODY_EXCERPT_CHARS = 200;

// ReadBody é o corpo lido uma vez: o trecho do texto cru, para citar no erro, e o JSON
// (undefined quando o texto não é JSON válido).
export type ReadBody = { excerpt: string; json: unknown };

// OAuthErrorFields é o erro do protocolo (RFC 6749, seção 5.2) lido do corpo.
export type OAuthErrorFields = { code: string; description: string };

const oauthErrorBody = z.object({
  error: z.string().min(1),
  error_description: z.string().optional(),
});

// readBody lê o corpo como texto e tenta o JSON sem lançar: proxy na frente do issuer
// pode devolver HTML.
export async function readBody(response: Response): Promise<ReadBody> {
  const text = await response.text();
  const excerpt = text.slice(0, BODY_EXCERPT_CHARS);
  try {
    return { excerpt, json: JSON.parse(text) as unknown };
  } catch {
    return { excerpt, json: undefined };
  }
}

// oauthErrorIn lê {error, error_description} do corpo de erro; fora desse formato, o
// código vira sem_codigo_oauth e a descrição cita o trecho do corpo.
export function oauthErrorIn(body: ReadBody): OAuthErrorFields {
  const parsed = oauthErrorBody.safeParse(body.json);
  if (!parsed.success) {
    return {
      code: "sem_codigo_oauth",
      description: `corpo fora do formato {error, error_description}: ${JSON.stringify(body.excerpt)}`,
    };
  }
  return {
    code: parsed.data.error,
    description: parsed.data.error_description ?? "",
  };
}
