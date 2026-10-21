import type { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { CallToolResultSchema } from "@modelcontextprotocol/sdk/types.js";

type ToolArguments = Record<string, string>;

// callToolTexts chama a ferramenta e devolve os blocos de texto, falhando no isError
// com a mensagem que o agente leria.
export async function callToolTexts(
  client: Client,
  name: string,
  args: ToolArguments,
): Promise<string[]> {
  const result = CallToolResultSchema.parse(
    await client.callTool({ name, arguments: args }),
  );
  const texts = result.content.flatMap((block) =>
    block.type === "text" ? [block.text] : [],
  );
  if (result.isError) {
    throw new Error(`${name} falhou: ${texts.join(" ")}, esperado sucesso`);
  }
  return texts;
}

// callToolJson lê o primeiro bloco de texto da ferramenta como JSON.
export async function callToolJson<T>(
  client: Client,
  name: string,
  args: ToolArguments,
): Promise<T> {
  const [first] = await callToolTexts(client, name, args);
  return JSON.parse(first ?? "null") as T;
}

export type TenantView = { id: string; name: string; role: string };
export type NoteHeader = { id: string; path: string; updated_at: string };
