// ApiTokenSource entrega um JWT da API válido no momento da chamada. No HTTP ele vem da
// troca de token com o Go, guardada até expirar; a sessão MCP dura mais que o JWT.
export type ApiTokenSource = () => Promise<string>;

// MarkuppSession é o que o authenticate do FastMCP devolve e cada ferramenta recebe.
export type MarkuppSession = {
  apiToken: ApiTokenSource;
};

// PublicSession é a sessão anônima de /mcp/public (ADR-0051): sem login, só o
// X-Forwarded-For do agente, para o Go contar o limite por IP dele.
export type PublicSession = {
  forwardedFor?: string;
};
