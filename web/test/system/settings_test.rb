require "application_system_test_case"

# Configurações unificadas: Ctrl+, ou Cmd+, abrem de qualquer tela logada, como no VS Code e no Obsidian.
class SettingsTest < ApplicationSystemTestCase
  setup do
    @api = use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])
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

  test "escape goes back to the note the settings were opened from" do
    visit note_path("1")
    find(".cm-content").send_keys([ :control, "," ])
    assert_current_path settings_account_path

    find("body").send_keys(:escape)

    assert_current_path note_path("1")
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

  # Lista como as configuracoes do Obsidian: uma linha por configuracao, nome e dica a esquerda, controle a direita.
  test "the editor settings are rows with the name and hint on the left and the control on the right" do
    visit settings_editor_path

    { "editor_style" => "[role=group][aria-labelledby=editor_style_heading]",
      "vim_keys" => "[role=switch][aria-labelledby=vim_keys_heading]" }.each do |name, control|
      hint, control_box = rect_of("##{name}_hint"), rect_of("main #{control}")
      assert_operator control_box["left"], :>=, hint["right"], "#{name}: the control should sit right of the hint"
      assert_operator control_box["top"], :<, hint["bottom"], "#{name}: the control should share the row of the hint"
    end
  end

  test "the vim key bindings switch turns on with the keyboard and stays outside turbo" do
    visit settings_editor_path
    find("main [role=switch][aria-checked=false]").send_keys(:enter)

    assert_selector "main [role=switch][aria-checked=true]"
    assert_selector "html[data-vim-keys=on]", visible: :all
  end

  test "change password emails the reset token and the token link opens the reset form" do
    visit settings_account_path
    within("main") { click_on "Change password" }

    assert_current_path settings_account_path
    assert_text "Password reset email sent to #{FakeMarkuppAccounts::DEFAULT_EMAIL}."
    within("main") { click_on "I have a reset token" }

    assert_current_path new_password_reset_path
  end

  test "resetting the password from the account ends the session and asks to log in again" do
    visit settings_account_path
    within("main") { click_on "Change password" }
    assert_text "Password reset email sent to #{FakeMarkuppAccounts::DEFAULT_EMAIL}."
    within("main") { click_on "I have a reset token" }
    fill_in "Token", with: @api.sent_tokens.last.fetch(:token)
    fill_in "New password", with: "senha-nova"
    click_on "Reset password"

    assert_text "Password changed. Log in again."
    assert_current_path new_session_path
    visit settings_account_path
    assert_current_path new_session_path
  end

  test "the settings sidebar opens a note from the tree" do
    visit settings_appearance_path
    within("nav#sidebar") { click_on "plan" }

    assert_current_path note_path("1")
  end

  private
    def rect_of(selector) = find(selector).evaluate_script("this.getBoundingClientRect().toJSON()")
end
