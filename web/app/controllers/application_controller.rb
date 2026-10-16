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

    # Sessão nova contra fixação, a partir do par de tokens do login ou do aceite de convite; devolve a página pedida antes.
    def start_session(pair)
      return_to = session[:return_to] || root_path
      reset_session
      credential.sign_in(pair)
      credential.account = credential.api.get("/auth/me")
      return_to
    end

    # Só nas páginas que agem num tenant: o papel vem da lista, sem cache, a cada página.
    def require_tenant
      @current_tenant = Tenant.pick(current_tenants, credential.tenant_id)
      return leave_for_tenant_choice unless @current_tenant

      credential.tenant_id = @current_tenant.id
    end

    # Sem tenant a página manda escolher; o frame da busca não tem como mostrar a escolha, então avisa.
    def leave_for_tenant_choice
      return redirect_to(tenants_path) unless turbo_frame_request?

      render_frame_alert(tenants_path, t("markupp_api.errors.tenant_unavailable"), status: :forbidden)
    end

    def current_tenant = @current_tenant

    def current_tenants = @current_tenants ||= Tenant.mine(credential.api)

    # O tenant some entre a lista e a chamada de notas (vínculo removido no meio): esquece e manda escolher outro.
    def forget_tenant(error)
      log_api_event("markupp_tenant_unavailable", error, tenant_id: credential.tenant_id)
      credential.tenant_id = nil
      leave_page(tenants_path, t("markupp_api.errors.tenant_unavailable"), frame_status: :forbidden)
    end

    # Papel mudou no meio (a página ainda oferecia a ação): volta à página de onde veio, que recarrega com o papel novo.
    def refuse_forbidden(error)
      log_api_event("markupp_api_forbidden", error)
      alert = t("markupp_api.errors.#{error.reason}", default: :"markupp_api.errors.forbidden")
      return render_frame_alert(root_path, alert, status: :forbidden) if turbo_frame_request?

      redirect_back_or_to root_path, alert:
    end

    # 401 que sobra depois da renovação também encerra: a sessão não vale mais.
    def expire_session(error)
      log_api_event("markupp_session_expired", error)
      credential.sign_out
      leave_page(new_session_path, t("markupp_api.errors.session_expired"), frame_status: :unauthorized)
    end

    def leave_page(path, alert, frame_status:)
      return render_frame_alert(path, alert, status: frame_status) if turbo_frame_request?

      redirect_to path, alert:
    end

    # Num turbo frame (a busca, os convites do seletor) o redirect cairia numa página sem o frame e o Turbo mostraria
    # "Content missing": o aviso fica dentro do frame, com um link que sai dele.
    def render_frame_alert(path, alert, status:)
      render partial: "application/frame_alert", locals: { path:, alert: }, status:
    end

    # Log estruturado de uma resposta da API que o painel tratou.
    def log_api_event(event, error, level: :info, **details)
      Rails.logger.public_send(level, { event:, **details, message: error.message }.to_json)
    end

    # Cada controller tem a sua view unreachable: página inteira nas notas, aviso dentro da paleta na busca.
    def render_unreachable(error)
      log_api_event("markupp_api_unreachable", error, level: :error)
      render :unreachable, status: :bad_gateway
    end
end
