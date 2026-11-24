# Criar conta com email e senha. A conta nova entra logada; sem tenant ainda, o painel leva à lista de tenants.
class RegistrationsController < ApplicationController
  include LoginProviders

  layout "public"

  skip_before_action :require_login
  before_action :require_open_registration, only: :new
  limit_create_attempts to: 10, within: 3.minutes, redirect_path: :new_registration_path

  def new
    @registration = AccountRegistration.new
  end

  def create
    @registration = AccountRegistration.new(params.expect(registration: [ :email, :password, :password_confirmation ]))
    pair = @registration.submit(credential.public_api)
    return render(:new, status: :unprocessable_content) unless pair

    redirect_to start_session(pair)
  end

  private
    # Sem método aberto, só o convite cria conta; o alerta de quem chegou recusado pelo provedor segue até o login.
    def require_open_registration
      return if registration_open?

      flash.keep
      redirect_to new_session_path, alert: flash[:alert] || t("activemodel.errors.models.account_registration.attributes.base.registration_disabled")
    end
end
