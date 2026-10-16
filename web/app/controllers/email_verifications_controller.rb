# Pede à API o email com o token de confirmação. Email já confirmado não é erro para quem pede.
class EmailVerificationsController < ApplicationController
  rate_limit to: 5, within: 1.minute, only: :create, with: -> { redirect_to account_path, alert: t("rate_limited") }

  def create
    credential.api.post("/auth/email/verification", {})
    redirect_to account_path, notice: t(".sent")
  rescue MarkuppApi::EmailAlreadyVerified
    redirect_to account_path, notice: t("accounts.already_confirmed")
  rescue MarkuppApi::EmailUnavailable
    redirect_to account_path, alert: t(".email_unavailable")
  end
end
