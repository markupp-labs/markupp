# Seção Autorizações: os clientes que a conta autorizou a agir em nome dela, cada um com o seu Revoke. É da conta, não
# pede tenant.
module Settings
  class AuthorizationsController < BaseController
    helper_method :oauth_grants

    def index
    end

    def destroy
      OAuthGrant.revoke(credential.api, params[:id])
      redirect_to settings_authorizations_path, notice: t(".revoked")
    rescue MarkuppApi::AuthorizationNotFound
      redirect_to settings_authorizations_path, alert: t(".not_found")
    end

    private
      def oauth_grants = @oauth_grants ||= OAuthGrant.all(credential.api)
  end
end
