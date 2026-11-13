require "test_helper"

# Publicações do tenant pela API autenticada (ADR-0048): a prévia do que fica público, publicar com o nome do alvo,
# a lista, a indexação e despublicar.
class PublicationTest < ActiveSupport::TestCase
  NOTES = [
    { id: "start", path: "start.md", content: "# Start\n" },
    { id: "deploy", path: "docs/deploy.md", content: "# Deploy\n" },
    { id: "setup", path: "docs/guides/setup.md", content: "# Setup\n" }
  ].freeze

  setup do
    @fake_api = FakeMarkuppApi.new(notes: NOTES, publications: [ { kind: "folder", vault_id: "vault-1", folder_path: "docs" } ])
  end

  test "preview says what a folder makes public, with the notes that come later" do
    preview = Publication.preview(@fake_api, "vault-1", Publication::Target.new(kind: "folder", path: "docs"))

    assert_equal Publication::Preview.new(kind: "folder", target_name: "docs", note_count: 2, includes_future: true), preview
  end

  test "preview of a note makes only the note public" do
    preview = Publication.preview(@fake_api, "vault-1", Publication::Target.new(kind: "note", note_id: "start"))

    assert_equal [ "start", 1, false ], [ preview.target_name, preview.note_count, preview.includes_future ]
  end

  test "publish sends the target, the typed name and the indexing, and answers the publication of the note" do
    publication = Publication.publish(@fake_api, "vault-1", Publication::Target.new(kind: "note", note_id: "start"), confirm: "start", indexable: true)

    assert_equal [ "note", "start", true ], [ publication.kind, publication.target_name, publication.indexable ]
    assert_equal "start-#{FakeMarkuppPublications.public_id_for("start")}", publication.public_param
    assert_equal Time.utc(2026, 10, 20, 12), publication.published_at
  end

  test "publish with another name raises ConfirmationMismatch" do
    assert_raises(MarkuppApi::ConfirmationMismatch) do
      Publication.publish(@fake_api, "vault-1", Publication::Target.new(kind: "vault"), confirm: "notes", indexable: false)
    end
  end

  test "all lists the publications of a vault, or of every vault without one" do
    Publication.publish(@fake_api, "vault-1", Publication::Target.new(kind: "note", note_id: "start"), confirm: "start", indexable: false)

    assert_equal %w[folder note], Publication.all(@fake_api).map(&:kind)
    assert_equal [], Publication.all(@fake_api, vault_id: "vault-2")
    folder = Publication.all(@fake_api, vault_id: "vault-1").first
    assert_equal [ "docs", 2, "Notes", FakeMarkuppAccounts::DEFAULT_EMAIL ], [ folder.target_name, folder.note_count, folder.vault_name, folder.published_by_name ]
    assert_nil folder.public_param
  end

  test "change_indexing turns the indexing on and off" do
    id = Publication.all(@fake_api).first.id

    assert Publication.change_indexing(@fake_api, id, true).indexable
    assert Publication.all(@fake_api).first.indexable
    assert_not Publication.change_indexing(@fake_api, id, false).indexable
  end

  test "unpublish removes the publication, and its notes stop being public" do
    Publication.unpublish(@fake_api, Publication.all(@fake_api).first.id)

    assert_empty Publication.all(@fake_api)
    assert_nil Note.find(@fake_api, "deploy").public_id
  end

  test "a target with an unknown kind is refused before calling the API" do
    error = assert_raises(MarkuppApi::InvalidRequest) { Publication::Target.from(kind: "tenant") }

    assert_match "tenant", error.message
    assert_match "note, folder or vault", error.message
  end
end
