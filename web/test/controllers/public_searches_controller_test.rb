require "test_helper"

# Busca da página pública (ADR-0048): só nas notas da publicação da nota lida, sem conta, com o trecho marcado.
class PublicSearchesControllerTest < ActionDispatch::IntegrationTest
  NOTES = [
    { id: "start", path: "docs/start.md", content: "# Start\n" },
    { id: "setup", path: "docs/setup.md", content: "Run the installer.\n" },
    { id: "secret", path: "secret.md", content: "The installer key.\n" }
  ].freeze
  START = FakeMarkuppPublications.public_id_for("start")

  setup do
    use_fake_api(notes: NOTES, publications: [ { kind: "folder", vault_id: "vault-1", folder_path: "docs" } ])
  end

  test "the search finds only inside the publication, with the term marked" do
    get public_note_search_path("start-#{START}", q: "installer")

    assert_response :success
    assert_select "header form[action=?] input[name=q][value=installer]", public_note_search_path("start-#{START}")
    assert_select "main ul[data-controller=context-menu] li a[href=?]", public_note_path("setup-#{FakeMarkuppPublications.public_id_for("setup")}") do
      assert_select "mark", "installer"
    end
    assert_select "main a", text: "secret", count: 0
    assert_select "nav a[href=?]", public_note_path("start-#{START}")
  end

  test "a search without hits says so" do
    get public_note_search_path("start-#{START}", q: "nothing")

    assert_response :success
    assert_select "main", text: /No public note mentions nothing/
  end

  test "the search of a private note answers 404" do
    get public_note_search_path("secret-#{FakeMarkuppPublications.public_id_for("secret")}", q: "installer")

    assert_response :not_found
  end
end
