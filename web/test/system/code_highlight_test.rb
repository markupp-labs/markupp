require "application_system_test_case"

class CodeHighlightTest < ApplicationSystemTestCase
  NOTE_WITH_CODE = <<~MARKDOWN.freeze
    ```ruby
    def greet(name)
      "Hello, \#{name}"
    end
    ```

    ```nosuchlang
    def plain
    ```

    ```
    def bare
    ```
  MARKDOWN

  setup do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: NOTE_WITH_CODE }, { id: "2", path: "blank.md", content: "" } ])
    sign_in_through_form
  end

  test "reading highlights a fenced ruby block" do
    visit note_path("1")
    click_on "Read"

    assert_selector ".note-preview pre code span.hljs-keyword", text: "def"
    assert_selector ".note-preview pre code span.hljs-string", text: /Hello/
  end

  # Sem as regras hljs-* no CSS os spans existem, mas herdam a cor do código e o destaque some.
  test "keywords and strings get a color of their own" do
    visit note_path("1")
    click_on "Read"

    code_color = find(".note-preview pre code", match: :first).style("color")["color"]
    %w[keyword string].each do |token|
      token_color = find(".note-preview pre code span.hljs-#{token}", match: :first).style("color")["color"]
      assert_not_equal code_color, token_color, "expected .hljs-#{token} to be colored apart from the code, both are #{code_color}"
    end
  end

  test "an unknown or missing language shows the code as plain text" do
    visit note_path("1")
    click_on "Read"

    assert_selector ".note-preview pre code", text: "def plain"
    assert_selector ".note-preview pre code", text: "def bare"
    assert_no_selector ".note-preview pre code span", text: /plain|bare/
  end

  test "side by side highlights a ruby block as it is typed" do
    visit note_path("2")
    click_on "Side by side"
    find(".cm-content").send_keys("```ruby", :enter, "def typed", :enter, "```")

    assert_selector ".note-preview pre code span.hljs-keyword", text: "def"
  end
end
