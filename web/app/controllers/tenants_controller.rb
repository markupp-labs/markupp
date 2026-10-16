# Escolher ou criar tenant: é o onboarding de quem entra sem nenhum.
class TenantsController < ApplicationController
  def index
  end

  # Tenant criado já fica ativo e abre as notas dele.
  def create
    @name = params.expect(tenant: [ :name ])[:name]
    tenant = Tenant.create(credential.api, @name)
    credential.tenant_id = tenant.id
    redirect_to root_path, notice: t(".created", name: tenant.name)
  rescue MarkuppApi::InvalidRequest
    @name_error = t(".invalid_name")
    render :index, status: :unprocessable_content
  end
end
