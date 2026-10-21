require "application_system_test_case"

# Edição mostra o markdown como está escrito; o renderizado fica para a leitura e o lado a lado.
class EditorStyleTest < ApplicationSystemTestCase
  TRANSPARENT = "rgba(0, 0, 0, 0)".freeze
  CODE_CONTENT = "Run `make` now.\n\n```\nlong fenced line\nx\n```\n".freeze
  CONTENT = "intro\n\n# Plan\n\nSome **bold** and [docs](https://example.com).\n".freeze

  FONT_SIZES_SCRIPT = <<~JS.freeze
    const heading = [ ...document.querySelectorAll(".cm-line") ].find((line) => line.textContent.includes("Plan"))
    const content = document.querySelector(".cm-content")
    return [ getComputedStyle(heading.lastElementChild || heading).fontSize, getComputedStyle(content).fontSize ]
  JS

  setup do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: CONTENT }, { id: "2", path: "code.md", content: CODE_CONTENT } ])
    sign_in_through_form
  end

  test "edit shows every markdown mark even away from the cursor" do
    visit note_path("1")

    assert_selector ".cm-content", text: "# Plan"
    assert_selector ".cm-content", text: "**bold**"
    assert_selector ".cm-content", text: "[docs](https://example.com)"
  end

  test "plain is the default and keeps headings the size of the text" do
    visit note_path("1")
    assert_selector ".cm-content", text: "Plan"

    heading_size, text_size = evaluate_script("(function() { #{FONT_SIZES_SCRIPT} })()")
    assert_equal text_size, heading_size
  end

  test "styled from the editor settings makes headings larger" do
    visit "/settings/editor"
    assert_selector "main h1", text: "Editor"
    within("[aria-labelledby=editor_style_heading]") do
      click_on "Styled"
      # Espera a volta do PATCH: visitar antes dela perde o cookie.
      assert_selector "button[aria-pressed=true]", text: "Styled"
    end

    visit note_path("1")
    assert_selector ".cm-content", text: "Plan"

    heading_size, text_size = evaluate_script("(function() { #{FONT_SIZES_SCRIPT} })()")
    assert_operator heading_size.to_f, :>, text_size.to_f
  end

  # O estilo mora no data-editor-style do html, que uma visita do Turbo nao troca: o botao vai sem Turbo, como o tema.
  test "styled from the editor settings holds when going back to the note through the sidebar" do
    visit "/settings/editor"
    assert_selector "main h1", text: "Editor"
    within("[aria-labelledby=editor_style_heading]") { click_on "Styled" }
    assert_selector "html[data-editor-style=styled]"

    within("nav#sidebar") { click_on "plan" }
    assert_selector ".cm-content", text: "Plan"

    heading_size, text_size = evaluate_script("(function() { #{FONT_SIZES_SCRIPT} })()")
    assert_operator heading_size.to_f, :>, text_size.to_f
  end

  # Fundo pintado por trecho de linha deixa o bloco cercado serrilhado: só o código inline leva fundo.
  test "the editor gives a background to inline code but not to a fenced block" do
    visit note_path("2")
    assert_selector ".cm-content", text: "long fenced line"

    assert_not_equal TRANSPARENT, background_of_innermost(".cm-line", "make")
    assert_equal TRANSPARENT, background_of_innermost(".cm-line", "long fenced line")
  end

  private
    def background_of_innermost(line_selector, text)
      line = find(line_selector, text: text, match: :first)
      element = line.all("span", text: text).last || line
      element.style("background-color")["background-color"]
    end
end
