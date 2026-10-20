require "application_system_test_case"

# Alt+h/j/k/l levam o foco a regiao mais proxima naquela direcao pela geometria da tela, como o navigateLeft/Down/
# Up/Right do VS Code: arvore, caminho, editor, preview e a lista principal (tenants, configuracoes).
class RegionNavigationTest < ApplicationSystemTestCase
  setup do
    @api = use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "abc" }, { id: "2", path: "ideas.md", content: "# Ideas\n" } ])
    sign_in_through_form
  end

  test "side by side, alt+h and alt+l walk the tree, the editor and the preview" do
    open_note(mode: "split")

    send_keys [ :alt, "h" ]
    assert_selector "[role=treeitem][aria-current=page]:focus"
    send_keys [ :alt, "l" ]
    assert_selector ".cm-content:focus"
    send_keys [ :alt, "l" ]
    assert_selector ".note-preview:focus"
    send_keys [ :alt, "h" ]
    assert_selector ".cm-content:focus"
  end

  test "alt+k goes up to the path from the editor and from the preview" do
    open_note(mode: "split")

    send_keys [ :alt, "k" ]
    assert_selector "input[name='note[path]']:focus"
    find(".cm-content").click
    send_keys [ :alt, "l" ]
    assert_selector ".note-preview:focus"
    send_keys [ :alt, "k" ]

    assert_selector "input[name='note[path]']:focus"
  end

  test "alt+j goes down from the editor to the preview when they stack" do
    page.current_window.resize_to(900, 900)
    open_note(mode: "split")

    send_keys [ :alt, "j" ]

    assert_selector ".note-preview:focus"
  ensure
    page.current_window.resize_to(1400, 900)
  end

  test "back in the editor the cursor is where it was and alt+hjkl wrote nothing" do
    open_note(mode: "edit")

    send_keys [ :control, :end ], [ :alt, "h" ], [ :alt, "l" ], "X"

    assert_equal "abcX", note_content
  end

  test "in reading the tree goes right to the preview" do
    open_note(mode: "read")

    send_keys [ :control, :shift, "e" ], [ :alt, "l" ]

    assert_selector ".note-preview:focus"
  end

  test "with no region in that direction the focus stays" do
    open_note(mode: "split")

    send_keys [ :alt, "l" ], [ :alt, "l" ]
    assert_selector ".note-preview:focus"
    send_keys [ :alt, "h" ], [ :alt, "h" ], [ :alt, "k" ]

    assert_selector "[role=treeitem][aria-current=page]:focus"
  end

  test "with the sidebar closed alt+h stays in the editor" do
    page.driver.browser.manage.add_cookie(name: "sidebar", value: "closed")
    open_note(mode: "edit")

    send_keys [ :control, :end ], [ :alt, "h" ], "X"

    assert_selector ".cm-content:focus"
    assert_equal "abcX", note_content
  end

  # O chromedriver manda o Alt+h com o texto h, que o navegador num teclado de verdade nao escreve: o teste so exige
  # que o foco fique e o que vem depois caia no campo.
  test "the path field keeps alt+h and does not move" do
    open_note(mode: "edit")

    find_field("Path").click
    send_keys :end, [ :alt, "h" ], "x"

    assert_field "Path", with: %r{\A/work/plan.*x\z}
  end

  test "with vim keys, insert mode keeps the keys and normal mode moves" do
    page.driver.browser.manage.add_cookie(name: "vim_keys", value: "on")
    open_note(mode: "edit")
    assert_selector ".cm-vim-panel", text: "NORMAL"

    send_keys "A", [ :alt, "h" ], "Z"
    assert_selector ".cm-content:focus"
    assert_match(/\Aabc.*Z\z/, note_content)
    send_keys :escape, [ :alt, "h" ]

    assert_selector "[role=treeitem][aria-current=page]:focus"
  end

  test "the tenants list and the settings are regions next to the tree" do
    visit tenants_path
    send_keys [ :control, :shift, "e" ], [ :alt, "l" ]
    assert_selector ".tenant-card-open[aria-current=true]:focus"
    send_keys [ :alt, "h" ]
    assert_selector "[role=treeitem]:focus"

    visit settings_appearance_path
    send_keys [ :control, :shift, "e" ], [ :alt, "l" ]
    assert_selector "nav a[aria-current=page]:focus", text: "Appearance"
  end

  test "an open context menu keeps alt+l and stays the only place with the focus" do
    open_note(mode: "split")

    find("nav [role=treeitem]", text: "plan").right_click
    assert_selector "[role=menu]"
    send_keys [ :alt, "l" ]

    assert_no_selector ".cm-content:focus"
    assert_selector "[role=menu]:focus-within, [role=menu]:focus"
  end

  test "the palette runs the move from where the focus was" do
    open_note(mode: "edit")

    send_keys [ :control, :shift, "p" ]
    send_keys "focus region left", :enter

    assert_selector "[role=treeitem][aria-current=page]:focus"
  end

  private
    def note_content = evaluate_script("document.querySelector('textarea[name=\"note[content]\"]').value")

    def open_note(mode:)
      page.driver.browser.manage.add_cookie(name: "note_mode", value: mode)
      visit note_path("1")
      find(mode == "read" ? ".note-preview" : ".cm-content").click
    end
end
