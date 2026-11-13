import {
  MarkuppApiError,
  type InstanceEmbedding,
  type InstanceEmbeddingUpdate,
} from "../markupp-api.js";

// FakeInstanceEmbedding guarda a escolha do provedor de embedding da instância e
// responde como a API: instance_admin_required para quem não administra e host_locked
// quando quem hospeda trava o provedor. A chave fica guardada, nunca devolvida.
export class FakeInstanceEmbedding {
  admin = false;
  locked = false;
  private choice: InstanceEmbeddingUpdate = { provider: "none" };
  private apiKey = "";

  view(): InstanceEmbedding {
    this.requireAdmin();
    const { provider, url = "", model = "" } = this.choice;
    return {
      provider,
      url,
      model,
      api_key_set: this.apiKey !== "",
      locked: this.locked,
      host_provider: this.locked ? "none" : "",
      model_status: null,
    };
  }

  update(update: InstanceEmbeddingUpdate): InstanceEmbedding {
    this.requireAdmin();
    if (this.locked) {
      throw new MarkuppApiError(
        409,
        "host_locked",
        "the embedding provider is set by the host configuration",
      );
    }
    const { api_key, ...choice } = update;
    this.choice = choice;
    if (api_key !== undefined) this.apiKey = api_key;
    return this.view();
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
