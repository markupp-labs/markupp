# Troca do cofre ativo pelo seletor da trilha da barra lateral, como a do tenant (CurrentTenantsController).
class CurrentVaultsController < ApplicationController
  before_action :require_tenant

  # Só grava cofre que a API lista para a conta no tenant ativo.
  def update
    vault = Vault.listed(reachable_vaults, params[:vault_id])
    return redirect_to(root_path, alert: t("markupp_api.errors.vault_unavailable")) unless vault

    credential.vault_id = vault.id
    redirect_to root_path
  end
end
