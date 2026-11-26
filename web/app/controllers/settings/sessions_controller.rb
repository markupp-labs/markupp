# Encerrar uma sessão de login da conta, que a seção Conta lista (ADR-0061). O aparelho dela sai em minutos, quando o
# token de acesso vence e a renovação é recusada.
module Settings
  class SessionsController < BaseController
    def destroy
      AccountSession.end(credential.api, params[:id])
      redirect_to settings_account_path, notice: t(".ended")
    rescue MarkuppApi::SessionNotFound
      redirect_to settings_account_path, alert: t(".not_found")
    end
  end
end
