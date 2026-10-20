require "application_system_test_case"

# Paleta de comandos à moda do VS Code: Ctrl+Shift+P abre, a busca fuzzy filtra, setas ou Ctrl+J e Ctrl+K andam, Enter
# roda a ação e Esc fecha.
class CommandPaletteTest < ApplicationSystemTestCase
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

  test "the arrows and ctrl+j or ctrl+k move the selection without opening the search" do
    open_palette
    send_keys "mode"
    assert_equal [ "Edit mode", "Read mode" ], option_labels

    send_keys [ :control, "j" ]
    assert_selector "[role=option][aria-selected=true]", text: "Read mode"
    send_keys [ :control, "k" ]
    assert_selector "[role=option][aria-selected=true]", text: "Edit mode"
    assert_no_selector "dialog[data-search-target=dialog][open]"
    send_keys :up, :enter

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

    run_command "light theme"
    assert_selector "html[data-theme=light]"

    run_command "settings"
    assert_selector "main h1", text: "Account"
  end

  test "new note from the palette opens in the folder of the open note" do
    run_command "new note"

    assert_field "Path", with: "/work/"
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

    def run_command(query)
      open_palette
      send_keys query
      assert_selector "[role=option][aria-selected=true]"
      send_keys :enter
    end

    def rgb(hex)
      evaluate_script("(() => { const probe = document.createElement('i'); probe.style.color = '#{hex.strip}'; document.body.append(probe); const color = getComputedStyle(probe).color; probe.remove(); return color })()")
    end

    def option_labels
      all("#command_palette [role=option]").map { it.find("[data-command-label]").text }
    end
end
