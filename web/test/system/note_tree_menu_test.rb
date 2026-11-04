require "application_system_test_case"

# Menu de contexto da árvore, como o do explorador do VS Code: botão direito, Shift+F10 ou a tecla Menu na linha.
# Nova nota sem botão: Alt+N de qualquer lugar ou "New note here" no menu.
class NoteTreeMenuTest < ApplicationSystemTestCase
  NOTES = [ { id: "1", path: "work/2026/goals.md", content: "# Goals\n" }, { id: "2", path: "work/plan.md", content: "# Plan\n" },
            { id: "3", path: "ideas.md", content: "# Ideas\n" }, { id: "4", path: "cost$&.md", content: "# Cost\n" } ].freeze
  NOTE_ITEMS = [ "New note here", "Rename / move", "Delete", "Copy path", "Copy wikilink" ].freeze

  setup do
    use_fake_api(notes: NOTES)
    sign_in_through_form
  end

  # Aberto pelo mouse, nenhum item ganha o foco (Enter logo depois não roda nada); a seta para baixo vai ao primeiro e
  # a para cima ao último.
  test "a right click on a note opens its menu with no item focused, the arrows move, esc closes and the focus goes back to the row" do
    visit note_path("1")
    row("plan").right_click

    assert_equal NOTE_ITEMS, menu_items
    assert_no_selector "[role=menuitem]:focus"
    send_keys :enter
    assert_selector "[role=menu]"
    send_keys :down
    assert_focused_item "New note here"
    send_keys :down
    assert_focused_item "Rename / move"
    send_keys :up, :up
    assert_focused_item "Copy wikilink"
    send_keys :escape

    assert_no_selector "[role=menu]"
    assert_equal "plan", evaluate_script("document.activeElement.textContent.trim().split(' ').pop()")
  end

  # Como o contorno do VS Code: a linha do menu se distingue da nota aberta enquanto o menu está aberto.
  test "the row the menu acts on is outlined while the menu is open" do
    visit note_path("2")
    row("ideas").right_click

    assert_selector "[role=treeitem][data-menu-open]", count: 1, text: "ideas"
    assert_equal "solid", row("ideas").evaluate_script("getComputedStyle(this).outlineStyle")
    send_keys :escape

    assert_no_selector "[role=treeitem][data-menu-open]"
  end

  test "shift+f10 and the menu key open the same menu on the focused row" do
    visit note_path("1")
    focus_row "plan"
    send_keys [ :shift, :f10 ]
    assert_equal NOTE_ITEMS, menu_items
    assert_focused_item "New note here"
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

  # O Delete da própria nota aberta também se chama Delete (pelo title): o clique é no item do menu. A pergunta diz o
  # caminho da linha, não o da nota aberta.
  test "delete asks first with the path of the row, in the danger color, and removes the note" do
    visit note_path("1")
    row("ideas").right_click
    assert_selector "[role=menuitem].btn-danger", text: "Delete"
    dismiss_panel_confirm("Delete /ideas? This cannot be undone.") { find("[role=menuitem]", text: "Delete").click }
    assert_selector "[role=treeitem]", text: "ideas"

    row("ideas").right_click
    accept_panel_confirm("Delete /ideas? This cannot be undone.") { find("[role=menuitem]", text: "Delete").click }

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

  # O caminho entra na mensagem como texto: o $& de um nome não vira padrão de substituição.
  test "the copied message shows a path with $& as it is" do
    grant_clipboard
    visit note_path("1")
    row("cost$&").right_click
    click_on "Copy path"

    assert_selector "[role=status]", text: "Copied /cost$&"
    assert_equal "/cost$&", clipboard
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
    accept_panel_confirm("Delete /ideas? This cannot be undone.") { send_keys :delete }
    assert_no_selector "[role=treeitem]", text: "ideas"
  end

  test "alt+n opens a new note in the folder of the open note from anywhere, the editor too" do
    visit note_path("1")
    find(".cm-content").send_keys([ :alt, "n" ])

    assert_field "Path", with: "/work/2026/", focused: true
  end

  # A entrada .layer começa menor (popin): medido no meio dela, o menu achava que cabia e vazava da tela do celular.
  test "on a phone with motion the menu opened near the right edge ends inside the screen" do
    emulate_motion_preference("no-preference")
    visit note_path("1")
    page.current_window.resize_to(390, 844)
    find("[popovertarget=sidebar]").click
    row("plan").execute_script("this.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, cancelable: true, clientX: 380, clientY: 200 }))")
    assert_selector("#tree_menu:popover-open") { |menu| menu.evaluate_script("this.getAnimations().length == 0") }

    assert_operator evaluate_script("document.querySelector('#tree_menu').getBoundingClientRect().right"), :<=, 390 - 4
  ensure
    page.current_window.resize_to(1400, 900)
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

    def clipboard
      evaluate_async_script("navigator.clipboard.readText().then(arguments[0])")
    end
end
