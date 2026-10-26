import type {
  AgentClient,
  AgentClientTemplate,
  DeeplinkTemplate,
  SnippetTemplate,
} from "./catalog-types.js";
import { encodeDeeplinkPayload, type JsonValue } from "./link-encoding.js";

// PLACEHOLDERS são os marcadores que os modelos usam no lugar das urls.
export const PLACEHOLDERS = {
  instanceUrl: "{{instanceUrl}}",
  mcpUrl: "{{mcpUrl}}",
} as const;

type InstanceUrls = { instanceUrl: string; mcpUrl: string };

// instanceUrlsFor valida a url da instância e deriva a do MCP, que o Go serve em /mcp.
export function instanceUrlsFor(rawUrl: string): InstanceUrls {
  if (!URL.canParse(rawUrl) || !/^https?:$/.test(new URL(rawUrl).protocol)) {
    throw new Error(
      `url da instancia ${JSON.stringify(rawUrl)}, esperado http(s)://host[/caminho]`,
    );
  }
  const instanceUrl = rawUrl.replace(/\/+$/, "");
  return { instanceUrl, mcpUrl: `${instanceUrl}/mcp` };
}

function expand(text: string, urls: InstanceUrls): string {
  return text
    .replaceAll(PLACEHOLDERS.instanceUrl, urls.instanceUrl)
    .replaceAll(PLACEHOLDERS.mcpUrl, urls.mcpUrl);
}

function expandJson(value: JsonValue, urls: InstanceUrls): JsonValue {
  if (typeof value === "string") return expand(value, urls);
  if (Array.isArray(value)) return value.map((item) => expandJson(item, urls));
  if (value === null || typeof value !== "object") return value;
  return Object.fromEntries(
    Object.entries(value).map(([key, item]) => [key, expandJson(item, urls)]),
  );
}

function renderSnippet(snippet: SnippetTemplate, urls: InstanceUrls) {
  const { template, ...rest } = snippet;
  return { ...rest, text: expand(template, urls) };
}

function renderDeeplink(deeplink: DeeplinkTemplate, urls: InstanceUrls) {
  const payload = expandJson(deeplink.payload, urls);
  return {
    id: deeplink.id,
    href: deeplink.base + encodeDeeplinkPayload(payload, deeplink.encoding),
  };
}

// renderAgentClient troca os marcadores de um cliente pelas urls da instância.
export function renderAgentClient(
  client: AgentClientTemplate,
  urls: InstanceUrls,
): AgentClient {
  return {
    id: client.id,
    name: client.name,
    docsUrl: client.docsUrl,
    snippets: client.snippets.map((each) => renderSnippet(each, urls)),
    deeplinks: client.deeplinks.map((each) => renderDeeplink(each, urls)),
  };
}
