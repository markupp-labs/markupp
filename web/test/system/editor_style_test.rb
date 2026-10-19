require "application_system_test_case"

# Edição mostra o markdown como está escrito; o renderizado fica para a leitura e o lado a lado.
class EditorStyleTest < ApplicationSystemTestCase
  CONTENT = "intro\n\n# Plan\n\nSome **bold** and [docs](https://example.com).\n".freeze

  FONT_SIZES_SCRIPT = <<~JS.freeze
    const heading = [ ...document.querySelectorAll(".cm-line") ].find((line) => line.textContent.includes("Plan"))
    const content = document.querySelector(".cm-content")
    return [ getComputedStyle(heading.lastElementChild || heading).fontSize, getComputedStyle(content).fontSize ]
  JS

  setup do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: CONTENT } ])
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

  test "styled from the appearance makes headings larger" do
    visit settings_appearance_path
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
end
