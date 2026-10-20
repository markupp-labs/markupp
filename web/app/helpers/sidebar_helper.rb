# Barra lateral no desktop: aberta ou fechada e a largura moram em cookies gravados pelo sidebar_controller.js,
# lidos aqui para o servidor já desenhar o estado certo (sem piscar, como o tema).
module SidebarHelper
  SIDEBAR_WIDTH_RANGE = 240..480
  SIDEBAR_DEFAULT_WIDTH = 288
  # Visões da barra lateral, na ordem da barra de atividades; a primeira é a padrão.
  SIDEBAR_VIEWS = %w[explorer links graph search].freeze

  def current_sidebar_view = SIDEBAR_VIEWS.first

  def sidebar_closed?
    cookies[:sidebar] == "closed"
  end

  # Largura em px dentro dos limites, ou nil sem um número (fica a largura padrão).
  def sidebar_width(raw)
    return nil unless raw.to_s.match?(/\A\d+\z/)

    raw.to_i.clamp(SIDEBAR_WIDTH_RANGE)
  end

  def current_sidebar_width
    sidebar_width(cookies[:sidebar_width]) || SIDEBAR_DEFAULT_WIDTH
  end

  # Atributos do <html>: estado fechado e largura escolhida, quando há.
  def sidebar_html_attributes
    width = sidebar_width(cookies[:sidebar_width])
    { data: { sidebar: ("closed" if sidebar_closed?) }, style: ("--sidebar-width: #{width}px" if width) }
  end

  # Lombada da barra (.sidebar-spine): botão só de ícone que recolhe ou expande; o title mostra o atalho.
  def sidebar_toggle_button(label)
    tag.button(type: "button", title: "#{label} (Ctrl+B)", class: "sidebar-spine",
               aria: { label:, controls: "sidebar", expanded: !sidebar_closed?, keyshortcuts: "Control+B Meta+B" },
               data: { action: "sidebar#toggle", sidebar_target: "toggle" }) { icon("sidebar-simple") }
  end
end
