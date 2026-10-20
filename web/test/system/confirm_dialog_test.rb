require "application_system_test_case"

# Confirmação de ação destrutiva num diálogo do próprio painel, no lugar do confirm() do navegador: uma camada com a
# pergunta, Cancel e o botão da ação no danger. O foco começa no Cancel, Enter responde com o botão focado, Esc cancela
# e o foco volta a quem pediu. O confirm() nativo fica trocado por um que só anota a chamada e diz não: ninguém o chama.
class ConfirmDialogTest < ApplicationSystemTestCase
  NOTES = [ { id: "1", path: "work/ideas.md", content: "# Ideas\n" }, { id: "2", path: "plan.md", content: "# Plan\n" } ].freeze

  setup do
    use_fake_api(notes: NOTES)
    sign_in_through_form
    visit note_path("1")
    assert_field "Path", with: "/work/ideas"
    execute_script("window.confirm = () => { window.nativeConfirmCalls = (window.nativeConfirmCalls ?? 0) + 1; return false }")
  end

  teardown { assert_nil evaluate_script("window.nativeConfirmCalls"), "the native confirm() was called" }

  test "delete on the open note asks in a panel layer with cancel focused, esc keeps the note and the focus goes back" do
    click_on "Delete"

    assert_selector "dialog#confirm_dialog[open].layer.layer-panel"
    within("dialog#confirm_dialog[open]") do
      assert_text "Delete /work/ideas? This cannot be undone."
      assert_selector "button.btn-danger", text: "Delete"
      assert_selector "button:focus", text: "Cancel"
    end
    send_keys :escape

    assert_no_selector "dialog#confirm_dialog[open]"
    assert_selector ".note-form header button[title=Delete]:focus"
    assert_field "Path", with: "/work/ideas"
  end

  test "closed, the dialog has no action label, so the page has no stray delete" do
    assert_equal "", find("dialog#confirm_dialog button[value=confirm]", visible: :all).text(:all)
    assert_nil find("dialog#confirm_dialog", visible: :all)["aria-label"]
  end

  test "tab to delete and enter removes the note" do
    click_on "Delete"
    assert_selector "dialog#confirm_dialog[open] button:focus", text: "Cancel"
    send_keys :tab
    assert_selector "dialog#confirm_dialog[open] button.btn-danger:focus"
    send_keys :enter

    assert_text "Deleted"
    assert_no_selector "[role=treeitem]", text: "ideas"
  end

  test "delete from the tree menu asks about the row, enter on cancel keeps it and the focus goes back to the row" do
    find("nav [role=treeitem]", text: "plan").right_click
    find("[role=menuitem]", text: "Delete").click

    assert_selector "dialog#confirm_dialog[open]", text: "Delete /plan? This cannot be undone."
    send_keys :enter

    assert_no_selector "dialog#confirm_dialog[open]"
    assert_selector "nav [role=treeitem][data-path='/plan']:focus"
    assert_selector "nav [role=treeitem]", text: "plan"
  end

  test "the backdrop fades and the dialog pops in as a layer" do
    emulate_motion_preference("no-preference")
    click_on "Delete"

    assert_selector "dialog#confirm_dialog[open]"
    assert_equal "0.38s", find("dialog#confirm_dialog[open]").evaluate_script("getComputedStyle(this).transitionDuration.split(', ')[0]")
  end
end
