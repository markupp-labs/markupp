class ThemesController < ApplicationController
  THEMES = %w[light dark].freeze

  def update
    theme = params[:theme]
    return render(plain: "Unknown theme #{theme.inspect}, expected light or dark", status: :bad_request) unless THEMES.include?(theme)

    cookies.permanent[:theme] = theme
    redirect_back_or_to root_path
  end
end
