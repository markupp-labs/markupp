require "application_system_test_case"

# Leitura sem conta da nota pública (ADR-0048): o [[link]] para nota que não é pública vira texto puro e o ![[embed]]
# dela não aparece, sem nada que diga que ela existe; o da nota pública abre e embute como no painel.
class PublicNotesTest < ApplicationSystemTestCase
  NOTES = [
    { id: "start", path: "docs/start.md", content: "See [[setup]] and [[secret]].\n\n![[setup]]\n\n![[secret]]\n" },
    { id: "setup", path: "docs/setup.md", content: "# Setup body\n" },
    { id: "secret", path: "secret.md", content: "# Secret body\n" }
  ].freeze

  test "links to public notes open them, and private ones read as plain text with no embed" do
    use_fake_api(notes: NOTES, publications: [ { kind: "folder", vault_id: "vault-1", folder_path: "docs" } ])

    visit public_note_path("start-#{FakeMarkuppPublications.public_id_for("start")}")

    within(".note-preview") do
      assert_link "setup", href: public_note_path("setup-#{FakeMarkuppPublications.public_id_for("setup")}")
      assert_selector ".note-embed h1", text: "Setup body"
      assert_text "and secret."
      assert_no_link "secret"
      assert_no_selector ".wikilink-missing"
      assert_no_text "Secret body"
      assert_selector ".note-embed", count: 1
    end
  end
end
