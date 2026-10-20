# Atalhos de teclado do painel, numa lista só: a ajuda do ? (layouts/_shortcut_help) mostra cada um com as teclas, e a
# paleta de comandos (layouts/_command_palette) oferece os que têm ação (run), com a mesma tecla.
# keys: alternativas, cada uma como aparece na ajuda ("Ctrl+Shift+P"). run diz como a paleta faz a ação:
#   click: clica no elemento da página (sem ele na página, a ação some da paleta);
#   press: repete o atalho, para quem o trata decidir o resto (a pasta da nota nova); when: o que precisa existir;
#   href: abre a página pelo Turbo (a nota com mudança pergunta antes de sair);
#   form: envia um form escondido na paleta (url, method, params; turbo: false quando muda o html, que o Turbo não troca).
# help: false deixa a ação só na paleta, quando a ajuda já tem a tecla numa linha mais geral (Ctrl+E alterna os dois modos).
module ShortcutsHelper
  KEY_LABELS = { "+" => " ", "Delete" => "Del" }.freeze

  Shortcut = Data.define(:name, :keys, :run, :help) do
    def self.of(name, *keys, run: nil, help: true) = new(name:, keys:, run:, help:)
  end

  SHORTCUT_GROUPS = {
    general: [
      Shortcut.of(:command_palette, "Ctrl+Shift+P"),
      Shortcut.of(:shortcuts_help, "?", run: { press: { key: "?", code: "Slash", shiftKey: true } }),
      Shortcut.of(:search, "Ctrl+K", run: { click: "button[data-action='search#open']" }),
      Shortcut.of(:settings, "Ctrl+,", run: { href: :settings_path }),
      Shortcut.of(:toggle_sidebar, "Ctrl+B", run: { click: ".sidebar-spine" }),
      Shortcut.of(:focus_tree, "Ctrl+Shift+E"),
      Shortcut.of(:resize_sidebar, "←", "→"),
      Shortcut.of(:back_from_settings, "Esc"),
      Shortcut.of(:light_theme, run: { form: { url: :theme_path, params: { theme: "light" }, turbo: false } }),
      Shortcut.of(:dark_theme, run: { form: { url: :theme_path, params: { theme: "dark" }, turbo: false } }),
      Shortcut.of(:editor_style_plain, run: { form: { url: :editor_style_path, params: { editor_style: "plain" }, turbo: false } }),
      Shortcut.of(:editor_style_styled, run: { form: { url: :editor_style_path, params: { editor_style: "styled" }, turbo: false } }),
      Shortcut.of(:tenants, "g t", run: { href: :tenants_path }),
      Shortcut.of(:switch_tenant, run: { click: "button[popovertarget=tenant_switcher]" }),
      Shortcut.of(:log_out, run: { form: { url: :session_path, method: :delete } })
    ],
    note: [
      Shortcut.of(:new_note, "Alt+N", run: { press: { key: "n", code: "KeyN", altKey: true }, when: "[data-note-tree-new-note-url-value]" }),
      Shortcut.of(:save, "Ctrl+S"),
      Shortcut.of(:switch_edit_read, "Ctrl+E"),
      Shortcut.of(:edit_mode, "Ctrl+E", run: { click: ".note-mode-edit" }, help: false),
      Shortcut.of(:read_mode, "Ctrl+E", run: { click: ".note-mode-read" }, help: false),
      Shortcut.of(:side_by_side, "Ctrl+\\", run: { click: ".note-mode-split" }),
      Shortcut.of(:path_to_content, "Tab"),
      Shortcut.of(:content_to_path, "Shift+Tab")
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
    tenants: [
      Shortcut.of(:filter_tenants, "/"),
      Shortcut.of(:new_tenant, "Alt+N", run: { href: :new_tenant_path }),
      Shortcut.of(:move_tenants, "hjkl", "←↓↑→"),
      Shortcut.of(:open_tenant, "Enter")
    ]
  }.freeze

  # Rótulo curto da tecla para o <kbd>, como no menu do VS Code: Alt+Shift+C vira "Alt Shift C", Delete vira "Del".
  def shortcut_label(keys) = keys.gsub(/\+|Delete/, KEY_LABELS)

  def shortcut_name(shortcut) = t("shortcuts.names.#{shortcut.name}")

  # Grupos da ajuda, cada um só com os atalhos que têm tecla.
  def help_shortcut_groups
    SHORTCUT_GROUPS.transform_values { |shortcuts| shortcuts.select { it.help && it.keys.any? } }
  end

  # Largura em caracteres da coluna das teclas na ajuda, a do atalho mais longo ("Shift+F10 / Menu"): todos os grupos usam
  # a mesma, e as descrições começam no mesmo x, como num --help.
  def help_keys_width = help_shortcut_groups.values.flatten.map { it.keys.join(" / ").length }.max

  # As ações da nota primeiro: é sobre a nota que a paleta mais abre.
  def palette_commands = [ :note, *SHORTCUT_GROUPS.keys ].uniq.flat_map { SHORTCUT_GROUPS[it] }.select(&:run)

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
