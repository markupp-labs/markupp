# Cada requisição roda no idioma escolhido nas configurações (cookie), senão no do Accept-Language do navegador, senão no
# padrão. O with_locale devolve o idioma anterior no fim, então ele não vaza para a próxima requisição da mesma thread.
module Localized
  extend ActiveSupport::Concern

  included do
    around_action :switch_locale
    helper_method :chosen_locale
  end

  private
    def switch_locale(&action) = I18n.with_locale(chosen_locale || browser_locale || I18n.default_locale, &action)

    # Idioma gravado no cookie, ou nil quando o painel segue o navegador.
    def chosen_locale = cookies[:locale].presence_in(I18n.available_locales.map(&:to_s))

    # pt-PT e pt também caem no pt-BR: a região só desempata quando o painel fala mais de uma.
    def browser_locale = http_accept_language.language_region_compatible_from(I18n.available_locales)
end
