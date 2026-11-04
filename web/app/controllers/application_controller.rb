class ApplicationController < ActionController::Base
  # Only allow modern browsers supporting webp images, web push, badges, import maps, CSS nesting, and CSS :has.
  allow_browser versions: :modern

  # Changes to the importmap will invalidate the etag for HTML responses
  stale_when_importmap_changes

  # Raiz de composição: o initializer injeta a fábrica do cliente da API; os testes trocam pela fake.
  class_attribute :markupp_api_factory

  before_action :require_login
  helper_method :credential, :current_tenant, :current_tenants, :current_vault, :current_vaults, :explorer_groups

  # O Rails procura do último para o primeiro: os específicos vêm depois do genérico.
  rescue_from MarkuppApi::Error, with: :render_unreachable
  rescue_from MarkuppApi::Forbidden, with: :refuse_forbidden
  rescue_from MarkuppApi::TenantUnavailable, with: :forget_tenant
  rescue_from MarkuppApi::Unauthorized, MarkuppApi::SessionExpired, with: :expire_session

  # rate_limit do create com a mensagem que cita a janela; quem passa do limite volta para redirect_path.
  def self.limit_create_attempts(to:, within:, redirect_path:)
    rate_limit to:, within:, only: :create,
               with: -> { redirect_to send(redirect_path), alert: t("rate_limited", count: self.class.retry_minutes(within)) }
  end

  # Minutos da janela para a mensagem de limite, arredondados para cima: 90s viram 2, nunca 0.
  def self.retry_minutes(within) = within.in_minutes.ceil

  private
    def credential = @credential ||= PanelCredential.new(session, markupp_api_factory)

    # Só GET de página inteira volta depois do login: formulário enviado sem sessão se perde, e frame sozinho ou json
    # (a busca do ![[nota]] em segundo plano) não é página.
    def require_login
      return if credential.signed_in?
      return head :unauthorized if request.format.json?
      return render_frame_alert(new_session_path, t("markupp_api.errors.session_expired"), status: :unauthorized) if turbo_frame_request?

      session[:return_to] = request.fullpath if request.get? && request.format.html?
      redirect_to new_session_path
    end

    # Sessão nova contra fixação, a partir do par de tokens do login ou do aceite de convite; devolve a página pedida antes.
    def start_session(pair)
      return_to = panel_path(session[:return_to]) || root_path
      reset_session
      credential.sign_in(pair)
      credential.account = credential.api.get("/auth/me")
      return_to
    end

    # Só caminho do próprio painel: //host e /\host o navegador lê como outro site.
    def panel_path(path) = (path if path.to_s.match?(%r{\A/(?![/\\])}))

    # Logout de verdade: o refresh é revogado na API antes de a sessão do Rails sumir.
    def end_session
      revoke_refresh_token
      credential.sign_out
    end

    # A API responde 204 sempre; fora do ar, o painel sai do mesmo jeito e o refresh vence sozinho.
    def revoke_refresh_token
      credential.public_api.post("/auth/logout", refresh_token: credential.refresh_token)
    rescue MarkuppApi::Error => error
      log_api_event("markupp_logout_not_revoked", error, level: :warn)
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

    # Cofres do tenant ativo, pedidos uma vez por página. nil quando a listagem cai (API fora): a falha fica guardada,
    # o layout mostra um aviso curto na trilha e não pede de novo (dentro do render_unreachable, viraria um 500).
    def current_vaults
      return @current_vaults if defined?(@current_vaults)

      @current_vaults = Vault.mine(credential.api)
    rescue MarkuppApi::Error => error
      raise unless error.instance_of?(MarkuppApi::Error)

      log_api_event("markupp_vaults_unreachable", error, level: :error)
      @vaults_failure = error
      @current_vaults = nil
    end

    # Para quem não segue sem a lista: a falha guardada sobe para o render_unreachable da página.
    def reachable_vaults = current_vaults || raise(@vaults_failure)

    # Cofre ativo do tenant ativo, a cada página como o tenant: o da sessão enquanto a conta for membro dele, senão o
    # primeiro, que fica gravado; nil quando a conta não tem cofre no tenant ou a lista não veio (a sessão o mantém).
    def current_vault
      return @current_vault if defined?(@current_vault)
      return unless current_vaults

      @current_vault = Vault.pick(current_vaults, credential.vault_id)
      credential.vault_id = @current_vault&.id
      @current_vault
    end

    # O cofre ativo para quem age nele: com a lista fora, a falha sobe em vez de parecer que a conta não tem cofre.
    def reachable_vault = reachable_vaults && current_vault

    # Nas páginas que agem no cofre ativo: sem ele, a tela padrão mostra o estado vazio, e o aviso segue o papel como
    # ele (vaults/_empty): quem escreve cria um cofre, o viewer pede a um membro.
    def require_vault
      return if reachable_vault

      redirect_to root_path, alert: t(current_tenant.writer? ? "markupp_api.errors.no_vault" : "vaults.empty.viewer_hint")
    end

    # Árvore da barra lateral, das notas do cofre ativo: as páginas de notas e as configurações desenham a mesma.
    def load_note_tree = Note::Tree.build(reachable_vault ? Note.every_in_vault(credential.api, current_vault.id) : [])

    # Notas de uma visão plana do Explorer, no cofre ativo. nil quando a API não responde ou recusa o cofre: a barra
    # mostra a falha no lugar da lista e a página segue.
    def explorer_groups(mode)
      current_vault ? ExplorerListing.load(credential.api, current_vault.id, mode) : []
    rescue MarkuppApi::Error => error
      raise unless error.instance_of?(MarkuppApi::Error) || error.is_a?(MarkuppApi::VaultUnavailable)

      log_api_event("markupp_explorer_unreachable", error, level: :error, mode:)
      nil
    end

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

    # Json não segue redirect para página: só o status, e o aviso fica para a próxima página de verdade.
    def leave_page(path, alert, frame_status:)
      return head frame_status if request.format.json?
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

    # Cada controller tem a sua view unreachable: página inteira nas notas, aviso dentro da paleta na busca. Json só o status.
    def render_unreachable(error)
      log_api_event("markupp_api_unreachable", error, level: :error)
      return head :bad_gateway if request.format.json?

      render :unreachable, status: :bad_gateway
    end
end
