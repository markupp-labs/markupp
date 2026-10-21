# Troca de senha de quem está logado: a API só troca pelo token do email, então pede o email de redefinição
# para a própria conta, pela mesma rota do esqueci a senha. O token entra na tela de redefinir senha.
class PasswordChangesController < ApplicationController
  limit_create_attempts to: 5, within: 1.minute, redirect_path: :settings_account_path

  def create
    credential.public_api.post("/auth/password/forgot", email: credential.account_email)
    redirect_to settings_account_path, notice: t(".sent", email: credential.account_email)
  rescue MarkuppApi::EmailUnavailable, MarkuppApi::LocalAuthDisabled => error
    redirect_to settings_account_path, alert: t("password_requests.create.#{error.reason}")
  end
end
