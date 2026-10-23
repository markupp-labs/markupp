# Cofres do tenant ativo. Criar vem do diálogo New vault (vaults/_new_dialog): o cofre novo já fica ativo e abre a
# árvore dele, vazia. A API decide quem pode (viewer recebe read_only, que o ApplicationController trata).
class VaultsController < ApplicationController
  before_action :require_tenant

  def create
    vault = Vault.create(credential.api, params.expect(vault: [ :name ])[:name])
    credential.vault_id = vault.id
    redirect_to root_path, notice: t(".created", name: vault.name)
  rescue MarkuppApi::InvalidName => error
    log_api_event("markupp_vault_name_refused", error)
    redirect_back_or_to root_path, alert: t("vaults.invalid_name")
  end
end
