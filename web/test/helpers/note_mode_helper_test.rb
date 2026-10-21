require "test_helper"

class NoteModeHelperTest < ActionView::TestCase
  include IconsHelper
  test "note_mode keeps the last mode chosen for a saved note" do
    assert_equal "read", note_mode("read", persisted: true)
    assert_equal "split", note_mode("split", persisted: true)
  end

  test "note_mode opens a new note in edit whatever was chosen last" do
    assert_equal "edit", note_mode("read", persisted: false)
  end

  test "note_mode falls back to edit without a known mode" do
    assert_equal "edit", note_mode(nil, persisted: true)
    assert_equal "edit", note_mode("fullscreen", persisted: true)
  end

  # Lado a lado é o quadrado dividido do "split editor" do VS Code; duas colunas soltas pareciam um botão de pausa.
  test "the side by side button shows a split square" do
    button = Nokogiri::HTML5.fragment(note_mode_button("split", current: "edit")).at("button")

    assert_equal "square-split-horizontal", button.at("svg")["data-icon"]
  end
end
