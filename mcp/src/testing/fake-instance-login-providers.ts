import {
  MarkuppApiError,
  type LoginProvider,
  type LoginProviderChange,
  type LoginProviderCreation,
  type LoginProviderList,
  type LoginProviderRemoval,
} from "../markupp-api.js";

// StoredLoginProvider é o provedor guardado: o segredo fica aqui, nunca na resposta.
type StoredLoginProvider = Omit<LoginProvider, "client_secret_set"> & {
  secret: string;
};

// FakeCallbackBase é a origem do callback_url que o fake devolve.
const FakeCallbackBase = "https://markupp.test/v1/auth/oidc";

// FakeInstanceLoginProviders guarda os provedores de login externo da instância e
// responde como a API: instance_admin_required, host_locked, login_provider_exists,
// login_provider_not_found, login_provider_issuer_locked e invalid_login_provider.
export class FakeInstanceLoginProviders {
  admin = false;
  locked = false;
  private readonly providers = new Map<string, StoredLoginProvider>();

  // seed grava direto, sem conferir papel nem trava, para montar o cenário.
  seed(id: string, linkedAccounts: number): void {
    this.providers.set(id, {
      id,
      name: id,
      issuer: `https://${id}.test`,
      client_id: `${id}-client`,
      secret: "seeded-secret",
      enabled: true,
      locked: false,
      callback_url: `${FakeCallbackBase}/${id}/callback`,
      linked_accounts: linkedAccounts,
    });
  }

  list(): LoginProviderList {
    this.requireAdmin();
    const providers = [...this.providers.values()].map(publicView);
    return { locked: this.locked, providers };
  }

  create(creation: LoginProviderCreation): LoginProvider {
    this.requireWritable();
    if (this.providers.has(creation.id)) {
      throw new MarkuppApiError(
        409,
        "login_provider_exists",
        `login provider ${creation.id} already exists`,
      );
    }
    requireIssuer(creation.id, creation.issuer);
    const { client_secret, ...fields } = creation;
    const callback_url = `${FakeCallbackBase}/${creation.id}/callback`;
    const stored = { ...fields, secret: client_secret, locked: false };
    this.providers.set(creation.id, {
      ...stored,
      callback_url,
      linked_accounts: 0,
    });
    return this.view(creation.id);
  }

  update(id: string, change: LoginProviderChange): LoginProvider {
    this.requireWritable();
    const stored = this.find(id);
    if (stored.linked_accounts > 0 && stored.issuer !== change.issuer) {
      throw new MarkuppApiError(
        409,
        "login_provider_issuer_locked",
        `login provider ${id} has ${stored.linked_accounts} linked accounts, its issuer cannot change`,
      );
    }
    requireIssuer(id, change.issuer);
    const { client_secret, ...fields } = change;
    Object.assign(stored, fields);
    if (client_secret !== undefined) stored.secret = client_secret;
    return this.view(id);
  }

  remove(id: string): LoginProviderRemoval {
    this.requireWritable();
    const removed_links = this.find(id).linked_accounts;
    this.providers.delete(id);
    return { removed_links };
  }

  private view(id: string): LoginProvider {
    return publicView(this.find(id));
  }

  private find(id: string): StoredLoginProvider {
    const stored = this.providers.get(id);
    if (stored) return stored;
    throw new MarkuppApiError(
      404,
      "login_provider_not_found",
      `login provider ${id} not found`,
    );
  }

  private requireWritable(): void {
    this.requireAdmin();
    if (!this.locked) return;
    throw new MarkuppApiError(
      409,
      "host_locked",
      "login providers are set by the host configuration",
    );
  }

  private requireAdmin(): void {
    if (this.admin) return;
    throw new MarkuppApiError(
      403,
      "instance_admin_required",
      "account is not an instance admin",
    );
  }
}

function publicView(stored: StoredLoginProvider): LoginProvider {
  const { secret, ...provider } = stored;
  return { ...provider, client_secret_set: secret !== "" };
}

// requireIssuer segue a API: https, ou vazio só no id github.
function requireIssuer(id: string, issuer: string): void {
  if (issuer.startsWith("https://") || (issuer === "" && id === "github"))
    return;
  throw new MarkuppApiError(
    422,
    "invalid_login_provider",
    `issuer must be an https URL, got ${issuer}`,
  );
}
