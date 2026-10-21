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

  # Só o owner apaga (a API recusa os outros papéis). Apagado, a sessão esquece o tenant e as origens do voltar, que
  # apontavam para páginas dele, e a lista de tenants manda escolher outro.
  # ponytail: destino fixo na lista de tenants; outro destino (o tenant que sobrar, por exemplo) se o usuário pedir.
  def destroy
    tenant = current_tenants.find { it.id == params[:id] } or raise MarkuppApi::TenantNotFound, "tenant #{params[:id].inspect} is not in /me/tenants"
    Tenant.delete(credential.api, tenant.id)
    leave_deleted_tenant(notice: t(".deleted", name: tenant.name))
  rescue MarkuppApi::TenantNotFound => error
    log_api_event("markupp_tenant_gone", error, tenant_id: params[:id])
    leave_deleted_tenant(alert: t(".gone"))
  end

  private
    def aside_area_path = "/tenants"

    def leave_deleted_tenant(**flash)
      credential.tenant_id = nil if credential.tenant_id == params[:id]
      AREA_PATHS.each { session.delete(origin_key_of(it)) }
      redirect_to tenants_path, flash:
    end

    def pending_invites = @pending_invites ||= PendingInvite.mine(credential.api)
end
