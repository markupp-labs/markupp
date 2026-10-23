# Cofres do tenant ativo. Criar vem do diálogo New vault (vaults/_new_dialog): o cofre novo já fica ativo e abre a
# árvore dele, vazia. Renomear e apagar vêm da General do cofre (settings/vault_general). A API decide quem pode
# (viewer recebe read_only e cofre fora de alcance vault_forbidden, que o ApplicationController trata).
class VaultsController < ApplicationController
  before_action :require_tenant

  def create
    vault = Vault.create(credential.api, vault_name_param)
    credential.vault_id = vault.id
    redirect_to root_path, notice: t(".created", name: vault.name)
  rescue MarkuppApi::InvalidName => error
    refuse_name(error, root_path)
  end

  def update
    vault = Vault.rename(credential.api, params[:id], vault_name_param)
    redirect_to settings_vault_path, notice: t(".renamed", name: vault.name)
  rescue MarkuppApi::InvalidName => error
    refuse_name(error, settings_vault_path)
  end

  # Só com o nome do cofre digitado exato, que o painel confere antes de chamar a API, como o apagar do tenant. Apagado,
  # a sessão esquece o cofre e a próxima página ativa o primeiro que sobrar.
  def destroy
    vault = current_vaults.find { it.id == params[:id] } or raise MarkuppApi::VaultUnavailable, "vault #{params[:id].inspect} is not in /v1/vaults"
    return redirect_to(settings_vault_path, alert: t(".name_mismatch")) unless params[:confirm_name] == vault.name

    Vault.delete(credential.api, vault.id)
    credential.vault_id = nil if credential.vault_id == vault.id
    redirect_to root_path, notice: t(".deleted", name: vault.name)
  end

  private
    def vault_name_param = params.expect(vault: [ :name ])[:name]

    # Volta para a página do formulário: a do diálogo New vault, ou a General do cofre.
    def refuse_name(error, fallback)
      log_api_event("markupp_vault_name_refused", error)
      redirect_back_or_to fallback, alert: t("vaults.invalid_name")
    end
end
