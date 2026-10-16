class CurrentTenantsController < ApplicationController
  # Só grava tenant que a API lista para a conta.
  def update
    tenant = current_tenants.find { it.id == params[:tenant_id] }
    return redirect_to(tenants_path, alert: t("markupp_api.errors.tenant_unavailable")) unless tenant

    credential.tenant_id = tenant.id
    redirect_to root_path
  end
end
