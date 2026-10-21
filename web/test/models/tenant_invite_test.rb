require "test_helper"

class TenantInviteTest < ActiveSupport::TestCase
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL
  TENANTS = [ { id: "tenant-1", name: "Acme research", members: { EMAIL => "owner", "caio@markupp.dev" => "member" } } ].freeze
  INVITES = [
    { id: "invite-1", tenant_id: "tenant-1", email: "eva@markupp.dev", role: "member", expires_at: "2099-10-23T12:00:00Z",
      accepted_at: "2026-10-02T12:00:00Z", created_at: "2026-10-01T12:00:00Z" },
    { id: "invite-2", tenant_id: "tenant-1", email: "fabi@markupp.dev", role: "viewer", expires_at: "2020-01-08T12:00:00Z", created_at: "2020-01-01T12:00:00Z" },
    { id: "invite-3", tenant_id: "tenant-1", email: "dani@markupp.dev", role: "admin", expires_at: "2099-10-23T12:00:00Z", created_at: "2026-10-01T12:00:00Z" }
  ].freeze

  test "all builds the invites of the tenant with the pending ones first" do
    invites = TenantInvite.all(signed_in_api)

    assert_equal %w[invite-3 invite-1 invite-2], invites.map(&:id)
    assert_equal %w[pending accepted expired], invites.map(&:status)
    assert_equal Time.utc(2099, 10, 23, 12), invites.first.expires_at
  end

  test "pending? is true only for the pending status" do
    assert_equal [ true, false, false ], TenantInvite.all(signed_in_api).map(&:pending?)
  end

  test "issue answers the new pending invite without a token when the server sends email" do
    invite, token = TenantInvite.issue(signed_in_api, email: "gil@markupp.dev", role: "member")

    assert_equal [ "gil@markupp.dev", "member", "pending" ], [ invite.email, invite.role, invite.status ]
    assert_nil token
  end

  test "issue answers the token when the server has no email" do
    _invite, token = TenantInvite.issue(signed_in_api(email_enabled: false), email: "gil@markupp.dev", role: "member")

    assert_equal "issued-token-1", token
  end

  test "issue raises AlreadyMember for an email already in the tenant" do
    assert_raises(MarkuppApi::AlreadyMember) { TenantInvite.issue(signed_in_api, email: "caio@markupp.dev", role: "member") }
  end

  test "issue raises InvalidRequest for an email the API refuses" do
    assert_raises(MarkuppApi::InvalidRequest) { TenantInvite.issue(signed_in_api, email: "gil", role: "member") }
  end

  test "revoke drops a pending invite" do
    api = signed_in_api

    assert_nil TenantInvite.revoke(api, "invite-3")
    assert_equal %w[invite-1 invite-2], TenantInvite.all(api).map(&:id)
  end

  test "revoke raises InviteNotFound for an invite already answered" do
    assert_raises(MarkuppApi::InviteNotFound) { TenantInvite.revoke(signed_in_api, "invite-1") }
  end

  private
    def signed_in_api(email_enabled: true)
      fake_api = FakeMarkuppApi.new(tenants: TENANTS, invites: INVITES, email_enabled:)
      pair = fake_api.post("/auth/login", email: EMAIL, password: FakeMarkuppAccounts::DEFAULT_PASSWORD)
      fake_api.as(access_token: pair.fetch("access_token"), tenant_id: "tenant-1")
    end
end
