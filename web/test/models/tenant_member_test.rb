require "test_helper"

class TenantMemberTest < ActiveSupport::TestCase
  ANA = FakeMarkuppAccounts::DEFAULT_EMAIL
  BIA = "bia@markupp.dev"
  PASSWORD = FakeMarkuppAccounts::DEFAULT_PASSWORD
  ACCOUNTS = [ { email: ANA, password: PASSWORD }, { email: BIA, password: PASSWORD } ].freeze
  TENANTS = [ { id: "tenant-1", name: "Acme research", members: { ANA => "owner", BIA => "admin", "caio@markupp.dev" => "member" } } ].freeze

  test "all builds the members of the active tenant with role and join date" do
    joined_at = Time.utc(2026, 9, 1, 12)

    assert_equal [ TenantMember.new(id: "account-1", email: ANA, role: "owner", joined_at:),
                   TenantMember.new(id: "account-2", email: BIA, role: "admin", joined_at:),
                   TenantMember.new(id: "account-caio", email: "caio@markupp.dev", role: "member", joined_at:) ],
                 TenantMember.all(api_as(ANA))
  end

  test "change_role gives the member the new role" do
    api = api_as(ANA)

    assert_nil TenantMember.change_role(api, "account-caio", "viewer")
    assert_equal "viewer", TenantMember.all(api).find { it.id == "account-caio" }.role
  end

  test "change_role raises LastOwner when the only owner steps down" do
    assert_raises(MarkuppApi::LastOwner) { TenantMember.change_role(api_as(ANA), "account-1", "admin") }
  end

  test "change_role raises RoleAboveOwn when an admin touches the owner" do
    assert_raises(MarkuppApi::RoleAboveOwn) { TenantMember.change_role(api_as(BIA), "account-1", "member") }
  end

  test "remove drops the member from the tenant" do
    api = api_as(ANA)

    assert_nil TenantMember.remove(api, "account-caio")
    assert_equal [ ANA, BIA ], TenantMember.all(api).map(&:email)
  end

  test "remove raises MemberNotFound for someone outside the tenant" do
    assert_raises(MarkuppApi::MemberNotFound) { TenantMember.remove(api_as(ANA), "account-9") }
  end

  test "changeable_by? is true for members up to the role of the actor" do
    admin = Tenant.new(id: "tenant-1", name: "Acme research", role: "admin")

    assert_equal({ "owner" => false, "admin" => true, "member" => true, "viewer" => true },
                 Tenant::ROLES.index_with { TenantMember.new(id: "account-9", email: "x@markupp.dev", role: it, joined_at: nil).changeable_by?(admin) })
  end

  private
    def api_as(email)
      fake_api = FakeMarkuppApi.new(accounts: ACCOUNTS, tenants: TENANTS)
      pair = fake_api.post("/auth/login", email:, password: PASSWORD)
      fake_api.as(access_token: pair.fetch("access_token"), tenant_id: "tenant-1")
    end
end
