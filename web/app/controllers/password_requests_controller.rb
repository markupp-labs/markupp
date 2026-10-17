# Esqueci a senha: a API manda o token por email. A resposta é a mesma exista a conta ou não.
class PasswordRequestsController < ApplicationController
  layout "public"

  skip_before_action :require_login
  limit_create_attempts to: 5, within: 1.minute, redirect_path: :new_password_request_path

  def new
  end

  def create
    @email = params.expect(password_request: [ :email ])[:email]
    credential.public_api.post("/auth/password/forgot", email: @email)
    redirect_to new_password_request_path, notice: t(".sent")
  rescue MarkuppApi::InvalidRequest
    @email_error = t(".invalid_email")
    render :new, status: :unprocessable_content
  rescue MarkuppApi::EmailUnavailable, MarkuppApi::LocalAuthDisabled => error
    flash.now[:alert] = t(".#{error.reason}")
    render :new, status: :unprocessable_content
  end
end
