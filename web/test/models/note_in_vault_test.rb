require "test_helper"

class NoteInVaultTest < ActiveSupport::TestCase
  setup do
    @api = FakeMarkuppApi.new(notes: [
      { id: "1", path: "hub.md", content: "# Hub", degree: 3, pagerank: 0.5, community: 7, updated_at: "2026-10-01T12:00:00Z" },
      { id: "2", path: "lone.md", content: "alone", degree: 0, pagerank: 0.1, updated_at: "2026-10-03T12:00:00Z" },
      { id: "3", path: "fresh.md", content: "new", updated_at: "2026-10-02T12:00:00Z" },
      { id: "4", path: "elsewhere.md", content: "x", vault_id: "vault-2", degree: 1 }
    ])
  end

  test "all keeps the vault of each note, the way the panel learns the vault" do
    assert_equal %w[vault-1 vault-1 vault-1 vault-2], Note.all(@api).map(&:vault_id)
  end

  test "in_vault lists the notes of the vault without content, by path, with the derived organization" do
    notes = Note.in_vault(@api, "vault-1")

    assert_equal %w[fresh.md hub.md lone.md], notes.map(&:path)
    hub = notes.second
    assert_equal [ "1", 3, 0.5, 7, Time.utc(2026, 10, 1, 12) ], [ hub.id, hub.degree, hub.pagerank, hub.community, hub.updated_at ]
    assert_nil hub.content
    assert_equal [ nil, nil, nil ], [ notes.first.degree, notes.first.pagerank, notes.first.community ]
  end

  test "list_all_from reads every page of the list until a short one, with the query" do
    notes = Note.list_all_from(@api, "vaults", "vault-1", "notes", page_limit: 2, orphan: false)

    assert_equal %w[3 1], notes.map(&:id)
    assert_equal %w[fresh.md hub.md lone.md], Note.list_all_from(@api, "vaults", "vault-1", "notes", page_limit: 1).map(&:path)
  end

  test "list_all_from stops at the offset cap of the API, with the list truncated there" do
    api = FakeOffsetCappedListing.new(offset_cap: 10_000, max_calls: 50)

    notes = Note.list_all_from(api, "public", "vaults", "docs", "notes", page_limit: 1_000)

    assert_equal (0..10_000).step(1_000).to_a, api.requested_offsets
    assert_equal 11_000, notes.size
  end

  test "in_vault with orphan lists only the notes without any link" do
    assert_equal [ "2" ], Note.in_vault(@api, "vault-1", orphan: true).map(&:id)
    assert_equal %w[3 1], Note.in_vault(@api, "vault-1", orphan: false).map(&:id)
  end

  test "in_vault with community lists only the notes of that community" do
    assert_equal [ "1" ], Note.in_vault(@api, "vault-1", community: 7).map(&:id)
  end

  test "in_vault orders by pagerank, degree or update, with the notes the indexer has not seen last" do
    assert_equal %w[1 2 3], Note.in_vault(@api, "vault-1", sort: "pagerank").map(&:id)
    assert_equal %w[2 1 3], Note.in_vault(@api, "vault-1", sort: "degree", direction: "asc").map(&:id)
    assert_equal %w[2 3 1], Note.in_vault(@api, "vault-1", sort: "updated").map(&:id)
  end

  test "in_vault pages with offset and limit" do
    assert_equal [ "1" ], Note.in_vault(@api, "vault-1", offset: 1, limit: 1).map(&:id)
  end
end
