require "test_helper"

# Mover a nota para fora da pasta publicada a torna privada (ADR-0048): a nota que só a pasta publica leva ao form as
# pastas que a cobrem e o aviso, que o note_form_controller.js mostra antes de salvar o caminho novo.
class NoteMoveWarningTest < ActionDispatch::IntegrationTest
  NOTES = [ { id: "deploy", path: "docs/deploy.md", content: "# Deploy\n" }, { id: "start", path: "start.md", content: "# Start\n" } ].freeze

  test "a note public only through its folder brings the folder and the warning to the form" do
    use_fake_api(notes: NOTES, publications: [ { kind: "folder", vault_id: "vault-1", folder_path: "docs" } ])
    sign_in_as

    get note_path("deploy")

    form = css_select("form.note-form").first
    assert_equal [ "docs" ], JSON.parse(form["data-note-form-published-folders-value"])
    assert_equal "deploy is public through the published folder docs. Moving it out of docs makes 1 note private, and its public link stops working.",
                 form["data-note-form-move-warning-value"]
    assert_equal "Move", form["data-note-form-move-button-value"]
  end

  test "a note with a publication of its own stays public anywhere, so there is nothing to warn" do
    use_fake_api(notes: NOTES, publications: [ { kind: "folder", vault_id: "vault-1", folder_path: "docs" }, { kind: "note", vault_id: "vault-1", note_id: "deploy" } ])
    sign_in_as

    get note_path("deploy")

    assert_select "form.note-form[data-note-form-published-folders-value]", count: 0
  end

  test "a private note does not ask for the publications" do
    api = use_fake_api(notes: NOTES)
    sign_in_as

    get note_path("start")

    assert_select "form.note-form[data-note-form-published-folders-value]", count: 0
    assert_not api.received_credentials.any? { it[:call].start_with?("GET /v1/publications") }
  end
end
