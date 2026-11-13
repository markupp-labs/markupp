# Login, cadastro e vínculo por provedor externo (ADR-0054). O create pede a tentativa ao servidor e manda o navegador
# ao provedor; o callback confere o state guardado na sessão e repassa o código ao servidor, que tem o segredo do
# provedor. O login nunca cria conta, e o vínculo (purpose link) só sai de uma sessão ativa.
class ExternalLoginsController < ApplicationController
  include LoginProviders

  skip_before_action :require_login
  limit_create_attempts to: 10, within: 3.minutes, redirect_path: :new_session_path

  # Cada recusa do servidor: a mensagem e para onde a pessoa volta (:back é a página de onde a tentativa saiu).
  REFUSALS = {
    MarkuppApi::IdentityNotLinked => [ "external_logins.errors.not_linked", :new_registration_path ],
    MarkuppApi::AccountExists => [ "external_logins.errors.account_exists", :new_session_path ],
    MarkuppApi::IdentityAlreadyLinked => [ "external_logins.errors.already_linked", :back ],
    MarkuppApi::RegistrationDisabled => [ "activemodel.errors.models.account_registration.attributes.base.registration_disabled", :new_registration_path ],
    MarkuppApi::ProviderEmailMissing => [ "external_logins.errors.email_missing", :new_registration_path ],
    MarkuppApi::LoginRejected => [ "external_logins.errors.rejected", :back ],
    MarkuppApi::ProviderUnavailable => [ "external_logins.errors.unavailable", :back ]
  }.freeze
  # Página de onde cada propósito sai, e para onde volta quando algo dá errado.
  BACK_PATHS = { "login" => :new_session_path, "signup" => :new_registration_path, "link" => :settings_account_path }.freeze

  # Só provedor que o servidor lista e propósito conhecido; vincular pede sessão.
  def create
    purpose = ExternalLoginAttempt::PURPOSES.find { it == params[:purpose] }
    provider = login_providers.find { it.id == params[:provider] }
    return redirect_to(new_session_path) unless purpose && provider
    return redirect_to(new_session_path) if purpose == "link" && !credential.signed_in?

    redirect_to ExternalLoginAttempt.start(session, credential.public_api, provider: provider.id, purpose:), allow_other_host: true
  rescue *REFUSALS.keys => error
    refuse(error, purpose)
  end

  def callback
    attempt = ExternalLoginAttempt.take(session, provider: params[:provider], state: params[:state])
    return redirect_to(new_session_path, alert: t("external_logins.errors.expired")) unless attempt
    return leave(attempt.purpose, alert: t("external_logins.errors.cancelled", provider: provider_name)) if params[:error].present?

    finish(attempt)
  rescue *REFUSALS.keys => error
    refuse(error, attempt.purpose)
  end

  private
    def finish(attempt)
      body = attempt.callback_body(params[:code])
      case attempt.purpose
      when "login" then redirect_to start_session(credential.public_api.post(MarkuppApi.path("v1", "auth", "external-login"), body))
      when "signup" then redirect_to start_session(credential.public_api.post(MarkuppApi.path("v1", "accounts", "external"), body))
      else link(body)
      end
    end

    # Vincular pede a sessão de quem começou: sem ela, o código não serve a conta nenhuma.
    def link(body)
      return redirect_to(new_session_path) unless credential.signed_in?

      credential.api.post(MarkuppApi.path("v1", "me", "identities"), body)
      redirect_to settings_account_path, notice: t("external_logins.connected", provider: provider_name)
    end

    def refuse(error, purpose)
      key, path = REFUSALS.fetch(error.class)
      log_api_event("markupp_external_login_refused", error, provider: params[:provider], purpose:)
      return leave(purpose, alert: t(key, provider: provider_name)) if path == :back

      redirect_to send(path), alert: t(key, provider: provider_name)
    end

    def leave(purpose, alert:) = redirect_to(send(BACK_PATHS.fetch(purpose, :new_session_path)), alert:)

    def provider_name = login_provider_name(params[:provider])
end
