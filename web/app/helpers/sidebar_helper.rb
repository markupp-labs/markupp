# Barra lateral no desktop: aberta ou fechada e a largura moram em cookies gravados pelo sidebar_controller.js,
# lidos aqui para o servidor já desenhar o estado certo (sem piscar, como o tema).
module SidebarHelper
  SIDEBAR_WIDTH_RANGE = 240..480
  SIDEBAR_DEFAULT_WIDTH = 288
  # Visões da barra lateral, na ordem da barra de atividades, com o ícone e o atalho de cada uma; a primeira é a padrão.
  SIDEBAR_VIEWS = {
    "explorer" => { icon: "files", shortcut: :focus_tree },
    "links" => { icon: "link", shortcut: :focus_links },
    "graph" => { icon: "graph", shortcut: :focus_graph }
  }.freeze

  # Onde o atalho da visão põe o foco (sidebar_controller.js): no Explorer, a linha da árvore; nas outras, o primeiro
  # link ou botão e, sem nenhum, a própria visão.
  SIDEBAR_VIEW_FOCUS = { "explorer" => "[role=tree] [role=treeitem][tabindex='0']" }.freeze

  # Visão escolhida (cookie sidebar_view, gravado pelo sidebar_controller.js); sem uma conhecida, a padrão.
  def current_sidebar_view
    view = cookies[:sidebar_view]
    SIDEBAR_VIEWS.key?(view) ? view : SIDEBAR_VIEWS.keys.first
  end

  # Ícone da barra de atividades (layouts/_activity_bar): só o da visão ativa entra no Tab; o title mostra o atalho.
  def sidebar_view_tab(view)
    label = t("layouts.activity_bar.#{view}")
    keys = shortcut_key(SIDEBAR_VIEWS.fetch(view)[:shortcut])
    selected = view == current_sidebar_view
    tag.button(type: "button", role: "tab", class: "activity-tab", title: "#{label} (#{keys})", tabindex: selected ? 0 : -1,
               aria: { label:, selected:, controls: "sidebar_view_#{view}", keyshortcuts: aria_keyshortcuts(keys) },
               data: { action: "sidebar#selectView keydown->sidebar#moveTab", sidebar_view_param: view, sidebar_target: "tab" }) do
      icon(SIDEBAR_VIEWS.fetch(view)[:icon])
    end
  end

  # Conteúdo de uma visão; as que ainda não têm onde pôr o foco recebem o foco elas mesmas (tabindex -1).
  def sidebar_view_panel(view, &)
    tag.div(id: "sidebar_view_#{view}", role: "tabpanel", hidden: view != current_sidebar_view,
            tabindex: (-1 unless view == "explorer"), aria: { label: t("layouts.activity_bar.#{view}") },
            data: { sidebar_target: "view", sidebar_view: view, sidebar_view_focus: SIDEBAR_VIEW_FOCUS.fetch(view, "a[href], button") },
            class: "outline-offset-[-2px]", &)
  end

  # "Ctrl+Shift+E" vira "Control+Shift+E Meta+Shift+E", a tecla do Linux e do Windows e a do Mac.
  def aria_keyshortcuts(keys) = %w[Control Meta].map { keys.sub("Ctrl", it) }.join(" ")

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
