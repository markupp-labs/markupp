# Provedores externos de login do servidor, pedidos uma vez por página, para os botões e para o nome nas mensagens, e o
# que o cadastro aceita agora (ADR-0059), para a tela de cadastro e o link para ela só aparecerem com algo aberto.
module LoginProviders
  extend ActiveSupport::Concern

  included do
    helper_method :login_providers, :signup_providers, :account_policy, :registration_open?
  end

  private
    def login_providers = @login_providers ||= LoginProvider.all(credential.public_api)

    def signup_providers = login_providers.select(&:registration_open)

    def account_policy = @account_policy ||= AccountPolicy.fetch(credential.public_api)

    def registration_open? = account_policy.password_registration_open || signup_providers.any?

    # Nome do provedor para quem lê; provedor que saiu da configuração aparece pelo id.
    def login_provider_name(id) = login_providers.find { it.id == id }&.name || id
end
