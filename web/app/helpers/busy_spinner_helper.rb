# Indicador de carregamento em braille (busy_spinner_controller.js): um role=status vazio que o controller preenche com o
# glifo e o rótulo só quando a espera passa de 300ms. busy_when é o seletor que diz que há espera (o Turbo marca com
# aria-busy o form no envio e o turbo-frame que carrega); sem ele, o indicador existir já é a espera.
module BusySpinnerHelper
  def busy_spinner(busy_when: nil, class: nil)
    tag.span(role: "status", class: [ "busy-spinner", binding.local_variable_get(:class) ],
             data: { controller: "busy-spinner", busy_spinner_busy_when_value: busy_when, busy_spinner_label_value: t("busy_spinner.label") })
  end

  # Molde da espera no lugar da ação (motion/busy_origin.js): o link ou botão que começou a visita ou o envio recebe uma
  # cópia no fim dele. Só o glifo; o rótulo fica para o leitor de tela.
  def busy_origin_spinner_template
    tag.template(busy_spinner(class: "busy-spinner-quiet busy-spinner-origin"), id: "busy_origin_spinner")
  end
end
