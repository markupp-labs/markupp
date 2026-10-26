import type { DeeplinkEncoding, JsonValue } from "./link-encoding.js";

// SnippetLanguage diz como o painel mostra o trecho: comando, arquivo json/toml ou texto.
export type SnippetLanguage = "shell" | "json" | "toml" | "text";

// SnippetTemplate é um trecho para copiar, com {{mcpUrl}} e {{instanceUrl}} no lugar da
// url. path é o arquivo onde o trecho entra, quando é configuração.
export type SnippetTemplate = {
  id: string;
  language: SnippetLanguage;
  path?: string;
  template: string;
};

// DeeplinkTemplate é um botão que abre o cliente já com o servidor: base + payload
// codificado. As strings do payload levam os mesmos marcadores dos trechos.
export type DeeplinkTemplate = {
  id: string;
  base: string;
  payload: JsonValue;
  encoding: DeeplinkEncoding;
};

// AgentClientTemplate é um cliente MCP do catálogo, sem url. O texto para pessoas fica
// em quem mostra (painel, site), pelo id do cliente e do trecho.
export type AgentClientTemplate = {
  id: string;
  name: string;
  docsUrl: string;
  snippets: SnippetTemplate[];
  deeplinks: DeeplinkTemplate[];
};

// AgentClient é o cliente montado para uma instância.
export type AgentClient = {
  id: string;
  name: string;
  docsUrl: string;
  snippets: {
    id: string;
    language: SnippetLanguage;
    path?: string;
    text: string;
  }[];
  deeplinks: { id: string; href: string }[];
};
