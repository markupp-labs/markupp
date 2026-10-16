class ApplicationController < ActionController::Base
  # Only allow modern browsers supporting webp images, web push, badges, import maps, CSS nesting, and CSS :has.
  allow_browser versions: :modern

  # Changes to the importmap will invalidate the etag for HTML responses
  stale_when_importmap_changes

  # Raiz de composição: o initializer injeta a fábrica do cliente da API; os testes trocam pela fake.
  class_attribute :markupp_api_factory

  before_action :require_login
  helper_method :credential, :current_tenant, :current_tenants

  # O Rails procura do último para o primeiro: os específicos vêm depois do genérico.
  rescue_from MarkuppApi::Error, with: :render_unreachable
  rescue_from MarkuppApi::Forbidden, with: :refuse_forbidden
  rescue_from MarkuppApi::TenantUnavailable, with: :forget_tenant
  rescue_from MarkuppApi::Unauthorized, MarkuppApi::SessionExpired, with: :expire_session

  private
    def credential = @credential ||= PanelCredential.new(session, markupp_api_factory)

    # Só GET volta depois do login: formulário enviado sem sessão se perde.
    def require_login
      return if credential.signed_in?

      session[:return_to] = request.fullpath if request.get?
      redirect_to new_session_path
    end

    # Só nas páginas que agem num tenant: o papel vem da lista, sem cache, a cada página.
    def require_tenant
      @current_tenant = Tenant.pick(current_tenants, credential.tenant_id)
      return redirect_to(tenants_path) unless @current_tenant

      credential.tenant_id = @current_tenant.id
    end

    def current_tenant = @current_tenant

    def current_tenants = @current_tenants ||= Tenant.mine(credential.api)

    # O tenant some entre a lista e a chamada de notas (vínculo removido no meio): esquece e manda escolher outro.
    def forget_tenant(error)
      Rails.logger.info({ event: "markupp_tenant_unavailable", tenant_id: credential.tenant_id, message: error.message }.to_json)
      credential.tenant_id = nil
      redirect_to tenants_path, alert: t("markupp_api.errors.tenant_unavailable")
    end

    # Papel mudou no meio (a página ainda oferecia a ação): volta à página de onde veio, que recarrega com o papel novo.
    def refuse_forbidden(error)
      Rails.logger.info({ event: "markupp_api_forbidden", message: error.message }.to_json)
      reason = error.class.name.demodulize.underscore
      redirect_back_or_to root_path, alert: t("markupp_api.errors.#{reason}", default: :"markupp_api.errors.forbidden")
    end

    # 401 que sobra depois da renovação também encerra: a sessão não vale mais.
    def expire_session(error)
      Rails.logger.info({ event: "markupp_session_expired", message: error.message }.to_json)
      credential.sign_out
      redirect_to new_session_path, alert: t("markupp_api.errors.session_expired")
    end

    # Cada controller tem a sua view unreachable: página inteira nas notas, aviso dentro da paleta na busca.
    def render_unreachable(error)
      Rails.logger.error({ event: "markupp_api_unreachable", message: error.message }.to_json)
      render :unreachable, status: :bad_gateway
    end
end
