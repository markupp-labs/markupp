// ApiTokenSource entrega um JWT da API válido no momento da chamada. No HTTP ele vem da
// troca de token com o Go, guardada até expirar; a sessão MCP dura mais que o JWT.
export type ApiTokenSource = () => Promise<string>;

// MarkuppSession é o que o authenticate do FastMCP devolve e cada ferramenta recebe.
export type MarkuppSession = {
  apiToken: ApiTokenSource;
};
