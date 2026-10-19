require "application_system_test_case"

# As páginas de erro saem do próprio painel: mascote, mensagem em tom de terminal e um caminho de volta.
class ErrorPagesTest < ApplicationSystemTestCase
  setup { use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ]) }

  test "an unknown address shows the styled 404 and leads to the login" do
    visit "/nowhere"

    assert_selector "h1", text: "404: Nothing at /nowhere"
    assert_selector "pre[data-mascot=large]", visible: :all
    click_on "Go to log in"
    assert_current_path new_session_path
  end

  test "an unknown address after logging in leads back to the notes" do
    sign_in_through_form
    visit "/nowhere"

    click_on "Back to your notes"
    assert_current_path root_path
  end

  test "an unknown note shows the 404 next to the sidebar" do
    sign_in_through_form
    visit note_path("missing")

    assert_selector "main h1", text: "404: Note not found"
    assert_selector "main pre[data-mascot=large]"
    assert_selector "nav#sidebar", text: "plan.md"
  end
end
