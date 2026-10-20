require "application_system_test_case"

# Comandos Ex do markupp no modo Vim (editor/vim_mode.js): :w, :wq, :q, :q!, :e caminho, :read, :edit e :split.
class VimExCommandsTest < ApplicationSystemTestCase
  setup do
    @api = use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "abc" }, { id: "2", path: "work/ideas.md", content: "" } ])
    sign_in_through_form
    page.driver.browser.manage.add_cookie(name: "vim_keys", value: "on")
    visit note_path("1")
    assert_selector ".cm-vim-panel", text: "NORMAL"
    find(".cm-content").click
  end

  test ":w saves like ctrl+s" do
    send_keys "A", "!", :escape
    assert_text "Unsaved changes"
    ex ":w"

    assert_text "Saved"
    assert_equal "abc!", @api.put_requests.sole[:content]
  end

  test ":wq saves and goes to the default note" do
    send_keys "A", "!", :escape
    ex ":wq"

    assert_text "Saved"
    assert_current_path root_path
    assert_equal "abc!", @api.put_requests.sole[:content]
  end

  test ":q without changes goes to the default note" do
    ex ":q"

    assert_current_path root_path
    assert_field "Path", with: "/"
  end

  test ":q with changes asks before leaving and stays when refused" do
    send_keys "A", "!", :escape

    dismiss_confirm("You have unsaved changes. Leave anyway?") { ex ":q" }
    assert_current_path note_path("1")
    accept_confirm("You have unsaved changes. Leave anyway?") { ex ":q" }

    assert_current_path root_path
    assert_empty @api.put_requests
  end

  test ":q! leaves without asking and drops the changes" do
    send_keys "A", "!", :escape
    ex ":q!"

    assert_current_path root_path
    assert_empty @api.put_requests
  end

  test ":e opens the note at a path relative to the folder of the open note" do
    ex ":e ideas"

    assert_current_path note_path("2")
  end

  test ":e with the path of no note opens a new note with that path" do
    ex ":e /2026/goals"

    assert_field "Path", with: "/2026/goals"
    assert_title "New note"
    assert_empty @api.post_requests
  end

  test ":read, :split and :edit switch the mode of the note" do
    ex ":split"
    assert_selector ".note-form[data-mode=split]"
    ex ":edit"
    assert_selector ".note-form[data-mode=edit]"
    ex ":read"

    assert_selector ".note-form[data-mode=read]"
  end

  private
    # O : abre a linha de comando do Vim no rodape do editor; Enter roda o comando. O : vai pelo CDP: o chromedriver
    # manda a pontuacao com key vazio, e o Vim leria outra tecla.
    def ex(command)
      press_through_cdp(":", "Semicolon", keycode: 186, modifiers: 8)
      assert_selector ".cm-vim-panel input:focus"
      send_keys command.delete_prefix(":"), :enter
    end
end
