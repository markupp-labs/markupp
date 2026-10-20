# Lista de tenants em cards, com o tenant novo num dialog; para quem entra sem nenhum, é o onboarding. Com um tenant
# ativo é área ao lado das notas, como as configurações: árvore na barra lateral e voltar com Esc (AsideArea).
class TenantsController < ApplicationController
  include AsideArea

  helper_method :pending_invites

  def index
  end

  # A lista com o formulário do tenant novo aberto: é para onde o New tenant do seletor e da paleta leva.
  def new
    @open_new_tenant = true
    render :index
  end

  # Tenant criado já fica ativo e abre as notas dele.
  def create
    @name = params.expect(tenant: [ :name ])[:name]
    tenant = Tenant.create(credential.api, @name)
    credential.tenant_id = tenant.id
    redirect_to root_path, notice: t(".created", name: tenant.name)
  rescue MarkuppApi::InvalidRequest
    @name_error = t(".invalid_name")
    @open_new_tenant = true
    render :index, status: :unprocessable_content
  end

  private
    def aside_area_path = "/tenants"

    def pending_invites = @pending_invites ||= PendingInvite.mine(credential.api)
end
