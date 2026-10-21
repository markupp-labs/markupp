# Atalhos de teclado do painel, numa lista só: a ajuda do ? (layouts/_shortcut_help) mostra cada um com as teclas, e a
# paleta de comandos (layouts/_command_palette) oferece os que têm ação (run), com a mesma tecla.
# keys: alternativas, cada uma como aparece na ajuda ("Ctrl+Shift+P"). run diz como a paleta faz a ação:
#   click: clica no elemento da página (sem ele na página, a ação some da paleta);
#   press: repete o atalho, para quem o trata decidir o resto (a pasta da nota nova); when: o que precisa existir;
#   href: abre a página pelo Turbo (a nota com mudança pergunta antes de sair);
#   form: envia um form escondido na paleta (url, method, params; turbo: false quando muda o html, que o Turbo não troca).
# help: false deixa a ação só na paleta, quando a ajuda já tem a tecla numa linha mais geral (Ctrl+E alterna os dois modos).
# areas: as áreas (data-area do body) onde o atalho vale; sem areas, todas. A mesma tecla em áreas diferentes (Alt+N) aparece
#   na ajuda e na paleta só na área dela.
module ShortcutsHelper
  KEY_LABELS = { "+" => " ", "Delete" => "Del" }.freeze

  Shortcut = Data.define(:name, :keys, :run, :help, :areas) do
    def self.of(name, *keys, run: nil, help: true, areas: nil) = new(name:, keys:, run:, help:, areas:)

    def in_area?(area) = areas.nil? || areas.include?(area)
  end

  # Visão da barra lateral pelo atalho dela (sidebar_controller.js), que a paleta repete; só com a visão na página.
  SIDEBAR_VIEW_PRESS = ->(view, letter) { { press: { key: letter, code: "Key#{letter}", ctrlKey: true, shiftKey: true }, when: "#sidebar_view_#{view}" } }
  # Filtros do grafo cheio pela tecla deles (graph_filters_controller.js), que a paleta repete; só na tela do grafo.
  GRAPH_FILTER_PRESS = ->(letter) { { press: { key: letter.downcase, code: "Key#{letter}" }, when: "#graph_filters" } }

  SHORTCUT_GROUPS = {
    general: [
      Shortcut.of(:command_palette, "Ctrl+Shift+P", "F1", ":"),
      Shortcut.of(:shortcuts_help, "?", run: { press: { key: "?", code: "Slash", shiftKey: true } }),
      Shortcut.of(:search, "Ctrl+K", run: { click: "button[data-action='search#open']" }),
      Shortcut.of(:settings, "Ctrl+,", run: { href: :settings_path }),
      Shortcut.of(:toggle_sidebar, "Ctrl+B", run: { click: ".sidebar-spine" }),
      Shortcut.of(:focus_tree, "Ctrl+Shift+E", run: SIDEBAR_VIEW_PRESS.("explorer", "E")),
      Shortcut.of(:focus_links, "Ctrl+Shift+L", run: SIDEBAR_VIEW_PRESS.("links", "L")),
      Shortcut.of(:focus_graph, "Ctrl+Shift+G", run: SIDEBAR_VIEW_PRESS.("graph", "G")),
      # Grafo do cofre em tela cheia, a partir da nota aberta (full_graph_shortcut_controller.js).
      Shortcut.of(:full_graph, "Alt+G", run: { press: { key: "g", code: "KeyG", altKey: true } }),
      # Visão do Explorer (pastas, centrais, ligadas, recentes, órfãs, comunidades): abre o seletor (explorer_mode_controller.js).
      Shortcut.of(:explorer_mode, "Alt+O", run: { press: { key: "o", code: "KeyO", altKey: true }, when: "#explorer_mode" }),
      # Região vizinha pela geometria (region_nav_controller.js); a paleta repete a tecla a partir do foco devolvido.
      Shortcut.of(:focus_region_left, "Alt+H", run: { press: { key: "h", code: "KeyH", altKey: true } }),
      Shortcut.of(:focus_region_below, "Alt+J", run: { press: { key: "j", code: "KeyJ", altKey: true } }),
      Shortcut.of(:focus_region_above, "Alt+K", run: { press: { key: "k", code: "KeyK", altKey: true } }),
      Shortcut.of(:focus_region_right, "Alt+L", run: { press: { key: "l", code: "KeyL", altKey: true } }),
      Shortcut.of(:resize_sidebar, "←", "→"),
      Shortcut.of(:back_from_settings, "Esc"),
      Shortcut.of(:light_theme, run: { form: { url: :theme_path, params: { theme: "light" }, turbo: false } }),
      Shortcut.of(:dark_theme, run: { form: { url: :theme_path, params: { theme: "dark" }, turbo: false } }),
      Shortcut.of(:editor_style_plain, run: { form: { url: :editor_style_path, params: { editor_style: "plain" }, turbo: false } }),
      Shortcut.of(:editor_style_styled, run: { form: { url: :editor_style_path, params: { editor_style: "styled" }, turbo: false } }),
      Shortcut.of(:vim_keys_on, run: { form: { url: :vim_keys_path, params: { vim_keys: "on" }, turbo: false } }),
      Shortcut.of(:vim_keys_off, run: { form: { url: :vim_keys_path, params: { vim_keys: "off" }, turbo: false } }),
      Shortcut.of(:tenants, "g t", run: { href: :tenants_path }),
      Shortcut.of(:switch_tenant, run: { click: "button[popovertarget=tenant_switcher]" }),
      # Só na Danger zone de Members, para o owner: abre a mesma confirmação com o nome do tenant.
      Shortcut.of(:delete_tenant, run: { click: "#delete_tenant" }),
      Shortcut.of(:log_out, run: { form: { url: :session_path, method: :delete } })
    ],
    note: [
      Shortcut.of(:new_note, "Alt+N", run: { press: { key: "n", code: "KeyN", altKey: true }, when: "[data-note-tree-new-note-url-value]" },
                                         areas: %w[notes settings]),
      Shortcut.of(:save, "Ctrl+S"),
      Shortcut.of(:switch_edit_read, "Ctrl+E"),
      Shortcut.of(:edit_mode, "Ctrl+E", run: { click: ".note-mode-edit" }, help: false),
      Shortcut.of(:read_mode, "Ctrl+E", run: { click: ".note-mode-read" }, help: false),
      Shortcut.of(:side_by_side, "Ctrl+\\", run: { click: ".note-mode-split" }),
      Shortcut.of(:path_to_content, "Tab"),
      Shortcut.of(:indent_content, "Tab", "Shift+Tab"),
      Shortcut.of(:content_to_path, "Esc Shift+Tab")
    ],
    tree: [
      Shortcut.of(:next_row, "j", "↓"),
      Shortcut.of(:previous_row, "k", "↑"),
      Shortcut.of(:open_folder, "l", "→"),
      Shortcut.of(:close_folder, "h", "←"),
      Shortcut.of(:first_last_row, "Home", "End"),
      Shortcut.of(:open_row, "Enter"),
      Shortcut.of(:row_menu, "Shift+F10", "Menu"),
      Shortcut.of(:rename, "F2"),
      Shortcut.of(:delete, "Delete"),
      Shortcut.of(:copy_path, "Alt+Shift+C"),
      Shortcut.of(:copy_wikilink, "Alt+Shift+W")
    ],
    lists: [
      Shortcut.of(:list_move, "↓", "↑"),
      Shortcut.of(:palette_move, "Ctrl+J", "Ctrl+K"),
      Shortcut.of(:menu_move, "j", "k"),
      Shortcut.of(:list_choose, "Enter"),
      Shortcut.of(:list_close, "Esc")
    ],
    # Comandos Ex e teclas do modo Vim (editor/vim_mode.js), com o foco no editor; a ajuda só mostra com o modo ligado.
    vim: [
      Shortcut.of(:vim_write, ":w"),
      Shortcut.of(:vim_write_quit, ":wq"),
      Shortcut.of(:vim_quit, ":q"),
      Shortcut.of(:vim_quit_force, ":q!"),
      Shortcut.of(:vim_open, ":e path"),
      Shortcut.of(:vim_edit, ":edit", ":e"),
      Shortcut.of(:vim_read, ":read"),
      Shortcut.of(:vim_split, ":split"),
      Shortcut.of(:vim_leave_editor, "Esc Esc Tab"),
      Shortcut.of(:vim_back_to_path, "Esc Esc Shift+Tab")
    ],
    tenants: [
      Shortcut.of(:filter_tenants, "/"),
      Shortcut.of(:new_tenant, "Alt+N", run: { href: :new_tenant_path }, areas: %w[tenants]),
      Shortcut.of(:move_tenants, "hjkl", "←↓↑→"),
      Shortcut.of(:open_tenant, "Enter")
    ],
    # Visão Links da nota aberta (note_links_controller.js), com o foco nela.
    links: [
      Shortcut.of(:links_move, "j", "k", "↓", "↑"),
      Shortcut.of(:links_open, "Enter"),
      Shortcut.of(:links_copy_wikilink, "y")
    ],
    # Canvas do grafo, na visão Graph e na tela cheia (graph_view_controller.js), com o foco nele.
    graph: [
      Shortcut.of(:graph_move, "j", "k", "↓", "↑"),
      Shortcut.of(:graph_open, "Enter"),
      Shortcut.of(:graph_zoom, "+", "-"),
      Shortcut.of(:graph_fit, "0"),
      Shortcut.of(:graph_back, "Esc"),
      # Filtros do grafo cheio (graphs/_filters), com o foco na tela do grafo fora de campo de texto.
      Shortcut.of(:graph_filters, "f", run: GRAPH_FILTER_PRESS.("F")),
      Shortcut.of(:graph_orphans, "o", run: GRAPH_FILTER_PRESS.("O")),
      Shortcut.of(:graph_unresolved, "u", run: GRAPH_FILTER_PRESS.("U")),
      Shortcut.of(:graph_inferred, "i", run: GRAPH_FILTER_PRESS.("I")),
      Shortcut.of(:graph_community, "c", "Shift+C", run: GRAPH_FILTER_PRESS.("C"))
    ]
  }.freeze

  SHORTCUTS_BY_NAME = SHORTCUT_GROUPS.values.flatten.index_by(&:name).freeze

  # Área da página (notas, configurações, tenants), que a view diz com content_for :area; sem ela, são as notas. O layout
  # a põe no data-area do body.
  def current_area = content_for(:area).presence || "notes"

  # Teclas de um atalho pelo nome, a fonte única para os title, os kbd e os aria-keyshortcuts fora da ajuda e da paleta.
  def shortcut_keys(name) = SHORTCUTS_BY_NAME.fetch(name).keys

  # Primeira tecla do atalho, a que os title e os kbd mostram.
  def shortcut_key(name) = shortcut_keys(name).first

  # Rótulo curto da tecla para o <kbd>, como no menu do VS Code: Alt+Shift+C vira "Alt Shift C", Delete vira "Del".
  def shortcut_label(keys) = keys.gsub(/\+|Delete/, KEY_LABELS)

  def shortcut_name(shortcut) = t("shortcuts.names.#{shortcut.name}")

  # Grupos da ajuda, cada um só com os atalhos que têm tecla e valem na área; o do Vim só com o modo Vim ligado.
  def help_shortcut_groups
    groups = SHORTCUT_GROUPS.transform_values { |shortcuts| shortcuts.select { it.help && it.keys.any? && it.in_area?(current_area) } }
    chosen_vim_keys == "on" ? groups : groups.except(:vim)
  end

  # Largura em caracteres da coluna das teclas na ajuda, a do atalho mais longo ("Shift+F10 / Menu"): todos os grupos usam
  # a mesma, e as descrições começam no mesmo x, como num --help.
  def help_keys_width = help_shortcut_groups.values.flatten.map { it.keys.join(" / ").length }.max

  # As ações da nota primeiro: é sobre a nota que a paleta mais abre.
  def palette_commands = [ :note, *SHORTCUT_GROUPS.keys ].uniq.flat_map { SHORTCUT_GROUPS[it] }.select { it.run && it.in_area?(current_area) }

  # Item da paleta: o nome, a tecla e, nos data-command-*, como rodar a ação (command_palette_controller.js).
  def command_option(command)
    tag.li(role: "option", id: "command_#{command.name}", aria: { selected: false }, data: command_data(command),
           class: "tree-row flex cursor-pointer items-center justify-between gap-4 aria-selected:bg-accent-soft aria-selected:text-ink") do
      safe_join([ tag.span(shortcut_name(command), data: { command_label: "" }),
                  (tag.kbd(shortcut_label(command.keys.first), class: "text-xs text-ink-muted") if command.keys.any?) ])
    end
  end

  # Form escondido de uma ação da paleta que muda algo no servidor (tema, estilo do editor, sair).
  def command_form(command)
    form = command.run.fetch(:form)
    button_to "", send(form.fetch(:url)), method: form.fetch(:method, :patch), params: form.fetch(:params, {}),
              form: { id: "command_form_#{command.name}", hidden: true, data: ({ turbo: false } if form[:turbo] == false) }
  end

  private
    def command_data(command)
      run = command.run
      { command_palette_target: "option", action: "click->command-palette#choose",
        command_click: run[:click], command_when: run[:when] || run[:click], command_href: (send(run[:href]) if run[:href]),
        command_press: run[:press]&.to_json, command_form: ("command_form_#{command.name}" if run[:form]) }
    end
end
