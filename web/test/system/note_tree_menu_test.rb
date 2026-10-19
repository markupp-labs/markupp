require "application_system_test_case"

# Menu de contexto da árvore, como o do explorador do VS Code: botão direito, Shift+F10 ou a tecla Menu na linha.
# Nova nota sem botão: Alt+N de qualquer lugar ou "New note here" no menu.
class NoteTreeMenuTest < ApplicationSystemTestCase
  NOTES = [ { id: "1", path: "work/2026/goals.md", content: "# Goals\n" }, { id: "2", path: "work/plan.md", content: "# Plan\n" },
            { id: "3", path: "ideas.md", content: "# Ideas\n" } ].freeze
  NOTE_ITEMS = [ "New note here", "Rename / move", "Delete", "Copy path", "Copy wikilink" ].freeze

  setup do
    use_fake_api(notes: NOTES)
    sign_in_through_form
  end

  test "a right click on a note opens its menu, the arrows move, esc closes and the focus goes back to the row" do
    visit note_path("1")
    row("plan").right_click

    assert_equal NOTE_ITEMS, menu_items
    assert_focused_item "New note here"
    send_keys :down
    assert_focused_item "Rename / move"
    send_keys :up, :up
    assert_focused_item "Copy wikilink"
    send_keys :escape

    assert_no_selector "[role=menu]"
    assert_equal "plan", evaluate_script("document.activeElement.textContent.trim().split(' ').pop()")
  end

  test "shift+f10 and the menu key open the same menu on the focused row" do
    visit note_path("1")
    focus_row "plan"
    send_keys [ :shift, :f10 ]
    assert_equal NOTE_ITEMS, menu_items
    send_keys :escape
    assert_no_selector "[role=menu]"

    execute_script("document.activeElement.dispatchEvent(new KeyboardEvent('keydown', { key: 'ContextMenu', bubbles: true, cancelable: true }))")
    assert_equal NOTE_ITEMS, menu_items
  end

  test "a folder offers a new note in it and its path" do
    visit note_path("1")
    row("2026/").right_click

    assert_equal [ "New note here", "Copy path" ], menu_items
  end

  test "new note here opens a new note in the folder clicked or in the folder of the note" do
    visit note_path("1")
    row("2026/").right_click
    click_on "New note here"
    assert_field "Path", with: "/work/2026/", focused: true

    row("plan").right_click
    click_on "New note here"
    assert_field "Path", with: "/work/", focused: true
  end

  test "rename / move opens the note with its path focused and selected" do
    visit note_path("1")
    row("plan").right_click
    click_on "Rename / move"

    assert_selector "[role=treeitem][aria-current=page]", text: "plan"
    assert_path_selected "/work/plan"

    row("plan").right_click
    click_on "Rename / move"
    assert_path_selected "/work/plan"
  end

  test "delete asks first, in the danger color, and removes the note" do
    visit note_path("1")
    row("ideas").right_click
    assert_selector "[role=menuitem].btn-danger", text: "Delete"
    dismiss_confirm { click_on "Delete" }
    assert_selector "[role=treeitem]", text: "ideas"

    row("ideas").right_click
    accept_confirm("Delete this note? This cannot be undone.") { click_on "Delete" }

    assert_text "Deleted"
    assert_no_selector "[role=treeitem]", text: "ideas"
  end

  test "copy path and copy wikilink put the path without .md and the link by name on the clipboard" do
    grant_clipboard
    visit note_path("1")
    row("plan").right_click
    click_on "Copy path"
    assert_selector "[role=status]", text: "Copied /work/plan"
    assert_equal "/work/plan", clipboard

    row("plan").right_click
    click_on "Copy wikilink"
    assert_selector "[role=status]", text: "Copied [[plan]]"
    assert_equal "[[plan]]", clipboard
  end

  test "each action has its key on the focused row, shown in the menu" do
    grant_clipboard
    visit note_path("1")
    row("plan").right_click
    assert_equal [ "Alt+N", "F2", "Delete", "Alt+Shift+C", "Alt+Shift+W" ], all("[role=menuitem]").map { it["aria-keyshortcuts"] }
    send_keys :escape

    focus_row "plan"
    send_keys [ :alt, :shift, "c" ]
    assert_selector "[role=status]", text: "Copied /work/plan"
    assert_equal "/work/plan", clipboard
    send_keys [ :alt, :shift, "w" ]
    assert_selector "[role=status]", text: "Copied [[plan]]"
    assert_equal "[[plan]]", clipboard
    send_keys :f2
    assert_path_selected "/work/plan"

    focus_row "ideas"
    accept_confirm { send_keys :delete }
    assert_no_selector "[role=treeitem]", text: "ideas"
  end

  test "alt+n opens a new note in the folder of the open note from anywhere, the editor too" do
    visit note_path("1")
    find(".cm-content").send_keys([ :alt, "n" ])

    assert_field "Path", with: "/work/2026/", focused: true
  end

  private
    def row(name)
      find("[role=treeitem]", text: name)
    end

    def focus_row(name)
      row(name).evaluate_script("this.focus()")
    end

    def menu_items
      find("[role=menu]")
      all("[role=menu] [role=menuitem]").map { it.native.accessible_name }
    end

    def assert_focused_item(name)
      assert_selector "[role=menuitem]:focus"
      assert_equal name, find("[role=menuitem]:focus").native.accessible_name
    end

    def assert_path_selected(path)
      assert_field "Path", with: path, focused: true
      assert_equal [ 0, path.length ], evaluate_script("[ document.activeElement.selectionStart, document.activeElement.selectionEnd ]")
    end

    def grant_clipboard
      page.driver.browser.execute_cdp("Browser.grantPermissions", origin: Capybara.current_session.server.base_url,
                                                                  permissions: %w[clipboardReadWrite clipboardSanitizedWrite])
    end

    def clipboard
      evaluate_async_script("navigator.clipboard.readText().then(arguments[0])")
    end
end
