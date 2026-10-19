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

  private
    # O chromedriver manda Ctrl+\ com key vazia; pelo CDP a tecla chega como no navegador de verdade.
    def press_ctrl_backslash
      %w[keyDown keyUp].each do |type|
        page.driver.browser.execute_cdp("Input.dispatchKeyEvent", type:, modifiers: 2, key: "\\", code: "Backslash", windowsVirtualKeyCode: 220)
      end
    end
end
