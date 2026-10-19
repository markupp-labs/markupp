# Pede à API o email com o token de confirmação. Email já confirmado não é erro para quem pede.
class EmailVerificationsController < ApplicationController
  limit_create_attempts to: 5, within: 1.minute, redirect_path: :settings_account_path

  def create
    credential.api.post("/auth/email/verification", {})
    redirect_to settings_account_path, notice: t(".sent")
  rescue MarkuppApi::EmailAlreadyVerified
    redirect_to settings_account_path, notice: t("settings.accounts.already_confirmed")
  rescue MarkuppApi::EmailUnavailable
    redirect_to settings_account_path, alert: t(".email_unavailable")
  end
end
