# Provedores externos de login do servidor, pedidos uma vez por página, para os botões e para o nome nas mensagens.
module LoginProviders
  extend ActiveSupport::Concern

  included do
    helper_method :login_providers
  end

  private
    def login_providers = @login_providers ||= LoginProvider.all(credential.public_api)

    # Nome do provedor para quem lê; provedor que saiu da configuração aparece pelo id.
    def login_provider_name(id) = login_providers.find { it.id == id }&.name || id
end
