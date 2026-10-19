# Indicador de carregamento em braille (busy_spinner_controller.js): um role=status vazio que o controller preenche com o
# glifo e o rótulo só quando a espera passa de 300ms. busy_when é o seletor que diz que há espera (o Turbo marca com
# aria-busy o <html> na visita, o form no envio e o turbo-frame que carrega); sem ele, o indicador existir já é a espera.
module BusySpinnerHelper
  # Visita ou envio de form que troca a página; o form que mira um frame tem o indicador do próprio frame.
  PAGE_BUSY_SELECTOR = "html[aria-busy], form[aria-busy]:not([data-turbo-frame])".freeze

  def busy_spinner(busy_when: nil, class: nil)
    tag.span(role: "status", class: [ "busy-spinner", binding.local_variable_get(:class) ],
             data: { controller: "busy-spinner", busy_spinner_busy_when_value: busy_when, busy_spinner_label_value: t("busy_spinner.label") })
  end

  # No topo, no lugar da barra de progresso do Turbo: só o glifo, o rótulo fica para o leitor de tela.
  def page_busy_spinner = busy_spinner(busy_when: PAGE_BUSY_SELECTOR, class: "busy-spinner-page busy-spinner-quiet")
end
