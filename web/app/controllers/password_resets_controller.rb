# Troca de senha com o token do email. O ?token= preenche o campo, para quando o servidor mandar o link.
class PasswordResetsController < ApplicationController
  layout "public"

  skip_before_action :require_login

  def new
    @form = TokenForm.new(token: params[:token])
  end

  def create
    @form = TokenForm.new(params.expect(password_reset: [ :token, :password ]))
    return render(:new, status: :unprocessable_content) unless @form.submit(credential.public_api, "/auth/password/reset")

    redirect_to new_session_path, notice: t(".changed")
  end
end
