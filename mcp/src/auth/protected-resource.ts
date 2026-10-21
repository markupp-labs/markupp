import type { ServerOptions } from "fastmcp";
import type { MarkuppSession } from "../session.js";

// MARKUPP_SCOPES espelha oauth.SupportedScopes do Go, na mesma ordem.
export const MARKUPP_SCOPES: readonly string[] = [
  "notas:leitura",
  "notas:escrita",
  "administracao",
];

export type ProtectedResourceOAuthOptions = NonNullable<
  ServerOptions<MarkuppSession>["oauth"]
>;

// protectedResourceOptions monta a opção oauth do FastMCP que publica só a metadata do
// recurso protegido (RFC 9728) e aponta o Go como authorization server. Sem
// authorizationServer nem proxy, o FastMCP não liga rotas de authorization server.
export function protectedResourceOptions(
  resourceUrl: string,
  issuerUrl: string,
  scopes: readonly string[],
): ProtectedResourceOAuthOptions {
  return {
    enabled: true,
    protectedResource: {
      resource: resourceUrl,
      authorizationServers: [issuerUrl],
      scopesSupported: [...scopes],
    },
  };
}

// protectedResourceMetadataUrl insere o well-known entre o host e o caminho do resource
// (RFC 9728, seção 3.1), igual ao FastMCP nos 401 que ele mesmo emite.
export function protectedResourceMetadataUrl(resourceUrl: string): string {
  const url = new URL(resourceUrl);
  const path = url.pathname === "/" ? "" : url.pathname;
  url.pathname = `/.well-known/oauth-protected-resource${path}`;
  return url.toString();
}
