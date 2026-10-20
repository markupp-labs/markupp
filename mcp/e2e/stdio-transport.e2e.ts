import { spawn, type ChildProcess } from "node:child_process";
import { once } from "node:events";
import {
  chmod,
  mkdir,
  mkdtemp,
  readFile,
  rm,
  writeFile,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StdioClientTransport } from "@modelcontextprotocol/sdk/client/stdio.js";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { markuppUrl } from "./markupp-stack.js";
import { approveInPanel, loginByPassword } from "./panel-consent.js";
import { callToolJson, callToolTexts, type TenantView } from "./tool-text.js";

const BIN = fileURLToPath(new URL("../dist/bin.js", import.meta.url));
const AUTHORIZE_LINE = /^(http\S+\/oauth\/authorize\?\S+)$/m;
const LOGIN_TIMEOUT_MS = 30_000;

// O stdio roda com uma pasta pessoal descartável: a credencial vai para XDG_CONFIG_HOME
// e o xdg-open falso no PATH faz o papel do navegador que não abre.
let home = "";
let apiToken = "";
const client = new Client({ name: "markupp-e2e-stdio", version: "0.0.0" });

function stdioEnv(): Record<string, string> {
  return {
    PATH: `${join(home, "bin")}:${process.env.PATH ?? ""}`,
    HOME: home,
    XDG_CONFIG_HOME: join(home, "config"),
  };
}

async function installFakeBrowser(): Promise<void> {
  const opener = join(home, "bin", "xdg-open");
  await mkdir(join(home, "bin"));
  await writeFile(opener, "#!/bin/sh\nexit 0\n");
  await chmod(opener, 0o755);
}

// authorizeUrlFrom espera o login escrever no stderr a URL que o navegador abriria.
async function authorizeUrlFrom(login: ChildProcess): Promise<URL> {
  let stderr = "";
  for await (const chunk of login.stderr ?? []) {
    stderr += String(chunk);
    const match = AUTHORIZE_LINE.exec(stderr);
    if (match?.[1]) return new URL(match[1]);
  }
  throw new Error(`login saiu sem escrever a URL do authorize: ${stderr}`);
}

// runRealLogin roda markupp-mcp login e faz o papel do navegador: aprova o pedido no
// painel e entrega o retorno ao callback em loopback que o próprio login serve.
async function runRealLogin(): Promise<void> {
  const login = spawn(process.execPath, [BIN, "login", markuppUrl()], {
    env: stdioEnv(),
    stdio: ["ignore", "ignore", "pipe"],
    timeout: LOGIN_TIMEOUT_MS,
  });
  const exited = once(login, "exit");
  const callback = await approveInPanel(
    await authorizeUrlFrom(login),
    apiToken,
  );
  expect((await fetch(callback)).status).toBe(200);
  const [code] = (await exited) as [number | null];
  expect(code).toBe(0);
}

beforeAll(async () => {
  home = await mkdtemp(join(tmpdir(), "markupp-e2e-"));
  apiToken = await loginByPassword(markuppUrl());
  await installFakeBrowser();
  await runRealLogin();
  await client.connect(
    new StdioClientTransport({
      command: process.execPath,
      args: [BIN],
      env: stdioEnv(),
    }),
  );
});

afterAll(async () => {
  await client.close();
  await rm(home, { recursive: true, force: true });
});

async function createNoteByApi(
  tenant: string,
  content: string,
): Promise<string> {
  const response = await fetch(`${markuppUrl()}/v1/notes`, {
    method: "POST",
    headers: {
      authorization: `Bearer ${apiToken}`,
      "content-type": "application/json",
      "x-markupp-tenant": tenant,
    },
    body: JSON.stringify({ path: `e2e/stdio-${Date.now()}.md`, content }),
  });
  expect(response.status).toBe(201);
  return ((await response.json()) as { id: string }).id;
}

describe("transporte stdio com login guardado", () => {
  it("grava a credencial do login no XDG_CONFIG_HOME com o issuer", async () => {
    const path = join(home, "config", "markupp", "credentials.json");
    const stored = JSON.parse(await readFile(path, "utf8")) as {
      issuer: string;
    };
    expect(stored.issuer).toBe(markuppUrl());
  });

  it("lista os tenants e le uma nota com o JWT da credencial", async () => {
    const [tenant] = await callToolJson<TenantView[]>(
      client,
      "list_tenants",
      {},
    );
    const content = "nota lida pelo stdio\n";
    const id = await createNoteByApi(tenant?.id ?? "", content);
    const [, read] = await callToolTexts(client, "read_note", {
      tenant: tenant?.id ?? "",
      id,
    });
    expect(read).toBe(content);
  });
});
