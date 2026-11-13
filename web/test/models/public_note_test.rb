require "test_helper"

# Nota pública lida sem conta pelo id público (ADR-0048): o id sai do fim do endereço, e a árvore, a busca e as
# ligações vêm recortadas pela publicação.
class PublicNoteTest < ActiveSupport::TestCase
  NOTES = [
    { id: "start", path: "docs/start.md", content: "# Start\n\nSee [[setup]] and [[secret]].\n" },
    { id: "setup", path: "docs/guides/setup.md", content: "# Setup\n\nRun the installer.\n" },
    { id: "secret", path: "secret.md", content: "# Secret\n" }
  ].freeze

  setup do
    @fake_api = FakeMarkuppApi.new(notes: NOTES, publications: [ { kind: "folder", vault_id: "vault-1", folder_path: "docs", indexable: true } ])
    @start_id = FakeMarkuppPublications.public_id_for("start")
  end

  test "id_in takes the 12 characters after the last hyphen, and the slug before them" do
    assert_equal [ "my-first-note", "AbCdEf123456" ], PublicNote.id_in("my-first-note-AbCdEf123456")
    assert_equal [ "", "AbCdEf123456" ], PublicNote.id_in("AbCdEf123456")
  end

  test "id_in refuses an address without a public id as NotFound, without calling the API" do
    [ "start", "start-abc", "start-AbCdEf12345!", "start-AbCdEf1234567" ].each do |address|
      error = assert_raises(MarkuppApi::NotFound) { PublicNote.id_in(address) }
      assert_match address, error.message
    end
  end

  test "find reads the note with its publication and links" do
    note = PublicNote.find(@fake_api, @start_id)

    assert_equal [ "start", "start", "start-#{@start_id}", true ], [ note.title, note.title_slug, note.to_param, note.indexable ]
    assert_equal [ "folder", "docs" ], [ note.publication.kind, note.publication.title ]
    assert_equal [ "setup" ], note.public_links.map(&:cited_title)
    assert_equal FakeMarkuppPublications.public_id_for("setup"), note.public_links.first.public_id
  end

  test "a note that is not public raises NotFound" do
    assert_raises(MarkuppApi::NotFound) { PublicNote.find(@fake_api, FakeMarkuppPublications.public_id_for("secret")) }
  end

  test "tree lists the notes of the publication, with the path inside the published folder" do
    notes = PublicNote.find(@fake_api, @start_id).tree(@fake_api, store: ActiveSupport::Cache::MemoryStore.new)

    assert_equal [ "guides/setup.md", "start.md" ], notes.map(&:path)
    assert_equal [ FakeMarkuppPublications.public_id_for("setup"), "setup" ], [ notes.first.public_id, notes.first.title_slug ]
  end

  test "tree stays in the store for a short while" do
    store = ActiveSupport::Cache::MemoryStore.new
    note = PublicNote.find(@fake_api, @start_id)
    note.tree(@fake_api, store:)
    @fake_api.limit_public_reads!

    assert_equal 2, note.tree(@fake_api, store:).size
  end

  test "search finds only inside the publication, with the marked snippet" do
    hits = PublicNote.find(@fake_api, @start_id).search(@fake_api, "installer")

    assert_equal [ "setup" ], hits.map(&:title)
    assert_equal "# Setup\n\nRun the \u0002installer\u0003.\n", hits.first.snippet
    assert_equal "setup-#{FakeMarkuppPublications.public_id_for("setup")}", hits.first.to_param
  end

  test "graph draws the publication with the notes by their public id" do
    graph = PublicNote.find(@fake_api, @start_id).graph(@fake_api, store: ActiveSupport::Cache::MemoryStore.new)

    assert_equal [ FakeMarkuppPublications.public_id_for("setup"), @start_id ], graph.nodes.map(&:id)
  end
end
