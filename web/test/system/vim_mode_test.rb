require "application_system_test_case"

# Modo Vim do editor (editor/vim_mode.js), ligado em Configuracoes > Aparencia > Editor: modos normal, insert e visual,
# atalhos do Vim com o foco no editor e Esc Esc antes do Tab para sair dele.
class VimModeTest < ApplicationSystemTestCase
  include ContrastAssertions

  setup do
    @api = use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "abc\ndef" } ])
    sign_in_through_form
  end

  test "vim keys turn on from the appearance and the editor opens in normal mode with a block cursor" do
    visit settings_appearance_path
    assert_selector "main button[aria-pressed=false]", text: "Vim keys"
    within("main") { click_on "Vim keys" }
    assert_selector "main button[aria-pressed=true]", text: "Vim keys"

    visit note_path("1")

    assert_selector ".cm-vim-panel", text: "NORMAL"
    find(".cm-content").click
    assert_selector ".cm-fat-cursor"
  end

  test "hjkl move in normal mode, i writes and Esc goes back to normal" do
    open_note_with_vim_keys

    send_keys "gg0j"
    send_keys "l", "i"
    assert_selector ".cm-vim-panel", text: "INSERT"
    send_keys "X", :escape
    assert_selector ".cm-vim-panel", text: "NORMAL"
    send_keys "k", "h", "i", "Y", :escape

    assert_selector ".cm-content", text: "Yabc"
    assert_selector ".cm-content", text: "dXef"
  end

  test "v shows the visual mode" do
    open_note_with_vim_keys

    send_keys "v"

    assert_selector ".cm-vim-panel", text: "VISUAL"
  end

  test "ctrl+b with the focus in the editor belongs to vim, outside it toggles the sidebar" do
    open_note_with_vim_keys

    send_keys [ :control, "b" ]
    send_keys "i", [ :control, "b" ], :escape
    find("body").send_keys([ :control, "b" ])

    # Se o editor tivesse deixado passar um dos Ctrl+B, a barra teria aberto e fechado de novo.
    assert_selector "html[data-sidebar=closed]"
  end

  test "esc in insert mode goes to normal, a second esc lets tab leave the editor" do
    open_note_with_vim_keys

    send_keys "i", :escape, :tab
    assert_selector ".cm-content:focus"
    send_keys :escape, :tab

    assert_no_selector ".cm-content:focus"
    assert_equal "abc\ndef", note_content
  end

  test "esc esc shift+tab goes back to the path" do
    open_note_with_vim_keys

    send_keys :escape, :escape, [ :shift, :tab ]

    assert_selector "input[name='note[path]']:focus"
  end

  test "tab in insert mode still writes a tab" do
    open_note_with_vim_keys

    send_keys "gg0i", :tab, :escape

    assert_selector ".cm-vim-panel", text: "NORMAL"
    assert_equal "\tabc\ndef", note_content
  end

  test "the mode and the block cursor read clearly in the light and the dark theme" do
    %w[light dark].each do |theme|
      page.driver.browser.manage.add_cookie(name: "theme", value: theme)
      open_note_with_vim_keys

      assert_readable find(".cm-vim-panel span", text: "NORMAL")
      assert_readable find(".cm-fat-cursor")
    end
  end

  test "with vim keys off the editor keeps its keys and never downloads vim" do
    visit note_path("1")
    find(".cm-content").click
    send_keys [ :control, :home ], "j"

    assert_selector ".cm-content", text: "jabc"
    assert_no_selector ".cm-vim-panel"
    assert_equal 0, evaluate_script("performance.getEntriesByType('resource').filter((entry) => entry.name.includes('codemirror-vim')).length")
  end

  private
    # O textarea escondido segue o editor a cada mudanca (editor_controller.js).
    def note_content = evaluate_script("document.querySelector('textarea[name=\"note[content]\"]').value")

    def open_note_with_vim_keys(id = "1")
      page.driver.browser.manage.add_cookie(name: "vim_keys", value: "on")
      visit note_path(id)
      assert_selector ".cm-vim-panel", text: "NORMAL"
      find(".cm-content").click
    end
end
