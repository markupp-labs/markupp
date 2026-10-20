require "test_helper"

class SearchHitTest < ActiveSupport::TestCase
  setup do
    @api = FakeMarkuppApi.new(notes: [
      { id: "1", path: "infra/cluster.md", content: "the kubernetes cluster" },
      { id: "2", path: "other.md", content: "kubernetes elsewhere", vault_id: "vault-2" },
      { id: "3", path: "inbox.md", content: "nothing here" }
    ])
  end

  test "search builds the hits with the note, the vault, the rank and the snippet with the markers" do
    hit = SearchHit.search(@api, "Kubernetes").find { it.note.id == "1" }

    assert_equal [ "cluster", "infra/cluster.md" ], [ hit&.note&.title, hit&.note&.path ]
    assert_equal [ "vault-1", 1.0 ], [ hit&.vault_id, hit&.rank ]
    assert_equal "the \u0002kubernetes\u0003 cluster", hit&.snippet
  end

  test "search with vault looks only in that vault" do
    assert_equal [ "2" ], SearchHit.search(@api, "kubernetes", vault: "vault-2").map { it.note.id }
  end

  test "search pages with offset and limit" do
    assert_equal 1, SearchHit.search(@api, "kubernetes", limit: 1).size
  end

  test "search with a blank query is empty without calling the API" do
    api = FakeMarkuppApi.new(unreachable: true)

    assert_empty SearchHit.search(api, " ")
    assert_empty SearchHit.search(api, nil)
  end
end
