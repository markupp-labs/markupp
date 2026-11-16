require "application_system_test_case"

# Diálogo de publicar (ADR-0048), como tornar público um repositório no GitHub: diz o que fica público e só libera o
# botão com o nome do alvo digitado exato. Alt+P abre o da nota aberta; Alt+Shift+P, o da linha da árvore com o foco.
class PublishDialogTest < ApplicationSystemTestCase
  NOTES = [ { id: "start", path: "start.md", content: "# Start\n" }, { id: "deploy", path: "docs/deploy.md", content: "# Deploy\n" } ].freeze

  setup do
    use_fake_api(notes: NOTES)
    sign_in_through_form
    visit note_path("start")
    assert_field "Path", with: "/start"
  end

  test "alt p opens the dialog of the note, and publish only frees up with the exact name" do
    find("body").send_keys([ :alt, "p" ])

    within("dialog#publish_dialog[open]") do
      assert_text "1 note goes public"
      assert_selector "input[name=confirm]:focus"
      fill_in "Type start to confirm", with: "star"
      assert_button "Publish", disabled: true
      fill_in "Type start to confirm", with: "start"
      click_on "Publish"
    end

    assert_text "Published. Anyone with the link reads it"
    assert_selector "a.public-mark", text: "Public"
  end

  test "esc closes the dialog without publishing" do
    click_on "Publish"
    assert_selector "dialog#publish_dialog[open] input[name=confirm]:focus"

    send_keys :escape

    assert_no_selector "dialog#publish_dialog[open]"
    assert_selector "a#publish_note:focus"
  end

  test "alt shift p on a folder of the tree opens the dialog of the folder" do
    find("[role=treeitem][data-path='/docs']").send_keys([ :alt, :shift, "p" ])

    within("dialog#publish_dialog[open]") do
      assert_text "Publish the folder docs"
      assert_text "notes added to docs later go public too"
    end
  end
end
