require "test_helper"

class CommunityTest < ActiveSupport::TestCase
  setup do
    @api = FakeMarkuppApi.new(graph: { communities: { "vault-1" => [
      { id: 7, size: 42, hub: { id: "1", title: "hub", path: "hub.md" } },
      { id: 3, size: 5, hub: { id: "9", title: "side", path: "side.md" } }
    ] } })
  end

  test "in_vault builds the communities of the vault with the size and the hub note" do
    communities = Community.in_vault(@api, "vault-1")

    assert_equal [ [ 7, 42, "hub.md" ], [ 3, 5, "side.md" ] ], communities.map { [ it.id, it.size, it.hub.path ] }
  end

  test "in_vault pages with offset and limit" do
    assert_equal [ 3 ], Community.in_vault(@api, "vault-1", offset: 1, limit: 1).map(&:id)
  end
end
