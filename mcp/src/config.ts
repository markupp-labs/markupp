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

// ResourceSecretSource é de onde vem o segredo do resource: o valor na variável, que
// tem precedência, ou o arquivo que o Go escreve no volume compartilhado.
export type ResourceSecretSource =
  { kind: "value"; value: string } | { kind: "file"; path: string };

// HttpEnvConfig é o que dá para resolver só com o ambiente; o issuer e o segredo em
// arquivo dependem de I/O e ficam para resolveHttpConfig.
export type HttpEnvConfig = {
  apiUrl: string;
  resourceUrl: string | undefined;
  secret: ResourceSecretSource;
  port: number;
};

// PublicUrlRule é a regra do Go para as URLs que o agente vê: https, ou http só em
// loopback, sem userinfo, query nem fragmento; o caminho varia por variável.
export type PublicUrlRule = {
  path: "forbidden" | "required";
  expected: string;
};

const DEFAULT_API_URL = "http://localhost:8080";
const DEFAULT_PORT = 3000;
const MIN_RESOURCE_SECRET_BYTES = 32;
const LOOPBACK_HOSTS = new Set(["localhost", "127.0.0.1", "[::1]"]);

export const ISSUER_RULE: PublicUrlRule = {
  path: "forbidden",
  expected:
    "https://host[:porta] sem caminho, query nem fragmento (http so em localhost, 127.0.0.1 ou [::1])",
};
const RESOURCE_RULE: PublicUrlRule = {
  path: "required",
  expected:
    "https://host[:porta]/caminho sem query nem fragmento (http so em localhost, 127.0.0.1 ou [::1]); o caminho e a rota do endpoint MCP",
};

// loadHttpEnv lê e valida as variáveis do transporte HTTP, todas opcionais, falhando
// na primeira inválida com o nome dela, o valor recebido e o formato esperado.
export function loadHttpEnv(env: McpEnvironment): HttpEnvConfig {
  return {
    apiUrl: internalUrl(env, "MARKUPP_MCP_API_URL"),
    resourceUrl: optionalResourceUrl(env),
    secret: secretSource(env),
    port: port(env),
  };
}

function optional(env: McpEnvironment, key: string): string | undefined {
  const value = env[key]?.trim() ?? "";
  return value === "" ? undefined : value;
}

// internalUrl aceita http em qualquer host: é o endereço do Go dentro do cluster ou do
// compose, que o agente nunca vê.
function internalUrl(env: McpEnvironment, key: string): string {
  const raw = optional(env, key) ?? DEFAULT_API_URL;
  const url = URL.parse(raw);
  if (!url || !["http:", "https:"].includes(url.protocol)) {
    throw new Error(
      `${key}="${raw}", esperado http(s)://host[:porta] do servidor Go`,
    );
  }
  return raw.replace(/\/+$/, "");
}

function optionalResourceUrl(env: McpEnvironment): string | undefined {
  const key = "MARKUPP_MCP_RESOURCE_URL";
  const raw = optional(env, key);
  if (raw === undefined) return undefined;
  return checkPublicUrl(key, raw, RESOURCE_RULE).href;
}

// checkPublicUrl valida a URL pela regra e cita a origem dela no erro: o nome da
// variável ou o documento de onde a URL veio.
export function checkPublicUrl(
  source: string,
  raw: string,
  rule: PublicUrlRule,
): URL {
  const url = URL.parse(raw);
  if (!url || !publicUrlAllowed(url, raw, rule)) {
    throw new Error(`${source}="${raw}", esperado ${rule.expected}`);
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

function secretSource(env: McpEnvironment): ResourceSecretSource {
  const key = "MARKUPP_MCP_RESOURCE_SECRET";
  const value = optional(env, key);
  if (value !== undefined) {
    return { kind: "value", value: checkSecretSize(key, value) };
  }
  const path = optional(env, "MARKUPP_MCP_SECRET_PATH");
  if (path !== undefined) return { kind: "file", path };
  throw new Error(
    `${key} e MARKUPP_MCP_SECRET_PATH vazias, esperado o segredo do resource em ${key} ou, em MARKUPP_MCP_SECRET_PATH, o caminho do arquivo em que o Go o escreve`,
  );
}

// checkSecretSize cita só a origem e o tamanho do segredo, nunca o valor.
export function checkSecretSize(source: string, secret: string): string {
  const bytes = new TextEncoder().encode(secret).length;
  if (bytes < MIN_RESOURCE_SECRET_BYTES) {
    throw new Error(
      `${source} com ${bytes} bytes, esperado ao menos ${MIN_RESOURCE_SECRET_BYTES} bytes aleatorios`,
    );
  }
  return secret;
}

function port(env: McpEnvironment): number {
  const raw = optional(env, "MARKUPP_MCP_PORT");
  if (raw === undefined) return DEFAULT_PORT;
  const value = Number(raw);
  if (!/^\d+$/.test(raw) || value < 1 || value > 65535) {
    throw new Error(`MARKUPP_MCP_PORT="${raw}", esperado inteiro de 1 a 65535`);
  }
  return value;
}
