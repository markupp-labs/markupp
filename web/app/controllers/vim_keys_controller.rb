# Modo Vim do editor (editor/vim_mode.js), ligado ou desligado em Configuracoes > Aparencia > Editor.
class VimKeysController < ApplicationController
  CHOICES = %w[on off].freeze

  def update
    choice = params[:vim_keys]
    return render(plain: "Unknown vim keys choice #{choice.inspect}, expected on or off", status: :bad_request) unless CHOICES.include?(choice)

    cookies.permanent[:vim_keys] = choice
    redirect_back_or_to settings_appearance_path
  end
end
