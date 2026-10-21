import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import {
  UnauthorizedError,
  type OAuthClientProvider,
} from "@modelcontextprotocol/sdk/client/auth.js";
import { StreamableHTTPClientTransport } from "@modelcontextprotocol/sdk/client/streamableHttp.js";
import type { FetchLike } from "@modelcontextprotocol/sdk/shared/transport.js";
import type {
  OAuthClientInformationMixed,
  OAuthClientMetadata,
  OAuthTokens,
} from "@modelcontextprotocol/sdk/shared/auth.js";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { markuppUrl } from "./markupp-stack.js";
import { approveInPanel, loginByPassword } from "./panel-consent.js";
import {
  callToolJson,
  callToolTexts,
  type NoteHeader,
  type TenantView,
} from "./tool-text.js";

// O agente usa o cliente embutido markupp-mcp, que aceita redirect em loopback por IP em
// qualquer porta: o DCR vem desligado e assim o compose não muda. O redirect não é
// servido, porque o e2e lê o code direto do redirect_to da aprovação.
const REDIRECT_URI = "http://127.0.0.1:53682/callback";
const STATE = "estado-do-e2e";

// AgentOAuthProvider guarda em memória o que um agente MCP real guardaria em disco.
class AgentOAuthProvider implements OAuthClientProvider {
  authorizationUrl: URL | undefined;
  private savedTokens: OAuthTokens | undefined;
  private verifier = "";

  get redirectUrl(): string {
    return REDIRECT_URI;
  }

  get clientMetadata(): OAuthClientMetadata {
    return {
      client_name: "markupp e2e",
      redirect_uris: [REDIRECT_URI],
      token_endpoint_auth_method: "none",
    };
  }

  state(): string {
    return STATE;
  }

  clientInformation(): OAuthClientInformationMixed {
    return { client_id: "markupp-mcp" };
  }

  tokens(): OAuthTokens | undefined {
    return this.savedTokens;
  }

  saveTokens(tokens: OAuthTokens): void {
    this.savedTokens = tokens;
  }

  redirectToAuthorization(url: URL): void {
    this.authorizationUrl = url;
  }

  saveCodeVerifier(verifier: string): void {
    this.verifier = verifier;
  }

  codeVerifier(): string {
    return this.verifier;
  }
}

const mcpUrl = new URL(`${markuppUrl()}/mcp`);
const requestedUrls: string[] = [];
const recordingFetch: FetchLike = (url, init) => {
  requestedUrls.push(String(url));
  return fetch(url, init);
};
const provider = new AgentOAuthProvider();
const client = new Client({ name: "markupp-e2e-http", version: "0.0.0" });

// authorizeWithConsent faz a primeira conexão, que o 401 desvia para o authorize, aprova
// o pedido como o painel e troca o code pelo token do agente.
async function authorizeWithConsent(): Promise<void> {
  const transport = new StreamableHTTPClientTransport(mcpUrl, {
    authProvider: provider,
    fetch: recordingFetch,
  });
  const firstTry = new Client({ name: "markupp-e2e-http", version: "0.0.0" });
  await expect(firstTry.connect(transport)).rejects.toThrow(UnauthorizedError);
  const authorizeUrl = provider.authorizationUrl;
  if (!authorizeUrl) throw new Error("o SDK nao pediu o authorize, esperado");
  const redirect = await approveInPanel(
    authorizeUrl,
    await loginByPassword(markuppUrl()),
  );
  expect(redirect.searchParams.get("state")).toBe(STATE);
  await transport.finishAuth(redirect.searchParams.get("code") ?? "");
}

beforeAll(async () => {
  await authorizeWithConsent();
  await client.connect(
    new StreamableHTTPClientTransport(mcpUrl, { authProvider: provider }),
  );
});

afterAll(() => client.close());

describe("transporte HTTP com OAuth", () => {
  it("descobre o authorization server pelo 401 e pede o MCP como resource", () => {
    expect(requestedUrls).toContain(
      `${markuppUrl()}/.well-known/oauth-protected-resource/mcp`,
    );
    expect(requestedUrls).toContain(
      `${markuppUrl()}/.well-known/oauth-authorization-server`,
    );
    const params = provider.authorizationUrl?.searchParams;
    expect(params?.get("resource")).toBe(mcpUrl.href);
    expect(params?.get("client_id")).toBe("markupp-mcp");
    expect(params?.get("code_challenge_method")).toBe("S256");
  });

  it("cria, acha, le, edita e apaga uma nota pelas ferramentas", async () => {
    const [tenant] = await callToolJson<TenantView[]>(
      client,
      "list_tenants",
      {},
    );
    expect(tenant?.role).toBe("owner");
    await exerciseNoteLifecycle(tenant?.id ?? "");
  });
});

async function exerciseNoteLifecycle(tenant: string): Promise<void> {
  const marker = `marcador-${Date.now()}`;
  const created = await callToolJson<NoteHeader>(client, "create_note", {
    tenant,
    path: `e2e/${marker}.md`,
    content: `# Nota do e2e\n\nconteudo com ${marker}\n`,
  });
  const found = await callToolJson<NoteHeader[]>(client, "search_notes", {
    tenant,
    query: marker,
  });
  expect(found.map((note) => note.id)).toEqual([created.id]);
  const note = { tenant, id: created.id };
  await callToolTexts(client, "edit_note", {
    ...note,
    old_text: `conteudo com ${marker}`,
    new_text: "conteudo editado",
  });
  const [, content] = await callToolTexts(client, "read_note", note);
  expect(content).toBe("# Nota do e2e\n\nconteudo editado\n");
  await callToolTexts(client, "delete_note", note);
  await expect(callToolTexts(client, "read_note", note)).rejects.toThrow(
    /not_found/,
  );
}
