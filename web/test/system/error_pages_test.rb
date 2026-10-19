require "application_system_test_case"

# As páginas de erro saem do próprio painel: mascote, mensagem em tom de terminal e um caminho de volta.
class ErrorPagesTest < ApplicationSystemTestCase
  # load() resolve com as faces que carregaram; fonte não declarada dá lista vazia e falha de rede rejeita.
  FONT_LOADED_SCRIPT = <<~JS.freeze
    const done = arguments[0]
    document.fonts.load('16px "Fira Code"').then((faces) => done(faces.length > 0), () => done(false))
  JS

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

  test "the static 500 page draws in the panel font without the app" do
    visit "/500.html"

    assert_selector "main h1", text: "500: Something broke on the panel"
    assert evaluate_async_script(FONT_LOADED_SCRIPT), "expected the Fira Code from /FiraCode-VF.woff2 to load on the static page"
  end
end
