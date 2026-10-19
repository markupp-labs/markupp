require "application_system_test_case"

class NoteModesTest < ApplicationSystemTestCase
  test "the mode buttons switch between edit, read and side by side" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n\nShip *it*.\n" } ])
    sign_in_through_form

    visit note_path("1")
    assert_selector "button[aria-label=Edit][aria-pressed=true]"
    click_on "Read"
    assert_selector ".note-preview h1", text: "Plan"
    assert_selector ".note-preview em", text: "it"
    assert_no_selector ".cm-editor"

    click_on "Side by side"
    assert_selector ".cm-editor"
    find(".cm-content").send_keys([ :control, :end ], "Now.")

    assert_selector ".note-preview p", text: "Now."
    assert_selector "button[aria-label='Side by side'][aria-pressed=true]"
  end

  test "the last mode survives a reload but a new note opens in edit" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])
    sign_in_through_form

    visit note_path("1")
    click_on "Read"
    refresh
    assert_selector ".note-preview h1", text: "Plan"

    click_on "New note"
    assert_selector ".cm-editor"
    assert_no_selector ".note-preview"
    assert_selector "button[aria-label=Edit][aria-pressed=true]"
  end

  test "ctrl+e toggles reading and ctrl+backslash toggles side by side, also from the editor" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])
    sign_in_through_form

    visit note_path("1")
    find(".cm-content").send_keys([ :control, "e" ])
    assert_selector ".note-preview h1", text: "Plan"
    assert_no_selector ".cm-editor"
    find("body").send_keys([ :control, "e" ])
    assert_no_selector ".note-preview"
    find(".cm-content").click
    press_ctrl_backslash

    assert_selector ".cm-editor"
    assert_selector ".note-preview h1", text: "Plan"
    assert_equal "# Plan\n", find("textarea", visible: :hidden).value
    assert_no_text "Unsaved changes"
  end

  test "the preview sanitizes html and runs no script from the markdown" do
    content = <<~MARKDOWN
      <script>window.pwned = "script"</script>

      <img src="https://example.invalid/pic.png" onerror="window.pwned = 'onerror'" alt="pic">

      [bad](javascript:window.pwned='link') and [site](https://example.com)
    MARKDOWN
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: } ])
    sign_in_through_form

    visit note_path("1")
    click_on "Read"

    assert_selector ".note-preview img[alt=pic]"
    assert_no_selector ".note-preview script, .note-preview [onerror], .note-preview a[href^=javascript]", visible: :all
    assert_selector ".note-preview a[href='https://example.com'][rel~=noopener][target=_blank]", text: "site"
    assert_nil evaluate_script("window.pwned")
  end

  test "tab in the title moves on when the editor is hidden in read mode" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])
    sign_in_through_form

    visit note_path("1")
    click_on "Read"
    find_field("Title").send_keys(:tab)

    assert_not_equal "note_title", evaluate_script("document.activeElement.id")
  end

  test "form controls in the markdown never join the note form" do
    content = <<~MARKDOWN
      # Plan

      <input type="hidden" name="_method" value="delete"><input type="hidden" name="note[content]" value="">
    MARKDOWN
    api = use_fake_api(notes: [ { id: "1", path: "plan.md", content: } ])
    sign_in_through_form

    visit note_path("1")
    click_on "Side by side"
    find(".cm-content").send_keys([ :control, :end ], "Kept.", [ :control, "s" ])

    assert_text "Saved"
    assert_equal "#{content}Kept.", api.put_requests.sole[:content]
    assert_no_selector ".note-preview input, .note-preview button", visible: :all
  end

  test "side by side is not offered at phone width" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])
    sign_in_through_form

    visit note_path("1")
    page.current_window.resize_to(390, 844)

    assert_button "Read"
    assert_no_button "Side by side"
  ensure
    page.current_window.resize_to(1400, 900)
  end

  test "side by side stacks by the width of the content, not of the window" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])
    sign_in_through_form

    visit note_path("1")
    click_on "Side by side"
    page.current_window.resize_to(1024, 768)
    assert_preview_below_editor

    within("main") { click_on "Hide sidebar" }
    assert_preview_beside_editor
  ensure
    page.current_window.resize_to(1400, 900)
  end

  test "side by side scrolls the preview along with the editor" do
    filler = ->(part, count) { (1..count).map { |n| "#{part} paragraph #{n}." }.join("\n\n") }
    content = "# Plan\n\n#{filler.('Opening', 12)}\n\n## Middle\n\n#{filler.('Closing', 40)}\n"
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: } ])
    sign_in_through_form

    visit note_path("1")
    click_on "Side by side"
    scroll_editor_to_line "## Middle"

    assert_selector(".note-preview h2", text: "Middle") { |heading| near_preview_top?(heading) }
  end

  private
    def pane_boxes
      evaluate_script(<<~JS)
        [".note-editor-pane", ".note-preview"].map((selector) => document.querySelector(selector).getBoundingClientRect().toJSON())
      JS
    end

    def assert_preview_below_editor
      editor, preview = pane_boxes
      assert_operator preview["top"], :>=, editor["bottom"] - 1, "expected the preview under the editor, got editor #{editor} and preview #{preview}"
    end

    def assert_preview_beside_editor
      editor, preview = pane_boxes
      assert_operator preview["left"], :>=, editor["right"] - 1, "expected the preview right of the editor, got editor #{editor} and preview #{preview}"
    end

    # O chromedriver manda Ctrl+\ com key vazia; pelo CDP a tecla chega como no navegador de verdade.
    def press_ctrl_backslash
      %w[keyDown keyUp].each do |type|
        page.driver.browser.execute_cdp("Input.dispatchKeyEvent", type:, modifiers: 2, key: "\\", code: "Backslash", windowsVirtualKeyCode: 220)
      end
    end

    # Rola o próprio scroller do CodeMirror até a linha ficar no topo, como a roda do mouse faria.
    def scroll_editor_to_line(text)
      line = find(".cm-line", text:, exact_text: true)
      execute_script(<<~JS, line)
        const scroller = arguments[0].closest(".cm-scroller")
        scroller.scrollTop += arguments[0].getBoundingClientRect().top - scroller.getBoundingClientRect().top
      JS
    end

    # O bloco tem de estar no topo da área visível do preview, não só em algum ponto da página.
    def near_preview_top?(element)
      evaluate_script(<<~JS, element)
        (() => {
          const preview = arguments[0].closest(".note-preview").getBoundingClientRect()
          const top = arguments[0].getBoundingClientRect().top - preview.top
          return top >= -1 && top < 32
        })()
      JS
    end
end
