# Seção Conta: o email e a confirmação dele, a senha, as contas conectadas e as sessões de login. Não pede tenant, porque
# quem confirma o email ainda pode não ter um.
module Settings
  class AccountsController < BaseController
    include LoginProviders

    def show
      @identities = login_providers.any? ? LinkedIdentity.mine(credential.api) : []
      @sessions = AccountSession.mine(credential.api)
    end
  end
end
