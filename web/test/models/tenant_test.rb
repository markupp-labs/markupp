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

  test "create answers the new tenant with the owner role" do
    assert_equal Tenant.new(id: "tenant-3", name: "Field notes", role: "owner"), Tenant.create(@api, "Field notes")
  end

  test "create raises InvalidRequest for a name the API refuses" do
    assert_raises(MarkuppApi::InvalidRequest) { Tenant.create(@api, " ") }
  end

  test "writer? is true for every role but viewer" do
    assert_equal({ "owner" => true, "admin" => true, "member" => true, "viewer" => false },
                 %w[owner admin member viewer].index_with { Tenant.new(id: "tenant-1", name: "Acme research", role: it).writer? })
  end

  test "admin? is true for owner and admin only" do
    assert_equal({ "owner" => true, "admin" => true, "member" => false, "viewer" => false },
                 Tenant::ROLES.index_with { Tenant.new(id: "tenant-1", name: "Acme research", role: it).admin? })
  end

  test "assignable_roles lists the roles up to its own, highest first" do
    assert_equal({ "owner" => %w[owner admin member viewer], "admin" => %w[admin member viewer] },
                 %w[owner admin].index_with { Tenant.new(id: "tenant-1", name: "Acme research", role: it).assignable_roles })
  end

  test "assignable_roles names an unknown role and the known ones" do
    error = assert_raises(ArgumentError) { Tenant.new(id: "tenant-1", name: "Acme research", role: "auditor").assignable_roles }

    assert_equal 'unknown role "auditor", expected one of owner, admin, member, viewer', error.message
  end
end
