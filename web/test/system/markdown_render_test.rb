require "application_system_test_case"

# Leitura renderizada como um README do GitHub: fonte do sistema no texto, Fira Code só no código, títulos sem o # do
# markdown, h1 e h2 sublinhados, listas, tabelas, citações e regras no desenho do GitHub, nas cores do tema.
class MarkdownRenderTest < ApplicationSystemTestCase
  GITHUB_FONT = '-apple-system, BlinkMacSystemFont, "Segoe UI", "Noto Sans", Helvetica, Arial, sans-serif'.freeze
  CONTENT = <<~MARKDOWN.freeze
    # Plan

    Run `make` now.

    ## Steps

    ### Details

    - one
      - nested

    1. first

    > quoted

    ---

    | a | b |
    |---|---|
    | 1 | 2 |
    | 3 | 4 |

    ```
    fenced
    ```
  MARKDOWN

  # Cor de um token do tema como o navegador a resolve, para comparar com as cores computadas.
  TOKEN_SCRIPT = <<~JS.freeze
    const probe = document.createElement("span"); probe.style.color = `var(${arguments[0]})`
    document.body.append(probe); const color = getComputedStyle(probe).color; probe.remove(); return color
  JS

  setup do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: CONTENT } ])
    sign_in_through_form
    visit note_path("1")
    click_on "Read"
    assert_selector ".note-preview h1", text: "Plan"
  end

  test "headings show only their text, without the markdown hash" do
    %w[h1 h2 h3].each do |tag|
      assert_equal "none", style_of(".note-preview #{tag}", "content", pseudo: "::before"), "expected no # before the #{tag}"
    end
  end

  test "the text uses the GitHub system font and only code keeps Fira Code" do
    assert_equal GITHUB_FONT, style_of(".note-preview", "fontFamily")
    [ ".note-preview p code", ".note-preview pre code", "#sidebar" ].each do |selector|
      assert_includes style_of(selector, "fontFamily"), "Fira Code", "expected Fira Code in #{selector}"
    end
  end

  test "the editor keeps Fira Code" do
    click_on "Edit"
    assert_includes style_of(".cm-content", "fontFamily"), "Fira Code"
  end

  test "the body text has the README size and h1 and h2 sit on a line in the theme line color" do
    assert_equal %w[16px 24px], [ style_of(".note-preview p", "fontSize"), style_of(".note-preview p", "lineHeight") ]
    %w[h1 h2].each do |tag|
      assert_equal [ "1px", "solid", token("--line") ], border_of(".note-preview #{tag}", "Bottom"), "expected a line under the #{tag}"
    end
    assert_equal "0px", style_of(".note-preview h3", "borderBottomWidth")
  end

  test "lists, quotes, rules and tables follow the GitHub layout in the theme colors" do
    assert_equal %w[disc circle decimal], [ ".note-preview ul", ".note-preview ul ul", ".note-preview ol" ].map { |selector| style_of(selector, "listStyleType") }
    assert_equal [ "4px", "solid", token("--line") ], border_of(".note-preview blockquote", "Left")
    assert_equal [ "4px", token("--line") ], [ style_of(".note-preview hr", "height"), style_of(".note-preview hr", "backgroundColor") ]
    assert_equal [ "1px", "solid", token("--line") ], border_of(".note-preview td", "Left")
    assert_equal token("--surface-sunken"), style_of(".note-preview tbody tr:nth-child(2)", "backgroundColor")
  end

  private
    def style_of(selector, property, pseudo: nil)
      find(selector, match: :first).evaluate_script("getComputedStyle(this, arguments[0])[arguments[1]]", pseudo, property)
    end

    def border_of(selector, side)
      %w[Width Style Color].map { |part| style_of(selector, "border#{side}#{part}") }
    end

    def token(name)
      evaluate_script("(function() { #{TOKEN_SCRIPT} }).apply(null, arguments)", name)
    end
end
