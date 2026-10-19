require "application_system_test_case"

# "?" fora de campo de texto abre a ajuda de atalhos, como no GitHub e no Gmail; Esc fecha. O ? vai pelo CDP: o
# chromedriver manda a pontuação com key vazio.
class ShortcutHelpTest < ApplicationSystemTestCase
  setup do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])
    sign_in_through_form
    visit note_path("1")
    assert_field "Path", with: "/plan"
  end

  test "? outside a text field opens the help and esc closes it" do
    find("[role=treeitem]", text: "plan").execute_script("this.focus()")
    press_question_mark

    assert_selector "dialog#shortcut_help[open]"
    within("#shortcut_help") do
      assert_selector "h2", text: "Keyboard shortcuts"
      assert_selector "kbd", text: "Ctrl+Shift+P"
    end
    send_keys :escape
    assert_no_selector "dialog#shortcut_help[open]"
  end

  test "? typed in the content or the path stays text" do
    find(".cm-content").send_keys([ :control, :end ], "Why")
    press_question_mark
    find_field("Path").send_keys(:end)
    press_question_mark

    assert_selector ".cm-content", text: "Why?"
    assert_field "Path", with: "/plan?"
    assert_no_selector "dialog#shortcut_help[open]"
  end

  private
    def press_question_mark
      %w[keyDown keyUp].each do |type|
        page.driver.browser.execute_cdp("Input.dispatchKeyEvent", type:, key: "?", code: "Slash", text: "?", modifiers: 8, windowsVirtualKeyCode: 191)
      end
    end
end
