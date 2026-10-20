// McpEnvironment é a parte do process.env que o servidor MCP lê.
export type McpEnvironment = Record<string, string | undefined>;

// HttpConfig é o que o transporte HTTP precisa: a API do Go pelo endereço interno, o
// issuer público anunciado na metadata e o servidor MCP como resource do Go.
export type HttpConfig = {
  apiUrl: string;
  issuerUrl: string;
  resourceUrl: string;
  resourceSecret: string;
  endpoint: `/${string}`;
  port: number;
};

const DEFAULT_PORT = 3000;
const MIN_RESOURCE_SECRET_BYTES = 32;
const LOOPBACK_HOSTS = new Set(["localhost", "127.0.0.1", "[::1]"]);

// PublicUrlRule é a regra do Go para as URLs que o agente vê: https, ou http só em
// loopback, sem userinfo, query nem fragmento; o caminho varia por variável.
type PublicUrlRule = { path: "forbidden" | "required"; expected: string };

const ISSUER_RULE: PublicUrlRule = {
  path: "forbidden",
  expected:
    "https://host[:porta] sem caminho, query nem fragmento (http so em localhost, 127.0.0.1 ou [::1])",
};
const RESOURCE_RULE: PublicUrlRule = {
  path: "required",
  expected:
    "https://host[:porta]/caminho sem query nem fragmento (http so em localhost, 127.0.0.1 ou [::1]); o caminho e a rota do endpoint MCP",
};

// loadHttpConfig lê e valida as variáveis do transporte HTTP, falhando na primeira
// inválida com o nome dela, o valor recebido e o formato esperado.
export function loadHttpConfig(env: McpEnvironment): HttpConfig {
  const resource = publicUrl(env, "MARKUPP_MCP_RESOURCE_URL", RESOURCE_RULE);
  return {
    apiUrl: internalUrl(env, "MARKUPP_MCP_API_URL"),
    issuerUrl: publicUrl(env, "MARKUPP_MCP_ISSUER_URL", ISSUER_RULE).origin,
    resourceUrl: resource.href,
    resourceSecret: resourceSecret(env),
    endpoint: resource.pathname as `/${string}`,
    port: port(env),
  };
}

function required(env: McpEnvironment, key: string, expected: string): string {
  const value = env[key]?.trim() ?? "";
  if (value === "") {
    throw new Error(`${key} vazia, esperado ${expected}`);
  }
  return value;
}

// internalUrl aceita http em qualquer host: é o endereço do Go dentro do cluster ou do
// compose, que o agente nunca vê.
function internalUrl(env: McpEnvironment, key: string): string {
  const expected = "http(s)://host[:porta] do servidor Go";
  const raw = required(env, key, expected);
  const url = URL.parse(raw);
  if (!url || !["http:", "https:"].includes(url.protocol)) {
    throw new Error(`${key}="${raw}", esperado ${expected}`);
  }
  return raw.replace(/\/+$/, "");
}

function publicUrl(env: McpEnvironment, key: string, rule: PublicUrlRule): URL {
  const raw = required(env, key, rule.expected);
  const url = URL.parse(raw);
  if (!url || !publicUrlAllowed(url, raw, rule)) {
    throw new Error(`${key}="${raw}", esperado ${rule.expected}`);
  }
  return url;
}

function publicUrlAllowed(url: URL, raw: string, rule: PublicUrlRule): boolean {
  const secure =
    url.protocol === "https:" ||
    (url.protocol === "http:" && LOOPBACK_HOSTS.has(url.hostname));
  const clean = !url.username && !url.password && !raw.includes("?");
  const hasPath = url.pathname !== "/";
  const pathOk = rule.path === "required" ? hasPath : !hasPath;
  return secure && clean && !raw.includes("#") && pathOk;
}

// resourceSecret cita só o tamanho do segredo, nunca o valor.
function resourceSecret(env: McpEnvironment): string {
  const key = "MARKUPP_MCP_RESOURCE_SECRET";
  const expected = `o segredo do par deste servidor em MARKUPP_OAUTH_RESOURCES no Go, com ao menos ${MIN_RESOURCE_SECRET_BYTES} bytes aleatorios`;
  const secret = required(env, key, expected);
  const bytes = new TextEncoder().encode(secret).length;
  if (bytes < MIN_RESOURCE_SECRET_BYTES) {
    throw new Error(
      `${key} com ${bytes} bytes, esperado ao menos ${MIN_RESOURCE_SECRET_BYTES} bytes aleatorios`,
    );
  }
  return secret;
}

function port(env: McpEnvironment): number {
  const raw = env.MARKUPP_MCP_PORT?.trim();
  if (!raw) return DEFAULT_PORT;
  const value = Number(raw);
  if (!/^\d+$/.test(raw) || value < 1 || value > 65535) {
    throw new Error(`MARKUPP_MCP_PORT="${raw}", esperado inteiro de 1 a 65535`);
  }
  return value;
}
