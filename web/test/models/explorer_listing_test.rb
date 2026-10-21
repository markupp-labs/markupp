require "test_helper"

class ExplorerListingTest < ActiveSupport::TestCase
  setup do
    @api = FakeMarkuppApi.new(notes: [
      { id: "hub", path: "hub.md", content: "", degree: 4, pagerank: 0.5, community: 2, updated_at: "2026-10-01T12:00:00Z" },
      { id: "leaf", path: "leaf.md", content: "", degree: 1, pagerank: 0.1, community: 2, updated_at: "2026-10-04T12:00:00Z" },
      { id: "pair", path: "pair.md", content: "", degree: 2, pagerank: 0.3, community: 9, updated_at: "2026-10-02T12:00:00Z" },
      { id: "lone", path: "lone.md", content: "", degree: 0, pagerank: 0.05, updated_at: "2026-10-03T12:00:00Z" },
      { id: "solo", path: "solo.md", content: "", degree: 0, pagerank: 0.05, updated_at: "2026-09-30T12:00:00Z" },
      { id: "far", path: "far.md", content: "", degree: 7, pagerank: 0.9, community: 2, vault_id: "vault-2" }
    ])
  end

  test "mode keeps a known mode and falls back to the folders" do
    assert_equal "orphans", ExplorerListing.mode("orphans")
    assert_equal "folders", ExplorerListing.mode("search")
    assert_equal "folders", ExplorerListing.mode(nil)
  end

  test "central, linked and recent list the notes of the vault in one group, by pagerank, degree and update" do
    assert_equal [ %w[hub pair leaf lone solo] ], ids(ExplorerListing.load(@api, "vault-1", "central"))
    assert_equal [ %w[hub pair leaf lone solo] ], ids(ExplorerListing.load(@api, "vault-1", "linked"))
    assert_equal [ %w[leaf lone pair hub solo] ], ids(ExplorerListing.load(@api, "vault-1", "recent"))
    assert_nil ExplorerListing.load(@api, "vault-1", "central").first.community
  end

  test "orphans lists every orphan of the vault, across pages" do
    assert_equal [ %w[lone solo] ], ids(ExplorerListing.load(@api, "vault-1", "orphans", page_limit: 1))
  end

  test "communities groups the notes by community, biggest first, central first, and the notes without one last" do
    groups = ExplorerListing.load(@api, "vault-1", "communities", page_limit: 2)

    assert_equal [ 2, 9, nil ], groups.map(&:community)
    assert_equal [ %w[hub leaf], %w[pair], %w[lone solo] ], ids(groups)
  end

  private
    def ids(groups) = groups.map { it.notes.map(&:id) }
end
