# Estilo do editor: plain mostra o markdown como no VS Code, styled como o source do Obsidian (títulos maiores).
class EditorStylesController < ApplicationController
  STYLES = %w[plain styled].freeze

  def update
    style = params[:editor_style]
    return render(plain: "Unknown editor style #{style.inspect}, expected plain or styled", status: :bad_request) unless STYLES.include?(style)

    cookies.permanent[:editor_style] = style
    redirect_back_or_to settings_appearance_path
  end
end
