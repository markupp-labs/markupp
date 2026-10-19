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

    find("body").send_keys([ :alt, "n" ])
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

  test "tab in the path jumps to the content and shift+tab comes back, in edit and side by side" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])
    sign_in_through_form

    visit note_path("1")
    [ "Edit", "Side by side" ].each do |mode|
      click_on mode
      find_field("Path").send_keys(:tab)
      assert_selector ".cm-content:focus"
      send_keys [ :shift, :tab ]
      assert_field "Path", focused: true
    end
  end

  test "tab in the path moves on when the editor is hidden in read mode" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])
    sign_in_through_form

    visit note_path("1")
    click_on "Read"
    assert_field "Path", with: "/plan"
    find_field("Path").send_keys(:tab)

    assert_not_equal "note_path", evaluate_script("document.activeElement.id")
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

    click_on "Hide sidebar"
    assert_preview_beside_editor
  ensure
    page.current_window.resize_to(1400, 900)
  end

  test "the title lines up with the rendered note in read mode and in the stacked side by side" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])
    sign_in_through_form

    visit note_path("1")
    click_on "Read"
    assert_title_aligned_with_preview

    click_on "Side by side"
    page.current_window.resize_to(1024, 768)
    assert_preview_below_editor
    assert_title_aligned_with_preview
  ensure
    page.current_window.resize_to(1400, 900)
  end

  test "side by side scrolls the preview along with the editor" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: long_note } ])
    sign_in_through_form

    visit note_path("1")
    click_on "Side by side"
    scroll_editor_to_line "## Middle"

    assert_selector(".note-preview h2", text: "Middle") { |heading| near_preview_top?(heading) }
  end

  # Mão dupla, como no VS Code: rolar o preview leva o editor, e o painel alinhado não devolve a rolagem a quem rolou.
  test "side by side scrolls the editor along with the preview, without bouncing back" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: long_note } ])
    sign_in_through_form

    visit note_path("1")
    click_on "Side by side"
    scroll_preview_to(find(".note-preview h2", text: "Middle"))

    assert_selector(".cm-line", text: "## Middle", exact_text: true) { |line| near_pane_top?(line, ".cm-scroller") }

    # Perto do fim o preview bate no fundo antes do editor; sem a trava, o preview no fundo mandaria o editor ao fundo.
    scrolled_to = scroll_editor_near_end
    wait_for_panes_to_settle
    assert_equal scrolled_to, evaluate_script("document.querySelector('.cm-scroller').scrollTop"), "expected the editor to stay where it was scrolled"
  end

  private
    # A primeira ida ao fim faz o CodeMirror medir as linhas que só tinha estimado (e corrigir a própria rolagem);
    # a volta ao topo tira o preview do fundo, e a segunda ida é a que vale.
    def scroll_editor_near_end
      [ true, false, true ].map do |near_end|
        wait_for_panes_to_settle
        evaluate_script(<<~JS, near_end)
          (() => {
            const scroller = document.querySelector(".cm-scroller")
            scroller.scrollTop = arguments[0] ? scroller.scrollHeight - scroller.clientHeight - 40 : 0
            return scroller.scrollTop
          })()
        JS
      end.last
    end

    # Espera os dois painéis ficarem parados por alguns quadros: o alinhamento roda num quadro e a medição do
    # CodeMirror no seguinte, então quadros quietos querem dizer que nada mais vai rolar.
    def wait_for_panes_to_settle(quiet_frames: 5)
      evaluate_async_script(<<~JS, quiet_frames)
        const [quietFrames, done] = arguments
        const positions = () => [".cm-scroller", ".note-preview"].map((selector) => document.querySelector(selector).scrollTop).join()
        let last = positions(), quiet = 0
        const check = () => {
          const now = positions()
          quiet = now == last ? quiet + 1 : 0
          last = now
          quiet >= quietFrames ? done() : requestAnimationFrame(check)
        }
        requestAnimationFrame(check)
      JS
    end

    def pane_boxes
      evaluate_script(<<~JS)
        [".note-editor-pane", ".note-preview"].map((selector) => document.querySelector(selector).getBoundingClientRect().toJSON())
      JS
    end

    # Onde começa o texto do caminho (dentro da borda e do padding do campo) e onde começa o markdown renderizado.
    def assert_title_aligned_with_preview
      assert_selector ".note-preview h1", text: "Plan"
      assert_selector "#note_path"
      title, preview = evaluate_script(<<~JS)
        (() => {
          const title = document.querySelector("#note_path")
          const style = getComputedStyle(title)
          const text = title.getBoundingClientRect().left + parseFloat(style.borderLeftWidth) + parseFloat(style.paddingLeft)
          return [text, document.querySelector(".note-preview h1").getBoundingClientRect().left]
        })()
      JS
      assert_in_delta title, preview, 1, "expected the title text at the left of the preview, got title #{title} and preview #{preview}"
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

    # Rola o próprio scroller do CodeMirror até a linha ficar no topo, como a roda do mouse faria, e devolve o scrollTop.
    def scroll_editor_to_line(text)
      scroll_to_top_of(find(".cm-line", text:, exact_text: true), ".cm-scroller")
    end

    def long_note
      filler = ->(part, count) { (1..count).map { |n| "#{part} paragraph #{n}." }.join("\n\n") }
      "# Plan\n\n#{filler.('Opening', 12)}\n\n## Middle\n\n#{filler.('Closing', 40)}\n"
    end

    def scroll_preview_to(element) = scroll_to_top_of(element, ".note-preview")

    def scroll_to_top_of(element, pane)
      evaluate_script(<<~JS, element, pane)
        (() => {
          const scroller = arguments[0].closest(arguments[1])
          scroller.scrollTop += arguments[0].getBoundingClientRect().top - scroller.getBoundingClientRect().top
          return scroller.scrollTop
        })()
      JS
    end

    # O bloco tem de estar no topo da área visível do preview, não só em algum ponto da página.
    def near_preview_top?(element) = near_pane_top?(element, ".note-preview")

    def near_pane_top?(element, pane)
      evaluate_script(<<~JS, element, pane)
        (() => {
          const box = arguments[0].closest(arguments[1]).getBoundingClientRect()
          const top = arguments[0].getBoundingClientRect().top - box.top
          return top >= -1 && top < 32
        })()
      JS
    end
end
