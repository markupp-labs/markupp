require "test_helper"

# Ajuda de atalhos (?) e paleta de comandos (Ctrl+Shift+P) saem do mesmo registro: a paleta mostra cada ação com a
# tecla que a ajuda lista para ela.
class ShortcutsTest < ActionDispatch::IntegrationTest
  PALETTE = [
    [ "New note", "Alt N" ], [ "Edit mode", "Ctrl E" ], [ "Read mode", "Ctrl E" ], [ "Side by side", "Ctrl \\" ],
    [ "Toggle sidebar", "Ctrl B" ], [ "Search notes", "Ctrl K" ], [ "Settings", "Ctrl ," ], [ "Keyboard shortcuts", "?" ],
    [ "Light theme", "" ], [ "Dark theme", "" ], [ "Editor style: plain", "" ], [ "Editor style: styled", "" ],
    [ "Tenants", "" ], [ "Switch tenant", "" ], [ "Log out", "" ]
  ].freeze

  setup do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])
    sign_in_as
  end

  test "the help lists the shortcuts of every part of the panel, grouped" do
    get note_path("1")

    assert_select "dialog#shortcut_help[aria-labelledby=shortcut_help_title]" do
      assert_select "#shortcut_help_title", "Keyboard shortcuts"
      [ "General", "Note", "Tree", "Menus and lists" ].each { assert_select "h3", text: it }
    end
    rows = css_select("#shortcut_help dt").to_h { |keys| [ keys.next_element.text.strip, keys.css("kbd").map(&:text) ] }
    { "Command palette" => [ "Ctrl+Shift+P" ], "Keyboard shortcuts" => [ "?" ], "New note" => [ "Alt+N" ], "Save" => [ "Ctrl+S" ],
      "Rename / move" => [ "F2" ], "Copy wikilink" => [ "Alt+Shift+W" ], "Focus the note tree" => [ "Ctrl+Shift+E" ],
      "Next row" => [ "j", "↓" ] }.each do |name, keys|
      assert_equal keys, rows[name], "expected #{name} with #{keys.inspect} in the help"
    end
  end

  test "the palette offers every action with its shortcut, from the same list as the help" do
    get note_path("1")

    assert_select "dialog#command_palette input[role=combobox][aria-controls=command_palette_options]"
    options = css_select("#command_palette_options [role=option]").map do |option|
      [ option.at_css("[data-command-label]").text.strip, option.at_css("kbd")&.text.to_s ]
    end
    assert_equal PALETTE, options
  end
end
