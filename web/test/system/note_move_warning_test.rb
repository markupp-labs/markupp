require "application_system_test_case"

# Antes de salvar o caminho que tira a nota da pasta publicada, o diálogo do painel diz que ela fica privada; mover
# dentro da pasta salva direto.
class NoteMoveWarningTest < ApplicationSystemTestCase
  NOTES = [ { id: "deploy", path: "docs/deploy.md", content: "# Deploy\n" } ].freeze
  WARNING = "Moving it out of docs makes 1 note private"

  setup do
    use_fake_api(notes: NOTES, publications: [ { kind: "folder", vault_id: "vault-1", folder_path: "docs" } ])
    sign_in_through_form
    visit note_path("deploy")
    assert_field "Path", with: "/docs/deploy"
  end

  test "moving out of the published folder asks first, and stay keeps the note where it was" do
    fill_in "Path", with: "/archive/deploy"
    dismiss_panel_confirm(WARNING) { find_field("Path").send_keys([ :control, "s" ]) }

    assert_field "Path", with: "/archive/deploy"
    assert_no_text "Saved"

    accept_panel_confirm(WARNING, button: "Move") { find_field("Path").send_keys([ :control, "s" ]) }
    assert_text "Saved"
    assert_field "Path", with: "/archive/deploy"
  end

  test "moving inside the published folder saves without asking" do
    fill_in "Path", with: "guides/deploy"
    find_field("Path").send_keys([ :control, "s" ])

    assert_text "Saved"
    assert_field "Path", with: "/docs/guides/deploy"
  end
end
