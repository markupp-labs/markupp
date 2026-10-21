# Idioma escolhido nas configurações ou na paleta: o cookie vale mais que o Accept-Language (Localized).
class LocalesController < ApplicationController
  LOCALES = I18n.available_locales.map(&:to_s).freeze

  def update
    locale = params[:locale]
    return render(plain: "Unknown locale #{locale.inspect}, expected #{LOCALES.join(" or ")}", status: :bad_request) unless LOCALES.include?(locale)

    cookies.permanent[:locale] = locale
    redirect_back_or_to settings_appearance_path
  end
end
