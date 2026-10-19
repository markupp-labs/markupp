# Confirma o email com o token, só logado: a API só aceita o token da conta que o pediu.
# O ?token= preenche o campo, para quando o servidor mandar o link. Fica nas configurações, sob Conta,
# mas a rota é a de antes: é ela que vai no link do email.
class EmailConfirmationsController < Settings::BaseController
  def new
    @form = TokenForm.new(token: params[:token])
  end

  def create
    @form = TokenForm.new(params.expect(email_confirmation: [ :token ]))
    return render(:new, status: :unprocessable_content) unless @form.submit(credential.api, "/auth/email/verify")

    redirect_to settings_account_path, notice: t(".confirmed")
  rescue MarkuppApi::EmailAlreadyVerified
    redirect_to settings_account_path, notice: t("settings.accounts.already_confirmed")
  end

  private
    def settings_section = "accounts"
end
