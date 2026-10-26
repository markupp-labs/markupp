require "test_helper"

class AccountNotificationTest < ActiveSupport::TestCase
  EMAIL = FakeMarkuppAccounts::DEFAULT_EMAIL
  TENANTS = [ { id: "tenant-1", name: "Acme research", members: { EMAIL => "owner" } },
              { id: "tenant-2", name: "Home lab", members: { "bia@markupp.dev" => "owner" } } ].freeze
  INVITE = { id: "invite-1", tenant_id: "tenant-2", email: EMAIL, role: "member", expires_at: "2099-10-23T12:00:00Z" }.freeze

  test "feed builds the pending invites of the account as invite notifications with the count" do
    invite = PendingInvite.new(id: "invite-1", tenant_id: "tenant-2", tenant_name: "Home lab", role: "member",
                               expires_at: Time.utc(2099, 10, 23, 12))

    assert_equal AccountNotification::Feed.new(items: [ AccountNotification.new(kind: "invite", invite:) ], unread: 1),
                 AccountNotification.feed(signed_in_api)
  end

  test "feed is empty without pending invites" do
    assert_equal AccountNotification::Feed.new(items: [], unread: 0), AccountNotification.feed(signed_in_api(invites: []))
  end

  test "feed reads the account route under v1, without a tenant" do
    api = signed_in_api

    AccountNotification.feed(api)

    assert_equal "GET /v1/me/notifications", api.received_credentials.last[:call]
  end

  test "a kind the panel does not know stays out of the list" do
    body = { "items" => [ { "kind" => "mention", "id" => "m1" } ], "unread" => 1 }

    assert_empty AccountNotification::Feed.from_api(body).items
  end

  private
    def signed_in_api(invites: [ INVITE ])
      fake_api = FakeMarkuppApi.new(tenants: TENANTS, invites:)
      pair = fake_api.post("/auth/login", email: EMAIL, password: FakeMarkuppAccounts::DEFAULT_PASSWORD)
      fake_api.as(access_token: pair.fetch("access_token"))
    end
end
