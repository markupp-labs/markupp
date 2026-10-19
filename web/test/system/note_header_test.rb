require "application_system_test_case"

# Cabeçalho da nota no celular: o caminho numa linha só dele; embaixo o seletor de modo, Save e Delete só com ícone e o
# "Unsaved changes" num ponto no Save.
class NoteHeaderTest < ApplicationSystemTestCase
  test "at phone width the path gets its own line above the mode switch and the icon buttons" do
    use_fake_api(notes: [ { id: "1", path: "work/research/plan.md", content: "# Plan\n" } ])
    sign_in_through_form

    visit note_path("1")
    assert_field "Path", with: "/work/research/plan"
    page.current_window.resize_to(390, 844)
    find(".cm-content").send_keys([ :control, :end ], "Ship it.")

    assert_selector "button[title='Save (Ctrl+S)']"
    assert_selector "button[title='Delete']"
    assert_selector "button[title='Save (Ctrl+S)'] [data-unsaved-dot]", visible: true
    path, *controls = header_row_tops
    assert_equal 1, controls.uniq.size, "expected the mode switch and the buttons in one line, got rows at #{controls.inspect}"
    assert_operator path, :<, controls.first, "expected the path above the controls, got #{path} and #{controls.first}"
    assert_path_uncut
  ensure
    page.current_window.resize_to(1400, 900)
  end

  test "on desktop the header keeps the labels and the unsaved text" do
    use_fake_api(notes: [ { id: "1", path: "plan.md", content: "# Plan\n" } ])
    sign_in_through_form

    visit note_path("1")
    find(".cm-content").send_keys([ :control, :end ], "Ship it.")

    assert_button "Save", text: "Save"
    assert_button "Delete", text: "Delete"
    assert_text "Unsaved changes"
    assert_no_selector "[data-unsaved-dot]"
  end

  private
    # O campo mostra o caminho inteiro, sem rolar o texto para dentro.
    def assert_path_uncut
      visible, full = evaluate_script("(({ clientWidth, scrollWidth }) => [clientWidth, scrollWidth])(document.querySelector('#note_path'))")
      assert_operator visible, :>=, full, "expected the whole path in its field, got #{visible}px of #{full}px"
    end

    # Topo de cada item visível do cabeçalho, arredondado: uma linha só dá um valor.
    def header_row_tops
      evaluate_script(<<~JS)
        [...document.querySelector(".note-form header").children]
          .filter((item) => item.offsetParent && !item.matches(".sr-only, [hidden]") && getComputedStyle(item).position != "absolute")
          .map((item) => Math.round(item.getBoundingClientRect().top))
      JS
    end
end
