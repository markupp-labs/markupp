# Seções do cofre ativo, no grupo Vault das configurações: pedem tenant e cofre. Quem pode mudar o quê decide a API
# (viewer só lê e sai); o painel esconde as ações que ela recusaria.
module Settings
  class VaultBaseController < BaseController
    skip_before_action :pick_tenant
    before_action :require_tenant, :require_vault
  end
end
