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
