# Configurações unificadas, como no VS Code e no Obsidian: Conta e Aparência para todos, Membros e Convites
# para owner e admin. Aqui o tenant é opcional: sem ele (onboarding) a navegação mostra só Conta e Aparência.
module Settings
  class BaseController < ApplicationController
    layout "settings"

    before_action :pick_tenant
    helper_method :settings_section

    private
      def pick_tenant
        @current_tenant = Tenant.pick(current_tenants, credential.tenant_id)
      end

      # Seção marcada na navegação: o nome do controller, que é também a chave do rótulo.
      def settings_section = controller_name
  end
end
