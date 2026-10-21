# Cada requisição roda no idioma do Accept-Language do navegador, ou no padrão se o painel não fala nenhum dos pedidos.
# O with_locale devolve o idioma anterior no fim, então ele não vaza para a próxima requisição da mesma thread.
module Localized
  extend ActiveSupport::Concern

  included { around_action :switch_locale }

  private
    def switch_locale(&action) = I18n.with_locale(browser_locale || I18n.default_locale, &action)

    # pt-PT e pt também caem no pt-BR: a região só desempata quando o painel fala mais de uma.
    def browser_locale = http_accept_language.language_region_compatible_from(I18n.available_locales)
end
