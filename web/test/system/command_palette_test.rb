require "application_system_test_case"

# Paleta de comandos à moda do VS Code: Ctrl+Shift+P abre, a busca fuzzy filtra, setas ou Ctrl+J e Ctrl+K andam, Enter
# roda a ação e Esc fecha.
class CommandPaletteTest < ApplicationSystemTestCase
  MODES = [ "Edit mode", "Read mode", "Side by side" ].freeze
  THEMES = [ "Light theme", "Dark theme" ].freeze

  setup do
    use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "# Plan\n" } ])
    sign_in_through_form
    visit note_path("1")
    assert_field "Path", with: "/work/plan"
  end

  test "ctrl+shift+p opens the palette, a fuzzy query filters and enter runs the first command" do
    open_palette
    send_keys "tgsd"

    assert_equal [ "Toggle sidebar" ], option_labels
    assert_selector "[role=option][aria-selected=true]", text: "Toggle sidebar"
    send_keys :enter

    assert_no_selector "dialog#command_palette[open]"
    assert_selector "html[data-sidebar=closed]"
  end

  # ":" como o modo de comando do Vim, fora de campo de texto (no editor e no caminho é texto); F1 como no VS Code, de
  # qualquer lugar. O ":" vai pelo CDP: o chromedriver manda pontuação com key vazio.
  test "colon outside a text field and f1 from anywhere open the palette" do
    find("[role=treeitem]", text: "plan").execute_script("this.focus()")
    press_colon
    assert_selector "dialog#command_palette[open] input:focus"
    send_keys :escape

    find(".cm-content").send_keys([ :control, :end ], "a")
    press_colon
    find_field("Path").send_keys(:end)
    press_colon
    assert_no_selector "dialog#command_palette[open]"
    assert_selector ".cm-content", text: "a:"
    assert_field "Path", with: "/work/plan:"

    find(".cm-content").send_keys(:f1)
    assert_selector "dialog#command_palette[open] input:focus"
  end

  test "the arrows and ctrl+j or ctrl+k move the selection without opening the search" do
    open_palette
    send_keys "ode"
    assert_equal [ "Read mode", "Toggle sidebar" ], option_labels

    send_keys [ :control, "j" ]
    assert_selector "[role=option][aria-selected=true]", text: "Toggle sidebar"
    send_keys [ :control, "k" ]
    assert_selector "[role=option][aria-selected=true]", text: "Read mode"
    assert_no_selector "dialog[data-search-target=dialog][open]"
    send_keys :up
    assert_selector "[role=option][aria-selected=true]", text: "Toggle sidebar"
    send_keys :down, :enter

    assert_selector "button[aria-label=Read][aria-pressed=true]"
    assert_selector ".note-preview h1", text: "Plan"
  end

  test "esc closes the palette and gives the focus back" do
    find(".cm-content").click
    open_palette
    send_keys :escape

    assert_no_selector "dialog#command_palette[open]"
    assert_selector ".cm-content:focus"
  end

  test "actions with nothing to act on in the page are left out" do
    visit settings_appearance_path
    open_palette
    send_keys "read mode"

    assert_empty option_labels
    assert_selector "#command_palette", text: "No matching command"
  end

  test "links, forms and other shortcuts run from the palette" do
    run_command "keyboard shortcuts"
    assert_selector "dialog#shortcut_help[open]"
    send_keys :escape
    # A próxima tecla vai para quem tem o foco; com a ajuda ainda fechando, a suíte paralela deu element not interactable.
    assert_no_selector "dialog#shortcut_help[open]"

    run_command "dark theme"
    assert_selector "html[data-theme=dark]"

    run_command "settings"
    assert_selector "main h1", text: "Account"
  end

  # Só oferece trocar: o tema ativo (inclusive quando segue o sistema, claro no Chromium headless) e o modo da nota ativo
  # ficam de fora.
  test "the theme and the note mode in use are left out" do
    assert_equal [ "Dark theme" ], switch_labels(THEMES)
    assert_equal [ "Read mode", "Side by side" ], switch_labels

    send_keys :escape
    run_command "read mode"
    assert_selector "button[aria-label=Read][aria-pressed=true]"
    assert_equal [ "Edit mode", "Side by side" ], switch_labels

    send_keys :escape
    run_command "side by side"
    assert_selector "button[aria-label='Side by side'][aria-pressed=true]"
    assert_equal [ "Edit mode", "Read mode" ], switch_labels

    send_keys :escape
    run_command "dark theme"
    assert_selector "html[data-theme=dark]"
    assert_equal [ "Light theme" ], switch_labels(THEMES)
  end

  test "new note from the palette opens in the folder of the open note" do
    run_command "new note"

    assert_field "Path", with: "/work/"
  end

  # Alt+N é a nota nova na nota e o New tenant na lista de tenants: a paleta e a ajuda só trazem o da tela.
  test "alt+n shows once in the palette and the help, as the new note on a note and the new tenant on the tenants" do
    assert_equal [ "New note" ], alt_n_entries

    visit tenants_path
    assert_selector "h1", text: "Tenants"
    assert_equal [ "New tenant" ], alt_n_entries
  end

  # A paleta fica aberta enquanto a página escolhida carrega; o Turbo guarda a página que sai no cache, e ela não pode
  # voltar pelo histórico com a paleta à mostra.
  test "going back after a page chosen in the palette restores the page without the palette" do
    visit settings_appearance_path
    run_command "all tenants"
    assert_selector "h1", text: "Tenants"
    page.go_back

    assert_selector "main h1", text: "Appearance"
    assert_no_selector "dialog#command_palette[open]"
  end

  # Barra de rolagem fina nos tokens do tema, como nos painéis que rolam, na lista da paleta e na ajuda de atalhos.
  test "the palette list and the shortcut help scroll with the thin theme scrollbar" do
    styles = evaluate_script(<<~JS)
      [ "#command_palette_options", "#shortcut_help" ].map((selector) => {
        const style = getComputedStyle(document.querySelector(selector))
        return [ style.scrollbarWidth, style.scrollbarColor ]
      })
    JS
    line = evaluate_script("getComputedStyle(document.body).getPropertyValue('--line')")

    assert_equal [ [ "thin", "#{rgb(line)} rgba(0, 0, 0, 0)" ] ] * 2, styles
  end

  private
    def open_palette
      send_keys [ :control, :shift, "p" ]
      assert_selector "dialog#command_palette[open] input:focus"
    end

    def press_colon = press_through_cdp(":", "Semicolon", keycode: 186, modifiers: 8)

    def run_command(query)
      open_palette
      send_keys query
      assert_selector "[role=option][aria-selected=true]"
      send_keys :enter
    end

    def rgb(hex)
      evaluate_script("(() => { const probe = document.createElement('i'); probe.style.color = '#{hex.strip}'; document.body.append(probe); const color = getComputedStyle(probe).color; probe.remove(); return color })()")
    end

    def switch_labels(among = MODES)
      open_palette
      option_labels & among
    end

    # Nomes das entradas de Alt+N: as da paleta e as da ajuda, que tem de dar o mesmo.
    def alt_n_entries
      entries = evaluate_script(<<~JS)
        [ [ ...document.querySelectorAll("#command_palette [role=option]") ].filter((option) => getComputedStyle(option).display != "none" && option.querySelector("kbd")?.textContent == "Alt N")
            .map((option) => option.querySelector("[data-command-label]").textContent),
          [ ...document.querySelectorAll("#shortcut_help dt") ].filter((keys) => keys.textContent == "Alt+N").map((keys) => keys.nextElementSibling.textContent) ]
      JS
      entries.first.zip(entries.last).each { |palette, help| assert_equal palette, help }
      assert_equal entries.first.size, entries.last.size
      entries.first
    end

    def option_labels
      all("#command_palette [role=option]").map { it.find("[data-command-label]").text }
    end
end
