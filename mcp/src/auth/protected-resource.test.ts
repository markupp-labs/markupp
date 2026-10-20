import { describe, expect, it } from "vitest";
import {
  MARKUPP_SCOPES,
  protectedResourceMetadataUrl,
  protectedResourceOptions,
} from "./protected-resource.js";

describe("protectedResourceOptions", () => {
  it("publica só a metadata do recurso, com o Go como authorization server", () => {
    const options = protectedResourceOptions(
      "https://mcp.exemplo/mcp",
      "https://markupp.exemplo",
      MARKUPP_SCOPES,
    );
    expect(options).toEqual({
      enabled: true,
      protectedResource: {
        resource: "https://mcp.exemplo/mcp",
        authorizationServers: ["https://markupp.exemplo"],
        scopesSupported: ["notas:leitura", "notas:escrita", "administracao"],
      },
    });
    expect(options.authorizationServer).toBeUndefined();
    expect(options.proxy).toBeUndefined();
  });
});

describe("protectedResourceMetadataUrl", () => {
  it("fica na raiz quando o resource não tem caminho", () => {
    expect(protectedResourceMetadataUrl("https://mcp.exemplo")).toBe(
      "https://mcp.exemplo/.well-known/oauth-protected-resource",
    );
  });

  it("insere o well-known antes do caminho do resource", () => {
    expect(protectedResourceMetadataUrl("https://mcp.exemplo/mcp")).toBe(
      "https://mcp.exemplo/.well-known/oauth-protected-resource/mcp",
    );
  });
});
