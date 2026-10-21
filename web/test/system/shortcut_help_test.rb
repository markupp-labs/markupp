require "application_system_test_case"

# "?" fora de campo de texto abre a ajuda de atalhos, como no GitHub e no Gmail; Esc fecha. O ? vai pelo CDP
# (press_through_cdp).
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

  # Como num --help de terminal: em cada coluna da ajuda, as descrições começam no mesmo x em todos os grupos.
  test "the descriptions line up in one column per help column" do
    find("[role=treeitem]", text: "plan").execute_script("this.focus()")
    press_question_mark
    assert_selector "dialog#shortcut_help[open]"

    columns = evaluate_script("[ ...document.querySelectorAll('#shortcut_help dd') ].map((dd) => Math.round(dd.getBoundingClientRect().left))").uniq
    assert_equal 2, columns.size, "expected one description column per help column, found x = #{columns.sort}"
  end

  test "with vim keys on the help lists the vim commands" do
    page.driver.browser.manage.add_cookie(name: "vim_keys", value: "on")
    visit note_path("1")
    find("[role=treeitem]", text: "plan").execute_script("this.focus()")
    press_question_mark

    within("dialog#shortcut_help[open]") do
      assert_selector "h3", text: "Vim"
      assert_selector "kbd", text: ":wq"
      [ "Save and quit", "Quit to the default note", "Quit, dropping the changes", "Open or start a note", "Back to the path" ].each do |name|
        assert_selector "dd", exact_text: name
      end
    end
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
    def press_question_mark = press_through_cdp("?", "Slash", keycode: 191, modifiers: 8)
end
