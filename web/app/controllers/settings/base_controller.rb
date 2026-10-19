# Configurações unificadas, como no VS Code e no Obsidian: Conta e Aparência para todos, Membros e Convites
# para owner e admin. Aqui o tenant é opcional: sem ele (onboarding) a navegação mostra só Conta e Aparência.
module Settings
  class BaseController < ApplicationController
    layout "settings"

    before_action :pick_tenant
    helper_method :settings_section, :sidebar_note_tree

    private
      def pick_tenant
        @current_tenant = Tenant.pick(current_tenants, credential.tenant_id)
      end

      # Seção marcada na navegação: o nome do controller, que é também a chave do rótulo.
      def settings_section = controller_name

      # A mesma árvore das notas na barra lateral. Se só a listagem cair (API fora), a barra mostra um aviso curto e as
      # configurações seguem; sessão vencida e tenant perdido continuam com o tratamento de sempre.
      def sidebar_note_tree
        load_note_tree
      rescue MarkuppApi::Error => error
        raise unless error.instance_of?(MarkuppApi::Error)

        log_api_event("markupp_note_tree_unreachable", error, level: :error)
        nil
      end
  end
end
