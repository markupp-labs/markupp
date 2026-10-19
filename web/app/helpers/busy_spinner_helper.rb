# Indicador de carregamento em braille (busy_spinner_controller.js).
module BusySpinnerHelper
  def busy_spinner(busy_when: nil, class: nil)
    tag.span
  end

  def page_busy_spinner = busy_spinner
end
