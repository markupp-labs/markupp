require "test_helper"

class TenantTest < ActiveSupport::TestCase
  setup do
    @fake_api = FakeMarkuppApi.new(tenants: [
      { id: "tenant-1", name: "Acme research", members: { FakeMarkuppAccounts::DEFAULT_EMAIL => "owner" } },
      { id: "tenant-2", name: "Home lab", members: { FakeMarkuppAccounts::DEFAULT_EMAIL => "viewer" } }
    ])
    pair = @fake_api.post("/auth/login", email: FakeMarkuppAccounts::DEFAULT_EMAIL, password: FakeMarkuppAccounts::DEFAULT_PASSWORD)
    @api = @fake_api.as(access_token: pair.fetch("access_token"))
  end

  test "mine builds the tenants of the account with its role in each" do
    assert_equal [ Tenant.new(id: "tenant-1", name: "Acme research", role: "owner"), Tenant.new(id: "tenant-2", name: "Home lab", role: "viewer") ],
                 Tenant.mine(@api)
  end

  test "pick answers the tenant of the session while it is still in the list" do
    assert_equal "tenant-2", Tenant.pick(Tenant.mine(@api), "tenant-2").id
  end

  test "pick answers the only tenant when the session has none or a stale one" do
    only = [ Tenant.new(id: "tenant-1", name: "Acme research", role: "owner") ]

    assert_equal only.sole, Tenant.pick(only, nil)
    assert_equal only.sole, Tenant.pick(only, "tenant-9")
  end

  test "pick answers nil among several tenants without a valid choice" do
    assert_nil Tenant.pick(Tenant.mine(@api), nil)
    assert_nil Tenant.pick(Tenant.mine(@api), "tenant-9")
  end

  test "pick answers nil without tenants" do
    assert_nil Tenant.pick([], nil)
  end
end
