# Desconectar um provedor externo da conta, em Configurações > Conta. Conectar passa pelo ExternalLoginsController.
module Settings
  class IdentitiesController < BaseController
    include LoginProviders

    # O nome do provedor vem da identidade da conta, não do formulário.
    def destroy
      identity = LinkedIdentity.mine(credential.api).find { it.id == params[:id] }
      return redirect_to(settings_account_path) unless identity

      LinkedIdentity.unlink(credential.api, identity.id)
      redirect_to settings_account_path, notice: t(".disconnected", provider: login_provider_name(identity.provider))
    rescue MarkuppApi::LastIdentity
      redirect_to settings_account_path, alert: t(".last_identity")
    end
  end
end
