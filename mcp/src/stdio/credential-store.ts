import { isAbsolute, join } from "node:path";
import { z } from "zod";
import type { McpEnvironment } from "../config.js";
import type { LocalFiles } from "./local-files.js";

// StoredCredential é o login do stdio no disco: o issuer, o refresh rotativo e o JWT
// com o vencimento, para outro processo reaproveitar o JWT sem girar o refresh.
export type StoredCredential = {
  issuer: string;
  refreshToken: string;
  accessToken: string;
  expiresAtMs: number;
};

const FILE_SHAPE =
  "esperado JSON {issuer, refresh_token, access_token, expires_at em ISO 8601}";

const credentialFile = z.object({
  issuer: z.string().min(1),
  refresh_token: z.string().min(1),
  access_token: z.string().min(1),
  expires_at: z.iso.datetime(),
});

// credentialsPath segue o XDG: $XDG_CONFIG_HOME, que só vale absoluto, ou ~/.config.
export function credentialsPath(env: McpEnvironment, homeDir: string): string {
  const xdg = env.XDG_CONFIG_HOME?.trim() ?? "";
  const configDir = isAbsolute(xdg) ? xdg : join(homeDir, ".config");
  return join(configDir, "markupp", "credentials.json");
}

// CredentialStore lê e grava a credencial em path; o lock fica ao lado, em path.lock.
export class CredentialStore {
  constructor(
    readonly path: string,
    private readonly files: LocalFiles,
  ) {}

  get lockPath(): string {
    return `${this.path}.lock`;
  }

  async load(): Promise<StoredCredential | undefined> {
    const raw = await this.files.readText(this.path);
    return raw === undefined ? undefined : parseCredential(this.path, raw);
  }

  save(credential: StoredCredential): Promise<void> {
    const json = JSON.stringify(credentialJson(credential), null, 2);
    return this.files.writePrivate(this.path, `${json}\n`);
  }

  remove(): Promise<void> {
    return this.files.remove(this.path);
  }
}

function credentialJson(
  credential: StoredCredential,
): z.infer<typeof credentialFile> {
  return {
    issuer: credential.issuer,
    refresh_token: credential.refreshToken,
    access_token: credential.accessToken,
    expires_at: new Date(credential.expiresAtMs).toISOString(),
  };
}

// parseCredential cita o caminho e os campos inválidos, nunca o conteúdo, que tem tokens.
function parseCredential(path: string, raw: string): StoredCredential {
  const parsed = credentialFile.safeParse(jsonOrUndefined(raw));
  if (!parsed.success) {
    const fields = parsed.error.issues.map((i) => i.path.join(".") || "(raiz)");
    throw new Error(
      `credencial em ${path} invalida nos campos [${fields.join(", ")}], ${FILE_SHAPE}; rode markupp-mcp login de novo`,
    );
  }
  const { issuer, refresh_token, access_token, expires_at } = parsed.data;
  return {
    issuer,
    refreshToken: refresh_token,
    accessToken: access_token,
    expiresAtMs: Date.parse(expires_at),
  };
}

function jsonOrUndefined(raw: string): unknown {
  try {
    return JSON.parse(raw) as unknown;
  } catch {
    return undefined;
  }
}
