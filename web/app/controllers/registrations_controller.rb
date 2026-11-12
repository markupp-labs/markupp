# Criar conta com email e senha. A conta nova entra logada; sem tenant ainda, o painel leva à lista de tenants.
class RegistrationsController < ApplicationController
  layout "public"

  skip_before_action :require_login
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
end
