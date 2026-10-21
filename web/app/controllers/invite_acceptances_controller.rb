# Aceite público de convite: o token do email e uma senha criam a conta, que entra logada no tenant do convite.
# O ?token= preenche o campo, para quando o servidor mandar o link.
class InviteAcceptancesController < ApplicationController
  layout "public"

  skip_before_action :require_login

  def new
    @form = TokenForm.new(token: params[:token])
  end

  def create
    @form = TokenForm.new(params.expect(invite_acceptance: [ :token, :password ]))
    pair = @form.submit(credential.public_api, "/auth/invites/accept")
    return render(:new, status: :unprocessable_content) unless pair

    redirect_to start_session(pair)
  end
end
