require "test_helper"

# Publicar uma nota, uma pasta ou o cofre ativo (ADR-0048): o diálogo diz o que fica público pela prévia da API e
# pede o nome do alvo; a API confere o nome de novo. Nota por quem escreve, pasta e cofre por owner e admin.
class PublicationsControllerTest < ActionDispatch::IntegrationTest
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL
  NOTES = [
    { id: "start", path: "start.md", content: "# Start\n" },
    { id: "deploy", path: "docs/deploy.md", content: "# Deploy\n" },
    { id: "setup", path: "docs/guides/setup.md", content: "# Setup\n" }
  ].freeze

  setup do
    @api = use_fake_api(notes: NOTES)
    sign_in_as
  end

  test "the dialog of a folder says how many notes go public, that later ones follow, and who reads them" do
    get new_publication_path(kind: "folder", path: "docs"), headers: { "Turbo-Frame" => "publish_preview" }

    assert_response :success
    assert_select "turbo-frame#publish_preview" do
      assert_select "h2", text: "Publish the folder docs"
      assert_select "p", text: /2 notes go public/
      assert_select "p", text: /notes added to docs later go public too/
      assert_select "p", text: /people and AI agents without an account/
      assert_select "form[action=?][data-turbo-frame=_top]", publications_path do
        assert_select "input[type=hidden][name=kind][value=folder]"
        assert_select "input[type=hidden][name=path][value=docs]"
        assert_select "label", text: "Type docs to confirm"
        assert_select "input[name=confirm][autocomplete=off][data-publish-dialog-target=name]"
        assert_select "input[type=checkbox][name=indexable]"
        assert_select "button[disabled][data-publish-dialog-target=submit]", text: "Publish"
      end
    end
    assert_select "[data-publish-dialog-expected-value=docs]"
  end

  test "the dialog of a note makes only that note public" do
    get new_publication_path(kind: "note", note_id: "start"), headers: { "Turbo-Frame" => "publish_preview" }

    assert_select "h2", text: "Publish the note start"
    assert_select "p", text: /1 note goes public/
    assert_select "p", text: /later/, count: 0
  end

  test "publishing a note with its name sends the target and comes back with the public link" do
    post publications_path, params: { kind: "note", note_id: "start", confirm: "start", indexable: "1" }, headers: { "Referer" => note_url("start") }

    assert_redirected_to note_url("start")
    public_id = FakeMarkuppPublications.public_id_for("start")
    assert_equal "Published. Anyone with the link reads it: http://www.example.com/p/start-#{public_id}", flash[:notice]
    assert @api.publications.last["indexable"]
  end

  test "publishing the vault says how many notes went public" do
    post publications_path, params: { kind: "vault", confirm: "Notes" }

    assert_redirected_to root_path
    assert_equal "Published 3 notes. Settings > Publications lists everything that is public.", flash[:notice]
    assert_not @api.publications.last["indexable"]
  end

  test "another name comes back with the name to type, and nothing is published" do
    post publications_path, params: { kind: "folder", path: "docs", confirm: "doc" }, headers: { "Referer" => root_url }

    assert_redirected_to root_url
    assert_equal "Nothing was published: type docs exactly as shown.", flash[:alert]
    assert_empty @api.publications
  end

  test "publishing what is already public says so" do
    2.times { post publications_path, params: { kind: "note", note_id: "start", confirm: "start" } }

    assert_equal "This is already public. Settings > Publications lists it.", flash[:alert]
  end

  test "a member who is not owner nor admin cannot publish a folder" do
    tenants = [ { id: "tenant-1", name: "Acme research", members: { EMAIL => "member" } } ]
    @api = use_fake_api(notes: NOTES, tenants:)
    sign_in_as

    post publications_path, params: { kind: "folder", path: "docs", confirm: "docs" }, headers: { "Referer" => root_url }

    assert_redirected_to root_url
    assert_equal "Only the owner and the admins publish folders and vaults; a note, whoever edits it.", flash[:alert]
  end

  test "an unknown kind answers 400 without calling the API" do
    get new_publication_path(kind: "tenant"), headers: { "Turbo-Frame" => "publish_preview" }

    assert_response :bad_request
  end
end
