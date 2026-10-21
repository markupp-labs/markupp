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

  test "mine keeps the note stats of each tenant when the API sends them" do
    email = FakeMarkuppAccounts::DEFAULT_EMAIL
    fake_api = FakeMarkuppApi.new(tenants: [
      { id: "tenant-1", name: "Acme research", members: { email => "owner" }, notes_count: 12, last_note_at: "2026-10-19T10:00:00Z" },
      { id: "tenant-2", name: "Home lab", members: { email => "viewer" }, notes_count: 0, last_note_at: nil }
    ])
    pair = fake_api.post("/auth/login", email:, password: FakeMarkuppAccounts::DEFAULT_PASSWORD)

    assert_equal [ { id: "tenant-1", name: "Acme research", role: "owner", notes_count: 12, last_note_at: Time.zone.parse("2026-10-19T10:00:00Z") },
                   { id: "tenant-2", name: "Home lab", role: "viewer", notes_count: 0, last_note_at: nil } ],
                 Tenant.mine(fake_api.as(access_token: pair.fetch("access_token"))).map(&:to_h)
  end

  test "a tenant without the stats from the API has neither a count nor a last edit" do
    assert_equal [ { id: "tenant-1", name: "Acme research", role: "owner", notes_count: nil, last_note_at: nil },
                   { id: "tenant-2", name: "Home lab", role: "viewer", notes_count: nil, last_note_at: nil } ],
                 Tenant.mine(@api).map(&:to_h)
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

  test "owner? is true only for the owner role" do
    owner, viewer = Tenant.mine(@api)

    assert_respond_to owner, :owner?
    assert_predicate owner, :owner?
    assert_not viewer.owner?
  end

  test "delete removes the tenant from the tenants of the account" do
    assert_respond_to Tenant, :delete
    Tenant.delete(@api, "tenant-1")

    assert_equal [ "tenant-2" ], Tenant.mine(@api).map(&:id)
  end

  test "delete by a role below owner is refused and keeps the tenant" do
    assert_respond_to Tenant, :delete
    assert_raises(MarkuppApi::Forbidden) { Tenant.delete(@api, "tenant-2") }

    assert_equal %w[tenant-1 tenant-2], Tenant.mine(@api).map(&:id)
  end

  test "delete of a tenant the account is not in raises a kind of NotFound" do
    assert_respond_to Tenant, :delete
    error = assert_raises(MarkuppApi::NotFound) { Tenant.delete(@api, "tenant-9") }

    assert_equal "MarkuppApi::TenantNotFound", error.class.name
  end
end
