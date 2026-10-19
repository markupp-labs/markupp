# Troca de senha com o token do email. O ?token= preenche o campo, para quando o servidor mandar o link.
# Quem troca estando logado (vindo do Change password da Conta) sai do painel, como no logout, e entra com a senha nova.
class PasswordResetsController < ApplicationController
  layout "public"

  skip_before_action :require_login

  def new
    @form = TokenForm.new(token: params[:token])
  end

  def create
    @form = TokenForm.new(params.expect(password_reset: [ :token, :password ]))
    return render(:new, status: :unprocessable_content) unless @form.submit(credential.public_api, "/auth/password/reset")

    redirect_to new_session_path, notice: t(leave_panel_session)
  end

  private
    # Devolve a chave do aviso: logado, a sessão termina e o aviso pede o login de novo.
    def leave_panel_session
      return ".changed" unless credential.signed_in?

      end_session
      ".changed_signed_out"
    end
end
