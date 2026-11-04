require "application_system_test_case"

# Ferramentas dos blocos de código na leitura: copiar, número de linha e rolagem horizontal.
class CodeBlocksTest < ApplicationSystemTestCase
  RUBY = "def greet(name)\n  \"Hello, \#{name}\"\nend\n".freeze
  PLAIN = "just words\n".freeze
  LONG = "const line = \"#{'x' * 300}\"\n".freeze

  setup do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "```ruby\n#{RUBY}```\n\n```\n#{PLAIN}```\n\n```js\n#{LONG}```\n" } ])
    sign_in_through_form
    visit note_path("1")
    click_on "Read"
  end

  test "the copy button copies the code and says so" do
    grant_clipboard
    assert_selector ".note-preview pre button[aria-label='Copy code'][title='Copy code']", count: 3
    first(".note-preview pre button[aria-label='Copy code']").click

    assert_selector ".note-preview pre [role=status][aria-live=polite]", text: "Copied", count: 1
    assert_equal RUBY, evaluate_async_script("navigator.clipboard.readText().then(arguments[0])")
  end

  # Faixa de cabeçalho como a barra de título de um terminal: linguagem à esquerda pelo nome do highlight.js (js é
  # JavaScript; "text" sem linguagem), copiar à direita, acima do código e não por cima dele.
  test "each block has a header strip above the code with the language on the left and copy on the right" do
    assert_equal [ "Ruby", "text", "JavaScript" ], all(".note-preview pre .code-header .code-language", count: 3).map(&:text)

    all(".note-preview pre").each do |pre|
      layout = pre.evaluate_script(<<~JS)
        (() => { const box = (selector) => this.querySelector(selector).getBoundingClientRect(), pre = this.getBoundingClientRect()
                 const header = box(".code-header"), language = box(".code-language"), copy = box(".code-header button[aria-label='Copy code']")
                 return { header: header.bottom <= box(":scope > code").top, top: header.top - pre.top,
                          width: pre.width - header.width, order: language.right < copy.left, right: pre.right - copy.right } })()
      JS
      assert layout["header"], "expected the header above the code, got #{layout}"
      assert layout["order"], "expected the language left of the copy button, got #{layout}"
      assert_operator layout["top"], :<=, 1
      assert_operator layout["width"], :<=, 2
      assert_operator layout["right"], :<=, 12
    end
  end

  test "line numbers sit beside the code and stay out of the selection" do
    assert_selector ".note-preview pre .code-lines[aria-hidden=true]", count: 3
    assert_equal "1\n2\n3", first(".note-preview pre .code-lines").evaluate_script("this.textContent")

    selected = evaluate_script(<<~JS)
      (() => { const selection = getSelection(); selection.selectAllChildren(document.querySelector(".note-preview pre")); return selection.toString() })()
    JS
    assert selected.start_with?("def greet"), "expected the selection to start at the code, got #{selected.inspect}"
  end

  # A barra fica no código, não no pre: os números não saem de vista ao rolar.
  test "a long line scrolls inside the code with the theme scrollbar" do
    assert_selector ".note-preview pre code", count: 3
    code = all(".note-preview pre code").last
    scroll = code.evaluate_script(<<~JS)
      (() => { const pre = this.closest("pre"), probe = document.createElement("span")
               probe.style.color = "var(--line)"; pre.append(probe)
               const line = getComputedStyle(probe).color; probe.remove()
               return { code: this.scrollWidth > this.clientWidth, pre: pre.scrollWidth > pre.clientWidth,
                        color: getComputedStyle(this).scrollbarColor, line } })()
    JS

    assert scroll["code"], "expected the long line to scroll inside the code, got #{scroll}"
    assert_not scroll["pre"], "expected the pre itself not to scroll, got #{scroll}"
    assert_equal "#{scroll['line']} rgba(0, 0, 0, 0)", scroll["color"]
  end
end
