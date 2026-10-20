# Configurações unificadas, como no VS Code e no Obsidian: Conta, Aparência e Editor para todos, Membros e Convites
# para owner e admin. Aqui o tenant é opcional: sem ele (onboarding) a navegação mostra só as seções de todos.
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
