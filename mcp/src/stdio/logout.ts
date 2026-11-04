import { errorMessage } from "../resolve-http-config.js";
import type { CredentialStore, StoredCredential } from "./credential-store.js";
import { withFileLock, type LockDeps } from "./file-lock.js";
import { discoverAuthServer, revokeToken } from "./public-client.js";

export type LogoutDeps = LockDeps & {
  store: CredentialStore;
  fetch: typeof fetch;
};

// LogoutResult diz se havia credencial para revogar.
export type LogoutResult = "revoked" | "absent";

// runLogout revoga o refresh no Go (RFC 7009) e apaga a credencial, com o lock para não
// cruzar com a renovação de um processo stdio. A credencial sai do disco mesmo quando a
// revogação falha, e o erro diz para revogar a Autorização pelo painel.
export async function runLogout(deps: LogoutDeps): Promise<LogoutResult> {
  if ((await deps.store.load()) === undefined) return "absent";
  return withFileLock(deps.store.lockPath, deps, () => logoutUnderLock(deps));
}

async function logoutUnderLock(deps: LogoutDeps): Promise<LogoutResult> {
  const credential = await deps.store.load();
  if (credential === undefined) return "absent";
  try {
    await revokeStored(credential, deps.fetch);
  } catch (error) {
    await deps.store.remove();
    throw new Error(
      `credential deleted from ${deps.store.path}, but revoking it on markupp failed (${errorMessage(error)}); the login stays valid on the server until it expires`,
    );
  }
  await deps.store.remove();
  return "revoked";
}

async function revokeStored(
  credential: StoredCredential,
  fetcher: typeof fetch,
): Promise<void> {
  const metadata = await discoverAuthServer(credential.issuer, fetcher);
  await revokeToken(
    metadata.revocationEndpoint,
    credential.refreshToken,
    fetcher,
  );
}
