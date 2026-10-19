require "application_system_test_case"

# Cabeçalho da nota no celular: Save e Delete só com ícone e o "Unsaved changes" num ponto no Save, tudo numa linha.
class NoteHeaderTest < ApplicationSystemTestCase
  test "at phone width the edited note header fits one line with icon buttons" do
    use_fake_api(notes: [ { id: "1", path: "work/plan.md", content: "# Plan\n" } ])
    sign_in_through_form

    visit note_path("1")
    page.current_window.resize_to(390, 844)
    find(".cm-content").send_keys([ :control, :end ], "Ship it.")

    assert_selector "button[title='Save (Ctrl+S)']"
    assert_selector "button[title='Delete']"
    assert_selector "button[title='Save (Ctrl+S)'] [data-unsaved-dot]", visible: true
    assert_equal 1, header_row_tops.uniq.size, "expected the header in one line, got rows at #{header_row_tops.inspect}"
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
    # Topo de cada item visível do cabeçalho, arredondado: uma linha só dá um valor.
    def header_row_tops
      evaluate_script(<<~JS)
        [...document.querySelector(".note-form header").children]
          .filter((item) => item.offsetParent && !item.matches(".sr-only, [hidden]") && getComputedStyle(item).position != "absolute")
          .map((item) => Math.round(item.getBoundingClientRect().top))
      JS
    end
end
