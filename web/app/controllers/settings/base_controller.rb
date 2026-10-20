# Configurações unificadas, como no VS Code e no Obsidian: Conta e Aparência para todos, Membros e Convites
# para owner e admin. Aqui o tenant é opcional: sem ele (onboarding) a navegação mostra só Conta e Aparência.
module Settings
  class BaseController < ApplicationController
    include AsideArea

    layout "settings"

    helper_method :settings_section

    private
      def aside_area_path = "/settings"

      # Seção marcada na navegação: o nome do controller, que é também a chave do rótulo.
      def settings_section = controller_name
  end
end
