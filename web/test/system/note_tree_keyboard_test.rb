require "application_system_test_case"

# Árvore de notas pelo teclado, como no explorador do VS Code e no NERDTree: um Tab só para a árvore, setas ou j/k,
# h/l para fechar, abrir e ir ao pai ou ao filho, Enter para abrir a nota e Ctrl+Shift+E de qualquer lugar.
class NoteTreeKeyboardTest < ApplicationSystemTestCase
  NOTES = [ { id: "1", path: "work/2026/goals.md", content: "# Goals\n" }, { id: "2", path: "work/plan.md", content: "# Plan\n" },
            { id: "3", path: "home/list.md", content: "" }, { id: "4", path: "ideas.md", content: "" } ].freeze

  setup do
    use_fake_api(notes: NOTES)
    sign_in_through_form
  end

  test "the tree takes a single tab stop, on the current note" do
    visit note_path("1")

    assert_selector "[role=tree] [role=treeitem][tabindex='0']", count: 1
    assert_selector "[role=tree] [role=treeitem][tabindex='0'][aria-current=page]", text: "goals"
  end

  test "the arrows and j and k move between the visible rows and carry the tab stop" do
    visit note_path("1")
    focus_row "goals"

    send_keys :down
    assert_focused "│   └── plan"
    send_keys "j"
    assert_focused "└── ideas"
    send_keys "k", :up
    assert_focused "│   │   └── goals"
    send_keys :up, :up, :up
    assert_focused "├── home/"
    assert_selector "[role=treeitem][tabindex='0']", count: 1, text: "home/"
  end

  test "h and l or the side arrows close and open folders and move to the parent or the first child" do
    visit note_path("1")
    focus_row "goals"

    send_keys "h"
    assert_focused "│   ├── 2026/"
    send_keys "h"
    assert_selector "[role=treeitem][aria-expanded=false]", text: "2026/"
    send_keys :left
    assert_focused "├── work/"
    send_keys :left
    assert_selector "[role=treeitem][aria-expanded=false]", text: "work/"

    send_keys :right
    assert_selector "[role=treeitem][aria-expanded=true]", text: "work/"
    send_keys "l"
    assert_focused "│   ├── 2026/"
    send_keys "l", "l"
    assert_focused "│   │   └── goals"
  end

  test "enter opens the focused note" do
    visit note_path("1")
    focus_row "goals"

    send_keys :down, :enter

    assert_current_path note_path("2")
    assert_selector "[role=treeitem][aria-current=page]", text: "plan"
  end

  test "ctrl+shift+e brings the focus to the tree from the editor, also with the sidebar hidden" do
    visit note_path("1")
    assert_selector "[role=tree][aria-keyshortcuts='Control+Shift+E Meta+Shift+E']"

    find(".cm-content").send_keys([ :control, :shift, "e" ])
    assert_focused "│   │   └── goals"

    click_on "Hide sidebar"
    assert_no_selector "nav#sidebar"
    find(".cm-content").send_keys([ :control, :shift, "e" ])
    assert_selector "nav#sidebar"
    assert_focused "│   │   └── goals"
    assert_equal "# Goals\n", find("textarea", visible: :hidden).value
  end

  private
    def focus_row(name)
      find("[role=treeitem]", text: name).evaluate_script("this.focus()")
    end

    def assert_focused(row)
      assert_selector "[role=treeitem]:focus"
      assert_equal row, evaluate_script("document.activeElement.textContent.trim()")
    end
end
