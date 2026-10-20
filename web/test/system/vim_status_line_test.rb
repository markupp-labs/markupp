require "application_system_test_case"

# Linha do modo Vim (--NORMAL--) como a statusline do Vim: no fim da janela, com o espaco dela reservado (a nota rola
# acima dela, sem a ultima linha ficar por baixo), e so com o editor visivel.
class VimStatusLineTest < ApplicationSystemTestCase
  setup do
    long = (1..120).map { "line #{it}" }.join("\n")
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "abc" }, { id: "2", path: "long.md", content: long } ])
    sign_in_through_form
    page.driver.browser.manage.add_cookie(name: "vim_keys", value: "on")
  end

  test "on a short note the mode sits at the bottom of the window, also side by side" do
    open_note("1")
    assert_status_line_at_bottom

    find(".note-mode-split").click
    assert_selector ".note-form[data-mode=split]"
    assert_status_line_at_bottom
  end

  test "on a long note the last line ends above the mode, which stays at the bottom of the window" do
    open_note("2")
    assert_status_line_at_bottom
    send_keys "G"

    assert_selector ".cm-line", text: "line 120"
    assert_status_line_at_bottom
    last_line_bottom = evaluate_script("[ ...document.querySelectorAll('.cm-line') ].at(-1).getBoundingClientRect().bottom")
    assert_operator last_line_bottom, :<=, status_line_box["top"], "expected the last line above the mode line"
  end

  test "at phone width the mode sits at the bottom of the window" do
    page.current_window.resize_to(390, 844)
    open_note("1")

    assert_status_line_at_bottom
  ensure
    page.current_window.resize_to(1400, 900)
  end

  test "the read mode has no mode line" do
    open_note("1")
    find(".note-mode-read").click

    assert_no_selector ".cm-vim-panel"
  end

  private
    def open_note(id)
      visit note_path(id)
      assert_selector ".cm-vim-panel", text: "NORMAL"
      find(".cm-content").click
    end

    def status_line_box
      evaluate_script(<<~JS)
        (() => {
          const { top, bottom } = document.querySelector(".cm-panels-bottom").getBoundingClientRect()
          return { top, bottom, viewport: document.documentElement.clientHeight }
        })()
      JS
    end

    def assert_status_line_at_bottom
      box = status_line_box
      assert_in_delta box["viewport"], box["bottom"], 1, "expected the mode line at the bottom of the window, got #{box.inspect}"
    end
end
