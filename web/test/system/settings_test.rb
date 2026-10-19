require "application_system_test_case"

# Configurações unificadas: Ctrl+, ou Cmd+, abrem de qualquer tela logada, como no VS Code e no Obsidian.
class SettingsTest < ApplicationSystemTestCase
  setup do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])
    sign_in_through_form
  end

  test "ctrl+comma opens the settings from a note, also with the focus in the editor" do
    visit note_path("1")
    find(".cm-content").send_keys([ :control, "," ])

    assert_current_path settings_account_path
    assert_selector "main h1", text: "Account"
  end

  test "cmd+comma opens the settings too" do
    visit root_path
    find("body").send_keys([ :meta, "," ])

    assert_current_path settings_account_path
  end

  test "the close button goes back to the notes" do
    visit settings_account_path
    click_on "Close settings"

    assert_current_path root_path
  end

  test "picking a theme in the appearance repaints and stays in the settings" do
    visit settings_appearance_path
    within("main") { click_on "Dark" }

    assert_selector "html[data-theme=dark]", visible: :all
    assert_current_path settings_appearance_path
    assert_selector "main button[aria-pressed=true]", text: "Dark"
  end
end
