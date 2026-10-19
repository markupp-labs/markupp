require "application_system_test_case"

# Blocos de código na leitura.
class CodeBlocksTest < ApplicationSystemTestCase
  RUBY = "def greet(name)\n  \"Hello, \#{name}\"\nend\n".freeze
  LONG = "const line = \"#{'x' * 300}\"\n".freeze

  setup do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "```ruby\n#{RUBY}```\n\n```js\n#{LONG}```\n" } ])
    sign_in_through_form
    visit note_path("1")
    click_on "Read"
  end

  test "line numbers sit beside the code and stay out of the selection" do
    assert_selector ".note-preview pre .code-lines[aria-hidden=true]", count: 2
    assert_equal "1\n2\n3", first(".note-preview pre .code-lines").evaluate_script("this.textContent")

    selected = evaluate_script(<<~JS)
      (() => { const selection = getSelection(); selection.selectAllChildren(document.querySelector(".note-preview pre")); return selection.toString() })()
    JS
    assert selected.start_with?("def greet"), "expected the selection to start at the code, got #{selected.inspect}"
  end

  # A barra fica no código, não no pre: o que mais morar no pre fica parado ao rolar.
  test "a long line scrolls inside the code with the theme scrollbar" do
    assert_selector ".note-preview pre code", count: 2
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
