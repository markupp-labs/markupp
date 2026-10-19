class CurrentTenantsController < ApplicationController
  # Para onde o card leva depois de trocar: os ícones de membros e configurações abrem aquela página do tenant; sem
  # destino, ou com um desconhecido, a nota padrão.
  DESTINATIONS = { "members" => :settings_members_path, "settings" => :settings_invites_path }.freeze

  # Só grava tenant que a API lista para a conta.
  def update
    tenant = current_tenants.find { it.id == params[:tenant_id] }
    return redirect_to(tenants_path, alert: t("markupp_api.errors.tenant_unavailable")) unless tenant

    credential.tenant_id = tenant.id
    redirect_to send(DESTINATIONS.fetch(params[:to].to_s, :root_path))
  end
end
