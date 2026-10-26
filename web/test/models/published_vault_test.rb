require "test_helper"

# Leitura sem conta do cofre publicado (ADR-0048): o cofre pelo slug, as notas dele e o grafo, pelas rotas /v1/public.
class PublishedVaultTest < ActiveSupport::TestCase
  NOTES = [
    { id: "start", path: "start.md", content: "# Start\n", updated_at: "2026-10-01T12:00:00Z" },
    { id: "guide", path: "guides/setup.md", content: "# Setup\n" },
    { id: "secret", path: "secret.md", content: "# Secret\n", vault_id: "vault-2" }
  ].freeze
  PUBLICATIONS = [ { vault_id: "vault-1", slug: "docs", title: "Markupp docs" } ].freeze
  GRAPH = { graph: { "vault-1" => { nodes: [ { id: "start", title: "start", path: "start.md", unresolved: false } ], links: [] } },
            graph_etag: 'W/"3"' }.freeze

  setup do
    @api = FakeMarkuppApi.new(notes: NOTES, publications: PUBLICATIONS, graph: GRAPH)
  end

  test "find builds the published vault from its slug" do
    vault = PublishedVault.find(@api, "docs")

    assert_equal [ "docs", "Markupp docs", 1 ], [ vault.slug, vault.title, vault.graph_version ]
    assert_equal Time.utc(2026, 10, 20, 12), vault.published_at
  end

  test "find raises NotFound for a slug that is not published" do
    assert_raises(MarkuppApi::NotFound) { PublishedVault.find(@api, "private") }
  end

  test "notes lists every note of the vault across pages, without the other vaults" do
    notes = PublishedVault.find(@api, "docs").notes(@api, page_limit: 1)

    assert_equal [ [ "guide", "guides/setup.md" ], [ "start", "start.md" ] ], notes.map { [ it.id, it.path ] }
  end

  test "find keeps the vault in the store for a short while, without asking the API again" do
    reader = @api.as(forwarded_for: "203.0.113.9")
    store = CacheWithoutExpiry.new
    PublishedVault.find(reader, "docs", store:)

    vault = PublishedVault.find(reader, "docs", store:)

    assert_equal "Markupp docs", vault.title
    assert_equal [ "GET /v1/public/vaults/docs" ], reader.received_credentials.pluck(:call)
    assert_equal [ PublishedVault::STORED_FOR ], store.expiries.values
    assert_operator PublishedVault::STORED_FOR, :<=, 1.minute
  end

  test "notes keeps the listing in the store by the graph version, and lists again when the version changes" do
    reader = @api.as(forwarded_for: "203.0.113.9")
    store = CacheWithoutExpiry.new
    vault = PublishedVault.find(@api, "docs")
    vault.notes(reader, store:)

    assert_equal %w[guide start], vault.notes(reader, store:).map(&:id)
    assert_equal 1, reader.received_credentials.size
    vault.with(graph_version: 2).notes(reader, store:)
    assert_equal 2, reader.received_credentials.size
    assert_equal [ PublishedVault::STORED_FOR ], store.expiries.values.uniq
  end

  test "note reads one note of the vault with its content" do
    note = PublishedVault.find(@api, "docs").note(@api, "start")

    assert_equal [ "start.md", "# Start\n" ], [ note.path, note.content ]
  end

  test "note raises NotFound for a note of another vault" do
    vault = PublishedVault.find(@api, "docs")

    assert_raises(MarkuppApi::NotFound) { vault.note(@api, "secret") }
  end

  test "Graph.of_published_vault reads the graph of the vault and keeps it by its ETag" do
    store = CacheWithoutExpiry.new
    Graph.of_published_vault(@api, "docs", store:)

    graph = Graph.of_published_vault(@api, "docs", store:)

    assert_equal [ nil, 'W/"3"' ], @api.received_etags
    assert_equal [ "start" ], graph.nodes.map(&:id)
  end
end
