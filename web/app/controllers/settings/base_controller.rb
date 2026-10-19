# Configurações unificadas, como no VS Code e no Obsidian: Conta e Aparência para todos, Membros e Convites
# para owner e admin. Aqui o tenant é opcional: sem ele (onboarding) a navegação mostra só Conta e Aparência.
module Settings
  class BaseController < ApplicationController
    layout "settings"

    before_action :pick_tenant, :remember_settings_origin
    helper_method :settings_section, :sidebar_note_tree, :settings_origin_path

    private
      def pick_tenant
        @current_tenant = Tenant.pick(current_tenants, credential.tenant_id)
      end

      # Página de onde as configurações foram abertas, para o voltar. Trocar de seção não muda a origem, e só vale
      # caminho do próprio painel (referer de outro site cai na raiz, sem redirecionar para fora).
      def remember_settings_origin
        origin = same_site_path(request.referer)
        session[:settings_origin] = origin if origin && !origin.start_with?("/settings")
      end

      def settings_origin_path = session[:settings_origin] || root_path

      def same_site_path(url)
        uri = URI.parse(url.to_s)
        return unless uri.host == request.host && uri.path.start_with?("/") && !uri.path.start_with?("//")

        [ uri.path, uri.query ].compact.join("?")
      rescue URI::InvalidURIError
        nil
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
