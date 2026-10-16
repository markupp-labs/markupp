# Confirma o email com o token, só logado: a API só aceita o token da conta que o pediu.
# O ?token= preenche o campo, para quando o servidor mandar o link.
class EmailConfirmationsController < ApplicationController
  def new
    @form = TokenForm.new(token: params[:token])
  end

  def create
    @form = TokenForm.new(params.expect(email_confirmation: [ :token ]))
    return render(:new, status: :unprocessable_content) unless @form.submit(credential.api, "/auth/email/verify")

    redirect_to account_path, notice: t(".confirmed")
  rescue MarkuppApi::EmailAlreadyVerified
    redirect_to account_path, notice: t("accounts.already_confirmed")
  end
end
