require "application_system_test_case"

class CodeHighlightTest < ApplicationSystemTestCase
  NOTE_WITH_CODE = <<~MARKDOWN.freeze
    ```ruby
    def greet(name)
      "Hello, \#{name}"
    end
    ```

    ```js
    const answer = "forty" + 2
    ```

    ```nosuchlang
    def plain
    ```

    ```
    def bare
    ```
  MARKDOWN

  # Linguagens fora do bundle inicial: baixam só quando um bloco delas aparece.
  NOTE_WITH_ON_DEMAND_CODE = <<~MARKDOWN.freeze
    ```java
    public class Hello {}
    ```

    ```rs
    fn main() {}
    ```

    ```kotlin
    fun greet() = 1
    ```
  MARKDOWN

  ON_DEMAND_LANGUAGE_MODULE = %r{highlightjs-languages/(java|rust|kotlin)-|legacy-modes--mode--(clike|rust)-}

  setup do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: NOTE_WITH_CODE }, { id: "2", path: "blank.md", content: "" },
                          { id: "3", path: "langs.md", content: NOTE_WITH_ON_DEMAND_CODE },
                          { id: "4", path: "ml.md", content: "```ml\nlet x = 1\n```\n" } ])
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

  # No editor o bloco cercado ganha as cores da leitura (--code-*), com a linguagem carregada sob demanda.
  test "editing highlights the language inside a fenced block" do
    visit note_path("1")

    assert_editor_token "def", "--code-keyword"
    assert_editor_token "const", "--code-keyword"
    assert_editor_token "\"forty\"", "--code-string"
    assert_editor_token "2", "--code-number"
  end

  test "editing leaves an unknown language as plain text" do
    visit note_path("1")

    assert_editor_token "def", "--code-keyword"
    within(find(".cm-line", text: "def plain", exact_text: true)) { assert_no_selector "span" }
  end

  test "side by side highlights a ruby block as it is typed" do
    visit note_path("2")
    click_on "Side by side"
    find(".cm-content").send_keys("```ruby", :enter, "def typed", :enter, "```")

    assert_selector ".note-preview pre code span.hljs-keyword", text: "def"
  end

  test "reading highlights java, rust by its rs alias and kotlin, loaded on demand" do
    visit note_path("3")
    click_on "Read"

    %w[public fn fun].each { |keyword| assert_selector ".note-preview pre code span.hljs-keyword", text: keyword, exact_text: true }
    assert_equal [ "Java", "Rust", "Kotlin" ], all(".note-preview pre .code-header .code-language", count: 3).map(&:text)
  end

  # ml é apelido do OCaml e do SML; o painel lê como OCaml, o mais usado, na leitura e no editor.
  test "a block fenced as ml reads as ocaml" do
    visit note_path("4")
    click_on "Read"

    assert_selector ".note-preview pre .code-header .code-language", text: "OCaml", exact_text: true
  end

  test "editing highlights java, rust by its rs alias and kotlin, loaded on demand" do
    visit note_path("3")

    %w[public fn fun].each { |keyword| assert_editor_token keyword, "--code-keyword" }
  end

  # Abrir uma nota sem esses blocos não baixa nem pré-carrega os módulos; a nota que os usa baixa.
  test "language modules download only for a note that uses them" do
    visit note_path("1")
    click_on "Read"
    assert_selector ".note-preview pre code span.hljs-keyword", text: "def", match: :first
    assert_empty on_demand_language_resources

    visit note_path("3")
    click_on "Side by side"
    assert_selector ".note-preview pre code span.hljs-keyword", text: "fn"
    assert_editor_token "fn", "--code-keyword"
    assert_equal %w[clike java kotlin rust], on_demand_language_resources.map { |name| name[/(clike|java|kotlin|rust)-/, 1] }.compact.uniq.sort
  end

  private
    # Módulos das linguagens sob demanda que a página baixou ou pôs em modulepreload.
    def on_demand_language_resources
      names = evaluate_script(<<~JS)
        [ ...performance.getEntriesByType("resource").map((entry) => entry.name),
          ...[ ...document.querySelectorAll("link[rel=modulepreload]") ].map((link) => link.href) ]
      JS
      names.grep(ON_DEMAND_LANGUAGE_MODULE)
    end

    # Cor do token no editor igual à da variável, medida num elemento de prova dentro do editor (mesmo tema).
    def assert_editor_token(text, variable)
      expected = evaluate_script(<<~JS, variable)
        (() => {
          const probe = document.createElement("span")
          probe.style.color = `var(${arguments[0]})`
          document.querySelector(".cm-content").append(probe)
          const color = getComputedStyle(probe).color
          probe.remove()
          return color
        })()
      JS
      assert_selector(".cm-line span", text:, exact_text: true) do |token|
        evaluate_script("getComputedStyle(arguments[0]).color", token) == expected
      end
    end
end
