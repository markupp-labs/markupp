require "test_helper"

class PendingInviteTest < ActiveSupport::TestCase
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL
  TENANTS = [ { id: "tenant-1", name: "Acme research", members: { EMAIL => "owner" } },
              { id: "tenant-2", name: "Home lab", members: { "bia@markupp.dev" => "owner" } } ].freeze
  INVITE = { id: "invite-1", tenant_id: "tenant-2", email: EMAIL, role: "member", expires_at: "2099-10-23T12:00:00Z" }.freeze

  test "mine builds the pending invites of the account with the tenant, the role and the expiry" do
    assert_equal [ PendingInvite.new(id: "invite-1", tenant_id: "tenant-2", tenant_name: "Home lab", role: "member",
                                     expires_at: Time.utc(2099, 10, 23, 12)) ],
                 PendingInvite.mine(signed_in_api)
  end

  test "mine is empty while the email of the account is not confirmed" do
    assert_empty PendingInvite.mine(signed_in_api(accounts: [ { email: EMAIL, password: FakeMarkuppAccounts::DEFAULT_PASSWORD, verified: false } ]))
  end

  test "accept answers the tenant joined with the role of the invite and the invite leaves the list" do
    api = signed_in_api

    assert_equal Tenant.new(id: "tenant-2", name: "Home lab", role: "member"), PendingInvite.accept(api, "invite-1")
    assert_empty PendingInvite.mine(api)
  end

  test "accept raises InviteNotFound for an invite that is not pending for the account" do
    assert_raises(MarkuppApi::InviteNotFound) { PendingInvite.accept(signed_in_api, "invite-9") }
  end

  test "invite not found is a kind of NotFound" do
    assert_operator MarkuppApi::InviteNotFound, :<, MarkuppApi::NotFound
  end

  test "accept raises AlreadyMember for a tenant the account is already in" do
    api = signed_in_api(invites: [ INVITE.merge(tenant_id: "tenant-1") ])

    assert_raises(MarkuppApi::AlreadyMember) { PendingInvite.accept(api, "invite-1") }
  end

  test "decline drops the invite without joining the tenant" do
    api = signed_in_api

    assert_nil PendingInvite.decline(api, "invite-1")
    assert_empty PendingInvite.mine(api)
    assert_equal [ "tenant-1" ], Tenant.mine(api).map(&:id)
  end

  private
    def signed_in_api(accounts: FakeMarkuppAccounts::DEFAULT_ACCOUNTS, invites: [ INVITE ])
      fake_api = FakeMarkuppApi.new(accounts:, tenants: TENANTS, invites:)
      pair = fake_api.post("/auth/login", email: EMAIL, password: FakeMarkuppAccounts::DEFAULT_PASSWORD)
      fake_api.as(access_token: pair.fetch("access_token"))
    end
end
